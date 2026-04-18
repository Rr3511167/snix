package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SamNet-dev/snix/core/bypass"
)

// settingsModel lets the user toggle anti-fingerprinting knobs on the
// active profile without leaving the TUI. Every change is persisted to
// config.yaml immediately so the next `snix start` picks it up.
type settingsModel struct {
	app    *App
	cursor int
}

func newSettingsModel(a *App) settingsModel { return settingsModel{app: a} }

// settingItem is one row in the Settings UI.
type settingItem struct {
	label string
	value string
	help  string
	// toggle mutates the active profile and returns a display string for
	// its new value.
	toggle func()
}

// items returns the live list of settings bound to the active profile.
// Called on every View so changes in the config (via external edit) show up.
func (m *settingsModel) items() []settingItem {
	cfg := m.app.cfg
	if cfg == nil {
		return nil
	}
	p, ok := cfg.Lookup(cfg.Active)
	if !ok {
		return nil
	}
	return []settingItem{
		{
			label:  "Randomize timing",
			value:  boolStr(p.Spoof.RandomizeTiming),
			help:   "Jitters the fake-packet inject delay in [MinDelay, MaxDelay] so\ndelay isn't a DPI fingerprint.",
			toggle: func() { p.Spoof.RandomizeTiming = !p.Spoof.RandomizeTiming },
		},
		{
			label:  "Randomize padding",
			value:  boolStr(p.Spoof.RandomizePadding),
			help:   "Varies fake ClientHello size in [517, 517+MaxExtraPad]. Breaks the\nfixed-517-byte pattern used by upstream pydivert.",
			toggle: func() { p.Spoof.RandomizePadding = !p.Spoof.RandomizePadding },
		},
		{
			label: "Strategy rotation",
			value: rotationLabel(p.Spoof.StrategyRotation),
			help:  "Cycles bypass strategies per flow. Common rotation pairs wrong_seq\nwith wrong_checksum so the DPI can't learn a single packet shape.",
			toggle: func() {
				if len(p.Spoof.StrategyRotation) == 0 {
					p.Spoof.StrategyRotation = []bypass.Name{bypass.NameWrongSeq, bypass.NameWrongChecksum}
				} else {
					p.Spoof.StrategyRotation = nil
				}
			},
		},
		{
			label:  "IP ID delta range",
			value:  fmt.Sprintf("%d", maxOne(p.Spoof.IPIDDeltaRange)),
			help:   "Random delta applied to the fake packet's IP identification field.\n1 = upstream behaviour; higher = more variance. Press +/- to adjust.",
			toggle: func() {}, // handled by numeric keys below
		},
		{
			label: "SNI selection",
			value: fallback(p.Spoof.SNISelection, "random"),
			help:  "random (default): pick a new SNI per flow uniformly at random.\nround_robin: deterministic cycle through sni_pool.",
			toggle: func() {
				if p.Spoof.SNISelection == "round_robin" {
					p.Spoof.SNISelection = "random"
				} else {
					p.Spoof.SNISelection = "round_robin"
				}
			},
		},
	}
}

func (m settingsModel) Init() tea.Cmd { return nil }

func (m settingsModel) Update(msg tea.Msg) (settingsModel, tea.Cmd) {
	if m.app.cfg == nil {
		return m, nil
	}
	its := m.items()
	if len(its) == 0 {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(its)-1 {
				m.cursor++
			}
		case " ", "enter":
			its[m.cursor].toggle()
			return m, m.persist()
		case "+", "=":
			if its[m.cursor].label == "IP ID delta range" {
				p, _ := m.app.cfg.Lookup(m.app.cfg.Active)
				p.Spoof.IPIDDeltaRange = maxOne(p.Spoof.IPIDDeltaRange) + 4
				return m, m.persist()
			}
		case "-", "_":
			if its[m.cursor].label == "IP ID delta range" {
				p, _ := m.app.cfg.Lookup(m.app.cfg.Active)
				if p.Spoof.IPIDDeltaRange > 1 {
					p.Spoof.IPIDDeltaRange -= 4
					if p.Spoof.IPIDDeltaRange < 1 {
						p.Spoof.IPIDDeltaRange = 1
					}
					return m, m.persist()
				}
			}
		}
	}
	return m, nil
}

func (m settingsModel) View() string {
	if m.app.cfg == nil {
		return warnStyle.Render("No config loaded. Run `snix init` first.")
	}
	its := m.items()
	if len(its) == 0 {
		return warnStyle.Render("Active profile not found in config.")
	}
	title := sectionStyle.Render("Settings  —  anti-fingerprinting knobs for " +
		okStyle.Render(m.app.cfg.Active))
	hints := "\n" + mutedStyle.Render("↑↓ move   space/⏎ toggle   +/- adjust   changes save immediately")

	var rows []string
	for i, it := range its {
		cursor := "  "
		if i == m.cursor {
			cursor = hintStyle.Render("▸ ")
		}
		rows = append(rows, cursor+lipgloss.NewStyle().Bold(true).Render(it.label)+
			mutedStyle.Render("  → ")+it.value)
	}

	// Help for the current row.
	helpBody := sectionStyle.Render("What this does") + "\n" + its[m.cursor].help

	w := m.app.width
	if w < 40 {
		w = 80
	}
	leftW := w/2 - 2
	rightW := w - leftW - 4
	left := panelStyle.Width(leftW).Render(strings.Join(rows, "\n"))
	right := panelStyle.Width(rightW).Render(helpBody)
	return lipgloss.JoinVertical(lipgloss.Left, title, hints, "",
		lipgloss.JoinHorizontal(lipgloss.Top, left, right))
}

func boolStr(b bool) string {
	if b {
		return okStyle.Render("on")
	}
	return mutedStyle.Render("off")
}

func rotationLabel(r []bypass.Name) string {
	if len(r) == 0 {
		return mutedStyle.Render("off  (single strategy)")
	}
	parts := make([]string, 0, len(r))
	for _, n := range r {
		parts = append(parts, string(n))
	}
	return okStyle.Render("on  ") + mutedStyle.Render("("+strings.Join(parts, ", ")+")")
}

// persist writes the current in-memory config back to disk and asks the app
// to reload so everything reflects the change.
func (m settingsModel) persist() tea.Cmd {
	return func() tea.Msg {
		if err := writeYAML(m.app.opts.ConfigPath, m.app.cfg); err != nil {
			return statusMsg(fmt.Sprintf("write failed: %v", err))
		}
		return tea.Sequence(
			setStatus("settings saved"),
			func() tea.Msg { return configReloadMsg{} },
		)()
	}
}
