package zmx

import (
	"context"
	"os/exec"

	"github.com/atotto/clipboard"
)

type runtimeDeps struct {
	command        func(name string, arg ...string) *exec.Cmd
	commandContext func(ctx context.Context, name string, arg ...string) *exec.Cmd
	clipboardWrite func(text string) error
}

var deps = runtimeDeps{
	command:        exec.Command,
	commandContext: exec.CommandContext,
	clipboardWrite: clipboard.WriteAll,
}

func runCombinedOutput(name string, arg ...string) ([]byte, error) {
	return deps.command(name, arg...).CombinedOutput()
}
