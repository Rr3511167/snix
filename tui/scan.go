package tui

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SamNet-dev/snix/scanner"
)

// scanMode selects which probe set is currently running.
type scanMode int

const (
	scanIdle scanMode = iota
	scanModeSNI
	scanModeIP
	scanModeAll
)

// scanModel is the Scan tab: drives the scanner package and renders live
// progress as each probe completes.
type scanModel struct {
	app *App

	mode    scanMode
	running bool
	cancel  context.CancelFunc
	target  textinput.Model // SNI probe target IP
	started time.Time

	// Results streamed from probes.
	snis     []scanner.Result
	ips      []scanner.IPResult
	sniTotal int
	sniDone  int
	ipTotal  int
	ipDone   int

	// Progress bars.
	sniBar progress.Model
	ipBar  progress.Model
}

func newScanModel(a *App) scanModel {
	t := textinput.New()
	t.Placeholder = "1.1.1.1"
	t.SetValue("1.1.1.1")
	t.CharLimit = 40
	t.Width = 20

	return scanModel{
		app:    a,
		target: t,
		sniBar: progress.New(progress.WithDefaultGradient(), progress.WithWidth(40)),
		ipBar:  progress.New(progress.WithDefaultGradient(), progress.WithWidth(40)),
	}
}

func (m scanModel) Init() tea.Cmd { return nil }

func (m scanModel) Update(msg tea.Msg) (scanModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.sniBar.Width = max(20, msg.Width-60)
		m.ipBar.Width = max(20, msg.Width-60)

	case tea.KeyMsg:
		// Editing the target field?
		if m.target.Focused() {
			if msg.String() == "esc" || msg.String() == "enter" {
				m.target.Blur()
				return m, nil
			}
			m.target, cmd = m.target.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "t":
			m.target.Focus()
			return m, textinput.Blink
		case "s":
			if !m.running {
				return m, m.startSNI()
			}
		case "i":
			if !m.running {
				return m, m.startIP()
			}
		case "a":
			if !m.running {
				return m, m.startAll()
			}
		case "x":
			if m.running && m.cancel != nil {
				m.cancel()
			}
		case "e":
			if !m.running && len(m.snis) > 0 {
				return m, m.saveToProfile()
			}
		}

	case sniResultMsg:
		m.snis = append(m.snis, scanner.Result(msg))
		m.sniDone++
	case ipResultMsg:
		m.ips = append(m.ips, scanner.IPResult(msg))
		m.ipDone++
	case scanDoneMsg:
		m.running = false
		m.mode = scanIdle
	}

	// Let the target input absorb any unconsumed text keys.
	if m.target.Focused() {
		m.target, cmd = m.target.Update(msg)
	}
	return m, cmd
}

func (m scanModel) View() string {
	header := sectionStyle.Render("Scan  —  discover SNIs and CDN IPs that work on this network")
	controls := "\n" +
		codeStyle.Render("s") + " scan SNIs   " +
		codeStyle.Render("i") + " scan IPs   " +
		codeStyle.Render("a") + " scan all   " +
		codeStyle.Render("t") + " edit target   " +
		codeStyle.Render("x") + " stop   " +
		codeStyle.Render("e") + " save top results → active profile"

	targetRow := "SNI target: " + m.target.View()
	if !m.target.Focused() {
		targetRow = "SNI target: " + codeStyle.Render(m.target.Value())
	}

	status := mutedStyle.Render("idle — pick an action above")
	if m.running {
		status = hintStyle.Render(fmt.Sprintf("running %s  (%s)",
			modeName(m.mode), time.Since(m.started).Round(time.Millisecond)))
	} else if m.sniDone+m.ipDone > 0 {
		status = okStyle.Render("done")
	}

	sniPanel := m.renderSNIPanel()
	ipPanel := m.renderIPPanel()

	w := m.app.width
	if w < 40 {
		w = 40
	}
	colW := (w - 6) / 2
	if colW < 30 {
		colW = 30
	}
	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		panelStyle.Width(colW).Render(sniPanel),
		panelStyle.Width(colW).Render(ipPanel),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		controls,
		targetRow,
		status,
		"",
		cols,
	)
}

func (m scanModel) renderSNIPanel() string {
	title := sectionStyle.Render("SNI probes")
	if m.sniTotal == 0 {
		return title + "\n" + mutedStyle.Render("press s or a to start")
	}
	pct := 0.0
	if m.sniTotal > 0 {
		pct = float64(m.sniDone) / float64(m.sniTotal)
	}
	bar := m.sniBar.ViewAs(pct)
	counts := map[scanner.Outcome]int{}
	for _, r := range m.snis {
		counts[r.Outcome]++
	}
	summary := fmt.Sprintf("%d/%d   ok=%s  reset=%s  timeout=%s  err=%s",
		m.sniDone, m.sniTotal,
		okStyle.Render(fmt.Sprintf("%d", counts[scanner.OutcomeOK])),
		errStyle.Render(fmt.Sprintf("%d", counts[scanner.OutcomeDPIReset])),
		warnStyle.Render(fmt.Sprintf("%d", counts[scanner.OutcomeTimeout])),
		mutedStyle.Render(fmt.Sprintf("%d", counts[scanner.OutcomeTLSError]+counts[scanner.OutcomeTCPRefused])),
	)

	// Top-10 working SNIs.
	top := scanner.RankSNIs(m.snis)
	if len(top) > 10 {
		top = top[:10]
	}
	var lines []string
	for i, r := range top {
		lines = append(lines, fmt.Sprintf("%2d. %-35s %s",
			i+1, truncate(r.SNI, 35), r.Handshake.Round(time.Millisecond)))
	}
	if len(lines) == 0 {
		lines = []string{mutedStyle.Render("no successful handshakes yet")}
	}
	return strings.Join([]string{title, bar, summary, "", strings.Join(lines, "\n")}, "\n")
}

func (m scanModel) renderIPPanel() string {
	title := sectionStyle.Render("IP probes")
	if m.ipTotal == 0 {
		return title + "\n" + mutedStyle.Render("press i or a to start")
	}
	pct := 0.0
	if m.ipTotal > 0 {
		pct = float64(m.ipDone) / float64(m.ipTotal)
	}
	bar := m.ipBar.ViewAs(pct)
	reach := 0
	for _, r := range m.ips {
		if r.Reachable {
			reach++
		}
	}
	summary := fmt.Sprintf("%d/%d   reachable=%d", m.ipDone, m.ipTotal, reach)
	top := scanner.RankIPs(m.ips)
	if len(top) > 10 {
		top = top[:10]
	}
	var lines []string
	for i, r := range top {
		lines = append(lines, fmt.Sprintf("%2d. %-18s %s",
			i+1, r.IP, r.RTT.Round(time.Millisecond)))
	}
	if len(lines) == 0 {
		lines = []string{mutedStyle.Render("no reachable IPs yet")}
	}
	return strings.Join([]string{title, bar, summary, "", strings.Join(lines, "\n")}, "\n")
}

// Scan commands --------------------------------------------------------

func (m *scanModel) startSNI() tea.Cmd {
	tgt, err := netip.ParseAddr(strings.TrimSpace(m.target.Value()))
	if err != nil {
		return setStatus("invalid target: %v", err)
	}
	m.reset()
	m.running = true
	m.mode = scanModeSNI
	m.started = time.Now()
	m.snis = nil
	m.sniDone = 0
	m.sniTotal = len(scanner.DefaultSNICandidates)

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	return func() tea.Msg {
		results := scanner.ProbeSNIs(ctx, scanner.DefaultSNICandidates, scanner.ProbeConfig{
			TargetIP:         tgt,
			TargetPort:       443,
			ConnectTimeout:   4 * time.Second,
			HandshakeTimeout: 4 * time.Second,
			Concurrency:      16,
		}, nil)
		// Push results in one chunk after completion. (For truly per-probe
		// streaming in Bubble Tea we'd need a channel-backed cmd; batch is
		// fine for now and keeps the code small.)
		cmds := make([]tea.Cmd, 0, len(results)+1)
		for _, r := range results {
			r := r
			cmds = append(cmds, func() tea.Msg { return sniResultMsg(r) })
		}
		cmds = append(cmds, func() tea.Msg { return scanDoneMsg{} })
		return tea.Sequence(cmds...)()
	}
}

func (m *scanModel) startIP() tea.Cmd {
	m.reset()
	m.running = true
	m.mode = scanModeIP
	m.started = time.Now()
	m.ips = nil
	m.ipDone = 0
	m.ipTotal = len(scanner.DefaultCloudflareIPs)

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	return func() tea.Msg {
		results := scanner.ProbeIPs(ctx, scanner.DefaultCloudflareIPs, 443,
			2*time.Second, 16, nil)
		cmds := make([]tea.Cmd, 0, len(results)+1)
		for _, r := range results {
			r := r
			cmds = append(cmds, func() tea.Msg { return ipResultMsg(r) })
		}
		cmds = append(cmds, func() tea.Msg { return scanDoneMsg{} })
		return tea.Sequence(cmds...)()
	}
}

func (m *scanModel) startAll() tea.Cmd {
	return tea.Batch(m.startSNI(), m.startIP())
}

func (m *scanModel) reset() {
	m.snis = nil
	m.ips = nil
	m.sniDone, m.sniTotal = 0, 0
	m.ipDone, m.ipTotal = 0, 0
}

// saveToProfile updates the active profile's sni_pool with the top-ranked
// SNIs and the connect.host + fallback_ips with the best IPs.
func (m *scanModel) saveToProfile() tea.Cmd {
	cfg := m.app.cfg
	if cfg == nil {
		return setStatus("no config loaded")
	}
	p, ok := cfg.Lookup(cfg.Active)
	if !ok {
		return setStatus("active profile %q not found", cfg.Active)
	}
	top := scanner.RankSNIs(m.snis)
	if len(top) == 0 {
		return setStatus("no successful SNI probes to save")
	}
	if len(top) > 10 {
		top = top[:10]
	}
	pool := make([]string, 0, len(top))
	for _, r := range top {
		pool = append(pool, r.SNI)
	}
	p.Spoof.SNIPool = pool
	p.Spoof.SNI = ""

	// If we also have IP results, refresh connect.host + fallback_ips.
	if ips := scanner.RankIPs(m.ips); len(ips) > 0 {
		p.Connect.Host = ips[0].IP.String()
		fb := make([]string, 0, len(ips))
		for _, r := range ips[1:] {
			fb = append(fb, r.IP.String())
		}
		if len(fb) > 5 {
			fb = fb[:5]
		}
		p.Connect.FallbackIPs = fb
	}

	return func() tea.Msg {
		if err := writeYAML(m.app.opts.ConfigPath, cfg); err != nil {
			return statusMsg(fmt.Sprintf("write failed: %v", err))
		}
		return tea.Sequence(
			setStatus("saved top %d SNIs to profile %q", len(pool), cfg.Active),
			func() tea.Msg { return configReloadMsg{} },
		)()
	}
}

// Messages -------------------------------------------------------------

type sniResultMsg scanner.Result
type ipResultMsg scanner.IPResult
type scanDoneMsg struct{}

// Helpers --------------------------------------------------------------

func modeName(m scanMode) string {
	switch m {
	case scanModeSNI:
		return "SNI scan"
	case scanModeIP:
		return "IP scan"
	case scanModeAll:
		return "full scan"
	}
	return "idle"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return s[:n-1] + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
