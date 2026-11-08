//go:build windows

package clientcommon

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableANSI turns on the console's virtual-terminal processing, which the
// progress box's escape sequences need. A console that does not take it (an
// old one, or a redirected handle reported as a character device) gets the
// plain one-line summary instead.
func enableANSI(f *os.File) bool {
	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return false
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true
	}
	return windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}
