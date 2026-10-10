//go:build !service

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"pbscommon"
)

// Opt-in (PBS_PAUSE=1): Pause and Resume against the test PBS. A folder
// backup with a 400 MB file is paused part way through that file, held for
// PBS_PAUSE_SECS (default 90; use 3600 to prove an hour), checked to have
// read nothing meanwhile, resumed, and must finish with a snapshot that
// restores byte for byte. Then a second run is paused and stopped, which must
// end promptly as cancelled.
func TestBackupPauseLive(t *testing.T) {
	if os.Getenv("PBS_PAUSE") == "" {
		t.Skip("set PBS_PAUSE=1")
	}
	hold := 90 * time.Second
	if s, err := strconv.Atoi(os.Getenv("PBS_PAUSE_SECS")); err == nil && s > 0 {
		hold = time.Duration(s) * time.Second
	}
	for _, mode := range []string{pbscommon.ChangeDetectionLegacy, pbscommon.ChangeDetectionMetadata} {
		t.Run(mode, func(t *testing.T) { backupPauseLive(t, mode, hold) })
	}
	t.Run("stop-while-paused", backupPauseStopLive)
}

func pauseLiveSource(t *testing.T) string {
	src := filepath.Join(t.TempDir(), "src")
	e2eBuildTree(t, src)
	e2eWriteRandom(t, filepath.Join(src, "zz-large", "large-400mb.bin"), 400<<20)
	return src
}

// startPausableBackup runs a backup in the background and returns the bytes
// read so far and a channel with the result.
func startPausableBackup(t *testing.T, src, backupID, mode string) (*atomic.Uint64, chan *BackupStatus) {
	var done atomic.Uint64
	res := make(chan *BackupStatus, 1)
	go func() {
		var rs *BackupStatus
		err := RunBackupInline(BackupOptions{
			BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
			BackupObjects: []string{src}, BackupID: backupID, BackupType: "host", Kind: "directory",
			Compression: "fastest", ChangeDetectionMode: mode,
			OnStats:  func(s *BackupProgressStats) { done.Store(s.BytesDone) },
			OnResult: func(s *BackupStatus) { rs = s },
		})
		if err != nil && rs == nil {
			t.Logf("backup returned %v", err)
			rs = &BackupStatus{Outcome: OutcomeFailed}
		}
		res <- rs
	}()
	return &done, res
}

func waitBytes(t *testing.T, done *atomic.Uint64, min uint64, res chan *BackupStatus) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Minute)
	for done.Load() < min {
		select {
		case rs := <-res:
			t.Fatalf("backup ended before %d bytes were read: %+v", min, rs)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d bytes read after 5 minutes", done.Load())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func backupPauseLive(t *testing.T, mode string, hold time.Duration) {
	src := pauseLiveSource(t)
	want := e2eSnapshotState(t, src)
	backupID := fmt.Sprintf("pause-%s-%d", mode, time.Now().Unix())
	t.Logf("backup id %s, pause held %s", backupID, hold)
	a := &App{}

	done, res := startPausableBackup(t, src, backupID, mode)
	// The tree's own files come to about 100 MB, so 250 MB is inside the
	// 400 MB file: this pauses part way through a file.
	waitBytes(t, done, 250<<20, res)
	if st, err := a.PauseBackup(); err != nil || !st.Paused {
		t.Fatalf("PauseBackup = %+v, %v", st, err)
	}
	time.Sleep(3 * time.Second) // the block in hand and the upload queue drain
	atPause := done.Load()
	time.Sleep(hold)
	select {
	case rs := <-res:
		t.Fatalf("backup ended during the pause: %+v", rs)
	default:
	}
	if moved := done.Load() - atPause; moved > 0 {
		t.Errorf("%d bytes were read during the pause", moved)
	}
	if st, _ := a.ResumeBackup(); st.Paused {
		t.Fatal("still paused after ResumeBackup")
	}
	t.Logf("paused at %.1f MB, resumed after %s", float64(atPause)/(1<<20), hold)

	var rs *BackupStatus
	select {
	case rs = <-res:
	case <-time.After(15 * time.Minute):
		t.Fatal("backup did not finish within 15 minutes of Resume")
	}
	if rs == nil || !rs.Success() {
		t.Fatalf("backup after resume: %+v", rs)
	}
	if p, _ := backupPause.Paused(); p {
		t.Error("the gate is still closed after the run ended")
	}

	dest := filepath.Join(t.TempDir(), "restore")
	var verified int
	var fails []string
	err := RestoreSnapshotInline(RestoreOptions{
		BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
		BackupID: backupID, SnapshotTime: time.Unix(rs.BackupTime, 0).UTC(), DestPath: dest,
		Mode: RestoreModeAlternateAbs, Overwrite: true, VerifyAfterRestore: true,
		OnVerifySummary: func(v int, f []string) { verified, fails = v, f },
	})
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if verified == 0 || len(fails) > 0 {
		t.Errorf("verify after restore: %d files checked, mismatches %v", verified, fails)
	}
	e2eCompare(t, "paused backup", want, e2eSnapshotState(t, e2eFindRoot(t, dest)), "")
}

func backupPauseStopLive(t *testing.T) {
	src := pauseLiveSource(t)
	backupID := fmt.Sprintf("pause-stop-%d", time.Now().Unix())
	a := &App{}
	done, res := startPausableBackup(t, src, backupID, pbscommon.ChangeDetectionLegacy)
	waitBytes(t, done, 250<<20, res)
	if _, err := a.PauseBackup(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Second)
	stopped := time.Now()
	if err := a.CancelBackup(); err != nil {
		t.Fatal(err)
	}
	select {
	case rs := <-res:
		if rs == nil || rs.Success() {
			t.Fatalf("a run stopped while paused should not succeed: %+v", rs)
		}
		t.Logf("stopped while paused: ended in %s, outcome %s", time.Since(stopped).Round(time.Millisecond), rs.Outcome)
	case <-time.After(60 * time.Second):
		t.Fatal("Stop while paused did not end the backup within 60 s")
	}
	if p, _ := backupPause.Paused(); p {
		t.Error("the gate is still closed after the stopped run ended")
	}
}
