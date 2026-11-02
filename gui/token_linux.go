//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// elevatedFetchToken starts this program as root through pkexec as the
// token-fetch copy and waits for it, then reads the handoff file (from
// upstream's token_linux.go). Upstream fell back to "sudo
// --non-interactive", which only works with passwordless sudo; without
// pkexec this says so instead.
func elevatedFetchToken(handoffFile string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot resolve own executable: %w", err)
	}
	if _, err := exec.LookPath("pkexec"); err != nil {
		return "", errors.New("connecting to the background service needs pkexec (polkit), which is not installed")
	}
	cmd := exec.Command("pkexec", exe, elevatedTokenFetchFlag, handoffFile) // #nosec G204 -- this program, and the file it created
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("cannot start the elevated token fetch: %w", err)
	}
	writeDebugLog("[ElevatedTokenFetch] started the elevated copy, waiting for the handoff")

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		token, readErr := readHandoff(handoffFile)
		if readErr == nil {
			return token, nil
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			// pkexec exits 126 when the dialog is dismissed, 127 when not
			// authorised.
			if c := ee.ExitCode(); c == 126 || c == 127 {
				return "", errors.New("the request for administrator rights was declined")
			}
			code := uint32(ee.ExitCode())
			return "", fmt.Errorf("elevated token fetch failed: %s (%s)",
				elevatedChildExitLabel(code), elevatedChildExitHint(code))
		}
		if err != nil {
			return "", fmt.Errorf("elevated token fetch failed: %w", err)
		}
		return "", errors.New("elevated token fetch failed: the elevated process exited without writing the token")
	case <-time.After(elevatedChildTimeout):
		// Still running: most likely the password dialog is still open.
		writeDebugLog("[ElevatedTokenFetch] the elevated copy is still running; polling the handoff file")
		return waitForHandoff(handoffFile, elevatedChildTimeout)
	}
}
