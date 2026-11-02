package main

// A machine set's folder is found in a disk image by the partition offset it
// has on the live disk (imageFolderBase). An older snapshot made before a
// partition was moved or resized has the volume somewhere else, or another
// volume at that offset. The NTFS volume serial number survives both, so
// when the live folder's serial is known it decides, and the offset is the
// fallback (other file systems, or a folder no longer on this computer).

// imagePart is one candidate partition in an image: its offset and, for
// NTFS, its volume serial number.
type imagePart struct {
	start     int64
	serial    uint64
	hasSerial bool
}

// pickImagePart returns the index in parts of the folder's volume, or -1.
// offset is where the volume is on the live disk; liveSerial (when liveOK)
// is its Windows volume serial number, the low 32 bits of the NTFS one.
func pickImagePart(parts []imagePart, offset int64, liveSerial uint32, liveOK bool) int {
	if liveOK {
		match := -1
		for i, p := range parts {
			if p.hasSerial && uint32(p.serial) == liveSerial {
				if p.start == offset {
					return i
				}
				if match < 0 {
					match = i
				}
			}
		}
		if match >= 0 {
			return match
		}
	}
	// No serial match: the volume at the same offset, as before. That also
	// covers a volume formatted again in place, whose older snapshots carry
	// another serial.
	for i, p := range parts {
		if p.start == offset {
			return i
		}
	}
	return -1
}
