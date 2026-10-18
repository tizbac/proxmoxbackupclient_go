//go:build windows

package main

import (
	"context"
	"os/exec"
)

// openInUserBrowser opens url in the default web browser without
// administrator rights: Explorer hands it to the desktop's own (unelevated)
// shell, where opening it directly from an elevated app would start the
// browser elevated too. Explorer's exit code means nothing here, so only a
// failure to start it is an error.
func openInUserBrowser(_ context.Context, url string) error {
	cmd := exec.Command("explorer.exe", url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
