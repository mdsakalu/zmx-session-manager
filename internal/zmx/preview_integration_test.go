package zmx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPreviewAfterRealScreenClear(t *testing.T) {
	if os.Getenv("ZSM_TEST_ZMX") != "1" {
		t.Skip("set ZSM_TEST_ZMX=1 to test with installed zmx 1.0.1+")
	}
	// Keep Unix socket paths short enough for macOS.
	dir, err := os.MkdirTemp("/tmp", "zsm-screen-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("ZMX_DIR", dir)
	t.Setenv("ZMX_SESSION_PREFIX", "")
	t.Setenv("ZMX_SESSION", "")
	t.Setenv("SHELL", "/bin/sh")
	run := func(args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "zmx", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("zmx %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	t.Cleanup(func() { run("kill", "preview-test") })
	run("run", "preview-test", "echo", "CLEARED_MARKER")
	run("run", "preview-test", "seq", "1", "100")
	run("run", "preview-test", "printf", "'\\033[2J\\033[HSCREEN_MARKER\\n'")
	if history := run("history", "preview-test", "--vt"); !strings.Contains(history, "CLEARED_MARKER") {
		t.Fatalf("test setup did not preserve scrollback: %q", history)
	}
	result := FetchPreview("preview-test", 20)
	if result.Scrollback {
		t.Fatal("modern session unexpectedly used scrollback")
	}
	preview := result.Content
	if !strings.Contains(preview, "SCREEN_MARKER") || strings.Contains(preview, "CLEARED_MARKER") || strings.Contains(preview, "old-row") {
		t.Fatalf("preview after clear = %q", preview)
	}
}

func TestPreviewWithRealLegacyClient(t *testing.T) {
	legacy := os.Getenv("ZSM_TEST_LEGACY_ZMX")
	if legacy == "" {
		t.Skip("set ZSM_TEST_LEGACY_ZMX to a zmx 0.7.1 executable")
	}
	dir, err := os.MkdirTemp("/tmp", "zsm-legacy-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("ZMX_DIR", dir)
	t.Setenv("ZMX_SESSION_PREFIX", "")
	t.Setenv("ZMX_SESSION", "")
	t.Setenv("PATH", filepath.Dir(legacy)+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Cleanup(func() { _ = exec.Command(legacy, "kill", "legacy-test").Run() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, legacy, "run", "legacy-test", "echo", "LEGACY_MARKER").CombinedOutput(); err != nil {
		t.Fatalf("create legacy session: %v: %s", err, out)
	}
	preview := FetchPreview("legacy-test", 20)
	if !preview.Scrollback || !strings.Contains(preview.Content, "LEGACY_MARKER") {
		t.Fatalf("legacy preview = %+v", preview)
	}
}
