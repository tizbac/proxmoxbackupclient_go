//go:build !windows && !linux && !service

package main

import "errors"

// ServiceStatus describes the background service for Preferences. Only the
// Windows service and the Linux systemd unit exist; elsewhere schedules run
// in the app.
type ServiceStatus struct {
	Supported bool   `json:"supported"`
	Installed bool   `json:"installed"`
	Message   string `json:"message,omitempty"`
}

var errNoService = errors.New("the background service is only available on Windows and Linux")

func (a *App) GetServiceStatus() ServiceStatus { return ServiceStatus{} }
func (a *App) InstallService() (ServiceStatus, error) {
	return ServiceStatus{}, errNoService
}
func (a *App) StartBackgroundService() (ServiceStatus, error) {
	return ServiceStatus{}, errNoService
}
func (a *App) StopBackgroundService() (ServiceStatus, error) {
	return ServiceStatus{}, errNoService
}
func (a *App) RemoveService() (ServiceStatus, error) {
	return ServiceStatus{}, errNoService
}
