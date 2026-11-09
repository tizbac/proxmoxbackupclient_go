package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// CheckAccess proves the connection test's permission outcomes through the
// real service against a real Proxmox Backup Server: for each account in the
// cases file a temporary server is made, tested the way the Servers screen
// tests it, and removed. The file is named by TUI_CHECK_ACCESS_CASES and holds
//
//	{"url": "https://pbs:8007", "fingerprint": "...", "datastore": "store", "namespace": "",
//	 "cases": [{"name": "...", "authid": "user@pbs!tok", "secret": "...", "namespace": "",
//	            "want": "full" | "restore-only" | "denied" | "no-permission" | "bad-login"}]}
//
// full: the test passes without a warning. restore-only: it passes with the
// restore-only warning. denied: it fails, the account may neither back up nor
// restore. no-permission: it fails, the account cannot read the datastore.
// bad-login: it fails as not authenticated.
func CheckAccess(be Backend, stdout, stderr io.Writer) int {
	path := strings.TrimSpace(os.Getenv("TUI_CHECK_ACCESS_CASES"))
	if path == "" {
		sayln(stderr, "access check: set TUI_CHECK_ACCESS_CASES to a cases file (see the CheckAccess comment)")
		return 2
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		sayln(stderr, err)
		return 2
	}
	var cfg struct {
		URL, Fingerprint, Datastore, Namespace string
		Cases                                  []struct{ Name, AuthID, Secret, Want, Namespace string }
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		sayln(stderr, "access check: the cases file: "+err.Error())
		return 2
	}
	stamp := time.Now().Unix()
	bad := 0
	for i, c := range cfg.Cases {
		id := fmt.Sprintf("tuiselftestacc%d%d", stamp, i)
		ns := cfg.Namespace
		if c.Namespace != "" {
			ns = c.Namespace
		}
		if err := be.SaveServer(map[string]interface{}{
			"id": id, "name": "__tui-selftest-access__", "baseurl": cfg.URL, "datastore": cfg.Datastore, "namespace": ns,
			"certfingerprint": cfg.Fingerprint, "authid": c.AuthID, "secret": c.Secret,
		}, true); err != nil {
			sayln(stderr, c.Name+": save server: "+describeErr(err))
			bad++
			continue
		}
		note, terr := be.TestServer(id)
		_ = be.DeleteServer(id)
		got := "passes"
		if note != "" {
			got += " with the warning: " + note
		}
		if terr != nil {
			got = "fails: " + describeErr(terr)
		}
		ok := false
		switch c.Want {
		case "full":
			ok = terr == nil && note == ""
		case "restore-only":
			ok = terr == nil && strings.Contains(note, "restore only")
		case "denied":
			ok = terr != nil && strings.Contains(terr.Error(), "may not back up or restore")
		case "no-permission":
			ok = terr != nil && strings.Contains(terr.Error(), "has no permission on")
		case "bad-login":
			ok = terr != nil && strings.Contains(terr.Error(), "authentication failed")
		default:
			sayln(stderr, c.Name+": unknown want "+c.Want)
			bad++
			continue
		}
		if ok {
			say(stdout, "ok   %-30s %s\n", c.Name, got)
		} else {
			sayln(stderr, fmt.Sprintf("FAIL %-30s wanted %s, the test %s", c.Name, c.Want, got))
			bad++
		}
	}
	if bad > 0 {
		return 1
	}
	say(stdout, "access: %d account(s) tested through the service, all as wanted\n", len(cfg.Cases))
	return 0
}
