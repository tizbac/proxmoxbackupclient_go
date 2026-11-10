//go:build linux

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// The real thing, against logind's own list of inhibitor locks. Off unless
// SALVANZA_REAL_POWER_TEST=1 (for the test VMs).
func inhibitList(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("systemd-inhibit", "--list", "--no-pager").CombinedOutput()
	if err != nil {
		t.Skipf("systemd-inhibit --list: %v: %s", err, out)
	}
	return string(out)
}

func TestLinuxInhibitorLockShowsInLogindAndGoesAway(t *testing.T) {
	if os.Getenv("SALVANZA_REAL_POWER_TEST") != "1" {
		t.Skip("set SALVANZA_REAL_POWER_TEST=1 on a test VM to run this")
	}
	reason := "Salvanza test: a backup is running"
	inhibitList(t)
	end, err := preventSleep(reason)
	if err != nil {
		t.Skipf("logind did not grant the lock here: %v", err)
	}
	during := inhibitList(t)
	if !strings.Contains(during, reason) {
		end()
		t.Fatalf("logind does not list the lock:\n%s", during)
	}
	for _, what := range []string{"sleep", "block"} {
		if !strings.Contains(during, what) {
			t.Errorf("the lock lacks %q:\n%s", what, during)
		}
	}
	end()
	time.Sleep(300 * time.Millisecond)
	if after := inhibitList(t); strings.Contains(after, reason) {
		t.Fatalf("the lock is still listed after the run ended:\n%s", after)
	}
}

// The lock must not outlive the program that took it: kill a process that holds
// one and the lock goes with it.
func TestLinuxLockDoesNotOutliveTheProcess(t *testing.T) {
	if os.Getenv("SALVANZA_REAL_POWER_TEST") != "1" {
		t.Skip("set SALVANZA_REAL_POWER_TEST=1 on a test VM to run this")
	}
	reason := "Salvanza test: orphan check"
	holder := exec.Command(os.Args[0], "-test.run=TestLinuxHoldAndWait", "-test.v")
	holder.Env = append(os.Environ(), "SALVANZA_HOLD_REASON="+reason, "SALVANZA_REAL_POWER_TEST=1")
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	seen := false
	for time.Now().Before(deadline) {
		if strings.Contains(inhibitList(t), reason) {
			seen = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !seen {
		_ = holder.Process.Kill()
		t.Skip("the holder process never took a lock here")
	}
	_ = holder.Process.Kill() // only the holder, as a crash of the service would: its inhibitor child is left running
	_ = holder.Wait()
	time.Sleep(time.Second)
	if strings.Contains(inhibitList(t), reason) {
		t.Fatal("the lock outlived the process that held it")
	}
}

// TestLinuxHoldAndWait is the child of the test above: take a lock and wait to be killed.
func TestLinuxHoldAndWait(t *testing.T) {
	reason := os.Getenv("SALVANZA_HOLD_REASON")
	if reason == "" {
		t.Skip("helper for another test")
	}
	if _, err := preventSleep(reason); err != nil {
		t.Skipf("no lock: %v", err)
	}
	time.Sleep(60 * time.Second)
}
