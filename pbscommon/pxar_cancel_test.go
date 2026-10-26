package pbscommon

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Ported from upstream #117, plus the fork's Ctx and split archive cases.

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

// The GUI cancels through Ctx: the same stop part way through a file, ending
// with context.Canceled, which the GUI reports as a stopped backup.
func TestPXARCtxCancelStopsInsideAFile(t *testing.T) {
	root := t.TempDir()
	big := bytes.Repeat([]byte("x"), 4<<20)
	if err := os.WriteFile(filepath.Join(root, "big.bin"), big, 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var written int
	archive := &PXARArchive{ArchiveName: "backup.pxar.didx", Ctx: ctx}
	archive.WriteCB = func(b []byte) error {
		written += len(b)
		if written > 1<<20 {
			cancel()
		}
		return nil
	}
	archive.Create()

	_, err := archive.WriteDir(root, "", true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WriteDir error = %v, want context.Canceled", err)
	}
	if written > 1<<20+64*1024 {
		t.Errorf("%d bytes written, want no more than one 64 KiB block after the cancel", written)
	}
}

// A split archive writes file contents to the payload stream: a cancel stops
// that stream part way through a file too.
func TestPXARCancelStopsInsideASplitPayload(t *testing.T) {
	root := t.TempDir()
	big := bytes.Repeat([]byte("p"), 4<<20)
	if err := os.WriteFile(filepath.Join(root, "big.bin"), big, 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var payload int
	archive := &PXARArchive{ArchiveName: "t.mpxar.didx", Split: true, Ctx: ctx}
	archive.WriteCB = func(b []byte) error { return nil }
	archive.PayloadWriteCB = func(b []byte) error {
		payload += len(b)
		if payload > 1<<20 {
			cancel()
		}
		return nil
	}
	archive.Create()

	_, err := archive.WriteDir(root, "", true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WriteDir error = %v, want context.Canceled", err)
	}
	if payload > 1<<20+64*1024 {
		t.Errorf("%d payload bytes written, want no more than one 64 KiB block after the cancel", payload)
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

// Without a cancel the hook and a live Ctx change nothing.
func TestPXARCancelHookUnsetOrFalse(t *testing.T) {
	root := t.TempDir()
	makeTimedTree(t, root)
	want, _ := writeTestArchive(t, root, false)

	var p bytes.Buffer
	archive := &PXARArchive{ArchiveName: "backup.pxar.didx", Ctx: context.Background()}
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
