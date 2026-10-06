//go:build windows

package main

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func TestMutexHeldByAnotherInstance(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"first instance (no error)", nil, false},
		{"mutex already exists", windows.ERROR_ALREADY_EXISTS, true},
		{"owned by an elevated instance (access denied)", windows.ERROR_ACCESS_DENIED, true},
		{"unrelated failure", errors.New("boom"), false},
	}
	for _, c := range cases {
		if got := mutexHeldByAnotherInstance(c.err); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

// A second launch must be refused. Before the fix both calls returned true.
func TestCheckSingleInstanceRefusesASecondLaunch(t *testing.T) {
	name := `Local` + "\\" + `PBSClientSingleInstanceTest`
	// keep the first caller's mutex alive for the duration of the test
	if !checkSingleInstanceNamed(name) {
		t.Fatal("the first launch must be allowed")
	}
	if checkSingleInstanceNamed(name) {
		t.Fatal("a second launch while the first is running must be refused")
	}
}
