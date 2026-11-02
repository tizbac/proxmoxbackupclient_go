//go:build !windows

package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)

// handoffNoFollow makes the elevated copy's open fail on a symbolic link.
const handoffNoFollow = syscall.O_NOFOLLOW

// checkOpenedHandoff checks the handoff file the elevated copy opened: the
// same file it inspected (before), a regular file with one link, and owned
// by the user who asked (pkexec says who in PKEXEC_UID), so the elevated
// write cannot land in a file swapped in or hard-linked from elsewhere.
func checkOpenedHandoff(f *os.File, before os.FileInfo) error {
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() || !os.SameFile(fi, before) {
		return fmt.Errorf("%s is not the file that was checked", f.Name())
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if st.Nlink != 1 {
		return fmt.Errorf("%s has %d links", f.Name(), st.Nlink)
	}
	if v := os.Getenv("PKEXEC_UID"); v != "" {
		if uid, err := strconv.Atoi(v); err == nil && int(st.Uid) != uid {
			return fmt.Errorf("%s belongs to uid %d, not to the user who asked (%d)", f.Name(), st.Uid, uid)
		}
	}
	return nil
}
