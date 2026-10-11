package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"machinebackuplib"
	"os"
	"path/filepath"
	"pbscommon"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/imagefs"
)

// Opt-in (PBS_FREE_DISK = a spare disk's number, PBS_FREE_LETTER = the
// drive letter of an NTFS volume on it; run elevated): writes and deletes
// 20 MB of random data on the volume so its free space holds garbage, makes
// a machine backup of the disk to the e2e server with free space skipped,
// then reads the image back and checks that every file matches the live one
// and that the deleted data did not go into the image.
func TestLiveFreeSpaceSkip(t *testing.T) {
	diskN, letter := os.Getenv("PBS_FREE_DISK"), os.Getenv("PBS_FREE_LETTER")
	if diskN == "" || letter == "" {
		t.Skip("set PBS_FREE_DISK and PBS_FREE_LETTER")
	}
	root := letter + `:\`
	junk := make([]byte, 20<<20)
	rand.Read(junk)
	marker := junk[1<<20 : 1<<20+64] // looked for in the image afterwards
	jp := filepath.Join(root, "junk.bin")
	if err := os.WriteFile(jp, junk, 0o644); err != nil {
		t.Fatal(err)
	}
	f, _ := os.OpenFile(jp, os.O_RDWR, 0)
	f.Sync()
	f.Close()
	if err := os.Remove(jp); err != nil {
		t.Fatal(err)
	}

	backupID := fmt.Sprintf("freespace-test-%d", time.Now().Unix())
	prog := &machinebackuplib.JobProgress{}
	start := time.Now()
	_, err := machinebackuplib.Backup(&machinebackuplib.Config{
		BaseURL: e2eBaseURL, CertFingerprint: e2eFP, AuthID: e2eAuthID, Secret: e2eSecret,
		Datastore: e2eStore, BackupID: backupID, BackupType: "host", BackupDevices: []string{`\\.\PhysicalDrive` + diskN},
		Progress: prog,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("backup %s in %v: free space not read %d MB; chunks new %d, reused %d (%d blank)", backupID, time.Since(start).Round(time.Millisecond),
		prog.FreeSkipped.Load()>>20, prog.NewChunks.Load(), prog.ReusedChunks.Load(), prog.BlankChunks.Load())
	if prog.FreeSkipped.Load() == 0 {
		t.Error("no free space was skipped")
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
	n, _ := strconv.Atoi(diskN)
	ra, size, err := client.NewFIDXReaderAt(fmt.Sprintf("drive-sata%d.img.fidx", n), 32, nil)
	if err != nil {
		t.Fatal(err)
	}
	parts, err := imagefs.ReadPartitions(ra, size)
	if err != nil {
		t.Fatal(err)
	}
	var vol *imagefs.NTFS
	var part imagefs.Partition
	for _, p := range parts {
		if p.FS == "ntfs" {
			part = p
			if vol, err = imagefs.OpenNTFS(io.NewSectionReader(ra, p.Start, p.Size)); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if vol == nil {
		t.Fatal("no NTFS volume in the image")
	}
	entries, err := vol.List(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir || strings.HasPrefix(e.Path, "System Volume Information") {
			continue
		}
		live, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(e.Path)))
		if err != nil {
			t.Errorf("%s: %v", e.Path, err)
			continue
		}
		fi, err := vol.Open(e.Record, int64(e.Size))
		if err != nil {
			t.Errorf("%s: %v", e.Path, err)
			continue
		}
		h := sha256.New()
		io.Copy(h, io.NewSectionReader(fi, 0, fi.Size()))
		lh := sha256.Sum256(live)
		if hex.EncodeToString(h.Sum(nil)) != hex.EncodeToString(lh[:]) {
			t.Errorf("%s: differs from the live file", e.Path)
		}
		checked++
	}
	t.Logf("%d files in the image match the live volume", checked)

	// The deleted junk lived in now-free clusters: the image must not hold it.
	buf := make([]byte, 4<<20)
	found := false
	for off := int64(0); off < part.Size && !found; off += int64(len(buf)) {
		k, _ := ra.ReadAt(buf, part.Start+off)
		found = strings.Contains(string(buf[:k]), string(marker))
	}
	if found {
		t.Error("the deleted data is in the image: free space was read")
	} else {
		t.Log("the deleted data is not in the image (free space went in as zeros)")
	}

	// PBS_FREE_VHD: also write the image out as a fixed VHD (the raw disk
	// plus a 512-byte footer), to mount read-only and check with chkdsk.
	if out := os.Getenv("PBS_FREE_VHD"); out != "" {
		w, err := os.Create(out)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(w, io.NewSectionReader(ra, 0, size)); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(fixedVHDFooter(size)); err != nil {
			t.Fatal(err)
		}
		w.Close()
		t.Logf("image written to %s", out)
	}
}

// fixedVHDFooter is the 512-byte footer that makes a raw disk image a fixed
// VHD (Microsoft's VHD specification, "Hard Disk Footer Format").
func fixedVHDFooter(size int64) []byte {
	f := make([]byte, 512)
	be32 := func(o int, v uint32) { f[o], f[o+1], f[o+2], f[o+3] = byte(v>>24), byte(v>>16), byte(v>>8), byte(v) }
	be64 := func(o int, v uint64) { be32(o, uint32(v>>32)); be32(o+4, uint32(v)) }
	copy(f[0:], "conectix")
	be32(8, 2)          // features: reserved bit
	be32(12, 0x00010000) // format version 1.0
	be64(16, ^uint64(0)) // data offset: none for a fixed disk
	be32(24, uint32(time.Now().Unix()-946684800))
	copy(f[28:], "pbsc")
	be32(32, 0x00010000)
	copy(f[36:], "Wi2k")
	be64(40, uint64(size))
	be64(48, uint64(size))
	// CHS geometry, per the specification's algorithm.
	sectors := size / 512
	if sectors > 65535*16*255 {
		sectors = 65535 * 16 * 255
	}
	var spt, heads, cyl int64
	if sectors >= 65535*16*63 {
		spt, heads = 255, 16
		cyl = sectors / (spt * heads)
	} else {
		spt = 17
		cyl = sectors / spt
		heads = (cyl + 1023) / 1024
		if heads < 4 {
			heads = 4
		}
		if cyl >= heads*1024 || heads > 16 {
			spt, heads = 31, 16
			cyl = sectors / spt
		}
		if cyl >= heads*1024 {
			spt, heads = 63, 16
			cyl = sectors / spt
		}
		cyl /= heads
	}
	f[56], f[57], f[58], f[59] = byte(cyl>>8), byte(cyl), byte(heads), byte(spt)
	be32(60, 2) // disk type: fixed
	rand.Read(f[68:84])
	var sum uint32
	for _, b := range f {
		sum += uint32(b)
	}
	be32(64, ^sum)
	return f
}
