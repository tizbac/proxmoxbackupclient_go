//go:build service
// +build service

package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/kardianos/service"
)

func main() {
	// Service configuration.
	//
	// On Windows the Name MUST equal the name the MSI registered the service
	// with (see service_name_windows.go); a mismatch makes the SCM refuse the
	// start with error 1061. On other platforms the systemd unit file names
	// the service explicitly, so the name here is only used for
	// install/uninstall actions.
	svcName, svcDisplayName := serviceIdentity()

	writeDebugLog(fmt.Sprintf("%s starting...", svcName))

	// Command-line flags for service control
	svcFlag := flag.String("service", "", "Control the system service: install, uninstall, start, stop, restart")
	flag.Parse()

	svcConfig := &service.Config{
		Name:        svcName,
		DisplayName: svcDisplayName,
		Description: "Executes scheduled backups to Proxmox Backup Server with VSS support",
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
	writeDebugLog(fmt.Sprintf("Starting service %q...", svcName))
	err = s.Run()
	if err != nil {
		writeDebugLog(fmt.Sprintf("service run failed: %v", err))
		log.Fatal(err)
	}
}
