//go:build !service

package main

// "Install it for me", in the machine backup screen's box that says no
// snapshot module is loaded (Linux). It runs the elastio-snap installer the
// package bundles (snapshot.ModuleInstall): through pkexec, which asks for an
// administrator's password and names the installer in its dialog, or
// directly when the app runs as root. The installer's output goes to the
// window line by line ("snapmodule:line") and to the debug log. With Secure
// Boot on, the one-time password the user chose for approving the signing key
// at the next start goes to the installer on its standard input.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	"snapshot"
)

// snapModuleInstalling: one install at a time.
var snapModuleInstalling atomic.Bool

// What InstallSnapshotModule returns when the installer finished.
const (
	snapModuleLoaded  = "loaded"
	snapModuleRestart = "restart"
)

// snapModuleInstallStatus is the installer as GetSystemInfo reports it.
func snapModuleInstallStatus() snapshot.ModuleInstallStatus {
	return snapshot.InspectModuleInstall(snapshot.ModuleInstallerPath(product().FileBase))
}

// checkMokPassword checks the one-time password the window sends: 8 to 16
// letters and digits, as the firmware's key manager types them on any
// keyboard layout it shows.
func checkMokPassword(pw string) error {
	if n := len(pw); n < 8 || n > 16 {
		return errors.New("the one-time password must be 8 to 16 characters")
	}
	for _, r := range pw {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return errors.New("the one-time password may have letters and digits only")
		}
	}
	return nil
}

// InstallSnapshotModule runs the installer and returns "loaded" (the module
// is loaded now) or "restart" (installed; Secure Boot loads it after the
// restart at which the user approves its key with mokPassword).
// "ELEVATION_CANCELLED" when the password dialog was dismissed.
func (a *App) InstallSnapshotModule(mokPassword string) (string, error) {
	st := snapModuleInstallStatus()
	if !st.Available {
		return "", fmt.Errorf("the snapshot module installer cannot run here (%s)", st.Reason)
	}
	if st.SecureBoot {
		if err := checkMokPassword(mokPassword); err != nil {
			return "", err
		}
	} else {
		mokPassword = ""
	}
	if !snapModuleInstalling.CompareAndSwap(false, true) {
		return "", errors.New("the snapshot module is being installed already")
	}
	defer snapModuleInstalling.Store(false)

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	elevate := os.Geteuid() != 0
	writeDebugLog(fmt.Sprintf("[SnapModule] running %s (elevate=%v, secure boot=%v)", st.Script, elevate, st.SecureBoot))

	pr, pw := io.Pipe()
	m := snapshot.ModuleInstall{Script: st.Script, MokPassword: mokPassword, Stdout: pw, Stderr: pw}
	cmd, err := m.Command(ctx, elevate)
	if err != nil {
		return "", err
	}
	// started: the installer itself ran (its first line is "### 1/6");
	// before that, any output is pkexec's own.
	var started, noAgent atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		snapshot.ScanLines(pr, func(line string) {
			if strings.HasPrefix(line, "### ") {
				started.Store(true)
			} else if !started.Load() && strings.Contains(line, "authentication agent") {
				// "No authentication agent found", or, with no desktop
				// agent, "Error creating textual authentication agent"
				// (no terminal for its own prompt either).
				noAgent.Store(true)
			}
			writeDebugLog("[SnapModule] " + line)
			a.emitEvent("snapmodule:line", line)
		})
	}()
	err = cmd.Run()
	_ = pw.Close()
	wg.Wait()

	if elevate && !started.Load() {
		// pkexec exits 126 when its dialog is dismissed and 127 when not
		// authorised, before the installer starts. (127 later is a command
		// the installer could not find.) Without a polkit agent (no desktop
		// session) it cannot ask at all, which is not a refusal.
		var ee *exec.ExitError
		if errors.As(err, &ee) && (ee.ExitCode() == 126 || ee.ExitCode() == 127) {
			if noAgent.Load() {
				writeDebugLog("[SnapModule] pkexec found no polkit agent to ask for the password")
				return "", errors.New("nothing here can ask for an administrator's password (no polkit agent): run sudo salvanza-cli install-snapshot-module in a terminal instead")
			}
			writeDebugLog("[SnapModule] the password dialog was dismissed")
			return "", errElevationCancelled
		}
	}
	needsRestart, err := snapshot.ModuleInstallResult(err)
	if err != nil {
		writeDebugLog(fmt.Sprintf("[SnapModule] %v", err))
		return "", err
	}
	if needsRestart {
		writeDebugLog("[SnapModule] installed; the signing key is approved at the next start")
		return snapModuleRestart, nil
	}
	if _, ok := snapshot.DetectControl(); !ok {
		return "", errors.New("the installer finished, but no snapshot module is loaded: its output says more")
	}
	writeDebugLog("[SnapModule] installed and loaded")
	return snapModuleLoaded, nil
}
