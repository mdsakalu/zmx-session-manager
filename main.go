package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
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
	opts, err := parseOptions(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
	if opts.version {
		fmt.Printf("zsm %s (%s, %s)\n", version, commit, date)
		return
	}

	zmxPath, err := exec.LookPath("zmx")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: zmx not found in PATH")
		os.Exit(1)
	}

	var sessionExists func(string) bool
	if opts.exitOnEnd {
		sessionExists = zmxSessionExists
	}

	runUI := func(selectedSession string) (tui.AttachRequest, error) {
		return runTUI(selectedSession, opts.ui)
	}
	err = runSessionManager(runUI, func(request tui.AttachRequest) error {
		return attachToSession(zmxPath, request)
	}, sessionExists)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

type options struct {
	version   bool
	exitOnEnd bool
	ui        tui.Options
}

// parseOptions reads ZSM_* environment defaults, then lets flags override them.
func parseOptions(args []string) (options, error) {
	opts := options{ui: tui.DefaultOptions()}

	exitOnEnd, err := envBool("ZSM_EXIT_ON_END", false)
	if err != nil {
		return options{}, err
	}
	wrap, err := envBool("ZSM_WRAP", opts.ui.Wrap)
	if err != nil {
		return options{}, err
	}

	fs := flag.NewFlagSet("zsm", flag.ContinueOnError)
	fs.BoolVar(&opts.version, "v", false, "print version and exit")
	fs.BoolVar(&opts.version, "version", false, "print version and exit")
	fs.BoolVar(&opts.exitOnEnd, "exit-on-end", exitOnEnd,
		"quit instead of returning to the picker when the attached session ends (env ZSM_EXIT_ON_END)")
	fs.BoolVar(&opts.ui.Wrap, "wrap", wrap, "wrap preview lines (env ZSM_WRAP)")
	sortSpec := fs.String("sort", os.Getenv("ZSM_SORT"),
		"initial sort: name, clients, pid, memory or uptime, optionally :asc or :desc (env ZSM_SORT)")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() > 0 {
		return options{}, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}

	if opts.ui.Sort, err = tui.ParseSort(*sortSpec); err != nil {
		return options{}, err
	}
	return opts, nil
}

func envBool(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s=%q: want true/false or 1/0", key, v)
	}
	return b, nil
}

func zmxSessionExists(name string) bool {
	sessions, err := zmx.FetchSessions()
	if err != nil {
		// Can't tell; returning to the picker is the safe default.
		return true
	}
	for _, s := range sessions {
		if s.Name == name {
			return true
		}
	}
	return false
}

// runSessionManager loops picker -> attach -> picker. If sessionExists is
// non-nil, it quits once an attached session no longer exists.
func runSessionManager(
	runUI func(string) (tui.AttachRequest, error),
	attach func(tui.AttachRequest) error,
	sessionExists func(string) bool,
) error {
	selectedSession := ""
	for {
		request, err := runUI(selectedSession)
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
		selectedSession = request.Target
		if request.NewSession {
			selectedSession = os.Getenv("ZMX_SESSION_PREFIX") + selectedSession
		}
		if sessionExists != nil && !sessionExists(selectedSession) {
			return nil
		}
	}
}

func runTUI(selectedSession string, opts tui.Options) (tui.AttachRequest, error) {
	finalModel, err := tea.NewProgram(tui.NewModel(selectedSession).WithOptions(opts)).Run()
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
