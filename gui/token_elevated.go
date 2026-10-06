package main

// token_elevated.go implements the elevated token-fetch protocol, shared by
// all platforms:
//
// The service's local-API token file is readable only by the privileged
// account (root on Linux, SYSTEM + Administrators on Windows). A
// normal-user GUI process therefore cannot read it directly. When the GUI
// probes the service and gets 401 (running, token missing), it:
//
//  1. creates a private handoff file in the user's temp dir (0600),
//  2. launches a copy of itself elevated (UAC "runas" on Windows,
//     pkexec/sudo on Linux) with `--elevated-token-fetch <handoffFile>`,
//  3. the elevated child reads the service token file, writes it to the
//     handoff file, and exits — it never starts the GUI,
//  4. the parent polls the handoff file, keeps the token in memory
//     (api.TokenOverride) and deletes the handoff file.
//
// The child handler MUST run before flag parsing and single-instance checks,
// so the elevated process does nothing else.

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// elevatedTokenFetchFlag is the argv marker identifying the elevated child.
const elevatedTokenFetchFlag = "--elevated-token-fetch"

// handleElevatedTokenFetchChild returns true (and exits the process) when this
// invocation IS the elevated child: it copies the service token into the
// handoff file and stops. It runs at the very top of main().
func handleElevatedTokenFetchChild(args []string) bool {
	for i, a := range args {
		if a != elevatedTokenFetchFlag || i+1 >= len(args) {
			continue
		}
		handoff := args[i + 1]
		writeDebugLog(fmt.Sprintf("[ElevatedTokenFetch] child: reading %s -> %s", serviceTokenPath(), handoff))

		token := ""
		if b, err := os.ReadFile(serviceTokenPath()); err == nil {
			token = strings.TrimSpace(string(b))
		} else {
			writeDebugLog(fmt.Sprintf("[ElevatedTokenFetch] child: cannot read token file: %v", err))
		}

		if token != "" {
			// The parent created the handoff file; open it WITHOUT creating
			// it, so a vanished file can never be re-created root-owned.
			f, err := os.OpenFile(handoff, os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				writeDebugLog(fmt.Sprintf("[ElevatedTokenFetch] child: cannot open handoff file: %v", err))
				os.Exit(1)
			}
			if _, err := f.WriteString(token); err != nil {
				writeDebugLog(fmt.Sprintf("[ElevatedTokenFetch] child: cannot write handoff file: %v", err))
				_ = f.Close()
				os.Exit(1)
			}
			_ = f.Close()
			writeDebugLog("[ElevatedTokenFetch] child: token handed off")
		}
		os.Exit(0)
	}
	return false
}

// elevatedFetchTokenWithHandoff creates the private handoff file, launches the
// platform elevation helper (token_windows.go / token_linux.go) and waits for
// the token. The token is returned in memory; the handoff file is deleted
// before returning.
func elevatedFetchTokenWithHandoff() (string, error) {
	tmp, err := os.CreateTemp("", "pbsgo-token-*")
	if err != nil {
		return "", fmt.Errorf("cannot create handoff file: %w", err)
	}
	name := tmp.Name()
	_ = tmp.Close()
	_ = os.Chmod(name, 0600)
	defer os.Remove(name)

	return elevatedFetchToken(name)
}

// waitForHandoff polls the handoff file until the elevated child writes the
// token or the deadline passes.
func waitForHandoff(name string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		b, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		if token := strings.TrimSpace(string(b)); token != "" {
			return token, nil
		}
	}
	return "", fmt.Errorf("elevated token fetch timed out after %s", timeout)
}
