//go:build !service

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"pbscommon"
)

// Opt-in (PBS_UNDELETE=1): Undelete and Roll back against the test PBS, for a
// Legacy and a Metadata set. Two backups with deletions and edits between
// them, then: the undelete scan of the newest snapshot and of both, an
// undelete to the original place (a file that has come back is left alone)
// and to another folder, and each roll back policy to the first snapshot,
// each followed by Undo, checking file contents against the states taken at
// backup time.
func TestUndeleteRollbackLive(t *testing.T) {
	if os.Getenv("PBS_UNDELETE") == "" {
		t.Skip("set PBS_UNDELETE=1")
	}
	for _, mode := range []string{pbscommon.ChangeDetectionLegacy, pbscommon.ChangeDetectionMetadata} {
		t.Run(mode, func(t *testing.T) { undeleteRollbackLive(t, mode) })
	}
}

func undeleteRollbackLive(t *testing.T, mode string) {
	base := t.TempDir()
	src := filepath.Join(base, "src")
	e2eBuildTree(t, src)
	e2eWriteBytes(t, filepath.Join(src, "projects", "alpha", "a1.txt"), []byte("alpha one"))
	e2eWriteBytes(t, filepath.Join(src, "projects", "alpha", "sub", "a2.txt"), []byte("alpha two"))
	e2eWriteBytes(t, filepath.Join(src, "docs", "report.docx"), []byte("report v1"))
	e2eWriteBytes(t, filepath.Join(src, "docs", "notes.txt"), []byte("notes v1"))

	backupID := fmt.Sprintf("undel-%s-%d", mode, time.Now().Unix())
	job := ScheduledJob{ID: "live-" + mode, Name: "Live " + mode, BackupDirs: []string{src}, BackupID: backupID, ExcludeList: []string{"*.tmp"}}
	cfg := &Config{BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP}
	t.Logf("backup id %s", backupID)

	backup := func(label string) int64 {
		t.Helper()
		var rs *BackupStatus
		err := RunBackupInline(BackupOptions{
			BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
			BackupObjects: []string{src}, BackupID: backupID, BackupType: "host", Kind: "directory",
			Compression: "fastest", ChangeDetectionMode: mode, ExcludeList: job.ExcludeList,
			OnResult: func(s *BackupStatus) { rs = s },
		})
		if err != nil || rs == nil || !rs.Success() {
			t.Fatalf("%s: backup failed: %v %+v", label, err, rs)
		}
		time.Sleep(1100 * time.Millisecond)
		return rs.BackupTime
	}

	s1 := backup("backup 1")
	state1 := e2eSnapshotState(t, src)
	must(t, os.Remove(filepath.Join(src, "docs", "notes.txt")))
	s2 := backup("backup 2")
	state2 := e2eSnapshotState(t, src)

	// After the second backup: a folder deleted, a file edited, files and a
	// folder added, a file moved, an excluded file added.
	must(t, os.RemoveAll(filepath.Join(src, "projects")))
	time.Sleep(1100 * time.Millisecond)
	e2eWriteBytes(t, filepath.Join(src, "docs", "report.docx"), []byte("report v2, longer"))
	e2eWriteBytes(t, filepath.Join(src, "new.txt"), []byte("added after"))
	e2eWriteBytes(t, filepath.Join(src, "newdir", "n.txt"), []byte("added folder"))
	must(t, os.MkdirAll(filepath.Join(src, "moved"), 0o755))
	must(t, os.Rename(filepath.Join(src, "unicode", "русский файл.dat"), filepath.Join(src, "moved", "русский файл.dat")))
	e2eWriteBytes(t, filepath.Join(src, "junk.tmp"), []byte("excluded"))

	a := &App{}
	snaps, err := listSetSnapshots(cfg, job)
	if err != nil || len(snaps) != 2 || snaps[0].Unix != s2 || snaps[1].Unix != s1 {
		t.Fatalf("set snapshots %+v (err %v), want %d then %d", snaps, err, s2, s1)
	}

	// --- Undelete scans
	latest, err := a.scanUndelete(cfg, job, snaps[:1], nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]UndeleteFile{}
	for _, f := range latest.Files {
		got[f.Path] = f
	}
	wantLatest := []string{"projects/alpha/a1.txt", "projects/alpha/sub/a2.txt", "unicode/русский файл.dat"}
	if len(got) != len(wantLatest) {
		t.Errorf("latest scan found %v, want %v", undeleteKeys(got), wantLatest)
	}
	for _, p := range wantLatest {
		f, ok := got[p]
		if !ok || f.SnapshotUnix != s2 || f.GoneByUnix != 0 {
			t.Errorf("latest scan: %s = %+v", p, f)
		}
	}
	if got["unicode/русский файл.dat"].MovedTo != "moved/русский файл.dat" {
		t.Errorf("move hint %q", got["unicode/русский файл.dat"].MovedTo)
	}
	both, err := a.scanUndelete(cfg, job, snaps, nil)
	if err != nil {
		t.Fatal(err)
	}
	var notes *UndeleteFile
	for i := range both.Files {
		if both.Files[i].Path == "docs/notes.txt" {
			notes = &both.Files[i]
		}
	}
	if len(both.Files) != 4 || notes == nil || notes.SnapshotUnix != s1 || notes.GoneByUnix != s2 {
		t.Errorf("two-snapshot scan: %d files, notes %+v", len(both.Files), notes)
	}

	// --- Undelete to the original place; a1.txt has come back meanwhile.
	e2eWriteBytes(t, filepath.Join(src, "projects", "alpha", "a1.txt"), []byte("came back"))
	bySnap := map[int64]map[string][]string{}
	for _, f := range both.Files {
		if f.MovedTo != "" {
			continue
		}
		if bySnap[f.SnapshotUnix] == nil {
			bySnap[f.SnapshotUnix] = map[string][]string{}
		}
		bySnap[f.SnapshotUnix][f.Archive] = append(bySnap[f.SnapshotUnix][f.Archive], f.Path)
	}
	runRestores(t, "undelete original", undeleteRuns(cfg, job, bySnap, []int64{s2, s1}, "", true, false))
	now := e2eSnapshotState(t, src)
	checkSame(t, "undelete notes.txt", state1, now, "docs/notes.txt")
	checkSame(t, "undelete a2.txt", state2, now, "projects/alpha/sub/a2.txt")
	if b, _ := os.ReadFile(filepath.Join(src, "projects", "alpha", "a1.txt")); string(b) != "came back" {
		t.Errorf("a1.txt was replaced: %q", b)
	}
	if _, err := os.Stat(filepath.Join(src, "unicode", "русский файл.dat")); err == nil {
		t.Errorf("moved file was restored although it was not selected")
	}

	// --- Undelete to another folder
	out := filepath.Join(base, "undelete-out")
	must(t, os.MkdirAll(out, 0o755))
	runRestores(t, "undelete elsewhere", undeleteRuns(cfg, job, bySnap, []int64{s2, s1}, out, false, false))
	found := 0
	filepath.Walk(out, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			found++
		}
		return nil
	})
	if found != 3 {
		t.Errorf("undelete to another folder wrote %d files, want 3", found)
	}

	// --- Roll back to the first snapshot, each policy, each undone
	oldRoot := rollbackSafetyRoot
	rollbackSafetyRoot = func(string) string { return filepath.Join(base, pbscommon.RollbackSafetyFolder) }
	defer func() { rollbackSafetyRoot = oldRoot }()
	for _, policy := range []string{rollbackExact, rollbackReplace, rollbackMissing} {
		label := "roll back " + policy
		before := e2eSnapshotState(t, src)
		plan, err := a.planRollbackFor(cfg, job, s1, nil)
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		pv := plan.preview
		t.Logf("%s preview: unchanged %d, missing %d, changed %d, new %d (%d folders), unknown %d",
			label, pv.Unchanged, pv.MissingCount, pv.ChangedCount, pv.NewCount, pv.NewDirCount, pv.Unknown)
		m, mp, runs, err := a.applyRollbackMoves(context.Background(), plan, policy, true, func(float64, string) {})
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		var ex []pbscommon.PXARExtractedFile
		for i := range runs {
			runs[i].OnExtracted = func(e []pbscommon.PXARExtractedFile) { ex = append(ex, e...) }
		}
		runRestores(t, label, runs)
		for _, f := range ex {
			if !f.Skipped && !f.IsDir && f.Path != "" {
				m.Restored = append(m.Restored, f.Path)
			}
		}
		m.Status = "done"
		must(t, writeRollbackManifest(mp, m))
		if len(m.MoveFailed) > 0 {
			t.Errorf("%s: moves failed: %v", label, m.MoveFailed)
		}
		after := e2eSnapshotState(t, src)
		switch policy {
		case rollbackExact:
			delete(after, "junk.tmp") // excluded from the set: never touched
			e2eCompare(t, label, state1, after, "")
		case rollbackReplace:
			for _, p := range []string{"docs/report.docx", "docs/notes.txt", "projects/alpha/a1.txt", "unicode/русский файл.dat"} {
				checkSame(t, label, state1, after, p)
			}
			for _, p := range []string{"new.txt", "newdir/n.txt", "moved/русский файл.dat", "junk.tmp"} {
				checkSame(t, label+" (kept)", before, after, p)
			}
		case rollbackMissing:
			checkSame(t, label, state1, after, "unicode/русский файл.dat")
			checkSame(t, label+" (kept)", before, after, "docs/report.docx")
			checkSame(t, label+" (kept)", before, after, "new.txt")
		}
		msg, err := a.UndoRollback(mp)
		if err != nil {
			t.Fatalf("%s undo: %v", label, err)
		}
		t.Logf("%s: %s", label, msg)
		e2eCompare(t, label+" undone", before, e2eSnapshotState(t, src), "")
	}
	if items, _ := os.ReadDir(filepath.Join(base, pbscommon.RollbackSafetyFolder)); len(items) > 0 {
		left := filepath.Join(base, pbscommon.RollbackSafetyFolder)
		filepath.Walk(left, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				t.Errorf("left in the safety folder after undo: %s", p)
			}
			return nil
		})
	}
}

func runRestores(t *testing.T, label string, runs []RestoreOptions) {
	t.Helper()
	for i, o := range runs {
		var fails []string
		o.OnVerifySummary = func(v int, f []string) { fails = f }
		if err := RestoreSnapshotInline(o); err != nil {
			t.Fatalf("%s run %d: %v", label, i, err)
		}
		if len(fails) > 0 {
			t.Errorf("%s run %d: verify failures %v", label, i, fails)
		}
	}
}

func checkSame(t *testing.T, label string, want, got map[string]e2eFile, p string) {
	t.Helper()
	w, ok1 := want[p]
	g, ok2 := got[p]
	if !ok1 || !ok2 || w.sum != g.sum || w.size != g.size {
		t.Errorf("%s: %s differs (want present=%v %d bytes, got present=%v %d bytes)", label, p, ok1, w.size, ok2, g.size)
	}
}

func undeleteKeys(m map[string]UndeleteFile) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil && !strings.Contains(err.Error(), "does not exist") {
		t.Fatal(err)
	}
}

// Opt-in (PBS_UNDELETE=1): Restore to the original location of a snapshot
// with two folders puts each folder's files back under that folder (it used
// to put both under the first folder's original path).
func TestRestoreOriginalMultiFolderLive(t *testing.T) {
	if os.Getenv("PBS_UNDELETE") == "" {
		t.Skip("set PBS_UNDELETE=1")
	}
	base := t.TempDir()
	d1 := filepath.Join(base, "alpha")
	d2 := filepath.Join(base, "beta")
	e2eWriteBytes(t, filepath.Join(d1, "one.txt"), []byte("in alpha"))
	e2eWriteBytes(t, filepath.Join(d1, "sub", "deep.txt"), []byte("deep in alpha"))
	e2eWriteBytes(t, filepath.Join(d2, "two.txt"), []byte("in beta"))
	backupID := fmt.Sprintf("multi-orig-%d", time.Now().Unix())
	var rs *BackupStatus
	if err := RunBackupInline(BackupOptions{
		BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
		BackupObjects: []string{d1, d2}, BackupID: backupID, BackupType: "host", Kind: "directory", Compression: "fastest",
		OnResult: func(s *BackupStatus) { rs = s },
	}); err != nil || rs == nil || !rs.Success() {
		t.Fatalf("backup: %v %+v", err, rs)
	}
	must(t, os.Remove(filepath.Join(d1, "sub", "deep.txt")))
	must(t, os.Remove(filepath.Join(d2, "two.txt")))
	o := RestoreOptions{BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
		BackupID: backupID, SnapshotTime: time.Unix(rs.BackupTime, 0), Mode: RestoreModeOriginal,
		IncludePaths: []string{"alpha/sub/deep.txt", "beta/two.txt"}}
	if err := RestoreSnapshotInline(o); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{filepath.Join(d1, "sub", "deep.txt"): "deep in alpha", filepath.Join(d2, "two.txt"): "in beta"} {
		if b, err := os.ReadFile(p); err != nil || string(b) != want {
			t.Errorf("%s: %q %v", p, b, err)
		}
	}
	if _, err := os.Stat(filepath.Join(d1, "two.txt")); err == nil {
		t.Errorf("beta's file landed in alpha")
	}
}
