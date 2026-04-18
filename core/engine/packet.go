// Package engine contains the platform-agnostic DPI-bypass state machine:
// it consumes an intercepted IPv4 packet stream, tracks the 3-way handshake
// per flow, and builds a mutated injection packet once the ACK closes the
// handshake.
package engine

import (
	"encoding/binary"
	"errors"
)

// Parsed fields extracted from an intercepted IPv4 + TCP packet. We only
// care about IPv4 with TCP payload for the handshake dance; everything else
// flows through unmodified.
type pkt struct {
	// IPv4 header fields (offsets within the original buffer).
	ipHdrLen int
	totalLen int
	ident    uint16
	srcIP    [4]byte
	dstIP    [4]byte

	// TCP header offsets.
	tcpOff    int // offset of TCP header within raw
	tcpHdrLen int
	srcPort   uint16
	dstPort   uint16
	seqNum    uint32
	ackNum    uint32
	flags     uint8 // TCP flag byte (offset 13 in TCP header)
	payload   []byte
}

// errNotIPv4TCP signals a packet we don't understand (IPv6, non-TCP, truncated).
// The caller should pass it through with Accept.
var errNotIPv4TCP = errors.New("engine: not IPv4+TCP")

// parse extracts IPv4 and TCP fields from raw. Returns a pkt whose slices
// alias raw — do not modify the returned pkt.raw unless you intend to mutate
// the original packet.
func parse(raw []byte) (pkt, error) {
	var p pkt
	if len(raw) < 20 {
		return p, errNotIPv4TCP
	}
	if raw[0]>>4 != 4 {
		return p, errNotIPv4TCP
	}
	p.ipHdrLen = int(raw[0]&0x0f) * 4
	if p.ipHdrLen < 20 || len(raw) < p.ipHdrLen+20 {
		return p, errNotIPv4TCP
	}
	if raw[9] != 6 { // IPPROTO_TCP
		return p, errNotIPv4TCP
	}
	p.totalLen = int(binary.BigEndian.Uint16(raw[2:4]))
	p.ident = binary.BigEndian.Uint16(raw[4:6])
	copy(p.srcIP[:], raw[12:16])
	copy(p.dstIP[:], raw[16:20])

	p.tcpOff = p.ipHdrLen
	p.tcpHdrLen = int(raw[p.tcpOff+12]>>4) * 4
	if p.tcpHdrLen < 20 || len(raw) < p.tcpOff+p.tcpHdrLen {
		return p, errNotIPv4TCP
	}
	p.srcPort = binary.BigEndian.Uint16(raw[p.tcpOff : p.tcpOff+2])
	p.dstPort = binary.BigEndian.Uint16(raw[p.tcpOff+2 : p.tcpOff+4])
	p.seqNum = binary.BigEndian.Uint32(raw[p.tcpOff+4 : p.tcpOff+8])
	p.ackNum = binary.BigEndian.Uint32(raw[p.tcpOff+8 : p.tcpOff+12])
	p.flags = raw[p.tcpOff+13]

	// Payload runs from end-of-headers to min(totalLen, len(raw)).
	// Clamp to len(raw) because some kernels deliver packets where
	// totalLen < buffer size (trailing padding) or > (truncated capture).
	payloadStart := p.tcpOff + p.tcpHdrLen
	ipEnd := p.totalLen
	if ipEnd > len(raw) {
		ipEnd = len(raw)
	}
	if payloadStart < ipEnd {
		p.payload = raw[payloadStart:ipEnd]
	}
	return p, nil
}

// TCP flag bit constants (byte 13 of TCP header).
const (
	flagFIN = 0x01
	flagSYN = 0x02
	flagRST = 0x04
	flagPSH = 0x08
	flagACK = 0x10
)

func (p pkt) isSYNOnly() bool {
	return p.flags&flagSYN != 0 && p.flags&flagACK == 0 && p.flags&(flagRST|flagFIN) == 0
}
func (p pkt) isSYNACK() bool {
	return p.flags&flagSYN != 0 && p.flags&flagACK != 0 && p.flags&(flagRST|flagFIN) == 0
}
func (p pkt) isACKOnly() bool  { return p.flags&flagACK != 0 && p.flags&(flagSYN|flagRST|flagFIN) == 0 }
func (p pkt) hasPayload() bool { return len(p.payload) > 0 }

// injectSpec is the set of mutation knobs the engine passes to buildInjection.
// Separated from bypass.Mutation so the engine can add bookkeeping (final
// seq number resolution, TCP checksum corruption toggle) without leaking
// engine internals into the public bypass API.
type injectSpec struct {
	seqNum          uint32
	setPSH          bool
	ipIDDelta       uint16
	payload         []byte
	corruptChecksum bool
}

// buildInjection constructs a fresh IPv4+TCP packet that mirrors the observed
// ACK closing the handshake but replaces the payload, sequence number, IP
// ident, and TCP flags per injectSpec.
//
// Returns a fully assembled packet. The IP checksum is always valid; the
// TCP checksum is valid unless spec.corruptChecksum is set, in which case it
// is XOR'd with a fixed nonzero constant so the destination host drops the
// packet while a checksum-agnostic DPI still parses it.
func buildInjection(ack pkt, spec injectSpec) []byte {
	seqNum := spec.seqNum
	ipIDDelta := spec.ipIDDelta
	setPSH := spec.setPSH
	payload := spec.payload
	// Minimal IPv4 (20 bytes) + minimal TCP (20 bytes) + payload. We
	// deliberately drop TCP options from the injected packet — they are not
	// required for DPI to parse the SNI and keep the packet small.
	const ipHL, tcpHL = 20, 20
	total := ipHL + tcpHL + len(payload)
	out := make([]byte, total)

	// IPv4 header.
	out[0] = 0x45 // version 4, IHL 5
	out[1] = 0x00 // DSCP/ECN
	binary.BigEndian.PutUint16(out[2:4], uint16(total))
	binary.BigEndian.PutUint16(out[4:6], ack.ident+ipIDDelta)
	binary.BigEndian.PutUint16(out[6:8], 0x4000) // DF, no fragment
	out[8] = 64                                  // TTL
	out[9] = 6                                   // protocol TCP
	// checksum at 10..12 left zero for now
	copy(out[12:16], ack.srcIP[:])
	copy(out[16:20], ack.dstIP[:])
	ipCsum := checksum(out[0:ipHL])
	binary.BigEndian.PutUint16(out[10:12], ipCsum)

	// TCP header.
	tcp := out[ipHL:]
	binary.BigEndian.PutUint16(tcp[0:2], ack.srcPort)
	binary.BigEndian.PutUint16(tcp[2:4], ack.dstPort)
	binary.BigEndian.PutUint32(tcp[4:8], seqNum)
	binary.BigEndian.PutUint32(tcp[8:12], ack.ackNum)
	tcp[12] = (tcpHL / 4) << 4
	flags := uint8(flagACK)
	if setPSH {
		flags |= flagPSH
	}
	tcp[13] = flags
	binary.BigEndian.PutUint16(tcp[14:16], 0x0800) // window — matches common Linux default
	// checksum placeholder at 16:18
	// urgent pointer at 18:20 = 0
	copy(tcp[tcpHL:], payload)

	// TCP checksum: pseudo-header + TCP header + payload.
	var psh [12]byte
	copy(psh[0:4], ack.srcIP[:])
	copy(psh[4:8], ack.dstIP[:])
	psh[8] = 0
	psh[9] = 6
	binary.BigEndian.PutUint16(psh[10:12], uint16(tcpHL+len(payload)))

	tcpCsum := checksumMany(psh[:], tcp[:tcpHL+len(payload)])
	if spec.corruptChecksum {
		// Flip several bits so recomputation by an aggressive middlebox can't
		// accidentally match. 0xBEEF is distinctive in captures for debugging.
		tcpCsum ^= 0xBEEF
		if tcpCsum == 0 {
			tcpCsum = 0x1234 // 0 means "no checksum"; avoid it.
		}
	}
	binary.BigEndian.PutUint16(tcp[16:18], tcpCsum)

	return out
}

// checksum computes the standard 16-bit one's-complement Internet checksum
// of b. b may be any length; a trailing odd byte is padded with zero.
func checksum(b []byte) uint16 {
	return checksumMany(b)
}

// checksumMany checksums the concatenation of multiple slices without
// allocating a combined buffer.
func checksumMany(parts ...[]byte) uint16 {
	var sum uint32
	var carry byte
	haveCarry := false
	for _, p := range parts {
		i := 0
		if haveCarry {
			sum += uint32(carry)<<8 | uint32(p[0])
			i = 1
			haveCarry = false
		}
		for ; i+1 < len(p); i += 2 {
			sum += uint32(p[i])<<8 | uint32(p[i+1])
		}
		if i < len(p) {
			carry = p[i]
			haveCarry = true
		}
	}
	if haveCarry {
		sum += uint32(carry) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}
