package machinebackuplib

import "testing"

// TestSlugify pins the archive-naming algorithm.
//
// The block archive is named Slugify(<device path>) + ".fidx", so the algorithm
// is part of the on-the-wire contract: the same device must always produce the
// same archive name, otherwise every backup looks like a new device.
//
// The shell mirror of this function lives in testing/e2e/lib.sh and is used by
// the end-to-end suite to predict the archive name. If you change the algorithm
// here, change it there too.
func TestSlugify(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"simple device", "/dev/sda", "devsda"},
		{"nvme partition", "/dev/nvme0n1p3", "devnvme0n1p3"},
		{"uppercase is folded", "/dev/SDB", "devsdb"},
		{"slash is dropped not replaced", "/dev/disk/by-id/foo", "devdiskby-idfoo"},
		{"dots are dropped", "/tmp/disk.img", "tmpdiskimg"},
		{"spaces are dropped", "/mnt/my disk.img", "mntmydiskimg"},
		{"underscores are dropped", "/mnt/my_disk.img", "mntmydiskimg"},
		{"existing dashes are kept and squeezed", "/dev/a--b", "deva-b"},
		{"trailing dash trimmed", "/dev/-sda-", "dev-sda"},
		{"leading dash trimmed", "/-dev/sda", "devsda"},
		{"runs of dashes collapse", "/mnt//a///b", "mntab"},
		{"empty input", "", ""},
		{"only removable characters", "/_ -.", ""},
		{"path with mixed entropy", "/tmp/pbs-e2e-AbC12/disk.img", "tmppbs-e2e-abc12diskimg"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Slugify(tc.in); got != tc.want {
				t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSlugifyIsDeterministic guards the property the whole point of the
// function rests on.
func TestSlugifyIsDeterministic(t *testing.T) {
	for _, in := range []string{"/dev/sda", "/tmp/pbs-e2e-AbC12/disk.img", "/dev/nvme0n1"} {
		first := Slugify(in)
		for i := 0; i < 100; i++ {
			if got := Slugify(in); got != first {
				t.Fatalf("Slugify(%q) is not deterministic: %q then %q", in, first, got)
			}
		}
	}
}
