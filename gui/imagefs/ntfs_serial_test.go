package imagefs

import "testing"

// The NTFS partition of the fixture has a serial number; the FAT32 one is
// not NTFS and has none to give.
func TestNTFSSerial(t *testing.T) {
	r, size := loadFixture(t)
	parts, err := ReadPartitions(r, size)
	if err != nil {
		t.Fatal(err)
	}
	serial, ok := NTFSSerial(r, parts[0].Start)
	if !ok || serial == 0 {
		t.Fatalf("NTFS partition: serial %x, ok %v", serial, ok)
	}
	if _, ok := NTFSSerial(r, parts[1].Start); ok {
		t.Error("the FAT32 partition was read as NTFS")
	}
	if again, _ := NTFSSerial(r, parts[0].Start); again != serial {
		t.Error("the serial changed between two reads")
	}
	t.Logf("fixture NTFS serial %016X (vol shows %04X-%04X)", serial, uint32(serial)>>16, uint32(serial)&0xFFFF)
}
