# Contributing to snix

Thanks for your interest. snix is a circumvention tool used by people in
high-risk situations, so quality and conservatism matter more than velocity.

## Before you start

- **Bug reports**: open an issue with the
  [bug template](https://github.com/SamNet-dev/snix/issues/new?template=bug.yml).
  Include OS, snix version (`snix --version`), the exact command you ran,
  and the first 30 lines of output. If you have a packet capture, even
  better — but redact any personal info first.
- **Feature requests**: open a discussion first. Not every feature belongs
  in core; some should be plugins (see `core/bypass` and the planned
  strategy plugin architecture).
- **Questions**: GitHub Discussions, not issues.

## Local development

```bash
git clone https://github.com/SamNet-dev/snix.git
cd snix
go test ./...
go vet ./...
go build ./cmd/snix
```

Go 1.23+ required. No cgo. No other dependencies to install (WinDivert is
a runtime-only requirement for the Windows engine subprocess, not the
build).

### Cross-compile

```bash
GOOS=linux   GOARCH=amd64 go build -o snix-linux   ./cmd/snix
GOOS=windows GOARCH=amd64 go build -o snix.exe      ./cmd/snix
GOOS=darwin  GOARCH=arm64 go build -o snix-macos    ./cmd/snix
```

### Integration tests (Linux)

```bash
sudo go test -tags=integration -count=1 ./platform/linux/
```

These install real iptables rules (scoped to a test-net IP so they can
never affect production traffic) and require root.

## Coding conventions

- **Comments are sparse.** We explain *why*, not *what*. Identifiers carry
  the what. Don't add a comment unless a future reader would be confused
  without it.
- **Errors returned, not logged.** The caller decides what to do.
- **No panics in library code.** Tests may panic. Packet-parsing paths
  MUST return errors on malformed input.
- **Platform code behind `//go:build` tags.** Every `platform/<os>/` file
  has a matching stub so `go build ./...` succeeds on every host.
- **Tests in the package under test.** Black-box tests in `package_test`
  only when validating the exported surface.
- **Golden tests for byte-exact behavior.** See
  `core/injector/clienthello_test.go` for the pattern.

## Adding a bypass strategy

1. Add the Name constant to `core/bypass/strategy.go`.
2. Implement the `Strategy` interface.
3. Add tests covering the Plan output for typical + edge-case inputs.
4. Register in `ByName`.
5. If the strategy touches new packet fields, extend `injectSpec` in
   `core/engine/packet.go`.
6. Document the strategy in `docs/strategies.md` including when it wins,
   when it loses, and what fingerprint it exposes.

## Pull request checklist

- [ ] `go vet ./...` clean
- [ ] `go test ./...` passes
- [ ] Cross-compile works for Linux + Windows
- [ ] Changes documented (README, docs/, or strategy-specific doc)
- [ ] No new dependencies without discussing in an issue
- [ ] Commit messages follow `type: summary` (feat, fix, docs, refactor,
  test, chore — same as conventional commits, lowercase)

## What we won't merge

- New features that require cgo.
- Logging that leaks user data (IPs, domains visited, timestamps
  correlated with content). Everything we log is coarse and local.
- Telemetry that isn't opt-in and locally auditable.
- Dependencies with unclear licensing or history.

## Licence

By contributing you agree your changes are licensed under GPL-3.0 (matches
upstream). If you copy code from elsewhere, it must be GPL-3.0 compatible
and you MUST note the source + upstream license in the commit body.
