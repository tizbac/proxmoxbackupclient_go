package imagefs

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"sort"
	"testing"
)

// eaEntry lays out one $EA entry; next is its next-entry offset (0 = none).
func eaEntry(flags byte, name string, value []byte, pad bool) []byte {
	b := make([]byte, 8, 8+len(name)+1+len(value)+3)
	b[4] = flags
	b[5] = byte(len(name))
	binary.LittleEndian.PutUint16(b[6:], uint16(len(value)))
	b = append(b, name...)
	b = append(b, 0)
	b = append(b, value...)
	for pad && len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

func TestParseEAs(t *testing.T) {
	a := eaEntry(0, "$LXUID", []byte{0xE8, 3, 0, 0}, true)
	k := eaEntry(0, "$KERNEL.PURGE.ESBCACHE", []byte{1, 2}, true)
	c := eaEntry(0x80, "NEED", []byte("v"), true)
	// NTFS sets the offset on the last entry too.
	binary.LittleEndian.PutUint32(a, uint32(len(a)))
	binary.LittleEndian.PutUint32(k, uint32(len(k)))
	binary.LittleEndian.PutUint32(c, uint32(len(c)))
	all := append(append(append([]byte{}, a...), k...), c...)
	all = append(all, make([]byte, 64)...) // slack after a non-resident $EA's content

	got := parseEAs(all)
	want := []EA{{Name: "$LXUID", Value: []byte{0xE8, 3, 0, 0}}, {Name: "NEED", Flags: 0x80, Value: []byte("v")}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}

	// The FILE_FULL_EA_INFORMATION way: the last entry's offset is 0.
	last := eaEntry(0, "ONE", []byte("x"), false)
	if got := parseEAs(append(last, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)); len(got) != 1 || got[0].Name != "ONE" {
		t.Fatalf("offset 0 ends the list: got %+v", got)
	}

	// Damaged: a value running past the end gives nothing, and no panic.
	bad := eaEntry(0, "LONG", []byte("abcdef"), false)
	binary.LittleEndian.PutUint16(bad[6:], 60000)
	if got := parseEAs(bad); len(got) != 0 {
		t.Fatalf("damaged entry: got %+v", got)
	}
	for n := 0; n < len(all); n++ {
		_ = parseEAs(all[:n])
	}
}

// testdata/ntfs-ea.vhd.gz was made on Windows (a 16 MiB fixed VHD, GPT, one
// NTFS partition), its files' extended attributes written by the app's own
// pbscommon.WriteStreamsAndEAs, and ntfs-ea.expected.json is what Windows
// reads back from them (pbscommon.ReadStreamsAndEAs; scratchpad eafix). It
// has a resident $EA (WSL's $LXUID, $LXGID, $LXMOD), a non-resident one
// (an 8 KiB value), one beside an alternate data stream, and one on a
// folder. The reader must flag exactly those entries and read the same
// attributes.
func TestNTFSExtendedAttributes(t *testing.T) {
	f, err := os.Open("testdata/ntfs-ea.vhd.gz")
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
	parts, err := ReadPartitions(r, int64(len(img))-512) // the VHD footer
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

	type wantEA struct {
		Name  string `json:"n"`
		Flags uint8  `json:"f"`
		Value []byte `json:"v"`
	}
	var expected map[string]struct {
		EAs     []wantEA `json:"eas"`
		Streams []string `json:"streams"`
	}
	raw, err := os.ReadFile("testdata/ntfs-ea.expected.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}

	seen := 0
	for _, e := range entries {
		want, ok := expected[e.Path]
		if !ok {
			continue // System Volume Information and the like
		}
		seen++
		if e.HasEAs != (len(want.EAs) > 0) {
			t.Errorf("%s: HasEAs %v, want %v", e.Path, e.HasEAs, len(want.EAs) > 0)
		}
		got, err := vol.ExtendedAttributes(e)
		if err != nil {
			t.Errorf("%s: %v", e.Path, err)
			continue
		}
		if len(got) != len(want.EAs) {
			t.Errorf("%s: %d extended attributes, want %d (%+v)", e.Path, len(got), len(want.EAs), got)
			continue
		}
		for i, w := range want.EAs {
			g := got[i]
			if g.Name != w.Name || g.Flags != w.Flags || !bytes.Equal(g.Value, w.Value) {
				t.Errorf("%s: attribute %d is %s (flags %#x, %d bytes), want %s (flags %#x, %d bytes)",
					e.Path, i, g.Name, g.Flags, len(g.Value), w.Name, w.Flags, len(w.Value))
			}
		}
		var streams []string
		for _, s := range e.Streams {
			streams = append(streams, s.Name)
		}
		sort.Strings(streams)
		if !reflect.DeepEqual(streams, want.Streams) && (len(streams) > 0 || len(want.Streams) > 0) {
			t.Errorf("%s: streams %v, want %v", e.Path, streams, want.Streams)
		}
	}
	if seen != len(expected) {
		t.Fatalf("found %d of the %d expected entries", seen, len(expected))
	}
}
