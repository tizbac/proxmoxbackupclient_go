package main

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"testing"

	"github.com/tizbac/proxmoxbackupclient_go/gui/imagefs"
)

// The cached image lists round-trip through gob with everything restore
// needs: record numbers, exFAT valid lengths, NTFS security ids and
// streams, POSIX modes, owners and link targets.
func TestImageVolumesCacheRoundTrip(t *testing.T) {
	in := imageVolumesCache{Schema: imageCacheSchema, Key: snapshotCacheKey{PBSID: "s", BackupID: "pc", SnapshotAt: 42},
		Vols: []imageVolume{{
			Display: "Partition 3 - Windows (NTFS, 1.0 GB)", Archive: "drive-sata0.img.fidx", Start: 1 << 20, Size: 1 << 30,
			FS: "NTFS", CaseInsensitive: true,
			Entries: []imagefs.Entry{
				{Path: "Users", IsDir: true, MTime: 1, Record: 70, SecurityID: 256},
				{Path: "Users/a.txt", Size: 5, MTime: 2, Created: 3, Attrs: 0x20, Record: 71, SecurityID: 257,
					Streams: []imagefs.Stream{{Name: "Zone.Identifier", Size: 26}}, HasEAs: true},
			},
		}, {
			Display: "Partition 4 (ext4, 2.0 GB)", FS: "ext4",
			Entries: []imagefs.Entry{{Path: "link", Symlink: true, LinkTarget: "etc/hosts", Mode: 0o120777, UID: 1000, GID: 1000, HasOwner: true}},
		}}}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(in); err != nil {
		t.Fatal(err)
	}
	var out imageVolumesCache
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip differs:\n%+v\n%+v", in, out)
	}
}
