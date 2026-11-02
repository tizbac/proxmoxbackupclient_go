package pbscommon

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func makeFIDX(size, chunkSize uint64, digests [][32]byte) []byte {
	b := make([]byte, fidxHeaderSize)
	copy(b, fidxMagic)
	binary.LittleEndian.PutUint64(b[64:], size)
	binary.LittleEndian.PutUint64(b[72:], chunkSize)
	for _, d := range digests {
		b = append(b, d[:]...)
	}
	return b
}

func TestParseFIDXIndex(t *testing.T) {
	d := [][32]byte{sha256.Sum256([]byte("a")), sha256.Sum256([]byte("b")), sha256.Sum256([]byte("c"))}
	// 2.5 chunks of 4 MiB: the last chunk is short.
	size := uint64(2*PBS_FIXED_CHUNK_SIZE + 1000)
	idx, err := parseFIDXIndex("x", makeFIDX(size, PBS_FIXED_CHUNK_SIZE, d))
	if err != nil {
		t.Fatal(err)
	}
	if idx.total != size || len(idx.offsets) != 3 {
		t.Fatalf("total %d, %d chunks", idx.total, len(idx.offsets))
	}
	if s, e := idx.chunkRange(2); s != 2*PBS_FIXED_CHUNK_SIZE || e != size {
		t.Fatalf("last chunk %d-%d", s, e)
	}
	if idx.digests[1] != hex.EncodeToString(d[1][:]) {
		t.Fatal("digest 1")
	}
	r := &DIDXReaderAt{idx: idx}
	if ci := r.chunkIndexAt(PBS_FIXED_CHUNK_SIZE); ci != 1 {
		t.Fatalf("chunkIndexAt(4 MiB) = %d, want 1", ci)
	}
}

func TestParseFIDXIndexRejects(t *testing.T) {
	good := makeFIDX(PBS_FIXED_CHUNK_SIZE, PBS_FIXED_CHUNK_SIZE, [][32]byte{{}})
	if _, err := parseFIDXIndex("x", good[:100]); err == nil {
		t.Fatal("short header accepted")
	}
	bad := append([]byte(nil), good...)
	bad[0] = 0
	if _, err := parseFIDXIndex("x", bad); err == nil {
		t.Fatal("bad magic accepted (an HTML error page would look like this)")
	}
	if _, err := parseFIDXIndex("x", makeFIDX(2*PBS_FIXED_CHUNK_SIZE, PBS_FIXED_CHUNK_SIZE, [][32]byte{{}})); err == nil {
		t.Fatal("missing digests accepted")
	}
}
