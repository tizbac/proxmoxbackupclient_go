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
//     (api.SetTokenOverride) and deletes the handoff file.
//
// The same protocol runs again whenever the service's token is rotated away
// under a running GUI: the rejected request triggers the hook installed by
// api.SetUnauthorizedHook, which repeats this fetch (token_refresh.go) instead
// of failing forever.
//
// The child handler MUST run before flag parsing and single-instance checks,
// so the elevated process does nothing else.

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
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
		handoff := args[i+1]
		writeDebugLog(fmt.Sprintf("[ElevatedTokenFetch] child: reading %s -> %s", serviceTokenPath(), handoff))
		if handoff == "" {
			os.Exit(exitChildBadArgument)
		}

		token := ""
		if b, err := os.ReadFile(serviceTokenPath()); err == nil {
			token = strings.TrimSpace(string(b))
		} else {
			writeDebugLog(fmt.Sprintf("[ElevatedTokenFetch] child: cannot read token file: %v", err))
		}
		if token == "" {
			// Tell the parent WHY: it waits on this exit code instead of
			// polling a handoff file that will never be filled.
			writeDebugLog("[ElevatedTokenFetch] child: no token available, exiting")
			os.Exit(exitTokenFileUnread)
		}

		// The parent created the handoff file; open it WITHOUT creating
		// it, so a vanished file can never be re-created root-owned.
		f, err := os.OpenFile(handoff, os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			writeDebugLog(fmt.Sprintf("[ElevatedTokenFetch] child: cannot open handoff file: %v", err))
			os.Exit(exitHandoffUnusable)
		}
		if _, err := f.WriteString(token); err != nil {
			writeDebugLog(fmt.Sprintf("[ElevatedTokenFetch] child: cannot write handoff file: %v", err))
			_ = f.Close()
			os.Exit(exitHandoffUnusable)
		}
		_ = f.Close()
		writeDebugLog("[ElevatedTokenFetch] child: token handed off")
		os.Exit(exitTokenHandedOff)
	}
	return false
}

// elevatedFetchTokenWithHandoff creates the private handoff file, launches the
// platform elevation helper (token_windows.go / token_linux.go) and waits for
// the token. The token is returned in memory; the handoff file is deleted
// before returning.
func elevatedFetchTokenWithHandoff() (string, error) {
	// When we are already privileged enough to read the service token file
	// (root, an elevated Windows process, or its owner), read it here instead
	// of launching a child that would read exactly the same bytes: a prompt
	// the user can decline would otherwise leave the GUI with a token it
	// knows to be dead (it happens on every rotation of a privileged
	// session). Everyone else still goes through the handoff below — the gate
	// in api.ReadOwnTokenFile is what keeps "connect without asking" closed.
	if token := api.ReadOwnTokenFile(serviceTokenPath()); token != "" {
		writeDebugLog("[ElevatedTokenFetch] token file is ours to read, no elevation needed")
		return token, nil
	}

	tmp, err := os.CreateTemp("", "pbsgo-token-*")
	if err != nil {
		return "", fmt.Errorf("cannot create handoff file: %w", err)
	}
	name := tmp.Name()
	_ = tmp.Close()
	_ = os.Chmod(name, 0600)
	defer func() { _ = os.Remove(name) }()

	return elevatedFetchToken(name)
}

// elevatedChildTimeout bounds one elevated token fetch: how long the parent
// waits for the child before giving up (the child itself finishes in
// milliseconds — it only reads a file — so hitting this is a failure, not
// normal operation).
const elevatedChildTimeout = 60 * time.Second

// elevated child exit codes, mirrored in handleElevatedTokenFetchChild. They
// live here rather than in the Windows-only file because the child handler is
// platform-independent — the Windows parent just reports them.
const (
	exitTokenHandedOff   = 0
	exitHandoffUnusable  = 1 // handoff file could not be opened/written
	exitTokenFileUnread  = 2 // service token file missing, unreadable or empty
	exitChildBadArgument = 3 // --elevated-token-fetch without a path
)

func elevatedChildExitLabel(code uint32) string {
	switch code {
	case exitHandoffUnusable:
		return "handoff file unusable"
	case exitTokenFileUnread:
		return "service token file unreadable"
	case exitChildBadArgument:
		return "bad handoff argument"
	default:
		return fmt.Sprintf("exit code %d", code)
	}
}

func elevatedChildExitHint(code uint32) string {
	switch code {
	case exitTokenFileUnread:
		return "the elevated process could not read the local API token; is the service running?"
	case exitHandoffUnusable:
		return "the elevated process could not write the handoff file in the user temp directory"
	default:
		return "see the debug log for the child's own report"
	}
}

// readHandoff returns the token the elevated child wrote into name, or an
// error when the file is missing, empty or unreadable.
func readHandoff(name string) (string, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(b))
	if token == "" {
		return "", fmt.Errorf("handoff file %s is empty", name)
	}
	return token, nil
}

// waitForHandoff polls the handoff file until the elevated child writes the
// token or the deadline passes.
func waitForHandoff(name string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		if token, err := readHandoff(name); err == nil {
			return token, nil
		}
	}
	return "", fmt.Errorf("elevated token fetch timed out after %s", timeout)
}
