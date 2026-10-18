package imagefs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"www.velocidex.com/golang/go-ntfs/parser"
)

// attrEA is the $EA attribute: a file's extended attributes (WSL keeps a
// Linux file's owner and mode in them, $LXUID, $LXGID, $LXMOD). It is
// resident unless large; NTFS allows 64 KiB of them on a file.
const attrEA = 0xE0

// maxEABytes bounds what is read of a damaged $EA.
const maxEABytes = 1 << 20

// EA is one NTFS extended attribute.
type EA struct {
	Name  string
	Flags uint8 // 0x80: the file needs it (FILE_NEED_EA)
	Value []byte
}

// ExtendedAttributes returns the extended attributes of an entry, from its
// $EA attribute, leaving out the $KERNEL.* ones only Windows itself may set
// (as a folder backup does). An entry without HasEAs has none.
func (v *NTFS) ExtendedAttributes(e Entry) (eas []EA, err error) {
	if !e.HasEAs {
		return nil, nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	defer func() {
		if p := recover(); p != nil {
			eas, err = nil, fmt.Errorf("NTFS: extended attributes of %s: %v", e.Path, p)
		}
	}()
	m, err := v.ctx.GetMFT(int64(e.Record))
	if err != nil {
		return nil, fmt.Errorf("NTFS: record %d: %w", e.Record, err)
	}
	rr, err := parser.OpenStream(v.ctx, m, attrEA, parser.WILDCARD_STREAM_ID, parser.WILDCARD_STREAM_NAME)
	if errors.Is(err, os.ErrNotExist) {
		// HasEAs came from a stale extension record (List merges them by
		// record number only): the file has none.
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("NTFS: extended attributes of %s: %w", e.Path, err)
	}
	size := parser.RangeSize(rr)
	if size <= 0 {
		return nil, nil
	}
	if size > maxEABytes {
		size = maxEABytes
	}
	buf := make([]byte, size)
	n, err := rr.ReadAt(buf, 0)
	if err != nil && err != io.EOF && int64(n) < size {
		// A short read (a chunk of a non-resident $EA that could not be
		// fetched) would give only the first attributes: report it instead.
		return nil, fmt.Errorf("NTFS: reading the extended attributes of %s: %w", e.Path, err)
	}
	return parseEAs(buf[:n]), nil
}

// parseEAs decodes the content of an $EA attribute: entries laid out as
// FILE_FULL_EA_INFORMATION (next entry offset, flags, name length, value
// length, the name and a NUL, the value). NTFS sets the offset on the last
// entry too, so the content's end is what stops the walk; a zero offset or
// an entry running past the end stops it as well.
func parseEAs(b []byte) []EA {
	var out []EA
	for off := 0; off+8 <= len(b); {
		next := int(binary.LittleEndian.Uint32(b[off:]))
		flags := b[off+4]
		nameLen := int(b[off+5])
		valLen := int(binary.LittleEndian.Uint16(b[off+6:]))
		if nameLen == 0 || off+8+nameLen+1+valLen > len(b) {
			break
		}
		name := string(b[off+8 : off+8+nameLen])
		if !strings.HasPrefix(strings.ToUpper(name), "$KERNEL.") {
			val := append([]byte{}, b[off+8+nameLen+1:off+8+nameLen+1+valLen]...)
			out = append(out, EA{Name: name, Flags: flags, Value: val})
		}
		if next <= 0 {
			break
		}
		off += next
	}
	return out
}
