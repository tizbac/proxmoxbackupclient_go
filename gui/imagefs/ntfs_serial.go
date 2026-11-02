package imagefs

import (
	"encoding/binary"
	"io"
)

// NTFSSerial reads the volume serial number from the NTFS boot sector of the
// partition starting at start. Windows shows its low 32 bits as the volume
// serial number (GetVolumeInformation, `vol`); it stays the same when the
// partition is moved or resized, unlike the partition's offset.
func NTFSSerial(r io.ReaderAt, start int64) (uint64, bool) {
	b := make([]byte, 0x50)
	if _, err := r.ReadAt(b, start); err != nil {
		return 0, false
	}
	if string(b[3:11]) != "NTFS    " {
		return 0, false
	}
	return binary.LittleEndian.Uint64(b[0x48:0x50]), true
}
