//go:build !windows

package pbscommon

import (
	"fmt"
	"os/exec"
	"strings"
)

// ShutdownComputer turns the computer off after a backup that asks for it:
// shutdown -h now (Windows: shutdown_windows.go). comment is for the Windows
// event log and is not used here.
func ShutdownComputer(comment string) (method string, err error) {
	if out, cerr := exec.Command("shutdown", "-h", "now").CombinedOutput(); cerr != nil {
		return "", fmt.Errorf("shutdown -h now: %v: %s", cerr, strings.TrimSpace(string(out)))
	}
	return "shutdown -h now", nil
}
