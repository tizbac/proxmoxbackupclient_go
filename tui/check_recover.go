package tui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// CheckRecover proves Undelete and Roll back through the real service program:
// a folder is backed up to a real Proxmox Backup Server by a temporary Backup
// Set, a file is then deleted, one changed and one added, Undelete brings the
// deleted file back, and Roll back (exact) restores the changed file and
// removes the added one. Everything it made is removed again.
//
// The server comes from TUI_CHECK_PBS_* as for CheckEncryption. The service
// program is found as the window finds it (SALVANZA_SVC names it).
func CheckRecover(be Backend, stdout, stderr io.Writer) int {
	env := func(k string) string { return strings.TrimSpace(os.Getenv("TUI_CHECK_PBS_" + k)) }
	base, authID, secret, store, fp := env("URL"), env("AUTHID"), env("SECRET"), env("DATASTORE"), env("FINGERPRINT")
	if base == "" || authID == "" || secret == "" || store == "" {
		sayln(stderr, "recover check: set TUI_CHECK_PBS_URL, _AUTHID, _SECRET, _DATASTORE (and _FINGERPRINT)")
		return 2
	}
	stamp := time.Now().Unix()
	srvID := fmt.Sprintf("tuiselftestrec%d", stamp)
	setID := fmt.Sprintf("tuiselftestrecset%d", stamp)
	group := fmt.Sprintf("tui-rec-%d", stamp)

	src, err := os.MkdirTemp("", "tui-rec-src-")
	if err != nil {
		sayln(stderr, err)
		return 1
	}
	defer func() { _ = os.RemoveAll(src) }()
	orig := map[string][]byte{
		"keep.txt":   []byte("never touched"),
		"gone.txt":   []byte("deleted after the backup, Undelete brings it back"),
		"edited.txt": []byte("the text in the backup"),
	}
	for name, data := range orig {
		if err := os.WriteFile(filepath.Join(src, name), data, 0o644); err != nil {
			sayln(stderr, err)
			return 1
		}
	}

	defer func() { _ = be.DeleteSet(setID) }()
	defer func() { _ = be.DeleteServer(srvID) }()
	defer deleteGroupHTTP(base, store, authID, secret, group)
	if err := be.SaveServer(map[string]interface{}{
		"id": srvID, "name": "__tui-selftest-recover__", "baseurl": base, "datastore": store, "certfingerprint": fp,
		"authid": authID, "secret": secret,
	}, true); err != nil {
		sayln(stderr, "save server: "+describeErr(err))
		return 1
	}
	if err := be.SaveSet(map[string]interface{}{
		"id": setID, "name": "__tui-selftest-recover__", "backupType": "directory", "enabled": true, "backupId": group,
		"backupDirs": []interface{}{src}, "scheduleMode": "manual", "useVSS": false, "pbsServerId": srvID,
		"changeDetectionMode": "legacy", "excludeList": []interface{}{},
		"schedules": []interface{}{map[string]interface{}{"id": "s1", "enabled": true, "unit": "days", "every": 1.0, "at": "03:30"}},
	}, true); err != nil {
		sayln(stderr, "save set: "+describeErr(err))
		return 1
	}
	if err := be.RunSet(setID); err != nil {
		sayln(stderr, "run set: "+describeErr(err))
		return 1
	}
	say(stdout, "backup of %s started\n", group)
	var found bool
	for deadline := time.Now().Add(3 * time.Minute); time.Now().Before(deadline); {
		time.Sleep(2 * time.Second)
		if snaps, err := be.RestoreSnapshots(srvID, Group{Type: "host", ID: group}); err == nil && len(snaps) > 0 {
			found = true
			break
		}
	}
	if !found {
		sayln(stderr, "the backup did not finish in 3 minutes")
		return 1
	}
	say(stdout, "backup finished\n")

	// after the backup: one file gone, one changed, one added
	_ = os.Remove(filepath.Join(src, "gone.txt"))
	if err := os.WriteFile(filepath.Join(src, "edited.txt"), []byte("changed after the backup"), 0o644); err != nil {
		sayln(stderr, err)
		return 1
	}
	if err := os.WriteFile(filepath.Join(src, "added.txt"), []byte("added after the backup"), 0o644); err != nil {
		sayln(stderr, err)
		return 1
	}

	var safety []string
	keptIn := regexp.MustCompile(`kept in (.+?) for Undo`)
	run := func(args ...string) ([]string, error) {
		var out []string
		err := be.RunSetTool(args, func(l string) {
			out = append(out, l)
			if m := keptIn.FindStringSubmatch(l); m != nil {
				for _, d := range strings.Split(m[1], ", ") {
					safety = append(safety, strings.TrimSpace(d))
				}
			}
		})
		return out, err
	}
	defer func() {
		for _, d := range safety {
			_ = os.RemoveAll(d)
		}
	}()
	has := func(lines []string, sub string) bool { return strings.Contains(strings.Join(lines, "\n"), sub) }

	out, err := run("-undelete", setID, "-dry-run")
	if err != nil || !has(out, "gone.txt") {
		sayln(stderr, fmt.Sprintf("undelete look: %v: %v", err, out))
		return 1
	}
	if _, err := os.Stat(filepath.Join(src, "gone.txt")); err == nil {
		sayln(stderr, "the undelete look restored a file")
		return 1
	}
	say(stdout, "undelete look: finds gone.txt and restores nothing\n")
	if out, err = run("-undelete", setID); err != nil {
		sayln(stderr, fmt.Sprintf("undelete: %v: %v", err, out))
		return 1
	}
	if got, err := os.ReadFile(filepath.Join(src, "gone.txt")); err != nil || !bytes.Equal(got, orig["gone.txt"]) {
		sayln(stderr, fmt.Sprintf("undelete: gone.txt is not back as it was (%v)", err))
		return 1
	}
	say(stdout, "undelete: gone.txt is back, identical to the original\n")

	if out, err = run("-rollback", setID, "-policy", "exact", "-dry-run"); err != nil || !has(out, "changed since: 1") {
		sayln(stderr, fmt.Sprintf("rollback preview: %v: %v", err, out))
		return 1
	}
	if got, _ := os.ReadFile(filepath.Join(src, "edited.txt")); !bytes.Equal(got, []byte("changed after the backup")) {
		sayln(stderr, "the rollback preview changed a file")
		return 1
	}
	say(stdout, "rollback preview: counts shown, nothing changed\n")
	if out, err = run("-rollback", setID, "-policy", "exact", "-yes"); err != nil {
		sayln(stderr, fmt.Sprintf("rollback: %v: %v", err, out))
		return 1
	}
	if got, err := os.ReadFile(filepath.Join(src, "edited.txt")); err != nil || !bytes.Equal(got, orig["edited.txt"]) {
		sayln(stderr, fmt.Sprintf("rollback: edited.txt is not back as it was (%v)", err))
		return 1
	}
	if _, err := os.Stat(filepath.Join(src, "added.txt")); err == nil {
		sayln(stderr, "rollback (exact): the file added since is still there")
		return 1
	}
	say(stdout, "rollback (exact): edited.txt restored, added.txt removed\n")

	// Undo it: the list names the roll back, and undoing puts the folder back as it was before
	recs, err := rollbackRecords(be, setID)
	if err != nil || len(recs) != 1 {
		sayln(stderr, fmt.Sprintf("earlier roll backs: want 1, got %d (%v)", len(recs), err))
		return 1
	}
	if out, err = run("-undo-rollback", setID, "-created", recs[0].Created); err != nil {
		sayln(stderr, fmt.Sprintf("undo roll back: %v: %v", err, out))
		return 1
	}
	if got, _ := os.ReadFile(filepath.Join(src, "edited.txt")); !bytes.Equal(got, []byte("changed after the backup")) {
		sayln(stderr, "undo roll back: edited.txt is not back as it was before the roll back")
		return 1
	}
	if got, err := os.ReadFile(filepath.Join(src, "added.txt")); err != nil || !bytes.Equal(got, []byte("added after the backup")) {
		sayln(stderr, fmt.Sprintf("undo roll back: added.txt is not back (%v)", err))
		return 1
	}
	say(stdout, "undo roll back: the folder is as it was before the roll back\n")

	// Roll back again, then delete its safety copies: they are gone and the list is empty
	if out, err = run("-rollback", setID, "-policy", "exact", "-yes"); err != nil {
		sayln(stderr, fmt.Sprintf("second rollback: %v: %v", err, out))
		return 1
	}
	if recs, err = rollbackRecords(be, setID); err != nil || len(recs) == 0 {
		sayln(stderr, fmt.Sprintf("second roll back is not listed (%v)", err))
		return 1
	}
	if out, err = run("-delete-rollback-copies", setID, "-created", recs[len(recs)-1].Created); err != nil {
		sayln(stderr, fmt.Sprintf("delete safety copies: %v: %v", err, out))
		return 1
	}
	if recs, err = rollbackRecords(be, setID); err != nil || len(recs) != 0 {
		sayln(stderr, fmt.Sprintf("after deleting the copies %d roll back(s) are still listed (%v)", len(recs), err))
		return 1
	}
	say(stdout, "safety copies deleted: no roll back is listed any more; %d safety folder(s) cleaned up\n", len(safety))
	return 0
}
