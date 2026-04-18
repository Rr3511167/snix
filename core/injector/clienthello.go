// Package injector constructs fake TLS ClientHello packets for SNI-spoofing
// DPI bypass. This is a Go port of the upstream Python ClientHelloMaker
// (patterniha/SNI-Spoofing), producing byte-identical output so golden tests
// can verify parity.
package injector

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
)

const (
	// RandLen is the size of the TLS client_random field.
	RandLen = 32
	// SessionIDLen is the fixed session_id length the template uses.
	SessionIDLen = 32
	// KeyShareLen is the X25519 key_share size.
	KeyShareLen = 32
	// OutputLen is the total fake ClientHello size. Constant across all SNIs
	// because the padding extension absorbs the difference.
	OutputLen = 517
	// paddingBudget is the total byte budget shared between the SNI and the
	// padding extension. Matches the upstream `219 - len(sni)` formula.
	paddingBudget = 219
)

// tlsCHTemplateHex is the reference 517-byte ClientHello with SNI="mci.ir".
// Taken verbatim from upstream utils/packet_templates.py to keep static slice
// offsets identical.
const tlsCHTemplateHex = "1603010200010001fc030341d5b549d9cd1adfa7296c8418d157dc7b624c842824ff493b9375bb48d34f2b20bf018bcc90a7c89a230094815ad0c15b736e38c01209d72d282cb5e2105328150024130213031301c02cc030c02bc02fcca9cca8c024c028c023c027009f009e006b006700ff0100018f0000000b00090000066d63692e6972000b000403000102000a00160014001d0017001e0019001801000101010201030104002300000010000e000c02683208687474702f312e310016000000170000000d002a0028040305030603080708080809080a080b080408050806040105010601030303010302040205020602002b00050403040303002d00020101003300260024001d0020435bacc4d05f9d41fef44ab3ad55616c36e0613473e2338770efdaa98693d217001500d5000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

var (
	tlsCHTemplate = mustHex(tlsCHTemplateHex)
	// templateSNI matches upstream's hardcoded "mci.ir" used to offset slices.
	templateSNI = []byte("mci.ir")

	// static1: fixed prefix (record header + handshake type + legacy version).
	static1 = tlsCHTemplate[:11]
	// static2: single byte 0x20 = session_id length (32).
	static2 = []byte{0x20}
	// static3: cipher suites + legacy_compression + extensions list opener.
	static3 = tlsCHTemplate[76:120]
	// static4: post-SNI extensions up to (but not including) the key_share bytes.
	static4 = tlsCHTemplate[127+len(templateSNI) : 262+len(templateSNI)]
	// static5: padding extension type (0x0015).
	static5 = []byte{0x00, 0x15}
)

// ErrInvalidInput is returned when random/session/key_share sizes are wrong,
// or SNI exceeds the padding budget.
var ErrInvalidInput = errors.New("snix/injector: invalid input size")

// BuildClientHello constructs a 517-byte fake TLS 1.3 ClientHello with the
// given SNI and randomized handshake fields. Output is byte-identical to the
// upstream Python implementation for equal inputs.
//
// rnd, sessID, keyShare MUST each be 32 bytes; sni MUST be <= 219 bytes.
// The result is newly allocated and safe to modify.
func BuildClientHello(rnd, sessID []byte, sni, keyShare []byte) ([]byte, error) {
	if len(rnd) != RandLen || len(sessID) != SessionIDLen || len(keyShare) != KeyShareLen {
		return nil, ErrInvalidInput
	}
	if len(sni) > paddingBudget {
		return nil, ErrInvalidInput
	}

	sniLen := len(sni)
	padN := paddingBudget - sniLen

	out := make([]byte, 0, OutputLen)
	out = append(out, static1...)
	out = append(out, rnd...)
	out = append(out, static2...)
	out = append(out, sessID...)
	out = append(out, static3...)

	// server_name extension body:
	//   uint16 ext_total_len = sniLen + 5
	//   uint16 list_len      = sniLen + 3
	//   uint8  name_type     = 0x00 (host_name)
	//   uint16 name_len      = sniLen
	//   bytes  name
	var snHdr [7]byte
	binary.BigEndian.PutUint16(snHdr[0:2], uint16(sniLen+5))
	binary.BigEndian.PutUint16(snHdr[2:4], uint16(sniLen+3))
	snHdr[4] = 0x00
	binary.BigEndian.PutUint16(snHdr[5:7], uint16(sniLen))
	out = append(out, snHdr[:]...)
	out = append(out, sni...)

	out = append(out, static4...)
	out = append(out, keyShare...)
	out = append(out, static5...)

	// padding extension body: uint16 pad_len followed by pad_len zero bytes.
	var padHdr [2]byte
	binary.BigEndian.PutUint16(padHdr[:], uint16(padN))
	out = append(out, padHdr[:]...)
	out = append(out, make([]byte, padN)...)

	return out, nil
}

// BuildClientHelloPadded returns a ClientHello whose total size is
// OutputLen + extraPad bytes, where extraPad ∈ [0, 65000]. The extension is
// byte-identical to BuildClientHello when extraPad == 0.
//
// Extra bytes are appended to the padding extension. The record and
// handshake length fields are recomputed so the resulting packet remains a
// valid TLS record; any TLS parser (including the DPI) will see a well-
// formed ClientHello carrying the same SNI, key_share, and cipher suites
// as the fixed-size variant.
//
// Breaking the fixed-517-byte signature is a key anti-fingerprinting win:
// a DPI that looked for "exact length 517 during handshake-ACK window"
// cannot match a variable-length stream.
func BuildClientHelloPadded(rnd, sessID, sni, keyShare []byte, extraPad int) ([]byte, error) {
	if extraPad < 0 || extraPad > 65000 {
		return nil, ErrInvalidInput
	}
	base, err := BuildClientHello(rnd, sessID, sni, keyShare)
	if err != nil {
		return nil, err
	}
	if extraPad == 0 {
		return base, nil
	}

	out := make([]byte, 0, len(base)+extraPad)
	out = append(out, base...)
	out = append(out, make([]byte, extraPad)...)

	// Fix up the three length fields that covered the inner structures.
	// Record header bytes 3-4: TLS record body length (handshake data only,
	// excludes the 5-byte record header itself).
	recBodyLen := uint16(len(out) - 5)
	out[3] = byte(recBodyLen >> 8)
	out[4] = byte(recBodyLen)
	// Handshake header bytes 6-8: uint24 handshake body length (excludes the
	// 4-byte handshake header itself).
	hsBodyLen := len(out) - 9
	out[6] = byte(hsBodyLen >> 16)
	out[7] = byte(hsBodyLen >> 8)
	out[8] = byte(hsBodyLen)
	// Padding extension is the last extension. Its 2-byte length field sits
	// immediately after static5 (bytes [len(base)-2-padN : len(base)-padN]).
	// Simpler: extension length = (old padding size) + extraPad. Old padding
	// size is at bytes [len(base)-2-oldPad : len(base)-oldPad]. oldPad =
	// (paddingBudget - len(sni)).
	oldPad := paddingBudget - len(sni)
	padLenOff := len(base) - 2 - oldPad
	newPadBody := oldPad + extraPad
	out[padLenOff] = byte(newPadBody >> 8)
	out[padLenOff+1] = byte(newPadBody)

	return out, nil
}

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
