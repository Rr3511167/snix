// snix is the cross-platform SNI-spoofing DPI-bypass CLI.
//
// It is a Go rewrite of patterniha/SNI-Spoofing, adding profile management,
// cross-platform support, a TUI, and auto-configuration.
package main

import (
	"fmt"
	"os"
)

// version is set via -ldflags at release time.
var version = "0.0.0-dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
