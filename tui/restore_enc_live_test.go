package tui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type liveCfg struct {
	Host        string
	Port        int
	Datastore   string
	Fingerprint string
	Backup      struct {
		AuthID string `json:"auth_id"`
		Secret string
	}
	Enc struct {
		Keyfile     string
		Passphrase  string
		KeyfileNone string `json:"keyfile_none"`
	}
}

func loadLive(t *testing.T) (liveCfg, Access, string, string) {
	t.Helper()
	cfgPath, cli := os.Getenv("TUI_LIVE_PBS"), os.Getenv("TUI_CLI")
	if cfgPath == "" || cli == "" {
		t.Skip("TUI_LIVE_PBS and TUI_CLI not set")
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg liveCfg
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Enc.Keyfile == "" {
		t.Skip("no test key in the config")
	}
	base := fmt.Sprintf("https://%s:%d", cfg.Host, cfg.Port)
	acc := Access{BaseURL: base, Fingerprint: cfg.Fingerprint, Datastore: cfg.Datastore, AuthID: cfg.Backup.AuthID, Secret: cfg.Backup.Secret}
	return cfg, acc, base, cli
}

func backupEncrypted(t *testing.T, cli, base string, cfg liveCfg, src, id, keyfile, pass string) {
	t.Helper()
	args := []string{"backup-folder", "-baseurl", base, "-certfingerprint", cfg.Fingerprint, "-authid", cfg.Backup.AuthID,
		"-datastore", cfg.Datastore, "-backupdir", src, "-backup-id", id, "-novss", "-keyfile", keyfile}
	if pass != "" {
		args = append(args, "-keyfile-passphrase", pass)
	}
	cmd := exec.Command(cli, args...)
	cmd.Env = append(os.Environ(), "PBS_PASSWORD="+cfg.Backup.Secret)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("encrypted backup: %v\n%s", err, tail(string(out), 1200))
	}
	t.Cleanup(func() { deleteGroup(t, base, cfg.Datastore, cfg.Backup.AuthID, cfg.Backup.Secret, id) })
}

// TestRestoreEncryptedLive takes encrypted backups with a passphrase-protected
// key made by the official Proxmox client and with a key without a passphrase,
// and restores them through the window's code with every key situation.
func TestRestoreEncryptedLive(t *testing.T) {
	cfg, acc, base, cli := loadLive(t)

	files := map[string][]byte{
		"secret.txt":         []byte("only with the key"),
		"docs/plan.md":       bytes.Repeat([]byte("plan line\n"), 4000),
		"docs/deep/data.bin": bytes.Repeat([]byte{7, 1, 4}, 90000),
		"other/readme.txt":   []byte("left out of the restore"),
	}
	src := t.TempDir()
	for name, data := range files {
		p := filepath.Join(src, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	keyScrypt, err := os.ReadFile(cfg.Enc.Keyfile)
	if err != nil {
		t.Fatal(err)
	}
	keyNone, err := os.ReadFile(cfg.Enc.KeyfileNone)
	if err != nil {
		t.Fatal(err)
	}

	stamp := time.Now().Unix()
	idPass, idNone := fmt.Sprintf("tui-enc-pass-%d", stamp), fmt.Sprintf("tui-enc-none-%d", stamp)
	backupEncrypted(t, cli, base, cfg, src, idPass, cfg.Enc.Keyfile, cfg.Enc.Passphrase)
	backupEncrypted(t, cli, base, cfg, src, idNone, cfg.Enc.KeyfileNone, "")

	snapOf := func(id string) SnapshotInfo {
		snaps, err := listSnapshotsOf(acc, "host", id)
		if err != nil || len(snaps) != 1 {
			t.Fatalf("snapshots of %s: %v %v", id, snaps, err)
		}
		if !snaps[0].Encrypted {
			t.Fatalf("%s is not marked encrypted: %+v", id, snaps[0])
		}
		return snaps[0]
	}
	sp, sn := snapOf(idPass), snapOf(idNone)

	// 1. no key at all
	if _, err := readCatalog(acc, sp, ""); !errors.Is(err, ErrNoKey) {
		t.Errorf("no key: got %v, want ErrNoKey", err)
	}
	// 2. the protected key without its passphrase
	withKey := acc
	withKey.KeyData = keyScrypt
	if _, err := readCatalog(withKey, sp, ""); !errors.Is(err, ErrNeedPassphrase) {
		t.Errorf("protected key, no passphrase: got %v, want ErrNeedPassphrase", err)
	}
	// 3. a wrong passphrase
	if _, err := readCatalog(withKey, sp, "definitely-wrong-passphrase"); err == nil || !strings.Contains(err.Error(), "could not be unlocked") {
		t.Errorf("wrong passphrase: got %v", err)
	}
	// 4. the other key cannot read this snapshot
	other := acc
	other.KeyData = keyNone
	if _, err := readCatalog(other, sp, ""); err == nil {
		t.Errorf("a different key read the snapshot")
	}

	check := func(label string, a Access, s SnapshotInfo, pass string) {
		cat, err := readCatalog(a, s, pass)
		if err != nil {
			t.Errorf("%s: catalog: %v", label, err)
			return
		}
		arch := s.Archives[0]
		var paths []string
		for _, c := range cat {
			paths = append(paths, c.Path)
		}
		for _, want := range []string{"secret.txt", "docs/plan.md", "docs/deep/data.bin"} {
			if !contains(paths, arch+"/"+want) {
				t.Errorf("%s: catalog lacks %s", label, want)
			}
		}
		dest := t.TempDir()
		n, err := restoreFiles(a, s, []string{arch + "/docs", arch + "/secret.txt"}, dest, false, pass, nil)
		if err != nil || n < 3 {
			t.Errorf("%s: restore: %d files, %v", label, n, err)
			return
		}
		for _, name := range []string{"secret.txt", "docs/plan.md", "docs/deep/data.bin"} {
			got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(name)))
			if err != nil || !bytes.Equal(got, files[name]) {
				t.Errorf("%s: %s wrong after the restore (%v)", label, name, err)
			}
		}
		if _, err := os.Stat(filepath.Join(dest, "other", "readme.txt")); err == nil {
			t.Errorf("%s: an unticked file was restored", label)
		}
	}
	// 5. the right passphrase, typed
	check("protected key, typed passphrase", withKey, sp, cfg.Enc.Passphrase)
	// 6. the passphrase the service remembers
	remembered := withKey
	remembered.KeyPassphrase = cfg.Enc.Passphrase
	check("protected key, remembered passphrase", remembered, sp, "")
	// 7. a key without a passphrase
	check("key without passphrase", other, sn, "")
}
