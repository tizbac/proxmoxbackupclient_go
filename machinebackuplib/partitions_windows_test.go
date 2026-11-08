//go:build windows

package machinebackuplib

import (
	"testing"
	"unsafe"
)

// windowsPartitionType reads the MBR/GPT union at byte 32 of
// PARTITION_INFORMATION_EX, as Windows lays the structure out (144 bytes).
func TestWindowsPartitionType(t *testing.T) {
	if s := unsafe.Sizeof(PARTITION_INFORMATION_EX{}); s != 144 {
		t.Fatalf("PARTITION_INFORMATION_EX is %d bytes, Windows gives 144", s)
	}
	at := func(E *PARTITION_INFORMATION_EX) []byte {
		return unsafe.Slice((*byte)(unsafe.Add(unsafe.Pointer(E), 32)), 18)
	}

	var E PARTITION_INFORMATION_EX
	E.PartitionStyle = PartitionStyleGPT
	copy(at(&E), []byte{0x28, 0x73, 0x2A, 0xC1, 0x1F, 0xF8, 0xD2, 0x11, 0xBA, 0x4B, 0x00, 0xA0, 0xC9, 0x3E, 0xC9, 0x3B})
	if typ, boot := windowsPartitionType(&E); typ != "EFI system" || !boot {
		t.Errorf("EFI system partition: %q, boot %v", typ, boot)
	}
	E = PARTITION_INFORMATION_EX{PartitionStyle: PartitionStyleGPT}
	// Microsoft basic data: EBD0A0A2-B9E5-4433-87C0-68B6B72699C7.
	copy(at(&E), []byte{0xA2, 0xA0, 0xD0, 0xEB, 0xE5, 0xB9, 0x33, 0x44, 0x87, 0xC0, 0x68, 0xB6, 0xB7, 0x26, 0x99, 0xC7})
	if typ, boot := windowsPartitionType(&E); typ != "Microsoft basic data" || boot {
		t.Errorf("basic data partition: %q, boot %v", typ, boot)
	}

	// MBR: the type byte, then the active flag.
	E = PARTITION_INFORMATION_EX{PartitionStyle: PartitionStyleMBR}
	at(&E)[0], at(&E)[1] = 0x07, 1
	if typ, boot := windowsPartitionType(&E); typ != "NTFS/exFAT" || !boot {
		t.Errorf("active NTFS partition (System Reserved): %q, boot %v", typ, boot)
	}
	at(&E)[1] = 0
	if _, boot := windowsPartitionType(&E); boot {
		t.Error("an NTFS partition that is not active is not needed to start")
	}
}
