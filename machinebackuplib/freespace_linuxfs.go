package machinebackuplib

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Free-space maps of the Linux file systems, read from a partition (the
// snapshot device of a mounted one, so the map is the snapshot's own and a
// block freed or allocated after it cannot be misjudged). Like the NTFS map
// on Windows, free space goes into the image as zeros: never read, and the
// deleted data in it never reaches the backup. Any doubt (an unknown layout,
// an inconsistent structure) returns an error and the caller reads the
// whole partition as before.

var errNoFreeSpaceMap = errors.New("no free-space map for this file system")

// LinuxFreeSpaceMap reads the free-space map of the ext2/3/4 or XFS file
// system on r (a partition of size bytes).
func LinuxFreeSpaceMap(r io.ReaderAt, size int64) (*UsedMap, error) {
	head := make([]byte, 2048)
	if _, err := r.ReadAt(head, 0); err != nil && err != io.EOF {
		return nil, err
	}
	switch {
	case binary.LittleEndian.Uint16(head[1024+56:]) == 0xEF53:
		return extUsedMap(r)
	case string(head[0:4]) == "XFSB":
		return xfsUsedMap(r)
	}
	return nil, errNoFreeSpaceMap
}

// extUsedMap builds the used map of an ext2/3/4 file system from its block
// bitmaps. A group whose bitmap was never initialised (BLOCK_UNINIT, common
// on a fresh file system) is free apart from the backup superblock and
// group descriptors it may hold and any group's bitmaps or inode table that
// sits in it, which is what the kernel itself marks when it first uses it.
func extUsedMap(r io.ReaderAt) (*UsedMap, error) {
	sb := make([]byte, 1024)
	if _, err := r.ReadAt(sb, 1024); err != nil {
		return nil, err
	}
	le16 := func(o int) uint32 { return uint32(binary.LittleEndian.Uint16(sb[o:])) }
	le32 := func(o int) uint32 { return binary.LittleEndian.Uint32(sb[o:]) }
	compat, incompat, roCompat := le32(92), le32(96), le32(100)
	const (
		compatSparse2  = 0x200
		incompatMetaBG = 0x10
		incompat64     = 0x80
		roSparseSuper  = 0x1
		roBigalloc     = 0x200
		bgBlockUninit  = 0x2
	)
	if incompat&incompatMetaBG != 0 || roCompat&roBigalloc != 0 {
		return nil, fmt.Errorf("ext: meta_bg or bigalloc layout: %w", errNoFreeSpaceMap)
	}
	logBS := le32(24)
	if logBS > 6 {
		return nil, errNoFreeSpaceMap
	}
	bs := int64(1024) << logBS
	blocks := int64(le32(4))
	descSize := int64(32)
	if incompat&incompat64 != 0 {
		blocks |= int64(le32(336)) << 32
		if ds := int64(le16(254)); ds >= 64 {
			descSize = ds
		}
	}
	firstData := int64(le32(20))
	bpg := int64(le32(32))
	ipg := int64(le32(40))
	inodeSize := int64(128)
	if le32(76) >= 1 {
		inodeSize = int64(le16(88))
	}
	reservedGDT := int64(le16(0xCE))
	if bpg <= 0 || bpg > bs*8 || ipg <= 0 || blocks <= firstData {
		return nil, fmt.Errorf("ext: superblock sizes: %w", errNoFreeSpaceMap)
	}
	groups := (blocks - firstData + bpg - 1) / bpg
	gdtBlocks := (groups*descSize + bs - 1) / bs
	itBlocks := (ipg*inodeSize + bs - 1) / bs

	hasBackup := func(g int64) bool {
		if g == 0 {
			return true
		}
		if compat&compatSparse2 != 0 {
			return g == int64(le32(0x24C)) || g == int64(le32(0x250))
		}
		if roCompat&roSparseSuper == 0 {
			return true
		}
		for _, base := range []int64{3, 5, 7} {
			for p := base; p <= g; p *= base {
				if p == g {
					return true
				}
			}
		}
		return g == 1
	}

	gdt := make([]byte, groups*descSize)
	if _, err := r.ReadAt(gdt, (firstData+1)*bs); err != nil && err != io.EOF {
		return nil, err
	}
	type meta struct{ bb, ib, it int64 }
	metas := make([]meta, groups)
	flags := make([]uint32, groups)
	for g := int64(0); g < groups; g++ {
		d := gdt[g*descSize:]
		get := func(lo, hi int) int64 {
			v := int64(binary.LittleEndian.Uint32(d[lo:]))
			if descSize >= 64 {
				v |= int64(binary.LittleEndian.Uint32(d[hi:])) << 32
			}
			return v
		}
		metas[g] = meta{bb: get(0, 0x20), ib: get(4, 0x24), it: get(8, 0x28)}
		flags[g] = uint32(binary.LittleEndian.Uint16(d[0x12:]))
		for _, b := range []int64{metas[g].bb, metas[g].ib, metas[g].it, metas[g].it + itBlocks - 1} {
			if b < firstData || b >= blocks {
				return nil, fmt.Errorf("ext: group %d descriptor points outside the file system: %w", g, errNoFreeSpaceMap)
			}
		}
	}

	m := newUsedMap(bs, blocks) // all used; each group's free blocks cleared below
	bitmap := make([]byte, bs)
	for g := int64(0); g < groups; g++ {
		start := firstData + g*bpg
		count := bpg
		if start+count > blocks {
			count = blocks - start
		}
		if flags[g]&bgBlockUninit != 0 {
			m.free(start, count)
			if hasBackup(g) {
				m.markUsed(start, 1+gdtBlocks+reservedGDT)
			}
			for _, o := range metas {
				for _, run := range [][2]int64{{o.bb, 1}, {o.ib, 1}, {o.it, itBlocks}} {
					if run[0]+run[1] > start && run[0] < start+count {
						m.markUsed(run[0], run[1])
					}
				}
			}
			continue
		}
		if _, err := r.ReadAt(bitmap, metas[g].bb*bs); err != nil && err != io.EOF {
			return nil, err
		}
		for i := int64(0); i < count; i++ {
			if bitmap[i/8]&(1<<(i%8)) == 0 {
				m.free(start+i, 1)
			}
		}
		// The group's own metadata must be marked in its bitmap; if it is
		// not, the bitmap is not what it seems: read everything.
		if hasBackup(g) && bitmap[0]&1 == 0 {
			return nil, fmt.Errorf("ext: group %d bitmap does not mark its own superblock: %w", g, errNoFreeSpaceMap)
		}
	}
	return m, nil
}

// xfsUsedMap builds the used map of an XFS file system from the free space
// B+trees (by block number) of its allocation groups.
func xfsUsedMap(r io.ReaderAt) (*UsedMap, error) {
	sb := make([]byte, 512)
	if _, err := r.ReadAt(sb, 0); err != nil {
		return nil, err
	}
	be16 := func(b []byte, o int) int64 { return int64(binary.BigEndian.Uint16(b[o:])) }
	be32 := func(b []byte, o int) int64 { return int64(binary.BigEndian.Uint32(b[o:])) }
	bs := be32(sb, 4)
	dblocks := int64(binary.BigEndian.Uint64(sb[8:]))
	agblocks := be32(sb, 84)
	agcount := be32(sb, 88)
	sectSize := be16(sb, 102)
	v5 := be16(sb, 100)&0xF == 5
	if bs < 512 || bs > 65536 || agblocks <= 0 || agcount <= 0 || sectSize < 512 || dblocks <= 0 {
		return nil, fmt.Errorf("xfs: superblock sizes: %w", errNoFreeSpaceMap)
	}
	hdr := int64(16)
	magic := "ABTB"
	if v5 {
		hdr, magic = 56, "AB3B"
	}
	m := newUsedMap(bs, dblocks)
	block := make([]byte, bs)
	var walk func(ag, agbno int64, depth int) error
	walk = func(ag, agbno int64, depth int) error {
		if depth > 16 || agbno <= 0 || agbno >= agblocks {
			return fmt.Errorf("xfs: bad free-space tree in AG %d: %w", ag, errNoFreeSpaceMap)
		}
		b := make([]byte, bs)
		if _, err := r.ReadAt(b, (ag*agblocks+agbno)*bs); err != nil && err != io.EOF {
			return err
		}
		if string(b[0:4]) != magic {
			return fmt.Errorf("xfs: AG %d block %d is not a free-space tree block: %w", ag, agbno, errNoFreeSpaceMap)
		}
		level := be16(b, 4)
		n := be16(b, 6)
		if level == 0 {
			for i := int64(0); i < n; i++ {
				o := hdr + i*8
				if o+8 > bs {
					break
				}
				start, count := be32(b, int(o)), be32(b, int(o+4))
				if start+count > agblocks {
					return fmt.Errorf("xfs: free extent past the end of AG %d: %w", ag, errNoFreeSpaceMap)
				}
				m.free(ag*agblocks+start, count)
			}
			return nil
		}
		maxrecs := (bs - hdr) / 12 // 8-byte keys, then 4-byte pointers
		for i := int64(0); i < n; i++ {
			if err := walk(ag, be32(b, int(hdr+maxrecs*8+i*4)), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	_ = block
	for ag := int64(0); ag < agcount; ag++ {
		agf := make([]byte, 64)
		if _, err := r.ReadAt(agf, ag*agblocks*bs+sectSize); err != nil && err != io.EOF {
			return nil, err
		}
		if string(agf[0:4]) != "XAGF" {
			return nil, fmt.Errorf("xfs: AG %d has no AGF: %w", ag, errNoFreeSpaceMap)
		}
		if err := walk(ag, be32(agf, 16), 0); err != nil {
			return nil, err
		}
	}
	return m, nil
}
