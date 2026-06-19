package pbscommon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cornelk/hashmap"
	"github.com/klauspost/compress/zstd"
	"golang.org/x/net/http2"
)

// CompressionLevel defines the Zstd compression level
type CompressionLevel string

const (
	// CompressionFastest - Fastest compression, lower CPU usage (~30-40% faster)
	CompressionFastest CompressionLevel = "fastest"
	// CompressionDefault - Balanced compression/speed (zstd default level 3)
	CompressionDefault CompressionLevel = "default"
	// CompressionBetter - Better compression ratio, higher CPU usage
	CompressionBetter CompressionLevel = "better"
	// CompressionBest - Best compression ratio, highest CPU usage
	CompressionBest CompressionLevel = "best"
)

// ParseCompressionLevel converts a string to CompressionLevel
// Returns CompressionFastest if invalid or empty
func ParseCompressionLevel(level string) CompressionLevel {
	switch strings.ToLower(level) {
	case "fastest", "fast":
		return CompressionFastest
	case "default", "":
		return CompressionDefault
	case "better":
		return CompressionBetter
	case "best", "max":
		return CompressionBest
	default:
		return CompressionFastest // Default to fastest
	}
}

type IndexCreateResp struct {
	WriterID int `json:"data"`
}

type IndexPutReq struct {
	DigestList []string `json:"digest-list"`
	OffsetList []uint64 `json:"offset-list"`
	WriterID   uint64   `json:"wid"`
}

type IndexCloseReq struct {
	ChunkCount uint64 `json:"chunk-count"`
	CheckSum   string `json:"csum"`
	Size       uint64 `json:"size"`
	WriterID   uint64 `json:"wid"`
}

type File struct {
	CryptMode string `json:"crypt-mode"`
	Csum      string `json:"csum"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
}

type ChunkUploadStats struct {
	CompressedSize int64 `json:"compressed_size"`
	Count          int   `json:"count"`
	Duplicates     int   `json:"duplicates"`
	Size           int64 `json:"size"`
}

type FixedIndexCreateReq struct {
	ArchiveName string `json:"archive-name"`
	Size        int64  `json:"size"`
}

type Unprotected struct {
	ChunkUploadStats ChunkUploadStats `json:"chunk_upload_stats"`
}

type BackupManifest struct {
	BackupID    string      `json:"backup-id"`
	BackupTime  int64       `json:"backup-time"`
	BackupType  string      `json:"backup-type"`
	Files       []File      `json:"files"`
	Signature   interface{} `json:"signature"`
	Unprotected Unprotected `json:"unprotected"`
}

type AuthErr struct {
	StatusCode   string
	ResponseBody string
}

func (e *AuthErr) Error() string {
	return fmt.Sprintf("PBS authentication failed: HTTP %s - %s", e.StatusCode, e.ResponseBody)
}

type PBSClient struct {
	BaseURL         string
	CertFingerPrint string
	APIToken        string
	Secret          string
	AuthID          string

	Datastore string
	Namespace string
	Manifest  BackupManifest

	Insecure bool

	Client    http.Client
	TLSConfig tls.Config

	WritersManifest  map[uint64]int
	SkippedFiles     []string         // ALL skips (errors + system auto-excludes + junctions) — for display
	ExcludedFiles    []string         // Track files/dirs excluded by user policy (H-04)
	ReadErrors       []string         // Outcome-affecting read failures + content instability (v2-H-02)
	CompressionLevel CompressionLevel // Zstd compression level (default: fastest)

	// activeConn is the raw TLS socket underlying the HTTP/2 transport for
	// this backup session. Close() uses it to force-terminate the connection
	// so PBS releases the writer / snapshot lock immediately, without waiting
	// on OS TCP keepalive (10+ min) — http2.Transport.CloseIdleConnections()
	// alone is a no-op while streams are open.
	activeConn   net.Conn
	activeConnMu sync.Mutex
}

// activeClients tracks every PBSClient currently holding an open HTTP/2
// session. A signal handler or Wails shutdown hook calls CloseAllActive
// to release writers before the process exits; otherwise the PBS-side
// snapshot lock lingers and blocks the next verify run on that group.
var (
	activeClientsMu sync.Mutex
	activeClients   = map[*PBSClient]struct{}{}
)

func registerActive(c *PBSClient) {
	activeClientsMu.Lock()
	activeClients[c] = struct{}{}
	activeClientsMu.Unlock()
}

func unregisterActive(c *PBSClient) {
	activeClientsMu.Lock()
	delete(activeClients, c)
	activeClientsMu.Unlock()
}

// CloseAllActive force-closes every live PBS backup session. Safe to
// call from a signal handler.
func CloseAllActive() {
	activeClientsMu.Lock()
	clients := make([]*PBSClient, 0, len(activeClients))
	for c := range activeClients {
		clients = append(clients, c)
	}
	activeClientsMu.Unlock()
	for _, c := range clients {
		c.Close()
	}
}

const PBS_FIXED_CHUNK_SIZE = 4 * 1024 * 1024

var blobCompressedMagic = []byte{49, 185, 88, 66, 111, 182, 163, 127}
var blobUncompressedMagic = []byte{66, 171, 56, 7, 190, 131, 112, 161}

type SnapshotsResp struct {
	Data []BackupManifest `json:"data"`
}

// buildTLSConfig returns the TLS config used by ALL PBS HTTP clients. When a
// fingerprint is configured (pbs.CertFingerPrint != ""), CA validation is skipped but the peer
// certificate's SHA-256 is pinned to CertFingerPrint; otherwise normal CA
// validation applies. Factored so every client that sends the auth token pins
// identically (audit H-02) — ListSnapshots/TestConnection previously skipped the
// pin (InsecureSkipVerify with no fingerprint check), accepting any certificate.
func (pbs *PBSClient) buildTLSConfig() *tls.Config {
	// Pinning is keyed on whether a fingerprint is configured, NOT on pbs.Insecure:
	// some callers (e.g. nbd) set Insecure with no fingerprint and must keep working
	// (skip verification), and a fingerprint-less pin would compare against "" and
	// fail every connection.
	if pbs.CertFingerPrint == "" {
		// No fingerprint: trust the system CA, unless explicitly insecure.
		return &tls.Config{InsecureSkipVerify: pbs.Insecure}
	}
	// Fingerprint configured: skip CA validation but pin the peer cert's SHA-256.
	return &tls.Config{
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return fmt.Errorf("no certificates presented by the peer")
			}
			peerCert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return fmt.Errorf("failed to parse certificate: %v", err)
			}
			expectedFingerprint := strings.ReplaceAll(pbs.CertFingerPrint, ":", "")
			sum := sha256.Sum256(peerCert.Raw)
			calculatedFingerprint := hex.EncodeToString(sum[:])
			if !strings.EqualFold(calculatedFingerprint, expectedFingerprint) {
				return fmt.Errorf("certificate fingerprint does not match (expected %s, got %s)", expectedFingerprint, calculatedFingerprint)
			}
			return nil
		},
	}
}

// FetchServerFingerprint dials baseURL over TLS without verifying the chain and
// returns the leaf certificate's SHA-256 fingerprint as colon-separated uppercase
// hex pairs (AA:BB:...), the format PBS displays and ValidateFingerprint accepts.
// Used for trust-on-first-use pinning when a server presents a self-signed
// certificate and no fingerprint is configured yet (audit H-02 made CA validation
// strict, so such servers now fail TestConnection with x509 unknown-authority).
// It performs no authentication and sends no token — it only inspects the
// certificate the server presents.
func FetchServerFingerprint(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	host := u.Host
	if u.Port() == "" {
		host = net.JoinHostPort(u.Hostname(), "8007")
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	// #nosec G402 -- intentionally unverified: we only read the presented cert to
	// compute its fingerprint for the user to pin (TOFU); no data is exchanged.
	conn, err := tls.DialWithDialer(dialer, "tcp", host, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return "", fmt.Errorf("failed to reach server: %w", err)
	}
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", fmt.Errorf("server presented no certificate")
	}
	sum := sha256.Sum256(certs[0].Raw)
	pairs := make([]string, len(sum))
	for i, b := range sum {
		pairs[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(pairs, ":"), nil
}

func (pbs *PBSClient) ListSnapshots() ([]BackupManifest, error) {
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: pbs.buildTLSConfig()},
	}

	ret := make([]BackupManifest, 0)
	var r SnapshotsResp
	params := url.Values{}
	params.Add("ns", pbs.Namespace)
	fullURL := fmt.Sprintf("%s/api2/json/admin/datastore/%s/snapshots?%s", pbs.BaseURL, pbs.Datastore, params.Encode())

	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return ret, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s", pbs.AuthID, pbs.Secret))
	resp, err := client.Do(req)
	if err != nil {
		return ret, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return ret, fmt.Errorf("HTTP error: %d - %s", resp.StatusCode, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return ret, err
	}
	return r.Data, nil

}

func (pbs *PBSClient) CreateFixedIndex(fic FixedIndexCreateReq) (uint64, error) {
	jd, err := json.Marshal(fic)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest("POST", pbs.BaseURL+"/fixed_index", bytes.NewBuffer(jd))
	if err != nil {
		return 0, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s", pbs.AuthID, pbs.Secret))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp2, err := pbs.Client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return 0, err
	}

	if resp2.StatusCode != http.StatusOK {
		resp1, _ := io.ReadAll(resp2.Body)
		fmt.Println("Error making request:", string(resp1), string(resp2.Proto))
		return 0, fmt.Errorf("request failed: %s %s", string(resp1), resp2.Proto)
	}

	resp1, err := io.ReadAll(resp2.Body)
	if err != nil {
		return 0, err
	}
	var R IndexCreateResp
	err = json.Unmarshal(resp1, &R)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return 0, err
	}
	fmt.Println("Writer id: ", R.WriterID)
	defer resp2.Body.Close()
	f := File{
		CryptMode: "none",
		Csum:      "",
		Filename:  fic.ArchiveName,
		Size:      0,
	}
	pbs.Manifest.Files = append(pbs.Manifest.Files, f)
	pbs.WritersManifest[uint64(R.WriterID)] = len(pbs.Manifest.Files) - 1
	return uint64(R.WriterID), nil

}

func (pbs *PBSClient) AssignFixedChunks(writerid uint64, digests []string, offsets []uint64) error {
	indexput := &IndexPutReq{
		WriterID:   writerid,
		DigestList: digests,
		OffsetList: offsets,
	}

	jsondata, err := json.Marshal(indexput)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", pbs.BaseURL+"/fixed_index", bytes.NewBuffer(jsondata))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	resp2, err := pbs.Client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return err
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("PBS assign fixed chunks failed: HTTP %d - %s", resp2.StatusCode, string(bodyBytes))
	}

	return nil
}

func (pbs *PBSClient) CloseFixedIndex(writerid uint64, checksum string, totalsize uint64, chunkcount uint64) error {
	finishreq := &IndexCloseReq{
		WriterID:   writerid,
		CheckSum:   checksum,
		Size:       totalsize,
		ChunkCount: chunkcount,
	}
	jsonpayload, err := json.Marshal(finishreq)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", pbs.BaseURL+"/fixed_close", bytes.NewBuffer(jsonpayload))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s", pbs.AuthID, pbs.Secret))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp2, err := pbs.Client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return err
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("PBS close fixed index failed: HTTP %d - %s", resp2.StatusCode, string(bodyBytes))
	}

	f := &pbs.Manifest.Files[pbs.WritersManifest[writerid]]
	f.Csum = checksum
	f.Size = int64(totalsize)

	return nil
}

// redactedHeaders returns a copy of h with the Authorization value masked, so
// request headers can be logged without leaking the PBS API token (audit L-01).
func redactedHeaders(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for k, v := range h {
		if k == "Authorization" {
			out[k] = []string{"PBSAPIToken=<redacted>"}
			continue
		}
		out[k] = v
	}
	return out
}

func (pbs *PBSClient) CreateDynamicIndex(name string) (uint64, error) {
	fmt.Printf("=== CreateDynamicIndex START ===\n")
	fmt.Printf("Archive name: %s\n", name)
	fmt.Printf("BaseURL: %s\n", pbs.BaseURL)

	// Use json.Marshal to properly escape special characters and ensure valid JSON
	payload := map[string]string{"archive-name": name}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("ERROR: Failed to marshal JSON: %v\n", err)
		return 0, fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := http.NewRequest("POST", pbs.BaseURL+"/dynamic_index", bytes.NewBuffer(jsonPayload))
	if err != nil {
		fmt.Printf("ERROR: Failed to create request: %v\n", err)
		return 0, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s", pbs.AuthID, pbs.Secret))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	fmt.Printf("Sending POST request to: %s\n", req.URL.String())
	fmt.Printf("Headers: %+v\n", redactedHeaders(req.Header))

	resp2, err := pbs.Client.Do(req)
	if err != nil {
		fmt.Printf("ERROR: HTTP request failed: %v\n", err)
		fmt.Printf("ERROR: Error type: %T\n", err)
		return 0, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp2.Body.Close()

	fmt.Printf("Response status: %d %s\n", resp2.StatusCode, resp2.Status)
	fmt.Printf("Response proto: %s\n", resp2.Proto)

	if resp2.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp2.Body)
		fmt.Printf("ERROR: PBS returned non-200 status\n")
		fmt.Printf("Status: %d\n", resp2.StatusCode)
		fmt.Printf("Body: %s\n", string(bodyBytes))
		return 0, fmt.Errorf("PBS returned HTTP %d: %s", resp2.StatusCode, string(bodyBytes))
	}

	resp1, err := io.ReadAll(resp2.Body)
	if err != nil {
		return 0, err
	}
	var R IndexCreateResp
	err = json.Unmarshal(resp1, &R)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return 0, err
	}
	fmt.Println("Writer id: ", R.WriterID)
	defer resp2.Body.Close()
	f := File{
		CryptMode: "none",
		Csum:      "",
		Filename:  name,
		Size:      0,
	}
	pbs.Manifest.Files = append(pbs.Manifest.Files, f)
	pbs.WritersManifest[uint64(R.WriterID)] = len(pbs.Manifest.Files) - 1
	return uint64(R.WriterID), nil
}

func (pbs *PBSClient) UploadDynamicUncompressedChunk(writerid uint64, digest string, chunkdata []byte) error {
	return pbs.UploadChunk(writerid, digest, chunkdata, true, false)
}
func (pbs *PBSClient) UploadFixedUncompressedChunk(writerid uint64, digest string, chunkdata []byte) error {
	return pbs.UploadChunk(writerid, digest, chunkdata, false, false)
}
func (pbs *PBSClient) UploadDynamicCompressedChunk(writerid uint64, digest string, chunkdata []byte) error {
	return pbs.UploadChunk(writerid, digest, chunkdata, true, true)
}
func (pbs *PBSClient) UploadFixedCompressedChunk(writerid uint64, digest string, chunkdata []byte) error {
	return pbs.UploadChunk(writerid, digest, chunkdata, false, true)
}

func (pbs *PBSClient) UploadChunk(writerid uint64, digest string, chunkdata []byte, dynamic bool, compressed bool) error {
	outBuffer := make([]byte, 0)
	if compressed {
		outBuffer = append(outBuffer, blobCompressedMagic...)
		compressedData := make([]byte, 0)

		// Select compression level based on client configuration
		var encoderLevel zstd.EncoderLevel
		switch pbs.CompressionLevel {
		case CompressionFastest:
			encoderLevel = zstd.SpeedFastest
		case CompressionBetter:
			encoderLevel = zstd.SpeedBetterCompression
		case CompressionBest:
			encoderLevel = zstd.SpeedBestCompression
		default: // CompressionDefault or empty
			encoderLevel = zstd.SpeedDefault
		}

		w, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(encoderLevel))
		compressedData = w.EncodeAll(chunkdata, compressedData)
		checksum := crc32.Checksum(compressedData, crc32.IEEETable)
		//binary.Write(outBuffer, binary.LittleEndian, checksum)
		outBuffer = binary.LittleEndian.AppendUint32(outBuffer, checksum)

		//fmt.Printf("Appended checksum %08x , len: %d\n", checksum, len(outBuffer))

		outBuffer = append(outBuffer, compressedData...)

		if len(compressedData) > len(chunkdata) {
			return pbs.UploadChunk(writerid, digest, chunkdata, dynamic, false)
		}
	} else {
		outBuffer = append(outBuffer, blobUncompressedMagic...)
		checksum := crc32.Checksum(chunkdata, crc32.IEEETable)
		outBuffer = binary.LittleEndian.AppendUint32(outBuffer, checksum)
		outBuffer = append(outBuffer, chunkdata...)
	}

	//fmt.Printf("Compressed: %d , Orig: %d\n", len(compressedData), len(chunkdata))

	q := &url.Values{}
	q.Add("digest", digest)
	q.Add("encoded-size", fmt.Sprintf("%d", len(outBuffer)))
	q.Add("size", fmt.Sprintf("%d", len(chunkdata)))
	q.Add("wid", fmt.Sprintf("%d", writerid))
	suburl := "/dynamic_chunk?"
	if !dynamic {
		suburl = "/fixed_chunk?"
	}
	req, err := http.NewRequest("POST", pbs.BaseURL+suburl+q.Encode(), bytes.NewBuffer(outBuffer))
	if err != nil {
		fmt.Println("Error making request:", err)
		return err
	}
	resp2, err := pbs.Client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return err
	}

	if resp2.StatusCode != http.StatusOK {
		resp1, _ := io.ReadAll(resp2.Body)
		fmt.Println("Error making request:", string(resp1), string(resp2.Proto))
		return fmt.Errorf("Error making request: %s %s", string(resp1), string(resp2.Proto))
	}

	return nil
}

func (pbs *PBSClient) AssignDynamicChunks(writerid uint64, digests []string, offsets []uint64) error {
	indexput := &IndexPutReq{
		WriterID:   writerid,
		DigestList: digests,
		OffsetList: offsets,
	}

	jsondata, err := json.Marshal(indexput)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", pbs.BaseURL+"/dynamic_index", bytes.NewBuffer(jsondata))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	resp2, err := pbs.Client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return err
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("PBS assign chunks failed: HTTP %d - %s", resp2.StatusCode, string(bodyBytes))
	}

	return nil
}

func (pbs *PBSClient) CloseDynamicIndex(writerid uint64, checksum string, totalsize uint64, chunkcount uint64) error {
	finishreq := &IndexCloseReq{
		WriterID:   writerid,
		CheckSum:   checksum,
		Size:       totalsize,
		ChunkCount: chunkcount,
	}
	jsonpayload, err := json.Marshal(finishreq)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", pbs.BaseURL+"/dynamic_close", bytes.NewBuffer(jsonpayload))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s", pbs.AuthID, pbs.Secret))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp2, err := pbs.Client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return err
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("PBS close dynamic index failed: HTTP %d - %s", resp2.StatusCode, string(bodyBytes))
	}

	f := &pbs.Manifest.Files[pbs.WritersManifest[writerid]]
	f.Csum = checksum
	f.Size = int64(totalsize)

	return nil
}

func (pbs *PBSClient) UploadBlob(name string, data []byte) error {
	out := make([]byte, 0)
	out = append(out, blobUncompressedMagic...)

	checksum := crc32.ChecksumIEEE(data)
	out = binary.LittleEndian.AppendUint32(out, checksum)
	out = append(out, data...)

	q := &url.Values{}
	q.Add("encoded-size", fmt.Sprintf("%d", len(out)))
	q.Add("file-name", name)

	req, _ := http.NewRequest("POST", pbs.BaseURL+"/blob?"+q.Encode(), bytes.NewBuffer(out))

	resp2, err := pbs.Client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return err
	}

	if resp2.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("PBS upload blob failed: HTTP %d - %s", resp2.StatusCode, string(bodyBytes))
	}

	// Manifest csum must match the SHA-256 of the blob file as stored
	// on the datastore — i.e. the encoded buffer (magic + crc32 +
	// payload) we just sent. PBS validates this field on /finish as a
	// strict 64-char hex string; leaving it empty triggers
	// "unable to update manifest blob - Invalid string length".
	//
	// Size must match DataBlob::raw_size() used by PBS verify, which
	// returns raw_data.len() — i.e. the full on-disk file size with
	// the 12-byte DataBlobHeader, not just the inner payload. Using
	// len(data) produces "wrong size (len(data) != len(data)+12)" at
	// verify time even though /finish accepts it.
	//
	// Historically this never blew up because the only UploadBlob
	// caller was UploadManifest, which appends the index.json.blob
	// entry AFTER the manifest JSON has already been serialized — so
	// the stored manifest never referenced itself with a broken csum.
	// As soon as another blob (the NTFS ACL side-car) is uploaded
	// before the manifest is serialized, its broken entry lands in
	// the persisted JSON and /finish rejects it.
	sum := sha256.Sum256(out)
	pbs.Manifest.Files = append(pbs.Manifest.Files, File{
		CryptMode: "none",
		Csum:      hex.EncodeToString(sum[:]),
		Filename:  name,
		Size:      int64(len(out)),
	})

	return nil
}

func (pbs *PBSClient) UploadManifest() error {
	manifestBin, err := json.Marshal(pbs.Manifest)
	if err != nil {
		return err
	}
	return pbs.UploadBlob("index.json.blob", manifestBin)
}

func (pbs *PBSClient) Finish() error {
	req, err := http.NewRequest("POST", pbs.BaseURL+"/finish", nil)
	if err != nil {
		return fmt.Errorf("failed to create finish request: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s", pbs.AuthID, pbs.Secret))

	resp2, err := pbs.Client.Do(req)
	if err != nil {
		return fmt.Errorf("finish request failed: %w", err)
	}
	defer resp2.Body.Close()

	// CRITICAL: Check HTTP status code
	if resp2.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("PBS finish failed: HTTP %d - %s", resp2.StatusCode, string(bodyBytes))
	}

	// Session committed. Tear the H2 connection down right away so PBS
	// drops the worker task and releases the snapshot lock — otherwise
	// the task UI keeps it active until the process exits or TCP
	// keepalive reaps the socket (~16 min), which blocks the next
	// scheduled run with "retry lock error".
	pbs.Close()

	return nil
}

// TestConnection performs a real HTTP request to verify PBS connectivity
// Returns error if hostname unreachable, credentials invalid, or datastore inaccessible
func (pbs *PBSClient) TestConnection() error {
	// Same TLS policy as the backup session: pin the fingerprint when configured,
	// otherwise CA-validate. The old `|| CertFingerPrint == ""` skipped verification
	// entirely when no fingerprint was set, accepting any certificate (audit H-02).
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: pbs.buildTLSConfig(),
		},
	}

	// Test endpoint: GET datastore status (requires auth + datastore access)
	testURL := fmt.Sprintf("%s/api2/json/admin/datastore/%s/status", pbs.BaseURL, pbs.Datastore)

	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add PBS authentication header
	req.Header.Set("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s", pbs.AuthID, pbs.Secret))

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode == 401 {
		return fmt.Errorf("authentication failed: invalid credentials")
	}
	if resp.StatusCode == 403 {
		return fmt.Errorf("access denied: check datastore permissions")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("server error: HTTP %d", resp.StatusCode)
	}

	return nil
}

func (pbs *PBSClient) Connect(reader bool, backuptype string) {
	pbs.WritersManifest = make(map[uint64]int)
	pbs.SkippedFiles = []string{}  // Reset skipped files for new backup
	pbs.ExcludedFiles = []string{} // Reset excluded files for new backup
	pbs.ReadErrors = []string{}    // Reset read errors for new backup
	// CRITICAL: Reset Manifest.Files for each new session. Otherwise files from
	// previous Connect() calls leak into the next session's manifest, causing PBS
	// to reject the manifest (files reference UUIDs from abandoned sessions).
	pbs.Manifest.Files = nil
	// Same pinning policy as ListSnapshots/TestConnection (audit H-02), factored
	// into buildTLSConfig so all PBS clients verify the certificate identically.
	pbs.TLSConfig = *pbs.buildTLSConfig()
	if !reader {
		pbs.Manifest.BackupTime = time.Now().Unix()
	}
	pbs.Manifest.BackupType = backuptype
	if pbs.Manifest.BackupID == "" {
		hostname, _ := os.Hostname()
		pbs.Manifest.BackupID = hostname
	}

	// Close any existing HTTP/2 connections from previous backups (successful or failed)
	// This prevents reusing stale/broken connections that might cause 400 errors
	if pbs.Client.Transport != nil {
		// Close all idle connections first
		pbs.Client.CloseIdleConnections()

		// If it's an http2.Transport, force close it
		if h2Transport, ok := pbs.Client.Transport.(*http2.Transport); ok {
			h2Transport.CloseIdleConnections()
		}

		// Nil out the transport to ensure it's garbage collected
		pbs.Client.Transport = nil
	}

	// Track whether DialTLSContext has already been called for this session.
	// If HTTP/2 tries to reconnect (connection dropped mid-backup), we must
	// fail rather than silently create a new PBS session with stale writer IDs.
	var dialCalled atomic.Bool

	// Create completely new client with fresh HTTP/2 transport.
	// ReadIdleTimeout + PingTimeout cause the transport to send PING frames on idle
	// connections and detect dead connections quickly. Without this, a silently
	// dropped connection (firewall, NAT timeout, etc.) only manifests when the
	// next chunk upload fails, wasting minutes of backup time.
	pbs.Client = http.Client{
		Transport: &http2.Transport{
			ReadIdleTimeout: 30 * time.Second,
			PingTimeout:     15 * time.Second,

			DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {

				// Prevent HTTP/2 auto-reconnect from creating orphaned PBS sessions.
				// If the connection drops mid-backup, a reconnect would start a new PBS
				// session with the same backup-time but fresh writer IDs on the server,
				// while the client still holds stale writer IDs → "writer not registered".
				if !dialCalled.CompareAndSwap(false, true) {
					return nil, fmt.Errorf("connection lost: PBS session cannot be resumed (reconnect blocked to prevent orphaned sessions)")
				}

				//This is one of the trickiest parts, GO http2 library does not support starting with http1 and upgrading to 2 after
				//So to achieve that the function to create SSL socket has been hijacked here
				//Here an http 1.1 request to authenticate, start the backup and require upgrade to HTTP2 is done then the socket is passed to
				// http2.Transport handler
				conn, err := tls.Dial(network, addr, &pbs.TLSConfig)
				if err != nil {
					return nil, err
				}
				// Stash the raw socket so Close() can force-terminate the
				// session on shutdown even while an H2 stream is in flight.
				pbs.activeConnMu.Lock()
				pbs.activeConn = conn
				pbs.activeConnMu.Unlock()
				q := &url.Values{}
				q.Add("backup-time", fmt.Sprintf("%d", pbs.Manifest.BackupTime))
				q.Add("backup-type", pbs.Manifest.BackupType)
				q.Add("store", pbs.Datastore)
				if pbs.Namespace != "" {
					q.Add("ns", pbs.Namespace)
				}

				q.Add("backup-id", pbs.Manifest.BackupID)
				fmt.Println(q.Encode())
				//q.Add("debug", "1")

				// Build and log the full HTTP request
				var requestLines []string
				if !reader {
					requestLines = append(requestLines, "GET /api2/json/backup?"+q.Encode()+" HTTP/1.1")
				} else {
					requestLines = append(requestLines, "GET /api2/json/reader?"+q.Encode()+" HTTP/1.1")
				}
				requestLines = append(requestLines, "Host: "+addr)
				requestLines = append(requestLines, "Authorization: "+fmt.Sprintf("PBSAPIToken=%s:%s", pbs.AuthID, pbs.Secret))
				if !reader {
					requestLines = append(requestLines, "Upgrade: proxmox-backup-protocol-v1")
				} else {
					requestLines = append(requestLines, "Upgrade: proxmox-backup-reader-protocol-v1")
				}
				requestLines = append(requestLines, "Connection: Upgrade")

				fullRequest := strings.Join(requestLines, "\r\n") + "\r\n\r\n"
				// Redact the API token secret before logging the raw request —
				// fullRequest carries "Authorization: PBSAPIToken=<id>:<secret>".
				redactedRequest := strings.Replace(fullRequest,
					fmt.Sprintf("PBSAPIToken=%s:%s", pbs.AuthID, pbs.Secret),
					fmt.Sprintf("PBSAPIToken=%s:<redacted>", pbs.AuthID), 1)
				fmt.Printf("=== SENDING HTTP REQUEST TO PBS ===\n%s=== END REQUEST ===\n", redactedRequest)

				// Send the request
				conn.Write([]byte(fullRequest))
				fmt.Print("Reading response to upgrade...\n")
				buf := make([]byte, 0)
				for !strings.HasSuffix(string(buf), "\r\n\r\n") && !strings.HasSuffix(string(buf), "\n\n") {
					//fmt.Println(buf)
					b2 := make([]byte, 1)
					nbytes, err := conn.Read(b2)
					if err != nil || nbytes == 0 {
						fmt.Println("Connection unexpectedly closed")
						return nil, err
					}
					buf = append(buf, b2[:nbytes]...)

					//fmt.Println(string(b2))
				}
				fmt.Printf("=== RECEIVED HTTP RESPONSE FROM PBS ===\n%s\n=== END RESPONSE ===\n", string(buf))

				lines := strings.Split(string(buf), "\n")

				if len(lines) > 0 {
					toks := strings.Split(lines[0], " ")
					if len(toks) > 1 && toks[1] != "101" {
						statusCode := strings.Join(toks[1:], " ")
						fmt.Printf("ERROR: PBS rejected upgrade with status: %s\n", statusCode)
						fmt.Printf("Response headers:\n%s\n", string(buf))

						// Read the response body (JSON error details) based on Content-Length
						var contentLength int
						for _, line := range lines {
							line = strings.TrimSpace(line)
							if strings.HasPrefix(strings.ToLower(line), "content-length:") {
								fmt.Sscanf(strings.TrimSpace(strings.SplitN(line, ":", 2)[1]), "%d", &contentLength)
							}
						}
						var responseBody string
						if contentLength > 0 && contentLength < 4096 {
							bodyBuf := make([]byte, contentLength)
							totalRead := 0
							for totalRead < contentLength {
								n, readErr := conn.Read(bodyBuf[totalRead:])
								if readErr != nil || n == 0 {
									break
								}
								totalRead += n
							}
							responseBody = string(bodyBuf[:totalRead])
							fmt.Printf("PBS error body: %s\n", responseBody)
						}

						errBody := string(buf)
						if responseBody != "" {
							errBody = errBody + "\nBody: " + responseBody
						}
						return nil, &AuthErr{
							StatusCode:   statusCode,
							ResponseBody: errBody,
						}
					}
				}

				fmt.Printf("Upgraderesp: %s\n", string(buf))
				fmt.Println("Successfully upgraded to HTTP/2.")
				return conn, nil
			},
		},
	}

	registerActive(pbs)
}

// Close force-terminates the HTTP/2 session so PBS releases the backup
// writer and its shared snapshot lock without waiting on TCP keepalive.
// Safe to call multiple times and before Connect.
func (pbs *PBSClient) Close() {
	unregisterActive(pbs)

	pbs.activeConnMu.Lock()
	conn := pbs.activeConn
	pbs.activeConn = nil
	pbs.activeConnMu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}

	if pbs.Client.Transport != nil {
		if h2, ok := pbs.Client.Transport.(*http2.Transport); ok {
			h2.CloseIdleConnections()
		} else {
			pbs.Client.CloseIdleConnections()
		}
	}
}

type FIDXHeader struct {
	Magic        [8]byte
	UUID         [16]byte
	CreationTime uint64
	IndexCsum    [32]byte
	Size         uint64
	ChunkSize    uint64
	Padding      [4016]byte
}

func (pbs *PBSClient) DownloadPreviousToBytes(archivename string) ([]byte, error) { //In the future also download to tmp if index is extremely big...
	q := &url.Values{}

	q.Add("archive-name", archivename)

	req, err := http.NewRequest("GET", pbs.BaseURL+"/previous?"+q.Encode(), nil)
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s", pbs.AuthID, pbs.Secret))
	if err != nil {
		return nil, err
	}
	resp2, err := pbs.Client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return nil, err
	}
	defer resp2.Body.Close()

	ret, err := io.ReadAll(resp2.Body)

	if err != nil {
		return nil, err
	}

	return ret, nil

}

func (pbs *PBSClient) DownloadToBytes(archivename string) ([]byte, error) { //In the future also download to tmp if index is extremely big...
	q := &url.Values{}

	q.Add("file-name", archivename)

	req, err := http.NewRequest("GET", pbs.BaseURL+"/download?"+q.Encode(), nil)
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s", pbs.AuthID, pbs.Secret))
	if err != nil {
		return nil, err
	}
	resp2, err := pbs.Client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return nil, err
	}
	defer resp2.Body.Close()

	ret, err := io.ReadAll(resp2.Body)

	if err != nil {
		return nil, err
	}

	return ret, nil

}

func (pbs *PBSClient) GetKnownSha265FromFIDX(archivename string) (*hashmap.Map[string, bool], error) {
	data, err := pbs.DownloadPreviousToBytes(archivename)
	if err != nil {
		return nil, err
	}
	rdr := bytes.NewReader(data)
	var hdr FIDXHeader
	err = binary.Read(rdr, binary.LittleEndian, &hdr)
	if err != nil {
		return nil, err
	}
	if !slices.Equal(hdr.Magic[:], []byte{47, 127, 65, 237, 145, 253, 15, 205}) {
		return nil, fmt.Errorf("FIDX: Invalid magic %+v", hdr.Magic)
	}
	ret := hashmap.New[string, bool]()
	for i := uint64(0); i < hdr.Size/hdr.ChunkSize; i++ {
		H := make([]byte, 32)
		nbytes, err := rdr.Read(H)
		if err != nil {
			return nil, err
		}
		if nbytes != len(H) {
			return nil, fmt.Errorf("FIDX: Short read")
		}
		ret.Insert(hex.EncodeToString(H), true)
	}
	return ret, nil

}

func (pbs *PBSClient) GetChunkData(digest string) ([]byte, error) {
	q := &url.Values{}

	q.Add("digest", digest)

	req, err := http.NewRequest("GET", pbs.BaseURL+"/chunk?"+q.Encode(), nil)
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s", pbs.AuthID, pbs.Secret))
	if err != nil {
		return nil, err
	}
	resp2, err := pbs.Client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return nil, err
	}
	defer resp2.Body.Close()

	ret, err := io.ReadAll(resp2.Body)

	if err != nil {
		return nil, err
	}

	// Guard against a truncated/error body (proxy 502, reset mid-restore): the
	// blob magic + CRC header is 12 bytes, so ret[:8]/ret[12:] would panic.
	if len(ret) < 12 {
		return nil, fmt.Errorf("short chunk response for %s: %d bytes", digest, len(ret))
	}

	if slices.Equal(ret[:8], blobUncompressedMagic) {
		return ret[12:], nil
	} else if slices.Equal(ret[:8], blobCompressedMagic) {
		rd1 := bytes.NewReader(ret[12:])
		dec, err := zstd.NewReader(rd1)

		if err != nil {
			return nil, err
		}
		defer dec.Close()
		ret2 := make([]byte, 0)
		ret2, err = dec.DecodeAll(ret[12:], ret2)
		if err != nil {
			return nil, err
		}
		return ret2, nil
	} else {
		return nil, fmt.Errorf("encrypted chunks not supported")
	}

}

