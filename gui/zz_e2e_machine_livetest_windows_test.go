package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"machinebackuplib"
	"os"
	"pbscommon"
	"testing"
	"time"
)

// Opt-in Windows machine-mode test (PBS_E2E_SRC_DISK / PBS_E2E_DST_DISK set):
// backs up \\.\PhysicalDriveSRC as a machine backup, downloads the fidx back
// from PBS, writes the image onto the (offline) disk DST and verifies every
// 4 MiB chunk read back from DST hashes to the digest PBS holds. The wrapping
// PowerShell then brings DST online and compares the file contents.
func TestE2EMachineWindows(t *testing.T) {
	srcN, dstN := os.Getenv("PBS_E2E_SRC_DISK"), os.Getenv("PBS_E2E_DST_DISK")
	if srcN == "" || dstN == "" {
		t.Skip("set PBS_E2E_SRC_DISK and PBS_E2E_DST_DISK")
	}
	// PBS_E2E_TYPE=vm also exercises the generated qemu-server.conf (needs a numeric ID).
	btype := "host" // what the GUI uses for machine backups
	backupID := fmt.Sprintf("e2e-machine-%d", time.Now().Unix())
	if os.Getenv("PBS_E2E_TYPE") == "vm" {
		btype = "vm"
		backupID = "9" + fmt.Sprint(time.Now().Unix()%100000)
	}
	srcDev := `\\.\PhysicalDrive` + srcN
	dstDev := `\\.\PhysicalDrive` + dstN

	start := time.Now()
	_, err := machinebackuplib.Backup(&machinebackuplib.Config{
		BaseURL: e2eBaseURL, CertFingerprint: e2eFP, AuthID: e2eAuthID, Secret: e2eSecret,
		Datastore: e2eStore, BackupID: backupID, BackupType: btype, BackupDevices: []string{srcDev},
	}, func(p float64, m string) bool { return false })
	if err != nil {
		t.Fatalf("machine backup: %v", err)
	}
	t.Logf("machine backup OK in %v", time.Since(start).Round(time.Millisecond))

	rc := &pbscommon.PBSClient{
		BaseURL: e2eBaseURL, CertFingerPrint: e2eFP, AuthID: e2eAuthID, Secret: e2eSecret,
		Datastore: e2eStore, Insecure: true, CompressionLevel: pbscommon.CompressionFastest,
		Manifest: pbscommon.BackupManifest{BackupID: backupID},
	}
	manifests, err := rc.ListSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	var snapTime int64
	for _, m := range manifests {
		if m.BackupID == backupID && m.BackupType == btype && m.BackupTime > snapTime {
			snapTime = m.BackupTime
		}
	}
	if snapTime == 0 {
		t.Fatalf("snapshot for %s not found", backupID)
	}
	t.Logf("snapshot %s/%s/%d", btype, backupID, snapTime)
	rc.Manifest.BackupType = btype
	rc.Manifest.BackupTime = snapTime
	rc.Connect(true, btype)
	defer rc.Close()

	if btype == "vm" {
		blob, err := rc.DownloadToBytes("qemu-server.conf.blob")
		if err != nil {
			t.Fatalf("qemu-server.conf.blob: %v", err)
		}
		t.Logf("qemu-server.conf.blob present, %d bytes", len(blob))
	}

	name := fmt.Sprintf("drive-sata%s.img.fidx", srcN)
	data, err := rc.DownloadToBytes(name)
	if err != nil {
		t.Fatal(err)
	}
	rdr := bytes.NewReader(data)
	var hdr pbscommon.FIDXHeader
	if err := binary.Read(rdr, binary.LittleEndian, &hdr); err != nil {
		t.Fatal(err)
	}
	nchunks := int((hdr.Size + hdr.ChunkSize - 1) / hdr.ChunkSize)
	digests := make([]string, nchunks)
	for i := range digests {
		h := make([]byte, 32)
		if _, err := rdr.Read(h); err != nil {
			t.Fatal(err)
		}
		digests[i] = hex.EncodeToString(h)
	}
	t.Logf("fidx: size=%d chunkSize=%d chunks=%d", hdr.Size, hdr.ChunkSize, nchunks)

	srcSize, err := machinebackuplib.GetDiskSize(srcDev)
	if err != nil {
		t.Fatal(err)
	}
	if uint64(srcSize) != hdr.Size {
		t.Errorf("fidx size %d != source disk size %d", hdr.Size, srcSize)
	}

	// restore: write image to DST disk
	dst, err := os.OpenFile(dstDev, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open %s: %v", dstDev, err)
	}
	defer dst.Close()
	dstSize, err := machinebackuplib.GetDiskSize(dstDev)
	if err != nil {
		t.Fatal(err)
	}
	if uint64(dstSize) < hdr.Size {
		t.Fatalf("destination disk (%d) smaller than image (%d)", dstSize, hdr.Size)
	}
	ctx := context.Background()
	rstart := time.Now()
	for i := 0; i < nchunks; i++ {
		chunk, err := rc.GetChunkData(ctx, digests[i])
		if err != nil {
			t.Fatalf("chunk %d: %v", i, err)
		}
		if sum := sha256.Sum256(chunk); hex.EncodeToString(sum[:]) != digests[i] {
			t.Fatalf("chunk %d: downloaded data does not hash to its digest", i)
		}
		buf := chunk
		if rem := len(buf) % 4096; rem != 0 {
			buf = append(append([]byte{}, buf...), make([]byte, 4096-rem)...)
		}
		if _, err := dst.WriteAt(buf, int64(i)*int64(hdr.ChunkSize)); err != nil {
			t.Fatalf("write chunk %d: %v", i, err)
		}
	}
	dst.Sync()
	t.Logf("restore wrote %d chunks in %v", nchunks, time.Since(rstart).Round(time.Millisecond))

	// read back DST and verify against the digests PBS holds
	bad := 0
	for i := 0; i < nchunks; i++ {
		n := int64(hdr.ChunkSize)
		if rest := int64(hdr.Size) - int64(i)*n; rest < n {
			n = rest
		}
		pad := n
		if rem := pad % 4096; rem != 0 {
			pad += 4096 - rem
		}
		buf := make([]byte, pad)
		if _, err := dst.ReadAt(buf, int64(i)*int64(hdr.ChunkSize)); err != nil {
			t.Fatalf("readback chunk %d: %v", i, err)
		}
		if sum := sha256.Sum256(buf[:n]); hex.EncodeToString(sum[:]) != digests[i] {
			bad++
		}
	}
	t.Logf("readback verification: %d of %d chunks differ", bad, nchunks)
	if bad > 0 {
		t.Errorf("%d chunks read back from the destination disk do not match PBS", bad)
	}
	t.Logf("E2E_MACHINE_SNAPSHOT %s %s %d", btype, backupID, snapTime)
}
