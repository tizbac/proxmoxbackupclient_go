//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pbscommon"
)

// Opt-in (PBS_IMG_URL and friends as TestLiveImageACLsAndStreams, PBS_IMG_ID
// a Windows machine backup; plus PBS_IMG_SRC, a folder on the backed-up
// volume such as T:\eatest, and PBS_IMG_SUB, its path in the volume such as
// eatest; run elevated): restores that folder from the newest snapshot and
// checks that every file and folder in it came back with the source's
// extended attributes, alternate data streams and modification time.
func TestLiveImageExtendedAttributes(t *testing.T) {
	src, sub := os.Getenv("PBS_IMG_SRC"), os.Getenv("PBS_IMG_SUB")
	if os.Getenv("PBS_IMG_URL") == "" || src == "" || sub == "" {
		t.Skip("set PBS_IMG_URL and friends, PBS_IMG_SRC and PBS_IMG_SUB")
	}
	opts := RestoreOptions{
		BaseURL: os.Getenv("PBS_IMG_URL"), CertFingerprint: os.Getenv("PBS_IMG_FP"), AuthID: os.Getenv("PBS_IMG_AUTHID"),
		Secret: os.Getenv("PBS_IMG_SECRET"), Datastore: os.Getenv("PBS_IMG_STORE"), BackupID: os.Getenv("PBS_IMG_ID"),
	}
	snaps, err := ListSnapshotsInline(opts.BaseURL, opts.AuthID, opts.Secret, "", "", opts.Datastore, "", opts.CertFingerprint, opts.BackupID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range snaps {
		if s.BackupID == opts.BackupID && s.BackupTime.After(opts.SnapshotTime) {
			opts.SnapshotTime = s.BackupTime
		}
	}
	fillSnapshotKind(&opts)
	sess, err := openImageSession(opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	vols, _, err := openImageVolumes(sess, opts, true, nil)
	if err != nil {
		sess.close()
		t.Fatal(err)
	}
	display, withEAs := "", 0
	for _, v := range vols {
		for _, e := range v.Entries {
			if e.Path == sub && e.IsDir {
				display = v.Display
			}
		}
		if display == v.Display {
			for _, e := range v.Entries {
				if strings.HasPrefix(e.Path, sub+"/") && e.HasEAs {
					withEAs++
				}
			}
			break
		}
	}
	sess.close()
	if display == "" {
		t.Fatalf("no volume in snapshot %v has a folder %s", opts.SnapshotTime, sub)
	}
	t.Logf("snapshot %v, volume %s: %d entries under %s flagged with extended attributes", opts.SnapshotTime, display, withEAs, sub)

	dest := t.TempDir()
	opts.DestPath = dest
	opts.Mode = RestoreModeAlternateAbs
	opts.IncludePaths = []string{display + "/" + sub}
	if err := RestoreSnapshotInline(opts); err != nil {
		t.Fatal(err)
	}
	restored := ""
	_ = filepath.Walk(dest, func(p string, fi os.FileInfo, err error) error {
		if err == nil && fi.IsDir() && strings.HasSuffix(filepath.ToSlash(p), "/"+sub) {
			restored = p
		}
		return nil
	})
	if restored == "" {
		t.Fatalf("restored folder %s not found under %s", sub, dest)
	}

	checkedEAs, checked := 0, 0
	err = filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		got := filepath.Join(restored, rel)
		gfi, err := os.Stat(got)
		if err != nil {
			t.Errorf("%s: not restored: %v", rel, err)
			return nil
		}
		if !gfi.ModTime().Equal(fi.ModTime()) {
			t.Errorf("%s: modified %v, source %v", rel, gfi.ModTime(), fi.ModTime())
		}
		ws, we, err := pbscommon.ReadStreamsAndEAs(p)
		if err != nil {
			t.Fatal(err)
		}
		gs, ge, err := pbscommon.ReadStreamsAndEAs(got)
		if err != nil {
			t.Fatal(err)
		}
		if len(ge) != len(we) {
			t.Errorf("%s: %d extended attributes, source has %d", rel, len(ge), len(we))
		} else {
			for i := range we {
				if ge[i].Name != we[i].Name || ge[i].Flags != we[i].Flags || !bytes.Equal(ge[i].Value, we[i].Value) {
					t.Errorf("%s: attribute %s came back as %s (%d bytes, source %d)", rel, we[i].Name, ge[i].Name, len(ge[i].Value), len(we[i].Value))
				}
			}
			checkedEAs += len(we)
		}
		if len(gs) != len(ws) {
			t.Errorf("%s: %d streams, source has %d", rel, len(gs), len(ws))
		} else {
			for i := range ws {
				if gs[i].Name != ws[i].Name || !bytes.Equal(gs[i].Data, ws[i].Data) {
					t.Errorf("%s: stream %s differs", rel, ws[i].Name)
				}
			}
		}
		checked++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checkedEAs == 0 {
		t.Fatal("no extended attributes in the source folder to compare")
	}
	t.Logf("%d files and folders compared, %d extended attributes identical", checked, checkedEAs)
}
