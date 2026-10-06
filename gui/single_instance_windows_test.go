//go:build windows

package main

import (
	"errors"
	"fmt"
	"syscall"
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
		{"wrapped already-exists", fmt.Errorf("x: %w", syscall.Errno(windows.ERROR_ALREADY_EXISTS)), true},
		{"unrelated failure", errors.New("boom"), false},
	}
	for _, c := range cases {
		if got := mutexHeldByAnotherInstance(c.err); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

// The real thing: the second CreateMutex on a name must be reported as held,
// using the error the call itself returns (not GetLastError).
func TestCreateMutexSecondCallIsDetected(t *testing.T) {
	name, _ := syscall.UTF16PtrFromString(`Local\PBSClientTestMutex-` + t.Name())
	h1, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	defer windows.CloseHandle(h1)
	if mutexHeldByAnotherInstance(err) {
		t.Fatal("the first creator must not see the mutex as held")
	}
	h2, err := windows.CreateMutex(nil, false, name)
	if h2 != 0 {
		defer windows.CloseHandle(h2)
	}
	if !mutexHeldByAnotherInstance(err) {
		t.Fatalf("the second creator must see the mutex as held, got err=%v", err)
	}
}
