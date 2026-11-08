//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Opt-in (PBS_BENCH_URL set): times restores of one snapshot with the
// sequential and the parallel extractor, alternating, PBS_BENCH_ROUNDS rounds
// (default 2). Each folder archive is restored on its own, then the whole
// snapshot. Prints wall time, MB/s, files/s and how many CPU cores the
// process kept busy. Each restore goes to a fresh folder under
// PBS_BENCH_DEST, removed afterwards.
//
//	PBS_BENCH_URL, _FP, _AUTHID, _SECRET, _STORE, _ID: the server and backup
//	PBS_BENCH_TIME: the newest snapshot whose UTC time starts so, e.g.
//	2026-11-08T02:00 (default the newest of all)
//	PBS_BENCH_DEST: where to restore (default a temp folder)
//	PBS_BENCH_NOACL=1: without permissions; PBS_BENCH_SEQ=1: sequential only;
//	PBS_BENCH_ONLY: only the cases whose name holds this
func TestLiveRestoreBench(t *testing.T) {
	url := os.Getenv("PBS_BENCH_URL")
	if url == "" {
		t.Skip("set PBS_BENCH_URL and friends")
	}
	base := RestoreOptions{
		BaseURL: url, CertFingerprint: os.Getenv("PBS_BENCH_FP"), AuthID: os.Getenv("PBS_BENCH_AUTHID"),
		Secret: os.Getenv("PBS_BENCH_SECRET"), Datastore: os.Getenv("PBS_BENCH_STORE"), BackupID: os.Getenv("PBS_BENCH_ID"),
		Mode: RestoreModeAlternateAbs, Overwrite: true, RestoreACLs: true,
	}
	if os.Getenv("PBS_BENCH_NOACL") == "1" {
		base.RestoreACLs = false
	}
	rounds := 2
	if n, err := strconv.Atoi(os.Getenv("PBS_BENCH_ROUNDS")); err == nil && n > 0 {
		rounds = n
	}
	only := os.Getenv("PBS_BENCH_ONLY")
	seqOnly := os.Getenv("PBS_BENCH_SEQ") == "1"
	dest := os.Getenv("PBS_BENCH_DEST")
	if dest == "" {
		dest = t.TempDir()
	}
	snaps, err := ListSnapshotsInline(base.BaseURL, base.AuthID, base.Secret, "", "", base.Datastore, "", base.CertFingerprint, base.BackupID)
	if err != nil {
		t.Fatal(err)
	}
	want := os.Getenv("PBS_BENCH_TIME")
	var snap *SnapshotInfo
	for i := range snaps {
		s := &snaps[i]
		if s.BackupID != base.BackupID || !strings.HasPrefix(s.BackupTime.UTC().Format(time.RFC3339), want) {
			continue
		}
		if snap == nil || s.BackupTime.After(snap.BackupTime) {
			snap = s
		}
	}
	if snap == nil {
		t.Fatalf("no snapshot of %s at %q", base.BackupID, want)
	}
	base.SnapshotTime = snap.BackupTime
	var archives []string
	for _, f := range snap.Files {
		if strings.HasSuffix(f, ".pxar.didx") || strings.HasSuffix(f, ".mpxar.didx") {
			archives = append(archives, f)
		}
	}
	t.Logf("snapshot %s %s (%d MB), archives %v, %d CPUs, rounds %d",
		base.BackupID, snap.BackupTime.UTC().Format(time.RFC3339), snap.Size>>20, archives, runtime.NumCPU(), rounds)

	type benchCase struct {
		name     string
		includes map[string][]string // nil: the whole snapshot
	}
	var cases []benchCase
	for _, a := range archives {
		cases = append(cases, benchCase{name: a, includes: map[string][]string{a: {""}}})
	}
	if len(archives) > 1 {
		cases = append(cases, benchCase{name: "whole snapshot"})
	}

	run := 0
	for _, c := range cases {
		if only != "" && !strings.Contains(c.name, only) {
			continue
		}
		for r := 1; r <= rounds; r++ {
			// Alternate which goes first, so a warm server cache or disk
			// favours neither.
			order := []bool{false, true}
			if r%2 == 0 {
				order = []bool{true, false}
			}
			if seqOnly {
				order = []bool{false}
			}
			for _, parallel := range order {
				run++
				opts := base
				opts.ArchiveIncludes = c.includes
				opts.ParallelExtraction = parallel
				opts.DestPath = filepath.Join(dest, fmt.Sprintf("run%02d", run))
				var files int
				var bytes int64
				opts.OnSummary = func(f, d int, b int64) { files, bytes = f, b }
				cpu0 := processCPU(t)
				start := time.Now()
				err := RestoreSnapshotInline(opts)
				wall := time.Since(start)
				cpu := processCPU(t) - cpu0
				mode := "sequential"
				if parallel {
					mode = "parallel  "
				}
				if err != nil {
					t.Errorf("%s round %d %s: %v", c.name, r, mode, err)
				} else {
					sec := wall.Seconds()
					t.Logf("BENCH %-34s round %d %s acl=%-5v %7.1fs %7.1f MB/s %7.0f files/s %6d files %8.1f MB  cores busy %.2f",
						c.name, r, mode, opts.RestoreACLs, sec, float64(bytes)/1e6/sec, float64(files)/sec, files, float64(bytes)/1e6, cpu.Seconds()/sec)
				}
				if err := os.RemoveAll(opts.DestPath); err != nil {
					t.Logf("cleanup %s: %v", opts.DestPath, err)
				}
			}
		}
	}
}

// processCPU is the user plus kernel time this process has used so far.
func processCPU(t *testing.T) time.Duration {
	t.Helper()
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(windows.CurrentProcess(), &creation, &exit, &kernel, &user); err != nil {
		t.Fatal(err)
	}
	ticks := func(f windows.Filetime) int64 { return int64(f.HighDateTime)<<32 | int64(f.LowDateTime) }
	return time.Duration((ticks(kernel) + ticks(user)) * 100)
}
