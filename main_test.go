package main

import (
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdsakalu/zmx-session-manager/internal/tui"
)

func TestAttachExistingSessionDoesNotApplyPrefixAgain(t *testing.T) {
	t.Setenv("ZMX_SESSION_PREFIX", "d.")
	dir := t.TempDir()
	output := filepath.Join(dir, "target")
	t.Setenv("ZSM_TEST_TARGET", output)
	command := filepath.Join(dir, "zmx")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nprintf '%s' \"${ZMX_SESSION_PREFIX}$2\" > \"$ZSM_TEST_TARGET\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"d.term", "d.d.term", "other.term"} {
		if err := attachToSession(command, tui.AttachRequest{Target: target, Mode: tui.AttachAndReturn}); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(got)) != target {
			t.Errorf("attached to %q, want existing session %q", got, target)
		}
	}
}

func TestAttachNewSessionPreservesPrefix(t *testing.T) {
	t.Setenv("ZMX_SESSION_PREFIX", "d.")
	dir := t.TempDir()
	output := filepath.Join(dir, "target")
	t.Setenv("ZSM_TEST_TARGET", output)
	command := filepath.Join(dir, "zmx")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nprintf '%s' \"${ZMX_SESSION_PREFIX}$2\" > \"$ZSM_TEST_TARGET\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := attachToSession(command, tui.AttachRequest{Target: "term", Mode: tui.AttachAndReturn, NewSession: true}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil || string(got) != "d.term" {
		t.Fatalf("new session = %q, err=%v; want d.term", got, err)
	}
}

func TestAttachReplaceProcessPreservesExactName(t *testing.T) {
	if command := os.Getenv("ZSM_TEST_REPLACE_COMMAND"); command != "" {
		if err := attachToSession(command, tui.AttachRequest{Target: "d.term", Mode: tui.AttachReplaceProcess}); err != nil {
			t.Fatal(err)
		}
		t.Fatal("exec unexpectedly returned")
	}
	dir := t.TempDir()
	command := filepath.Join(dir, "zmx")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nprintf '%s' \"${ZMX_SESSION_PREFIX}$2\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZMX_SESSION_PREFIX", "d.")
	t.Setenv("ZSM_TEST_REPLACE_COMMAND", command)
	cmd := exec.Command(os.Args[0], "-test.run=^TestAttachReplaceProcessPreservesExactName$")
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "d.term" {
		t.Fatalf("replace attach output=%q err=%v; want d.term", out, err)
	}
}

func TestRunSessionManagerExitsWhenAttachingCurrentSession(t *testing.T) {
	t.Setenv("ZMX_SESSION", "d.current")
	for _, mode := range []tui.AttachMode{tui.AttachAndReturn, tui.AttachReplaceProcess} {
		calls := 0
		err := runSessionManager(func(string) (tui.AttachRequest, error) {
			calls++
			if calls > 1 {
				return tui.AttachRequest{}, nil
			}
			return tui.AttachRequest{Target: "d.current", Mode: mode}, nil
		}, func(tui.AttachRequest) error {
			t.Error("attempted to attach recursively to the current session")
			return nil
		}, nil)
		if err != nil || calls != 1 {
			t.Errorf("current-session selection: calls=%d err=%v, want one UI invocation and no error", calls, err)
		}
	}
}

func TestRunSessionManagerReturnsToUIAfterDetach(t *testing.T) {
	requests := []tui.AttachRequest{
		{Target: "demo", Mode: tui.AttachAndReturn},
		{},
	}
	uiCalls := 0
	runUI := func(selectedSession string) (tui.AttachRequest, error) {
		wantSelection := ""
		if uiCalls > 0 {
			wantSelection = "demo"
		}
		if selectedSession != wantSelection {
			t.Errorf("picker selected %q, want %q", selectedSession, wantSelection)
		}
		request := requests[uiCalls]
		uiCalls++
		return request, nil
	}

	var attached []tui.AttachRequest
	attach := func(request tui.AttachRequest) error {
		attached = append(attached, request)
		return nil
	}

	if err := runSessionManager(runUI, attach, nil); err != nil {
		t.Fatalf("runSessionManager() error = %v", err)
	}
	if uiCalls != 2 {
		t.Fatalf("TUI ran %d times, want 2", uiCalls)
	}
	if len(attached) != 1 || attached[0] != requests[0] {
		t.Fatalf("attach requests = %+v, want %+v", attached, requests[:1])
	}
}

func TestRunSessionManagerRemembersLatestSessionByExactName(t *testing.T) {
	t.Setenv("ZMX_SESSION", "")
	t.Setenv("ZMX_SESSION_PREFIX", "d.")
	requests := []tui.AttachRequest{
		{Target: "other.existing", Mode: tui.AttachAndReturn},
		{Target: "new", Mode: tui.AttachAndReturn, NewSession: true},
		{Target: "d.existing", Mode: tui.AttachAndReturn},
		{},
	}
	wantSelections := []string{"", "other.existing", "d.new", "d.existing"}
	calls := 0
	err := runSessionManager(func(selectedSession string) (tui.AttachRequest, error) {
		if calls >= len(requests) {
			t.Fatal("unexpected picker invocation")
		}
		if selectedSession != wantSelections[calls] {
			t.Errorf("picker %d selected %q, want %q", calls, selectedSession, wantSelections[calls])
		}
		request := requests[calls]
		calls++
		return request, nil
	}, func(tui.AttachRequest) error { return nil }, nil)
	if err != nil || calls != len(requests) {
		t.Fatalf("runSessionManager: calls=%d err=%v", calls, err)
	}
}

func TestRunSessionManagerExitsWhenAttachedSessionEnds(t *testing.T) {
	t.Setenv("ZMX_SESSION", "")
	t.Setenv("ZMX_SESSION_PREFIX", "d.")
	for _, request := range []tui.AttachRequest{
		{Target: "d.gone", Mode: tui.AttachAndReturn},
		{Target: "gone", Mode: tui.AttachAndReturn, NewSession: true},
	} {
		uiCalls := 0
		var checked []string
		err := runSessionManager(func(string) (tui.AttachRequest, error) {
			uiCalls++
			if uiCalls > 1 {
				t.Fatal("picker reopened after the attached session ended")
			}
			return request, nil
		}, func(tui.AttachRequest) error { return nil }, func(name string) bool {
			checked = append(checked, name)
			return false
		})
		if err != nil || uiCalls != 1 {
			t.Fatalf("calls=%d err=%v, want one picker run and no error", uiCalls, err)
		}
		if len(checked) != 1 || checked[0] != "d.gone" {
			t.Fatalf("checked sessions %q, want [d.gone]", checked)
		}
	}
}

func TestRunSessionManagerReturnsToUIWhenSessionSurvives(t *testing.T) {
	t.Setenv("ZMX_SESSION", "")
	requests := []tui.AttachRequest{{Target: "alive", Mode: tui.AttachAndReturn}, {}}
	uiCalls := 0
	err := runSessionManager(func(string) (tui.AttachRequest, error) {
		request := requests[uiCalls]
		uiCalls++
		return request, nil
	}, func(tui.AttachRequest) error { return nil }, func(string) bool { return true })
	if err != nil || uiCalls != 2 {
		t.Fatalf("calls=%d err=%v, want the picker to reopen after detach", uiCalls, err)
	}
}

func TestFailedAttachReturnsToPickerWithExitOnEnd(t *testing.T) {
	t.Setenv("ZMX_SESSION", "")
	calls := 0
	err := runSessionManager(func(string) (tui.AttachRequest, error) {
		calls++
		if calls == 1 {
			return tui.AttachRequest{Target: "gone", Mode: tui.AttachAndReturn}, nil
		}
		return tui.AttachRequest{}, nil
	}, func(r tui.AttachRequest) error {
		return attachToSession("/usr/bin/false", r)
	}, func(string) bool {
		t.Fatal("checked session existence after failed attach")
		return false
	})
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestOptions(t *testing.T) {
	for _, tc := range []struct {
		name, file string
		args       []string
		sort       string
		wrap, exit bool
		warnings   []string
	}{
		{"missing", "", nil, "name", true, false, nil},
		{"file", "wrap=false\nsort='memory:desc'\nexit_on_end=true", nil, "memory:desc", false, true, nil},
		{"flags", "wrap=false\nsort='memory:desc'\nexit_on_end=true", []string{"--wrap", "--sort=pid", "--exit-on-end=false"}, "pid", true, false, nil},
		{"invalid entries", "wrap=false\nsort='bad'\nexit_on_end=1\nfoo=true", nil, "name", false, false, []string{"sort:", "exit_on_end:", "foo:"}},
		{"malformed", "wrap=false\n?!!", []string{"--exit-on-end"}, "name", true, true, []string{"config.toml"}},
		{"unreadable", "<directory>", nil, "name", true, false, []string{"config.toml"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", root)
			dir := filepath.Join(root, "zsm")
			path := filepath.Join(dir, "config.toml")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			switch tc.file {
			case "<directory>":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "":
			default:
				if err := os.WriteFile(path, []byte(tc.file), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var output strings.Builder
			opts, err := parseOptions(tc.args, &output)
			wantSort, _ := tui.ParseSort(tc.sort)
			if err != nil || opts.ui.Sort != wantSort || opts.ui.Wrap != tc.wrap || opts.exitOnEnd != tc.exit {
				t.Fatalf("options=%+v err=%v", opts, err)
			}
			if n := strings.Count(output.String(), "Warning: "+path); n != len(tc.warnings) {
				t.Errorf("unexpected warnings: %q", output.String())
			}
			for _, w := range tc.warnings {
				if !strings.Contains(output.String(), w) {
					t.Errorf("missing %q in %q", w, output.String())
				}
			}
		})
	}
}

func TestInformationFlagsAndInvalidCLI(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", "")
	dir := filepath.Join(root, ".config", "zsm")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("wrap=false\nunknown=1"), 0600); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if o, err := parseOptions(nil, &out); err != nil || o.ui.Wrap {
		t.Fatalf("home config: %+v %v", o, err)
	}
	for _, arg := range []string{"--version", "-v", "--help", "--wrap=invalid", "--sort=", "--sort=invalid", "--unknown"} {
		out.Reset()
		opts, err := parseOptions([]string{arg}, &out)
		switch arg {
		case "--help":
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatal(err)
			}
		case "--version", "-v":
			if err != nil || !opts.version {
				t.Fatalf("%s: %+v %v", arg, opts, err)
			}
		default:
			if err == nil {
				t.Fatalf("%s accepted", arg)
			}
		}
		if strings.Contains(out.String(), "Warning:") {
			t.Fatalf("%s read config", arg)
		}
	}
}

func TestAttachNewSessionUsesSelectedWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	workdir := filepath.Join(root, "project")
	if err := os.Mkdir(workdir, 0700); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "cwd")
	t.Setenv("ZSM_TEST_TARGET", output)
	command := filepath.Join(root, "zmx")
	if err := os.WriteFile(command, []byte("#!/bin/sh\npwd > \"$ZSM_TEST_TARGET\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	err := attachToSession(command, tui.AttachRequest{Target: "project", Mode: tui.AttachAndReturn, NewSession: true, WorkingDir: workdir})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != workdir {
		t.Fatalf("working directory = %q, want %q", got, workdir)
	}
}
