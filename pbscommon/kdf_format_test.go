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

	var flat KeyDerivationConfig
	if err := json.Unmarshal([]byte(`{"kdf":"scrypt","n":32768,"r":8,"p":1,"salt":"c2FsdHNhbHRzYWx0c2FsdA=="}`), &flat); err != nil {
		t.Fatalf("the flat layout written by earlier versions must still be read: %v", err)
	}
	if flat.KDF != "scrypt" || flat.N != 32768 || flat.R != 8 || flat.P != 1 {
		t.Fatalf("flat layout parsed wrongly: %+v", flat)
	}

	var bad KeyDerivationConfig
	if err := json.Unmarshal([]byte(`{"kdf":"argon2"}`), &bad); err == nil {
		t.Fatal("an unknown key derivation function must be rejected")
	}
}