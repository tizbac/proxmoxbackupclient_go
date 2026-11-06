package snapshot

import "strings"

// isNetworkPath reports a UNC path to a share (\\server\share\..., or the long
// form \\?\UNC\server\share\...), which is never on a local volume. Device and
// long local paths (\\.\..., \\?\C:\...) are not. (Plain string work, tested
// on every platform.)
func isNetworkPath(path string) bool {
	p := strings.ReplaceAll(path, "/", `\`)
	switch {
	case strings.HasPrefix(strings.ToUpper(p), `\\?\UNC\`):
		return true
	case strings.HasPrefix(p, `\\?\`), strings.HasPrefix(p, `\\.\`):
		return false
	default:
		return strings.HasPrefix(p, `\\`)
	}
}

// volumeRoot extracts the volume portion GetDriveType expects: "C:\" for a
// drive-letter path, or "\\server\share\" for a UNC path. Returns "" if path
// doesn't look like either. (Used on Windows; plain string work, so it is
// tested on every platform.)
func volumeRoot(path string) string {
	if len(path) >= 2 && path[1] == ':' {
		return path[:2] + `\`
	}
	if strings.HasPrefix(path, `\\`) {
		rest := path[2:]
		// First two path segments after the leading \\ are server\share;
		// GetDriveType wants exactly \\server\share\, nothing deeper.
		parts := strings.SplitN(rest, `\`, 3)
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			return `\\` + parts[0] + `\` + parts[1] + `\`
		}
	}
	return ""
}
