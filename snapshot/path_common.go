package snapshot

import "strings"

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
