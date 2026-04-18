# snix — The Complete Zero-to-Finish Guide

This is the long-form walkthrough. Start here if this is your first time
touching a circumvention tool. If you know what you're doing, the
[README](README.md) has the short version.

Everything you need to go from **nothing on your computer** to **a working,
hardened, daily-driver bypass setup** is in this file.

## Table of contents

1. [What this guide covers](#1-what-this-guide-covers)
2. [Mental model: what are all the pieces?](#2-mental-model-what-are-all-the-pieces)
3. [Decide your setup](#3-decide-your-setup)
4. [Part A — Get a proxy server](#4-part-a--get-a-proxy-server)
   - [A1. Cloudflare Worker (free, easiest)](#a1-cloudflare-worker-free-easiest)
   - [A2. VPS server (paid, flexible)](#a2-vps-server-paid-flexible)
   - [A3. Use a server someone gave you](#a3-use-a-server-someone-gave-you)
5. [Part B — Optional: get a custom domain](#5-part-b--optional-get-a-custom-domain)
6. [Part C — Install and configure the proxy client](#6-part-c--install-and-configure-the-proxy-client)
7. [Part D — Install snix](#7-part-d--install-snix)
8. [Part E — First-run wizard](#8-part-e--first-run-wizard)
9. [Part F — Start using it](#9-part-f--start-using-it)
10. [Part G — Harden against stronger DPI](#10-part-g--harden-against-stronger-dpi)
11. [Part H — Day 2 operations](#11-part-h--day-2-operations)
12. [Troubleshooting by symptom](#12-troubleshooting-by-symptom)
13. [Appendix A — Full glossary](#13-appendix-a--full-glossary)
14. [Appendix B — Security hygiene](#14-appendix-b--security-hygiene)
15. [Appendix C — Uninstall cleanly](#15-appendix-c--uninstall-cleanly)

---

## 1. What this guide covers

At the end of this guide you will have:

- A proxy server running somewhere outside the censored network.
- A proxy client (Xray or sing-box) installed and configured.
- snix installed, configured, and running with anti-fingerprinting on.
- A working HTTPS connection through both, verified with a test site.
- Know how to maintain, update, and troubleshoot your setup.

**Total time: 15–45 minutes** depending on which path you pick.

**Platforms covered here**: Linux (every major distro), Windows 10/11.
Android is planned but not yet shipped. macOS works today for the
client/scanner CLI but the bypass engine there is still in development.

**What you need before you start**:

- A computer where you'll run snix (Linux or Windows).
- A phone or second device with mobile data OR access to an unblocked
  network (e.g. guest wifi, a friend's tether) — you need this to initially
  sign up for Cloudflare, since that's the only tricky piece if you're
  behind a DPI right now.
- About 15 minutes of focused attention. Wizards have been written so
  you're not improvising at any step.

**What you do NOT need**:

- A paid VPN or VPS (unless you want one — Path A2 below).
- Any coding knowledge.
- A custom domain (it's optional — Part B).
- Linux experience beyond running `sudo` and pasting commands.

---

## 2. Mental model: what are all the pieces?

Before you start, understand how the parts fit together. You will make
better choices through the rest of this guide.

```
[your device, the censored side]                      [the uncensored internet]
                                                     
  browser / app                                       your proxy server
     │                                                (Cloudflare Worker
     │  (app talks to a local proxy)                   or a VPS)
     ▼                                                    ▲
  proxy client                                            │
  (Xray / sing-box)                                       │
     │                                                    │
     │  (its "outbound" is 127.0.0.1:40443                │
     │   instead of the real server)                      │
     ▼                                                    │
  snix on 127.0.0.1:40443 ──────── internet + DPI ────────┘
  (adds the SNI-spoofing trick to
   every connection going out)
```

Four moving parts. You install or arrange three of them on your device:

1. **The proxy server** — lives on the internet. You don't install it on
   your computer; you either rent it or deploy it to someone else's
   infrastructure (Cloudflare). Runs something like VLESS / VMess / Trojan.
2. **The proxy client** — on your device. Xray / sing-box / NekoBox — a
   local program that speaks the proxy protocol to the proxy server.
3. **snix** — on your device. Sits *between* the proxy client and the
   internet, so that when the proxy client tries to reach the proxy server
   the TLS handshake gets the SNI-spoof treatment.
4. **Your apps** (browser, etc.) — pointed at the proxy client.

**Why can't snix just be a VPN by itself?** Because it doesn't speak any
proxy protocol. It only tricks the DPI. You still need a proxy protocol
(that's what the server + client give you) to actually get data to and
from the outside world.

---

## 3. Decide your setup

Three big decisions first.

### Decision 1 — Free or paid server?

|                | **Free (Cloudflare Worker)**                | **Paid (VPS)**                         |
|---|---|---|
| Upfront cost    | $0                                          | $3–$10 / month typical                 |
| Setup time      | 5 minutes in a browser                      | 15–30 minutes, one-time                |
| Good for        | Browsing, messaging, light video            | Heavy video, torrents, many devices    |
| Speed           | 15–50 Mbps typical (CF free tier)           | Whatever the VPS provides              |
| Bandwidth cap   | 100k HTTP requests/day (plenty for a person)| Usually unlimited                      |
| Hard to block   | Very (shares IP with millions of CF sites)  | Moderate (one IP per server)           |
| Skill needed    | None                                        | Copy-paste SSH commands                |

**Recommendation: start with Cloudflare Worker (Path A1).** You can always
upgrade to a VPS later. The setup steps for snix itself are identical
either way.

### Decision 2 — Domain?

Do you want a nice-looking URL like `proxy.yourname.com` instead of
`snix-ab12cd.yourname.workers.dev`?

- **No**: skip Part B. Everything works without a domain.
- **Yes**: Part B walks you through it. Adds ~10 minutes + $10/year.

Most users: **skip the domain** until you have something working. You can
add it later in 5 minutes.

### Decision 3 — Which proxy client?

snix works with anything that speaks VLESS / VMess / Trojan / Shadowsocks.
Your options:

| Client     | Platforms           | Good for                                        |
|---|---|---|
| **Xray**   | Linux, Windows      | Most flexible; our default recommendation.      |
| sing-box   | Linux, Windows      | Newer, cleaner config format.                   |
| v2rayN     | Windows             | Has a GUI — good if you dislike terminal.       |
| NekoBox    | Android only        | Mobile; we don't yet support Android but listed |
|            |                     | for future reference.                           |

**Recommendation: Xray.** The Cloudflare Worker template in this repo is
designed for VLESS which Xray handles natively.

---

## 4. Part A — Get a proxy server

Pick **one** of A1, A2, or A3.

### A1. Cloudflare Worker (free, easiest)

You'll deploy a small piece of code to Cloudflare's free "Workers"
platform. It acts as your proxy server. Zero servers to maintain, zero
cost, Cloudflare does the TLS and DDoS protection for free.

#### A1.1 Make a Cloudflare account

You may need a device on an **unblocked network** for this step, because
you'll be signing up for Cloudflare which some ISPs partially block.
Options:

- Mobile data on your phone
- A friend's wifi or a hotspot
- An existing (non-snix) VPN you already have

If you have zero alternative networks, skip to Path A2 (VPS) which doesn't
require Cloudflare.

1. Open <https://dash.cloudflare.com/sign-up> in a browser.
2. Sign up with an email + password. Verify the email.
3. You're done for now. You do NOT need to add a domain or payment card.
   The free tier is genuinely free.

#### A1.2 Generate a UUID for authentication

Your Worker will only accept traffic that knows a secret UUID. Generate
one now and keep it safe.

Pick one method:

- Open <https://www.uuidgenerator.net/version4> in a browser and copy the
  UUID shown.
- On Linux: `cat /proc/sys/kernel/random/uuid`.
- On Windows PowerShell: `[guid]::NewGuid().ToString()`.
- snix will also generate one for you in the wizard later — if you
  prefer, skip this and note the one the wizard prints.

Example (yours will be different):
```
f47ac10b-58cc-4372-a567-0e02b2c3d479
```
**Save this somewhere.** You'll paste it into Cloudflare in a minute and
into your proxy client later. Treat it like a password — anyone who has
this UUID can use your Worker.

#### A1.3 Deploy the Worker

1. Sign in at <https://dash.cloudflare.com/>.
2. In the left sidebar click **Workers & Pages**.
3. Click **Create application → Create Worker**.
4. Name it something like `snix` (you can change later). Click **Deploy**.
   Cloudflare drops you into a default "hello world" Worker. Fine.
5. Click **Edit code** in the top right.
6. In your snix repo, open the file [`cfworker/worker.js`](cfworker/worker.js).
   Copy its entire contents.
7. Back in the Cloudflare editor, delete everything on the left side and
   paste in the `worker.js` contents.
8. Click **Save and deploy**.
9. Now add your UUID as an environment variable:
   - Click **← Back to service** (top-left).
   - Click the **Settings** tab.
   - Click **Variables and Secrets**.
   - Click **+ Add**.
   - Name: `UUID`, Value: your UUID from A1.2, Type: **Plaintext**
     (leave "encrypt" off for now).
   - Click **Save and deploy**.
10. At the top of the Worker page, you'll see a URL ending in
    `.workers.dev`. Something like `snix.yourname.workers.dev`. Copy it.

You now have a working proxy server. Verify by opening the URL in a
browser — you should see a "snix worker" landing page.

#### A1.4 Write down your credentials

You now need these three things to finish setup. Write them down (not on
your main computer if it's being monitored):

- **Worker host**: `snix.yourname.workers.dev`
- **UUID**: the one from A1.2
- **Port**: `443` (always, for Workers)
- **Protocol**: VLESS over WebSocket with TLS, path `/?ed=2048`

Continue to [Part B](#5-part-b--optional-get-a-custom-domain) (optional)
or [Part C](#6-part-c--install-a-proxy-client).

---

### A2. VPS server (paid, flexible)

If you already have a Linux VPS with a public IP, or you want to buy one,
this path gives you a more flexible server without Worker limits.

#### A2.1 Get a VPS

Cheap Linux VPS providers:
- **RackNerd** — often $10–$15/year promos. US/EU/Asia.
- **BuyVM** — $3.50/month, good network.
- **Vultr** — $5/month, global.
- **Hetzner** — €4/month, Europe.

When you order, pick:
- OS: **Ubuntu 22.04 LTS** (what this guide assumes).
- Arch: **amd64** (standard).
- Location: somewhere NOT blocked from your ISP. Good default: Germany or
  Netherlands for most of the world.

You'll get:
- An IP address (write it down).
- An SSH password (or an SSH key you uploaded).

#### A2.2 SSH into the VPS

From Linux/macOS or Windows PowerShell:

```bash
ssh root@YOUR.VPS.IP
```

Enter the password the provider gave you when prompted. You should see a
prompt like `root@vps:~#`.

**Immediately change the password** to something strong:
```bash
passwd
```

Type the new password twice. Don't forget it.

#### A2.3 Install Xray as a VLESS server

We'll install Xray on the VPS to receive proxied traffic on port 443.

```bash
# 1. Install Xray via the official one-liner.
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install

# 2. Generate a UUID.
xray uuid

# (copy the UUID printed; you'll paste it below)

# 3. Open the Xray config for editing.
nano /usr/local/etc/xray/config.json
```

Replace the file contents with this (paste the UUID you generated):

```json
{
  "log": { "loglevel": "warning" },
  "inbounds": [
    {
      "port": 443,
      "protocol": "vless",
      "settings": {
        "clients": [
          { "id": "PASTE-YOUR-UUID-HERE", "flow": "" }
        ],
        "decryption": "none"
      },
      "streamSettings": {
        "network": "tcp",
        "security": "tls",
        "tlsSettings": {
          "certificates": [
            {
              "certificateFile": "/etc/xray/fullchain.pem",
              "keyFile":         "/etc/xray/privkey.pem"
            }
          ]
        }
      }
    }
  ],
  "outbounds": [ { "protocol": "freedom" } ]
}
```

Save: Ctrl+O, Enter, Ctrl+X.

You need a TLS certificate. You have two options:

**Option A (recommended)** — use a domain you own: see
[Part B](#5-part-b--optional-get-a-custom-domain) to buy + point a
domain, then run on the VPS:
```bash
apt update && apt install -y certbot
certbot certonly --standalone -d proxy.yourname.com
mkdir -p /etc/xray
cp /etc/letsencrypt/live/proxy.yourname.com/fullchain.pem  /etc/xray/
cp /etc/letsencrypt/live/proxy.yourname.com/privkey.pem    /etc/xray/
chown -R nobody:nogroup /etc/xray
chmod 600 /etc/xray/privkey.pem
```

**Option B** — self-signed cert (uglier but functional):
```bash
mkdir -p /etc/xray
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout /etc/xray/privkey.pem \
  -out    /etc/xray/fullchain.pem \
  -subj "/CN=your-vps-ip" -days 3650
```
Your client will need to allow "insecure" TLS later (we'll cover this).

Now start Xray:
```bash
systemctl restart xray
systemctl enable  xray
systemctl status  xray       # should say "active (running)"
```

Open firewall port 443 if your VPS has a firewall:
```bash
ufw allow 443/tcp
# or: iptables rule; depends on the distro.
```

Write down:
- **Server host**: your VPS IP (or domain if you did Option A).
- **UUID**: the one you generated.
- **Port**: 443.
- **Protocol**: VLESS over TCP with TLS.

Continue to [Part C](#6-part-c--install-a-proxy-client).

---

### A3. Use a server someone gave you

If a friend or paid service gave you a proxy config already, skip the
server setup. You should have been told:

- Server host (domain or IP)
- Port (usually 443)
- Protocol (VLESS / VMess / Trojan / …)
- UUID or password
- Transport (TCP / WebSocket / gRPC)
- TLS settings (SNI, whether to skip verification)

Write these down and continue to [Part C](#6-part-c--install-a-proxy-client).

---

## 5. Part B — Optional: get a custom domain

Skip this if you're fine with `*.workers.dev` or just using the VPS IP.
Come back later if you want a prettier URL.

### B1. Buy a domain

Cheap registrars (all support A/CNAME records, which is what we need):

- **Cloudflare Registrar** — at-cost pricing, about $9/year for `.com`.
  No upsells.
- **Porkbun** — $9–$12/year, clean UI.
- **Namecheap** — well-known, slightly more expensive.

Pick any domain. Doesn't need to be short. `my-proxy-2026.com` is fine.

### B2. Point your domain at your Cloudflare Worker

(Skip to [B3](#b3-point-your-domain-at-a-vps) if you're on the VPS path.)

The modern Cloudflare approach handles DNS + TLS + routing in one click.
No Workers Routes, no AAAA tricks.

1. If you bought the domain at **Cloudflare Registrar**, it's already on
   Cloudflare DNS. Skip to step 4.
2. Otherwise, in the Cloudflare dashboard, click **Add a site**, paste
   your domain, pick the **Free plan**, and click through.
3. Cloudflare tells you its two nameservers (`aaron.ns.cloudflare.com`
   etc.). Go to your registrar, change the nameservers to those two. Wait
   5–60 minutes for propagation.
4. In Cloudflare, open **Workers & Pages → your `snix` worker**.
5. Click the **Settings** tab, then **Triggers** on the left.
6. Under **Custom Domains**, click **Add Custom Domain**.
7. Enter `proxy.yourname.com` and click **Add Custom Domain**.
8. Cloudflare automatically creates the DNS record and provisions a TLS
   certificate. Takes 30–60 seconds.

Verify:
```bash
curl -sI https://proxy.yourname.com/
```
Should return `200 OK` and HTML headers from the "snix worker" landing
page.

Use `proxy.yourname.com` in place of `xxx.workers.dev` everywhere from
now on (in your snix config, in Xray's config, in the wizard prompts).

### B3. Point your domain at a VPS

(Skip this if you're on the Worker path.)

1. In Cloudflare DNS (or your registrar's DNS panel), add an **A record**:
   - Name: `proxy`
   - IPv4 address: your VPS IP
   - Proxy status: **DNS only** (grey cloud). We are NOT proxying through
     Cloudflare because you're terminating TLS on the VPS yourself.
2. Save. Wait a couple of minutes.
3. `curl -k https://proxy.yourname.com` should reach your VPS.
4. On the VPS, run `certbot` to get a real TLS cert for the domain (see
   A2.3 Option A).

Use `proxy.yourname.com` in place of the VPS IP from now on.

---

## 6. Part C — Install and configure the proxy client

You need **one** client (Xray in this guide). Install it on your local
device, NOT on the VPS.

**Two stages:**
1. **Install** the Xray binary (C1 / C2 / C3 below).
2. **Configure** it with your server details so Xray knows how to talk to
   the Worker / VPS (C4 below — has ready-to-paste templates).

Later, in [Part E](#8-part-e--first-run-wizard), snix's wizard rewrites
just the `address` and `port` fields in this config to point at snix's
local listener. Everything else (your UUID, TLS settings, WebSocket path)
stays as you set it up here.

### C1. Install on Linux

```bash
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install
```

After install:
- Binary: `/usr/local/bin/xray`
- Config: `/usr/local/etc/xray/config.json`
- systemd service: `xray.service`

### C2. Install on Windows

Two options:

- **v2rayN** (GUI, easier): download from
  <https://github.com/2dust/v2rayN/releases>. Runs as a tray app.
  Config lives in `%APPDATA%\v2rayN\`.
- **Xray core** (CLI, lighter): download from
  <https://github.com/XTLS/Xray-core/releases>, extract to e.g.
  `C:\Program Files\xray\`, and run `xray.exe run -c config.json`.

### C3. Install on macOS

```bash
brew install xray
```
Config goes to `~/.config/xray/config.json`.

### C4. Write your Xray client config

Pick the template that matches how you set up your server in Part A.

Both templates:
- Open a local SOCKS5 inbound on `127.0.0.1:1080` (what your browser
  talks to).
- Send all traffic out as VLESS to your server.
- Will be auto-patched in Part E so the `address`/`port` fields point at
  snix's local listener instead of your real server.

**Template 1 — Cloudflare Worker (Path A1)**

Save as `/usr/local/etc/xray/config.json` on Linux, or the equivalent on
your OS:

```json
{
  "log": { "loglevel": "warning" },
  "inbounds": [
    {
      "listen": "127.0.0.1",
      "port": 1080,
      "protocol": "socks",
      "settings": { "auth": "noauth", "udp": true, "ip": "127.0.0.1" }
    }
  ],
  "outbounds": [
    {
      "protocol": "vless",
      "settings": {
        "vnext": [
          {
            "address": "snix.yourname.workers.dev",
            "port": 443,
            "users": [
              { "id": "PASTE-YOUR-UUID-HERE", "encryption": "none" }
            ]
          }
        ]
      },
      "streamSettings": {
        "network": "ws",
        "security": "tls",
        "tlsSettings": {
          "serverName": "snix.yourname.workers.dev",
          "allowInsecure": false
        },
        "wsSettings": {
          "path": "/?ed=2048",
          "headers": { "Host": "snix.yourname.workers.dev" }
        }
      }
    }
  ]
}
```

Replace:
- `snix.yourname.workers.dev` with your actual Worker hostname (or your
  custom domain from Part B, in all three places the template mentions it).
- `PASTE-YOUR-UUID-HERE` with the UUID you stored in A1.4.

**Template 2 — Your own VPS (Path A2)**

```json
{
  "log": { "loglevel": "warning" },
  "inbounds": [
    {
      "listen": "127.0.0.1",
      "port": 1080,
      "protocol": "socks",
      "settings": { "auth": "noauth", "udp": true, "ip": "127.0.0.1" }
    }
  ],
  "outbounds": [
    {
      "protocol": "vless",
      "settings": {
        "vnext": [
          {
            "address": "proxy.yourname.com",
            "port": 443,
            "users": [
              { "id": "PASTE-YOUR-UUID-HERE", "encryption": "none" }
            ]
          }
        ]
      },
      "streamSettings": {
        "network": "tcp",
        "security": "tls",
        "tlsSettings": {
          "serverName": "proxy.yourname.com",
          "allowInsecure": false
        }
      }
    }
  ]
}
```

Replace:
- `proxy.yourname.com` with your VPS's domain (or IP if you used the
  self-signed cert variant in A2.3 — in that case also set
  `"allowInsecure": true`).
- `PASTE-YOUR-UUID-HERE` with the UUID you generated on the VPS.

### C5. Start and verify Xray

Linux:
```bash
sudo systemctl restart xray
sudo systemctl enable  xray
sudo systemctl status  xray     # must say "active (running)"
ss -tlnp | grep 1080            # Xray listening on SOCKS5
```

Windows (v2rayN): click **Servers → Add server → VLESS Import** and
paste a vless:// URL, or paste the template contents into
`%APPDATA%\v2rayN\config\config.json` and click **Restart service**.

Windows (Xray-core CLI):
```cmd
"C:\Program Files\xray\xray.exe" run -c "C:\Program Files\xray\config.json"
```

### C6. Quick smoke test (WITHOUT snix yet)

This test confirms the server is reachable and Xray can talk to it. We
haven't installed snix yet so DPI is still in the way — if your ISP
blocks cloud.cloudflare.com SNI directly, this will fail, but the same
test succeeds later once snix is in the path.

```bash
curl --socks5 127.0.0.1:1080 -sI https://www.cloudflare.com/ | head -3
```

If you see `HTTP/2 200`, Xray is healthy. If you get a timeout or
"connection reset", the DPI is active — continue with the rest of the
guide. snix fixes this.

If you get any other error (TLS failure, UUID mismatch, etc.) fix Xray
now before continuing:
- `sudo journalctl -u xray -n 50` (Linux) to see Xray errors.
- Common mistakes: mistyped UUID, wrong SNI, forgetting `/?ed=2048` for
  the Worker path.

---

## 7. Part D — Install snix

Now install snix on the same device where you installed the proxy client.

### D1. Linux (one-liner)

```bash
curl -fsSL https://raw.githubusercontent.com/SamNet-dev/snix/main/installer/linux/install.sh | sudo sh
```

A shorter `https://get.snix.sh` alias is planned once the domain is
registered. Until then, use the raw GitHub URL above.

Verifies the signature, installs `/usr/local/bin/snix`, adds a systemd
service (disabled by default), bash/zsh completions.

### D2. Linux (manual)

```bash
curl -LO https://github.com/SamNet-dev/snix/releases/latest/download/snix-linux-amd64.tar.gz
tar -xzf snix-linux-amd64.tar.gz
sudo install -m 0755 snix /usr/local/bin/snix
```

(Replace `amd64` with `arm64` or `armv7` for Raspberry Pi / arm devices.)

### D3. Windows

1. Download `snix-setup.exe` from
   <https://github.com/SamNet-dev/snix/releases>.
2. Right-click → **Run as Administrator**.
3. Click through the wizard. Accept the defaults unless you know you need
   to change them.
4. The installer:
   - Puts `snix.exe` + `WinDivert.dll` + `WinDivert64.sys` into
     `C:\Program Files\snix\`.
   - Creates Start Menu shortcuts for **snix (TUI)** and **snix (wizard)**.
   - Optionally installs and starts a Windows Service.

### D4. Build from source (any platform)

```bash
git clone https://github.com/SamNet-dev/snix.git
cd snix
go build -o snix ./cmd/snix
```

Go 1.23+ required. On Windows you also need `WinDivert.dll` +
`WinDivert64.sys` next to the built exe — download from
<https://github.com/basil00/Divert/releases/tag/v2.2.2> (`x64` variant).

### D5. Verify

```bash
snix --version      # prints "snix version X.Y.Z"
snix --help         # shows every command
```

---

## 8. Part E — First-run wizard

This is where everything comes together. The wizard takes ~60 seconds.

### E1. Launch the wizard

Linux:
```bash
sudo snix init --wizard
```

Windows (elevated):
```cmd
snix.exe init --wizard
```

You'll see:

```
snix first-run wizard
─────────────────────

Five short steps. Safe to Ctrl-C at any time; nothing is written
to disk until the end.
```

### E2. Step 1 — Do you have a proxy server?

Answer **`y`** and paste the details from Part A / B. The wizard asks:

```
  Host (domain or IP):   snix.yourname.workers.dev     <- or proxy.yourname.com / VPS IP
  Port [443]:            443
```

### E3. Step 3 — Scanning your network

```
Step 3/5 — Scanning your network for working SNIs and CDN IPs…
  ip probes:  (20 candidates)
  sni probes: (55 candidates against 1.1.1.1)
  (takes up to ~10 seconds)
  ✓ 47/55 SNI candidates OK, 18/20 IPs reachable
```

The wizard has learned which SNIs your ISP doesn't block and which CDN
IPs are reachable. These go into your profile automatically.

### E4. Step 4 — Anti-fingerprinting knobs

Say **`y`** to all three. They make DPI much harder.

```
  Randomize inject timing?   (Y/n) y
  Randomize ClientHello size? (Y/n) y
  Rotate bypass strategies per flow? (Y/n) y
```

### E5. Step 5 — Proxy-client integration

```
Step 5/5 — Proxy-client integration
  Detected xray at /usr/local/bin/xray
  Update xray config to route through snix? (y/N) y
    done (backup: /usr/local/etc/xray/config.json.bak)
```

**Important**: the wizard rewrites the Xray config so its outbound is now
`127.0.0.1:40443` (snix) instead of directly to your proxy server. snix
then handles forwarding to the real server. A backup of the original
config is always saved first.

If the wizard says "No supported proxy client detected", come back to
this step after installing Xray (Part C).

### E6. What the wizard wrote

Final output:
```
Config written to /root/.config/snix/config.yaml
Next:
  sudo snix start           # foreground
  sudo systemctl enable --now snix   # managed service (linux)
```

Open the config if you're curious:
```bash
cat /root/.config/snix/config.yaml
```

You'll see a complete profile with your server, top-10 working SNIs, top-4
fallback IPs, and all anti-fingerprinting knobs turned on.

You also need to paste your UUID into Xray. If the wizard patched Xray's
outbound to point at snix, the UUID stayed where it was (inside Xray's
client config). If you're using v2rayN or NekoBox, open that app and make
sure the outbound address is `127.0.0.1:40443` with your UUID.

---

## 9. Part F — Start using it

### F1. Start snix

Linux (foreground, for testing):
```bash
sudo snix start
```

You should see:
```
snix: starting profile "default" listen=127.0.0.1:40443 connect=snix.yourname.workers.dev:443 strategy=wrong_seq
snix: local=10.0.0.5  remote=104.21.2.17:443
snix: iptables rules installed, NFQUEUE active
snix: bypass engine running
snix: listening on 127.0.0.1:40443
```

Leave that terminal open.

Linux (managed service, for daily use):
```bash
sudo systemctl enable --now snix
journalctl -u snix -f      # follow logs in another terminal
```

Windows (GUI): Start Menu → **snix (TUI)** (it auto-elevates).

### F2. Start Xray

Linux:
```bash
systemctl restart xray
systemctl enable  xray
```

Windows: open v2rayN from the system tray, click the green "start" button.

### F3. Point your browser

In your browser's proxy settings:
- Type: **SOCKS5** (or HTTP — depends on your Xray config).
- Host: `127.0.0.1`
- Port: `1080` (Xray's default SOCKS; check your Xray config's `inbounds`
  section — the port is there).
- No auth.

Or install a browser extension like **SwitchyOmega** (Chrome/Firefox)
and create a proxy profile pointing at `127.0.0.1:1080`.

### F4. Test it

With the proxy enabled in your browser, visit:
- <https://www.youtube.com/>
- <https://twitter.com/>
- <https://chatgpt.com/>

All three should load. If they do — congratulations, you have a working
hardened DPI bypass.

Test it a second way, with curl:
```bash
curl --socks5 127.0.0.1:1080 https://www.youtube.com/ -I
```

Should return `HTTP/2 200`.

Check snix's logs:
```bash
journalctl -u snix | grep injected
```

You'll see one line per outbound connection:
```
snix: injected wrong_seq sni=auth.vercel.com size=843 seq=0xab12... id_delta=42
snix: injected wrong_checksum sni=cdn.segment.io size=1102 seq=0xcd34... id_delta=18
```

Different SNI, different size, different strategy every time. **That's
anti-fingerprinting working.**

---

## 10. Part G — Harden against stronger DPI

If your ISP's DPI gets more aggressive, turn more knobs up. All changes
save to disk immediately; no restart needed in the TUI.

### G1. Open the TUI

```bash
sudo snix tui
```

Press **5** for Settings.

### G2. Recommended production settings

- **Randomize timing**: ON (min 500µs, max 5ms).
- **Randomize padding**: ON (max extra pad 600 bytes).
- **Strategy rotation**: ON.
- **IP ID delta range**: 64.
- **SNI selection**: random (not round-robin).

### G3. Re-scan periodically

Every few weeks, or whenever something stops working:
```bash
snix scan all
```

Paste the updated `sni_pool` into your config, or use the TUI's Scan tab
which can save directly into your profile with one keypress.

### G4. Multiple profiles for different networks

You can have one profile per network you use. In the TUI, Profiles tab,
copy an existing profile, edit its `connect.host` to a different server,
and switch between them with Enter.

Or edit `~/.config/snix/config.yaml` directly — add another entry under
`profiles:`.

---

## 11. Part H — Day 2 operations

### H1. Update snix

```bash
snix update
```

Fetches the latest signed release, verifies the SHA-256, atomically
replaces the binary. Works on Linux and Windows (Windows needs the TUI or
installer closed).

### H2. See what's happening

```bash
journalctl -u snix -f                # Linux service logs
sudo snix tui  →  Run tab            # live log viewer in TUI
```

### H3. Switch profiles

```bash
snix profile list
snix profile switch workerB
sudo systemctl restart snix          # or let the TUI-driven reload happen
```

### H4. Scan periodically

```bash
snix scan all
```

Cron it if you want automated weekly logs. Because `scan all` is
interactive, use the individual JSON-capable subcommands:
```cron
0 3 * * 1 /usr/local/bin/snix scan sni --target 1.1.1.1 --json >> /var/log/snix-sni.jsonl
5 3 * * 1 /usr/local/bin/snix scan ip --json >> /var/log/snix-ip.jsonl
```

### H5. Backup your config

```bash
cp ~/.config/snix/config.yaml ~/.config/snix/config.yaml.$(date +%F)
```

The config is the only state worth backing up. If you lose it, re-run
`snix init --wizard`.

### H6. Move to a new device

1. Install snix.
2. Copy `config.yaml` from old device to the new one's equivalent path
   (Linux `~/.config/snix/`, Windows `%APPDATA%\snix\`).
3. Start as usual. (If the new device has a different proxy client, re-run
   `snix init --wizard --force` and use the existing profile details.)

---

## 12. Troubleshooting by symptom

### 12.1 "No config loaded" in the TUI / Home tab

Means snix can't find `config.yaml`. Run:
```bash
snix status   # shows the expected path
snix init --wizard
```

### 12.2 Windows: `ERROR_ACCESS_DENIED (run as Administrator)`

The TUI itself doesn't need admin, but `snix start` does. Launch the TUI
from an elevated prompt, or let it run the engine through the Windows
Service.

### 12.3 Windows: `WinDivert.dll not loadable`

`snix.exe`, `WinDivert.dll`, and `WinDivert64.sys` must be in the same
folder, or the DLL on `%PATH%`. The installer handles this automatically.
If you're running a portable build, keep all three files together.

### 12.4 `snix scan` says everything is "reset" or "timeout"

Your ISP is probably interfering with raw TLS connections to `1.1.1.1`.
Try:
```bash
snix scan sni --target 104.21.2.17      # any reachable Cloudflare IP
```

If even that fails, your ISP is very aggressive. You'll need the full
hardening stack (Part G) plus possibly a VPS on an unusual port.

### 12.5 `snix start` works, but browser can't reach anything

Check the chain: browser → Xray → snix → internet.

Step 1: is Xray running and listening? `ss -tlnp | grep 1080` (or the
SOCKS port from Xray's config).

Step 2: does Xray's outbound point at `127.0.0.1:40443`?
```bash
grep -A3 outbounds /usr/local/etc/xray/config.json
```
If not, run `snix init --wizard --force` (it patches Xray's config).

Step 3: does snix log injection lines when you browse?
```bash
journalctl -u snix | grep injected
```
No injections = Xray isn't hitting `127.0.0.1:40443`. Xray config error.

Step 4: is the real proxy server reachable at all? From a different
network, curl it:
```bash
curl -I https://snix.yourname.workers.dev/
```
If 502 or timeout → server problem; redeploy the Worker / restart the VPS.

### 12.6 Connection works for 30 seconds, then drops

Flow-based DPI catching unusual patterns after a delay. Enable everything
in Part G. Also consider adding more entries to your `sni_pool` (at least
10) — the DPI has a harder time if every new connection uses a different
name.

### 12.7 High CPU

Expected at this version; the zero-copy relay is v0.8 work. If CPU is
*really* bad (100% of one core with only a few connections), open an
issue with `go tool pprof` output.

### 12.8 I got disconnected from the proxy server, now reconnect fails

Wait 30 seconds (TCP `TIME_WAIT`), then try again. If it persists:

```bash
# Linux:
sudo systemctl restart snix xray

# Windows:
# right-click tray → restart snix, restart Xray
```

### 12.9 The wizard couldn't find my Xray

Xray wasn't on `$PATH` or its config was in a non-standard location. Edit
`~/.config/snix/config.yaml` manually and follow the wizard's final
instructions to point Xray at `127.0.0.1:40443`.

---

## 13. Appendix A — Full glossary

- **DPI** — Deep Packet Inspection. Censorship hardware that looks inside
  network packets to decide what to block.
- **SNI** — Server Name Indication. A plain-text field in the first
  packet of a TLS connection, containing the domain the client is trying
  to reach. The DPI's main trigger.
- **ClientHello** — The first TLS packet, containing the SNI.
- **wrong_seq** — snix's default bypass strategy. Inject a fake
  ClientHello with an out-of-window TCP sequence number.
- **wrong_checksum** — Alternate strategy. Inject the fake with a bad
  TCP checksum AND out-of-window seq. Different packet shape to a
  fingerprinter.
- **VLESS** — A stealth proxy protocol used by Xray. Our Cloudflare
  Worker template implements the server side of VLESS-over-WebSocket.
- **VMess / Trojan** — Other stealth proxy protocols. Snix works with
  any of them; it doesn't care about the payload.
- **Cloudflare Worker** — Small piece of JS code that runs on
  Cloudflare's global network for free. Good cover: your traffic looks
  like it's going to cloudflare.com.
- **NFQUEUE** — Linux kernel mechanism that lets snix intercept packets.
- **WinDivert** — Equivalent on Windows. Kernel driver.
- **TUI** — Text User Interface. What `snix tui` shows.
- **Profile** — One named bundle of snix settings (listen address,
  server, SNI pool, randomization knobs). You can have many; one is
  "active" at a time.

## 14. Appendix B — Security hygiene

- **Never share your UUID publicly.** Anyone who has it can use your
  Worker / VPS. Rotate if leaked.
- **Never commit your config.yaml to a public repo.** It has your server
  address in plain text.
- **Don't run random snix builds from strangers.** Only use signed
  releases from this repo.
- **Verify your release signatures** if you got a binary from elsewhere:
  ```bash
  minisign -Vm snix-linux-amd64.tar.gz \
    -P "$(cat snix-pubkey.txt)"
  ```
- **Use a strong password for your Cloudflare account + 2FA.**
- snix itself performs zero telemetry. `snix update` is the only outbound
  connection it makes on its own, and only when you run it.

## 15. Appendix C — Uninstall cleanly

### Linux

```bash
sudo systemctl disable --now snix 2>/dev/null
sudo rm -f /usr/local/bin/snix
sudo rm -f /etc/systemd/system/snix.service
sudo systemctl daemon-reload
# Optional: remove config + logs
rm -rf ~/.config/snix
```

### Windows

Control Panel → Programs → snix → Uninstall. This also removes the
Windows Service if one was installed. The WinDivert driver lingers until
reboot (harmless).

### Cloudflare Worker

In the Cloudflare dashboard → Workers → your snix worker → **Delete**.

### VPS

Just cancel/destroy the VM at your provider. Nothing on your local
machine to clean up beyond snix itself.

---

You made it. If something isn't in this guide, open an issue — we'll add
it.
