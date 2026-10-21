//go:build windows
// +build windows

package snapshot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/st-matskevich/go-vss"
)

// hidden returns a SysProcAttr that suppresses the console window a child
// process would otherwise briefly flash on screen — this app has no console
// of its own (it's a windowed GUI), so cmd/vssadmin/net always get a new,
// visible one unless told not to.
func hidden() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}

// shadowIDRe matches a bare VSS shadow-copy GUID (8-4-4-4-12 hex).
var shadowIDRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// normalizeShadowID strips optional braces and validates a VSS shadow ID,
// returning the bare GUID or "" if the name is not shadow-id-shaped.
func normalizeShadowID(name string) string {
	s := strings.TrimSpace(name)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if !shadowIDRe.MatchString(s) {
		return ""
	}
	return s
}

func SymlinkSnapshot(symlinkPath string, id string, deviceObjectPath string) (string, error) {

	snapshotSymLinkFolder := symlinkPath + "\\" + id + "\\"

	snapshotSymLinkFolder = filepath.Clean(snapshotSymLinkFolder)
	os.RemoveAll(snapshotSymLinkFolder)
	if err := os.MkdirAll(snapshotSymLinkFolder, 0700); err != nil {
		return "", fmt.Errorf("failed to create snapshot symlink folder for snapshot: %s, err: %s", id, err)
	}

	os.Remove(snapshotSymLinkFolder)

	fmt.Println("Symlink from: ", deviceObjectPath, " to: ", snapshotSymLinkFolder)

	if err := os.Symlink(deviceObjectPath, snapshotSymLinkFolder); err != nil {
		return "", fmt.Errorf("failed to create symlink from: %s to: %s, error: %s", deviceObjectPath, snapshotSymLinkFolder, err)
	}

	return snapshotSymLinkFolder, nil
}

func getAppDataFolder() (string, error) {
	// Get information about the current user
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}

	// Construct the path to the application data folder
	appDataFolder := filepath.Join(currentUser.HomeDir, "AppData", "Roaming", "PBSBackupGO")

	// Create the folder if it doesn't exist
	err = os.MkdirAll(appDataFolder, os.ModePerm)
	if err != nil {
		return "", err
	}

	return appDataFolder, nil
}

// vssTimeoutMs is how long each step of a shadow copy (gather, prepare,
// snapshot) may take, in milliseconds (go-vss passes it to IVssAsync::Wait
// and raises anything under 180000). Three minutes was not enough on a busy
// machine: a run of five backups in a row timed out in DoSnapshotSet while the
// WMI writer was slow to freeze (2026-10-21, winclient).
const vssTimeoutMs = 10 * 60 * 1000

// vssBusyWait is how long a snapshot waits, in total, for another shadow copy
// creation to finish (VSS_E_SNAPSHOT_SET_IN_PROGRESS: "Wait a few moments and
// try again"), and vssBusyRetry how often it tries again.
var (
	vssBusyWait  = 10 * time.Minute
	vssBusyRetry = 30 * time.Second
)

func CreateVSSSnapshot(paths []string, needFiles bool, backup_callback func(sn map[string]SnapShot) error) error {
	// go-vss initialises COM (a single-threaded apartment) for the length of
	// CreateSnapshot only, while the snapshot's COM objects are used again at
	// Release, after the backup. Without this, the goroutine could carry on
	// on another OS thread, and Release (BackupComplete) then called them
	// from a thread outside their apartment. Keep the whole snapshot, from
	// creation to release, on one thread with the apartment alive throughout
	// (CoInitialize here, go-vss's own then only nests inside it). The defers
	// run last, after the snapshotters are released below.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole.CoInitialize(0)
	defer ole.CoUninitialize()

	// One Snapshotter per volume: go-vss rejects reuse of a single Snapshotter
	// for a second volume ("snapshotter is already in use"), which made every
	// whole-machine backup of a disk with >= 2 mounted volumes fail. Each
	// successful snapshot is held until the backup callback has consumed them,
	// then released together.
	//
	// Releasing is not enough on its own: go-vss creates the snapshot in the
	// auto-release backup context, yet the shadow copy outlives the release
	// until the process exits (seen on Windows 10 and Server 2022). The GUI
	// stays running between backups, so each shadow lingered for hours,
	// kept copy-on-write going on the volume, and its storage showed up as
	// used space in the next machine backup (27.8 GB on a laptop C:). So
	// each of our shadows is also deleted by ID once the backup is done.
	snapshotters := make([]*vss.Snapshotter, 0, len(paths))
	var created []string
	defer func() {
		for _, s := range snapshotters {
			if err := s.Release(); err != nil {
				fmt.Printf("VSS release: %v\n", err)
			}
		}
		for _, id := range created {
			deleteShadow(id)
		}
	}()
	snapshots := make(map[string]SnapShot)

	for _, path := range paths {
		path, _ = filepath.Abs(path)
		volName := filepath.VolumeName(path)
		volName += "\\"
		subPath := path[len(volName):] //Strp C:\, 3 chars or whatever it is

		appDataFolder, err := getAppDataFolder()
		if err != nil {
			fmt.Println("Error:", err)
			return err
		}

		fmt.Print("Creating VSS Snapshot...")

		// Check VSS writers status before creating snapshot (only for the
		// warnings below; a vssadmin that does not answer must not hold up
		// the backup).
		listCtx, cancelList := context.WithTimeout(context.Background(), time.Minute)
		checkWritersCmd := exec.CommandContext(listCtx, "vssadmin", "list", "writers")
		checkWritersCmd.SysProcAttr = hidden()
		writersOutput, _ := checkWritersCmd.CombinedOutput()
		cancelList()
		writersStatus := string(writersOutput)

		// Log warnings for writers with known errors
		hasWriterWarnings := false
		if strings.Contains(writersStatus, "System Writer") && strings.Contains(writersStatus, "Last error") {
			fmt.Println("⚠️  WARNING: System Writer has errors - system state may not be fully captured")
			hasWriterWarnings = true
		}
		if strings.Contains(writersStatus, "NTDS") && (strings.Contains(writersStatus, "Last error") || strings.Contains(writersStatus, "0x800423f4")) {
			fmt.Println("⚠️  WARNING: NTDS Writer refuses to participate - Active Directory state will not be captured")
			hasWriterWarnings = true
		}
		if strings.Contains(writersStatus, "Dhcp") && strings.Contains(writersStatus, "Last error") {
			fmt.Println("⚠️  WARNING: DHCP Jet Writer has errors - DHCP configuration may not be captured")
			hasWriterWarnings = true
		}

		if hasWriterWarnings {
			fmt.Println("         → Backup will continue with available writers only")
			fmt.Println("         → File-level backup will work normally")
		}

		// Another shadow copy being made (by another program, or VSS still
		// finishing one of ours that timed out) is waited for: VSS says to
		// try again in a moment. This used to delete every shadow copy on the
		// computer (vssadmin delete shadows /all: System Restore points,
		// other backup programs' copies) and restart the VSS service. When
		// the busy snapshot was this process's own, the restart could not
		// finish (VSS waits for our in-progress call), so the backup hung for
		// good with the service stuck stopping and every queued backup behind
		// it (found 2026-10-21 running five backups in a row on winclient).
		// A context left by a crashed process is released by Windows once
		// that process is gone, so waiting covers that case too.
		// Do NOT Release a failed Snapshotter: go-vss already aborted and
		// released its components on the failed CreateSnapshot (without
		// nil-ing them), so a second Release would touch freed COM state.
		var sn *vss.Snapshotter
		snapshot, err := retryWhileBusy(func() (*vss.Snapshot, error) {
			sn = &vss.Snapshotter{}
			return sn.CreateSnapshot(volName, false, vssTimeoutMs)
		})
		if err != nil && isShadowAlreadyInProgress(err) {
			return fmt.Errorf("VSS snapshot creation failed: another shadow copy was still being made after %v: %w", vssBusyWait, err)
		}
		if err != nil {
			errMsg := err.Error()
			// Check if error is ONLY due to writer failures (0x80070005 = Access Denied, 0x800423f4 = Non-retryable)
			if strings.Contains(errMsg, "0x80070005") || strings.Contains(errMsg, "0x800423f4") {
				// These are writer-specific errors, not snapshot creation errors
				// Log but DON'T fail - the snapshot might still be usable for file backup
				fmt.Printf("⚠️  VSS Writers error during snapshot creation: %v\n", err)
				fmt.Println("         → Attempting to use snapshot anyway for file-level backup")
				// snapshot might still be valid even with writer errors - check below
			} else {
				// Other errors are critical
				return fmt.Errorf("VSS snapshot creation failed: %v", err)
			}
		}

		// Verify snapshot was actually created
		if snapshot == nil || snapshot.Id == "" {
			return fmt.Errorf("VSS snapshot creation failed: no valid snapshot created")
		}

		// Snapshot is valid: track it so it is released and deleted after the
		// backup callback.
		snapshotters = append(snapshotters, sn)
		created = append(created, snapshot.Id)

		fmt.Printf("✓ Snapshot created: %s\n", snapshot.Id)
		if hasWriterWarnings {
			fmt.Println("  Note: Snapshot created despite writer warnings - file-level backup will proceed")
		}

		_, err = SymlinkSnapshot(filepath.Join(appDataFolder, "VSS"), snapshot.Id, snapshot.DeviceObjectPath)

		if err != nil {
			return err
		}

		snapshots[path] = SnapShot{FullPath: filepath.Join(appDataFolder, "VSS", snapshot.Id, subPath), Id: snapshot.Id, ObjectPath: snapshot.DeviceObjectPath, Valid: true}

	}

	return backup_callback(snapshots)

}

// VSSCleanup removes orphaned VSS snapshots left by a previously crashed Proxmox Backup Client
// run. It deletes ONLY shadow copies Proxmox Backup Client created — recorded as the subfolder
// names of the <appData>/VSS symlink directory — never `vssadmin delete shadows
// /all`, which destroyed EVERY shadow copy on the host (other backup tools, DCs,
// SQL/Exchange) on each service start (audit v2-H-05).
//
// Best-effort by design: the worst case here is that an orphan remains until a
// later run, which is far safer than wiping other applications' shadow copies.
//
// Deletion goes through the VSS API by ID (deleteShadow): vssadmin refuses
// backup-context shadows, so the earlier vssadmin form never removed one. If a
// delete fails, the orphan simply remains (no collateral damage).
func VSSCleanup() error {
	appData, err := getAppDataFolder()
	if err != nil {
		fmt.Printf("VSS Cleanup: cannot resolve app data folder: %v\n", err)
		return nil
	}
	vssDir := filepath.Join(appData, "VSS")
	entries, err := os.ReadDir(vssDir)
	if err != nil {
		// No VSS symlink directory ⇒ no Proxmox Backup Client-created shadows to clean.
		return nil
	}

	for _, e := range entries {
		id := normalizeShadowID(e.Name())
		if id == "" {
			continue // not a shadow-id-shaped entry
		}
		marker := filepath.Join(vssDir, e.Name())

		// If the symlink no longer resolves, the shadow was already released (a
		// normal-backup leftover, not a live orphan): just drop the stale marker so
		// these don't accumulate and slow every startup.
		if _, statErr := os.Stat(marker); statErr != nil {
			_ = os.Remove(marker)
			continue
		}

		// Live symlink ⇒ the shadow still exists ⇒ a genuine orphan from a crash.
		// (vssadmin cannot delete backup-context shadows; see deleteShadowVSS.)
		fmt.Printf("VSS Cleanup: removing orphaned Proxmox Backup Client shadow %s...\n", id)
		if deleteShadow(id) {
			// Keep the marker on failure so a later run retries; never /all.
			_ = os.Remove(marker)
		}
	}

	// NOTE: we deliberately do NOT bounce the Windows VSS service, here or
	// anywhere. `net stop/start VSS` affects EVERY VSS consumer on the host — on
	// a Domain Controller, or a machine running third-party backup software
	// (Veritas Backup Exec, Windows Server Backup, SQL/Exchange agents),
	// restarting VSS can abort their in-flight snapshots and corrupt their
	// backup state. A context left by a crashed run is released by Windows once
	// that process has gone; CreateVSSSnapshot waits for it ("already in
	// progress") instead.
	return nil
}

// deleteShadow deletes one shadow copy this client created, by ID (never
// all of them), and the symlink marker that VSSCleanup would otherwise find.
// Best-effort: if it fails the marker stays, so VSSCleanup retries at the
// next start. It reports whether the shadow is gone.
func deleteShadow(id string) bool {
	bare := normalizeShadowID(id)
	if bare == "" {
		return false
	}
	if err := deleteShadowVSS(bare); err != nil {
		fmt.Printf("VSS: could not delete shadow %s (will retry at next start): %v\n", id, err)
		return false
	}
	fmt.Printf("VSS: deleted shadow %s\n", id)
	if appData, err := getAppDataFolder(); err == nil {
		_ = os.Remove(filepath.Join(appData, "VSS", "{"+strings.ToUpper(bare)+"}"))
		_ = os.Remove(filepath.Join(appData, "VSS", bare))
	}
	return true
}

// deleteShadowVSS deletes a shadow copy through the VSS API. vssadmin can't:
// our shadows are made in the backup context, and vssadmin refuses those
// ("Snapshots were found, but they were outside of your allowed context.
// Try removing them with the backup application which created them").
// COM state is per OS thread, so the whole call stays on one.
func deleteShadowVSS(bare string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole.CoInitialize(0)
	defer ole.CoUninitialize()
	c, err := vss.LoadAndInitVSS()
	if err != nil {
		return err
	}
	defer c.Release()
	// VSS_CTX_ALL is 0xffffffff in vss.h; go-vss numbers its contexts 0..6
	// with iota, so only its VSS_CTX_BACKUP (0) is right.
	if err := c.SetContext(vss.VssContext(0xffffffff)); err != nil {
		return fmt.Errorf("set context: %w", err)
	}
	guid := ole.NewGUID("{" + bare + "}")
	if guid == nil {
		return fmt.Errorf("bad shadow id %s", bare)
	}
	_, _, err = c.DeleteSnapshots(*guid)
	return err
}

// retryWhileBusy calls create until it succeeds, fails for another reason, or
// another shadow copy creation has kept VSS busy for vssBusyWait, trying again
// every vssBusyRetry.
func retryWhileBusy(create func() (*vss.Snapshot, error)) (*vss.Snapshot, error) {
	busyUntil := time.Now().Add(vssBusyWait)
	for {
		snapshot, err := create()
		if err == nil || !isShadowAlreadyInProgress(err) || time.Now().Add(vssBusyRetry).After(busyUntil) {
			return snapshot, err
		}
		fmt.Printf("⚠️  VSS busy (%v): trying again in %v\n", err, vssBusyRetry)
		time.Sleep(vssBusyRetry)
	}
}

// isShadowAlreadyInProgress detects VSS_E_SNAPSHOT_SET_IN_PROGRESS
// (0x80042316): another shadow copy creation has not finished yet. go-vss
// reports it as "... Another shadow copy creation is already in progress.
// Wait a few moments and try again."
func isShadowAlreadyInProgress(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already in progress") ||
		strings.Contains(msg, "snapshot_set_in_progress") ||
		strings.Contains(msg, "0x80042316")
}
