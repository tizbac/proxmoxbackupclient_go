//go:build windows

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in end-to-end test of the NTFS metadata side file against a real PBS:
// a folder is backed up (with VSS when PBS_E2E_VSS is set, which needs an
// elevated shell) and restored elsewhere, with and without ACLs, and the
// restored tree is compared with the source.
//
//	PBS_E2E=1 PBS_E2E_URL=https://pbs:8007 PBS_E2E_AUTHID=user@pbs!token
//	PBS_E2E_SECRET=... PBS_E2E_STORE=store [PBS_E2E_FP=AA:BB:...]
//	[PBS_E2E_ID_PREFIX=e2e-] [PBS_E2E_VSS=1] go test -run TestE2ENTFSMeta
//
// The snapshots are left on the server; the log names them.
type e2eNTFSServer struct {
	url, authID, secret, store, fp string
}

func e2eNTFSServerFromEnv(t *testing.T) e2eNTFSServer {
	t.Helper()
	if os.Getenv("PBS_E2E") == "" {
		t.Skip("set PBS_E2E=1 and the PBS_E2E_* connection variables")
	}
	s := e2eNTFSServer{
		url: os.Getenv("PBS_E2E_URL"), authID: os.Getenv("PBS_E2E_AUTHID"),
		secret: os.Getenv("PBS_E2E_SECRET"), store: os.Getenv("PBS_E2E_STORE"), fp: os.Getenv("PBS_E2E_FP"),
	}
	if s.url == "" || s.authID == "" || s.secret == "" || s.store == "" {
		t.Fatal("PBS_E2E_URL, PBS_E2E_AUTHID, PBS_E2E_SECRET and PBS_E2E_STORE are required")
	}
	return s
}

// e2eNTFSBackupRestore backs up src once and restores the snapshot into a new
// folder per variant of restore options, calling check on the restored copy
// of src.
func e2eNTFSBackupRestore(t *testing.T, name, src string, variants map[string]RestoreOptions, check func(label, root string)) {
	t.Helper()
	srv := e2eNTFSServerFromEnv(t)
	prefix := os.Getenv("PBS_E2E_ID_PREFIX")
	if prefix == "" {
		prefix = "e2e-"
	}
	useVSS := os.Getenv("PBS_E2E_VSS") != ""
	backupID := fmt.Sprintf("%s%s-%d", prefix, name, time.Now().Unix())
	var rs *BackupStatus
	err := RunBackupInline(BackupOptions{
		BaseURL: srv.url, AuthID: srv.authID, Secret: srv.secret, Datastore: srv.store, CertFingerprint: srv.fp,
		BackupObjects: []string{src}, BackupID: backupID, BackupType: "host", Kind: "directory",
		Compression: "fastest", UseVSS: useVSS,
		OnResult: func(s *BackupStatus) { rs = s },
	})
	if err != nil || rs == nil || !rs.Success() {
		t.Fatalf("backup: %v %+v", err, rs)
	}
	t.Logf("snapshot host/%s/%d (vss=%v) left on PBS", backupID, rs.BackupTime, useVSS)

	for label, opts := range variants {
		dest := filepath.Join(t.TempDir(), "restore")
		if err := os.MkdirAll(dest, 0o755); err != nil {
			t.Fatal(err)
		}
		opts.BaseURL, opts.AuthID, opts.Secret, opts.Datastore, opts.CertFingerprint = srv.url, srv.authID, srv.secret, srv.store, srv.fp
		opts.BackupID, opts.SnapshotTime, opts.DestPath = backupID, time.Unix(rs.BackupTime, 0).UTC(), dest
		opts.Mode, opts.Overwrite = RestoreModeAlternateAbs, true
		if err := RestoreSnapshotInline(opts); err != nil {
			t.Errorf("%s: restore: %v", label, err)
			continue
		}
		check(label, e2eNTFSFindRoot(t, dest))
		e2eNTFSMakeWritable(dest)
	}
}

// e2eNTFSFindRoot returns the restored copy of the source folder, the one
// holding e2e-marker.txt.
func e2eNTFSFindRoot(t *testing.T, dest string) string {
	t.Helper()
	var found string
	filepath.Walk(dest, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() == "e2e-marker.txt" && found == "" {
			found = filepath.Dir(p)
		}
		return nil
	})
	if found == "" {
		t.Fatalf("e2e-marker.txt not found in %s", dest)
	}
	return found
}

// e2eNTFSMakeWritable clears read-only attributes so t.TempDir can clean up.
func e2eNTFSMakeWritable(root string) {
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Mode().Perm()&0o200 == 0 {
			os.Chmod(p, 0o644)
		}
		return nil
	})
}

// Creation times of files and folders come back, with and without ACLs, and
// the modification times stay as backed up.
func TestE2ENTFSMetaCreationTimes(t *testing.T) {
	e2eNTFSServerFromEnv(t)
	src := filepath.Join(t.TempDir(), "ntfs-meta-src")
	mk := func(rel, data string) string {
		t.Helper()
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	mk("e2e-marker.txt", "marker")
	mk("docs/report.txt", "a report")
	mk("docs/old/letter.txt", "a letter")
	ro := mk("read only.txt", "read only")
	if err := os.Chmod(ro, 0o444); err != nil {
		t.Fatal(err)
	}
	created := map[string]time.Time{
		"docs/report.txt":     time.Date(2004, 5, 6, 7, 8, 9, 0, time.UTC),
		"docs/old/letter.txt": time.Date(1999, 12, 31, 23, 59, 58, 0, time.UTC),
		"read only.txt":       time.Date(2010, 1, 2, 3, 4, 5, 0, time.UTC),
		"docs/old":            time.Date(1998, 2, 3, 4, 5, 6, 0, time.UTC),
		"docs":                time.Date(2001, 7, 8, 9, 10, 11, 0, time.UTC),
	}
	mtime := time.Date(2015, 3, 4, 5, 6, 7, 0, time.UTC)
	// Deepest first, so writing a time on an entry never touches one set before.
	for _, rel := range []string{"docs/report.txt", "docs/old/letter.txt", "read only.txt", "docs/old", "docs"} {
		p := filepath.Join(src, filepath.FromSlash(rel))
		setCreatedForTest(t, p, created[rel])
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}

	e2eNTFSBackupRestore(t, "ctime", src, map[string]RestoreOptions{
		"no ACLs":   {},
		"with ACLs": {RestoreACLs: true},
	}, func(label, root string) {
		for rel, want := range created {
			p := filepath.Join(root, filepath.FromSlash(rel))
			if got := readCreatedForTest(t, p); !got.Equal(want) {
				t.Errorf("%s: %s creation time %v, want %v", label, rel, got.UTC(), want)
			}
			if fi, err := os.Stat(p); err != nil {
				t.Errorf("%s: %s: %v", label, rel, err)
			} else if fi.ModTime().Unix() != mtime.Unix() {
				t.Errorf("%s: %s modification time %v, want %v", label, rel, fi.ModTime().UTC(), mtime)
			}
		}
		t.Logf("%s: %d creation times checked", label, len(created))
	})
}

// Alternate data streams (on files and a folder, an empty one, one too large
// to record) and extended attributes come back when RestoreADS is set and not
// otherwise; the modification and creation times stay as backed up although
// writing a stream changes them, and the read-only attribute is applied after
// the streams.
func TestE2ENTFSMetaStreams(t *testing.T) {
	e2eNTFSServerFromEnv(t)
	src := filepath.Join(t.TempDir(), "ntfs-streams-src")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	mk := func(rel, data string) string {
		t.Helper()
		p := filepath.Join(src, filepath.FromSlash(rel))
		must(os.MkdirAll(filepath.Dir(p), 0o755))
		must(os.WriteFile(p, []byte(data), 0o644))
		return p
	}
	zone := "[ZoneTransfer]\r\nZoneId=3\r\nReferrerUrl=https://example.com/\r\nHostUrl=https://example.com/setup.exe\r\n"
	mk("e2e-marker.txt", "marker")
	setup := mk("downloads/setup.exe", "an installer")
	mk("downloads/setup.exe:Zone.Identifier", zone)
	two := mk("downloads/two streams.docx", "a document")
	mk("downloads/two streams.docx:Zone.Identifier", zone)
	mk("downloads/two streams.docx:com.dropbox.attrs", "\x00\x01dropbox")
	mk("downloads/two streams.docx:empty", "")
	bigFile := mk("big stream.bin", "x")
	must(os.WriteFile(bigFile+":huge", bytes.Repeat([]byte("h"), maxStreamBytes+10), 0o644))
	folder := filepath.Join(src, "folder with stream")
	must(os.MkdirAll(folder, 0o755))
	must(os.WriteFile(folder+":folderinfo", []byte("on the folder"), 0o644))
	wsl := mk("wsl/linux file", "from WSL")
	must(setEAs(wsl, []EAEntry{{Name: "$LXUID", Value: []byte{0xe8, 0x03, 0, 0}}, {Name: "$LXMOD", Value: []byte{0xa4, 0x81, 0, 0}}, {Name: "USER.NOTE", Value: []byte("hello")}}))
	ro := mk("readonly with stream.txt", "read only")
	mk("readonly with stream.txt:note", "stream on a read-only file")

	old := time.Date(2015, 3, 4, 5, 6, 7, 0, time.UTC)
	created := time.Date(2004, 5, 6, 7, 8, 9, 0, time.UTC)
	for _, p := range []string{setup, two, bigFile, wsl, ro, folder} {
		setCreatedForTest(t, p, created)
		must(os.Chtimes(p, old, old))
	}
	must(os.Chmod(ro, 0o444))

	// What a restore with streams must bring back, read from the source.
	type want struct {
		streams        map[string]string
		eas            map[string]string
		mtime, created time.Time
	}
	expect := map[string]want{}
	filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		must(err)
		rel, _ := filepath.Rel(src, p)
		if rel == "." {
			return nil
		}
		budget := int64(maxStreamBytesTotal)
		ss, eas, rerr := readStreamsAndEAs(p, &budget)
		must(rerr)
		w := want{streams: map[string]string{}, eas: map[string]string{}, mtime: info.ModTime(), created: readCreatedForTest(t, p)}
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
	if len(expect["wsl/linux file"].eas) != 3 || len(expect["downloads/two streams.docx"].streams) != 3 ||
		expect["big stream.bin"].streams["huge"] != "(not recorded)" {
		t.Fatalf("source not set up: %+v", expect)
	}

	e2eNTFSBackupRestore(t, "streams", src, map[string]RestoreOptions{
		"streams and ACLs": {RestoreACLs: true, RestoreADS: true},
		"streams, no ACLs": {RestoreADS: true},
		"streams unticked": {RestoreACLs: true},
	}, func(label, root string) {
		skip := label == "streams unticked"
		bad, nStreams, nEAs := 0, 0, 0
		for rel, w := range expect {
			p := filepath.Join(root, filepath.FromSlash(rel))
			info, err := os.Stat(p)
			if err != nil {
				t.Errorf("%s: %s missing: %v", label, rel, err)
				bad++
				continue
			}
			budget := int64(maxStreamBytesTotal)
			ss, eas, _ := readStreamsAndEAs(p, &budget)
			got := map[string]string{}
			for _, s := range ss {
				got[s.Name] = string(s.Data)
			}
			for name, data := range w.streams {
				g, ok := got[name]
				switch {
				case skip && ok:
					t.Errorf("%s: %s:%s restored although RestoreADS is off", label, rel, name)
					bad++
				case skip:
				case data == "(not recorded)" && ok:
					t.Errorf("%s: %s:%s (too large to record) exists after restore", label, rel, name)
					bad++
				case data != "(not recorded)" && (!ok || g != data):
					t.Errorf("%s: %s:%s = %q, %v; want %q", label, rel, name, g, ok, data)
					bad++
				case ok:
					nStreams++
				}
			}
			gotEA := map[string]string{}
			for _, e := range eas {
				gotEA[e.Name] = string(e.Value)
			}
			for name, v := range w.eas {
				if skip {
					if _, ok := gotEA[name]; ok {
						t.Errorf("%s: %s EA %s restored although RestoreADS is off", label, rel, name)
						bad++
					}
				} else if gotEA[name] != v {
					t.Errorf("%s: %s EA %s = %q; want %q", label, rel, name, gotEA[name], v)
					bad++
				} else {
					nEAs++
				}
			}
			// Whole seconds: the archive stores seconds. A stream write
			// changing the time would show as the time of the restore.
			if info.ModTime().Unix() != w.mtime.Unix() {
				t.Errorf("%s: %s modification time %v; want %v", label, rel, info.ModTime().UTC(), w.mtime.UTC())
				bad++
			}
			if c := readCreatedForTest(t, p); !c.Equal(w.created) {
				t.Errorf("%s: %s creation time %v; want %v", label, rel, c.UTC(), w.created.UTC())
				bad++
			}
		}
		if label != "streams, no ACLs" {
			if info, err := os.Stat(filepath.Join(root, "readonly with stream.txt")); err == nil && info.Mode().Perm()&0o200 != 0 {
				t.Errorf("%s: read-only attribute not restored", label)
				bad++
			}
		}
		t.Logf("%s: %d paths checked, %d streams and %d EAs restored, %d problems", label, len(expect), nStreams, nEAs, bad)
	})
}
