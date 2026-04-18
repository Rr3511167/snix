//go:build !linux && !windows

package main

import (
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/SamNet-dev/snix/config"
)

// runEngine on non-Linux platforms is a stub until the Windows backend lands.
func runEngine(ctx context.Context, out io.Writer, _ *config.Profile) error {
	fmt.Fprintf(out, "snix: platform %s has no backend yet; start is a stub\n", runtime.GOOS)
	<-ctx.Done()
	return nil
}
