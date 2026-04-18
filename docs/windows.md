# snix on Windows

snix uses [WinDivert](https://reqrypt.org/windivert.html) to intercept TCP
packets in the Windows kernel. WinDivert ships as two files:

- `WinDivert.dll` — user-mode library we call via `windows.LazyDLL`
- `WinDivertNN.sys` — kernel driver (signed by the WinDivert maintainer with
  an EV certificate; Windows loads it on first open without any user action)

## One-time install

1. Download the latest WinDivert 2.x zip from
   https://github.com/basil00/Divert/releases (we pin v2.2.2).
2. Extract `x64/WinDivert.dll` and `x64/WinDivert64.sys` into the same
   directory as `snix.exe`. That's it — no installer runs.
3. Run `snix.exe` from an **elevated** command prompt the first time. Windows
   loads the signed `WinDivert64.sys` driver automatically on first
   `WinDivertOpen` call. Subsequent elevated launches reuse the loaded driver.

## Running snix

```cmd
REM elevated prompt
cd C:\path\to\snix
snix.exe start -c config.yaml
```

You'll see:

```
snix: WinDivert filter installed
snix: bypass engine running
snix: listening on 127.0.0.1:40443
```

## Common errors

- `ERROR_ACCESS_DENIED (run as Administrator)` — you started from a
  non-elevated shell. snix does an explicit pre-check and prints the
  friendlier `snix/windows: process is not running elevated` message before
  even calling WinDivert.
- `ERROR_INVALID_IMAGE_HASH` — Secure Boot is enforced in a mode that
  rejects the WinDivert driver. Either disable Secure Boot or use the
  WinDivert variant signed with Microsoft's Attestation CA. Most users don't
  see this.
- `ERROR_DRIVER_BLOCKED` — driver was previously unloaded but the service
  entry still exists. Reboot once and it clears.

## Architecture note

WinDivert bindings are **pure-syscall** (no cgo). That means:

- snix.exe is a single file you can copy anywhere — no MinGW / MSVC / CRT.
- Building from source only needs `go 1.23+`; no C toolchain.
- One 8.6 MB .exe plus the two WinDivert files is the entire distribution.

See `platform/windows/windivert_api.go` for the binding implementation.
