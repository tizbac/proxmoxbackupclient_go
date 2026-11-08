package main

import "testing"

func TestPickImagePart(t *testing.T) {
	const live = uint32(0xA1B2C3D4)
	own := uint64(0x11223344_A1B2C3D4) // the folder's volume (low 32 bits = live)
	other := uint64(0x55667788_99AABBCC)
	cases := []struct {
		name   string
		parts  []imagePart
		liveOK bool
		want   int
	}{
		{"same place", []imagePart{{start: 1 << 20, serial: other, hasSerial: true}, {start: 100 << 20, serial: own, hasSerial: true}}, true, 1},
		{"moved since the snapshot", []imagePart{{start: 1 << 20, serial: other, hasSerial: true}, {start: 300 << 20, serial: own, hasSerial: true}}, true, 1},
		{"another volume now at the offset", []imagePart{{start: 100 << 20, serial: other, hasSerial: true}, {start: 600 << 20, serial: own, hasSerial: true}}, true, 1},
		{"formatted again in place", []imagePart{{start: 100 << 20, serial: other, hasSerial: true}}, true, 0},
		{"serial unknown: offset", []imagePart{{start: 1 << 20}, {start: 100 << 20}}, false, 1},
		{"not NTFS: offset", []imagePart{{start: 100 << 20}}, true, 0},
		{"nowhere", []imagePart{{start: 1 << 20, serial: other, hasSerial: true}}, true, -1},
	}
	for _, c := range cases {
		if got := pickImagePart(c.parts, 100<<20, live, c.liveOK); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}
