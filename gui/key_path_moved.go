package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// A settings folder taken over from an earlier install is moved (adoptFolder),
// keys subfolder and all, but config.json names each encryption key by its
// full path: after the move that path still names the earlier folder, which
// is gone. Every encrypted backup and restore then fails for want of a key
// that is on the computer, one folder along. repointMovedKeyPaths puts the
// paths right; LoadConfig runs it, so a computer whose folder was taken over
// by an earlier version is put right too.

// earlierSettingsFolder reports whether name is a settings folder name of an
// earlier install: the one every brand shared before the rename
// (ProxmoxBackupClient, .proxmox-backup-guardian in the home folder), the
// pbsgo package's, or a brand's (Linux has them in lower case, the home
// folder with a dot in front).
func earlierSettingsFolder(name string) bool {
	n := strings.ToLower(strings.TrimPrefix(name, "."))
	if n == strings.ToLower(legacyFolder) || n == strings.TrimPrefix(legacyUserFolder, ".") || n == "pbsgo" {
		return true
	}
	for _, b := range brandCatalog {
		if b.Folder != "" && n == strings.ToLower(b.Folder) {
			return true
		}
	}
	return false
}

// movedKeyPath is where the key file that the settings in dir name as p is
// now, when p was inside an earlier install's settings folder beside dir and
// went along when that folder became dir. It only answers for a p that is not
// there any more and whose place inside dir holds a file: a key that is still
// where the settings say, or one that is nowhere, is left as it is.
// #nosec G703 -- p is the key path in the computer's own settings, dir its settings folder
func movedKeyPath(p, dir string) (string, bool) {
	if p == "" || !filepath.IsAbs(p) {
		return "", false
	}
	if _, err := os.Stat(p); !errors.Is(err, fs.ErrNotExist) {
		return "", false
	}
	rel, err := filepath.Rel(filepath.Dir(dir), p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	if len(parts) != 2 || samePath(parts[0], filepath.Base(dir)) || !earlierSettingsFolder(parts[0]) {
		return "", false
	}
	moved := filepath.Join(dir, parts[1])
	if fi, err := os.Stat(moved); err != nil || !fi.Mode().IsRegular() {
		return "", false
	}
	return moved, true
}

// repointMovedKeyPaths gives every PBS server of the settings in dir (and the
// single server of the oldest settings) the path its encryption key has now,
// where the key went along with the settings folder (movedKeyPath). It
// returns one line per path changed, for the log.
func (c *Config) repointMovedKeyPaths(dir string) []string {
	var changed []string
	fix := func(path *string) {
		if moved, ok := movedKeyPath(*path, dir); ok {
			changed = append(changed, *path+" is now "+moved)
			*path = moved
		}
	}
	fix(&c.EncryptionKeyFile)
	for _, s := range c.PBSServers {
		if s != nil {
			fix(&s.EncryptionKeyFile)
		}
	}
	return changed
}
