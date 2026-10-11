//go:build linux

package main

import (
	"bytes"
	"fmt"
	"io"
	"machinebackuplib"
	"os"
	"pbscommon"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Opt-in (PBS_LMB_DEV = a whole block device, e.g. /dev/loop0 or /dev/dm-3;
// run as root): makes a machine backup of the device to the e2e server and
// reads the image back. Optional PBS_LMB_BAD = "start-end[,start-end]" byte
// ranges the device cannot read (a device-mapper error target there): the
// backup must finish, report them as unreadable, and hold zeros there. With
// PBS_LMB_COMPARE=1 every other byte of the image must match the device (only
// valid for a device nothing has mounted). PBS_LMB_OUT writes the image to a
// file, to mount and compare.
func TestLiveLinuxMachineBackup(t *testing.T) {
	dev := os.Getenv("PBS_LMB_DEV")
	if dev == "" {
		t.Skip("set PBS_LMB_DEV")
	}
	type rng struct{ a, b uint64 }
	var bad []rng
	for _, r := range strings.Split(os.Getenv("PBS_LMB_BAD"), ",") {
		if a, b, ok := strings.Cut(r, "-"); ok {
			// Reads of a block device go through the page cache, so a bad
			// sector makes its whole 4 KiB page unreadable.
			x, _ := strconv.ParseUint(a, 10, 64)
			y, _ := strconv.ParseUint(b, 10, 64)
			bad = append(bad, rng{x &^ 4095, (y + 4095) &^ 4095})
		}
	}
	inBad := func(off uint64) bool {
		for _, r := range bad {
			if off >= r.a && off < r.b {
				return true
			}
		}
		return false
	}

	backupID := fmt.Sprintf("linux-machine-test-%d", time.Now().Unix())
	prog := &machinebackuplib.JobProgress{}
	start := time.Now()
	var msgs []string
	// PBS_LMB_CANCEL_AFTER=n: stop the backup after n progress calls, as the
	// GUI's Stop does; the backup must return the cancel and close its
	// session (check the server's task list: no backup task left running).
	cancelAfter, _ := strconv.Atoi(os.Getenv("PBS_LMB_CANCEL_AFTER"))
	if cancelAfter > 0 {
		calls := 0
		_, err := machinebackuplib.Backup(&machinebackuplib.Config{
			BaseURL: e2eBaseURL, CertFingerprint: e2eFP, AuthID: e2eAuthID, Secret: e2eSecret,
			Datastore: e2eStore, BackupID: backupID, BackupType: "host", BackupDevices: []string{dev},
			Progress: prog,
		}, func(float64, string) bool { calls++; return calls > cancelAfter })
		t.Logf("cancelled backup %s returned after %v: %v", backupID, time.Since(start).Round(time.Millisecond), err)
		if err == nil || !strings.Contains(err.Error(), "cancelled") {
			t.Fatalf("want a cancel error, got %v", err)
		}
		return
	}
	_, err := machinebackuplib.Backup(&machinebackuplib.Config{
		BaseURL: e2eBaseURL, CertFingerprint: e2eFP, AuthID: e2eAuthID, Secret: e2eSecret,
		Datastore: e2eStore, BackupID: backupID, BackupType: "host", BackupDevices: []string{dev},
		Progress: prog,
	}, func(pct float64, msg string) bool {
		if len(msgs) == 0 || msgs[len(msgs)-1] != msg {
			msgs = append(msgs, msg)
		}
		return false // true would cancel
	})
	if err != nil {
		t.Fatal(err)
	}
	w := prog.Where()
	t.Logf("backup %s in %v: read %d MB of %d MB; chunks new %d, reused %d (%d blank), uploaded %d MB; unreadable %d bytes %+v; last location %+v",
		backupID, time.Since(start).Round(time.Millisecond), prog.BytesDone.Load()>>20, prog.BytesTotal.Load()>>20,
		prog.NewChunks.Load(), prog.ReusedChunks.Load(), prog.BlankChunks.Load(), prog.UploadedBytes.Load()>>20,
		prog.UnreadableBytes.Load(), prog.Unreadable(), w)
	for i, m := range msgs {
		if i < 3 || i >= len(msgs)-3 {
			t.Logf("  progress message: %s", m)
		}
	}
	if prog.BlankChunks.Load() > prog.ReusedChunks.Load() {
		t.Errorf("blank %d > reused %d", prog.BlankChunks.Load(), prog.ReusedChunks.Load())
	}
	var wantBad uint64
	for _, r := range bad {
		wantBad += r.b - r.a
	}
	if got := prog.UnreadableBytes.Load(); got != wantBad {
		t.Errorf("unreadable %d bytes, want %d", got, wantBad)
	}

	client := &pbscommon.PBSClient{BaseURL: e2eBaseURL, CertFingerPrint: e2eFP, Insecure: true, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore}
	snaps, err := client.ListSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	var when int64
	for _, s := range snaps {
		if s.BackupID == backupID {
			when = s.BackupTime
		}
	}
	client.Manifest = pbscommon.BackupManifest{BackupType: "host", BackupID: backupID, BackupTime: when}
	client.Connect(true, "host")
	defer client.Close()
	ra, size, err := client.NewFIDXReaderAt("drive-sata0.img.fidx", 32, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("image %d bytes", size)

	var out *os.File
	if p := os.Getenv("PBS_LMB_OUT"); p != "" {
		if out, err = os.Create(p); err != nil {
			t.Fatal(err)
		}
		defer out.Close()
	}
	var src *os.File
	if os.Getenv("PBS_LMB_COMPARE") == "1" {
		if src, err = os.Open(dev); err != nil {
			t.Fatal(err)
		}
		defer src.Close()
	}
	const unit = 1 << 20
	img := make([]byte, unit)
	live := make([]byte, unit)
	var diffs, zeroBad, nonZeroBad int
	for off := int64(0); off < size; off += unit {
		n := int(min(unit, size-off))
		if _, err := ra.ReadAt(img[:n], off); err != nil && err != io.EOF {
			t.Fatalf("image read at %d: %v", off, err)
		}
		if out != nil {
			out.Write(img[:n])
		}
		for s := 0; s < n; s += 512 {
			if inBad(uint64(off) + uint64(s)) {
				if bytes.Count(img[s:s+512], []byte{0}) == 512 {
					zeroBad++
				} else {
					nonZeroBad++
				}
			}
		}
		if src == nil {
			continue
		}
		// Read the device page by page around the bad ranges.
		for s := 0; s < n; s += 4096 {
			if inBad(uint64(off) + uint64(s)) {
				continue
			}
			if _, err := src.ReadAt(live[s:s+4096], off+int64(s)); err != nil {
				t.Fatalf("device read at %d: %v", off+int64(s), err)
			}
			if !bytes.Equal(live[s:s+4096], img[s:s+4096]) {
				if diffs < 5 {
					t.Errorf("image differs from the device at %d", off+int64(s))
				}
				diffs++
			}
		}
	}
	t.Logf("bad-range sectors: %d zero in the image, %d not zero; %d differing 4 KiB pages elsewhere (compare=%v)", zeroBad, nonZeroBad, diffs, src != nil)
	if nonZeroBad > 0 {
		t.Errorf("%d unreadable sectors are not zero in the image", nonZeroBad)
	}
}
