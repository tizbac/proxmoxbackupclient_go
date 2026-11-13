package security

import (
	"strings"
	"testing"
)

// TestValidateDatastore: every name Proxmox Backup Server accepts for a
// datastore is accepted here, dots included (issue #147), and nothing that
// could change the request path gets through.
func TestValidateDatastore(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"backup", false},
		{"store_1", false},
		{"my-store", false},
		{"mn5-pbs.mnt", false}, // the name from issue #147
		{"a.b.c", false},
		{"_hidden", false},
		{"9lives", false},
		{"", true},
		{".hidden", true}, // the server refuses a leading dot
		{"-dash", true},   // and a leading hyphen
		{"..", true},      // never a path step
		{"a/b", true},     // no path separators
		{"a\\b", true},    // of either kind
		{"a b", true},     // no spaces
		{"a?b=c", true},   // nothing that changes the request
		{"store\n", true}, // no control characters
		{"stör", true},    // ASCII only, as on the server
		{strings.Repeat("a", 64), false},
		{strings.Repeat("a", 65), true},
	}
	for _, c := range cases {
		err := ValidateDatastore(c.name)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidateDatastore(%q) error = %v, wantErr = %v", c.name, err, c.wantErr)
		}
	}
}

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
