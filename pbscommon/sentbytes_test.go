package pbscommon

import (
	"bytes"
	"testing"
)

// SentBytes counts each uploaded chunk as it went to the server: compressed
// when unencrypted, the encrypted blob otherwise.
func TestSentBytesCountsEncodedSize(t *testing.T) {
	pt := bytes.Repeat([]byte("A"), 1<<20)
	for _, tc := range []struct {
		name  string
		crypt *CryptConfig
	}{{"plain", nil}, {"encrypted", testCryptConfig(t)}} {
		t.Run(tc.name, func(t *testing.T) {
			srv, captured := newFakePBSServer(t)
			client := newTestClient(srv.URL, tc.crypt)
			if got := client.SentBytes(); got != 0 {
				t.Fatalf("SentBytes before any upload = %d, want 0", got)
			}
			for i := 0; i < 2; i++ {
				if err := client.UploadChunk(1, client.ChunkDigestHex(pt), pt, true, true); err != nil {
					t.Fatalf("UploadChunk: %v", err)
				}
			}
			want := uint64(len((*captured)[0].body) + len((*captured)[1].body))
			if got := client.SentBytes(); got != want {
				t.Errorf("SentBytes = %d, want %d (the two bodies sent)", got, want)
			}
			if tc.crypt == nil && client.SentBytes() >= uint64(len(pt)) {
				t.Errorf("SentBytes = %d for two compressible 1 MiB chunks, want far less than %d", client.SentBytes(), len(pt))
			}
		})
	}
}
