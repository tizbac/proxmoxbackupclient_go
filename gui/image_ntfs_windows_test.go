//go:build windows

package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/imagefs"
	"pbscommon"
)

// Files and folders restored from an NTFS volume in a disk image get their
// extended attributes and alternate data streams back, as a folder restore
// does, and keep the image's modification time although writing a stream
// changes it (imagefs/testdata/ntfs-ea.vhd.gz, see imagefs's
// TestNTFSExtendedAttributes).
func TestImageRestoreExtendedAttributes(t *testing.T) {
	f, err := os.Open(filepath.Join("imagefs", "testdata", "ntfs-ea.vhd.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	img, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	r := bytes.NewReader(img)
	parts, err := imagefs.ReadPartitions(r, int64(len(img))-512)
	if err != nil {
		t.Fatal(err)
	}
	var vol *imagefs.NTFS
	for _, p := range parts {
		if p.FS == "ntfs" {
			if vol, err = imagefs.OpenNTFS(io.NewSectionReader(r, p.Start, p.Size)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if vol == nil {
		t.Fatal("no NTFS partition")
	}
	entries, err := vol.List(nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	buf := make([]byte, 1<<20)
	opts := RestoreOptions{}
	files, dirs, withStreams := 0, 0, 0
	for _, e := range entries {
		if !e.HasEAs && len(e.Streams) == 0 {
			continue
		}
		dest := filepath.Join(root, filepath.FromSlash(e.Path))
		want, err := vol.ExtendedAttributes(e)
		if err != nil {
			t.Fatal(err)
		}
		if e.IsDir {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				t.Fatal(err)
			}
			if _, err := restoreImageEAs(vol, dest, e); err != nil {
				t.Fatalf("%s: %v", e.Path, err)
			}
			dirs++
		} else {
			fr, err := vol.Open(e)
			if err != nil {
				t.Fatal(err)
			}
			res := writeImageFile(opts, dest, fr, e, buf, nil, func(d string) {
				if _, err := restoreImageStreams(vol, d, e, buf); err != nil {
					t.Errorf("%s: streams: %v", e.Path, err)
				}
				if _, err := restoreImageEAs(vol, d, e); err != nil {
					t.Errorf("%s: extended attributes: %v", e.Path, err)
				}
			})
			if res.Skipped {
				t.Fatalf("%s: skipped: %s", e.Path, res.SkipReason)
			}
			fi, err := os.Stat(dest)
			if err != nil {
				t.Fatal(err)
			}
			wantTime := time.Unix(imagefs.UnixSeconds(e.MTime), int64(filetimeNanos(e.MTime)))
			if !fi.ModTime().Equal(wantTime) {
				t.Errorf("%s: modified %v, want the image's %v", e.Path, fi.ModTime(), wantTime)
			}
			files++
		}

		streams, eas, err := pbscommon.ReadStreamsAndEAs(dest)
		if err != nil {
			t.Fatal(err)
		}
		if len(eas) != len(want) {
			t.Errorf("%s: %d extended attributes restored, want %d", e.Path, len(eas), len(want))
		} else {
			for i := range want {
				if eas[i].Name != want[i].Name || eas[i].Flags != want[i].Flags || !bytes.Equal(eas[i].Value, want[i].Value) {
					t.Errorf("%s: attribute %s restored as %s (%d bytes, want %d)", e.Path, want[i].Name, eas[i].Name, len(eas[i].Value), len(want[i].Value))
				}
			}
		}
		if len(streams) != len(e.Streams) {
			t.Errorf("%s: %d streams restored, want %d", e.Path, len(streams), len(e.Streams))
		}
		if len(e.Streams) > 0 {
			withStreams++
		}
	}
	if files < 3 || dirs < 1 || withStreams < 1 {
		t.Fatalf("fixture not covered: %d files, %d folders, %d with streams", files, dirs, withStreams)
	}
	t.Logf("%d files and %d folders restored with their extended attributes (%d with streams)", files, dirs, withStreams)
}
