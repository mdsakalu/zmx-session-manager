package zmx

import (
	"context"
	"os"
	"os/exec"
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
	preview := FetchPreview("preview-test", 20)
	if !strings.Contains(preview, "SCREEN_MARKER") || strings.Contains(preview, "CLEARED_MARKER") || strings.Contains(preview, "old-row") {
		t.Fatalf("preview after clear = %q", preview)
	}
}
