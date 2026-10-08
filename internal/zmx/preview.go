package zmx

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// FetchPreview returns the last `lines` lines of `zmx history <name> --vt`,
// with all ANSI escape sequences stripped (except colors). Lines are NOT
// truncated so that the caller can apply horizontal scrolling before display.
func FetchPreview(name string, lines int) string {
	if lines < 1 {
		lines = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := deps.commandContext(ctx, "zmx", "history", name, "--vt")
	cmd.Env = ExistingSessionEnv()
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
// It is ANSI-aware and preserves color sequences.
func ScrollPreview(raw string, offsetX, maxWidth int) string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		lines[i] = scrollAndTruncateLine(line, offsetX, maxWidth)
	}
	return strings.Join(lines, "\n")
}

func scrollAndTruncateLine(line string, offsetX, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if offsetX < 0 {
		offsetX = 0
	}

	var prefix, out strings.Builder
	parser := ansi.NewParser()
	var state byte
	visualPos := 0
	writtenWidth := 0
	started := false
	styled := false

	for len(line) > 0 {
		sequence, width, n, newState := ansi.DecodeSequence(line, state, parser)
		if n == 0 {
			break
		}
		line = line[n:]
		state = newState

		if isSGR(sequence, parser) {
			if !started {
				prefix.WriteString(sequence)
			} else if visualPos < offsetX+maxWidth {
				out.WriteString(sequence)
				styled = true
			}
			continue
		}
		if width == 0 {
			continue
		}

		nextPos := visualPos + width
		if nextPos <= offsetX {
			visualPos = nextPos
			continue
		}
		if visualPos >= offsetX+maxWidth {
			break
		}
		if visualPos >= offsetX && nextPos <= offsetX+maxWidth {
			if !started {
				out.WriteString(prefix.String())
				styled = prefix.Len() > 0
				started = true
			}
			out.WriteString(sequence)
			writtenWidth += width
		}
		visualPos = nextPos
	}

	if styled {
		out.WriteString("\x1b[0m")
	}
	if writtenWidth < maxWidth {
		out.WriteString(strings.Repeat(" ", maxWidth-writtenWidth))
	}
	return out.String()
}

func isSGR(sequence string, parser *ansi.Parser) bool {
	return strings.HasPrefix(sequence, "\x1b[") && ansi.Cmd(parser.Command()).Final() == 'm'
}

// stripANSI removes all ANSI escape sequences and non-printable control
// characters (except newline and tab) from s, but preserves SGR (color) sequences.
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	parser := ansi.NewParser()
	var state byte
	for len(s) > 0 {
		sequence, width, n, newState := ansi.DecodeSequence(s, state, parser)
		if n == 0 {
			break
		}
		s = s[n:]
		state = newState

		if width > 0 || sequence == "\n" || sequence == "\t" || isSGR(sequence, parser) {
			b.WriteString(sequence)
		}
	}
	return b.String()
}
