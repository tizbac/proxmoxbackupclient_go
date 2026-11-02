package main

// PhysicalDiskInfo represents information about a physical disk.
//
// No build tag (shared by both the GUI and service binaries): the OS-specific
// listPhysicalDisks() implementations (disklist_windows.go, disklist_linux.go)
// are tagged only by GOOS, not by !service, so this type must be visible
// under -tags service too — it previously lived in the GUI-only main.go,
// which broke the service build (found 2026-09-23 while fixing that build).
type PhysicalDiskInfo struct {
	DiskNumber   int64  `json:"disk_number"`
	Size         int64  `json:"size"`
	Model        string `json:"model"`
	IsBootDisk   bool   `json:"is_boot_disk"`
	IsSystemDisk bool   `json:"is_system_disk"`
	DeviceID     string `json:"device_id"`
	DevicePath   string `json:"device_path"`
	// UniqueID identifies the disk itself rather than the number the system
	// gave it this boot: "serial:<serial>", "guid:<GPT disk GUID>" or
	// "sig:<MBR disk signature>" on Windows, "byid:<name in /dev/disk/by-id>"
	// on Linux, or a weak "size:<bytes>" when nothing better can be read. A
	// machine Backup Set records it for each disk (diskpin.go), so a disk the
	// system renumbers is still the one backed up.
	UniqueID string `json:"unique_id,omitempty"`
	// OtherIDs are further identities of the same disk: on Windows its GPT
	// GUID or MBR signature when UniqueID is the serial, on Linux its other
	// /dev/disk/by-id names. A set's disk is found by any of them, so a set
	// that recorded one still finds its disk when another is preferred now.
	OtherIDs []string `json:"other_ids,omitempty"`
	// DriveLetters are the Windows volumes mounted on the disk (["C:", "D:"]),
	// shown in the disk picker. Empty when there are none or on Linux.
	DriveLetters []string `json:"drive_letters,omitempty"`
}
