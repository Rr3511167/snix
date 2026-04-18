package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/SamNet-dev/snix/tui"
)

// newTUICmd creates the `snix tui` command: a full-screen Bubble Tea UI
// that exposes every other CLI subcommand (profile, scan, start) with
// live feedback, progress bars, and a log-tailing Run screen.
func newTUICmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive TUI (Bubble Tea)",
		Long: `snix tui opens a terminal UI with tabs for Home, Profiles, Scan, Run,
Settings, Help, and About. Every CLI capability is available there, plus
live scan progress and engine log tailing. Does not require Administrator
on its own — only the engine subprocess started from the Run tab does.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			exe, err := os.Executable()
			if err != nil {
				exe = "snix"
			}
			return tui.Run(tui.Options{
				Version:    version,
				ExePath:    exe,
				ConfigPath: g.configPath,
			})
		},
	}
}
