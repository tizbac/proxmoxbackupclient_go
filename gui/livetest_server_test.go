package main

import (
	"os"
	"strings"
	"testing"
)

// The test PBS the live tests back up to and restore from, from the
// environment, so no address or credential is kept in the source:
//
//	PBS_LIVE_URL          https://pbs.example.com:8007
//	PBS_LIVE_AUTHID       the API token, user@pbs!token
//	PBS_LIVE_SECRET       the token's secret
//	PBS_LIVE_FINGERPRINT  the server certificate's SHA-256 fingerprint (AA:BB:...)
//	PBS_LIVE_DATASTORE    a datastore the tests may fill and prune
//
// Each live test still has its own variable that turns it on (PBS_LIVE_TEST=1,
// PBS_E2E=1 ...), and calls liveServer first, so it skips without these.
var (
	e2eBaseURL = os.Getenv("PBS_LIVE_URL")
	e2eAuthID  = os.Getenv("PBS_LIVE_AUTHID")
	e2eSecret  = os.Getenv("PBS_LIVE_SECRET")
	e2eFP      = os.Getenv("PBS_LIVE_FINGERPRINT")
	e2eStore   = os.Getenv("PBS_LIVE_DATASTORE")
)

// liveServer skips t unless every PBS_LIVE_ variable above is set.
func liveServer(t testing.TB) {
	t.Helper()
	var missing []string
	for _, v := range []struct{ name, value string }{
		{"PBS_LIVE_URL", e2eBaseURL},
		{"PBS_LIVE_AUTHID", e2eAuthID},
		{"PBS_LIVE_SECRET", e2eSecret},
		{"PBS_LIVE_FINGERPRINT", e2eFP},
		{"PBS_LIVE_DATASTORE", e2eStore},
	} {
		if v.value == "" {
			missing = append(missing, v.name)
		}
	}
	if len(missing) > 0 {
		t.Skipf("this live test needs a test PBS: set %s", strings.Join(missing, ", "))
	}
}
