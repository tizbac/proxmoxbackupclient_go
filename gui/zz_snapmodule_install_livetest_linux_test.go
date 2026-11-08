//go:build linux && !service

package main

// Live test of "Install it for me" (InstallSnapshotModule) on a Linux test
// computer with the package installed (the installer in
// /usr/share/salvanza/elastio-snap) and Secure Boot off. As root it runs the
// installer directly and must end with the module loaded; as a normal user
// with no desktop session, pkexec has no agent to ask for the password, which
// must say so (and name the terminal command), not report a failed install.
// Opt-in:
//
//	go test -c -o snapinst_livetest.test .
//	sudo SNAPINST_LIVE_TEST=1 ./snapinst_livetest.test -test.v -test.run TestInstallSnapshotModuleLive
//	SNAPINST_LIVE_TEST=1 ./snapinst_livetest.test -test.v -test.run TestInstallSnapshotModuleLive

import (
	"os"
	"strings"
	"testing"

	"snapshot"
)

func TestInstallSnapshotModuleLive(t *testing.T) {
	if os.Getenv("SNAPINST_LIVE_TEST") == "" {
		t.Skip("set SNAPINST_LIVE_TEST=1 to run this opt-in live test (it builds and loads elastio-snap)")
	}
	st := snapModuleInstallStatus()
	if !st.Available {
		t.Fatalf("the installer cannot run here: %+v", st)
	}
	if st.SecureBoot {
		t.Skip("Secure Boot is on: this test is for a computer without it")
	}
	a := &App{}
	result, err := a.InstallSnapshotModule("")
	if os.Geteuid() != 0 {
		if err == nil || !strings.Contains(err.Error(), "no polkit agent") {
			t.Fatalf("as a normal user without a polkit agent: got %q, %v; want the no-agent message", result, err)
		}
		t.Logf("normal user: %v", err)
		return
	}
	if err != nil || result != snapModuleLoaded {
		t.Fatalf("as root: got %q, %v; want %q", result, err, snapModuleLoaded)
	}
	if c, ok := snapshot.DetectControl(); !ok || c.Name != "elastio-snap" {
		t.Fatalf("after the install: module %+v, loaded %v", c, ok)
	}
	// A second install at once is refused while one runs.
	snapModuleInstalling.Store(true)
	_, err = a.InstallSnapshotModule("")
	snapModuleInstalling.Store(false)
	if err == nil {
		t.Fatal("a second install while one runs: want an error")
	}
}
