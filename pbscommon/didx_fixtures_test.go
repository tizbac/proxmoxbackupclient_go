package pbscommon

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Test fixtures: synthetic .didx archives plus a fake PBS that serves them.
// ---------------------------------------------------------------------------

// didxArchive is a synthetic dynamic index plus the chunk payloads it points at.
type didxArchive struct {
	didx    []byte            // the .didx file body
	chunks  map[string][]byte // hex digest -> plaintext chunk
	order   []string          // hex digests, in archive order
	offsets []uint64          // cumulative end offsets, in archive order
}

// chunkDigest returns the digest PBS would record for a chunk: a plain SHA-256
// for an unencrypted snapshot, and the id_key-salted digest when encrypted.
func chunkDigest(crypt *CryptConfig, chunk []byte) string {
	if crypt == nil {
		sum := sha256.Sum256(chunk)
		return hex.EncodeToString(sum[:])
	}
	return crypt.ComputeDigestHex(chunk)
}

// makeDIDX builds a .didx body and the matching chunks for the given payloads.
func makeDIDX(t *testing.T, crypt *CryptConfig, payloads ...[]byte) *didxArchive {
	t.Helper()
	a := &didxArchive{
		didx:   make([]byte, didxHeaderSize, didxHeaderSize+len(payloads)*didxEntrySize),
		chunks: map[string][]byte{},
	}
	copy(a.didx, didxMagic)

	var off uint64
	for _, p := range payloads {
		off += uint64(len(p))
		d := chunkDigest(crypt, p)

		var entry [didxEntrySize]byte
		binary.LittleEndian.PutUint64(entry[:8], off)
		raw, err := hex.DecodeString(d)
		if err != nil {
			t.Fatalf("bad digest %q: %v", d, err)
		}
		copy(entry[8:], raw)
		a.didx = append(a.didx, entry[:]...)

		a.chunks[d] = p
		a.order = append(a.order, d)
		a.offsets = append(a.offsets, off)
	}
	return a
}

// mustEncodeBlob wraps a payload in a PBS DataBlob, encrypted when crypt != nil.
func mustEncodeBlob(t *testing.T, payload []byte, crypt *CryptConfig) []byte {
	t.Helper()
	if crypt != nil {
		raw, err := crypt.EncodeEncrypted(payload)
		if err != nil {
			t.Fatalf("encode encrypted blob: %v", err)
		}
		return raw
	}
	out := make([]byte, DataBlobHeaderSize+len(payload))
	copy(out[:8], blobUncompressedMagic)
	binary.LittleEndian.PutUint32(out[8:12], crc32.ChecksumIEEE(payload))
	copy(out[DataBlobHeaderSize:], payload)
	return out
}

// fakePBSWithDIDX serves one archive: GET /download?file-name=<name> returns the
// .didx body, GET /chunk?digest=<hex> returns the stored DataBlob. It counts
// chunk fetches so cache behaviour can be asserted.
type fakePBSWithDIDX struct {
	*PBSClient
	chunkFetches map[string]int
	mu           sync.Mutex

	// overrideDidx, when set, is served instead of the real index (malformed-index tests).
	overrideDidx []byte
	// corrupt maps a digest to replacement plaintext, to exercise integrity checks.
	corrupt map[string][]byte
	// manifest, when set, is served as index.json.blob (a plain DataBlob);
	// without it the manifest is missing, as for the tests written before
	// the key check (CheckArchiveKey then lets the reader open).
	manifest []byte
}

// setManifest serves m as the snapshot's index.json.blob.
func (f *fakePBSWithDIDX) setManifest(t *testing.T, m BackupManifest) {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	f.manifest = mustEncodeBlob(t, raw, nil)
}

// readerTestClient returns srv's HTTP client with room to keep a connection
// per prefetch slot. With the default of two idle connections per host,
// every parallel chunk fetch beyond two dialled a new connection and closed
// it straight after: about 4000 per run of the prefetch window test, each
// holding a local port for two minutes on Windows (TIME_WAIT), until a run
// with -count=30 failed all over the package with "Only one usage of each
// socket address". Real reader sessions use one HTTP/2 connection.
func readerTestClient(srv *httptest.Server) http.Client {
	c := srv.Client()
	c.Transport.(*http.Transport).MaxIdleConnsPerHost = 4 * chunkPrefetchConcurrency
	return *c
}

func newFakePBSWithDIDX(t *testing.T, name string, a *didxArchive, crypt *CryptConfig) *fakePBSWithDIDX {
	t.Helper()
	f := &fakePBSWithDIDX{
		chunkFetches: map[string]int{},
		corrupt:      map[string][]byte{},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download":
			if r.URL.Query().Get("file-name") == ManifestBlobName && f.manifest != nil {
				w.Write(f.manifest)
				return
			}
			if r.URL.Query().Get("file-name") != name {
				http.Error(w, "no such archive", http.StatusNotFound)
				return
			}
			if f.overrideDidx != nil {
				w.Write(f.overrideDidx)
				return
			}
			w.Write(a.didx)
		case "/chunk":
			d := r.URL.Query().Get("digest")
			f.mu.Lock()
			f.chunkFetches[d]++
			f.mu.Unlock()
			payload, ok := a.chunks[d]
			if !ok {
				http.Error(w, "chunk not found", http.StatusNotFound)
				return
			}
			if repl, ok := f.corrupt[d]; ok {
				payload = repl
			}
			w.Write(mustEncodeBlob(t, payload, crypt))
		default:
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	f.PBSClient = &PBSClient{
		BaseURL:   srv.URL,
		Datastore: "teststore",
		Client:    readerTestClient(srv),
		Crypt:     crypt,
	}
	return f
}

func (f *fakePBSWithDIDX) fetchCount(digest string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.chunkFetches[digest]
}

func (f *fakePBSWithDIDX) totalFetches() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.chunkFetches {
		n += c
	}
	return n
}

// payloadsOf builds n distinct chunks of the given size.
func payloadsOf(n, size int) [][]byte {
	out := make([][]byte, n)
	for i := range out {
		p := make([]byte, size)
		for j := range p {
			p[j] = byte(i*7 + j)
		}
		out[i] = p
	}
	return out
}
