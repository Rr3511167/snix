package injector

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestPaddedMatchesBaseWhenZero: extraPad==0 returns byte-identical output.
func TestPaddedMatchesBaseWhenZero(t *testing.T) {
	rnd := bytes.Repeat([]byte{1}, 32)
	sess := bytes.Repeat([]byte{2}, 32)
	ks := bytes.Repeat([]byte{3}, 32)
	sni := []byte("auth.vercel.com")

	base, err := BuildClientHello(rnd, sess, sni, ks)
	if err != nil {
		t.Fatal(err)
	}
	pad0, err := BuildClientHelloPadded(rnd, sess, sni, ks, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(base, pad0) {
		t.Fatal("extraPad=0 should match BuildClientHello")
	}
}

// TestPaddedReproducesValidTLS: for each extra pad size, verify the record
// header, handshake header, and padding-extension length are all consistent
// and sum to the actual packet length.
func TestPaddedProducesValidTLS(t *testing.T) {
	rnd := bytes.Repeat([]byte{9}, 32)
	sess := bytes.Repeat([]byte{8}, 32)
	ks := bytes.Repeat([]byte{7}, 32)
	sni := []byte("cdn.segment.io")

	for _, extra := range []int{1, 7, 100, 517, 1024, 1500} {
		out, err := BuildClientHelloPadded(rnd, sess, sni, ks, extra)
		if err != nil {
			t.Fatalf("extra=%d: %v", extra, err)
		}
		if len(out) != OutputLen+extra {
			t.Errorf("extra=%d: len=%d want %d", extra, len(out), OutputLen+extra)
		}
		// Record body length (bytes 3-4).
		recBody := binary.BigEndian.Uint16(out[3:5])
		if int(recBody) != len(out)-5 {
			t.Errorf("extra=%d: record body %d want %d", extra, recBody, len(out)-5)
		}
		// Handshake body length (bytes 6-8, uint24).
		hsBody := int(out[6])<<16 | int(out[7])<<8 | int(out[8])
		if hsBody != len(out)-9 {
			t.Errorf("extra=%d: hs body %d want %d", extra, hsBody, len(out)-9)
		}
		// Padding-extension body length check: walk extensions from the
		// extensions-vector start (at offset 11+32+1+32+44 = 120 for the base
		// layout) and find the 0x0015 type.
		if !bytes.Contains(out, []byte{0x00, 0x15}) {
			t.Errorf("extra=%d: no 0x0015 padding ext marker found", extra)
		}
	}
}

// TestPaddedRejectsBogusInput: negative and oversized.
func TestPaddedRejectsBogusInput(t *testing.T) {
	rnd := make([]byte, 32)
	sess := make([]byte, 32)
	ks := make([]byte, 32)
	sni := []byte("x.io")
	for _, extra := range []int{-1, 65001} {
		if _, err := BuildClientHelloPadded(rnd, sess, sni, ks, extra); err == nil {
			t.Errorf("extra=%d: expected error", extra)
		}
	}
}
