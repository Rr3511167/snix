// Package fingerprint centralizes the anti-DPI-fingerprinting knobs:
// timing jitter, size padding, IP ID deltas, SNI selection, and
// per-connection strategy rotation.
//
// Each individual knob is small; the cumulative effect is that an on-path
// DPI cannot pattern-match snix traffic by any single invariant (fixed
// 517-byte packet, fixed 1 ms delay, fixed +1 IP ID, single SNI).
package fingerprint

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
	"time"

	"github.com/SamNet-dev/snix/core/bypass"
)

// Knobs configures the randomizer. Zero values mean "use upstream defaults"
// (matches fixed-shape upstream behaviour).
type Knobs struct {
	// Timing: if RandomizeTiming, each injection waits a uniform delay in
	// [MinDelay, MaxDelay]. Otherwise the fixed InjectDelay is used.
	RandomizeTiming bool
	MinDelay        time.Duration
	MaxDelay        time.Duration

	// Padding: if RandomizePadding, an extra [MinExtraPad, MaxExtraPad]
	// bytes are appended to the fake ClientHello so its size is not
	// invariantly 517. Upstream used fixed 517.
	RandomizePadding bool
	MinExtraPad      int
	MaxExtraPad      int

	// IPIDDeltaRange sets the maximum IP ID delta applied to the fake
	// packet. If zero, delta is fixed at 1 (upstream behaviour).
	// Actual delta is uniform in [1, IPIDDeltaRange].
	IPIDDeltaRange int

	// StrategyRotation, if non-empty, overrides a single fixed Strategy
	// with a per-flow random choice. Each flow independently picks one.
	// Typical value: ["wrong_seq", "wrong_checksum"].
	StrategyRotation []bypass.Name

	// SNISelection selects one SNI per new flow. "round_robin" preserves
	// upstream-style deterministic rotation; "random" picks uniformly.
	// Default: "random".
	SNISelection string
}

// Randomizer owns the cryptographic RNG and exposes per-flow picks.
type Randomizer struct {
	k     Knobs
	mu    sync.Mutex
	rrIdx uint64 // round-robin counter for SNI
	buf   [8]byte
}

// New returns a Randomizer with the given knobs; normalizes defaults.
func New(k Knobs) *Randomizer {
	if !k.RandomizeTiming {
		// Fallback is 1ms like upstream; caller may override via engine cfg.
		k.MinDelay = 1 * time.Millisecond
		k.MaxDelay = 1 * time.Millisecond
	} else {
		if k.MinDelay <= 0 {
			k.MinDelay = 500 * time.Microsecond
		}
		if k.MaxDelay <= k.MinDelay {
			k.MaxDelay = 5 * time.Millisecond
		}
	}
	if k.RandomizePadding {
		if k.MinExtraPad < 0 {
			k.MinExtraPad = 0
		}
		if k.MaxExtraPad < k.MinExtraPad {
			k.MaxExtraPad = k.MinExtraPad + 512
		}
	}
	if k.IPIDDeltaRange < 1 {
		k.IPIDDeltaRange = 1
	}
	if k.SNISelection == "" {
		k.SNISelection = "random"
	}
	return &Randomizer{k: k}
}

// Delay returns the per-injection delay to wait before emitting the fake packet.
func (r *Randomizer) Delay() time.Duration {
	if r.k.MinDelay == r.k.MaxDelay {
		return r.k.MinDelay
	}
	span := int64(r.k.MaxDelay - r.k.MinDelay)
	return r.k.MinDelay + time.Duration(r.uint64()%uint64(span))
}

// ExtraPad returns the number of bytes to append to the fake ClientHello.
func (r *Randomizer) ExtraPad() int {
	if !r.k.RandomizePadding {
		return 0
	}
	span := r.k.MaxExtraPad - r.k.MinExtraPad
	if span <= 0 {
		return r.k.MinExtraPad
	}
	return r.k.MinExtraPad + int(r.uint64()%uint64(span+1))
}

// IPIDDelta returns a uint16 delta in [1, IPIDDeltaRange].
func (r *Randomizer) IPIDDelta() uint16 {
	if r.k.IPIDDeltaRange <= 1 {
		return 1
	}
	return 1 + uint16(r.uint64()%uint64(r.k.IPIDDeltaRange))
}

// PickSNI returns one SNI from pool according to SNISelection.
// Returns "" if pool is empty.
func (r *Randomizer) PickSNI(pool []string) string {
	if len(pool) == 0 {
		return ""
	}
	switch r.k.SNISelection {
	case "round_robin":
		r.mu.Lock()
		i := r.rrIdx % uint64(len(pool))
		r.rrIdx++
		r.mu.Unlock()
		return pool[i]
	default: // "random"
		return pool[int(r.uint64()%uint64(len(pool)))]
	}
}

// PickStrategy returns a strategy name from StrategyRotation or the fallback.
func (r *Randomizer) PickStrategy(fallback bypass.Name) bypass.Name {
	if len(r.k.StrategyRotation) == 0 {
		return fallback
	}
	return r.k.StrategyRotation[int(r.uint64()%uint64(len(r.k.StrategyRotation)))]
}

// uint64 reads 8 bytes from crypto/rand. Panics on read failure — rand.Read
// on Linux never fails outside of catastrophic entropy exhaustion.
func (r *Randomizer) uint64() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := rand.Read(r.buf[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return binary.LittleEndian.Uint64(r.buf[:])
}
