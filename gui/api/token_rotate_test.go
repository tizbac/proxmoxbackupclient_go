package api

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// From upstream, adapted: the file and the server must agree (an elevated
// fetch reads the file), and a replaced token keeps working only for the
// grace window.

func currentToken(s *Server) string {
	s.tokenMu.RLock()
	defer s.tokenMu.RUnlock()
	return s.token
}

func readTokenFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	return strings.TrimSpace(string(b))
}

func TestTokenRotationFromEnv(t *testing.T) {
	t.Setenv("PBSGO_TOKEN_ROTATION", "")
	t.Setenv("PBSGO_TOKEN_GRACE", "")
	if i, g := TokenRotationFromEnv(); i != DefaultTokenRotation || g != DefaultTokenGrace {
		t.Errorf("defaults = (%s, %s), want (%s, %s)", i, g, DefaultTokenRotation, DefaultTokenGrace)
	}

	t.Setenv("PBSGO_TOKEN_ROTATION", "2h")
	t.Setenv("PBSGO_TOKEN_GRACE", "30m")
	if i, g := TokenRotationFromEnv(); i != 2*time.Hour || g != 30*time.Minute {
		t.Errorf("set = (%s, %s), want (2h, 30m)", i, g)
	}

	// An unreadable value gives the default, not rotation off.
	t.Setenv("PBSGO_TOKEN_ROTATION", "banana")
	t.Setenv("PBSGO_TOKEN_GRACE", "later")
	if i, g := TokenRotationFromEnv(); i != DefaultTokenRotation || g != DefaultTokenGrace {
		t.Errorf("unreadable = (%s, %s), want the defaults", i, g)
	}

	t.Setenv("PBSGO_TOKEN_ROTATION", "0")
	if i, _ := TokenRotationFromEnv(); i != 0 {
		t.Errorf("interval = %s, want 0 (off)", i)
	}
}

func TestServerAcceptsReplacedTokenOnlyInsideGrace(t *testing.T) {
	s := NewServer("127.0.0.1:0", &fakeHandler{}, "token-a", "test")
	s.SetTokenGrace(80 * time.Millisecond)
	if prev := s.SetToken("token-b"); prev != "token-a" {
		t.Errorf("SetToken returned %q, want token-a", prev)
	}

	if !s.tokenAccepted("token-b") {
		t.Error("the new token must be accepted")
	}
	if !s.tokenAccepted("token-a") {
		t.Error("the replaced token must be accepted inside the grace window")
	}
	if s.tokenAccepted("token-c") || s.tokenAccepted("") {
		t.Error("an unknown or empty token must never be accepted")
	}

	time.Sleep(120 * time.Millisecond)
	if s.tokenAccepted("token-a") {
		t.Error("the replaced token must be refused after the grace window")
	}
	if !s.tokenAccepted("token-b") {
		t.Error("the current token must outlive the grace window")
	}
}

func TestServerWithNoTokenAcceptsNothing(t *testing.T) {
	s := NewServer("127.0.0.1:0", &fakeHandler{}, "", "test")
	for _, tok := range []string{"", "anything"} {
		if s.tokenAccepted(tok) {
			t.Errorf("tokenAccepted(%q) = true with no token set", tok)
		}
	}
}

func TestTokenRotatorRotatesOnInterval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(path, []byte("initial-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewServer("127.0.0.1:0", &fakeHandler{}, "initial-token", "test")
	// Count the files given the token file's permissions: only the
	// temporary file, before it replaces the live one.
	var secured, notTmp atomic.Int32
	prevSecure := SecureTokenFile
	SecureTokenFile = func(p string) error {
		if !strings.HasSuffix(p, ".tmp") {
			notTmp.Add(1)
		}
		secured.Add(1)
		return prevSecure(p)
	}
	t.Cleanup(func() { SecureTokenFile = prevSecure })
	r := NewTokenRotator(s, path, 40*time.Millisecond, 30*time.Millisecond)
	r.Start()
	t.Cleanup(r.Stop)

	deadline := time.Now().Add(3 * time.Second)
	for currentToken(s) == "initial-token" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	r.Stop()
	rotated := currentToken(s)
	if rotated == "initial-token" {
		t.Fatal("the token was never replaced")
	}
	// The file must hold the token the server requires: the GUI sends what
	// the file holds.
	if got := readTokenFile(t, path); got != rotated {
		t.Errorf("file holds %q, server requires %q", got, rotated)
	}
	if secured.Load() == 0 {
		t.Error("the new token file never got its permissions")
	}
	if notTmp.Load() != 0 {
		t.Error("permissions were set on the live token file, want the temporary file before the rename")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temporary token file left behind: %v", err)
	}

	time.Sleep(60 * time.Millisecond)
	if s.tokenAccepted("initial-token") {
		t.Error("the replaced token must expire after the grace window")
	}
	stopped := currentToken(s)
	time.Sleep(120 * time.Millisecond)
	if currentToken(s) != stopped {
		t.Error("the token kept changing after Stop")
	}
}

func TestTokenRotatorOffWithZeroInterval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(path, []byte("initial-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewServer("127.0.0.1:0", &fakeHandler{}, "initial-token", "test")
	r := NewTokenRotator(s, path, 0, DefaultTokenGrace)
	r.Start()
	t.Cleanup(r.Stop)

	time.Sleep(60 * time.Millisecond)
	if got := currentToken(s); got != "initial-token" {
		t.Errorf("token = %q, want no rotation with interval 0", got)
	}
	// Rotate still works by hand.
	if err := r.Rotate(); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if got := currentToken(s); got == "initial-token" || readTokenFile(t, path) != got {
		t.Errorf("after Rotate: server %q, file %q", got, readTokenFile(t, path))
	}
}

// A token file that cannot be written leaves the server on the token the
// file still holds.
func TestTokenRotatorKeepsTokenWhenFileWriteFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-dir", "api-token")
	s := NewServer("127.0.0.1:0", &fakeHandler{}, "initial-token", "test")
	r := NewTokenRotator(s, path, 0, DefaultTokenGrace)
	if err := r.Rotate(); err == nil {
		t.Fatal("Rotate into a missing folder succeeded")
	}
	if got := currentToken(s); got != "initial-token" {
		t.Errorf("token = %q after a failed rotation, want initial-token", got)
	}
}
