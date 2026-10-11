//go:build windows

package machinebackuplib

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
)

const (
	fsctlGetVolumeBitmap     = 0x0009006F
	fsctlGetNTFSVolumeData   = 0x00090064
	fsctlGetRetrievalPointer = 0x00090073
	errorMoreData            = syscall.Errno(234)
)

// usedMap says which parts of an NTFS volume hold data: one bit per
// cluster, set when the file system uses it. Free space is not read in a
// machine backup; it goes into the image as zeros, which cost nothing to
// store or send after the first such chunk.
type usedMap struct {
	clusterSize int64
	bits        []byte
	clusters    int64
}

// anyUsed reports whether any cluster in [off, off+n) of the volume is used.
// Space past the clusters the bitmap covers counts as used (read as before).
func (m *usedMap) anyUsed(off, n int64) bool {
	first := off / m.clusterSize
	last := (off + n - 1) / m.clusterSize
	if last >= m.clusters {
		return true
	}
	for c := first; c <= last; {
		// Whole bytes at a time where possible.
		if c%8 == 0 && c+8 <= last+1 {
			if m.bits[c/8] != 0 {
				return true
			}
			c += 8
			continue
		}
		if m.bits[c/8]&(1<<(c%8)) != 0 {
			return true
		}
		c++
	}
	return false
}

// zeroFree zeroes the bytes of data (read from volume offset off) that lie
// in free clusters.
func (m *usedMap) zeroFree(data []byte, off int64) {
	end := off + int64(len(data))
	for c := off / m.clusterSize; c*m.clusterSize < end && c < m.clusters; c++ {
		if m.bits[c/8]&(1<<(c%8)) != 0 {
			continue
		}
		from, to := c*m.clusterSize, (c+1)*m.clusterSize
		if from < off {
			from = off
		}
		if to > end {
			to = end
		}
		clear(data[from-off : to-off])
	}
}

// free clears the clusters [lcn, lcn+count): they are read as zeros.
func (m *usedMap) free(lcn, count int64) {
	for c := lcn; c < lcn+count && c < m.clusters; c++ {
		if c >= 0 {
			m.bits[c/8] &^= 1 << (c % 8)
		}
	}
}

// readUsedMap reads the cluster bitmap of the NTFS volume open in f (a
// volume shadow copy), then marks the paging, hibernation and swap files'
// clusters free as well: Windows rebuilds them, and they are often the
// largest files on the disk. Any failure returns an error, and the caller
// reads the whole volume as before.
func readUsedMap(f *os.File, volumePath string) (*usedMap, error) {
	h := syscall.Handle(f.Fd())
	var vd [128]byte
	var n uint32
	if err := syscall.DeviceIoControl(h, fsctlGetNTFSVolumeData, nil, 0, &vd[0], uint32(len(vd)), &n, nil); err != nil {
		return nil, fmt.Errorf("NTFS volume data: %w", err)
	}
	total := int64(binary.LittleEndian.Uint64(vd[16:]))
	clusterSize := int64(binary.LittleEndian.Uint32(vd[44:]))
	if clusterSize <= 0 || total <= 0 {
		return nil, fmt.Errorf("unexpected volume data (%d clusters of %d bytes)", total, clusterSize)
	}
	m := &usedMap{clusterSize: clusterSize, clusters: total, bits: make([]byte, (total+7)/8)}

	// FSCTL_GET_VOLUME_BITMAP hands the bitmap out in pieces: ask from a
	// starting cluster, get as much as fits, repeat.
	const piece = 8 << 20
	out := make([]byte, 16+piece)
	for start := int64(0); start < total; {
		var in [8]byte
		binary.LittleEndian.PutUint64(in[:], uint64(start))
		err := syscall.DeviceIoControl(h, fsctlGetVolumeBitmap, &in[0], 8, &out[0], uint32(len(out)), &n, nil)
		if err != nil && err != errorMoreData {
			return nil, fmt.Errorf("volume bitmap at cluster %d: %w", start, err)
		}
		got := int64(binary.LittleEndian.Uint64(out[0:]))
		size := int64(binary.LittleEndian.Uint64(out[8:])) // clusters from got to the end
		if got != start&^7 || n <= 16 {
			return nil, fmt.Errorf("volume bitmap: asked for cluster %d, got %d (%d bytes)", start, got, n)
		}
		bytes := int64(n) - 16
		if bytes*8 > size {
			bytes = (size + 7) / 8
		}
		copy(m.bits[got/8:], out[16:16+bytes])
		start = got + bytes*8
	}

	for _, name := range []string{"pagefile.sys", "hiberfil.sys", "swapfile.sys"} {
		freeFileClusters(m, volumePath+`\`+name)
	}
	return m, nil
}

// freeFileClusters marks a file's clusters free, best effort (the file may
// not be in the shadow copy at all: VSS leaves these out by default).
func freeFileClusters(m *usedMap, path string) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	h, err := syscall.CreateFile(p, 0x80 /* FILE_READ_ATTRIBUTES */, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return
	}
	defer syscall.CloseHandle(h)
	out := make([]byte, 64<<10)
	var vcn int64
	for {
		var in [8]byte
		binary.LittleEndian.PutUint64(in[:], uint64(vcn))
		var n uint32
		err := syscall.DeviceIoControl(h, fsctlGetRetrievalPointer, &in[0], 8, &out[0], uint32(len(out)), &n, nil)
		if err != nil && err != errorMoreData {
			return
		}
		count := int(binary.LittleEndian.Uint32(out[0:]))
		prev := int64(binary.LittleEndian.Uint64(out[8:]))
		for i := 0; i < count && 16+16*i+16 <= int(n); i++ {
			next := int64(binary.LittleEndian.Uint64(out[16+16*i:]))
			lcn := int64(binary.LittleEndian.Uint64(out[24+16*i:]))
			if lcn >= 0 { // -1: a sparse or compressed run, no clusters
				m.free(lcn, next-prev)
			}
			prev = next
		}
		if err != errorMoreData || count == 0 {
			return
		}
		vcn = prev
	}
}
