package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/mattn/go-runewidth"
)

// Session represents a zmx session parsed from `zmx list`.
type Session struct {
	Name      string
	PID       string
	Clients   int
	StartedIn string
	Cmd       string
}

// DisplayDir returns a shortened version of StartedIn, replacing $HOME with ~.
func (s Session) DisplayDir() string {
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(s.StartedIn, home) {
		return "~" + s.StartedIn[len(home):]
	}
	return s.StartedIn
}

// FetchSessions parses `zmx list` output into a slice of Session.
// Format: tab-separated key=value pairs per line.
func FetchSessions() ([]Session, error) {
	out, err := exec.Command("zmx", "list").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("zmx list: %w\n%s", err, out)
	}

	var sessions []Session
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		s := Session{}
		for _, field := range strings.Split(line, "\t") {
			k, v, ok := strings.Cut(field, "=")
			if !ok {
				continue
			}
			switch k {
			case "session_name":
				s.Name = v
			case "pid":
				s.PID = v
			case "clients":
				if v == "0" {
					s.Clients = 0
				} else {
					n := 0
					for _, c := range v {
						n = n*10 + int(c-'0')
					}
					s.Clients = n
				}
			case "started_in":
				s.StartedIn = v
			case "cmd":
				s.Cmd = v
			}
		}
		if s.Name != "" {
			sessions = append(sessions, s)
		}
	}
	return sessions, nil
}

// FetchPreview returns the last `lines` lines of `zmx history <name> --vt`,
// with all ANSI escape sequences stripped. Lines are NOT truncated so that
// the caller can apply horizontal scrolling before display.
func FetchPreview(name string, lines int) string {
	out, err := exec.Command("zmx", "history", name, "--vt").CombinedOutput()
	if err != nil {
		return fmt.Sprintf("(preview unavailable: %v)", err)
	}

	clean := stripANSI(string(out))
	all := strings.Split(clean, "\n")

	// Take last N lines
	start := 0
	if len(all) > lines {
		start = len(all) - lines
	}

	return strings.Join(all[start:], "\n")
}

// ScrollPreview applies a horizontal offset and width to raw preview text,
// truncating and padding each line for display in the preview pane.
func ScrollPreview(raw string, offsetX, maxWidth int) string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		// Skip offsetX cells from the left
		skipped := 0
		runeIdx := 0
		runes := []rune(line)
		for runeIdx < len(runes) && skipped < offsetX {
			w := runewidth.RuneWidth(runes[runeIdx])
			skipped += w
			runeIdx++
		}
		rest := string(runes[runeIdx:])
		lines[i] = runewidth.FillRight(runewidth.Truncate(rest, maxWidth, ""), maxWidth)
	}
	return strings.Join(lines, "\n")
}

// KillSession runs `zmx kill <name>`.
func KillSession(name string) error {
	out, err := exec.Command("zmx", "kill", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("zmx kill %s: %w\n%s", name, err, out)
	}
	return nil
}

// CopyToClipboard copies text to the system clipboard.
func CopyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

// stripANSI removes all ANSI escape sequences and non-printable control
// characters (except newline and tab) from s.
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			i++
			if i >= len(s) {
				break
			}
			switch s[i] {
			case '[': // CSI sequence: ESC [ ... <final byte 0x40-0x7E>
				i++
				for i < len(s) && (s[i] < 0x40 || s[i] > 0x7E) {
					i++
				}
				if i < len(s) {
					i++ // skip final byte
				}
			case ']': // OSC sequence: ESC ] ... (BEL or ST)
				i++
				for i < len(s) {
					if s[i] == '\x07' {
						i++
						break
					}
					if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
						i += 2
						break
					}
					i++
				}
			case '(', ')': // Charset designation: ESC ( X or ESC ) X
				i++
				if i < len(s) {
					i++
				}
			default: // ESC + single character
				i++
			}
		} else if s[i] == '\r' {
			// Skip carriage return — we only want newlines
			i++
		} else if s[i] < 0x20 && s[i] != '\n' && s[i] != '\t' {
			// Skip other control characters
			i++
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}
