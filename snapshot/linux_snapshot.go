//go:build linux
// +build linux

package snapshot

import (
	"bufio"
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
// trusted, so every failure is propagated to the caller.
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

	if err := destroyStaleTracers(c, device, mountpoint); err != nil {
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
	if os.Geteuid() != 0 {
		return fmt.Errorf("consistent snapshot requires root: refusing to proceed with an inconsistent full-system snapshot")
	}
	control, ok := detectControl()
	if !ok {
		return fmt.Errorf("no usable block snapshot module loaded: install and load elastio-snap or dattobd first (see \"Linux machine backup prerequisites\" in this project's README for build/install steps) - refusing to proceed with an inconsistent full-system snapshot")
	}

	snapshots := make(map[string]SnapShot, len(paths))
	var created []*linuxSnapshot

	defer func() {
		for i := len(created) - 1; i >= 0; i-- {
			retErr = errors.Join(retErr, cleanupOne(created[i]))
		}
	}()

	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}
		ls, subPath, err := createOne(control, absPath, needFiles)
		if err != nil {
			return fmt.Errorf("could not snapshot %s: %w: refusing to proceed with an inconsistent full-system snapshot", absPath, err)
		}
		created = append(created, ls)
		sn := SnapShot{
			ObjectPath: ls.device,
			Id:         strconv.Itoa(ls.minor),
			Valid:      true,
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
