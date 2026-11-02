//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// profilesDirectory is where Windows keeps the user profiles (normally
// C:\Users). A variable so tests can point it elsewhere.
var profilesDirectory = func() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList`, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		if v, _, err := k.GetStringValue("ProfilesDirectory"); err == nil {
			if exp, err := registry.ExpandString(v); err == nil && exp != "" {
				return exp
			}
		}
	}
	return os.Getenv("SystemDrive") + `\Users`
}

// userHomeOf returns the profile folder dir is in (C:\Users\<name>), if it is
// in a user's own profile. The shared profiles (Public, Default, All Users)
// are not anyone's own.
func userHomeOf(dir string) (string, bool) {
	profiles := filepath.Clean(profilesDirectory())
	d := filepath.Clean(dir)
	if !pathWithin(d, profiles) || samePath(d, profiles) {
		return "", false
	}
	rel, err := filepath.Rel(profiles, d)
	if err != nil {
		return "", false
	}
	name := strings.Split(rel, string(filepath.Separator))[0]
	switch strings.ToLower(name) {
	case "", ".", "..", "public", "default", "default user", "all users":
		return "", false
	}
	return filepath.Join(profiles, name), true
}

// secureSafetyFolder locks the safety folder down before anything is moved
// into it: SYSTEM, the Administrators group and the account making it get
// full control, nobody else gets in, and nothing is inherited from the folder
// above (at the root of C:, every signed-in user may change what is created
// there). Files and folders made inside later get the same. Hidden, as it is
// not the user's own folder.
func secureSafetyFolder(path string) error {
	tok := windows.GetCurrentProcessToken()
	u, err := tok.GetTokenUser()
	if err != nil {
		return fmt.Errorf("securing %s: %w", path, err)
	}
	me := u.User.Sid.String()
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;" + me + ")")
	if err != nil {
		return fmt.Errorf("securing %s: %w", path, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("securing %s: %w", path, err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		return fmt.Errorf("securing %s: %w", path, err)
	}
	if p, err := syscall.UTF16PtrFromString(path); err == nil {
		if attrs, err := windows.GetFileAttributes(p); err == nil {
			_ = windows.SetFileAttributes(p, attrs|windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_NOT_CONTENT_INDEXED)
		}
	}
	return nil
}
