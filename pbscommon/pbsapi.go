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
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"golang.org/x/net/http2"
)

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
	Size int64	`json:"size"`
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
}

func (e *AuthErr) Error() string {
	return "Authentication error"
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

	WritersManifest map[uint64]int
}

var blobCompressedMagic = []byte{49, 185, 88, 66, 111, 182, 163, 127}
var blobUncompressedMagic = []byte{66, 171, 56, 7, 190, 131, 112, 161}
func (pbs *PBSClient) CreateFixedIndex(fic FixedIndexCreateReq) (uint64, error) {
	jd,err := json.Marshal(fic)
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
		resp1, err := io.ReadAll(resp2.Body)
		fmt.Println("Error making request:", string(resp1), string(resp2.Proto))
		return 0, err
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

	f := &pbs.Manifest.Files[pbs.WritersManifest[writerid]]

	f.Csum = checksum
	f.Size = int64(totalsize)

	defer resp2.Body.Close()
	return nil
}

func (pbs *PBSClient) CreateDynamicIndex(name string) (uint64, error) {
	
	req, err := http.NewRequest("POST", pbs.BaseURL+"/dynamic_index", bytes.NewBuffer([]byte(fmt.Sprintf("{\"archive-name\": \"%s\"}", name))))
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
		resp1, err := io.ReadAll(resp2.Body)
		fmt.Println("Error making request:", string(resp1), string(resp2.Proto))
		return 0, err
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

		//opt := zstd.WithEncoderLevel(zstd.SpeedFastest)
		w, _ := zstd.NewWriter(nil)
		compressedData = w.EncodeAll(chunkdata, compressedData)
		checksum := crc32.Checksum(compressedData, crc32.IEEETable)
		//binary.Write(outBuffer, binary.LittleEndian, checksum)
		outBuffer = binary.LittleEndian.AppendUint32(outBuffer, checksum)

		//fmt.Printf("Appended checksum %08x , len: %d\n", checksum, len(outBuffer))

		outBuffer = append(outBuffer, compressedData...)

		if len(compressedData) > len(chunkdata) {
			return pbs.UploadChunk(writerid, digest, chunkdata, dynamic, false)
		}
	}else{
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
		resp1, err := io.ReadAll(resp2.Body)
		fmt.Println("Error making request:", string(resp1), string(resp2.Proto))
		return err
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

	f := &pbs.Manifest.Files[pbs.WritersManifest[writerid]]

	f.Csum = checksum
	f.Size = int64(totalsize)

	defer resp2.Body.Close()
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
		resp1, err := io.ReadAll(resp2.Body)
		fmt.Println("Error making request:", string(resp1), string(resp2.Proto))
		return err
	}

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
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s", pbs.AuthID, pbs.Secret))
	if err != nil {
		return err
	}
	resp2, err := pbs.Client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		if err != nil {
			return err
		}
	}
	defer resp2.Body.Close()
	return nil
}

func (pbs *PBSClient) Connect(reader bool) {
	pbs.WritersManifest = make(map[uint64]int)
	pbs.TLSConfig = tls.Config{
		InsecureSkipVerify: pbs.Insecure,
	}
	if pbs.Insecure {
		pbs.TLSConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			// Extract the peer certificate
			if len(rawCerts) == 0 {
				return fmt.Errorf("no certificates presented by the peer")
			}
			peerCert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return fmt.Errorf("failed to parse certificate: %v", err)
			}

			// Calculate the SHA-256 fingerprint of the certificate
			expectedFingerprint := strings.ReplaceAll(pbs.CertFingerPrint, ":", "")
			calculatedFingerprint := sha256.Sum256(peerCert.Raw)

			// Compare the calculated fingerprint with the expected one
			if hex.EncodeToString(calculatedFingerprint[:]) != expectedFingerprint {
				return fmt.Errorf("certificate fingerprint does not match (%s,%s)", expectedFingerprint, hex.EncodeToString(calculatedFingerprint[:]))
			}

			// If the fingerprint matches, the certificate is considered valid
			return nil
		}
	}

	pbs.Manifest.BackupTime = time.Now().Unix()
	pbs.Manifest.BackupType = "host"
	if pbs.Manifest.BackupID == "" {
		hostname, _ := os.Hostname()
		pbs.Manifest.BackupID = hostname
	}
	pbs.Client = http.Client{
		Transport: &http2.Transport{

			DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {

				//This is one of the trickiest parts, GO http2 library does not support starting with http1 and upgrading to 2 after
				//So to achieve that the function to create SSL socket has been hijacked here
				//Here an http 1.1 request to authenticate, start the backup and require upgrade to HTTP2 is done then the socket is passed to
				// http2.Transport handler
				conn, err := tls.Dial(network, addr, &pbs.TLSConfig)
				if err != nil {
					return nil, err
				}
				q := &url.Values{}
				q.Add("backup-time", fmt.Sprintf("%d", pbs.Manifest.BackupTime))
				q.Add("backup-type", pbs.Manifest.BackupType)
				q.Add("store", pbs.Datastore)
				if pbs.Namespace != "" {
					q.Add("ns", pbs.Namespace)
				}

				q.Add("backup-id", pbs.Manifest.BackupID)
				q.Add("debug", "1")
				conn.Write([]byte("GET /api2/json/backup?" + q.Encode() + " HTTP/1.1\r\n"))
				conn.Write([]byte("Authorization: " + fmt.Sprintf("PBSAPIToken=%s:%s", pbs.AuthID, pbs.Secret) + "\r\n"))
				if !reader {
					conn.Write([]byte("Upgrade: proxmox-backup-protocol-v1\r\n"))
				} else {
					conn.Write([]byte("Upgrade: proxmox-backup-reader-protocol-v1\r\n"))
				}
				conn.Write([]byte("Connection: Upgrade\r\n\r\n"))
				fmt.Printf("Reading response to upgrade...\n")
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
				lines := strings.Split(string(buf), "\n")

				if len(lines) > 0 {
					toks := strings.Split(lines[0], " ")
					if len(toks) > 1 && toks[1] != "101" {
						fmt.Println("Unexpected response code: " + strings.Join(toks[1:], " "))
						return nil, &AuthErr{}
					}
				}

				fmt.Printf("Upgraderesp: %s\n", string(buf))
				fmt.Println("Successfully upgraded to HTTP/2.")
				return conn, nil
			},
		},
	}

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
