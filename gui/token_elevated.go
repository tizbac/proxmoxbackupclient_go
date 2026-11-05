package main

// The elevated token fetch (from upstream's token_elevated.go).
//
// The service's API token file is readable only by the service's account and
// elevated administrators (root on Linux). An app run without elevation
// therefore cannot read it, and gets it like this:
//
//  1. it creates a private handoff file in the user's temp folder (0600),
//  2. it starts a copy of itself elevated (UAC "runas" on Windows, pkexec on
//     Linux) with --elevated-token-fetch <handoff file>,
//  3. that copy reads the token file, writes the token into the handoff file
//     and exits: it never starts a window,
//  4. the app reads the handoff file, keeps the token in memory
//     (api.SetTokenOverride) and deletes the file.
//
// It runs again when the service replaces the token under a running app
// (every 24 hours): the refused request runs the unauthorized hook
// (service_connect.go), which fetches again.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/salvanza/salvanza_client/gui/api"
)

// elevatedTokenFetchFlag marks the elevated copy on its command line.
const elevatedTokenFetchFlag = "--elevated-token-fetch"

// checkHandoffPath refuses a handoff path the app could not have created.
// The elevated copy writes to the path it is given, so only an absolute,
// regular (not a link) file is accepted: a FIFO or device would block, and a
// link could send the elevated write elsewhere.
func checkHandoffPath(p string) error {
	if !filepath.IsAbs(p) {
		return fmt.Errorf("not an absolute path: %q", p)
	}
	// #nosec G703 -- Lstat is the check itself: it looks at the path without
	// following a link or writing.
	fi, err := os.Lstat(filepath.Clean(p))
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %q", p)
	}
	return nil
}

// handleElevatedTokenFetchChild returns false, or exits the process when this
// is the elevated copy: it copies the service's token into the handoff file
// and stops. main() calls it first, before flags and the single-instance
// check.
func handleElevatedTokenFetchChild(args []string) bool {
	for i, a := range args {
		if a != elevatedTokenFetchFlag || i+1 >= len(args) {
			continue
		}
		handoff := args[i+1]
		writeDebugLog(fmt.Sprintf("[ElevatedTokenFetch] child: reading %s -> %s", getAPITokenPath(), handoff))
		if handoff == "" {
			os.Exit(exitChildBadArgument)
		}

		token := ""
		if b, err := os.ReadFile(getAPITokenPath()); err == nil {
			token = strings.TrimSpace(string(b))
		} else {
			writeDebugLog(fmt.Sprintf("[ElevatedTokenFetch] child: cannot read token file: %v", err))
		}
		if token == "" {
			writeDebugLog("[ElevatedTokenFetch] child: no token available, exiting")
			os.Exit(exitTokenFileUnread)
		}
		os.Exit(writeTokenHandoff("[ElevatedTokenFetch]", handoff, token))
	}
	return false
}

// writeTokenHandoff is the elevated copy's last step: it writes token into
// the handoff file at handoff and returns the copy's exit code. tag starts
// its log lines.
func writeTokenHandoff(tag, handoff, token string) int {
	// The path arrives on the command line of an elevated process: only
	// the plain file the app created, so the prompt the user approved
	// cannot be used to truncate any other file.
	if err := checkHandoffPath(handoff); err != nil {
		writeDebugLog(fmt.Sprintf("%s child: refusing handoff path: %v", tag, err))
		return exitHandoffUnusable
	}

	// Opened without O_CREATE (a file that vanished is never made again,
	// owned by the elevated account), without following a link where the
	// system allows, and not emptied until it is shown to be the file
	// checked above: the path could have been swapped meanwhile.
	before, err := os.Lstat(handoff) // #nosec G703 -- handoff was checked by checkHandoffPath just above
	if err != nil {
		return exitHandoffUnusable
	}
	// #nosec G703 -- handoff was checked by checkHandoffPath just above
	f, err := os.OpenFile(handoff, os.O_WRONLY|handoffNoFollow, 0o600)
	if err != nil {
		writeDebugLog(fmt.Sprintf("%s child: cannot open handoff file: %v", tag, err))
		return exitHandoffUnusable
	}
	if err := checkOpenedHandoff(f, before); err != nil {
		writeDebugLog(fmt.Sprintf("%s child: refusing handoff file: %v", tag, err))
		_ = f.Close()
		return exitHandoffUnusable
	}
	if err := f.Truncate(0); err != nil {
		_ = f.Close()
		return exitHandoffUnusable
	}
	if _, err := f.WriteString(token); err != nil {
		writeDebugLog(fmt.Sprintf("%s child: cannot write handoff file: %v", tag, err))
		_ = f.Close()
		return exitHandoffUnusable
	}
	_ = f.Close()
	writeDebugLog(tag + " child: token handed off")
	return exitTokenHandedOff
}

// fetchServiceToken gets the service's token: from the file when this app
// may read it (elevated, or root: no prompt), otherwise through the elevated
// copy. The token is kept in memory only; the handoff file is deleted.
func fetchServiceToken() (string, error) {
	if token := api.ReadOwnTokenFile(getAPITokenPath()); token != "" {
		writeDebugLog("[ElevatedTokenFetch] the token file is ours to read, no elevation needed")
		return token, nil
	}

	name, err := newHandoffFile()
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(name) }()

	return elevatedFetchToken(name)
}

// newHandoffFile creates the private handoff file an elevated copy writes
// the token into. The caller deletes it.
//
// The temp folder can be gone: on a Windows server each remote session has
// its own (Temp\2 for session 2), and a clean-up that empties Temp while the
// user stays signed in takes it away, so Connect failed with "cannot create
// handoff file ... The system cannot find the path specified" (deepthought,
// 2026-11-04). The folder is then made again, and failing that the app's own
// settings folder (private to the user too) takes the file.
func newHandoffFile() (string, error) {
	pattern := product().FileBase + "-token-*"
	tmp, err := os.CreateTemp("", pattern)
	if err != nil {
		if dir := os.TempDir(); os.MkdirAll(dir, 0o700) == nil {
			tmp, err = os.CreateTemp(dir, pattern)
		}
	}
	if err != nil {
		if dir, derr := userConfigDir(); derr == nil && os.MkdirAll(dir, 0o700) == nil {
			tmp, err = os.CreateTemp(dir, pattern)
		}
	}
	if err != nil {
		return "", fmt.Errorf("cannot create handoff file: %w", err)
	}
	name := tmp.Name()
	_ = tmp.Close()
	_ = os.Chmod(name, 0o600)
	return name, nil
}

// elevatedFetchToken starts this program elevated as the token-fetch copy
// and returns the token it hands back (runElevatedHandoff).
func elevatedFetchToken(handoffFile string) (string, error) {
	return runElevatedHandoff(elevatedTokenFetchFlag, handoffFile, elevatedChildTimeout)
}

// ErrElevationDeclined is a dismissed elevation prompt (UAC, or pkexec's
// password dialog).
var ErrElevationDeclined = errors.New("the request for administrator rights was declined")

// elevatedChildTimeout bounds one fetch: how long the app waits for the
// elevated copy (which takes milliseconds once the prompt is answered).
const elevatedChildTimeout = 60 * time.Second

// The elevated copy's exit codes.
const (
	exitTokenHandedOff    = 0
	exitHandoffUnusable   = 1 // handoff file could not be opened or written
	exitTokenFileUnread   = 2 // token file missing, unreadable or empty
	exitChildBadArgument  = 3 // --elevated-token-fetch without a path
	exitServiceNotStarted = 4 // --elevated-service-start: the service did not start
)

// elevatedChildError is an elevated copy that exited with code, not 0.
type elevatedChildError struct{ code uint32 }

func (e *elevatedChildError) Error() string {
	return fmt.Sprintf("the elevated process failed: %s (%s)", elevatedChildExitLabel(e.code), elevatedChildExitHint(e.code))
}

func elevatedChildExitLabel(code uint32) string {
	switch code {
	case exitHandoffUnusable:
		return "handoff file unusable"
	case exitTokenFileUnread:
		return "service token file unreadable"
	case exitChildBadArgument:
		return "bad handoff argument"
	case exitServiceNotStarted:
		return "the background service did not start"
	default:
		return fmt.Sprintf("exit code %d", code)
	}
}

func elevatedChildExitHint(code uint32) string {
	switch code {
	case exitTokenFileUnread:
		return "the elevated process could not read the local API token; is the service running?"
	case exitHandoffUnusable:
		return "the elevated process could not write the handoff file in the user's temp folder"
	case exitServiceNotStarted:
		return "the debug log says why; Windows Event Viewer or journalctl may say more"
	default:
		return "see the debug log for the elevated process's own report"
	}
}

// readHandoff returns the token the elevated copy wrote into name.
func readHandoff(name string) (string, error) {
	b, err := os.ReadFile(name) // #nosec G304 -- the handoff file this app created
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(b))
	if token == "" {
		return "", fmt.Errorf("handoff file %s is empty", name)
	}
	return token, nil
}

// waitForHandoff polls the handoff file until the token is in it or timeout
// has passed.
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
