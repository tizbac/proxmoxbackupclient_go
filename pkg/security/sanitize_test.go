package security

import "testing"

// TestValidateURLLoopback covers v2-H-07: plain HTTP is allowed only for a true
// loopback host, never for "localhost.<something>" or "127.0.0.1.<something>",
// which previously slipped through a substring check and leaked the PBS token.
func TestValidateURLLoopback(t *testing.T) {
	cases := []struct {
		url     string
		wantErr bool
	}{
		{"https://pbs.example.com:8007", false}, // HTTPS to any host is fine
		{"http://localhost:8007", false},        // true loopback
		{"http://127.0.0.1:8007", false},        // true loopback
		{"http://[::1]:8007", false},            // IPv6 loopback
		{"http://127.0.0.5:8007", false},        // 127.0.0.0/8 loopback
		{"http://localhost.attacker.tld", true}, // the bypass — must be refused
		{"http://127.0.0.1.evil.tld", true},     // the bypass — must be refused
		{"http://pbs.example.com:8007", true},   // plain HTTP to a non-loopback host
		{"ftp://localhost", true},               // loopback exemption is HTTP-only
		{"", true},                              // empty
	}
	for _, c := range cases {
		err := ValidateURL(c.url)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidateURL(%q) error = %v, wantErr = %v", c.url, err, c.wantErr)
		}
	}
}
