package main

import (
	"log"
	"os"
	"os/exec"
	"runtime"

	"golang.org/x/crypto/ssh/terminal"
)

// mustSucceed logs a fatal error if err is non-nil.
// Use for operations that should never fail in normal operation.
func mustSucceed(operation string, err error) {
	if err != nil {
		log.Fatalf("Failed to %s: %v", operation, err)
	}
}

// clearTerminal clears the terminal screen.
// Supports Linux, macOS, and Windows.
func clearTerminal() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}

// getTerminalHeight returns the terminal height in rows.
func getTerminalHeight() (int, error) {
	_, height, err := terminal.GetSize(0)
	return height, err
}

// getTerminalWidth returns the terminal width in columns.
func getTerminalWidth() (int, error) {
	width, _, err := terminal.GetSize(0)
	return width, err
}

// getTerminalSize returns the terminal dimensions (width, height).
func getTerminalSize() (width, height int, err error) {
	return terminal.GetSize(0)
}
