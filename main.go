package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	tea "charm.land/bubbletea/v2"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Println("zsm", version)
		return
	}

	zmxPath, err := exec.LookPath("zmx")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: zmx not found in PATH")
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// If the user pressed Enter to attach, exec into zmx attach
	if m, ok := finalModel.(Model); ok && m.attachTarget != "" {
		env := os.Environ()
		syscall.Exec(zmxPath, []string{"zmx", "attach", m.attachTarget}, env)
	}
}
