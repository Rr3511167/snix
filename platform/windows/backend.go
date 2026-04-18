//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"sync"

	snixplatform "github.com/SamNet-dev/snix/platform"
)

// Config tunes the Windows backend.
type Config struct {
	// Priority is the WinDivert filter priority. Lower wins; leave 0 for default.
	Priority int16
	// ReadBufferSize is the per-Recv buffer length. 65535 covers every frame.
	ReadBufferSize int
	// QueueLength is the kernel-side packet queue cap (WINDIVERT_PARAM_QUEUE_LENGTH).
	// 0 means keep WinDivert's default (4096).
	QueueLength uint64
}

// Backend implements snix/platform.Backend via WinDivert on Windows.
type Backend struct {
	cfg Config

	mu      sync.Mutex
	opened  bool
	scope   snixplatform.Scope
	handle  handleRaw
	packets chan snixplatform.Packet
	closed  chan struct{}
	reader  sync.WaitGroup
	// recvAddr is stored per-packet so Inject() can re-use it if needed.
	// In our design the engine builds packets from scratch, so we only need
	// to stash a representative outbound address for reinjection.
	outboundAddr *Address
}

// New constructs an unopened Backend.
func New(cfg Config) *Backend {
	if cfg.ReadBufferSize <= 0 {
		cfg.ReadBufferSize = 65535
	}
	return &Backend{
		cfg:     cfg,
		handle:  invalidHandle,
		packets: make(chan snixplatform.Packet, 256),
		closed:  make(chan struct{}),
	}
}

// Open installs a WinDivert filter matching scope and starts the recv loop.
// Requires Administrator and the WinDivert driver installed.
func (b *Backend) Open(ctx context.Context, scope snixplatform.Scope) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.opened {
		return errors.New("snix/windows: backend already open")
	}
	if !scope.RemoteIP.Is4() {
		return fmt.Errorf("snix/windows: IPv6 scope not yet supported (%s)", scope.RemoteIP)
	}

	// Fail fast with a clear message when not elevated. WinDivertOpen will
	// also reject us, but this check runs before we even touch the DLL so
	// diagnostics are crisp (no "WinDivertOpen: ERROR_ACCESS_DENIED" noise).
	if elevated, err := IsElevated(); err != nil {
		return fmt.Errorf("snix/windows: privilege check failed: %w", err)
	} else if !elevated {
		return fmt.Errorf("%w; relaunch from an elevated shell (Run as Administrator)", ErrNotElevated)
	}

	filter := fmt.Sprintf(
		"tcp and ip and ((ip.DstAddr == %s and tcp.DstPort == %d) or (ip.SrcAddr == %s and tcp.SrcPort == %d))",
		scope.RemoteIP, scope.RemotePort, scope.RemoteIP, scope.RemotePort)

	h, err := Open(filter, LayerNetwork, b.cfg.Priority, FlagDefault)
	if err != nil {
		return err
	}
	if b.cfg.QueueLength > 0 {
		if e := SetParam(h, ParamQueueLength, b.cfg.QueueLength); e != nil {
			_ = Close(h)
			return fmt.Errorf("SetParam QueueLength: %w", e)
		}
	}

	b.handle = h
	b.scope = scope
	b.opened = true

	b.reader.Add(1)
	go b.recvLoop(ctx)
	return nil
}

// recvLoop reads WinDivert packets and forwards them on b.packets.
// The caller's Verdict decides whether the packet is reinjected (Accept),
// dropped (Drop), or reinjected-modified (AcceptModified — caller must have
// handed us the mutated bytes elsewhere; today the engine uses Inject()
// instead, so we just treat all non-Drop verdicts as Accept).
func (b *Backend) recvLoop(ctx context.Context) {
	defer b.reader.Done()
	defer close(b.packets)

	buf := make([]byte, b.cfg.ReadBufferSize)
	for {
		n, addr, err := Recv(b.handle, buf)
		if err != nil {
			// ERROR_NO_DATA (232) is the graceful signal that Close() tore
			// the handle down mid-Recv; exit silently. Anything else gets
			// printed once so users notice unexpected errors.
			if IsHandleClosed(err) {
				return
			}
			select {
			case <-b.closed:
				return
			default:
			}
			fmt.Printf("snix/windows: recv error: %v\n", err)
			return
		}
		// Copy payload so the caller can keep it past next Recv.
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		// Copy address too (80 bytes) so the verdict closure can reinject.
		addrCopy := addr
		dir := snixplatform.DirInbound
		if addr.Outbound() {
			dir = snixplatform.DirOutbound
			// Stash last outbound address for Inject() to reuse.
			b.mu.Lock()
			a := addr
			b.outboundAddr = &a
			b.mu.Unlock()
		}

		verdict := func(k snixplatform.VerdictKind) {
			switch k {
			case snixplatform.Drop:
				return // don't reinject; packet is dropped by not calling Send.
			default:
				// Reinject the original packet via WinDivertSend.
				_, _ = Send(b.handle, pkt, &addrCopy)
			}
		}

		select {
		case b.packets <- snixplatform.Packet{Dir: dir, Raw: pkt, Verdict: verdict}:
		case <-b.closed:
			// Shutting down: reinject so the stack keeps flowing.
			_, _ = Send(b.handle, pkt, &addrCopy)
			return
		case <-ctx.Done():
			_, _ = Send(b.handle, pkt, &addrCopy)
			return
		}
	}
}

// Packets is the observed-packet stream.
func (b *Backend) Packets() <-chan snixplatform.Packet { return b.packets }

// Inject emits a raw IPv4 packet back onto the stack. The direction is
// encoded on the address; if the engine hasn't seen any outbound packet yet
// (very early in a flow), we build a plausible outbound address from zero.
func (b *Backend) Inject(pkt []byte, dir snixplatform.Direction) error {
	if len(pkt) < 20 {
		return fmt.Errorf("snix/windows: inject packet too short")
	}
	b.mu.Lock()
	base := b.outboundAddr
	h := b.handle
	b.mu.Unlock()
	var addr Address
	if base != nil {
		addr = *base
	}
	addr.SetOutbound(dir == snixplatform.DirOutbound)

	_, err := Send(h, pkt, &addr)
	return err
}

// Close tears down the WinDivert handle and stops the recv loop.
func (b *Backend) Close() error {
	b.mu.Lock()
	if !b.opened {
		b.mu.Unlock()
		return nil
	}
	b.opened = false
	select {
	case <-b.closed:
	default:
		close(b.closed)
	}
	h := b.handle
	b.handle = invalidHandle
	b.mu.Unlock()

	var firstErr error
	if h != invalidHandle {
		if err := Close(h); err != nil {
			firstErr = err
		}
	}
	b.reader.Wait()
	return firstErr
}
