//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Opt-in (PBS_IMG_URL and friends as TestLiveImageRestore, PBS_IMG_ID a
// Windows machine backup; run elevated): restores up to 40 files of its
// first NTFS volume with permissions, then checks that each restored file's
// DACL matches the one $Secure records for it, and that files with
// alternate data streams got them back.
func TestLiveImageACLsAndStreams(t *testing.T) {
	if os.Getenv("PBS_IMG_URL") == "" {
		t.Skip("set PBS_IMG_URL and friends")
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
	defer sess.close() // the volumes read $Secure through it at the end
	vols, opened, err := openImageVolumes(sess, opts, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	var vol *imageVolume
	for i := range vols {
		if vols[i].FS == "NTFS" {
			vol = &vols[i]
			break
		}
	}
	if vol == nil {
		t.Fatal("no NTFS volume")
	}
	// Files with streams first, then others, 40 in all.
	var picks []string
	withStreams := map[string]int{}
	for _, e := range vol.Entries {
		if !e.IsDir && len(e.Streams) > 0 && len(picks) < 20 {
			picks = append(picks, vol.Display+"/"+e.Path)
			withStreams[e.Path] = len(e.Streams)
		}
	}
	for _, e := range vol.Entries {
		if !e.IsDir && len(e.Streams) == 0 && len(picks) < 40 {
			picks = append(picks, vol.Display+"/"+e.Path)
		}
	}
	t.Logf("%s: %d entries, restoring %d files (%d with streams)", vol.Display, len(vol.Entries), len(picks), len(withStreams))

	dest := t.TempDir()
	opts.DestPath = dest
	opts.Mode = RestoreModeAlternateAbs
	opts.RestoreACLs = true
	opts.IncludePaths = picks
	start := time.Now()
	if err := RestoreSnapshotInline(opts); err != nil {
		t.Fatal(err)
	}
	t.Logf("restored in %v", time.Since(start).Round(time.Millisecond))

	// Where the restore put each file: below dest, ending in its path.
	restored := map[string]string{}
	_ = filepath.Walk(dest, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			restored[filepath.ToSlash(p)] = p
		}
		return nil
	})
	cache := map[uint32]string{}
	checked := 0
	for _, e := range vol.Entries {
		p := ""
		for k, v := range restored {
			if strings.HasSuffix(k, "/"+e.Path) {
				p = v
			}
		}
		if p == "" {
			continue
		}
		want, ok := imageSDDL(opened[vol.Display], e, cache)
		if !ok {
			continue
		}
		sd, err := windows.GetNamedSecurityInfo(p, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		wantDACL := want[strings.Index(want, "D:"):]
		if i := strings.Index(wantDACL, "S:"); i > 0 {
			wantDACL = wantDACL[:i]
		}
		// The restored file also inherits from the restore folder (an
		// unprotected DACL, as at the source): compare the explicit entries.
		if got := explicitDACL(sd.String()); got != explicitDACL(wantDACL) {
			t.Errorf("%s:\n got  %s\n want %s", e.Path, got, wantDACL)
		}
		if n := withStreams[e.Path]; n > 0 {
			for _, s := range e.Streams {
				if fi, err := os.Stat(p + ":" + s.Name); err != nil || fi.Size() != int64(s.Size) {
					t.Errorf("%s:%s not restored (%v)", e.Path, s.Name, err)
				}
			}
		}
		checked++
	}
	if checked == 0 {
		for k := range restored {
			t.Logf("restored: %s", k)
			break
		}
		t.Fatal("nothing checked")
	}
	t.Logf("%d restored files have the image's DACL", checked)
}

// explicitDACL drops inherited entries (ID in the flags field) and the
// auto-inherit flag from an SDDL DACL, leaving what the file sets itself.
func explicitDACL(s string) string {
	s = strings.Replace(s, "D:AI", "D:", 1)
	out := strings.Builder{}
	for _, part := range strings.SplitAfter(s, ")") {
		if open := strings.IndexByte(part, '('); open >= 0 {
			if f := strings.Split(part[open+1:], ";"); len(f) > 1 && strings.Contains(f[1], "ID") {
				out.WriteString(part[:open])
				continue
			}
		}
		out.WriteString(part)
	}
	return out.String()
}
