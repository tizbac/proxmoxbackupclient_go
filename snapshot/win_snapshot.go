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
	"time"

	"github.com/go-ole/go-ole"
	"github.com/st-matskevich/go-vss"
)

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
// machine: five backups in a row timed out in DoSnapshotSet while the WMI
// writer was slow to freeze.
const vssTimeoutMs = 10 * 60 * 1000

// vssBusyWait is how long a snapshot waits, in total, for another shadow copy
// creation to finish (VSS_E_SNAPSHOT_SET_IN_PROGRESS: "Wait a few moments and
// try again"), and vssBusyRetry how often it tries again.
var (
	vssBusyWait  = 10 * time.Minute
	vssBusyRetry = 30 * time.Second
)

func CreateVSSSnapshot(paths []string, needFiles bool, backup_callback func(sn map[string]SnapShot) error) (retErr error) {
	// go-vss initialises COM (a single-threaded apartment) for the length of
	// CreateSnapshot only, while the snapshot's COM objects are used again at
	// Release, after the backup. Without this, the goroutine could carry on
	// on another OS thread, and Release (BackupComplete) then called them
	// from a thread outside their apartment. Keep the whole snapshot, from
	// creation to release, on one thread with the apartment alive throughout
	// (CoInitialize here, go-vss's own then only nests inside it). These
	// defers run last, after the snapshotters are released below.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole.CoInitialize(0)
	defer ole.CoUninitialize()

	// One Snapshotter per volume: go-vss rejects reuse of a single Snapshotter
	// for a second volume ("snapshotter is already in use"), which made every
	// whole-machine backup of a disk with >= 2 mounted volumes fail. Each
	// successful snapshot is held until the backup callback has consumed them,
	// then released together.
	snapshotters := make([]*vss.Snapshotter, 0, len(paths))
	// IDs of the shadows this call actually created, in creation order.
	createdIDs := make([]string, 0, len(paths))
	defer func() {
		for _, s := range snapshotters {
			if err := s.Release(); err != nil {
				fmt.Printf("⚠️  VSS: releasing snapshot: %v\n", err)
			}
		}
		// Release() ends the writer session but does not remove the shadow
		// copy: go-vss makes it in the auto-release backup context, yet it
		// outlives the release, often until the process exits. A GUI or
		// service stays running between backups, so each shadow lingered for
		// hours, kept copy-on-write going on the volume, and its storage
		// counted as used space in the next machine backup (27.8 GB on a
		// laptop C:). So the shadows this call created are always deleted
		// once the backup is done, successful or not. Only ever our own IDs,
		// never a blanket `delete shadows /all`.
		if len(createdIDs) > 0 {
			deleteShadowsBestEffort(createdIDs)
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
		// it. A context left by a crashed process is released by Windows once
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

		// Snapshot is valid: track it so it is released after the backup callback.
		snapshotters = append(snapshotters, sn)

		fmt.Printf("✓ Snapshot created: %s\n", snapshot.Id)
		if hasWriterWarnings {
			fmt.Println("  Note: Snapshot created despite writer warnings - file-level backup will proceed")
		}

		_, err = SymlinkSnapshot(filepath.Join(appDataFolder, "VSS"), snapshot.Id, snapshot.DeviceObjectPath)

		if err != nil {
			return err
		}

		snapshots[path] = SnapShot{FullPath: filepath.Join(appDataFolder, "VSS", snapshot.Id, subPath), Id: snapshot.Id, ObjectPath: snapshot.DeviceObjectPath, Valid: true}
		createdIDs = append(createdIDs, snapshot.Id)

	}

	return backup_callback(snapshots)

}

// deleteShadowsBestEffort removes shadow copies this process created during a
// backup that ended in an error or was cancelled by the user, plus the
// <appData>/VSS/<id> marker symlinks pointing at them.
//
// Best-effort by design: `vssadmin delete shadows /shadow={id}` may need
// `/for=<volume>` on some Windows versions and the shadow may already be gone
// (a normal release leaves a dangling marker). Either way we log and continue —
// a leftover shadow is cleaned up by VSSCleanup() on the next start, and we
// never fall back to `/all`, which would wipe other applications' shadows.
func deleteShadowsBestEffort(ids []string) {
	appData, appDataErr := getAppDataFolder()
	for _, id := range ids {
		norm := normalizeShadowID(id)
		if norm == "" {
			continue // not a shadow-id-shaped entry
		}
		if err := deleteShadowVSS(norm); err != nil {
			fmt.Printf("⚠️  VSS: could not delete shadow %s (VSSCleanup retries at the next start): %v\n", norm, err)
			continue
		}
		fmt.Printf("✓ VSS: deleted shadow %s\n", norm)
		if appDataErr == nil {
			_ = os.Remove(filepath.Join(appData, "VSS", id))
		}
	}
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
// WINDOWS-VERIFY: the exact `vssadmin delete shadows /shadow={id}` form may
// require `/for=<volume>` on some Windows versions; we do not persist the source
// volume. The robust long-term path is the VSS API DeleteSnapshots(by ID). If the
// invocation is rejected, the orphan simply remains (no collateral damage).
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
		// vssadmin cannot delete it (see deleteShadowVSS), so use the VSS API.
		fmt.Printf("VSS Cleanup: removing orphaned Proxmox Backup Client shadow %s...\n", id)
		if derr := deleteShadowVSS(id); derr != nil {
			// Keep the marker so a later run retries; never fall back to /all.
			fmt.Printf("VSS Cleanup: could not delete shadow %s (best-effort, will retry): %v\n", id, derr)
			continue
		}
		fmt.Printf("VSS Cleanup: removed Proxmox Backup Client shadow %s\n", id)
		_ = os.Remove(marker)
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

// DetectControl reports whether a Linux block-snapshot control module is
// available. Always false on this platform.
func DetectControl() (SnapControl, bool) {
	return SnapControl{}, false
}

// deleteShadowVSS deletes one shadow copy, by ID, through the VSS API.
// vssadmin cannot: our shadows are made in the backup context, and vssadmin
// refuses those ("Snapshots were found, but they were outside of your allowed
// context. Try removing them with the backup application which created
// them"), so the earlier vssadmin calls never removed one.
//
// go-vss numbers its VSS_CTX_* constants with iota, so its VSS_CTX_ALL (6) is
// rejected (VSS_E_UNSUPPORTED_CONTEXT); the real value is 0xffffffff.
// COM state is per OS thread, so the whole call stays on one.
func deleteShadowVSS(bare string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitialize(0); err != nil {
		// S_FALSE (already initialised on this thread) is fine.
		if oe, ok := err.(*ole.OleError); !ok || oe.Code() != 1 {
			return fmt.Errorf("CoInitialize: %w", err)
		}
	}
	defer ole.CoUninitialize()
	c, err := vss.LoadAndInitVSS()
	if err != nil {
		return fmt.Errorf("VSS init: %w", err)
	}
	defer c.Release()
	const vssCtxAll = vss.VssContext(0xffffffff)
	if err := c.SetContext(vssCtxAll); err != nil {
		return fmt.Errorf("VSS context: %w", err)
	}
	guid := ole.NewGUID("{" + bare + "}")
	if guid == nil {
		return fmt.Errorf("not a shadow id: %s", bare)
	}
	if _, _, err := c.DeleteSnapshots(*guid); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}
