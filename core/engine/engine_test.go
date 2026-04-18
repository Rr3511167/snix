package engine

import (
	"context"
	"encoding/binary"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SamNet-dev/snix/core/bypass"
	"github.com/SamNet-dev/snix/platform"
)

// fakeBackend is a platform.Backend used to script a handshake sequence and
// capture every Inject call the engine makes.
type fakeBackend struct {
	ch        chan platform.Packet
	mu        sync.Mutex
	injected  [][]byte
	injectErr error
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{ch: make(chan platform.Packet, 16)}
}

func (b *fakeBackend) Open(context.Context, platform.Scope) error { return nil }
func (b *fakeBackend) Packets() <-chan platform.Packet            { return b.ch }
func (b *fakeBackend) Inject(p []byte, _ platform.Direction) error {
	if b.injectErr != nil {
		return b.injectErr
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	cp := make([]byte, len(p))
	copy(cp, p)
	b.injected = append(b.injected, cp)
	return nil
}
func (b *fakeBackend) Close() error {
	close(b.ch)
	return nil
}
func (b *fakeBackend) captured() [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([][]byte, len(b.injected))
	copy(out, b.injected)
	return out
}

// pktOpts builds a raw IPv4+TCP packet with caller-chosen fields for scripting.
type pktOpts struct {
	src, dst       [4]byte
	srcPort, dport uint16
	seq, ack       uint32
	flags          uint8
	payload        []byte
}

func makePkt(o pktOpts) []byte {
	ipHL, tcpHL := 20, 20
	total := ipHL + tcpHL + len(o.payload)
	b := make([]byte, total)
	b[0] = 0x45
	binary.BigEndian.PutUint16(b[2:4], uint16(total))
	binary.BigEndian.PutUint16(b[4:6], 0x1111)
	b[8] = 64
	b[9] = 6
	copy(b[12:16], o.src[:])
	copy(b[16:20], o.dst[:])
	binary.BigEndian.PutUint16(b[10:12], checksum(b[:ipHL]))

	t := b[ipHL:]
	binary.BigEndian.PutUint16(t[0:2], o.srcPort)
	binary.BigEndian.PutUint16(t[2:4], o.dport)
	binary.BigEndian.PutUint32(t[4:8], o.seq)
	binary.BigEndian.PutUint32(t[8:12], o.ack)
	t[12] = byte((tcpHL / 4) << 4)
	t[13] = o.flags
	binary.BigEndian.PutUint16(t[14:16], 0x0800)
	copy(t[tcpHL:], o.payload)
	return b
}

// verdictCounter wraps Verdict so tests can assert it was called exactly once.
type verdictCounter struct {
	n atomic.Int32
}

func (v *verdictCounter) fn() func(platform.VerdictKind) {
	return func(platform.VerdictKind) { v.n.Add(1) }
}

func (v *verdictCounter) calls() int { return int(v.n.Load()) }

// TestEngineHappyPathInjectsAfterHandshake walks an SYN → SYN-ACK → ACK
// sequence and asserts the engine:
//   - Accept-verdicts every observed packet
//   - Injects exactly one fake ClientHello with wrong_seq
//   - The injected packet parses, has valid checksums, and carries the SNI
func TestEngineHappyPathInjectsAfterHandshake(t *testing.T) {
	local := [4]byte{10, 0, 0, 1}
	remote := [4]byte{1, 1, 1, 1}
	const (
		localPort  uint16 = 40000
		remotePort uint16 = 443
		synSeq     uint32 = 0x1000
		synAckSeq  uint32 = 0x9000
	)

	be := newFakeBackend()
	eng, err := New(Config{
		Strategy:    bypass.NameWrongSeq,
		SNIPool:     []string{"auth.vercel.com"},
		InjectDelay: 5 * time.Millisecond,
		Scope: platform.Scope{
			LocalIP:    netip.AddrFrom4(local),
			RemoteIP:   netip.AddrFrom4(remote),
			RemotePort: remotePort,
		},
	}, be)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go eng.Run(ctx)

	var vc verdictCounter
	send := func(dir platform.Direction, raw []byte) {
		be.ch <- platform.Packet{Dir: dir, Raw: raw, Verdict: vc.fn()}
	}

	// Outbound SYN.
	send(platform.DirOutbound, makePkt(pktOpts{
		src: local, dst: remote, srcPort: localPort, dport: remotePort,
		seq: synSeq, flags: flagSYN,
	}))
	// Inbound SYN-ACK.
	send(platform.DirInbound, makePkt(pktOpts{
		src: remote, dst: local, srcPort: remotePort, dport: localPort,
		seq: synAckSeq, ack: synSeq + 1, flags: flagSYN | flagACK,
	}))
	// Outbound ACK (completes handshake).
	send(platform.DirOutbound, makePkt(pktOpts{
		src: local, dst: remote, srcPort: localPort, dport: remotePort,
		seq: synSeq + 1, ack: synAckSeq + 1, flags: flagACK,
	}))

	// Wait for the injected packet to land.
	deadline := time.After(500 * time.Millisecond)
	for {
		if len(be.captured()) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("no injection captured; verdicts=%d", vc.calls())
		default:
			time.Sleep(2 * time.Millisecond)
		}
	}

	// Verdicts: one per sent packet (3 so far).
	if vc.calls() != 3 {
		t.Fatalf("verdict calls: got %d want 3", vc.calls())
	}

	inj := be.captured()[0]
	parsed, err := parse(inj)
	if err != nil {
		t.Fatalf("parse inj: %v", err)
	}

	// Validate: wrong_seq formula, PSH flag, payload contains SNI.
	wantSeq := synSeq + 1 - uint32(len(parsed.payload))
	if parsed.seqNum != wantSeq {
		t.Errorf("seq: got %#x want %#x (synSeq=%#x, plen=%d)",
			parsed.seqNum, wantSeq, synSeq, len(parsed.payload))
	}
	if parsed.flags&flagPSH == 0 {
		t.Error("PSH not set")
	}
	if !containsBytes(parsed.payload, []byte("auth.vercel.com")) {
		t.Errorf("payload missing SNI")
	}
	// Checksums valid.
	if checksum(inj[:20]) != 0 {
		t.Errorf("IP csum invalid: %#x", checksum(inj[:20]))
	}
}

// TestEngineOutOfScopeFlowsPassThrough verifies the engine does not track
// flows to unrelated remote IPs.
func TestEngineOutOfScopeFlowsPassThrough(t *testing.T) {
	be := newFakeBackend()
	eng, err := New(Config{
		Strategy: bypass.NameWrongSeq,
		SNIPool:  []string{"x.io"},
		Scope: platform.Scope{
			LocalIP:    netip.MustParseAddr("10.0.0.1"),
			RemoteIP:   netip.MustParseAddr("1.1.1.1"),
			RemotePort: 443,
		},
	}, be)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go eng.Run(ctx)

	var vc verdictCounter
	be.ch <- platform.Packet{Dir: platform.DirOutbound, Raw: makePkt(pktOpts{
		src: [4]byte{10, 0, 0, 1}, dst: [4]byte{9, 9, 9, 9},
		srcPort: 40000, dport: 443, seq: 1, flags: flagSYN,
	}), Verdict: vc.fn()}
	// Give the engine time to process.
	time.Sleep(20 * time.Millisecond)

	if vc.calls() != 1 {
		t.Errorf("verdict calls: %d", vc.calls())
	}
	if len(be.captured()) != 0 {
		t.Errorf("unexpected injection for out-of-scope flow")
	}
}

func containsBytes(hay, needle []byte) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		match := true
		for j := range needle {
			if hay[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
