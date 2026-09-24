package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"pbscommon"
	"retry"
	"slices"
	"sync"
	"time"
)
const LRU_CACHE_LIFE = 16 //will be 16*4MB usage

// chunkFetchTimeout bounds a SINGLE chunk fetch attempt — see the retry loop
// in ReadAt. Matches pbscommon/didx_reader.go's DIDXReaderAt.chunkFetchTimeout
// (the GUI restore path's own equivalent), for the same reason: chunks are at
// most ~16MB, so even a slow link should comfortably finish well inside this.
const chunkFetchTimeout = 60 * time.Second

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
			// A per-attempt timeout, not a deadline on the whole session
			// (pbsnbd serves reads for however long the NBD device stays
			// attached — a Clonezilla restore can run for hours): each
			// individual chunk fetch gets chunkFetchTimeout, retried up to
			// 5 times, mirroring pbscommon/didx_reader.go's DIDXReaderAt
			// (the GUI restore path's own equivalent protection). Added
			// 2026-09-24 after a real bare-metal restore test hung
			// indefinitely mid-partition-clone: GetChunkData(context.
			// Background()) has no deadline at all, so one stuck HTTP/2
			// stream (the exact class of hang seen earlier the same night
			// in the GUI, root cause still unconfirmed server-side) blocked
			// this ReadAt — and therefore Clonezilla's partclone — forever,
			// with the only recovery being to kill pbsnbd and restart the
			// whole attach+restore from scratch. A retried, bounded fetch
			// gives a stuck stream a real chance to recover on its own
			// before that drastic step is needed.
			var data []byte
			retryCfg := retry.DefaultConfig()
			retryCfg.MaxAttempts = 5
			fetchErr := retry.DoWithJitter(context.Background(), retryCfg, retry.DefaultRetryable, func() error {
				fetchCtx, cancel := context.WithTimeout(context.Background(), chunkFetchTimeout)
				defer cancel()
				d, ferr := f.client.GetChunkData(fetchCtx, f.chunks[idx.Index])
				if ferr != nil {
					return ferr
				}
				data = d
				return nil
			})
			if fetchErr != nil {
				// All retries exhausted — this used to be the ONLY outcome
				// (immediately, on the very first error, timeout or not).
				// Still fatal: FIDXServer has no way to report a read
				// failure through io.ReaderAt's contract back to the NBD
				// protocol layer gracefully at this call depth, so this
				// crashes pbsnbd same as before. The real improvement is
				// upstream — a transient stall now gets up to 5 chances,
				// each up to chunkFetchTimeout, to clear on its own first.
				panic(fetchErr)
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

