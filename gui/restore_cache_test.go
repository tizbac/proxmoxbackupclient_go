package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// useTempConfigDir points getConfigDir (and therefore the restore cache) at a
// throwaway directory so the tests never touch the real user profile.
func useTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("ProgramData", "")
	t.Setenv("SystemDrive", "")
	return dir
}

func testKey() snapshotCacheKey {
	return snapshotCacheKey{
		PBSID:      "root@pam!pbs",
		Datastore:  "backup",
		Namespace:  "",
		BackupType: "host",
		BackupID:   "HOST_PART-A",
		SnapshotAt: 1772366820,
	}
}

func TestSnapshotCacheKeyFingerprintIsStable(t *testing.T) {
	a, b := testKey(), testKey()
	if a.fingerprint() != b.fingerprint() {
		t.Fatal("the same key produced two different fingerprints")
	}
	if len(a.fingerprint()) != 64 {
		t.Errorf("fingerprint is %d chars, want a 64-char sha256 hex digest", len(a.fingerprint()))
	}
}

func TestSnapshotCacheKeyFingerprintIsolatesFields(t *testing.T) {
	base := testKey()
	mutations := map[string]func(k *snapshotCacheKey){
		"PBSID":      func(k *snapshotCacheKey) { k.PBSID = "other@pam!pbs" },
		"Datastore":  func(k *snapshotCacheKey) { k.Datastore = "other" },
		"Namespace":  func(k *snapshotCacheKey) { k.Namespace = "root" },
		"BackupType": func(k *snapshotCacheKey) { k.BackupType = "vm" },
		"BackupID":   func(k *snapshotCacheKey) { k.BackupID = "HOST_PART-B" },
		"SnapshotAt": func(k *snapshotCacheKey) { k.SnapshotAt++ },
	}
	seen := map[string]string{base.fingerprint(): "base"}
	for field, mutate := range mutations {
		k := base
		mutate(&k)
		fp := k.fingerprint()
		if fp == base.fingerprint() {
			t.Errorf("changing %s did not change the fingerprint", field)
			continue
		}
		if prev, dup := seen[fp]; dup {
			t.Errorf("changing %s collided with %s", field, prev)
		}
		seen[fp] = field
	}
}

func TestSnapshotCacheKeyFingerprintSeparatesFields(t *testing.T) {
	// "ab|c" and "a|bc" must not hash the same. A 0 separator byte is what
	// guarantees this; assert it on a pair where the concatenation would
	// otherwise be identical.
	a := snapshotCacheKey{PBSID: "ab", Datastore: "c"}
	b := snapshotCacheKey{PBSID: "a", Datastore: "bc"}
	if a.fingerprint() == b.fingerprint() {
		t.Error("field boundaries are not encoded in the fingerprint")
	}
}

func TestSnapshotCacheKeyFingerprintHandlesUnicode(t *testing.T) {
	base := testKey()
	base.BackupID = "hôte-π"
	other := base
	other.BackupID = "hote-p"
	if base.fingerprint() == other.fingerprint() {
		t.Error("unicode and ascii backup ids collided")
	}
}

func TestSnapshotTreeCacheRoundTrip(t *testing.T) {
	useTempConfigDir(t)
	key := testKey()
	entries := []SnapshotEntry{
		{Path: "etc/ssh/sshd_config", IsDir: false, Size: 1234, ModTime: 1772366000},
		{Path: "etc/ssh", IsDir: true, Size: 0, ModTime: 1772365900},
		{Path: "unicode/ünicode & spaces.txt", IsDir: false, Size: 7, ModTime: 1772365800},
	}
	meta := &BackupMeta{
		BackupID:      key.BackupID,
		OriginalPath:  "/srv/backup",
		Hostname:      "box",
		BackupTime:    "2026-03-01T00:07:00Z",
		ClientVersion: "test",
		OS:            "linux",
	}

	if err := saveSnapshotTreeCache(key, entries, meta); err != nil {
		t.Fatalf("saveSnapshotTreeCache: %v", err)
	}
	got, ok := loadSnapshotTreeCache(key)
	if !ok {
		t.Fatal("loadSnapshotTreeCache reported a miss right after a save")
	}
	if got.Key != key {
		t.Errorf("stored key %+v, want %+v", got.Key, key)
	}
	if got.Schema != cachedTreeSchema {
		t.Errorf("schema = %d, want %d", got.Schema, cachedTreeSchema)
	}
	if got.GeneratedAt == 0 {
		t.Error("GeneratedAt was not set")
	}
	if len(got.Entries) != len(entries) {
		t.Fatalf("stored %d entries, want %d", len(got.Entries), len(entries))
	}
	for i := range entries {
		if got.Entries[i] != entries[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got.Entries[i], entries[i])
		}
	}
	if got.Meta == nil {
		t.Fatal("Meta was dropped")
	}
	if *got.Meta != *meta {
		t.Errorf("Meta = %+v, want %+v", *got.Meta, *meta)
	}
}

func TestSnapshotTreeCacheRoundTripsNilMeta(t *testing.T) {
	useTempConfigDir(t)
	key := testKey()
	// A legacy snapshot has no sidecar; nil must mean "no sidecar", not
	// "lookup failed".
	if err := saveSnapshotTreeCache(key, []SnapshotEntry{{Path: "a"}}, nil); err != nil {
		t.Fatalf("saveSnapshotTreeCache: %v", err)
	}
	got, ok := loadSnapshotTreeCache(key)
	if !ok {
		t.Fatal("cache miss after save")
	}
	if got.Meta != nil {
		t.Errorf("Meta = %+v, want nil", *got.Meta)
	}
}

func TestSnapshotTreeCacheMiss(t *testing.T) {
	useTempConfigDir(t)
	if got, ok := loadSnapshotTreeCache(testKey()); ok || got != nil {
		t.Fatalf("loadSnapshotTreeCache on an empty cache = %+v, %v", got, ok)
	}
}

func TestSnapshotTreeCacheKeyMismatchIsIgnored(t *testing.T) {
	useTempConfigDir(t)
	key := testKey()
	if err := saveSnapshotTreeCache(key, []SnapshotEntry{{Path: "a"}}, nil); err != nil {
		t.Fatalf("saveSnapshotTreeCache: %v", err)
	}

	// Overwrite the file with a valid envelope for a different key, mimicking a
	// hash collision or a cache directory copied between profiles.
	path, err := key.filename()
	if err != nil {
		t.Fatalf("filename: %v", err)
	}
	other := key
	other.BackupID = "SOMEONE-ELSE"
	data, err := json.Marshal(cachedSnapshotTree{
		Schema:      cachedTreeSchema,
		GeneratedAt: time.Now().Unix(),
		Key:         other,
		Entries:     []SnapshotEntry{{Path: "leak.txt"}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, ok := loadSnapshotTreeCache(key)
	if ok || got != nil {
		t.Fatalf("a key mismatch was accepted: %+v", got)
	}
}

func TestSnapshotTreeCacheSchemaMismatchIsIgnored(t *testing.T) {
	useTempConfigDir(t)
	key := testKey()
	if err := saveSnapshotTreeCache(key, []SnapshotEntry{{Path: "a"}}, nil); err != nil {
		t.Fatalf("saveSnapshotTreeCache: %v", err)
	}
	path, err := key.filename()
	if err != nil {
		t.Fatalf("filename: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env cachedSnapshotTree
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	env.Schema = cachedTreeSchema + 1
	bumped, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, bumped, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got, ok := loadSnapshotTreeCache(key); ok || got != nil {
		t.Fatal("a cache file from another schema was accepted")
	}
}

func TestSnapshotTreeCacheMalformedJSONIsIgnored(t *testing.T) {
	useTempConfigDir(t)
	key := testKey()
	path, err := key.filename()
	if err != nil {
		t.Fatalf("filename: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got, ok := loadSnapshotTreeCache(key); ok || got != nil {
		t.Fatal("malformed cache JSON was accepted")
	}
}

func TestSnapshotTreeCacheSaveLeavesNoTempFile(t *testing.T) {
	useTempConfigDir(t)
	key := testKey()
	if err := saveSnapshotTreeCache(key, []SnapshotEntry{{Path: "a"}}, nil); err != nil {
		t.Fatalf("saveSnapshotTreeCache: %v", err)
	}
	dir, err := getRestoreCacheDir()
	if err != nil {
		t.Fatalf("getRestoreCacheDir: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("save left a temp file behind: %s", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Errorf("cache dir holds %d files, want 1", len(entries))
	}
}

func TestSnapshotTreeCacheFilenameIsInsideCacheDir(t *testing.T) {
	useTempConfigDir(t)
	path, err := testKey().filename()
	if err != nil {
		t.Fatalf("filename: %v", err)
	}
	dir, err := getRestoreCacheDir()
	if err != nil {
		t.Fatalf("getRestoreCacheDir: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("cache file %s is outside %s", path, dir)
	}
	if ext := filepath.Ext(path); ext != ".json" {
		t.Errorf("cache file extension is %q, want .json (trim skips other files)", ext)
	}
}

func TestTrimSnapshotTreeCacheRemovesOnlyOldJSON(t *testing.T) {
	useTempConfigDir(t)
	dir, err := getRestoreCacheDir()
	if err != nil {
		t.Fatalf("getRestoreCacheDir: %v", err)
	}

	old := filepath.Join(dir, "old.json")
	fresh := filepath.Join(dir, "fresh.json")
	stale := filepath.Join(dir, "notes.txt")
	subdir := filepath.Join(dir, "sub.json")

	if err := saveSnapshotTreeCache(testKey(), []SnapshotEntry{{Path: "a"}}, nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	for _, p := range []string{old, fresh, stale} {
		if err := os.WriteFile(p, []byte("{}"), 0644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	past := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if err := os.Chtimes(subdir, past, past); err != nil {
		t.Fatalf("chtimes dir: %v", err)
	}

	if removed := trimSnapshotTreeCache(48 * time.Hour); removed != 1 {
		t.Errorf("trimSnapshotTreeCache removed %d files, want 1", removed)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("the aged cache file survived the trim")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("the fresh cache file was removed: %v", err)
	}
	if _, err := os.Stat(stale); err != nil {
		t.Errorf("the non-JSON file was removed: %v", err)
	}
	if _, err := os.Stat(subdir); err != nil {
		t.Errorf("the subdirectory was removed: %v", err)
	}
	// The entry saveSnapshotTreeCache wrote is also fresh, so it must survive.
	path, err := testKey().filename()
	if err != nil {
		t.Fatalf("filename: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the freshly saved cache file was removed: %v", err)
	}
}

func TestTrimSnapshotTreeCacheOnEmptyDir(t *testing.T) {
	useTempConfigDir(t)
	if removed := trimSnapshotTreeCache(time.Hour); removed != 0 {
		t.Errorf("trimSnapshotTreeCache removed %d files from an empty cache, want 0", removed)
	}
}

func TestTrimSnapshotTreeCacheNegativeAgeRemovesEverything(t *testing.T) {
	useTempConfigDir(t)
	key := testKey()
	if err := saveSnapshotTreeCache(key, []SnapshotEntry{{Path: "a"}}, nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	if removed := trimSnapshotTreeCache(-time.Hour); removed != 1 {
		t.Errorf("trimSnapshotTreeCache removed %d files, want 1", removed)
	}
	if _, ok := loadSnapshotTreeCache(key); ok {
		t.Error("the trimmed entry is still loadable")
	}
}
