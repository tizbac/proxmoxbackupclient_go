package pbscommon

import "fmt"

// Change-detection modes, named after the official proxmox-backup-client's
// --change-detection-mode values:
//
//   - legacy (the default, also ""): one self-contained .pxar.didx archive
//     plus the catalog. Every file is read on every run.
//   - data: a split archive (.mpxar.didx metadata stream + .ppxar.didx payload
//     stream). Every file is still read on every run.
//   - metadata: the split archive, reusing unchanged files' content from the
//     previous snapshot without reading them. Until that reuse is implemented
//     this behaves exactly like data.
const (
	ChangeDetectionLegacy   = "legacy"
	ChangeDetectionData     = "data"
	ChangeDetectionMetadata = "metadata"
)

// ValidateChangeDetectionMode accepts "", legacy, data and metadata.
func ValidateChangeDetectionMode(mode string) error {
	switch mode {
	case "", ChangeDetectionLegacy, ChangeDetectionData, ChangeDetectionMetadata:
		return nil
	}
	return fmt.Errorf("unknown change-detection mode %q (use legacy, data or metadata)", mode)
}

// SplitArchiveMode reports whether mode writes a split (format version 2) archive.
func SplitArchiveMode(mode string) bool {
	return mode == ChangeDetectionData || mode == ChangeDetectionMetadata
}
