package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestZZProgressAggregationManual is a manual, opt-in live-integration test
// proving jobProgress correctly sums sizes across multiple directories in one
// backup, instead of each directory's own live BytesTotal only reflecting
// that one directory's own size (the bug found live 2026-09-26: Mick backed
// up one local + one network folder, the displayed total stayed at ~1.1GB —
// one directory's own size — even though the combined output was 1.2GB).
//
// Creates two directories of known, clearly DIFFERENT sizes (3MB and 15MB —
// dirB alone must clear the pre-existing, unrelated "report every 10MB" gate
// in ChunkState's own progress reporting, or no OnStats callback would ever
// fire at all to check) and backs both up in one job, asserting BytesTotal
// matches the COMBINED size (~18MB), never just one directory's own size.
//
//	set PBS_LIVE_TEST=1
//	go test -run TestZZProgressAggregationManual -v ./...
func TestZZProgressAggregationManual(t *testing.T) {
	if os.Getenv("PBS_LIVE_TEST") == "" {
		t.Skip("set PBS_LIVE_TEST=1 to run this opt-in live integration test")
	}

	dirA := t.TempDir()
	dirB := t.TempDir()
	writeFileOfSize(t, filepath.Join(dirA, "a.bin"), 3*1024*1024)
	writeFileOfSize(t, filepath.Join(dirB, "b.bin"), 15*1024*1024)

	pbsOpts := struct {
		BaseURL, AuthID, Secret, Datastore, CertFingerprint string
	}{
		BaseURL:         "https://192.168.0.242:8007",
		AuthID:          "testclient@pbs!testclient",
		Secret:          "64376a7f-1aeb-42cb-a604-b75bf6a5561b",
		Datastore:       "teststore",
		CertFingerprint: "07:49:87:AE:76:0F:A8:E7:97:D8:1C:F0:44:1A:65:5D:2C:F3:10:49:00:EA:2C:53:E4:0B:CC:09:95:29:C5:22",
	}

	var mu sync.Mutex
	var maxBytesTotalSeen uint64
	var sawAny bool

	backupID := "progress-aggregation-livetest"
	var resultStatus *BackupStatus
	bOpts := BackupOptions{
		Ctx:             context.Background(),
		BaseURL:         pbsOpts.BaseURL,
		AuthID:          pbsOpts.AuthID,
		Secret:          pbsOpts.Secret,
		Datastore:       pbsOpts.Datastore,
		CertFingerprint: pbsOpts.CertFingerprint,
		BackupObjects:   []string{dirA, dirB},
		BackupID:        backupID,
		BackupType:      "host",
		Kind:            "directory",
		Compression:     "fastest",
		OnResult:        func(s *BackupStatus) { resultStatus = s },
		OnStats: func(stats *BackupProgressStats) {
			if stats.BytesTotal == 0 {
				return // background size scan for this directory hasn't finished yet
			}
			mu.Lock()
			defer mu.Unlock()
			sawAny = true
			if stats.BytesTotal > maxBytesTotalSeen {
				maxBytesTotalSeen = stats.BytesTotal
			}
			t.Logf("OnStats: dir=%s percent=%.2f bytesDone=%d bytesTotal=%d", stats.CurrentDir, stats.Percent, stats.BytesDone, stats.BytesTotal)
		},
	}
	if err := RunBackupInline(bOpts); err != nil {
		t.Fatalf("RunBackupInline: %v", err)
	}
	if resultStatus == nil || !resultStatus.Success() {
		t.Fatalf("backup did not report success: %+v", resultStatus)
	}

	if !sawAny {
		t.Fatal("never saw a single OnStats callback with a nonzero BytesTotal — can't verify aggregation at all")
	}

	const combinedExact = 18 * 1024 * 1024
	const oneDirApprox = 15 * 1024 * 1024 // the larger of the two individual directories

	t.Logf("max BytesTotal seen: %d, combined expected: %d", maxBytesTotalSeen, combinedExact)

	if maxBytesTotalSeen <= oneDirApprox {
		t.Errorf("FAIL: max BytesTotal seen (%d) never exceeded a single directory's own size (~%d) — totals are NOT being aggregated across directories", maxBytesTotalSeen, oneDirApprox)
	} else if maxBytesTotalSeen != combinedExact {
		t.Errorf("BytesTotal (%d) grew past one directory's size but doesn't exactly match the combined total (%d)", maxBytesTotalSeen, combinedExact)
	} else {
		t.Log("PASS: BytesTotal exactly matches the combined size of both directories")
	}
}

func writeFileOfSize(t *testing.T, path string, size int) {
	t.Helper()
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 251)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}
