// Package tui is the text-mode window of Salvanza: the desktop app's Backup
// Sets, running backups, history, Message Log and servers in a terminal, over
// the local service API (gui/api). It runs over SSH and in any console.
package tui

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
	write := fs.Bool("write", false, "with -check: also create, change and delete a temporary Backup Set, to prove the service takes what the editor sends")
	fs.Usage = func() {
		fmt.Fprintln(stderr, `Usage: salvanza-cli tui [-url <address>] [-token-file <file>] [-demo] [-check]

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
		return code
	}
	if err := NewApp(be, version).Run(); err != nil {
		fmt.Fprintln(stderr, "salvanza-cli tui:", err)
		return 1
	}
	return 0
}

// Check reads everything the window shows and prints a summary; it returns 0
// when the service answered, 1 when it did not.
func Check(be Backend, stdout, stderr io.Writer) int {
	st, err := be.Status()
	if err != nil {
		fmt.Fprintln(stderr, "service: "+describeErr(err))
		return 1
	}
	fmt.Fprintf(stdout, "service: running, version %s, %d backup(s) active\n", st.Version, st.ActiveJobs)
	sets, err := be.Sets()
	if err != nil {
		fmt.Fprintln(stderr, "sets: "+describeErr(err))
		return 1
	}
	fmt.Fprintf(stdout, "backup sets: %d\n", len(sets))
	for _, s := range sets {
		state := "on"
		if !s.Enabled {
			state = "off"
		}
		fmt.Fprintf(stdout, "  %-48s %-8s %-3s %-28s next %s\n", s.Name, s.Type, state, s.Schedule, when(s.NextRun))
	}
	runs, err := be.Runs()
	if err != nil {
		fmt.Fprintln(stderr, "runs: "+describeErr(err))
		return 1
	}
	fmt.Fprintf(stdout, "runs known: %d\n", len(runs))
	hist, _ := be.History()
	msgs, _ := be.Messages()
	srv, _ := be.Servers()
	fmt.Fprintf(stdout, "history: %d, messages: %d, servers: %d\n", len(hist), len(msgs), len(srv))
	for _, s := range srv {
		def := ""
		if s.Default {
			def = " (default)"
		}
		fmt.Fprintf(stdout, "  server %s%s: %s datastore %s\n", s.Name, def, s.URL, s.Datastore)
	}
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
		fmt.Fprintln(stderr, "create: "+describeErr(err))
		return 1
	}
	s, ok := find(name)
	if !ok {
		fmt.Fprintln(stderr, "create: the new set is not in the list")
		return 1
	}
	fmt.Fprintf(stdout, "created %q, schedule %q\n", s.Name, s.Schedule)
	// change it as the editor does: the service's own document plus the changes
	upd := copyMap(s.Raw)
	upd["name"] = name + " changed"
	upd["scheduleMode"] = "scheduled"
	upd["schedules"] = []interface{}{map[string]interface{}{"id": "s1", "enabled": true, "unit": "days", "every": 1.0, "at": "04:45", "weekdays": []interface{}{"Mon", "Fri"}}}
	if err := be.SaveSet(upd, false); err != nil {
		fmt.Fprintln(stderr, "change: "+describeErr(err))
		return 1
	}
	s2, ok := find(name + " changed")
	if !ok || s2.ID != id {
		fmt.Fprintln(stderr, "change: the changed set is not in the list")
		return 1
	}
	fmt.Fprintf(stdout, "changed to %q, schedule %q, next run %s\n", s2.Name, s2.Schedule, when(s2.NextRun))
	if err := be.DeleteSet(id); err != nil {
		fmt.Fprintln(stderr, "delete: "+describeErr(err))
		return 1
	}
	if _, ok := find(name + " changed"); ok {
		fmt.Fprintln(stderr, "delete: the set is still listed")
		return 1
	}
	fmt.Fprintln(stdout, "deleted again: the editor's documents are accepted by this service")
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
