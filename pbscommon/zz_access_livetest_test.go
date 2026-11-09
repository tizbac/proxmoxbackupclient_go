package pbscommon

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Live check of the connection test's access outcomes against a real PBS.
// Opt-in: PBS_ACCESS_CASES names a JSON file with
//
//	{"url": "https://pbs:8007", "fingerprint": "...", "datastore": "store", "namespace": "",
//	 "cases": [{"name": "...", "authid": "user@pbs!tok", "secret": "...",
//	            "want": "full" | "restore-only" | "denied" | "no-permission" | "bad-login"}]}
//
// full: no error and no note. restore-only: no error, the restore-only note.
// denied: the account can look at the datastore but may neither back up nor
// restore. no-permission: it cannot even read the status. bad-login: refused as
// not authenticated.
func TestLiveAccessOutcomes(t *testing.T) {
	path := os.Getenv("PBS_ACCESS_CASES")
	if path == "" {
		t.Skip("PBS_ACCESS_CASES not set")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		URL, Fingerprint, Datastore, Namespace string
		Cases                                  []struct{ Name, AuthID, Secret, Want, Namespace string }
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, c := range cfg.Cases {
		t.Run(c.Name, func(t *testing.T) {
			ns := cfg.Namespace
			if c.Namespace != "" {
				ns = c.Namespace
			}
			pbs := &PBSClient{BaseURL: cfg.URL, CertFingerPrint: cfg.Fingerprint, Insecure: true, Datastore: cfg.Datastore,
				Namespace: ns, AuthID: c.AuthID, Secret: c.Secret}
			acc, err := pbs.TestConnectionAccess()
			note := acc.Note()
			t.Logf("level %v, note %q, err %v", func() interface{} {
				if acc == nil {
					return "unknown"
				}
				return acc.Level
			}(), note, err)
			fail := func(msg string) { t.Errorf("%s: level/err/note = %v / %v / %q", msg, acc, err, note) }
			switch c.Want {
			case "full":
				if err != nil || note != "" || acc == nil || acc.Level != AccessFull {
					fail("want full access")
				}
			case "restore-only":
				if err != nil || !strings.Contains(note, "restore only") || acc == nil || acc.Level != AccessRestoreOnly {
					fail("want restore only")
				}
			case "denied":
				if err == nil || !strings.Contains(err.Error(), "may not back up or restore") {
					fail("want denied")
				}
			case "no-permission":
				if err == nil || !strings.Contains(err.Error(), "has no permission on") {
					fail("want no permission")
				}
			case "bad-login":
				if err == nil || !strings.Contains(err.Error(), "authentication failed") {
					fail("want a bad login")
				}
			default:
				t.Fatalf("unknown want %q", c.Want)
			}
		})
	}
}
