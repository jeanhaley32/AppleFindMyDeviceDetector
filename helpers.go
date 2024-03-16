package main

import (
	"log"
	"os"
	"os/exec"
	"runtime"
)

// must is a helper function that wraps a call to a function returning an error and logs it if the error is non-nil.
func must(action string, err error) {
	if err != nil {
		log.Fatalf("Failed to %s: %v", action, err)
	}
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
