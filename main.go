package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"hash"
	"os"
)

var didxMagic = []byte{28, 145, 78, 165, 25, 186, 179, 205}

type ChunkState struct {
	assignments        []string
	assignments_offset []uint64
	pos                uint64
	wrid               uint64
	chunkcount         uint64
	chunkdigests       hash.Hash
	current_chunk      []byte
	C                  Chunker
}

type DidxEntry struct {
	offset uint64
	digest []byte
}

func (c *ChunkState) Init() {
	c.assignments = make([]string, 0)
	c.assignments_offset = make([]uint64, 0)
	c.pos = 0
	c.chunkcount = 0
	c.chunkdigests = sha256.New()
	c.current_chunk = make([]byte, 0)
	c.C = Chunker{}
	c.C.New(1024 * 1024 * 4)
}

func main() {
	newchunk := 0
	reusechunk := 0
	known_chunks_digest := make(map[string]bool)

	// Define command-line flags
	baseURLFlag := flag.String("baseurl", "", "Base URL for the PBS client")
	certFingerprintFlag := flag.String("certfingerprint", "", "Certificate fingerprint for authentication")
	authIDFlag := flag.String("authid", "", "Authentication ID (PBS Api token)")
	secretFlag := flag.String("secret", "", "Secret for authentication")
	datastoreFlag := flag.String("datastore", "", "Datastore name")
	backupSourceDirFlag := flag.String("backupdir", "", "Backup source directory")

	// Parse command-line flags
	flag.Parse()

	// Validate required flags
	if *baseURLFlag == "" || *certFingerprintFlag == "" || *authIDFlag == "" || *secretFlag == "" || *datastoreFlag == "" || *backupSourceDirFlag == "" {
		fmt.Println("All options are mandatory")

		flag.PrintDefaults()

		os.Exit(1)
	}

	client := &PBSClient{
		baseurl:         *baseURLFlag,
		certfingerprint: *certFingerprintFlag, //"ea:7d:06:f9:87:73:a4:72:d0:e8:05:a4:b3:3d:95:d7:0a:26:dd:6d:5c:ca:e6:99:83:e4:11:3b:5f:10:f4:4b",
		authid:          *authIDFlag,
		secret:          *secretFlag,
		datastore:       *datastoreFlag,
	}

	backupdir := *backupSourceDirFlag

	backupdir = createVSSSnapshot(backupdir)

	fmt.Printf("Starting backup of %s\n", backupdir)

	client.Connect(false)

	A := &PXARArchive{}
	A.archivename = "backup.pxar.didx"

	previous_didx := client.DownloadPreviousToBytes(A.archivename)

	fmt.Printf("Downloaded previous DIDX: %d bytes\n", len(previous_didx))

	/*f2, _ := os.Create("test.didx")
	defer f2.Close()

	f2.Write(previous_didx)*/

	if !bytes.HasPrefix(previous_didx, didxMagic) {
		fmt.Printf("Previous index has wrong magic (%s)!\n", previous_didx[:8])

	} else {

		previous_didx = previous_didx[4096:]
		for i := 0; i*40 < len(previous_didx); i += 1 {
			e := DidxEntry{}
			e.offset = binary.LittleEndian.Uint64(previous_didx[i*40 : i*40+8])
			e.digest = previous_didx[i*40+8 : i*40+40]
			shahash := hex.EncodeToString(e.digest)
			fmt.Printf("Previous: %s\n", shahash)
			known_chunks_digest[shahash] = true
		}
	}

	fmt.Printf("Known chunks: %d!\n", len(known_chunks_digest))

	f, _ := os.Create("test.pxar")
	defer f.Close()

	PXAR_CHK := ChunkState{}
	PXAR_CHK.Init()

	PCAT1_CHK := ChunkState{}
	PCAT1_CHK.Init()

	PXAR_CHK.wrid = client.CreateDynamicIndex(A.archivename)
	PCAT1_CHK.wrid = client.CreateDynamicIndex("catalog.pcat1.didx")

	A.writeCB = func(b []byte) {
		chunkpos := PXAR_CHK.C.Scan(b)

		if chunkpos > 0 {
			for chunkpos > 0 {

				PXAR_CHK.current_chunk = append(PXAR_CHK.current_chunk, b[:chunkpos]...)

				h := sha256.New()
				h.Write(PXAR_CHK.current_chunk)
				bindigest := h.Sum(nil)
				shahash := hex.EncodeToString(bindigest)

				if !known_chunks_digest[shahash] {
					fmt.Printf("New chunk[%s] %d bytes\n", shahash, len(PXAR_CHK.current_chunk))
					newchunk++
					client.UploadCompressedChunk(PXAR_CHK.wrid, shahash, PXAR_CHK.current_chunk)
				} else {
					fmt.Printf("Reuse chunk[%s] %d bytes\n", shahash, len(PXAR_CHK.current_chunk))
					reusechunk++
				}

				binary.Write(PXAR_CHK.chunkdigests, binary.LittleEndian, (PXAR_CHK.pos + uint64(len(PXAR_CHK.current_chunk))))
				PXAR_CHK.chunkdigests.Write(h.Sum(nil))

				PXAR_CHK.assignments_offset = append(PXAR_CHK.assignments_offset, PXAR_CHK.pos)
				PXAR_CHK.assignments = append(PXAR_CHK.assignments, shahash)
				PXAR_CHK.pos += uint64(len(PXAR_CHK.current_chunk))
				PXAR_CHK.chunkcount += 1

				PXAR_CHK.current_chunk = b[chunkpos:]
				chunkpos = PXAR_CHK.C.Scan(b[chunkpos:])
			}
		} else {
			PXAR_CHK.current_chunk = append(PXAR_CHK.current_chunk, b...)
		}

		f.Write(b)
	}

	A.catalogWriteCB = func(b []byte) {
		chunkpos := PCAT1_CHK.C.Scan(b)

		if chunkpos > 0 {
			for chunkpos > 0 {

				PCAT1_CHK.current_chunk = append(PCAT1_CHK.current_chunk, b[:chunkpos]...)

				h := sha256.New()
				h.Write(PCAT1_CHK.current_chunk)
				shahash := hex.EncodeToString(h.Sum(nil))

				fmt.Printf("Catalog: New chunk[%s] %d bytes\n", shahash, len(PCAT1_CHK.current_chunk))

				client.UploadCompressedChunk(PCAT1_CHK.wrid, shahash, PCAT1_CHK.current_chunk)
				binary.Write(PCAT1_CHK.chunkdigests, binary.LittleEndian, (PCAT1_CHK.pos + uint64(len(PCAT1_CHK.current_chunk))))
				PCAT1_CHK.chunkdigests.Write(h.Sum(nil))

				PCAT1_CHK.assignments_offset = append(PCAT1_CHK.assignments_offset, PCAT1_CHK.pos)
				PCAT1_CHK.assignments = append(PCAT1_CHK.assignments, shahash)
				PCAT1_CHK.pos += uint64(len(PCAT1_CHK.current_chunk))
				PCAT1_CHK.chunkcount += 1

				PCAT1_CHK.current_chunk = b[chunkpos:]
				chunkpos = PCAT1_CHK.C.Scan(b[chunkpos:])
			}
		} else {
			PCAT1_CHK.current_chunk = append(PCAT1_CHK.current_chunk, b...)
		}
	}
	A.WriteDir(backupdir, "", true)

	if len(PXAR_CHK.current_chunk) > 0 {
		h := sha256.New()
		h.Write(PXAR_CHK.current_chunk)
		shahash := hex.EncodeToString(h.Sum(nil))
		binary.Write(PXAR_CHK.chunkdigests, binary.LittleEndian, (PXAR_CHK.pos + uint64(len(PXAR_CHK.current_chunk))))
		PXAR_CHK.chunkdigests.Write(h.Sum(nil))

		if !known_chunks_digest[shahash] {
			fmt.Printf("New chunk[%s] %d bytes\n", shahash, len(PXAR_CHK.current_chunk))
			client.UploadCompressedChunk(PXAR_CHK.wrid, shahash, PXAR_CHK.current_chunk)
			newchunk++
		} else {
			fmt.Printf("Reuse chunk[%s] %d bytes\n", shahash, len(PXAR_CHK.current_chunk))
			reusechunk++
		}
		PXAR_CHK.assignments_offset = append(PXAR_CHK.assignments_offset, PXAR_CHK.pos)
		PXAR_CHK.assignments = append(PXAR_CHK.assignments, shahash)
		PXAR_CHK.pos += uint64(len(PXAR_CHK.current_chunk))
		PXAR_CHK.chunkcount += 1

	}

	if len(PCAT1_CHK.current_chunk) > 0 {
		h := sha256.New()
		h.Write(PCAT1_CHK.current_chunk)
		shahash := hex.EncodeToString(h.Sum(nil))
		binary.Write(PCAT1_CHK.chunkdigests, binary.LittleEndian, (PCAT1_CHK.pos + uint64(len(PCAT1_CHK.current_chunk))))
		PCAT1_CHK.chunkdigests.Write(h.Sum(nil))

		fmt.Printf("New chunk[%s] %d bytes\n", shahash, len(PCAT1_CHK.current_chunk))
		PCAT1_CHK.assignments_offset = append(PCAT1_CHK.assignments_offset, PCAT1_CHK.pos)
		PCAT1_CHK.assignments = append(PCAT1_CHK.assignments, shahash)
		PCAT1_CHK.pos += uint64(len(PCAT1_CHK.current_chunk))
		PCAT1_CHK.chunkcount += 1
		client.UploadCompressedChunk(PCAT1_CHK.wrid, shahash, PCAT1_CHK.current_chunk)
	}

	client.AssignChunks(PXAR_CHK.wrid, PXAR_CHK.assignments, PXAR_CHK.assignments_offset)

	client.CloseDynamicIndex(PXAR_CHK.wrid, hex.EncodeToString(PXAR_CHK.chunkdigests.Sum(nil)), PXAR_CHK.pos, PXAR_CHK.chunkcount)

	client.AssignChunks(PCAT1_CHK.wrid, PCAT1_CHK.assignments, PCAT1_CHK.assignments_offset)

	client.CloseDynamicIndex(PCAT1_CHK.wrid, hex.EncodeToString(PCAT1_CHK.chunkdigests.Sum(nil)), PCAT1_CHK.pos, PCAT1_CHK.chunkcount)

	client.UploadManifest()
	client.Finish()

	fmt.Printf("New %d , Reused %d\n", newchunk, reusechunk)

	VSSCleanup()
}
