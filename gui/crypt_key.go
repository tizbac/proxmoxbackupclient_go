package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"pbscommon"
	"security"
)

// EncryptionKeyInfo is what the frontend needs to describe a key file without
// ever holding the key itself: the path (not secret), the fingerprint (safe to
// display, and what proves two keys match), and whether the GUI can unlock it.
type EncryptionKeyInfo struct {
	Path        string `json:"path"`
	Exists      bool   `json:"exists"`
	Fingerprint string `json:"fingerprint"`
	Created     string `json:"created,omitempty"`
	Modified    string `json:"modified,omitempty"`
	Hint        string `json:"hint,omitempty"`
	// PassphraseProtected is true for a key file created with
	// `proxmox-backup-client key create` (the default, KDF=scrypt). Its
	// passphrase is either stored in the settings or asked for when needed.
	PassphraseProtected bool `json:"passphrase_protected"`
	// OpenToOthers is true when other local accounts can read the key file
	// (keyFileOpenToOthers). The app does not change the permissions of a
	// file the user picked by itself: it warns, and RestrictEncryptionKeyFileAccess
	// does it when asked.
	OpenToOthers bool `json:"open_to_others"`
	// Usable is false when the file cannot be used by the GUI: it does not
	// exist or is malformed. When false, Reason says why.
	Usable bool   `json:"usable"`
	Reason string `json:"reason,omitempty"`
}

// inspectKeyFile collects EncryptionKeyInfo for path. It never unlocks the key.
func inspectKeyFile(path string) EncryptionKeyInfo {
	info := EncryptionKeyInfo{Path: path}
	if path == "" {
		info.Usable = true // no key configured: valid, backups are simply plain
		info.Reason = ""
		return info
	}

	st, err := os.Stat(path)
	if err != nil {
		info.Reason = fmt.Sprintf("file not found or not readable: %v", err)
		return info
	}
	info.Exists = true
	if st.IsDir() {
		info.Reason = "the path is a folder, not a key file"
		return info
	}
	if open, err := keyFileOpenToOthers(path); err == nil {
		info.OpenToOthers = open
	}

	keyCfg, err := pbscommon.LoadKeyConfig(path)
	if err != nil {
		info.Reason = fmt.Sprintf("cannot read the key file: %v", err)
		return info
	}
	info.Fingerprint = keyCfg.Fingerprint
	info.Created = keyCfg.Created
	info.Modified = keyCfg.Modified
	info.Hint = keyCfg.Hint
	info.PassphraseProtected = keyCfg.KDF != nil

	if info.PassphraseProtected {
		// Usable: the passphrase is stored in the settings or asked for when
		// needed. It cannot be checked here without it.
		info.Usable = true
		return info
	}

	// Actually unlock it: a fingerprint alone does not prove the key material is
	// intact, and a corrupt key would otherwise only fail mid-backup.
	if _, err := keyCfg.CryptConfig(nil); err != nil {
		info.Reason = fmt.Sprintf("the key cannot be read: %v", err)
		return info
	}

	info.Usable = true
	return info
}

// InspectEncryptionKeyFile reports whether a key file can be used by the GUI.
// A missing file is reported, not raised as an error: the frontend calls this
// on every keystroke while the user is typing a path.
func (a *App) InspectEncryptionKeyFile(path string) EncryptionKeyInfo {
	return inspectKeyFile(strings.TrimSpace(path))
}

// RestrictEncryptionKeyFileAccess limits who can read the key file at path to
// this account, SYSTEM and Administrators on Windows (the service reads it as
// SYSTEM), or to its owner on Linux, and returns the key's info again.
func (a *App) RestrictEncryptionKeyFileAccess(path string) (EncryptionKeyInfo, error) {
	path = strings.TrimSpace(path)
	if err := security.ValidatePath(path); err != nil {
		return EncryptionKeyInfo{}, fmt.Errorf("invalid key file path: %w", err)
	}
	if info := inspectKeyFile(path); !info.Exists || !info.Usable {
		return info, fmt.Errorf("not a usable key file: %s", info.Reason)
	}
	if err := restrictKeyAccess(path, false); err != nil {
		return EncryptionKeyInfo{}, fmt.Errorf("changing the permissions of %s: %w", path, err)
	}
	info := inspectKeyFile(path)
	writeDebugLog(fmt.Sprintf("RestrictEncryptionKeyFileAccess: %s (still open to others: %v)", path, info.OpenToOthers))
	return info, nil
}

// GenerateEncryptionKeyFile creates a brand new encryption key at path and
// returns its info (fingerprint included, so the user can confirm it later).
//
// It deliberately REFUSES to overwrite an existing file: the key file is the
// only thing that can decrypt the snapshots encrypted with it, so clobbering it
// turns every existing encrypted snapshot into garbage. The parent directory is
// created when missing, and must already exist as a directory otherwise.
func (a *App) GenerateEncryptionKeyFile(path string) (EncryptionKeyInfo, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return EncryptionKeyInfo{}, fmt.Errorf("enter a path for the key file")
	}
	if err := security.ValidatePath(path); err != nil {
		return EncryptionKeyInfo{}, fmt.Errorf("invalid key file path: %w", err)
	}

	if _, err := os.Stat(path); err == nil {
		return EncryptionKeyInfo{}, fmt.Errorf("%s already exists: it is not overwritten, as that would make the snapshots already encrypted with it unreadable", path)
	} else if !os.IsNotExist(err) {
		return EncryptionKeyInfo{}, fmt.Errorf("checking %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	createdDir := false
	if st, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			return EncryptionKeyInfo{}, fmt.Errorf("folder %s: %w", dir, err)
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return EncryptionKeyInfo{}, fmt.Errorf("creating the folder %s: %w", dir, err)
		}
		createdDir = true
		if err := restrictKeyAccess(dir, true); err != nil {
			writeDebugLog(fmt.Sprintf("GenerateEncryptionKeyFile: could not restrict access to %s: %v", dir, err))
		}
	} else if !st.IsDir() {
		return EncryptionKeyInfo{}, fmt.Errorf("%s is not a folder", dir)
	}

	encKey, err := pbscommon.GenerateEncryptionKey()
	if err != nil {
		return EncryptionKeyInfo{}, fmt.Errorf("generating the key: %w", err)
	}
	crypt, err := pbscommon.NewCryptConfig(encKey)
	if err != nil {
		return EncryptionKeyInfo{}, fmt.Errorf("invalid key: %w", err)
	}

	// SaveKeyFile writes 0600, which Windows ignores, and the parent directory's
	// ACL is usually inherited from %ProgramData% where every local user can
	// read. So lock the file down explicitly. Failure is only logged: a key on a
	// FAT/exFAT USB stick has no ACLs to set, and that is a legitimate place for it.
	if err := pbscommon.SaveKeyFile(path, crypt); err != nil {
		return EncryptionKeyInfo{}, fmt.Errorf("writing %s: %w", path, err)
	}
	if err := restrictKeyAccess(path, false); err != nil {
		writeDebugLog(fmt.Sprintf("GenerateEncryptionKeyFile: could not restrict access to %s: %v (createdDir=%v)", path, err, createdDir))
	}

	info := inspectKeyFile(path)
	if !info.Usable {
		// A key we just wrote that we cannot read back is a serious inconsistency;
		// surface it instead of handing the frontend a silently unusable path.
		return EncryptionKeyInfo{}, fmt.Errorf("the key %s was written but cannot be read back: %s", path, info.Reason)
	}
	writeDebugLog(fmt.Sprintf("GenerateEncryptionKeyFile: created %s (fingerprint %s)", path, info.Fingerprint))
	return info, nil
}

// DefaultEncryptionKeyPath returns the path a newly generated key should
// default to: <config dir>/keys/<name>.json, created on demand by
// GenerateEncryptionKeyFile.
func DefaultEncryptionKeyPath() (string, error) {
	dir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "keys", "encryption-key.json"), nil
}

// OpenEncryptionKeyDialog opens a native picker for an EXISTING key file, so the
// user does not have to type a Windows path. Returns "" when cancelled.
func (a *App) OpenEncryptionKeyDialog() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("the window is not ready yet")
	}
	defaultDir, err := os.UserHomeDir()
	if err != nil || defaultDir == "" {
		defaultDir = os.TempDir()
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choose the encryption key file",
		DefaultDirectory: defaultDir,
		Filters: []runtime.FileFilter{
			{DisplayName: "PBS key files (*.json)", Pattern: "*.json"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
}

// OpenEncryptionKeySaveDialog opens a native "save as" picker for a NEW key
// file, defaulting to DefaultEncryptionKeyPath's directory. Returns "" when
// cancelled.
func (a *App) OpenEncryptionKeySaveDialog() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("the window is not ready yet")
	}
	defaultDir, err := os.UserHomeDir()
	if err != nil || defaultDir == "" {
		defaultDir = os.TempDir()
	}
	if def, derr := DefaultEncryptionKeyPath(); derr == nil {
		if st, serr := os.Stat(filepath.Dir(def)); serr == nil && st.IsDir() {
			defaultDir = filepath.Dir(def)
		}
	}
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:            "Save the new encryption key",
		DefaultDirectory: defaultDir,
		DefaultFilename:  "encryption-key.json",
		Filters: []runtime.FileFilter{
			{DisplayName: "PBS key files (*.json)", Pattern: "*.json"},
		},
	})
}
