package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestSessionNameEditingAndPaste(t *testing.T) {
	m := initialModel()
	update := func(msg tea.Msg) {
		t.Helper()
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	key := func(code rune, mod tea.KeyMod) { update(tea.KeyPressMsg{Code: code, Mod: mod}) }
	update(tea.KeyPressMsg{Text: "n"})
	update(tea.PasteMsg{Content: "a界c"})
	key(tea.KeyLeft, 0)
	key(tea.KeyBackspace, 0)
	update(tea.KeyPressMsg{Text: "b"})
	if m.newSessionName != "abc" {
		t.Fatalf("middle edit = %q, want abc", m.newSessionName)
	}
	key(tea.KeyHome, 0)
	key(tea.KeyDelete, 0)
	key(tea.KeyEnd, 0)
	update(tea.PasteMsg{Content: "d\n\r\t\x00"})
	key(tea.KeyEnter, 0)
	if m.attach.Target != "bcd" {
		t.Fatalf("created name = %q, want bcd", m.attach.Target)
	}
}

func TestLegacyPreviewIsLabeled(t *testing.T) {
	m := initialModel()
	m.width, m.height = 120, 24
	m.sessions = []Session{{Name: "legacy"}}
	next, _ := m.Update(previewMsg{name: "legacy", content: "old history", scrollback: true})
	m = next.(Model)
	if !strings.Contains(ansi.Strip(m.View().Content), "[scrollback]") {
		t.Fatal("legacy preview is not labeled")
	}
	next, _ = m.Update(previewMsg{name: "legacy", content: "current screen"})
	m = next.(Model)
	if strings.Contains(ansi.Strip(m.View().Content), "[scrollback]") {
		t.Fatal("scrollback label remains on a current-screen preview")
	}
}

func TestSessionNameClearShortcuts(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{
		{Code: tea.KeyBackspace, Mod: tea.ModCtrl},
		{Code: tea.KeyBackspace, Mod: tea.ModSuper},
		{Code: 'u', Mod: tea.ModCtrl},
	} {
		m := initialModel()
		m.state = stateNewSession
		m.newSessionName = "clear me"
		m.newSessionCursor = 3
		next, _ := m.Update(key)
		got := next.(Model)
		if got.newSessionName != "" || got.newSessionCursor != 0 {
			t.Errorf("%v did not clear field: %q", key, got.newSessionName)
		}
	}
}

func TestSessionNameUsesVisibleNativeCursor(t *testing.T) {
	for _, width := range []int{5, 20, 80} {
		m := initialModel()
		m.width, m.height = width, 24
		m.state = stateNewSession
		m.newSessionName = strings.Repeat("界", 100)
		m.newSessionCursor = 99
		v := m.View()
		if v.Cursor == nil || v.Cursor.X < 0 || v.Cursor.X >= width || v.Cursor.Y != 23 {
			t.Fatalf("width %d: cursor = %+v", width, v.Cursor)
		}
		if v.Cursor.Shape != -1 || v.Cursor.Blink {
			t.Fatalf("cursor should use terminal default: %+v", v.Cursor)
		}
		if strings.Contains(m.renderHelp(), "█") {
			t.Fatal("synthetic block cursor still rendered")
		}
	}
}
