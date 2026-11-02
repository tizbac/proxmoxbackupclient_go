//go:build windows

package machinebackuplib

import (
	"os"
	"testing"
)

// Opt-in (PBS_DISKMODEL=1): the model of PhysicalDrive0 and the file system
// of C:, the strings the machine backup card shows.
func TestLiveDiskModel(t *testing.T) {
	if os.Getenv("PBS_DISKMODEL") != "1" {
		t.Skip("set PBS_DISKMODEL=1")
	}
	m := DiskModel(`\\.\PhysicalDrive0`)
	fs := volumeFileSystem("C")
	t.Logf("PhysicalDrive0 model %q, C: file system %q", m, fs)
	if m == "" || fs == "" {
		t.Fatal("empty")
	}
}
