//go:build windows

package tui

// defaultEditor returns notepad on Windows when $EDITOR is unset. notepad.exe
// lives on %PATH% for every Windows install, so this is always reachable.
func defaultEditor() string { return "notepad.exe" }
