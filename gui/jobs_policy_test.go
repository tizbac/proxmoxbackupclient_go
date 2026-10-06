package main

import (
	"flag"
	"testing"
)

func TestJobsChangeAllowed(t *testing.T) {
	cases := []struct {
		name                         string
		supported, requireAdmin, adm bool
		want                         bool
	}{
		{"unsupported platform never blocks", false, true, false, true},
		{"policy off allows standard user", true, false, false, true},
		{"policy on blocks standard user", true, true, false, false},
		{"policy on allows administrator", true, true, true, true},
		{"policy off allows administrator", true, false, true, true},
	}
	for _, c := range cases {
		if got := jobsChangeAllowed(c.supported, c.requireAdmin, c.adm); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

// The relaunched elevated copy is started with this flag. If it is not a
// registered flag, flag.Parse in main() exits with status 2 immediately.
func TestElevatedRestartFlagIsRegistered(t *testing.T) {
	if flag.Lookup(elevatedRestartFlagName) == nil {
		t.Fatalf("flag -%s must be registered, otherwise the elevated relaunch exits at startup", elevatedRestartFlagName)
	}
}
