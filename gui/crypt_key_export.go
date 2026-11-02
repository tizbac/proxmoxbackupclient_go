package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"pbscommon"
	"security"
)

// The key file is the only thing that can decrypt encrypted backups, so a
// copy of it must exist somewhere other than the computer being backed up.
// These give the user two ways to make one: a copy of the file (to a USB
// stick, say) and a paper key to print, like `proxmox-backup-client key
// paperkey`. Both are only ever made when the user asks, and both hold the
// key exactly as the file does: a passphrase-protected key stays protected.

// PaperKey is what the frontend prints. Unlike EncryptionKeyInfo it carries
// the key itself (the file's text), so it is only produced on request.
type PaperKey struct {
	Path                string `json:"path"`
	Fingerprint         string `json:"fingerprint"`
	Created             string `json:"created,omitempty"`
	Hint                string `json:"hint,omitempty"`
	PassphraseProtected bool   `json:"passphrase_protected"`
	KeyText             string `json:"key_text"`
	// QRPNG is a PNG of a QR code holding KeyText, base64, for a data: URL.
	QRPNG   string `json:"qr_png"`
	Printed string `json:"printed"`
	Host    string `json:"host,omitempty"`
	// PrintInBrowser: print through the web browser (PrintPaperKeyInBrowser),
	// because the app's window cannot print here.
	PrintInBrowser bool `json:"print_in_browser"`
}

// readKeyForExport reads and checks the key file at path, returning its text
// and parsed config.
func readKeyForExport(path string) (string, *pbscommon.KeyConfig, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil, fmt.Errorf("no key file is set")
	}
	if err := security.ValidatePath(path); err != nil {
		return "", nil, fmt.Errorf("invalid key file path: %w", err)
	}
	if info := inspectKeyFile(path); !info.Usable || !info.Exists {
		return "", nil, fmt.Errorf("not a usable key file: %s", info.Reason)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("reading %s: %w", path, err)
	}
	keyCfg, err := pbscommon.LoadKeyConfig(path)
	if err != nil {
		return "", nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return strings.TrimSpace(string(stripUTF8BOM(raw))), keyCfg, nil
}

// EncryptionPaperKey returns the key at path laid out for printing, with a QR
// code of the whole key file so it can be scanned back into a file.
func (a *App) EncryptionPaperKey(path string) (PaperKey, error) {
	text, keyCfg, err := readKeyForExport(path)
	if err != nil {
		return PaperKey{}, err
	}
	png, err := qrcode.Encode(text, qrcode.Medium, 640)
	if err != nil {
		return PaperKey{}, fmt.Errorf("making the QR code: %w", err)
	}
	host, _ := os.Hostname()
	writeDebugLog(fmt.Sprintf("EncryptionPaperKey: paper key shown for %s (fingerprint %s)", path, keyCfg.Fingerprint))
	return PaperKey{
		Path:                strings.TrimSpace(path),
		Fingerprint:         keyCfg.Fingerprint,
		Created:             keyCfg.Created,
		Hint:                keyCfg.Hint,
		PassphraseProtected: keyCfg.KDF != nil,
		KeyText:             text,
		QRPNG:               base64.StdEncoding.EncodeToString(png),
		Printed:             time.Now().Format("2006-01-02 15:04"),
		Host:                host,
		PrintInBrowser:      paperKeyPrintsInBrowser(),
	}, nil
}

// SaveEncryptionKeyCopy asks where to save a copy of the key file at path and
// writes it there, byte for byte. Returns the copy's path, or "" when the
// dialog was cancelled.
func (a *App) SaveEncryptionKeyCopy(path string) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("the window is not ready yet")
	}
	path = strings.TrimSpace(path)
	if _, _, err := readKeyForExport(path); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	fp := inspectKeyFile(path).Fingerprint
	name := "encryption-key.json"
	if short := strings.ReplaceAll(fp, ":", ""); len(short) >= 8 {
		name = "encryption-key-" + short[:8] + ".json"
	}
	dest, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save a copy of the encryption key",
		DefaultFilename: name,
		Filters: []runtime.FileFilter{
			{DisplayName: "PBS key files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil || dest == "" {
		return "", err
	}
	if same, _ := sameFile(path, dest); same {
		return "", fmt.Errorf("that is the key file itself: choose another place for the copy")
	}
	if err := atomicWriteFile(dest, raw, 0o600); err != nil {
		return "", fmt.Errorf("writing %s: %w", dest, err)
	}
	// Best effort, as for a new key: a FAT/exFAT stick has no permissions.
	if err := restrictKeyAccess(dest, false); err != nil {
		writeDebugLog(fmt.Sprintf("SaveEncryptionKeyCopy: could not restrict access to %s: %v", dest, err))
	}
	if back := inspectKeyFile(dest); back.Fingerprint != fp {
		return "", fmt.Errorf("the copy at %s does not read back as the same key (fingerprint %q, want %q)", dest, back.Fingerprint, fp)
	}
	writeDebugLog(fmt.Sprintf("SaveEncryptionKeyCopy: %s copied to %s (fingerprint %s)", path, dest, fp))
	return dest, nil
}

// sameFile reports whether a and b name the same existing file.
func sameFile(a, b string) (bool, error) {
	sa, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	sb, err := os.Stat(b)
	if err != nil {
		return false, nil
	}
	return os.SameFile(sa, sb) || filepath.Clean(a) == filepath.Clean(b), nil
}
