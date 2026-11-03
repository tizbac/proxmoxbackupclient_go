package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"pbscommon"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// -exclude end to end: patterns from the config file and from the command
// line (which add to it) leave the right entries out of the archive, which is
// read back from -pxarout. On Windows the run uses VSS, so the patterns are
// matched while the shadow copy is walked, and the excluded paths are printed
// by their real names. Needs a PBS: PBS_LIVE_URL, PBS_LIVE_AUTHID,
// PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT and PBS_LIVE_DATASTORE (and on
// Windows, administrator rights for VSS).
func TestExcludeLive(t *testing.T) {
	url := os.Getenv("PBS_LIVE_URL")
	if url == "" {
		t.Skip("set PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT, PBS_LIVE_DATASTORE")
	}
	dir := t.TempDir()
	for _, f := range []string{
		"keep.txt", "skip.tmp", "sub/keep2.txt", "sub/UPPER.TMP",
		"node_modules/a.js", "sub/node_modules/b.js",
		"logs/a.log", "logs/keep.txt", "logs/deep/b.log",
		"Cache/c.bin", "sub/Cache/kept.bin",
	} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := filepath.Join(t.TempDir(), "cfg.json")
	if err := os.WriteFile(cfg, []byte(`{"exclude": ["*.tmp"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// The folder as an absolute path, and as a relative one run from its
	// parent: patterns with a separator missed for "./name" before the walk
	// used the cleaned absolute path. Each run has its own group, as PBS
	// refuses two snapshots of a group in the same second.
	for _, c := range []struct{ name, backupdir, cwd string }{
		{"abs", dir, ""},
		{"rel", "." + string(filepath.Separator) + filepath.Base(dir), filepath.Dir(dir)},
	} {
		t.Run(c.name, func(t *testing.T) {
			excludeLiveRun(t, url, cfg, dir, c.backupdir, c.cwd, "dir-exclude-test-"+c.name)
		})
	}
}

// excludeLiveRun backs up dir, named as backupdir from cwd, with the
// patterns of TestExcludeLive, and checks the archive and the output.
func excludeLiveRun(t *testing.T, url, cfg, dir, backupdir, cwd, backupID string) {
	pxar := filepath.Join(t.TempDir(), "out.pxar")
	args := []string{
		"-config", cfg,
		"-baseurl", url,
		"-certfingerprint", os.Getenv("PBS_LIVE_FINGERPRINT"),
		"-authid", os.Getenv("PBS_LIVE_AUTHID"),
		"-secret", os.Getenv("PBS_LIVE_SECRET"),
		"-datastore", os.Getenv("PBS_LIVE_DATASTORE"),
		"-backupdir", backupdir,
		"-backup-id", backupID,
		"-pxarout", pxar,
		"-exclude", "node_modules",
		"-exclude", "logs/*.log",
		"-exclude", filepath.Join(dir, "Cache"), // absolute: only the top one
	}
	if runtime.GOOS != "windows" {
		args = append(args, "-novss")
	}
	// The program by its absolute path: a relative os.Args[0] would be taken
	// from cwd.
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "DIRECTORYBACKUP_RUN_MAIN=1")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		t.Skipf("another backup job holds the process lock on this machine\n%s", out)
	}
	if err != nil {
		t.Fatalf("exit %v\n%s", err, out)
	}

	data, err := os.ReadFile(pxar)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := pbscommon.NewPXARReader(data).ListEntries()
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir {
			files = append(files, strings.TrimPrefix(strings.TrimPrefix(e.Path, "./"), "/"))
		}
	}
	sort.Strings(files)
	want := []string{pbscommon.BackupMetaFilename, "keep.txt", "logs/deep/b.log", "logs/keep.txt", "sub/Cache/kept.bin", "sub/keep2.txt"}
	sort.Strings(want)
	if strings.Join(files, "|") != strings.Join(want, "|") {
		t.Errorf("archive holds\n  %v\nwant\n  %v", files, want)
	}

	// Excluded entries are listed by their real paths, one line each, with the
	// count: two .tmp files, two node_modules folders, logs/a.log and Cache.
	text := string(out)
	if !strings.Contains(text, "6 files and folders excluded by -exclude") {
		t.Errorf("no count of 6 in the output\n%s", text)
	}
	for _, rel := range []string{"skip.tmp", "node_modules", "logs/a.log", "Cache"} {
		if !strings.Contains(text, "Excluded: "+filepath.Join(dir, filepath.FromSlash(rel))+"\n") {
			t.Errorf("no %q line for %s", "Excluded: ", rel)
		}
	}
	if strings.Contains(text, `Excluded: \\?\`) {
		t.Errorf("an excluded path is named by the shadow copy\n%s", text)
	}
	t.Logf("%s", text)
}
