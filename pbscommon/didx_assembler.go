package pbscommon

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

// 8-byte magic prefixing every PBS dynamic-index (.didx) file.
var didxMagic = []byte{28, 145, 78, 165, 25, 186, 179, 205}

const (
	didxHeaderSize = 4096
	didxEntrySize  = 40 // 8 bytes cumulative end-offset (uint64 LE) + 32 bytes SHA-256 digest
)

// ParsePreviousDIDXChunkDigests extracts the hex chunk digests from a previously
// downloaded .didx body so a caller can seed a known-chunks set for dedup. It is
// deliberately defensive: a body that is missing, shorter than the 4096-byte
// header, has the wrong magic, or whose entry section is not a whole number of
// 40-byte records returns nil instead of panicking — the caller then simply
// re-uploads all chunks (correct, just without dedup). This replaces the ad-hoc
// previousDidx[4096:] slicing that panicked on a truncated or odd-length transfer
// (and on a sub-8-byte 404 body).
func ParsePreviousDIDXChunkDigests(previousDidx []byte) []string {
	if len(previousDidx) < didxHeaderSize || !bytes.HasPrefix(previousDidx, didxMagic) {
		return nil
	}
	entries := previousDidx[didxHeaderSize:]
	if len(entries)%didxEntrySize != 0 {
		return nil
	}
	digests := make([]string, 0, len(entries)/didxEntrySize)
	for base := 0; base < len(entries); base += didxEntrySize {
		digests = append(digests, hex.EncodeToString(entries[base+8:base+40]))
	}
	return digests
}

// didxIndex is the parsed contents of a .didx dynamic-index file: the ordered
// list of chunk digests and their cumulative end-offsets in the reconstructed
// stream. offsets[i] is the byte offset one past the end of chunk i.
type didxIndex struct {
	offsets []uint64
	digests []string
	total   uint64
}

// downloadDIDXIndex fetches and parses a .didx index. The .didx file is NOT the
// archive itself — it is an index of cumulative end-offsets and chunk digests.
func (pbs *PBSClient) downloadDIDXIndex(archiveName string) (*didxIndex, error) {
	indexBytes, err := pbs.DownloadToBytes(archiveName)
	if err != nil {
		return nil, fmt.Errorf("download index %q: %w", archiveName, err)
	}
	if len(indexBytes) < didxHeaderSize {
		return nil, fmt.Errorf("index %q: short read (%d bytes, need at least %d)",
			archiveName, len(indexBytes), didxHeaderSize)
	}
	if !bytes.HasPrefix(indexBytes, didxMagic) {
		return nil, fmt.Errorf("index %q: invalid DIDX magic", archiveName)
	}

	entries := indexBytes[didxHeaderSize:]
	if len(entries)%didxEntrySize != 0 {
		return nil, fmt.Errorf("index %q: entries section length %d is not a multiple of %d",
			archiveName, len(entries), didxEntrySize)
	}
	chunkCount := len(entries) / didxEntrySize

	idx := &didxIndex{
		offsets: make([]uint64, chunkCount),
		digests: make([]string, chunkCount),
	}
	for i := 0; i < chunkCount; i++ {
		base := i * didxEntrySize
		idx.offsets[i] = binary.LittleEndian.Uint64(entries[base : base+8])
		idx.digests[i] = hex.EncodeToString(entries[base+8 : base+40])
		// Offsets are cumulative END offsets and must be strictly ascending (each
		// chunk has non-zero length): chunkIndexAt (sort.Search) and chunkRange
		// rely on it. A corrupt or hostile index with a non-monotonic offset would
		// otherwise yield an out-of-range chunk index and panic on chunk[pos-start:]
		// in ReadAt (uint64 underflow), reachable from listing/search.
		prev := uint64(0)
		if i > 0 {
			prev = idx.offsets[i-1]
		}
		if idx.offsets[i] <= prev {
			return nil, fmt.Errorf("index %q: non-monotonic offset at entry %d (%d <= %d)",
				archiveName, i, idx.offsets[i], prev)
		}
	}
	if chunkCount > 0 {
		idx.total = idx.offsets[chunkCount-1]
	}
	return idx, nil
}

// chunkRange returns the [start,end) byte span of chunk i in the reconstructed
// stream.
func (idx *didxIndex) chunkRange(i int) (start, end uint64) {
	if i > 0 {
		start = idx.offsets[i-1]
	}
	return start, idx.offsets[i]
}

// AssembleDIDXToFile downloads a dynamic-index archive (e.g. "backup.pxar.didx")
// from PBS and reconstructs the original byte stream into a temp file by
// fetching every referenced chunk and writing it at its offset.
//
// Unlike an in-memory assembly, peak memory is bounded to roughly maxParallel
// decompressed chunks (a few MB each) rather than the full archive size — so a
// 100 GB split restores without OOM-killing the process.
//
// On success it returns the path of the temp file and its total size. The
// caller owns the file and MUST remove it when done. On any error the temp
// file is cleaned up before returning.
//
// Chunks are fetched with bounded parallelism (maxParallel; defaults to 8).
// progress is invoked after each chunk lands with (done, total). It may be nil.
func (pbs *PBSClient) AssembleDIDXToFile(archiveName string, maxParallel int, progress func(done, total int)) (path string, size int64, err error) {
	if maxParallel <= 0 {
		maxParallel = 8
	}

	idx, err := pbs.downloadDIDXIndex(archiveName)
	if err != nil {
		return "", 0, err
	}
	chunkCount := len(idx.digests)

	tmp, err := os.CreateTemp("", "nimbus-restore-*.pxar")
	if err != nil {
		return "", 0, fmt.Errorf("create temp file for archive assembly: %w", err)
	}
	tmpPath := tmp.Name()
	// Any early return past this point must clean up the temp file.
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	if chunkCount == 0 {
		if cerr := tmp.Close(); cerr != nil {
			_ = os.Remove(tmpPath)
			return "", 0, fmt.Errorf("close temp file: %w", cerr)
		}
		return tmpPath, 0, nil
	}

	// Preallocate so concurrent WriteAt calls land at the right offsets.
	if terr := tmp.Truncate(int64(idx.total)); terr != nil {
		cleanup()
		return "", 0, fmt.Errorf("preallocate %d bytes: %w", idx.total, terr)
	}

	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	var firstErr atomic.Value
	var done atomic.Int64

	for i := 0; i < chunkCount; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idxNum int) {
			defer wg.Done()
			defer func() { <-sem }()

			if firstErr.Load() != nil {
				return
			}
			chunk, gerr := pbs.GetChunkData(idx.digests[idxNum])
			if gerr != nil {
				firstErr.CompareAndSwap(nil, fmt.Errorf("chunk %s (index %d/%d): %w",
					idx.digests[idxNum], idxNum, chunkCount, gerr))
				return
			}
			start, end := idx.chunkRange(idxNum)
			expected := int(end - start)
			if len(chunk) != expected {
				firstErr.CompareAndSwap(nil, fmt.Errorf("chunk %s (index %d): decompressed size %d != expected %d",
					idx.digests[idxNum], idxNum, len(chunk), expected))
				return
			}
			// Verify the chunk content against its index digest. PBS dynamic-index
			// digests are the SHA-256 of the chunk plaintext, so a mismatch means a
			// corrupted or tampered chunk — fail the restore rather than silently
			// writing wrong data.
			sum := sha256.Sum256(chunk)
			if hex.EncodeToString(sum[:]) != idx.digests[idxNum] {
				firstErr.CompareAndSwap(nil, fmt.Errorf("chunk %s (index %d): content hash mismatch",
					idx.digests[idxNum], idxNum))
				return
			}
			// Concurrent WriteAt at non-overlapping offsets is safe on *os.File.
			if _, werr := tmp.WriteAt(chunk, int64(start)); werr != nil {
				firstErr.CompareAndSwap(nil, fmt.Errorf("chunk %s (index %d): write at %d: %w",
					idx.digests[idxNum], idxNum, start, werr))
				return
			}

			n := done.Add(1)
			if progress != nil {
				progress(int(n), chunkCount)
			}
		}(i)
	}

	wg.Wait()

	if v := firstErr.Load(); v != nil {
		cleanup()
		return "", 0, v.(error)
	}

	if cerr := tmp.Close(); cerr != nil {
		_ = os.Remove(tmpPath)
		return "", 0, fmt.Errorf("close temp file: %w", cerr)
	}
	return tmpPath, int64(idx.total), nil
}
