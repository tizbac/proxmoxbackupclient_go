package pbscommon

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// failingFile reads a real file but fails once failAt bytes have been read,
// like a file with a bad sector or a range another process has locked.
type failingFile struct {
	f      *os.File
	pos    int64
	failAt int64
	err    error
}

func (ff *failingFile) Read(p []byte) (int, error) {
	if ff.pos >= ff.failAt {
		return 0, ff.err
	}
	if room := ff.failAt - ff.pos; int64(len(p)) > room {
		p = p[:room]
	}
	n, err := ff.f.Read(p)
	ff.pos += int64(n)
	return n, err
}

func (ff *failingFile) Close() error { return ff.f.Close() }

// A read error part way through one file used to end the whole backup. Now
// that file is zero-padded after its readable part and flagged, and the
// files after it are still backed up.
func TestReadErrorMidFilePadsAndContinues(t *testing.T) {
	root := t.TempDir()
	big := make([]byte, 3*1024*1024)
	rand.Read(big)
	files := map[string][]byte{"a.txt": []byte("first\n"), "big.bin": big, "z.txt": []byte("last\n")}
	for n, d := range files {
		if err := os.WriteFile(filepath.Join(root, n), d, 0644); err != nil {
			t.Fatal(err)
		}
	}

	const failAt = 1024*1024 + 4096
	readErr := errors.New("the process cannot access the file because another process has locked a portion of the file")
	oldOpen := openForRead
	t.Cleanup(func() { openForRead = oldOpen })
	openForRead = func(path string) (io.ReadCloser, error) {
		f, err := os.Open(path)
		if err != nil || filepath.Base(path) != "big.bin" {
			return f, err
		}
		return &failingFile{f: f, failAt: failAt, err: readErr}, nil
	}

	var p bytes.Buffer
	archive := &PXARArchive{ArchiveName: "backup.pxar.didx"}
	archive.WriteCB = func(b []byte) error { p.Write(b); return nil }
	archive.Create()
	if _, err := archive.WriteDir(root, "", true); err != nil {
		t.Fatalf("backup failed on one unreadable file: %v", err)
	}

	if len(archive.ReadErrors) != 1 ||
		!strings.Contains(archive.ReadErrors[0], "Read failed after 1052672 of 3145728 bytes") ||
		!strings.Contains(archive.ReadErrors[0], "locked a portion") {
		t.Fatalf("read errors %v, want one for big.bin with the cause", archive.ReadErrors)
	}

	dest := t.TempDir()
	if _, err := NewPXARReader(p.Bytes()).ExtractAll(dest); err != nil {
		t.Fatalf("archive does not restore: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dest, "big.bin"))
	want := append(append([]byte{}, big[:failAt]...), make([]byte, len(big)-failAt)...)
	if !bytes.Equal(got, want) {
		t.Errorf("big.bin is not its readable part plus zero padding (%d bytes)", len(got))
	}
	for _, n := range []string{"a.txt", "z.txt"} {
		if g, _ := os.ReadFile(filepath.Join(dest, n)); !bytes.Equal(g, files[n]) {
			t.Errorf("%s not backed up around the unreadable file", n)
		}
	}
}
