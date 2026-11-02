package imagefs

import (
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"time"
	"unicode/utf16"
)

// ExFAT is an exFAT volume (large memory cards and USB drives). Read-only.
type ExFAT struct {
	r           io.ReaderAt
	label       string
	clusterSize int64
	heapStart   int64 // byte offset of cluster 2
	clusters    uint32
	rootCluster uint32
	fat         []byte
}

// An exFAT Entry's Record holds the first cluster, with exfatNoChain set
// when the file's clusters are contiguous (no FAT chain); Aux holds its
// valid data length (past it the file reads as zeros).
const exfatNoChain = 1 << 63

// OpenExFAT reads the boot sector and the FAT of the volume r holds.
func OpenExFAT(r io.ReaderAt, size int64) (*ExFAT, error) {
	b := make([]byte, 512)
	if _, err := r.ReadAt(b, 0); err != nil {
		return nil, fmt.Errorf("exFAT: boot sector: %w", err)
	}
	if string(b[3:11]) != "EXFAT   " {
		return nil, fmt.Errorf("exFAT: not an exFAT boot sector")
	}
	bpsShift, spcShift := uint(b[108]), uint(b[109])
	if bpsShift < 9 || bpsShift > 12 || spcShift > 25-bpsShift {
		return nil, fmt.Errorf("exFAT: bad sector or cluster size")
	}
	bps := int64(1) << bpsShift
	v := &ExFAT{
		r:           r,
		clusterSize: bps << spcShift,
		heapStart:   int64(binary.LittleEndian.Uint32(b[88:])) * bps,
		clusters:    binary.LittleEndian.Uint32(b[92:]),
		rootCluster: binary.LittleEndian.Uint32(b[96:]),
	}
	fatOff := int64(binary.LittleEndian.Uint32(b[80:])) * bps
	fatLen := int64(v.clusters+2) * 4
	if max := int64(binary.LittleEndian.Uint32(b[84:])) * bps; fatLen > max {
		fatLen = max
	}
	v.fat = make([]byte, fatLen)
	if _, err := r.ReadAt(v.fat, fatOff); err != nil && err != io.EOF {
		return nil, fmt.Errorf("exFAT: reading the FAT: %w", err)
	}
	// The volume label is an entry of the root directory.
	if root, err := v.readChain(v.rootCluster, false, 0); err == nil {
		for o := 0; o+32 <= len(root); o += 32 {
			if root[o] == 0 {
				break
			}
			if root[o] == 0x83 {
				n := int(root[o+1])
				if n > 11 {
					n = 11
				}
				u := make([]uint16, n)
				for i := range u {
					u[i] = binary.LittleEndian.Uint16(root[o+2+2*i:])
				}
				v.label = string(utf16.Decode(u))
			}
		}
	}
	return v, nil
}

func (v *ExFAT) FSName() string        { return "exFAT" }
func (v *ExFAT) Label() string         { return v.label }
func (v *ExFAT) CaseInsensitive() bool { return true }

func (v *ExFAT) clusterOffset(c uint32) int64 { return v.heapStart + int64(c-2)*v.clusterSize }

// clusterList lists the clusters of a stream: contiguous when noChain (the
// NoFatChain flag), else by following the FAT. want is the number of
// clusters the stream needs (0 = follow the chain to its end).
func (v *ExFAT) clusterList(first uint32, noChain bool, want int64) []int64 {
	var out []int64
	if first < 2 || first >= v.clusters+2 {
		return nil
	}
	if noChain {
		for i := int64(0); i < want && first+uint32(i) < v.clusters+2; i++ {
			out = append(out, v.clusterOffset(first+uint32(i)))
		}
		return out
	}
	limit := want
	if limit <= 0 {
		limit = 1 << 20
	}
	for c := first; c >= 2 && c < v.clusters+2 && int64(len(out)) < limit; {
		out = append(out, v.clusterOffset(c))
		o := int(c) * 4
		if o+4 > len(v.fat) {
			break
		}
		c = binary.LittleEndian.Uint32(v.fat[o:])
	}
	return out
}

// readChain reads a directory's data.
func (v *ExFAT) readChain(first uint32, noChain bool, size int64) ([]byte, error) {
	want := int64(0)
	if size > 0 {
		want = (size + v.clusterSize - 1) / v.clusterSize
	}
	cl := v.clusterList(first, noChain, want)
	b := make([]byte, int64(len(cl))*v.clusterSize)
	for i, off := range cl {
		if _, err := v.r.ReadAt(b[int64(i)*v.clusterSize:int64(i+1)*v.clusterSize], off); err != nil && err != io.EOF {
			return nil, err
		}
	}
	return b, nil
}

// exfatTime decodes an exFAT timestamp, its 10 ms increment and UTC offset.
func exfatTime(ts uint32, ms10 byte, utcOff byte) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	loc := FATTimeLocation
	if utcOff&0x80 != 0 {
		q := int(int8(utcOff<<1) >> 1) // 7-bit signed, 15-minute steps
		loc = time.FixedZone("", q*15*60)
	}
	t := time.Date(1980+int(ts>>25), time.Month(ts>>21&15), int(ts>>16&31),
		int(ts>>11&31), int(ts>>5&63), int(ts&31)*2, 0, loc)
	if ms10 < 200 {
		t = t.Add(time.Duration(ms10) * 10 * time.Millisecond)
	}
	return t
}

// exfatFile is what an exFAT entry set (file, stream, names) says.
type exfatFile struct {
	name    string
	attrs   uint16
	mtime   time.Time
	ctime   time.Time
	first   uint32
	noChain bool
	size    int64
	valid   int64
}

// List returns every file and folder, sorted by path.
func (v *ExFAT) List(cancel func() bool, progress func(done, total int64)) ([]Entry, error) {
	var out []Entry
	type dir struct {
		path    string
		first   uint32
		noChain bool
		size    int64
		depth   int
	}
	queue := []dir{{first: v.rootCluster}}
	for len(queue) > 0 {
		if cancel != nil && cancel() {
			return nil, ErrCancelled
		}
		d := queue[0]
		queue = queue[1:]
		raw, err := v.readChain(d.first, d.noChain, d.size)
		if err != nil {
			return nil, fmt.Errorf("exFAT: reading folder %q: %w", d.path, err)
		}
		for o := 0; o+32 <= len(raw); {
			t := raw[o]
			if t == 0 {
				break
			}
			if t != 0x85 { // only file entry sets start a file; skip the rest
				o += 32
				continue
			}
			sec := int(raw[o+1])
			if sec < 2 || o+32*(sec+1) > len(raw) {
				o += 32
				continue
			}
			set := raw[o : o+32*(sec+1)]
			o += 32 * (sec + 1)
			if set[32] != 0xC0 {
				continue
			}
			f := exfatFile{
				attrs: binary.LittleEndian.Uint16(set[4:]),
				ctime: exfatTime(binary.LittleEndian.Uint32(set[8:]), set[20], set[22]),
				mtime: exfatTime(binary.LittleEndian.Uint32(set[12:]), set[21], set[23]),
			}
			st := set[32:64]
			f.noChain = st[1]&2 != 0
			nameLen := int(st[3])
			f.valid = int64(binary.LittleEndian.Uint64(st[8:]))
			f.first = binary.LittleEndian.Uint32(st[20:])
			f.size = int64(binary.LittleEndian.Uint64(st[24:]))
			var name []uint16
			for i := 2; i <= sec && len(name) < nameLen; i++ {
				ne := set[32*i : 32*(i+1)]
				if ne[0] != 0xC1 {
					break
				}
				for k := 2; k < 32 && len(name) < nameLen; k += 2 {
					name = append(name, binary.LittleEndian.Uint16(ne[k:]))
				}
			}
			f.name = string(utf16.Decode(name))
			if f.name == "" {
				continue
			}
			p := f.name
			if d.path != "" {
				p = d.path + "/" + f.name
			}
			isDir := f.attrs&0x10 != 0
			e := Entry{Path: p, IsDir: isDir, MTime: ftOrZero(f.mtime), Created: ftOrZero(f.ctime),
				Attrs: uint32(f.attrs & 0x27), Record: uint64(f.first), Aux: uint64(f.valid)}
			if f.noChain {
				e.Record |= exfatNoChain
			}
			if !isDir {
				e.Size = uint64(f.size)
			}
			out = append(out, e)
			if isDir && d.depth < 256 {
				queue = append(queue, dir{path: p, first: f.first, noChain: f.noChain, size: f.size, depth: d.depth + 1})
			}
		}
		if progress != nil {
			progress(int64(len(out)), 0)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Open opens a file entry from List.
func (v *ExFAT) Open(e Entry) (FileReader, error) {
	f := exfatFile{first: uint32(e.Record), noChain: e.Record&exfatNoChain != 0, size: int64(e.Size), valid: int64(e.Aux)}
	cf := &clusterFile{r: v.r, size: f.size, zeroFrom: f.size, clusterSize: v.clusterSize}
	if f.valid >= 0 && f.valid < f.size {
		cf.zeroFrom = f.valid
	}
	if f.size == 0 {
		return cf, nil
	}
	need := (f.size + v.clusterSize - 1) / v.clusterSize
	cf.clusters = v.clusterList(f.first, f.noChain, need)
	if int64(len(cf.clusters)) < need {
		return nil, fmt.Errorf("exFAT: %s: fewer clusters than the file needs", e.Path)
	}
	return cf, nil
}
