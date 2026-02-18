package main

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	listOuterWidth   = 42
	logContentHeight = 4
)

type state int

const (
	stateNormal state = iota
	stateConfirmKill
	stateKilling
	stateFilter
)

type sortMode int

const (
	sortByName sortMode = iota
	sortByClients
	sortByPID
	sortModeCount
)

func (s sortMode) label() string {
	switch s {
	case sortByName:
		return "name"
	case sortByClients:
		return "clients"
	case sortByPID:
		return "newest"
	}
	return ""
}

// Messages

type sessionsMsg struct {
	sessions []Session
	err      error
}

type previewMsg struct {
	content string
}

type statusClearMsg struct{}

type killOneResultMsg struct {
	name string
	err  error
}

type waitCheckMsg struct {
	names   []string
	attempt int
}

type allGoneMsg struct{}

// Commands

func fetchSessionsCmd() tea.Msg {
	sessions, err := FetchSessions()
	return sessionsMsg{sessions: sessions, err: err}
}

func fetchPreviewCmd(name string, lines, maxWidth int) tea.Cmd {
	return func() tea.Msg {
		return previewMsg{content: FetchPreview(name, lines, maxWidth)}
	}
}

func killOneCmd(name string) tea.Cmd {
	return func() tea.Msg {
		err := KillSession(name)
		return killOneResultMsg{name: name, err: err}
	}
}

func waitForGoneCmd(names []string, attempt int) tea.Cmd {
	return func() tea.Msg {
		if attempt >= 20 {
			return allGoneMsg{}
		}
		time.Sleep(200 * time.Millisecond)
		sessions, err := FetchSessions()
		if err != nil {
			return allGoneMsg{}
		}
		alive := make(map[string]bool, len(sessions))
		for _, s := range sessions {
			alive[s.Name] = true
		}
		for _, name := range names {
			if alive[name] {
				return waitCheckMsg{names: names, attempt: attempt + 1}
			}
		}
		return allGoneMsg{}
	}
}

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return statusClearMsg{}
	})
}

// Model

type Model struct {
	sessions   []Session
	cursor     int
	listOffset int
	selected   map[string]bool

	filterText   string
	sortMode     sortMode
	attachTarget string // non-empty → exec zmx attach after quit

	preview string
	state   state
	status  string

	// Kill tracking
	killQueue     []string
	killNow       string
	killDoneNames []string

	// Activity log
	logLines  []string
	logOffset int

	width  int
	height int
	err    error
}

func initialModel() Model {
	return Model{
		selected: make(map[string]bool),
	}
}

// visibleSessions returns sessions matching the current filter, sorted by sortMode.
func (m Model) visibleSessions() []Session {
	var filtered []Session
	if m.filterText == "" {
		filtered = make([]Session, len(m.sessions))
		copy(filtered, m.sessions)
	} else {
		lower := strings.ToLower(m.filterText)
		for _, s := range m.sessions {
			if strings.Contains(strings.ToLower(s.Name), lower) ||
				strings.Contains(strings.ToLower(s.StartedIn), lower) {
				filtered = append(filtered, s)
			}
		}
	}

	switch m.sortMode {
	case sortByName:
		slices.SortFunc(filtered, func(a, b Session) int {
			return cmp.Compare(a.Name, b.Name)
		})
	case sortByClients:
		slices.SortFunc(filtered, func(a, b Session) int {
			if a.Clients != b.Clients {
				return b.Clients - a.Clients
			}
			return cmp.Compare(a.Name, b.Name)
		})
	case sortByPID:
		slices.SortFunc(filtered, func(a, b Session) int {
			ai, _ := strconv.Atoi(a.PID)
			bi, _ := strconv.Atoi(b.PID)
			if ai != bi {
				return bi - ai
			}
			return cmp.Compare(a.Name, b.Name)
		})
	}

	return filtered
}

func (m *Model) addLog(line string) {
	ts := logDimStyle.Render(time.Now().Format("15:04:05"))
	m.logLines = append(m.logLines, ts+" "+line)
	maxOff := len(m.logLines) - logContentHeight
	if maxOff < 0 {
		maxOff = 0
	}
	m.logOffset = maxOff
}

// clampCursor ensures cursor and listOffset are valid for the visible list.
func (m *Model) clampCursor() {
	visible := m.visibleSessions()
	if m.cursor >= len(visible) {
		m.cursor = max(0, len(visible)-1)
	}
	if m.listOffset > m.cursor {
		m.listOffset = m.cursor
	}
}

func (m Model) Init() tea.Cmd {
	return fetchSessionsCmd
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		visible := m.visibleSessions()
		if m.state != stateKilling && m.cursor < len(visible) {
			return m, m.previewCmd()
		}

	case sessionsMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.sessions = msg.sessions
		live := make(map[string]bool, len(m.sessions))
		for _, s := range m.sessions {
			live[s.Name] = true
		}
		for name := range m.selected {
			if !live[name] {
				delete(m.selected, name)
			}
		}
		m.clampCursor()
		visible := m.visibleSessions()
		if len(visible) > 0 && m.cursor < len(visible) {
			return m, m.previewCmd()
		}
		m.preview = ""

	case previewMsg:
		m.preview = msg.content

	case killOneResultMsg:
		if msg.err != nil {
			m.addLog(confirmStyle.Render("  ✗ " + msg.name))
		} else {
			m.addLog(statusStyle.Render("  ✓ " + msg.name))
			m.killDoneNames = append(m.killDoneNames, msg.name)
		}
		m.killNow = ""
		if len(m.killQueue) > 0 {
			next := m.killQueue[0]
			m.killQueue = m.killQueue[1:]
			m.killNow = next
			m.addLog(helpStyle.Render("  ⋯ " + next))
			return m, killOneCmd(next)
		}
		if len(m.killDoneNames) > 0 {
			m.addLog(logDimStyle.Render("  Waiting for cleanup..."))
			return m, waitForGoneCmd(m.killDoneNames, 0)
		}
		return m, m.finishKill()

	case waitCheckMsg:
		return m, waitForGoneCmd(msg.names, msg.attempt)

	case allGoneMsg:
		return m, m.finishKill()

	case statusClearMsg:
		m.status = ""

	case tea.KeyPressMsg:
		if m.state == stateKilling {
			if isQuit(msg) {
				return m, tea.Quit
			}
			m.handleLogScroll(msg)
			return m, nil
		}
		if m.state == stateFilter {
			return m.handleFilterKey(msg)
		}
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *Model) finishKill() tea.Cmd {
	killed := len(m.killDoneNames)
	m.addLog(statusStyle.Render(fmt.Sprintf("  Done. Killed %d session(s).", killed)))
	m.state = stateNormal
	m.selected = make(map[string]bool)
	m.filterText = ""
	m.cursor = 0
	m.listOffset = 0
	m.killQueue = nil
	m.killDoneNames = nil
	m.killNow = ""
	return tea.Batch(fetchSessionsCmd, clearStatusAfter(3*time.Second))
}

func (m Model) previewCmd() tea.Cmd {
	visible := m.visibleSessions()
	if m.cursor >= len(visible) {
		return nil
	}
	return fetchPreviewCmd(visible[m.cursor].Name, m.mainContentHeight(), m.previewInnerWidth())
}

func (m Model) killTargets() []string {
	if len(m.selected) > 0 {
		names := make([]string, 0, len(m.selected))
		for name := range m.selected {
			names = append(names, name)
		}
		return names
	}
	visible := m.visibleSessions()
	if m.cursor < len(visible) {
		return []string{visible[m.cursor].Name}
	}
	return nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.state == stateConfirmKill {
		return m.handleConfirmKey(msg)
	}

	if isQuit(msg) {
		return m, tea.Quit
	}

	m.handleLogScroll(msg)

	// Escape or Backspace clears active filter in normal mode
	if (msg.Code == tea.KeyEscape || msg.Code == tea.KeyBackspace) && m.filterText != "" {
		m.filterText = ""
		m.cursor = 0
		m.listOffset = 0
		return m, m.previewCmd()
	}

	visible := m.visibleSessions()

	// Ctrl+A toggles select all
	if msg.Code == 'a' && msg.Mod.Contains(tea.ModCtrl) {
		m.toggleSelectAll(visible)
		return m, nil
	}

	switch msg.Code {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
			m.ensureVisible()
			return m, m.previewCmd()
		}

	case tea.KeyDown:
		if m.cursor < len(visible)-1 {
			m.cursor++
			m.ensureVisible()
			return m, m.previewCmd()
		}

	case tea.KeySpace:
		if m.cursor < len(visible) {
			name := visible[m.cursor].Name
			if m.selected[name] {
				delete(m.selected, name)
			} else {
				m.selected[name] = true
			}
		}

	case tea.KeyEnter:
		if m.cursor < len(visible) {
			m.attachTarget = visible[m.cursor].Name
			return m, tea.Quit
		}

	default:
		if msg.Text != "" {
			switch msg.Text {
			case "k":
				targets := m.killTargets()
				if len(targets) > 0 {
					m.state = stateConfirmKill
				}
			case "c":
				if m.cursor < len(visible) {
					name := visible[m.cursor].Name
					text := fmt.Sprintf("zmx attach %s", name)
					if err := CopyToClipboard(text); err != nil {
						m.status = fmt.Sprintf("Copy failed: %v", err)
						m.addLog(confirmStyle.Render(fmt.Sprintf("  ✗ Copy failed: %v", err)))
					} else {
						m.status = "Copied!"
						m.addLog(statusStyle.Render(fmt.Sprintf("  Copied: %s", text)))
					}
					return m, clearStatusAfter(2 * time.Second)
				}
			case "r":
				return m, fetchSessionsCmd
			case "/":
				m.state = stateFilter
			case "s":
				m.sortMode = (m.sortMode + 1) % sortModeCount
				m.cursor = 0
				m.listOffset = 0
				return m, m.previewCmd()
			}
		}
	}

	return m, nil
}

func (m *Model) toggleSelectAll(visible []Session) {
	if len(visible) == 0 {
		return
	}
	allSelected := true
	for _, s := range visible {
		if !m.selected[s.Name] {
			allSelected = false
			break
		}
	}
	if allSelected {
		for _, s := range visible {
			delete(m.selected, s.Name)
		}
	} else {
		for _, s := range visible {
			m.selected[s.Name] = true
		}
	}
}

func (m Model) handleFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if isQuit(msg) {
		return m, tea.Quit
	}

	switch msg.Code {
	case tea.KeyEscape:
		m.filterText = ""
		m.state = stateNormal
		m.cursor = 0
		m.listOffset = 0
		return m, m.previewCmd()

	case tea.KeyEnter:
		m.state = stateNormal
		m.clampCursor()
		return m, m.previewCmd()

	case tea.KeyBackspace:
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
			m.cursor = 0
			m.listOffset = 0
		} else {
			// Backspace on empty filter exits filter mode
			m.state = stateNormal
			return m, m.previewCmd()
		}

	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
			m.ensureVisible()
			return m, m.previewCmd()
		}

	case tea.KeyDown:
		visible := m.visibleSessions()
		if m.cursor < len(visible)-1 {
			m.cursor++
			m.ensureVisible()
			return m, m.previewCmd()
		}

	default:
		if msg.Text != "" {
			m.filterText += msg.Text
			m.cursor = 0
			m.listOffset = 0
		}
	}

	return m, nil
}

func (m Model) handleConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if isQuit(msg) {
		return m, tea.Quit
	}
	if msg.Code == tea.KeyEscape || msg.Code == tea.KeyBackspace {
		m.state = stateNormal
		return m, nil
	}
	if isRune(msg, "y") {
		targets := m.killTargets()
		total := len(targets)
		m.state = stateKilling
		m.killDoneNames = nil

		m.addLog(titleStyle.Render(fmt.Sprintf("Killing %d session(s)...", total)))

		first := targets[0]
		m.killQueue = targets[1:]
		m.killNow = first
		m.addLog(helpStyle.Render("  ⋯ " + first))
		return m, killOneCmd(first)
	}
	if isRune(msg, "n") {
		m.state = stateNormal
	}
	return m, nil
}

func (m *Model) handleLogScroll(msg tea.KeyPressMsg) {
	if !isRune(msg, "[") && !isRune(msg, "]") {
		return
	}
	maxOffset := len(m.logLines) - logContentHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if isRune(msg, "[") && m.logOffset > 0 {
		m.logOffset--
	}
	if isRune(msg, "]") && m.logOffset < maxOffset {
		m.logOffset++
	}
}

func (m *Model) ensureVisible() {
	h := m.mainContentHeight()
	if h <= 0 {
		return
	}
	if m.cursor < m.listOffset {
		m.listOffset = m.cursor
	}
	if m.cursor >= m.listOffset+h {
		m.listOffset = m.cursor - h + 1
	}
}

// Layout

func (m Model) mainContentHeight() int {
	h := m.height - logContentHeight - 5
	if h < 1 {
		h = 1
	}
	return h
}

func (m Model) listInnerWidth() int {
	return listOuterWidth - 2
}

func (m Model) previewOuterWidth() int {
	w := m.width - listOuterWidth
	if w < 10 {
		w = 10
	}
	return w
}

func (m Model) previewInnerWidth() int {
	return m.previewOuterWidth() - 2
}

// View

func (m Model) View() tea.View {
	if m.err != nil {
		v := tea.NewView(fmt.Sprintf("\n  Error: %v\n\n  Is zmx installed and in your PATH?\n", m.err))
		v.AltScreen = true
		return v
	}
	if m.width == 0 {
		v := tea.NewView("  Loading...")
		v.AltScreen = true
		return v
	}

	visible := m.visibleSessions()
	ch := m.mainContentHeight()

	// --- List pane ---
	listContent := m.renderList(visible, ch)
	listContent = clampLines(listContent, ch)

	selCount := len(m.selected)
	listTitle := fmt.Sprintf(" zmx sessions (%d) ", len(visible))
	if len(visible) != len(m.sessions) {
		listTitle = fmt.Sprintf(" zmx (%d/%d) ", len(visible), len(m.sessions))
	}
	if selCount > 0 {
		listTitle += fmt.Sprintf("[%d sel] ", selCount)
	}
	listTitle += fmt.Sprintf("sort:%s ", m.sortMode.label())

	listPane := listBorderStyle.
		Width(listOuterWidth).
		Height(ch + 2).
		Render(listContent)
	listPane = replaceTopBorder(listPane, buildTopBorder(listTitle, listOuterWidth))

	// --- Preview pane ---
	previewContent := clampLines(m.preview, ch)
	previewTitle := " Preview "
	if m.cursor < len(visible) {
		previewTitle = fmt.Sprintf(" %s ", truncate(visible[m.cursor].Name, m.previewInnerWidth()-4))
	}
	pow := m.previewOuterWidth()

	previewPane := previewBorderStyle.
		Width(pow).
		Height(ch + 2).
		Render(previewContent)
	previewPane = replaceTopBorder(previewPane, buildTopBorder(previewTitle, pow))

	body := lipgloss.JoinHorizontal(lipgloss.Top, listPane, previewPane)

	// --- Log pane ---
	logContent := m.renderLog()

	logPane := logBorderStyle.
		Width(m.width).
		Height(logContentHeight + 2).
		Render(logContent)

	logTitle := " Activity Log "
	if m.state == stateKilling {
		logTitle = " Killing... "
	}
	logPane = replaceTopBorder(logPane, buildTopBorder(logTitle, m.width))

	// --- Help bar ---
	help := m.renderHelp()

	full := lipgloss.JoinVertical(lipgloss.Left, body, logPane, help)
	v := tea.NewView(clampLines(full, m.height))
	v.AltScreen = true
	return v
}

func (m Model) renderLog() string {
	if len(m.logLines) == 0 {
		return logDimStyle.Render("  No activity yet.")
	}

	end := m.logOffset + logContentHeight
	if end > len(m.logLines) {
		end = len(m.logLines)
	}
	start := m.logOffset
	if start < 0 {
		start = 0
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		b.WriteString(m.logLines[i])
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m Model) renderList(visible []Session, maxRows int) string {
	if len(visible) == 0 {
		if m.filterText != "" {
			return normalStyle.Render("  No matches. Esc to clear filter.")
		}
		return normalStyle.Render("  No sessions found. Press r to refresh.")
	}

	lw := m.listInnerWidth()
	var b strings.Builder

	end := m.listOffset + maxRows
	if end > len(visible) {
		end = len(visible)
	}

	for i := m.listOffset; i < end; i++ {
		s := visible[i]
		isCursor := i == m.cursor
		isSelected := m.selected[s.Name]

		var indicator string
		switch {
		case isCursor && isSelected:
			indicator = selectedStyle.Render("▸●")
		case isCursor:
			indicator = selectedStyle.Render("▸ ")
		case isSelected:
			indicator = selectedStyle.Render(" ●")
		default:
			indicator = "  "
		}

		var clientInd string
		if s.Clients > 0 {
			clientInd = activeClientStyle.Render(fmt.Sprintf("●%d", s.Clients))
		} else {
			clientInd = inactiveClientStyle.Render("○0")
		}

		nameWidth := lw - 6
		if nameWidth < 10 {
			nameWidth = 10
		}
		name := truncate(s.Name, nameWidth)

		style := normalStyle
		if isCursor || isSelected {
			style = selectedStyle
		}

		row := fmt.Sprintf("%s%s %s", indicator, style.Render(padRight(name, nameWidth)), clientInd)
		b.WriteString(row)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) renderHelp() string {
	if m.state == stateKilling {
		return helpStyle.Render(" [] scroll log  ") + helpKeyStyle.Render("q") + helpStyle.Render(" quit")
	}

	if m.state == stateFilter {
		cursor := "█"
		return helpStyle.Render(" /") + helpKeyStyle.Render(m.filterText) + helpStyle.Render(cursor+"  Enter accept | Esc clear")
	}

	if m.state == stateConfirmKill {
		targets := m.killTargets()
		if len(targets) == 1 {
			return confirmStyle.Render(fmt.Sprintf(" Kill %s? y/n ", targets[0]))
		}
		return confirmStyle.Render(fmt.Sprintf(" Kill %d sessions? y/n ", len(targets)))
	}

	parts := []string{
		helpKeyStyle.Render("↑↓") + helpStyle.Render(" nav"),
		helpKeyStyle.Render("space") + helpStyle.Render(" sel"),
		helpKeyStyle.Render("^a") + helpStyle.Render(" all"),
		helpKeyStyle.Render("enter") + helpStyle.Render(" attach"),
		helpKeyStyle.Render("k") + helpStyle.Render(" kill"),
		helpKeyStyle.Render("c") + helpStyle.Render(" copy cmd"),
		helpKeyStyle.Render("s") + helpStyle.Render(" sort"),
	}
	if m.filterText != "" {
		parts = append(parts, helpKeyStyle.Render("esc")+helpStyle.Render(" clear"))
	} else {
		parts = append(parts, helpKeyStyle.Render("/")+helpStyle.Render(" filter"))
	}
	parts = append(parts,
		helpKeyStyle.Render("[]")+helpStyle.Render(" log"),
		helpKeyStyle.Render("q")+helpStyle.Render(" quit"),
	)

	help := " " + strings.Join(parts, "  ")

	if m.status != "" {
		help += "  " + statusStyle.Render(m.status)
	}

	return help
}

// Border helpers

var borderCharStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

func buildTopBorder(title string, outerWidth int) string {
	maxTitleVW := outerWidth - 4
	if maxTitleVW < 1 {
		maxTitleVW = 1
	}
	if lipgloss.Width(title) > maxTitleVW {
		title = truncate(title, maxTitleVW)
	}
	styledTitle := titleStyle.Render(title)
	titleVW := lipgloss.Width(styledTitle)

	fill := outerWidth - 3 - titleVW
	if fill < 0 {
		fill = 0
	}

	return borderCharStyle.Render("╭─") + styledTitle + borderCharStyle.Render(strings.Repeat("─", fill)+"╮")
}

func replaceTopBorder(pane, newTop string) string {
	_, rest, ok := strings.Cut(pane, "\n")
	if !ok {
		return pane
	}
	return newTop + "\n" + rest
}

func clampLines(s string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n")
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
