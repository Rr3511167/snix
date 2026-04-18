package engine

import (
	"encoding/binary"
	"testing"
)

// handcrafted IPv4+TCP SYN packet for testing the parser. Fields chosen to
// be distinct so any byte-order mistake surfaces immediately.
func synPacket(srcIP, dstIP [4]byte, srcPort, dstPort uint16, seq uint32) []byte {
	ipHL, tcpHL := 20, 20
	total := ipHL + tcpHL
	b := make([]byte, total)
	b[0] = 0x45
	binary.BigEndian.PutUint16(b[2:4], uint16(total))
	binary.BigEndian.PutUint16(b[4:6], 0x1234) // ident
	b[8] = 64
	b[9] = 6
	copy(b[12:16], srcIP[:])
	copy(b[16:20], dstIP[:])
	// Valid IP checksum (important: parser does not verify but builder does).
	binary.BigEndian.PutUint16(b[10:12], checksum(b[:ipHL]))

	tcp := b[ipHL:]
	binary.BigEndian.PutUint16(tcp[0:2], srcPort)
	binary.BigEndian.PutUint16(tcp[2:4], dstPort)
	binary.BigEndian.PutUint32(tcp[4:8], seq)
	binary.BigEndian.PutUint32(tcp[8:12], 0)
	tcp[12] = 5 << 4
	tcp[13] = flagSYN
	binary.BigEndian.PutUint16(tcp[14:16], 0x0800)
	return b
}

func TestParseSYN(t *testing.T) {
	src := [4]byte{10, 0, 0, 1}
	dst := [4]byte{1, 1, 1, 1}
	raw := synPacket(src, dst, 40000, 443, 0xDEADBEEF)
	p, err := parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.srcIP != src || p.dstIP != dst {
		t.Errorf("IPs: got %v/%v", p.srcIP, p.dstIP)
	}
	if p.srcPort != 40000 || p.dstPort != 443 {
		t.Errorf("ports: got %d/%d", p.srcPort, p.dstPort)
	}
	if p.seqNum != 0xDEADBEEF {
		t.Errorf("seq: got %#x", p.seqNum)
	}
	if !p.isSYNOnly() {
		t.Error("isSYNOnly false")
	}
	if p.hasPayload() {
		t.Error("SYN should have no payload")
	}
}

func TestParseRejectsIPv6AndTruncated(t *testing.T) {
	if _, err := parse([]byte{0x60, 0, 0, 0}); err == nil {
		t.Error("IPv6 prefix should fail")
	}
	if _, err := parse([]byte{0x45, 0}); err == nil {
		t.Error("truncated should fail")
	}
	// UDP (protocol 17) should be rejected.
	udp := synPacket([4]byte{1, 2, 3, 4}, [4]byte{5, 6, 7, 8}, 1, 2, 0)
	udp[9] = 17
	if _, err := parse(udp); err == nil {
		t.Error("UDP should be rejected")
	}
}

// TestChecksumKnownVector verifies against a hand-computed value.
// Sum of 16-bit words of [0x01 0x02 0x03 0x04] = 0x0102 + 0x0304 = 0x0406.
// One's complement: ~0x0406 = 0xFBF9.
func TestChecksumKnownVector(t *testing.T) {
	got := checksum([]byte{0x01, 0x02, 0x03, 0x04})
	want := uint16(0xFBF9)
	if got != want {
		t.Fatalf("checksum: got %#x want %#x", got, want)
	}
}

// TestBuildInjectionChecksums verifies both IP and TCP checksums are valid
// and the TCP mutation fields land in the right bytes.
func TestBuildInjectionChecksums(t *testing.T) {
	src := [4]byte{10, 0, 0, 1}
	dst := [4]byte{1, 1, 1, 1}
	ackRaw := synPacket(src, dst, 40000, 443, 0xABC)
	// Promote it to an ACK for the test.
	ackRaw[ackRaw[0]&0x0f*4+13] = flagACK
	ack, err := parse(ackRaw)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("HELLO-WORLD-FAKE-CLIENT-HELLO")
	inj := buildInjection(ack, injectSpec{
		seqNum: 0x12345678, ipIDDelta: 1, setPSH: true, payload: payload,
	})

	// IP checksum must verify to zero.
	if checksum(inj[:20]) != 0 {
		t.Fatalf("IP checksum invalid: %#x", checksum(inj[:20]))
	}

	// TCP checksum must verify over pseudo-header + TCP + payload.
	var psh [12]byte
	copy(psh[0:4], src[:])
	copy(psh[4:8], dst[:])
	psh[9] = 6
	binary.BigEndian.PutUint16(psh[10:12], uint16(20+len(payload)))
	if checksumMany(psh[:], inj[20:]) != 0 {
		t.Fatalf("TCP checksum invalid")
	}

	// Verify mutated fields.
	injp, err := parse(inj)
	if err != nil {
		t.Fatal(err)
	}
	if injp.seqNum != 0x12345678 {
		t.Errorf("seq: %#x", injp.seqNum)
	}
	if injp.flags&flagPSH == 0 || injp.flags&flagACK == 0 {
		t.Errorf("flags: %#x", injp.flags)
	}
	if injp.ident != 0x1234+1 {
		t.Errorf("ident: %#x", injp.ident)
	}
	if string(injp.payload) != string(payload) {
		t.Errorf("payload mismatch")
	}
}
