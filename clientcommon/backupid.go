package clientcommon

import (
	"fmt"
	"path/filepath"
	"strings"
)

// GenerateBackupID builds a PBS backup-id for one source directory from a base
// (the hostname or a user-chosen backup-id) and the directory path, e.g.
// ("SERVER01", `D:\DATA\Users`) -> "SERVER01_D_DATA_Users".
// The result always matches PBS's ^[A-Za-z0-9_][A-Za-z0-9._\-]*$. This is the
// same id the GUI gives a folder in a multi-folder backup.
func GenerateBackupID(base, path string) string {
	cleanPath := filepath.Clean(path)
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "_")
	cleanPath = strings.ReplaceAll(cleanPath, "/", "_")
	cleanPath = strings.ReplaceAll(cleanPath, ":", "")
	cleanPath = strings.ReplaceAll(cleanPath, " ", "-")

	var sanitized []byte
	for _, c := range []byte(cleanPath) {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '.' || c == '-' {
			sanitized = append(sanitized, c)
		}
	}
	cleanPath = strings.Trim(string(sanitized), "_")

	if cleanPath == "" {
		return base
	}
	return fmt.Sprintf("%s_%s", base, cleanPath)
}
