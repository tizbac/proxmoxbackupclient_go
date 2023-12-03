package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
)

type ChunkState struct {
	assignments []string 
	assignments_offset []uint64 
	pos uint64
	wrid uint64
	chunkcount uint64
	chunkdigests hash.Hash
	current_chunk []byte 
	C Chunker
}

func (c *ChunkState) Init() {
	c.assignments = make([]string, 0)
	c.assignments_offset = make([]uint64, 0)
	c.pos = 0
	c.chunkcount = 0
	c.chunkdigests = sha256.New()
	c.current_chunk = make([]byte,0)
	c.C = Chunker{}
	c.C.New(1024*1024*4)
}

func main() {
	client := &PBSClient{
		baseurl: os.Args[1],
		certfingerprint: os.Args[2],//"ea:7d:06:f9:87:73:a4:72:d0:e8:05:a4:b3:3d:95:d7:0a:26:dd:6d:5c:ca:e6:99:83:e4:11:3b:5f:10:f4:4b",
		authid: os.Args[3],
		secret: os.Args[4],
		datastore: os.Args[5],
	}

	client.Connect()
	


    A := &PXARArchive{}
	
	//f, _ := os.Create("test.pcat1")
	//defer f.Close()

	PXAR_CHK := ChunkState{}
	PXAR_CHK.Init()

	PCAT1_CHK := ChunkState{}
	PCAT1_CHK.Init()
	
	PXAR_CHK.wrid = client.CreateDynamicIndex("backup.pxar.didx")
	PCAT1_CHK.wrid = client.CreateDynamicIndex("catalog.pcat1.didx")

	A.writeCB = func(b []byte) {
		chunkpos := PXAR_CHK.C.Scan(b)
		

		if ( chunkpos > 0 ) {

			PXAR_CHK.current_chunk = append(PXAR_CHK.current_chunk, b[:chunkpos]...)

			h := sha256.New()
			h.Write(PXAR_CHK.current_chunk)
			shahash := hex.EncodeToString(h.Sum(nil))
			

			fmt.Printf("New chunk[%s] %d bytes\n",shahash, len(PXAR_CHK.current_chunk))
			
			client.UploadCompressedChunk(PXAR_CHK.wrid, shahash, PXAR_CHK.current_chunk)
			binary.Write(PXAR_CHK.chunkdigests, binary.LittleEndian, (PXAR_CHK.pos+uint64(len(PXAR_CHK.current_chunk))))
			PXAR_CHK.chunkdigests.Write(h.Sum(nil))

			PXAR_CHK.assignments_offset = append(PXAR_CHK.assignments_offset, PXAR_CHK.pos)
			PXAR_CHK.assignments = append(PXAR_CHK.assignments, shahash)
			PXAR_CHK.pos += uint64(len(PXAR_CHK.current_chunk))
			PXAR_CHK.chunkcount += 1
			
			
			PXAR_CHK.current_chunk = b[chunkpos:]
		}else{
			PXAR_CHK.current_chunk = append(PXAR_CHK.current_chunk, b...)
		}

		
		//f.Write(b)
	}

	A.catalogWriteCB = func(b []byte) {
		chunkpos := PCAT1_CHK.C.Scan(b)
		

		if ( chunkpos > 0 ) {

			PCAT1_CHK.current_chunk = append(PCAT1_CHK.current_chunk, b[:chunkpos]...)

			h := sha256.New()
			h.Write(PCAT1_CHK.current_chunk)
			shahash := hex.EncodeToString(h.Sum(nil))
			

			fmt.Printf("Catalog: New chunk[%s] %d bytes\n",shahash, len(PCAT1_CHK.current_chunk))
			
			client.UploadCompressedChunk(PCAT1_CHK.wrid, shahash, PCAT1_CHK.current_chunk)
			binary.Write(PCAT1_CHK.chunkdigests, binary.LittleEndian, (PCAT1_CHK.pos+uint64(len(PCAT1_CHK.current_chunk))))
			PCAT1_CHK.chunkdigests.Write(h.Sum(nil))

			PCAT1_CHK.assignments_offset = append(PCAT1_CHK.assignments_offset, PCAT1_CHK.pos)
			PCAT1_CHK.assignments = append(PCAT1_CHK.assignments, shahash)
			PCAT1_CHK.pos += uint64(len(PCAT1_CHK.current_chunk))
			PCAT1_CHK.chunkcount += 1
			
			
			PCAT1_CHK.current_chunk = b[chunkpos:]
		}else{
			PCAT1_CHK.current_chunk = append(PCAT1_CHK.current_chunk, b...)
		}
	}
	A.WriteDir(os.Args[6],"",true)

	if len(PXAR_CHK.current_chunk) > 0 {
		h := sha256.New()
		h.Write(PXAR_CHK.current_chunk)
		shahash := hex.EncodeToString(h.Sum(nil))
		binary.Write(PXAR_CHK.chunkdigests, binary.LittleEndian, (PXAR_CHK.pos+uint64(len(PXAR_CHK.current_chunk))))
		PXAR_CHK.chunkdigests.Write(h.Sum(nil))

		fmt.Printf("New chunk[%s] %d bytes\n",shahash, len(PXAR_CHK.current_chunk))
		PXAR_CHK.assignments_offset = append(PXAR_CHK.assignments_offset, PXAR_CHK.pos)
		PXAR_CHK.assignments = append(PXAR_CHK.assignments, shahash)
		PXAR_CHK.pos += uint64(len(PXAR_CHK.current_chunk))
		PXAR_CHK.chunkcount += 1
		client.UploadCompressedChunk(PXAR_CHK.wrid, shahash, PXAR_CHK.current_chunk)
	}

	if len(PCAT1_CHK.current_chunk) > 0 {
		h := sha256.New()
		h.Write(PCAT1_CHK.current_chunk)
		shahash := hex.EncodeToString(h.Sum(nil))
		binary.Write(PCAT1_CHK.chunkdigests, binary.LittleEndian, (PCAT1_CHK.pos+uint64(len(PCAT1_CHK.current_chunk))))
		PCAT1_CHK.chunkdigests.Write(h.Sum(nil))

		fmt.Printf("New chunk[%s] %d bytes\n",shahash, len(PCAT1_CHK.current_chunk))
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

}