package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The same folder backed up without and then with -read-ahead, to one group:
// the archive must come out the same, so the second run sends only the chunk
// with the root's backup time in it (and the catalog's) and reuses the rest.
// The first run sends every chunk through the upload pool. Needs a PBS:
// PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT and
// PBS_LIVE_DATASTORE.
func TestReadAheadSameArchiveLive(t *testing.T) {
	url := os.Getenv("PBS_LIVE_URL")
	if url == "" {
		t.Skip("set PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT, PBS_LIVE_DATASTORE")
	}
	dir := t.TempDir()
	write := func(name string, size int) {
		data := make([]byte, size)
		if _, err := rand.Read(data); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 300; i++ {
		write(fmt.Sprintf("small%03d.bin", i), 100*1000)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join("sub", "big1.bin"), 5*1000*1000)
	write(filepath.Join("sub", "big2.bin"), 5*1000*1000)

	run := func(extra ...string) (newChunks, reused int) {
		t.Helper()
		args := []string{
			"-baseurl", url,
			"-certfingerprint", os.Getenv("PBS_LIVE_FINGERPRINT"),
			"-authid", os.Getenv("PBS_LIVE_AUTHID"),
			"-secret", os.Getenv("PBS_LIVE_SECRET"),
			"-datastore", os.Getenv("PBS_LIVE_DATASTORE"),
			"-backupdir", dir,
			"-backup-id", "dir-readahead-test",
		}
		if runtime.GOOS != "windows" {
			args = append(args, "-novss")
		}
		cmd := exec.Command(os.Args[0], append(args, extra...)...)
		cmd.Env = append(os.Environ(), "DIRECTORYBACKUP_RUN_MAIN=1")
		out, err := cmd.CombinedOutput()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			t.Skipf("another backup job holds the process lock on this machine\n%s", out)
		}
		if err != nil {
			t.Fatalf("%v: exit %v\n%s", extra, err, out)
		}
		text := string(out)
		return strings.Count(text, "New chunk["), strings.Count(text, "Reuse chunk[")
	}

	n1, r1 := run()
	n2, r2 := run("-read-ahead", "8")
	t.Logf("without read-ahead: %d new, %d reused; with: %d new, %d reused", n1, r1, n2, r2)
	if n1 < 8 {
		t.Errorf("the first run sent %d new chunks, want the whole folder's (8 or more)", n1)
	}
	if n2 > 3 || r2 < n1-3 {
		t.Errorf("with read-ahead %d chunks were new and %d reused: the archive is not the same", n2, r2)
	}
}
