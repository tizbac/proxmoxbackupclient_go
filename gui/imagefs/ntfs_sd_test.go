package imagefs

import (
	"encoding/binary"
	"io"
	"testing"
)

// The NTFS fixture's files carry security ids that resolve in $Secure:$SDS
// to self-relative descriptors, and its file with an alternate data stream
// lists that stream and reads it.
func TestNTFSSecurityAndStreams(t *testing.T) {
	r, size := loadFixture(t)
	parts, err := ReadPartitions(r, size)
	if err != nil {
		t.Fatal(err)
	}
	var vol *NTFS
	for _, p := range parts {
		if p.FS == "ntfs" {
			if vol, err = OpenNTFS(io.NewSectionReader(r, p.Start, p.Size)); err != nil {
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
	withSD, streams := 0, 0
	for _, e := range entries {
		if e.SecurityID == 0 {
			t.Errorf("%s: no security id", e.Path)
			continue
		}
		sd, ok := vol.SecurityDescriptor(e.SecurityID)
		if !ok || len(sd) < 20 || sd[0] != 1 || binary.LittleEndian.Uint16(sd[2:])&0x8000 == 0 {
			t.Errorf("%s: security id %d has no self-relative descriptor (%d bytes)", e.Path, e.SecurityID, len(sd))
			continue
		}
		withSD++
		for _, s := range e.Streams {
			f, err := vol.OpenStream(e, s.Name)
			if err != nil {
				t.Errorf("%s:%s: %v", e.Path, s.Name, err)
				continue
			}
			b := make([]byte, f.Size())
			if n, err := f.ReadAt(b, 0); (err != nil && err != io.EOF) || n != len(b) {
				t.Errorf("%s:%s: read %d of %d: %v", e.Path, s.Name, n, len(b), err)
			}
			t.Logf("%s:%s (%d bytes) %q", e.Path, s.Name, s.Size, string(b[:min(len(b), 40)]))
			streams++
		}
	}
	if streams == 0 {
		t.Error("the fixture's alternate data stream was not found")
	}
	t.Logf("%d entries with descriptors, %d streams", withSD, streams)
}
