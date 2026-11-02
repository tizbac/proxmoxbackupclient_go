//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// runElevatedHandoff starts this program as root through pkexec with flag
// and the handoff file, waits for it (timeout, then as long again polling
// the file), and returns the token it wrote into the file: the token-fetch
// copy (elevatedFetchToken, from upstream's token_linux.go), or the copy
// that starts the service first (service_start.go). Upstream fell back to
// "sudo --non-interactive", which only works with passwordless sudo; without
// pkexec this says so instead.
func runElevatedHandoff(flag, handoffFile string, timeout time.Duration) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot resolve own executable: %w", err)
	}
	if _, err := exec.LookPath("pkexec"); err != nil {
		return "", errors.New("asking for administrator rights needs pkexec (polkit), which is not installed")
	}
	cmd := exec.Command("pkexec", exe, flag, handoffFile) // #nosec G204 -- this program, and the file it created
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("cannot start the elevated copy: %w", err)
	}
	writeDebugLog(fmt.Sprintf("[ElevatedTokenFetch] %s: started the elevated copy, waiting for the handoff", flag))

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
				return "", ErrElevationDeclined
			}
			return "", &elevatedChildError{code: uint32(ee.ExitCode())} // #nosec G115 -- an exit status, 0 to 255
		}
		if err != nil {
			return "", fmt.Errorf("the elevated process failed: %w", err)
		}
		return "", errors.New("the elevated process exited without writing the token")
	case <-time.After(timeout):
		// Still running: most likely the password dialog is still open.
		writeDebugLog("[ElevatedTokenFetch] the elevated copy is still running; polling the handoff file")
		return waitForHandoff(handoffFile, timeout)
	}
}
