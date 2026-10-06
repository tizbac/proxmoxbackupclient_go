//go:build windows

package main

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

// Point the policy at a throwaway HKCU key so the test never touches HKLM.
func useTempPolicyKey(t *testing.T) {
	t.Helper()
	oldRoot, oldPath := jobPolicyRoot, jobPolicyPath
	jobPolicyRoot = registry.CURRENT_USER
	jobPolicyPath = `SOFTWARE\ProxmoxBackupClientTest\Policy`
	t.Cleanup(func() {
		registry.DeleteKey(registry.CURRENT_USER, jobPolicyPath)
		registry.DeleteKey(registry.CURRENT_USER, `SOFTWARE\ProxmoxBackupClientTest`)
		jobPolicyRoot, jobPolicyPath = oldRoot, oldPath
	})
	registry.DeleteKey(registry.CURRENT_USER, jobPolicyPath)
}

func TestJobPolicyDefaultsOn(t *testing.T) {
	useTempPolicyKey(t)
	if !readRequireAdminForJobs() {
		t.Fatal("a missing policy value must mean ON")
	}
}

func TestJobPolicyRoundTrip(t *testing.T) {
	useTempPolicyKey(t)
	if err := writeRequireAdminForJobs(false); err != nil {
		t.Fatal(err)
	}
	if readRequireAdminForJobs() {
		t.Fatal("expected OFF after writing 0")
	}
	if err := writeRequireAdminForJobs(true); err != nil {
		t.Fatal(err)
	}
	if !readRequireAdminForJobs() {
		t.Fatal("expected ON after writing 1")
	}
}

func TestJobPolicyBadValueFailsClosed(t *testing.T) {
	useTempPolicyKey(t)
	k, _, err := registry.CreateKey(jobPolicyRoot, jobPolicyPath, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	if err := k.SetStringValue(jobPolicyValue, "off"); err != nil { // wrong type
		t.Fatal(err)
	}
	if !readRequireAdminForJobs() {
		t.Fatal("an unreadable value must fail closed (ON)")
	}
}
