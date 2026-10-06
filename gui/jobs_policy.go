package main

import (
	"errors"
	"flag"
	"fmt"
)

// Scheduled-job protection.
//
// With the policy ON (the default), adding, changing or deleting a Backup Set
// needs an elevated (administrator) session. Running a set, viewing sets and
// history are never restricted. The policy value lives in the machine-wide
// registry (HKLM), which Windows already makes writable only by administrators,
// so the switch itself cannot be flipped by a standard user: the OS enforces
// that, not just this code. A missing value means ON, and an unreadable one is
// treated as ON (fail closed).
//
// The protection applies where the job list is a shared machine-wide store,
// which is Windows (ProgramData). On Linux the job list is the signed-in
// user's own file, so there is nothing to protect and the policy is not offered.

// elevatedRestartFlagName is passed to the copy relaunched through UAC so it
// waits for the instance that asked for elevation to exit. It must be a
// registered flag: main() calls flag.Parse, which exits with status 2 on an
// unknown one, and the elevated copy used to vanish a moment after the UAC prompt.
const elevatedRestartFlagName = "elevated-restart"

var _ = flag.Bool(elevatedRestartFlagName, false, "internal: relaunched elevated; wait for the previous instance to exit")

// errJobsAdminRequired is the stable code the frontend maps to a translated
// message and the "Restart as administrator" action.
var errJobsAdminRequired = errors.New("JOBS_ADMIN_REQUIRED")

// JobPolicy is what the UI needs to draw the Advanced switch and the notices.
type JobPolicy struct {
	Supported    bool `json:"supported"`     // policy applies on this platform
	RequireAdmin bool `json:"require_admin"` // current setting (ON by default)
	IsAdmin      bool `json:"is_admin"`      // this process is elevated
	CanModify    bool `json:"can_modify"`    // jobs may be changed right now
}

// jobsChangeAllowed is the one decision rule, kept free of OS calls so it can
// be unit tested.
func jobsChangeAllowed(supported, requireAdmin, admin bool) bool {
	if !supported || !requireAdmin {
		return true
	}
	return admin
}

// GetJobPolicy reports the policy and whether the caller may change jobs now.
func (a *App) GetJobPolicy() JobPolicy {
	supported := jobPolicySupported()
	req := true
	if supported {
		req = readRequireAdminForJobs()
	}
	admin := isAdmin()
	return JobPolicy{
		Supported:    supported,
		RequireAdmin: req,
		IsAdmin:      admin,
		CanModify:    jobsChangeAllowed(supported, req, admin),
	}
}

// SetRequireAdminForJobs switches the policy. It needs an elevated session; a
// standard user gets JOBS_ADMIN_REQUIRED (the registry write is refused by
// Windows, and we also check up front for a clean message).
func (a *App) SetRequireAdminForJobs(require bool) error {
	if !jobPolicySupported() {
		return fmt.Errorf("scheduled job protection is not available on this platform")
	}
	if !isAdmin() {
		return errJobsAdminRequired
	}
	if err := writeRequireAdminForJobs(require); err != nil {
		return err
	}
	writeDebugLog(fmt.Sprintf("Scheduled job protection set to require-admin=%v", require))
	return nil
}

// checkJobsChangeAllowed gates the GUI's own job add/update/delete.
func (a *App) checkJobsChangeAllowed() error {
	if jobsChangeAllowed(jobPolicySupported(), readRequireAdminForJobsOrDefault(), isAdmin()) {
		return nil
	}
	return errJobsAdminRequired
}

// checkJobsChangeAllowedViaAPI gates the service's HTTP job routes. The service
// runs as SYSTEM and cannot tell who is calling, so with the policy on these
// routes are refused outright; the GUI edits the job list directly (as an
// elevated process) and never uses them.
func (a *App) checkJobsChangeAllowedViaAPI() error {
	if jobPolicySupported() && readRequireAdminForJobsOrDefault() {
		return errJobsAdminRequired
	}
	return nil
}

func readRequireAdminForJobsOrDefault() bool {
	if !jobPolicySupported() {
		return false
	}
	return readRequireAdminForJobs()
}
