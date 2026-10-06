//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Overridable so tests can point at a throwaway HKCU key instead of HKLM.
var (
	jobPolicyRoot = registry.LOCAL_MACHINE
	jobPolicyPath = `SOFTWARE\ProxmoxBackupClient\Policy`
)

const jobPolicyValue = "RequireAdminForJobs"

// elevatedRestartFlag tells the relaunched copy to wait for the old instance to exit.
const elevatedRestartFlag = "--" + elevatedRestartFlagName

func jobPolicySupported() bool { return true }

// readRequireAdminForJobs: missing value = ON; any read problem = ON.
func readRequireAdminForJobs() bool {
	k, err := registry.OpenKey(jobPolicyRoot, jobPolicyPath, registry.QUERY_VALUE)
	if err != nil {
		return true
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue(jobPolicyValue)
	if err != nil {
		return true
	}
	return v != 0
}

func writeRequireAdminForJobs(require bool) error {
	k, _, err := registry.CreateKey(jobPolicyRoot, jobPolicyPath, registry.SET_VALUE)
	if err != nil {
		if errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
			return errJobsAdminRequired
		}
		return fmt.Errorf("open policy key: %w", err)
	}
	defer k.Close()
	var v uint32
	if require {
		v = 1
	}
	if err := k.SetDWordValue(jobPolicyValue, v); err != nil {
		if errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
			return errJobsAdminRequired
		}
		return fmt.Errorf("write policy value: %w", err)
	}
	return nil
}

// RequestElevation relaunches this program with a UAC prompt, then closes this
// instance so the elevated one takes over. If the user declines the prompt the
// app keeps running and an error is returned.
func (a *App) RequestElevation() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	args, _ := syscall.UTF16PtrFromString(elevatedRestartFlag)
	dir, _ := syscall.UTF16PtrFromString(filepath.Dir(exe))
	err = windows.ShellExecute(0, verb, file, args, dir, windows.SW_NORMAL)
	if err != nil {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return errors.New("ELEVATION_CANCELLED")
		}
		return err
	}
	// The elevated copy waits for this instance's single-instance lock to clear.
	a.RequestQuit()
	return nil
}
