package tui

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRestoreLive backs a known folder up to a real test PBS with salvanza-cli,
// then lists, browses and restores part of it with the window's restore code.
// It runs only when TUI_LIVE_PBS names a JSON file with the server and a token
// (host, port, datastore, fingerprint, backup{auth_id, secret}) and TUI_CLI the
// salvanza-cli to take the backup with.
func TestRestoreLive(t *testing.T) {
	cfgPath, cli := os.Getenv("TUI_LIVE_PBS"), os.Getenv("TUI_CLI")
	if cfgPath == "" || cli == "" {
		t.Skip("TUI_LIVE_PBS and TUI_CLI not set")
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Host        string
		Port        int
		Datastore   string
		Fingerprint string
		Backup      struct {
			AuthID string `json:"auth_id"`
			Secret string
		}
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	base := fmt.Sprintf("https://%s:%d", cfg.Host, cfg.Port)
	acc := Access{BaseURL: base, Fingerprint: cfg.Fingerprint, Datastore: cfg.Datastore, AuthID: cfg.Backup.AuthID, Secret: cfg.Backup.Secret}

	// a known tree
	src := t.TempDir()
	files := map[string][]byte{
		"top.txt":              []byte("top level file"),
		"docs/readme.md":       []byte("# readme\nhello\n"),
		"docs/deep/notes.txt":  bytes.Repeat([]byte("note "), 5000),
		"pictures/one.bin":     bytes.Repeat([]byte{1, 2, 3, 4}, 70000),
		"pictures/two.bin":     bytes.Repeat([]byte{9, 8, 7}, 50000),
		"empty-dir-sibling.md": []byte("x"),
	}
	for name, data := range files {
		p := filepath.Join(src, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	id := fmt.Sprintf("tui-restore-%d", time.Now().Unix())
	cmd := exec.Command(cli, "backup-folder", "-baseurl", base, "-certfingerprint", cfg.Fingerprint, "-authid", cfg.Backup.AuthID,
		"-datastore", cfg.Datastore, "-backupdir", src, "-backup-id", id, "-novss")
	cmd.Env = append(os.Environ(), "PBS_PASSWORD="+cfg.Backup.Secret)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("backup: %v\n%s", err, tail(string(out), 1500))
	}
	t.Cleanup(func() { deleteGroup(t, base, cfg.Datastore, cfg.Backup.AuthID, cfg.Backup.Secret, id) })

	groups, err := listGroups(acc)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range groups {
		if g.ID == id && g.Type == "host" && g.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("group %s not listed: %+v", id, groups)
	}
	snaps, err := listSnapshotsOf(acc, "host", id)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("snapshots: %v %v", snaps, err)
	}
	s := snaps[0]
	if len(s.Archives) != 1 || !s.HasCatalog || s.Encrypted {
		t.Fatalf("snapshot info: %+v", s)
	}

	cat, err := readCatalog(acc, s)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, c := range cat {
		paths = append(paths, c.Path)
	}
	arch := s.Archives[0]
	for _, want := range []string{"top.txt", "docs/readme.md", "docs/deep/notes.txt", "pictures/one.bin"} {
		if !contains(paths, arch+"/"+want) {
			t.Errorf("catalog lacks %s/%s; has %v", arch, want, paths)
		}
	}

	// restore a folder and one more file; the rest must stay out
	dest := t.TempDir()
	var seen []string
	n, err := restoreFiles(acc, s, []string{arch + "/docs", arch + "/pictures/two.bin"}, dest, false, func(f string) { seen = append(seen, f) })
	if err != nil {
		t.Fatal(err)
	}
	if n < 3 {
		t.Errorf("restored %d files, want at least 3", n)
	}
	for _, name := range []string{"docs/readme.md", "docs/deep/notes.txt", "pictures/two.bin"} {
		got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("%s not restored: %v", name, err)
		} else if !bytes.Equal(got, files[name]) {
			t.Errorf("%s differs after the restore", name)
		}
	}
	for _, name := range []string{"top.txt", "pictures/one.bin", "empty-dir-sibling.md"} {
		if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(name))); err == nil {
			t.Errorf("%s was restored although it was not ticked", name)
		}
	}

	// everything, into a fresh folder
	all := t.TempDir()
	if _, err := restoreFiles(acc, s, nil, all, false, nil); err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		got, err := os.ReadFile(filepath.Join(all, filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(got, data) {
			t.Errorf("full restore: %s wrong (%v)", name, err)
		}
	}
	t.Logf("restored %d of %d files selectively; %d progress calls", n, len(files), len(seen))
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func tail(s string, n int) string {
	if len(s) > n {
		return "..." + s[len(s)-n:]
	}
	return s
}

// deleteGroup removes the test snapshots (the token needs the prune right).
func deleteGroup(t *testing.T, base, store, authID, secret, id string) {
	t.Helper()
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, Timeout: 60 * time.Second}
	req, _ := http.NewRequest("DELETE", base+"/api2/json/admin/datastore/"+store+"/groups?"+url.Values{"backup-type": {"host"}, "backup-id": {id}}.Encode(), nil)
	req.Header.Set("Authorization", "PBSAPIToken="+authID+":"+secret)
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("cleanup: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		t.Logf("cleanup of %s: HTTP %d (delete it by hand)", id, resp.StatusCode)
	}
	_ = strings.TrimSpace
}
