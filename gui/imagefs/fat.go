package imagefs

import (
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf16"
)

// FAT is a FAT12, FAT16 or FAT32 volume (EFI system partitions, memory cards,
// USB sticks). Read-only; long file names (VFAT) are read, 8.3 names are used
// where there is no long name.
type FAT struct {
	r           io.ReaderAt
	kind        string // "FAT12", "FAT16", "FAT32"
	label       string
	bps         int64 // bytes per sector
	clusterSize int64
	dataStart   int64  // byte offset of cluster 2
	rootStart   int64  // FAT12/16: byte offset of the fixed root directory
	rootSize    int64  // FAT12/16: its size
	rootCluster uint32 // FAT32: first cluster of the root directory
	clusters    uint32 // number of data clusters
	fat         []byte
}

// OpenFAT reads the boot sector and the first FAT of the volume r holds.
func OpenFAT(r io.ReaderAt, size int64) (*FAT, error) {
	b := make([]byte, 512)
	if _, err := r.ReadAt(b, 0); err != nil {
		return nil, fmt.Errorf("FAT: boot sector: %w", err)
	}
	le16 := func(o int) int64 { return int64(binary.LittleEndian.Uint16(b[o:])) }
	le32 := func(o int) int64 { return int64(binary.LittleEndian.Uint32(b[o:])) }
	bps := le16(11)
	spc := int64(b[13])
	rsvd := le16(14)
	nfats := int64(b[16])
	rootEnt := le16(17)
	tot := le16(19)
	if tot == 0 {
		tot = le32(32)
	}
	fatSz := le16(22)
	if fatSz == 0 {
		fatSz = le32(36)
	}
	if bps < 512 || bps > 4096 || bps&(bps-1) != 0 || spc == 0 || spc&(spc-1) != 0 || nfats == 0 || fatSz == 0 || tot == 0 {
		return nil, fmt.Errorf("FAT: not a valid boot sector")
	}
	rootSecs := (rootEnt*32 + bps - 1) / bps
	firstData := rsvd + nfats*fatSz + rootSecs
	if firstData >= tot {
		return nil, fmt.Errorf("FAT: boot sector sizes do not fit the volume")
	}
	v := &FAT{r: r, bps: bps, clusterSize: bps * spc, dataStart: firstData * bps,
		rootStart: (rsvd + nfats*fatSz) * bps, rootSize: rootSecs * bps}
	v.clusters = uint32((tot - firstData) / spc)
	labelAt := 43
	switch {
	case v.clusters < 4085:
		v.kind = "FAT12"
	case v.clusters < 65525:
		v.kind = "FAT16"
	default:
		v.kind = "FAT32"
		v.rootCluster = uint32(le32(44))
		labelAt = 71
	}
	if lbl := strings.TrimSpace(string(b[labelAt : labelAt+11])); lbl != "NO NAME" {
		v.label = lbl
	}
	// The FAT entries the clusters need, not the whole table (FAT32 tables
	// are allocated generously).
	need := int64(v.clusters+2) * 4
	switch v.kind {
	case "FAT12":
		need = int64(v.clusters+2)*3/2 + 2
	case "FAT16":
		need = int64(v.clusters+2) * 2
	}
	if need > fatSz*bps {
		need = fatSz * bps
	}
	v.fat = make([]byte, need)
	if _, err := r.ReadAt(v.fat, rsvd*bps); err != nil && err != io.EOF {
		return nil, fmt.Errorf("FAT: reading the table: %w", err)
	}
	return v, nil
}

func (v *FAT) FSName() string        { return v.kind }
func (v *FAT) Label() string         { return v.label }
func (v *FAT) CaseInsensitive() bool { return true }

// next is the FAT entry for cluster c: the next cluster of its chain, or
// 0 (free) / >= end-of-chain / bad.
func (v *FAT) next(c uint32) uint32 {
	switch v.kind {
	case "FAT12":
		o := int(c) * 3 / 2
		if o+1 >= len(v.fat) {
			return 0xFFFFFFFF
		}
		x := uint32(binary.LittleEndian.Uint16(v.fat[o:]))
		if c&1 == 1 {
			x >>= 4
		}
		x &= 0xFFF
		if x >= 0xFF7 {
			return 0xFFFFFFFF
		}
		return x
	case "FAT16":
		o := int(c) * 2
		if o+2 > len(v.fat) {
			return 0xFFFFFFFF
		}
		x := uint32(binary.LittleEndian.Uint16(v.fat[o:]))
		if x >= 0xFFF7 {
			return 0xFFFFFFFF
		}
		return x
	}
	o := int(c) * 4
	if o+4 > len(v.fat) {
		return 0xFFFFFFFF
	}
	x := binary.LittleEndian.Uint32(v.fat[o:]) & 0x0FFFFFFF
	if x >= 0x0FFFFFF7 {
		return 0xFFFFFFFF
	}
	return x
}

// chain follows a cluster chain from first, stopping at its end, at a
// cluster out of range, or after limit clusters (a loop in a damaged table).
func (v *FAT) chain(first uint32, limit int64) []uint32 {
	var out []uint32
	for c := first; c >= 2 && c < v.clusters+2 && int64(len(out)) < limit; c = v.next(c) {
		out = append(out, c)
	}
	return out
}

func (v *FAT) clusterOffset(c uint32) int64 { return v.dataStart + int64(c-2)*v.clusterSize }

// readDir returns the raw directory entries of the directory starting at
// cluster first (root: the fixed root area on FAT12/16, rootCluster on FAT32).
func (v *FAT) readDir(first uint32, root bool) ([]byte, error) {
	if root && v.kind != "FAT32" {
		b := make([]byte, v.rootSize)
		_, err := v.r.ReadAt(b, v.rootStart)
		if err == io.EOF {
			err = nil
		}
		return b, err
	}
	if root {
		first = v.rootCluster
	}
	cl := v.chain(first, 1<<20)
	b := make([]byte, int64(len(cl))*v.clusterSize)
	for i, c := range cl {
		if _, err := v.r.ReadAt(b[int64(i)*v.clusterSize:int64(i+1)*v.clusterSize], v.clusterOffset(c)); err != nil && err != io.EOF {
			return nil, err
		}
	}
	return b, nil
}

// fatTime decodes a FAT date and time (2-second steps) plus hundredths.
func fatTime(date, tm uint16, hundredths byte) time.Time {
	if date == 0 {
		return time.Time{}
	}
	t := time.Date(1980+int(date>>9), time.Month(date>>5&15), int(date&31),
		int(tm>>11), int(tm>>5&63), int(tm&31)*2, 0, FATTimeLocation)
	if hundredths > 0 && hundredths < 200 {
		t = t.Add(time.Duration(hundredths) * 10 * time.Millisecond)
	}
	return t
}

func ftOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return Filetime(t)
}

// List returns every file and folder, sorted by path.
func (v *FAT) List(cancel func() bool, progress func(done, total int64)) ([]Entry, error) {
	var out []Entry
	type dir struct {
		path  string
		first uint32
		root  bool
		depth int
	}
	queue := []dir{{root: true}}
	seen := map[uint32]bool{}
	for len(queue) > 0 {
		if cancel != nil && cancel() {
			return nil, ErrCancelled
		}
		d := queue[0]
		queue = queue[1:]
		raw, err := v.readDir(d.first, d.root)
		if err != nil {
			return nil, fmt.Errorf("%s: reading folder %q: %w", v.kind, d.path, err)
		}
		var lfn []uint16
		var lfnSum byte
		for o := 0; o+32 <= len(raw); o += 32 {
			e := raw[o : o+32]
			if e[0] == 0 {
				break
			}
			if e[0] == 0xE5 {
				lfn = nil
				continue
			}
			attr := e[11]
			if attr&0x3F == 0x0F { // long name part, last part first
				if e[0]&0x40 != 0 {
					lfn = nil
				}
				part := make([]uint16, 0, 13)
				for _, r := range [][2]int{{1, 11}, {14, 26}, {28, 32}} {
					for i := r[0]; i < r[1]; i += 2 {
						part = append(part, binary.LittleEndian.Uint16(e[i:]))
					}
				}
				lfn = append(part, lfn...)
				lfnSum = e[13]
				continue
			}
			if attr&0x08 != 0 { // volume label
				if d.root && v.label == "" {
					v.label = strings.TrimSpace(string(e[0:11]))
				}
				lfn = nil
				continue
			}
			name := shortName(e)
			if lfn != nil && lfnChecksum(e[0:11]) == lfnSum {
				for i, c := range lfn {
					if c == 0 {
						lfn = lfn[:i]
						break
					}
				}
				name = string(utf16.Decode(lfn))
			}
			lfn = nil
			if name == "." || name == ".." || name == "" {
				continue
			}
			first := uint32(binary.LittleEndian.Uint16(e[26:])) | uint32(binary.LittleEndian.Uint16(e[20:]))<<16
			p := name
			if d.path != "" {
				p = d.path + "/" + name
			}
			ent := Entry{
				Path:    p,
				IsDir:   attr&0x10 != 0,
				MTime:   ftOrZero(fatTime(binary.LittleEndian.Uint16(e[24:]), binary.LittleEndian.Uint16(e[22:]), 0)),
				Created: ftOrZero(fatTime(binary.LittleEndian.Uint16(e[16:]), binary.LittleEndian.Uint16(e[14:]), e[13])),
				Attrs:   uint32(attr & 0x27), // read-only, hidden, system, archive
				Record:  uint64(first),
			}
			if !ent.IsDir {
				ent.Size = uint64(binary.LittleEndian.Uint32(e[28:]))
			}
			out = append(out, ent)
			if ent.IsDir && first >= 2 && !seen[first] && d.depth < 256 {
				seen[first] = true
				queue = append(queue, dir{path: p, first: first, depth: d.depth + 1})
			}
		}
		if progress != nil {
			progress(int64(len(out)), 0)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// shortName decodes an 8.3 entry name, with the lower-case flags Windows
// sets for names like "readme.txt" that fit 8.3 in one case.
func shortName(e []byte) string {
	base := strings.TrimRight(string(e[0:8]), " ")
	ext := strings.TrimRight(string(e[8:11]), " ")
	if len(base) > 0 && base[0] == 0x05 {
		base = "\xe5" + base[1:]
	}
	if e[12]&0x08 != 0 {
		base = strings.ToLower(base)
	}
	if e[12]&0x10 != 0 {
		ext = strings.ToLower(ext)
	}
	if ext == "" {
		return base
	}
	return base + "." + ext
}

func lfnChecksum(n []byte) byte {
	var s byte
	for _, c := range n {
		s = (s>>1 | s<<7) + c
	}
	return s
}

// Open opens a file entry from List.
func (v *FAT) Open(e Entry) (FileReader, error) {
	f := &clusterFile{r: v.r, size: int64(e.Size), zeroFrom: int64(e.Size), clusterSize: v.clusterSize}
	if e.Size == 0 {
		return f, nil
	}
	need := (int64(e.Size) + v.clusterSize - 1) / v.clusterSize
	for _, c := range v.chain(uint32(e.Record), need) {
		f.clusters = append(f.clusters, v.clusterOffset(c))
	}
	if int64(len(f.clusters)) < need {
		return nil, fmt.Errorf("%s: %s: the cluster chain is shorter than the file", v.kind, e.Path)
	}
	return f, nil
}

// clusterFile reads a file stored in a list of clusters (FAT chains, and
// exFAT files with or without a chain).
type clusterFile struct {
	r           io.ReaderAt
	size        int64
	zeroFrom    int64 // bytes from here on read as zeros (exFAT ValidDataLength); = size when all valid
	clusterSize int64
	clusters    []int64 // byte offset of each cluster in the volume
}

func (f *clusterFile) Size() int64 { return f.size }

func (f *clusterFile) Extents() [][2]int64 {
	var out [][2]int64
	for _, c := range f.clusters {
		if n := len(out); n > 0 && out[n-1][1] == c {
			out[n-1][1] = c + f.clusterSize
			continue
		}
		out = append(out, [2]int64{c, c + f.clusterSize})
	}
	return out
}

func (f *clusterFile) ReadAt(p []byte, off int64) (int, error) {
	if off >= f.size {
		return 0, io.EOF
	}
	n := 0
	for n < len(p) && off < f.size {
		ci := off / f.clusterSize
		in := off % f.clusterSize
		chunk := f.clusterSize - in
		if rem := f.size - off; chunk > rem {
			chunk = rem
		}
		if chunk > int64(len(p)-n) {
			chunk = int64(len(p) - n)
		}
		dst := p[n : n+int(chunk)]
		if off >= f.zeroFrom {
			for i := range dst {
				dst[i] = 0
			}
		} else {
			if int(ci) >= len(f.clusters) {
				return n, io.ErrUnexpectedEOF
			}
			if _, err := f.r.ReadAt(dst, f.clusters[ci]+in); err != nil && err != io.EOF {
				return n, err
			}
			if off+chunk > f.zeroFrom {
				for i := f.zeroFrom - off; i < chunk; i++ {
					dst[i] = 0
				}
			}
		}
		n += int(chunk)
		off += chunk
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
