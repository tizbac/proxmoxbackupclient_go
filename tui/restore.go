package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/salvanza/salvanza_client/gui/api"
	"pbscommon"
)

// The restore side of the window works on the shared archive reader
// (pbscommon), the same one the desktop app's Restore page is built on: a
// snapshot's catalog gives the file tree quickly, and the data archive is read
// lazily, chunk by chunk, only for what is ticked.

const (
	classicSuffix = ".pxar.didx"
	splitMeta     = ".mpxar.didx"
	splitPayload  = ".ppxar.didx"
	catalogName   = "catalog.pcat1.didx"
)

// Access is what one PBS operation needs: the service hands it out
// (POST /pbs/access) so the window never holds a stored login.
type Access struct {
	BaseURL, Fingerprint, Datastore, Namespace string
	AuthID, Secret                             string
	Ticket, CSRF                               string
	// The encryption key of the server, when the service handed it over: the
	// key file's contents and the passphrase it remembers (may be empty).
	KeyData       []byte
	KeyPassphrase string
}

func accessFrom(a *api.PBSAccess) Access {
	return Access{
		BaseURL: a.BaseURL, Fingerprint: a.CertFingerprint, Datastore: a.Datastore, Namespace: a.Namespace,
		AuthID: a.AuthID, Secret: a.Secret, Ticket: a.Ticket, CSRF: a.CSRFToken,
		KeyData: a.KeyFileData, KeyPassphrase: a.KeyPassphrase,
	}
}

// ErrNeedPassphrase is returned for an encrypted snapshot whose key is
// protected by a passphrase that is not known yet: ask for it and try again.
var ErrNeedPassphrase = errors.New("this snapshot is encrypted with a key that needs its passphrase")

// ErrNoKey is returned for an encrypted snapshot when the server has no key.
var ErrNoKey = errors.New("this snapshot is encrypted, but no encryption key is set up for this server: add its key file in the desktop app (Preferences, Servers)")

// crypt unwraps the server's key for an encrypted snapshot. passphrase
// overrides the remembered one; it is only needed for a protected key.
func (a Access) crypt(passphrase string) (*pbscommon.CryptConfig, error) {
	if len(a.KeyData) == 0 {
		return nil, ErrNoKey
	}
	cfg := &pbscommon.KeyConfig{}
	if err := json.Unmarshal(a.KeyData, cfg); err != nil || cfg.Data == "" {
		return nil, fmt.Errorf("the server's key file cannot be read")
	}
	if passphrase == "" {
		passphrase = a.KeyPassphrase
	}
	if cfg.KDF != nil && passphrase == "" {
		return nil, ErrNeedPassphrase
	}
	crypt, err := cfg.CryptConfig([]byte(passphrase))
	if err != nil {
		if cfg.KDF != nil {
			return nil, fmt.Errorf("the key could not be unlocked, is the passphrase right? (%w)", err)
		}
		return nil, err
	}
	return crypt, nil
}

// Group is a backup group on the server (a machine or Backup Set's backups).
type Group struct {
	Type, ID string
	Count    int
	Latest   time.Time
}

// SnapshotInfo is one backup of a group.
type SnapshotInfo struct {
	Type, BackupID string
	Time           time.Time
	Archives       []string // the data archives: "docs.pxar.didx" or the metadata half "docs.mpxar.didx"
	Size           int64
	Encrypted      bool
	HasCatalog     bool
}

// CatalogItem is a file or folder of a snapshot, its path starting with the archive's name.
type CatalogItem struct {
	Path string
	Dir  bool
	Size uint64
	Mod  time.Time
}

func (a Access) client(backupType, backupID string, t time.Time) *pbscommon.PBSClient {
	return &pbscommon.PBSClient{
		BaseURL: a.BaseURL, CertFingerPrint: a.Fingerprint, AuthID: a.AuthID, Secret: a.Secret,
		Ticket: a.Ticket, CSRFToken: a.CSRF, Datastore: a.Datastore, Namespace: a.Namespace,
		Insecure: a.Fingerprint != "", CompressionLevel: pbscommon.CompressionFastest,
		Manifest: pbscommon.BackupManifest{BackupID: backupID, BackupType: backupType, BackupTime: t.Unix()},
	}
}

// clientFor is client with the snapshot's key set when it is encrypted.
func (a Access) clientFor(s SnapshotInfo, passphrase string) (*pbscommon.PBSClient, error) {
	c := a.client(s.Type, s.BackupID, s.Time)
	if s.Encrypted {
		crypt, err := a.crypt(passphrase)
		if err != nil {
			return nil, err
		}
		c.Crypt = crypt
	}
	return c, nil
}

func isDataArchive(n string) bool {
	return strings.HasSuffix(n, classicSuffix) || strings.HasSuffix(n, splitMeta)
}

func archiveBase(n string) string {
	if strings.HasSuffix(n, splitMeta) {
		return strings.TrimSuffix(n, splitMeta)
	}
	return strings.TrimSuffix(n, classicSuffix)
}

func manifestToInfo(m pbscommon.BackupManifest) SnapshotInfo {
	s := SnapshotInfo{Type: m.BackupType, BackupID: m.BackupID, Time: time.Unix(m.BackupTime, 0)}
	for _, f := range m.Files {
		if isDataArchive(f.Filename) {
			s.Archives = append(s.Archives, f.Filename)
		}
		if f.Filename == catalogName {
			s.HasCatalog = true
		}
		if f.CryptMode == pbscommon.CryptModeEncrypt {
			s.Encrypted = true
		}
		s.Size += f.Size
	}
	sort.Strings(s.Archives)
	return s
}

// listGroups lists the backup groups of the datastore (and namespace).
func listGroups(a Access) ([]Group, error) {
	manifests, err := a.client("host", "", time.Now()).ListSnapshots()
	if err != nil {
		return nil, err
	}
	byKey := map[string]*Group{}
	for _, m := range manifests {
		k := m.BackupType + "/" + m.BackupID
		g := byKey[k]
		if g == nil {
			g = &Group{Type: m.BackupType, ID: m.BackupID}
			byKey[k] = g
		}
		g.Count++
		if t := time.Unix(m.BackupTime, 0); t.After(g.Latest) {
			g.Latest = t
		}
	}
	out := make([]Group, 0, len(byKey))
	for _, g := range byKey {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Latest.After(out[j].Latest) })
	return out, nil
}

// listSnapshotsOf lists the snapshots of one group, newest first.
func listSnapshotsOf(a Access, backupType, backupID string) ([]SnapshotInfo, error) {
	manifests, err := a.client(backupType, backupID, time.Now()).ListGroupSnapshots(backupType, backupID)
	if err != nil {
		return nil, err
	}
	out := make([]SnapshotInfo, 0, len(manifests))
	for _, m := range manifests {
		out = append(out, manifestToInfo(m))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}

// readCatalog lists the files of a snapshot from its catalog.
func readCatalog(a Access, s SnapshotInfo, passphrase string) ([]CatalogItem, error) {
	if !s.HasCatalog {
		return nil, fmt.Errorf("this snapshot has no catalog, so its files cannot be listed quickly")
	}
	c, err := a.clientFor(s, passphrase)
	if err != nil {
		return nil, err
	}
	c.Connect(true, s.Type)
	defer c.Close()
	ra, size, err := c.NewDIDXReaderAt(catalogName, 64, nil)
	if err != nil {
		return nil, fmt.Errorf("reading the catalog: %w", err)
	}
	buf := make([]byte, size)
	if _, err := ra.ReadAt(buf, 0); err != nil && err != io.EOF {
		return nil, fmt.Errorf("reading the catalog: %w", err)
	}
	entries, err := pbscommon.ParseCatalog(buf)
	if err != nil {
		return nil, err
	}
	out := make([]CatalogItem, 0, len(entries))
	for _, e := range entries {
		item := CatalogItem{Path: e.Path, Dir: e.IsDir, Size: e.Size}
		if e.ModTime > 0 {
			item.Mod = time.Unix(e.ModTime, 0)
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// restoreFiles extracts the ticked paths (catalog paths, archive name first;
// none means everything) below dest. Several archives each get a folder of
// their own below dest; one archive goes straight into it. It returns how many
// files were written.
func restoreFiles(a Access, s SnapshotInfo, includes []string, dest string, overwrite bool, passphrase string, progress func(file string)) (int, error) {
	if dest == "" {
		return 0, fmt.Errorf("choose a folder to restore into")
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return 0, fmt.Errorf("the destination folder: %w", err)
	}
	if len(s.Archives) == 0 {
		return 0, fmt.Errorf("this snapshot holds no files to restore (a disk image is restored with the bare-metal restore)")
	}
	c, err := a.clientFor(s, passphrase)
	if err != nil {
		return 0, err
	}
	c.Connect(true, s.Type)
	defer c.Close()

	total := 0
	for _, arch := range s.Archives {
		var mine []string
		if len(includes) == 0 {
			mine = nil
		} else {
			prefix := arch + "/"
			for _, p := range includes {
				p = strings.ReplaceAll(p, "\\", "/")
				switch {
				case p == arch:
					mine = nil
					mine = append(mine, "") // the whole archive
				case strings.HasPrefix(p, prefix):
					mine = append(mine, strings.TrimPrefix(p, prefix))
				}
			}
			if len(mine) == 0 {
				continue // nothing ticked in this archive
			}
			if len(mine) == 1 && mine[0] == "" {
				mine = nil
			}
		}
		target := dest
		if len(s.Archives) > 1 {
			target = filepath.Join(dest, archiveBase(arch))
			if err := os.MkdirAll(target, 0o755); err != nil {
				return total, err
			}
		}
		ra, size, err := c.NewDIDXReaderAt(arch, 64, nil)
		if err != nil {
			return total, fmt.Errorf("opening %s: %w", arch, err)
		}
		var reader *pbscommon.PXARReader
		if strings.HasSuffix(arch, splitMeta) {
			pra, psize, perr := c.NewDIDXReaderAt(strings.TrimSuffix(arch, splitMeta)+splitPayload, 64, nil)
			if perr != nil {
				return total, fmt.Errorf("opening the payload of %s: %w", arch, perr)
			}
			reader, err = pbscommon.NewSplitPXARReaderAt(ra, size, pra, psize)
			if err != nil {
				return total, err
			}
		} else {
			reader = pbscommon.NewPXARReaderAt(ra, size)
		}
		if progress != nil {
			reader.SetFileCallback(progress)
		}
		files, err := reader.ExtractFiltered(target, pbscommon.NormalizeIncludes(mine), overwrite)
		if err != nil {
			return total, err
		}
		total += len(files)
	}
	return total, nil
}
