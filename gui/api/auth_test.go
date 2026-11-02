package api

import (
	"os"
	"path/filepath"
	"testing"
)

// EnsureToken writes a token when there is none and keeps the one there
// across calls (a service restart), so open apps are not asked again.
func TestEnsureTokenKeepsToken(t *testing.T) {
	p := filepath.Join(t.TempDir(), "api-token")
	tok, err := EnsureToken(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != 64 {
		t.Fatalf("token %q", tok)
	}
	again, err := EnsureToken(p)
	if err != nil || again != tok {
		t.Fatalf("EnsureToken again = %q, %v; want %q", again, err, tok)
	}
	if _, err := os.Stat(p + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind: %v", err)
	}
}

// The token taken from memory (an elevated fetch) comes first, then the
// environment, then the file.
func TestResolveTokenOrder(t *testing.T) {
	t.Cleanup(func() { SetTokenOverride("") })
	p := filepath.Join(t.TempDir(), "api-token")
	tok, err := EnsureToken(p)
	if err != nil {
		t.Fatal(err)
	}
	SetTokenOverride("")
	t.Setenv(TokenEnvVar, "")
	if got := resolveToken(p); got != tok {
		t.Errorf("file: got %q, want the file's token", got)
	}
	t.Setenv(TokenEnvVar, "from-env")
	if got := resolveToken(p); got != "from-env" {
		t.Errorf("env: got %q", got)
	}
	SetTokenOverride("fetched")
	if got := resolveToken(p); got != "fetched" {
		t.Errorf("override: got %q", got)
	}
}

func TestTokenFileOwnedByProcess(t *testing.T) {
	for _, c := range []struct {
		file, euid int
		want       bool
	}{
		{1000, 1000, true}, // the owner
		{0, 0, true},       // root
		{1000, 0, true},    // root reads anything
		{0, 1000, false},   // the service's (root's) file, a user's app
		{1001, 1000, false},
	} {
		if got := tokenFileOwnedByProcess(c.file, c.euid); got != c.want {
			t.Errorf("file %d, euid %d: %v, want %v", c.file, c.euid, got, c.want)
		}
	}
}
