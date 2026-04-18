package conntrack

import (
	"net/netip"
	"testing"
)

func tup(srcPort, dstPort uint16) Tuple {
	return Tuple{
		SrcIP:   netip.MustParseAddr("10.0.0.1"),
		SrcPort: srcPort,
		DstIP:   netip.MustParseAddr("188.114.98.0"),
		DstPort: dstPort,
	}
}

func TestTableAddGetRemove(t *testing.T) {
	tab := NewTable()
	c := New(tup(40000, 443), []byte("fake"))
	if !tab.Add(c) {
		t.Fatal("Add returned false on empty table")
	}
	if tab.Add(c) {
		t.Fatal("duplicate Add should return false")
	}
	if tab.Len() != 1 {
		t.Fatalf("Len: got %d want 1", tab.Len())
	}
	got, ok := tab.Get(c.Tuple)
	if !ok || got != c {
		t.Fatalf("Get: ok=%v same=%v", ok, got == c)
	}
	tab.Remove(c.Tuple)
	if tab.Len() != 0 {
		t.Fatalf("Len after Remove: got %d want 0", tab.Len())
	}
	tab.Remove(c.Tuple) // idempotent
}

func TestFinishIsIdempotentAndClosesDone(t *testing.T) {
	c := New(tup(1, 443), nil)
	c.Mu.Lock()
	c.Finish(true, "ok")
	c.Mu.Unlock()

	select {
	case <-c.Done:
	default:
		t.Fatal("Done not closed after Finish")
	}
	if c.Stage != StageFakeAcked {
		t.Fatalf("stage: got %d want FakeAcked", c.Stage)
	}
	if c.Monitor {
		t.Fatal("Monitor should be false after Finish")
	}
	// Second call must not panic (close of closed channel).
	c.Mu.Lock()
	c.Finish(false, "second call")
	c.Mu.Unlock()
	if c.Result.Reason != "ok" {
		t.Fatalf("result clobbered: %+v", c.Result)
	}
}
