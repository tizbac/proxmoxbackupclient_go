package main

import (
	"fmt"
	"strings"
)

// Disk identity prefixes. Windows hands out \\.\PhysicalDriveN names whose N is
// assigned in enumeration order, so it shifts when a disk is added/removed or a
// driver is reinstalled — a saved selection can silently start pointing at a
// DIFFERENT physical disk. A UniqueID built from what the disk itself carries
// (its serial number, its GPT disk GUID, its MBR disk signature) does not move,
// and is what the config pins.
const (
	diskIDPrefixSerial    = "serial:" // vendor serial — survives everything but a firmware flash
	diskIDPrefixWWN       = "wwn:"    // SCSI/ATA world wide name
	diskIDPrefixGUID      = "guid:"   // GPT disk GUID
	diskIDPrefixSig       = "sig:"    // MBR disk signature (4 bytes, hex)
	diskIDPrefixSize      = "size:"   // NOT stable: only a last-resort hint
	diskIDPrefixModelSize = "mdl:"    // NOT stable: model+size, two equal disks collide
)

// stableDiskIDPrefixes identify a specific PHYSICAL disk across reboots; only
// these are ever written to Config.PinnedDisks. The weak ones are kept in
// PhysicalDiskInfo.UniqueID for display/diagnostics but never trusted to
// resolve a backup target.
var stableDiskIDPrefixes = []string{
	diskIDPrefixSerial,
	diskIDPrefixWWN,
	diskIDPrefixGUID,
	diskIDPrefixSig,
}

// allDiskIDPrefixes recognises any id this package may have produced, weak or
// not (a weak id stored as a request is still answered, just without a pin).
var allDiskIDPrefixes = []string{
	diskIDPrefixSerial,
	diskIDPrefixWWN,
	diskIDPrefixGUID,
	diskIDPrefixSig,
	diskIDPrefixSize,
	diskIDPrefixModelSize,
}

// isStableDiskID reports whether id can be used as a pin: it must come from a
// property the disk itself carries, not from how the machine happens to number
// it right now.
func isStableDiskID(id string) bool {
	for _, p := range stableDiskIDPrefixes {
		if strings.HasPrefix(id, p) && len(id) > len(p) {
			return true
		}
	}
	return false
}

// looksLikeDiskID reports whether ref is one of our own unique ids rather than
// a device path.
func looksLikeDiskID(ref string) bool {
	for _, p := range allDiskIDPrefixes {
		if strings.HasPrefix(ref, p) {
			return true
		}
	}
	return false
}

// normalizePhysicalDrivePath canonicalises a physical-drive reference to the
// form machinebackuplib opens ("\\.\PhysicalDrive3"). Accepts the shorthand
// "PhysicalDrive3", the \\?\ prefix and any case, so entries written by older
// builds — or by hand — keep working. Returns ok=false for anything that is not
// a physical drive reference (Linux device nodes, plain file paths, ids).
func normalizePhysicalDrivePath(ref string) (string, bool) {
	s := strings.TrimSpace(ref)
	for _, prefix := range []string{`\\.\`, `\\?\`, `//./`, `//?/`} {
		if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
			s = s[len(prefix):]
			break
		}
	}
	const marker = "PhysicalDrive"
	if len(s) <= len(marker) || !strings.EqualFold(s[:len(marker)], marker) {
		return "", false
	}
	num := s[len(marker):]
	for _, r := range num {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return `\\.\PhysicalDrive` + num, true
}

// resolvePinnedDevices maps a saved disk selection onto the devices that are
// actually present, so a config keeps backing up the same PHYSICAL disk.
//
// Accepted reference shapes, all of which must keep working:
//
//   - "\\.\PhysicalDrive3" / "PhysicalDrive3" — what older configs store. If
//     that path was pinned to a disk and the disk moved, the disk's CURRENT
//     path is returned; if there is no pin the path is returned unchanged, so
//     configs written before pinning existed behave exactly as before.
//   - a unique id ("serial:...", "guid:...", "sig:...") — resolved directly.
//   - anything else (Linux /dev nodes, file paths) — passed through untouched.
//
// It returns an error only when the selection names a disk that is NOT the one
// the caller asked for (pinned disk missing while another disk sits at its old
// path): backing up the wrong disk is worse than failing loudly.
//
// present may be empty or partial (an unelevated process cannot open disks on
// Windows): with no stable ids to work with nothing is resolved and the
// request is returned unchanged.
func resolvePinnedDevices(requested []string, present []PhysicalDiskInfo, pins map[string]string) ([]string, error) {
	if len(requested) == 0 {
		return requested, nil
	}

	byID := make(map[string]string, len(present))   // unique id -> current path
	byPath := make(map[string]string, len(present)) // normalized path -> unique id
	stable := 0
	for _, d := range present {
		if !isStableDiskID(d.UniqueID) {
			continue
		}
		stable++
		if _, dup := byID[d.UniqueID]; !dup {
			byID[d.UniqueID] = d.DevicePath
		}
		if p, ok := normalizePhysicalDrivePath(d.DevicePath); ok {
			byPath[p] = d.UniqueID
		}
	}
	if stable == 0 {
		// No reliable identity available (listing failed, not elevated, or a
		// platform we do not identify disks on): behave exactly like a build
		// without pinning rather than failing a working backup.
		return append([]string(nil), requested...), nil
	}

	pinByPath := make(map[string]string, len(pins))
	for id, path := range pins {
		if !isStableDiskID(id) {
			continue
		}
		if p, ok := normalizePhysicalDrivePath(path); ok {
			pinByPath[p] = id
		}
	}

	out := make([]string, 0, len(requested))
	seen := make(map[string]bool, len(requested))
	for _, ref := range requested {
		resolved, err := resolvePinnedDevice(ref, byID, byPath, pinByPath)
		if err != nil {
			return nil, err
		}
		// Two saved paths can resolve to one disk (the same disk seen at two
		// numbers over time); handing it to PBS twice would be a hard error.
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		out = append(out, resolved)
	}
	return out, nil
}

// resolvePinnedDevice resolves a single saved reference. byID maps a stable
// unique id to the path it currently has, byPath the reverse, pinByPath maps a
// saved path to the unique id it was pinned to.
func resolvePinnedDevice(ref string, byID, byPath, pinByPath map[string]string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("empty disk selection")
	}

	if looksLikeDiskID(ref) {
		if path, ok := byID[ref]; ok {
			return path, nil
		}
		return "", fmt.Errorf("pinned disk %q is not connected", ref)
	}

	path, isDrive := normalizePhysicalDrivePath(ref)
	if !isDrive {
		return ref, nil // not a physical drive reference: leave it alone
	}

	pinnedID, pinned := pinByPath[path]
	if !pinned {
		return path, nil // legacy entry, never pinned: keep the saved path
	}
	if current, ok := byID[pinnedID]; ok {
		// The disk we pinned before — wherever Windows put it this boot.
		return current, nil
	}
	if holder, ok := byPath[path]; ok && holder != pinnedID {
		return "", fmt.Errorf("disk %s was pinned to %s, but %s now holds %s — refusing to back up a different disk",
			pinnedID, path, path, holder)
	}
	return "", fmt.Errorf("pinned disk %q (saved as %s) is not connected", pinnedID, path)
}

// collectDiskPins returns the id→path pins implied by this run: every resolved
// device that carries a stable unique id, mapped to the path it has right now.
// Existing entries are kept (an id is only overwritten when the same disk moved
// to a new number).
func collectDiskPins(resolved []string, present []PhysicalDiskInfo) map[string]string {
	byPath := make(map[string]PhysicalDiskInfo, len(present))
	for _, d := range present {
		if p, ok := normalizePhysicalDrivePath(d.DevicePath); ok {
			byPath[p] = d
		}
	}
	out := make(map[string]string)
	for _, ref := range resolved {
		p, ok := normalizePhysicalDrivePath(ref)
		if !ok {
			continue
		}
		if d, ok := byPath[p]; ok && isStableDiskID(d.UniqueID) {
			out[d.UniqueID] = p
		}
	}
	return out
}

// mergeDiskPins folds newPins into cfg's pin table, reporting whether anything
// changed (so callers only rewrite config.json when it matters). A nil map is
// allocated on first use.
func mergeDiskPins(cfg *Config, newPins map[string]string) bool {
	if cfg == nil || len(newPins) == 0 {
		return false
	}
	if cfg.PinnedDisks == nil {
		cfg.PinnedDisks = make(map[string]string, len(newPins))
	}
	changed := false
	for id, path := range newPins {
		if cur, ok := cfg.PinnedDisks[id]; !ok || cur != path {
			cfg.PinnedDisks[id] = path
			changed = true
		}
	}
	return changed
}
