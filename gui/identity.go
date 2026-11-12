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
		if c == "" || samePath(c, dst) || copiedAway(c) {
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
	keepSettingsCopy(dst, product().Folder)
	return from, nil
}

// nimbusCopiedMarker is the file Nimbus Backup 0.4 (rdemsystems/NimbusBackupClient)
// left in the folder of its 0.3 and earlier releases, C:\ProgramData\NimbusBackup,
// once it had copied that folder's settings to C:\ProgramData\ProxmoxBackupClient.
// Nimbus Backup carried on in the new folder, so the folder that holds this file
// only has the settings of before the copy: older servers and Backup Sets, and no
// encryption key, which came with 0.4.
const nimbusCopiedMarker = "COPIED-TO-ProxmoxBackupClient.txt"

// copiedAway reports whether dir's settings were copied to another folder that
// carried on (nimbusCopiedMarker): they are out of date and never taken over.
// #nosec G703 -- the folders are ProgramData's, with the brands' fixed names: not user input
func copiedAway(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, nimbusCopiedMarker))
	return err == nil
}

// adoptOverCopiedAway is adoptFolder for a dst whose settings were copied
// away (copiedAway): its out-of-date config.json would stop the takeover, so
// dst is moved out of the way first, and kept inside dst afterwards as
// <dst name>.copied-<date>, where dst's permissions cover it. Upgrading
// Nimbus Backup 0.4, which kept the 0.3 folder C:\ProgramData\NimbusBackup
// after copying it, would otherwise give the Nimbus Backup brand those 0.3
// settings instead of the current ones in C:\ProgramData\ProxmoxBackupClient.
//
// dst is renamed beside itself when it can be; when it cannot (a file held
// open), its entries are moved into dst\<name>.copied-<date>, all or none,
// the marker last. When nothing could be taken over, dst is put back as it
// was, so the next start tries again. It returns adoptFolder's result and where the old
// settings are ("" when nothing was moved).
// #nosec G703 -- the folders are ProgramData's, with the brands' fixed names: not user input
func adoptOverCopiedAway(dst string, candidates []string) (from, kept string, err error) {
	hasCandidate := false
	for _, c := range candidates {
		if c == "" || samePath(c, dst) || copiedAway(c) {
			continue
		}
		if _, err := os.Stat(filepath.Join(c, "config.json")); err == nil {
			hasCandidate = true
			break
		}
	}
	if !hasCandidate {
		// Nothing newer to take over: these are the only settings there are.
		return "", "", nil
	}
	name := filepath.Base(dst) + ".copied-" + time.Now().Format("20060102-150405")

	// Fast path: rename dst beside itself, take over, move it inside.
	aside := filepath.Join(filepath.Dir(dst), name)
	if os.Rename(dst, aside) == nil {
		from, err = adoptFolder(dst, candidates)
		if from == "" {
			// Nothing taken over: put dst back as it was. Whatever is in its
			// place now is a partial copy adoptFolder made (the source is left
			// where it was when the copy fails).
			_ = os.RemoveAll(dst)
			if rerr := os.Rename(aside, dst); rerr != nil {
				return "", aside, err
			}
			return "", "", err
		}
		inside := filepath.Join(dst, name)
		if os.Rename(aside, inside) != nil {
			return from, aside, err
		}
		return from, inside, err
	}

	// Slow path: move dst's entries into dst\<name>, all or none; the marker
	// stays on top until the takeover is done.
	inside := filepath.Join(dst, name)
	if err := os.MkdirAll(inside, 0o700); err != nil {
		return "", "", err
	}
	entries, err := os.ReadDir(dst)
	if err != nil {
		_ = os.Remove(inside)
		return "", "", err
	}
	var moved []string
	putBack := func() {
		for _, n := range moved {
			_ = os.Rename(filepath.Join(inside, n), filepath.Join(dst, n))
		}
		_ = os.Remove(inside)
	}
	for _, e := range entries {
		n := e.Name()
		if n == name || n == nimbusCopiedMarker {
			continue
		}
		if rerr := os.Rename(filepath.Join(dst, n), filepath.Join(inside, n)); rerr != nil {
			putBack()
			return "", "", fmt.Errorf("moving the out-of-date settings in %s aside: %v", dst, rerr)
		}
		moved = append(moved, n)
	}
	from, err = adoptFolder(dst, candidates)
	if from == "" {
		// Nothing taken over: what is on top now, beside the marker, is a
		// partial copy (the originals are inside); remove it and put the
		// originals back, so the next start tries again.
		if now, rerr := os.ReadDir(dst); rerr == nil {
			for _, e := range now {
				if n := e.Name(); n != name && n != nimbusCopiedMarker {
					_ = os.RemoveAll(filepath.Join(dst, n))
				}
			}
		}
		putBack()
		return "", "", err
	}
	_ = os.Rename(filepath.Join(dst, nimbusCopiedMarker), filepath.Join(inside, nimbusCopiedMarker))
	return from, inside, err
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
