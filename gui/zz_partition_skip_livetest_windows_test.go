//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"machinebackuplib"
	"pbscommon"
)

// Opt-in (PBS_SKIP_SRC_DISK=N, elevated, test PBS only; never the system
// disk): backs up \\.\PhysicalDriveN leaving out its largest lettered
// partition, and checks the image on PBS: that partition is zeros, and the
// partition table is the disk's own. Then backs it up again with a skip saved
// before the table changed (one sector off): the partition is backed up after
// all, with a warning. Nothing is written to any disk.
func TestLivePartitionSkip(t *testing.T) {
	n := os.Getenv("PBS_SKIP_SRC_DISK")
	if n == "" {
		t.Skip("set PBS_SKIP_SRC_DISK (not the system disk)")
	}
	liveServer(t)
	dev := `\\.\PhysicalDrive` + n
	parts, err := machinebackuplib.ListPartitions(dev)
	if err != nil {
		t.Fatalf("ListPartitions: %v", err)
	}
	var target *machinebackuplib.PartitionInfo
	for i, p := range parts {
		t.Logf("partition %d at %d, %d bytes: mount %q fs %q type %q role %q", p.Number, p.Start, p.Size, p.Mount, p.FileSystem, p.Type, p.Role)
		if p.Role == "system" {
			t.Fatalf("%s holds the running system: use a spare disk", dev)
		}
		if p.Mount != "" && (target == nil || p.Size > target.Size) {
			target = &parts[i]
		}
	}
	if target == nil {
		t.Fatalf("%s has no lettered partition to leave out", dev)
	}
	t.Logf("leaving out partition %d (%s)", target.Number, target.Mount)

	backup := func(skip machinebackuplib.PartitionSkip) (string, []string) {
		id := fmt.Sprintf("e2e-partskip-%d", time.Now().UnixNano())
		var msgs []string
		_, err := machinebackuplib.Backup(&machinebackuplib.Config{
			BaseURL: e2eBaseURL, CertFingerprint: e2eFP, AuthID: e2eAuthID, Secret: e2eSecret,
			Datastore: e2eStore, BackupID: id, BackupType: "host", BackupDevices: []string{dev},
			SkipPartitions: map[string][]machinebackuplib.PartitionSkip{dev: {skip}},
		}, func(p float64, m string) bool {
			if !strings.Contains(m, ": Block ") {
				msgs = append(msgs, m)
			}
			return false
		})
		if err != nil {
			t.Fatalf("machine backup: %v", err)
		}
		return id, msgs
	}
	// image returns the bytes of the backed-up disk from off, n long.
	image := func(id string) func(off, n int64) []byte {
		rc := &pbscommon.PBSClient{
			BaseURL: e2eBaseURL, CertFingerPrint: e2eFP, AuthID: e2eAuthID, Secret: e2eSecret,
			Datastore: e2eStore, Insecure: true, Manifest: pbscommon.BackupManifest{BackupID: id},
		}
		snaps, err := rc.ListSnapshots()
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range snaps {
			if m.BackupID == id && m.BackupType == "host" {
				rc.Manifest.BackupTime = m.BackupTime
			}
		}
		rc.Manifest.BackupType = "host"
		rc.Connect(true, "host")
		t.Cleanup(rc.Close)
		data, err := rc.DownloadToBytes(fmt.Sprintf("drive-sata%s.img.fidx", n))
		if err != nil {
			t.Fatal(err)
		}
		rdr := bytes.NewReader(data)
		var hdr pbscommon.FIDXHeader
		if err := binary.Read(rdr, binary.LittleEndian, &hdr); err != nil {
			t.Fatal(err)
		}
		digests := make([]string, (hdr.Size+hdr.ChunkSize-1)/hdr.ChunkSize)
		for i := range digests {
			h := make([]byte, 32)
			if _, err := io.ReadFull(rdr, h); err != nil {
				t.Fatal(err)
			}
			digests[i] = hex.EncodeToString(h)
		}
		cs := int64(hdr.ChunkSize)
		return func(off, n int64) []byte {
			out := make([]byte, 0, n)
			for pos := off; pos < off+n; {
				i := pos / cs
				chunk, err := rc.GetChunkData(context.Background(), digests[i])
				if err != nil {
					t.Fatalf("chunk %d: %v", i, err)
				}
				from := pos - i*cs
				to := min(int64(len(chunk)), off+n-i*cs)
				out = append(out, chunk[from:to]...)
				pos = i*cs + to
			}
			return out
		}
	}

	src, err := os.Open(dev)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	// The partition table and the space before the first partition.
	head := make([]byte, min(uint64(64*1024), parts[0].Start))
	if _, err := src.ReadAt(head, 0); err != nil {
		t.Fatal(err)
	}

	// Left out: zeros where the partition is, the table as on the disk.
	id, msgs := backup(machinebackuplib.PartitionSkip{Start: target.Start, Size: target.Size, Number: target.Number, Label: "the test partition"})
	joined := strings.Join(msgs, "\n")
	t.Logf("run messages:\n%s", joined)
	if !strings.Contains(joined, "leaving out") || strings.Contains(joined, "WARNING") {
		t.Errorf("run messages do not say the partition is left out, or warn")
	}
	img := image(id)
	if got := img(0, int64(len(head))); !bytes.Equal(got, head) {
		t.Error("the image's partition table differs from the disk's")
	}
	part := img(int64(target.Start), int64(target.Size))
	nonZero := -1
	for i, b := range part {
		if b != 0 {
			nonZero = i
			break
		}
	}
	if nonZero >= 0 || uint64(len(part)) != target.Size {
		t.Errorf("the partition left out is not zeros in the image (byte %d of %d)", nonZero, len(part))
	} else {
		t.Logf("partition %d is %d zero bytes in the image", target.Number, len(part))
	}

	// A skip from before the table changed matches nothing: backed up, with
	// a warning. The partition's boot sector is in the image.
	id, msgs = backup(machinebackuplib.PartitionSkip{Start: target.Start + 512, Size: target.Size, Number: target.Number, Label: "the moved partition"})
	joined = strings.Join(msgs, "\n")
	if !strings.Contains(joined, "WARNING") || !strings.Contains(joined, "the moved partition") || strings.Contains(joined, "leaving out") {
		t.Errorf("unmatched skip: messages\n%s", joined)
	}
	boot := image(id)(int64(target.Start), 512)
	if bytes.Count(boot, []byte{0}) == len(boot) {
		t.Error("with an unmatched skip the partition's boot sector is zeros: it was left out")
	}
}
