package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// homeModel is a static dashboard showing the user what snix is, what's
// loaded, and where to go next. It is the landing screen.
type homeModel struct {
	app *App
}

func newHomeModel(a *App) homeModel { return homeModel{app: a} }

func (m homeModel) Init() tea.Cmd { return nil }

func (m homeModel) Update(msg tea.Msg) (homeModel, tea.Cmd) { return m, nil }

func (m homeModel) View() string {
	cfg := m.app.cfg
	var right string
	if cfg == nil {
		right = errStyle.Render("No config loaded") + "\n\n" +
			"Path tried: " + codeStyle.Render(m.app.opts.ConfigPath) + "\n"
		if m.app.cfgErr != nil {
			right += mutedStyle.Render(m.app.cfgErr.Error()) + "\n"
		}
		right += "\n" + hintStyle.Render("Run `snix init` from a shell to create a starter config,\n"+
			"or open the Profiles tab to view the path it expects.")
	} else {
		active, _ := cfg.Lookup(cfg.Active)
		rows := []string{
			kvLine("Config path", codeStyle.Render(m.app.opts.ConfigPath)),
			kvLine("Profiles", fmt.Sprintf("%d", len(cfg.Profiles))),
			kvLine("Active profile", okStyle.Render(cfg.Active)),
		}
		if active != nil {
			rows = append(rows,
				kvLine("  listen", active.Listen),
				kvLine("  connect", fmt.Sprintf("%s:%d", active.Connect.Host, active.Connect.Port)),
				kvLine("  strategy", string(active.Spoof.Strategy)),
				kvLine("  sni pool", fmt.Sprintf("%d entries", len(active.EffectiveSNIPool()))),
			)
			if active.Spoof.RandomizeTiming || active.Spoof.RandomizePadding {
				rows = append(rows, kvLine("  randomize", "timing+padding active"))
			}
			if len(active.Spoof.StrategyRotation) > 0 {
				rows = append(rows, kvLine("  rotation", fmt.Sprintf("%v", active.Spoof.StrategyRotation)))
			}
		}
		right = strings.Join(rows, "\n")
	}

	explainer := sectionStyle.Render("What is snix?") + "\n" +
		"snix is a client-side DPI-bypass proxy. It sits between your VPN client\n" +
		"and the internet, and fools censorship middleboxes (like Iran's DPI) into\n" +
		"thinking each TLS connection is headed to a permitted domain.\n\n" +
		"How: during the TCP 3-way handshake, snix injects a fake TLS ClientHello\n" +
		"with a spoofed SNI (e.g. " + codeStyle.Render("auth.vercel.com") + "). The DPI accepts the\n" +
		"connection; the real server drops the fake packet (bad sequence number)\n" +
		"and the real TLS handshake proceeds unobstructed.\n"

	nextSteps := sectionStyle.Render("Get started") + "\n" +
		"  " + codeStyle.Render("3") + "  run " + hintStyle.Render("Scan") + " to find working SNIs for your ISP\n" +
		"  " + codeStyle.Render("2") + "  view " + hintStyle.Render("Profiles") + " to see loaded config + switch active\n" +
		"  " + codeStyle.Render("4") + "  open " + hintStyle.Render("Run") + " to start the bypass engine\n" +
		"  " + codeStyle.Render("5") + "  tweak " + hintStyle.Render("Settings") + " for anti-fingerprint knobs\n" +
		"  " + codeStyle.Render("?") + "  or " + codeStyle.Render("6") + "  for full " + hintStyle.Render("Help") + "\n"

	rightPanel := panelStyle.Render(sectionStyle.Render("Status") + "\n" + right)
	left := lipgloss.JoinVertical(lipgloss.Left, explainer, nextSteps)
	w := m.app.width
	if w < 40 {
		w = 40
	}
	leftW := w * 3 / 5
	rightW := w - leftW - 4
	if rightW < 20 {
		rightW = 20
	}

	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(leftW).Render(left),
		lipgloss.NewStyle().Width(rightW).Render(rightPanel),
	)
}
