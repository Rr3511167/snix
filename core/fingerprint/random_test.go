package fingerprint

import (
	"testing"
	"time"

	"github.com/SamNet-dev/snix/core/bypass"
)

func TestDelayRespectsBounds(t *testing.T) {
	r := New(Knobs{
		RandomizeTiming: true,
		MinDelay:        500 * time.Microsecond,
		MaxDelay:        5 * time.Millisecond,
	})
	for i := 0; i < 500; i++ {
		d := r.Delay()
		if d < 500*time.Microsecond || d > 5*time.Millisecond {
			t.Fatalf("delay %v out of [500us, 5ms]", d)
		}
	}
}

func TestDelayFixedWhenDisabled(t *testing.T) {
	r := New(Knobs{RandomizeTiming: false})
	if d := r.Delay(); d != 1*time.Millisecond {
		t.Fatalf("fixed delay: got %v", d)
	}
}

func TestExtraPadRange(t *testing.T) {
	r := New(Knobs{RandomizePadding: true, MinExtraPad: 10, MaxExtraPad: 100})
	seen := make(map[int]bool)
	for i := 0; i < 500; i++ {
		n := r.ExtraPad()
		if n < 10 || n > 100 {
			t.Fatalf("extra %d out of [10, 100]", n)
		}
		seen[n] = true
	}
	if len(seen) < 20 {
		t.Fatalf("only saw %d distinct values; randomization too weak", len(seen))
	}
}

func TestIPIDDeltaRange(t *testing.T) {
	r := New(Knobs{IPIDDeltaRange: 1})
	if d := r.IPIDDelta(); d != 1 {
		t.Fatalf("range=1: got %d", d)
	}
	r2 := New(Knobs{IPIDDeltaRange: 100})
	for i := 0; i < 200; i++ {
		d := r2.IPIDDelta()
		if d < 1 || d > 100 {
			t.Fatalf("delta %d out of [1, 100]", d)
		}
	}
}

func TestPickSNIVariants(t *testing.T) {
	pool := []string{"a.com", "b.com", "c.com"}
	rr := New(Knobs{SNISelection: "round_robin"})
	got := []string{rr.PickSNI(pool), rr.PickSNI(pool), rr.PickSNI(pool)}
	want := []string{"a.com", "b.com", "c.com"}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("rr[%d]: got %q want %q", i, got[i], want[i])
		}
	}
	rnd := New(Knobs{SNISelection: "random"})
	for i := 0; i < 50; i++ {
		s := rnd.PickSNI(pool)
		found := false
		for _, p := range pool {
			if p == s {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("random picked outside pool: %q", s)
		}
	}
	if rnd.PickSNI(nil) != "" {
		t.Error("empty pool should return empty string")
	}
}

func TestPickStrategy(t *testing.T) {
	r := New(Knobs{StrategyRotation: []bypass.Name{bypass.NameWrongSeq, bypass.NameWrongChecksum}})
	seen := map[bypass.Name]int{}
	for i := 0; i < 200; i++ {
		seen[r.PickStrategy(bypass.NameWrongSeq)]++
	}
	if seen[bypass.NameWrongSeq] == 0 || seen[bypass.NameWrongChecksum] == 0 {
		t.Fatalf("strategy rotation uneven: %v", seen)
	}

	// Empty rotation → fallback.
	r2 := New(Knobs{})
	if r2.PickStrategy(bypass.NameWrongChecksum) != bypass.NameWrongChecksum {
		t.Error("empty rotation must return fallback")
	}
}
