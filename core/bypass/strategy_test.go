package bypass

import (
	"errors"
	"testing"
)

func TestWrongSeqMatchesUpstream(t *testing.T) {
	// Upstream Python: seq = (syn_seq + 1 - len(payload)) & 0xffffffff
	cases := []struct {
		synSeq  uint32
		payload []byte
		wantSeq uint32
	}{
		{synSeq: 0x10000000, payload: make([]byte, 517), wantSeq: 0x10000000 + 1 - 517},
		{synSeq: 0, payload: []byte{1, 2, 3}, wantSeq: 0xFFFFFFFF - 1},       // wrap
		{synSeq: 100, payload: []byte{1}, wantSeq: 100},                      // 100+1-1
		{synSeq: 0xFFFFFFFF, payload: make([]byte, 10), wantSeq: 0xFFFFFFF6}, // edge
	}
	s := WrongSeq{}
	for _, c := range cases {
		m, err := s.Plan(Inputs{SynSeq: c.synSeq, FakePayload: c.payload})
		if err != nil {
			t.Fatalf("Plan error: %v", err)
		}
		if m.SeqNum != c.wantSeq {
			t.Fatalf("seq: got %#x want %#x (synSeq=%#x, plen=%d)",
				m.SeqNum, c.wantSeq, c.synSeq, len(c.payload))
		}
		if !m.SetPSH {
			t.Error("SetPSH must be true")
		}
		if m.IPIDDelta != 1 {
			t.Errorf("IPIDDelta: got %d want 1", m.IPIDDelta)
		}
	}
}

func TestWrongSeqRejectsEmptyPayload(t *testing.T) {
	if _, err := (WrongSeq{}).Plan(Inputs{SynSeq: 1}); !errors.Is(err, ErrInvalidInputs) {
		t.Fatalf("expected ErrInvalidInputs, got %v", err)
	}
}

func TestByName(t *testing.T) {
	if _, err := ByName(NameWrongSeq); err != nil {
		t.Fatalf("wrong_seq lookup: %v", err)
	}
	if _, err := ByName(NameWrongChecksum); err != nil {
		t.Fatalf("wrong_checksum lookup: %v", err)
	}
	if _, err := ByName("nope"); !errors.Is(err, ErrInvalidInputs) {
		t.Fatal("unknown name should error")
	}
}

func TestWrongChecksumPlan(t *testing.T) {
	payload := []byte("hello")
	m, err := (WrongChecksum{}).Plan(Inputs{SynSeq: 100, FakePayload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if !m.CorruptChecksum {
		t.Error("CorruptChecksum must be true")
	}
	if !m.OverrideSeq {
		t.Error("OverrideSeq must be true (belt-and-suspenders with wrong_seq)")
	}
	// SeqNum must match wrong_seq formula.
	if got, want := m.SeqNum, uint32(100+1-len(payload)); got != want {
		t.Errorf("SeqNum: got %#x want %#x", got, want)
	}
	if !m.SetPSH {
		t.Error("SetPSH must be true")
	}
}
