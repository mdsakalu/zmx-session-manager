package tui

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-runewidth"
	"github.com/mdsakalu/zmx-session-manager/internal/zmx"
)

const (
	listMaxOuterWidth = 56
	logContentHeight  = 4
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
	sortByMemory
	sortByUptime
	sortModeCount
)

func (s sortMode) label() string {
	switch s {
	case sortByName:
		return "name"
	case sortByClients:
		return "clients"
	case sortByPID:
		return "pid"
	case sortByMemory:
		return "memory"
	case sortByUptime:
		return "uptime"
	}
	return ""
}

// Messages

type sessionsMsg struct {
	sessions []Session
	err      error
}

type Session = zmx.Session

type previewMsg struct {
	name    string
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

type processInfoMsg struct {
	info map[string]zmx.ProcessInfo
}

type allGoneMsg struct{}

// Commands

func fetchSessionsCmd() tea.Msg {
	sessions, err := zmx.FetchSessions()
	return sessionsMsg{sessions: sessions, err: err}
}

func fetchProcessInfoCmd(sessions []Session) tea.Cmd {
	return func() tea.Msg {
		return processInfoMsg{info: zmx.FetchProcessInfo(sessions)}
	}
}

func fetchPreviewCmd(name string, lines int) tea.Cmd {
	return func() tea.Msg {
		return previewMsg{name: name, content: zmx.FetchPreview(name, lines)}
	}
}

func killOneCmd(name string) tea.Cmd {
	return func() tea.Msg {
		err := zmx.KillSession(name)
		return killOneResultMsg{name: name, err: err}
	}
}

func waitForGoneCmd(names []string, attempt int) tea.Cmd {
	return func() tea.Msg {
		if attempt >= 20 {
			return allGoneMsg{}
		}
		time.Sleep(200 * time.Millisecond)
		sessions, err := zmx.FetchSessions()
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
	sortAsc      bool
	attachTarget string // non-empty → exec zmx attach after quit

	preview        string
	previewScrollX int
	state          state
	status         string

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

	visibleCache      []Session
	visibleCacheDirty bool
	visibleMetrics    listMetrics
	allMetrics        listMetrics
	allMetricsDirty   bool
}

type listMetrics struct {
	nameW   int
	pidW    int
	memW    int
	uptimeW int
	clientW int
}

func initialModel() Model {
	return Model{
		selected:          make(map[string]bool),
		sortAsc:           true,
		visibleCacheDirty: true,
		allMetricsDirty:   true,
	}
}

func NewModel() Model {
	return initialModel()
}

func (m Model) AttachTarget() string {
	return m.attachTarget
}

// visibleSessions returns sessions matching the current filter, sorted by sortMode.
func (m *Model) visibleSessions() []Session {
	if !m.visibleCacheDirty {
		return m.visibleCache
	}
	m.visibleCache = m.computeVisibleSessions()
	m.visibleMetrics = computeListMetrics(m.visibleCache)
	m.visibleCacheDirty = false
	return m.visibleCache
}

func (m *Model) markSessionsChanged() {
	m.visibleCacheDirty = true
	m.allMetricsDirty = true
}

func (m *Model) markVisibleChanged() {
	m.visibleCacheDirty = true
}

func (m *Model) allSessionMetrics() listMetrics {
	if m.allMetricsDirty {
		m.allMetrics = computeListMetrics(m.sessions)
		m.allMetricsDirty = false
	}
	return m.allMetrics
}

func (m *Model) computeVisibleSessions() []Session {
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

	dir := 1
	if !m.sortAsc {
		dir = -1
	}
	switch m.sortMode {
	case sortByName:
		slices.SortFunc(filtered, func(a, b Session) int {
			return dir * cmp.Compare(a.Name, b.Name)
		})
	case sortByClients:
		slices.SortFunc(filtered, func(a, b Session) int {
			if a.Clients != b.Clients {
				return dir * (a.Clients - b.Clients)
			}
			return cmp.Compare(a.Name, b.Name)
		})
	case sortByPID:
		slices.SortFunc(filtered, func(a, b Session) int {
			ai, _ := strconv.Atoi(a.PID)
			bi, _ := strconv.Atoi(b.PID)
			if ai != bi {
				return dir * (ai - bi)
			}
			return cmp.Compare(a.Name, b.Name)
		})
	case sortByMemory:
		slices.SortFunc(filtered, func(a, b Session) int {
			if a.Memory != b.Memory {
				return dir * cmp.Compare(a.Memory, b.Memory)
			}
			return cmp.Compare(a.Name, b.Name)
		})
	case sortByUptime:
		slices.SortFunc(filtered, func(a, b Session) int {
			if a.Uptime != b.Uptime {
				return dir * (a.Uptime - b.Uptime)
			}
			return cmp.Compare(a.Name, b.Name)
		})
	}

	return filtered
}

func computeListMetrics(sessions []Session) listMetrics {
	metrics := listMetrics{
		pidW:    1,
		memW:    1,
		uptimeW: 1,
		clientW: 2,
	}
	for _, s := range sessions {
		if w := runewidth.StringWidth(s.Name); w > metrics.nameW {
			metrics.nameW = w
		}
		if w := runewidth.StringWidth(s.PID); w > metrics.pidW {
			metrics.pidW = w
		}
		memLabel := "-"
		if s.Memory > 0 {
			memLabel = zmx.FormatBytes(s.Memory)
		}
		if w := runewidth.StringWidth(memLabel); w > metrics.memW {
			metrics.memW = w
		}
		uptimeLabel := "-"
		if s.Uptime > 0 {
			uptimeLabel = zmx.FormatUptime(s.Uptime)
		}
		if w := runewidth.StringWidth(uptimeLabel); w > metrics.uptimeW {
			metrics.uptimeW = w
		}
		clientLabel := fmt.Sprintf("●%d", s.Clients)
		if w := runewidth.StringWidth(clientLabel); w > metrics.clientW {
			metrics.clientW = w
		}
	}
	return metrics
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
		m.markSessionsChanged()
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
		cmds := []tea.Cmd{fetchProcessInfoCmd(m.sessions)}
		visible := m.visibleSessions()
		if len(visible) > 0 && m.cursor < len(visible) {
			cmds = append(cmds, m.previewCmd())
		} else {
			m.preview = ""
		}
		return m, tea.Batch(cmds...)

	case processInfoMsg:
		updated := false
		for i := range m.sessions {
			if info, ok := msg.info[m.sessions[i].Name]; ok {
				m.sessions[i].Memory = info.Memory
				m.sessions[i].Uptime = info.Uptime
				updated = true
			}
		}
		if updated {
			m.markSessionsChanged()
		}

	case previewMsg:
		visible := m.visibleSessions()
		if m.cursor < len(visible) && visible[m.cursor].Name == msg.name {
			m.preview = msg.content
		}

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
	m.markVisibleChanged()
	m.cursor = 0
	m.listOffset = 0
	m.killQueue = nil
	m.killDoneNames = nil
	m.killNow = ""
	return tea.Batch(fetchSessionsCmd, clearStatusAfter(3*time.Second))
}

func (m *Model) previewCmd() tea.Cmd {
	visible := m.visibleSessions()
	if m.cursor >= len(visible) {
		return nil
	}
	return fetchPreviewCmd(visible[m.cursor].Name, m.mainContentHeight(1))
}

func (m *Model) killTargets() []string {
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
