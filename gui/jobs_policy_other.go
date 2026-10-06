//go:build !windows

package main

import "errors"

func jobPolicySupported() bool            { return false }
func readRequireAdminForJobs() bool       { return false }
func writeRequireAdminForJobs(bool) error { return errors.New("not supported on this platform") }

// RequestElevation is only meaningful for the Windows job-protection policy.
func (a *App) RequestElevation() error {
	return errors.New("elevation is not supported on this platform")
}
