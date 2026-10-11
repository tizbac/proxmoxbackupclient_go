package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"machinebackuplib"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tizbac/proxmoxbackupclient_go/gui/imagefs"
)

// The Linux free-space maps (machinebackuplib.LinuxFreeSpaceMap) must never
// call a used block free: zero everything they call free in the fixture
// images, then every file must still read back byte for byte through the
// image readers. They must also find real free space (the images are
// mostly empty).
func TestLinuxFreeSpaceMapsKeepEveryFile(t *testing.T) {
	for _, name := range []string{"ext4", "ext2", "ext4small", "xfs"} {
		t.Run(name, func(t *testing.T) {
			f, err := os.Open(filepath.Join("imagefs", "testdata", "fs", name+".img.gz"))
			if err != nil {
				t.Fatal(err)
			}
			zr, _ := gzip.NewReader(f)
			img, err := io.ReadAll(zr)
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			m, err := machinebackuplib.LinuxFreeSpaceMap(bytes.NewReader(img), int64(len(img)))
			if err != nil {
				t.Fatal(err)
			}
			used := m.UsedBytes()
			m.ZeroFree(img, 0)
			// PBS_FS_DUMP=<dir>: keep the zeroed images, to check them with
			// e2fsck -fn / xfs_repair -n on Linux.
			if d := os.Getenv("PBS_FS_DUMP"); d != "" {
				_ = os.WriteFile(filepath.Join(d, name+".zeroed.img"), img, 0o644)
			}
			if used >= int64(len(img))/2 {
				t.Errorf("only %d of %d bytes free", int64(len(img))-used, len(img))
			}
			r := bytes.NewReader(img)
			vol, err := imagefs.OpenVolume(imagefs.Probe(r), r, r.Size())
			if err != nil {
				t.Fatal(err)
			}
			entries, err := vol.List(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			sums, _ := os.ReadFile(filepath.Join("imagefs", "testdata", "fs", name+".sha256.txt"))
			want := map[string]string{}
			for _, line := range strings.Split(strings.TrimSpace(string(sums)), "\n") {
				line = strings.TrimRight(line, "\r")
				if i := strings.Index(line, "  "); i > 0 {
					want[line[i+2:]] = line[:i]
				}
			}
			checked := 0
			for _, e := range entries {
				sum, ok := want[e.Path]
				if !ok {
					continue
				}
				fr, err := vol.Open(e)
				if err != nil {
					t.Errorf("%s: %v", e.Path, err)
					continue
				}
				h := sha256.New()
				io.Copy(h, io.NewSectionReader(fr, 0, fr.Size()))
				if hex.EncodeToString(h.Sum(nil)) != sum {
					t.Errorf("%s: content lost by the free-space map", e.Path)
				}
				checked++
			}
			if checked < 150 {
				t.Errorf("only %d files checked", checked)
			}
			t.Logf("%s: %d of %d bytes used; %d files intact", name, used, len(img), checked)
		})
	}
}
