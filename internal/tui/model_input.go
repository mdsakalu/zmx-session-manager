package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/mdsakalu/zmx-session-manager/internal/zmx"
)

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.state == stateConfirmKill {
		return m.handleConfirmKey(msg)
	}

	if isQuit(msg) {
		return m, tea.Quit
	}

	m.handleLogScroll(msg)
	key := msg.String()
	if key != "g" {
		m.pendingGoTop = false
	}

	// Escape or Backspace clears active filter in normal mode
	if (msg.Code == tea.KeyEscape || msg.Code == tea.KeyBackspace) && m.filterText != "" {
		m.filterText = ""
		m.markVisibleChanged()
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
		if m.moveCursorUp(true) {
			return m, m.previewCmd()
		}

	case tea.KeyDown:
		if m.moveCursorDown(visible, true) {
			return m, m.previewCmd()
		}

	case tea.KeyLeft:
		if m.previewScrollX > 0 {
			m.previewScrollX -= 4
			if m.previewScrollX < 0 {
				m.previewScrollX = 0
			}
		}

	case tea.KeyRight:
		maxW := previewMaxWidth(m.preview)
		limit := maxW - m.previewInnerWidth()
		if limit < 0 {
			limit = 0
		}
		if m.previewScrollX+4 <= limit {
			m.previewScrollX += 4
		} else {
			m.previewScrollX = limit
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
		switch key {
		case "k":
			if m.moveCursorUp(true) {
				return m, m.previewCmd()
			}
		case "j":
			if m.moveCursorDown(visible, true) {
				return m, m.previewCmd()
			}
		case "g":
			if m.pendingGoTop {
				m.pendingGoTop = false
				if len(visible) > 0 {
					m.cursor = 0
					m.listOffset = 0
					m.previewScrollX = 0
					return m, m.previewCmd()
				}
				return m, nil
			}
			m.pendingGoTop = true
		case "G", "shift+g":
			if len(visible) > 0 {
				m.cursor = len(visible) - 1
				m.previewScrollX = 0
				m.ensureVisible()
				return m, m.previewCmd()
			}
		case "K", "shift+k":
			targets := m.killTargets()
			if len(targets) > 0 {
				m.state = stateConfirmKill
			}
		case "c":
			if m.cursor < len(visible) {
				name := visible[m.cursor].Name
				text := fmt.Sprintf("zmx attach %s", name)
				if err := zmx.CopyToClipboard(text); err != nil {
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
			if m.sortAsc {
				m.sortAsc = false
			} else {
				m.sortAsc = true
				m.sortMode = (m.sortMode + 1) % sortModeCount
			}
			m.markVisibleChanged()
			m.cursor = 0
			m.listOffset = 0
			return m, m.previewCmd()
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

// pruneSelections removes selections for sessions not in the current visible set.
func (m *Model) pruneSelections() {
	visible := m.visibleSessions()
	allowed := make(map[string]bool, len(visible))
	for _, s := range visible {
		allowed[s.Name] = true
	}
	for name := range m.selected {
		if !allowed[name] {
			delete(m.selected, name)
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
		m.markVisibleChanged()
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
			m.markVisibleChanged()
			m.pruneSelections()
			m.cursor = 0
			m.listOffset = 0
		} else {
			// Backspace on empty filter exits filter mode
			m.state = stateNormal
			return m, m.previewCmd()
		}

	case tea.KeyUp:
		if m.moveCursorUp(false) {
			return m, m.previewCmd()
		}

	case tea.KeyDown:
		visible := m.visibleSessions()
		if m.moveCursorDown(visible, false) {
			return m, m.previewCmd()
		}

	default:
		if msg.Text != "" {
			m.filterText += msg.Text
			m.markVisibleChanged()
			m.pruneSelections()
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

func (m *Model) moveCursorUp(resetPreviewScroll bool) bool {
	if m.cursor <= 0 {
		return false
	}
	m.cursor--
	if resetPreviewScroll {
		m.previewScrollX = 0
	}
	m.ensureVisible()
	return true
}

func (m *Model) moveCursorDown(visible []Session, resetPreviewScroll bool) bool {
	if m.cursor >= len(visible)-1 {
		return false
	}
	m.cursor++
	if resetPreviewScroll {
		m.previewScrollX = 0
	}
	m.ensureVisible()
	return true
}

func (m *Model) ensureVisible() {
	h := m.mainContentHeight(1)
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
