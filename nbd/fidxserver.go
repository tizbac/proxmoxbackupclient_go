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
	Data  []byte
	Index int64
	Life  int
}

// chunkSource is the slice of *pbscommon.PBSClient that FIDXServer actually
// needs. Keeping it an interface lets the tests drive ReadAt without a live
// PBS server.
type chunkSource interface {
	GetChunkData(digest string) ([]byte, error)
}

type FIDXServer struct {
	header pbscommon.FIDXHeader
	cached map[int64]*CachedChunk
	chunks []string
	lock   sync.Mutex
	client chunkSource
}

func NewFIDXServer(data []byte, client chunkSource) (*FIDXServer, error) {
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
	// Guard before the chunk-count division below: a zero ChunkSize (corrupt or
	// truncated index) used to panic the whole process on an integer divide by
	// zero.
	if ret.header.ChunkSize == 0 {
		return nil, fmt.Errorf("FIDX: Invalid chunk size 0")
	}
	fmt.Printf("%+v\n", ret.header)
	for i := uint64(0); i < ret.header.Size/ret.header.ChunkSize+min(1, ret.header.Size%ret.header.ChunkSize); i++ {
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
	Index      int64
	SliceStart int64
	SliceEnd   int64
}

func (f *FIDXServer) getChunksIndexes(offset int64, size int64) []ChunkIndex {
	ret := make([]ChunkIndex, 0)
	cs := int64(f.header.ChunkSize)
	for i := offset / cs; i < (offset+size)/cs+1; i++ {
		ss := max(0, offset-i*cs)
		se := min(cs, (offset+size)-i*cs)
		if se-ss == 0 {
			continue
		}
		ret = append(ret, ChunkIndex{
			Index:      i,
			SliceStart: ss,
			SliceEnd:   se,
		})
	}
	return ret
}

func (f *FIDXServer) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, fmt.Errorf("FIDX: negative offset %d", off)
	}
	if off >= int64(f.header.Size) {
		return 0, io.EOF
	}
	// Clamp to the image size instead of trusting len(p): a request that starts
	// inside the image but runs off the end used to slice ch.Data past its end
	// and panic. Report the resulting short read the way os.File.ReadAt does.
	want := int64(len(p))
	eof := false
	if off+want > int64(f.header.Size) {
		want = int64(f.header.Size) - off
		eof = true
	}

	// ReadAt mutates f.cached (insert + LRU eviction), so it needs the write
	// lock; under RLock this was a data race on the map.
	f.lock.Lock()
	defer f.lock.Unlock()

	indexes := f.getChunksIndexes(off, want)
	var pos int64 = 0
	for _, idx := range indexes {
		if idx.Index < 0 || idx.Index >= int64(len(f.chunks)) {
			return 0, fmt.Errorf("FIDX: chunk index %d out of range (%d chunks)", idx.Index, len(f.chunks))
		}
		ch, ok := f.cached[idx.Index]
		if !ok {
			data, cerr := f.client.GetChunkData(f.chunks[idx.Index])
			if cerr != nil {
				return 0, cerr
			}

			for _, idx2 := range f.cached {
				idx2.Life--
			}
			f.cached[idx.Index] = &CachedChunk{
				Data:  data,
				Index: idx.Index,
				Life:  LRU_CACHE_LIFE,
			}

			fmt.Printf("Got %s\n", f.chunks[idx.Index])

			ch = f.cached[idx.Index]
		}

		if idx.SliceEnd > int64(len(ch.Data)) {
			return 0, fmt.Errorf("FIDX: chunk %d is %d bytes, need %d", idx.Index, len(ch.Data), idx.SliceEnd)
		}

		copy(p[pos:pos+(idx.SliceEnd-idx.SliceStart)], ch.Data[idx.SliceStart:idx.SliceEnd])

		ch.Life = LRU_CACHE_LIFE
		pos += idx.SliceEnd - idx.SliceStart
	}

	// Clean up expired cache entries
	for key := range f.cached {
		if f.cached[key].Life <= 0 {
			delete(f.cached, key)
			fmt.Printf("Remove from cache %s\n", f.chunks[key])
		}
	}

	if pos != want {
		return 0, fmt.Errorf("FIDX: short read: got %d of %d", pos, want)
	}

	if eof {
		return int(pos), io.EOF
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
