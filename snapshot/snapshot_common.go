package snapshot

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
