// Package ui — interactive Bubble Tea app for git-regret.
package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Arch087146/git-regret/internal/regret"
)

const (
	reset   = "\x1b[0m"
	bold    = "\x1b[1m"
	dim     = "\x1b[2m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	magenta = "\x1b[35m"
	cyan    = "\x1b[36m"
)

func paint(code, s string) string { return code + s + reset }

var banner = []string{
	"  ╔═╗╦═╗╦═╗╦╦  ╔═╗╦═╗╔═╗╦ ╦╔═╗╦═╗",
	"  ║ ║╠╦╝╠╦╝║║  ║ ║╠╦╝║ ╦║ ║╚═╗╠╦╝",
	"  ╚═╝╩╚═╩╚═╩═╝╚═╝╩╚═╚═╝╚═╝╚═╝╩╚═",
}

const (
	stateSplash = iota
	stateMenu
	statePickRepo
	stateScanning
	stateResults
)

var scanMsgs = []string{
	"dusting off your git history…",
	"linking hacks to consequences…",
	"checking the 2am commits…",
	"matching fixes to sins…",
	"calculating regret…",
	"preparing the hall of shame…",
}

var frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type scanResult struct {
	report *regret.Report
	err    error
}

type model struct {
	root     string
	state    int
	sel      int
	input    string
	frame    int
	msgIdx   int
	reveal   int
	full     string
	report   *regret.Report
	scanErr  string
	quitting bool
	splash   int
	saved    string
}

type tickMsg struct{}
type scanDoneMsg scanResult

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return tickMsg{} })
}

// Run launches the TUI (blocking).
func Run(root string) {
	m := model{root: root, state: stateSplash}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui error:", err)
		os.Exit(1)
	}
}

func (m model) Init() tea.Cmd { return tick(time.Millisecond * 90) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
		switch m.state {
		case stateSplash:
			m.state = stateMenu
		case stateMenu:
			switch msg.String() {
			case "up", "k":
				m.sel = (m.sel + 3) % 4
			case "down", "j":
				m.sel = (m.sel + 1) % 4
			case "enter":
				return m, m.choose()
			case "q", "esc":
				m.quitting = true
				return m, tea.Quit
			}
		case statePickRepo:
			switch msg.String() {
			case "enter":
				val := strings.TrimSpace(m.input)
				m.input = ""
				if val == "" {
					m.state = stateMenu
					return m, nil
				}
				root, _ := filepath.Abs(val)
				return m, m.startScan(root)
			case "backspace":
				if len(m.input) > 0 {
					m.input = m.input[:len(m.input)-1]
				}
			case "esc":
				m.state = stateMenu
				m.input = ""
			default:
				if len(msg.String()) == 1 {
					m.input += msg.String()
				}
			}
		case stateResults:
			if m.reveal < len(m.full) {
				m.reveal = len(m.full)
				return m, tick(time.Millisecond * 60)
			}
			switch msg.String() {
			case "s":
				m.saveMD()
				return m, nil
			case "enter", "q", "esc":
				m.state = stateMenu
				m.sel, m.reveal, m.saved = 0, 0, ""
			}
		}
		return m, nil

	case tickMsg:
		var d time.Duration
		switch m.state {
		case stateSplash:
			m.splash++
			if m.splash >= len(banner)+2 {
				m.state = stateMenu
			}
			d = time.Millisecond * 90
		case stateScanning:
			m.frame++
			if m.frame%3 == 0 {
				m.msgIdx++
			}
			d = time.Millisecond * 90
		case stateResults:
			if m.reveal < len(m.full) {
				m.reveal += 3
				d = time.Millisecond * 12
			} else {
				d = time.Millisecond * 200
			}
		default:
			d = time.Millisecond * 200
		}
		return m, tick(d)

	case scanDoneMsg:
		r := scanResult(msg)
		if r.err != nil {
			m.scanErr = r.err.Error()
			m.state = stateMenu
			return m, nil
		}
		m.scanErr, m.saved = "", ""
		m.report = r.report
		m.state = stateResults
		m.reveal = 0
		m.full = m.buildFull()
		return m, tick(time.Millisecond * 12)
	}
	return m, nil
}

func (m *model) choose() tea.Cmd {
	switch m.sel {
	case 0:
		return m.startScan(m.root)
	case 1:
		m.state = statePickRepo
		m.input = ""
	case 2:
		m.quitting = true
		return tea.Quit
	case 3:
		m.quitting = true
		return tea.Quit
	}
	return nil
}

func (m *model) startScan(root string) tea.Cmd {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		m.scanErr = fmt.Sprintf("%q is not a git repository", root)
		return nil
	}
	m.scanErr, m.saved = "", ""
	m.state = stateScanning
	m.frame, m.msgIdx = 0, 0
	return tea.Batch(tick(time.Millisecond*90), func() tea.Msg {
		commits, err := regret.LoadHistory(root, "", "")
		if err != nil {
			return scanDoneMsg{err: err}
		}
		regret.LinkRegrets(commits, 90)
		rep := regret.Score(commits)
		return scanDoneMsg{report: &rep}
	})
}

func (m *model) saveMD() {
	if m.report == nil {
		m.quitting = true
		return
	}
	name := filepath.Base(m.root)
	file := filepath.Join(m.root, "REGRET.md")
	var b strings.Builder
	fmt.Fprintf(&b, "# 🔥 Regret report for %q\n\n", name)
	fmt.Fprintf(&b, "> Regret Index: **%d/100** · Grade **%s** — %s\n\n", m.report.Score, m.report.Grade, m.report.Verdict)
	fmt.Fprintf(&b, "%d commits · %d suspects · %d proven regrets\n\n", m.report.Total, m.report.Suspects, m.report.Proven)
	for _, s := range m.report.Hall {
		fmt.Fprintf(&b, "- `%s` **%s**\n  — undone %d days later by `%s`: %q\n", s.Hash, s.Msg, s.DaysLater, s.UndoneBy, s.UndoneMsg)
	}
	b.WriteString("\n<sub>generated by git-regret (Go edition)</sub>\n")
	if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
		m.saved = "✗ " + err.Error()
	} else {
		m.saved = "✔ saved → " + file
	}
}

func (m *model) buildFull() string {
	r := m.report
	lines := []string{}
	lines = append(lines, m.meter("REGRET INDEX", r.Score))
	lines = append(lines, paint(bold, fmt.Sprintf("Grade %s — %s", r.Grade, r.Verdict)))
	lines = append(lines, "")
	lines = append(lines, regret.Bar("Regret rate (hacks undone)", r.RegretRate, fmt.Sprintf("%d/%d hacks undone", r.Proven, r.Suspects)))
	lines = append(lines, regret.Bar("Late-night commits", pctOf(r.LateNight, r.Total), fmt.Sprintf("%d/%d commits", r.LateNight, r.Total)))
	lines = append(lines, regret.Bar("Weekend commits", pctOf(r.Weekend, r.Total), fmt.Sprintf("%d/%d commits", r.Weekend, r.Total)))
	if len(r.Hall) == 0 {
		lines = append(lines, "", paint(green, "✔ No proven regrets found. Either you're careful or you're lucky. Usually lucky."))
	} else {
		lines = append(lines, "", paint(bold, fmt.Sprintf("HALL OF SHAME (%d proven regrets)", len(r.Hall))))
		for _, s := range r.Hall {
			lines = append(lines,
				paint(red, "✗ ")+paint(dim, s.Hash)+" "+paint(yellow, s.Msg),
				"   "+paint(dim, fmt.Sprintf("undone %d days later by %s: %q", s.DaysLater, s.UndoneBy, s.UndoneMsg)))
		}
	}
	return strings.Join(lines, "\n")
}

func pctOf(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole)
}

func (m *model) meter(label string, score int) string {
	w := 30
	filled := score * w / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", w-filled)
	return fmt.Sprintf("%s %s %3d/100", label, paint(red, bar), score)
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder

	switch m.state {
	case stateSplash:
		b.WriteString("\n\n")
		for i := 0; i < m.splash && i < len(banner); i++ {
			b.WriteString(paint(bold+magenta, banner[i]) + "\n")
		}
	case stateMenu:
		b.WriteString("\n")
		for _, l := range banner {
			b.WriteString(paint(bold+magenta, l) + "\n")
		}
		b.WriteString("\n  " + paint(dim, "prove your hacks were regrets") + "\n\n")
		items := []string{
			fmt.Sprintf("Judge current repo (%s)", filepath.Base(m.root)),
			"Judge another repo",
			"Save report & exit",
			"Exit",
		}
		for i, it := range items {
			if i == m.sel {
				b.WriteString(paint(bold+cyan, "  ❯ ")+paint(bold, it)+"\n")
			} else {
				b.WriteString("    " + paint(dim, it) + "\n")
			}
		}
		if m.scanErr != "" {
			b.WriteString("\n  " + paint(red, "✗ "+m.scanErr) + "\n")
		}
		b.WriteString("\n  " + paint(dim, "↑/↓ select · Enter confirm · q quit") + "\n")
	case statePickRepo:
		b.WriteString("\n  path to repo:\n\n  ❯ " + m.input + "▌\n\n  " + paint(dim, "Enter confirm · Esc back") + "\n")
	case stateScanning:
		f := paint(cyan, frames[m.frame%len(frames)])
		b.WriteString("\n  " + f + " " + paint(dim, scanMsgs[m.msgIdx%len(scanMsgs)]) + "\n")
	case stateResults:
		b.WriteString("\n")
		b.WriteString(paint(bold+magenta, fmt.Sprintf("  🔥 JUDGING %q 🔥", filepath.Base(m.root))) + "\n\n")
		b.WriteString(m.full[:min(m.reveal, len(m.full))])
		if m.reveal >= len(m.full) {
			b.WriteString("\n\n  " + paint(dim, "s save REGRET.md · Enter menu · q quit"))
			if m.saved != "" {
				col := green
				if strings.HasPrefix(m.saved, "✗") {
					col = red
				}
				b.WriteString("\n  " + paint(col, m.saved))
			}
		}
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
