//go:build linux && !service

package main

// Preferences > Advanced > Run as Service on Linux: the background service is
// the systemd unit proxmoxbackupclient.service, running ProxmoxBackupClientSVC
// (beside the app) as root. The .deb and .rpm ship the unit, not enabled; a
// copy run from a folder writes its own unit to /etc/systemd/system.
//
// Installing moves the Backup Sets to the shared /var/lib/proxmoxbackupclient
// (see useSharedConfigDir): it copies the current user's settings there, then
// the app and the service both use that folder.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// ServiceStatus describes the background service for Preferences (same JSON
// as the Windows one, plus the platform and the shared folder).
type ServiceStatus struct {
	Supported    bool   `json:"supported"`
	Platform     string `json:"platform"`
	Installed    bool   `json:"installed"`
	Name         string `json:"name"`
	State        string `json:"state"`     // running, stopped, starting, stopping, unknown
	StartType    string `json:"startType"` // boot, manual, disabled
	ExePath      string `json:"exePath"`
	ExpectedExe  string `json:"expectedExe"`
	ExeAvailable bool   `json:"exeAvailable"`
	OtherExe     bool   `json:"otherExe"`
	IsAdmin      bool   `json:"isAdmin"`
	ServiceMode  bool   `json:"serviceMode"`
	SettingsDir  string `json:"settingsDir"`
	Message      string `json:"message,omitempty"`
}

const unitFileDir = "/etc/systemd/system"

// serviceExeOverride lets a test name the service program.
var serviceExeOverride string

func linuxServiceExe() string {
	if serviceExeOverride != "" {
		return serviceExeOverride
	}
	exe, err := os.Executable()
	if err != nil {
		return "/usr/lib/proxmoxbackupclient/ProxmoxBackupClientSVC"
	}
	return filepath.Join(filepath.Dir(exe), "ProxmoxBackupClientSVC")
}

// systemdUnit is what systemctl show reports about the unit.
type systemdUnit struct {
	LoadState, ActiveState, UnitFileState, FragmentPath, ExecPath string
}

var execPathRe = regexp.MustCompile(`path=(\S+)`)

// parseSystemctlShow reads "Key=Value" lines from systemctl show.
func parseSystemctlShow(out string) systemdUnit {
	var u systemdUnit
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch k {
		case "LoadState":
			u.LoadState = v
		case "ActiveState":
			u.ActiveState = v
		case "UnitFileState":
			u.UnitFileState = v
		case "FragmentPath":
			u.FragmentPath = v
		case "ExecStart":
			if m := execPathRe.FindStringSubmatch(v); m != nil {
				u.ExecPath = m[1]
			}
		}
	}
	return u
}

func (u systemdUnit) installed() bool { return u.LoadState == "loaded" }

func (u systemdUnit) state() string {
	switch u.ActiveState {
	case "active", "reloading":
		return "running"
	case "inactive", "failed":
		return "stopped"
	case "activating":
		return "starting"
	case "deactivating":
		return "stopping"
	}
	return "unknown"
}

func (u systemdUnit) startType() string {
	switch u.UnitFileState {
	case "enabled", "enabled-runtime", "static":
		return "boot"
	case "masked", "masked-runtime":
		return "disabled"
	case "":
		return ""
	}
	return "manual"
}

func systemctl(args ...string) (string, error) {
	out, err := exec.Command("systemctl", args...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return string(out), fmt.Errorf("systemctl %s: %s", strings.Join(args, " "), msg)
	}
	return string(out), nil
}

func showUnit() (systemdUnit, error) {
	out, err := systemctl("show", linuxServiceUnit+".service",
		"--property=LoadState,ActiveState,UnitFileState,FragmentPath,ExecStart")
	if err != nil {
		return systemdUnit{}, err
	}
	return parseSystemctlShow(out), nil
}

// GetServiceStatus reports whether the background service is installed and
// running, and whether this app is handing its work to it.
func (a *App) GetServiceStatus() ServiceStatus {
	expected := linuxServiceExe()
	st := ServiceStatus{Supported: true, Platform: "linux", Name: linuxServiceUnit + ".service",
		ExpectedExe: expected, IsAdmin: os.Geteuid() == 0, ServiceMode: a.mode == api.ModeService,
		SettingsDir: sharedConfigDir}
	if _, err := exec.LookPath("systemctl"); err != nil {
		st.Supported = false
		st.Message = "This computer does not use systemd, which the background service needs."
		return st
	}
	if _, err := os.Stat(expected); err == nil {
		st.ExeAvailable = true
	}
	u, err := showUnit()
	if err != nil {
		st.Message = err.Error()
		return st
	}
	if u.installed() {
		st.Installed = true
		st.State = u.state()
		st.StartType = u.startType()
		st.ExePath = u.ExecPath
		st.OtherExe = u.ExecPath != "" && filepath.Clean(u.ExecPath) != filepath.Clean(expected)
	}
	return st
}

func needAdmin() error {
	if os.Geteuid() != 0 {
		return errors.New("this needs an administrator: use Restart as administrator first")
	}
	return nil
}

// waitActive polls the unit until its state is want.
func waitActive(want string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		u, err := showUnit()
		if err != nil {
			return err
		}
		if u.state() == want {
			return nil
		}
		if u.ActiveState == "failed" && want == "running" {
			return errors.New("the service failed to start (journalctl -u " + linuxServiceUnit + " says why)")
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the service is %s after %s", u.state(), timeout)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// unitFileFor is the unit written when no packaged one is installed.
func unitFileFor(exe string) string {
	return `[Unit]
Description=Proxmox Backup Client service (scheduled backups)
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
ExecStart=` + exe + `
Restart=on-failure
RestartSec=60
TimeoutStopSec=90

[Install]
WantedBy=multi-user.target
`
}

// InstallService moves the settings to the shared folder, enables and starts
// the service, and hands this app's backups and schedules to it.
func (a *App) InstallService() (ServiceStatus, error) {
	if err := needAdmin(); err != nil {
		return a.GetServiceStatus(), err
	}
	exe := linuxServiceExe()
	if _, err := os.Stat(exe); err != nil {
		return a.GetServiceStatus(), fmt.Errorf("the service program %s is not here; it comes with the .deb and .rpm packages", exe)
	}
	u, err := showUnit()
	if err != nil {
		return a.GetServiceStatus(), err
	}
	if !u.installed() {
		path := filepath.Join(unitFileDir, linuxServiceUnit+".service")
		if err := atomicWriteFile(path, []byte(unitFileFor(exe)), 0o644); err != nil {
			return a.GetServiceStatus(), fmt.Errorf("could not write %s: %w", path, err)
		}
		if _, err := systemctl("daemon-reload"); err != nil {
			return a.GetServiceStatus(), err
		}
		LogMessage("Service", "info", "Wrote the service unit "+path, "", "", nil)
	}
	if err := moveSettingsToShared(); err != nil {
		return a.GetServiceStatus(), err
	}
	if _, err := systemctl("enable", "--now", linuxServiceUnit+".service"); err != nil {
		return a.GetServiceStatus(), fmt.Errorf("could not start the service: %w", err)
	}
	if err := waitActive("running", 30*time.Second); err != nil {
		return a.GetServiceStatus(), fmt.Errorf("installed, but %v", err)
	}
	LogMessage("Service", "info", "Installed and started the background service "+linuxServiceUnit, "", "", nil)
	a.switchToServiceMode()
	return a.GetServiceStatus(), nil
}

// moveSettingsToShared creates the shared settings folder and, if it holds no
// settings yet, copies the current user's there (the user's own folder is
// left as it was). The root app keeps the user's HOME (proxmoxbackupclient-root),
// so that is the user who installs the service.
func moveSettingsToShared() error {
	if err := os.MkdirAll(sharedConfigDir, 0o750); err != nil {
		return fmt.Errorf("could not create %s: %w", sharedConfigDir, err)
	}
	if !sharedSettingsPresent() {
		if src, err := userConfigDir(); err == nil {
			if info, err := os.Stat(src); err == nil && info.IsDir() {
				n, err := copyTree(src, sharedConfigDir)
				if err != nil {
					return fmt.Errorf("could not copy the settings from %s: %w", src, err)
				}
				LogMessage("Service", "info", fmt.Sprintf("Copied %d settings file(s) from %s to %s", n, src, sharedConfigDir), "", "", nil)
			}
		}
	}
	return hardenConfigDir(sharedConfigDir)
}

func sharedSettingsPresent() bool {
	for _, f := range []string{"config.json", "scheduled_jobs.json"} {
		if _, err := os.Stat(filepath.Join(sharedConfigDir, f)); err == nil {
			return true
		}
	}
	return false
}

// copyTree copies regular files and folders from src into dst (symbolic
// links and anything else are skipped) and returns how many files it copied.
func copyTree(src, dst string) (int, error) {
	n := 0
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o750)
		case d.Type().IsRegular():
			if err := copyFile(p, target); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// StartBackgroundService starts the installed service.
func (a *App) StartBackgroundService() (ServiceStatus, error) {
	if err := needAdmin(); err != nil {
		return a.GetServiceStatus(), err
	}
	if _, err := systemctl("start", linuxServiceUnit+".service"); err != nil {
		return a.GetServiceStatus(), err
	}
	if err := waitActive("running", 30*time.Second); err != nil {
		return a.GetServiceStatus(), err
	}
	a.switchToServiceMode()
	return a.GetServiceStatus(), nil
}

// stopLinuxService stops the service. Refuses while it runs a backup.
func (a *App) stopLinuxService() error {
	if a.mode == api.ModeService && a.apiClient != nil {
		if busy, label := a.serviceBusy(); busy {
			return fmt.Errorf("the service is running %s; stop it when that has finished", label)
		}
	}
	if _, err := systemctl("stop", linuxServiceUnit+".service"); err != nil {
		return err
	}
	return waitActive("stopped", 100*time.Second)
}

// StopBackgroundService stops the service; this app runs the schedules again.
func (a *App) StopBackgroundService() (ServiceStatus, error) {
	if err := needAdmin(); err != nil {
		return a.GetServiceStatus(), err
	}
	if err := a.stopLinuxService(); err != nil {
		return a.GetServiceStatus(), err
	}
	a.switchToStandalone()
	return a.GetServiceStatus(), nil
}

// RemoveService stops and disables the service (and deletes the unit if this
// app wrote it); this app runs the schedules again. Backup Sets, history and
// settings stay in the shared folder.
func (a *App) RemoveService() (ServiceStatus, error) {
	if err := needAdmin(); err != nil {
		return a.GetServiceStatus(), err
	}
	u, err := showUnit()
	if err != nil {
		return a.GetServiceStatus(), err
	}
	if !u.installed() {
		a.switchToStandalone()
		return a.GetServiceStatus(), nil
	}
	if err := a.stopLinuxService(); err != nil {
		return a.GetServiceStatus(), err
	}
	if _, err := systemctl("disable", linuxServiceUnit+".service"); err != nil {
		return a.GetServiceStatus(), err
	}
	own := filepath.Join(unitFileDir, linuxServiceUnit+".service")
	if u.FragmentPath == own {
		if err := os.Remove(own); err != nil {
			return a.GetServiceStatus(), fmt.Errorf("could not remove %s: %w", own, err)
		}
		_, _ = systemctl("daemon-reload")
	}
	LogMessage("Service", "info", "Removed the background service "+linuxServiceUnit, "", "", nil)
	a.switchToStandalone()
	return a.GetServiceStatus(), nil
}
