package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"github.com/klauspost/compress/zstd"
	"golang.org/x/net/http2"
	"hash/crc32"
	"encoding/json"
)

type IndexCreateResp struct {
	WriterID int `json:"data"`
}


type IndexPutReq struct {
	DigestList []string `json:"digest-list"`
	OffsetList []uint64 `json:"offset-list"`
	WriterID   uint64   `json:"wid"`
}

type DynamicCloseReq struct {
	ChunkCount uint64 `json:"chunk-count"`
	CheckSum string `json:"csum"`
	Size uint64 `json:"size"`
	WriterID uint64 `json:"wid"`
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

type PBSClient struct {
	baseurl string
	certfingerprint string 
	apitoken string 
	secret string
	authid string

	datastore string
	manifest BackupManifest


	client http.Client
	tlsConfig tls.Config

	writersManifest map[uint64]int
}


var blobCompressedMagic = []byte{49, 185, 88, 66, 111, 182, 163, 127};
var blobUncompressedMagic = []byte{66, 171, 56, 7, 190, 131, 112, 161};

func (pbs* PBSClient) CreateDynamicIndex(name string) uint64 {

	req, err := http.NewRequest("POST", pbs.baseurl+"/dynamic_index", bytes.NewBuffer([]byte(fmt.Sprintf( "{\"archive-name\": \"%s\"}", name))))
	if err != nil {
		panic(err)
	}
	
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s",pbs.authid, pbs.secret))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")


	resp2, err := pbs.client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		panic(err)
		return 0
	}

	if resp2.StatusCode != http.StatusOK {
		resp1 , err := io.ReadAll(resp2.Body)
		fmt.Println("Error making request:", string(resp1), string(resp2.Proto) )
		panic(err)
		return 0
	}

	resp1 , err := io.ReadAll(resp2.Body)
	var R IndexCreateResp
	err = json.Unmarshal(resp1, &R)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return 0
	}
	fmt.Println("Writer id: ",R.WriterID)
	defer resp2.Body.Close()
	f := File{
		CryptMode: "none",
		Csum: "",
		Filename: name,
		Size: 0,
	}
	pbs.manifest.Files = append(pbs.manifest.Files, f)
	pbs.writersManifest[uint64(R.WriterID)] = len(pbs.manifest.Files)-1
	return uint64(R.WriterID)
}

func (pbs* PBSClient) UploadCompressedChunk(writerid uint64, digest string, chunkdata []byte ){
	outBuffer := make([]byte,0)
	outBuffer = append(outBuffer, blobCompressedMagic...)
	compressedData := make([]byte,0)
	
	//opt := zstd.WithEncoderLevel(zstd.SpeedFastest)
	w, _ := zstd.NewWriter(nil)
	compressedData = w.EncodeAll(chunkdata, compressedData)
	checksum := crc32.Checksum(compressedData, crc32.IEEETable)
	//binary.Write(outBuffer, binary.LittleEndian, checksum)
	outBuffer = binary.LittleEndian.AppendUint32(outBuffer, checksum)

	fmt.Printf("Appended checksum %08x , len: %d\n", checksum, len(outBuffer))

	outBuffer = append(outBuffer, compressedData...)

	fmt.Printf("Compressed: %d , Orig: %d\n", len(compressedData), len(chunkdata))

	q := &url.Values{}
	q.Add("digest", digest)
	q.Add("encoded-size", fmt.Sprintf("%d", len(outBuffer)))
	q.Add("size", fmt.Sprintf("%d", len(chunkdata)))
	q.Add("wid", fmt.Sprintf("%d", writerid))

	req, err := http.NewRequest("POST", pbs.baseurl+"/dynamic_chunk?"+q.Encode(), bytes.NewBuffer(outBuffer))

	resp2, err := pbs.client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		panic(err)
		return
	}

	if resp2.StatusCode != http.StatusOK {
		resp1 , err := io.ReadAll(resp2.Body)
		fmt.Println("Error making request:", string(resp1), string(resp2.Proto) )
		panic(err)
		return
	}
}

func (pbs* PBSClient) AssignChunks(writerid uint64, digests []string, offsets []uint64){
	indexput := &IndexPutReq{
		WriterID: writerid,
		DigestList: digests,
		OffsetList: offsets,
	}

	jsondata, _ := json.Marshal(indexput)

	req, err := http.NewRequest("PUT", pbs.baseurl+"/dynamic_index", bytes.NewBuffer(jsondata))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	resp2, err := pbs.client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		panic(err)
		return
	}
	defer resp2.Body.Close()
}

func (pbs* PBSClient) CloseDynamicIndex(writerid uint64, checksum string, totalsize uint64, chunkcount uint64) {
	finishreq := &DynamicCloseReq{
		WriterID: writerid,
		CheckSum: checksum,
		Size: totalsize,
		ChunkCount: chunkcount,
	}
	jsonpayload, _ := json.Marshal(finishreq)
	req, err := http.NewRequest("POST", pbs.baseurl+"/dynamic_close", bytes.NewBuffer(jsonpayload))
	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s",pbs.authid, pbs.secret))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")


	resp2, err := pbs.client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		panic(err)
		return
	}

	f := &pbs.manifest.Files[pbs.writersManifest[writerid]]

	f.Csum = checksum
	f.Size = int64(totalsize)
	

	defer resp2.Body.Close()
}

func (pbs* PBSClient) UploadBlob(name string, data []byte){
	out := make([]byte,0)
	out = append(out, blobUncompressedMagic...)
	
	checksum := crc32.ChecksumIEEE(data)
	out = binary.LittleEndian.AppendUint32(out, checksum)
	out = append(out, data...)

	q := &url.Values{}
	q.Add("encoded-size", fmt.Sprintf("%d", len(out)))
	q.Add("file-name", name)

	req, _ := http.NewRequest("POST", pbs.baseurl+"/blob?"+q.Encode(), bytes.NewBuffer(out))

	resp2, err := pbs.client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		panic(err)
		return
	}

	if resp2.StatusCode != http.StatusOK {
		resp1 , err := io.ReadAll(resp2.Body)
		fmt.Println("Error making request:", string(resp1), string(resp2.Proto) )
		panic(err)
		return
	}

}

func (pbs* PBSClient) UploadManifest(){
	manifestBin, _ := json.Marshal(pbs.manifest)
	pbs.UploadBlob("index.json.blob", manifestBin)
}

func (pbs* PBSClient) Finish() {
	req, err := http.NewRequest("POST", pbs.baseurl+"/finish", nil)
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s",pbs.authid, pbs.secret))
	if err != nil {
		panic(err)
	}
	resp2, err := pbs.client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		if err != nil {
			panic(err)
		}
		return
	}
	defer resp2.Body.Close()
}

func (pbs* PBSClient) Connect(reader bool) {
	pbs.writersManifest = make(map[uint64]int)
	pbs.tlsConfig = tls.Config{
		InsecureSkipVerify: true, // Set to true if you want to skip certificate verification entirely (not recommended for production)
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			// Extract the peer certificate
			if len(rawCerts) == 0 {
				return fmt.Errorf("no certificates presented by the peer")
			}
			peerCert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return fmt.Errorf("failed to parse certificate: %v", err)
			}

			// Calculate the SHA-256 fingerprint of the certificate
			expectedFingerprint := strings.ReplaceAll(pbs.certfingerprint, ":", "")
			calculatedFingerprint := sha256.Sum256(peerCert.Raw)

			// Compare the calculated fingerprint with the expected one
			if hex.EncodeToString(calculatedFingerprint[:]) != expectedFingerprint {
				return fmt.Errorf("certificate fingerprint does not match (%s,%s)", expectedFingerprint, hex.EncodeToString(calculatedFingerprint[:]))
			}

			// If the fingerprint matches, the certificate is considered valid
			return nil
		},
		//ServerName: "127.0.0.1",

	}

	pbs.manifest.BackupTime = time.Now().Unix()
	pbs.manifest.BackupType = "host"
	hostname, _:= os.Hostname()
	pbs.manifest.BackupID = hostname
	pbs.client = http.Client {
		Transport: &http2.Transport{

		DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
			conn, err := tls.Dial(network, addr, &pbs.tlsConfig)
			if err != nil {
				return nil, err
			}
			q := &url.Values{}
			q.Add("backup-time", fmt.Sprintf("%d",pbs.manifest.BackupTime))
			q.Add("backup-type", pbs.manifest.BackupType)
			q.Add("store", pbs.datastore)
			
			q.Add("backup-id", pbs.manifest.BackupID)
			q.Add("debug", "1")
			conn.Write([]byte("GET /api2/json/backup?"+q.Encode()+" HTTP/1.1\r\n"))
			conn.Write([]byte("Authorization: "+fmt.Sprintf("PBSAPIToken=%s:%s",pbs.authid, pbs.secret)+"\r\n"))
			if !reader {
				conn.Write([]byte("Upgrade: proxmox-backup-protocol-v1\r\n"))
			}else{
				conn.Write([]byte("Upgrade: proxmox-backup-reader-protocol-v1\r\n"))
			}
			conn.Write([]byte("Connection: Upgrade\r\n\r\n"))
			fmt.Println("Reading response to upgrade...\n")
			buf := make([]byte,0)
			for !strings.HasSuffix(string(buf),"\r\n\r\n") && !strings.HasSuffix(string(buf),"\n\n"){
				//fmt.Println(buf)
				b2 := make([]byte, 1)
				nbytes, err := conn.Read(b2)
				if err != nil || nbytes == 0 {
					fmt.Println("Connessione chiusa inasp.")
					return nil, err
				}
				buf = append(buf, b2[:nbytes]...)
				
				//fmt.Println(string(b2))
			}
			fmt.Printf("Upgraderesp: %s\n",string(buf))
			return conn, nil
		},
	},
		
	}




	/*req, err = http.NewRequest("POST", pbs.baseurl+"/api2/json/finish", nil)
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s",pbs.authid, pbs.secret))

	resp2, err := pbs.client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}
	defer resp2.Body.Close()*/

	
	fmt.Println("Successfully upgraded to HTTP/2.")
}


func (pbs* PBSClient) DownloadPreviousToBytes(archivename string) []byte { //In the future also download to tmp if index is extremely big...

	q := &url.Values{}

	q.Add("archive-name", archivename)

	req, err := http.NewRequest("GET", pbs.baseurl+"/previous?"+q.Encode(), nil)
	req.Header.Add("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s",pbs.authid, pbs.secret))
	if err != nil {
		panic(err)
	}
	resp2, err := pbs.client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		if err != nil {
			panic(err)
		}
		return make([]byte, 0)
	}
	defer resp2.Body.Close()

	ret, err := io.ReadAll(resp2.Body)

	if err != nil {
		panic(err)
	}

	return ret
	
	
}