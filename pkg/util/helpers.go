package util

import (
	"log"
	"os"
	"os/exec"
	"runtime"

	"golang.org/x/crypto/ssh/terminal"
)

// Must is a helper function that wraps a call to a function returning an error and logs it if the error is non-nil.
func Must(action string, err error) {
	if err != nil {
		log.Fatalf("Failed to %s: %v", action, err)
	}
}

// ClearScreen executes whichever clear command exists for the OS running this application
// Supports Linux, Windows, and Mac OS
func ClearScreen() {
	cmd := exec.Command("clear") // Linux or macOS
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls") // Windows
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func GetTerminalHeight() (int, error) {
	_, height, err := terminal.GetSize(0)
	if err != nil {
		return 0, err
	}
	return height, nil
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
