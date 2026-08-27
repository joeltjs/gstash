package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/joeltjs/gstash/internal/git"
	"github.com/joeltjs/gstash/internal/web"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	selStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("61"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	branchStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	inferredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("179"))
	unknownStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	borderStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238"))
	barStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)

type mode int

const (
	modeList mode = iota
	modeConfirmDrop
	modeInputBranch
	modeInputSave
)

type Model struct {
	dir           string
	curBranch     string
	entries       []git.Entry
	filtered      []int
	cursor        int
	filterAll     bool
	preview       viewport.Model
	prevRef       string
	status        string
	opErr         bool
	mode          mode
	ti            textinput.Model
	width         int
	height        int
	loaded        bool
	dashboardAddr string
	showHelp      bool
}

func New(dir string) Model {
	ti := textinput.New()
	ti.Placeholder = "new branch name"
	ti.Prompt = "> "
	return Model{
		dir:     dir,
		preview: viewport.New(60, 20),
		ti:      ti,
	}
}

func (m *Model) openSaveInput() {
	m.mode = modeInputSave
	m.ti.Placeholder = "stash message (empty = wip)"
	m.ti.SetValue("")
	m.ti.Focus()
}

type listLoadedMsg struct {
	entries []git.Entry
	err     error
}

type previewMsg struct {
	ref     string
	content string
}

type opDoneMsg struct {
	out  string
	err  error
	drop bool
}

type dashboardMsg struct {
	addr string
	err  error
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadList(m.dir), loadBranch(m.dir))
}

func loadList(dir string) tea.Cmd {
	return func() tea.Msg {
		e, err := git.StashList(dir)
		return listLoadedMsg{entries: e, err: err}
	}
}

func loadBranch(dir string) tea.Cmd {
	return func() tea.Msg {
		b, _ := git.CurrentBranch(dir)
		return branchMsg(b)
	}
}

type branchMsg string

func (m Model) selectedRef() string {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return ""
	}
	return m.entries[m.filtered[m.cursor]].Ref
}

func (m Model) fetchPreviewCmd(ref string) tea.Cmd {
	return func() tea.Msg {
		content, err := git.Show(m.dir, ref)
		if err != nil {
			content = err.Error()
		}
		return previewMsg{ref: ref, content: content}
	}
}

func runOp(dir string, fn func(string, string) (string, error), ref string, drop bool) tea.Cmd {
	return func() tea.Msg {
		out, err := fn(dir, ref)
		return opDoneMsg{out: out, err: err, drop: drop}
	}
}

func runOpCmd(dir string, f func(string) (string, error)) tea.Cmd {
	return func() tea.Msg {
		out, err := f(dir)
		return opDoneMsg{out: out, err: err}
	}
}

func (m *Model) refilter() {
	m.filtered = m.filtered[:0]
	for i, e := range m.entries {
		if m.filterAll || e.Source == git.SourceUnknown || e.Branch == m.curBranch {
			m.filtered = append(m.filtered, i)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil

	case branchMsg:
		m.curBranch = string(msg)
		m.refilter()
		return m, nil

	case listLoadedMsg:
		m.loaded = true
		if msg.err != nil {
			m.status = msg.err.Error()
			m.opErr = true
			return m, nil
		}
		m.entries = msg.entries
		m.refilter()
		cmds := []tea.Cmd{}
		if ref := m.selectedRef(); ref != "" && ref != m.prevRef {
			m.prevRef = ref
			cmds = append(cmds, m.fetchPreviewCmd(ref))
		}
		return m, tea.Batch(cmds...)

	case previewMsg:
		if msg.ref == m.prevRef {
			m.preview.SetContent(msg.content)
			m.preview.GotoTop()
		}
		return m, nil

	case opDoneMsg:
		if msg.err != nil {
			m.status = truncateOneLine(msg.err.Error(), maxInt(m.width-4, 20))
			m.opErr = true
			return m, loadList(m.dir)
		}
		m.status = strings.TrimSpace(msg.out)
		if m.status == "" {
			m.status = "done"
		}
		m.opErr = false
		if msg.drop {
			m.mode = modeList
		}
		return m, loadList(m.dir)

	case dashboardMsg:
		if msg.err != nil {
			m.status = truncateOneLine(msg.err.Error(), maxInt(m.width-4, 20))
			m.opErr = true
			return m, nil
		}
		m.dashboardAddr = msg.addr
		m.status = fmt.Sprintf("Dashboard terbuka di http://%s (server tetap jalan di background)", msg.addr)
		m.opErr = false
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeConfirmDrop:
			switch msg.String() {
			case "y", "Y":
				ref := m.selectedRef()
				m.mode = modeList
				if ref == "" {
					return m, nil
				}
				return m, runOp(m.dir, git.Drop, ref, true)
			case "n", "N", "esc":
				m.mode = modeList
				return m, nil
			}
			return m, nil

		case modeInputBranch, modeInputSave:
			var cmd tea.Cmd
			if msg.String() == "esc" {
				m.mode = modeList
				return m, nil
			}
			if msg.String() == "enter" {
				val := strings.TrimSpace(m.ti.Value())
				kind := m.mode
				m.mode = modeList
				m.ti.Blur()
				if kind == modeInputSave {
					if val == "" {
						val = "wip"
					}
					return m, runOpCmd(m.dir, func(d string) (string, error) {
						return git.Save(d, val, false)
					})
				}
				ref := m.selectedRef()
				if val == "" || ref == "" {
					return m, nil
				}
				return m, runOpCmd(m.dir, func(d string) (string, error) {
					return git.BranchFromStash(d, ref, val)
				})
			}
			m.ti, cmd = m.ti.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "esc":
			if m.showHelp {
				m.showHelp = false
				return m, nil
			}
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				return m, m.fetchPreviewCmd(m.selectedRef())
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				return m, m.fetchPreviewCmd(m.selectedRef())
			}
		case "tab":
			m.filterAll = !m.filterAll
			m.refilter()
			m.status = ""
			if ref := m.selectedRef(); ref != "" && ref != m.prevRef {
				m.prevRef = ref
				return m, m.fetchPreviewCmd(ref)
			}
			return m, nil
		case "r":
			return m, loadList(m.dir)
		case "v":
			if m.dashboardAddr != "" {
				m.status = fmt.Sprintf("Dashboard sudah jalan di http://%s", m.dashboardAddr)
				m.opErr = false
				return m, nil
			}
			return m, openDashboard(m.dir)
		case "a":
			if ref := m.selectedRef(); ref != "" {
				return m, runOp(m.dir, git.Apply, ref, false)
			}
		case "p":
			if ref := m.selectedRef(); ref != "" {
				return m, runOp(m.dir, git.Pop, ref, true)
			}
		case "d":
			if ref := m.selectedRef(); ref != "" {
				m.mode = modeConfirmDrop
				return m, nil
			}
		case "b":
			if ref := m.selectedRef(); ref != "" {
				m.mode = modeInputBranch
				m.ti.Placeholder = "new branch name"
				m.ti.SetValue(strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("stash-%s", strings.TrimPrefix(ref, "stash@{")), "/", "-"), ":", ""))
				m.ti.Focus()
				return m, textinput.Blink
			}
		case "n":
			m.openSaveInput()
			return m, textinput.Blink
		case "pgup", "ctrl+u":
			m.preview.HalfPageUp()
			return m, nil
		case "pgdown", "ctrl+d":
			m.preview.HalfPageDown()
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) resize() {
	leftW := clamp(m.width/3, 36, 56)
	h := clamp(m.height-6, 6, 100)
	m.preview.Width = m.width - leftW - 4
	m.preview.Height = h
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncateOneLine(s string, n int) string {
	s = strings.SplitN(s, "\n", 2)[0]
	if n > 0 && len(s) > n {
		return s[:n]
	}
	return s
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	var b strings.Builder

	head := titleStyle.Render("gstash") +
		dimStyle.Render("  "+filepath.Base(m.dir)) +
		dimStyle.Render("  branch: ") + branchStyle.Render(orDash(m.curBranch)) +
		dimStyle.Render(fmt.Sprintf("  %s  [%d/%d]", filterLabel(m.filterAll), len(m.filtered), len(m.entries)))
	b.WriteString(head + "\n")

	leftW := clamp(m.width/3, 50, 76)
	lines := m.renderTable(leftW)
	left := borderStyle.Width(leftW).Render(strings.Join(lines, "\n"))

	rightTitle := m.prevRef
	rightBody := m.preview.View()
	if m.showHelp {
		rightTitle = "help  (? or esc to close)"
		lines := strings.Split(gstashHelpText, "\n")
		h := clamp(m.height-6, 6, 100)
		if len(lines) > h {
			lines = lines[:h]
		}
		rightBody = strings.Join(lines, "\n")
	}
	right := borderStyle.Width(m.width-leftW-4).Render(rightTitle+"\n"+rightBody)

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	b.WriteString(body + "\n")

	help := "↑/↓ move · tab filter · n save-new · a apply · p pop · d drop · b branch · v view · ? help · r refresh · q quit"
	if m.mode == modeConfirmDrop {
		help = errStyle.Render(fmt.Sprintf("drop %s? y/n", m.selectedRef()))
	} else if m.mode == modeInputBranch {
		help = "branch name: " + m.ti.View() + "  (enter create · esc cancel)"
	} else if m.mode == modeInputSave {
		help = "new stash message: " + m.ti.View() + "  (enter save · esc cancel)"
	}
	b.WriteString(barStyle.Render(help))
	if m.status != "" && m.mode == modeList {
		st := okStyle.Render(truncateOneLine(m.status, m.width))
		if m.opErr {
			st = errStyle.Render(truncateOneLine(m.status, m.width))
		}
		b.WriteString("\n" + st)
	}
	return b.String()
}

type column struct {
	title string
	width int
}

func (m Model) renderTable(totalWidth int) []string {
	cols := []column{
		{"ID", 4},
		{"BRANCH", 14},
		{"AGE", 12},
	}
	msgW := totalWidth - 4 - (len(cols)-1)*3
	for i := range cols {
		msgW -= cols[i].width
	}
	if msgW < 8 {
		msgW = 8
	}
	cols = append(cols, column{"MESSAGE", msgW})

	header := ""
	for _, c := range cols {
		header += dimStyle.Render(padRight(c.title, c.width+3))
	}
	header = strings.TrimRight(header, " ")
	out := []string{header, dimStyle.Render(strings.Repeat("─", totalWidth-2))}

	if !m.loaded {
		out = append(out, dimStyle.Render("loading..."))
		return out
	}
	if len(m.filtered) == 0 {
		out = append(out, dimStyle.Render("no stashes"+filterHint(m.filterAll)))
		return out
	}

	for i, idx := range m.filtered {
		e := m.entries[idx]
		style := branchStyle

		idCell := fmt.Sprintf("#%-3d", e.Index)
		brCell := truncatePlain(e.Branch, cols[1].width)
		ageCell := truncatePlain(e.Age, cols[2].width)
		msgCell := truncatePlain(e.Message, cols[3].width)

		row := idCell + "  " +
			style.Render(padRight(brCell, cols[1].width)) + "  " +
			dimStyle.Render(padRight(ageCell, cols[2].width)) + "  " +
			msgCell

		if i == m.cursor {
			row = selStyle.Render(padTo(row, totalWidth-2))
		}
		out = append(out, row)
	}
	return out
}

func padRight(s string, n int) string {
	w := len([]rune(s))
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func truncatePlain(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		if n <= 1 {
			return string(r[:n])
		}
		return string(r[:n-1]) + "…"
	}
	return s
}

func padTo(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func filterLabel(all bool) string {
	if all {
		return "all branches"
	}
	return "current branch"
}

func filterHint(all bool) string {
	if all {
		return " for this repo"
	}
	return " on this branch (tab for all)"
}

func Run(dir string) error {
	p := tea.NewProgram(New(dir), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

const gstashHelpText = `NAVIGASI
  ↑/k, ↓/j        select a stash
  tab             filter: current branch ↔ all branches
  pgup/pgdn       scroll preview diff

OPERASI STASH
  n               buat stash baru dari perubahan sekarang
  a               apply the stash without deleting it
  p               pop: apply and delete (the Accept button in the dashboard)
  d               drop: delete permanently (the Reject button in the dashboard)
  b               buat branch baru dari stash terpilih

LAINNYA
  v               buka dashboard web di browser
  r               refresh daftar
  ?               show/hide this help
  q / ctrl+c      keluar

CATATAN
  · Semua operasi memakai index asli git (stash@{n}),
    sehingga selalu akurat menargetkan stash yang benar.`

func openDashboard(dir string) tea.Cmd {
	return func() tea.Msg {
		port, err := web.ResolvePort(dir)
		if err != nil {
			return dashboardMsg{err: err}
		}
		addr, err := web.Serve(dir, port)
		return dashboardMsg{addr: addr, err: err}
	}
}
