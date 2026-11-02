package pbscommon

import (
	"encoding/json"
	"strings"
	"testing"
)

// proxmox-backup-client writes the key derivation function of a protected key
// file as an externally tagged enum, not as a flat object with a "kdf" string.
func TestKeyDerivationConfigOfficialFormat(t *testing.T) {
	var scrypt KeyDerivationConfig
	in := `{"Scrypt":{"n":65536,"r":8,"p":1,"salt":"c2FsdHNhbHRzYWx0c2FsdA=="}}`
	if err := json.Unmarshal([]byte(in), &scrypt); err != nil {
		t.Fatal(err)
	}
	if scrypt.KDF != "scrypt" || scrypt.N != 65536 || scrypt.R != 8 || scrypt.P != 1 || scrypt.Salt == "" {
		t.Fatalf("scrypt parsed wrongly: %+v", scrypt)
	}

	var pbkdf2 KeyDerivationConfig
	if err := json.Unmarshal([]byte(`{"PBKDF2":{"iter":100000,"salt":"c2FsdHNhbHRzYWx0c2FsdA=="}}`), &pbkdf2); err != nil {
		t.Fatal(err)
	}
	if pbkdf2.KDF != "PBKDF2" || pbkdf2.Iter != 100000 {
		t.Fatalf("PBKDF2 parsed wrongly: %+v", pbkdf2)
	}

	out, err := json.Marshal(scrypt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), `{"Scrypt":{`) {
		t.Fatalf("scrypt marshalled in the wrong shape: %s", out)
	}

	var bad KeyDerivationConfig
	if err := json.Unmarshal([]byte(`{"kdf":"argon2"}`), &bad); err == nil {
		t.Fatal("an unknown key derivation function must be rejected")
	}
}

// Upstream's code before tizbac/proxmoxbackupclient_go#95 expected a flat
// layout, {"kdf":"scrypt",...}. A key file in that layout is read, and
// written back in the official tagged layout.
func TestKeyDerivationConfigFlatLayoutReadWrittenTagged(t *testing.T) {
	for _, c := range []struct {
		flat, prefix string
	}{
		{`{"kdf":"scrypt","n":32768,"r":8,"p":1,"salt":"c2FsdHNhbHRzYWx0c2FsdA=="}`, `{"Scrypt":{`},
		{`{"kdf":"PBKDF2","iter":4096,"salt":"c2FsdHNhbHRzYWx0c2FsdA=="}`, `{"PBKDF2":{`},
	} {
		var k KeyDerivationConfig
		if err := json.Unmarshal([]byte(c.flat), &k); err != nil {
			t.Fatalf("%s: the flat layout must still be read: %v", c.flat, err)
		}
		out, err := json.Marshal(k)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(out), c.prefix) {
			t.Fatalf("%s: written as %s, want the tagged layout %s...", c.flat, out, c.prefix)
		}
		var again KeyDerivationConfig
		if err := json.Unmarshal(out, &again); err != nil || again != k {
			t.Fatalf("%s: round trip gave %+v (err %v), want %+v", c.flat, again, err, k)
		}
	}
}
