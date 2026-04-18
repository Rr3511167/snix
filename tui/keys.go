package tui

import "github.com/charmbracelet/bubbles/key"

// keymap centralises every keybinding in the TUI so the help screen can
// render them without each screen redefining labels. Screen-specific
// bindings still live in their own model when they do not need to appear
// in the global help.
type keymap struct {
	// Global navigation.
	Up, Down, Left, Right key.Binding
	Tab, ShiftTab         key.Binding
	NextTab, PrevTab      key.Binding
	Quit                  key.Binding
	Help                  key.Binding
	// Screen shortcuts 1..7 select tabs directly.
	Tab1, Tab2, Tab3, Tab4, Tab5, Tab6, Tab7 key.Binding
	// Action keys.
	Enter   key.Binding
	Escape  key.Binding
	Refresh key.Binding
	Edit    key.Binding
	Save    key.Binding
	// Scan-specific.
	ScanSNI key.Binding
	ScanIP  key.Binding
	ScanAll key.Binding
	// Run-specific.
	Start key.Binding
	Stop  key.Binding
}

// defaultKeys returns the baseline keybindings; editing here updates the
// help screen automatically.
func defaultKeys() keymap {
	return keymap{
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:     key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
		Right:    key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
		Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")),
		ShiftTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev field")),
		NextTab:  key.NewBinding(key.WithKeys("]", "ctrl+right"), key.WithHelp("]", "next tab")),
		PrevTab:  key.NewBinding(key.WithKeys("[", "ctrl+left"), key.WithHelp("[", "prev tab")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Tab1:     key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "Home")),
		Tab2:     key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "Profiles")),
		Tab3:     key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "Scan")),
		Tab4:     key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "Run")),
		Tab5:     key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "Settings")),
		Tab6:     key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "Help")),
		Tab7:     key.NewBinding(key.WithKeys("7"), key.WithHelp("7", "About")),
		Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "select")),
		Escape:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Save:     key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save")),
		ScanSNI:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "scan SNIs")),
		ScanIP:   key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "scan IPs")),
		ScanAll:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "scan all")),
		Start:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start engine")),
		Stop:     key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop engine")),
	}
}
