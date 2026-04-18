//go:build linux && integration

// Integration tests require root + netfilter and are guarded by the
// "integration" build tag. Run with:
//
//	go test -tags=integration -count=1 ./platform/linux/ -v
//
// These tests are narrowly scoped to a safe (unused) remote IP/port pair so
// they cannot affect the SSH control plane or any live traffic.
package linux

import (
	"context"
	"net"
	"net/netip"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/SamNet-dev/snix/platform"
)

func mustBeRoot(t *testing.T) {
	t.Helper()
	// iptables listing requires root on this system.
	if err := exec.Command("iptables", "-t", "mangle", "-L", "-n").Run(); err != nil {
		t.Skipf("skipping: no root / iptables: %v", err)
	}
}

// safeScope uses a deliberately unused IP+port so installed rules can't
// interfere with real traffic. 198.51.100.0/24 is TEST-NET-2 (RFC 5737).
func safeScope() platform.Scope {
	return platform.Scope{
		LocalIP:    netip.MustParseAddr("127.0.0.1"),
		RemoteIP:   netip.MustParseAddr("198.51.100.77"),
		RemotePort: 59999,
	}
}

// TestOpenInstallsAndCloseRemovesRules verifies our iptables bookkeeping is
// correct: after Open the rules exist, after Close they're gone.
func TestOpenInstallsAndCloseRemovesRules(t *testing.T) {
	mustBeRoot(t)
	b := New(Config{QueueNum: 4242})

	if err := b.Open(context.Background(), safeScope()); err != nil {
		t.Fatalf("Open: %v", err)
	}

	out, err := exec.Command("iptables", "-t", "mangle", "-S").CombinedOutput()
	if err != nil {
		b.Close()
		t.Fatalf("iptables -S: %v", err)
	}
	dump := string(out)
	wantSubstrs := []string{"198.51.100.77", "59999", "NFQUEUE", "--queue-num 4242"}
	for _, s := range wantSubstrs {
		if !strings.Contains(dump, s) {
			b.Close()
			t.Fatalf("rules missing %q in:\n%s", s, dump)
		}
	}

	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	out2, err := exec.Command("iptables", "-t", "mangle", "-S").CombinedOutput()
	if err != nil {
		t.Fatalf("iptables -S: %v", err)
	}
	if strings.Contains(string(out2), "198.51.100.77") {
		t.Fatalf("rules leaked after Close:\n%s", string(out2))
	}
}

// TestDoubleCloseIsSafe verifies Close is idempotent.
func TestDoubleCloseIsSafe(t *testing.T) {
	mustBeRoot(t)
	b := New(Config{QueueNum: 4243})
	if err := b.Open(context.Background(), safeScope()); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestRawSocketAvailable verifies we can acquire a raw socket + IP_HDRINCL.
// If this fails, Inject() cannot work on this host (missing CAP_NET_RAW).
func TestRawSocketAvailable(t *testing.T) {
	mustBeRoot(t)
	b := New(Config{QueueNum: 4244})
	if err := b.Open(context.Background(), safeScope()); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if b.rawSock < 0 {
		t.Fatal("raw socket not opened")
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

var _ = net.Listen // keep net imported for future use
var _ = time.Second
