//go:build linux

package pbscommon

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func buildPosixTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "sub", "deep"), 0o750))
	must(os.WriteFile(filepath.Join(root, "plain.txt"), []byte("plain"), 0o640))
	must(os.Chmod(filepath.Join(root, "plain.txt"), 0o640))
	must(os.WriteFile(filepath.Join(root, "suid"), []byte("x"), 0o755))
	must(os.Chmod(filepath.Join(root, "suid"), os.ModeSetuid|0o755))
	must(os.WriteFile(filepath.Join(root, "sub", "deep", "f.txt"), []byte("deep"), 0o600))
	must(os.Chmod(filepath.Join(root, "sub"), 0o750))
	must(os.Symlink("plain.txt", filepath.Join(root, "link-rel")))
	must(os.Symlink("/etc/hostname", filepath.Join(root, "sub", "link-abs")))
	must(os.Symlink("does-not-exist", filepath.Join(root, "dangling")))
	must(syscall.Mkfifo(filepath.Join(root, "fifo"), 0o644))
	mt := time.Unix(1_700_000_000, 123_456_789)
	must(os.Chtimes(filepath.Join(root, "plain.txt"), mt, mt))
	return root
}

func writePosixArchive(t *testing.T, root string) (pxar, catalog []byte, a *PXARArchive) {
	t.Helper()
	var pb, cb bytes.Buffer
	a = &PXARArchive{
		ArchiveName: "test.pxar.didx",
		WriteCB:     func(b []byte) error { pb.Write(b); return nil },
		CatalogWriteCB: func(b []byte) error {
			cb.Write(b)
			return nil
		},
	}
	a.Create()
	if _, err := a.WriteDir(root, "", true); err != nil {
		t.Fatal(err)
	}
	return pb.Bytes(), cb.Bytes(), a
}

func TestPosixArchiveRoundTrip(t *testing.T) {
	root := buildPosixTree(t)
	pxar, catalog, a := writePosixArchive(t, root)

	// The FIFO is skipped and reported, not hung on.
	foundFifo := false
	for _, s := range a.SkippedFiles {
		if filepath.Base(s) == "fifo" {
			foundFifo = true
		}
	}
	if !foundFifo {
		t.Errorf("fifo not reported as skipped: %v", a.SkippedFiles)
	}

	// Catalog lists the symlinks (by name) and not the fifo.
	cat, err := ParseCatalog(catalog)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, e := range cat {
		have[e.Path] = true
	}
	for _, p := range []string{"test.pxar.didx/link-rel", "test.pxar.didx/dangling", "test.pxar.didx/sub/link-abs", "test.pxar.didx/plain.txt"} {
		if !have[p] {
			t.Errorf("catalog missing %s (have %v)", p, have)
		}
	}
	if have["test.pxar.didx/fifo"] {
		t.Error("catalog lists the fifo")
	}

	// Reader sees real modes, symlink targets and nanosecond mtime.
	pr := NewPXARReader(pxar)
	entries, err := pr.ListEntries()
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]PXARTreeEntry{}
	for _, e := range entries {
		byPath[e.Path] = e
	}
	if e := byPath["plain.txt"]; e.Mode&0o7777 != 0o640 || e.ModTime != 1_700_000_000 || e.ModNanos != 123_456_789 {
		t.Errorf("plain.txt: mode %o mtime %d.%d", e.Mode&0o7777, e.ModTime, e.ModNanos)
	}
	if e := byPath["suid"]; e.Mode&0o7777 != 0o4755 {
		t.Errorf("suid mode %o", e.Mode&0o7777)
	}
	if e := byPath["sub"]; e.Mode&0o7777 != 0o750 {
		t.Errorf("sub mode %o", e.Mode&0o7777)
	}
	if e := byPath["link-rel"]; !e.IsSymlink || e.LinkTarget != "plain.txt" {
		t.Errorf("link-rel: %+v", e)
	}
	if e := byPath["sub/link-abs"]; !e.IsSymlink || e.LinkTarget != "/etc/hostname" {
		t.Errorf("link-abs: %+v", e)
	}
	if e := byPath["dangling"]; !e.IsSymlink || e.LinkTarget != "does-not-exist" {
		t.Errorf("dangling: %+v", e)
	}
	if _, ok := byPath["fifo"]; ok {
		t.Error("fifo was archived")
	}
	if e := byPath["plain.txt"]; e.UID != uint32(os.Getuid()) || e.GID != uint32(os.Getgid()) {
		t.Errorf("owner %d:%d, want %d:%d", e.UID, e.GID, os.Getuid(), os.Getgid())
	}

	// Extract with ownership/mode restore and compare.
	dest := t.TempDir()
	pr = NewPXARReader(pxar)
	pr.SetRestoreOwnership(true)
	res, err := pr.ExtractAll(dest)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.Skipped {
			t.Errorf("skipped %s: %s", r.Path, r.SkipReason)
		}
	}
	if tgt, err := os.Readlink(filepath.Join(dest, "link-rel")); err != nil || tgt != "plain.txt" {
		t.Errorf("link-rel restored as %q, %v", tgt, err)
	}
	if tgt, err := os.Readlink(filepath.Join(dest, "dangling")); err != nil || tgt != "does-not-exist" {
		t.Errorf("dangling restored as %q, %v", tgt, err)
	}
	if fi, _ := os.Stat(filepath.Join(dest, "plain.txt")); fi == nil || fi.Mode().Perm() != 0o640 ||
		fi.ModTime().UnixNano() != 1_700_000_000_123_456_789 {
		t.Errorf("plain.txt restored wrong: %v", fi)
	}
	if fi, _ := os.Stat(filepath.Join(dest, "suid")); fi == nil || fi.Mode()&os.ModeSetuid == 0 {
		t.Errorf("setuid lost: %v", fi)
	}
	if _, err := os.Lstat(filepath.Join(dest, "fifo")); err == nil {
		t.Error("fifo was restored")
	}

	// Without the ownership switch the old 0777-masked behaviour applies.
	dest2 := t.TempDir()
	if _, err := NewPXARReader(pxar).ExtractAll(dest2); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(filepath.Join(dest2, "suid")); fi == nil || fi.Mode()&os.ModeSetuid != 0 {
		t.Errorf("setuid applied without SetRestoreOwnership: %v", fi)
	}
}

func TestSelectiveSymlinkRestoreViaBST(t *testing.T) {
	root := buildPosixTree(t)
	pxar, _, _ := writePosixArchive(t, root)
	dest := t.TempDir()
	pr := NewPXARReader(pxar)
	if _, err := pr.ExtractFiltered(dest, []string{"sub"}, false); err != nil {
		t.Fatal(err)
	}
	if tgt, err := os.Readlink(filepath.Join(dest, "sub", "link-abs")); err != nil || tgt != "/etc/hostname" {
		t.Errorf("link-abs: %q %v", tgt, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "sub", "deep", "f.txt")); err != nil {
		t.Error(err)
	}
}

func TestUnderCreatedLink(t *testing.T) {
	pr := &PXARReader{createdLinks: map[string]struct{}{"/r/a": {}}}
	if !pr.underCreatedLink("/r/a/passwd") || !pr.underCreatedLink("/r/a/b/c") {
		t.Error("path through created link not detected")
	}
	if pr.underCreatedLink("/r/a") || pr.underCreatedLink("/r/ab/c") {
		t.Error("false positive")
	}
}

func TestApplyDirMetadata(t *testing.T) {
	d := filepath.Join(t.TempDir(), "d")
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	ApplyDirMetadata(d, uint32(IFDIR|0o1750), uint32(os.Getuid()), uint32(os.Getgid()))
	fi, err := os.Stat(d)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSticky == 0 || fi.Mode().Perm() != 0o750 {
		t.Fatalf("dir mode %v, want sticky 0750", fi.Mode())
	}
}
