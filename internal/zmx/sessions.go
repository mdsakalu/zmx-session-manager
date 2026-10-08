package zmx

import (
	"fmt"
	"os"
	"strings"
)

// Session represents a zmx session parsed from `zmx list`.
type Session struct {
	Name      string
	Current   bool
	PID       string
	Clients   int
	StartedIn string
	Cmd       string
	Memory    uint64 // RSS of process tree in bytes
	Uptime    int    // elapsed seconds from ps etime
}

// IsCurrent reports whether this session is running zsm.
func (s Session) IsCurrent() bool {
	return s.Current || (s.Name != "" && s.Name == os.Getenv("ZMX_SESSION"))
}

// ExistingSessionEnv disables prefix expansion for names returned by zmx list.
// Other zmx settings, including the socket directory and current session, remain.
func ExistingSessionEnv() []string {
	env := os.Environ()
	result := make([]string, 0, len(env))
	for _, entry := range env {
		if !strings.HasPrefix(entry, "ZMX_SESSION_PREFIX=") {
			result = append(result, entry)
		}
	}
	return result
}

// AttachCommand returns a shell-safe command for an existing, fully named session.
func AttachCommand(name string) string {
	quoted := "'" + strings.ReplaceAll(name, "'", "'\"'\"'") + "'"
	return "env ZMX_SESSION_PREFIX= zmx attach " + quoted
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
	out, err := runCombinedOutput("zmx", "list")
	if err != nil {
		return nil, fmt.Errorf("zmx list: %w\n%s", err, out)
	}

	var sessions []Session
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		current := strings.HasPrefix(line, "→")
		line = strings.TrimSpace(strings.TrimPrefix(line, "→"))
		if line == "" {
			continue
		}

		s := Session{Current: current}
		for _, field := range strings.Split(line, "\t") {
			k, v, ok := strings.Cut(field, "=")
			if !ok {
				continue
			}
			switch k {
			case "session_name", "name":
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
			case "started_in", "start_dir":
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

// KillSession runs `zmx kill <name>`.
func KillSession(name string) error {
	cmd := deps.command("zmx", "kill", name)
	cmd.Env = ExistingSessionEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("zmx kill %s: %w\n%s", name, err, out)
	}
	return nil
}

// CopyToClipboard copies text to the system clipboard.
func CopyToClipboard(text string) error {
	return deps.clipboardWrite(text)
}
