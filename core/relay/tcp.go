// Package relay implements the bidirectional TCP proxy that sits between a
// local VPN client and the upstream (cloaked) server.
//
// The relay is platform-agnostic: once the DPI bypass (packet injection) has
// succeeded for a flow, the relay simply shuttles bytes in both directions.
// Zero-copy fast paths (splice on Linux, TransmitFile on Windows) are added
// in the platform packages.
package relay

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// defaultBufSize tunes io.CopyBuffer. 32 KiB matches Go's default and fits
// within one IPv4/TCP MSS window comfortably on most links.
const defaultBufSize = 32 * 1024

// bufferPool recycles relay buffers across connections to keep allocations
// off the hot path.
var bufferPool = sync.Pool{
	New: func() any {
		b := make([]byte, defaultBufSize)
		return &b
	},
}

// Options configures relay behaviour.
type Options struct {
	// IdleTimeout aborts a relay if no data moves in either direction for
	// this long. Zero means no timeout.
	IdleTimeout time.Duration
	// OnBytes, if set, is called periodically with (txToRemote, rxFromRemote)
	// cumulative byte counts for metrics.
	OnBytes func(tx, rx uint64)
}

// Run copies bytes between client and remote until either side closes or ctx
// is cancelled. Both conns are closed on return. It returns the first non-EOF
// error encountered, or nil on normal close.
func Run(ctx context.Context, client, remote net.Conn, opt Options) error {
	defer client.Close()
	defer remote.Close()

	// Byte counters must be atomic: each goroutine writes its own counter,
	// but when OnBytes is enabled the per-tick callback reads BOTH
	// counters from whichever goroutine fires. Plain uint64 access would
	// race (triggered only when OnBytes != nil, which is why the race
	// detector misses it unless tests cover that path).
	type dir struct {
		src, dst net.Conn
		label    string
		n        atomic.Uint64
	}
	var (
		c2r = &dir{src: client, dst: remote, label: "c->r"}
		r2c = &dir{src: remote, dst: client, label: "r->c"}

		wg       sync.WaitGroup
		firstErr error
		errMu    sync.Mutex
	)

	copy := func(d *dir) {
		defer wg.Done()
		// Pull a buffer from the pool.
		bp := bufferPool.Get().(*[]byte)
		defer bufferPool.Put(bp)
		buf := *bp

		for {
			if opt.IdleTimeout > 0 {
				_ = d.src.SetReadDeadline(time.Now().Add(opt.IdleTimeout))
			}
			n, err := d.src.Read(buf)
			if n > 0 {
				if _, werr := d.dst.Write(buf[:n]); werr != nil {
					recordErr(&errMu, &firstErr, werr)
					// Half-close the other side so the peer copy exits.
					_ = halfCloseWrite(d.src)
					return
				}
				total := d.n.Add(uint64(n))
				if opt.OnBytes != nil {
					if d == c2r {
						opt.OnBytes(total, r2c.n.Load())
					} else {
						opt.OnBytes(c2r.n.Load(), total)
					}
				}
			}
			if err != nil {
				if !isExpectedCloseErr(err) {
					recordErr(&errMu, &firstErr, err)
				}
				// Signal EOF to the peer: shut down writes on dst so peer's
				// Read eventually returns.
				_ = halfCloseWrite(d.dst)
				return
			}
		}
	}

	wg.Add(2)
	go copy(c2r)
	go copy(r2c)

	// Cancel support: if ctx is cancelled, force-close both ends so the
	// goroutines exit.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = client.Close()
			_ = remote.Close()
		case <-done:
		}
	}()
	wg.Wait()
	close(done)
	return firstErr
}

func recordErr(mu *sync.Mutex, dst *error, err error) {
	mu.Lock()
	defer mu.Unlock()
	if *dst == nil {
		*dst = err
	}
}

func isExpectedCloseErr(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return false
}

// halfCloseWrite attempts a CloseWrite on TCP conns so the peer sees EOF
// cleanly. For non-TCP conns it falls back to Close.
func halfCloseWrite(c net.Conn) error {
	type closeWriter interface{ CloseWrite() error }
	if cw, ok := c.(closeWriter); ok {
		return cw.CloseWrite()
	}
	return c.Close()
}

// Listener is a thin wrapper that applies relay-friendly TCP socket options.
// Individual platform backends may replace it with one that also installs
// packet-filter rules, but the defaults here work anywhere Go runs.
type Listener struct {
	net.Listener
	KeepAlive time.Duration
}

// NewListener binds addr and returns a *Listener with TCP keepalive preset.
func NewListener(addr string, keepAlive time.Duration) (*Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &Listener{Listener: l, KeepAlive: keepAlive}, nil
}

// Accept returns the next inbound conn with KeepAlive configured.
func (l *Listener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	if tc, ok := c.(*net.TCPConn); ok && l.KeepAlive > 0 {
		_ = tc.SetKeepAlive(true)
		_ = tc.SetKeepAlivePeriod(l.KeepAlive)
	}
	return c, nil
}
