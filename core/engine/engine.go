package engine

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/SamNet-dev/snix/core/bypass"
	"github.com/SamNet-dev/snix/core/conntrack"
	"github.com/SamNet-dev/snix/core/fingerprint"
	"github.com/SamNet-dev/snix/core/injector"
	"github.com/SamNet-dev/snix/platform"
)

// Logger is the minimum logging surface the engine needs. Implementations
// can plug into zap/zerolog/slog — the default uses Printf-style lines.
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
}

// Config tunes engine behaviour.
type Config struct {
	// Strategy names the bypass algorithm used when Knobs.StrategyRotation
	// is empty. Either wrong_seq or wrong_checksum today.
	Strategy bypass.Name
	// SNIPool is the list of fake SNI values to choose from per flow.
	// Must be non-empty.
	SNIPool []string
	// Scope narrows the flow to a specific 5-tuple-family; only flows
	// matching Scope.LocalIP/RemoteIP/RemotePort are bypassed.
	Scope platform.Scope
	// InjectDelay is used when Knobs.RandomizeTiming is false.
	InjectDelay time.Duration
	// Knobs is the anti-fingerprinting configuration. Zero value matches
	// upstream (fixed delay, fixed size, +1 IP ID, no strategy rotation).
	Knobs fingerprint.Knobs
	// Log is the logger; nil means silent.
	Log Logger
}

// Engine is the cross-platform bypass state machine.
type Engine struct {
	cfg     Config
	backend platform.Backend
	rnd     *fingerprint.Randomizer
	table   *conntrack.Table
	// flows holds per-flow randomization picks keyed by conntrack.Tuple.
	// Kept separate from conntrack.Connection to avoid coupling conntrack
	// to bypass decisions. Values are *flowState.
	flows sync.Map
}

// New returns an unstarted Engine bound to a backend.
func New(cfg Config, be platform.Backend) (*Engine, error) {
	if len(cfg.SNIPool) == 0 {
		return nil, errors.New("engine: SNIPool must be non-empty")
	}
	// Validate the fallback and any rotation entries up front so we fail fast.
	if _, err := bypass.ByName(cfg.Strategy); err != nil {
		return nil, err
	}
	for _, n := range cfg.Knobs.StrategyRotation {
		if _, err := bypass.ByName(n); err != nil {
			return nil, fmt.Errorf("engine: invalid strategy_rotation entry: %w", err)
		}
	}
	if cfg.InjectDelay <= 0 {
		cfg.InjectDelay = 1 * time.Millisecond
	}
	// If caller didn't set timing knob min/max, fold the static InjectDelay in.
	if !cfg.Knobs.RandomizeTiming {
		cfg.Knobs.MinDelay = cfg.InjectDelay
		cfg.Knobs.MaxDelay = cfg.InjectDelay
	}
	return &Engine{
		cfg:     cfg,
		backend: be,
		rnd:     fingerprint.New(cfg.Knobs),
		table:   conntrack.NewTable(),
	}, nil
}

// Run consumes the backend's packet stream until ctx is done.
func (e *Engine) Run(ctx context.Context) error {
	ch := e.backend.Packets()
	for {
		select {
		case <-ctx.Done():
			return nil
		case packet, ok := <-ch:
			if !ok {
				return nil
			}
			e.dispatch(packet)
		}
	}
}

// dispatch classifies the packet, updates flow state, and decides on a
// verdict. It MUST call packet.Verdict exactly once.
func (e *Engine) dispatch(p platform.Packet) {
	parsed, err := parse(p.Raw)
	if err != nil {
		p.Verdict(platform.Accept)
		return
	}

	key, relevant := e.flowKey(p.Dir, parsed)
	if !relevant {
		p.Verdict(platform.Accept)
		return
	}

	if p.Dir == platform.DirOutbound {
		e.handleOutbound(key, parsed, p)
		return
	}
	e.handleInbound(key, parsed, p)
}

// flowState is what we stash alongside each conntrack.Connection so each
// flow can independently choose its own SNI, strategy, and IP-ID delta.
// Padding is rolled into the fake ClientHello at build time, not stored.
type flowState struct {
	sni      string
	strategy bypass.Name
	delta    uint16
}

// flowKey returns the conntrack.Tuple (always oriented local→remote) and
// whether this packet is within our scope.
func (e *Engine) flowKey(dir platform.Direction, p pkt) (conntrack.Tuple, bool) {
	sc := e.cfg.Scope
	var localAddr, remoteAddr netip.Addr
	var localPort, remotePort uint16
	if dir == platform.DirOutbound {
		localAddr, _ = netip.AddrFromSlice(p.srcIP[:])
		remoteAddr, _ = netip.AddrFromSlice(p.dstIP[:])
		localPort, remotePort = p.srcPort, p.dstPort
	} else {
		localAddr, _ = netip.AddrFromSlice(p.dstIP[:])
		remoteAddr, _ = netip.AddrFromSlice(p.srcIP[:])
		localPort, remotePort = p.dstPort, p.srcPort
	}
	if remoteAddr != sc.RemoteIP || remotePort != sc.RemotePort {
		return conntrack.Tuple{}, false
	}
	return conntrack.Tuple{
		SrcIP: localAddr, SrcPort: localPort,
		DstIP: remoteAddr, DstPort: remotePort,
	}, true
}

func (e *Engine) handleOutbound(key conntrack.Tuple, parsed pkt, p platform.Packet) {
	switch {
	case parsed.isSYNOnly():
		fake, sni, err := e.newFakeClientHello()
		if err != nil {
			e.warnf("build fake ClientHello: %v", err)
			p.Verdict(platform.Accept)
			return
		}
		conn := conntrack.New(key, fake)
		conn.Mu.Lock()
		conn.SynSeq = parsed.seqNum
		conn.Stage = conntrack.StageSynSent
		conn.Mu.Unlock()

		fs := &flowState{
			sni:      sni,
			strategy: e.rnd.PickStrategy(e.cfg.Strategy),
			delta:    e.rnd.IPIDDelta(),
		}
		e.flows.Store(key, fs)

		if !e.table.Add(conn) {
			if existing, ok := e.table.Get(key); ok {
				existing.Mu.Lock()
				existing.SynSeq = parsed.seqNum
				existing.Mu.Unlock()
			}
		}
		p.Verdict(platform.Accept)
		return

	case parsed.isACKOnly() && !parsed.hasPayload():
		conn, ok := e.table.Get(key)
		if !ok {
			p.Verdict(platform.Accept)
			return
		}
		conn.Mu.Lock()
		doInject := conn.Stage == conntrack.StageSynAckSeen
		if doInject {
			conn.Stage = conntrack.StageAckSent
		}
		synSeq := conn.SynSeq
		fakeData := conn.FakePayload
		conn.Mu.Unlock()

		p.Verdict(platform.Accept)

		if doInject {
			fsAny, ok := e.flows.Load(key)
			if !ok {
				// Defensive: the flow entry should always exist because we
				// stored it on the SYN before allowing the handshake to
				// progress, but if it does not, skip injection rather than
				// panic. We intentionally do NOT Finish the conn here because
				// the handshake itself is still valid — the user just loses
				// bypass for this one flow.
				e.warnf("missing flowState for %v; skipping inject", key)
				return
			}
			fs := fsAny.(*flowState)
			go e.scheduleInject(conn, parsed, synSeq, fakeData, fs)
		}
		return
	}

	p.Verdict(platform.Accept)
}

func (e *Engine) handleInbound(key conntrack.Tuple, parsed pkt, p platform.Packet) {
	switch {
	case parsed.isSYNACK():
		conn, ok := e.table.Get(key)
		if !ok {
			p.Verdict(platform.Accept)
			return
		}
		conn.Mu.Lock()
		if conn.Stage == conntrack.StageSynSent {
			conn.SynAckSeq = parsed.seqNum
			conn.Stage = conntrack.StageSynAckSeen
		}
		conn.Mu.Unlock()
		p.Verdict(platform.Accept)
		return

	case parsed.isACKOnly() && !parsed.hasPayload():
		conn, ok := e.table.Get(key)
		if !ok {
			p.Verdict(platform.Accept)
			return
		}
		conn.Mu.Lock()
		if conn.Stage == conntrack.StageFakeSent {
			conn.Finish(true, "fake_data_ack_recv")
			e.table.Remove(key)
			e.flows.Delete(key)
		}
		conn.Mu.Unlock()
		p.Verdict(platform.Accept)
		return
	}

	p.Verdict(platform.Accept)
}

// scheduleInject sleeps the randomizer-chosen delay then emits the spoofed
// packet. The ACK that triggered us is passed by value so we can build the
// injection even if the original buffer has been released.
func (e *Engine) scheduleInject(conn *conntrack.Connection, ack pkt,
	synSeq uint32, fakeData []byte, fs *flowState) {

	time.Sleep(e.rnd.Delay())

	strat, _ := bypass.ByName(fs.strategy) // validated at New() time
	mut, err := strat.Plan(bypass.Inputs{
		SynSeq:      synSeq,
		FakePayload: fakeData,
	})
	if err != nil {
		e.warnf("strategy.Plan: %v", err)
		return
	}

	spec := injectSpec{
		ipIDDelta:       fs.delta,
		setPSH:          mut.SetPSH,
		payload:         mut.Payload,
		corruptChecksum: mut.CorruptChecksum,
	}
	if mut.OverrideSeq {
		spec.seqNum = mut.SeqNum
	} else {
		spec.seqNum = ack.seqNum
	}

	inj := buildInjection(ack, spec)

	conn.Mu.Lock()
	if conn.Stage == conntrack.StageAckSent {
		conn.Stage = conntrack.StageFakeSent
	}
	conn.Mu.Unlock()

	if err := e.backend.Inject(inj, platform.DirOutbound); err != nil {
		e.warnf("inject: %v", err)
		conn.Mu.Lock()
		conn.Finish(false, fmt.Sprintf("inject error: %v", err))
		conn.Mu.Unlock()
		e.table.Remove(conn.Tuple)
		e.flows.Delete(conn.Tuple)
		return
	}
	e.infof("snix: injected %s sni=%s size=%d seq=%#x id_delta=%d",
		fs.strategy, fs.sni, len(inj), spec.seqNum, fs.delta)
}

// newFakeClientHello picks an SNI per the randomizer, builds a ClientHello
// with randomized extra padding, and returns both the bytes and the SNI used
// (for logging).
func (e *Engine) newFakeClientHello() ([]byte, string, error) {
	sni := e.rnd.PickSNI(e.cfg.SNIPool)
	if sni == "" {
		return nil, "", errors.New("engine: empty SNI pool")
	}
	var rnd, sess, ks [32]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return nil, "", err
	}
	if _, err := rand.Read(sess[:]); err != nil {
		return nil, "", err
	}
	if _, err := rand.Read(ks[:]); err != nil {
		return nil, "", err
	}
	extra := e.rnd.ExtraPad()
	buf, err := injector.BuildClientHelloPadded(rnd[:], sess[:], []byte(sni), ks[:], extra)
	if err != nil {
		return nil, "", err
	}
	return buf, sni, nil
}

func (e *Engine) infof(format string, args ...any) {
	if e.cfg.Log != nil {
		e.cfg.Log.Infof(format, args...)
	}
}

func (e *Engine) warnf(format string, args ...any) {
	if e.cfg.Log != nil {
		e.cfg.Log.Warnf(format, args...)
	}
}
