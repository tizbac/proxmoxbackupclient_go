package pbscommon

import "testing"

// The upgrade request's Host is the BaseURL host as written: a default port
// must not appear when the URL has none, or a reverse proxy routing on the
// exact Host misses it (upstream #89; test from upstream #116).
func TestUpgradeHostHeader(t *testing.T) {
	cases := []struct{ baseURL, addr, want string }{
		{"https://pbs.example.com", "pbs.example.com:443", "pbs.example.com"},
		{"https://pbs.example.com:8007", "pbs.example.com:8007", "pbs.example.com:8007"},
		{"https://[2001:db8::1]:8007", "[2001:db8::1]:8007", "[2001:db8::1]:8007"},
		{"", "10.0.0.1:8007", "10.0.0.1:8007"},
	}
	for _, c := range cases {
		if got := upgradeHostHeader(c.baseURL, c.addr); got != c.want {
			t.Errorf("upgradeHostHeader(%q, %q) = %q, want %q", c.baseURL, c.addr, got, c.want)
		}
	}
}
