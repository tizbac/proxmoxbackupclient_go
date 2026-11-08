package machinebackuplib

import "testing"

func TestBytesToString(t *testing.T) {
	for _, c := range []struct {
		b    int64
		want string
	}{
		{0, "0B"},
		{999, "999B"},
		{1000, "1.0kB"},
		{3_000_000, "3.0MB"},       // printed "2MB" before (2.86 MiB, cut down)
		{3_060_000, "3.1MB"},       // rounded, not cut
		{53_687_091_200, "53.7GB"}, // a 50 GiB disk, "50GB" before
		{999_949_999, "999.9MB"},
		{999_950_000, "1.0GB"}, // never "1000.0MB"
		{4_000_000_000_000, "4.0TB"},
		{2_500_000_000_000_000, "2500.0TB"},
	} {
		if got := BytesToString(c.b); got != c.want {
			t.Errorf("BytesToString(%d) = %q, want %q", c.b, got, c.want)
		}
	}
}
