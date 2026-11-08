//go:build !windows

package main

import (
	"errors"
	"os"
)

// On Linux the policy is not a setting either: with upstream's strict model
// only an app that fetched the service's token through pkexec reaches the
// service's Backup Sets, and without a service each user owns their own.

func jobPolicySupported() bool      { return false }
func readRequireAdminForJobs() bool { return true }
func writeRequireAdminForJobs(bool) error {
	return errors.New("on Linux the shared settings can only be changed by root; this is not a setting")
}

// jobPolicyFixed: the operating system enforces the policy, so it has no
// switch in Preferences.
func jobPolicyFixed() bool { return true }

// jobsAdmin: may this process change the shared Backup Sets?
func jobsAdmin() bool { return os.Geteuid() == 0 }
