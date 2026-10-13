//go:build service
// +build service

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kardianos/service"
)

func main() {
	// -list-sets, -undelete, -rollback: a command-line run, not the service.
	if runSetCLI(os.Args[1:]) {
		return
	}
	writeDebugLog("ProxmoxBackupClientSVC starting...")
	startStallWatchdog()

	// Command-line flags for service control
	svcFlag := flag.String("service", "", "Control the system service: install, uninstall, start, stop, restart")
	flag.Parse()

	// Service configuration. The name matches what the installers and
	// Preferences > Run as Service use (on Windows this program's own name,
	// ProxmoxBackupClientSVC; on Linux the packaged unit), so
	// "-service install" does not make a second one.
	name, display := windowsServiceIdentity()
	svcConfig := &service.Config{
		Name:        name,
		DisplayName: display,
		Description: "Runs scheduled backups to Proxmox Backup Server with VSS support",
	}
	if runtime.GOOS == "linux" {
		svcConfig.Name = linuxServiceUnit
		svcConfig.Description = "Runs scheduled backups to Proxmox Backup Server"
	}

	backupSvc := &BackupService{}
	s, err := service.New(backupSvc, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	// Handle service control commands (install, uninstall, start, stop)
	if len(*svcFlag) != 0 {
		err := service.Control(s, *svcFlag)
		if err != nil {
			log.Printf("Valid actions: %q\n", service.ControlAction)
			log.Fatal(err)
		}
		writeDebugLog(fmt.Sprintf("Service control action '%s' completed", *svcFlag))
		return
	}

	// Run service
	writeDebugLog("Starting service...")
	err = s.Run()
	if err != nil {
		log.Fatal(err)
	}
}

// windowsServiceIdentity gives the Windows service name and display name for
// this program: its exe name (ProxmoxBackupClientSVC, or a brand's own), and
// the brand title plus "Service", as the MSI and InstallService do.
func windowsServiceIdentity() (name, display string) {
	name = "ProxmoxBackupClientSVC"
	if exe, err := os.Executable(); err == nil {
		name = strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	}
	return name, ResolveBrand(strings.TrimSuffix(name, "SVC")).Title + " Service"
}
