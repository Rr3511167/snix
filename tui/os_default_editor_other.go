//go:build !windows

package tui

// defaultEditor returns the OS-appropriate fallback when $EDITOR is unset.
func defaultEditor() string { return "vi" }
