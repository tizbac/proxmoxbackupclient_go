//go:build linux

package machinebackuplib

import (
	"encoding/binary"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

const fsIocFiemap = 0xC020660B // FS_IOC_FIEMAP

// fileExtents returns the physical byte ranges [start, len) of a file on the
// block device holding its file system, from FS_IOC_FIEMAP. Extents the
// kernel flags as not on a device (inline, unknown, delayed allocation,
// encoded) are left out.
func fileExtents(path string) ([][2]int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	const (
		hdr          = 32
		extSize      = 56
		perCall      = 512
		flagLast     = 0x1
		flagUnknown  = 0x2
		flagDelalloc = 0x4
		flagEncoded  = 0x8
		flagInline   = 0x200
		flagSync     = 0x1 // FIEMAP_FLAG_SYNC: flush first
	)
	var out [][2]int64
	buf := make([]byte, hdr+perCall*extSize)
	var start uint64
	for {
		for i := range buf {
			buf[i] = 0
		}
		binary.LittleEndian.PutUint64(buf[0:], start)
		binary.LittleEndian.PutUint64(buf[8:], ^uint64(0)) // to the end
		binary.LittleEndian.PutUint32(buf[16:], flagSync)
		binary.LittleEndian.PutUint32(buf[24:], perCall)
		if _, _, e := unix.Syscall(unix.SYS_IOCTL, f.Fd(), fsIocFiemap, uintptr(unsafe.Pointer(&buf[0]))); e != 0 {
			return nil, e
		}
		n := int(binary.LittleEndian.Uint32(buf[20:]))
		if n == 0 {
			return out, nil
		}
		last := false
		for i := 0; i < n; i++ {
			e := buf[hdr+i*extSize:]
			logical := binary.LittleEndian.Uint64(e[0:])
			phys := binary.LittleEndian.Uint64(e[8:])
			length := binary.LittleEndian.Uint64(e[16:])
			flags := binary.LittleEndian.Uint32(e[40:])
			if flags&(flagUnknown|flagDelalloc|flagEncoded|flagInline) == 0 {
				out = append(out, [2]int64{int64(phys), int64(length)})
			}
			start = logical + length
			if flags&flagLast != 0 {
				last = true
			}
		}
		if last {
			return out, nil
		}
	}
}
