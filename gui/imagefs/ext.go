package imagefs

import (
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// Ext is an ext2, ext3 or ext4 volume. Read-only: block maps and extent
// trees, linear and hashed (htree) directories (read linearly), inline data,
// fast and slow symbolic links. Owners, modes and nanosecond times come from
// the inodes.
type Ext struct {
	r              io.ReaderAt
	size           int64
	blockSize      int64
	inodeSize      int64
	inodesPerGroup uint32
	groups         uint32
	inodeTables    []int64 // byte offset of each group's inode table
	label          string
	incompat       uint32
	roCompat       uint32
	hasExtraTimes  bool

	// itCache keeps recently read 64 KiB pieces of the inode tables:
	// listing reads every inode, and neighbours are usually in the same piece.
	itMu    sync.Mutex
	itCache map[int64][]byte

	// SkippedFolders is how many folders the last List could not read.
	SkippedFolders int
}

const itPiece = 64 << 10

const (
	extIncompatFiletype   = 0x2
	extIncompatExtents    = 0x40
	extIncompat64bit      = 0x80
	extIncompatMetaBG     = 0x10
	extIncompatInlineData = 0x8000
	extIncompatEncrypt    = 0x10000

	extFlagExtents    = 0x80000
	extFlagInlineData = 0x10000000

	extRootInode = 2
)

// OpenExt reads the superblock and group descriptors of the volume r holds.
func OpenExt(r io.ReaderAt, size int64) (*Ext, error) {
	sb := make([]byte, 1024)
	if _, err := r.ReadAt(sb, 1024); err != nil {
		return nil, fmt.Errorf("ext: superblock: %w", err)
	}
	if binary.LittleEndian.Uint16(sb[56:]) != 0xEF53 {
		return nil, fmt.Errorf("ext: no superblock")
	}
	le32 := func(o int) uint32 { return binary.LittleEndian.Uint32(sb[o:]) }
	v := &Ext{r: r, size: size}
	logBS := le32(24)
	if logBS > 6 {
		return nil, fmt.Errorf("ext: block size 2^(10+%d) is not supported", logBS)
	}
	v.blockSize = 1024 << logBS
	v.inodesPerGroup = le32(40)
	blocksPerGroup := le32(32)
	revLevel := le32(76)
	v.inodeSize = 128
	if revLevel >= 1 {
		v.inodeSize = int64(binary.LittleEndian.Uint16(sb[88:]))
	}
	v.incompat = le32(96)
	v.roCompat = le32(100)
	v.label = strings.TrimRight(string(sb[120:136]), "\x00")
	if v.incompat&extIncompatMetaBG != 0 {
		return nil, fmt.Errorf("ext: meta_bg layouts are not supported yet")
	}
	blocks := uint64(le32(4))
	descSize := int64(32)
	if v.incompat&extIncompat64bit != 0 {
		blocks |= uint64(le32(336)) << 32
		if ds := int64(binary.LittleEndian.Uint16(sb[254:])); ds >= 64 {
			descSize = ds
		}
	}
	if v.inodesPerGroup == 0 || blocksPerGroup == 0 || v.inodeSize < 128 {
		return nil, fmt.Errorf("ext: superblock sizes are invalid")
	}
	firstData := uint64(le32(20))
	v.groups = uint32((blocks - firstData + uint64(blocksPerGroup) - 1) / uint64(blocksPerGroup))
	if v.groups == 0 || v.groups > 1<<24 {
		return nil, fmt.Errorf("ext: %d block groups is not plausible", v.groups)
	}
	v.hasExtraTimes = v.inodeSize > 128
	// Group descriptors follow the superblock's block.
	gdt := make([]byte, int64(v.groups)*descSize)
	if _, err := r.ReadAt(gdt, int64(firstData+1)*v.blockSize); err != nil && err != io.EOF {
		return nil, fmt.Errorf("ext: group descriptors: %w", err)
	}
	v.inodeTables = make([]int64, v.groups)
	for g := int64(0); g < int64(v.groups); g++ {
		d := gdt[g*descSize:]
		blk := uint64(binary.LittleEndian.Uint32(d[8:]))
		if descSize >= 64 {
			blk |= uint64(binary.LittleEndian.Uint32(d[0x28:])) << 32
		}
		v.inodeTables[g] = int64(blk) * v.blockSize
	}
	return v, nil
}

func (v *Ext) FSName() string {
	switch {
	case v.incompat&(extIncompatExtents|extIncompat64bit|extIncompatInlineData) != 0:
		return "ext4"
	}
	return "ext2/3"
}
func (v *Ext) Label() string         { return v.label }
func (v *Ext) CaseInsensitive() bool { return false }

// extInode is the part of an inode the reader needs.
type extInode struct {
	num    uint32
	mode   uint16
	uid    uint32
	gid    uint32
	size   int64
	mtime  int64 // FILETIME
	ctime  int64 // crtime when recorded, FILETIME
	flags  uint32
	block  [60]byte
	inline []byte // inline data beyond i_block (system.data xattr)
}

func (v *Ext) inode(num uint32) (*extInode, error) {
	if num == 0 || num > v.inodesPerGroup*v.groups {
		return nil, fmt.Errorf("ext: inode %d out of range", num)
	}
	g := (num - 1) / v.inodesPerGroup
	idx := int64((num - 1) % v.inodesPerGroup)
	b, err := v.inodeBytes(v.inodeTables[g] + idx*v.inodeSize)
	if err != nil {
		return nil, fmt.Errorf("ext: inode %d: %w", num, err)
	}
	le16 := func(o int) uint32 { return uint32(binary.LittleEndian.Uint16(b[o:])) }
	le32 := func(o int) uint32 { return binary.LittleEndian.Uint32(b[o:]) }
	in := &extInode{num: num, mode: uint16(le16(0)), flags: le32(32)}
	in.uid = le16(2) | le16(120)<<16
	in.gid = le16(24) | le16(122)<<16
	in.size = int64(le32(4)) | int64(le32(108))<<32
	copy(in.block[:], b[40:100])
	mtime, mextra := int64(int32(le32(16))), uint32(0)
	extra := int64(0)
	if v.inodeSize > 128 {
		extra = int64(le16(128))
	}
	if extra >= 12 { // i_mtime_extra at 0x88
		mextra = le32(0x88)
	}
	in.mtime = extTime(mtime, mextra)
	if extra >= 24 { // i_crtime at 0x90, i_crtime_extra at 0x94
		if cr := int64(int32(le32(0x90))); cr != 0 {
			in.ctime = extTime(cr, le32(0x94))
		}
	}
	if in.flags&extFlagInlineData != 0 && extra > 0 {
		in.inline = inlineXattr(b[128+extra:])
	}
	return in, nil
}

// inodeBytes returns the inode at byte offset off, through itCache. The
// lock is not held while reading, so several folders can be listed at once
// (two may read the same piece; both results are the same).
func (v *Ext) inodeBytes(off int64) ([]byte, error) {
	piece := off - off%itPiece
	v.itMu.Lock()
	buf, ok := v.itCache[piece]
	v.itMu.Unlock()
	if !ok {
		buf = make([]byte, itPiece)
		n, err := v.r.ReadAt(buf, piece)
		if err != nil && err != io.EOF {
			return nil, err
		}
		buf = buf[:n]
		v.itMu.Lock()
		if v.itCache == nil || len(v.itCache) >= 512 { // at most 32 MiB
			v.itCache = map[int64][]byte{}
		}
		v.itCache[piece] = buf
		v.itMu.Unlock()
	}
	in := off - piece
	if in+v.inodeSize > int64(len(buf)) {
		return nil, io.ErrUnexpectedEOF
	}
	return append([]byte(nil), buf[in:in+v.inodeSize]...), nil
}

// extTime turns seconds and the *_extra field (2 epoch bits, 30 bits of
// nanoseconds) into a FILETIME.
func extTime(sec int64, extra uint32) int64 {
	sec += int64(extra&3) << 32
	return unixFiletime(sec, int64(extra>>2))
}

// inlineXattr finds the "system.data" in-inode extended attribute (name
// index 7, name "data"), where inline data longer than i_block continues.
func inlineXattr(b []byte) []byte {
	if len(b) < 4 || binary.LittleEndian.Uint32(b) != 0xEA020000 {
		return nil
	}
	base := b[4:]
	for o := 0; o+16 <= len(base); {
		nameLen := int(base[o])
		idx := base[o+1]
		if nameLen == 0 && idx == 0 {
			break
		}
		valOff := int(binary.LittleEndian.Uint16(base[o+2:]))
		valSize := int(binary.LittleEndian.Uint32(base[o+8:]))
		if o+16+nameLen > len(base) {
			break
		}
		name := string(base[o+16 : o+16+nameLen])
		if idx == 7 && name == "data" && valOff+valSize <= len(base) {
			return base[valOff : valOff+valSize]
		}
		o += (16 + nameLen + 3) &^ 3
	}
	return nil
}

func (in *extInode) isDir() bool     { return in.mode&0xF000 == 0x4000 }
func (in *extInode) isReg() bool     { return in.mode&0xF000 == 0x8000 }
func (in *extInode) isSymlink() bool { return in.mode&0xF000 == 0xA000 }

// extent is one mapped run of a file: logical block, physical block, length.
type extent struct {
	logical   uint64
	phys      uint64
	length    uint64
	unwritten bool
}

// extents maps a file's logical blocks to disk blocks (extent tree or the
// classic block map). Holes are left out.
func (v *Ext) extents(in *extInode) ([]extent, error) {
	if in.flags&extFlagExtents != 0 {
		var out []extent
		err := v.walkExtentNode(in.block[:], 0, &out)
		return out, err
	}
	nblocks := uint64((in.size + v.blockSize - 1) / v.blockSize)
	var out []extent
	add := func(logical, phys uint64) {
		if phys == 0 {
			return
		}
		if n := len(out); n > 0 && out[n-1].logical+out[n-1].length == logical && out[n-1].phys+out[n-1].length == phys {
			out[n-1].length++
			return
		}
		out = append(out, extent{logical: logical, phys: phys, length: 1})
	}
	ptr := func(i int) uint64 { return uint64(binary.LittleEndian.Uint32(in.block[i*4:])) }
	for i := 0; i < 12 && uint64(i) < nblocks; i++ {
		add(uint64(i), ptr(i))
	}
	per := uint64(v.blockSize / 4)
	logical := uint64(12)
	var walk func(blk uint64, depth int) error
	walk = func(blk uint64, depth int) error {
		span := uint64(1)
		for d := 0; d < depth; d++ {
			span *= per
		}
		if blk == 0 {
			logical += span * per
			return nil
		}
		b := make([]byte, v.blockSize)
		if _, err := v.r.ReadAt(b, int64(blk)*v.blockSize); err != nil && err != io.EOF {
			return err
		}
		for k := uint64(0); k < per && logical < nblocks; k++ {
			p := uint64(binary.LittleEndian.Uint32(b[k*4:]))
			if depth == 0 {
				add(logical, p)
				logical++
				continue
			}
			if err := walk(p, depth-1); err != nil {
				return err
			}
		}
		return nil
	}
	for lvl, i := 0, 12; i < 15 && logical < nblocks; lvl, i = lvl+1, i+1 {
		if err := walk(ptr(i), lvl); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (v *Ext) walkExtentNode(node []byte, depth int, out *[]extent) error {
	if depth > 8 || len(node) < 12 || binary.LittleEndian.Uint16(node) != 0xF30A {
		return fmt.Errorf("ext: bad extent tree")
	}
	entries := int(binary.LittleEndian.Uint16(node[2:]))
	level := binary.LittleEndian.Uint16(node[6:])
	for i := 0; i < entries; i++ {
		e := node[12+12*i:]
		if len(e) < 12 {
			return fmt.Errorf("ext: extent tree overruns its node")
		}
		if level == 0 {
			l := uint64(binary.LittleEndian.Uint16(e[4:]))
			unwritten := false
			if l > 32768 {
				l -= 32768
				unwritten = true
			}
			start := uint64(binary.LittleEndian.Uint16(e[6:]))<<32 | uint64(binary.LittleEndian.Uint32(e[8:]))
			*out = append(*out, extent{logical: uint64(binary.LittleEndian.Uint32(e)), phys: start, length: l, unwritten: unwritten})
			continue
		}
		leaf := uint64(binary.LittleEndian.Uint32(e[4:])) | uint64(binary.LittleEndian.Uint16(e[8:]))<<32
		b := make([]byte, v.blockSize)
		if _, err := v.r.ReadAt(b, int64(leaf)*v.blockSize); err != nil && err != io.EOF {
			return err
		}
		if err := v.walkExtentNode(b, depth+1, out); err != nil {
			return err
		}
	}
	return nil
}

// content returns a whole (small) file or directory's data.
func (v *Ext) content(in *extInode) ([]byte, error) {
	if in.flags&extFlagInlineData != 0 {
		d := append(append([]byte{}, in.block[:]...), in.inline...)
		if int64(len(d)) > in.size {
			d = d[:in.size]
		}
		return d, nil
	}
	if in.size > 1<<30 {
		return nil, fmt.Errorf("ext: inode %d: %d bytes is too large to read whole", in.num, in.size)
	}
	f, err := v.openInode(in)
	if err != nil {
		return nil, err
	}
	b := make([]byte, in.size)
	if _, err := f.ReadAt(b, 0); err != nil && err != io.EOF {
		return nil, err
	}
	return b, nil
}

// List walks the directory tree from the root and returns every file,
// folder and symbolic link, sorted by path (devices, pipes and sockets are
// left out). The folders of each level are read by several workers at once:
// on a disk image every folder block and inode table piece can be a separate
// chunk download, and waiting for them one by one dominates. progress gets
// the folders read so far.
func (v *Ext) List(cancel func() bool, progress func(done, total int64)) ([]Entry, error) {
	type dir struct {
		path  string
		ino   uint32
		depth int
	}
	type result struct {
		entries []Entry
		subdirs []dir
		err     error
	}
	readDir := func(d dir) (res result) {
		in, err := v.inode(d.ino)
		if err != nil {
			return result{err: err}
		}
		data, err := v.content(in)
		if err != nil {
			return result{err: fmt.Errorf("ext: folder %q: %w", d.path, err)}
		}
		start := 0
		if in.flags&extFlagInlineData != 0 {
			start = 4 // inline directories start with the parent's inode number
		}
		if start > len(data) {
			return result{}
		}
		names, err := v.parseDir(data[start:], in.flags&extFlagInlineData != 0)
		if err != nil {
			return result{err: fmt.Errorf("ext: folder %q: %w", d.path, err)}
		}
		for _, de := range names {
			if de.name == "." || de.name == ".." || (d.ino == extRootInode && de.name == "lost+found") {
				continue
			}
			child, err := v.inode(de.ino)
			if err != nil {
				continue
			}
			p := de.name
			if d.path != "" {
				p = d.path + "/" + de.name
			}
			e := Entry{Path: p, MTime: child.mtime, Created: child.ctime, Record: uint64(child.num),
				Mode: uint32(child.mode), UID: child.uid, GID: child.gid, HasOwner: true}
			switch {
			case child.isDir():
				e.IsDir = true
				if d.depth < 4096 {
					res.subdirs = append(res.subdirs, dir{path: p, ino: child.num, depth: d.depth + 1})
				}
			case child.isReg():
				e.Size = uint64(child.size)
			case child.isSymlink():
				e.Symlink = true
				if t, err := v.readLink(child); err == nil {
					e.LinkTarget = t
				}
			default:
				continue
			}
			res.entries = append(res.entries, e)
		}
		return res
	}

	const workers = 8
	var out []Entry
	level := []dir{{ino: extRootInode}}
	seenDir := map[uint32]bool{extRootInode: true}
	done := int64(0)
	skipped := 0
	for len(level) > 0 {
		if cancel != nil && cancel() {
			return nil, ErrCancelled
		}
		results := make([]result, len(level))
		var wg sync.WaitGroup
		next := make(chan int)
		for w := 0; w < workers && w < len(level); w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range next {
					if cancel != nil && cancel() {
						results[i] = result{err: ErrCancelled}
						continue
					}
					results[i] = readDir(level[i])
				}
			}()
		}
		for i := range level {
			next <- i
		}
		close(next)
		wg.Wait()
		var following []dir
		for i, r := range results {
			if r.err != nil {
				// A folder that cannot be read (a damaged block) is left out
				// rather than losing the whole listing; the root is needed.
				if r.err == ErrCancelled || level[i].ino == extRootInode {
					return nil, r.err
				}
				skipped++
				continue
			}
			out = append(out, r.entries...)
			for _, sd := range r.subdirs {
				if !seenDir[sd.ino] { // a loop in a damaged volume is read once
					seenDir[sd.ino] = true
					following = append(following, sd)
				}
			}
		}
		done += int64(len(level))
		if progress != nil {
			progress(done, 0)
		}
		level = following
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	v.SkippedFolders = skipped
	return out, nil
}

type extDirent struct {
	ino  uint32
	name string
}

// parseDir reads ext directory entries from a directory's data. Hashed
// (htree) directories keep their entries in the same format; the index
// blocks look like one empty entry spanning the block and are skipped.
func (v *Ext) parseDir(b []byte, inline bool) ([]extDirent, error) {
	var out []extDirent
	for o := 0; o+8 <= len(b); {
		ino := binary.LittleEndian.Uint32(b[o:])
		recLen := int(binary.LittleEndian.Uint16(b[o+4:]))
		nameLen := int(b[o+6])
		if v.incompat&extIncompatFiletype == 0 {
			nameLen |= int(b[o+7]) << 8
		}
		if recLen < 8 || o+recLen > len(b) {
			if inline {
				break
			}
			// Skip to the next block on a damaged entry.
			next := (o/int(v.blockSize) + 1) * int(v.blockSize)
			if next <= o {
				break
			}
			o = next
			continue
		}
		if ino != 0 && nameLen > 0 && o+8+nameLen <= len(b) {
			out = append(out, extDirent{ino: ino, name: string(b[o+8 : o+8+nameLen])})
		}
		o += recLen
	}
	return out, nil
}

func (v *Ext) readLink(in *extInode) (string, error) {
	// A fast symlink keeps a target shorter than 60 bytes in i_block.
	if in.size < 60 && in.flags&extFlagExtents == 0 {
		return string(in.block[:in.size]), nil
	}
	b, err := v.content(in)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Open opens a file entry from List.
func (v *Ext) Open(e Entry) (FileReader, error) {
	in, err := v.inode(uint32(e.Record))
	if err != nil {
		return nil, err
	}
	return v.openInode(in)
}

func (v *Ext) openInode(in *extInode) (FileReader, error) {
	if in.flags&extFlagInlineData != 0 {
		d, _ := v.content(in)
		return &bytesFile{b: d}, nil
	}
	ex, err := v.extents(in)
	if err != nil {
		return nil, err
	}
	return &extFile{v: v, size: in.size, ex: ex}, nil
}

// extFile reads a file through its extent list; unmapped and unwritten
// ranges read as zeros.
type extFile struct {
	v    *Ext
	size int64
	ex   []extent
}

func (f *extFile) Size() int64 { return f.size }

func (f *extFile) Extents() [][2]int64 {
	out := make([][2]int64, 0, len(f.ex))
	for _, e := range f.ex {
		if e.unwritten {
			continue
		}
		out = append(out, [2]int64{int64(e.phys) * f.v.blockSize, int64(e.phys+e.length) * f.v.blockSize})
	}
	return out
}

func (f *extFile) ReadAt(p []byte, off int64) (int, error) {
	if off >= f.size {
		return 0, io.EOF
	}
	want := p
	if rem := f.size - off; int64(len(want)) > rem {
		want = want[:rem]
	}
	bs := f.v.blockSize
	n := 0
	for n < len(want) {
		pos := off + int64(n)
		lb := uint64(pos / bs)
		in := pos % bs
		// Find the extent holding lb (extents are sorted by logical block).
		i := sort.Search(len(f.ex), func(i int) bool { return f.ex[i].logical+f.ex[i].length > lb })
		var run int64
		if i < len(f.ex) && f.ex[i].logical <= lb && !f.ex[i].unwritten {
			e := f.ex[i]
			run = int64(e.logical+e.length-lb)*bs - in
			if run > int64(len(want)-n) {
				run = int64(len(want) - n)
			}
			disk := int64(e.phys+(lb-e.logical))*bs + in
			if _, err := f.v.r.ReadAt(want[n:n+int(run)], disk); err != nil && err != io.EOF {
				return n, err
			}
		} else {
			// A hole (or unwritten extent): zeros up to the next mapped block.
			next := int64(len(want) - n)
			if i < len(f.ex) && f.ex[i].logical > lb {
				if gap := int64(f.ex[i].logical-lb)*bs - in; gap < next {
					next = gap
				}
			} else if i < len(f.ex) && f.ex[i].unwritten {
				if r := int64(f.ex[i].logical+f.ex[i].length-lb)*bs - in; r < next {
					next = r
				}
			}
			run = next
			for k := n; k < n+int(run); k++ {
				want[k] = 0
			}
		}
		n += int(run)
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// bytesFile is a file held in memory (inline data).
type bytesFile struct{ b []byte }

func (f *bytesFile) Size() int64         { return int64(len(f.b)) }
func (f *bytesFile) Extents() [][2]int64 { return nil }
func (f *bytesFile) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(f.b)) {
		return 0, io.EOF
	}
	n := copy(p, f.b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
