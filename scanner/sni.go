// Package scanner probes candidate SNIs and CDN IPs to discover which
// combination works on the user's current network. This directly addresses
// the #1 user request upstream (patterniha/SNI-Spoofing Issue #8): "please
// tell me which SNIs work on my ISP".
//
// Design rationale:
//   - We do NOT use the bypass engine during scanning. We send real TLS
//     ClientHellos directly to a target IP and observe outcomes. This
//     faithfully reflects what the user's ISP+DPI would do to a normal
//     TLS connection using that SNI.
//   - Results are classified into discrete Outcomes so the caller can rank
//     and act on them without parsing ad-hoc error strings.
package scanner

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Outcome classifies what happened when we tried to open a TLS connection
// with a given SNI. The ordering encodes "worst to best" for sorting.
type Outcome int

const (
	// OutcomeUnknown is the zero value; only seen when a probe never ran.
	OutcomeUnknown Outcome = iota
	// OutcomeDPIReset: TCP RST before or during TLS handshake, typical
	// DPI-based SNI block.
	OutcomeDPIReset
	// OutcomeTimeout: DPI silently dropped our ClientHello.
	OutcomeTimeout
	// OutcomeTCPRefused: TCP couldn't even connect — address/route problem.
	OutcomeTCPRefused
	// OutcomeTLSError: TLS handshake failed for a non-network reason
	// (bad_certificate, unknown_ca, unrecognized_name). Target-side reject.
	OutcomeTLSError
	// OutcomeOK: TLS handshake completed.
	OutcomeOK
)

// String returns a short label for reports.
func (o Outcome) String() string {
	switch o {
	case OutcomeOK:
		return "ok"
	case OutcomeDPIReset:
		return "dpi_reset"
	case OutcomeTimeout:
		return "timeout"
	case OutcomeTCPRefused:
		return "refused"
	case OutcomeTLSError:
		return "tls_error"
	default:
		return "unknown"
	}
}

// Result is a single SNI probe's output.
type Result struct {
	SNI       string
	TargetIP  netip.Addr
	Outcome   Outcome
	RTT       time.Duration // TCP connect time
	Handshake time.Duration // TCP + TLS time (zero on non-OK)
	Err       string        // human-readable error, "" on success
}

// ProbeConfig tunes a single scan.
type ProbeConfig struct {
	// TargetIP is the IP we connect to. If zero, the scanner resolves each
	// SNI via DNS instead (useful for testing SNI against its own host).
	TargetIP netip.Addr
	// TargetPort is typically 443.
	TargetPort uint16
	// ConnectTimeout bounds the TCP connect phase.
	ConnectTimeout time.Duration
	// HandshakeTimeout bounds the TLS handshake from TCP accept onwards.
	HandshakeTimeout time.Duration
	// Concurrency is the max in-flight probes. <=0 means 16.
	Concurrency int
}

// defaults normalizes zero-values into sane defaults.
func (c *ProbeConfig) defaults() {
	if c.TargetPort == 0 {
		c.TargetPort = 443
	}
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = 3 * time.Second
	}
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = 4 * time.Second
	}
	if c.Concurrency <= 0 {
		c.Concurrency = 16
	}
}

// ProbeSNI runs a single TLS handshake attempt against cfg.TargetIP using
// sni as the SNI. Returns a classified Result; Err is always non-empty for
// non-OK outcomes and always empty for OK.
func ProbeSNI(ctx context.Context, sni string, cfg ProbeConfig) Result {
	cfg.defaults()

	host := sni
	if cfg.TargetIP.IsValid() {
		host = cfg.TargetIP.String()
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", cfg.TargetPort))

	r := Result{SNI: sni, TargetIP: cfg.TargetIP}

	// Stage 1: TCP connect.
	dialCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	start := time.Now()
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", addr)
	r.RTT = time.Since(start)
	if err != nil {
		r.Outcome, r.Err = classifyDialErr(err)
		return r
	}
	defer conn.Close()

	// Stage 2: TLS ClientHello. We deliberately use InsecureSkipVerify so
	// cert mismatches (e.g. TargetIP != sni host) don't count against the
	// SNI — we only care whether the handshake *reached* a server response.
	tlsConf := &tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: true,
		NextProtos:         []string{"h2", "http/1.1"},
	}
	hsCtx, hsCancel := context.WithTimeout(ctx, cfg.HandshakeTimeout)
	defer hsCancel()

	tlsConn := tls.Client(conn, tlsConf)
	hsStart := time.Now()
	err = tlsConn.HandshakeContext(hsCtx)
	r.Handshake = time.Since(hsStart) + r.RTT
	if err != nil {
		r.Outcome, r.Err = classifyTLSErr(err)
		return r
	}
	r.Outcome = OutcomeOK
	return r
}

// ProbeSNIs runs ProbeSNI for every sni in the slice concurrently bounded
// by cfg.Concurrency. Results are returned in the same order as input.
func ProbeSNIs(ctx context.Context, snis []string, cfg ProbeConfig, onResult func(Result)) []Result {
	cfg.defaults()
	sem := make(chan struct{}, cfg.Concurrency)
	results := make([]Result, len(snis))
	var wg sync.WaitGroup
	for i, sni := range snis {
		i, sni := i, sni
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			r := ProbeSNI(ctx, sni, cfg)
			results[i] = r
			if onResult != nil {
				onResult(r)
			}
		}()
	}
	wg.Wait()
	return results
}

// classifyDialErr maps net.Dialer errors into the Outcome taxonomy.
func classifyDialErr(err error) (Outcome, string) {
	if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
		return OutcomeTimeout, err.Error()
	}
	// syscall.ECONNREFUSED, syscall.ECONNRESET: infrastructure problems or
	// an aggressive on-path RST injection. We split refused (no host) from
	// reset (DPI) based on the surface error string.
	if errors.Is(err, syscall.ECONNREFUSED) {
		return OutcomeTCPRefused, err.Error()
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return OutcomeDPIReset, err.Error()
	}
	// On Windows Go surfaces connection reset as a string.
	s := err.Error()
	if strings.Contains(s, "connection reset") || strings.Contains(s, "reset by peer") {
		return OutcomeDPIReset, s
	}
	return OutcomeTCPRefused, s
}

// classifyTLSErr splits TLS-layer errors into "DPI interfered" vs "target
// rejected SNI" buckets. RSTs during handshake are classic DPI signatures.
func classifyTLSErr(err error) (Outcome, string) {
	if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
		return OutcomeTimeout, err.Error()
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, io.ErrUnexpectedEOF) {
		return OutcomeDPIReset, err.Error()
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "connection reset"),
		strings.Contains(s, "reset by peer"),
		strings.Contains(s, "EOF"):
		return OutcomeDPIReset, s
	case strings.Contains(s, "unrecognized name"),
		strings.Contains(s, "unknown_ca"),
		strings.Contains(s, "bad_certificate"):
		return OutcomeTLSError, s
	}
	return OutcomeTLSError, s
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
