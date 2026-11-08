//go:build !linux

package pbscommon

import (
	"os"
	"testing"
)

// Archives written off Linux must keep the historic fixed shape.
func TestEntryMetaLegacyShape(t *testing.T) {
	fi, err := os.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	mode, uid, gid, secs, nanos := entryMeta(fi, IFREG)
	if mode != IFREG|0o777 || uid != 1000 || gid != 1000 || nanos != 0 || secs != uint64(fi.ModTime().Unix()) {
		t.Errorf("got %o %d %d %d %d", mode, uid, gid, secs, nanos)
	}
	if posixFidelity {
		t.Error("posixFidelity must be false off Linux")
	}
}
