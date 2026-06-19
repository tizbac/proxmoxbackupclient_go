package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"pbscommon"
	"slices"
	"sync"
)
const LRU_CACHE_LIFE = 16 //will be 16*4MB usage

type CachedChunk struct {
	Data []byte
	Index int64
	Life int
}

type FIDXServer struct {
	header pbscommon.FIDXHeader
	cached map[int64]*CachedChunk
	chunks []string
	lock sync.RWMutex
	client *pbscommon.PBSClient
}

func NewFIDXServer(data []byte, client *pbscommon.PBSClient) (*FIDXServer, error) {
	var ret FIDXServer
	ret.client = client
	rdr := bytes.NewReader(data)
	err := binary.Read(rdr, binary.LittleEndian, &ret.header)
	if err != nil {
		return nil, err
	}
	if !slices.Equal(ret.header.Magic[:], []byte{47, 127, 65, 237, 145, 253, 15, 205}) {
		return nil, fmt.Errorf("FIDX: Invalid magic %+v", ret.header.Magic)
	}
	fmt.Printf("%+v\n", ret.header)
	for i := uint64(0); i < ret.header.Size/ret.header.ChunkSize + min(1, ret.header.Size%ret.header.ChunkSize); i++ {
		H := make([]byte, 32)
		nbytes, err := rdr.Read(H)
		if err != nil {
			return nil, err
		}
		if nbytes != len(H) {
			return nil, fmt.Errorf("FIDX: Short read")
		}
		ret.chunks = append(ret.chunks, hex.EncodeToString(H))
	}
	ret.cached = make(map[int64]*CachedChunk)
	fmt.Printf("Read ok %d\n", len(ret.chunks))
	return &ret, nil
}

type ChunkIndex struct {
	Index int64 
	SliceStart int64 
	SliceEnd int64
}

func (f * FIDXServer) getChunksIndexes(offset int64, size int64) ([]ChunkIndex) {
	ret := make([]ChunkIndex, 0)
	for i := int64(offset/pbscommon.PBS_FIXED_CHUNK_SIZE); i < (offset+size)/pbscommon.PBS_FIXED_CHUNK_SIZE+1; i++ {
		ss := max(0,offset-i*pbscommon.PBS_FIXED_CHUNK_SIZE)
		se := min(pbscommon.PBS_FIXED_CHUNK_SIZE, (offset+size)-i*pbscommon.PBS_FIXED_CHUNK_SIZE)
		if se-ss == 0 {
			continue
		}
		ret = append(ret, ChunkIndex{
			Index: i,
			SliceStart: ss,
			SliceEnd: se,
		})
	}
	//fmt.Printf("%+v %d %d\n", ret, offset, size)
	return ret
}

func (f * FIDXServer) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(f.header.Size) {
		return 0, io.EOF
	}
	f.lock.RLock()
	indexes := f.getChunksIndexes(off, int64(len(p)))
	var pos int64 = 0
	for _, idx := range indexes {
		ch, ok := f.cached[idx.Index]
		if ok {
			
		} else {
			data, err := f.client.GetChunkData(f.chunks[idx.Index])
			if err != nil {
				panic(err)
			}

			for _, idx2 := range f.cached {
				idx2.Life--
			}
			f.cached[idx.Index] = &CachedChunk{
				Data: data,
				Index: idx.Index,
				Life: LRU_CACHE_LIFE,
			}

			fmt.Printf("Got %s\n", f.chunks[idx.Index])

			ch , _ = f.cached[idx.Index]
		}

		copy(p[pos:pos+(idx.SliceEnd-idx.SliceStart)], ch.Data[idx.SliceStart:idx.SliceEnd])
			
		ch.Life = LRU_CACHE_LIFE
		pos += (idx.SliceEnd-idx.SliceStart)
	}



	// Clean up expired cache entries (Go 1.22 compatible)
	for key := range f.cached {
		if f.cached[key].Life <= 0 {
			delete(f.cached, key)
			fmt.Printf("Remove from cache %s\n", f.chunks[key])
		}
	}

	f.lock.RUnlock()

	if pos != int64(len(p)) {
		panic(fmt.Errorf("Short read"))
	}

	return int(pos), nil
}

func (f *FIDXServer) WriteAt(p []byte, off int64) (n int, err error) {
	

	return 0, fmt.Errorf("Read only")
}


func (f *FIDXServer) Size() (int64, error) {

	return int64(f.header.Size), nil
}

func (f *FIDXServer) Sync() error {
	return fmt.Errorf("Read only")
}

