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
		info.Reason = fmt.Sprintf("fichier introuvable ou illisible: %v", err)
		return info
	}
	info.Exists = true
	if st.IsDir() {
		info.Reason = "le chemin designe un dossier, pas un fichier de cle"
		return info
	}

	keyCfg, err := pbscommon.LoadKeyConfig(path)
	if err != nil {
		info.Reason = fmt.Sprintf("lecture impossible: %v", err)
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
		info.Reason = fmt.Sprintf("cle illisible: %v", err)
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
		return EncryptionKeyInfo{}, fmt.Errorf("chemin de fichier de cle requis")
	}
	if err := security.ValidatePath(path); err != nil {
		return EncryptionKeyInfo{}, fmt.Errorf("chemin de cle invalide: %w", err)
	}

	if _, err := os.Stat(path); err == nil {
		return EncryptionKeyInfo{}, fmt.Errorf("le fichier %s existe deja: refusing de l'ecraser, car cela rendrait illisibles les snapshots deja chiffres avec cette cle", path)
	} else if !os.IsNotExist(err) {
		return EncryptionKeyInfo{}, fmt.Errorf("verification de %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	if st, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			return EncryptionKeyInfo{}, fmt.Errorf("dossier %s: %w", dir, err)
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return EncryptionKeyInfo{}, fmt.Errorf("creation du dossier %s: %w", dir, err)
		}
	} else if !st.IsDir() {
		return EncryptionKeyInfo{}, fmt.Errorf("%s n'est pas un dossier", dir)
	}

	encKey, err := pbscommon.GenerateEncryptionKey()
	if err != nil {
		return EncryptionKeyInfo{}, fmt.Errorf("generation de la cle: %w", err)
	}
	crypt, err := pbscommon.NewCryptConfig(encKey)
	if err != nil {
		return EncryptionKeyInfo{}, fmt.Errorf("cle invalide: %w", err)
	}

	// SaveKeyFile writes 0600. On Windows the mode is advisory only, so the key
	// is additionally hidden from other users by ACL inheritance of the parent
	// directory created above; nothing else to do here.
	if err := pbscommon.SaveKeyFile(path, crypt); err != nil {
		return EncryptionKeyInfo{}, fmt.Errorf("ecriture de %s: %w", path, err)
	}

	info := inspectKeyFile(path)
	if !info.Usable {
		// A key we just wrote that we cannot read back is a serious inconsistency;
		// surface it instead of handing the frontend a silently unusable path.
		return EncryptionKeyInfo{}, fmt.Errorf("la cle %s a ete ecrite mais ne peut pas etre relue: %s", path, info.Reason)
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
		return "", fmt.Errorf("interface graphique non initialisee")
	}
	defaultDir, err := os.UserHomeDir()
	if err != nil || defaultDir == "" {
		defaultDir = os.TempDir()
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choisir le fichier de cle de chiffrement",
		DefaultDirectory: defaultDir,
		Filters: []runtime.FileFilter{
			{DisplayName: "Fichiers de cle PBS (*.json)", Pattern: "*.json"},
			{DisplayName: "Tous les fichiers (*.*)", Pattern: "*.*"},
		},
	})
}

// OpenEncryptionKeySaveDialog opens a native "save as" picker for a NEW key
// file, defaulting to DefaultEncryptionKeyPath's directory. Returns "" when
// cancelled.
func (a *App) OpenEncryptionKeySaveDialog() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("interface graphique non initialisee")
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
		Title:            "Enregistrer la nouvelle cle de chiffrement",
		DefaultDirectory: defaultDir,
		DefaultFilename:  "encryption-key.json",
		Filters: []runtime.FileFilter{
			{DisplayName: "Fichiers de cle PBS (*.json)", Pattern: "*.json"},
		},
	})
}
