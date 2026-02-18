package zmx

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
)

// FetchPreview returns the last `lines` lines of `zmx history <name> --vt`,
// with all ANSI escape sequences stripped. Lines are NOT truncated so that
// the caller can apply horizontal scrolling before display.
func FetchPreview(name string, lines int) string {
	if lines < 1 {
		lines = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := deps.commandContext(ctx, "zmx", "history", name, "--vt")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Sprintf("(preview unavailable: %v)", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("(preview unavailable: %v)", err)
	}

	preview, readErr := tailLinesFromReader(stdout, lines)
	waitErr := cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		return "(preview unavailable: timed out)"
	}
	if readErr != nil {
		return fmt.Sprintf("(preview unavailable: %v)", readErr)
	}
	if waitErr != nil {
		return fmt.Sprintf("(preview unavailable: %v)", waitErr)
	}
	return preview
}

func tailLinesFromReader(r io.Reader, lines int) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	tail := make([]string, 0, lines)
	for scanner.Scan() {
		tail = append(tail, stripANSI(scanner.Text()))
		if len(tail) > lines {
			tail = tail[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return strings.Join(tail, "\n"), nil
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
