//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUserHomeOfUnix(t *testing.T) {
	homes := t.TempDir()
	old := homesDirectory
	homesDirectory = homes
	t.Cleanup(func() { homesDirectory = old })
	docs := filepath.Join(homes, "alice", "Documents")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := userHomeOf(docs); !ok || got != filepath.Join(homes, "alice") {
		t.Errorf("userHomeOf(%s) = %q %v", docs, got, ok)
	}
	if got := rollbackSafetyRoot(docs); got != filepath.Join(homes, "alice", ".pbs-rollback") {
		t.Errorf("safety folder %s", got)
	}
	if _, ok := userHomeOf(homes); ok {
		t.Error("the homes folder itself is nobody's home")
	}
	outside := t.TempDir()
	if _, ok := userHomeOf(outside); ok {
		t.Errorf("%s is not in a home folder", outside)
	}
}

// The safety folder is its owner's alone, also when an earlier version made
// it open.
func TestSecureSafetyFolderUnix(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".pbs-rollback")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := secureSafetyFolder(dir); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o700 {
		t.Errorf("mode %o, want 700", perm)
	}
}
