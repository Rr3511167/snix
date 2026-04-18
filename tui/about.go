package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// aboutModel is the static credits/version screen.
type aboutModel struct{ app *App }

func newAboutModel(a *App) aboutModel { return aboutModel{app: a} }

func (m aboutModel) Init() tea.Cmd                        { return nil }
func (m aboutModel) Update(tea.Msg) (aboutModel, tea.Cmd) { return m, nil }

func (m aboutModel) View() string {
	version := m.app.opts.Version
	if version == "" {
		version = "0.0.0-dev"
	}
	content := lipgloss.JoinVertical(lipgloss.Left,
		sectionStyle.Render("snix"),
		"",
		kvLine("Version", version),
		kvLine("License", "GPL-3.0 (matches upstream)"),
		kvLine("Upstream", "github.com/patterniha/SNI-Spoofing"),
		kvLine("Author", "@patterniha (core algorithm, Python)"),
		kvLine("This fork", "Go rewrite with cross-platform backends, TUI, scanner"),
		"",
		sectionStyle.Render("Acknowledgements"),
		"",
		"• WinDivert — by Basil Fierz (reqrypt.org/windivert.html)",
		"• NFQUEUE / netfilter — Linux kernel",
		"• Bubble Tea — charmbracelet/bubbletea",
		"• Lipgloss  — charmbracelet/lipgloss",
		"• Cobra     — spf13/cobra",
		"",
		hintStyle.Render("Support the upstream author:"),
		"  USDT (BEP20): 0x76a768B53Ca77B43086946315f0BDF21156bF424",
		"  Telegram:     @patterniha",
	)
	return panelStyle.Render(content)
}
