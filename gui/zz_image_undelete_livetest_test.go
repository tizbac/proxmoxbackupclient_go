package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Opt-in (PBS_IMG_URL set, plus PBS_SET_DISK, the machine set's disk number,
// and PBS_SET_FOLDER, a folder on it): Undelete and Roll back for a machine
// set on THIS computer, whose disk images are on the server. Lists the set's
// snapshots, loads the folder's file list from the newest image, runs an
// Undelete scan and a Roll back PREVIEW (nothing on disk changes), then
// restores up to 5 unchanged files to a temp folder and compares them with
// the live ones byte for byte.
func TestLiveImageUndelete(t *testing.T) {
	url := os.Getenv("PBS_IMG_URL")
	if url == "" || os.Getenv("PBS_SET_FOLDER") == "" {
		t.Skip("set PBS_IMG_URL and friends, PBS_SET_DISK and PBS_SET_FOLDER")
	}
	cfg := &Config{BaseURL: url, CertFingerprint: os.Getenv("PBS_IMG_FP"), AuthID: os.Getenv("PBS_IMG_AUTHID"),
		Secret: os.Getenv("PBS_IMG_SECRET"), Datastore: os.Getenv("PBS_IMG_STORE")}
	disk, _ := strconv.Atoi(os.Getenv("PBS_SET_DISK"))
	folder := os.Getenv("PBS_SET_FOLDER")
	job := ScheduledJob{ID: "live", Name: "live machine set", BackupType: "machine", BackupID: os.Getenv("PBS_IMG_ID"),
		MachineAsVM: os.Getenv("PBS_IMG_TYPE") == "vm", DriveLetters: []string{`\\.\PhysicalDrive` + strconv.Itoa(disk)},
		ImageFolders: []string{folder}}

	folders := setFoldersOf(job)
	if len(folders) != 1 {
		t.Fatalf("folders %+v", folders)
	}
	t.Logf("folder %s -> %s", folders[0].Dir, folders[0].Base)
	snaps, err := listSetSnapshots(cfg, job)
	if err != nil || len(snaps) == 0 {
		t.Fatalf("snapshots: %v (%d)", err, len(snaps))
	}
	t.Logf("%d snapshots, newest %s", len(snaps), snaps[0].ID)

	start := time.Now()
	idx, err := loadSnapshotIndex(cfg, job, snaps[0], nil)
	if err != nil {
		t.Fatal(err)
	}
	entries := idx[folders[0].Base]
	t.Logf("file list of %s from the image: %d entries in %v", folder, len(entries), time.Since(start).Round(time.Millisecond))
	start = time.Now()
	if _, err := loadSnapshotIndex(cfg, job, snaps[0], nil); err != nil {
		t.Fatal(err)
	}
	t.Logf("again from the local cache in %v", time.Since(start).Round(time.Millisecond))

	a := &App{}
	scan, err := a.scanUndelete(cfg, job, snaps[:1], nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Undelete: %d files in the backup but not on disk, %d unknown", len(scan.Files), scan.Unknown)
	for i, f := range scan.Files {
		if i >= 10 {
			break
		}
		t.Logf("  missing: %s (%d bytes)", f.Path, f.Size)
	}

	plan, err := a.planRollbackFor(cfg, job, snaps[0].Unix, nil)
	if err != nil {
		t.Fatal(err)
	}
	pv := plan.preview
	t.Logf("Roll back preview: %d missing, %d changed, %d new since, %d unchanged", pv.MissingCount, pv.ChangedCount, pv.NewCount, pv.Unchanged)

	// Restore a few unchanged files elsewhere and compare with the live ones.
	cr := plan.compares[folders[0].Base]
	_ = cr
	live, err := walkLiveFolder(folder, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	liveBy := map[string]IndexEntry{}
	for _, e := range live.Entries {
		liveBy[pathKey(e.Path)] = e
	}
	var pick []string
	for _, e := range entries {
		if l, ok := liveBy[pathKey(e.Path)]; ok && !e.IsDir && e.Size > 0 && e.Size < 20<<20 && l.Size == e.Size && l.MTime == e.MTime {
			pick = append(pick, e.Path)
			if len(pick) == 5 {
				break
			}
		}
	}
	if len(pick) == 0 {
		t.Skip("no unchanged files to compare")
	}
	dest := t.TempDir()
	runs := undeleteRuns(cfg, job, map[int64]map[string][]string{snaps[0].Unix: {folders[0].Base: pick}}, []int64{snaps[0].Unix}, dest, true, false)
	for _, o := range runs {
		if err := RestoreSnapshotInline(o); err != nil {
			t.Fatal(err)
		}
	}
	name := filepath.Base(folder)
	for _, p := range pick {
		got, err := os.ReadFile(filepath.Join(dest, name, filepath.FromSlash(p)))
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		want, err := os.ReadFile(filepath.Join(folder, filepath.FromSlash(p)))
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: restored copy differs from the live file", p)
		} else {
			t.Logf("  restored %s: %d bytes, identical", strings.TrimPrefix(p, "/"), len(got))
		}
	}
}
