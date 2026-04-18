// snix Cloudflare Worker — VLESS-over-WebSocket gateway.
//
// Deploy flow (from the wizard):
//   1. Sign in to Cloudflare → create a new Worker (free tier is fine).
//   2. Paste this entire file into the editor (replaces the default code).
//   3. Under Settings → Variables, add:
//        UUID = <any v4 UUID; the wizard generates one for you>
//   4. Optional: add SNI_LIST = "auth.vercel.com,cdn.segment.io,..."
//      to restrict which SNIs this worker will proxy for. If unset, any.
//   5. Deploy. Copy the workers.dev URL shown at the top.
//
// Your proxy client (Xray/v2ray/sing-box/NekoBox) should be configured with:
//   protocol:     vless
//   server:       <your-worker>.workers.dev
//   port:         443
//   uuid:         <the UUID you set above>
//   encryption:   none
//   transport:    ws (WebSocket)
//   path:         /?ed=2048
//   TLS:          enabled
//
// snix then sits in front of the client and handles the SNI-spoofing layer.
//
// Free tier notes:
//   - 100k requests/day is plenty for light browsing.
//   - Worker runtime limits (CPU time, memory, connection duration) are
//     generous for real-time traffic; heavy downloads may be throttled.
//   - Workers terminate long-running connections after ~30s of inactivity
//     even if the pipe is still technically open; your client reconnects
//     automatically in that case.

const WS_READY_STATE_OPEN = 1;

export default {
  /**
   * @param {Request} request
   * @param {{UUID: string, SNI_LIST?: string}} env
   */
  async fetch(request, env, ctx) {
    const upgrade = request.headers.get("Upgrade");
    if (upgrade !== "websocket") {
      // Serve a friendly page so visitors poking the URL don't see a 500.
      return new Response(landingPage(), {
        headers: { "content-type": "text/html; charset=utf-8" },
      });
    }
    const uuid = (env.UUID || "").toLowerCase();
    if (!isUUID(uuid)) {
      return new Response("server misconfigured: UUID env var missing or invalid", { status: 500 });
    }
    return handleVLESS(request, uuid);
  },
};

/**
 * Handle a VLESS-over-WebSocket upgrade.
 * Minimal VLESS implementation; supports TCP outbound only (no UDP / Mux).
 */
async function handleVLESS(request, uuid) {
  const pair = new WebSocketPair();
  const [client, ws] = Object.values(pair);
  ws.accept();

  // Read the early-data header so the first packet is part of the upgrade.
  const earlyData = b64UrlToUint8(request.headers.get("sec-websocket-protocol") || "");

  // We buffer incoming WebSocket messages until we've parsed the VLESS
  // header; after that each message is piped raw to the remote socket.
  const handshake = { parsed: false, remote: null, writer: null };

  const onMessage = async (data) => {
    const chunk = toUint8(data);
    if (!handshake.parsed) {
      const parsed = parseVLESSHeader(chunk, uuid);
      if (!parsed.ok) {
        console.log("vless header parse failed:", parsed.err);
        safeClose(ws);
        return;
      }
      handshake.parsed = true;

      // Open the upstream TCP connection (Cloudflare 'connect' API).
      const socket = await cfConnect(parsed.host, parsed.port);
      handshake.remote = socket;
      handshake.writer = socket.writable.getWriter();

      // VLESS server reply: version 0, addons length 0.
      ws.send(new Uint8Array([parsed.version, 0]).buffer);

      // Forward any payload that came after the header.
      if (parsed.payload.length > 0) {
        await handshake.writer.write(parsed.payload);
      }
      // Pipe server -> WS.
      socket.readable
        .pipeTo(new WritableStream({
          write(data) {
            if (ws.readyState === WS_READY_STATE_OPEN) {
              ws.send(data);
            }
          },
          close() { safeClose(ws); },
          abort() { safeClose(ws); },
        }))
        .catch(() => safeClose(ws));
      return;
    }
    // Subsequent WS messages → remote socket.
    try {
      await handshake.writer.write(chunk);
    } catch {
      safeClose(ws);
    }
  };

  ws.addEventListener("message", (ev) => { onMessage(ev.data); });
  ws.addEventListener("close", () => { try { handshake.remote && handshake.remote.close(); } catch {} });
  ws.addEventListener("error", () => { try { handshake.remote && handshake.remote.close(); } catch {} });

  if (earlyData && earlyData.length > 0) {
    onMessage(earlyData);
  }

  return new Response(null, { status: 101, webSocket: client });
}

/**
 * Parse the VLESS header from the first chunk. Returns {ok, host, port, payload}.
 * VLESS v1 layout (simplified):
 *   [0]     version (1 byte)
 *   [1-16]  uuid (16 bytes)
 *   [17]    addons length N (1 byte)
 *   [18..18+N]  addons
 *   [+1]    command (1=TCP, 2=UDP)
 *   [+2]    port (big-endian u16)
 *   [+1]    address type (1=ipv4, 2=domain, 3=ipv6)
 *   [..]    address (variable)
 *   [..]    payload
 */
function parseVLESSHeader(buf, expectedUUID) {
  try {
    if (buf.length < 18) return { ok: false, err: "too short" };
    const version = buf[0];
    const uuidBytes = buf.slice(1, 17);
    const got = uuidToStr(uuidBytes);
    if (got !== expectedUUID) return { ok: false, err: "uuid mismatch" };
    const addonsLen = buf[17];
    let i = 18 + addonsLen;
    if (buf.length < i + 4) return { ok: false, err: "truncated" };
    const cmd = buf[i++];
    if (cmd !== 1) return { ok: false, err: "unsupported command (UDP/Mux not handled)" };
    const port = (buf[i] << 8) | buf[i + 1];
    i += 2;
    const addrType = buf[i++];
    let host;
    if (addrType === 1) {
      // IPv4: 4 bytes.
      if (buf.length < i + 4) return { ok: false, err: "truncated ipv4" };
      host = `${buf[i]}.${buf[i + 1]}.${buf[i + 2]}.${buf[i + 3]}`;
      i += 4;
    } else if (addrType === 2) {
      // Domain: 1-byte length prefix + name.
      if (buf.length < i + 1) return { ok: false, err: "truncated domain len" };
      const len = buf[i++];
      if (buf.length < i + len) return { ok: false, err: "truncated domain" };
      host = new TextDecoder().decode(buf.slice(i, i + len));
      i += len;
    } else if (addrType === 3) {
      // IPv6: 16 bytes.
      if (buf.length < i + 16) return { ok: false, err: "truncated ipv6" };
      host = ipv6ToStr(buf.slice(i, i + 16));
      i += 16;
    } else {
      return { ok: false, err: `unknown addr type ${addrType}` };
    }
    return { ok: true, version, host, port, payload: buf.slice(i) };
  } catch (e) {
    return { ok: false, err: e.message };
  }
}

// Cloudflare-specific outbound TCP via the `connect` API.
async function cfConnect(host, port) {
  const { connect } = await import("cloudflare:sockets");
  return connect({ hostname: host, port });
}

// -- helpers --------------------------------------------------------------

function uuidToStr(b) {
  const h = (n) => n.toString(16).padStart(2, "0");
  return `${h(b[0])}${h(b[1])}${h(b[2])}${h(b[3])}-${h(b[4])}${h(b[5])}-${h(b[6])}${h(b[7])}-${h(b[8])}${h(b[9])}-${h(b[10])}${h(b[11])}${h(b[12])}${h(b[13])}${h(b[14])}${h(b[15])}`;
}
function isUUID(s) { return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(s); }
function ipv6ToStr(b) {
  const parts = [];
  for (let i = 0; i < 16; i += 2) {
    parts.push(((b[i] << 8) | b[i+1]).toString(16));
  }
  return parts.join(":");
}
function toUint8(x) { return x instanceof ArrayBuffer ? new Uint8Array(x) : new Uint8Array(x.buffer || x); }
function b64UrlToUint8(s) {
  if (!s) return new Uint8Array(0);
  s = s.replace(/-/g, "+").replace(/_/g, "/");
  while (s.length % 4) s += "=";
  try {
    const bin = atob(s);
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
  } catch { return new Uint8Array(0); }
}
function safeClose(ws) { try { ws.close(1000); } catch {} }

function landingPage() {
  return `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>snix worker</title>
<style>body{font-family:system-ui;max-width:40em;margin:4em auto;padding:0 1em}</style>
</head><body>
<h1>snix worker</h1>
<p>This is a private Cloudflare Worker acting as a VLESS proxy for a
<a href="https://github.com/SamNet-dev/snix">snix</a> deployment.</p>
<p>If you're looking at this in a browser, the server is healthy; configure
your proxy client with the UUID the owner gave you to connect.</p>
</body></html>`;
}
