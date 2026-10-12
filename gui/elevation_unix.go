//go:build !windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// RequestElevation starts this app again as root and closes this copy once
// the root one is up. It uses the packaged launcher (proxmoxbackupclient-root:
// pkexec, keeping the user's display and home) when the app is the installed
// one, else pkexec directly. If the password prompt is cancelled the app keeps
// running and ELEVATION_CANCELLED is returned.
func (a *App) RequestElevation() error {
	if os.Geteuid() == 0 {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd, err := elevationCommand(exe)
	if err != nil {
		return err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // outlives this copy
	if err := cmd.Start(); err != nil {
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	deadline := time.After(3 * time.Minute) // time to type the password
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-exited:
			// pkexec: 126 = dialog dismissed, 127 = not authorised.
			if err != nil {
				return errors.New("ELEVATION_CANCELLED")
			}
			return nil // the root copy ran and has already closed
		case <-deadline:
			return nil // still waiting for the password; keep this copy
		case <-tick.C:
			if rootCopyRunning(exe) {
				a.RequestQuit()
				return nil
			}
		}
	}
}

func elevationCommand(exe string) (*exec.Cmd, error) {
	if launcher, err := exec.LookPath("proxmoxbackupclient-root"); err == nil &&
		filepath.Dir(exe) == "/usr/lib/proxmoxbackupclient" {
		return exec.Command(launcher), nil
	}
	pkexec, err := exec.LookPath("pkexec")
	if err != nil {
		return nil, errors.New("pkexec is not installed: start the app with sudo instead")
	}
	args := []string{"env"}
	for _, k := range []string{"HOME", "DISPLAY", "XAUTHORITY", "WAYLAND_DISPLAY", "XDG_RUNTIME_DIR"} {
		if v := os.Getenv(k); v != "" {
			args = append(args, k+"="+v)
		}
	}
	args = append(args, exe)
	return exec.Command(pkexec, args...), nil
}

// rootCopyRunning: is another copy of this program running as root?
func rootCopyRunning(exe string) bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	self := os.Getpid()
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		// /proc/<pid>/exe of a root process is not readable by a user; the
		// command line is, and the launchers exec the full path.
		cmdline, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil || strings.SplitN(string(cmdline), "\x00", 2)[0] != exe {
			continue
		}
		status, err := os.ReadFile(filepath.Join("/proc", e.Name(), "status"))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(status), "\n") {
			if f := strings.Fields(line); len(f) >= 3 && f[0] == "Uid:" && f[2] == "0" {
				return true
			}
		}
	}
	return false
}
