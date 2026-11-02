//go:build windows && !service

package main

// Preferences > Advanced > Run as Service: install, start, stop and remove
// the background Windows service (ProxmoxBackupClientSVC.exe beside the app)
// from the GUI, until an installer does it.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// ServiceStatus describes the background service for Preferences.
type ServiceStatus struct {
	Supported    bool   `json:"supported"`
	Platform     string `json:"platform"`
	Installed    bool   `json:"installed"`
	Name         string `json:"name"`         // service name (installed, or the one Install would use)
	State        string `json:"state"`        // running, stopped, starting, stopping, paused, unknown
	StartType    string `json:"startType"`    // automatic, manual, disabled
	ExePath      string `json:"exePath"`      // what the installed service runs
	ExpectedExe  string `json:"expectedExe"`  // the service program beside this app
	ExeAvailable bool   `json:"exeAvailable"` // ExpectedExe exists
	OtherExe     bool   `json:"otherExe"`     // installed service runs a different program
	IsAdmin      bool   `json:"isAdmin"`
	ServiceMode  bool   `json:"serviceMode"` // this app hands backups and schedules to the service
	Message      string `json:"message,omitempty"`
}

// serviceNames returns the service name for this app's brand (the installer's
// convention: the app's exe name plus "SVC") and the older name the service's
// own "-service install" uses.
func serviceNames() (preferred string, all []string, expectedExe string) {
	exe, err := os.Executable()
	if err != nil {
		exe = "ProxmoxBackupClient.exe"
	}
	base := strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	preferred = base + "SVC"
	expectedExe = filepath.Join(filepath.Dir(exe), preferred+".exe")
	all = []string{preferred}
	if !strings.EqualFold(base, "ProxmoxBackupClientSVC") {
		all = append(all, "ProxmoxBackupClient")
	}
	return preferred, all, expectedExe
}

func stateName(s svc.State) string {
	switch s {
	case svc.Running:
		return "running"
	case svc.Stopped:
		return "stopped"
	case svc.StartPending, svc.ContinuePending:
		return "starting"
	case svc.StopPending, svc.PausePending:
		return "stopping"
	case svc.Paused:
		return "paused"
	}
	return "unknown"
}

func startTypeName(t uint32) string {
	switch t {
	case mgr.StartAutomatic:
		return "automatic"
	case mgr.StartManual:
		return "manual"
	case mgr.StartDisabled:
		return "disabled"
	}
	return "unknown"
}

// openInstalled finds this app's service. The connection only needs to read.
// serviceInstalled reports whether the background service is installed on
// this computer, running or not (resolveMode: an app started before the
// service answers must not run the computer's Backup Sets itself).
// It asks only for the rights a standard account has (mgr.Connect asks for
// all of them, which Windows refuses without elevation).
func serviceInstalled() bool {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseServiceHandle(scm) }()
	_, names, _ := serviceNames()
	for _, n := range names {
		p, err := windows.UTF16PtrFromString(n)
		if err != nil {
			continue
		}
		if h, err := windows.OpenService(scm, p, windows.SERVICE_QUERY_STATUS); err == nil {
			_ = windows.CloseServiceHandle(h)
			return true
		}
	}
	return false
}

func openInstalled(m *mgr.Mgr, names []string) (*mgr.Service, string) {
	for _, n := range names {
		if s, err := m.OpenService(n); err == nil {
			return s, n
		}
	}
	return nil, ""
}

// connectReadOnly opens the service manager for queries, which an ordinary
// user may do (mgr.Connect asks for full access and needs an administrator).
func connectReadOnly() (*mgr.Mgr, error) {
	h, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT|windows.SC_MANAGER_ENUMERATE_SERVICE)
	if err != nil {
		return nil, err
	}
	return &mgr.Mgr{Handle: h}, nil
}

func openServiceRead(m *mgr.Mgr, name string) (*mgr.Service, error) {
	h, err := windows.OpenService(m.Handle, windows.StringToUTF16Ptr(name), windows.SERVICE_QUERY_STATUS|windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		return nil, err
	}
	return &mgr.Service{Name: name, Handle: h}, nil
}

// GetServiceStatus reports whether the background service is installed and
// running, and whether this app is handing its work to it.
func (a *App) GetServiceStatus() ServiceStatus {
	preferred, names, expected := serviceNames()
	st := ServiceStatus{Supported: true, Platform: "windows", Name: preferred, ExpectedExe: expected, IsAdmin: isAdmin(), ServiceMode: a.mode == api.ModeService}
	if _, err := os.Stat(expected); err == nil {
		st.ExeAvailable = true
	}
	m, err := connectReadOnly()
	if err != nil {
		st.Message = fmt.Sprintf("Cannot read the Windows services: %v", err)
		return st
	}
	defer m.Disconnect()
	for _, n := range names {
		s, err := openServiceRead(m, n)
		if err != nil {
			continue
		}
		st.Installed, st.Name = true, n
		if q, err := s.Query(); err == nil {
			st.State = stateName(q.State)
		}
		if c, err := s.Config(); err == nil {
			st.StartType = startTypeName(c.StartType)
			st.ExePath = strings.Trim(strings.SplitN(c.BinaryPathName, `" `, 2)[0], `"`)
			st.OtherExe = !strings.EqualFold(filepath.Clean(st.ExePath), filepath.Clean(expected))
		}
		s.Close()
		break
	}
	return st
}

func needAdmin() error {
	if !isAdmin() {
		return errors.New("this needs an administrator: use Restart as administrator first")
	}
	return nil
}

func waitState(s *mgr.Service, want svc.State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		q, err := s.Query()
		if err != nil {
			return err
		}
		if q.State == want {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the service is %s after %s", stateName(q.State), timeout)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// InstallService installs the service program beside this app as an
// automatic Windows service running as Local System, starts it, and hands
// this app's backups and schedules to it.
func (a *App) InstallService() (ServiceStatus, error) {
	if err := needAdmin(); err != nil {
		return a.GetServiceStatus(), err
	}
	preferred, names, expected := serviceNames()
	if _, err := os.Stat(expected); err != nil {
		return a.GetServiceStatus(), fmt.Errorf("the service program %s is not here. It comes with the app download; put it beside %s", filepath.Base(expected), filepath.Base(strings.TrimSuffix(expected, "SVC.exe")+".exe"))
	}
	m, err := mgr.Connect()
	if err != nil {
		return a.GetServiceStatus(), fmt.Errorf("cannot open the Windows services: %w", err)
	}
	defer m.Disconnect()
	s, name := openInstalled(m, names)
	if s == nil {
		brand := BrandFromExecutable()
		s, err = m.CreateService(preferred, expected, mgr.Config{
			StartType:    mgr.StartAutomatic,
			DisplayName:  brand.Title + " Service",
			Description:  "Runs scheduled backups to Proxmox Backup Server with VSS support",
			Dependencies: []string{"RPCSS"},
		})
		if err != nil {
			return a.GetServiceStatus(), fmt.Errorf("could not install the service: %w", err)
		}
		name = preferred
		// Restart it if it ever stops unexpectedly.
		_ = s.SetRecoveryActions([]mgr.RecoveryAction{
			{Type: mgr.ServiceRestart, Delay: time.Minute},
			{Type: mgr.ServiceRestart, Delay: time.Minute},
			{Type: mgr.ServiceRestart, Delay: 5 * time.Minute},
		}, 24*60*60)
		LogMessage("Service", "info", fmt.Sprintf("Installed the background service %s (%s)", name, expected), "", "", nil)
	} else if c, cerr := s.Config(); cerr == nil && c.StartType == mgr.StartDisabled {
		c.StartType = mgr.StartAutomatic
		if err := s.UpdateConfig(c); err != nil {
			s.Close()
			return a.GetServiceStatus(), fmt.Errorf("could not enable the service: %w", err)
		}
	}
	defer s.Close()
	copyUserSettingsToService()
	if q, err := s.Query(); err == nil && q.State != svc.Running {
		if err := s.Start(); err != nil {
			return a.GetServiceStatus(), fmt.Errorf("installed, but the service did not start: %w", err)
		}
		if err := waitState(s, svc.Running, 30*time.Second); err != nil {
			return a.GetServiceStatus(), fmt.Errorf("installed, but %v", err)
		}
	}
	a.switchToServiceMode()
	return a.GetServiceStatus(), nil
}

// copyUserSettingsToService gives the service the settings this app used
// without it (the user's ~/.proxmox-backup-guardian), when the service's
// folder has none yet: the PBS servers, Backup Sets, history and Message Log
// carry on with the service. The user's own files are left as they were.
// Key files stay where the settings name them (the service reads them as
// SYSTEM); the service locks its folder down when it starts.
func copyUserSettingsToService() {
	src, err := userConfigDir()
	if err != nil {
		return
	}
	dst, err := serviceStateDir()
	if err != nil {
		return
	}
	for _, f := range []string{"config.json", "scheduled_jobs.json"} {
		if _, err := os.Stat(filepath.Join(dst, f)); err == nil {
			return // the service has settings of its own
		}
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		writeDebugLog(fmt.Sprintf("[Service] could not create %s: %v", dst, err))
		return
	}
	// Lock the folder before the secrets go in: under ProgramData it starts
	// readable by every user, and the service locks it only when it starts.
	if err := hardenConfigDir(dst); err != nil {
		writeDebugLog(fmt.Sprintf("[Service] not copying the settings: could not lock %s: %v", dst, err))
		return
	}
	n := 0
	for _, f := range []string{"config.json", "scheduled_jobs.json", "job_history.json", "message_log.json"} {
		data, err := os.ReadFile(filepath.Join(src, f)) // #nosec G304 -- the user's own settings folder
		if err != nil {
			continue
		}
		if err := atomicWriteFile(filepath.Join(dst, f), data, 0o600); err != nil {
			writeDebugLog(fmt.Sprintf("[Service] could not copy %s to %s: %v", f, dst, err))
			continue
		}
		n++
	}
	if n > 0 {
		LogMessage("Service", "info", fmt.Sprintf("Copied %d settings file(s) from %s to %s", n, src, dst), "", "", nil)
	}
}

// StartBackgroundService starts the installed service.
func (a *App) StartBackgroundService() (ServiceStatus, error) {
	if err := needAdmin(); err != nil {
		return a.GetServiceStatus(), err
	}
	_, names, _ := serviceNames()
	m, err := mgr.Connect()
	if err != nil {
		return a.GetServiceStatus(), err
	}
	defer m.Disconnect()
	s, _ := openInstalled(m, names)
	if s == nil {
		return a.GetServiceStatus(), errors.New("the service is not installed")
	}
	defer s.Close()
	if err := s.Start(); err != nil {
		return a.GetServiceStatus(), err
	}
	if err := waitState(s, svc.Running, 30*time.Second); err != nil {
		return a.GetServiceStatus(), err
	}
	a.switchToServiceMode()
	return a.GetServiceStatus(), nil
}

// stopService stops s if it is running. Refuses while it runs a backup.
func (a *App) stopService(s *mgr.Service) error {
	if a.mode == api.ModeService && a.apiClient != nil {
		if busy, label := a.serviceBusy(); busy {
			return fmt.Errorf("the service is running %s; stop it when that has finished", label)
		}
	}
	q, err := s.Query()
	if err != nil {
		return err
	}
	if q.State == svc.Stopped {
		return nil
	}
	if _, err := s.Control(svc.Stop); err != nil {
		return err
	}
	return waitState(s, svc.Stopped, 60*time.Second)
}

// StopBackgroundService stops the service; this app runs the schedules again.
func (a *App) StopBackgroundService() (ServiceStatus, error) {
	if err := needAdmin(); err != nil {
		return a.GetServiceStatus(), err
	}
	_, names, _ := serviceNames()
	m, err := mgr.Connect()
	if err != nil {
		return a.GetServiceStatus(), err
	}
	defer m.Disconnect()
	s, _ := openInstalled(m, names)
	if s == nil {
		return a.GetServiceStatus(), errors.New("the service is not installed")
	}
	defer s.Close()
	if err := a.stopService(s); err != nil {
		return a.GetServiceStatus(), err
	}
	a.switchToStandalone()
	return a.GetServiceStatus(), nil
}

// RemoveService stops and uninstalls the service; this app runs the
// schedules again. Backup Sets, history and settings are not touched.
func (a *App) RemoveService() (ServiceStatus, error) {
	if err := needAdmin(); err != nil {
		return a.GetServiceStatus(), err
	}
	_, names, _ := serviceNames()
	m, err := mgr.Connect()
	if err != nil {
		return a.GetServiceStatus(), err
	}
	defer m.Disconnect()
	s, name := openInstalled(m, names)
	if s == nil {
		a.switchToStandalone()
		return a.GetServiceStatus(), nil
	}
	if err := a.stopService(s); err != nil {
		s.Close()
		return a.GetServiceStatus(), err
	}
	err = s.Delete()
	s.Close()
	if err != nil {
		return a.GetServiceStatus(), fmt.Errorf("could not remove the service: %w", err)
	}
	LogMessage("Service", "info", fmt.Sprintf("Removed the background service %s", name), "", "", nil)
	a.switchToStandalone()
	return a.GetServiceStatus(), nil
}
