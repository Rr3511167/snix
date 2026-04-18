package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"

	"github.com/SamNet-dev/snix/config"
)

// profilesModel is the Profiles tab. Left: list of profiles with a cursor.
// Right: details of the selected profile.
type profilesModel struct {
	app    *App
	cursor int
}

func newProfilesModel(a *App) profilesModel { return profilesModel{app: a} }

func (m profilesModel) Init() tea.Cmd { return nil }

func (m profilesModel) Update(msg tea.Msg) (profilesModel, tea.Cmd) {
	cfg := m.app.cfg
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if cfg == nil {
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(cfg.Profiles)-1 {
				m.cursor++
			}
		case "enter":
			return m, m.setActive(cfg.Profiles[m.cursor].Name)
		case "e":
			return m, m.editExternal()
		case "r":
			return m, func() tea.Msg { return configReloadMsg{} }
		}
	}
	return m, nil
}

func (m profilesModel) View() string {
	cfg := m.app.cfg
	if cfg == nil {
		return warnStyle.Render("No config loaded. Run `snix init` to create one.")
	}
	// Left list.
	var lines []string
	for i, p := range cfg.Profiles {
		var marker string
		if p.Name == cfg.Active {
			marker = okStyle.Render("● ")
		} else {
			marker = mutedStyle.Render("○ ")
		}
		cursor := "  "
		if i == m.cursor {
			cursor = hintStyle.Render("▸ ")
		}
		lines = append(lines, cursor+marker+p.Name)
	}
	list := sectionStyle.Render("Profiles") + "\n" + strings.Join(lines, "\n") + "\n\n" +
		mutedStyle.Render("⏎ make active   e edit in $EDITOR   r reload")

	// Right details.
	detail := ""
	if m.cursor < len(cfg.Profiles) {
		p := cfg.Profiles[m.cursor]
		detail = renderProfileDetail(&p)
	}

	w := m.app.width
	if w < 40 {
		w = 40
	}
	leftW := 34
	rightW := w - leftW - 2
	if rightW < 20 {
		rightW = 20
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		panelStyle.Width(leftW).Render(list),
		panelStyle.Width(rightW).Render(detail),
	)
}

func renderProfileDetail(p *config.Profile) string {
	lines := []string{
		sectionStyle.Render("Profile: " + p.Name),
		"",
		kvLine("Listen", p.Listen),
		kvLine("Connect", fmt.Sprintf("%s:%d", p.Connect.Host, p.Connect.Port)),
	}
	if len(p.Connect.FallbackIPs) > 0 {
		lines = append(lines, kvLine("Fallback IPs", strings.Join(p.Connect.FallbackIPs, ", ")))
	}
	lines = append(lines,
		"",
		sectionStyle.Render("Bypass"),
		kvLine("Strategy", string(p.Spoof.Strategy)),
	)
	if len(p.Spoof.StrategyRotation) > 0 {
		rot := make([]string, 0, len(p.Spoof.StrategyRotation))
		for _, s := range p.Spoof.StrategyRotation {
			rot = append(rot, string(s))
		}
		lines = append(lines, kvLine("Rotation", strings.Join(rot, ", ")))
	}
	lines = append(lines, kvLine("SNI selection", fallback(p.Spoof.SNISelection, "random")))
	pool := p.EffectiveSNIPool()
	lines = append(lines, kvLine("SNI pool", fmt.Sprintf("%d entries", len(pool))))
	for _, sni := range pool {
		lines = append(lines, "    "+codeStyle.Render(sni))
	}
	lines = append(lines,
		"",
		sectionStyle.Render("Randomization"),
		kvLine("Timing", onOff(p.Spoof.RandomizeTiming, p.Spoof.MinDelay.String(), p.Spoof.MaxDelay.String())),
		kvLine("Padding", onOff(p.Spoof.RandomizePadding,
			fmt.Sprintf("%d", p.Spoof.MinExtraPad),
			fmt.Sprintf("%d bytes", p.Spoof.MaxExtraPad))),
		kvLine("IP ID delta range", fmt.Sprintf("%d", maxOne(p.Spoof.IPIDDeltaRange))),
	)
	return strings.Join(lines, "\n")
}

func onOff(on bool, a, b string) string {
	if on {
		return okStyle.Render("on") + "  " + mutedStyle.Render(fmt.Sprintf("(%s..%s)", a, b))
	}
	return mutedStyle.Render("off")
}

func fallback(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func maxOne(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

// setActive writes the new active profile to disk, then asks the app to reload.
func (m profilesModel) setActive(name string) tea.Cmd {
	cfg := m.app.cfg
	if cfg == nil {
		return setStatus("no config loaded")
	}
	cfg.Active = name
	return func() tea.Msg {
		if err := writeYAML(m.app.opts.ConfigPath, cfg); err != nil {
			return statusMsg(fmt.Sprintf("write failed: %v", err))
		}
		return tea.Sequence(
			setStatus("active profile set to %q", name),
			func() tea.Msg { return configReloadMsg{} },
		)()
	}
}

// editExternal opens the config file in $EDITOR and returns control to the
// TUI after the editor exits. On Windows we fall back to notepad.
func (m profilesModel) editExternal() tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = defaultEditor()
	}
	cmd := exec.Command(editor, m.app.opts.ConfigPath)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return statusMsg(fmt.Sprintf("editor exited: %v", err))
		}
		return configReloadMsg{}
	})
}

// writeYAML serialises cfg to path atomically (tmp file + rename).
func writeYAML(path string, cfg *config.Config) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	enc.Close()
	f.Close()
	return os.Rename(tmp, path)
}
