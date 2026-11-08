//go:build !windows

package clientcommon

import "os"

// enableANSI reports whether the terminal under f takes the escape sequences
// the progress box is drawn with. Every system but Windows does.
func enableANSI(*os.File) bool { return true }
