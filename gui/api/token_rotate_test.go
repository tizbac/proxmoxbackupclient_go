package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The service owns the local API token: it creates it root-only, regenerates
// it on a schedule, and keeps honouring the replaced one for a grace window so
// a GUI mid-request is not cut off. What it must never do is hand the secret
// to anyone who did not ask for elevation (0600, no group ownership).

func TestGenerateTokenIsRandomHex(t *testing.T) {
	a, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	b, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if len(a) != 64 {
		t.Errorf("token length = %d, want 64 hex chars (256 bits)", len(a))
	}
	if a == b {
		t.Error("two generated tokens are identical")
	}
	if strings.TrimFunc(a, func(r rune) bool {
		return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
	}) != "" {
		t.Errorf("token %q is not lowercase hex", a)
	}
}

func TestRotateTokenRewritesFileOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api-token")
	// A legacy build left the token group-readable (0640 + wheel group).
	if err := os.WriteFile(path, []byte("legacy-token"), 0o640); err != nil {
		t.Fatalf("seed token: %v", err)
	}

	token, err := RotateToken(path)
	if err != nil {
		t.Fatalf("RotateToken: %v", err)
	}
	if token == "legacy-token" || token == "" {
		t.Fatalf("RotateToken returned %q, want a fresh token", token)
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat token: %v", err)
	}
	if got := st.Mode().Perm(); got != 0o600 {
		t.Errorf("token file mode = %04o, want 0600 (no group/other read)", got)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read token: %v", err)
	}
	if strings.TrimSpace(string(b)) != token {
		t.Errorf("file holds %q, want the rotated token", strings.TrimSpace(string(b)))
	}
}

func TestEnsureTokenKeepsSecretButTightensLegacyPerms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(path, []byte("legacy-token"), 0o640); err != nil {
		t.Fatalf("seed token: %v", err)
	}

	got, err := EnsureToken(path)
	if err != nil {
		t.Fatalf("EnsureToken: %v", err)
	}
	if got != "legacy-token" {
		t.Errorf("EnsureToken = %q, want the existing token preserved", got)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat token: %v", err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Errorf("legacy 0640 token file left at %04o, want 0600", perm)
	}
}

func TestEnsureTokenCreatesPrivateToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api-token")

	first, err := EnsureToken(path)
	if err != nil {
		t.Fatalf("EnsureToken: %v", err)
	}
	if first == "" {
		t.Fatal("EnsureToken returned an empty token")
	}
	second, err := EnsureToken(path)
	if err != nil {
		t.Fatalf("EnsureToken (second): %v", err)
	}
	if second != first {
		t.Errorf("EnsureToken = %q, want the stored token %q on the second call", second, first)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat token: %v", err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Errorf("token file mode = %04o, want 0600", perm)
	}
}

func TestTokenFileOwnedByProcess(t *testing.T) {
	cases := []struct {
		uid, euid int
		want      bool
	}{
		{0, 0, true},       // root reading the service's own file
		{1000, 1000, true}, // a process reading its own file
		{0, 1000, false},   // unprivileged user vs the root-owned service token
		{1000, 0, true},    // root may read anything
	}
	for _, c := range cases {
		if got := tokenFileOwnedByProcess(c.uid, c.euid); got != c.want {
			t.Errorf("tokenFileOwnedByProcess(%d, %d) = %v, want %v", c.uid, c.euid, got, c.want)
		}
	}
}

func TestResolveTokenPrefersInMemoryTokenAndSkipsForeignFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(path, []byte("file-token"), 0o600); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	t.Setenv(TokenEnvVar, "")

	// A file we own is readable without elevation...
	if got := resolveToken(path); got != "file-token" {
		t.Errorf("resolveToken(file we own) = %q, want file-token", got)
	}

	// ...but the environment wins over it (launcher did the elevated fetch)...
	t.Setenv(TokenEnvVar, "env-token")
	if got := resolveToken(path); got != "env-token" {
		t.Errorf("resolveToken(env) = %q, want env-token", got)
	}

	// ...and an in-memory token from an elevated fetch wins over both.
	SetTokenOverride("override-token")
	t.Cleanup(func() { SetTokenOverride("") })
	if got := resolveToken(path); got != "override-token" {
		t.Errorf("resolveToken(override) = %q, want override-token", got)
	}

	// A missing file yields no token at all: the caller gets a 401 and the
	// elevated fetch runs instead of a silent connection.
	SetTokenOverride("")
	t.Setenv(TokenEnvVar, "")
	if got := resolveToken(filepath.Join(t.TempDir(), "nope")); got != "" {
		t.Errorf("resolveToken(missing file) = %q, want empty", got)
	}
}

func TestTokenRotationFromEnv(t *testing.T) {
	t.Setenv("PBSGO_TOKEN_ROTATION", "")
	t.Setenv("PBSGO_TOKEN_GRACE", "")
	interval, grace := TokenRotationFromEnv()
	if interval != DefaultTokenRotation || grace != DefaultTokenGrace {
		t.Errorf("defaults = (%s, %s), want (%s, %s)", interval, grace, DefaultTokenRotation, DefaultTokenGrace)
	}

	t.Setenv("PBSGO_TOKEN_ROTATION", "2h")
	t.Setenv("PBSGO_TOKEN_GRACE", "30m")
	interval, grace = TokenRotationFromEnv()
	if interval != 2*time.Hour || grace != 30*time.Minute {
		t.Errorf("overridden = (%s, %s), want (2h, 30m)", interval, grace)
	}

	// Unparsable falls back to the defaults instead of disabling rotation.
	t.Setenv("PBSGO_TOKEN_ROTATION", "banana")
	t.Setenv("PBSGO_TOKEN_GRACE", "later")
	interval, grace = TokenRotationFromEnv()
	if interval != DefaultTokenRotation || grace != DefaultTokenGrace {
		t.Errorf("invalid = (%s, %s), want the defaults", interval, grace)
	}

	// 0 means "disable rotation" (tests, installs without a daily prompt).
	t.Setenv("PBSGO_TOKEN_ROTATION", "0")
	interval, _ = TokenRotationFromEnv()
	if interval != 0 {
		t.Errorf("interval = %s, want 0 to disable rotation", interval)
	}
}

// currentToken reads the server's active token under its lock (tests share the
// package, so this is safe where a bare field read would not be).
func currentToken(s *Server) string {
	s.tokenMu.RLock()
	defer s.tokenMu.RUnlock()
	return s.token
}

func TestServerAcceptsPreviousTokenOnlyInsideGrace(t *testing.T) {
	s := NewServer("127.0.0.1:0", &fakeHandler{}, "token-a", "test")
	s.SetTokenGrace(80 * time.Millisecond)
	s.SetToken("token-b")

	if got := currentToken(s); got != "token-b" {
		t.Fatalf("current token = %q, want token-b", got)
	}
	if !s.tokenAccepted("token-b") {
		t.Error("new token must authenticate")
	}
	if !s.tokenAccepted("token-a") {
		t.Error("previous token must still authenticate inside the grace window")
	}
	if s.tokenAccepted("token-c") || s.tokenAccepted("") {
		t.Error("an unknown or empty token must never authenticate")
	}

	time.Sleep(120 * time.Millisecond)
	if s.tokenAccepted("token-a") {
		t.Error("previous token must stop authenticating once the grace window passed")
	}
	if !s.tokenAccepted("token-b") {
		t.Error("current token must outlive the grace window")
	}
}

func TestServerWithGraceZeroDropsPreviousTokenImmediately(t *testing.T) {
	s := NewServer("127.0.0.1:0", &fakeHandler{}, "token-a", "test")
	s.SetTokenGrace(0)
	s.SetToken("token-b")

	if s.tokenAccepted("token-a") {
		t.Error("with a zero grace window the replaced token must be dead on arrival")
	}
	if !s.tokenAccepted("token-b") {
		t.Error("new token must authenticate")
	}
}

func TestServerFailsClosedWithoutToken(t *testing.T) {
	s := NewServer("127.0.0.1:0", &fakeHandler{}, "", "test")
	for _, tok := range []string{"", "anything"} {
		if s.tokenAccepted(tok) {
			t.Errorf("tokenAccepted(%q) = true on a server with no token, want false", tok)
		}
	}
}

func TestTokenRotatorRotatesOnInterval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(path, []byte("initial-token"), 0o600); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	s := NewServer("127.0.0.1:0", &fakeHandler{}, "initial-token", "test")

	r := NewTokenRotator(s, path, 40*time.Millisecond, 30*time.Millisecond)
	r.Start()
	t.Cleanup(r.Stop)

	// Wait for the first rotation.
	deadline := time.Now().Add(3 * time.Second)
	for currentToken(s) == "initial-token" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	rotated := currentToken(s)
	if rotated == "initial-token" {
		t.Fatal("rotator never replaced the token")
	}

	// The file must always carry the token the server is enforcing, or the
	// next GUI to fetch it would authenticate with a dead secret.
	if b, err := os.ReadFile(path); err != nil {
		t.Fatalf("read token file: %v", err)
	} else if strings.TrimSpace(string(b)) != rotated {
		t.Errorf("file holds %q, server enforces %q", strings.TrimSpace(string(b)), rotated)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Errorf("rotated token file mode = %04o, want 0600", perm)
	}
	if !s.tokenAccepted(rotated) {
		t.Error("rotated token must authenticate")
	}

	// The replaced token keeps working for the grace window...
	if !s.tokenAccepted("initial-token") {
		t.Error("previous token must be honoured inside the grace window")
	}
	// ...but not beyond it.
	time.Sleep(60 * time.Millisecond)
	if s.tokenAccepted("initial-token") {
		t.Error("previous token must expire after the grace window")
	}

	// Stopping ends the rotations.
	r.Stop()
	stopped := currentToken(s)
	time.Sleep(120 * time.Millisecond)
	if currentToken(s) != stopped {
		t.Error("rotator kept rotating after Stop")
	}
}

func TestTokenRotatorDisabledByZeroInterval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(path, []byte("initial-token"), 0o600); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	s := NewServer("127.0.0.1:0", &fakeHandler{}, "initial-token", "test")

	r := NewTokenRotator(s, path, 0, DefaultTokenGrace)
	r.Start() // must not panic or start a goroutine
	t.Cleanup(r.Stop)

	time.Sleep(60 * time.Millisecond)
	if got := currentToken(s); got != "initial-token" {
		t.Errorf("token = %q, want no rotation when the interval is 0", got)
	}

	// Direct rotation still works with rotation disabled.
	if _, err := r.Rotate(); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if got := currentToken(s); got == "initial-token" {
		t.Error("Rotate() must still work when the ticker is disabled")
	}
}
