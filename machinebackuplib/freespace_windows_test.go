//go:build windows

package machinebackuplib

import "testing"

func TestUsedMap(t *testing.T) {
	// 64 clusters of 4 KiB; clusters 3 and 40-47 used.
	m := &usedMap{clusterSize: 4096, clusters: 64, bits: make([]byte, 8)}
	m.bits[0] = 1 << 3
	m.bits[5] = 0xFF
	cases := []struct {
		off, n int64
		want   bool
	}{
		{0, 4096 * 3, false},            // clusters 0-2
		{0, 4096*3 + 1, true},           // reaches cluster 3
		{4096 * 4, 4096 * 36, false},    // 4-39
		{4096 * 39, 4096 * 2, true},     // 39-40
		{4096 * 48, 4096 * 16, false},   // 48-63
		{4096 * 60, 4096 * 8, true},     // runs past the bitmap: read it
		{4096*8 + 100, 4096 * 8, false}, // unaligned, clusters 8-16
	}
	for _, c := range cases {
		if got := m.anyUsed(c.off, c.n); got != c.want {
			t.Errorf("anyUsed(%d, %d) = %v, want %v", c.off, c.n, got, c.want)
		}
	}
	m.free(40, 8) // e.g. pagefile.sys
	if m.anyUsed(4096*40, 4096*8) {
		t.Error("freed clusters still used")
	}
	// zeroFree: only the free clusters' bytes are zeroed, unaligned too.
	data := make([]byte, 4096*3)
	for i := range data {
		data[i] = 0xAA
	}
	m.zeroFree(data, 4096*2+100) // clusters 2 (free), 3 (used), 4 (free), 5 (free)
	if data[0] != 0 || data[4096-100-1] != 0 || data[4096-100] != 0xAA || data[2*4096-100-1] != 0xAA || data[2*4096-100] != 0 || data[len(data)-1] != 0 {
		t.Errorf("zeroFree: %x %x %x %x %x %x", data[0], data[4096-101], data[4096-100], data[2*4096-101], data[2*4096-100], data[len(data)-1])
	}
	m.free(-5, 3)    // a bad run must not panic
	m.free(60, 1000) // nor one past the end
}
