# snix — Cloudflare Worker template

A minimal VLESS-over-WebSocket server that runs on Cloudflare's free tier.
Pair it with snix on your device when you don't have a dedicated VPS.

## One-time setup

1. Create a free [Cloudflare account](https://dash.cloudflare.com/sign-up)
   if you don't have one.
2. In the dashboard, go to **Workers & Pages → Create application → Create Worker**.
3. Replace the editor contents with [worker.js](worker.js).
4. Go to **Settings → Variables** and add:
   - `UUID` — any v4 UUID. Generate one at <https://www.uuidgenerator.net/>
     or let `snix init --wizard` generate one for you.
5. Click **Deploy**. Copy the `*.workers.dev` URL.

## Point snix at it

In `snix init --wizard`, when asked for your server:

```
Host:  <your-worker>.workers.dev
Port:  443
```

Record the UUID — your proxy client will need it.

## Point your proxy client at snix + the worker

In Xray / v2ray / sing-box / NekoBox:

| Field | Value |
|---|---|
| Protocol   | VLESS |
| Server     | `127.0.0.1` (snix's listen address) |
| Port       | `40443` |
| UUID       | the one you set in the Worker |
| Encryption | none |
| Transport  | WebSocket |
| Path       | `/?ed=2048` |
| TLS        | on |
| SNI        | `<your-worker>.workers.dev` |

snix then speaks SNI-spoofing to the network; the real TLS handshake goes
to your Worker.

## Limits

Cloudflare's free tier gives:
- 100 000 Worker requests per day
- 30 s max CPU time per request
- ~30 MB Worker memory
- No long-lived connection parking (server-side reconnection is handled by your client)

For heavy use, upgrade to the Paid plan ($5/month includes 10M requests).

## Security

- The Worker only accepts traffic from clients that know the UUID you set.
- **Do not commit your UUID to public repos.** Keep it out of git.
- If you want to further restrict who can use the Worker, add an
  `Authorization` header check inside `handleVLESS()` — see the code
  comments for the insertion point.
