// Package platform defines the cross-OS backend contract that the DPI
// bypass engine uses to intercept, mutate, and emit raw TCP packets during
// the 3-way handshake.
//
// Each subpackage (linux/, windows/, darwin/) provides a concrete Backend.
// The core engine (built on top) is platform-agnostic.
package platform

import (
	"context"
	"errors"
	"net/netip"
)

// Backend is implemented by per-OS packet interception providers.
//
// Semantics:
//   - Open installs filter rules (iptables / WinDivert filter / pf) and
//     prepares the userspace packet queue.
//   - Close cleans up all side effects (removes rules, unloads drivers).
//   - Packets returns a channel of observed packets in both directions for
//     the configured filter scope. Backends call Verdict on each delivered
//     packet exactly once.
type Backend interface {
	// Open installs the filter rules and starts packet interception. The
	// returned error indicates a fatal setup failure (missing driver,
	// insufficient privileges, rule conflict).
	Open(ctx context.Context, scope Scope) error

	// Packets returns the stream of intercepted packets. The channel is
	// closed when the backend is stopped.
	Packets() <-chan Packet

	// Inject emits a raw IP packet onto the wire, bypassing the filter.
	// Used to send the spoofed ClientHello.
	Inject(p []byte, dir Direction) error

	// Close tears down rules and stops interception. Safe to call multiple
	// times.
	Close() error
}

// Scope narrows packet interception to a specific 5-tuple family.
// Backends translate this into their native filter language.
type Scope struct {
	// LocalIP is this host's address on the interface used to reach RemoteIP.
	LocalIP netip.Addr
	// RemoteIP is the upstream server.
	RemoteIP netip.Addr
	// RemotePort is typically 443.
	RemotePort uint16
}

// Direction indicates which way a packet is flowing relative to this host.
type Direction uint8

const (
	DirOutbound Direction = iota // local -> remote
	DirInbound                   // remote -> local
)

// Packet is an intercepted IP packet passed from the kernel to userspace.
// The Raw slice is owned by the backend; do not retain after calling Verdict.
type Packet struct {
	Dir Direction
	Raw []byte
	// Verdict must be called exactly once, with either Accept or Drop,
	// to release the packet back to the kernel (or discard it).
	Verdict func(VerdictKind)
}

// VerdictKind tells the backend what to do with a queued packet.
type VerdictKind uint8

const (
	// Accept passes the packet through unchanged.
	Accept VerdictKind = iota
	// Drop discards the packet.
	Drop
	// AcceptModified passes a mutated version through (backend-specific).
	AcceptModified
)

// ErrNotSupported is returned by a backend when the current OS or privilege
// level cannot host packet interception.
var ErrNotSupported = errors.New("snix/platform: backend not supported on this host")
