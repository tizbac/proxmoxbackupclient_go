//go:build linux
// +build linux

package main

import (
	"errors"
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

	writeDebugLog("[ElevatedTokenFetch] launched elevated child, waiting for handoff")
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	select {
	case err := <-waitDone:
		// The child is done (it only reads the token file and exits, unless a
		// pkexec/sudo prompt was up), so its result is final: report the real
		// reason instead of polling a handoff file that will never be filled.
		token, readErr := readHandoff(handoffFile)
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) && readErr != nil {
				code := uint32(ee.ExitCode())
				return "", fmt.Errorf("elevated token fetch failed: child exited with code %s — %s",
					elevatedChildExitLabel(code), elevatedChildExitHint(code))
			}
			if readErr == nil {
				return token, nil
			}
			return "", fmt.Errorf("elevated token fetch failed: %w", err)
		}
		if readErr != nil {
			return "", errors.New("elevated token fetch failed: the child exited without writing the token")
		}
		return token, nil
	case <-time.After(elevatedChildTimeout):
		// Still running — typically a credential prompt the user is taking
		// their time with. Keep waiting on the handoff file instead of
		// abandoning a prompt that may still succeed.
		writeDebugLog("[ElevatedTokenFetch] child still running after the wait budget; polling the handoff file")
		return waitForHandoff(handoffFile, elevatedChildTimeout)
	}
}

func executableAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
