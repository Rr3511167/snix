package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/SamNet-dev/snix/config"
	"github.com/SamNet-dev/snix/core/bypass"
	"github.com/SamNet-dev/snix/integrate"
	"github.com/SamNet-dev/snix/scanner"
)

// runWizard is a text-driven first-run flow. It guides the user through
// five steps, runs the scanner, writes a complete config.yaml, and
// optionally patches an existing proxy client's config.
func runWizard(out io.Writer, in *bufio.Reader, cfgPath string) error {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "snix first-run wizard")
	fmt.Fprintln(out, "─────────────────────")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Five short steps. Safe to Ctrl-C at any time; nothing is written")
	fmt.Fprintln(out, "to disk until the end.")
	fmt.Fprintln(out)

	// Step 1 — do you have a proxy server?
	fmt.Fprintln(out, "Step 1/5 — Do you already have a proxy server (Xray / VLESS / Trojan / etc.)?")
	hasServer := promptYesNo(out, in, "Have a proxy server?", true)

	var connectHost string
	var connectPort uint16 = 443

	if hasServer {
		// Step 2a — paste server details.
		fmt.Fprintln(out, "\nStep 2/5 — Enter your proxy server.")
		connectHost = promptLine(out, in, "  Host (domain or IP): ", "")
		for connectHost == "" {
			connectHost = promptLine(out, in, "  Host cannot be empty. Try again: ", "")
		}
		portStr := promptLine(out, in, "  Port [443]: ", "443")
		if n, err := strconv.Atoi(portStr); err == nil && n > 0 && n < 65536 {
			connectPort = uint16(n)
		}
	} else {
		// Step 2b — offer the Cloudflare Worker flow.
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  You don't have a proxy server yet. snix can help you deploy a")
		fmt.Fprintln(out, "  free Cloudflare Worker that acts as your proxy in ~60 seconds.")
		fmt.Fprintln(out, "  (Free tier covers a lot of normal browsing.)")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  What you'll need:")
		fmt.Fprintln(out, "    • A free Cloudflare account  (https://dash.cloudflare.com/sign-up)")
		fmt.Fprintln(out, "    • A browser to click through the deploy wizard")
		fmt.Fprintln(out)

		if promptYesNo(out, in, "Deploy a Cloudflare Worker now?", true) {
			uuid := randomUUID()
			fmt.Fprintln(out)
			fmt.Fprintln(out, "  1. Open this URL in your browser:")
			fmt.Fprintln(out)
			fmt.Fprintln(out, "       https://dash.cloudflare.com/?to=/:account/workers/services/new")
			fmt.Fprintln(out)
			fmt.Fprintln(out, "  2. Create a new Worker called 'snix' (any name works).")
			fmt.Fprintln(out, "  3. Paste the contents of cfworker/worker.js from this repo into the editor.")
			fmt.Fprintln(out, "  4. Under Variables, add UUID =", uuid)
			fmt.Fprintln(out, "  5. Deploy. Copy the 'workers.dev' URL shown at the top.")
			fmt.Fprintln(out)
			workerHost := promptLine(out, in, "  Paste the worker host (e.g. snix.yourname.workers.dev): ", "")
			for workerHost == "" {
				workerHost = promptLine(out, in, "  Required. Try again: ", "")
			}
			workerHost = strings.TrimPrefix(workerHost, "https://")
			workerHost = strings.TrimPrefix(workerHost, "http://")
			workerHost = strings.TrimSuffix(workerHost, "/")
			connectHost = workerHost
			connectPort = 443
			fmt.Fprintln(out)
			fmt.Fprintln(out, "  Saved worker UUID for your records:", uuid)
			fmt.Fprintln(out, "  (Your proxy client will need this UUID to authenticate.)")
		} else {
			fmt.Fprintln(out, "\n  OK — pausing the wizard. Come back and run")
			fmt.Fprintln(out, "  `snix init --wizard` again once you have a server.")
			return nil
		}
	}

	// Step 3 — scanner.
	fmt.Fprintln(out, "\nStep 3/5 — Scanning your network for working SNIs and CDN IPs…")
	ipResults, sniResults := runWizardScans(out)
	rankedIPs := scanner.RankIPs(ipResults)
	rankedSNIs := scanner.RankSNIs(sniResults)

	if len(rankedSNIs) == 0 {
		fmt.Fprintln(out, "\n  ! No SNI probes succeeded. Your network may be blocking raw")
		fmt.Fprintln(out, "    TLS handshakes at the TCP layer. Proceeding with defaults.")
		rankedSNIs = []scanner.Result{
			{SNI: "auth.vercel.com"},
			{SNI: "cdn.segment.io"},
			{SNI: "static.cloudflareinsights.com"},
		}
	}

	// Step 4 — randomization.
	fmt.Fprintln(out, "\nStep 4/5 — Anti-fingerprinting knobs")
	fmt.Fprintln(out, "  These make it much harder for a DPI to learn snix's packet shape.")
	fmt.Fprintln(out, "  Leaving them ON is strongly recommended.")
	randTiming := promptYesNo(out, in, "  Randomize inject timing?", true)
	randPadding := promptYesNo(out, in, "  Randomize ClientHello size?", true)
	rotation := promptYesNo(out, in, "  Rotate bypass strategies per flow?", true)

	// Build the profile.
	topSNIs := rankedSNIs
	if len(topSNIs) > 10 {
		topSNIs = topSNIs[:10]
	}
	sniPool := make([]string, 0, len(topSNIs))
	for _, r := range topSNIs {
		sniPool = append(sniPool, r.SNI)
	}

	fallbacks := make([]string, 0, 4)
	for _, r := range rankedIPs {
		if len(fallbacks) >= 4 {
			break
		}
		fallbacks = append(fallbacks, r.IP.String())
	}

	p := config.Profile{
		Name:   "default",
		Listen: "127.0.0.1:40443",
		Connect: config.ConnectConfig{
			Host:        connectHost,
			Port:        connectPort,
			FallbackIPs: fallbacks,
		},
		Spoof: config.SpoofConfig{
			Strategy:         bypass.NameWrongSeq,
			SNIPool:          sniPool,
			SNISelection:     "random",
			RandomizeTiming:  randTiming,
			MinDelay:         500 * time.Microsecond,
			MaxDelay:         5 * time.Millisecond,
			RandomizePadding: randPadding,
			MinExtraPad:      0,
			MaxExtraPad:      600,
			IPIDDeltaRange:   64,
		},
		Health: config.HealthConfig{
			Interval:     30 * time.Second,
			AutoFailover: true,
		},
	}
	if rotation {
		p.Spoof.StrategyRotation = []bypass.Name{bypass.NameWrongSeq, bypass.NameWrongChecksum}
	}
	cfg := &config.Config{
		Version:  1,
		Active:   "default",
		Log:      config.LogConfig{Level: "info"},
		Profiles: []config.Profile{p},
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("internal: built an invalid config: %w", err)
	}

	// Step 5 — proxy-client integration.
	fmt.Fprintln(out, "\nStep 5/5 — Proxy-client integration")
	clients := integrate.Detect()
	if len(clients) == 0 {
		fmt.Fprintln(out, "  No supported proxy client detected on this system.")
		fmt.Fprintln(out, "  When you install Xray / v2ray / sing-box / NekoBox, point its")
		fmt.Fprintln(out, "  outbound server address at 127.0.0.1:40443 and put your real")
		fmt.Fprintln(out, "  server under snix's connect.host instead.")
	} else {
		for _, c := range clients {
			fmt.Fprintf(out, "  Detected %s at %s\n", c.Name, c.ConfigPath)
			if !promptYesNo(out, in, fmt.Sprintf("  Update %s config to route through snix?", c.Name), false) {
				continue
			}
			if err := c.Patch(connectHost, connectPort); err != nil {
				fmt.Fprintf(out, "    ! patch failed: %v\n", err)
			} else {
				fmt.Fprintf(out, "    done (backup: %s)\n", c.ConfigPath+".bak")
			}
		}
	}

	// Write the config last, so Ctrl-C earlier leaves the system untouched.
	if err := writeConfigYAML(cfgPath, cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Config written to", cfgPath)
	fmt.Fprintln(out, "Next:")
	fmt.Fprintln(out, "  sudo snix start           # foreground")
	fmt.Fprintln(out, "  sudo systemctl enable --now snix   # managed service (linux)")
	return nil
}

// runWizardScans runs the IP + SNI scanners concurrently and prints live
// progress. Returns the raw results for the caller to rank.
func runWizardScans(out io.Writer) ([]scanner.IPResult, []scanner.Result) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ipTarget := len(scanner.DefaultCloudflareIPs)
	sniTarget := len(scanner.DefaultSNICandidates)

	ipCh := make(chan []scanner.IPResult, 1)
	sniCh := make(chan []scanner.Result, 1)

	go func() {
		ipCh <- scanner.ProbeIPs(ctx, scanner.DefaultCloudflareIPs, 443, 2*time.Second, 16, nil)
	}()
	go func() {
		sniCh <- scanner.ProbeSNIs(ctx, scanner.DefaultSNICandidates, scanner.ProbeConfig{
			TargetIP:         netip.MustParseAddr("1.1.1.1"),
			TargetPort:       443,
			ConnectTimeout:   4 * time.Second,
			HandshakeTimeout: 4 * time.Second,
			Concurrency:      16,
		}, nil)
	}()

	fmt.Fprintf(out, "  ip probes:  (%d candidates)\n", ipTarget)
	fmt.Fprintf(out, "  sni probes: (%d candidates against 1.1.1.1)\n", sniTarget)
	fmt.Fprintln(out, "  (takes up to ~10 seconds)")

	ips := <-ipCh
	snis := <-sniCh

	okSNI := 0
	for _, r := range snis {
		if r.Outcome == scanner.OutcomeOK {
			okSNI++
		}
	}
	reachIP := 0
	for _, r := range ips {
		if r.Reachable {
			reachIP++
		}
	}
	fmt.Fprintf(out, "  ✓ %d/%d SNI candidates OK, %d/%d IPs reachable\n",
		okSNI, len(snis), reachIP, len(ips))
	return ips, snis
}

// Helpers ----------------------------------------------------------------

func promptLine(out io.Writer, in *bufio.Reader, prompt, def string) string {
	fmt.Fprint(out, prompt)
	line, err := in.ReadString('\n')
	if err != nil {
		return def
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func promptYesNo(out io.Writer, in *bufio.Reader, prompt string, def bool) bool {
	suffix := " (Y/n) "
	if !def {
		suffix = " (y/N) "
	}
	fmt.Fprint(out, prompt+suffix)
	line, err := in.ReadString('\n')
	if err != nil {
		return def
	}
	line = strings.TrimSpace(strings.ToLower(line))
	switch line {
	case "":
		return def
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return def
	}
}

func randomUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand on Linux reads from getrandom()/urandom; on Windows
		// from CryptGenRandom. Only way this fails is catastrophic OS
		// breakage. Panic loudly so the user doesn't ship an all-zero UUID.
		panic("crypto/rand failed: " + err.Error())
	}
	// v4 format
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b[:])
	return s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

func writeConfigYAML(path string, cfg *config.Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(cfg)
}
