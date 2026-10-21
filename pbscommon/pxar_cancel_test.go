package pbscommon

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A cancel stops the walk inside a file, not at the end of the folder: the
// write fails with ErrWriteCancelled long before the whole file is written.
func TestPXARCancelStopsInsideAFile(t *testing.T) {
	root := t.TempDir()
	big := bytes.Repeat([]byte("x"), 4<<20)
	if err := os.WriteFile(filepath.Join(root, "big.bin"), big, 0644); err != nil {
		t.Fatal(err)
	}
	makeTimedTree(t, root)

	var written int
	cancelled := false
	archive := &PXARArchive{ArchiveName: "backup.pxar.didx"}
	archive.WriteCB = func(b []byte) error {
		written += len(b)
		// Cancel once 1 MiB of big.bin is out.
		if written > 1<<20 {
			cancelled = true
		}
		return nil
	}
	archive.CatalogWriteCB = func(b []byte) error { return nil }
	archive.Cancelled = func() bool { return cancelled }
	archive.Create()

	_, err := archive.WriteDir(root, "", true)
	if !errors.Is(err, ErrWriteCancelled) {
		t.Fatalf("WriteDir error = %v, want ErrWriteCancelled", err)
	}
	if written >= len(big) {
		t.Errorf("%d bytes written after the cancel, the file was not interrupted", written)
	}
}

// A write buffered in one go (a virtual file here) is cut at the next 64 KiB
// block too, not drained to the end.
func TestPXARCancelStopsInsideABufferedWrite(t *testing.T) {
	payload := bytes.Repeat([]byte("m"), 1<<20)

	var written int
	archive := &PXARArchive{ArchiveName: "backup.pxar.didx"}
	archive.WriteCB = func(b []byte) error { written += len(b); return nil }
	// The first block goes out, the cancel lands during it.
	archive.Cancelled = func() bool { return written > 0 }
	archive.Create()

	_, err := archive.WriteVirtualFile("meta.json", payload, 0)
	if !errors.Is(err, ErrWriteCancelled) {
		t.Fatalf("WriteVirtualFile error = %v, want ErrWriteCancelled", err)
	}
	if written != 64*1024 {
		t.Errorf("%d bytes written, want exactly one 64 KiB block before the cancel", written)
	}
}

// Without a cancel the hook changes nothing.
func TestPXARCancelHookUnsetOrFalse(t *testing.T) {
	root := t.TempDir()
	makeTimedTree(t, root)
	want, _ := writeTestArchive(t, root, false)

	var p bytes.Buffer
	archive := &PXARArchive{ArchiveName: "backup.pxar.didx"}
	archive.WriteCB = func(b []byte) error { p.Write(b); return nil }
	archive.CatalogWriteCB = func(b []byte) error { return nil }
	archive.Cancelled = func() bool { return false }
	archive.Create()
	if _, err := archive.WriteDir(root, "", true); err != nil {
		t.Fatalf("WriteDir: %v", err)
	}
	if !bytes.Equal(p.Bytes(), want) {
		t.Error("an archive with a Cancelled hook that never fires differs from one without")
	}
}
