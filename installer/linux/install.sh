#!/bin/sh
# snix installer.
#
#   curl -fsSL https://get.snix.sh | sudo sh
#
# Does:
#   - detect distro + arch
#   - download latest release tarball (or a version pinned via SNIX_VERSION)
#   - verify sha256 + minisign signature
#   - install /usr/local/bin/snix
#   - install systemd unit (Linux with systemd)
#   - install bash + zsh completions
#
# This script is POSIX sh — no bashisms. Tested on Alpine (busybox sh),
# Ubuntu, Debian, Fedora, Arch.

set -eu

REPO="SamNet-dev/snix"
INSTALL_DIR="${SNIX_INSTALL_DIR:-/usr/local/bin}"
BINARY="snix"
VERSION="${SNIX_VERSION:-latest}"
# Paste the minisign public key here once generated + published. Until
# this is a real key, signature verification is skipped (sha256 only).
# When filling in: use the single-line output of `minisign -G` (starts
# with "RWQ" for Ed25519).
PUBKEY=""

# -- output helpers --------------------------------------------------------

tty_bold=""
tty_red=""
tty_green=""
tty_reset=""
if [ -t 1 ]; then
  # shellcheck disable=SC2034
  tty_bold=$(printf '\033[1m')
  tty_red=$(printf '\033[31m')
  tty_green=$(printf '\033[32m')
  tty_reset=$(printf '\033[0m')
fi

info()  { printf '%s==>%s %s\n' "$tty_green$tty_bold" "$tty_reset$tty_bold" "$*$tty_reset"; }
warn()  { printf '%s!! %s%s\n' "$tty_red$tty_bold" "$*" "$tty_reset" >&2; }
die()   { warn "$*"; exit 1; }

# -- preflight --------------------------------------------------------------

[ "$(id -u)" -eq 0 ] || die "please run as root (sudo sh -c 'curl -fsSL https://get.snix.sh | sh')"

need() {
  command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"
}
need curl
need tar
need uname

# -- detect os + arch -------------------------------------------------------

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  linux)    : ;;
  *)        die "unsupported OS: $os (this installer is for Linux; see docs for macOS/Windows)" ;;
esac

arch_raw="$(uname -m)"
case "$arch_raw" in
  x86_64|amd64)          arch="amd64" ;;
  aarch64|arm64)         arch="arm64" ;;
  armv7l|armv7|armhf)    arch="armv7" ;;
  *)                     die "unsupported architecture: $arch_raw" ;;
esac

info "detected linux/$arch"

# -- resolve version --------------------------------------------------------

if [ "$VERSION" = "latest" ]; then
  info "resolving latest release from github.com/$REPO"
  # Use the redirect from /releases/latest to find the tag without needing jq.
  latest_url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
    "https://github.com/$REPO/releases/latest")
  VERSION="${latest_url##*/v}"
fi
case "$VERSION" in
  v*)  VERSION="${VERSION#v}" ;;
esac
info "installing snix v$VERSION"

# -- download --------------------------------------------------------------

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

archive="snix-linux-${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/v$VERSION"
url="$base/$archive"

info "downloading $url"
if ! curl -fsSL -o "$tmp/$archive" "$url"; then
  die "download failed for $url"
fi

# sha256
if curl -fsSL -o "$tmp/$archive.sha256" "$url.sha256" 2>/dev/null; then
  info "verifying sha256"
  (cd "$tmp" && sha256sum -c "$archive.sha256") >/dev/null \
    || die "sha256 mismatch — refusing to install"
else
  warn "no sha256 file published; skipping hash verification"
fi

# minisign — only attempted when a pinned pubkey is compiled into this
# script AND minisign is available on the host. Pre-release installs
# fall back to sha256 only; a warning is printed so the user knows.
if [ -n "$PUBKEY" ] && command -v minisign >/dev/null 2>&1; then
  if curl -fsSL -o "$tmp/$archive.minisig" "$url.minisig" 2>/dev/null; then
    info "verifying minisign signature"
    printf '%s' "$PUBKEY" > "$tmp/pubkey"
    (cd "$tmp" && minisign -Vm "$archive" -p pubkey) \
      || die "minisign verification FAILED — aborting"
  fi
elif [ -z "$PUBKEY" ]; then
  warn "no minisign pubkey pinned in this installer (pre-release build); relying on sha256 only"
elif ! command -v minisign >/dev/null 2>&1; then
  warn "minisign not installed; skipping signature verification"
  warn "install it with your package manager and re-run for full safety"
fi

# -- extract + install -----------------------------------------------------

info "extracting"
tar -xzf "$tmp/$archive" -C "$tmp"

if [ ! -f "$tmp/$BINARY" ]; then
  die "archive did not contain expected binary '$BINARY'"
fi

info "installing $BINARY to $INSTALL_DIR"
install -m 0755 "$tmp/$BINARY" "$INSTALL_DIR/$BINARY"

# -- systemd unit (optional) -----------------------------------------------

if command -v systemctl >/dev/null 2>&1 && [ -d /etc/systemd/system ]; then
  unit_path="/etc/systemd/system/snix.service"
  if [ ! -f "$unit_path" ]; then
    info "installing systemd unit at $unit_path (disabled; enable with: systemctl enable --now snix)"
    cat >"$unit_path" <<'UNIT'
[Unit]
Description=snix SNI-spoofing DPI bypass
Documentation=https://snix.sh
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/snix start
Restart=on-failure
RestartSec=5
# snix installs iptables rules; needs CAP_NET_ADMIN + CAP_NET_RAW.
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/etc/snix /run

[Install]
WantedBy=multi-user.target
UNIT
    systemctl daemon-reload || true
  else
    info "systemd unit already present at $unit_path — leaving it alone"
  fi
fi

# -- shell completions (best-effort) ---------------------------------------

if [ -d /etc/bash_completion.d ]; then
  if "$INSTALL_DIR/$BINARY" completion bash >"$tmp/snix.bash" 2>/dev/null; then
    install -m 0644 "$tmp/snix.bash" /etc/bash_completion.d/snix
    info "bash completion installed"
  fi
fi
if [ -d /usr/share/zsh/vendor-completions ]; then
  if "$INSTALL_DIR/$BINARY" completion zsh >"$tmp/_snix" 2>/dev/null; then
    install -m 0644 "$tmp/_snix" /usr/share/zsh/vendor-completions/_snix
    info "zsh completion installed"
  fi
fi

# -- success ----------------------------------------------------------------

cat <<EOT

${tty_green}${tty_bold}snix v$VERSION installed.${tty_reset}

Next steps:

  1. Create a config with the first-run wizard:
       ${tty_bold}sudo snix init --wizard${tty_reset}

  2. Start the engine in a foreground terminal:
       ${tty_bold}sudo snix start${tty_reset}

     or as a managed service:
       ${tty_bold}sudo systemctl enable --now snix${tty_reset}
       ${tty_bold}journalctl -u snix -f${tty_reset}

Docs: https://snix.sh
Issues: https://github.com/$REPO/issues

EOT
