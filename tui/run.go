package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// runModel launches `snix start` as a subprocess and tails its output into
// a viewport.
//
// Bubble Tea passes models by value through Update; a sync.Mutex inside the
// model would be copied (and zero-init'd) on every tick. So we stash all
// mutex-protected subprocess state in a pointer-held runState and only hold
// the view-local fields directly. The pointer lets multiple copies of
// runModel share the same underlying lock.
type runModel struct {
	app *App

	st   *runState // shared; pointer so copies alias
	logs []string
	vp   viewport.Model
}

// runState holds the subprocess state protected by mu. Separated out so
// copying a runModel by value doesn't duplicate a zero Mutex.
type runState struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	started time.Time
	running bool
	msgCh   chan tea.Msg
}

const maxLogLines = 2000

func newRunModel(a *App) runModel {
	return runModel{
		app: a,
		st:  &runState{},
		vp:  viewport.New(80, 20),
	}
}

func (m runModel) Init() tea.Cmd { return nil }

func (m runModel) Update(msg tea.Msg) (runModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.vp.Width = msg.Width - 4
		m.vp.Height = msg.Height - 12

	case tea.KeyMsg:
		switch msg.String() {
		case "s":
			if !m.isRunning() {
				return m, m.start()
			}
		case "x":
			if m.isRunning() {
				return m, m.stop()
			}
		case "r":
			cmds := []tea.Cmd{}
			if m.isRunning() {
				cmds = append(cmds, m.stop())
			}
			cmds = append(cmds, m.start())
			return m, tea.Sequence(cmds...)
		case "c":
			m.logs = nil
			m.vp.SetContent("")
		}

	case runLogMsg:
		m.appendLog(string(msg))
		if ch := m.stChan(); ch != nil {
			return m, pumpCmd(ch)
		}
	case runExitMsg:
		m.st.mu.Lock()
		m.st.running = false
		m.st.msgCh = nil
		m.st.mu.Unlock()
		line := "[engine exited cleanly]"
		if msg.err != nil {
			line = fmt.Sprintf("[engine exited: %v]", msg.err)
		}
		m.appendLog(errStyle.Render(line))
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *runModel) appendLog(line string) {
	m.logs = append(m.logs, line)
	if len(m.logs) > maxLogLines {
		m.logs = m.logs[len(m.logs)-maxLogLines:]
	}
	m.vp.SetContent(strings.Join(m.logs, "\n"))
	m.vp.GotoBottom()
}

func (m runModel) View() string {
	header := sectionStyle.Render("Run  —  start/stop the snix engine and watch logs")
	controls := "\n" +
		codeStyle.Render("s") + " start   " +
		codeStyle.Render("x") + " stop   " +
		codeStyle.Render("r") + " restart   " +
		codeStyle.Render("c") + " clear logs"

	state := mutedStyle.Render("stopped")
	if m.isRunning() {
		state = okStyle.Render("running") + "  " +
			mutedStyle.Render(fmt.Sprintf("(pid=%d, uptime=%s)",
				m.pid(), time.Since(m.startTime()).Round(time.Second)))
	}

	var warnings string
	if runtime.GOOS == "windows" {
		warnings = "\n" + hintStyle.Render(
			"Note: on Windows the engine needs Administrator privilege. If the TUI was\n"+
				"launched non-elevated, pressing s will print the missing-privilege error\n"+
				"into the log panel; relaunch this TUI from an elevated shell to start.")
	}

	status := sectionStyle.Render("Status") + "\n" + state + warnings

	w := m.app.width
	if w < 20 {
		w = 20
	}
	m.vp.Width = w - 4

	logPanel := panelStyle.Width(w - 2).Render(
		sectionStyle.Render("Engine output") + "\n" + m.vp.View())

	return lipgloss.JoinVertical(lipgloss.Left,
		header, controls, status, "", logPanel,
	)
}

func (m runModel) isRunning() bool {
	m.st.mu.Lock()
	defer m.st.mu.Unlock()
	return m.st.running
}

func (m runModel) startTime() time.Time {
	m.st.mu.Lock()
	defer m.st.mu.Unlock()
	return m.st.started
}

func (m runModel) stChan() chan tea.Msg {
	m.st.mu.Lock()
	defer m.st.mu.Unlock()
	return m.st.msgCh
}

func (m runModel) pid() int {
	m.st.mu.Lock()
	defer m.st.mu.Unlock()
	if m.st.cmd == nil || m.st.cmd.Process == nil {
		return 0
	}
	return m.st.cmd.Process.Pid
}

// start spawns `snix start -c CONFIG` and returns a tea.Cmd that pumps the
// subprocess's output into the model's log viewport.
func (m runModel) start() tea.Cmd {
	m.st.mu.Lock()
	if m.st.running {
		m.st.mu.Unlock()
		return setStatus("engine already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, m.app.opts.ExePath, "start", "-c", m.app.opts.ConfigPath)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		cancel()
		m.st.mu.Unlock()
		return setStatus("failed to start engine: %v", err)
	}
	m.st.cmd = cmd
	m.st.cancel = cancel
	m.st.running = true
	m.st.started = time.Now()
	ch := make(chan tea.Msg, 64)
	m.st.msgCh = ch
	m.st.mu.Unlock()

	// Owner goroutine: scans stdout+stderr into ch, waits for exit, emits
	// runExitMsg, closes ch. Calling cmd.Wait() exactly once.
	go func() {
		var wg sync.WaitGroup
		feed := func(r io.ReadCloser) {
			defer wg.Done()
			sc := bufio.NewScanner(r)
			sc.Buffer(make([]byte, 64*1024), 1024*1024)
			for sc.Scan() {
				select {
				case ch <- runLogMsg(sc.Text()):
				case <-ctx.Done():
					return
				}
			}
		}
		wg.Add(2)
		go feed(stdout)
		go feed(stderr)
		err := cmd.Wait()
		wg.Wait()
		select {
		case ch <- runExitMsg{err: err}:
		default:
		}
		close(ch)
	}()

	return pumpCmd(ch)
}

// pumpCmd blocks on ch and returns each message. When ch closes, emits a
// sentinel runExitMsg with a nil err so the model clears its state.
func pumpCmd(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return runExitMsg{err: nil}
		}
		return msg
	}
}

// stop sends SIGINT (Unix) or Kill (Windows) to the engine subprocess.
func (m runModel) stop() tea.Cmd {
	return func() tea.Msg {
		m.st.mu.Lock()
		defer m.st.mu.Unlock()
		if m.st.cmd == nil || m.st.cmd.Process == nil {
			return statusMsg("engine not running")
		}
		p := m.st.cmd.Process
		if runtime.GOOS == "windows" {
			_ = p.Kill()
		} else {
			_ = p.Signal(syscall.SIGINT)
		}
		return statusMsg("engine stopping")
	}
}

type runLogMsg string
type runExitMsg struct{ err error }
