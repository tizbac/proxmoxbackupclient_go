//go:build linux

package pbscommon

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// With read-ahead on, a FIFO in a folder must not be read ahead. The worker
// blocked opening it for good, and with one worker the file queued behind it
// never arrived, so the backup hung.
func TestSpecialFilesNotReadAhead(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "a-pipe"), 0644); err != nil {
		t.Fatal(err)
	}
	last := []byte("last\n")
	if err := os.WriteFile(filepath.Join(root, "z.txt"), last, 0644); err != nil {
		t.Fatal(err)
	}

	var p bytes.Buffer
	archive := &PXARArchive{ArchiveName: "backup.pxar.didx", PrefetchWorkers: 1}
	archive.WriteCB = func(b []byte) error { p.Write(b); return nil }
	archive.Create()

	done := make(chan error, 1)
	go func() {
		_, err := archive.WriteDir(root, "", true)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WriteDir: %v", err)
		}
	case <-time.After(10 * time.Second):
		// Open the write end so the stuck read-ahead can end.
		if f, err := os.OpenFile(filepath.Join(root, "a-pipe"), os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			f.Close()
		}
		t.Fatal("backup still running after 10 s: read-ahead is stuck on the FIFO")
	}

	if !strings.Contains(strings.Join(archive.SkippedFiles, "\n"), "Pipe (FIFO) (skipped, holds no file data)") {
		t.Errorf("FIFO not in the skipped list: %v", archive.SkippedFiles)
	}
	dest := t.TempDir()
	if _, err := NewPXARReader(p.Bytes()).ExtractAll(dest); err != nil {
		t.Fatalf("archive does not restore: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(dest, "z.txt")); !bytes.Equal(got, last) {
		t.Errorf("z.txt not backed up after the FIFO: %q", got)
	}
}
