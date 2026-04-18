//go:build windows

// Package windows implements the snix Backend on Windows via WinDivert.
//
// We talk to WinDivert.dll directly via windows.LazyDLL so no cgo is
// required — the snix binary stays a single .exe. The signed WinDivert.sys
// driver and DLL must be present at runtime; they are loaded by first use
// of the backend and unloaded on Close.
//
// API version: WinDivert 2.x (filter language v2, 80-byte WINDIVERT_ADDRESS).
package windows

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Layer selects what kind of events WinDivert intercepts. We always use
// Network (raw IP packets pre/post-routing).
const (
	LayerNetwork        = 0
	LayerNetworkForward = 1
	LayerFlow           = 2
	LayerSocket         = 3
	LayerReflect        = 4
)

// Flags passed to WinDivertOpen. Default 0 = captures + drops, we re-inject.
const (
	FlagDefault = 0
	FlagSniff   = 0x0001 // Observe only; packets not dropped from stack.
	FlagDrop    = 0x0002 // Silently drop after capture; don't forward.
)

// WinDivertParam values used with SetParam.
const (
	ParamQueueLength = 0 // max packets waiting userspace, default 4096
	ParamQueueTime   = 1 // ms packets may wait, default 2000
	ParamQueueSize   = 2 // bytes of kernel queue, default 4M
)

// Address is the Go mirror of WINDIVERT_ADDRESS (80 bytes). Layout must
// match exactly; we verify via a compile-time assertion below.
type Address struct {
	Timestamp int64
	// Packed flags byte for fields Layer(8), Event(8), Flags(8) bits:
	//   [0] Sniffed,[1] Outbound,[2] Loopback,[3] Impostor,
	//   [4] IPv6,[5] IPChecksum,[6] TCPChecksum,[7] UDPChecksum
	// Followed by Reserved1(8).
	LayerEvent uint32 // [0:8]=Layer, [8:16]=Event, [16:24]=flags, [24:32]=Reserved1
	Reserved2  uint32
	Data       [64]byte // union of per-layer data; for network: iface indexes etc.
}

var _ [80]byte = [unsafe.Sizeof(Address{})]byte{} // compile-time size check

// Outbound extracts the Outbound bit (bit 17 overall: 16 for Layer|Event +1).
func (a *Address) Outbound() bool {
	return a.LayerEvent&(1<<17) != 0
}

// SetOutbound sets/clears the Outbound bit (used when re-injecting).
func (a *Address) SetOutbound(v bool) {
	if v {
		a.LayerEvent |= 1 << 17
	} else {
		a.LayerEvent &^= 1 << 17
	}
}

// IPChecksum reports whether WinDivert wants us to recompute the IP csum on send.
func (a *Address) IPChecksum() bool { return a.LayerEvent&(1<<21) != 0 }

// TCPChecksum reports whether WinDivert wants us to recompute the TCP csum on send.
func (a *Address) TCPChecksum() bool { return a.LayerEvent&(1<<22) != 0 }

// dynamically-loaded bindings. All use stdcall; syscall.SyscallN handles it.
var (
	dll                   = windows.NewLazyDLL("WinDivert.dll")
	procWinDivertOpen     = dll.NewProc("WinDivertOpen")
	procWinDivertRecv     = dll.NewProc("WinDivertRecv")
	procWinDivertSend     = dll.NewProc("WinDivertSend")
	procWinDivertClose    = dll.NewProc("WinDivertClose")
	procWinDivertSetParam = dll.NewProc("WinDivertSetParam")
)

// handleRaw is the opaque HANDLE returned by WinDivertOpen.
type handleRaw uintptr

// invalidHandle matches INVALID_HANDLE_VALUE (-1 cast to HANDLE).
const invalidHandle handleRaw = ^handleRaw(0)

// Open invokes WinDivertOpen. Returns a handle or an error.
func Open(filter string, layer uint32, priority int16, flags uint64) (handleRaw, error) {
	if err := dll.Load(); err != nil {
		return invalidHandle, fmt.Errorf("snix/windows: WinDivert.dll not loadable: %w (install WinDivert and ensure the DLL is on PATH or next to snix.exe)", err)
	}
	fptr, err := windows.BytePtrFromString(filter)
	if err != nil {
		return invalidHandle, err
	}
	r1, _, e1 := procWinDivertOpen.Call(
		uintptr(unsafe.Pointer(fptr)),
		uintptr(layer),
		uintptr(uint16(priority)),
		uintptr(flags),
	)
	h := handleRaw(r1)
	if h == invalidHandle {
		return invalidHandle, fmt.Errorf("WinDivertOpen: %w", translateErr(e1))
	}
	return h, nil
}

// Recv fetches the next packet into buf. Returns the number of bytes and
// the address metadata. On error, n is undefined.
func Recv(h handleRaw, buf []byte) (n uint32, addr Address, err error) {
	var recvLen uint32
	r1, _, e1 := procWinDivertRecv.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(uint32(len(buf))),
		uintptr(unsafe.Pointer(&recvLen)),
		uintptr(unsafe.Pointer(&addr)),
	)
	if r1 == 0 {
		return 0, addr, fmt.Errorf("WinDivertRecv: %w", translateErr(e1))
	}
	return recvLen, addr, nil
}

// Send writes packet pkt back to the stack using addr as the re-injection
// metadata. Returns bytes written.
func Send(h handleRaw, pkt []byte, addr *Address) (uint32, error) {
	if len(pkt) == 0 {
		return 0, fmt.Errorf("send: empty packet")
	}
	var sentLen uint32
	r1, _, e1 := procWinDivertSend.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&pkt[0])),
		uintptr(uint32(len(pkt))),
		uintptr(unsafe.Pointer(&sentLen)),
		uintptr(unsafe.Pointer(addr)),
	)
	if r1 == 0 {
		return 0, fmt.Errorf("WinDivertSend: %w", translateErr(e1))
	}
	return sentLen, nil
}

// Close unloads the WinDivert handle.
func Close(h handleRaw) error {
	r1, _, e1 := procWinDivertClose.Call(uintptr(h))
	if r1 == 0 {
		return fmt.Errorf("WinDivertClose: %w", translateErr(e1))
	}
	return nil
}

// SetParam tunes a kernel parameter on an open handle.
func SetParam(h handleRaw, param uint32, value uint64) error {
	r1, _, e1 := procWinDivertSetParam.Call(
		uintptr(h), uintptr(param), uintptr(value))
	if r1 == 0 {
		return fmt.Errorf("WinDivertSetParam(%d,%d): %w", param, value, translateErr(e1))
	}
	return nil
}

// Windows errno values WinDivert returns. Exposed as typed constants so
// callers (the backend recv loop) can recognize graceful-close without
// resorting to string matching.
const (
	errnoAccessDenied     = 5    // ERROR_ACCESS_DENIED
	errnoInvalidParameter = 87   // ERROR_INVALID_PARAMETER
	errnoNoData           = 232  // ERROR_NO_DATA — returned on a closed handle mid-Recv.
	errnoInvalidImageHash = 577  // ERROR_INVALID_IMAGE_HASH
	errnoDriverBlocked    = 1275 // ERROR_DRIVER_BLOCKED
)

// IsHandleClosed reports whether err is the expected graceful signal
// ("WinDivert handle was closed") that Recv returns after Close().
func IsHandleClosed(err error) bool {
	if err == nil {
		return false
	}
	var en windows.Errno
	if !errors.As(err, &en) {
		return false
	}
	return en == errnoNoData
}

// translateErr keeps the Errno value but adds human-readable context when
// the common codes appear. Makes driver-missing diagnostics obvious.
func translateErr(e error) error {
	if e == nil {
		return nil
	}
	if en, ok := e.(windows.Errno); ok {
		switch en {
		case errnoAccessDenied:
			return fmt.Errorf("ERROR_ACCESS_DENIED (run as Administrator): %w", en)
		case errnoInvalidImageHash:
			return fmt.Errorf("ERROR_INVALID_IMAGE_HASH (WinDivert driver blocked; kernel signature mismatch or Secure Boot issue): %w", en)
		case errnoDriverBlocked:
			return fmt.Errorf("ERROR_DRIVER_BLOCKED (WinDivert driver not installed or disabled): %w", en)
		case errnoInvalidParameter:
			return fmt.Errorf("ERROR_INVALID_PARAMETER (check filter syntax): %w", en)
		case errnoNoData:
			return fmt.Errorf("ERROR_NO_DATA (handle closed): %w", en)
		}
	}
	return e
}
