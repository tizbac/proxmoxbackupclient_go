package machinebackuplib

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"

	"pbscommon"
)

// JobProgress holds the running figures of one machine backup job, shared by
// every disk in it, for a GUI to sample on its own timer. Pass one in
// Config.Progress; every method is safe on a nil *JobProgress, so the CLI and
// the tests can leave it out.
type JobProgress struct {
	NewChunks    atomic.Uint64 // chunks uploaded to the server
	ReusedChunks atomic.Uint64 // chunks the server already had (or this image already sent)
	// BlankChunks counts the reused all-zero chunks (blank disk space), a
	// part of ReusedChunks, so "Reused: N, M of them blank" always holds. The
	// one blank chunk uploaded when the server has none counts as New only.
	BlankChunks   atomic.Uint64
	UploadedBytes atomic.Uint64 // uncompressed size of the chunks uploaded
	BytesDone     atomic.Uint64 // bytes of the whole job read so far
	BytesTotal    atomic.Uint64 // size of every disk in the job
	// UnreadableBytes is how much of the disks could not be read and went
	// into the image as zeros (see Unreadable for where).
	UnreadableBytes atomic.Uint64
	// FreeSkipped is the free space (and paging/hibernation files) not read
	// but sent as zeros.
	FreeSkipped atomic.Uint64

	// sent reads the job's PBS client counter (see SentBytes).
	sent atomic.Pointer[func() uint64]

	mu        sync.Mutex
	bad       []BadRange
	phase     string
	disk      string
	diskModel string
	diskNo    int // 1-based
	diskCount int
	diskSize  uint64
	diskPos   uint64
	partition string
}

// Location is where the reader is: which disk of how many, how far into it,
// and the partition at that point ("" when unknown).
type Location struct {
	Phase     string // "connecting", "snapshot", "data" or "finishing"
	Disk      string
	DiskModel string // "Samsung SSD 860 EVO 500GB", "" when unknown
	DiskNo    int
	DiskCount int
	DiskSize  uint64
	DiskPos   uint64
	Partition string
}

func (p *JobProgress) setPhase(phase string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.phase = phase
	p.mu.Unlock()
}

func (p *JobProgress) startDisk(disk, model string, no, count int, size uint64) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.disk, p.diskModel, p.diskNo, p.diskCount, p.diskSize = disk, model, no, count, size
	p.diskPos = 0
	p.partition = ""
	p.mu.Unlock()
}

func (p *JobProgress) setDiskPos(pos uint64, jobDone uint64) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.diskPos = pos
	p.mu.Unlock()
	p.BytesDone.Store(jobDone)
}

func (p *JobProgress) setPartition(name string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.partition = name
	p.mu.Unlock()
}

// Where returns the reader's current location.
func (p *JobProgress) Where() Location {
	if p == nil {
		return Location{}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return Location{Phase: p.phase, Disk: p.disk, DiskModel: p.diskModel, DiskNo: p.diskNo, DiskCount: p.diskCount,
		DiskSize: p.diskSize, DiskPos: p.diskPos, Partition: p.partition}
}

// BadRange is a stretch of a disk that could not be read (zeros in the image).
type BadRange struct {
	Disk   string
	Offset uint64 // bytes from the start of the disk
	Length uint64
}

// maxBadRanges caps the list kept for the report; UnreadableBytes still counts all.
const maxBadRanges = 50

// noteUnreadable records n unreadable bytes at off on disk, merging a range
// that continues the previous one.
func (p *JobProgress) noteUnreadable(disk string, off, n uint64) {
	if p == nil || n == 0 {
		return
	}
	p.UnreadableBytes.Add(n)
	p.mu.Lock()
	defer p.mu.Unlock()
	if k := len(p.bad); k > 0 && p.bad[k-1].Disk == disk && p.bad[k-1].Offset+p.bad[k-1].Length == off {
		p.bad[k-1].Length += n
		return
	}
	if len(p.bad) < maxBadRanges {
		p.bad = append(p.bad, BadRange{Disk: disk, Offset: off, Length: n})
	}
}

// Unreadable lists where the disks could not be read (at most 50 ranges).
func (p *JobProgress) Unreadable() []BadRange {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]BadRange(nil), p.bad...)
}

// SentBytes is how much the uploaded chunks took on the network so far:
// UploadedBytes after compression (or encryption). 0 before the job connects.
func (p *JobProgress) SentBytes() uint64 {
	if p == nil {
		return 0
	}
	if f := p.sent.Load(); f != nil {
		return (*f)()
	}
	return 0
}

func (p *JobProgress) setSentSource(f func() uint64) {
	if p != nil {
		p.sent.Store(&f)
	}
}

// countChunk records one chunk of an image. blank is true for an all-zero
// chunk, uploaded for one sent to the server rather than reused.
func (p *JobProgress) countChunk(size int, blank, uploaded bool) {
	if p == nil {
		return
	}
	if uploaded {
		p.NewChunks.Add(1)
		p.UploadedBytes.Add(uint64(size))
	} else {
		p.ReusedChunks.Add(1)
		if blank {
			p.BlankChunks.Add(1)
		}
	}
}

// Blocks is how many 4 MiB blocks (fixed-index chunks) a disk of size bytes
// has; block numbers in the progress messages run 1..Blocks(size).
func Blocks(size uint64) uint64 {
	return (size + pbscommon.PBS_FIXED_CHUNK_SIZE - 1) / pbscommon.PBS_FIXED_CHUNK_SIZE
}

// blockMessage is the progress line for a disk read up to pos: "Block N / M".
func blockMessage(dev string, pos, size uint64) string {
	n := pos / pbscommon.PBS_FIXED_CHUNK_SIZE
	if pos%pbscommon.PBS_FIXED_CHUNK_SIZE != 0 || n == 0 {
		n++
	}
	if m := Blocks(size); n > m {
		n = m
	}
	return fmt.Sprintf("%s: Block %d / %d", dev, n, Blocks(size))
}

// zeroChunkDetector spots all-zero chunks (blank disk space) and hands them
// the digest of a zero chunk, worked out once with the client's own digest
// (so it holds for encrypted backups too), instead of hashing every one.
// Comparing with zeros stops at the first nonzero byte, so chunks with data
// pay next to nothing for it. Only full-size chunks are checked; a short tail
// chunk is hashed as before.
type zeroChunkDetector struct {
	zeros  []byte
	digest [32]byte
}

func newZeroChunkDetector(client *pbscommon.PBSClient) zeroChunkDetector {
	zeros := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
	return zeroChunkDetector{zeros: zeros, digest: client.ChunkDigest(zeros)}
}

// digestOf returns data's digest and whether data is a blank chunk, calling
// hash only for chunks that are not.
func (z zeroChunkDetector) digestOf(data []byte, hash func([]byte) [32]byte) ([32]byte, bool) {
	if len(data) == len(z.zeros) && bytes.Equal(data, z.zeros) {
		return z.digest, true
	}
	return hash(data), false
}
