package main

import (
	"log"
	"os"
	"os/exec"
	"runtime"

	"golang.org/x/crypto/ssh/terminal"
)

// must is a helper function that wraps a call to a function returning an error and logs it if the error is non-nil.
func must(action string, err error) {
	log.Printf("[DEBUG] Checking error for action: %s", action)
	if err != nil {
		log.Printf("[DEBUG] Fatal error occurred during %s: %v", action, err)
		log.Fatalf("Failed to %s: %v", action, err)
	}
	log.Printf("[DEBUG] Action completed successfully: %s", action)
}

// Executes whichever clear command exists for the OS running this application
// Supports Linux, Windows, and Mac OS
func clearScreen() {
	log.Printf("[DEBUG] Clearing screen for OS: %s", runtime.GOOS)
	cmd := exec.Command("clear") // Linux or macOS
	if runtime.GOOS == "windows" {
		log.Printf("[DEBUG] Using Windows clear command (cls)")
		cmd = exec.Command("cmd", "/c", "cls") // Windows
	} else {
		log.Printf("[DEBUG] Using Unix-like clear command")
	}
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		log.Printf("[DEBUG] Clear screen command failed: %v", err)
	} else {
		log.Printf("[DEBUG] Screen cleared successfully")
	}
}

func getTerminalHeight() (int, error) {
	log.Printf("[DEBUG] Attempting to get terminal dimensions")
	_, height, err := terminal.GetSize(0)
	if err != nil {
		log.Printf("[DEBUG] Failed to get terminal height: %v", err)
		return 0, err
	}
	log.Printf("[DEBUG] Terminal height retrieved: %d", height)
	return height, nil
}
