//go:build linux

// Package linux implements the snix Backend using Linux netfilter NFQUEUE
// plus managed iptables rules.
//
// Safety invariants:
//   - Installed rules are narrowly scoped to a single (remoteIP, remotePort)
//     pair and never touch the SSH control plane.
//   - All rules use `--queue-bypass` so a crashed or wedged snix never
//     blackholes user traffic.
//   - installRules and removeRules are idempotent. Close() is safe to call
//     multiple times.
//   - We record the exact rule specs we added and only delete those; we
//     never flush chains.
package linux

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"sync"
	"syscall"

	nfqueue "github.com/florianl/go-nfqueue"
	"golang.org/x/sys/unix"

	"github.com/SamNet-dev/snix/platform"
)

const (
	// DefaultQueueNum is the default NFQUEUE number we install rules for.
	// Picked high to avoid collisions with common systems.
	DefaultQueueNum = 42

	// defaultMaxPacketLen is generous enough for jumbo frames; real traffic
	// will almost always be much smaller.
	defaultMaxPacketLen = 0xffff
)

// Config tunes the Linux backend.
type Config struct {
	// QueueNum is the netfilter queue number. Default: DefaultQueueNum.
	QueueNum uint16
	// MaxPacketLen is the per-packet copy-range limit passed to NFQUEUE.
	MaxPacketLen uint32
}

// Backend implements platform.Backend on Linux.
type Backend struct {
	cfg Config

	mu      sync.Mutex
	opened  bool
	scope   platform.Scope
	nfq     *nfqueue.Nfqueue
	rawSock int
	packets chan platform.Packet
	closed  chan struct{}
	// cancelReader cancels the context passed to nfqueue.Register. nfqueue's
	// internal reader goroutine only exits when this context is Done, so
	// Close() must call it before invoking nfq.Close() or Close hangs.
	cancelReader context.CancelFunc
	addedRules   [][]string // exact args we passed to iptables -A, for safe cleanup
}

// New returns an unopened Backend.
func New(cfg Config) *Backend {
	if cfg.QueueNum == 0 {
		cfg.QueueNum = DefaultQueueNum
	}
	if cfg.MaxPacketLen == 0 {
		cfg.MaxPacketLen = defaultMaxPacketLen
	}
	return &Backend{
		cfg:     cfg,
		rawSock: -1,
		packets: make(chan platform.Packet, 256),
		closed:  make(chan struct{}),
	}
}

// Open installs iptables rules, opens an NFQUEUE reader, and opens a raw
// socket for Inject(). It is a configuration error to call Open twice.
func (b *Backend) Open(ctx context.Context, scope platform.Scope) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.opened {
		return errors.New("snix/linux: backend already open")
	}
	if !scope.RemoteIP.Is4() {
		// IPv6 support is straightforward (ip6tables + AF_INET6) but out of
		// Phase 1 scope.
		return fmt.Errorf("snix/linux: IPv6 scope not yet supported (%s)", scope.RemoteIP)
	}

	if err := b.installRules(scope); err != nil {
		return fmt.Errorf("install rules: %w", err)
	}

	// Raw socket for Inject.
	rs, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		_ = b.removeRulesLocked()
		return fmt.Errorf("raw socket: %w", err)
	}
	if err := syscall.SetsockoptInt(rs, syscall.IPPROTO_IP, syscall.IP_HDRINCL, 1); err != nil {
		_ = syscall.Close(rs)
		_ = b.removeRulesLocked()
		return fmt.Errorf("set IP_HDRINCL: %w", err)
	}
	b.rawSock = rs

	// NFQUEUE reader.
	nf, err := nfqueue.Open(&nfqueue.Config{
		NfQueue:      b.cfg.QueueNum,
		MaxPacketLen: b.cfg.MaxPacketLen,
		MaxQueueLen:  4096,
		Copymode:     nfqueue.NfQnlCopyPacket,
	})
	if err != nil {
		_ = syscall.Close(b.rawSock)
		b.rawSock = -1
		_ = b.removeRulesLocked()
		return fmt.Errorf("nfqueue open: %w", err)
	}
	b.nfq = nf
	b.scope = scope

	// nfqueue's reader goroutine only exits when the context passed to
	// Register is cancelled. We own our own cancellable context derived from
	// the caller's so Close() can shut it down deterministically.
	readerCtx, cancelReader := context.WithCancel(ctx)
	b.cancelReader = cancelReader

	// Register packet callback.
	err = nf.RegisterWithErrorFunc(readerCtx,
		func(attr nfqueue.Attribute) int {
			b.onPacket(attr)
			return 0
		},
		func(e error) int {
			// Treat EINTR as transient.
			if errors.Is(e, unix.EINTR) {
				return 0
			}
			return 1
		},
	)
	if err != nil {
		cancelReader()
		_ = nf.Close()
		_ = syscall.Close(b.rawSock)
		b.rawSock = -1
		_ = b.removeRulesLocked()
		return fmt.Errorf("nfqueue register: %w", err)
	}

	b.opened = true
	return nil
}

// onPacket is the NFQUEUE callback, invoked per received packet.
func (b *Backend) onPacket(attr nfqueue.Attribute) {
	if attr.PacketID == nil || attr.Payload == nil {
		return
	}
	id := *attr.PacketID
	payload := *attr.Payload
	// Copy payload so the caller can retain Raw safely past this callback.
	buf := make([]byte, len(payload))
	copy(buf, payload)

	// Direction is derived from whether src IP matches our scope.LocalIP.
	dir := platform.DirOutbound
	if len(buf) >= 20 && (buf[0]>>4) == 4 {
		var src [4]byte
		copy(src[:], buf[12:16])
		if net.IP(src[:]).Equal(b.scope.RemoteIP.AsSlice()) {
			dir = platform.DirInbound
		}
	}

	verdict := func(k platform.VerdictKind) {
		switch k {
		case platform.Accept, platform.AcceptModified:
			_ = b.nfq.SetVerdict(id, nfqueue.NfAccept)
		case platform.Drop:
			_ = b.nfq.SetVerdict(id, nfqueue.NfDrop)
		}
	}

	select {
	case b.packets <- platform.Packet{Dir: dir, Raw: buf, Verdict: verdict}:
	case <-b.closed:
		// Accept by default if we're shutting down so traffic keeps flowing.
		_ = b.nfq.SetVerdict(id, nfqueue.NfAccept)
	}
}

// Packets returns the observed-packet channel.
func (b *Backend) Packets() <-chan platform.Packet { return b.packets }

// Inject writes a fully formed IPv4 packet to the raw socket. The destination
// address is read from the IP header (bytes 16:20).
func (b *Backend) Inject(pkt []byte, _ platform.Direction) error {
	b.mu.Lock()
	rs := b.rawSock
	b.mu.Unlock()
	if rs < 0 {
		return errors.New("snix/linux: raw socket closed")
	}
	if len(pkt) < 20 {
		return errors.New("snix/linux: inject: packet too short")
	}
	var dst [4]byte
	copy(dst[:], pkt[16:20])
	sa := &syscall.SockaddrInet4{Addr: dst}
	return syscall.Sendto(rs, pkt, 0, sa)
}

// Close tears down everything Open set up. Safe to call multiple times.
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.opened {
		return nil
	}
	b.opened = false
	select {
	case <-b.closed:
	default:
		close(b.closed)
	}
	var firstErr error
	if b.cancelReader != nil {
		b.cancelReader()
		b.cancelReader = nil
	}
	if b.nfq != nil {
		if err := b.nfq.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		b.nfq = nil
	}
	if b.rawSock >= 0 {
		_ = syscall.Close(b.rawSock)
		b.rawSock = -1
	}
	if err := b.removeRulesLocked(); err != nil && firstErr == nil {
		firstErr = err
	}
	// Drain packet channel so receivers don't block; close signals EOF.
	close(b.packets)
	return firstErr
}

// installRules adds narrowly-scoped iptables rules redirecting only the
// target flow into our NFQUEUE.
func (b *Backend) installRules(scope platform.Scope) error {
	port := fmt.Sprintf("%d", scope.RemotePort)
	qn := fmt.Sprintf("%d", b.cfg.QueueNum)
	ruleSpecs := [][]string{
		// Outbound SYN/ACK/data to the remote server.
		{"-t", "mangle", "-A", "OUTPUT", "-p", "tcp",
			"-d", scope.RemoteIP.String(), "--dport", port,
			"-j", "NFQUEUE", "--queue-num", qn, "--queue-bypass"},
		// Inbound SYN-ACK/ACK from the remote server.
		{"-t", "mangle", "-A", "INPUT", "-p", "tcp",
			"-s", scope.RemoteIP.String(), "--sport", port,
			"-j", "NFQUEUE", "--queue-num", qn, "--queue-bypass"},
	}
	for _, spec := range ruleSpecs {
		if err := runIptables(spec...); err != nil {
			// Best-effort rollback of any partially installed rules.
			_ = b.removeRulesLocked()
			return fmt.Errorf("iptables %v: %w", spec, err)
		}
		b.addedRules = append(b.addedRules, spec)
	}
	return nil
}

// removeRulesLocked deletes every rule we successfully added, converting
// each `-A` into `-D`. Missing rules (already removed) are ignored.
func (b *Backend) removeRulesLocked() error {
	var firstErr error
	// Remove in reverse order.
	for i := len(b.addedRules) - 1; i >= 0; i-- {
		spec := append([]string{}, b.addedRules[i]...)
		for j, s := range spec {
			if s == "-A" {
				spec[j] = "-D"
				break
			}
		}
		if err := runIptables(spec...); err != nil && firstErr == nil {
			// Don't fail hard: rule might already be gone.
			firstErr = fmt.Errorf("iptables delete %v: %w", spec, err)
		}
	}
	b.addedRules = nil
	return firstErr
}

var iptablesPath = func() string {
	if p, err := exec.LookPath("iptables"); err == nil {
		return p
	}
	return "/usr/sbin/iptables"
}()

func runIptables(args ...string) error {
	cmd := exec.Command(iptablesPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w (%s)", cmd.String(), err, string(out))
	}
	return nil
}
