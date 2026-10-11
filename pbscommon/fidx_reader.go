package pbscommon

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// fidxMagic starts every .fidx (fixed index) file.
var fidxMagic = []byte{47, 127, 65, 237, 145, 253, 15, 205}

// fidxHeaderSize is the fixed index header (FIDXHeader); the 32-byte chunk
// digests follow it.
const fidxHeaderSize = 4096

// parseFIDXIndex turns a .fidx file into the same index a dynamic index
// gives (cumulative end offsets and digests), so DIDXReaderAt reads disk
// images unchanged: every chunk is ChunkSize long except the last.
func parseFIDXIndex(name string, b []byte) (*didxIndex, error) {
	if len(b) < fidxHeaderSize {
		return nil, fmt.Errorf("index %q: short read (%d bytes, need at least %d)", name, len(b), fidxHeaderSize)
	}
	if !bytes.HasPrefix(b, fidxMagic) {
		return nil, fmt.Errorf("index %q: invalid FIDX magic", name)
	}
	// Magic (8), UUID (16), creation time (8), index checksum (32), then:
	size := binary.LittleEndian.Uint64(b[64:72])
	chunkSize := binary.LittleEndian.Uint64(b[72:80])
	if chunkSize == 0 {
		return nil, fmt.Errorf("index %q: chunk size 0", name)
	}
	count := (size + chunkSize - 1) / chunkSize
	digests := b[fidxHeaderSize:]
	if uint64(len(digests)) < count*32 {
		return nil, fmt.Errorf("index %q: %d digests for %d chunks", name, len(digests)/32, count)
	}
	idx := &didxIndex{
		offsets: make([]uint64, count),
		digests: make([]string, count),
		total:   size,
	}
	for i := uint64(0); i < count; i++ {
		end := (i + 1) * chunkSize
		if end > size {
			end = size
		}
		idx.offsets[i] = end
		idx.digests[i] = hex.EncodeToString(digests[i*32 : i*32+32])
	}
	return idx, nil
}

// NewFIDXReaderAt is NewDIDXReaderAt for a fixed-index archive (a disk image,
// drive-*.img.fidx): a lazy io.ReaderAt over the image that fetches, checks
// and caches 4 MiB chunks as they are read. Read-ahead starts OFF, because
// reading a file system inside an image jumps about; turn it on for a known
// run of reads with LimitPrefetchToRanges (a file's extents).
func (pbs *PBSClient) NewFIDXReaderAt(archiveName string, cacheChunks int, progress func(fetched, total int)) (*DIDXReaderAt, int64, error) {
	if cacheChunks <= 0 {
		cacheChunks = 16
	}
	raw, err := pbs.DownloadToBytes(archiveName)
	if err != nil {
		return nil, 0, fmt.Errorf("download index %q: %w", archiveName, err)
	}
	idx, err := parseFIDXIndex(archiveName, raw)
	if err != nil {
		return nil, 0, err
	}
	r := &DIDXReaderAt{
		pbs:         pbs,
		idx:         idx,
		cache:       newChunkCache(cacheChunks),
		progress:    progress,
		inflight:    make(map[int]chan struct{}),
		prefetchSem: make(chan struct{}, chunkPrefetchConcurrency),
		prefetchOff: true,
	}
	r.SetContext(nil)
	return r, int64(idx.total), nil
}
