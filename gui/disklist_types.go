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
}
