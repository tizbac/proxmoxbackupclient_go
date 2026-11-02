//go:build windows

package pbscommon

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modntdll          = windows.NewLazySystemDLL("ntdll.dll")
	procNtQueryEaFile = modntdll.NewProc("NtQueryEaFile")
	procNtSetEaFile   = modntdll.NewProc("NtSetEaFile")
)

// eaBufferSize holds the largest EA set NTFS allows on a file (64 KiB) with
// room for the entry headers.
const eaBufferSize = 72 * 1024

// readStreamsAndEAs lists the alternate data streams and NTFS extended
// attributes of path (a file or a folder), with the streams' content as far as
// budget allows (budget is decreased by what is recorded). One handle is
// opened, without access to the file's data, so files open in other programs
// are read too. Errors opening the file are returned; a stream that cannot be
// read is recorded as skipped.
func readStreamsAndEAs(path string, budget *int64) ([]StreamEntry, []EAEntry, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, nil, err
	}
	h, err := windows.CreateFile(p, windows.FILE_READ_EA|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, nil, err
	}
	defer windows.CloseHandle(h)
	streams, eas := streamsAndEAsOf(h, path, budget)
	return streams, eas, nil
}

// openMetaHandle opens path for its security descriptor, attributes, stream
// list and extended attributes, without access to its data (so files open in
// other programs open too). The caller closes the handle.
func openMetaHandle(path string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(p, windows.READ_CONTROL|windows.FILE_READ_EA|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
}

// streamsAndEAsOf lists the streams (reading their content from path as far
// as budget allows) and extended attributes of an open file.
func streamsAndEAsOf(h windows.Handle, path string, budget *int64) ([]StreamEntry, []EAEntry) {
	var streams []StreamEntry
	for _, s := range streamNames(h) {
		e := StreamEntry{Name: s.name, Size: s.size}
		switch {
		case s.size > MaxStreamBytes:
			e.Skipped = fmt.Sprintf("larger than %d MiB", MaxStreamBytes>>20)
		case s.size > *budget:
			e.Skipped = "too much stream data in this backup"
		default:
			data, rerr := readStream(path + ":" + s.name)
			if rerr != nil {
				e.Skipped = rerr.Error()
			} else {
				e.Data = data
				*budget -= int64(len(data))
			}
		}
		streams = append(streams, e)
	}
	return streams, queryEAs(h)
}

// ReadStreamsAndEAs lists a file's alternate data streams (with content, up
// to MaxStreamBytes each) and extended attributes, as a backup records them.
func ReadStreamsAndEAs(path string) ([]StreamEntry, []EAEntry, error) {
	budget := int64(MaxStreamBytesTotal)
	return readStreamsAndEAs(path, &budget)
}

type streamName struct {
	name string
	size int64
}

// streamNames returns the named $DATA streams of an open file (not the
// unnamed main stream), from FILE_STREAM_INFO.
func streamNames(h windows.Handle) []streamName {
	buf := alignedBuffer(4096)
	for {
		err := windows.GetFileInformationByHandleEx(h, windows.FileStreamInfo, &buf[0], uint32(len(buf)))
		if err == nil {
			break
		}
		if errors.Is(err, windows.ERROR_MORE_DATA) && len(buf) < 1<<20 {
			buf = alignedBuffer(len(buf) * 4)
			continue
		}
		return nil // ERROR_HANDLE_EOF: no streams at all (a folder), or a file system without them
	}
	var out []streamName
	for off := 0; off+24 <= len(buf); {
		next := binary.LittleEndian.Uint32(buf[off:])
		nameLen := int(binary.LittleEndian.Uint32(buf[off+4:]))
		size := int64(binary.LittleEndian.Uint64(buf[off+8:]))
		if off+24+nameLen > len(buf) {
			break
		}
		u := make([]uint16, nameLen/2)
		for i := range u {
			u[i] = binary.LittleEndian.Uint16(buf[off+24+2*i:])
		}
		full := string(utf16.Decode(u)) // ":Zone.Identifier:$DATA"; "::$DATA" is the main stream
		if name, ok := strings.CutSuffix(strings.TrimPrefix(full, ":"), ":$DATA"); ok && name != "" {
			out = append(out, streamName{name, size})
		}
		if next == 0 {
			break
		}
		off += int(next)
	}
	return out
}

// alignedBuffer returns a zeroed n-byte buffer aligned to 8 bytes. The
// stream and EA information structures hold 64-bit fields, and Windows fails
// the call with ERROR_NOACCESS ("Invalid access to memory location") when a
// buffer is not aligned; a []byte from make has no such guarantee.
func alignedBuffer(n int) []byte {
	words := make([]uint64, (n+7)/8)
	return unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), n)
}

// readStream reads one alternate data stream ("path:name").
func readStream(streamPath string) ([]byte, error) {
	f, err := os.Open(streamPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, MaxStreamBytes+1))
}

// queryEAs returns the extended attributes of an open file, leaving out the
// $KERNEL.* ones that only Windows itself may set.
func queryEAs(h windows.Handle) []EAEntry {
	if procNtQueryEaFile.Find() != nil {
		return nil
	}
	buf := alignedBuffer(eaBufferSize)
	var iosb windows.IO_STATUS_BLOCK
	r, _, _ := procNtQueryEaFile.Call(uintptr(h), uintptr(unsafe.Pointer(&iosb)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
		0, 0, 0, 0, 1)
	if r != 0 { // STATUS_NO_EAS_ON_FILE, or a file system without EAs
		return nil
	}
	var out []EAEntry
	for off := 0; off+8 <= len(buf); {
		next := binary.LittleEndian.Uint32(buf[off:])
		flags := buf[off+4]
		nameLen := int(buf[off+5])
		valLen := int(binary.LittleEndian.Uint16(buf[off+6:]))
		if off+8+nameLen+1+valLen > len(buf) {
			break
		}
		name := string(buf[off+8 : off+8+nameLen])
		val := append([]byte(nil), buf[off+8+nameLen+1:off+8+nameLen+1+valLen]...)
		if !strings.HasPrefix(strings.ToUpper(name), "$KERNEL.") {
			out = append(out, EAEntry{Name: name, Flags: flags, Value: val})
		}
		if next == 0 {
			break
		}
		off += int(next)
	}
	return out
}

// WriteStreamsAndEAs puts recorded alternate data streams and extended
// attributes back on path (a restored file or folder). Streams recorded
// without content (Skipped set) are not written and are counted in skipped. Writing a
// stream changes the file's modification time; the caller sets it again.
func WriteStreamsAndEAs(path string, streams []StreamEntry, eas []EAEntry) (written, skipped int, err error) {
	for _, s := range streams {
		if s.Skipped != "" {
			skipped++
			continue
		}
		f, oerr := os.OpenFile(path+":"+s.Name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if oerr != nil {
			err = errors.Join(err, fmt.Errorf("stream %s: %w", s.Name, oerr))
			continue
		}
		_, werr := f.Write(s.Data)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			err = errors.Join(err, fmt.Errorf("stream %s: %w", s.Name, werr))
			continue
		}
		written++
	}
	if len(eas) > 0 {
		if eerr := setEAs(path, eas); eerr != nil {
			err = errors.Join(err, fmt.Errorf("extended attributes: %w", eerr))
		}
	}
	return written, skipped, err
}

// setEAs writes extended attributes with NtSetEaFile.
func setEAs(path string, eas []EAEntry) error {
	if err := procNtSetEaFile.Find(); err != nil {
		return err
	}
	built := buildEABuffer(eas)
	if len(built) == 0 {
		return nil
	}
	buf := alignedBuffer(len(built))
	copy(buf, built)
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(p, windows.FILE_WRITE_EA|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	var iosb windows.IO_STATUS_BLOCK
	r, _, _ := procNtSetEaFile.Call(uintptr(h), uintptr(unsafe.Pointer(&iosb)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r != 0 {
		return windows.NTStatus(r)
	}
	return nil
}

// buildEABuffer lays out FILE_FULL_EA_INFORMATION entries, each aligned to
// 4 bytes, the last with NextEntryOffset 0. Names over 255 bytes and values
// over 65535 bytes cannot be expressed and are left out.
func buildEABuffer(eas []EAEntry) []byte {
	var out []byte
	last := -1
	for _, e := range eas {
		if len(e.Name) == 0 || len(e.Name) > 255 || len(e.Value) > 0xFFFF {
			continue
		}
		if last >= 0 {
			for len(out)%4 != 0 {
				out = append(out, 0)
			}
			binary.LittleEndian.PutUint32(out[last:], uint32(len(out)-last))
		}
		last = len(out)
		hdr := make([]byte, 8)
		hdr[4] = e.Flags
		hdr[5] = byte(len(e.Name))
		binary.LittleEndian.PutUint16(hdr[6:], uint16(len(e.Value)))
		out = append(out, hdr...)
		out = append(out, e.Name...)
		out = append(out, 0)
		out = append(out, e.Value...)
	}
	return out
}
