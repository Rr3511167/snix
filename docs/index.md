# snix

Cross-platform SNI-spoofing DPI-bypass proxy. Single binary. No cgo.

## What is it

snix injects a fake TLS ClientHello containing a whitelisted SNI during
the TCP handshake, tricking DPI-based censorship into accepting your
connection. The real TLS handshake proceeds unobstructed.

It is a Go rewrite of [patterniha/SNI-Spoofing][upstream] with:

- Linux + Windows backends (macOS + Android planned).
- Anti-fingerprinting: size + timing + IP ID randomization, strategy rotation.
- Built-in SNI/IP scanner.
- Full terminal UI.
- First-run wizard.

[upstream]: https://github.com/patterniha/SNI-Spoofing

## Where the documentation lives

The long-form user guides currently live alongside the source:

- **English**: [GUIDE.md](https://github.com/SamNet-dev/snix/blob/main/GUIDE.md)
  — zero-to-finish walkthrough covering Cloudflare Worker setup, VPS,
  custom domains, Xray config, and troubleshooting.
- **Farsi / فارسی**: [GUIDE-fa.md](https://github.com/SamNet-dev/snix/blob/main/GUIDE-fa.md)
- **Short reference**: [README.md](https://github.com/SamNet-dev/snix/blob/main/README.md)

This docs site will be filled in page-by-page over time; for now please
read the guides above.

## Quick links

- [Windows setup details](windows.md)
- [Releases + downloads](https://github.com/SamNet-dev/snix/releases)
- [Issues + discussions](https://github.com/SamNet-dev/snix)
