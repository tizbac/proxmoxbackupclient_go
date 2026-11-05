//go:build linux

package snapshot

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Backups in different programs (two command-line runs, or one beside the
// app's service) see nothing of each other's holdDevices. Setting up a
// snapshot first clears any tracer of ours left on the device
// (destroyStaleTracers), so the second of two runs on one file system took the
// first one's live snapshot for a stale one and tried to destroy it (refused
// as busy, and then the second run failed; found 2026-11-05). So a backup
// also takes a lock file per device, /run/salvanza-snap-<device>.lock, for as
// long as its snapshots last, and waits while another program holds it. The
// lock goes with the process (flock), so a snapshot found on a device whose
// lock this backup holds was left by a run that has ended: it is stale.

// lockDir is where the device lock files are kept. A variable so tests can
// use a folder of their own.
var lockDir = "/run"

// devLocks are the lock files this process holds, by device. Backups of
// different devices take theirs side by side, hence devLocksMu.
var (
	devLocksMu sync.Mutex
	devLocks   = map[string]*os.File{}
)

// lockFileFor is the lock file of a device: /dev/sda3 is
// /run/salvanza-snap-dev-sda3.lock.
func lockFileFor(dev string) string {
	name := strings.NewReplacer("/", "-", " ", "_").Replace(strings.TrimPrefix(dev, "/"))
	return filepath.Join(lockDir, "salvanza-snap-"+name+".lock")
}

// tryLockDevices takes the lock files of every device in devs, or none: if
// another program holds one, it lets go of those it took and names that
// device.
func tryLockDevices(devs []string) (busy string, err error) {
	var taken []string
	for _, d := range devs {
		f, err := os.OpenFile(lockFileFor(d), os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			unlockDevices(taken)
			return "", fmt.Errorf("lock file for %s: %w", d, err)
		}
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			f.Close()
			unlockDevices(taken)
			if err == syscall.EWOULDBLOCK {
				return d, nil
			}
			return "", fmt.Errorf("locking %s: %w", lockFileFor(d), err)
		}
		// Who holds it, for anyone looking.
		_ = f.Truncate(0)
		_, _ = f.WriteAt([]byte(fmt.Sprintf("%d\n", os.Getpid())), 0)
		devLocksMu.Lock()
		devLocks[d] = f
		devLocksMu.Unlock()
		taken = append(taken, d)
	}
	return "", nil
}

// lockDevicesAcrossPrograms waits until no other program holds any of devs'
// lock files, then holds them all, checking again every lockPoll. A
// cancelled ctx ends the wait. The caller holds devs in this process
// (holdDevices), so no other backup here takes the same lock files meanwhile.
func lockDevicesAcrossPrograms(ctx context.Context, devs []string) error {
	logged := false
	for {
		busy, err := tryLockDevices(devs)
		if err != nil || busy == "" {
			return err
		}
		if !logged {
			log.Printf("Waiting for the snapshot of %s: another program is backing it up", busy)
			logged = true
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(lockPoll):
		}
	}
}

// lockPoll is how often a backup waiting for another program's snapshot
// looks again.
var lockPoll = time.Second

// unlockDevices lets go of the lock files of devs. The files stay: removing
// one another program has just opened would let two programs hold "the" lock.
func unlockDevices(devs []string) {
	devLocksMu.Lock()
	defer devLocksMu.Unlock()
	for _, d := range devs {
		if f := devLocks[d]; f != nil {
			_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			f.Close()
			delete(devLocks, d)
		}
	}
}

// unmountLeftovers unmounts what a run that has ended left mounted from the
// snapshot device snapDev (a run killed with SIGKILL, or one older than the
// signal handling): while it stays mounted the snapshot cannot be destroyed,
// and every later snapshot of the file system failed as "busy". A mount
// folder of ours (pbs-snap-* in the temporary folder) is removed too.
func unmountLeftovers(snapDev string) {
	for _, mp := range mountsOf(snapDev) {
		if err := syscall.Unmount(mp, 0); err != nil {
			if err := syscall.Unmount(mp, syscall.MNT_DETACH); err != nil {
				log.Printf("Could not unmount %s, left mounted from %s by an earlier run: %v", mp, snapDev, err)
				continue
			}
		}
		log.Printf("Unmounted %s, left mounted from %s by an earlier run", mp, snapDev)
		if strings.HasPrefix(filepath.Base(mp), "pbs-snap-") && filepath.Dir(mp) == filepath.Clean(os.TempDir()) {
			os.Remove(mp)
		}
	}
}

// mountsOf lists where dev is mounted, from /proc/self/mountinfo.
func mountsOf(dev string) []string {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil
	}
	defer f.Close()
	return parseMountsOf(bufio.NewScanner(f), dev)
}

func parseMountsOf(sc *bufio.Scanner, dev string) []string {
	var out []string
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		sep := -1
		for i, fld := range fields {
			if fld == "-" {
				sep = i
				break
			}
		}
		if len(fields) < 5 || sep == -1 || sep+2 >= len(fields) {
			continue
		}
		if unescapeMountinfo(fields[sep+2]) == dev {
			out = append(out, unescapeMountinfo(fields[4]))
		}
	}
	return out
}
