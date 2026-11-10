//go:build windows

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The real thing, against Windows' own list of power requests. It changes the
// machine's power state while it runs, so it is only for the test VMs and is off
// unless SALVANZA_REAL_POWER_TEST=1; powercfg /requests needs an administrator.
func powercfgRequests(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("powercfg", "/requests").CombinedOutput()
	if err != nil {
		t.Skipf("powercfg /requests: %v (needs an administrator): %s", err, out)
	}
	return string(out)
}

func TestWindowsPowerRequestShowsInPowercfgAndGoesAway(t *testing.T) {
	if os.Getenv("SALVANZA_REAL_POWER_TEST") != "1" {
		t.Skip("set SALVANZA_REAL_POWER_TEST=1 on a test VM to run this")
	}
	reason := "Salvanza test: a backup is running " + t.Name()
	before := powercfgRequests(t)
	if strings.Contains(before, reason) {
		t.Fatal("the request is listed before it was made")
	}
	end, err := preventSleep(reason)
	if err != nil {
		t.Fatalf("the system refused the request: %v", err)
	}
	during := powercfgRequests(t)
	if !strings.Contains(during, reason) {
		end()
		t.Fatalf("powercfg /requests does not list the request:\n%s", during)
	}
	if !strings.Contains(during, "SYSTEM:") {
		t.Errorf("the request is not a SYSTEM request:\n%s", during)
	}
	end()
	after := powercfgRequests(t)
	if strings.Contains(after, reason) {
		t.Fatalf("the request is still listed after the run ended:\n%s", after)
	}
}

func TestWindowsTwoRunsShareOneRequestInPowercfg(t *testing.T) {
	if os.Getenv("SALVANZA_REAL_POWER_TEST") != "1" {
		t.Skip("set SALVANZA_REAL_POWER_TEST=1 on a test VM to run this")
	}
	reason := "Salvanza test: shared request"
	powercfgRequests(t)
	a, err := preventSleep(reason)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := preventSleep(reason)
	if n := strings.Count(powercfgRequests(t), reason); n < 1 {
		t.Fatalf("not listed with two runs")
	}
	a()
	if !strings.Contains(powercfgRequests(t), reason) {
		t.Fatalf("one run ending released the request of the other")
	}
	b()
	if strings.Contains(powercfgRequests(t), reason) {
		t.Fatalf("still listed after both ended")
	}
}
