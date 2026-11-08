package zmx

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestFetchPreviewUsesCurrentScreen(t *testing.T) {
	orig := deps
	t.Cleanup(func() { deps = orig })
	deps.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if args[0] == "help" {
			return exec.CommandContext(ctx, "echo", "--screen")
		}
		// Model zmx's observable output: scrollback contains cleared text;
		// screen capture contains only current rows and unused blank rows.
		return exec.CommandContext(ctx, "sh", "-c", `
case " $* " in
  *" --screen "*) printf '\033[32mcurrent\033[0m\n\n\n\n' ;;
  *) printf 'cleared history\ncurrent\n' ;;
esac`, "sh", strings.Join(args, " "))
	}
	if got := FetchPreview("test", 2).Content; got != "\x1b[32mcurrent\x1b[0m" {
		t.Fatalf("preview = %q, want current screen without cleared history or trailing blank rows", got)
	}
}

func TestScreenPreviewPreservesInteriorBlankRows(t *testing.T) {
	got, err := tailLinesFromReader(strings.NewReader("old\n\ncurrent\n\n\n"), 2)
	if err != nil || got != "\ncurrent" {
		t.Fatalf("screen tail = %q, %v", got, err)
	}
}

func TestPreviewCompatibility(t *testing.T) {
	for _, tt := range []struct {
		name           string
		help           string
		screenError    string
		wantScrollback bool
		wantContent    string
	}{
		{"old client", "history --vt", "", true, "legacy output"},
		{"old daemon", "history --screen", "daemon too old?", true, "legacy output"},
		{"missing session", "history --screen", "session does not exist", false, "(preview unavailable:"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			orig := deps
			t.Cleanup(func() { deps = orig })
			deps.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
				if args[0] == "help" {
					return exec.CommandContext(ctx, "echo", tt.help)
				}
				if strings.Contains(strings.Join(args, " "), "--screen") {
					return exec.CommandContext(ctx, "sh", "-c", `echo "$1" >&2; exit 1`, "sh", tt.screenError)
				}
				return exec.CommandContext(ctx, "echo", "legacy output")
			}
			got := FetchPreview("test", 10)
			if got.Scrollback != tt.wantScrollback || !strings.Contains(got.Content, tt.wantContent) {
				t.Fatalf("preview = %+v", got)
			}
		})
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"\x1b[31mRed\x1b[0m", "\x1b[31mRed\x1b[0m"},
		{"\x1b[1;32mBold Green\x1b[0m", "\x1b[1;32mBold Green\x1b[0m"},
		{"Plain text", "Plain text"},
		{"\x1b[H\x1b[2JClear screen", "Clear screen"},   // CSI H and J should be stripped
		{"\x1bP1;2|terminal-payload\x1b\\Safe", "Safe"}, // DCS payload must not reach the preview
	}

	for _, tt := range tests {
		got := stripANSI(tt.input)
		if got != tt.expected {
			t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestScrollPreviewANSI(t *testing.T) {
	input := "\x1b[31mRed\x1b[0m and \x1b[32mGreen\x1b[0m"

	// Test basic display (no scroll)
	got := ScrollPreview(input, 0, 10)
	// Expect colors to be preserved
	if !strings.Contains(got, "\x1b[31m") || !strings.Contains(got, "\x1b[32m") {
		t.Errorf("ScrollPreview(input, 0, 10) lost colors: %q", got)
	}

	// Test scrolling (skip "Red and ")
	// "Red and " is 8 chars
	got = ScrollPreview(input, 8, 5)
	// Should contain Green color and "Green"
	if !strings.Contains(got, "\x1b[32m") || !strings.Contains(got, "Green") {
		t.Errorf("ScrollPreview(input, 8, 5) lost Green color or text: %q", got)
	}
	// Should STILL contain Red color start because we preserve all SGR
	if !strings.Contains(got, "\x1b[31m") {
		t.Errorf("ScrollPreview(input, 8, 5) lost Red color start: %q", got)
	}
}

func TestScrollPreviewKeepsActiveStyleWithinCrop(t *testing.T) {
	input := "\x1b[31mabcdefghij\x1b[32moutside"

	got := ScrollPreview(input, 2, 3)
	want := "\x1b[31mcde\x1b[0m"
	if got != want {
		t.Fatalf("ScrollPreview() = %q, want %q", got, want)
	}
}
