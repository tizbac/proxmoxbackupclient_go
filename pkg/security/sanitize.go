package security

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// SanitizeForLog masks sensitive data for logging
// Shows first 4 and last 4 chars, masks the middle
func SanitizeForLog(s string) string {
	if s == "" {
		return ""
	}

	length := len(s)

	// For very short strings, mask completely
	if length <= 8 {
		return "***"
	}

	// For longer strings, show first and last 4 chars
	return s[:4] + "..." + s[length-4:]
}

// SanitizeSecret completely masks a secret value
func SanitizeSecret(s string) string {
	if s == "" {
		return ""
	}
	return "***REDACTED***"
}

// SanitizeURL removes credentials from URLs for logging
func SanitizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "***INVALID_URL***"
	}

	// Remove user info (username:password@)
	parsed.User = nil

	return parsed.String()
}

// isLoopbackHost reports whether host (a URL hostname, no port) is an exact
// loopback target: "localhost", a 127.0.0.0/8 IPv4, or ::1. Used to allow plain
// HTTP only for true loopback — never for "localhost.<something>" hostnames.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// ValidateURL ensures URL is safe and uses HTTPS
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Require HTTPS unless the host is a TRUE loopback target. A substring check
	// (strings.Contains host "localhost"/"127.0.0.1") let hosts like
	// "localhost.attacker.tld" or "127.0.0.1.evil.tld" pass the HTTP exemption and
	// receive the PBS token in cleartext (audit v2-H-07). Match on the parsed
	// hostname only.
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return fmt.Errorf("only HTTPS URLs allowed (got %s)", parsed.Scheme)
	}

	// Check for valid hostname
	if parsed.Host == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	return nil
}

// ValidateBackupID prevents path traversal attacks
func ValidateBackupID(id string) error {
	if id == "" {
		return fmt.Errorf("backup ID cannot be empty")
	}

	// No path traversal
	if strings.Contains(id, "..") {
		return fmt.Errorf("backup ID cannot contain '..'")
	}

	if strings.Contains(id, "/") || strings.Contains(id, "\\") {
		return fmt.Errorf("backup ID cannot contain path separators")
	}

	// No control characters
	for _, r := range id {
		if r < 32 || r == 127 {
			return fmt.Errorf("backup ID cannot contain control characters")
		}
	}

	// Reasonable length limit
	if len(id) > 255 {
		return fmt.Errorf("backup ID too long (max 255 characters)")
	}

	return nil
}

// ValidateDatastore validates datastore name
func ValidateDatastore(name string) error {
	if name == "" {
		return fmt.Errorf("datastore name cannot be empty")
	}

	// Must be alphanumeric with hyphens and underscores
	matched, err := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	if err != nil {
		return fmt.Errorf("regex error: %w", err)
	}

	if !matched {
		return fmt.Errorf("datastore name must be alphanumeric with hyphens/underscores only")
	}

	if len(name) > 64 {
		return fmt.Errorf("datastore name too long (max 64 characters)")
	}

	return nil
}

// ValidateAuthID validates PBS authentication ID format
func ValidateAuthID(authID string) error {
	if authID == "" {
		return fmt.Errorf("auth ID cannot be empty")
	}

	// Format: user@realm!token-name
	parts := strings.Split(authID, "!")
	if len(parts) != 2 {
		return fmt.Errorf("auth ID must be in format: user@realm!token-name")
	}

	userRealm := parts[0]
	tokenName := parts[1]

	if !strings.Contains(userRealm, "@") {
		return fmt.Errorf("auth ID must contain user@realm")
	}

	if tokenName == "" {
		return fmt.Errorf("token name cannot be empty")
	}

	return nil
}

// SecureCompare performs constant-time string comparison
// Use this for comparing secrets to prevent timing attacks
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ValidateFingerprint validates SSL certificate fingerprint format
func ValidateFingerprint(fp string) error {
	if fp == "" {
		// Empty is OK (means don't verify)
		return nil
	}

	// SHA256 fingerprint format: AA:BB:CC:DD:...
	parts := strings.Split(fp, ":")
	if len(parts) != 32 {
		return fmt.Errorf("fingerprint must have 32 hex pairs separated by colons")
	}

	for _, part := range parts {
		if len(part) != 2 {
			return fmt.Errorf("each fingerprint part must be 2 hex digits")
		}

		matched, err := regexp.MatchString(`^[0-9a-fA-F]{2}$`, part)
		if err != nil {
			return fmt.Errorf("regex error: %w", err)
		}

		if !matched {
			return fmt.Errorf("fingerprint contains invalid hex: %s", part)
		}
	}

	return nil
}

// ValidatePath checks if a path is safe (no traversal)
func ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// No path traversal
	if strings.Contains(path, "..") {
		return fmt.Errorf("path cannot contain '..'")
	}

	// No null bytes
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("path cannot contain null bytes")
	}

	return nil
}
