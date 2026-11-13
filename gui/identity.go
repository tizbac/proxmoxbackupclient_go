package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The program's names on disk, all from its brand (brand.go): the programs
// <FileBase>-gui and <FileBase>-svc, and the folders named after Folder.
// Before the rename to Salvanza the project was Proxmox Backup Client, with
// one set of folders shared by every brand; legacyFolder and the lists below
// name what those releases left behind, so it can be taken over or removed.

var (
	productOnce  sync.Once
	productBrand Brand
)

// product is the brand of the running program, worked out once.
func product() Brand {
	productOnce.Do(func() { productBrand = BrandFromExecutable() })
	return productBrand
}

// appExeBase and serviceExeBase are the file names, without extension, of
// this brand's app and background service: salvanza-gui, salvanza-svc.
func appExeBase() string     { return product().FileBase + "-gui" }
func serviceExeBase() string { return product().FileBase + "-svc" }

// folderName is this brand's folder name on Windows (C:\ProgramData\<it>,
// %APPDATA%\<it>); folderNameLower the Linux one (/var/lib/<it>, ~/.<it>).
func folderName() string      { return product().Folder }
func folderNameLower() string { return strings.ToLower(product().Folder) }

// roamingAppDir is the app's folder in the user's roaming profile on Windows,
// %APPDATA%\<brand folder>: its web view data and themes. The first time it
// takes over %APPDATA%\ProxmoxBackupClient, the folder before the rename.
func roamingAppDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		if home, err := os.UserHomeDir(); err == nil {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
	}
	dir := filepath.Join(appData, folderName())
	adoptRoamingOnce.Do(func() { adoptPlainFolder(dir, []string{filepath.Join(appData, legacyFolder)}) })
	return dir
}

var adoptRoamingOnce sync.Once

// legacyFolder is the folder name every brand shared before the rename:
// C:\ProgramData\ProxmoxBackupClient, %APPDATA%\ProxmoxBackupClient,
// /var/lib/proxmoxbackupclient.
const legacyFolder = "ProxmoxBackupClient"

// legacyUserFolder is the per-user settings folder before the rename.
const legacyUserFolder = ".proxmox-backup-guardian"

// otherBrandFolders are the folder names of the other brands (Windows form).
func otherBrandFolders() []string {
	var names []string
	for key, b := range brandCatalog {
		if key != product().Name && b.Folder != "" {
			names = append(names, b.Folder)
		}
	}
	return names
}

// adoptFolder gives dst the settings of an earlier install when dst has none
// yet (no config.json): of the candidate folders that have a config.json, the
// one changed last is moved to dst, so its PBS servers, keys and Backup Sets
// carry on. A copy of its settings files is kept in dst\pre-<brand>-<date>,
// for a rollback. Moving is a rename, so nothing is read twice and a big
// folder costs nothing; when the rename fails (another volume, a file held
// open) the folder is copied instead and the original left where it was.
// It returns the folder taken over, or "".
// #nosec G703 -- the folders are ProgramData's, the profile's or the home folder's, with the brands' fixed names: not user input
func adoptFolder(dst string, candidates []string) (string, error) {
	if _, err := os.Stat(filepath.Join(dst, "config.json")); err == nil {
		return "", nil
	}
	var from string
	var newest time.Time
	for _, c := range candidates {
		if c == "" || samePath(c, dst) {
			continue
		}
		fi, err := os.Stat(filepath.Join(c, "config.json"))
		if err != nil {
			continue
		}
		if from == "" || fi.ModTime().After(newest) {
			from, newest = c, fi.ModTime()
		}
	}
	if from == "" {
		return "", nil
	}
	// An empty dst (made by an earlier start without settings) is in the way
	// of the rename; anything else in it is kept and the copy fills in.
	_ = os.Remove(dst)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(from, dst); err != nil {
		if cerr := copySettingsTree(from, dst); cerr != nil {
			return "", fmt.Errorf("taking over the settings in %s: %v (moving it: %v)", from, cerr, err)
		}
	}
	// Before the copy, which can only keep what it can read.
	unlockTakenOverSettings(dst)
	keepSettingsCopy(dst, product().Folder)
	return from, nil
}

// keepSettingsCopy copies the settings files at the top of dir (the .json
// files and the key files) to dir\pre-<brand>-<date>, best effort.
// #nosec G703 -- the folders are ProgramData's, the profile's or the home folder's, with the brands' fixed names: not user input
func keepSettingsCopy(dir, brand string) {
	keep := filepath.Join(dir, "pre-"+strings.ToLower(brand)+"-"+time.Now().Format("20060102-1504"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := strings.ToLower(e.Name())
		if !strings.HasSuffix(n, ".json") && !strings.HasSuffix(n, ".key") && !strings.Contains(n, "key") {
			continue
		}
		if err := os.MkdirAll(keep, 0o700); err != nil {
			return
		}
		_ = copySettingsFile(filepath.Join(dir, e.Name()), filepath.Join(keep, e.Name()))
	}
}

// adoptPlainFolder moves an earlier install's folder that holds no settings
// (logs, the app's web view data, themes) to dst when dst does not exist.
// #nosec G703 -- the folders are ProgramData's, the profile's or the home folder's, with the brands' fixed names: not user input
func adoptPlainFolder(dst string, candidates []string) {
	if _, err := os.Stat(dst); err == nil {
		return
	}
	for _, c := range candidates {
		if c == "" || samePath(c, dst) {
			continue
		}
		if _, err := os.Stat(c); err != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return
		}
		if os.Rename(c, dst) == nil {
			return
		}
	}
}

// copySettingsTree copies the folder src to dst (made if needed), files and
// folders, keeping what dst already has.
// #nosec G703 -- the folders are ProgramData's, the profile's or the home folder's, with the brands' fixed names: not user input
func copySettingsTree(src, dst string) error {
	return filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		if _, err := os.Stat(target); err == nil {
			return nil // already there: kept
		}
		return copySettingsFile(p, target)
	})
}

// copySettingsFile copies one file to a new file dst (0600).
// #nosec G703 -- the folders are ProgramData's, the profile's or the home folder's, with the brands' fixed names: not user input
func copySettingsFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
