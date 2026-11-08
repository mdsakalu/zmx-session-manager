package tui

import (
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m *Model) insertSessionText(text string) {
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
	runes := []rune(m.newSessionName)
	pos := min(max(m.newSessionCursor, 0), len(runes))
	m.newSessionName = string(runes[:pos]) + text + string(runes[pos:])
	m.newSessionCursor = pos + len([]rune(text))
	m.status = ""
}

func (m *Model) editSessionName(msg tea.KeyPressMsg) {
	runes := []rune(m.newSessionName)
	pos := min(max(m.newSessionCursor, 0), len(runes))
	ctrl := msg.Mod.Contains(tea.ModCtrl)
	alt := msg.Mod.Contains(tea.ModAlt)
	super := msg.Mod.Contains(tea.ModSuper)
	switch {
	case (msg.Code == tea.KeyBackspace && (ctrl || super)) || (ctrl && msg.Code == 'u'):
		runes = nil
		pos = 0
	case msg.Code == tea.KeyHome || (ctrl && msg.Code == 'a'):
		pos = 0
	case msg.Code == tea.KeyEnd || (ctrl && msg.Code == 'e'):
		pos = len(runes)
	case msg.Code == tea.KeyLeft:
		pos = max(0, pos-1)
	case msg.Code == tea.KeyRight:
		pos = min(len(runes), pos+1)
	case (ctrl && msg.Code == 'w') || (alt && msg.Code == tea.KeyBackspace):
		start := pos
		for start > 0 && unicode.IsSpace(runes[start-1]) {
			start--
		}
		for start > 0 && !unicode.IsSpace(runes[start-1]) {
			start--
		}
		runes = append(runes[:start], runes[pos:]...)
		pos = start
	case ctrl && msg.Code == 'k':
		runes = runes[:pos]
	case msg.Code == tea.KeyBackspace:
		if pos > 0 {
			runes = append(runes[:pos-1], runes[pos:]...)
			pos--
		}
	case msg.Code == tea.KeyDelete || (ctrl && msg.Code == 'd'):
		if pos < len(runes) {
			runes = append(runes[:pos], runes[pos+1:]...)
		}
	default:
		if msg.Text != "" && !ctrl && !alt && !super {
			m.insertSessionText(msg.Text)
		}
		return
	}
	m.newSessionName = string(runes)
	m.newSessionCursor = pos
	m.status = ""
}

// Keep the insertion point visible even when the name exceeds the terminal width.
func (m Model) sessionInputView() (string, int) {
	prompt := " New session: "
	if m.width > 0 {
		prompt = ansi.Truncate(prompt, max(0, m.width-1), "")
	}
	width := max(1, m.width-ansi.StringWidth(prompt))
	runes := []rune(m.newSessionName)
	pos := min(max(m.newSessionCursor, 0), len(runes))
	start := 0
	for start < pos && ansi.StringWidth(string(runes[start:pos])) >= width {
		start++
	}
	name := helpKeyStyle.Render(ansi.Truncate(string(runes[start:]), width, ""))
	if len(runes) == 0 {
		name = logDimStyle.Render(ansi.Truncate(m.newSessionDefault, width, ""))
	}
	return helpStyle.Render(prompt) + name, ansi.StringWidth(prompt) + ansi.StringWidth(string(runes[start:pos]))
}
