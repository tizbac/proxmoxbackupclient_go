package api

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

// tokenHeader carries the shared local API token. Requiring a CUSTOM header also
// blocks browser-originated requests: a web page cannot set it on a cross-origin
// request without a CORS preflight, which this server never approves (audit H-01
// / v2-H-06). The token authenticates the GUI to the privileged local service.
const tokenHeader = "X-Proxmox-Client-Token"

// TokenEnvVar carries the local API token in the environment. The Linux GUI
// launcher (and the in-GUI elevated fetch) inject the service token this way so
// a normal user's GUI process never has to read the root-owned token file.
const TokenEnvVar = "PBSGO_API_TOKEN"

// TokenOverride takes precedence over the environment variable and the token
// file for every request when it is non-empty. The GUI sets it after a
// successful elevated token fetch.
var TokenOverride string

// resolveToken returns the token to attach to a request, in priority order:
// explicit override → environment → token file at path. An empty result means
// "no token available" and the request will simply be unauthenticated.
func resolveToken(path string) string {
	if TokenOverride != "" {
		return TokenOverride
	}
	if env := os.Getenv(TokenEnvVar); env != "" {
		return env
	}
	if b, err := os.ReadFile(path); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			return t
		}
	}
	return ""
}

// maxRequestBody bounds request bodies to avoid unbounded memory from a local
// caller (audit: "borner les tailles de body").
const maxRequestBody = 1 << 20 // 1 MiB

// tokenTransport injects the shared token header into every client request,
// reading the token file on each call so the GUI picks up a token the service
// created after the GUI started.
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
