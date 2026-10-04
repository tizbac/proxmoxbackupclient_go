package main

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/crypto/pbkdf2"
	"pbscommon"
)

// writeKeyFile generates a real passphrase-less key file at dir/name and returns
// its path. Generating through the same pbscommon helpers the GUI uses keeps the
// fixture honest — a hand-rolled JSON blob would happily pass tests that the
// real "proxmox-backup-client key create --kdf none" file would fail.
func writeKeyFile(t *testing.T, dir, name string) string {
	t.Helper()
	encKey, err := pbscommon.GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("GenerateEncryptionKey: %v", err)
	}
	crypt, err := pbscommon.NewCryptConfig(encKey)
	if err != nil {
		t.Fatalf("NewCryptConfig: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := pbscommon.SaveKeyFile(path, crypt); err != nil {
		t.Fatalf("SaveKeyFile(%s): %v", path, err)
	}
	return path
}

// writePassphraseKeyFile writes a passphrase-protected key file, i.e. what
// `proxmox-backup-client key create` produces by default and what the GUI must
// refuse (it has no console to prompt on).
//
// It uses PBKDF2 rather than scrypt because deriveKey hardcodes scrypt's N at
// 1<<15, which would make this fixture take seconds to derive. The GUI's
// decision is driven purely by `KDF != nil`, so the KDF flavour is irrelevant
// to it — but deriving the real key here keeps the fixture honest: it is a key
// file that really can be unlocked with the right passphrase, so a test that
// reports it as "protected" cannot be passing just because the payload is junk.
func writePassphraseKeyFile(t *testing.T, dir, name, passphrase string) string {
	t.Helper()
	encKey, err := pbscommon.GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("GenerateEncryptionKey: %v", err)
	}
	crypt, err := pbscommon.NewCryptConfig(encKey)
	if err != nil {
		t.Fatalf("NewCryptConfig: %v", err)
	}

	// Same derivation pbscommon.KeyDerivationConfig.deriveKey performs for
	// "PBKDF2": sha256, keyLen = BlobEncryptionKeySize. The salt string MUST be
	// the base64 of saltBytes, or the file derives a different key than the one
	// the inner data was sealed with.
	const saltBytes = "saltsaltsalt"
	salt := base64Encode([]byte(saltBytes))
	derived := pbkdf2.Key([]byte(passphrase), []byte(saltBytes), 4096, pbscommon.BlobEncryptionKeySize, sha256.New)
	derivedCrypt, err := pbscommon.NewCryptConfig(derived)
	if err != nil {
		t.Fatalf("NewCryptConfig(derived): %v", err)
	}
	iv, sealed, tag, err := derivedCrypt.Seal(encKey)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	// CryptConfig slices the wrapped material as IV || TAG || CIPHERTEXT (see
	// KeyConfig.CryptConfig), NOT the IV||ct||tag that Seal returns.
	wrapped := append(append(append([]byte(nil), iv...), tag...), sealed...)

	cfg := map[string]any{
		"kdf": map[string]any{
			"kdf":  "PBKDF2",
			"iter": 4096,
			"salt": salt,
		},
		"hint":        passphrase[:1],
		"fingerprint": crypt.Fingerprint(),
		"data":        base64Encode(wrapped),
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func base64Encode(b []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var sb strings.Builder
	for i := 0; i < len(b); i += 3 {
		var n uint32
		rem := len(b) - i
		n = uint32(b[i]) << 16
		if rem > 1 {
			n |= uint32(b[i+1]) << 8
		}
		if rem > 2 {
			n |= uint32(b[i+2])
		}
		sb.WriteByte(alphabet[(n>>18)&0x3f])
		sb.WriteByte(alphabet[(n>>12)&0x3f])
		if rem > 1 {
			sb.WriteByte(alphabet[(n>>6)&0x3f])
		} else {
			sb.WriteByte('=')
		}
		if rem > 2 {
			sb.WriteByte(alphabet[n&0x3f])
		} else {
			sb.WriteByte('=')
		}
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// loadCryptConfig
// ---------------------------------------------------------------------------

func TestConfigLoadCryptConfigEmptyPathLeavesCryptNil(t *testing.T) {
	c := &Config{}
	if err := c.loadCryptConfig(); err != nil {
		t.Fatalf("loadCryptConfig on empty path: %v", err)
	}
	if c.Crypt != nil {
		t.Fatal("Crypt must stay nil when no key file is configured (plain snapshots)")
	}
}

func TestConfigLoadCryptConfigUnlocksKeyFile(t *testing.T) {
	path := writeKeyFile(t, t.TempDir(), "key.json")
	want, err := pbscommon.LoadKeyFile(path, nil)
	if err != nil {
		t.Fatalf("LoadKeyFile: %v", err)
	}

	c := &Config{EncryptionKeyFile: path}
	if err := c.loadCryptConfig(); err != nil {
		t.Fatalf("loadCryptConfig: %v", err)
	}
	if c.Crypt == nil {
		t.Fatal("Crypt is nil: the key file was not unlocked")
	}
	if got := c.Crypt.Fingerprint(); got != want.Fingerprint() {
		t.Fatalf("fingerprint mismatch: got %s want %s", got, want.Fingerprint())
	}
}

func TestConfigLoadCryptConfigMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.json")
	c := &Config{EncryptionKeyFile: path}
	err := c.loadCryptConfig()
	if err == nil {
		t.Fatal("expected an error for a missing key file, got nil")
	}
	// The message must name the file, otherwise the user has no idea which of
	// several configured keys is broken.
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error does not mention the offending path %q: %v", path, err)
	}
	if c.Crypt != nil {
		t.Fatal("Crypt must stay nil after a failed load")
	}
}

func TestConfigLoadCryptConfigRejectsPassphraseProtected(t *testing.T) {
	dir := t.TempDir()
	// A genuine passphrase-protected key file: loadable with the right
	// passphrase, so this test proves the GUI refuses on the KDF tag ALONE
	// (it must never try to prompt) and not because the file is broken.
	path := writePassphraseKeyFile(t, dir, "protected.json", "hunter2")
	if _, err := pbscommon.LoadKeyFile(path, []byte("hunter2")); err != nil {
		t.Fatalf("fixture is not a real passphrase key file: %v", err)
	}

	c := &Config{EncryptionKeyFile: path}
	err := c.loadCryptConfig()
	if err == nil {
		t.Fatal("expected an error for a passphrase-protected key file, got nil")
	}
	if !strings.Contains(err.Error(), "--kdf none") {
		t.Fatalf("error should point at the --kdf none workaround, got: %v", err)
	}
	if c.Crypt != nil {
		t.Fatal("Crypt must stay nil for a passphrase-protected key file")
	}
}

func TestConfigLoadCryptConfigIsIdempotent(t *testing.T) {
	path := writeKeyFile(t, t.TempDir(), "key.json")
	c := &Config{EncryptionKeyFile: path}
	for i := 0; i < 3; i++ {
		if err := c.loadCryptConfig(); err != nil {
			t.Fatalf("loadCryptConfig call %d: %v", i, err)
		}
		if c.Crypt == nil {
			t.Fatalf("Crypt nil after call %d", i)
		}
	}
	first := c.Crypt.Fingerprint()
	if err := c.loadCryptConfig(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if c.Crypt.Fingerprint() != first {
		t.Fatal("reloading the same key file produced a different key")
	}
}

// ---------------------------------------------------------------------------
// validateEncryptionKeyFile
// ---------------------------------------------------------------------------

func TestConfigValidateEncryptionKeyFile(t *testing.T) {
	dir := t.TempDir()
	good := writeKeyFile(t, dir, "good.json")
	missing := filepath.Join(dir, "missing.json")
	notAKey := filepath.Join(dir, "garbage.json")
	if err := os.WriteFile(notAKey, []byte("this is not json"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Built by concatenation: filepath.Join would clean the ".." away and this
	// case would silently degrade into "missing file".
	traversal := dir + string(filepath.Separator) + ".." + string(filepath.Separator) + "escape.json"
	protected := writePassphraseKeyFile(t, dir, "protected.json", "hunter2")

	tests := []struct {
		name    string
		path    string
		wantErr string // substring, "" means valid
	}{
		{"empty means unencrypted", "", ""},
		{"valid key file", good, ""},
		{"missing file", missing, "missing.json"},
		{"malformed file", notAKey, "garbage.json"},
		{"passphrase protected rejected", protected, "phrase de passe"},
		{"path traversal rejected", traversal, "cannot contain '..'"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{EncryptionKeyFile: tc.path}
			err := c.validateEncryptionKeyFile()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error mentioning %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestConfigValidateChecksTheKeyFile(t *testing.T) {
	// Validate() is the gate run by SaveConfig, so a bad key path must be caught
	// there instead of surfacing halfway through a backup.
	dir := t.TempDir()
	c := &Config{
		BaseURL:           "https://pbs.example.com:8007",
		AuthID:            "root@pam!backup",
		Secret:            "s3cr3t",
		Datastore:         "store",
		EncryptionKeyFile: filepath.Join(dir, "nope.json"),
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate accepted a dangling encryption key path")
	}
	if !strings.Contains(err.Error(), "nope.json") {
		t.Fatalf("error does not name the key file: %v", err)
	}

	// The same config with a real key file validates.
	c.EncryptionKeyFile = writeKeyFile(t, dir, "good.json")
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate rejected a valid key file: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Multi-PBS propagation: the reason multi-PBS mode silently ran unencrypted.
// ---------------------------------------------------------------------------

func TestPBSServerToConfigCarriesTheEncryptionKey(t *testing.T) {
	path := writeKeyFile(t, t.TempDir(), "key.json")
	pbs := &PBSServer{
		ID:                "pbs1",
		Name:              "big",
		BaseURL:           "https://pbs.example.com:8007",
		AuthID:            "root@pam!backup",
		Secret:            "s3cr3t",
		Datastore:         "store",
		Namespace:         "clients",
		EncryptionKeyFile: path,
	}
	cfg := pbs.ToConfig()
	if cfg.EncryptionKeyFile != path {
		t.Fatalf("ToConfig dropped the encryption key: got %q want %q", cfg.EncryptionKeyFile, path)
	}
}

func TestEffectivePBSCarriesTheServerEncryptionKey(t *testing.T) {
	path := writeKeyFile(t, t.TempDir(), "key.json")
	// A multi-PBS-only config: the legacy top-level fields are EMPTY, which is
	// exactly the case where the key used to be lost.
	c := &Config{
		PBSServers: map[string]*PBSServer{
			"pbs1": {
				ID:                "pbs1",
				Name:              "one",
				BaseURL:           "https://one.example.com:8007",
				AuthID:            "root@pam!backup",
				Secret:            "s3cr3t",
				Datastore:         "store",
				EncryptionKeyFile: path,
			},
			"pbs2": {
				ID:        "pbs2",
				Name:      "two",
				BaseURL:   "https://two.example.com:8007",
				AuthID:    "root@pam!backup",
				Secret:    "s3cr3t",
				Datastore: "store",
			},
		},
		DefaultPBSID: "pbs1",
	}

	eff := c.EffectivePBS()
	if eff.EncryptionKeyFile != path {
		t.Fatalf("EffectivePBS dropped the default server's key: got %q want %q", eff.EncryptionKeyFile, path)
	}

	// The other server must stay keyless rather than inheriting pbs1's key:
	// silently encrypting pbs2's backups with pbs1's key would be far worse.
	c.DefaultPBSID = "pbs2"
	eff2 := c.EffectivePBS()
	if eff2.EncryptionKeyFile != "" {
		t.Fatalf("server without a key inherited %q", eff2.EncryptionKeyFile)
	}
}

func TestEffectivePBSDoesNotMutateTheReceiver(t *testing.T) {
	path := writeKeyFile(t, t.TempDir(), "key.json")
	c := &Config{
		PBSServers: map[string]*PBSServer{
			"pbs1": {ID: "pbs1", Name: "one", BaseURL: "https://one.example.com:8007", Datastore: "store", EncryptionKeyFile: path},
		},
		DefaultPBSID: "pbs1",
	}
	_ = c.EffectivePBS()
	if c.EncryptionKeyFile != "" {
		t.Fatalf("EffectivePBS mutated the receiver's EncryptionKeyFile to %q", c.EncryptionKeyFile)
	}
	if c.BaseURL != "" {
		t.Fatalf("EffectivePBS mutated the receiver's BaseURL to %q", c.BaseURL)
	}
}

func TestPBSServerValidateRejectsUnusableKey(t *testing.T) {
	dir := t.TempDir()
	base := func() *PBSServer {
		return &PBSServer{
			ID:        "pbs1",
			Name:      "one",
			BaseURL:   "https://one.example.com:8007",
			AuthID:    "root@pam!backup",
			Secret:    "s3cr3t",
			Datastore: "store",
		}
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("baseline server should validate: %v", err)
	}

	bad := base()
	bad.EncryptionKeyFile = filepath.Join(dir, "nope.json")
	err := bad.Validate()
	if err == nil {
		t.Fatal("Validate accepted a dangling key path")
	}
	if !strings.Contains(err.Error(), "nope.json") {
		t.Fatalf("error does not name the key file: %v", err)
	}

	ok := base()
	ok.EncryptionKeyFile = writeKeyFile(t, dir, "key.json")
	if err := ok.Validate(); err != nil {
		t.Fatalf("Validate rejected a valid key path: %v", err)
	}
}

func TestPBSServerSanitizedKeepsTheKeyPath(t *testing.T) {
	// The key PATH is not secret, so the frontend needs it to display/edit the
	// setting — only Secret/Password are stripped.
	pbs := &PBSServer{ID: "pbs1", Secret: "s", Password: "p", EncryptionKeyFile: "/etc/pbs/key.json"}
	s := pbs.sanitized()
	if s.EncryptionKeyFile != "/etc/pbs/key.json" {
		t.Fatalf("sanitized() dropped the key path: %q", s.EncryptionKeyFile)
	}
	if s.Secret != "" || s.Password != "" {
		t.Fatal("sanitized() leaked credentials")
	}
}

func TestConfigJSONRoundTripsTheEncryptionKeyPath(t *testing.T) {
	// SaveConfig replaces the whole Config, so the field must survive the JSON
	// round trip used by config.json — otherwise a save silently disables
	// encryption (or breaks restores of already-encrypted snapshots).
	path := "/etc/pbs/keys/key.json"
	raw, err := json.Marshal(&Config{EncryptionKeyFile: path})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back Config
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back.EncryptionKeyFile != path {
		t.Fatalf("round trip lost the key path: %q", back.EncryptionKeyFile)
	}

	// Crypt must NEVER be persisted: it holds the unlocked key.
	encKey, err := pbscommon.GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("GenerateEncryptionKey: %v", err)
	}
	crypt, err := pbscommon.NewCryptConfig(encKey)
	if err != nil {
		t.Fatalf("NewCryptConfig: %v", err)
	}
	c := &Config{EncryptionKeyFile: path, Crypt: crypt}
	raw2, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Substring matching on "crypt" would false-positive on the field name
	// "encryption_key_file", so check for the actual key material and for the
	// field being absent from the document.
	if strings.Contains(string(raw2), base64Encode(encKey)) {
		t.Fatalf("serialized config leaks the raw key: %s", raw2)
	}
	var mapKeys map[string]json.RawMessage
	if err := json.Unmarshal(raw2, &mapKeys); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, forbidden := range []string{"Crypt", "crypt", "encKey", "idKey"} {
		if _, present := mapKeys[forbidden]; present {
			t.Fatalf("serialized config contains the runtime-only field %q: %s", forbidden, raw2)
		}
	}
}

// ---------------------------------------------------------------------------
// inspectKeyFile / GenerateEncryptionKeyFile
// ---------------------------------------------------------------------------

func TestInspectKeyFile(t *testing.T) {
	dir := t.TempDir()
	good := writeKeyFile(t, dir, "good.json")

	info := inspectKeyFile(good)
	if !info.Exists || !info.Usable {
		t.Fatalf("freshly written key reported unusable: %+v", info)
	}
	if info.PassphraseProtected {
		t.Fatal("a --kdf none key must not be reported as passphrase protected")
	}
	if info.Fingerprint == "" {
		t.Fatal("no fingerprint reported for a valid key")
	}

	// The empty path means "not configured", which is VALID, not an error the
	// user needs to see in red.
	if e := inspectKeyFile(""); !e.Usable || e.Exists {
		t.Fatalf("empty path: %+v", e)
	}

	if e := inspectKeyFile(filepath.Join(dir, "missing.json")); e.Usable || e.Exists || e.Reason == "" {
		t.Fatalf("missing file: %+v", e)
	}

	if e := inspectKeyFile(dir); e.Usable {
		t.Fatalf("a directory was accepted as a key file: %+v", e)
	}

	garbage := filepath.Join(dir, "garbage.json")
	if err := os.WriteFile(garbage, []byte("this is not json"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if e := inspectKeyFile(garbage); e.Usable || e.Reason == "" {
		t.Fatalf("garbage file: %+v", e)
	}
}

func TestInspectKeyFileFlagsPassphraseProtected(t *testing.T) {
	path := writePassphraseKeyFile(t, t.TempDir(), "protected.json", "hunter2")
	info := inspectKeyFile(path)
	if !info.PassphraseProtected {
		t.Fatalf("PBKDF2 key not flagged: %+v", info)
	}
	if info.Usable {
		t.Fatal("a passphrase-protected key must not be reported usable by the GUI")
	}
	if info.Fingerprint == "" {
		t.Fatal("the fingerprint is still displayable, so it should be populated")
	}
}

func TestGenerateEncryptionKeyFile(t *testing.T) {
	a := &App{}
	dir := t.TempDir()
	// Nested under a directory that does not exist yet: the parent must be
	// created, otherwise the default path under the config dir would fail.
	path := filepath.Join(dir, "sub", "dir", "key.json")

	info, err := a.GenerateEncryptionKeyFile(path)
	if err != nil {
		t.Fatalf("GenerateEncryptionKeyFile: %v", err)
	}
	if !info.Usable || info.Fingerprint == "" {
		t.Fatalf("generated key is not usable: %+v", info)
	}
	if st, err := os.Stat(path); err != nil {
		t.Fatalf("key file not created: %v", err)
	} else if perm := st.Mode().Perm(); runtime.GOOS != "windows" && perm != 0600 {
		t.Fatalf("key file mode %o: the key must not be world/group readable", perm)
	}

	// It must really be a key file the CLI would accept.
	crypt, err := pbscommon.LoadKeyFile(path, nil)
	if err != nil {
		t.Fatalf("generated file is not loadable: %v", err)
	}
	if crypt.Fingerprint() != info.Fingerprint {
		t.Fatal("reported fingerprint does not match the written key")
	}
}

func TestGenerateEncryptionKeyFileRefusesToOverwrite(t *testing.T) {
	a := &App{}
	dir := t.TempDir()
	path := writeKeyFile(t, dir, "key.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Overwriting is refused: the key file is the only way to read snapshots
	// already encrypted with it.
	if _, err := a.GenerateEncryptionKeyFile(path); err == nil {
		t.Fatal("GenerateEncryptionKeyFile overwrote an existing key file")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("the existing key file was modified")
	}
}

func TestGenerateEncryptionKeyFileRejectsBadPaths(t *testing.T) {
	a := &App{}
	dir := t.TempDir()
	if _, err := a.GenerateEncryptionKeyFile(""); err == nil {
		t.Fatal("expected an error for an empty path")
	}
	// filepath.Join would CLEAN the "..", hiding exactly what is being tested
	// here, so the traversing path is built by concatenation on purpose — this
	// is what a user-typed path actually looks like.
	traversal := dir + string(filepath.Separator) + ".." + string(filepath.Separator) + "escape.json"
	if _, err := a.GenerateEncryptionKeyFile(traversal); err == nil {
		t.Fatalf("expected an error for a traversing path %q", traversal)
	}
	// A path whose parent is a regular file can never work.
	file := filepath.Join(dir, "regular")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := a.GenerateEncryptionKeyFile(filepath.Join(file, "key.json")); err == nil {
		t.Fatal("expected an error when the parent is a regular file")
	}
}

// The generated key must actually round-trip data through the same Seal/Open
// pair the backup/restore paths use, otherwise "usable" would be a lie.
func TestGeneratedKeyEncryptsAndDecrypts(t *testing.T) {
	a := &App{}
	path := filepath.Join(t.TempDir(), "key.json")
	if _, err := a.GenerateEncryptionKeyFile(path); err != nil {
		t.Fatalf("GenerateEncryptionKeyFile: %v", err)
	}
	crypt, err := pbscommon.LoadKeyFile(path, nil)
	if err != nil {
		t.Fatalf("LoadKeyFile: %v", err)
	}
	plaintext := []byte("a chunk of backup data")
	iv, sealed, tag, err := crypt.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if string(sealed) == string(plaintext) {
		t.Fatal("Seal returned the plaintext unchanged")
	}
	back, err := crypt.Open(iv, sealed, tag)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if string(back) != string(plaintext) {
		t.Fatalf("round trip mismatch: %q", back)
	}
}

func TestGeneratedKeysAreDistinct(t *testing.T) {
	a := &App{}
	dir := t.TempDir()
	seen := map[string]string{}
	for i := 0; i < 5; i++ {
		path := filepath.Join(dir, "key", string(rune('a'+i))+".json")
		info, err := a.GenerateEncryptionKeyFile(path)
		if err != nil {
			t.Fatalf("GenerateEncryptionKeyFile #%d: %v", i, err)
		}
		if prev, dup := seen[info.Fingerprint]; dup {
			t.Fatalf("two generated keys share a fingerprint (%s): %s and %s", info.Fingerprint, prev, path)
		}
		seen[info.Fingerprint] = path
	}
}
