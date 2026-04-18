//go:build linux

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"

	"github.com/SamNet-dev/snix/config"
	"github.com/SamNet-dev/snix/core/engine"
	"github.com/SamNet-dev/snix/core/fingerprint"
	"github.com/SamNet-dev/snix/platform"
	linuxbe "github.com/SamNet-dev/snix/platform/linux"
)

// stdoutLogger adapts io.Writer to engine.Logger with Printf-style lines.
type stdoutLogger struct{ w io.Writer }

func (l stdoutLogger) Debugf(f string, args ...any) { fmt.Fprintf(l.w, "DEBUG "+f+"\n", args...) }
func (l stdoutLogger) Infof(f string, args ...any)  { fmt.Fprintf(l.w, f+"\n", args...) }
func (l stdoutLogger) Warnf(f string, args ...any)  { fmt.Fprintf(l.w, "WARN  "+f+"\n", args...) }

// runEngine is the platform-specific entry point invoked by `snix start`.
// It opens the Linux NFQUEUE backend, spawns the bypass engine, and runs
// a pass-through TCP relay on profile.Listen.
func runEngine(ctx context.Context, out io.Writer, p *config.Profile) error {
	remoteAddr, err := resolveIPv4(p.Connect.Host)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", p.Connect.Host, err)
	}
	localIP, err := defaultIfaceIPv4(remoteAddr)
	if err != nil {
		return fmt.Errorf("local iface: %w", err)
	}
	sniPool := p.EffectiveSNIPool()
	fmt.Fprintf(out, "snix: local=%s  remote=%s:%d  sni_pool=%v  strategy=%s\n",
		localIP, remoteAddr, p.Connect.Port, sniPool, p.Spoof.Strategy)
	if p.Spoof.RandomizeTiming || p.Spoof.RandomizePadding || len(p.Spoof.StrategyRotation) > 0 {
		fmt.Fprintf(out, "snix: randomization active timing=%v padding=%v rotation=%v id_delta_range=%d\n",
			p.Spoof.RandomizeTiming, p.Spoof.RandomizePadding,
			p.Spoof.StrategyRotation, p.Spoof.IPIDDeltaRange)
	}

	scope := platform.Scope{
		LocalIP:    localIP,
		RemoteIP:   remoteAddr,
		RemotePort: p.Connect.Port,
	}

	be := linuxbe.New(linuxbe.Config{})
	if err := be.Open(ctx, scope); err != nil {
		return fmt.Errorf("backend open: %w", err)
	}
	fmt.Fprintf(out, "snix: iptables rules installed, NFQUEUE active\n")
	defer func() {
		if err := be.Close(); err != nil {
			fmt.Fprintf(out, "snix: close: %v\n", err)
		} else {
			fmt.Fprintf(out, "snix: iptables rules removed\n")
		}
	}()

	// Spin up the bypass engine with all anti-fingerprinting knobs from config.
	eng, err := engine.New(engine.Config{
		Strategy: p.Spoof.Strategy,
		SNIPool:  sniPool,
		Scope:    scope,
		Log:      stdoutLogger{w: out},
		Knobs: fingerprint.Knobs{
			RandomizeTiming:  p.Spoof.RandomizeTiming,
			MinDelay:         p.Spoof.MinDelay,
			MaxDelay:         p.Spoof.MaxDelay,
			RandomizePadding: p.Spoof.RandomizePadding,
			MinExtraPad:      p.Spoof.MinExtraPad,
			MaxExtraPad:      p.Spoof.MaxExtraPad,
			IPIDDeltaRange:   p.Spoof.IPIDDeltaRange,
			StrategyRotation: p.Spoof.StrategyRotation,
			SNISelection:     p.Spoof.SNISelection,
		},
	}, be)
	if err != nil {
		return fmt.Errorf("engine: %w", err)
	}
	go eng.Run(ctx)
	fmt.Fprintf(out, "snix: bypass engine running\n")

	ln, err := net.Listen("tcp", p.Listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", p.Listen, err)
	}
	defer ln.Close()
	fmt.Fprintf(out, "snix: listening on %s\n", p.Listen)

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go handleConn(ctx, out, c, remoteAddr, p.Connect.Port)
		}
	}()

	<-ctx.Done()
	return nil
}

func handleConn(ctx context.Context, out io.Writer, c net.Conn, remote netip.Addr, port uint16) {
	defer c.Close()
	dialer := &net.Dialer{}
	up, err := dialer.DialContext(ctx, "tcp",
		net.JoinHostPort(remote.String(), fmt.Sprintf("%d", port)))
	if err != nil {
		fmt.Fprintf(out, "snix: dial %s:%d: %v\n", remote, port, err)
		return
	}
	defer up.Close()

	go io.Copy(up, c)
	_, _ = io.Copy(c, up)
}

func resolveIPv4(host string) (netip.Addr, error) {
	if a, err := netip.ParseAddr(host); err == nil && a.Is4() {
		return a, nil
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return netip.Addr{}, err
	}
	for _, a := range addrs {
		if v4 := a.To4(); v4 != nil {
			addr, _ := netip.AddrFromSlice(v4)
			return addr, nil
		}
	}
	return netip.Addr{}, fmt.Errorf("no IPv4 address for %s", host)
}

func defaultIfaceIPv4(remote netip.Addr) (netip.Addr, error) {
	c, err := net.Dial("udp", net.JoinHostPort(remote.String(), "1"))
	if err != nil {
		return netip.Addr{}, err
	}
	defer c.Close()
	la := c.LocalAddr().(*net.UDPAddr)
	addr, _ := netip.AddrFromSlice(la.IP.To4())
	return addr, nil
}
