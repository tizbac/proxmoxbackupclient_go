package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/rand"
	"pbscommon"
	"testing"
	"time"
)

// serialIndex chunks data, fed in the given pieces, the way ChunkState did
// before its workers hashed: a digest at each boundary and the index checksum
// built as it goes. It is the reference the parallel index must match.
func serialIndex(data []byte, pieces []int) (digests []string, offsets []uint64, csum string) {
	var ch pbscommon.Chunker
	ch.New(4 << 20)
	h := sha256.New()
	var cur []byte
	var pos uint64
	emit := func() {
		d := sha256.Sum256(cur)
		_ = binary.Write(h, binary.LittleEndian, pos+uint64(len(cur)))
		h.Write(d[:])
		digests = append(digests, hex.EncodeToString(d[:]))
		offsets = append(offsets, pos)
		pos += uint64(len(cur))
		cur = nil
	}
	at := 0
	for _, n := range pieces {
		b := data[at : at+n]
		at += n
		for {
			p := ch.Scan(b)
			if p == 0 {
				cur = append(cur, b...)
				break
			}
			cur = append(cur, b[:p]...)
			emit()
			b = b[p:]
		}
	}
	if len(cur) > 0 {
		emit()
	}
	return digests, offsets, hex.EncodeToString(h.Sum(nil))
}

// The index built by the workers, which finish in any order, is the one the
// walk built when it hashed each chunk itself: the same chunks at the same
// offsets with the same digests, and the same checksum. The data comes in
// pieces of random size, as files and archive headers do.
func TestIndexMatchesSerialHashing(t *testing.T) {
	rng := rand.New(rand.NewSource(68))
	data := make([]byte, 60<<20)
	rng.Read(data)
	// A stretch repeated later in the stream gives chunks with the same
	// content, so some are reused within the stream.
	copy(data[40<<20:52<<20], data[4<<20:16<<20])
	var pieces []int
	for left := len(data); left > 0; {
		n := 1 + rng.Intn(300<<10)
		if n > left {
			n = left
		}
		pieces = append(pieces, n)
		left -= n
	}
	wantD, wantO, wantSum := serialIndex(data, pieces)

	c, newchunk, reusechunk := poolChunks(func(uint64, string, []byte) error {
		time.Sleep(time.Duration(rand.Intn(3)) * time.Millisecond) // finish out of order
		return nil
	})
	client := &pbscommon.PBSClient{}
	at := 0
	for _, n := range pieces {
		if err := c.HandleData(data[at:at+n], client); err != nil {
			t.Fatal(err)
		}
		at += n
	}
	if err := c.emitCurrent(client); err != nil {
		t.Fatal(err)
	}
	c.stopUploads(false)
	if err := c.uploadErr(); err != nil {
		t.Fatal(err)
	}
	gotD, gotO, gotSum := c.indexList()
	if len(gotD) != len(wantD) {
		t.Fatalf("%d chunks, want %d", len(gotD), len(wantD))
	}
	for i := range wantD {
		if gotD[i] != wantD[i] || gotO[i] != wantO[i] {
			t.Fatalf("chunk %d: %s at %d, want %s at %d", i, gotD[i][:12], gotO[i], wantD[i][:12], wantO[i])
		}
	}
	if gotSum != wantSum {
		t.Errorf("index checksum %s, want %s", gotSum[:12], wantSum[:12])
	}
	if c.pos != uint64(len(data)) || c.chunkcount != uint64(len(wantD)) {
		t.Errorf("size %d in %d chunks, want %d in %d", c.pos, c.chunkcount, len(data), len(wantD))
	}
	if newchunk.Load()+reusechunk.Load() != uint64(len(wantD)) || reusechunk.Load() == 0 {
		t.Errorf("new %d + reused %d, want %d with some reused", newchunk.Load(), reusechunk.Load(), len(wantD))
	}
	t.Logf("%d chunks, %d new, %d reused within the stream", len(wantD), newchunk.Load(), reusechunk.Load())
}

// A chunk reused from the previous snapshot without its data (metadata change
// detection) keeps its place in the index between chunks the workers hash.
func TestIndexKeepsInjectedChunks(t *testing.T) {
	c, _, _ := poolChunks(func(uint64, string, []byte) error {
		time.Sleep(2 * time.Millisecond)
		return nil
	})
	prev := sha256.Sum256([]byte("from the previous snapshot"))
	c.knownChunks.Set(hex.EncodeToString(prev[:]), true)
	client := &pbscommon.PBSClient{}
	for _, s := range []string{"first", "second"} {
		if err := c.HandleData([]byte(s), client); err != nil {
			t.Fatal(err)
		}
		if err := c.ForceBoundary(client); err != nil {
			t.Fatal(err)
		}
		if err := c.InjectKnownChunk(prev, 1000); err != nil {
			t.Fatal(err)
		}
	}
	c.stopUploads(false)
	d, o, _ := c.indexList()
	p := hex.EncodeToString(prev[:])
	want := []string{digestOf("first"), p, digestOf("second"), p}
	wantO := []uint64{0, 5, 1005, 1011}
	for i := range want {
		if d[i] != want[i] || o[i] != wantO[i] {
			t.Errorf("entry %d: %s at %d, want %s at %d", i, d[i][:12], o[i], want[i][:12], wantO[i])
		}
	}
}
