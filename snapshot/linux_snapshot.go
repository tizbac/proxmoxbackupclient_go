//go:build linux
// +build linux

package snapshot

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type snapControl struct {
	name      string
	devPrefix string
	module    string
	ctlDevice string
	infoFile  string
}

type linuxSnapshot struct {
	control    snapControl
	minor      int
	device     string
	mountpoint string
	cowFile    string
}

var (
	trackedMu sync.Mutex
	tracked   []*linuxSnapshot
)

// Several backups can run at once in one process now. elastio-snap and
// dattobd allow one snapshot per block device, and setting one up first
// clears any tracer of ours left on the device (destroyStaleTracers), so a
// second backup of the same filesystem used to destroy the first one's
// snapshot under it. A backup now holds each device it snapshots for as long
// as its snapshots last, and one that needs a device another holds waits for
// it (different devices are snapshotted side by side). setupMu makes the
// set-up itself one at a time, so two cannot take the same minor number.
var (
	devMu   sync.Mutex
	devCond = sync.NewCond(&devMu)
	devHeld = map[string]bool{}
	setupMu sync.Mutex
)

// holdDevices waits until none of devs is held by another backup in this
// process, then holds them all at once (all at once, so two backups can never
// each hold one the other waits for). A cancelled ctx ends the wait.
func holdDevices(ctx context.Context, devs []string) error {
	stop := context.AfterFunc(ctx, func() {
		devMu.Lock()
		devCond.Broadcast()
		devMu.Unlock()
	})
	defer stop()
	devMu.Lock()
	defer devMu.Unlock()
	logged := false
	for {
		busy := ""
		for _, d := range devs {
			if devHeld[d] {
				busy = d
				break
			}
		}
		if busy == "" {
			for _, d := range devs {
				devHeld[d] = true
			}
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !logged {
			log.Printf("Waiting for the snapshot of %s: another backup is reading from one", busy)
			logged = true
		}
		devCond.Wait()
	}
}

func releaseDevices(devs []string) {
	devMu.Lock()
	for _, d := range devs {
		delete(devHeld, d)
	}
	devCond.Broadcast()
	devMu.Unlock()
}

func detectControl() (snapControl, bool) {
	candidates := []snapControl{
		{name: "elastio-snap", devPrefix: "/dev/elastio-snap", module: "elastio-snap", ctlDevice: "/dev/elastio-snap-ctl", infoFile: "/proc/elastio-snap-info"},
		{name: "dattobd", devPrefix: "/dev/datto", module: "dattobd", ctlDevice: "/dev/datto-ctl", infoFile: "/proc/datto-info"},
	}
	for _, c := range candidates {
		if ensureModule(c) {
			return c, true
		}
	}
	return snapControl{}, false
}

// DetectControl reports which block snapshot module a machine backup would
// use, for the GUI to show (upstream e87213f). Like a backup, it loads the
// module with modprobe when it is installed but not loaded yet, which needs
// root; run as a normal user it finds only a module that is already loaded.
func DetectControl() (SnapControl, bool) {
	c, ok := detectControl()
	if !ok {
		return SnapControl{}, false
	}
	return SnapControl{Name: c.name, DevPrefix: c.devPrefix, Module: c.module, CtlDevice: c.ctlDevice, InfoFile: c.infoFile}, true
}

func moduleLoaded(module string) bool {
	f, err := os.Open("/proc/modules")
	if err != nil {
		return false
	}
	defer f.Close()
	want := strings.ReplaceAll(module, "-", "_")
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		name, _, _ := strings.Cut(sc.Text(), " ")
		if strings.ReplaceAll(name, "-", "_") == want {
			return true
		}
	}
	return false
}

func ensureModule(c snapControl) bool {
	if !moduleLoaded(c.module) {
		if out, err := exec.Command("modprobe", c.module).CombinedOutput(); err != nil {
			log.Printf("Warning: modprobe %s failed: %v: %s", c.module, err, strings.TrimSpace(string(out)))
			return false
		}
	}
	if !moduleLoaded(c.module) {
		return false
	}
	if c.ctlDevice != "" {
		if _, err := os.Stat(c.ctlDevice); err != nil {
			log.Printf("Warning: %s module is loaded but control device %s is missing", c.name, c.ctlDevice)
			return false
		}
	}
	return true
}

func unescapeMountinfo(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func findMount(path string) (mountpoint, device, fstype string, err error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", "", "", err
	}
	defer f.Close()

	bestLen := -1
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {

		fields := strings.Fields(sc.Text())
		if len(fields) < 10 {
			continue
		}
		sep := -1
		for i, fld := range fields {
			if fld == "-" {
				sep = i
				break
			}
		}
		if sep == -1 || sep+2 >= len(fields) {
			continue
		}
		mp := unescapeMountinfo(fields[4])
		fst := fields[sep+1]
		src := unescapeMountinfo(fields[sep+2])

		if path == mp || mp == "/" || strings.HasPrefix(path, strings.TrimRight(mp, "/")+"/") {
			if len(mp) > bestLen {
				bestLen = len(mp)
				mountpoint, device, fstype = mp, src, fst
			}
		}
	}
	if err := sc.Err(); err != nil {
		return "", "", "", err
	}
	if bestLen == -1 {
		return "", "", "", fmt.Errorf("could not find mount for %s", path)
	}
	if !strings.HasPrefix(device, "/dev/") {
		return "", "", "", fmt.Errorf("mount source %q for %s is not a block device", device, path)
	}
	return mountpoint, device, fstype, nil
}

func allocMinor(c snapControl) (int, error) {
	trackedMu.Lock()
	inUse := make(map[int]bool)
	for _, t := range tracked {
		if t.control.devPrefix == c.devPrefix {
			inUse[t.minor] = true
		}
	}
	trackedMu.Unlock()

	for n := 0; n < 256; n++ {
		if inUse[n] {
			continue
		}
		if _, err := os.Stat(fmt.Sprintf("%s%d", c.devPrefix, n)); os.IsNotExist(err) {
			return n, nil
		}
	}
	return 0, fmt.Errorf("no free snapshot minor number available")
}

type snapInfo struct {
	Devices []struct {
		Minor       int    `json:"minor"`
		CowFile     string `json:"cow_file"`
		BlockDevice string `json:"block_device"`
	} `json:"devices"`
}

func isOurCowFile(cowFile string) bool {
	base := filepath.Base(cowFile)
	return strings.HasPrefix(base, ".pbs_snapshot_") && strings.HasSuffix(base, ".cow")
}

// destroyStaleTracers removes tracers of a previous crashed run that are still
// attached to device. Failing to do so means the device tracer state cannot be
// trusted, so every failure is propagated to the caller. The caller holds the
// device's lock file (linux_lock.go), so no live backup in another program
// owns them; one still mounted (its run was killed) is unmounted first.
func destroyStaleTracers(c snapControl, device, originMount string) error {
	if c.infoFile == "" {
		return nil
	}
	data, err := os.ReadFile(c.infoFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s state: %w", c.infoFile, err)
	}
	var info snapInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return fmt.Errorf("parsing %s state: %w", c.infoFile, err)
	}
	var errs []error
	for _, d := range info.Devices {
		if d.BlockDevice != device || !isOurCowFile(d.CowFile) {
			continue
		}
		log.Printf("Removing stale %s snapshot %d left tracing %s by a previous run",
			c.name, d.Minor, device)
		unmountLeftovers(fmt.Sprintf("%s%d", c.devPrefix, d.Minor))
		if err := ioctlDestroySnapshot(c.ctlDevice, c.name, d.Minor); err != nil {
			errs = append(errs, fmt.Errorf("could not destroy stale %s snapshot %d on %s: %w",
				c.name, d.Minor, device, err))
			continue
		}
		os.Remove(filepath.Join(originMount, filepath.Base(d.CowFile)))
	}
	return errors.Join(errs...)
}

func createOne(c snapControl, absPath string, needFiles bool) (*linuxSnapshot, string, error) {
	mountpoint, device, fstype, err := findMount(absPath)
	if err != nil {
		return nil, "", err
	}
	// The module would refuse with only "invalid argument" (seen on Fedora
	// 44, whose default file system is btrfs): say what is wrong instead.
	if fstype == "btrfs" {
		return nil, "", fmt.Errorf("%w (btrfs, mounted at %s)", ErrUnsupportedFS, mountpoint)
	}

	if err := destroyStaleTracers(c, device, mountpoint); err != nil {
		return nil, "", err
	}

	// Less than 10% free and the module cannot create the copy-on-write file
	// (or it fills up mid-backup): say so now, with the figures. Checked after
	// destroyStaleTracers, whose file it may just have removed.
	if err := checkCowFreeSpace(mountpoint, device, fstype); err != nil {
		return nil, "", err
	}

	minor, err := allocMinor(c)
	if err != nil {
		return nil, "", err
	}

	cowFile := filepath.Join(mountpoint, fmt.Sprintf(".pbs_snapshot_%d.cow", minor))
	os.Remove(cowFile)

	syscall.Sync()

	if err := ioctlSetupSnapshot(c.ctlDevice, c.name, device, cowFile, minor); err != nil {
		os.Remove(cowFile)
		if errors.Is(err, syscall.ENOSPC) {
			err = fmt.Errorf("%w: the snapshot keeps its copy-on-write file on the file system it snapshots (%s), which needs at least %d%% of its size free for it", err, mountpoint, minCowFreePercent)
		}
		return nil, "", err
	}

	snapDev := fmt.Sprintf("%s%d", c.devPrefix, minor)
	ls := &linuxSnapshot{control: c, minor: minor, device: snapDev, cowFile: cowFile}

	if err := waitForDevice(snapDev, 5*time.Second); err != nil {
		return nil, "", errors.Join(err, cleanupOne(ls))
	}

	trackedMu.Lock()
	tracked = append(tracked, ls)
	trackedMu.Unlock()

	subPath := strings.TrimPrefix(absPath, strings.TrimRight(mountpoint, "/"))
	subPath = strings.TrimPrefix(subPath, "/")

	if !needFiles {
		log.Printf("Created %s snapshot of %s (%s) -> %s (raw block device)",
			c.name, device, absPath, snapDev)
		return ls, subPath, nil
	}

	tmpMount, err := os.MkdirTemp("", "pbs-snap-")
	if err != nil {
		return nil, "", errors.Join(err, cleanupOne(ls))
	}
	if err := mountReadOnly(snapDev, tmpMount, fstype); err != nil {
		os.Remove(tmpMount)
		return nil, "", errors.Join(err, cleanupOne(ls))
	}
	ls.mountpoint = tmpMount

	log.Printf("Created %s snapshot of %s (%s) -> %s mounted at %s",
		c.name, device, absPath, snapDev, tmpMount)
	return ls, subPath, nil
}

func waitForDevice(dev string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(dev); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("snapshot device %s did not appear within %s", dev, timeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func mountReadOnly(dev, target, fstype string) error {
	candidates := []string{"", "norecovery"}
	if fstype == "xfs" {
		candidates = []string{"nouuid", "nouuid,norecovery"}
	}
	var lastErr error
	for _, opts := range candidates {
		if err := syscall.Mount(dev, target, fstype, syscall.MS_RDONLY, opts); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("mounting %s (%s) read-only at %s: %v", dev, fstype, target, lastErr)
}

func cleanupOne(ls *linuxSnapshot) error {
	if ls == nil {
		return nil
	}
	if ls.mountpoint != "" {
		if err := syscall.Unmount(ls.mountpoint, 0); err != nil {

			syscall.Unmount(ls.mountpoint, syscall.MNT_DETACH)
		}
		os.Remove(ls.mountpoint)
		ls.mountpoint = ""
	}
	var retErr error
	if err := ioctlDestroySnapshot(ls.control.ctlDevice, ls.control.name, ls.minor); err != nil {
		retErr = fmt.Errorf("failed to destroy %s snapshot %d: %w", ls.control.name, ls.minor, err)
	}
	if ls.cowFile != "" {
		os.Remove(ls.cowFile)
	}

	trackedMu.Lock()
	for i, t := range tracked {
		if t == ls {
			tracked = append(tracked[:i], tracked[i+1:]...)
			break
		}
	}
	trackedMu.Unlock()
	return retErr
}

func CreateVSSSnapshot(paths []string, needFiles bool, backup_callback func(sn map[string]SnapShot) error) (retErr error) {
	return CreateVSSSnapshotCtx(context.Background(), paths, needFiles, backup_callback)
}

// CreateVSSSnapshotCtx is CreateVSSSnapshot for a backup that can be stopped
// while it waits for a device another backup has snapshotted.
func CreateVSSSnapshotCtx(ctx context.Context, paths []string, needFiles bool, backup_callback func(sn map[string]SnapShot) error) (retErr error) {
	// Both checked, so one message names everything that is missing.
	control, haveModule := detectControl()
	isRoot := os.Geteuid() == 0
	const install = "so install and load one: \"Install it for me\" in the app's machine backup screen, or sudo salvanza-cli install-snapshot-module (the GUI manual's section \"Linux snapshots: installing elastio-snap or dattobd\" has more)"
	const refuse = "refusing to back up without a snapshot, as files could change while they are read"
	switch {
	case !isRoot && !haveModule:
		return fmt.Errorf("%s: %w, so run the backup as root; and %w, %s", refuse, ErrNeedsRoot, ErrNoSnapshotModule, install)
	case !isRoot:
		return fmt.Errorf("%s: %w, so run the backup as root", refuse, ErrNeedsRoot)
	case !haveModule:
		return fmt.Errorf("%s: %w, %s", refuse, ErrNoSnapshotModule, install)
	}

	// The block devices these paths are on, held until the snapshots are gone.
	absPaths := make([]string, len(paths))
	var devs []string
	seenDev := map[string]bool{}
	for i, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}
		absPaths[i] = absPath
		if _, device, _, err := findMount(absPath); err == nil && !seenDev[device] {
			seenDev[device] = true
			devs = append(devs, device)
		}
	}
	// Refuse before waiting for the devices' locks or taking any snapshot,
	// naming the volume and how much room is missing. A copy-on-write file
	// already there (a crashed run's) is not counted: createOne removes stale
	// ones first, and checks again just before it creates its own.
	for _, absPath := range absPaths {
		mountpoint, device, fstype, err := findMount(absPath)
		if err != nil || fstype == "btrfs" || hasCowFile(mountpoint) {
			continue // createOne says why, or frees the room first
		}
		if err := checkCowFreeSpace(mountpoint, device, fstype); err != nil {
			return fmt.Errorf("could not snapshot %s: %w: refusing to back up without a snapshot", absPath, err)
		}
	}
	if err := holdDevices(ctx, devs); err != nil {
		return fmt.Errorf("stopped while waiting for another backup's snapshot to end: %w", err)
	}
	// And against other programs (linux_lock.go).
	if err := lockDevicesAcrossPrograms(ctx, devs); err != nil {
		releaseDevices(devs)
		return fmt.Errorf("waiting for another program's snapshot to end: %w", err)
	}

	snapshots := make(map[string]SnapShot, len(paths))
	var created []*linuxSnapshot

	defer func() {
		for i := len(created) - 1; i >= 0; i-- {
			retErr = errors.Join(retErr, cleanupOne(created[i]))
		}
		unlockDevices(devs)
		releaseDevices(devs)
	}()

	for i, path := range paths {
		absPath := absPaths[i]
		setupMu.Lock()
		ls, subPath, err := createOne(control, absPath, needFiles)
		setupMu.Unlock()
		if err != nil {
			return fmt.Errorf("could not snapshot %s: %w: refusing to back up without a snapshot", absPath, err)
		}
		created = append(created, ls)
		sn := SnapShot{
			ObjectPath: ls.device,
			Id:         strconv.Itoa(ls.minor),
			Valid:      true,
			CowFile:    ls.cowFile,
		}
		if needFiles {
			sn.FullPath = filepath.Join(ls.mountpoint, subPath)
		}
		snapshots[path] = sn
	}

	return backup_callback(snapshots)
}

func VSSCleanup() error {
	trackedMu.Lock()
	remaining := make([]*linuxSnapshot, len(tracked))
	copy(remaining, tracked)
	trackedMu.Unlock()

	var errs []error
	for i := len(remaining) - 1; i >= 0; i-- {
		if err := cleanupOne(remaining[i]); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
