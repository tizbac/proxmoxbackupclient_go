//go:build windows
// +build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// listRoots returns the tree-picker's root nodes on Windows: every logical
// drive letter that's actually present (GetLogicalDrives, not a fixed A-Z
// scan — avoids the multi-second timeout Windows can impose probing an empty
// floppy/optical drive letter), plus any mapped network drive GetLogicalDrives
// missed (see below).
func listRoots() ([]DirEntry, error) {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, fmt.Errorf("GetLogicalDrives failed: %w", err)
	}

	seen := make(map[string]bool, 26)
	roots := make([]DirEntry, 0, 8)
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		letter := string(rune('A' + i))
		path := letter + `:\`
		roots = append(roots, DirEntry{
			Name:  letter + ":",
			Path:  path,
			IsDir: true,
		})
		seen[letter] = true
	}

	// GetLogicalDrives only reports drives visible in THIS process's logon
	// session. A drive mapped in the normal interactive desktop session is
	// invisible here when the app runs elevated ("Run as Administrator"),
	// because UAC elevation creates a separate logon session with its own
	// network-redirector connection table — found live 2026-09-26 (Mick:
	// "the tree picker only lists local drives ... if i map a network
	// share it doesn't present as available"). The mapping itself is still
	// recorded per-user in the registry regardless of session, so fall back
	// to that for any letter GetLogicalDrives missed, pointing straight at
	// the UNC target rather than the (session-invisible) drive letter:
	// opening \\server\share directly establishes a fresh SMB connection
	// using the user's cached credentials, which works independent of the
	// drive-letter redirector table that's actually broken here.
	roots = appendRegistryMappedDrives(roots, seen)

	return roots, nil
}

// appendRegistryMappedDrives reads HKCU\Network\<Letter>\RemotePath for every
// drive letter not already in seen, appending one root entry per mapping
// found. Best-effort: any registry error just means fewer fallback entries,
// never a failure for the whole roots list — the app already worked before
// this fallback existed.
func appendRegistryMappedDrives(roots []DirEntry, seen map[string]bool) []DirEntry {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Network`, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return roots
	}
	defer k.Close()

	names, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return roots
	}

	for _, letter := range names {
		if seen[letter] {
			continue
		}
		sub, err := registry.OpenKey(k, letter, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		remotePath, _, err := sub.GetStringValue("RemotePath")
		sub.Close()
		if err != nil || remotePath == "" {
			continue
		}
		roots = append(roots, DirEntry{
			Name:  fmt.Sprintf("%s: (%s)", letter, remotePath),
			Path:  remotePath,
			IsDir: true,
		})
	}
	return roots
}
