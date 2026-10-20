package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alphadose/haxmap"
	"pbscommon"
)

// The GUI/service directory backup must publish an encrypted snapshot's chunks
// under sha256(plaintext || id_key), like the CLI and proxmox-backup-client.
// It used the bare sha256, which the official client rejects with "wrong
// digest".
func TestChunkStateEncryptedDigests(t *testing.T) {
	encKey, err := pbscommon.GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("GenerateEncryptionKey: %v", err)
	}
	crypt, err := pbscommon.NewCryptConfig(encKey)
	if err != nil {
		t.Fatalf("NewCryptConfig: %v", err)
	}

	type upload struct {
		digest string
		body   []byte
	}
	var mu sync.Mutex
	var uploads []upload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path == "/dynamic_chunk" {
			mu.Lock()
			uploads = append(uploads, upload{digest: r.URL.Query().Get("digest"), body: body})
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := &pbscommon.PBSClient{
		BaseURL:   srv.URL,
		Datastore: "teststore",
		Client:    *http.DefaultClient,
		Crypt:     crypt,
	}

	// Several average-size (4 MiB) chunks, so both HandleData's cut points and
	// EOF's remainder are exercised.
	data := make([]byte, 12*1024*1024)
	rand.New(rand.NewSource(13)).Read(data)

	var newchunk, reusechunk, failedchunk, totalSize atomic.Uint64
	c := ChunkState{}
	c.Init(&newchunk, &reusechunk, &failedchunk, haxmap.New[string, bool](), nil, &totalSize, nil, "test")
	if err := c.HandleData(data, client); err != nil {
		t.Fatalf("HandleData: %v", err)
	}
	if err := c.EOF(client); err != nil {
		t.Fatalf("EOF: %v", err)
	}

	// The handler goroutines wrote uploads; take the lock so -race sees the
	// hand-over.
	mu.Lock()
	defer mu.Unlock()
	if len(uploads) < 2 {
		t.Fatalf("got %d chunk uploads, want at least 2", len(uploads))
	}
	var restored []byte
	for i, u := range uploads {
		digest, err := hex.DecodeString(u.digest)
		if err != nil {
			t.Fatalf("upload %d: bad digest %q: %v", i, u.digest, err)
		}
		// DecodeEncryptedBlob checks the digest as sha256(plaintext || id_key).
		plaintext, err := pbscommon.DecodeEncryptedBlob(u.body, crypt, digest)
		if err != nil {
			t.Fatalf("upload %d: %v", i, err)
		}
		if bare := sha256.Sum256(plaintext); hex.EncodeToString(bare[:]) == u.digest {
			t.Errorf("upload %d is indexed under the bare sha256", i)
		}
		if c.assignments[i] != u.digest {
			t.Errorf("assignment %d = %s, want the uploaded digest %s", i, c.assignments[i], u.digest)
		}
		restored = append(restored, plaintext...)
	}
	if string(restored) != string(data) {
		t.Error("the uploaded chunks do not reassemble into the input")
	}
}
