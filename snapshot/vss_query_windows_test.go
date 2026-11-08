//go:build windows

package snapshot

import (
	"testing"
	"unsafe"

	"github.com/st-matskevich/go-vss"
)

// vssObjectProps is read through a pointer to go-vss's VssObjectProperties,
// so the two must be laid out alike: the same size, the snapshot after the
// 4-byte object type and its padding.
func TestVSSObjectPropsLayout(t *testing.T) {
	var o vssObjectProps
	if got, want := unsafe.Sizeof(o), unsafe.Sizeof(vss.VssObjectProperties{}); got != want {
		t.Fatalf("vssObjectProps is %d bytes, go-vss's VssObjectProperties %d", got, want)
	}
	if off := unsafe.Offsetof(o.snapshot); off != 8 {
		t.Fatalf("snapshot at offset %d, want 8", off)
	}
}
