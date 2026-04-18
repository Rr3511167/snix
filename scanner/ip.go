package scanner

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"sync"
	"time"
)

// IPResult is one IP probe's outcome.
type IPResult struct {
	IP        netip.Addr
	Port      uint16
	Reachable bool
	RTT       time.Duration
	Err       string
}

// ProbeIP dials ip:port with cfg.ConnectTimeout and measures RTT. This is a
// plain TCP connect; no TLS. Returns Reachable=true only on successful
// three-way handshake.
func ProbeIP(ctx context.Context, ip netip.Addr, port uint16, timeout time.Duration) IPResult {
	if port == 0 {
		port = 443
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	r := IPResult{IP: ip, Port: port}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp",
		net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port)))
	r.RTT = time.Since(start)
	if err != nil {
		r.Err = err.Error()
		return r
	}
	_ = conn.Close()
	r.Reachable = true
	return r
}

// ProbeIPs runs ProbeIP concurrently. Output order matches input.
func ProbeIPs(ctx context.Context, ips []string, port uint16, timeout time.Duration,
	concurrency int, onResult func(IPResult)) []IPResult {
	if concurrency <= 0 {
		concurrency = 16
	}
	sem := make(chan struct{}, concurrency)
	out := make([]IPResult, len(ips))
	var wg sync.WaitGroup
	for i, s := range ips {
		addr, err := netip.ParseAddr(s)
		if err != nil {
			out[i] = IPResult{Err: "parse: " + err.Error()}
			continue
		}
		i, addr := i, addr
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			r := ProbeIP(ctx, addr, port, timeout)
			out[i] = r
			if onResult != nil {
				onResult(r)
			}
		}()
	}
	wg.Wait()
	return out
}

// RankIPs sorts reachable IPs by RTT ascending. Unreachable are dropped.
func RankIPs(in []IPResult) []IPResult {
	out := make([]IPResult, 0, len(in))
	for _, r := range in {
		if r.Reachable {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RTT < out[j].RTT })
	return out
}

// RankSNIs keeps only OK outcomes and sorts by handshake time ascending.
func RankSNIs(in []Result) []Result {
	out := make([]Result, 0, len(in))
	for _, r := range in {
		if r.Outcome == OutcomeOK {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Handshake < out[j].Handshake })
	return out
}
