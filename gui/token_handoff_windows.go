//go:build windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// handoffNoFollow: Windows has no open flag for it; checkOpenedHandoff
// compares the opened file with the one inspected instead.
const handoffNoFollow = 0

// checkOpenedHandoff checks the handoff file the elevated copy opened: the
// same file it inspected (before; a reparse point opens its target, which is
// another file), a regular file, with one link, so the elevated write cannot
// land in a file swapped in or hard-linked from elsewhere.
func checkOpenedHandoff(f *os.File, before os.FileInfo) error {
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() || !os.SameFile(fi, before) {
		return fmt.Errorf("%s is not the file that was checked", f.Name())
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info); err != nil {
		return err
	}
	if info.NumberOfLinks != 1 {
		return fmt.Errorf("%s has %d links", f.Name(), info.NumberOfLinks)
	}
	return nil
}
