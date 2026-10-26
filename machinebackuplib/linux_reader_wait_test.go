//go:build linux
// +build linux

package machinebackuplib

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"pbscommon"
)

// When the upload fails, streamStitchedDisk must not return until the disk
// reader has stopped: the caller destroys the snapshots next, and a reader
// still holding a snapshot device open made that fail with "resource busy".
func TestStreamStitchedDiskWaitsForReader(t *testing.T) {
	disk := filepath.Join(t.TempDir(), "disk.img")
	if err := os.WriteFile(disk, make([]byte, 2*pbscommon.PBS_FIXED_CHUNK_SIZE), 0644); err != nil {
		t.Fatal(err)
	}

	uploadErr := errors.New("http2: client connection lost")
	var readerFinished atomic.Bool
	oldRead, oldUpload := readDiskSegments, uploadDiskImage
	t.Cleanup(func() { readDiskSegments, uploadDiskImage = oldRead, oldUpload })
	// The upload fails at once, without taking any data.
	uploadDiskImage = func(client *pbscommon.PBSClient, filename string, total uint64, ch chan []byte, readErrCh <-chan error) error {
		return uploadErr
	}
	// The reader takes a moment to let go of the device after the upload ends.
	readDiskSegments = func(dev string, segments []diskSegment, total uint64, ch chan []byte, uploadDone <-chan struct{}, cb ProgressCallback) error {
		err := writeSegments(dev, segments, total, ch, uploadDone, cb)
		time.Sleep(300 * time.Millisecond)
		readerFinished.Store(true)
		return err
	}

	err := streamStitchedDisk(nil, disk, "drive-sata0.img.fidx", 2*pbscommon.PBS_FIXED_CHUNK_SIZE, nil, nil, nil)
	if !errors.Is(err, uploadErr) {
		t.Fatalf("err = %v, want the upload error", err)
	}
	if !readerFinished.Load() {
		t.Fatal("streamStitchedDisk returned while the disk reader was still running")
	}
}
