package main

import (
	"log"
	"os"
	"os/exec"
	"runtime"

	"golang.org/x/crypto/ssh/terminal"
)

import "fmt"

// must is a helper function that wraps a call to a function returning an error and logs it if the error is non-nil.
func must(action string, err error) error {
	if err != nil {
		return fmt.Errorf("Failed to %s: %w", action, err)
	}
	return nil
}

// Executes whichever clear command exists for the OS running this application
// Supports Linux, Windows, and Mac OS
func clearScreen() {
	cmd := exec.Command("clear") // Linux or macOS
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls") // Windows
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func getTerminalHeight() (int, error) {
	_, height, err := terminal.GetSize(0)
	if err != nil {
		return 0, err
	}
	return height, nil
}
