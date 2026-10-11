package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"pbscommon"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/imagefs"
)

// Opt-in (PBS_IMG_URL set): opens a real machine backup on a PBS server, reads
// its partition table and lists its largest NTFS volume straight from the
// fixed index, then reads a few files end to end. Read only. Variables:
// PBS_IMG_URL, PBS_IMG_FP, PBS_IMG_AUTHID, PBS_IMG_SECRET, PBS_IMG_STORE,
// PBS_IMG_TYPE (host|vm), PBS_IMG_ID, optional PBS_IMG_TIME (unix; newest
// if unset) and PBS_IMG_ARCHIVE (default drive-sata0.img.fidx).
func TestLiveImageBrowse(t *testing.T) {
	url := os.Getenv("PBS_IMG_URL")
	if url == "" {
		t.Skip("set PBS_IMG_URL and friends")
	}
	client := &pbscommon.PBSClient{
		BaseURL: url, CertFingerPrint: os.Getenv("PBS_IMG_FP"), Insecure: os.Getenv("PBS_IMG_FP") != "",
		AuthID: os.Getenv("PBS_IMG_AUTHID"), Secret: os.Getenv("PBS_IMG_SECRET"), Datastore: os.Getenv("PBS_IMG_STORE"),
	}
	btype, id := os.Getenv("PBS_IMG_TYPE"), os.Getenv("PBS_IMG_ID")
	snaps, err := client.ListSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	var when int64
	if s := os.Getenv("PBS_IMG_TIME"); s != "" {
		when, _ = strconv.ParseInt(s, 10, 64)
	}
	for _, s := range snaps {
		if s.BackupType == btype && s.BackupID == id && (when == 0 && s.BackupTime > when || s.BackupTime == when) {
			when = s.BackupTime
		}
	}
	if when == 0 {
		t.Fatalf("no %s/%s snapshot", btype, id)
	}
	client.Manifest = pbscommon.BackupManifest{BackupType: btype, BackupID: id, BackupTime: when}
	client.Connect(true, btype)
	archive := os.Getenv("PBS_IMG_ARCHIVE")
	if archive == "" {
		archive = "drive-sata0.img.fidx"
	}

	start := time.Now()
	ra, size, err := client.NewFIDXReaderAt(archive, 32, nil)
	if err != nil {
		t.Fatal(err)
	}
	parts, err := imagefs.ReadPartitions(ra, size)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s/%s %s: %d bytes, %d partitions (%v)", btype, id, time.Unix(when, 0).UTC().Format(time.RFC3339), size, len(parts), time.Since(start))
	var best *imagefs.Partition
	for i := range parts {
		p := parts[i]
		t.Logf("  %s  type %q name %q at %d", p.Label(), p.Type, p.Name, p.Start)
		if p.FS == "ntfs" && (best == nil || p.Size > best.Size) {
			best = &parts[i]
		}
	}
	if best == nil {
		t.Fatal("no NTFS partition")
	}

	start = time.Now()
	vol, err := imagefs.OpenNTFS(io.NewSectionReader(ra, best.Start, best.Size))
	if err != nil {
		t.Fatal(err)
	}
	var lastPct int64 = -1
	entries, err := vol.List(nil, func(d, tot int64) {
		if p := d * 10 / tot; p != lastPct {
			lastPct = p
			t.Logf("  MFT %d%% (%d MB of %d MB) after %v", p*10, d>>20, tot>>20, time.Since(start).Round(time.Second))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	st := ra.Stats()
	var files, dirs int
	var bytes uint64
	for _, e := range entries {
		if e.IsDir {
			dirs++
		} else {
			files++
			bytes += e.Size
		}
	}
	t.Logf("listed %d files, %d folders, %.1f GB in %v (%d chunks fetched, fetch %v summed)",
		files, dirs, float64(bytes)/1e9, time.Since(start).Round(time.Millisecond), st.ChunksFetched, time.Duration(st.FetchNanos).Round(time.Second))
	for _, want := range []string{"Windows/System32/notepad.exe", "Windows/explorer.exe", "Users"} {
		found := false
		for _, e := range entries {
			if strings.EqualFold(e.Path, want) {
				found = true
				t.Logf("  %s: %d bytes, modified %s", e.Path, e.Size, time.Unix(imagefs.UnixSeconds(e.MTime), 0).UTC().Format(time.RFC3339))
			}
		}
		if !found {
			t.Errorf("%s not found", want)
		}
	}

	// Read the three largest files under 50 MB and a few small ones end to end.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Size > entries[j].Size })
	n := 0
	for _, e := range entries {
		if e.IsDir || e.Size > 50<<20 || e.Size == 0 || n >= 3 {
			continue
		}
		n++
		readOne(t, vol, e)
	}
	for _, e := range entries[max(0, len(entries)-200):] {
		if !e.IsDir && e.Size > 0 && e.Size < 4096 {
			readOne(t, vol, e)
			break
		}
	}
}

func readOne(t *testing.T, vol *imagefs.NTFS, e imagefs.Entry) {
	start := time.Now()
	f, err := vol.Open(e.Record, int64(e.Size))
	if err != nil {
		t.Errorf("%s: %v", e.Path, err)
		return
	}
	h := sha256.New()
	n, err := io.Copy(h, io.NewSectionReader(f, 0, f.Size()))
	if err != nil {
		t.Errorf("%s: read: %v", e.Path, err)
		return
	}
	t.Logf("  read %s: %d bytes in %v, sha256 %s", e.Path, n, time.Since(start).Round(time.Millisecond), hex.EncodeToString(h.Sum(nil))[:16])
	if uint64(n) != e.Size {
		t.Errorf("%s: read %d of %d", e.Path, n, e.Size)
	}
	_ = fmt.Sprint
}
