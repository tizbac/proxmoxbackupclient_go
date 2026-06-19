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
	writeDebugLog("NimbusBackupSVC starting...")

	// Command-line flags for service control
	svcFlag := flag.String("service", "", "Control the system service: install, uninstall, start, stop, restart")
	flag.Parse()

	// Service configuration
	svcConfig := &service.Config{
		Name:        "NimbusBackup",
		DisplayName: "Nimbus Backup SVC",
		Description: "Executes scheduled backups to Proxmox Backup Server with VSS support",
	}

	nimbusSvc := &NimbusService{}
	s, err := service.New(nimbusSvc, svcConfig)
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
