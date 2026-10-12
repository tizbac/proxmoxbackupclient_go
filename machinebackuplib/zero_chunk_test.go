package machinebackuplib

import (
	"bytes"
	"testing"

	"pbscommon"
)

// The short cut must give exactly the digest hashing gives, for plain and
// encrypted clients, and must hash anything that is not a full chunk of
// zeros: one non-zero byte anywhere, or a short (last) chunk.
func TestZeroChunkDetector(t *testing.T) {
	crypt, err := pbscommon.NewCryptConfig(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range []*pbscommon.PBSClient{{}, {Crypt: crypt}} {
		z := newZeroChunkDetector(client)
		hashed := 0
		hash := func(b []byte) [32]byte { hashed++; return client.ChunkDigest(b) }

		blank := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
		if got := z.digestOf(blank, hash); got != client.ChunkDigest(blank) || hashed != 0 {
			t.Fatalf("blank chunk: digest differs or was hashed (%d)", hashed)
		}
		for _, at := range []int{0, 12345, len(blank) - 1} {
			b := make([]byte, len(blank))
			b[at] = 1
			if got := z.digestOf(b, hash); got != client.ChunkDigest(b) {
				t.Errorf("byte at %d set: wrong digest", at)
			}
		}
		short := make([]byte, 4096)
		if got := z.digestOf(short, hash); got != client.ChunkDigest(short) {
			t.Error("short chunk: wrong digest")
		}
		if hashed != 4 {
			t.Errorf("hashed %d times, want 4", hashed)
		}
	}
}

func BenchmarkBlankChunkDigest(b *testing.B) {
	client := &pbscommon.PBSClient{}
	z := newZeroChunkDetector(client)
	blank := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
	b.SetBytes(int64(len(blank)))
	b.Run("detector", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			z.digestOf(blank, client.ChunkDigest)
		}
	})
	b.Run("sha256", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			client.ChunkDigest(blank)
		}
	})
}
