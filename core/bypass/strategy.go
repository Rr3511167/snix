// Package bypass defines the platform-agnostic DPI-bypass strategies.
//
// A Strategy takes an observed TCP ACK packet closing the 3-way handshake and
// returns the transformations needed to emit a spoofed ClientHello that the
// DPI accepts but the real server drops. Strategies are pure computation;
// actual packet I/O lives in the platform/* backends.
package bypass

import (
	"errors"
	"fmt"
)

// Name identifies a bypass strategy in configs and logs.
type Name string

const (
	// NameWrongSeq injects a fake ClientHello with a deliberately out-of-window
	// TCP sequence number. Matches upstream patterniha/SNI-Spoofing behaviour.
	NameWrongSeq Name = "wrong_seq"
	// NameWrongChecksum (planned) emits the fake packet with a broken TCP
	// checksum so the server drops it while the DPI — which often skips
	// checksum validation — still reads the SNI.
	NameWrongChecksum Name = "wrong_checksum"
	// NameTTL (planned) sets a TTL that expires between the DPI box and the
	// server, so only the DPI ingests the fake packet.
	NameTTL Name = "ttl"
)

// Inputs are the handshake observations a Strategy may use.
type Inputs struct {
	// SynSeq is the sequence number of the SYN our side sent.
	SynSeq uint32
	// SynAckSeq is the sequence number of the SYN-ACK the server sent.
	SynAckSeq uint32
	// FakePayload is the fake ClientHello bytes to inject.
	FakePayload []byte
}

// Mutation describes how to derive the injection packet from the final
// client-sent ACK of the 3-way handshake.
type Mutation struct {
	// SeqNum is the TCP sequence number to put on the fake packet.
	// Ignored if OverrideSeq is false (in which case the caller's in-window
	// ACK seq is used).
	SeqNum      uint32
	OverrideSeq bool
	// SetPSH indicates the PSH flag should be set on the fake packet.
	SetPSH bool
	// IPIDDelta is how much to add to the source ACK's IPv4 identification
	// field. Upstream uses +1; anti-fingerprinting may randomize.
	IPIDDelta uint16
	// Payload is the TCP payload bytes (the fake ClientHello).
	Payload []byte
	// CorruptChecksum, if true, tells the injector to deliberately emit the
	// packet with an invalid TCP checksum so the destination host drops it
	// while still-permissive DPI middleboxes (which skip checksum validation)
	// read the embedded SNI. Used by NameWrongChecksum.
	CorruptChecksum bool
}

// Strategy computes the mutation for a given handshake.
type Strategy interface {
	// Name returns the strategy identifier.
	Name() Name
	// Plan returns the mutation to apply, or an error if inputs are invalid.
	Plan(in Inputs) (Mutation, error)
}

// ErrInvalidInputs is returned by Plan when preconditions are unmet.
var ErrInvalidInputs = errors.New("snix/bypass: invalid inputs")

// WrongSeq is the default strategy used by upstream: send the fake
// ClientHello with seq = SynSeq + 1 - len(payload), which is wildly
// out-of-window for the real server but visible to a DPI that does not
// reassemble TCP streams.
type WrongSeq struct{}

// Name reports "wrong_seq".
func (WrongSeq) Name() Name { return NameWrongSeq }

// Plan returns the mutation described for NameWrongSeq.
func (WrongSeq) Plan(in Inputs) (Mutation, error) {
	if len(in.FakePayload) == 0 {
		return Mutation{}, fmt.Errorf("%w: empty fake payload", ErrInvalidInputs)
	}
	// Wraps on uint32; Go's modular arithmetic handles this exactly like the
	// upstream Python `& 0xffffffff`.
	seq := in.SynSeq + 1 - uint32(len(in.FakePayload))
	return Mutation{
		SeqNum:      seq,
		OverrideSeq: true,
		SetPSH:      true,
		IPIDDelta:   1,
		Payload:     in.FakePayload,
	}, nil
}

// WrongChecksum injects the fake ClientHello with BOTH an out-of-window
// sequence number AND a corrupted TCP checksum. Two independent mechanisms
// ensure the destination host drops it (belt-and-suspenders), while a DPI
// that does not verify checksums OR does not reassemble TCP streams still
// reads the SNI.
//
// Why both: in testing, corrupting the checksum alone caused real
// connections to stall because our injected packet's valid seq+ack would
// land in-window alongside the subsequent real ClientHello, and some
// middleboxes (tested against Cloudflare 1.1.1.1) silently wedged the flow
// when they saw overlapping-but-checksum-bad segments. Moving the seq
// out-of-window removes the overlap and restores normal traffic while still
// presenting an alternate "shape" to the DPI fingerprinter than WrongSeq
// alone — the packet has the same seq as WrongSeq but also a bad TCP csum.
type WrongChecksum struct{}

// Name reports "wrong_checksum".
func (WrongChecksum) Name() Name { return NameWrongChecksum }

// Plan returns the mutation for NameWrongChecksum.
func (WrongChecksum) Plan(in Inputs) (Mutation, error) {
	if len(in.FakePayload) == 0 {
		return Mutation{}, fmt.Errorf("%w: empty fake payload", ErrInvalidInputs)
	}
	seq := in.SynSeq + 1 - uint32(len(in.FakePayload))
	return Mutation{
		SeqNum:          seq,
		OverrideSeq:     true,
		SetPSH:          true,
		IPIDDelta:       1,
		Payload:         in.FakePayload,
		CorruptChecksum: true,
	}, nil
}

// ByName looks up a registered strategy by its Name.
func ByName(n Name) (Strategy, error) {
	switch n {
	case NameWrongSeq:
		return WrongSeq{}, nil
	case NameWrongChecksum:
		return WrongChecksum{}, nil
	default:
		return nil, fmt.Errorf("%w: unknown strategy %q", ErrInvalidInputs, n)
	}
}
