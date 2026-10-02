package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"pbscommon"
	"strings"
	"testing"
)

func writeVerifyFile(t *testing.T, dir, name, content string) pbscommon.PXARExtractedFile {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return pbscommon.PXARExtractedFile{Path: p, Size: uint64(len(content)), SHA256: sum[:]}
}

func TestVerifyRestoredFiles(t *testing.T) {
	dir := t.TempDir()
	good := writeVerifyFile(t, dir, "good.txt", "hello world")
	empty := writeVerifyFile(t, dir, "empty.txt", "")

	sameSize := writeVerifyFile(t, dir, "samesize.txt", "AAAAAAAA")
	if err := os.WriteFile(sameSize.Path, []byte("AAAAAAAB"), 0644); err != nil {
		t.Fatal(err)
	}
	wrongSize := writeVerifyFile(t, dir, "short.txt", "twelve bytes")
	if err := os.WriteFile(wrongSize.Path, []byte("twelve"), 0644); err != nil {
		t.Fatal(err)
	}
	missing := writeVerifyFile(t, dir, "gone.txt", "bye")
	os.Remove(missing.Path)

	files := []pbscommon.PXARExtractedFile{
		good, empty, sameSize, wrongSize, missing,
		{Path: filepath.Join(dir, "dir"), IsDir: true},
		{Path: filepath.Join(dir, "skipped"), Skipped: true, SHA256: bytes.Repeat([]byte{1}, 32)},
		{Path: filepath.Join(dir, "nohash")},
	}
	verified, fails, err := verifyRestoredFiles(context.Background(), files, nil)
	if err != nil {
		t.Fatal(err)
	}
	if verified != 2 {
		t.Errorf("verified = %d, want 2 (good, empty)", verified)
	}
	if len(fails) != 3 {
		t.Fatalf("failures = %v, want 3", fails)
	}
	joined := strings.Join(fails, "\n")
	for _, want := range []string{"samesize.txt: content on disk differs", "short.txt: size on disk is 6 bytes, snapshot has 12", "gone.txt: cannot be read back"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing failure %q in:\n%s", want, joined)
		}
	}
	for _, f := range []string{"good.txt", "empty.txt", "samesize.txt", "short.txt"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s was removed by verify: %v", f, err)
		}
	}
}

func TestVerifyRestoredFilesCancel(t *testing.T) {
	dir := t.TempDir()
	f := writeVerifyFile(t, dir, "a.txt", "x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := verifyRestoredFiles(ctx, []pbscommon.PXARExtractedFile{f}, nil); err == nil {
		t.Error("expected cancel error")
	}
}
