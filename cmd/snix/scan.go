package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"os/signal"
	"sort"
	"sync/atomic"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/SamNet-dev/snix/scanner"
)

func newScanRootCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Discover working SNIs and CDN IPs on this network",
		Long: "snix scan helps answer the first question every user of SNI-spoofing asks: " +
			"\"which SNIs work for my ISP?\" Use `snix scan sni` to probe SNI candidates, " +
			"`snix scan ip` to probe CDN IPs for reachability, or `snix scan all` for both.",
	}
	cmd.AddCommand(newScanSNICmd(g), newScanIPCmd(g), newScanAllCmd(g))
	return cmd
}

func newScanSNICmd(g *globalFlags) *cobra.Command {
	var (
		targetIP string
		port     uint16
		conc     int
		jsonOut  bool
		topN     int
		timeout  time.Duration
		limit    int
	)
	cmd := &cobra.Command{
		Use:   "sni",
		Short: "Probe candidate SNIs against a target IP",
		Long: "Probes a curated list of Cloudflare-hosted SNIs by sending a real TLS\n" +
			"ClientHello and observing whether the handshake succeeds, is RST'd, or\n" +
			"times out. Ranks by TLS handshake time. Use the top results to populate\n" +
			"your profile's sni_pool.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := installSignalCancel(cmd.Context())
			defer cancel()

			ip, err := netip.ParseAddr(targetIP)
			if err != nil {
				return fmt.Errorf("invalid --target %q: %w", targetIP, err)
			}

			snis := scanner.DefaultSNICandidates
			if limit > 0 && limit < len(snis) {
				snis = snis[:limit]
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"snix scan sni: probing %d candidates against %s:%d (conc=%d, timeout=%s)\n",
				len(snis), ip, port, conc, timeout)

			var done atomic.Int32
			total := len(snis)
			start := time.Now()
			onResult := func(r scanner.Result) {
				n := done.Add(1)
				var mark string
				switch r.Outcome {
				case scanner.OutcomeOK:
					mark = "✓"
				case scanner.OutcomeDPIReset:
					mark = "✗"
				case scanner.OutcomeTimeout:
					mark = "t"
				default:
					mark = "-"
				}
				if !jsonOut {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"\r[%3d/%3d] %s %-40s %s %10s",
						n, total, mark, r.SNI, r.Outcome, r.Handshake.Round(time.Millisecond))
				}
			}

			results := scanner.ProbeSNIs(ctx, snis, scanner.ProbeConfig{
				TargetIP:         ip,
				TargetPort:       port,
				ConnectTimeout:   timeout,
				HandshakeTimeout: timeout,
				Concurrency:      conc,
			}, onResult)

			if !jsonOut {
				fmt.Fprintf(cmd.ErrOrStderr(), "\r%*s\r", 80, "")
				fmt.Fprintf(cmd.OutOrStdout(), "done in %s\n\n", time.Since(start).Round(time.Millisecond))
			}

			ranked := scanner.RankSNIs(results)
			if topN > 0 && topN < len(ranked) {
				ranked = ranked[:topN]
			}

			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			return writeSNITable(cmd.OutOrStdout(), results, ranked)
		},
	}
	cmd.Flags().StringVarP(&targetIP, "target", "t", "1.1.1.1", "target IP to probe against")
	cmd.Flags().Uint16VarP(&port, "port", "p", 443, "target port")
	cmd.Flags().IntVarP(&conc, "concurrency", "j", 16, "max concurrent probes")
	cmd.Flags().DurationVar(&timeout, "timeout", 4*time.Second, "per-probe timeout")
	cmd.Flags().IntVarP(&topN, "top", "n", 10, "show only top-N successful results")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON instead of a table")
	cmd.Flags().IntVar(&limit, "limit", 0, "probe only first N candidates (debugging)")
	return cmd
}

func newScanIPCmd(g *globalFlags) *cobra.Command {
	var (
		port    uint16
		conc    int
		timeout time.Duration
		topN    int
		jsonOut bool
	)
	cmd := &cobra.Command{
		Use:   "ip",
		Short: "Probe Cloudflare IPs for reachability and latency",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := installSignalCancel(cmd.Context())
			defer cancel()

			ips := scanner.DefaultCloudflareIPs
			fmt.Fprintf(cmd.OutOrStdout(),
				"snix scan ip: probing %d Cloudflare IPs on port %d (conc=%d, timeout=%s)\n",
				len(ips), port, conc, timeout)

			var done atomic.Int32
			total := len(ips)
			start := time.Now()
			onResult := func(r scanner.IPResult) {
				n := done.Add(1)
				mark := "✓"
				if !r.Reachable {
					mark = "✗"
				}
				if !jsonOut {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"\r[%3d/%3d] %s %-18s rtt=%s",
						n, total, mark, r.IP, r.RTT.Round(time.Millisecond))
				}
			}
			results := scanner.ProbeIPs(ctx, ips, port, timeout, conc, onResult)
			if !jsonOut {
				fmt.Fprintf(cmd.ErrOrStderr(), "\r%*s\r", 80, "")
				fmt.Fprintf(cmd.OutOrStdout(), "done in %s\n\n", time.Since(start).Round(time.Millisecond))
			}

			ranked := scanner.RankIPs(results)
			if topN > 0 && topN < len(ranked) {
				ranked = ranked[:topN]
			}
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			return writeIPTable(cmd.OutOrStdout(), results, ranked)
		},
	}
	cmd.Flags().Uint16VarP(&port, "port", "p", 443, "port to probe")
	cmd.Flags().IntVarP(&conc, "concurrency", "j", 16, "max concurrent probes")
	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Second, "per-probe timeout")
	cmd.Flags().IntVarP(&topN, "top", "n", 10, "show only top-N by latency")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON instead of a table")
	return cmd
}

func newScanAllCmd(g *globalFlags) *cobra.Command {
	var sniTarget string
	cmd := &cobra.Command{
		Use:   "all",
		Short: "Run IP and SNI scans together and emit a suggested profile",
		Long: `Runs both scans and emits a ready-to-paste sni_pool plus the top CDN IPs.

Two scans run in parallel:

  1. IP scan     — TCP reachability + RTT for the Cloudflare IP pool.
  2. SNI scan    — TLS handshakes against --sni-target (default 1.1.1.1,
                   which uses a universal SSL cert and accepts any SNI).

The SNI scan's purpose is to find SNIs that the user's ISP does NOT block;
it does not tell you which IPs serve which customer domains. Use the IP
scan results for your profile's connect.host / fallback_ips.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := installSignalCancel(cmd.Context())
			defer cancel()

			sniTargetAddr, err := netip.ParseAddr(sniTarget)
			if err != nil {
				return fmt.Errorf("invalid --sni-target: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"Scanning %d Cloudflare IPs and %d SNI candidates (SNI probes → %s)...\n",
				len(scanner.DefaultCloudflareIPs), len(scanner.DefaultSNICandidates), sniTarget)

			type ipOut struct {
				results []scanner.IPResult
			}
			type sniOut struct {
				results []scanner.Result
			}
			ipCh := make(chan ipOut, 1)
			sniCh := make(chan sniOut, 1)

			go func() {
				r := scanner.ProbeIPs(ctx, scanner.DefaultCloudflareIPs, 443,
					2*time.Second, 16, nil)
				ipCh <- ipOut{r}
			}()
			go func() {
				r := scanner.ProbeSNIs(ctx, scanner.DefaultSNICandidates,
					scanner.ProbeConfig{
						TargetIP:         sniTargetAddr,
						TargetPort:       443,
						ConnectTimeout:   4 * time.Second,
						HandshakeTimeout: 4 * time.Second,
						Concurrency:      16,
					}, nil)
				sniCh <- sniOut{r}
			}()
			ips := (<-ipCh).results
			snis := (<-sniCh).results

			rankedIPs := scanner.RankIPs(ips)
			rankedSNIs := scanner.RankSNIs(snis)

			fmt.Fprintf(cmd.OutOrStdout(), "\nTop IPs by latency:\n")
			top := rankedIPs
			if len(top) > 10 {
				top = top[:10]
			}
			writeIPTable(cmd.OutOrStdout(), ips, top)

			fmt.Fprintf(cmd.OutOrStdout(), "\nTop working SNIs:\n")
			topSNIs := rankedSNIs
			if len(topSNIs) > 10 {
				topSNIs = topSNIs[:10]
			}
			writeSNITable(cmd.OutOrStdout(), snis, topSNIs)

			if len(topSNIs) == 0 || len(top) == 0 {
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nSuggested profile snippet:\n")
			fmt.Fprintln(cmd.OutOrStdout(), "  connect:")
			fmt.Fprintf(cmd.OutOrStdout(), "    host: \"%s\"\n", top[0].IP)
			fmt.Fprintln(cmd.OutOrStdout(), "    port: 443")
			fmt.Fprintln(cmd.OutOrStdout(), "    fallback_ips:")
			for _, ip := range top[:min(5, len(top))] {
				fmt.Fprintf(cmd.OutOrStdout(), "      - %s\n", ip.IP)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "  spoof:")
			fmt.Fprintln(cmd.OutOrStdout(), "    strategy: wrong_seq")
			fmt.Fprintln(cmd.OutOrStdout(), "    sni_pool:")
			for _, r := range topSNIs {
				fmt.Fprintf(cmd.OutOrStdout(), "      - %s\n", r.SNI)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sniTarget, "sni-target", "1.1.1.1",
		"IP to send SNI probes against (default 1.1.1.1 has universal TLS default cert)")
	return cmd
}

func installSignalCancel(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()
	return ctx, cancel
}

func writeSNITable(w interface{ Write(p []byte) (int, error) }, all, ranked []scanner.Result) error {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)

	// Summary counts.
	counts := map[scanner.Outcome]int{}
	for _, r := range all {
		counts[r.Outcome]++
	}
	fmt.Fprintf(w, "Summary: %d ok, %d reset, %d timeout, %d tls_error, %d refused\n\n",
		counts[scanner.OutcomeOK], counts[scanner.OutcomeDPIReset],
		counts[scanner.OutcomeTimeout], counts[scanner.OutcomeTLSError],
		counts[scanner.OutcomeTCPRefused])

	if len(ranked) == 0 {
		fmt.Fprintln(w, "No SNI worked against this target. Try a different --target IP.")
		return nil
	}
	fmt.Fprintln(tw, "RANK\tSNI\tHANDSHAKE\tTCP_RTT")
	for i, r := range ranked {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n",
			i+1, r.SNI, r.Handshake.Round(time.Millisecond), r.RTT.Round(time.Millisecond))
	}
	return tw.Flush()
}

func writeIPTable(w interface{ Write(p []byte) (int, error) }, all, ranked []scanner.IPResult) error {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)

	reachable := 0
	for _, r := range all {
		if r.Reachable {
			reachable++
		}
	}
	fmt.Fprintf(w, "Reachable: %d / %d\n\n", reachable, len(all))
	if len(ranked) == 0 {
		fmt.Fprintln(w, "No reachable IPs. Network issue?")
		return nil
	}
	// Sort displayed-ranked by RTT already, but keep it explicit.
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].RTT < ranked[j].RTT })
	fmt.Fprintln(tw, "RANK\tIP\tRTT")
	for i, r := range ranked {
		fmt.Fprintf(tw, "%d\t%s\t%s\n",
			i+1, r.IP, r.RTT.Round(time.Millisecond))
	}
	return tw.Flush()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
