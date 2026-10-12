//go:build !windows

package main

import (
	"errors"
	"os"
)

// On Linux the policy is not a setting: once the background service is
// installed the Backup Sets live in its shared folder, which only root may
// change (hardenConfigDir), so changing them needs the app run as root,
// exactly as "Require administrator" does on Windows. Before that each user
// owns their own settings and there is nothing to protect.

func jobPolicySupported() bool      { return useSharedConfigDir() }
func readRequireAdminForJobs() bool { return true }
func writeRequireAdminForJobs(bool) error {
	return errors.New("on Linux the shared settings can only be changed by root; this is not a setting")
}

// jobPolicyFixed: the operating system enforces the policy, so it has no
// switch in Preferences.
func jobPolicyFixed() bool { return true }

// jobsAdmin: may this process change the shared Backup Sets?
func jobsAdmin() bool { return os.Geteuid() == 0 }
