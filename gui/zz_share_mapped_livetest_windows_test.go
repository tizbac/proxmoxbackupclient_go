//go:build windows && !service

package main

import (
	"os"
	"strings"
	"testing"
)

// TestLiveResolveMappedDrive: a folder on a drive letter mapped to a share is
// kept as the share's own path. Run in the session that mapped the letter:
//
//	net use Z: \\server\share /user:...
//	set PBS_MAPPED_DRIVE=Z:
//	set PBS_MAPPED_SHARE=\\server\share
func TestLiveResolveMappedDrive(t *testing.T) {
	drive, share := os.Getenv("PBS_MAPPED_DRIVE"), os.Getenv("PBS_MAPPED_SHARE")
	if drive == "" || share == "" {
		t.Skip("set PBS_MAPPED_DRIVE and PBS_MAPPED_SHARE in a session with the drive mapped")
	}
	for in, want := range map[string]string{
		drive + `\Test Files`:         share + `\Test Files`,
		drive + `\`:                   share,
		drive:                         share,
		strings.ToLower(drive) + `\x`: share + `\x`,
	} {
		if got := resolveMappedDrive(in); !strings.EqualFold(got, want) {
			t.Errorf("resolveMappedDrive(%q) = %q, want %q", in, got, want)
		}
	}
	if got := resolveMappedDrive(`C:\Windows`); got != `C:\Windows` {
		t.Errorf("a local drive changed: %q", got)
	}
	got := resolveMappedDrives([]string{`C:\Windows`, drive + `\Test Files`})
	t.Logf("set folders kept as %q", got)
	if !strings.EqualFold(got[1], share+`\Test Files`) {
		t.Errorf("resolveMappedDrives = %q", got)
	}
}
