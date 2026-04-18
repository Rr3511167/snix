// Package tui implements the snix terminal UI: a Bubble Tea application
// that exposes every CLI capability (profile management, scanning, running
// the bypass engine, settings, help) in a single interactive interface.
package tui

import "github.com/charmbracelet/lipgloss"

// Color palette. We keep it minimal so the UI looks consistent in both
// dark and light terminals.
var (
	colorAccent  = lipgloss.Color("#7D56F4") // purple — headings, tabs
	colorAccent2 = lipgloss.Color("#5FAAFF") // blue — links, hints
	colorSuccess = lipgloss.Color("#5FD787") // green — OK outcomes
	colorWarning = lipgloss.Color("#FFB86C") // orange — caution
	colorError   = lipgloss.Color("#FF6C6C") // red — failures
	colorMuted   = lipgloss.Color("#737A82") // gray — secondary text
)

var (
	// Title banner shown at the top of every screen.
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(colorAccent).
			Padding(0, 1).
			Bold(true)

	// Tab styles.
	tabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(colorMuted)
	tabActiveStyle = tabStyle.
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(colorAccent).
			Bold(true)

	// Section headers within a screen.
	sectionStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true).
			Padding(1, 0, 0, 0)

	// Key-value entries shown on the dashboard.
	keyStyle   = lipgloss.NewStyle().Foreground(colorMuted).Width(18)
	valueStyle = lipgloss.NewStyle()

	// Status badges.
	okStyle    = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	warnStyle  = lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
	errStyle   = lipgloss.NewStyle().Foreground(colorError).Bold(true)
	mutedStyle = lipgloss.NewStyle().Foreground(colorMuted)
	hintStyle  = lipgloss.NewStyle().Foreground(colorAccent2).Italic(true)
	codeStyle  = lipgloss.NewStyle().Foreground(colorAccent2)

	// Status bar at bottom.
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#3C3C3C")).
			Padding(0, 1)

	// Panel / box used to group content.
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(0, 1)
)

// kvLine formats a key/value pair aligned for the dashboard.
func kvLine(k, v string) string {
	return keyStyle.Render(k) + valueStyle.Render(v)
}
