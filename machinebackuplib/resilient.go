package machinebackuplib

import (
	"errors"
	"io"
)

// errCopyStopped is what readResilient returns when emit or emitZeros
// refused the data (the upload stopped, or the user pressed Stop); the caller
// knows which.
var errCopyStopped = errors.New("copy stopped")

// readResilient copies length bytes of src, starting at srcOffset, through
// emit, so that one bad sector does not stop a whole disk image.
//
// A read that fails with a media error (isMediaError: a CRC or sector-not-
// found error from the disk) is tried again one unit at a time over the part
// it did not return; units that still fail are replaced by zeros and passed
// to onBad (offset in src, length). Any other error stops the copy and is
// returned. EOF before length pads the rest with zeros: a volume snapshot can
// be a little smaller than its partition.
//
// unit must be a multiple of the device's sector size: 512 on Linux (block
// devices take any size), 4096 on Windows (raw reads must be sector-aligned,
// and 4096 suits both 512-byte and 4K-sector disks).
func readResilient(src io.ReaderAt, srcOffset, length uint64, block []byte, unit uint64,
	emit func([]byte) bool, emitZeros func(uint64) bool, onBad func(off, n uint64)) error {
	var pos uint64
	padToEnd := func() error {
		if pos < length && !emitZeros(length-pos) {
			return errCopyStopped
		}
		pos = length
		return nil
	}
	for pos < length {
		want := uint64(len(block))
		if want > length-pos {
			want = length - pos
		}
		n, err := src.ReadAt(block[:want], int64(srcOffset+pos))
		if n > 0 {
			if !emit(block[:n]) {
				return errCopyStopped
			}
			pos += uint64(n)
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			return padToEnd()
		}
		if !isMediaError(err) {
			return err
		}

		// Go over the rest of this read one unit at a time.
		winEnd := pos + (want - uint64(n))
		for pos < winEnd {
			s := unit
			if s > winEnd-pos {
				s = winEnd - pos
			}
			m, serr := src.ReadAt(block[:s], int64(srcOffset+pos))
			if m > 0 {
				if !emit(block[:m]) {
					return errCopyStopped
				}
				pos += uint64(m)
			}
			if serr == nil {
				continue
			}
			if serr == io.EOF {
				return padToEnd()
			}
			if !isMediaError(serr) {
				return serr
			}
			if rem := s - uint64(m); rem > 0 {
				if !emitZeros(rem) {
					return errCopyStopped
				}
				if onBad != nil {
					onBad(srcOffset+pos, rem)
				}
				pos += rem
			}
		}
	}
	return nil
}
