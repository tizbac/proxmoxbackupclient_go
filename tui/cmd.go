// Package tui is the text-mode window of Salvanza: the desktop app's Backup
// Sets, running backups, history, Message Log and servers in a terminal, over
// the local service API (gui/api). It runs over SSH and in any console.
package tui

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"
)

// Run is `salvanza-cli tui`. It returns the process exit code.
func Run(args []string, stdout, stderr io.Writer, version string) int {
	fs := flag.NewFlagSet("salvanza-cli tui", flag.ContinueOnError)
	fs.SetOutput(stderr)
	url := fs.String("url", "http://127.0.0.1:18765", "address of the Salvanza service API")
	tokenFile := fs.String("token-file", "", "the service's api-token file (default: found in the service's settings folder)")
	demo := fs.Bool("demo", false, "show invented data instead of connecting to the service")
	check := fs.Bool("check", false, "connect, print what the service reports and exit (no window), for scripts")
	restoreCheck := fs.Bool("restore-check", false, "with -check: list the server's backups, then restore a few small files of the newest readable snapshot into a temporary folder and delete it")
	write := fs.Bool("write", false, "with -check: also create, change and delete a temporary Backup Set, to prove the service takes what the editor sends")
	fs.Usage = func() {
		sayln(stderr, `Usage: salvanza-cli tui [-url <address>] [-token-file <file>] [-demo] [-check]

A text-mode window for the Backup Sets, running backups, history, Message Log
and servers of this computer's Salvanza service. Run it as administrator
(Windows) or root (Linux): the service's token is readable only to them. The
token can also be given in the environment variable SALVANZA_API_TOKEN.

Keys: Tab or 1-5 switch screen, r run now, p pause or resume, s stop,
e enable or disable, g refresh, ? help, q quit.`)
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	var be Backend
	if *demo {
		be = newDemoBackend()
	} else {
		path := *tokenFile
		if path == "" {
			path = DefaultTokenPath()
		}
		be = newServiceBackend(*url, path)
	}
	if *check {
		code := Check(be, stdout, stderr)
		if code == 0 && *write {
			code = CheckWrite(be, stdout, stderr)
		}
		if code == 0 && *restoreCheck {
			code = CheckRestore(be, stdout, stderr)
		}
		return code
	}
	// The shared PBS client prints its connection chatter to standard output,
	// which would scribble over the screen: keep it out of the window.
	if f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0); err == nil {
		os.Stdout, os.Stderr = f, f
		log.SetOutput(io.Discard)
	}
	if err := NewApp(be, version).Run(); err != nil {
		sayln(stderr, "salvanza-cli tui:", err)
		return 1
	}
	return 0
}

// Check reads everything the window shows and prints a summary; it returns 0
// when the service answered, 1 when it did not.
func Check(be Backend, stdout, stderr io.Writer) int {
	st, err := be.Status()
	if err != nil {
		sayln(stderr, "service: "+describeErr(err))
		return 1
	}
	say(stdout, "service: running, version %s, %d backup(s) active\n", st.Version, st.ActiveJobs)
	sets, err := be.Sets()
	if err != nil {
		sayln(stderr, "sets: "+describeErr(err))
		return 1
	}
	say(stdout, "backup sets: %d\n", len(sets))
	for _, s := range sets {
		state := "on"
		if !s.Enabled {
			state = "off"
		}
		say(stdout, "  %-48s %-8s %-3s %-28s next %s\n", s.Name, s.Type, state, s.Schedule, when(s.NextRun))
	}
	runs, err := be.Runs()
	if err != nil {
		sayln(stderr, "runs: "+describeErr(err))
		return 1
	}
	say(stdout, "runs known: %d\n", len(runs))
	hist, _ := be.History()
	msgs, _ := be.Messages()
	srv, _ := be.Servers()
	say(stdout, "history: %d, messages: %d, servers: %d\n", len(hist), len(msgs), len(srv))
	for _, s := range srv {
		def := ""
		if s.Default {
			def = " (default)"
		}
		say(stdout, "  server %s%s: %s datastore %s\n", s.Name, def, s.URL, s.Datastore)
	}
	return 0
}

// CheckRestore follows the restore wizard's path against the default server:
// groups, the newest snapshot that can be read, its file list, and a restore of
// a few small files into a temporary folder, which is removed again.
func CheckRestore(be Backend, stdout, stderr io.Writer) int {
	groups, err := be.RestoreGroups("")
	if err != nil {
		sayln(stderr, "groups: "+describeErr(err))
		return 1
	}
	say(stdout, "backup groups on the default server: %d\n", len(groups))
	for _, g := range groups {
		snaps, err := be.RestoreSnapshots("", g)
		if err != nil || len(snaps) == 0 {
			continue
		}
		for _, s := range snaps {
			if s.Encrypted || len(s.Archives) == 0 || !s.HasCatalog {
				continue
			}
			items, err := be.RestoreCatalog("", s)
			if err != nil {
				continue
			}
			// the smallest few files
			var files []CatalogItem
			for _, it := range items {
				if !it.Dir && it.Size > 0 && it.Size < 1<<20 {
					files = append(files, it)
				}
			}
			if len(files) == 0 {
				continue
			}
			sort.Slice(files, func(i, j int) bool { return files[i].Size < files[j].Size })
			if len(files) > 3 {
				files = files[:3]
			}
			var inc []string
			for _, f := range files {
				inc = append(inc, f.Path)
			}
			dest, err := os.MkdirTemp("", "tui-restore-check-")
			if err != nil {
				sayln(stderr, err)
				return 1
			}
			defer func() { _ = os.RemoveAll(dest) }()
			say(stdout, "snapshot %s/%s %s: %d catalog entries, restoring %d files\n", g.Type, g.ID, s.Time.Format(time.RFC3339), len(items), len(inc))
			n, err := be.RestoreFiles("", s, inc, dest, false, nil)
			if err != nil {
				sayln(stderr, "restore: "+describeErr(err))
				return 1
			}
			var got int
			_ = filepath.Walk(dest, func(p string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() {
					got++
				}
				return nil
			})
			if got < len(inc) {
				say(stderr, "restore: wrote %d files, expected %d (reported %d)\n", got, len(inc), n)
				return 1
			}
			say(stdout, "restored %d files into a temporary folder, removed again: the restore path works\n", got)
			return 0
		}
	}
	sayln(stdout, "no unencrypted snapshot with a file catalog was found to try a restore on")
	return 0
}

// CheckWrite makes a temporary manual Backup Set the way the editor does,
// changes it, and deletes it again (always, also after a failure).
func CheckWrite(be Backend, stdout, stderr io.Writer) int {
	const name = "__tui-selftest__"
	id := fmt.Sprintf("%d", time.Now().UnixMilli())
	doc := map[string]interface{}{
		"id": id, "name": name, "backupType": "directory", "enabled": true, "backupId": "tui-selftest",
		"backupDirs": []interface{}{os.TempDir()}, "scheduleMode": "manual", "useVSS": false,
		"changeDetectionMode": "legacy", "excludeList": []interface{}{},
		"schedules": []interface{}{map[string]interface{}{"id": "s1", "enabled": true, "unit": "days", "every": 1.0, "at": "03:30"}},
	}
	cleanup := func() {
		if sets, err := be.Sets(); err == nil {
			for _, s := range sets {
				if s.Name == name || s.Name == name+" changed" {
					_ = be.DeleteSet(s.ID)
				}
			}
		}
	}
	defer cleanup()
	find := func(n string) (Set, bool) {
		sets, err := be.Sets()
		if err != nil {
			return Set{}, false
		}
		for _, s := range sets {
			if s.Name == n {
				return s, true
			}
		}
		return Set{}, false
	}
	if err := be.SaveSet(doc, true); err != nil {
		sayln(stderr, "create: "+describeErr(err))
		return 1
	}
	s, ok := find(name)
	if !ok {
		sayln(stderr, "create: the new set is not in the list")
		return 1
	}
	say(stdout, "created %q, schedule %q\n", s.Name, s.Schedule)
	// change it as the editor does: the service's own document plus the changes
	upd := copyMap(s.Raw)
	upd["name"] = name + " changed"
	upd["scheduleMode"] = "scheduled"
	upd["schedules"] = []interface{}{map[string]interface{}{"id": "s1", "enabled": true, "unit": "days", "every": 1.0, "at": "04:45", "weekdays": []interface{}{"Mon", "Fri"}}}
	if err := be.SaveSet(upd, false); err != nil {
		sayln(stderr, "change: "+describeErr(err))
		return 1
	}
	s2, ok := find(name + " changed")
	if !ok || s2.ID != id {
		sayln(stderr, "change: the changed set is not in the list")
		return 1
	}
	say(stdout, "changed to %q, schedule %q, next run %s\n", s2.Name, s2.Schedule, when(s2.NextRun))
	if err := be.DeleteSet(id); err != nil {
		sayln(stderr, "delete: "+describeErr(err))
		return 1
	}
	if _, ok := find(name + " changed"); ok {
		sayln(stderr, "delete: the set is still listed")
		return 1
	}
	sayln(stdout, "deleted again: the editor's documents are accepted by this service")
	if code := checkWriteServer(be, stdout, stderr); code != 0 {
		return code
	}
	if code := checkWritePrefs(be, stdout, stderr); code != 0 {
		return code
	}
	return checkLogs(be, stdout, stderr)
}

// checkWriteServer adds, changes and deletes a temporary server (never the
// default, and nothing connects to it).
func checkWriteServer(be Backend, stdout, stderr io.Writer) int {
	const name = "__tui-selftest-server__"
	id := fmt.Sprintf("tuiselftest%d", time.Now().Unix())
	doc := map[string]interface{}{
		"id": id, "name": name, "baseurl": "https://tui-selftest.invalid:8007", "datastore": "selftest",
		"authid": "selftest@pbs!tui", "secret": "not-a-real-secret",
	}
	defer func() { _ = be.DeleteServer(id) }()
	if err := be.SaveServer(doc, true); err != nil {
		sayln(stderr, "server create: "+describeErr(err))
		return 1
	}
	list, err := be.Servers()
	if err != nil {
		sayln(stderr, "servers: "+describeErr(err))
		return 1
	}
	var found *Server
	for i := range list {
		if list[i].ID == id {
			found = &list[i]
		}
	}
	if found == nil || found.Default {
		sayln(stderr, "server create: the new server is missing or became the default")
		return 1
	}
	// change it the way the editor does: the service's own document, the secret left empty
	upd := copyMap(found.Raw)
	upd["description"] = "temporary, made by the text window's check"
	upd["secret"] = ""
	delete(upd, "secret_set")
	if err := be.SaveServer(upd, false); err != nil {
		sayln(stderr, "server change: "+describeErr(err))
		return 1
	}
	if err := be.DeleteServer(id); err != nil {
		sayln(stderr, "server delete: "+describeErr(err))
		return 1
	}
	say(stdout, "server: created, changed (secret kept) and deleted again\n")
	return 0
}

// checkWritePrefs sets one preference and puts it back.
func checkWritePrefs(be Backend, stdout, stderr io.Writer) int {
	old, err := be.Prefs()
	if err != nil {
		sayln(stderr, "preferences: "+describeErr(err))
		return 1
	}
	now := old
	now.ChunkTimeout = 120
	if old.ChunkTimeout == 120 {
		now.ChunkTimeout = 150
	}
	if err := be.SavePrefs(old, now); err != nil {
		sayln(stderr, "preferences save: "+describeErr(err))
		return 1
	}
	got, err := be.Prefs()
	restoreErr := be.SavePrefs(got, old)
	if err != nil || got.ChunkTimeout != now.ChunkTimeout {
		say(stderr, "preferences: the chunk timeout reads back as %d, wanted %d (%v)\n", got.ChunkTimeout, now.ChunkTimeout, err)
		return 1
	}
	if restoreErr != nil {
		sayln(stderr, "preferences: could not put the old value back: "+describeErr(restoreErr))
		return 1
	}
	say(stdout, "preferences: chunk timeout changed to %d and put back to %d\n", now.ChunkTimeout, old.ChunkTimeout)
	return 0
}

// checkLogs lists the log files and reads the end of the newest.
func checkLogs(be Backend, stdout, stderr io.Writer) int {
	files, err := be.LogFiles()
	if err != nil {
		sayln(stderr, "log files: "+describeErr(err))
		return 1
	}
	say(stdout, "log files: %d\n", len(files))
	if len(files) == 0 {
		return 0
	}
	lines, err := be.ReadLogFile(files[0], "", true)
	if err != nil {
		sayln(stderr, "log read: "+describeErr(err))
		return 1
	}
	say(stdout, "read %d lines of %s\n", len(lines), files[0].Name)
	return 0
}

// DefaultTokenPath finds the service's api-token file: the settings folder of
// the installed brand (Salvanza, or another brand's) under ProgramData on
// Windows or /var/lib on Linux.
func DefaultTokenPath() string {
	var roots []string
	if runtime.GOOS == "windows" {
		pd := os.Getenv("ProgramData")
		if pd == "" {
			pd = `C:\ProgramData`
		}
		roots = []string{pd}
	} else {
		roots = []string{"/var/lib"}
	}
	for _, root := range roots {
		if p := filepath.Join(root, "Salvanza", "api-token"); exists(p) {
			return p
		}
		if p := filepath.Join(root, "salvanza", "api-token"); exists(p) {
			return p
		}
		entries, _ := os.ReadDir(root)
		for _, e := range entries {
			if e.IsDir() {
				if p := filepath.Join(root, e.Name(), "api-token"); exists(p) {
					return p
				}
			}
		}
	}
	return filepath.Join(roots[0], "Salvanza", "api-token")
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// say and sayln write to an output stream; a failed write to a closed pipe is not worth stopping for.
func say(w io.Writer, format string, args ...interface{}) { _, _ = fmt.Fprintf(w, format, args...) }

func sayln(w io.Writer, args ...interface{}) { _, _ = fmt.Fprintln(w, args...) }
