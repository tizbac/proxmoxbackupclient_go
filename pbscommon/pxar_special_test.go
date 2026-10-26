//go:build linux

package pbscommon

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A FIFO, a socket and a device node in a backed-up folder hold no file
// data. Opening the FIFO used to block until a writer appeared, so the
// backup hung for good; the socket could not be opened (a read error); the
// device node was opened as the device and stored as an empty regular file.
// They are now skipped like junction points, and everything around them is
// backed up.
func TestSpecialFilesSkipped(t *testing.T) {
	root := t.TempDir()
	files := map[string][]byte{"a.txt": []byte("first\n"), "z.txt": []byte("last\n")}
	for n, d := range files {
		if err := os.WriteFile(filepath.Join(root, n), d, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0644); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("unix", filepath.Join(root, "sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	// A device node needs root to create; /dev/null's twin is enough.
	devMade := syscall.Mknod(filepath.Join(root, "null"), syscall.S_IFCHR|0666, int(1<<8|3)) == nil

	var p bytes.Buffer
	archive := &PXARArchive{ArchiveName: "backup.pxar.didx"}
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
		// Unblock the stuck open so the goroutine can end.
		if f, err := os.OpenFile(filepath.Join(root, "pipe"), os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			f.Close()
		}
		t.Fatal("backup still running after 10 s: it is stuck opening the FIFO")
	}

	if len(archive.ReadErrors) != 0 {
		t.Errorf("read errors %v, want none: special files are skipped, not unreadable", archive.ReadErrors)
	}
	want := []string{"Pipe (FIFO) (skipped, holds no file data)", "Socket (skipped, holds no file data)"}
	if devMade {
		want = append(want, "Device (skipped, holds no file data)")
	}
	skipped := strings.Join(archive.SkippedFiles, "\n")
	for _, w := range want {
		if !strings.Contains(skipped, w) {
			t.Errorf("skipped list has no %q:\n%s", w, skipped)
		}
	}

	dest := t.TempDir()
	if _, err := NewPXARReader(p.Bytes()).ExtractAll(dest); err != nil {
		t.Fatalf("archive does not restore: %v", err)
	}
	for n, d := range files {
		if got, _ := os.ReadFile(filepath.Join(dest, n)); !bytes.Equal(got, d) {
			t.Errorf("%s not backed up around the special files", n)
		}
	}
	for _, n := range []string{"pipe", "sock", "null"} {
		if _, err := os.Lstat(filepath.Join(dest, n)); err == nil {
			t.Errorf("%s was restored, want it left out", n)
		}
	}
	t.Logf("device node created and checked: %v", devMade)
}
