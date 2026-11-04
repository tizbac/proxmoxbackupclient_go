package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/rand"
	"sync/atomic"
	"testing"

	"github.com/alphadose/haxmap"

	"pbscommon"
)

// serialChunkIndex chunks data, fed in the given pieces, the way the walk did
// before the workers hashed: a digest at each boundary and the index checksum
// built as it goes. It is the reference the workers' index must match.
func serialChunkIndex(data []byte, pieces []int) (digests []string, offsets []uint64, csum string) {
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

// The app's index, hashed by the workers in any order, is the one the walk
// built when it hashed each chunk itself: the same chunks at the same offsets
// with the same digests and checksum. Every chunk is already known to PBS
// here, so the workers hash and look up but send nothing (no server needed).
func TestChunkStateIndexMatchesSerialHashing(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	data := make([]byte, 48<<20)
	rng.Read(data)
	var pieces []int
	for left := len(data); left > 0; {
		n := 1 + rng.Intn(256<<10)
		if n > left {
			n = left
		}
		pieces = append(pieces, n)
		left -= n
	}
	wantD, wantO, wantSum := serialChunkIndex(data, pieces)

	known := haxmap.New[string, bool]()
	for _, d := range wantD {
		known.Set(d, true)
	}
	var newchunk, reusechunk, failedchunk atomic.Uint64
	c := &ChunkState{}
	client := &pbscommon.PBSClient{}
	c.Init(client, &newchunk, &reusechunk, &failedchunk, known, nil, nil, nil, "")
	at := 0
	for _, n := range pieces {
		if err := c.HandleData(data[at:at+n], client); err != nil {
			t.Fatal(err)
		}
		at += n
	}
	if len(c.currentChunk) > 0 {
		if err := c.processChunk(client); err != nil {
			t.Fatal(err)
		}
	}
	c.closeUploads()
	c.uploadWG.Wait()
	if err := c.checkFatal(); err != nil {
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
	if reusechunk.Load() != uint64(len(wantD)) || newchunk.Load() != 0 {
		t.Errorf("reused %d, new %d, want %d and 0", reusechunk.Load(), newchunk.Load(), len(wantD))
	}
}

// A chunk reused from the previous snapshot without its data keeps its place
// between chunks the workers hash.
func TestChunkStateIndexKeepsInjectedChunks(t *testing.T) {
	prev := sha256.Sum256([]byte("from the previous snapshot"))
	first, second := sha256.Sum256([]byte("first")), sha256.Sum256([]byte("second"))
	known := haxmap.New[string, bool]()
	for _, d := range [][32]byte{prev, first, second} {
		known.Set(hex.EncodeToString(d[:]), true)
	}
	var newchunk, reusechunk, failedchunk atomic.Uint64
	c := &ChunkState{}
	client := &pbscommon.PBSClient{}
	c.Init(client, &newchunk, &reusechunk, &failedchunk, known, nil, nil, nil, "")
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
	c.closeUploads()
	c.uploadWG.Wait()
	d, o, _ := c.indexList()
	p := hex.EncodeToString(prev[:])
	want := []string{hex.EncodeToString(first[:]), p, hex.EncodeToString(second[:]), p}
	wantO := []uint64{0, 5, 1005, 1011}
	for i := range want {
		if d[i] != want[i] || o[i] != wantO[i] {
			t.Errorf("entry %d: %s at %d, want %s at %d", i, d[i][:12], o[i], want[i][:12], wantO[i])
		}
	}
}
