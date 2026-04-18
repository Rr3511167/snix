package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// -- snix start ------------------------------------------------------------

func newStartCmd(g *globalFlags) *cobra.Command {
	var profileName string
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the bypass proxy using the active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(g)
			if err != nil {
				return err
			}
			name := profileName
			if name == "" {
				name = cfg.Active
			}
			p, ok := cfg.Lookup(name)
			if !ok {
				return fmt.Errorf("profile %q not found", name)
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"snix: starting profile %q  listen=%s  connect=%s:%d  strategy=%s\n",
				p.Name, p.Listen, p.Connect.Host, p.Connect.Port, p.Spoof.Strategy)

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigCh
				fmt.Fprintln(cmd.OutOrStdout(), "\nsnix: shutdown requested")
				cancel()
			}()

			return runEngine(ctx, cmd.OutOrStdout(), p)
		},
	}
	cmd.Flags().StringVarP(&profileName, "profile", "p", "", "profile name (defaults to active)")
	return cmd
}

// -- snix status -----------------------------------------------------------

func newStatusCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show config summary and active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(g)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "config: %s\n", g.configPath)
			fmt.Fprintf(out, "active: %s\n\n", cfg.Active)
			tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "PROFILE\tLISTEN\tCONNECT\tSTRATEGY\tSNIs")
			for _, p := range cfg.Profiles {
				marker := ""
				if p.Name == cfg.Active {
					marker = "*"
				}
				fmt.Fprintf(tw, "%s%s\t%s\t%s:%d\t%s\t%d\n",
					marker, p.Name, p.Listen, p.Connect.Host, p.Connect.Port,
					p.Spoof.Strategy, len(p.EffectiveSNIPool()))
			}
			return tw.Flush()
		},
	}
}

// -- snix profile ... ------------------------------------------------------

func newProfileCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Manage profiles"}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List profiles",
			RunE: func(cmd *cobra.Command, _ []string) error {
				cfg, err := loadConfig(g)
				if err != nil {
					return err
				}
				for _, p := range cfg.Profiles {
					marker := " "
					if p.Name == cfg.Active {
						marker = "*"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", marker, p.Name)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "switch NAME",
			Short: "Switch the active profile",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return fmt.Errorf("not yet implemented (phase 4 TUI / API)")
			},
		},
	)
	return cmd
}

// -- snix init -------------------------------------------------------------

func newInitCmd(g *globalFlags) *cobra.Command {
	var wizard, force bool
	c := &cobra.Command{
		Use:   "init",
		Short: "Create a starter config, or run the interactive wizard",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := os.MkdirAll(filepath.Dir(g.configPath), 0o755); err != nil {
				return err
			}
			if _, err := os.Stat(g.configPath); err == nil && !force {
				return fmt.Errorf("config already exists at %s (pass --force to overwrite)", g.configPath)
			}
			if wizard {
				in := bufio.NewReader(cmd.InOrStdin())
				return runWizard(cmd.OutOrStdout(), in, g.configPath)
			}
			return os.WriteFile(g.configPath, []byte(sampleConfig), 0o644)
		},
	}
	c.Flags().BoolVar(&wizard, "wizard", false, "interactive first-run wizard (recommended for new users)")
	c.Flags().BoolVar(&force, "force", false, "overwrite any existing config")
	return c
}

const sampleConfig = `version: 1
active: default
log:
  level: info
profiles:
  - name: default
    listen: "127.0.0.1:40443"
    connect:
      host: my-proxy.example.com
      port: 443
      fallback_ips:
        - 188.114.98.0
        - 104.16.0.1
    spoof:
      # Fallback strategy if strategy_rotation is empty.
      strategy: wrong_seq
      # Rotate strategies per connection to fight fingerprinting.
      strategy_rotation: [wrong_seq, wrong_checksum]
      # SNI pool, chosen "random" by default.
      sni_pool:
        - auth.vercel.com
        - cdn.segment.io
        - static.cloudflareinsights.com
      sni_selection: random
      # Jitter the injection delay (breaks the fixed 1ms timing signature).
      randomize_timing: true
      min_delay: 500us
      max_delay: 5ms
      # Vary ClientHello size (breaks the fixed 517-byte signature).
      randomize_padding: true
      min_extra_pad: 0
      max_extra_pad: 600
      # Randomize the IP ident delta (breaks the +1 signature).
      ip_id_delta_range: 32
    health:
      interval: 30s
      auto_failover: true
`
