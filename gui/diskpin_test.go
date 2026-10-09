package main

import "testing"

func disk(num int64, path, id string) PhysicalDiskInfo {
	return PhysicalDiskInfo{
		DiskNumber:   num,
		DeviceID:     "PhysicalDrive" + itoa(num),
		DevicePath:   path,
		UniqueID:     id,
		Size:         512 * 1024 * 1024 * 1024,
		Model:        "TEST DISK",
		DriveLetters: nil,
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestNormalizePhysicalDrivePath(t *testing.T) {
	ok := map[string]string{
		`\\.\PhysicalDrive0`:   `\\.\PhysicalDrive0`,
		`PhysicalDrive12`:      `\\.\PhysicalDrive12`,
		`\\?\PhysicalDrive3`:   `\\.\PhysicalDrive3`,
		`  \\.\PhysicalDrive7`: `\\.\PhysicalDrive7`,
	}
	for in, want := range ok {
		got, ok := normalizePhysicalDrivePath(in)
		if !ok || got != want {
			t.Errorf("normalizePhysicalDrivePath(%q) = (%q, %v), want (%q, true)", in, got, ok, want)
		}
	}

	bad := []string{
		"", "PhysicalDrive", "PhysicalDriveA", `\\.\PhysicalDrive1x`,
		`\\.\C:`, "/dev/sda", "/dev/disk/by-id/ata-foo",
		"serial:ABC123", "config.json",
	}
	for _, in := range bad {
		if got, ok := normalizePhysicalDrivePath(in); ok {
			t.Errorf("normalizePhysicalDrivePath(%q) = (%q, true), want ok=false", in, got)
		}
	}
}

func TestIsStableDiskID(t *testing.T) {
	stable := []string{
		"serial:S4XNX0A123", "guid:12345678-1234-1234-1234-123456789abc",
		"sig:deadbeef", "wwn:5000c29f1a2b3c4d",
	}
	for _, id := range stable {
		if !isStableDiskID(id) {
			t.Errorf("isStableDiskID(%q) = false, want true", id)
		}
	}
	weak := []string{
		"", "size:512000000000", "mdl:Samsung860:512000000000",
		"serial:", "no-prefix", `\\.\PhysicalDrive0`,
	}
	for _, id := range weak {
		if isStableDiskID(id) {
			t.Errorf("isStableDiskID(%q) = true, want false", id)
		}
	}
	// Every stable id must also be recognised as an id by the resolver.
	for _, id := range stable {
		if !looksLikeDiskID(id) {
			t.Errorf("looksLikeDiskID(%q) = false, want true", id)
		}
	}
}

// A config written before pinning existed must resolve exactly as it did then.
func TestResolvePinnedDevicesLegacyPathsUnchanged(t *testing.T) {
	present := []PhysicalDiskInfo{
		disk(0, `\\.\PhysicalDrive0`, "serial:AAA"),
		disk(1, `\\.\PhysicalDrive1`, "serial:BBB"),
	}
	cases := []struct {
		req  []string
		want []string
	}{
		{[]string{`\\.\PhysicalDrive0`}, []string{`\\.\PhysicalDrive0`}},
		{[]string{`\\.\PhysicalDrive1`, `\\.\PhysicalDrive0`}, []string{`\\.\PhysicalDrive1`, `\\.\PhysicalDrive0`}},
		// The shorthand form is canonicalised, nothing more.
		{[]string{`PhysicalDrive1`}, []string{`\\.\PhysicalDrive1`}},
		// The same disk twice collapses to one device.
		{[]string{`\\.\PhysicalDrive0`, `\\.\PhysicalDrive0`}, []string{`\\.\PhysicalDrive0`}},
	}
	for _, c := range cases {
		got, err := resolvePinnedDevices(c.req, present, nil)
		if err != nil {
			t.Fatalf("resolvePinnedDevices(%v): %v", c.req, err)
		}
		if len(got) != len(c.want) {
			t.Fatalf("resolvePinnedDevices(%v) = %v, want %v", c.req, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("resolvePinnedDevices(%v)[%d] = %q, want %q", c.req, i, got[i], c.want[i])
			}
		}
	}
}

// The whole point: Windows renumbered the disk, the saved path must follow it.
func TestResolvePinnedDevicesFollowsRenumberedDisk(t *testing.T) {
	// Config says PhysicalDrive0; that disk is now PhysicalDrive2.
	pins := map[string]string{"serial:AAA": `\\.\PhysicalDrive0`}
	present := []PhysicalDiskInfo{
		disk(2, `\\.\PhysicalDrive2`, "serial:AAA"), // our disk, moved
		disk(0, `\\.\PhysicalDrive0`, "serial:BBB"), // some other disk took its place
	}
	got, err := resolvePinnedDevices([]string{`\\.\PhysicalDrive0`}, present, pins)
	if err != nil {
		t.Fatalf("resolvePinnedDevices: %v", err)
	}
	if len(got) != 1 || got[0] != `\\.\PhysicalDrive2` {
		t.Errorf("got %v, want [%s]", got, `\\.\PhysicalDrive2`)
	}
}

// A direct id reference resolves too (that is what gets persisted).
func TestResolvePinnedDevicesByUniqueID(t *testing.T) {
	present := []PhysicalDiskInfo{
		disk(2, `\\.\PhysicalDrive2`, "serial:AAA"),
		disk(0, `\\.\PhysicalDrive0`, "serial:BBB"),
	}
	got, err := resolvePinnedDevices([]string{"serial:AAA"}, present, nil)
	if err != nil {
		t.Fatalf("resolvePinnedDevices: %v", err)
	}
	if len(got) != 1 || got[0] != `\\.\PhysicalDrive2` {
		t.Errorf("got %v, want [%s]", got, `\\.\PhysicalDrive2`)
	}

	if _, err := resolvePinnedDevices([]string{"serial:GONE"}, present, nil); err == nil {
		t.Error("resolving a disconnected pinned disk must fail")
	}
}

// A different disk sitting at the pinned path must never be backed up.
func TestResolvePinnedDevicesRefusesWrongDisk(t *testing.T) {
	pins := map[string]string{"serial:AAA": `\\.\PhysicalDrive0`}
	present := []PhysicalDiskInfo{
		disk(0, `\\.\PhysicalDrive0`, "serial:BBB"), // AAA is gone, BBB took its place
	}
	if _, err := resolvePinnedDevices([]string{`\\.\PhysicalDrive0`}, present, pins); err == nil {
		t.Error("must refuse to back up the disk that replaced a pinned one")
	}
	// A listing that produced no stable id at all (unelevated process, odd
	// driver) is not evidence the disk is gone: the request passes through
	// instead of failing, exactly like a build without pinning.
	got, err := resolvePinnedDevices([]string{`\\.\PhysicalDrive0`}, nil, pins)
	if err != nil {
		t.Fatalf("an empty listing must not fail the backup: %v", err)
	}
	if len(got) != 1 || got[0] != `\\.\PhysicalDrive0` {
		t.Errorf("got %v, want the saved path untouched", got)
	}
	// When the listing DID work and the pinned disk simply is not among the
	// disks present, that is a real error: the caller must not proceed.
	present = []PhysicalDiskInfo{
		disk(1, `\\.\PhysicalDrive1`, "serial:BBB"),
	}
	if _, err := resolvePinnedDevices([]string{`\\.\PhysicalDrive0`}, present, pins); err == nil {
		t.Error("must fail when the pinned disk is absent from a working listing")
	}
}

// Without stable ids (unelevated listing, platform we do not identify) the
// request must pass through untouched — never fail a working backup.
func TestResolvePinnedDevicesWithoutStableIDsPassesThrough(t *testing.T) {
	pins := map[string]string{"serial:AAA": `\\.\PhysicalDrive0`}
	present := []PhysicalDiskInfo{
		disk(0, `\\.\PhysicalDrive0`, "size:512000000000"),
	}
	got, err := resolvePinnedDevices([]string{`\\.\PhysicalDrive0`}, present, pins)
	if err != nil {
		t.Fatalf("resolvePinnedDevices: %v", err)
	}
	if len(got) != 1 || got[0] != `\\.\PhysicalDrive0` {
		t.Errorf("got %v, want the saved path untouched", got)
	}

	// Same when the listing came back empty.
	got, err = resolvePinnedDevices([]string{`\\.\PhysicalDrive0`}, nil, pins)
	if err != nil {
		t.Fatalf("resolvePinnedDevices(empty listing): %v", err)
	}
	if len(got) != 1 || got[0] != `\\.\PhysicalDrive0` {
		t.Errorf("got %v, want the saved path untouched", got)
	}
}

// Non-physical-drive references (Linux device nodes, plain files) are never
// rewritten, pins or not.
func TestResolvePinnedDevicesLeavesOtherReferencesAlone(t *testing.T) {
	present := []PhysicalDiskInfo{
		disk(0, `\\.\PhysicalDrive0`, "serial:AAA"),
	}
	req := []string{"/dev/sda", "/home/user/disk.img"}
	got, err := resolvePinnedDevices(req, present, map[string]string{"serial:AAA": `\\.\PhysicalDrive0`})
	if err != nil {
		t.Fatalf("resolvePinnedDevices: %v", err)
	}
	if len(got) != 2 || got[0] != req[0] || got[1] != req[1] {
		t.Errorf("got %v, want %v unchanged", got, req)
	}
}

func TestCollectAndMergeDiskPins(t *testing.T) {
	present := []PhysicalDiskInfo{
		disk(0, `\\.\PhysicalDrive0`, "serial:AAA"),
		disk(1, `\\.\PhysicalDrive1`, "size:1234"), // too weak to pin
		disk(2, `\\.\PhysicalDrive2`, "guid:abc"),
	}
	pins := collectDiskPins([]string{`\\.\PhysicalDrive0`, `\\.\PhysicalDrive1`, `\\.\PhysicalDrive2`}, present)
	if len(pins) != 2 {
		t.Fatalf("got %d pins, want 2 (weak ids excluded): %v", len(pins), pins)
	}
	if pins["serial:AAA"] != `\\.\PhysicalDrive0` || pins["guid:abc"] != `\\.\PhysicalDrive2` {
		t.Errorf("unexpected pins: %v", pins)
	}

	cfg := &Config{}
	if !mergeDiskPins(cfg, pins) {
		t.Error("first merge must report a change")
	}
	if mergeDiskPins(cfg, pins) {
		t.Error("merging the same pins again must not report a change")
	}
	if len(cfg.PinnedDisks) != 2 {
		t.Errorf("stored pins: %v", cfg.PinnedDisks)
	}
	// A pin never created is untouched by a run that did not see it.
	if !mergeDiskPins(cfg, map[string]string{"serial:ZZZ": `\\.\PhysicalDrive9`}) {
		t.Error("adding a new pin must report a change")
	}
	if cfg.PinnedDisks["serial:AAA"] != `\\.\PhysicalDrive0` {
		t.Errorf("existing pin was lost: %v", cfg.PinnedDisks)
	}
	if mergeDiskPins(nil, pins) {
		t.Error("nil config must not report a change")
	}
}
