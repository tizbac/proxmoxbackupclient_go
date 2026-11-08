//go:build windows

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

// openInUserBrowser opens url in the default web browser without
// administrator rights: Explorer hands it to the desktop's own (unelevated)
// shell, where opening it directly from an elevated app would start the
// browser elevated too. Explorer's exit code means nothing here, so only a
// failure to start it is an error.
func openInUserBrowser(_ context.Context, url string) error {
	explorer := "explorer.exe"
	if win := os.Getenv("SystemRoot"); win != "" {
		explorer = filepath.Join(win, "explorer.exe") // not whatever PATH finds first
	}
	cmd := exec.Command(explorer, url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
