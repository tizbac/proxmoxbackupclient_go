package snapshot

import "errors"

// Why a Linux snapshot could not be taken, for the caller to say what to do
// (errors.Is). Both can apply at once.
var (
	// ErrNeedsRoot: running as a normal user; a snapshot needs root.
	ErrNeedsRoot = errors.New("this is running as a normal user, and a snapshot needs root")
	// ErrNoSnapshotModule: neither elastio-snap nor dattobd is loaded. Run
	// as a normal user, a module that is installed but not loaded counts.
	ErrNoSnapshotModule = errors.New("no snapshot module (elastio-snap or dattobd) is loaded")
	// ErrUnsupportedFS: the path is on a file system the modules cannot
	// snapshot (btrfs, Fedora's default).
	ErrUnsupportedFS = errors.New("elastio-snap and dattobd cannot snapshot this file system")
	// ErrNotEnoughSpace: the file system has less than 10% of its size free,
	// too little for the snapshot's copy-on-write file, which lives on it.
	ErrNotEnoughSpace = errors.New("not enough free space on the file system for the snapshot's copy-on-write file")
)

type SnapShot struct {
	FullPath   string
	Id         string
	ObjectPath string
	Valid      bool
	// CowFile (Linux block snapshots) is the copy-on-write store the
	// snapshot keeps on the snapshotted file system itself; a machine
	// backup leaves its blocks out of the image.
	CowFile string
}

// SnapControl describes the Linux block snapshot module (elastio-snap or
// dattobd) a machine backup would use. DetectControl fills it in; on other
// platforms it is always empty. Exported for the GUI, which shows the user
// which module was found, or a warning when there is none.
type SnapControl struct {
	Name      string // "elastio-snap" or "dattobd"
	DevPrefix string // snapshot devices are DevPrefix + minor, e.g. /dev/elastio-snap0
	Module    string // kernel module name, as given to modprobe
	CtlDevice string // control device the ioctls go to
	InfoFile  string // /proc file that lists the module's snapshots
}
