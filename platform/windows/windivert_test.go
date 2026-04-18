//go:build windows

package windows

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"
)

// TestAddressSize is a compile+runtime check that our Go Address mirrors
// WINDIVERT_ADDRESS's 80-byte layout. A wrong size would silently corrupt
// flags when calling Recv/Send.
func TestAddressSize(t *testing.T) {
	got := unsafe.Sizeof(Address{})
	if got != 80 {
		t.Fatalf("sizeof(Address) = %d, want 80", got)
	}
}

// TestOutboundBit toggles the Outbound bit and verifies it round-trips.
func TestOutboundBit(t *testing.T) {
	var a Address
	if a.Outbound() {
		t.Fatal("zero-value Address reports Outbound=true")
	}
	a.SetOutbound(true)
	if !a.Outbound() {
		t.Fatal("SetOutbound(true) did not stick")
	}
	a.SetOutbound(false)
	if a.Outbound() {
		t.Fatal("SetOutbound(false) did not clear")
	}
}

// TestDLLLoadable is a soft check: if WinDivert.dll is present next to the
// test binary (or on %PATH%), loading it should succeed. We do NOT call
// Open() here because that requires Administrator + the signed driver,
// which not every CI environment provides.
func TestDLLLoadable(t *testing.T) {
	// Put third_party/windivert next to wherever Go is running the test
	// from. `go test` runs with cwd = package dir, so look upward.
	for _, rel := range []string{
		"../../third_party/windivert/WinDivert.dll",
		"../../../third_party/windivert/WinDivert.dll",
	} {
		if _, err := os.Stat(rel); err == nil {
			// Seed the current directory with the DLL so LazyDLL finds it.
			dir := filepath.Dir(rel)
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
	}
	if err := dll.Load(); err != nil {
		t.Skipf("WinDivert.dll not present on PATH or test dir; skipping: %v", err)
	}
	// Confirm one proc symbol resolves.
	if err := procWinDivertOpen.Find(); err != nil {
		t.Fatalf("WinDivertOpen not exported: %v", err)
	}
}
