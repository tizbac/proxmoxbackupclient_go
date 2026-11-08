//go:build linux

package snapshot

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// CleanupOnSignal makes Ctrl+C, SIGTERM and SIGHUP remove this process's
// snapshots before it exits. A command-line backup stopped that way used to
// leave its snapshot set up and mounted (/tmp/pbs-snap-*), and every later
// snapshot of that file system then failed as "busy" until someone unmounted
// it by hand (found 2026-11-05). The command-line tools call it; the app and
// its service have their own handling. SIGKILL cannot be caught: what it
// leaves is removed by the next backup of the file system
// (destroyStaleTracers, unmountLeftovers).
func CleanupOnSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		s := <-ch
		fmt.Fprintf(os.Stderr, "\nStopped (%v): removing the backup's snapshots\n", s)
		// The backup still has files open on the snapshot (or the
		// snapshot device itself, for a disk): while it does, the
		// snapshot can only be detached, not destroyed. It is stopping
		// anyway, so its files are closed first.
		closeFilesOnSnapshots()
		if err := VSSCleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
		code := 1
		if sig, ok := s.(syscall.Signal); ok {
			code = 128 + int(sig)
		}
		os.Exit(code)
	}()
}

// closeFilesOnSnapshots closes this process's open files that are on one of
// its snapshots, or are a snapshot device.
func closeFilesOnSnapshots() {
	trackedMu.Lock()
	var mounts, devices []string
	for _, ls := range tracked {
		if ls.mountpoint != "" {
			mounts = append(mounts, ls.mountpoint)
		}
		devices = append(devices, ls.device)
	}
	trackedMu.Unlock()
	if len(mounts)+len(devices) == 0 {
		return
	}
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return
	}
	for _, e := range entries {
		fd, err := strconv.Atoi(e.Name())
		if err != nil || fd <= 2 {
			continue
		}
		target, err := os.Readlink(filepath.Join("/proc/self/fd", e.Name()))
		if err != nil {
			continue
		}
		if onSnapshot(target, mounts, devices) {
			_ = syscall.Close(fd)
		}
	}
}

// onSnapshot reports whether path is inside one of mounts or is one of
// devices.
func onSnapshot(path string, mounts, devices []string) bool {
	for _, d := range devices {
		if path == d {
			return true
		}
	}
	for _, m := range mounts {
		if path == m || strings.HasPrefix(path, strings.TrimRight(m, "/")+"/") {
			return true
		}
	}
	return false
}
