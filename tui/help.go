package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// helpModel is a scrollable long-form help document. It also lists every
// keybinding so users discover everything without reading source.
type helpModel struct {
	app *App
	vp  viewport.Model
}

func newHelpModel(a *App) helpModel {
	m := helpModel{app: a}
	m.vp = viewport.New(80, 20)
	m.vp.SetContent(helpContent())
	return m
}

func (m helpModel) Init() tea.Cmd { return nil }

func (m helpModel) Update(msg tea.Msg) (helpModel, tea.Cmd) {
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.vp.Width = sz.Width - 4
		m.vp.Height = sz.Height - 8
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m helpModel) View() string {
	return sectionStyle.Render("Help  (↑/↓ or j/k to scroll, q to quit)") + "\n" + m.vp.View()
}

// helpContent returns the full help document. Kept as a function so future
// versions can interpolate version info or conditional notes.
func helpContent() string {
	s := func(ss ...string) string { return strings.Join(ss, "") }
	return s(
		"# snix — client-side DPI bypass via SNI spoofing\n\n",

		"## The one-paragraph explanation\n",
		"When your browser opens an HTTPS connection, the very first TLS packet\n",
		"— the ClientHello — contains the server name in plain text (the SNI\n",
		"field). Nation-state DPI systems read this field to decide whether to\n",
		"block your connection. snix exploits a quirk: most DPI devices don't\n",
		"reassemble TCP streams, they inspect individual packets. We inject a\n",
		"fake ClientHello with an ALLOWED SNI immediately after the TCP\n",
		"handshake. The DPI sees \"allowed domain, let this flow through.\" Our\n",
		"fake packet has a deliberately wrong TCP sequence number (or a\n",
		"corrupted checksum), so the real server silently drops it. Meanwhile,\n",
		"the real TLS handshake proceeds normally — and because the DPI has\n",
		"already whitelisted the flow, it passes.\n\n",

		"## Prerequisites\n",
		"- Linux or Windows (macOS / Android planned).\n",
		"- On Linux: root (iptables + NFQUEUE).\n",
		"- On Windows: Administrator + WinDivert.dll + WinDivert64.sys\n",
		"  next to snix.exe. See docs/windows.md.\n",
		"- A TLS-based proxy server reachable on port 443 (Xray, v2ray, etc.).\n\n",

		"## Screens\n",
		"1. Home     — overview + config status.\n",
		"2. Profiles — list configured profiles, switch active, view details.\n",
		"3. Scan     — test SNIs + IPs against the live network; paste results.\n",
		"4. Run      — start/stop the bypass engine as a subprocess.\n",
		"5. Settings — toggle randomization knobs in the active profile.\n",
		"6. Help     — this screen.\n",
		"7. About    — version + credits.\n\n",

		"## Navigation keys (global)\n",
		"  1–7           jump to tab by number\n",
		"  ]  /  [       cycle tabs right / left\n",
		"  ↑↓  /  j k    scroll\n",
		"  ?             help (you are here)\n",
		"  q             quit (Ctrl+C everywhere)\n\n",

		"## Scan screen\n",
		"  s  probe SNIs against the target IP\n",
		"  i  probe the default Cloudflare IP pool\n",
		"  a  run both in parallel\n",
		"  r  re-run last scan\n",
		"  e  save top results into the active profile (writes config.yaml)\n\n",

		"## Run screen\n",
		"  s  start the snix engine in a subprocess (opens platform backend)\n",
		"  x  stop the engine (SIGINT → graceful shutdown)\n",
		"  r  restart\n",
		"  The Run screen tails the engine's stdout live.\n\n",

		"## Profiles screen\n",
		"  ↑↓  move cursor\n",
		"  ⏎   mark as active (writes config.yaml immediately)\n",
		"  e   open this profile in $EDITOR (exits TUI temporarily)\n\n",

		"## Settings screen\n",
		"Toggles apply to the active profile only and write back to disk.\n",
		"  Randomize timing   — jitters inject delay in [MinDelay, MaxDelay]\n",
		"  Randomize padding  — varies fake ClientHello size\n",
		"  Strategy rotation  — alternates wrong_seq and wrong_checksum\n",
		"  IP ID delta range  — spreads the identity field increment\n",
		"  SNI selection      — random (default) vs round_robin\n\n",

		"## Why these matter\n",
		"DPI vendors fingerprint censorship-bypass tools by the shapes of their\n",
		"packets: fixed 517-byte ClientHello, fixed 1 ms timing between ACK\n",
		"and fake, fixed +1 IP identity field, predictable SNI rotation. Every\n",
		"knob in Settings breaks one invariant. Turn them all on in production.\n\n",

		"## CLI parity\n",
		"Everything the TUI does is also available as a CLI subcommand; the\n",
		"TUI simply wraps them and adds live feedback. List:\n",
		"  snix start    — run the bypass engine (same as Run tab)\n",
		"  snix scan sni — same as Scan tab's 's' action\n",
		"  snix scan ip  — same as Scan tab's 'i' action\n",
		"  snix scan all — same as Scan tab's 'a' action\n",
		"  snix profile list / switch — same as Profiles tab\n",
		"  snix init    — creates a starter config\n",
		"  snix status  — prints profile summary\n",
		"  snix tui     — launch this TUI\n\n",

		"## Troubleshooting\n",
		"  \"No config loaded\" on Home — run `snix init` first, or pass -c PATH.\n",
		"  Run tab fails immediately on Windows — needs Administrator. The\n",
		"    TUI itself does not need admin, only the engine subprocess.\n",
		"  Scan reports \"0 OK\" — try a different --sni-target; default 1.1.1.1\n",
		"    works because Cloudflare DNS accepts any SNI with a default cert.\n",
		"  Real traffic stalls during bypass — see the Help entry on strategy\n",
		"    choices. wrong_checksum on its own can overlap with real data; we\n",
		"    always pair it with out-of-window seq in this release.\n\n",

		"## More info\n",
		"  README       — quick start\n",
		"  docs/windows.md — Windows setup details\n",
		"  PLAN.md      — the full project roadmap\n",
		"  Upstream     — github.com/patterniha/SNI-Spoofing (Python original)\n",
	)
}
