//go:build windows

package pbscommon

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// withShutdowns replaces the two ways ShutdownComputer turns the computer
// off, counting the calls, for the length of the test.
func withShutdowns(t *testing.T, apiErr, cmdErr error) (apiCalls, cmdCalls *int) {
	t.Helper()
	a, c := 0, 0
	oldAPI, oldCmd := apiShutdown, commandShutdown
	apiShutdown = func(string) error { a++; return apiErr }
	commandShutdown = func() error { c++; return cmdErr }
	t.Cleanup(func() { apiShutdown, commandShutdown = oldAPI, oldCmd })
	return &a, &c
}

// The Windows API is tried first; shutdown.exe only when it fails.
func TestShutdownUsesTheAPI(t *testing.T) {
	api, cmd := withShutdowns(t, nil, nil)
	method, err := ShutdownComputer("test")
	if err != nil || *api != 1 || *cmd != 0 || method != "the Windows API" {
		t.Errorf("%q, %v, API called %d, shutdown.exe %d: want the Windows API, nil, 1, 0", method, err, *api, *cmd)
	}
}

func TestShutdownFallsBackToShutdownExe(t *testing.T) {
	api, cmd := withShutdowns(t, errors.New("this account does not hold SeShutdownPrivilege"), nil)
	method, err := ShutdownComputer("test")
	if err != nil || *api != 1 || *cmd != 1 || !strings.HasPrefix(method, "shutdown.exe (the Windows API failed: this account") {
		t.Errorf("%q, %v, API called %d, shutdown.exe %d", method, err, *api, *cmd)
	}
}

// When both fail, the error names both, for the log.
func TestShutdownBothFail(t *testing.T) {
	withShutdowns(t, errors.New("InitiateSystemShutdownEx: Access is denied."), errors.New("exit status 5"))
	_, err := ShutdownComputer("test")
	if err == nil || !strings.Contains(err.Error(), "InitiateSystemShutdownEx") || !strings.Contains(err.Error(), "shutdown.exe: exit status 5") {
		t.Errorf("got %v, want an error naming both attempts", err)
	}
}

// The reason recorded in the System event log: a planned shutdown by an
// application, for maintenance.
func TestShutdownReason(t *testing.T) {
	if shutdownReason != 0x80040001 {
		t.Errorf("reason %#x, want 0x80040001", shutdownReason)
	}
}

// TURNS THE COMPUTER OFF, for a test VM only: PBS_SHUTDOWN_TEST=api asks
// Windows through the API, =command through shutdown.exe. Passing means
// Windows accepted the request; the VM powering off is checked from outside.
func TestLiveShutdownRequest(t *testing.T) {
	var err error
	switch os.Getenv("PBS_SHUTDOWN_TEST") {
	case "api":
		err = shutdownViaAPI("Shutting down after a backup (live test of the Windows API).")
	case "command":
		err = shutdownViaCommand()
	default:
		t.Skip("turns the computer off: set PBS_SHUTDOWN_TEST=api or command, on a test VM only")
	}
	if err != nil {
		t.Fatalf("shutdown request refused: %v", err)
	}
	t.Log("Windows accepted the shutdown request")
}
