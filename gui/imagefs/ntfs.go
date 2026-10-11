package imagefs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"unicode/utf16"

	"www.velocidex.com/golang/go-ntfs/parser"
)

// Entry is one file, folder or symbolic link of a volume.
type Entry struct {
	Path    string // from the volume root, '/'-separated, no leading slash
	IsDir   bool
	Size    uint64 // the file's (unnamed stream's) size; 0 for folders
	MTime   int64  // last write, FILETIME (100 ns since 1601)
	Created int64  // creation, FILETIME (0 when the file system has none)
	Attrs   uint32 // FILE_ATTRIBUTE_* (NTFS, FAT, exFAT)
	Record  uint64 // where the file system finds the file again (MFT record, inode, first cluster), for Open
	Aux     uint64 // more of that, file-system specific (exFAT: valid data length)

	// POSIX file systems (ext, XFS): Mode is st_mode with its type bits,
	// UID and GID the owner (HasOwner). A symbolic link has Symlink set and
	// its target in LinkTarget.
	Mode       uint32
	UID, GID   uint32
	HasOwner   bool
	Symlink    bool
	LinkTarget string
}

// UnixSeconds converts a FILETIME to Unix seconds.
func UnixSeconds(ft int64) int64 {
	if ft == 0 {
		return 0
	}
	return (ft - 116444736000000000) / 10000000
}

// NTFS is an NTFS volume inside an image.
type NTFS struct {
	ctx        *parser.NTFSContext
	recordSize int64
	diskType   string     // the volume reader's type, as go-ntfs's DebugRuns names it
	mu         sync.Mutex // go-ntfs keeps an MFT entry cache that is not safe for concurrent use
}

// OpenNTFS reads the boot sector and bootstraps the master file table of
// the NTFS volume that r (a partition) holds.
func OpenNTFS(r io.ReaderAt) (v *NTFS, err error) {
	// go-ntfs panics on some malformed structures; a damaged image must give
	// an error, not take the GUI down.
	defer func() {
		if p := recover(); p != nil {
			v, err = nil, fmt.Errorf("NTFS: %v", p)
		}
	}()
	ctx, err := parser.GetNTFSContext(r, 0)
	if err != nil {
		return nil, fmt.Errorf("NTFS: %w", err)
	}
	return &NTFS{ctx: ctx, recordSize: ctx.GetRecordSize(), diskType: fmt.Sprintf("%T", r)}, nil
}

// FSName is "NTFS".
func (v *NTFS) FSName() string { return "NTFS" }

// CaseInsensitive: NTFS names match without regard to case.
func (v *NTFS) CaseInsensitive() bool { return true }

// Open opens a file entry from List.
func (v *NTFS) Open(e Entry) (FileReader, error) { return v.OpenRecord(e.Record, int64(e.Size)) }

// Label is the volume's name (what Explorer shows, e.g. "Windows"), "" if
// it has none.
func (v *NTFS) Label() (label string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	defer func() {
		if recover() != nil {
			label = ""
		}
	}()
	e, err := v.ctx.GetMFT(3) // $Volume
	if err != nil {
		return ""
	}
	for _, a := range e.EnumerateAttributes(v.ctx) {
		if a.Type().Value == 0x60 && a.IsResident() { // $VOLUME_NAME
			b := make([]byte, a.Content_size())
			n, _ := a.Data(v.ctx).ReadAt(b, 0)
			return decodeUTF16(b[:n])
		}
	}
	return ""
}

// mftSize is the length of the $MFT stream.
func (v *NTFS) mftSize() (int64, error) {
	e, err := v.ctx.GetMFT(0)
	if err != nil {
		return 0, err
	}
	rr, err := parser.OpenStream(v.ctx, e, parser.ATTR_TYPE_DATA, parser.WILDCARD_STREAM_ID, parser.WILDCARD_STREAM_NAME)
	if err != nil {
		return 0, err
	}
	return parser.RangeSize(rr), nil
}

// record is what one MFT record (base or extension) says about a file.
type record struct {
	num     uint64
	base    uint64 // the base record for an extension record, else num
	seq     uint16
	inUse   bool
	isDir   bool
	hasSI   bool
	mtime   int64
	created int64
	attrs   uint32
	names   []fileName
	size    uint64
	hasSize bool
}

type fileName struct {
	parent    uint64
	parentSeq uint16
	name      string
}

// ErrCancelled is returned by List when cancel reported true.
var ErrCancelled = errors.New("cancelled")

// List returns every file and folder in the volume, sorted by path,
// leaving out the NTFS metafiles ($MFT, $LogFile, $Extend...). Each hard
// link is its own entry; short (8.3) names are not listed. progress, if
// set, gets the bytes of the MFT read so far and its size.
func (v *NTFS) List(cancel func() bool, progress func(done, total int64)) ([]Entry, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	total, err := v.mftSize()
	if err != nil {
		return nil, fmt.Errorf("NTFS: size of $MFT: %w", err)
	}
	rs := v.recordSize
	if rs <= 0 || rs > 65536 {
		return nil, fmt.Errorf("NTFS: unexpected MFT record size %d", rs)
	}

	// Read the MFT in blocks, several at once: on a disk image each block is
	// a chunk or two fetched from the server, so this is latency-bound.
	const block = 4 << 20
	blocks := (total + block - 1) / block
	results := make([][]record, blocks)
	var (
		next     int64
		done     int64
		firstErr error
		mu       sync.Mutex
		wg       sync.WaitGroup
	)
	worker := func() {
		defer wg.Done()
		buf := make([]byte, block)
		for {
			mu.Lock()
			if firstErr != nil || next >= blocks {
				mu.Unlock()
				return
			}
			b := next
			next++
			mu.Unlock()
			if cancel != nil && cancel() {
				mu.Lock()
				if firstErr == nil {
					firstErr = ErrCancelled
				}
				mu.Unlock()
				return
			}
			off := b * block
			n := int64(block)
			if off+n > total {
				n = total - off
			}
			got, err := v.ctx.MFTReader.ReadAt(buf[:n], off)
			if err != nil && err != io.EOF && int64(got) < n {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("NTFS: read $MFT at %d: %w", off, err)
				}
				mu.Unlock()
				return
			}
			recs := make([]record, 0, int64(got)/rs)
			for o := int64(0); o+rs <= int64(got); o += rs {
				if r, ok := parseRecord(buf[o:o+rs], uint64((off+o)/rs)); ok {
					recs = append(recs, r)
				}
			}
			results[b] = recs
			mu.Lock()
			done += n
			d := done
			mu.Unlock()
			if progress != nil {
				progress(d, total)
			}
		}
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go worker()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	// Merge extension records into their base records.
	files := make(map[uint64]*record, total/rs)
	var ext []record
	for _, recs := range results {
		for i := range recs {
			r := &recs[i]
			if r.base != r.num {
				ext = append(ext, *r)
				continue
			}
			files[r.num] = r
		}
	}
	for _, e := range ext {
		b, ok := files[e.base]
		if !ok {
			continue
		}
		b.names = append(b.names, e.names...)
		if e.hasSize && !b.hasSize {
			b.size, b.hasSize = e.size, true
		}
	}
	return buildEntries(files), nil
}

// buildEntries turns the records into paths.
func buildEntries(files map[uint64]*record) []Entry {
	const root = 5
	const extend = 11 // $Extend: $Quota, $ObjId, $Reparse, $UsnJrnl...
	type resolved struct {
		path string
		ok   bool
	}
	dirPath := map[uint64]resolved{root: {"", true}}
	var resolve func(num uint64, seq uint16, depth int) (string, bool)
	resolve = func(num uint64, seq uint16, depth int) (string, bool) {
		if r, ok := dirPath[num]; ok {
			return r.path, r.ok
		}
		f := files[num]
		if depth > 256 || f == nil || !f.inUse || !f.isDir || num == extend || (seq != 0 && f.seq != seq) || len(f.names) == 0 {
			return "", false
		}
		n := f.names[0]
		p, ok := resolve(n.parent, n.parentSeq, depth+1)
		if ok {
			p = joinPath(p, n.name)
		}
		dirPath[num] = resolved{p, ok}
		return p, ok
	}

	entries := make([]Entry, 0, len(files))
	for num, f := range files {
		if !f.inUse || num < 16 || len(f.names) == 0 {
			continue // metafiles, the root itself, unused or nameless records
		}
		for _, n := range f.names {
			parent, ok := resolve(n.parent, n.parentSeq, 0)
			if !ok {
				continue // orphan, or under $Extend
			}
			if parent == "" && strings.HasPrefix(n.name, "$") && num < 64 {
				continue // a reserved metafile in the root
			}
			e := Entry{Path: joinPath(parent, n.name), IsDir: f.isDir, MTime: f.mtime, Created: f.created, Attrs: f.attrs, Record: num}
			if !f.isDir {
				e.Size = f.size
			}
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}

func joinPath(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// parseRecord decodes one MFT record. ok is false for an empty or damaged
// one. The buffer is modified (update sequence fix-ups).
func parseRecord(b []byte, num uint64) (r record, ok bool) {
	if len(b) < 48 || string(b[0:4]) != "FILE" {
		return r, false
	}
	usaOff := int(binary.LittleEndian.Uint16(b[4:]))
	usaCount := int(binary.LittleEndian.Uint16(b[6:]))
	if usaCount < 2 || usaOff+2*usaCount > len(b) {
		return r, false
	}
	// Each sector's last two bytes were replaced by the update sequence
	// number; put the real bytes back, and reject a torn record.
	stride := len(b) / (usaCount - 1)
	usn := b[usaOff : usaOff+2]
	for i := 1; i < usaCount; i++ {
		p := i*stride - 2
		if p+2 > len(b) {
			break
		}
		if b[p] != usn[0] || b[p+1] != usn[1] {
			return r, false
		}
		b[p], b[p+1] = b[usaOff+2*i], b[usaOff+2*i+1]
	}
	flags := binary.LittleEndian.Uint16(b[22:])
	r.num = num
	r.seq = binary.LittleEndian.Uint16(b[16:])
	r.inUse = flags&1 != 0
	r.isDir = flags&2 != 0
	r.base = binary.LittleEndian.Uint64(b[32:]) & 0xFFFFFFFFFFFF
	if r.base == 0 {
		r.base = num
	}
	if !r.inUse {
		return r, true
	}

	off := int(binary.LittleEndian.Uint16(b[20:]))
	used := int(binary.LittleEndian.Uint32(b[24:]))
	if used > len(b) {
		used = len(b)
	}
	for off+16 <= used {
		typ := binary.LittleEndian.Uint32(b[off:])
		if typ == 0xFFFFFFFF {
			break
		}
		length := int(binary.LittleEndian.Uint32(b[off+4:]))
		if length < 16 || off+length > used {
			break
		}
		a := b[off : off+length]
		nonResident := a[8] != 0
		nameLen := int(a[9])
		var content []byte
		if !nonResident && len(a) >= 24 {
			cl := int(binary.LittleEndian.Uint32(a[16:]))
			co := int(binary.LittleEndian.Uint16(a[20:]))
			if co+cl <= len(a) {
				content = a[co : co+cl]
			}
		}
		switch typ {
		case 0x10: // $STANDARD_INFORMATION
			if len(content) >= 36 {
				r.hasSI = true
				r.created = int64(binary.LittleEndian.Uint64(content[0:]))
				r.mtime = int64(binary.LittleEndian.Uint64(content[8:]))
				r.attrs = binary.LittleEndian.Uint32(content[32:])
			}
		case 0x30: // $FILE_NAME
			if len(content) >= 66 {
				n := int(content[64])
				ns := content[65]
				if ns == 2 || 66+2*n > len(content) {
					break // the DOS 8.3 alias of a long name
				}
				ref := binary.LittleEndian.Uint64(content[0:])
				r.names = append(r.names, fileName{
					parent:    ref & 0xFFFFFFFFFFFF,
					parentSeq: uint16(ref >> 48),
					name:      decodeUTF16(content[66 : 66+2*n]),
				})
			}
		case 0x80: // $DATA: only the unnamed stream gives the file size
			if nameLen != 0 {
				break
			}
			if !nonResident {
				r.size, r.hasSize = uint64(len(content)), true
			} else if len(a) >= 56 && binary.LittleEndian.Uint64(a[16:]) == 0 {
				r.size, r.hasSize = binary.LittleEndian.Uint64(a[48:]), true // first extent carries the real size
			}
		}
		off += length
	}
	return r, true
}

func decodeUTF16(b []byte) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	return string(utf16.Decode(u))
}

// File is the unnamed data stream of one file, for reading.
type File struct {
	r    io.ReaderAt
	v    *NTFS
	size int64
}

// OpenRecord opens the data of the file in MFT record num. size is the
// file's size from List; reads stop there.
func (v *NTFS) OpenRecord(num uint64, size int64) (f *File, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	defer func() {
		if p := recover(); p != nil {
			f, err = nil, fmt.Errorf("NTFS: open record %d: %v", num, p)
		}
	}()
	e, err := v.ctx.GetMFT(int64(num))
	if err != nil {
		return nil, fmt.Errorf("NTFS: record %d: %w", num, err)
	}
	rr, err := parser.OpenStream(v.ctx, e, parser.ATTR_TYPE_DATA, parser.WILDCARD_STREAM_ID, parser.WILDCARD_STREAM_NAME)
	if err != nil {
		if size == 0 {
			return &File{r: emptyReader{}, v: v}, nil
		}
		return nil, fmt.Errorf("NTFS: record %d data: %w", num, err)
	}
	return &File{r: rr, v: v, size: size}, nil
}

// Size is the file's length.
func (f *File) Size() int64 { return f.size }

// Extents are the byte ranges of the volume that hold the file's data,
// [start, end), for read-ahead: sparse holes are left out, and a resident
// or empty file has none. For a compressed file they over-cover a little,
// which read-ahead does not mind.
func (f *File) Extents() [][2]int64 {
	rr, ok := f.r.(parser.RangeReaderAt)
	if !ok {
		return nil
	}
	f.v.mu.Lock()
	defer f.v.mu.Unlock()
	var out [][2]int64
	// DebugRuns flattens go-ntfs's run mapping: a byte-level wrapper around
	// cluster-level runs. The leaf runs whose delegate is the volume reader
	// itself (not another go-ntfs reader, nor a resident file's bytes) are
	// the data on disk
	// (offsets and lengths in clusters). Compressed and sparse runs are skipped.
	for _, r := range parser.DebugRuns(rr, 0) {
		if r.Reader != f.v.diskType || r.IsSparse || r.Length <= 0 || r.CompressedLength != 0 {
			continue
		}
		cs := r.ClusterSize
		if cs <= 0 {
			cs = 1
		}
		out = append(out, [2]int64{r.ToOffset * cs, (r.ToOffset + r.Length) * cs})
	}
	return out
}

// ReadAt reads the file's data (zeros in sparse ranges, decompressed if
// NTFS-compressed).
func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= f.size {
		return 0, io.EOF
	}
	want := p
	if rem := f.size - off; int64(len(want)) > rem {
		want = want[:rem]
	}
	f.v.mu.Lock()
	defer f.v.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			n, err = 0, fmt.Errorf("NTFS: read: %v", r)
		}
	}()
	n, err = f.r.ReadAt(want, off)
	if err == io.EOF && n == len(want) {
		err = nil
	}
	if err == nil && n < len(want) {
		// go-ntfs returns short reads past the initialised size: the rest is zeros.
		for i := n; i < len(want); i++ {
			want[i] = 0
		}
		n = len(want)
	}
	if n < len(p) && err == nil {
		err = io.EOF
	}
	return n, err
}

type emptyReader struct{}

func (emptyReader) ReadAt([]byte, int64) (int, error) { return 0, io.EOF }
