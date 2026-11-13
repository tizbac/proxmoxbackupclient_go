package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeKeyAt(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"kdf":null}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A key that went along when an earlier install's settings folder became this
// one is found at the same place inside this one; nothing else is touched.
func TestMovedKeyPath(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "NimbusBackup")
	writeKeyAt(t, filepath.Join(dir, "keys", "encryption-key.json"))
	writeKeyAt(t, filepath.Join(dir, "keys", "deep", "other.json"))
	elsewhere := filepath.Join(t.TempDir(), "my-key.json")
	writeKeyAt(t, elsewhere)
	stillThere := filepath.Join(parent, legacyFolder, "keys", "kept.json")
	writeKeyAt(t, stillThere)
	writeKeyAt(t, filepath.Join(dir, "keys", "kept.json"))
	if err := os.MkdirAll(filepath.Join(dir, "keys", "a-folder.json"), 0o700); err != nil {
		t.Fatal(err)
	}

	in := func(folder string, rest ...string) string {
		return filepath.Join(append([]string{parent, folder}, rest...)...)
	}
	cases := []struct {
		name string
		path string
		want string // "" = left as it is
	}{
		{"the folder every brand shared", in(legacyFolder, "keys", "encryption-key.json"), in("NimbusBackup", "keys", "encryption-key.json")},
		{"deeper in the folder", in(legacyFolder, "keys", "deep", "other.json"), in("NimbusBackup", "keys", "deep", "other.json")},
		{"another brand's folder", in("Salvanza", "keys", "encryption-key.json"), in("NimbusBackup", "keys", "encryption-key.json")},
		{"the Linux name of the shared folder", in("proxmoxbackupclient", "keys", "encryption-key.json"), in("NimbusBackup", "keys", "encryption-key.json")},
		{"the pbsgo package's folder", in("pbsgo", "keys", "encryption-key.json"), in("NimbusBackup", "keys", "encryption-key.json")},
		{"the home folder's earlier name", in(legacyUserFolder, "keys", "encryption-key.json"), in("NimbusBackup", "keys", "encryption-key.json")},
		{"no key set", "", ""},
		{"a key that is where the settings say", elsewhere, ""},
		{"a key still in the earlier folder", stillThere, ""},
		{"a key that is nowhere", in(legacyFolder, "keys", "gone.json"), ""},
		{"a folder that is not an earlier install's", in("SomethingElse", "keys", "encryption-key.json"), ""},
		{"a lost key in this folder itself", in("NimbusBackup", "keys", "gone.json"), ""},
		{"outside the folder's parent", filepath.Join(t.TempDir(), legacyFolder, "keys", "encryption-key.json"), ""},
		{"a file straight in the parent", in(legacyFolder), ""},
		{"a folder in the key's place", in(legacyFolder, "keys", "a-folder.json"), ""},
		{"not a full path", filepath.Join(legacyFolder, "keys", "encryption-key.json"), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := movedKeyPath(tc.path, dir)
			if tc.want == "" {
				if ok {
					t.Fatalf("%s: moved to %s, want it left alone", tc.path, got)
				}
				return
			}
			if !ok || !samePath(got, tc.want) {
				t.Fatalf("%s: got %q (%v), want %q", tc.path, got, ok, tc.want)
			}
		})
	}
}

// Every server's key, and the single server's of the oldest settings, is
// looked at; one line is returned per path changed.
func TestRepointMovedKeyPaths(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "Salvanza")
	now := filepath.Join(dir, "keys", "encryption-key.json")
	writeKeyAt(t, now)
	own := filepath.Join(t.TempDir(), "own.json")
	writeKeyAt(t, own)
	was := filepath.Join(parent, legacyFolder, "keys", "encryption-key.json")

	c := &Config{
		EncryptionKeyFile: was,
		PBSServers: map[string]*PBSServer{
			"a": {ID: "a", EncryptionKeyFile: was},
			"b": {ID: "b", EncryptionKeyFile: own},
			"c": {ID: "c"},
			"d": nil,
		},
	}
	changed := c.repointMovedKeyPaths(dir)
	if len(changed) != 2 {
		t.Fatalf("changed %d path(s), want 2: %v", len(changed), changed)
	}
	if !samePath(c.EncryptionKeyFile, now) || !samePath(c.PBSServers["a"].EncryptionKeyFile, now) {
		t.Errorf("moved key not followed: %q, %q", c.EncryptionKeyFile, c.PBSServers["a"].EncryptionKeyFile)
	}
	if c.PBSServers["b"].EncryptionKeyFile != own || c.PBSServers["c"].EncryptionKeyFile != "" {
		t.Errorf("other keys touched: %q, %q", c.PBSServers["b"].EncryptionKeyFile, c.PBSServers["c"].EncryptionKeyFile)
	}
	if again := c.repointMovedKeyPaths(dir); len(again) != 0 {
		t.Errorf("a second look changed something: %v", again)
	}
}

// The whole of it: Proxmox Backup Client's folder, with a key in its keys
// folder, is taken over; the settings then name the key in the new folder.
func TestTakenOverFolderKeepsItsKey(t *testing.T) {
	programData := t.TempDir()
	earlier := filepath.Join(programData, legacyFolder)
	dir := filepath.Join(programData, folderName())
	was := filepath.Join(earlier, "keys", "encryption-key.json")
	writeKeyAt(t, was)
	raw, err := json.Marshal(&Config{DefaultPBSID: "a", PBSServers: map[string]*PBSServer{
		"a": {ID: "a", BaseURL: "https://pbs.example:8007", Datastore: "store", EncryptionKeyFile: was},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(earlier, "config.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	adoptWindowsState(programData, dir)

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("settings not taken over: %v", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	if got := c.PBSServers["a"].EncryptionKeyFile; !samePath(got, was) {
		t.Fatalf("the takeover itself changed the key path to %q", got)
	}
	if changed := c.repointMovedKeyPaths(dir); len(changed) != 1 {
		t.Fatalf("changed %v, want the one key", changed)
	}
	got := c.PBSServers["a"].EncryptionKeyFile
	if want := filepath.Join(dir, "keys", "encryption-key.json"); !samePath(got, want) {
		t.Fatalf("key path %q, want %q", got, want)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("no key at the new path: %v", err)
	}
}

// LoadConfig puts the path right and saves it, so a computer whose folder was
// taken over by an earlier version finds its key again.
func TestLoadConfigFollowsMovedKey(t *testing.T) {
	programData := t.TempDir()
	useServiceDirForTest(t)
	t.Setenv("ProgramData", programData)
	dir, err := getConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(filepath.Dir(dir), programData) {
		t.Fatalf("settings folder %s is not in %s", dir, programData)
	}
	now := filepath.Join(dir, "keys", "encryption-key.json")
	writeKeyAt(t, now)
	was := filepath.Join(programData, legacyFolder, "keys", "encryption-key.json")
	raw, err := json.Marshal(&Config{DefaultPBSID: "a", PBSServers: map[string]*PBSServer{
		"a": {ID: "a", BaseURL: "https://pbs.example:8007", Datastore: "store", EncryptionKeyFile: was},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	c := LoadConfig()
	if s := c.PBSServers["a"]; s == nil || !samePath(s.EncryptionKeyFile, now) {
		t.Fatalf("loaded settings name the key at %+v, want %s", s, now)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved Config
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if s := saved.PBSServers["a"]; s == nil || !samePath(s.EncryptionKeyFile, now) {
		t.Fatalf("config.json still names the key at %+v, want %s", s, now)
	}
}
