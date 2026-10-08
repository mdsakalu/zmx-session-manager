package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/mdsakalu/zmx-session-manager/internal/tui"
	"github.com/mdsakalu/zmx-session-manager/internal/zmx"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Printf("zsm %s (%s, %s)\n", version, commit, date)
		return
	}

	zmxPath, err := exec.LookPath("zmx")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: zmx not found in PATH")
		os.Exit(1)
	}

	err = runSessionManager(runTUI, func(request tui.AttachRequest) error {
		return attachToSession(zmxPath, request)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runSessionManager(
	runUI func() (tui.AttachRequest, error),
	attach func(tui.AttachRequest) error,
) error {
	for {
		request, err := runUI()
		if err != nil {
			return fmt.Errorf("run TUI: %w", err)
		}
		if request.Target == "" || request.Mode == tui.AttachNone {
			return nil
		}
		if !request.NewSession && request.Target == os.Getenv("ZMX_SESSION") {
			return nil
		}
		if err := attach(request); err != nil {
			return fmt.Errorf("attach to session %q: %w", request.Target, err)
		}
		if request.Mode == tui.AttachReplaceProcess {
			return fmt.Errorf("replace attach for session %q returned unexpectedly", request.Target)
		}
	}
}

func runTUI() (tui.AttachRequest, error) {
	finalModel, err := tea.NewProgram(tui.NewModel()).Run()
	if err != nil {
		return tui.AttachRequest{}, err
	}
	m, ok := finalModel.(tui.Model)
	if !ok {
		return tui.AttachRequest{}, nil
	}
	return m.AttachRequest(), nil
}

func attachToSession(zmxPath string, request tui.AttachRequest) error {
	env := os.Environ()
	if !request.NewSession {
		env = zmx.ExistingSessionEnv()
	}
	switch request.Mode {
	case tui.AttachAndReturn:
		cmd := exec.Command(zmxPath, "attach", request.Target)
		cmd.Env = env
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to attach to session %q: %v\n", request.Target, err)
			fmt.Print("Press Enter to return to zsm...")
			var input string
			fmt.Scanln(&input)
		}
		return nil
	case tui.AttachReplaceProcess:
		return syscall.Exec(zmxPath, []string{"zmx", "attach", request.Target}, env)
	default:
		return fmt.Errorf("unsupported attach mode %d", request.Mode)
	}
}
