package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

// tokenHeader carries the shared local API token. Requiring a CUSTOM header also
// blocks browser-originated requests: a web page cannot set it on a cross-origin
// request without a CORS preflight, which this server never approves (audit H-01
// / v2-H-06). The token authenticates the GUI to the privileged local service.
const tokenHeader = "X-Proxmox-Client-Token"

// TokenEnvVar carries the local API token in the environment (upstream's name),
// for a launcher that fetched it already. The app itself never sets it: it keeps
// the token it fetched in memory (SetTokenOverride).
const TokenEnvVar = "PBSGO_API_TOKEN"

// maxRequestBody bounds request bodies to avoid unbounded memory from a local
// caller (audit: "borner les tailles de body").
const maxRequestBody = 8 << 20 // 8 MiB: a settings import goes over as base64

// The token file is the service's secret: SYSTEM and the Administrators group
// only on Windows, which a program running without elevation cannot use even
// under an administrator account (its Administrators SID is deny-only), and
// root 0600 on Linux, without the sudo/wheel group of earlier builds (from
// upstream, which removed that group as a vulnerability). The app gets the
// token from an elevated copy of itself (token_elevated.go in the gui
// package) and keeps it in memory here.
var (
	tokenOverrideMu sync.RWMutex
	tokenOverride   string
)

// SecureTokenFile gives the token file at path its permissions. On Linux and
// other Unix systems it is chmod 0600 (auth_unix.go). On Windows the service
// sets it to the ACL code in the gui package (which has golang.org/x/sys);
// until then it fails, so a service that forgot to set it writes no token and
// accepts no request (fail closed).
var SecureTokenFile = secureTokenFileDefault

// SetTokenOverride installs the token an elevated fetch handed over, replacing
// the previous one; "" clears it.
func SetTokenOverride(token string) {
	tokenOverrideMu.Lock()
	tokenOverride = token
	tokenOverrideMu.Unlock()
}

// GetTokenOverride returns the token installed by SetTokenOverride, or "".
func GetTokenOverride() string {
	tokenOverrideMu.RLock()
	defer tokenOverrideMu.RUnlock()
	return tokenOverride
}

// tokenFileOwnedByProcess reports whether a process with euid may take a
// token file owned by fileUID straight from disk: it is that owner, or root.
// A file owned by another account means the caller is not privileged and must
// fetch the token through an elevation prompt instead.
func tokenFileOwnedByProcess(fileUID, euid int) bool {
	return euid == 0 || fileUID == euid
}

// resolveToken returns the token to send: the fetched one, then the
// environment, then the token file when this process may read it directly
// (ReadOwnTokenFile). "" means none, and the service answers 401.
func resolveToken(path string) string {
	if t := GetTokenOverride(); t != "" {
		return t
	}
	if env := os.Getenv(TokenEnvVar); env != "" {
		return env
	}
	return ReadOwnTokenFile(path)
}

// ReadOwnTokenFile returns the token stored at path when this process may take
// it from the file without a prompt: the service itself, root, or an elevated
// app on Windows. Otherwise "", so the app asks for elevation (or, without a
// window to ask in, stays unconnected) instead of authenticating quietly.
func ReadOwnTokenFile(path string) string {
	if !canReadTokenFile(path) {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// EnsureToken returns the token stored at path, writing a new one if there is
// none, and (re)applies the file's permissions (SecureTokenFile). Tests use
// it; the service takes a new token at every start (NewToken).
func EnsureToken(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			if err := SecureTokenFile(path); err != nil {
				return "", err
			}
			return t, nil
		}
	}
	t, err := generateToken()
	if err != nil {
		return "", err
	}
	if err := replaceTokenFile(path, t); err != nil {
		return "", err
	}
	return t, nil
}

// NewToken writes a new random token to path, replacing any token there, and
// returns it. The service calls it when it starts, so a token copied earlier,
// or planted before the service first ran, stops working at the next start.
func NewToken(path string) (string, error) {
	t, err := generateToken()
	if err != nil {
		return "", err
	}
	if err := replaceTokenFile(path, t); err != nil {
		return "", err
	}
	return t, nil
}

// generateToken returns a new random 256-bit token, hex encoded.
func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// replaceTokenFile writes t to path through a temporary file and a rename,
// so a reader sees the old token or the new one, never part of one. The
// temporary file gets the token file's permissions before the rename.
func replaceTokenFile(path, t string) error {
	tmp := path + ".tmp"
	_ = os.Remove(tmp)
	if err := os.WriteFile(tmp, []byte(t), 0o600); err != nil {
		return fmt.Errorf("write api token %q: %w", tmp, err)
	}
	if err := SecureTokenFile(tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace api token %q: %w", path, err)
	}
	return nil
}

// tokenTransport adds the token header to every request (resolveToken).
type tokenTransport struct {
	tokenPath string
	base      http.RoundTripper
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if token := resolveToken(t.tokenPath); token != "" {
		req.Header.Set(tokenHeader, token)
	}
	return t.base.RoundTrip(req)
}

// authMiddleware enforces the local-token auth and request hardening on every
// route: it bounds the body, rejects browser-originated requests (Origin set),
// and requires the shared token (tokenAccepted).
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)

		// Our GUI never sets Origin; a browser always does on cross-origin POST.
		if r.Header.Get("Origin") != "" {
			s.writeError(w, "forbidden", http.StatusForbidden)
			return
		}

		if !s.tokenAccepted(r.Header.Get(tokenHeader)) {
			s.writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
