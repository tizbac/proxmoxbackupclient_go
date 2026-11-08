package api

import (
	"net/http"
	"net/http/httptest"
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
	t.Setenv(LegacyTokenEnvVar, "")
	if got := resolveToken(p); got != tok {
		t.Errorf("file: got %q, want the file's token", got)
	}
	t.Setenv(LegacyTokenEnvVar, "from-legacy-env")
	if got := resolveToken(p); got != "from-legacy-env" {
		t.Errorf("legacy env: got %q", got)
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

// The token is taken under X-Salvanza-Token and, for scripts written before
// the rename, X-Proxmox-Client-Token; any other header is refused.
func TestTokenHeaderNames(t *testing.T) {
	s := NewServer("127.0.0.1:0", &fakeHandler{}, "tok", "test")
	h := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for name, want := range map[string]int{
		"X-Salvanza-Token":       http.StatusNoContent,
		"X-Proxmox-Client-Token": http.StatusNoContent,
		"X-Other-Token":          http.StatusUnauthorized,
	} {
		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		req.Header.Set(name, "tok")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("%s: %d, want %d", name, rec.Code, want)
		}
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
