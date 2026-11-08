package machinebackuplib

import "testing"

func TestJoinModel(t *testing.T) {
	for _, c := range [][3]string{
		{"ATA", "Samsung SSD 860 EVO 500GB", "Samsung SSD 860 EVO 500GB"},
		{"NVMe", "BIWIN SSD", "BIWIN SSD"},
		{"", "QEMU HARDDISK", "QEMU HARDDISK"},
		{"WDC", "WDC WD20EFRX-68E", "WDC WD20EFRX-68E"},
		{"Seagate", "Expansion HDD", "Seagate Expansion HDD"},
		{"QEMU", "", "QEMU"},
		{"", "", ""},
		{"  Kingston ", " DataTraveler 3.0 ", "Kingston DataTraveler 3.0"},
	} {
		if got := joinModel(c[0], c[1]); got != c[2] {
			t.Errorf("joinModel(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

func TestPartitionLabelWithFileSystem(t *testing.T) {
	for _, c := range []struct {
		p    Partition
		want string
	}{
		{Partition{Letter: "C", FileSystem: "NTFS"}, "C: (NTFS)"},
		{Partition{Letter: "U"}, "U:"},
		{Partition{Number: 2}, "partition 2"},
		{Partition{}, "unpartitioned space"},
	} {
		if got := c.p.label(); got != c.want {
			t.Errorf("%+v label = %q, want %q", c.p, got, c.want)
		}
	}
}
