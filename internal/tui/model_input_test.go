package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestHandleKey_JMovesCursorDown(t *testing.T) {
	m := initialModel()
	m.sessions = []Session{{Name: "a"}, {Name: "b"}}
	m.markSessionsChanged()

	updated, _ := m.handleKey(tea.KeyPressMsg{Code: 'j'})
	got := updated.(Model)

	if got.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", got.cursor)
	}
	if got.state != stateNormal {
		t.Fatalf("state = %v, want stateNormal", got.state)
	}
}

func TestHandleKey_KMovesCursorUpWithoutKill(t *testing.T) {
	m := initialModel()
	m.sessions = []Session{{Name: "a"}, {Name: "b"}}
	m.cursor = 1
	m.markSessionsChanged()

	updated, _ := m.handleKey(tea.KeyPressMsg{Code: 'k'})
	got := updated.(Model)

	if got.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", got.cursor)
	}
	if got.state != stateNormal {
		t.Fatalf("state = %v, want stateNormal", got.state)
	}
}

func TestHandleKey_ShiftKStartsKillConfirm(t *testing.T) {
	m := initialModel()
	m.sessions = []Session{{Name: "a"}}
	m.markSessionsChanged()

	updated, _ := m.handleKey(tea.KeyPressMsg{Text: "K", Code: 'k', Mod: tea.ModShift})
	got := updated.(Model)

	if got.state != stateConfirmKill {
		t.Fatalf("state = %v, want stateConfirmKill", got.state)
	}
	if got.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", got.cursor)
	}
}

func TestHandleKey_ShiftKStartsKillConfirmWithEmptyText(t *testing.T) {
	m := initialModel()
	m.sessions = []Session{{Name: "a"}}
	m.markSessionsChanged()

	updated, _ := m.handleKey(tea.KeyPressMsg{Code: 'k', Mod: tea.ModShift})
	got := updated.(Model)

	if got.state != stateConfirmKill {
		t.Fatalf("state = %v, want stateConfirmKill", got.state)
	}
}

func TestHandleKey_GGJumpsToTop(t *testing.T) {
	m := initialModel()
	m.sessions = []Session{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	m.cursor = 2
	m.markSessionsChanged()

	updated, _ := m.handleKey(tea.KeyPressMsg{Code: 'g'})
	got := updated.(Model)
	if got.cursor != 2 {
		t.Fatalf("cursor after first g = %d, want 2", got.cursor)
	}
	if !got.pendingGoTop {
		t.Fatalf("pendingGoTop should be true after first g")
	}

	updated, _ = got.handleKey(tea.KeyPressMsg{Code: 'g'})
	got = updated.(Model)
	if got.cursor != 0 {
		t.Fatalf("cursor after gg = %d, want 0", got.cursor)
	}
	if got.listOffset != 0 {
		t.Fatalf("listOffset after gg = %d, want 0", got.listOffset)
	}
	if got.pendingGoTop {
		t.Fatalf("pendingGoTop should be false after gg")
	}
}

func TestHandleKey_ShiftGMovesCursorToBottom(t *testing.T) {
	m := initialModel()
	m.sessions = []Session{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	m.markSessionsChanged()

	updated, _ := m.handleKey(tea.KeyPressMsg{Code: 'g', Mod: tea.ModShift})
	got := updated.(Model)

	if got.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", got.cursor)
	}
	if got.pendingGoTop {
		t.Fatalf("pendingGoTop should be false after G")
	}
}
