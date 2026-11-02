package clientcommon

import "testing"

func TestEnvelopeAddress(t *testing.T) {
	for in, want := range map[string]string{
		"backup@example.com":                 "backup@example.com",
		"  backup@example.com ":              "backup@example.com",
		"<backup@example.com>":               "backup@example.com",
		"Backups <backup@example.com>":       "backup@example.com",
		`"PBS, Office" <backup@example.com>`: "backup@example.com",
		"not an address":                     "not an address",
	} {
		if got := envelopeAddress(in); got != want {
			t.Errorf("envelopeAddress(%q) = %q; want %q", in, got, want)
		}
	}
}
