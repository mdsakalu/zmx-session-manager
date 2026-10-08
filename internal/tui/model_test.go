package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestPreviewWrapsAtPaneWidth(t *testing.T) {
	for _, width := range []int{80, 100, 160} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m := initialModel()
			m.width, m.height = width, 30
			m.sessions = []Session{{Name: "demo"}}
			m.preview = strings.Repeat("x", m.previewInnerWidth()) + "CONTINUATION"
			content := m.View().Content
			if !strings.Contains(content, "CONTINUATION") {
				t.Fatal("long output was cropped instead of wrapping inside the preview")
			}
			for _, line := range strings.Split(content, "\n") {
				if got := lipgloss.Width(line); got > width {
					t.Fatalf("rendered line width %d exceeds terminal width %d", got, width)
				}
			}
		})
	}
}

func TestWrappedPreviewKeepsLatestRowsAndColors(t *testing.T) {
	m := initialModel()
	m.width, m.height = 80, 30
	width := m.previewInnerWidth()
	m.preview = "\x1b[31m" + strings.Repeat("x", width*2) + "LATEST\x1b[0m"
	got := m.renderPreview(1)
	if plain := strings.TrimSpace(ansi.Strip(got)); plain != "LATEST" {
		t.Fatalf("last visible row = %q, want LATEST", plain)
	}
	if !strings.Contains(got, "\x1b[31m") || !strings.Contains(got, "\x1b[0m") {
		t.Fatalf("cropping lost the active color or reset: %q", got)
	}
}

func TestWrappedPreviewPreservesWideGraphemesAndIndentation(t *testing.T) {
	m := initialModel()
	m.width, m.height = 80, 30
	width := m.previewInnerWidth()
	m.preview = "  " + strings.Repeat("界", width) + "👩🏽 END"
	got := ansi.Strip(m.renderPreview(10))
	if strings.Count(got, "界") != width || !strings.Contains(got, "👩🏽") || !strings.HasPrefix(got, "  ") {
		t.Fatalf("wrapping lost a character or indentation: %q", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if ansi.StringWidth(line) != width {
			t.Fatalf("wrapped row does not fill the pane width: %q", line)
		}
	}
}

func TestPreviewWrapToggleRetainsHorizontalScrolling(t *testing.T) {
	m := initialModel()
	m.width, m.height = 80, 30
	m.preview = "abcd" + strings.Repeat("x", m.previewInnerWidth())
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	m = updated.(Model)
	if m.previewScrollX != 0 {
		t.Fatal("horizontal offset changed while wrapping was enabled")
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'w', Text: "w"}))
	m = updated.(Model)
	if m.previewWrap {
		t.Fatal("w did not disable wrapping")
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	m = updated.(Model)
	if m.previewScrollX != 4 || strings.Contains(m.renderPreview(5), "abcd") {
		t.Fatal("unwrapped preview did not scroll horizontally")
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'w', Text: "w"}))
	m = updated.(Model)
	if !m.previewWrap || m.previewScrollX != 0 || !strings.Contains(m.renderPreview(5), "abcd") {
		t.Fatal("reenabling wrapping did not restore the full line")
	}
}

func TestWrappedPreviewReflowsAfterResize(t *testing.T) {
	m := initialModel()
	m.width, m.height = 100, 30
	m.preview = strings.Repeat("x", 100)
	before := m.renderPreview(10)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = updated.(Model)
	after := m.renderPreview(10)
	if strings.Count(after, "\n") <= strings.Count(before, "\n") || strings.Count(after, "x") != 100 {
		t.Fatalf("resize did not reflow the full output: before=%q after=%q", before, after)
	}
}

func TestCurrentSessionPreviewDoesNotFetchItsOwnOutput(t *testing.T) {
	t.Setenv("ZMX_SESSION", "current")
	m := NewModel()
	m.sessions = []Session{{Name: "current"}}
	m.preview = "old preview"
	if cmd := m.previewCmd(); cmd != nil {
		t.Fatal("current session must not fetch a recursive preview")
	}
	if m.preview == "old preview" || m.preview == "" {
		t.Fatalf("expected a current-session explanation, got %q", m.preview)
	}
	updated, _ := m.Update(previewMsg{name: "current", content: "recursive output"})
	if updated.(Model).preview == "recursive output" {
		t.Fatal("late preview result overwrote the current-session explanation")
	}
}

func TestCurrentSessionKeysExitWithoutAttaching(t *testing.T) {
	t.Setenv("ZMX_SESSION", "env-current")
	for _, session := range []Session{{Name: "env-current"}, {Name: "marked-current", Current: true}} {
		for _, key := range []tea.Key{{Code: tea.KeyEnter}, {Code: 'e', Text: "e"}} {
			m := NewModel()
			m.sessions = []Session{session}
			updated, cmd := m.Update(tea.KeyPressMsg(key))
			if updated.(Model).AttachRequest() != (AttachRequest{}) || cmd == nil {
				t.Fatalf("current session %q should quit without an attach request", session.Name)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("current-session selection must quit")
			}
		}
	}
}

func TestChangingSessionClearsPreviousPreview(t *testing.T) {
	m := initialModel()
	m.sessions = []Session{{Name: "alpha"}, {Name: "beta"}}
	m.preview = "alpha output"
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if updated.(Model).preview != "" || cmd == nil {
		t.Fatal("switching sessions must clear old output while loading the selected preview")
	}
}

func TestNewSessionChecksPrefixedNames(t *testing.T) {
	t.Setenv("ZMX_SESSION_PREFIX", "d.")
	m := NewModel()
	m.sessions = []Session{{Name: "d.project"}, {Name: "d.project-2"}}
	if got := m.availableSessionName("project"); got != "project-3" {
		t.Errorf("default name = %q, want project-3", got)
	}
	m.state = stateNewSession
	m.newSessionName = "project"
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd != nil || updated.(Model).status == "" {
		t.Fatal("new session accepted an existing prefixed name")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 8, "hello..."},
		{"hello world", 3, "hel"},
		{"hello world", 2, "he"},
		{"hello world", 1, "h"},
		{"hello world", 0, ""},
		{"hi", 4, "hi"},
		{"abcdefgh", 7, "abcd..."},
	}
	for _, tt := range tests {
		got := truncate(tt.s, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
		}
	}
}

func TestHighlightMatch(t *testing.T) {
	// Use unstyled styles so we can verify the string content
	noStyle := lipgloss.NewStyle()

	tests := []struct {
		s     string
		query string
		want  string
	}{
		{"my-session", "ses", "my-session"},  // match present
		{"my-session", "xyz", "my-session"},  // no match
		{"My-Session", "my-s", "My-Session"}, // case insensitive
		{"frontend", "front", "frontend"},    // match at start
		{"backend", "end", "backend"},        // match at end
	}
	for _, tt := range tests {
		got := highlightMatch(tt.s, tt.query, noStyle, noStyle)
		// Strip any ANSI sequences for comparison since unstyled lipgloss
		// may still produce reset sequences
		plain := stripStyleCodes(got)
		if plain != tt.want {
			t.Errorf("highlightMatch(%q, %q) plain = %q, want %q", tt.s, tt.query, plain, tt.want)
		}
	}
}

func TestHighlightMatch_ContainsQuery(t *testing.T) {
	base := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	hl := lipgloss.NewStyle().Bold(true)

	result := highlightMatch("my-session", "ses", base, hl)
	// The highlighted portion should be present unsplit
	if !strings.Contains(result, "ses") {
		t.Errorf("expected highlighted result to contain 'ses', got %q", result)
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		s     string
		width int
		want  string
	}{
		{"hi", 5, "hi   "},
		{"hello", 5, "hello"},
		{"toolong", 3, "toolong"}, // doesn't truncate
		{"", 3, "   "},
	}
	for _, tt := range tests {
		got := padRight(tt.s, tt.width)
		if got != tt.want {
			t.Errorf("padRight(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
		}
	}
}

func TestPadWidthUnicode(t *testing.T) {
	left := padLeft("界", 4)
	if got := lipgloss.Width(left); got != 4 {
		t.Fatalf("padLeft unicode width=%d want 4 (%q)", got, left)
	}
	right := padRight("界", 4)
	if got := lipgloss.Width(right); got != 4 {
		t.Fatalf("padRight unicode width=%d want 4 (%q)", got, right)
	}
}

func TestTruncateUnicodeWidth(t *testing.T) {
	got := truncate("你好世界", 5)
	if w := lipgloss.Width(got); w > 5 {
		t.Fatalf("truncate width=%d exceeds max: %q", w, got)
	}
}

func TestPreviewMaxWidthIgnoresANSI(t *testing.T) {
	got := previewMaxWidth("\x1b[31mred\x1b[0m\nplain")
	if got != 5 {
		t.Fatalf("previewMaxWidth() = %d, want 5", got)
	}
}

func TestPreviewMsgIgnoresStaleSession(t *testing.T) {
	m := initialModel()
	m.sessions = []Session{{Name: "alpha"}, {Name: "beta"}}
	m.cursor = 1

	updated, _ := m.Update(previewMsg{name: "alpha", content: "stale"})
	got := updated.(Model)
	if got.preview != "" {
		t.Fatalf("stale preview should be ignored, got %q", got.preview)
	}

	updated, _ = got.Update(previewMsg{name: "beta", content: "fresh"})
	got = updated.(Model)
	if got.preview != "fresh" {
		t.Fatalf("current preview should be applied, got %q", got.preview)
	}
}

func TestPreviewMsgAppliesForCurrentSessionDuringResize(t *testing.T) {
	m := initialModel()
	m.sessions = []Session{{Name: "alpha"}}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	got := updated.(Model)

	updated, _ = got.Update(previewMsg{name: "alpha", content: "ok"})
	got = updated.(Model)
	if got.preview != "ok" {
		t.Fatalf("preview should be updated for current session, got %q", got.preview)
	}
}

func TestVisibleSessionsInvalidatesAfterFilterAndSortChange(t *testing.T) {
	m := initialModel()
	m.sessions = []Session{
		{Name: "beta", PID: "2"},
		{Name: "alpha", PID: "1"},
	}
	m.markSessionsChanged()

	visible := m.visibleSessions()
	if len(visible) != 2 || visible[0].Name != "alpha" {
		t.Fatalf("unexpected initial ordering: %+v", visible)
	}

	m.filterText = "bet"
	m.markVisibleChanged()
	visible = m.visibleSessions()
	if len(visible) != 1 || visible[0].Name != "beta" {
		t.Fatalf("filter invalidation failed: %+v", visible)
	}

	m.filterText = ""
	m.sortAsc = false
	m.markVisibleChanged()
	visible = m.visibleSessions()
	if len(visible) != 2 || visible[0].Name != "beta" {
		t.Fatalf("sort invalidation failed: %+v", visible)
	}
}

func TestAttachKeysProduceExplicitRequests(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyPressMsg
		want AttachRequest
	}{
		{
			name: "enter returns to zsm after detach",
			key:  tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}),
			want: AttachRequest{Target: "demo", Mode: AttachAndReturn},
		},
		{
			name: "e replaces zsm process",
			key:  tea.KeyPressMsg(tea.Key{Code: 'e', Text: "e"}),
			want: AttachRequest{Target: "demo", Mode: AttachReplaceProcess},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := initialModel()
			m.sessions = []Session{{Name: "demo"}}
			m.markSessionsChanged()

			updated, _ := m.Update(tt.key)
			got := updated.(Model).AttachRequest()
			if got != tt.want {
				t.Fatalf("AttachRequest() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNewSessionUsesTypedName(t *testing.T) {
	m := initialModel()
	m.sessionNameBase = "project"

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	got := updated.(Model)
	if got.state != stateNewSession {
		t.Fatalf("state = %v, want stateNewSession", got.state)
	}

	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "demo"}))
	got = updated.(Model)
	updated, cmd := got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	got = updated.(Model)

	want := AttachRequest{Target: "demo", Mode: AttachAndReturn, NewSession: true}
	if got.AttachRequest() != want {
		t.Fatalf("AttachRequest() = %+v, want %+v", got.AttachRequest(), want)
	}
	if cmd == nil {
		t.Fatal("creating a session returned no quit command")
	}
}

func TestNewSessionUsesUniqueDirectoryDefault(t *testing.T) {
	m := initialModel()
	m.sessionNameBase = "project"
	m.sessions = []Session{{Name: "project"}, {Name: "project-2"}}
	m.markSessionsChanged()

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	got := updated.(Model)
	if got.newSessionDefault != "project-3" {
		t.Fatalf("newSessionDefault = %q, want %q", got.newSessionDefault, "project-3")
	}

	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	got = updated.(Model)
	want := AttachRequest{Target: "project-3", Mode: AttachAndReturn, NewSession: true}
	if got.AttachRequest() != want {
		t.Fatalf("AttachRequest() = %+v, want %+v", got.AttachRequest(), want)
	}
}

func TestNewSessionAcceptsQAsInput(t *testing.T) {
	m := initialModel()
	m.state = stateNewSession
	m.newSessionDefault = "session"

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"}))
	got := updated.(Model)
	if cmd != nil {
		t.Fatal("q in new-session input should not quit")
	}
	if got.newSessionName != "q" {
		t.Fatalf("newSessionName = %q, want %q", got.newSessionName, "q")
	}
}

func TestNewSessionRejectsExistingAndInvalidNames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		sessions []Session
	}{
		{name: "existing", input: "demo", sessions: []Session{{Name: "demo"}}},
		{name: "path separator", input: "team/demo"},
		{name: "dot", input: "."},
		{name: "dot dot", input: ".."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := initialModel()
			m.state = stateNewSession
			m.newSessionDefault = "session"
			m.newSessionName = tt.input
			m.sessions = tt.sessions

			updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
			got := updated.(Model)
			if cmd != nil {
				t.Fatal("invalid name should not quit")
			}
			if got.state != stateNewSession {
				t.Fatalf("state = %v, want stateNewSession", got.state)
			}
			if got.status == "" {
				t.Fatal("invalid name should display a status message")
			}
		})
	}
}

func TestNewSessionEscapeCancels(t *testing.T) {
	m := initialModel()
	m.state = stateNewSession
	m.newSessionName = "demo"
	m.status = "error"

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	got := updated.(Model)
	if cmd != nil {
		t.Fatal("Escape should cancel without quitting")
	}
	if got.state != stateNormal || got.newSessionName != "" || got.status != "" {
		t.Fatalf("cancelled model = %+v", got)
	}
}

func TestEscapeQuitsOnlyWhenNoFilterIsActive(t *testing.T) {
	t.Run("unfiltered normal view quits", func(t *testing.T) {
		m := initialModel()
		_, cmd := m.handleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
		if cmd == nil {
			t.Fatal("Escape returned no command, want tea.Quit")
		}
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Fatalf("Escape command returned %T, want tea.QuitMsg", msg)
		}
	})

	t.Run("active filter is cleared", func(t *testing.T) {
		m := initialModel()
		m.filterText = "demo"
		updated, _ := m.handleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
		got := updated.(Model)
		if got.filterText != "" {
			t.Fatalf("filterText = %q, want empty", got.filterText)
		}
	})
}

func TestEmptyListHighlightsRefreshKey(t *testing.T) {
	m := initialModel()
	got := m.renderList(5)

	if plain := stripStyleCodes(got); plain != "  No sessions found. Press n to create or r to refresh." {
		t.Fatalf("empty-list message = %q", plain)
	}
	if styledKey := helpKeyStyle.Render("n"); !strings.Contains(got, styledKey) {
		t.Fatalf("new-session key is not highlighted in %q", got)
	}
	if styledKey := helpKeyStyle.Render("r"); !strings.Contains(got, styledKey) {
		t.Fatalf("refresh key is not highlighted in %q", got)
	}
}

// stripStyleCodes removes ANSI escape sequences for test comparison.
func stripStyleCodes(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}
