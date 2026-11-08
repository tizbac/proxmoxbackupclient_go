//go:build linux

package pbscommon

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestXattrCollectorCapturesUserXattr(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Setxattr(f, "user.test", []byte("hello"), 0); err != nil {
		t.Skipf("filesystem has no user xattr support: %v", err)
	}
	c := NewXattrCollector(dir, "h")
	fi, _ := os.Lstat(f)
	if err := c.Collect(f, fi, false); err != nil {
		t.Fatal(err)
	}
	m, err := c.FinalizeRaw()
	if err != nil || m == nil || len(m.Entries) != 1 {
		t.Fatalf("meta=%v err=%v", m, err)
	}
	e := m.Entries[0]
	if e.Path != "a.txt" || string(e.Xattrs["user.test"]) != "hello" {
		t.Fatalf("unexpected entry %+v", e)
	}
	if blob, err := (&CombinedBackupFileMeta{Archives: map[string]*BackupFileMeta{"backup": m}}).Serialize(); err != nil || len(blob) == 0 {
		t.Fatalf("serialize: %v", err)
	}
}
