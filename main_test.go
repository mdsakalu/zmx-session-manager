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
		err := runSessionManager(func() (tui.AttachRequest, error) {
			calls++
			if calls > 1 {
				return tui.AttachRequest{}, nil
			}
			return tui.AttachRequest{Target: "d.current", Mode: mode}, nil
		}, func(tui.AttachRequest) error {
			t.Error("attempted to attach recursively to the current session")
			return nil
		})
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
	runUI := func() (tui.AttachRequest, error) {
		request := requests[uiCalls]
		uiCalls++
		return request, nil
	}

	var attached []tui.AttachRequest
	attach := func(request tui.AttachRequest) error {
		attached = append(attached, request)
		return nil
	}

	if err := runSessionManager(runUI, attach); err != nil {
		t.Fatalf("runSessionManager() error = %v", err)
	}
	if uiCalls != 2 {
		t.Fatalf("TUI ran %d times, want 2", uiCalls)
	}
	if len(attached) != 1 || attached[0] != requests[0] {
		t.Fatalf("attach requests = %+v, want %+v", attached, requests[:1])
	}
}
