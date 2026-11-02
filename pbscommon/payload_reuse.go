package pbscommon

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
)

// Metadata change detection ("metadata" mode, as in the official
// proxmox-backup-client): a file whose metadata and size match the previous
// snapshot is not read at all. Its content is taken from the previous
// snapshot's payload stream by appending the chunks that hold it to the new
// payload index (they are already on the server), and its PXAR_PAYLOAD_REF
// points at where its PXAR_PAYLOAD record lands in the new payload stream.
//
// Chunks are whole, so reusing a file usually drags in some neighbouring
// bytes (other files' old content, padding). Those bytes are never referenced
// and cost nothing on the server (the chunks are shared), but they make the
// payload stream longer than its content, so the waste is budgeted: a file is
// reused only while the padding stays within PaddingRatio of the reused bytes
// (plus a small allowance so the first files of a run are not rejected
// before their neighbours have had a chance to use the rest of the chunk).
// Otherwise the file is read and written normally.
//
// Rules taken from the official client and its later fixes:
//   - every metadata field we write (mode, flags, uid, gid, mtime to the
//     nanosecond) and the size must match;
//   - reused ranges are appended in strictly increasing order of their
//     previous offset; a file that lies before the end of the run so far
//     starts a new run (its chunks are appended again, which is allowed);
//   - chunks are identified by their position in the previous index, never
//     by digest alone, so a repeated chunk is not mistaken for the last one.

// PreviousFile is a regular file of the previous snapshot's split archive.
type PreviousFile struct {
	Mode   uint32
	UID    uint32
	GID    uint32
	Secs   int64
	Nanos  uint32
	Size   uint64
	Offset uint64 // position of its PXAR_PAYLOAD header in the previous payload stream
}

// PreviousSplitArchive is what metadata change detection knows about the
// previous snapshot of one split archive: its files, keyed by archive path
// ("dir/file.txt"), and its payload index.
type PreviousSplitArchive struct {
	Files   map[string]PreviousFile
	ends    []uint64 // payload chunk end offsets
	digests [][32]byte
}

// PayloadSize is the length of the previous payload stream.
func (p *PreviousSplitArchive) PayloadSize() uint64 {
	if len(p.ends) == 0 {
		return 0
	}
	return p.ends[len(p.ends)-1]
}

// parseDIDX parses a dynamic index body into chunk end offsets and digests,
// rejecting anything malformed.
func parseDIDX(b []byte) ([]uint64, [][32]byte, error) {
	if len(b) < didxHeaderSize || !bytes.HasPrefix(b, didxMagic) {
		return nil, nil, fmt.Errorf("not a dynamic index")
	}
	entries := b[didxHeaderSize:]
	if len(entries)%didxEntrySize != 0 {
		return nil, nil, fmt.Errorf("dynamic index entries are %d bytes, not a multiple of %d", len(entries), didxEntrySize)
	}
	n := len(entries) / didxEntrySize
	ends := make([]uint64, n)
	digests := make([][32]byte, n)
	prev := uint64(0)
	for i := 0; i < n; i++ {
		base := i * didxEntrySize
		ends[i] = binary.LittleEndian.Uint64(entries[base : base+8])
		if ends[i] <= prev {
			return nil, nil, fmt.Errorf("dynamic index offset %d at entry %d does not increase", ends[i], i)
		}
		prev = ends[i]
		copy(digests[i][:], entries[base+8:base+40])
	}
	return ends, digests, nil
}

// NewPreviousSplitArchive builds the lookup from a metadata-only listing of
// the previous archive (NewSplitMetadataReaderAt) and the previous payload
// index body. Files whose reference falls outside the payload stream are left
// out, so they are simply read again.
func NewPreviousSplitArchive(entries []PXARTreeEntry, payloadIndex []byte) (*PreviousSplitArchive, error) {
	ends, digests, err := parseDIDX(payloadIndex)
	if err != nil {
		return nil, fmt.Errorf("previous payload index: %w", err)
	}
	p := &PreviousSplitArchive{Files: make(map[string]PreviousFile), ends: ends, digests: digests}
	total := p.PayloadSize()
	for _, e := range entries {
		if e.IsDir || e.IsSymlink || !e.HasPayloadRef {
			continue
		}
		if e.Size > total || e.PayloadOffset > total || e.PayloadOffset+16+e.Size > total {
			continue
		}
		p.Files[e.Path] = PreviousFile{
			Mode: e.Mode, UID: e.UID, GID: e.GID, Secs: e.ModTime, Nanos: e.ModNanos,
			Size: e.Size, Offset: e.PayloadOffset,
		}
	}
	return p, nil
}

// ReuseStats counts what metadata change detection did for one archive.
type ReuseStats struct {
	Files       uint64 // files reused without reading them
	Bytes       uint64 // their payload records (content + 16-byte headers)
	Padding     uint64 // bytes of appended chunks no file uses
	Chunks      uint64 // chunks appended from the previous payload index
	ChunkBytes  uint64
	Candidates  uint64 // unchanged files found (reused or not)
	OverPadding uint64 // unchanged files read anyway to keep padding in budget
}

// PayloadReuse decides, file by file, whether to reuse a previous payload
// range and appends the chunks for it.
type PayloadReuse struct {
	prev *PreviousSplitArchive

	// ForceBoundary must end the payload chunk in progress so that the next
	// payload byte starts a new chunk.
	ForceBoundary func() error
	// InjectChunk must append a chunk of the previous payload index to the new
	// payload index without its data (it is already on the server).
	InjectChunk func(digest [32]byte, size uint64) error

	PaddingRatio     float64
	PaddingAllowance uint64

	run          bool
	runPrevStart uint64 // previous-stream offset of the run's first chunk
	runNewStart  uint64 // new-stream offset where that chunk was appended
	lastIdx      int    // last chunk appended in this run
	lastEnd      uint64 // previous-stream end of the last file reused in this run

	Stats ReuseStats
}

// NewPayloadReuse returns a planner over prev with the official client's
// padding ratio (10%) and a one-average-chunk allowance.
func NewPayloadReuse(prev *PreviousSplitArchive, forceBoundary func() error, inject func([32]byte, uint64) error) *PayloadReuse {
	return &PayloadReuse{
		prev: prev, ForceBoundary: forceBoundary, InjectChunk: inject,
		PaddingRatio: 0.1, PaddingAllowance: 4 * 1024 * 1024,
		lastIdx: -1,
	}
}

// Lookup returns the previous file at archive path rel if its metadata and
// size are unchanged.
func (r *PayloadReuse) Lookup(rel string, mode uint32, uid, gid uint32, secs int64, nanos uint32, size uint64) (PreviousFile, bool) {
	pf, ok := r.prev.Files[rel]
	if !ok || pf.Size != size || pf.Mode != mode || pf.UID != uid || pf.GID != gid || pf.Secs != secs || pf.Nanos != nanos {
		return PreviousFile{}, false
	}
	return pf, true
}

func (r *PayloadReuse) chunkStart(i int) uint64 {
	if i == 0 {
		return 0
	}
	return r.prev.ends[i-1]
}

// chunkAt returns the index of the chunk holding previous-stream offset off.
func (r *PayloadReuse) chunkAt(off uint64) int {
	return sort.Search(len(r.prev.ends), func(i int) bool { return r.prev.ends[i] > off })
}

// trail is the part of the run's last chunk after the last reused file.
func (r *PayloadReuse) trail() uint64 {
	if !r.run || r.lastIdx < 0 {
		return 0
	}
	return r.prev.ends[r.lastIdx] - r.lastEnd
}

// Break ends the current run: payload data that is not reused has been
// written, so the next reused file needs a chunk boundary of its own.
func (r *PayloadReuse) Break() {
	if r.run {
		r.Stats.Padding += r.trail()
		r.run = false
	}
}

// Reuse appends the chunks holding pf's payload record, if the padding
// budget allows, and returns where that record now sits in the new payload
// stream. payloadPos is the new payload stream's length so far, all of it
// already handed to the chunk layer. injected is how many bytes the appended
// chunks add to the new payload stream. ok is false when the file should be
// read and written normally instead.
func (r *PayloadReuse) Reuse(pf PreviousFile, payloadPos uint64) (newOffset, injected uint64, ok bool, err error) {
	r.Stats.Candidates++
	s := pf.Offset
	e := s + 16 + pf.Size
	if e > r.prev.PayloadSize() {
		return 0, 0, false, nil
	}
	ci, cj := r.chunkAt(s), r.chunkAt(e-1)

	continuing := r.run && s >= r.lastEnd && ci <= r.lastIdx+1
	var lead uint64 // padding this file adds in front of itself
	if continuing {
		lead = s - r.lastEnd
	} else {
		lead = s - r.chunkStart(ci) + r.trail()
	}
	budget := uint64(r.PaddingRatio*float64(r.Stats.Bytes+(e-s))) + r.PaddingAllowance
	if lead > 0 && r.Stats.Padding+lead > budget {
		r.Stats.OverPadding++
		return 0, 0, false, nil
	}

	if !continuing {
		r.Break()
		if err := r.ForceBoundary(); err != nil {
			return 0, 0, false, err
		}
		r.run = true
		r.runPrevStart = r.chunkStart(ci)
		r.runNewStart = payloadPos
		r.lastIdx = ci - 1
		r.lastEnd = s
		r.Stats.Padding += s - r.chunkStart(ci)
	} else {
		r.Stats.Padding += s - r.lastEnd
	}
	for k := r.lastIdx + 1; k <= cj; k++ {
		size := r.prev.ends[k] - r.chunkStart(k)
		if err := r.InjectChunk(r.prev.digests[k], size); err != nil {
			return 0, 0, false, err
		}
		injected += size
		r.Stats.Chunks++
		r.Stats.ChunkBytes += size
	}
	if cj > r.lastIdx {
		r.lastIdx = cj
	}
	r.lastEnd = e
	r.Stats.Files++
	r.Stats.Bytes += e - s
	return r.runNewStart + (s - r.runPrevStart), injected, true, nil
}

// PreviousBackupTime asks the backup (writer) session which snapshot it
// treats as the previous one: the same snapshot /previous serves indexes
// from, kept locked by the server for the whole session. ok is false when
// there is none.
func (pbs *PBSClient) PreviousBackupTime() (t int64, ok bool, err error) {
	req, err := http.NewRequest("GET", pbs.BaseURL+"/previous_backup_time", nil)
	if err != nil {
		return 0, false, err
	}
	pbs.setAuth(req)
	resp, err := pbs.Client.Do(req)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, false, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, false, fmt.Errorf("previous_backup_time: HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(body))
	}
	var r struct {
		Data *int64 `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return 0, false, fmt.Errorf("previous_backup_time: %w", err)
	}
	if r.Data == nil {
		return 0, false, nil
	}
	return *r.Data, true, nil
}

// LoadPreviousSplitArchive prepares metadata change detection for one
// archive of this backup (writer) session: it finds the previous snapshot,
// checks it holds metaName and payloadName encrypted the same way as this
// backup (reused chunks are not re-encrypted, so a different key or mode
// would produce a snapshot that cannot be restored), and lists the previous
// metadata stream through a separate reader session. payloadIndex is the
// previous payload index as downloaded through this session's /previous,
// which is also what makes its chunks usable here.
func (pbs *PBSClient) LoadPreviousSplitArchive(metaName, payloadName string, payloadIndex []byte) (*PreviousSplitArchive, int64, error) {
	prevTime, ok, err := pbs.PreviousBackupTime()
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		return nil, 0, fmt.Errorf("no previous snapshot")
	}
	rc := &PBSClient{
		BaseURL:          pbs.BaseURL,
		CertFingerPrint:  pbs.CertFingerPrint,
		APIToken:         pbs.APIToken,
		Secret:           pbs.Secret,
		AuthID:           pbs.AuthID,
		Username:         pbs.Username,
		Password:         pbs.Password,
		Ticket:           pbs.Ticket,
		CSRFToken:        pbs.CSRFToken,
		Datastore:        pbs.Datastore,
		Namespace:        pbs.Namespace,
		Insecure:         pbs.Insecure,
		CompressionLevel: pbs.CompressionLevel,
		Crypt:            pbs.Crypt,
		Manifest:         BackupManifest{BackupID: pbs.Manifest.BackupID, BackupTime: prevTime},
	}
	rc.Connect(true, pbs.Manifest.BackupType)
	defer rc.Close()

	raw, err := rc.DownloadBlob("index.json.blob")
	if err != nil {
		return nil, prevTime, fmt.Errorf("previous manifest: %w", err)
	}
	var m BackupManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, prevTime, fmt.Errorf("previous manifest: %w", err)
	}
	want := pbs.defaultCryptMode()
	for _, name := range []string{metaName, payloadName} {
		found := false
		for _, f := range m.Files {
			if f.Filename != name {
				continue
			}
			found = true
			mode := f.CryptMode
			if mode == "" {
				mode = CryptModeNone
			}
			if mode != want {
				return nil, prevTime, fmt.Errorf("previous %s is crypt-mode %s, this backup is %s", name, mode, want)
			}
		}
		if !found {
			return nil, prevTime, fmt.Errorf("previous snapshot has no %s", name)
		}
	}
	if pbs.Crypt != nil {
		fp := pbs.Crypt.Fingerprint()
		if m.Unprotected.KeyFingerprint == nil || *m.Unprotected.KeyFingerprint != fp {
			return nil, prevTime, fmt.Errorf("previous snapshot was encrypted with a different key")
		}
	}

	ra, size, err := rc.NewDIDXReaderAt(metaName, 64, nil)
	if err != nil {
		return nil, prevTime, fmt.Errorf("open previous %s: %w", metaName, err)
	}
	reader, err := NewSplitMetadataReaderAt(ra, size)
	if err != nil {
		return nil, prevTime, fmt.Errorf("previous %s: %w", metaName, err)
	}
	entries, err := reader.ListEntries()
	if err != nil {
		return nil, prevTime, fmt.Errorf("list previous %s: %w", metaName, err)
	}
	prev, err := NewPreviousSplitArchive(entries, payloadIndex)
	return prev, prevTime, err
}
