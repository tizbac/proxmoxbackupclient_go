package api

import (
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

// TokenEnvVar carries the local API token in the environment. The Linux GUI
// launcher (and the in-GUI elevated fetch) inject the service token this way so
// a normal user's GUI process never has to read the root-owned token file.
const TokenEnvVar = "PBSGO_API_TOKEN"

// tokenOverride takes precedence over the environment variable and the token
// file for every request when it is non-empty. The GUI sets it after a
// successful elevated token fetch — at launch and again whenever the service
// rotates the token (see SetUnauthorizedHook). It is guarded: the reauth
// transport writes it from whichever request notices the rotation, while every
// other goroutine reads it.
var (
	tokenOverrideMu sync.RWMutex
	tokenOverride   string
)

// SetTokenOverride installs the in-memory token handed over by an elevated
// fetch, replacing the previous one. An empty token clears it (used when a
// fetch succeeded but the service still rejects the token).
func SetTokenOverride(token string) {
	tokenOverrideMu.Lock()
	tokenOverride = token
	tokenOverrideMu.Unlock()
}

// GetTokenOverride returns the token installed by the last successful elevated
// fetch, or "" when there is none.
func GetTokenOverride() string {
	tokenOverrideMu.RLock()
	defer tokenOverrideMu.RUnlock()
	return tokenOverride
}

// tokenFileOwnedByProcess reports whether a token file owned by fileUID may be
// used without an elevation prompt by a process with euid: either this process
// IS that owner, or it is root (root can read any file anyway). A file owned by
// another account — the service's, root's — means the caller is not privileged:
// it must fetch the token through an elevation prompt instead of quietly
// authenticating against the privileged service.
func tokenFileOwnedByProcess(fileUID, euid int) bool {
	return euid == 0 || fileUID == euid
}

// resolveToken returns the token to attach to a request, in priority order:
// explicit override → environment → token file at path. An empty result means
// "no token available" and the request will simply be unauthenticated.
func resolveToken(path string) string {
	if t := GetTokenOverride(); t != "" {
		return t
	}
	if env := os.Getenv(TokenEnvVar); env != "" {
		return env
	}
	// The token file is the service's root-owned secret (0600 in a 0700
	// directory); see ReadOwnTokenFile for why an unprivileged process gets ""
	// here and therefore a 401, which triggers the elevated fetch
	// (pkexec/sudo, UAC) instead of a silent connection.
	return ReadOwnTokenFile(path)
}

// ReadOwnTokenFile returns the token stored at path only when this process is
// entitled to take it straight from the file — root, or the file's own owner,
// never merely a process that happens to be able to open it (group-readable
// leftovers of older installs are excluded on purpose) — and "" otherwise.
//
// It is the "no prompt needed" half of the elevated fetch: when we may already
// read the file, a pkexec/UAC child would only re-read the very same bytes, so
// the user must not be bothered with a prompt they could only decline and then
// get stuck behind.
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
		if !s.tokenAccepted(got) {
			s.writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
