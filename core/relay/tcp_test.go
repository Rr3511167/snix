package relay

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// echoServer accepts one conn, echoes everything it reads, then exits.
func echoServer(t *testing.T) (addr string, stop func()) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = io.Copy(c, c)
	}()
	return l.Addr().String(), func() { _ = l.Close() }
}

// tcpPair returns two already-connected TCP conns for loopback testing.
// Both support CloseWrite, unlike net.Pipe.
func tcpPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	type result struct {
		c   net.Conn
		err error
	}
	ch := make(chan result, 1)
	go func() {
		c, err := l.Accept()
		ch <- result{c, err}
	}()
	dialed, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	r := <-ch
	if r.err != nil {
		t.Fatal(r.err)
	}
	return dialed, r.c
}

// TestRunEchoes verifies bytes flow in both directions and both sides see
// each other's EOF.
func TestRunEchoes(t *testing.T) {
	srvAddr, stopSrv := echoServer(t)
	defer stopSrv()

	driverSide, clientRelaySide := tcpPair(t)
	remote, err := net.Dial("tcp", srvAddr)
	if err != nil {
		t.Fatal(err)
	}

	payload := make([]byte, 64*1024)
	_, _ = rand.Read(payload)

	var (
		runErr error
		wg     sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		runErr = Run(context.Background(), clientRelaySide, remote, Options{})
	}()

	if _, err := driverSide.Write(payload); err != nil {
		t.Fatal(err)
	}
	_ = driverSide.(*net.TCPConn).CloseWrite()

	got, err := io.ReadAll(driverSide)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("echo mismatch: got %d bytes want %d", len(got), len(payload))
	}

	wg.Wait()
	if runErr != nil {
		t.Fatalf("Run returned err: %v", runErr)
	}
}

// TestRunOnBytesNoDataRace exercises the OnBytes callback, which reads
// the peer direction's byte counter. Intended to catch regressions where
// the counters are not atomic. Run with `go test -race`.
func TestRunOnBytesNoDataRace(t *testing.T) {
	srvAddr, stopSrv := echoServer(t)
	defer stopSrv()

	driverSide, clientRelaySide := tcpPair(t)
	remote, err := net.Dial("tcp", srvAddr)
	if err != nil {
		t.Fatal(err)
	}

	var callbackCount int64
	runOpts := Options{
		OnBytes: func(tx, rx uint64) { atomic.AddInt64(&callbackCount, 1) },
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = Run(context.Background(), clientRelaySide, remote, runOpts)
	}()

	// Drive bidirectional traffic for a bit.
	payload := bytes.Repeat([]byte("x"), 4*1024)
	for i := 0; i < 10; i++ {
		if _, err := driverSide.Write(payload); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, len(payload))
		if _, err := io.ReadFull(driverSide, buf); err != nil {
			t.Fatal(err)
		}
	}
	_ = driverSide.(*net.TCPConn).CloseWrite()
	_, _ = io.Copy(io.Discard, driverSide)
	wg.Wait()
	if atomic.LoadInt64(&callbackCount) == 0 {
		t.Fatal("OnBytes never fired")
	}
}

// TestRunContextCancelClosesConns ensures cancelling ctx stops the relay.
func TestRunContextCancelClosesConns(t *testing.T) {
	a1, a2 := net.Pipe()
	b1, b2 := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		_ = Run(ctx, a1, b1, Options{})
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancel")
	}
	// Reads on the other ends should now return quickly.
	_ = a2.SetDeadline(time.Now().Add(100 * time.Millisecond))
	_ = b2.SetDeadline(time.Now().Add(100 * time.Millisecond))
	_, _ = io.ReadAll(a2)
	_, _ = io.ReadAll(b2)
}
