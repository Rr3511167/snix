package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SamNet-dev/snix/config"
)

// tab identifies a top-level screen.
type tab int

const (
	tabHome tab = iota
	tabProfiles
	tabScan
	tabRun
	tabSettings
	tabHelp
	tabAbout
)

var tabTitles = map[tab]string{
	tabHome:     "Home",
	tabProfiles: "Profiles",
	tabScan:     "Scan",
	tabRun:      "Run",
	tabSettings: "Settings",
	tabHelp:     "Help",
	tabAbout:    "About",
}

var tabOrder = []tab{tabHome, tabProfiles, tabScan, tabRun, tabSettings, tabHelp, tabAbout}

// Options configure the TUI launch.
type Options struct {
	// Version string surfaced on the About screen.
	Version string
	// ExePath is the path to the snix binary the Run screen will spawn
	// for start/stop.
	ExePath string
	// ConfigPath is the path the TUI loads/edits.
	ConfigPath string
}

// App is the Bubble Tea root model.
type App struct {
	opts Options
	keys keymap

	width, height int
	active        tab

	// Shared state.
	cfg    *config.Config
	cfgErr error

	// Per-screen models.
	home     homeModel
	profiles profilesModel
	scan     scanModel
	run      runModel
	settings settingsModel
	help     helpModel
	about    aboutModel

	// Transient UI.
	statusLine string
}

// Run starts the Bubble Tea program. Blocking; returns when the user quits.
func Run(opts Options) error {
	app := newApp(opts)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

func newApp(opts Options) App {
	cfg, cfgErr := config.Load(opts.ConfigPath)
	a := App{
		opts:   opts,
		keys:   defaultKeys(),
		active: tabHome,
		cfg:    cfg,
		cfgErr: cfgErr,
	}
	a.home = newHomeModel(&a)
	a.profiles = newProfilesModel(&a)
	a.scan = newScanModel(&a)
	a.run = newRunModel(&a)
	a.settings = newSettingsModel(&a)
	a.help = newHelpModel(&a)
	a.about = newAboutModel(&a)
	return a
}

// Init starts any background tick processes the screens need.
func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.scan.Init(),
		a.run.Init(),
	)
}

// Update is the Bubble Tea reducer.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
	case tea.KeyMsg:
		// Global shortcuts first.
		k := msg.String()
		switch {
		case k == "q" && a.active != tabRun:
			// `q` quits — except on Run which uses `x` for stop.
			// If the engine subprocess is still running, kill it first so
			// it doesn't orphan and keep kernel-level WinDivert / NFQUEUE
			// state alive with no owner.
			return a, a.shutdown()
		case k == "ctrl+c":
			return a, a.shutdown()
		case k == "?":
			a.active = tabHelp
			return a, nil
		case k == "1":
			a.active = tabHome
			return a, nil
		case k == "2":
			a.active = tabProfiles
			return a, nil
		case k == "3":
			a.active = tabScan
			return a, nil
		case k == "4":
			a.active = tabRun
			return a, nil
		case k == "5":
			a.active = tabSettings
			return a, nil
		case k == "6":
			a.active = tabHelp
			return a, nil
		case k == "7":
			a.active = tabAbout
			return a, nil
		case k == "]" || k == "ctrl+right":
			a.active = nextTab(a.active)
			return a, nil
		case k == "[" || k == "ctrl+left":
			a.active = prevTab(a.active)
			return a, nil
		}
	case configReloadMsg:
		cfg, err := config.Load(a.opts.ConfigPath)
		a.cfg = cfg
		a.cfgErr = err
		a.statusLine = fmt.Sprintf("config reloaded: %s", a.opts.ConfigPath)
		// Let screens rebuild any derived state.
		a.profiles = newProfilesModel(&a)
	case statusMsg:
		a.statusLine = string(msg)
	}

	// Route message to the active screen's Update.
	var cmd tea.Cmd
	switch a.active {
	case tabHome:
		a.home, cmd = a.home.Update(msg)
	case tabProfiles:
		a.profiles, cmd = a.profiles.Update(msg)
	case tabScan:
		a.scan, cmd = a.scan.Update(msg)
	case tabRun:
		a.run, cmd = a.run.Update(msg)
	case tabSettings:
		a.settings, cmd = a.settings.Update(msg)
	case tabHelp:
		a.help, cmd = a.help.Update(msg)
	case tabAbout:
		a.about, cmd = a.about.Update(msg)
	}
	return a, cmd
}

// View renders the full app: title bar, tabs, active screen, status line.
func (a App) View() string {
	if a.width == 0 {
		return "loading…"
	}
	title := titleStyle.Render(" snix  •  DPI-bypass TUI ")
	tabsRow := renderTabs(a.active)
	header := lipgloss.JoinVertical(lipgloss.Left, title, tabsRow)

	body := ""
	switch a.active {
	case tabHome:
		body = a.home.View()
	case tabProfiles:
		body = a.profiles.View()
	case tabScan:
		body = a.scan.View()
	case tabRun:
		body = a.run.View()
	case tabSettings:
		body = a.settings.View()
	case tabHelp:
		body = a.help.View()
	case tabAbout:
		body = a.about.View()
	}

	status := a.statusLine
	if status == "" {
		status = "q: quit   ?: help   1-7: tabs   ]/[: cycle tabs"
	}
	statusBar := statusBarStyle.Width(a.width).Render(status)

	return lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", statusBar)
}

func renderTabs(active tab) string {
	var parts []string
	for _, t := range tabOrder {
		label := fmt.Sprintf("%d %s", int(t)+1, tabTitles[t])
		if t == active {
			parts = append(parts, tabActiveStyle.Render(label))
		} else {
			parts = append(parts, tabStyle.Render(label))
		}
	}
	return strings.Join(parts, "")
}

func nextTab(t tab) tab {
	for i, x := range tabOrder {
		if x == t {
			return tabOrder[(i+1)%len(tabOrder)]
		}
	}
	return tabHome
}

func prevTab(t tab) tab {
	for i, x := range tabOrder {
		if x == t {
			return tabOrder[(i-1+len(tabOrder))%len(tabOrder)]
		}
	}
	return tabHome
}

// shutdown kills the engine subprocess if one is running, then quits the
// Bubble Tea program. Idempotent — safe to call when no engine is running.
// Without this, quitting the TUI while the engine is alive orphans the
// subprocess (on Windows it also leaks the WinDivert kernel handle).
func (a App) shutdown() tea.Cmd {
	if a.run.isRunning() {
		// Run stop() synchronously, then quit.
		return tea.Sequence(a.run.stop(), tea.Quit)
	}
	return tea.Quit
}

// configReloadMsg is sent after external config changes (e.g. after saving
// scan results) so the tabs refresh.
type configReloadMsg struct{}

// statusMsg updates the bottom status line from a screen.
type statusMsg string

// setStatus returns a tea.Cmd that emits a statusMsg.
func setStatus(format string, args ...any) tea.Cmd {
	return func() tea.Msg { return statusMsg(fmt.Sprintf(format, args...)) }
}
