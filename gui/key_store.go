package main

// Storing an imported encryption key in a keys folder. Compiled into both
// programs: the app stores into its own folder without a service, and the
// service into its keys folder for an app that works through it (the app
// checks the key and its passphrase first, in key_import.go).

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pbscommon"
)

// storeKeyFromApp stores a key file the app checked (SaveImportedKey) in the
// service's keys folder. The app checked it with its passphrase; here it
// must at least be a key file naming its fingerprint.
func (a *App) storeKeyFromApp(out string) (ImportedKeyResult, error) {
	cfg, err := parseKeyConfig([]byte(out))
	if err != nil {
		return ImportedKeyResult{}, err
	}
	if cfg.Fingerprint == "" {
		return ImportedKeyResult{}, fmt.Errorf("the key file names no fingerprint")
	}
	dir, err := getConfigDir()
	if err != nil {
		return ImportedKeyResult{}, err
	}
	path, existed, err := storeImportedKey(filepath.Join(dir, "keys"), out, cfg, strings.ToLower(cfg.Fingerprint))
	if err != nil {
		return ImportedKeyResult{}, err
	}
	info := inspectKeyFile(path)
	writeDebugLog(fmt.Sprintf("StoreImportedKey: %s (fingerprint %s, already there: %v)", path, info.Fingerprint, existed))
	return ImportedKeyResult{Info: info, AlreadyThere: existed}, nil
}

// ImportedKeyResult is SaveImportedKey's answer.
type ImportedKeyResult struct {
	Info EncryptionKeyInfo `json:"info"`
	// AlreadyThere: the keys folder already held this key, so nothing was
	// written and Info is that file.
	AlreadyThere bool `json:"already_there"`
}

// sameKeyInFolder returns a key file in dir that holds exactly the key in
// cfg (whose fingerprint, checked, is fp): the same key data, protected the
// same way with the same salt, and naming that fingerprint. A file merely
// naming the fingerprint is not enough: a protected file's fingerprint is
// not checked without its passphrase, and the user may be importing an
// unprotected copy because a protected one (say, with a forgotten
// passphrase) cannot be used.
func sameKeyInFolder(dir string, cfg *pbscommon.KeyConfig, fp string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	norm := func(s string) string { return strings.Join(strings.Fields(s), "") }
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		have, err := pbscommon.LoadKeyConfig(p)
		if err != nil || norm(have.Data) != cfg.Data || strings.ToLower(have.Fingerprint) != fp {
			continue
		}
		switch {
		case have.KDF == nil && cfg.KDF == nil:
		case have.KDF != nil && cfg.KDF != nil:
			k := *have.KDF
			k.Salt = norm(k.Salt)
			if k != *cfg.KDF {
				continue
			}
		default:
			continue
		}
		return p
	}
	return ""
}

// storeImportedKey saves the checked key file out (parsed cfg, fingerprint
// fp) as a new file in dir, named after its fingerprint; a key already in
// dir is not saved again: that file is returned, with existed true. Nothing
// is ever overwritten.
func storeImportedKey(dir, out string, cfg *pbscommon.KeyConfig, fp string) (path string, existed bool, err error) {
	if existing := sameKeyInFolder(dir, cfg, fp); existing != "" {
		return existing, true, nil
	}
	if st, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			return "", false, fmt.Errorf("folder %s: %w", dir, err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", false, fmt.Errorf("creating the folder %s: %w", dir, err)
		}
		if err := restrictKeyAccess(dir, true); err != nil {
			writeDebugLog(fmt.Sprintf("saveImportedKey: could not restrict access to %s: %v", dir, err))
		}
	} else if !st.IsDir() {
		return "", false, fmt.Errorf("%s is not a folder", dir)
	}

	base := "encryption-key-" + strings.ReplaceAll(fp, ":", "")[:8]
	for n := 1; n < 100; n++ {
		name := base + ".json"
		if n > 1 {
			name = fmt.Sprintf("%s-%d.json", base, n)
		}
		path = filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", false, fmt.Errorf("creating %s: %w", path, err)
		}
		_, werr := f.WriteString(out)
		if serr := f.Sync(); werr == nil {
			werr = serr
		}
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			_ = os.Remove(path) // the write error is the one to report
			return "", false, fmt.Errorf("writing %s: %w", path, werr)
		}
		// Best effort, as for a generated key: a FAT/exFAT folder has no
		// permissions to set.
		if err := restrictKeyAccess(path, false); err != nil {
			writeDebugLog(fmt.Sprintf("saveImportedKey: could not restrict access to %s: %v", path, err))
		}
		if back := inspectKeyFile(path); !back.Usable || back.Fingerprint != fp {
			_ = os.Remove(path)
			return "", false, fmt.Errorf("the key was written to %s but did not read back as the same key (%s)", path, back.Reason)
		}
		return path, false, nil
	}
	return "", false, fmt.Errorf("could not find a free file name for the key in %s", dir)
}
