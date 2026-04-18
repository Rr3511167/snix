// Package conntrack tracks per-connection state across the TCP handshake so
// the bypass engine knows when to inject the fake ClientHello and whether
// the real server acknowledged it.
//
// This is the Go port of the upstream MonitorConnection + FakeInjectiveConnection
// classes, split along state vs. I/O concerns.
package conntrack

import (
	"net/netip"
	"sync"
)

// HandshakeStage tracks how far through the TCP 3-way handshake we are, and
// whether the fake ClientHello has been emitted yet. Transitions are linear:
//
//	Init → SynSent → SynAckSeen → AckSent → FakeScheduled → FakeSent → FakeAcked
//	(or → Failed at any point)
type HandshakeStage uint8

const (
	StageInit HandshakeStage = iota
	StageSynSent
	StageSynAckSeen
	StageAckSent
	StageFakeScheduled
	StageFakeSent
	StageFakeAcked
	StageFailed
)

// Tuple is the 4-tuple identifying a TCP connection.
type Tuple struct {
	SrcIP   netip.Addr
	SrcPort uint16
	DstIP   netip.Addr
	DstPort uint16
}

// Connection is the mutable per-flow state observed by the packet injector.
// All field access must hold Mu.
type Connection struct {
	Mu sync.Mutex

	Tuple Tuple

	// Sequence numbers learned from observed packets. Zero means "unknown";
	// callers should gate on Stage rather than checking these directly.
	SynSeq    uint32
	SynAckSeq uint32

	Stage HandshakeStage

	// FakePayload is the fake ClientHello bytes to inject after the handshake.
	FakePayload []byte

	// Done is closed exactly once when the handshake reaches a terminal state
	// (FakeAcked or Failed). Consumers wait on it to learn the outcome.
	Done   chan struct{}
	Result Result

	// Monitor indicates whether the packet filter should still intercept this
	// flow. Flipped to false by the owner once the bypass completes or fails,
	// after which packets pass through unchanged.
	Monitor bool
}

// Result carries the terminal outcome reported to the connection owner.
type Result struct {
	OK     bool
	Reason string
}

// New returns a fresh Connection in StageInit with Monitor enabled.
func New(t Tuple, fakePayload []byte) *Connection {
	return &Connection{
		Tuple:       t,
		Stage:       StageInit,
		FakePayload: fakePayload,
		Done:        make(chan struct{}),
		Monitor:     true,
	}
}

// Finish records the outcome and closes Done. Idempotent under Mu.
// Callers MUST hold c.Mu.
func (c *Connection) Finish(ok bool, reason string) {
	if c.Stage == StageFakeAcked || c.Stage == StageFailed {
		return
	}
	c.Result = Result{OK: ok, Reason: reason}
	if ok {
		c.Stage = StageFakeAcked
	} else {
		c.Stage = StageFailed
	}
	c.Monitor = false
	close(c.Done)
}

// Table is a thread-safe map of in-flight connections keyed by 4-tuple.
type Table struct {
	mu sync.RWMutex
	m  map[Tuple]*Connection
}

// NewTable returns an empty Table.
func NewTable() *Table {
	return &Table{m: make(map[Tuple]*Connection)}
}

// Add registers c. Returns false if an entry for the same tuple already exists.
func (t *Table) Add(c *Connection) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.m[c.Tuple]; exists {
		return false
	}
	t.m[c.Tuple] = c
	return true
}

// Get returns the connection for the given tuple, if any.
func (t *Table) Get(k Tuple) (*Connection, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	c, ok := t.m[k]
	return c, ok
}

// Remove deletes the entry for k. Safe to call repeatedly.
func (t *Table) Remove(k Tuple) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.m, k)
}

// Len returns the current connection count (for metrics/telemetry).
func (t *Table) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.m)
}
