//go:build !windows

package api

import (
	"os"
	"path/filepath"
	"testing"
)

// A token file an earlier build left group-readable (0640, sudo or wheel)
// is made owner-only when the service starts.
func TestEnsureTokenTightensOldFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(p, []byte("old-token"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o640); err != nil {
		t.Fatal(err)
	}
	tok, err := EnsureToken(p)
	if err != nil || tok != "old-token" {
		t.Fatalf("EnsureToken = %q, %v", tok, err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", fi.Mode().Perm())
	}
}

// A new token file is owner-only from the start.
func TestEnsureTokenNewFileOwnerOnly(t *testing.T) {
	p := filepath.Join(t.TempDir(), "api-token")
	if _, err := EnsureToken(p); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", fi.Mode().Perm())
	}
}
