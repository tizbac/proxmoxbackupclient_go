//go:build !service

package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pbscommon"
)

// Opt-in (PBS_CONCURRENT=1): several backups at once against the test PBS,
// through the queue, runs and backup code the app itself uses
// (runLocalBackup, the same path as a Backup Set's run). The case it proves:
// a long first backup of a large folder does not hold up a small set's
// repeated runs, nor a machine backup.
//
//   - PBS_CONC_BIG_MB (default 3000) sizes the long backup's folder;
//   - PBS_CONC_DISK (Windows: a disk number, Linux: a device such as
//     /dev/sdb) and PBS_CONC_DISK2 add machine backups beside it, the second
//     one having to wait for the first (one machine backup at a time);
//   - PBS_CONC_VSS=1 takes snapshots (VSS, elastio-snap) for every run.
//
// Every snapshot made is then restored and compared with its source.
func TestConcurrentRunsLive(t *testing.T) {
	if os.Getenv("PBS_CONCURRENT") == "" {
		t.Skip("set PBS_CONCURRENT=1")
	}
	bigMB := int64(3000)
	if n, err := strconv.ParseInt(os.Getenv("PBS_CONC_BIG_MB"), 10, 64); err == nil && n > 0 {
		bigMB = n
	}
	useVSS := os.Getenv("PBS_CONC_VSS") == "1"
	withRunLimit(t, 3)
	a := &App{config: &Config{}}
	stamp := time.Now().Unix()

	// The long first backup: a folder of 500 MB random files.
	bigSrc := filepath.Join(t.TempDir(), "big")
	for left, i := bigMB<<20, 0; left > 0; i++ {
		n := min(left, int64(500<<20))
		e2eWriteRandom(t, filepath.Join(bigSrc, fmt.Sprintf("part-%02d.bin", i)), n)
		left -= n
	}
	e2eWriteBytes(t, filepath.Join(bigSrc, "e2e-marker.txt"), []byte("marker"))
	bigWant := e2eSnapshotState(t, bigSrc)
	// The small set that runs again and again meanwhile.
	smallSrc := filepath.Join(t.TempDir(), "small")
	e2eBuildTree(t, smallSrc)

	longID := fmt.Sprintf("conc-long-%d", stamp)
	smallID := fmt.Sprintf("conc-small-%d", stamp)
	t.Logf("long %s (%d MB), small %s, VSS %v", longID, bigMB, smallID, useVSS)

	long := startLiveRun(t, a, "Long first backup", liveDirOpts(bigSrc, longID, useVSS), opBackup)
	long.waitBytes(t, 100<<20)
	t.Logf("long backup admitted and reading (%.0f MB so far)", float64(long.bytes.Load())/(1<<20))

	// 1. A small set's run goes through while the long one is still
	// reading, and both move at the same time.
	atStart := long.bytes.Load()
	s1 := startLiveRun(t, a, "Small set", liveDirOpts(smallSrc, smallID, useVSS), opBackup)
	s1.mustFinish(t, "small run 1", 10*time.Minute)
	if long.finished() {
		t.Fatalf("the long backup finished before the small run did: make PBS_CONC_BIG_MB larger")
	}
	moved := long.bytes.Load() - atStart
	if s1.queued.Load() {
		t.Errorf("small run 1 waited for its turn: %s", s1.queuedMsg.Load())
	}
	if moved == 0 {
		t.Errorf("the long backup read nothing while the small one ran")
	}
	t.Logf("small run 1: %s, while the long backup read %.0f MB more", s1.took(), float64(moved)/(1<<20))
	smallSnaps := []int64{s1.status().BackupTime}

	// Keep the long run going but paused from here, so it holds its place
	// through the rest (as a day-long first backup would).
	if !long.run.pause.Pause() {
		t.Fatal("could not pause the long backup")
	}
	time.Sleep(3 * time.Second)
	pausedAt := long.bytes.Load()

	// 2. The small set again, changed (its next scheduled run).
	e2eMutate(t, smallSrc)
	s2 := startLiveRun(t, a, "Small set", liveDirOpts(smallSrc, smallID, useVSS), opBackup)
	s2.mustFinish(t, "small run 2", 10*time.Minute)
	smallSnaps = append(smallSnaps, s2.status().BackupTime)
	smallWant := e2eSnapshotState(t, smallSrc)

	// 3. Two runs of the same set: the second waits for the first.
	s3 := startLiveRun(t, a, "Small set (Run now)", liveDirOpts(smallSrc, smallID, useVSS), opBackup)
	s3.waitAdmitted(t)
	s4 := startLiveRun(t, a, "Small set", liveDirOpts(smallSrc, smallID, useVSS), opBackup)
	s4.waitQueued(t, "the same set's second run")
	if msg := s4.queuedMsg.Load(); !strings.Contains(fmt.Sprint(msg), "Small set (Run now)") {
		t.Errorf("second run of the set waits with %q, want it to name the first", msg)
	}
	s3.mustFinish(t, "first of two runs of the set", 10*time.Minute)
	s4.mustFinish(t, "second of two runs of the set", 10*time.Minute)
	if !s4.admittedAt().After(s3.endedAt()) && !s4.admittedAt().Equal(s3.endedAt()) {
		t.Errorf("the set's second run started at %s, before its first ended at %s", s4.admittedAt(), s3.endedAt())
	}

	// 4. Stop on a run that is still waiting: it ends at once, cancelled.
	s5 := startLiveRun(t, a, "Small set (Run now)", liveDirOpts(smallSrc, smallID, useVSS), opBackup)
	s5.waitAdmitted(t)
	s6 := startLiveRun(t, a, "Small set", liveDirOpts(smallSrc, smallID, useVSS), opBackup)
	s6.waitQueued(t, "a run to stop while queued")
	stopAt := time.Now()
	s6.run.cancel()
	s6.waitDone(t, 30*time.Second)
	if s6.ok.Load() || !strings.Contains(strings.ToLower(fmt.Sprint(s6.msg.Load())), "cancel") {
		t.Errorf("a run stopped while queued ended with ok=%v %q", s6.ok.Load(), s6.msg.Load())
	}
	t.Logf("stopped while queued: ended in %s", time.Since(stopAt).Round(time.Millisecond))
	s5.mustFinish(t, "the run the stopped one waited for", 10*time.Minute)

	// 5. Machine backups beside the long one: the first runs at once, a
	// second waits for it (one at a time), and a small run still goes
	// through while it runs: three at once.
	var machineIDs []string
	if disk := os.Getenv("PBS_CONC_DISK"); disk != "" {
		m1ID := fmt.Sprintf("conc-machine-%d", stamp)
		m1 := startLiveRun(t, a, "Machine A", liveMachineOpts(disk, m1ID, useVSS), opMachine)
		m1.waitAdmitted(t)
		var m2 *liveRun
		if disk2 := os.Getenv("PBS_CONC_DISK2"); disk2 != "" {
			m2 = startLiveRun(t, a, "Machine B", liveMachineOpts(disk2, fmt.Sprintf("conc-machine2-%d", stamp), useVSS), opMachine)
			m2.waitQueued(t, "a second machine backup")
			if msg := fmt.Sprint(m2.queuedMsg.Load()); !strings.Contains(msg, "Machine A") {
				t.Errorf("second machine backup waits with %q, want it to name the first", msg)
			}
		}
		s7 := startLiveRun(t, a, "Small set", liveDirOpts(smallSrc, smallID, useVSS), opBackup)
		s7.waitAdmitted(t)
		// Only while the machine backup still runs (a tiny test disk is
		// done in under a second).
		if n := runningOperationCount(); n != 3 && !m1.finished() {
			t.Errorf("%d operations running with a long, a machine and a small backup, want 3", n)
		} else if n == 3 {
			t.Logf("three backups running at once: the long one, a machine and a small one")
		}
		s7.mustFinish(t, "small run beside a machine backup", 10*time.Minute)
		m1.mustFinish(t, "machine backup", 30*time.Minute)
		machineIDs = append(machineIDs, m1ID)
		if m2 != nil {
			m2.mustFinish(t, "second machine backup", 30*time.Minute)
		}
	}

	// 6. "Maximum Concurrent Backups" at 1: a new run waits for a free place even for
	// another set, and starts once the limit is raised again.
	maxConcurrentRuns = func() int { return 1 }
	s8 := startLiveRun(t, a, "Small set", liveDirOpts(smallSrc, smallID, useVSS), opBackup)
	s8.waitQueued(t, "a run with Maximum Concurrent Backups at 1")
	maxConcurrentRuns = func() int { return 3 }
	opMu.Lock()
	admitWaitingLocked()
	opMu.Unlock()
	s8.mustFinish(t, "the run let through when the limit went back up", 10*time.Minute)

	// The long backup read nothing while paused, and finishes once resumed.
	if moved := long.bytes.Load() - pausedAt; moved > 64<<20 {
		t.Errorf("the paused long backup read %d MB while paused", moved>>20)
	}
	if _, ok := long.run.pause.Resume(); !ok {
		t.Error("the long backup was not paused")
	}
	long.mustFinish(t, "the long backup", 2*time.Hour)
	t.Logf("long backup: %s, %s", long.took(), long.status().Outcome)

	// Each run's own log file has its own lines only.
	checkRunLogs(t, longID, smallID, smallSrc, bigSrc)

	// Everything restores and matches its source.
	concRestoreAndCompare(t, "long first backup", longID, long.status().BackupTime, bigWant)
	concRestoreAndCompare(t, "small set, latest", smallID, smallSnaps[1], smallWant)
	if runtime.GOOS == "windows" && len(machineIDs) > 0 {
		t.Logf("machine snapshots made: %v (images are checked by TestE2EMachineWindows)", machineIDs)
	}
	checkIdle(t)
	if localBackupRunning() {
		t.Error("runs still listed after all ended")
	}
}

// liveRun is one backup started the way a Backup Set's run is.
type liveRun struct {
	name      string
	run       *backupRun
	done      chan struct{}
	bytes     atomic.Uint64
	queued    atomic.Bool
	queuedMsg atomic.Value
	ok        atomic.Bool
	msg       atomic.Value
	result    atomic.Pointer[BackupStatus]
	mu        sync.Mutex
	started   time.Time
	admitted  time.Time
	ended     time.Time
}

func startLiveRun(t *testing.T, a *App, name string, opts BackupOptions, kind opKind) *liveRun {
	t.Helper()
	lr := &liveRun{name: name, done: make(chan struct{}), started: time.Now()}
	opts.Comment = name
	opts.OnProgress = func(p float64, m string) {
		if strings.HasPrefix(m, "Queued: ") {
			lr.queued.Store(true)
			lr.queuedMsg.Store(m)
			return
		}
		lr.markAdmitted()
	}
	opts.OnStats = func(s *BackupProgressStats) {
		lr.markAdmitted()
		lr.bytes.Store(s.BytesDone)
	}
	opts.OnResult = func(s *BackupStatus) { lr.result.Store(s) }
	opts.OnComplete = func(ok bool, m string, _ MessageKey, _ msgParams) {
		lr.ok.Store(ok)
		lr.msg.Store(m)
		lr.mu.Lock()
		lr.ended = time.Now()
		lr.mu.Unlock()
	}
	kindName := "directory"
	if kind == opMachine {
		kindName = "machine"
	}
	lr.run = newBackupRun(name, kindName, opts.BackupID)
	go a.runLocalBackup(lr.run, opts, kind, lr.done)
	return lr
}

func (lr *liveRun) markAdmitted() {
	lr.mu.Lock()
	if lr.admitted.IsZero() {
		lr.admitted = time.Now()
	}
	lr.mu.Unlock()
}

func (lr *liveRun) admittedAt() time.Time { lr.mu.Lock(); defer lr.mu.Unlock(); return lr.admitted }
func (lr *liveRun) endedAt() time.Time    { lr.mu.Lock(); defer lr.mu.Unlock(); return lr.ended }
func (lr *liveRun) took() time.Duration {
	return lr.endedAt().Sub(lr.started).Round(time.Second)
}
func (lr *liveRun) status() *BackupStatus {
	if s := lr.result.Load(); s != nil {
		return s
	}
	return &BackupStatus{}
}
func (lr *liveRun) finished() bool {
	select {
	case <-lr.done:
		return true
	default:
		return false
	}
}

func (lr *liveRun) waitBytes(t *testing.T, n uint64) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Minute)
	for lr.bytes.Load() < n {
		if lr.finished() {
			t.Fatalf("%s ended before reading %d MB: %v", lr.name, n>>20, lr.msg.Load())
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s read only %d bytes in 10 minutes", lr.name, lr.bytes.Load())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// waitAdmitted waits until the run's turn has come (it reports progress).
func (lr *liveRun) waitAdmitted(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Minute)
	for lr.admittedAt().IsZero() {
		if lr.finished() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s did not start within 5 minutes (queued: %v %v)", lr.name, lr.queued.Load(), lr.queuedMsg.Load())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (lr *liveRun) waitQueued(t *testing.T, what string) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for !lr.queued.Load() {
		if !lr.admittedAt().IsZero() || lr.finished() {
			t.Fatalf("%s started at once, but should have waited for its turn", what)
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s neither started nor queued within a minute", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !lr.run.queued.Load() {
		t.Errorf("%s: its run is not marked as queued (the window lists it from that)", what)
	}
}

func (lr *liveRun) waitDone(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case <-lr.done:
	case <-time.After(d):
		t.Fatalf("%s did not end within %s", lr.name, d)
	}
}

func (lr *liveRun) mustFinish(t *testing.T, what string, d time.Duration) {
	t.Helper()
	lr.waitDone(t, d)
	if !lr.ok.Load() {
		t.Fatalf("%s failed: %v", what, lr.msg.Load())
	}
	t.Logf("%s: done in %s (%s)", what, lr.took(), lr.status().Outcome)
}

func liveDirOpts(src, backupID string, vss bool) BackupOptions {
	return BackupOptions{
		BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
		BackupObjects: []string{src}, BackupID: backupID, BackupType: "host", Kind: "directory",
		Compression: "fastest", ChangeDetectionMode: pbscommon.ChangeDetectionMetadata, UseVSS: vss,
	}
}

func liveMachineOpts(disk, backupID string, vss bool) BackupOptions {
	dev := disk
	if runtime.GOOS == "windows" && !strings.HasPrefix(disk, `\\`) {
		dev = `\\.\PhysicalDrive` + disk
	}
	return BackupOptions{
		BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
		BackupObjects: []string{dev}, BackupID: backupID, BackupType: "host", Kind: "machine",
		Compression: "fastest", UseVSS: vss,
	}
}

// checkRunLogs reads the per-run log files of this test's runs: each must
// name its own backup-id and folder, and never the other set's.
func checkRunLogs(t *testing.T, longID, smallID, smallSrc, bigSrc string) {
	t.Helper()
	read := func(id string) []string {
		files, _ := filepath.Glob(filepath.Join(logDir, "backup-*-"+sanitizeForFilename(id)+".log*"))
		var texts []string
		for _, f := range files {
			fh, err := os.Open(f)
			if err != nil {
				t.Errorf("open %s: %v", f, err)
				continue
			}
			var sc *bufio.Scanner
			if strings.HasSuffix(f, ".gz") {
				gz, err := gzip.NewReader(fh)
				if err != nil {
					fh.Close()
					t.Errorf("gunzip %s: %v", f, err)
					continue
				}
				sc = bufio.NewScanner(gz)
			} else {
				sc = bufio.NewScanner(fh)
			}
			sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
			var b strings.Builder
			for sc.Scan() {
				b.WriteString(sc.Text())
				b.WriteByte('\n')
			}
			fh.Close()
			texts = append(texts, b.String())
		}
		return texts
	}
	longLogs, smallLogs := read(longID), read(smallID)
	if len(longLogs) != 1 {
		t.Errorf("%d log files for the long backup, want 1", len(longLogs))
	}
	if len(smallLogs) < 5 {
		t.Errorf("%d log files for the small set's runs, want one per run", len(smallLogs))
	}
	for _, s := range longLogs {
		if !strings.Contains(s, longID) || strings.Contains(s, smallID) || strings.Contains(s, smallSrc) {
			t.Errorf("the long backup's log has another run's lines (or lacks its own)")
		}
		if !strings.Contains(s, "[RESULT]") {
			t.Errorf("the long backup's log has no [RESULT] line")
		}
	}
	for i, s := range smallLogs {
		if !strings.Contains(s, smallID) || strings.Contains(s, longID) || strings.Contains(s, bigSrc) {
			t.Errorf("small run log %d has another run's lines (or lacks its own)", i+1)
		}
	}
	t.Logf("per-run logs: 1 for the long backup, %d for the small set, none mixed", len(smallLogs))
}

func concRestoreAndCompare(t *testing.T, label, backupID string, backupTime int64, want map[string]e2eFile) {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "restore")
	var verified int
	var fails []string
	err := RestoreSnapshotInline(RestoreOptions{
		BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
		BackupID: backupID, SnapshotTime: time.Unix(backupTime, 0).UTC(), DestPath: dest,
		Mode: RestoreModeAlternateAbs, Overwrite: true, VerifyAfterRestore: true,
		OnVerifySummary: func(v int, f []string) { verified, fails = v, f },
	})
	if err != nil {
		t.Fatalf("restore %s: %v", label, err)
	}
	if verified == 0 || len(fails) > 0 {
		t.Errorf("%s: verify after restore checked %d files, mismatches %v", label, verified, fails)
	}
	e2eCompare(t, label, want, e2eSnapshotState(t, e2eFindRoot(t, dest)), "")
	t.Logf("%s: restored and matches (%d files verified)", label, verified)
}
