package pbscommon

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// prefetchDir writes n small files to a new folder and returns it with its
// listing, as WriteDir reads it.
func prefetchDir(t *testing.T, n int) (string, []os.DirEntry) {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < n; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%02d", i)), []byte(fmt.Sprintf("file %d\n", i)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir, files
}

// readAhead counts the files of set whose read ahead has ended.
func readAhead(set *prefetchSet) int {
	n := 0
	for _, pe := range set.entries {
		select {
		case <-pe.done:
			n++
		default:
		}
	}
	return n
}

// waitReadAhead waits until want files are read ahead, then checks that no
// more are.
func waitReadAhead(t *testing.T, set *prefetchSet, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for readAhead(set) < want && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // time for a read too many
	if got := readAhead(set); got != want {
		t.Fatalf("%d files read ahead, want %d", got, want)
	}
}

// Read-ahead holds at most PrefetchWorkers files: the next is read only once
// the walk has archived one, and once the walk has ended nothing more is read.
func TestPrefetchWindow(t *testing.T) {
	dir, files := prefetchDir(t, 10)
	a := &PXARArchive{PrefetchWorkers: 3}
	set := a.startPrefetch(dir, files)
	if set == nil {
		t.Fatal("no read-ahead")
	}
	waitReadAhead(t, set, 3)

	first := filepath.Join(dir, "f00")
	if pe := set.entry(first); pe == nil || string(pe.data) != "file 0\n" {
		t.Fatalf("f00 read ahead as %v", pe)
	}
	set.release(first)
	if set.entry(first).data != nil {
		t.Error("f00's bytes still held after it was archived")
	}
	waitReadAhead(t, set, 4)
	if pe := set.entry(filepath.Join(dir, "f03")); pe == nil || string(pe.data) != "file 3\n" {
		t.Errorf("the 4th file read ahead is not f03")
	}

	set.stop()
	set.release(filepath.Join(dir, "f01"))
	time.Sleep(100 * time.Millisecond)
	for name, pe := range set.entries {
		select {
		case <-pe.done:
			if pe.data != nil && pe.err == nil && filepath.Base(name) > "f04" {
				t.Errorf("%s read after the walk ended", filepath.Base(name))
			}
		default:
		}
	}
}

// The archive is the same with read-ahead as without, also when it holds
// fewer files than the folder has.
func TestPrefetchSameArchive(t *testing.T) {
	dir, _ := prefetchDir(t, 25)
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "inner"), []byte("inner\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := func(workers int) []byte {
		var out bytes.Buffer
		a := &PXARArchive{ArchiveName: "backup.pxar.didx", PrefetchWorkers: workers}
		a.WriteCB = func(b []byte) error { out.Write(b); return nil }
		a.Create()
		done := make(chan error, 1)
		go func() { _, err := a.WriteDir(dir, "", true); done <- err }()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("read-ahead %d: %v", workers, err)
			}
		case <-time.After(20 * time.Second):
			t.Fatalf("read-ahead %d: the walk is stuck", workers)
		}
		return out.Bytes()
	}
	plain := archive(0)
	for _, w := range []int{1, 2, 8, 32} {
		if !bytes.Equal(archive(w), plain) {
			t.Errorf("read-ahead %d gives another archive", w)
		}
	}
}
