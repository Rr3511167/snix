# 🛰️ snix

> **Cross-platform SNI-spoofing DPI-bypass proxy.** Single binary. No cgo. Batteries included.

[![build](https://github.com/SamNet-dev/snix/actions/workflows/release.yml/badge.svg)](https://github.com/SamNet-dev/snix/actions/workflows/release.yml)
[![release](https://img.shields.io/github/v/release/SamNet-dev/snix.svg)](https://github.com/SamNet-dev/snix/releases)
[![license](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)
[![platforms](https://img.shields.io/badge/platforms-linux%20%7C%20windows%20%7C%20macOS-blue)](https://github.com/SamNet-dev/snix/releases)

snix tricks DPI censorship systems into accepting your TLS connections by
injecting a fake ClientHello with a whitelisted SNI during the TCP
handshake. The real TLS handshake then proceeds unobstructed. It is a Go
rewrite of [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing),
expanded with multi-platform backends, anti-fingerprinting, a TUI, and a
built-in network scanner.

> 📖 **New here?** Read the full walkthrough:
> - 🇬🇧 **[GUIDE.md](GUIDE.md)** — zero-to-finish in English.
> - 🇮🇷 **[GUIDE-fa.md](GUIDE-fa.md)** — صفر تا صد به فارسی.
>
> Both cover every step from signing up for Cloudflare through a working
> bypass, including Xray client configs and optional domain setup. Start
> there if this is the first time you've set up any circumvention tool.
> The README below is the short reference for everyone else.

### ✨ At a glance

- ⚡ **Single binary, no runtime deps** — 10 MB static executable for Linux + Windows.
- 🥷 **Anti-fingerprinting** — randomized packet size, timing, IP ID, and SNI pool.
- 🔍 **Built-in scanner** — `snix scan` finds working SNIs and CDN IPs for your ISP.
- 🎛️ **Full TUI** — `snix tui` gives you a dashboard, live logs, and one-key profile switching.
- 🪄 **First-run wizard** — `snix init --wizard` writes a working config in under a minute.
- ☁️ **Free-tier server path** — deploy a Cloudflare Worker as your proxy server in ~60 seconds.
- 🔄 **Self-update** — `snix update` verifies SHA-256 and atomically swaps the binary.
- 🌍 **English + Farsi docs** throughout.

## Table of contents

- [What snix does](#what-snix-does)
- [What snix is NOT](#what-snix-is-not)
- [What you need before installing](#what-you-need-before-installing)
- [Install — Linux](#install--linux)
- [Install — Windows](#install--windows)
- [Install — build from source](#install--build-from-source)
- [First-time setup](#first-time-setup)
- [Daily usage](#daily-usage)
- [TUI walkthrough](#tui-walkthrough)
- [CLI reference](#cli-reference)
- [Configuration reference](#configuration-reference)
- [Anti-fingerprinting knobs](#anti-fingerprinting-knobs)
- [Hooking snix into your proxy client](#hooking-snix-into-your-proxy-client)
- [Troubleshooting](#troubleshooting)
- [Relation to patterniha/SNI-Spoofing](#relation-to-patternihasni-spoofing)
- [Credits + license](#credits--license)
- [نسخه فارسی / Persian version](#نسخه-فارسی)

---

## 🎯 What snix does

```
[your browser or proxy client]
        │
        ▼
[snix on 127.0.0.1:40443]  ──── at TCP handshake time snix:
        │                         1) observes outbound SYN
        │                         2) observes inbound SYN-ACK
        │                         3) observes outbound ACK
        │                         4) injects a fake TLS ClientHello
        ▼                            with a whitelisted SNI such as
[your upstream proxy server, e.g.    `auth.vercel.com` and a deliberately
 VLESS over Cloudflare on port 443]  wrong TCP sequence number
```

- The DPI middlebox reads the fake ClientHello first, sees a whitelisted
  domain (`auth.vercel.com`), and marks the flow as allowed.
- The real server receives the fake packet too — but because its sequence
  number is out-of-window (or its checksum is corrupted) the server
  silently drops it. No connection damage.
- The real TLS handshake to your proxy server then proceeds over the
  already-whitelisted TCP flow and succeeds.

## ⚠️ What snix is NOT

snix is a **client-side DPI-bypass layer**, not a VPN.

You still need:
- A proxy server somewhere outside the censored network (Xray/VLESS,
  VMess, Trojan, sing-box, etc.), ideally on port 443 and fronted via a
  CDN such as Cloudflare.
- A client that speaks that proxy protocol (Xray, v2ray, sing-box,
  NekoBox, v2rayN, etc.). snix does not speak VLESS/VMess/Trojan yet —
  that's in the v1.0 roadmap.

snix sits between the client and the internet. Every connection the
client opens gets the SNI-spoof treatment.

## 📋 What you need before installing

1. **A proxy server.** If you don't have one, the first-run wizard
   (`snix init --wizard`) can walk you through deploying a free
   Cloudflare Worker that acts as your proxy in about 60 seconds.
2. **Root / Administrator access** on the device you're installing on.
   Packet interception requires kernel-level filter access on both Linux
   (NFQUEUE + iptables) and Windows (WinDivert).
3. **A TLS-based proxy client** already installed. snix is the layer in
   front; you still need Xray / v2ray / sing-box / NekoBox to speak the
   proxy protocol itself.

---

## 🐧 Install — Linux

### One-liner (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/SamNet-dev/snix/main/installer/linux/install.sh | sudo sh
```

*(A shorter `https://get.snix.sh` form will be set up once the domain is
registered; until then the raw GitHub URL above is the canonical install
command.)*

The script:
1. Detects your distro + architecture.
2. Downloads the signed release tarball from GitHub.
3. Verifies the `minisign` signature.
4. Places `snix` into `/usr/local/bin`.
5. Writes `/etc/systemd/system/snix.service` (disabled by default).
6. Installs bash + zsh completions.

Then:

```bash
sudo snix init --wizard      # create config + pick best SNIs/IPs for your ISP
sudo snix start              # run the engine in your shell, or:
sudo systemctl enable --now snix    # run as a managed service
```

### Manual install

Download from <https://github.com/SamNet-dev/snix/releases>:

```bash
curl -LO https://github.com/SamNet-dev/snix/releases/latest/download/snix-linux-amd64.tar.gz
curl -LO https://github.com/SamNet-dev/snix/releases/latest/download/snix-linux-amd64.tar.gz.minisig

minisign -Vm snix-linux-amd64.tar.gz -P $(cat snix-pubkey.txt)   # verify

tar -xzf snix-linux-amd64.tar.gz
sudo install -m 0755 snix /usr/local/bin/snix
```

Architectures: `amd64`, `arm64`, `armv7`.

### Distribution packages

- Debian/Ubuntu `.deb` and Fedora/RHEL `.rpm` ship with each release.
- AUR: `yay -S snix-bin`.
- Arch: coming soon.

---

## 🪟 Install — Windows

### Installer (recommended)

1. Download `snix-setup.exe` from <https://github.com/SamNet-dev/snix/releases>.
2. Right-click → Run as Administrator. Follow the installer.
3. The installer:
   - Puts `snix.exe` into `C:\Program Files\snix\`.
   - Bundles `WinDivert.dll` and `WinDivert64.sys` alongside it.
   - Adds a Start Menu shortcut that launches the TUI with auto-UAC.
   - Optionally registers a Windows Service for automatic startup.
4. Launch from the Start Menu. First launch shows the wizard.

### Portable zip

1. Download `snix-windows-amd64.zip`, extract anywhere.
2. Open an **elevated** Command Prompt or PowerShell.
3. `cd` to the folder, then:
   ```cmd
   snix.exe init --wizard
   snix.exe tui
   ```

The zip contains:

```
snix-windows-amd64/
  snix.exe
  WinDivert.dll
  WinDivert64.sys
  LICENSE
  README.md
```

All three files must live in the same directory for snix to find the
WinDivert driver.

---

## 🔨 Install — build from source

```bash
git clone https://github.com/SamNet-dev/snix.git
cd snix
go build -o snix ./cmd/snix
```

Requirements: Go 1.23+, no C toolchain, no other tooling. Cross-compile:

```bash
GOOS=linux   GOARCH=amd64 go build -o snix-linux   ./cmd/snix
GOOS=windows GOARCH=amd64 go build -o snix.exe     ./cmd/snix
GOOS=darwin  GOARCH=arm64 go build -o snix-macos   ./cmd/snix
```

On Windows the engine requires WinDivert. Download v2.2.2 from
<https://github.com/basil00/Divert/releases/tag/v2.2.2>, extract
`x64/WinDivert.dll` and `x64/WinDivert64.sys`, put them next to the built
`snix.exe`.

---

## 🚀 First-time setup

Run the wizard:

```bash
snix init --wizard
```

You'll see:

```
snix first-run wizard

Step 1/5 — Do you have a proxy server already?  (y/n)
> y

Step 2/5 — Paste your server address:
  Host:        my-proxy.example.com
  Port [443]:  443

Step 3/5 — Scanning your network for working SNIs and CDN IPs…
  SNI probes  [████████████████] 55/55  ok=55  reset=0  timeout=0
  IP probes   [████████████████] 20/20  reachable=18

  Top SNIs:
    1. auth.vercel.com           12ms
    2. cdn.segment.io            14ms
    3. static.cloudflareinsights 16ms
  Top IPs:
    1. 104.21.2.17    3ms
    2. 172.65.250.78  3ms

Step 4/5 — Anti-fingerprinting knobs (recommended ON):
  Randomize timing?   (Y/n) y
  Randomize padding?  (Y/n) y
  Strategy rotation?  (Y/n) y

Step 5/5 — Integrate with a proxy client?
  Detected: xray at /usr/local/bin/xray
  Update Xray config to use snix on 127.0.0.1:40443? (y/N) y
  Backup written to ~/.config/xray/config.json.bak
  Done.

Config written to /root/.config/snix/config.yaml
Run `sudo snix start` to launch.
```

If you said "n" at Step 1, the wizard offers to deploy a Cloudflare
Worker:

```
  You don't have a proxy server yet. snix can deploy one for free on
  Cloudflare in ~60 seconds.

  Deploy Cloudflare Worker? (Y/n) y
  Open this URL in your browser:
    https://dash.cloudflare.com/?to=/:account/workers/services/new?name=snix-<random>
  After deploy completes, paste the Worker URL here:
  > worker-name.username.workers.dev
```

## 🎮 Daily usage

Two ways to drive snix: the TUI (interactive) or the CLI (scriptable).

### TUI

```bash
snix tui        # opens full-screen dashboard
```

Press `1-7` to switch tabs, `?` for help, `q` to quit. See [TUI walkthrough](#tui-walkthrough).

### CLI + systemd (Linux)

```bash
sudo systemctl enable --now snix
journalctl -u snix -f            # follow logs
sudo systemctl stop snix
```

### CLI + Services (Windows)

If you chose "Install as Service" during setup:

```cmd
sc query snix
sc start snix
sc stop snix
```

Otherwise, launch from the Start Menu shortcut.

---

## 🖥️ TUI walkthrough

### Tab 1 — Home

Shows:
- Which config file is loaded.
- How many profiles are defined.
- Which profile is active and its summary (listen, connect, strategy).
- A short explainer of what snix does.

### Tab 2 — Profiles

List of profiles. Keys:

| key | action |
|---|---|
| `↑↓` / `jk` | move cursor |
| `⏎`         | set the selected profile as active (writes config.yaml) |
| `e`         | open the config file in `$EDITOR` / notepad |
| `r`         | reload config from disk |

Right pane shows full details of the highlighted profile including its
SNI pool, randomization settings, and fallback IPs.

### Tab 3 — Scan

Probe the network to find which SNIs and CDN IPs work. Keys:

| key | action |
|---|---|
| `s` | probe candidate SNIs against the target IP |
| `i` | probe the Cloudflare IP pool for reachability + RTT |
| `a` | run both concurrently |
| `t` | edit the SNI-probe target IP |
| `x` | stop the running scan |
| `e` | save top results into the active profile (writes config.yaml) |

Live progress bars for each scan type. Top-10 leaderboards update as
results arrive.

### Tab 4 — Run

Starts / stops the bypass engine as a subprocess. Keys:

| key | action |
|---|---|
| `s` | start engine |
| `x` | stop engine (SIGINT / Kill) |
| `r` | restart |
| `c` | clear logs |

Engine stdout and stderr are tailed live into the Engine Output panel.
Scrollable with `↑↓` / `PgUp`/`PgDn`.

Windows: the engine subprocess needs Administrator. If the TUI was
launched non-elevated you'll see the missing-privilege error in the log
panel; restart the TUI from an elevated shell.

### Tab 5 — Settings

Toggle anti-fingerprinting knobs on the active profile. All changes save
to disk immediately. Keys:

| key | action |
|---|---|
| `↑↓` / `jk` | move cursor |
| `space` / `⏎` | toggle the selected knob |
| `+` / `-` | adjust numeric values |

The right pane explains what the highlighted knob does and why you'd
want it.

### Tab 6 — Help

Scrollable long-form help: what SNI spoofing is, every keybinding, CLI
parity table, troubleshooting.

### Tab 7 — About

Version, license, credits, support links.

---

## 📖 CLI reference

```
snix init                     create a starter config at ~/.config/snix/config.yaml
snix init --wizard            interactive first-time setup (recommended)
snix status                   show loaded config + active profile
snix tui                      launch the full interactive TUI
snix start                    run the bypass engine (needs root / admin)
snix start -p NAME            start using a non-active profile
snix scan sni --target IP     probe SNIs against IP (default 1.1.1.1)
snix scan ip                  probe Cloudflare IPs for reachability + RTT
snix scan all                 run both scans and print a ready-to-paste profile
snix profile list             list profiles
snix profile switch NAME      make NAME the active profile
snix update                   upgrade to the latest release (verifies signature)
snix --version                print version
snix --help                   print top-level help
```

Global flags:

```
-c, --config PATH    use a non-default config path
-v, --verbose        verbose logging
```

---

## ⚙️ Configuration reference

Default location:
- Linux / macOS: `~/.config/snix/config.yaml` (or `$XDG_CONFIG_HOME/snix/config.yaml`).
- Windows: `%APPDATA%\snix\config.yaml`.

Full example with every field:

```yaml
version: 1

# Which profile to use on `snix start`. Must match one of `profiles[].name`.
active: default

log:
  # debug | info | warn | error
  level: info

profiles:
  - name: default

    # Where snix listens for client connections (Xray, v2ray, etc.).
    listen: "127.0.0.1:40443"

    connect:
      # Your upstream proxy server. Domain or IP.
      host: my-proxy.example.com
      port: 443
      # If `host` fails or its DNS resolution is blocked, snix will try
      # these IPs in order. Useful to pin known-working Cloudflare edges.
      fallback_ips:
        - 104.21.2.17
        - 172.65.250.78

    spoof:
      # Default strategy. Used when `strategy_rotation` is empty.
      # wrong_seq     — inject fake ClientHello with out-of-window seq
      # wrong_checksum — same + corrupted TCP checksum (double-safety)
      strategy: wrong_seq

      # If set (recommended), snix picks a strategy per flow at random
      # from this list. Breaks DPI pattern-matching on packet shape.
      strategy_rotation: [wrong_seq, wrong_checksum]

      # The SNIs snix rotates through when building the fake ClientHello.
      # Run `snix scan sni` to discover which ones work on your ISP.
      sni_pool:
        - auth.vercel.com
        - cdn.segment.io
        - static.cloudflareinsights.com

      # random (default): uniform random pick per flow
      # round_robin: deterministic cycle
      sni_selection: random

      # Jitters the delay between the real ACK and the fake ClientHello.
      # Defeats timing-based DPI fingerprints.
      randomize_timing: true
      min_delay: 500us
      max_delay: 5ms

      # Varies the ClientHello packet length so it's not always 517 bytes.
      randomize_padding: true
      min_extra_pad: 0
      max_extra_pad: 600

      # Random offset applied to the fake packet's IP identification
      # field. 1 = upstream behaviour (always +1). Higher = more variance.
      ip_id_delta_range: 64

    health:
      # Background probe interval (not yet wired to auto-failover).
      interval: 30s
      auto_failover: true
```

Defaults (if you omit the field):

| field | default |
|---|---|
| `connect.port` | 443 |
| `spoof.sni_selection` | `random` |
| `spoof.randomize_timing` | `false` |
| `spoof.min_delay` | `1ms` |
| `spoof.max_delay` | `1ms` |
| `spoof.randomize_padding` | `false` |
| `spoof.ip_id_delta_range` | `1` |
| `health.interval` | `30s` |
| `log.level` | `info` |

## 🎛️ Anti-fingerprinting knobs

Why each knob matters:

- **`randomize_timing`** — upstream emits the fake ClientHello exactly
  1 ms after the final ACK. A DPI watching for "ACK immediately followed
  by 517-byte PSH packet with specific seq pattern" can flag this. Random
  jitter in [500 µs, 5 ms] makes the timing an uninteresting noise field.
- **`randomize_padding`** — upstream's fake ClientHello is always 517
  bytes. With `randomize_padding: true` + `max_extra_pad: 600`, snix
  emits packets in [517, 1117] bytes. Removes a trivially fingerprintable
  invariant.
- **`strategy_rotation`** — wrong_seq alone always produces the same
  shape (valid TLS, valid checksum, wildly wrong seq). Rotating with
  wrong_checksum means half of your connections have valid seq +
  invalid checksum instead. Two different shapes to pattern-match means
  twice the DPI engineering required to catch you.
- **`ip_id_delta_range`** — upstream always sets the fake packet's IPv4
  `ident` field to `ack_packet.ident + 1`. A range of 64 scatters the
  delta uniformly in [1, 64], removing the predictable tell.
- **`sni_selection: random`** — round-robin is deterministic; random
  makes per-flow SNI choice unrelated to flow order.

Recommended for production:

```yaml
spoof:
  strategy_rotation: [wrong_seq, wrong_checksum]
  sni_pool: [... at least 3 SNIs ...]
  sni_selection: random
  randomize_timing: true
  min_delay: 500us
  max_delay: 5ms
  randomize_padding: true
  max_extra_pad: 600
  ip_id_delta_range: 64
```

## 🔌 Hooking snix into your proxy client

Point your proxy client's outbound at `127.0.0.1:40443`. Every proxy
stack works slightly differently; quick notes:

### Xray / v2ray

In your `config.json`, under `outbounds[0].settings.vnext[0].address`
(or equivalent), replace your real proxy server address with
`"127.0.0.1"` and port `40443`. Then in snix's config put the real
server in `connect.host` + `connect.port`.

snix's wizard can edit this for you: `snix init --wizard` → Step 5.

### sing-box

Same concept: set the `server` field of your outbound to
`"127.0.0.1"` and `server_port: 40443`. snix handles the actual TLS.

### NekoBox / v2rayN

Import your existing profile, then edit the node's address → `127.0.0.1`,
port → `40443`.

### SOCKS5-ready clients (browsers, curl)

snix doesn't currently expose SOCKS5; it's a transparent TCP relay. Use
Xray or sing-box to expose SOCKS5 on a different port, with snix as the
outbound address.

---

## 🔧 Troubleshooting

**"No config loaded" on the Home screen**
→ Run `snix init` or `snix init --wizard` to create one. Check
`snix status` to see where it's looking.

**`ERROR_ACCESS_DENIED (run as Administrator)` on Windows**
→ Launch from an elevated shell. The TUI does a pre-check and prints
`snix/windows: process is not running elevated`; relaunch elevated.

**`WinDivert.dll not loadable` on Windows**
→ The DLL and driver must be in the same folder as `snix.exe`, or on
`%PATH%`. If you used the installer, this is automatic. For portable
usage, keep all three files together.

**Scan reports "0 OK" in Scan → SNI**
→ Try a different `--target`. The default `1.1.1.1` accepts any SNI
because Cloudflare DNS has a universal default cert. Other edge IPs
only serve their own customers' domains. If even `1.1.1.1` shows resets,
your ISP may be blocking raw TLS handshakes at the TCP layer — use the
`--target` of a known-working proxy IP instead.

**`snix start` succeeds but the proxy client can't reach anything**
→ Verify your proxy server is actually reachable without DPI first
(try `curl -vk --resolve myserver:443:YOUR_IP https://myserver/` on a
network without DPI). If that works, turn on snix's `--verbose` and watch
the log for `snix: injected ...` lines. No injection log = client isn't
hitting `127.0.0.1:40443`; misconfigured proxy client.

**Connection works for 30 seconds, then drops**
→ Some ISPs use flow-based classifiers that catch unusual patterns
later. Enable full anti-fingerprinting (all three knobs) and restart.

**Real traffic stalls when `strategy_rotation: [wrong_seq, wrong_checksum]`**
→ Shouldn't happen in this release; we always pair wrong_checksum with
out-of-window seq. If it does, remove `wrong_checksum` from rotation and
file an issue with the tcpdump from the failing flow.

**High CPU under load**
→ Expected at this version (we don't yet use `splice()` on Linux or
`TransmitFile` on Windows). v0.8 brings a zero-copy relay.

**How do I uninstall?**
- Linux: `sudo rm /usr/local/bin/snix /etc/systemd/system/snix.service`
- Windows: Control Panel → Apps → snix → Uninstall.

**How do I update?**
- `snix update` auto-upgrades to the latest release (signature verified).
- Or re-run the installer; it's safe to overwrite.

**Where are the logs?**
- When run via `snix start`: stdout/stderr.
- When run via systemd: `journalctl -u snix`.
- When run via Windows Service: Event Viewer → Application logs.
- TUI Run tab: in-memory tail (not persistent).

---

## 🤝 Relation to patterniha/SNI-Spoofing

snix is a Go rewrite of
[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing).
That project discovered and implemented the wrong-seq bypass technique
that makes everything here possible. If you like snix, **please star
their repo too**. This project would not exist without their work.

Our bypass engine is **byte-for-byte identical** to theirs for the core
trick: the fake ClientHello, the wrong TCP sequence number, the timing,
and the state machine all produce wire-identical packets. That parity is
locked in with a golden test that regenerates reference bytes from the
upstream Python and fails any build that diverges.

On top of that identical foundation we add:

| Area | patterniha/SNI-Spoofing | snix |
|---|---|---|
| **Core bypass (`wrong_seq`)** | ✓ | ✓ *(byte-identical)* |
| Platforms | Windows only | **Windows + Linux** *(macOS + Android planned)* |
| Runtime dependency | Python + pydivert + WinDivert | **Single 10 MB binary** *(no cgo, no Python)* |
| Second bypass strategy | not available | **`wrong_checksum`** rotates with `wrong_seq` |
| Timing randomization | fixed 1 ms | uniform in `[min, max]` (e.g. 500 µs to 5 ms) |
| Packet-size randomization | fixed 517 bytes | `[517, 517 + max_extra_pad]` |
| IP-ID randomization | always `+1` | uniform in `[1, IP ID delta range]` |
| SNI strategy | single fixed SNI | pool + random/round-robin selection |
| Network scanner | not available | **`snix scan`**: probes SNIs and CDN IPs, ranks by success + latency |
| First-run wizard | not available | **`snix init --wizard`**: 5 prompts to a working profile |
| Proxy-client integration | manual | **auto-detects Xray / v2ray / sing-box** and patches their config (with backup) |
| Free-tier server path | not available | **Cloudflare Worker** template + deploy instructions |
| UI | CLI only | **CLI + full TUI** (dashboard, profiles, scan, live logs, settings, help) |
| Multiple profiles | edit config file | **switch active profile at runtime** |
| Self-update | not available | **`snix update`** (signature-verified) |
| Graceful shutdown | not available | removes iptables / WinDivert rules on exit |
| Test coverage | not available | **race-detector-clean**, golden tests, unit tests per package |
| CI + signed releases | not available | GitHub Actions matrix + minisign + SBOM |
| Localisation | English + Persian (README) | **English + Persian** (README + full user guide) |

**Design decision:** we never remove a capability that upstream has. You
can run snix with every randomization knob turned OFF and the engine
behaves exactly like patterniha/SNI-Spoofing: same packet on the wire,
same timing, same state transitions. The randomization layer is additive.

Where we are **stricter** than upstream: we narrow the packet filter to
the exact IP+port pair instead of just the host, so unrelated traffic
never gets pulled into the bypass engine. Where we are **more
permissive** than upstream: we don't tear down a connection when we see
an unexpected packet. We skip the bypass for that flow but let the
connection continue. Both are safety improvements that don't change the
core behaviour.

If you want the smaller, simpler, Windows-only Python version, go use
the upstream. If you want the same bypass as a single signed binary on
Linux and Windows, with a wizard, a scanner, and a TUI, that's snix.

---

## 🙏 Credits + license

- Upstream algorithm + first implementation:
  [patterniha](https://github.com/patterniha) — **this project would not
  exist without their work**. The core wrong-seq bypass is theirs;
  everything else here is a cross-platform rewrite and expansion.
- [WinDivert](https://reqrypt.org/windivert.html) by Basil Fierz — the
  signed kernel driver we use on Windows.
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) and
  [Lipgloss](https://github.com/charmbracelet/lipgloss) — the TUI framework.
- Linux kernel netfilter team — for NFQUEUE.

Support the upstream author:
```
USDT (BEP20): 0x76a768B53Ca77B43086946315f0BDF21156bF424
Telegram:     @patterniha
```

License: GPL-3.0. Matches upstream. See [LICENSE](LICENSE).

## 💬 Community

- GitHub Discussions: <https://github.com/SamNet-dev/snix/discussions>
- Issues: <https://github.com/SamNet-dev/snix/issues>
- Telegram: *(link once channel exists)*

---

<a id="نسخه-فارسی"></a>

# snix — نسخهٔ فارسی

<div dir="rtl">

> پروکسیِ دور زدنِ DPI با جعلِ SNI، چند-پلتفرمی، تک‌فایلی، بدون cgo، با ابزارهای کامل تویِ بسته.

> **تازه‌وارد؟** راهنمایِ کاملِ صفر تا صد را بخوانید: **[GUIDE-fa.md](GUIDE-fa.md)** — همهٔ مراحل از ثبتِ Cloudflare تا دور زدنِ کاری، شاملِ پیکربندیِ کلاینتِ Xray و دامنهٔ اختیاری.

snix سامانه‌هایِ سانسور مبتنی بر DPI را گول می‌زند: در لحظهٔ دست‌دادنِ TCP، یک ClientHello جعلی با دامنه‌ای مجاز (مثل `auth.vercel.com`) تزریق می‌شود. سپس دست‌دادنِ واقعیِ TLS بدون مزاحمت انجام می‌گیرد. این پروژه بازنویسیِ [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing) در زبان Go است، با پشتیبانی از چند پلتفرم، ضد-اثرانگشت، رابطِ TUI، و اسکنرِ شبکه.

## فهرست

- [snix چه می‌کند](#snix-چه-میکند)
- [snix چه نیست](#snix-چه-نیست)
- [پیش‌نیازها](#پیشنیازها)
- [نصب — لینوکس](#نصب--لینوکس)
- [نصب — ویندوز](#نصب--ویندوز)
- [نصب — ساختن از سورس](#نصب--ساختن-از-سورس)
- [راه‌اندازیِ اول](#راهاندازیِ-اول)
- [استفادهٔ روزانه](#استفادهٔ-روزانه)
- [راهنمایِ TUI](#راهنمایِ-tui)
- [مرجعِ CLI](#مرجعِ-cli)
- [مرجعِ پیکربندی](#مرجعِ-پیکربندی)
- [کلیدهایِ ضد-اثرانگشت](#کلیدهایِ-ضد-اثرانگشت)
- [اتصالِ snix به کلاینتِ پروکسی](#اتصالِ-snix-به-کلاینتِ-پروکسی)
- [عیب‌یابی](#عیبیابی)
- [نسبت با patterniha/SNI-Spoofing](#نسبت-با-patternihasni-spoofing)
- [اعتبار و مجوز](#اعتبار-و-مجوز)

## snix چه می‌کند

<div dir="ltr" markdown="1">

```
[مرورگر یا کلاینتِ پروکسیِ شما]
        │
        ▼
[snix روی 127.0.0.1:40443] ─── در لحظهٔ دست‌دادنِ TCP:
        │                       ۱) بستهٔ SYNِ خروجی را رصد می‌کند
        │                       ۲) بستهٔ SYN-ACKِ ورودی را رصد می‌کند
        │                       ۳) بستهٔ ACKِ خروجی را رصد می‌کند
        │                       ۴) یک ClientHelloِ جعلی تزریق می‌کند
        ▼                         با SNIِ مجاز مثلِ `auth.vercel.com`
[سرورِ پروکسیِ شما، مثلِ          و با شمارهٔ توالیِ TCPِ عمداً غلط
 VLESS پشتِ Cloudflare رویِ ۴۴۳]
```

</div>

- دیوارهٔ DPI اول بستهٔ جعلی را می‌خواند، دامنهٔ مجاز را می‌بیند، و جریان را «مجاز» علامت می‌زند.
- سرورِ واقعی هم همان بسته را می‌گیرد، ولی چون شمارهٔ توالی در بازهٔ درست نیست (یا checksum خراب است) ساکت dropش می‌کند — هیچ آسیبی به اتصال نمی‌خورد.
- سپس دست‌دادنِ واقعیِ TLS رویِ همان جریانِ از پیش تأییدشده انجام می‌گیرد و موفق می‌شود.

## snix چه نیست

snix **لایهٔ دور زدنِ DPI در سمتِ کلاینت** است، نه VPN.

به این‌ها هنوز نیاز دارید:
- یک سرورِ پروکسی بیرون از شبکهٔ سانسور‌شده (Xray/VLESS، VMess، Trojan، sing-box و …)، ترجیحاً رویِ پورتِ ۴۴۳ و پشتِ CDN مثلِ Cloudflare.
- یک کلاینت که آن پروتکل را بفهمد (Xray، v2ray، sing-box، NekoBox، v2rayN و …). فعلاً snix خودش VLESS/VMess/Trojan صحبت نمی‌کند — این در نقشهٔ v1.0 است.

snix بین کلاینت و اینترنت می‌نشیند. هر اتصالِ جدیدی که کلاینت باز می‌کند از همین ترفندِ SNI-spoof بهره می‌برد.

## پیش‌نیازها

۱. **یک سرورِ پروکسی.** اگر ندارید، دستیارِ راه‌اندازیِ اول (`snix init --wizard`) می‌تواند یک Cloudflare Workerِ رایگان را در حدودِ ۶۰ ثانیه برایتان بسازد.
۲. **دسترسیِ root یا Administrator** رویِ دستگاهِ نصب. رهگیریِ بسته‌ها نیاز به دسترسیِ لایهٔ کرنل دارد — در لینوکس (NFQUEUE + iptables) و در ویندوز (WinDivert).
۳. **یک کلاینتِ پروکسی مبتنی بر TLS از قبل نصب‌شده.** snix لایهٔ جلویی است؛ برایِ صحبتِ واقعیِ VLESS/VMess/Trojan هنوز به Xray / v2ray / sing-box / NekoBox نیاز دارید.

## نصب — لینوکس

### تک‌خطی (توصیه‌شده)

<div dir="ltr" markdown="1">

```bash
curl -fsSL https://raw.githubusercontent.com/SamNet-dev/snix/main/installer/linux/install.sh | sudo sh
```

</div>

*(فرمِ کوتاه‌تر `https://get.snix.sh` پس از ثبتِ دامنه فعال می‌شود؛ تا
آن موقع، URLِ بالا مسیرِ رسمیِ نصب است.)*

اسکریپت:
۱. توزیع و معماریِ شما را تشخیص می‌دهد.
۲. آرشیوِ امضاشدهٔ نسخه را از GitHub دانلود می‌کند.
۳. امضایِ `minisign` را بررسی می‌کند.
۴. `snix` را در `/usr/local/bin` قرار می‌دهد.
۵. فایلِ systemd به نامِ `/etc/systemd/system/snix.service` می‌نویسد (به‌صورتِ پیش‌فرض غیرفعال).
۶. auto-completionِ bash و zsh نصب می‌کند.

سپس:

<div dir="ltr" markdown="1">

```bash
sudo snix init --wizard      # ساختِ پیکربندی + پیدا کردنِ بهترین SNI/IP
sudo snix start              # اجرایِ موتور در همین شلِ جاری، یا:
sudo systemctl enable --now snix    # اجرا به‌صورتِ سرویسِ مدیریت‌شده
```

</div>

### نصبِ دستی

از <https://github.com/SamNet-dev/snix/releases>:

<div dir="ltr" markdown="1">

```bash
curl -LO https://github.com/SamNet-dev/snix/releases/latest/download/snix-linux-amd64.tar.gz
curl -LO https://github.com/SamNet-dev/snix/releases/latest/download/snix-linux-amd64.tar.gz.minisig

minisign -Vm snix-linux-amd64.tar.gz -P $(cat snix-pubkey.txt)   # بررسیِ امضا

tar -xzf snix-linux-amd64.tar.gz
sudo install -m 0755 snix /usr/local/bin/snix
```

</div>

معماری‌ها: `amd64`, `arm64`, `armv7`.

### بسته‌هایِ توزیع

- فایلِ `.deb` برای Debian/Ubuntu و `.rpm` برای Fedora/RHEL در هر نسخه منتشر می‌شوند.
- AUR: `yay -S snix-bin`.

## نصب — ویندوز

### نصب‌کننده (توصیه‌شده)

۱. `snix-setup.exe` را از <https://github.com/SamNet-dev/snix/releases> دانلود کنید.
۲. راست‌کلیک → Run as Administrator. مراحلِ نصب را دنبال کنید.
۳. نصب‌کننده:
   - `snix.exe` را در `C:\Program Files\snix\` قرار می‌دهد.
   - `WinDivert.dll` و `WinDivert64.sys` را در کنارِ آن می‌گذارد.
   - یک میانبرِ Start Menu اضافه می‌کند که TUI را با UAC خودکار اجرا می‌کند.
   - اختیاری: snix را به‌عنوانِ سرویسِ ویندوز برایِ استارتِ خودکار ثبت می‌کند.
۴. از Start Menu اجرا کنید. اولین اجرا دستیارِ راه‌اندازی را نشان می‌دهد.

### بستهٔ قابل‌حمل (zip)

۱. `snix-windows-amd64.zip` را دانلود و در هر پوشه‌ای استخراج کنید.
۲. یک Command Prompt یا PowerShellِ **بالارفته (Administrator)** باز کنید.
۳. به آن پوشه بروید و اجرا کنید:
<div dir="ltr" markdown="1">

   ```cmd
   snix.exe init --wizard
   snix.exe tui
   ```

</div>

محتوایِ zip:

<div dir="ltr" markdown="1">

```
snix-windows-amd64/
  snix.exe
  WinDivert.dll
  WinDivert64.sys
  LICENSE
  README.md
```

</div>

هر سه فایل باید در یک پوشه کنارِ هم باشند تا snix درایورِ WinDivert را پیدا کند.

## نصب — ساختن از سورس

<div dir="ltr" markdown="1">

```bash
git clone https://github.com/SamNet-dev/snix.git
cd snix
go build -o snix ./cmd/snix
```

</div>

پیش‌نیاز: Go نسخهٔ ۱.۲۳ یا بالاتر، بدونِ نیاز به ابزارهایِ C، بدونِ وابستگیِ دیگر.

کراس-کامپایل:

<div dir="ltr" markdown="1">

```bash
GOOS=linux   GOARCH=amd64 go build -o snix-linux   ./cmd/snix
GOOS=windows GOARCH=amd64 go build -o snix.exe     ./cmd/snix
GOOS=darwin  GOARCH=arm64 go build -o snix-macos   ./cmd/snix
```

</div>

در ویندوز، موتورِ snix به WinDivert نیاز دارد. نسخهٔ ۲.۲.۲ را از <https://github.com/basil00/Divert/releases/tag/v2.2.2> دانلود کنید، `x64/WinDivert.dll` و `x64/WinDivert64.sys` را استخراج و کنارِ `snix.exe` بگذارید.

## راه‌اندازیِ اول

<div dir="ltr" markdown="1">

```bash
snix init --wizard
```

</div>

چیزی شبیهِ این می‌بینید:

<div dir="ltr" markdown="1">

```
snix first-run wizard

گامِ ۱ از ۵ — آیا از قبل سرورِ پروکسی دارید؟  (y/n)
> y

گامِ ۲ از ۵ — آدرسِ سرور را وارد کنید:
  Host:        my-proxy.example.com
  Port [443]:  443

گامِ ۳ از ۵ — در حالِ اسکنِ شبکه برای SNI و IPهایِ کارآمد…
  SNI probes  [████████████████] 55/55  ok=55  reset=0  timeout=0
  IP probes   [████████████████] 20/20  reachable=18

گامِ ۴ از ۵ — کلیدهایِ ضد-اثرانگشت (پیشنهاد: فعال):
  Randomize timing?   (Y/n) y
  Randomize padding?  (Y/n) y
  Strategy rotation?  (Y/n) y

گامِ ۵ از ۵ — ادغام با کلاینتِ پروکسی؟
  شناسایی شد: xray در /usr/local/bin/xray
  آیا تنظیماتِ Xray برایِ استفاده از snix تغییر کند؟ (y/N) y
  نسخهٔ پشتیبان در ~/.config/xray/config.json.bak
  انجام شد.

پیکربندی در /root/.config/snix/config.yaml ذخیره شد.
برای اجرا: `sudo snix start`
```

</div>

اگر در گامِ ۱ «n» بگویید، دستیار پیشنهادِ ساختِ Cloudflare Worker می‌دهد:

<div dir="ltr" markdown="1">

```
  هنوز سرورِ پروکسی ندارید. snix می‌تواند یک سرورِ رایگان رویِ
  Cloudflare در حدودِ ۶۰ ثانیه برایتان بسازد.

  آیا Cloudflare Worker مستقر شود؟ (Y/n) y
  این آدرس را در مرورگر باز کنید:
    https://dash.cloudflare.com/?to=/:account/workers/services/new?name=snix-<random>
  پس از اتمام، آدرسِ Worker را اینجا جای‌گذاری کنید:
  > worker-name.username.workers.dev
```

</div>

## استفادهٔ روزانه

### TUI

<div dir="ltr" markdown="1">

```bash
snix tui
```

</div>

عددِ ۱ تا ۷ برایِ تعویضِ تب، `?` برای راهنما، `q` برای خروج.

### CLI + systemd (لینوکس)

<div dir="ltr" markdown="1">

```bash
sudo systemctl enable --now snix
journalctl -u snix -f
sudo systemctl stop snix
```

</div>

### CLI + Services (ویندوز)

اگر در نصب گزینهٔ «Install as Service» را انتخاب کرده باشید:

<div dir="ltr" markdown="1">

```cmd
sc query snix
sc start snix
sc stop snix
```

</div>

## راهنمایِ TUI

هر تب به اختصار:

- **Home** — فایلِ پیکربندیِ بارگذاری‌شده، تعدادِ profile، profileِ فعال، توضیحِ کوتاهِ snix.
- **Profiles** — فهرستِ profileها. `⏎` انتخاب برایِ فعال کردن، `e` بازکردن در ویرایشگر، `r` بارگذاریِ مجدد.
- **Scan** — اسکنِ SNI و IP. `s` شروعِ SNI، `i` شروعِ IP، `a` اسکنِ همزمانِ هر دو، `t` ویرایشِ هدفِ SNI، `x` توقف، `e` ذخیره در profileِ فعال.
- **Run** — اجرایِ موتور. `s` شروع، `x` توقف، `r` راه‌اندازیِ مجدد، `c` پاک‌کردنِ لاگ.
- **Settings** — کلیدهایِ ضد-اثرانگشت. `↑↓` حرکت، `space` یا `⏎` تغییر، `+/−` تنظیمِ عددی.
- **Help** — راهنمایِ کامل.
- **About** — نسخه، مجوز، اعتبار.

## مرجعِ CLI

<div dir="ltr" markdown="1">

```
snix init                     ساختِ پیکربندیِ اولیه
snix init --wizard            راه‌اندازیِ تعاملی (توصیه‌شده)
snix status                   نمایشِ پیکربندیِ بارگذاری‌شده و profileِ فعال
snix tui                      اجرایِ TUIِ کامل
snix start                    اجرایِ موتور (نیازمند root/admin)
snix start -p NAME            اجرا با profileِ غیرفعال
snix scan sni --target IP     اسکنِ SNI
snix scan ip                  اسکنِ IPهایِ Cloudflare
snix scan all                 اسکنِ کامل و چاپِ پیکربندیِ آماده
snix profile list             فهرستِ profileها
snix profile switch NAME      تغییرِ profileِ فعال
snix update                   ارتقا به آخرین نسخه
```

</div>

## مرجعِ پیکربندی

مکانِ پیش‌فرض:
- لینوکس/macOS: `~/.config/snix/config.yaml` (یا `$XDG_CONFIG_HOME/snix/config.yaml`).
- ویندوز: `%APPDATA%\snix\config.yaml`.

نمونهٔ کامل در بخشِ انگلیسی بالا آمده — ساختار دقیقاً همان است و تمامِ کلیدها توضیح داده شده‌اند.

## کلیدهایِ ضد-اثرانگشت

- **`randomize_timing`** — نسخهٔ اصلی دقیقاً ۱ میلی‌ثانیه پس از ACK بسته می‌فرستد؛ DPI می‌تواند این الگو را تشخیص دهد. jitterِ تصادفی در بازهٔ [۵۰۰µs، ۵ms] این ثابت را نویزی می‌کند.
- **`randomize_padding`** — اندازهٔ بستهٔ اصلی همیشه ۵۱۷ بایت است. با `max_extra_pad: 600`، snix بسته‌هایی بین ۵۱۷ تا ۱۱۱۷ بایت می‌سازد.
- **`strategy_rotation`** — تنها `wrong_seq` همیشه شکلِ یکسانی می‌سازد. با چرخش بینِ `wrong_seq` و `wrong_checksum` نصفِ اتصال‌ها شکلِ دیگری دارند.
- **`ip_id_delta_range`** — به‌جایِ +۱ ثابت، بازهٔ [۱، ۶۴] را تصادفی می‌کند.
- **`sni_selection: random`** — round-robin پیش‌بینی‌پذیر است؛ random شانسِ هر SNI را یکنواخت می‌کند.

پیکربندیِ توصیه‌شده برای استفادهٔ واقعی:

<div dir="ltr" markdown="1">

```yaml
spoof:
  strategy_rotation: [wrong_seq, wrong_checksum]
  sni_pool: [... دستِ‌کم ۳ SNI ...]
  sni_selection: random
  randomize_timing: true
  min_delay: 500us
  max_delay: 5ms
  randomize_padding: true
  max_extra_pad: 600
  ip_id_delta_range: 64
```

</div>

## اتصالِ snix به کلاینتِ پروکسی

کلاینتِ پروکسیِ شما باید به `127.0.0.1:40443` متصل شود. یعنی در تنظیماتِ کلاینت، آدرسِ سرور را `127.0.0.1` و پورت را `40443` بگذارید، و در پیکربندیِ snix آدرسِ واقعیِ سرور در `connect.host` و `connect.port` باشد.

دستیارِ راه‌اندازی (`snix init --wizard` گامِ ۵) این کار را خودکار برایِ Xray / v2ray / sing-box / NekoBox انجام می‌دهد — همیشه اول نسخهٔ پشتیبانِ پیکربندیِ شما را می‌گیرد.

## عیب‌یابی

- **«No config loaded»** → `snix init` یا `snix init --wizard` را اجرا کنید.
- **`ERROR_ACCESS_DENIED (run as Administrator)` در ویندوز** → از شلِ بالارفته اجرا کنید.
- **`WinDivert.dll not loadable` در ویندوز** → هر سه فایل باید در یک پوشه باشند.
- **Scan گزارش «0 OK» می‌دهد** → `--target` را عوض کنید (پیش‌فرض `1.1.1.1` است چون سرتیفیکیتِ پیش‌فرض دارد).
- **اتصال پس از ۳۰ ثانیه قطع می‌شود** → تمامِ کلیدهایِ ضد-اثرانگشت را روشن کنید.
- **لاگ‌ها کجاست؟**
  - `snix start` → stdout/stderr
  - systemd → `journalctl -u snix`
  - سرویسِ ویندوز → Event Viewer → Application
  - تبِ Run در TUI → بافرِ زنده (ناپایدار)
- **به‌روزرسانی** → `snix update` (با تأییدِ امضا) یا اجرایِ دوبارهٔ نصب‌کننده.

## نسبت با patterniha/SNI-Spoofing

snix یک بازنویسی در Go از
[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing) است.
آن پروژه ترفندِ wrong-seq را کشف و پیاده کرده که همهٔ این کار رویِ شانه‌ی
آن ایستاده. اگر snix دوست‌تان آمد، **لطفاً repoِ ایشان را هم ستاره بزنید**.
این پروژه بدونِ کارِ ایشان وجود نداشت.

موتورِ دور زدنِ ما برایِ ترفندِ اصلی **دقیقاً بایت-به-بایت همانِ
upstream** است: ClientHelloِ جعلی، شمارهٔ توالیِ اشتباه، زمان‌بندی، و
ماشینِ حالت همگی بسته‌هایی هم‌سان در شبکه تولید می‌کنند. این تطابق با یک
تستِ «golden» قفل شده که بایت‌هایِ مرجع را از پایتونِ upstream تولید
می‌کند و هر buildِ نا-منطبق را fail می‌کند.

رویِ همین پایهٔ یکسان این‌ها را اضافه کرده‌ایم:

| مورد | patterniha/SNI-Spoofing | snix |
|---|---|---|
| **هستهٔ بای‌پس (`wrong_seq`)** | ✓ | ✓ *(بایت-به-بایت یکسان)* |
| پلتفرم‌ها | فقط ویندوز | **ویندوز + لینوکس** *(macOS و اندروید در برنامه)* |
| وابستگیِ اجرا | Python + pydivert + WinDivert | **یک باینریِ ۱۰ مگابایتی** *(بدونِ cgo، بدونِ پایتون)* |
| استراتژیِ دومِ بای‌پس | ندارد | **`wrong_checksum`** با `wrong_seq` چرخش می‌کند |
| randomization زمانی | ۱ میلی‌ثانیهٔ ثابت | یکنواخت در `[min, max]` (مثلاً ۵۰۰µs تا ۵ms) |
| randomization اندازه | ثابتِ ۵۱۷ بایت | `[۵۱۷, ۵۱۷ + max_extra_pad]` |
| randomization IP-ID | همیشه `+1` | یکنواخت در `[1, IP ID delta range]` |
| استراتژیِ SNI | یک SNIِ ثابت | استخر + انتخابِ تصادفی یا round-robin |
| اسکنرِ شبکه | ندارد | **`snix scan`**: SNI و IPهایِ CDN را بررسی می‌کند، بر اساسِ موفقیت و تأخیر رتبه می‌دهد |
| دستیارِ راه‌اندازیِ اول | ندارد | **`snix init --wizard`**: ۵ پرسش تا profileِ کاری |
| ادغام با کلاینتِ پروکسی | دستی | **شناساییِ خودکارِ Xray / v2ray / sing-box** و ویرایشِ پیکربندیِ آن‌ها (با پشتیبان) |
| مسیرِ سرورِ رایگان | ندارد | قالبِ **Cloudflare Worker** + دستورالعملِ استقرار |
| UI | فقط CLI | **CLI + TUIِ کامل** (داشبورد، profileها، اسکن، لاگِ زنده، تنظیمات، راهنما) |
| چند profile | ویرایشِ فایلِ پیکربندی | **جابه‌جاییِ profileِ فعال در زمانِ اجرا** |
| به‌روزرسانیِ خودکار | ندارد | **`snix update`** (با تأییدِ امضا) |
| خاموشیِ تمیز | ندارد | قوانینِ iptables و WinDivert را هنگامِ خروج حذف می‌کند |
| پوششِ تست | ندارد | **race-detector تمیز**، تست‌هایِ golden، تستِ واحد به ازایِ هر بسته |
| CI و release امضاشده | ندارد | ماتریسِ GitHub Actions + minisign + SBOM |
| بومی‌سازی | انگلیسی + فارسی (README) | **انگلیسی + فارسی** (README + راهنمایِ کاملِ کاربر) |

**اصلِ طراحی:** هیچ‌گاه قابلیتی را که upstream دارد حذف نمی‌کنیم. اگر
همهٔ کلیدهایِ randomization را خاموش کنید، snix دقیقاً مثلِ
patterniha/SNI-Spoofing رفتار می‌کند: همان بسته در سیم، همان زمان‌بندی،
همان حالت‌ها. لایهٔ randomization فقط اضافه است.

جاهایی که از upstream **سخت‌گیرتر** هستیم: فیلترِ بسته را به جفتِ دقیقِ
IP+port محدود می‌کنیم تا ترافیکِ غیرِ مربوط وارد موتور نشود. جاهایی که
**آزادتر** هستیم: اتصال را در صورتِ دیدنِ بستهٔ غیرمنتظره قطع نمی‌کنیم.
بای‌پس را برایِ آن جریان رد می‌کنیم ولی اتصالِ کاربر زنده می‌ماند. هر دو
بهبودهایِ ایمنی‌اند که رفتارِ هسته را تغییر نمی‌دهند.

اگر نسخهٔ کوچک‌تر، ساده‌تر، و فقط-ویندوزِ پایتون را می‌خواهید، از upstream
استفاده کنید. اگر همان بای‌پس را به‌صورتِ یک باینریِ امضاشده برایِ لینوکس
و ویندوز، به‌همراهِ wizard و scanner و TUI می‌خواهید، این snix است.

## اعتبار و مجوز

- الگوریتمِ اصلی و اولین پیاده‌سازی: [patterniha](https://github.com/patterniha) — **این پروژه بدون کارِ ایشان وجود نداشت**. هستهٔ wrong-seq از ایشان است؛ بقیه بازنویسیِ چند‌پلتفرمی و گسترش است.
- [WinDivert](https://reqrypt.org/windivert.html) اثرِ Basil Fierz — درایورِ امضاشده‌ای که در ویندوز استفاده می‌کنیم.
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) و [Lipgloss](https://github.com/charmbracelet/lipgloss) — چارچوبِ TUI.
- تیمِ netfilterِ کرنلِ لینوکس — برایِ NFQUEUE.

حمایت از نویسندهٔ اصلی:
<div dir="ltr" markdown="1">

```
USDT (BEP20): 0x76a768B53Ca77B43086946315f0BDF21156bF424
Telegram:     @patterniha
```

</div>

مجوز: GPL-3.0. مطابقِ upstream. در فایلِ [LICENSE](LICENSE).

</div>
