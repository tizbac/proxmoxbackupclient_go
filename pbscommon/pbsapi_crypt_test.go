package pbscommon

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// capturedRequest holds what a fake PBS endpoint saw.
type capturedRequest struct {
	path       string
	query      url.Values
	body       []byte
	authHeader string
}

// newFakePBSServer records every request and replies 200 OK, so upload paths
// can be exercised without a real Proxmox Backup Server.
func newFakePBSServer(t *testing.T) (*httptest.Server, *[]capturedRequest) {
	t.Helper()
	var captured []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured = append(captured, capturedRequest{
			path:       r.URL.Path,
			query:      r.URL.Query(),
			body:       body,
			authHeader: r.Header.Get("Authorization"),
		})
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

func newTestClient(baseURL string, crypt *CryptConfig) *PBSClient {
	return &PBSClient{
		BaseURL:   baseURL,
		Namespace: "testns",
		Datastore: "teststore",
		Client:    *http.DefaultClient,
		Crypt:     crypt,
	}
}

func testCryptConfig(t *testing.T) *CryptConfig {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	cc, err := NewCryptConfig(key)
	if err != nil {
		t.Fatalf("NewCryptConfig: %v", err)
	}
	return cc
}

func TestChunkDigestFollowsCryptMode(t *testing.T) {
	pt := []byte("the quick brown fox jumps over the lazy dog")

	plain := newTestClient("http://example.invalid", nil)
	wantPlain := hex.EncodeToString(sha256Sum(pt))
	if got := plain.ChunkDigestHex(pt); got != wantPlain {
		t.Errorf("unencrypted ChunkDigestHex = %s, want sha256 %s", got, wantPlain)
	}

	enc := newTestClient("http://example.invalid", testCryptConfig(t))
	cc := enc.Crypt
	wantEnc := cc.ComputeDigestHex(pt)
	if got := enc.ChunkDigestHex(pt); got != wantEnc {
		t.Errorf("encrypted ChunkDigestHex = %s, want %s", got, wantEnc)
	}
	if wantEnc == wantPlain {
		t.Fatal("encrypted and unencrypted digests must differ for the same plaintext")
	}
	if sum, _ := hex.DecodeString(wantEnc); len(sum) != 32 {
		t.Fatalf("digest is not a 32-byte hex string: %q", wantEnc)
	}
}

func TestUploadChunkEncryptedFraming(t *testing.T) {
	srv, captured := newFakePBSServer(t)
	crypt := testCryptConfig(t)
	client := newTestClient(srv.URL, crypt)

	pt := bytes.Repeat([]byte("proxmox"), 4096) // 28672 bytes, compressible
	digest := client.ChunkDigestHex(pt)

	if err := client.UploadChunk(7, digest, pt, false, true); err != nil {
		t.Fatalf("UploadChunk: %v", err)
	}

	if len(*captured) != 1 {
		t.Fatalf("got %d requests, want 1", len(*captured))
	}
	got := (*captured)[0]

	if got.path != "/fixed_chunk" {
		t.Errorf("path = %s, want /fixed_chunk", got.path)
	}
	if !IsEncryptedBlob(got.body) {
		t.Fatalf("uploaded body is not an encrypted blob (magic % x)", got.body[:8])
	}
	if len(got.body) != EncryptedDataBlobHeaderSize+len(pt) {
		t.Errorf("body len = %d, want %d", len(got.body), EncryptedDataBlobHeaderSize+len(pt))
	}
	// The index must reference the *plaintext* size, the endpoint the
	// encoded size — this is what backup_writer.rs sends.
	if got.query.Get("size") != "28672" {
		t.Errorf("size = %s, want 28672 (plaintext length)", got.query.Get("size"))
	}
	if got.query.Get("encoded-size") != strconv.Itoa(len(got.body)) {
		t.Errorf("encoded-size = %s, want %d", got.query.Get("encoded-size"), len(got.body))
	}
	if got.query.Get("digest") != digest {
		t.Errorf("digest = %s, want %s", got.query.Get("digest"), digest)
	}

	// Decoding must yield the original plaintext back.
	back, err := DecodeEncryptedBlob(got.body, crypt, nil)
	if err != nil {
		t.Fatalf("DecodeEncryptedBlob: %v", err)
	}
	if !bytes.Equal(back, pt) {
		t.Error("decrypted chunk does not match the original plaintext")
	}
}

// An encrypted chunk upload must not silently fall back to the compressed
// framing even though the caller asked for compression.
func TestUploadChunkEncryptedIgnoresCompression(t *testing.T) {
	srv, captured := newFakePBSServer(t)
	client := newTestClient(srv.URL, testCryptConfig(t))

	pt := bytes.Repeat([]byte("A"), 1<<20)
	if err := client.UploadChunk(1, client.ChunkDigestHex(pt), pt, true, true); err != nil {
		t.Fatalf("UploadChunk: %v", err)
	}
	body := (*captured)[0].body
	if !IsEncryptedBlob(body) {
		t.Fatalf("expected encrypted framing, got magic % x", body[:8])
	}
	if bytes.Contains(body[:EncryptedDataBlobHeaderSize+64], []byte("AAAA")) {
		t.Error("plaintext leaked into the uploaded encrypted blob header region")
	}
}

func TestUploadBlobEncrypted(t *testing.T) {
	srv, captured := newFakePBSServer(t)
	crypt := testCryptConfig(t)
	client := newTestClient(srv.URL, crypt)

	blob := []byte("qemu-server.conf.blob payload")
	if err := client.UploadBlob("qemu-server.conf.blob", blob); err != nil {
		t.Fatalf("UploadBlob: %v", err)
	}

	got := (*captured)[0]
	if got.path != "/blob" {
		t.Fatalf("path = %s, want /blob", got.path)
	}
	if got.query.Get("file-name") != "qemu-server.conf.blob" {
		t.Errorf("file-name = %s", got.query.Get("file-name"))
	}
	if !IsEncryptedBlob(got.body) {
		t.Fatalf("blob body is not encrypted (magic % x)", got.body[:8])
	}

	dec, err := client.DownloadBlobFrom(got.body)
	if err != nil {
		t.Fatalf("decode blob: %v", err)
	}
	if !bytes.Equal(dec, blob) {
		t.Errorf("decoded blob = %q, want %q", dec, blob)
	}

	// The manifest entry must describe the encrypted form as stored.
	if len(client.Manifest.Files) != 1 {
		t.Fatalf("manifest files = %d, want 1", len(client.Manifest.Files))
	}
	f := client.Manifest.Files[0]
	if f.CryptMode != CryptModeEncrypt {
		t.Errorf("manifest crypt-mode = %q, want %q", f.CryptMode, CryptModeEncrypt)
	}
	if f.Size != int64(len(got.body)) {
		t.Errorf("manifest size = %d, want encoded size %d", f.Size, len(got.body))
	}
	if f.Csum != hex.EncodeToString(sha256Sum(got.body)) {
		t.Error("manifest csum must be the sha256 of the encoded blob, not the plaintext")
	}
}

func TestUploadBlobUnencryptedUnchanged(t *testing.T) {
	srv, captured := newFakePBSServer(t)
	client := newTestClient(srv.URL, nil)

	blob := []byte("acl sidecar")
	if err := client.UploadBlob("acl.bin", blob); err != nil {
		t.Fatalf("UploadBlob: %v", err)
	}
	got := (*captured)[0]
	if !bytes.Equal(got.body[:8], blobUncompressedMagic) {
		t.Fatalf("magic = % x, want % x", got.body[:8], blobUncompressedMagic)
	}
	if len(got.body) != DataBlobHeaderSize+len(blob) {
		t.Errorf("body len = %d, want %d", len(got.body), DataBlobHeaderSize+len(blob))
	}
	if client.Manifest.Files[0].CryptMode != CryptModeNone {
		t.Errorf("crypt-mode = %q, want none", client.Manifest.Files[0].CryptMode)
	}
}

// downloadBlobFromBytes is a test helper: UploadBlob's inverse for a body we
// captured off the wire, without needing a live snapshot.
func (pbs *PBSClient) DownloadBlobFrom(raw []byte) ([]byte, error) {
	return DecodeEncryptedBlob(raw, pbs.Crypt, nil)
}

func TestUploadManifestSignedWhenEncrypted(t *testing.T) {
	srv, captured := newFakePBSServer(t)
	crypt := testCryptConfig(t)
	client := newTestClient(srv.URL, crypt)

	client.Manifest.BackupType = "host"
	client.Manifest.BackupID = "e2ehost"
	client.Manifest.BackupTime = 1593179765
	client.Manifest.Files = []File{
		{CryptMode: CryptModeNone, Filename: "backup.pxar.didx", Size: 1},
		{CryptMode: CryptModeNone, Filename: "catalog.pcat1.didx", Size: 2},
	}

	if err := client.UploadManifest(); err != nil {
		t.Fatalf("UploadManifest: %v", err)
	}

	got := (*captured)[0]
	if got.query.Get("file-name") != ManifestBlobName {
		t.Fatalf("file-name = %s, want %s", got.query.Get("file-name"), ManifestBlobName)
	}
	// The manifest must never be encrypted, or nobody could read the
	// key fingerprint needed to decrypt anything else.
	if IsEncryptedBlob(got.body) {
		t.Fatal("index.json.blob must not be encrypted")
	}
	if !bytes.Equal(got.body[:8], blobUncompressedMagic) {
		t.Fatalf("manifest magic = % x", got.body[:8])
	}
	var stored map[string]any
	if err := json.Unmarshal(got.body[DataBlobHeaderSize:], &stored); err != nil {
		t.Fatalf("stored manifest is not valid JSON: %v", err)
	}
	if stored["backup-type"] != "host" || stored["backup-id"] != "e2ehost" {
		t.Errorf("stored identity = %v/%v, want host/e2ehost", stored["backup-type"], stored["backup-id"])
	}

	// Every archive is flagged as encrypted so the server/official client
	// knows its chunks need the key.
	files, _ := stored["files"].([]any)
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2", len(files))
	}
	for _, raw := range files {
		f, _ := raw.(map[string]any)
		if f["crypt-mode"] != CryptModeEncrypt {
			t.Errorf("%v crypt-mode = %v, want encrypt", f["filename"], f["crypt-mode"])
		}
	}

	// The fingerprint must be exposed so a restore can select the key.
	unprotected, _ := stored["unprotected"].(map[string]any)
	fp, _ := unprotected["key-fingerprint"].(string)
	if fp != crypt.Fingerprint() {
		t.Errorf("key-fingerprint = %q, want %q", fp, crypt.Fingerprint())
	}

	// And the signature must verify against the canonical JSON with
	// "signature" and "unprotected" removed.
	sig, _ := stored["signature"].(string)
	if sig == "" {
		t.Fatal("signature is empty")
	}
	canonical, err := canonicalJSONBytes(got.body[DataBlobHeaderSize:])
	if err != nil {
		t.Fatalf("canonicalJSONBytes: %v", err)
	}
	want := hex.EncodeToString(crypt.ComputeAuthTag(canonical))
	if sig != want {
		t.Errorf("signature = %s, want %s", sig, want)
	}
}

func TestUploadManifestUnencryptedHasNoSignature(t *testing.T) {
	srv, captured := newFakePBSServer(t)
	client := newTestClient(srv.URL, nil)

	client.Manifest.BackupType = "host"
	client.Manifest.BackupID = "plainhost"
	client.Manifest.Files = []File{{CryptMode: CryptModeNone, Filename: "backup.pxar.didx"}}

	if err := client.UploadManifest(); err != nil {
		t.Fatalf("UploadManifest: %v", err)
	}
	body := (*captured)[0].body[DataBlobHeaderSize:]

	var stored map[string]any
	if err := json.Unmarshal(body, &stored); err != nil {
		t.Fatalf("stored manifest is not valid JSON: %v", err)
	}
	if got, ok := stored["signature"]; ok && got != nil && !strings.EqualFold(toStr(got), "") {
		t.Errorf("signature = %v, want nil for an unencrypted snapshot", got)
	}
	unprotected, _ := stored["unprotected"].(map[string]any)
	if _, present := unprotected["key-fingerprint"]; present {
		t.Error("key-fingerprint must be absent for an unencrypted snapshot")
	}
	files, _ := stored["files"].([]any)
	f0, _ := files[0].(map[string]any)
	if f0["crypt-mode"] != CryptModeNone {
		t.Errorf("crypt-mode = %v, want none", f0["crypt-mode"])
	}
}

func toStr(v any) string {
	s, _ := v.(string)
	return s
}

// PBS re-serializes index.json.blob through its own BackupManifest struct at
// /finish, dropping any key that struct does not declare, and the official
// client verifies the signature against that re-serialized shape. If we ever
// serialize an extra field, encrypted restores break with "wrong signature in
// manifest" while every local test still passes - so pin the exact key set.
func TestManifestSignedShapeMatchesProxmoxBackup(t *testing.T) {
	m := BackupManifest{
		BackupID:   "e2e",
		BackupTime: 1593179765,
		BackupType: "host",
		Comment:    "a comment PBS will silently drop",
		Files: []File{{
			CryptMode: CryptModeEncrypt,
			Csum:      "0101",
			Filename:  "drive-sata0.img.fidx",
			Size:      200,
		}},
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	want := map[string]bool{
		"backup-id": true, "backup-time": true, "backup-type": true,
		"files": true, "signature": true, "unprotected": true,
	}
	for k := range stored {
		if !want[k] {
			t.Errorf("manifest serializes key %q, which proxmox-backup's BackupManifest does not declare "+
				"(PBS drops it at /finish, so the signature would never verify)", k)
		}
	}
	for k := range want {
		if _, ok := stored[k]; !ok {
			t.Errorf("manifest is missing key %q", k)
		}
	}
	files, _ := stored["files"].([]any)
	f0, _ := files[0].(map[string]any)
	for k := range f0 {
		switch k {
		case "crypt-mode", "csum", "filename", "size":
		default:
			t.Errorf("file entry serializes key %q, which proxmox-backup's FileInfo does not declare", k)
		}
	}
}
