package api

import (
	"crypto/subtle"
	"log"
	"os"
	"sync"
	"time"
)

// The service takes a new API token at every start (NewToken) and replaces
// it on a schedule while it runs, so a token copied out of memory or the file
// stops working within a day (from upstream). An app holding the replaced
// token gets a 401 once the grace window is over and fetches the new one
// through elevation again (reauth.go); the grace window keeps requests
// already on their way from being refused.

const (
	// DefaultTokenRotation is how often the running service replaces the
	// token, unless PBSGO_TOKEN_ROTATION says otherwise.
	DefaultTokenRotation = 24 * time.Hour
	// DefaultTokenGrace is how long the replaced token is still accepted,
	// unless PBSGO_TOKEN_GRACE says otherwise.
	DefaultTokenGrace = 10 * time.Minute
)

// TokenRotationFromEnv returns the rotation interval and grace window.
// The variable names are upstream's, so a setting made for 1.2.0-rc still
// applies. An unset or unreadable value gives the default;
// PBSGO_TOKEN_ROTATION=0 turns rotation off.
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

// SetToken makes token the one the server requires and returns the one it
// replaced, which is still accepted for the grace window.
func (s *Server) SetToken(token string) string {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	prev := s.token
	if prev != "" && prev != token {
		s.prevToken = prev
		s.prevValidUntil = time.Now().Add(s.tokenGrace)
	}
	s.token = token
	return prev
}

// SetTokenGrace sets how long a replaced token is still accepted.
func (s *Server) SetTokenGrace(grace time.Duration) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	s.tokenGrace = grace
}

// tokenAccepted reports whether got is the current token, or the replaced
// one inside its grace window. With no current token nothing is accepted
// (fail closed, H-01).
func (s *Server) tokenAccepted(got string) bool {
	if got == "" {
		return false
	}
	s.tokenMu.RLock()
	cur, prev, until := s.token, s.prevToken, s.prevValidUntil
	s.tokenMu.RUnlock()
	if cur == "" {
		return false
	}
	// Both compares always run, so the time taken does not tell which of the
	// two a guess matched.
	current := subtle.ConstantTimeCompare([]byte(got), []byte(cur)) == 1
	previous := prev != "" && subtle.ConstantTimeCompare([]byte(got), []byte(prev)) == 1
	return current || (previous && time.Now().Before(until))
}

// TokenRotator replaces the service's API token on a fixed interval.
type TokenRotator struct {
	server   *Server
	path     string
	interval time.Duration
	grace    time.Duration

	stop     chan struct{}
	stopOnce sync.Once
}

// NewTokenRotator returns a rotator for the server's token file. An interval
// of 0 or less turns the schedule off (Rotate still works).
func NewTokenRotator(server *Server, path string, interval, grace time.Duration) *TokenRotator {
	server.SetTokenGrace(grace)
	return &TokenRotator{server: server, path: path, interval: interval, grace: grace, stop: make(chan struct{})}
}

// Start rotates on the interval until Stop.
func (r *TokenRotator) Start() {
	if r.interval <= 0 {
		log.Printf("local API token rotation off (PBSGO_TOKEN_ROTATION=%s)", r.interval)
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
				if err := r.Rotate(); err != nil {
					log.Printf("local API token rotation failed: %v", err)
				}
			}
		}
	}()
	log.Printf("local API token rotates every %s (the replaced one is accepted for %s)", r.interval, r.grace)
}

// Rotate makes the server require a new token and then writes it to the
// token file (upstream's order), so nothing can read a token the server does
// not accept yet. If the file cannot be written, the server goes back to the
// token in the file.
func (r *TokenRotator) Rotate() error {
	next, err := generateToken()
	if err != nil {
		return err
	}
	prev := r.server.SetToken(next)
	if err := replaceTokenFile(r.path, next); err != nil {
		r.server.SetToken(prev)
		return err
	}
	log.Printf("local API token rotated (the replaced one is accepted for %s)", r.grace)
	return nil
}

// Stop ends the rotation. Safe to call more than once, or without Start.
func (r *TokenRotator) Stop() {
	r.stopOnce.Do(func() { close(r.stop) })
}
