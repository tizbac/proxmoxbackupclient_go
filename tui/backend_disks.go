package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/salvanza/salvanza_client/gui/api"
)

// Partition is one partition of a disk, as the service lists it
// (machinebackuplib.PartitionInfo).
type Partition struct {
	Number     int    `json:"number"`
	Start      uint64 `json:"start"`
	Size       uint64 `json:"size"`
	Mount      string `json:"mount,omitempty"`
	FileSystem string `json:"file_system,omitempty"`
	Type       string `json:"type,omitempty"`
	Role       string `json:"role,omitempty"` // "system", "boot" or ""
}

// Disk is a physical disk as the service lists it for the disk picker. Only
// the service can read them (administrator or root), so the window asks it.
type Disk struct {
	DiskNumber   int64       `json:"disk_number"`
	Size         int64       `json:"size"`
	Model        string      `json:"model"`
	IsBootDisk   bool        `json:"is_boot_disk"`
	IsSystemDisk bool        `json:"is_system_disk"`
	DevicePath   string      `json:"device_path"`
	UniqueID     string      `json:"unique_id,omitempty"`
	OtherIDs     []string    `json:"other_ids,omitempty"`
	DriveLetters []string    `json:"drive_letters,omitempty"`
	Partitions   []Partition `json:"partitions"` // nil when they could not be listed
}

// hasID reports whether id names this disk (its main identity or another).
func (d Disk) hasID(id string) bool {
	if id == "" {
		return false
	}
	if d.UniqueID == id {
		return true
	}
	for _, o := range d.OtherIDs {
		if o == id {
			return true
		}
	}
	return false
}

// stableID is a disk identity that survives renumbering: the weak
// "size:<bytes>" identity is no identity at all.
func stableID(id string) string {
	if id == "" || strings.HasPrefix(id, "size:") {
		return ""
	}
	return id
}

func (b *serviceBackend) Disks() ([]Disk, error) {
	var out []Disk
	if err := b.c.Call("ListPhysicalDisks", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (b *serviceBackend) StartMachineBackup(req api.BackupRequest) error {
	resp, err := b.c.StartMachineBackup(&req)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", firstNonEmpty(resp.Error, resp.Message, "the service did not start the backup"))
	}
	return nil
}

// demoDisks is what the demo and the tests list: a system disk with an EFI,
// a Windows and a recovery partition, and a data disk.
func demoDisks() []Disk {
	const gib = int64(1) << 30
	return []Disk{
		{DiskNumber: 0, Size: 476 * gib, Model: "Samsung SSD 980", IsBootDisk: true, IsSystemDisk: true,
			DevicePath: `\\.\PhysicalDrive0`, UniqueID: "serial:S64DNX0T", DriveLetters: []string{"C:"},
			Partitions: []Partition{
				{Number: 1, Start: 1 << 20, Size: 100 << 20, FileSystem: "FAT32", Type: "EFI system", Role: "boot"},
				{Number: 2, Start: 101 << 20, Size: uint64(475 * gib), Mount: "C:", FileSystem: "NTFS", Type: "Basic data", Role: "system"},
				{Number: 3, Start: uint64(476 * gib), Size: 1 << 29, FileSystem: "NTFS", Type: "Windows recovery"},
			}},
		{DiskNumber: 1, Size: 1862 * gib, Model: "WDC WD20EZAZ", DevicePath: `\\.\PhysicalDrive1`,
			UniqueID: "serial:WD-WX12A", DriveLetters: []string{"D:"},
			Partitions: []Partition{{Number: 1, Start: 1 << 20, Size: uint64(1862 * gib), Mount: "D:", FileSystem: "NTFS", Type: "Basic data"}}},
	}
}

func (d *demoBackend) Disks() ([]Disk, error) { return demoDisks(), nil }

func (d *demoBackend) StartMachineBackup(req api.BackupRequest) error {
	if len(req.DriveLetters) == 0 {
		return fmt.Errorf("no disk to back up")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nextID++
	d.runs = append(d.runs, api.BackupProgress{
		JobID: fmt.Sprintf("demo-%d", d.nextID), Name: "One-off: " + req.DriveLetters[0], Running: true, Progress: 1,
		StartTime: time.Now().Format(time.RFC3339), Message: "Starting",
	})
	return nil
}
