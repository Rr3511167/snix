package scanner

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

// tlsTestServer spins up a self-signed TLS listener and returns its addr.
func tlsTestServer(t *testing.T) (addr string, stop func()) {
	t.Helper()
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"test"},
	}
	der, _ := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			// Complete handshake by reading one byte.
			_ = c.(*tls.Conn).Handshake()
			_ = c.Close()
		}
	}()
	return ln.Addr().String(), func() { _ = ln.Close() }
}

func hostPort(t *testing.T, addr string) (netip.Addr, uint16) {
	t.Helper()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	ap, err := netip.ParseAddrPort(net.JoinHostPort(host, port))
	if err != nil {
		t.Fatal(err)
	}
	return ap.Addr(), ap.Port()
}

func TestProbeSNISuccess(t *testing.T) {
	addr, stop := tlsTestServer(t)
	defer stop()
	ip, port := hostPort(t, addr)

	r := ProbeSNI(context.Background(), "whatever.example.com", ProbeConfig{
		TargetIP:   ip,
		TargetPort: port,
	})
	if r.Outcome != OutcomeOK {
		t.Fatalf("outcome: %s err=%q", r.Outcome, r.Err)
	}
	if r.Handshake == 0 {
		t.Error("handshake time not measured")
	}
}

func TestProbeSNIRefused(t *testing.T) {
	// Port 1 on loopback should refuse.
	r := ProbeSNI(context.Background(), "example.com", ProbeConfig{
		TargetIP:       netip.MustParseAddr("127.0.0.1"),
		TargetPort:     1,
		ConnectTimeout: 500 * time.Millisecond,
	})
	if r.Outcome != OutcomeTCPRefused && r.Outcome != OutcomeDPIReset {
		t.Fatalf("outcome: %s err=%q (want refused/reset)", r.Outcome, r.Err)
	}
}

func TestProbeSNITimeout(t *testing.T) {
	// 10.255.255.1 is TEST-NET-unused-ish; on many hosts it silently drops.
	// Use a very short timeout so the test doesn't stall if the net does route.
	r := ProbeSNI(context.Background(), "example.com", ProbeConfig{
		TargetIP:       netip.MustParseAddr("192.0.2.1"), // TEST-NET-1
		TargetPort:     443,
		ConnectTimeout: 250 * time.Millisecond,
	})
	if r.Outcome != OutcomeTimeout && r.Outcome != OutcomeTCPRefused {
		t.Fatalf("outcome: %s (want timeout or refused)", r.Outcome)
	}
}

func TestProbeSNIsConcurrent(t *testing.T) {
	addr, stop := tlsTestServer(t)
	defer stop()
	ip, port := hostPort(t, addr)

	in := []string{"a.com", "b.com", "c.com", "d.com", "e.com"}
	// onResult may fire from multiple goroutines — caller is responsible
	// for synchronising. Using atomic to keep the race detector happy.
	var resultsSeen atomic.Int64
	results := ProbeSNIs(context.Background(), in, ProbeConfig{
		TargetIP:    ip,
		TargetPort:  port,
		Concurrency: 2,
	}, func(Result) { resultsSeen.Add(1) })
	if len(results) != len(in) {
		t.Fatalf("len: %d want %d", len(results), len(in))
	}
	if int(resultsSeen.Load()) != len(in) {
		t.Errorf("onResult callback count: %d want %d", resultsSeen.Load(), len(in))
	}
	for i, r := range results {
		if r.SNI != in[i] {
			t.Errorf("order not preserved: [%d] %s != %s", i, r.SNI, in[i])
		}
		if r.Outcome != OutcomeOK {
			t.Errorf("%s: outcome %s err=%q", r.SNI, r.Outcome, r.Err)
		}
	}
}

func TestRankSNIsKeepsOnlyOK(t *testing.T) {
	in := []Result{
		{SNI: "a", Outcome: OutcomeOK, Handshake: 100 * time.Millisecond},
		{SNI: "b", Outcome: OutcomeDPIReset},
		{SNI: "c", Outcome: OutcomeOK, Handshake: 50 * time.Millisecond},
		{SNI: "d", Outcome: OutcomeOK, Handshake: 200 * time.Millisecond},
	}
	ranked := RankSNIs(in)
	if len(ranked) != 3 {
		t.Fatalf("len: %d want 3", len(ranked))
	}
	if ranked[0].SNI != "c" || ranked[1].SNI != "a" || ranked[2].SNI != "d" {
		t.Fatalf("order: %+v", ranked)
	}
}
