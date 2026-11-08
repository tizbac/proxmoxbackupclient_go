package pbscommon

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestIsVolumeRoot(t *testing.T) {
	for path, want := range map[string]bool{
		`C:\`: true, `C:`: true, `c:/`: true, `/`: true,
		`\\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy12\`: true,
		`\\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy12`:  true,
		`\\server\share`: true, `\\server\share\`: true,
		`C:\Data`: false, `/home`: false, `\\server\share\dir`: false,
		`\\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy12\Users`: false,
		``: false,
	} {
		if got := IsVolumeRoot(path); got != want {
			t.Errorf("IsVolumeRoot(%q) = %v, want %v", path, got, want)
		}
	}
}

// "Recovery" is left out of a backup only at the root of a volume (Windows'
// C:\Recovery); a folder of that name anywhere else is the user's and is
// backed up. The tree stands in for a volume root through ExcludeRoot, the
// folder as the user named it.
func TestRecoveryExcludedOnlyAtVolumeRoot(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"Recovery/WindowsRE/winre.wim", "Data/Recovery/photo.jpg", "Data/keep.txt"} {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(p), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write := func(logical string) []byte {
		var m bytes.Buffer
		a := &PXARArchive{ArchiveName: "test.pxar.didx", ExcludeRoot: logical}
		a.WriteCB = func(b []byte) error { m.Write(b); return nil }
		if _, err := a.WriteDir(root, "", true); err != nil {
			t.Fatal(err)
		}
		return m.Bytes()
	}
	has := func(b []byte, s string) bool { return bytes.Contains(b, []byte(s)) }

	asVolume := write(`C:\`)
	if has(asVolume, "winre.wim") {
		t.Error("Recovery at the volume root was backed up")
	}
	if !has(asVolume, "photo.jpg") {
		t.Error("Data/Recovery (not at the root) was left out")
	}
	asFolder := write(`D:\Projects`)
	if !has(asFolder, "winre.wim") || !has(asFolder, "photo.jpg") {
		t.Error("a folder backup that is not a volume root left out a Recovery folder")
	}
}
