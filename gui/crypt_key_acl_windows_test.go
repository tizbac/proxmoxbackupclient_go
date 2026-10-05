//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestrictKeyAccess(t *testing.T) {
	dir := t.TempDir()
	keys := filepath.Join(dir, "keys")
	if err := os.MkdirAll(keys, 0700); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(keys, "k.json")
	if err := os.WriteFile(f, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := restrictKeyAccess(keys, true); err != nil {
		t.Fatalf("dir: %v", err)
	}
	if err := restrictKeyAccess(f, false); err != nil {
		t.Fatalf("file: %v", err)
	}
	for _, p := range []string{keys, f} {
		out, err := exec.Command("icacls", p).CombinedOutput()
		if err != nil {
			t.Fatalf("icacls: %v %s", err, out)
		}
		s := string(out)
		for _, bad := range []string{`BUILTIN\Users`, "Authenticated Users", "Everyone"} {
			if strings.Contains(s, bad) {
				t.Errorf("%s still grants %s:\n%s", p, bad, s)
			}
		}
		if !strings.Contains(s, `NT AUTHORITY\SYSTEM:`) || !strings.Contains(s, `BUILTIN\Administrators:`) {
			t.Errorf("%s missing SYSTEM/Administrators:\n%s", p, s)
		}
		if strings.Contains(s, "(I)") {
			t.Errorf("%s still inherits:\n%s", p, s)
		}
	}
	if _, err := os.ReadFile(f); err != nil {
		t.Errorf("owner can no longer read key: %v", err)
	}
}
