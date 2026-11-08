//go:build windows || service

package main

// serviceName is the background service's name: on Windows the service
// program's own name (salvanza-svc); the Linux systemd unit is
// <FileBase>.service (service_names.go).
func serviceName() string { return serviceExeBase() }
