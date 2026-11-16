package main

import (
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

func TestParseOptions(t *testing.T) {
	mustSort := func(s string) tui.Sort {
		sort, err := tui.ParseSort(s)
		if err != nil {
			t.Fatal(err)
		}
		return sort
	}
	cases := []struct {
		args []string
		env  map[string]string
		want options
	}{
		{nil, nil, options{ui: tui.DefaultOptions()}},
		{[]string{"--exit-on-end"}, nil, options{exitOnEnd: true, ui: tui.DefaultOptions()}},
		{nil, map[string]string{"ZSM_EXIT_ON_END": "1"}, options{exitOnEnd: true, ui: tui.DefaultOptions()}},
		{nil, map[string]string{"ZSM_EXIT_ON_END": "0"}, options{ui: tui.DefaultOptions()}},
		{[]string{"-v"}, nil, options{version: true, ui: tui.DefaultOptions()}},
		{[]string{"--version"}, nil, options{version: true, ui: tui.DefaultOptions()}},
		{
			nil,
			map[string]string{"ZSM_SORT": "memory:desc", "ZSM_WRAP": "0"},
			options{ui: tui.Options{Sort: mustSort("memory:desc"), Wrap: false}},
		},
		{
			// flags override the environment
			[]string{"--sort=uptime", "--wrap"},
			map[string]string{"ZSM_SORT": "memory:desc", "ZSM_WRAP": "0"},
			options{ui: tui.Options{Sort: mustSort("uptime"), Wrap: true}},
		},
		{[]string{"--wrap=false"}, nil, options{ui: tui.Options{Sort: mustSort("name"), Wrap: false}}},
	}
	for _, c := range cases {
		for _, k := range []string{"ZSM_EXIT_ON_END", "ZSM_SORT", "ZSM_WRAP"} {
			t.Setenv(k, c.env[k])
		}
		got, err := parseOptions(c.args)
		if err != nil {
			t.Fatalf("parseOptions(%q) env=%v error = %v", c.args, c.env, err)
		}
		if got != c.want {
			t.Errorf("parseOptions(%q) env=%v = %+v, want %+v", c.args, c.env, got, c.want)
		}
	}
}

func TestParseOptionsRejectsBadInput(t *testing.T) {
	for _, c := range []struct {
		args []string
		env  map[string]string
	}{
		{[]string{"--sort=size"}, nil},
		{nil, map[string]string{"ZSM_SORT": "size"}},
		{nil, map[string]string{"ZSM_WRAP": "maybe"}},
		{[]string{"--bogus"}, nil},
	} {
		for _, k := range []string{"ZSM_EXIT_ON_END", "ZSM_SORT", "ZSM_WRAP"} {
			t.Setenv(k, c.env[k])
		}
		if _, err := parseOptions(c.args); err == nil {
			t.Errorf("parseOptions(%q) env=%v succeeded, want error", c.args, c.env)
		}
	}
}
