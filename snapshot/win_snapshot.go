//go:build windows
// +build windows

package snapshot

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

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

func CreateVSSSnapshot(paths []string, backup_callback func(sn map[string]SnapShot) error) error {

	// One Snapshotter per volume: go-vss rejects reuse of a single Snapshotter
	// for a second volume ("snapshotter is already in use"), which made every
	// whole-machine backup of a disk with >= 2 mounted volumes fail. Each
	// successful snapshot is held until the backup callback has consumed them,
	// then released together.
	snapshotters := make([]*vss.Snapshotter, 0, len(paths))
	defer func() {
		for _, s := range snapshotters {
			s.Release()
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

		// Check VSS writers status before creating snapshot
		checkWritersCmd := exec.Command("vssadmin", "list", "writers")
		writersOutput, _ := checkWritersCmd.CombinedOutput()
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

		sn := &vss.Snapshotter{}
		snapshot, err := sn.CreateSnapshot(volName, false, 180)
		if err != nil && isShadowAlreadyInProgress(err) {
			// IVssBackupComponents stuck from a previous crashed run.
			// vssadmin delete shadows alone won't release it — we have to bounce
			// the VSS service to drop the orphaned context, then retry once.
			if len(snapshotters) > 0 {
				// We already hold shadows for earlier volumes in this set, and
				// vssForceReset deletes shadows / bounces the VSS service, which
				// would destroy them. Fail instead of self-sabotaging the set.
				return fmt.Errorf("VSS busy while building a multi-volume snapshot set: %w", err)
			}
			fmt.Printf("⚠️  VSS busy: %v\n", err)
			fmt.Println("         → Resetting VSS service state and retrying once...")
			// Do NOT Release the failed Snapshotter: go-vss already aborted and
			// released its components on the failed CreateSnapshot (without
			// nil-ing them), so a second Release would touch freed COM state.
			if resetErr := vssForceReset(); resetErr != nil {
				fmt.Printf("         → VSS reset failed: %v\n", resetErr)
			}
			sn = &vss.Snapshotter{}
			snapshot, err = sn.CreateSnapshot(volName, false, 180)
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

	}

	return backup_callback(snapshots)

}

// VSSCleanup removes orphaned VSS snapshots left by a previously crashed Nimbus
// run. It deletes ONLY shadow copies Nimbus created — recorded as the subfolder
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
		// No VSS symlink directory ⇒ no Nimbus-created shadows to clean.
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
		fmt.Printf("VSS Cleanup: removing orphaned Nimbus shadow %s...\n", id)
		deleteCmd := exec.Command("vssadmin", "delete", "shadows", "/shadow={"+id+"}", "/quiet")
		if out, derr := deleteCmd.CombinedOutput(); derr != nil {
			// Keep the marker so a later run retries; never fall back to /all.
			fmt.Printf("VSS Cleanup: could not delete shadow %s (best-effort, will retry): %v - %s\n", id, derr, string(out))
			continue
		}
		fmt.Printf("VSS Cleanup: removed Nimbus shadow %s\n", id)
		_ = os.Remove(marker)
	}

	// NOTE: we deliberately do NOT bounce the Windows VSS service here.
	// `net stop/start VSS` affects EVERY VSS consumer on the host — on a Domain
	// Controller, or a machine running third-party backup software (Veritas Backup
	// Exec, Windows Server Backup, SQL/Exchange agents), restarting VSS can abort
	// their in-flight snapshots and corrupt their backup state. Doing it on every
	// service startup is especially hostile and runs even when no backup is due.
	// A stuck IVssBackupComponents context from a previously crashed run ("shadow
	// copy creation already in progress") is instead recovered lazily and only when
	// it actually blocks us, by vssForceReset() on the next CreateSnapshot attempt
	// — right before our own backup. See CreateVSSSnapshot.
	return nil
}

// isShadowAlreadyInProgress detects the "shadow copy creation is already in
// progress" error returned by go-vss / Windows VSS when a previous
// IVssBackupComponents context is still held.
func isShadowAlreadyInProgress(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// Windows surfaces this either as VSS_E_BAD_STATE (0x8004230f) or as a
	// human-readable message containing "already in progress".
	return strings.Contains(msg, "already in progress") ||
		strings.Contains(msg, "0x8004230f")
}

// vssForceReset is the aggressive recovery used mid-backup when a snapshot
// attempt returns "already in progress". It deletes orphan shadows then
// bounces the VSS service so the next CreateSnapshot starts from a clean
// state.
//
// DELIBERATE /all here (unlike startup VSSCleanup which is now scoped): this runs
// ONLY when our own CreateSnapshot is already blocked by a stuck VSS context, not
// on every service start — a much smaller blast radius. The "in progress" state is
// an in-flight requester/provider sequence, not a completed shadow we can target
// by ID, so clearing it needs the service bounce below. WINDOWS-VERIFY: tighten
// this (and the error classifier above; the in-progress code may be 0x80042316,
// not 0x8004230f) before relying on it.
func vssForceReset() error {
	deleteCmd := exec.Command("vssadmin", "delete", "shadows", "/all", "/quiet")
	if out, err := deleteCmd.CombinedOutput(); err != nil {
		fmt.Printf("VSS reset: delete shadows warning: %v - %s\n", err, string(out))
	}
	return restartVSSService()
}

// restartVSSService bounces the Windows Volume Shadow Copy service. Safe at
// service startup and during error recovery because Nimbus is the only VSS
// consumer on backup-dedicated hosts, and stopping VSS just discards any
// in-flight shadow context (which is exactly what we want when it's stuck).
func restartVSSService() error {
	fmt.Println("VSS Cleanup: Restarting VSS service to clear stuck state...")
	stopCmd := exec.Command("net", "stop", "VSS")
	if out, err := stopCmd.CombinedOutput(); err != nil {
		// "service is not started" is fine — we'll start it next.
		if !strings.Contains(string(out), "not started") &&
			!strings.Contains(strings.ToLower(string(out)), "n'est pas démarr") {
			fmt.Printf("VSS service stop warning: %v - %s\n", err, string(out))
		}
	}
	startCmd := exec.Command("net", "start", "VSS")
	if out, err := startCmd.CombinedOutput(); err != nil {
		// "already started" is fine.
		if strings.Contains(string(out), "already been started") ||
			strings.Contains(strings.ToLower(string(out)), "déjà été démarr") {
			return nil
		}
		return fmt.Errorf("net start VSS failed: %v - %s", err, string(out))
	}
	fmt.Println("VSS Cleanup: VSS service restarted")
	return nil
}
