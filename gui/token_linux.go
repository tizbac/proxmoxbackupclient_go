//go:build linux
// +build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// elevatedFetchToken launches this same executable as root (pkexec, with sudo
// as fallback) as the token-fetch child and waits for the handoff file. The
// package launcher (pbsgo-gui) already performs this dance before exec'ing
// the GUI and exports PBSGO_API_TOKEN; this in-GUI path is the self-healing
// fallback for direct binary launches (developer builds, running the binary
// by hand).
func elevatedFetchToken(handoffFile string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot resolve own executable: %w", err)
	}

	var cmd *exec.Cmd
	switch {
	case executableAvailable("pkexec"):
		cmd = exec.Command("pkexec", exe, elevatedTokenFetchFlag, handoffFile)
	case executableAvailable("sudo"):
		cmd = exec.Command("sudo", "--non-interactive", exe, elevatedTokenFetchFlag, handoffFile)
	default:
		return "", fmt.Errorf("neither pkexec nor sudo is available for elevation")
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("cannot launch elevated token fetch: %w", err)
	}
	// The handoff file is the completion signal, not the child process;
	// reap the child in the background.
	go func() { _ = cmd.Wait() }()

	writeDebugLog("[ElevatedTokenFetch] launched elevated child, waiting for handoff")
	return waitForHandoff(handoffFile, 30*time.Second)
}

func executableAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
