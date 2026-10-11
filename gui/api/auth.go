package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// tokenHeader carries the shared local API token. Requiring a CUSTOM header also
// blocks browser-originated requests: a web page cannot set it on a cross-origin
// request without a CORS preflight, which this server never approves (audit H-01
// / v2-H-06). The token authenticates the GUI to the privileged local service.
const tokenHeader = "X-Proxmox-Client-Token"

// maxRequestBody bounds request bodies to avoid unbounded memory from a local
// caller (audit: "borner les tailles de body").
const maxRequestBody = 1 << 20 // 1 MiB

// EnsureToken returns the local API token stored at path, generating and writing
// a fresh random one (0600) if the file is missing or empty. Called by the
// service before it starts the API server.
//
// On Windows 0600 means nothing; the file inherits the settings folder's ACL,
// which hardenConfigDir (gui/config_acl_windows.go) limits to SYSTEM,
// Administrators and the local administrator accounts. The custom-header and
// Origin checks close the browser/CSRF vector regardless.
func EnsureToken(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			return t, nil
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api token: %w", err)
	}
	t := hex.EncodeToString(buf)
	if err := os.WriteFile(path, []byte(t), 0o600); err != nil {
		return "", fmt.Errorf("write api token %q: %w", path, err)
	}
	return t, nil
}

// NewToken writes a fresh random token to path, replacing any token already
// there, and returns it. The service calls it each time it starts, so a token
// copied earlier, or a token file planted before the service first ran, stops
// working at the next restart. The GUI reads the file on every request, so it
// picks the new token up without a restart.
func NewToken(path string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api token: %w", err)
	}
	t := hex.EncodeToString(buf)
	tmp := path + ".tmp"
	_ = os.Remove(tmp)
	if err := os.WriteFile(tmp, []byte(t), 0o600); err != nil {
		return "", fmt.Errorf("write api token %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("replace api token %q: %w", path, err)
	}
	return t, nil
}

// tokenTransport injects the shared token header into every client request,
// reading the token file on each call so the GUI picks up a token the service
// created after the GUI started.
type tokenTransport struct {
	tokenPath string
	base      http.RoundTripper
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if b, err := os.ReadFile(t.tokenPath); err == nil {
		req.Header.Set(tokenHeader, strings.TrimSpace(string(b)))
	}
	return t.base.RoundTrip(req)
}

// authMiddleware enforces the local-token auth and request hardening on every
// route: it bounds the body, rejects browser-originated requests (Origin set),
// and requires the shared token via a constant-time compare.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)

		// Our GUI never sets Origin; a browser always does on cross-origin POST.
		if r.Header.Get("Origin") != "" {
			s.writeError(w, "forbidden", http.StatusForbidden)
			return
		}

		got := r.Header.Get(tokenHeader)
		if s.token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
			s.writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
