package snapshot

type SnapShot struct {
	FullPath   string
	Id         string
	ObjectPath string
	Valid      bool
}

// SnapControl describes a Linux block-snapshot control module
// (elastio-snap or dattobd). Only meaningful on Linux; on other platforms
// DetectControl always reports false.
type SnapControl struct {
	Name      string
	DevPrefix string
	Module    string
	CtlDevice string
	InfoFile  string
}
