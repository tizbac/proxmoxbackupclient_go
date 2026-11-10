package tui

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/salvanza/salvanza_client/gui/api"
)

// CheckEncryption proves the whole encryption chain through a running service:
// a key is made, a temporary server uses it, a folder is backed up to a real
// Proxmox Backup Server with it, and the backup is restored through the
// window's restore code and compared with the original. The server and the
// test backups are removed again; the key file it made stays in the service's
// keys folder (there is no way to delete one through the service) and its path
// is printed so the caller can remove it.
//
// The Proxmox Backup Server comes from the environment, so no secret is on a
// command line: TUI_CHECK_PBS_URL (https://host:8007), TUI_CHECK_PBS_AUTHID,
// TUI_CHECK_PBS_SECRET, TUI_CHECK_PBS_DATASTORE and TUI_CHECK_PBS_FINGERPRINT.
// The token needs to back up, read and delete its own backups.
func CheckEncryption(be Backend, stdout, stderr io.Writer) int {
	env := func(k string) string { return strings.TrimSpace(os.Getenv("TUI_CHECK_PBS_" + k)) }
	base, authID, secret, store, fp := env("URL"), env("AUTHID"), env("SECRET"), env("DATASTORE"), env("FINGERPRINT")
	if base == "" || authID == "" || secret == "" || store == "" {
		sayln(stderr, "encryption check: set TUI_CHECK_PBS_URL, _AUTHID, _SECRET, _DATASTORE (and _FINGERPRINT)")
		return 2
	}
	stamp := time.Now().Unix()
	keyName := fmt.Sprintf("tui-selftest-key-%d.json", stamp)
	srvID := fmt.Sprintf("tuiselftestenc%d", stamp)
	group := fmt.Sprintf("tui-e2e-%d", stamp)

	// 1. a new key
	key, err := be.GenerateKey(keyName)
	if err != nil {
		sayln(stderr, "generate key: "+describeErr(err))
		return 1
	}
	say(stdout, "key made: %s\nKEYFILE=%s\n", key.Fingerprint, key.Path)
	if !key.Usable || key.PassphraseProtected {
		sayln(stderr, "the new key is not usable, or has a passphrase it should not have")
		return 1
	}
	if info, err := be.RestrictKey(key.Path); err != nil || info.OpenToOthers {
		sayln(stderr, fmt.Sprintf("restrict access: %v (still open to others: %v)", err, info.OpenToOthers))
		return 1
	}
	say(stdout, "key access restricted\n")

	// 2. a temporary server that uses it
	doc := map[string]interface{}{
		"id": srvID, "name": "__tui-selftest-encryption__", "baseurl": base, "datastore": store, "certfingerprint": fp,
		"authid": authID, "secret": secret, "encryption_key_file": key.Path,
	}
	defer func() { _ = be.DeleteServer(srvID) }()
	if err := be.SaveServer(doc, true); err != nil {
		sayln(stderr, "save server: "+describeErr(err))
		return 1
	}
	var srv *Server
	if list, err := be.Servers(); err == nil {
		for i := range list {
			if list[i].ID == srvID {
				srv = &list[i]
			}
		}
	}
	if srv == nil || srv.KeyFile != key.Path {
		sayln(stderr, "the server was saved without its key file")
		return 1
	}
	if info, err := be.KeyInfo(srv.KeyFile); err != nil || info.Fingerprint != key.Fingerprint {
		sayln(stderr, fmt.Sprintf("the server's key reads back as %q (%v)", info.Fingerprint, err))
		return 1
	}
	say(stdout, "server saved with the key; the service reports the same fingerprint\n")
	defer deleteGroupHTTP(base, store, authID, secret, group)

	// 3. a backup with it
	src, err := os.MkdirTemp("", "tui-e2e-src-")
	if err != nil {
		sayln(stderr, err)
		return 1
	}
	defer func() { _ = os.RemoveAll(src) }()
	want := map[string][]byte{
		"hello.txt":      []byte("encrypted on the way up, read with the key on the way back"),
		"deep/inner.bin": bytes.Repeat([]byte{3, 1, 4, 1, 5, 9}, 40000),
	}
	for name, data := range want {
		p := filepath.Join(src, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			sayln(stderr, err)
			return 1
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			sayln(stderr, err)
			return 1
		}
	}
	if err := be.StartFolderBackup(api.BackupRequest{BackupType: "directory", BackupID: group, BackupDirs: []string{src}, PBSServerID: srvID}); err != nil {
		sayln(stderr, "start backup: "+describeErr(err))
		return 1
	}
	say(stdout, "backup started as %s\n", group)

	var snap SnapshotInfo
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		snaps, err := be.RestoreSnapshots(srvID, Group{Type: "host", ID: group})
		if err == nil && len(snaps) > 0 {
			snap = snaps[0]
			break
		}
	}
	if snap.BackupID == "" {
		sayln(stderr, "the backup did not finish in 3 minutes")
		return 1
	}
	if !snap.Encrypted {
		sayln(stderr, "the snapshot was stored UNENCRYPTED although the server has a key")
		return 1
	}
	say(stdout, "backup finished and is marked encrypted\n")

	// 4. restore it with the server's key
	items, err := be.RestoreCatalog(srvID, snap, "")
	if err != nil {
		sayln(stderr, "catalog: "+describeErr(err))
		return 1
	}
	dest, err := os.MkdirTemp("", "tui-e2e-dst-")
	if err != nil {
		sayln(stderr, err)
		return 1
	}
	defer func() { _ = os.RemoveAll(dest) }()
	n, err := be.RestoreFiles(srvID, snap, nil, dest, false, "", nil)
	if err != nil {
		sayln(stderr, "restore: "+describeErr(err))
		return 1
	}
	for name, data := range want {
		got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(got, data) {
			sayln(stderr, fmt.Sprintf("restored %s is wrong (%v)", name, err))
			return 1
		}
	}
	say(stdout, "restored %d files (%d catalog entries) with the server's key: identical to the originals\n", n, len(items))
	return 0
}

// deleteGroupHTTP removes the test backups from the Proxmox Backup Server.
func deleteGroupHTTP(base, store, authID, secret, group string) {
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, Timeout: 60 * time.Second} // #nosec G402 -- test cleanup against a test server
	q := url.Values{"backup-type": {"host"}, "backup-id": {group}}
	req, err := http.NewRequest(http.MethodDelete, base+"/api2/json/admin/datastore/"+store+"/groups?"+q.Encode(), nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "PBSAPIToken="+authID+":"+secret)
	if resp, err := client.Do(req); err == nil {
		_ = resp.Body.Close()
	}
}
