package imagefs

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The images in testdata/fs were made on Linux (scratchpad mkfixtures.sh):
// the same tree on ext4, ext2, ext4 with 1 KiB blocks and inline data, XFS,
// exFAT and FAT32, with `find -printf` and sha256sum listings taken from the
// mounted file system. Each reader must list the same entries (type, size,
// time; on ext and XFS the permissions, on ext the owners and link targets)
// and read every file back byte for byte.

func loadImage(t *testing.T, name string) *bytes.Reader {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "fs", name+".img.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b)
}

type wantEntry struct {
	kind     byte // d, f, l
	size     int64
	perm     uint32
	uid, gid uint32
	mtime    float64
}

func loadExpected(t *testing.T, name string) (map[string]wantEntry, map[string]string) {
	t.Helper()
	want := map[string]wantEntry{}
	f, err := os.Open(filepath.Join("testdata", "fs", name+".expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), " ", 7)
		if len(parts) != 7 {
			continue
		}
		size, _ := strconv.ParseInt(parts[1], 10, 64)
		perm, _ := strconv.ParseUint(parts[2], 8, 32)
		uid, _ := strconv.ParseUint(parts[3], 10, 32)
		gid, _ := strconv.ParseUint(parts[4], 10, 32)
		mt, _ := strconv.ParseFloat(parts[5], 64)
		want[parts[6]] = wantEntry{kind: parts[0][0], size: size, perm: uint32(perm), uid: uint32(uid), gid: uint32(gid), mtime: mt}
	}
	sums := map[string]string{}
	s, err := os.ReadFile(filepath.Join("testdata", "fs", name+".sha256.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(s)), "\n") {
		if i := strings.Index(line, "  "); i > 0 {
			sums[line[i+2:]] = line[:i]
		}
	}
	return want, sums
}

func TestFileSystemFixtures(t *testing.T) {
	saved := FATTimeLocation
	FATTimeLocation = time.UTC // the images were written with UTC FAT times
	defer func() { FATTimeLocation = saved }()

	for _, tc := range []struct {
		name, probe, fsName, label string
		posix, owners, links       bool
		timeSlack                  float64 // seconds
	}{
		{"ext4", "ext", "ext4", "EXT4TEST", true, true, true, 0.001},
		{"ext2", "ext", "ext2/3", "EXT2TEST", true, true, true, 1},
		{"ext4small", "ext", "ext4", "SMALLBLK", true, true, true, 0.001},
		{"xfs", "xfs", "XFS", "XFSTEST", true, false, false, 0.001},
		{"exfat", "exfat", "exFAT", "EXFATTEST", false, false, false, 2},
		{"fat32", "fat32", "FAT32", "FAT32TEST", false, false, false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := loadImage(t, tc.name)
			if got := Probe(r); got != tc.probe {
				t.Fatalf("Probe = %q, want %q", got, tc.probe)
			}
			vol, err := OpenVolume(tc.probe, r, r.Size())
			if err != nil {
				t.Fatal(err)
			}
			entries, err := vol.List(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if vol.FSName() != tc.fsName {
				t.Errorf("FSName %q, want %q", vol.FSName(), tc.fsName)
			}
			if vol.Label() != tc.label {
				t.Errorf("Label %q, want %q", vol.Label(), tc.label)
			}
			want, sums := loadExpected(t, tc.name)
			got := map[string]Entry{}
			for _, e := range entries {
				got[e.Path] = e
				if _, ok := want[e.Path]; !ok {
					t.Errorf("unexpected entry %q", e.Path)
				}
			}
			for p, w := range want {
				if !tc.links && w.kind == 'l' {
					if e, ok := got[p]; ok && !e.Symlink {
						t.Errorf("%s: not marked as a link", p)
					}
					continue
				}
				e, ok := got[p]
				if !ok {
					t.Errorf("missing %q", p)
					continue
				}
				switch w.kind {
				case 'd':
					if !e.IsDir {
						t.Errorf("%s: not a folder", p)
					}
				case 'f':
					if e.IsDir || e.Symlink || int64(e.Size) != w.size {
						t.Errorf("%s: dir %v link %v size %d, want a file of %d", p, e.IsDir, e.Symlink, e.Size, w.size)
					}
				case 'l':
					if !e.Symlink || e.LinkTarget != "docs/sub/deeper/deep.txt" {
						t.Errorf("%s: link %v to %q", p, e.Symlink, e.LinkTarget)
					}
				}
				if tc.posix && e.Mode&0o7777 != w.perm {
					t.Errorf("%s: mode %o, want %o", p, e.Mode&0o7777, w.perm)
				}
				if tc.owners && (e.UID != w.uid || e.GID != w.gid || !e.HasOwner) {
					t.Errorf("%s: owner %d:%d (%v), want %d:%d", p, e.UID, e.GID, e.HasOwner, w.uid, w.gid)
				}
				if w.kind != 'l' || tc.links {
					gotT := float64(e.MTime-116444736000000000) / 1e7
					if math.Abs(gotT-w.mtime) > tc.timeSlack {
						t.Errorf("%s: mtime %.3f, want %.3f", p, gotT, w.mtime)
					}
				}
			}
			for p, sum := range sums {
				e, ok := got[p]
				if !ok {
					continue
				}
				f, err := vol.Open(e)
				if err != nil {
					t.Errorf("open %s: %v", p, err)
					continue
				}
				h := sha256.New()
				if _, err := io.Copy(h, io.NewSectionReader(f, 0, f.Size())); err != nil {
					t.Errorf("read %s: %v", p, err)
					continue
				}
				if hex.EncodeToString(h.Sum(nil)) != sum {
					t.Errorf("%s: content differs", p)
				}
			}
			if e, ok := got["sparse.bin"]; ok && tc.name == "ext4" {
				f, _ := vol.Open(e)
				var mapped int64
				for _, x := range f.Extents() {
					mapped += x[1] - x[0]
				}
				if mapped == 0 || mapped >= f.Size() {
					t.Errorf("sparse.bin maps %d of %d bytes; the hole should not be mapped", mapped, f.Size())
				}
			}
			t.Logf("%s: %d entries, %d files checked", vol.FSName(), len(entries), len(sums))
		})
	}
}
