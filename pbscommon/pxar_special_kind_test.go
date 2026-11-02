package pbscommon

import (
	"os"
	"testing"
)

// Only FIFOs, sockets and device nodes are skipped. ModeIrregular is not:
// on Windows it marks any reparse point that is not a link (OneDrive
// placeholders among them), and those open and read normally.
func TestSpecialFileKind(t *testing.T) {
	for _, c := range []struct {
		mode os.FileMode
		want string
	}{
		{os.ModeNamedPipe, "Pipe (FIFO)"},
		{os.ModeSocket, "Socket"},
		{os.ModeDevice, "Device"},
		{os.ModeDevice | os.ModeCharDevice, "Device"},
		{0, ""},
		{os.ModeIrregular, ""},
		{os.ModeDir, ""},
		{os.ModeSymlink, ""},
	} {
		if got := specialFileKind(c.mode); got != c.want {
			t.Errorf("specialFileKind(%v) = %q, want %q", c.mode, got, c.want)
		}
	}
}
