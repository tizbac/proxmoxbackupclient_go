package pbscommon

import "testing"

// A setting or option may hold the four levels (any case) or nothing; the
// spellings ParseCompressionLevel also takes ("fast", "max") and anything
// else are refused, so a typo is not run as fastest.
func TestValidCompressionLevel(t *testing.T) {
	for _, ok := range []string{"", "fastest", "default", "better", "best", "Best", "DEFAULT"} {
		if !ValidCompressionLevel(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"fast", "max", "3", "zstd", "fastest "} {
		if ValidCompressionLevel(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
	if ParseCompressionLevel("") != CompressionDefault {
		t.Error("empty is not default")
	}
}
