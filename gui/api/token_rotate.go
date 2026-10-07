package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

const (
	// DefaultTokenRotation is how often the service regenerates the local API
	// token when PBSGO_TOKEN_ROTATION does not say otherwise.
	DefaultTokenRotation = 24 * time.Hour
	// DefaultTokenGrace is how long the previous token keeps authenticating
	// after a rotation (PBSGO_TOKEN_GRACE), so requests already in flight are
	// not cut off at the exact rotation instant.
	DefaultTokenGrace = 10 * time.Minute
)

// TokenRotationFromEnv returns the rotation interval and the grace window for
// the previous token. Unset or unparsable values fall back to the defaults;
// PBSGO_TOKEN_ROTATION=0 disables rotation entirely (tests, and installs that
// do not want a daily elevation prompt).
func TokenRotationFromEnv() (interval, grace time.Duration) {
	interval, grace = DefaultTokenRotation, DefaultTokenGrace
	if v := os.Getenv("PBSGO_TOKEN_ROTATION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		} else {
			log.Printf("PBSGO_TOKEN_ROTATION=%q is not a duration, using %s: %v", v, DefaultTokenRotation, err)
		}
	}
	if v := os.Getenv("PBSGO_TOKEN_GRACE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			grace = d
		} else {
			log.Printf("PBSGO_TOKEN_GRACE=%q is not a duration, using %s: %v", v, DefaultTokenGrace, err)
		}
	}
	return interval, grace
}

// GenerateToken returns a fresh 256-bit local-API token, hex-encoded.
func GenerateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// RotateToken writes a freshly generated token to path and returns it.
//
// The file is (re)written 0600: WriteFile does not touch the mode of an
// existing file, so the explicit chmod also migrates a token file left behind
// by an older build that made it group-readable. On Windows the DACL applied by
// EnsureToken survives the rewrite.
func RotateToken(path string) (string, error) {
	t, err := GenerateToken()
	if err != nil {
		return "", err
	}
	if err := writeTokenFile(path, t); err != nil {
		return "", err
	}
	return t, nil
}

// writeTokenFile stores token at path with owner-only permissions, tightening
// the mode of a pre-existing file as well.
func writeTokenFile(path, token string) error {
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		return fmt.Errorf("write api token %q: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("chmod api token %q: %w", path, err)
	}
	return nil
}

// TokenRotator regenerates the service's local API token on a fixed interval
// and hands each new token to the running API server, which keeps accepting
// the previous one for a grace window. A GUI that sits idle past that window is
// rejected with 401 on its next request and re-runs the elevated fetch (the
// 401 hook installed with api.SetUnauthorizedHook), so the secret never lives
// forever in a user session.
type TokenRotator struct {
	server   *Server
	path     string
	interval time.Duration
	grace    time.Duration

	stop     chan struct{}
	stopOnce sync.Once
}

// NewTokenRotator builds a rotator for the API server's token file. interval
// <= 0 disables the periodic rotation (Rotate can still be called directly).
// The grace window is pushed onto the server so it knows how long to honour the
// token it is about to replace.
func NewTokenRotator(server *Server, path string, interval, grace time.Duration) *TokenRotator {
	r := &TokenRotator{
		server:   server,
		path:     path,
		interval: interval,
		grace:    grace,
		stop:     make(chan struct{}),
	}
	server.SetTokenGrace(grace)
	return r
}

// Start begins rotating on the configured interval. It is a no-op when rotation
// is disabled.
func (r *TokenRotator) Start() {
	if r.interval <= 0 {
		log.Printf("local API token rotation disabled (PBSGO_TOKEN_ROTATION=%s)", r.interval)
		return
	}
	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-r.stop:
				return
			case <-ticker.C:
				if _, err := r.Rotate(); err != nil {
					log.Printf("local API token rotation failed: %v", err)
				}
			}
		}
	}()
	log.Printf("local API token rotates every %s (previous token accepted for %s)", r.interval, r.grace)
}

// Rotate installs a freshly generated token on the server first and only then
// publishes it in the token file. The order matters: while the file still
// holds the old token every client keeps authenticating (the server accepts
// the previous token for the grace window anyway), and nobody can be holding
// the new one yet — so there is no instant at which a client can read a token
// the server does not accept. If the file write fails, the server is rolled
// back to the previous token so file and memory never disagree about which
// secret is live.
func (r *TokenRotator) Rotate() (string, error) {
	next, err := GenerateToken()
	if err != nil {
		return "", err
	}
	prev := r.server.SetToken(next)
	if err := writeTokenFile(r.path, next); err != nil {
		r.server.SetToken(prev)
		return "", err
	}
	log.Printf("local API token rotated (previous one accepted for %s)", r.grace)
	return next, nil
}

// Stop ends the periodic rotation. Safe to call when Start never ran.
func (r *TokenRotator) Stop() {
	r.stopOnce.Do(func() { close(r.stop) })
}
