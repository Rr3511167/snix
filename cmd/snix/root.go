package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/SamNet-dev/snix/config"
)

type globalFlags struct {
	configPath string
	verbose    bool
}

func newRootCmd() *cobra.Command {
	g := &globalFlags{}

	root := &cobra.Command{
		Use:           "snix",
		Short:         "Cross-platform SNI-spoofing DPI-bypass proxy",
		Long:          "snix spoofs the TLS ClientHello SNI during the TCP handshake to bypass DPI-based censorship.\nIt is a Go rewrite of patterniha/SNI-Spoofing with multi-profile, TUI, and cross-platform support.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVarP(&g.configPath, "config", "c", defaultConfigPath(), "config file path")
	root.PersistentFlags().BoolVarP(&g.verbose, "verbose", "v", false, "verbose logging")

	root.AddCommand(
		newStartCmd(g),
		newStatusCmd(g),
		newProfileCmd(g),
		newScanRootCmd(g),
		newInitCmd(g),
		newTUICmd(g),
		newUpdateCmd(g),
	)
	return root
}

func defaultConfigPath() string {
	// Follows XDG on Linux/macOS, %APPDATA% on Windows.
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "snix", "config.yaml")
	}
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, "snix", "config.yaml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "snix", "config.yaml")
}

func loadConfig(g *globalFlags) (*config.Config, error) {
	c, err := config.Load(g.configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return c, nil
}
