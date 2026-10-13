//go:build windows

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pbscommon"
)

// Opt-in (PBS_E2E=1) end-to-end test of alternate data streams and NTFS
// extended attributes in folder backups: a tree with streams on files and a
// folder, a stream too large to record, extended attributes and a read-only
// file with a stream, backed up (classic and metadata change detection, the
// second metadata run reusing every file, with VSS when PBS_E2E_VSS is set)
// and restored sequentially and in parallel. The restore must bring back the
// streams, the extended attributes, the modification times (writing a stream
// changes them) and the read-only attribute; "Restore alternate data
// streams" unticked must bring back none.
func TestE2EStreamsAndEAs(t *testing.T) {
	if os.Getenv("PBS_E2E") == "" {
		t.Skip("set PBS_E2E=1")
	}
	useVSS := os.Getenv("PBS_E2E_VSS") != ""
	base := os.Getenv("PBS_E2E_DIR")
	if base == "" {
		base = t.TempDir()
	}
	src := filepath.Join(base, "streams-src")
	os.RemoveAll(src)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	mk := func(p string, data string) {
		t.Helper()
		must(os.MkdirAll(filepath.Dir(p), 0o755))
		must(os.WriteFile(p, []byte(data), 0o644))
	}
	zone := "[ZoneTransfer]\r\nZoneId=3\r\nReferrerUrl=https://example.com/\r\nHostUrl=https://example.com/setup.exe\r\n"
	mk(filepath.Join(src, "e2e-marker.txt"), "marker")
	mk(filepath.Join(src, "downloads", "setup.exe"), "an installer")
	mk(filepath.Join(src, "downloads", "setup.exe:Zone.Identifier"), zone)
	mk(filepath.Join(src, "downloads", "two streams.docx"), "a document")
	mk(filepath.Join(src, "downloads", "two streams.docx:Zone.Identifier"), zone)
	mk(filepath.Join(src, "downloads", "two streams.docx:com.dropbox.attrs"), "\x00\x01dropbox")
	mk(filepath.Join(src, "big stream.bin"), "x")
	must(os.WriteFile(filepath.Join(src, "big stream.bin")+":huge", bytes.Repeat([]byte("h"), pbscommon.MaxStreamBytes+10), 0o644))
	must(os.MkdirAll(filepath.Join(src, "folder with stream"), 0o755))
	must(os.WriteFile(filepath.Join(src, "folder with stream")+":folderinfo", []byte("on the folder"), 0o644))
	mk(filepath.Join(src, "wsl", "linux file"), "from WSL")
	if _, _, err := pbscommon.WriteStreamsAndEAs(filepath.Join(src, "wsl", "linux file"), nil, []pbscommon.EAEntry{
		{Name: "$LXUID", Value: []byte{0xe8, 0x03, 0, 0}}, {Name: "$LXMOD", Value: []byte{0xa4, 0x81, 0, 0}}, {Name: "USER.NOTE", Value: []byte("hello")},
	}); err != nil {
		t.Fatalf("setting extended attributes: %v", err)
	}
	ro := filepath.Join(src, "readonly with stream.txt")
	mk(ro, "read only")
	mk(ro+":note", "stream on a read-only file")
	must(os.Chmod(ro, 0o444))
	old := time.Date(2015, 3, 4, 5, 6, 7, 0, time.UTC)
	for _, p := range []string{"downloads/setup.exe", "downloads/two streams.docx", "wsl/linux file", "readonly with stream.txt"} {
		must(os.Chtimes(filepath.Join(src, filepath.FromSlash(p)), old, old))
	}

	// What every restore must bring back: streams and EAs per path, and mtimes.
	type want struct {
		streams map[string]string
		eas     map[string]string
		mtime   time.Time
	}
	expect := map[string]want{}
	filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		ss, eas, rerr := pbscommon.ReadStreamsAndEAs(p)
		if rerr != nil {
			t.Fatalf("reading %s: %v", p, rerr)
		}
		w := want{streams: map[string]string{}, eas: map[string]string{}, mtime: info.ModTime()}
		for _, s := range ss {
			if s.Skipped == "" {
				w.streams[s.Name] = string(s.Data)
			} else {
				w.streams[s.Name] = "(not recorded)"
			}
		}
		for _, e := range eas {
			w.eas[e.Name] = string(e.Value)
		}
		expect[filepath.ToSlash(rel)] = w
		return nil
	})
	if len(expect["wsl/linux file"].eas) != 3 || len(expect["downloads/two streams.docx"].streams) != 2 {
		t.Fatalf("source not set up: %+v", expect)
	}

	check := func(label, root string, skip bool) {
		t.Helper()
		bad := 0
		for rel, w := range expect {
			p := filepath.Join(root, filepath.FromSlash(rel))
			info, err := os.Stat(p)
			if err != nil {
				t.Errorf("%s: %s missing: %v", label, rel, err)
				bad++
				continue
			}
			ss, eas, _ := pbscommon.ReadStreamsAndEAs(p)
			got := map[string]string{}
			for _, s := range ss {
				got[s.Name] = string(s.Data)
			}
			for name, data := range w.streams {
				g, ok := got[name]
				switch {
				case skip && ok:
					t.Errorf("%s: %s:%s restored although streams were unticked", label, rel, name)
					bad++
				case skip:
				case data == "(not recorded)" && ok:
					t.Errorf("%s: %s:%s (too large to record) exists after restore", label, rel, name)
					bad++
				case data != "(not recorded)" && (!ok || g != data):
					t.Errorf("%s: %s:%s = %q, %v; want %q", label, rel, name, g, ok, data)
					bad++
				}
			}
			gotEA := map[string]string{}
			for _, e := range eas {
				gotEA[e.Name] = string(e.Value)
			}
			for name, v := range w.eas {
				if skip {
					if _, ok := gotEA[name]; ok {
						t.Errorf("%s: %s EA %s restored although streams were unticked", label, rel, name)
						bad++
					}
				} else if gotEA[name] != v {
					t.Errorf("%s: %s EA %s = %q; want %q", label, rel, name, gotEA[name], v)
					bad++
				}
			}
			// Whole seconds: the classic format writes no sub-second mtime on
			// Windows (by design, see entryMetaFor). A stream write changing
			// the time would show as today's date.
			if !info.IsDir() && info.ModTime().Unix() != w.mtime.Unix() {
				t.Errorf("%s: %s mtime %v; want %v", label, rel, info.ModTime(), w.mtime)
				bad++
			}
		}
		if info, err := os.Stat(filepath.Join(root, "readonly with stream.txt")); err == nil && info.Mode().Perm()&0o200 != 0 {
			t.Errorf("%s: read-only attribute not restored", label)
			bad++
		}
		t.Logf("%s: %d paths checked, %d problems", label, len(expect), bad)
	}

	for _, mode := range []string{"legacy", "metadata"} {
		backupID := fmt.Sprintf("e2e-streams-%s-%d", mode, time.Now().Unix())
		runs := 1
		if mode == "metadata" {
			runs = 2 // the second run reuses every unchanged file
		}
		var snapTime int64
		for r := 1; r <= runs; r++ {
			var rs *BackupStatus
			err := RunBackupInline(BackupOptions{
				BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
				BackupObjects: []string{src}, BackupID: backupID, BackupType: "host", Kind: "directory",
				Compression: "fastest", UseVSS: useVSS, ChangeDetectionMode: mode,
				OnResult: func(s *BackupStatus) { rs = s },
			})
			if err != nil || rs == nil || !rs.Success() {
				t.Fatalf("%s backup %d: %v %+v", mode, r, err, rs)
			}
			t.Logf("%s backup %d: reused files %d, vss=%v", mode, r, rs.ReusedFiles, useVSS)
			snapTime = rs.BackupTime
			time.Sleep(1100 * time.Millisecond)
		}
		for _, parallel := range []bool{false, true} {
			for _, skip := range []bool{false, true} {
				label := fmt.Sprintf("%s parallel=%v skipStreams=%v", mode, parallel, skip)
				dest := filepath.Join(base, fmt.Sprintf("streams-restore-%s-%v-%v", mode, parallel, skip))
				os.RemoveAll(dest)
				must(os.MkdirAll(dest, 0o755))
				err := RestoreSnapshotInline(RestoreOptions{
					BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
					BackupID: backupID, SnapshotTime: time.Unix(snapTime, 0).UTC(), DestPath: dest,
					Mode: RestoreModeAlternateAbs, Overwrite: true, ParallelExtraction: parallel,
					RestoreACLs: true, SkipStreams: skip,
				})
				if err != nil {
					t.Errorf("%s: restore: %v", label, err)
					continue
				}
				check(label, e2eFindRoot(t, dest), skip)
				clearReadOnlyTree(dest)
				os.RemoveAll(dest)
			}
		}
		t.Logf("snapshots left on PBS for cleanup: type=host id=%s", backupID)
	}
	clearReadOnlyTree(src)
	os.RemoveAll(src)
}

// clearReadOnlyTree makes every file under root writable so it can be removed.
func clearReadOnlyTree(root string) {
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.Contains(info.Mode().String(), "r--") {
			os.Chmod(p, 0o644)
		}
		return nil
	})
}
