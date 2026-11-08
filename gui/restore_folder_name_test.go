package main

import "testing"

// The name a backed-up folder shows under in the restore tree, and its
// folder in an alternate-path restore, is the same on every system: a Windows
// backup browsed on Linux shows "Backup of Compaq 486 Laptop", not
// "C:\TestData\Backup of Compaq 486 Laptop" (whose restore matched nothing).
func TestOriginalFolderName(t *testing.T) {
	cases := map[string]string{
		`C:\TestData\Backup of Compaq 486 Laptop`: "Backup of Compaq 486 Laptop",
		`C:\Users\me\Docs\`:                       "Docs",
		`\\server\share\Projects`:                 "Projects",
		`\\server\share`:                          "share",
		"/root/snaplive/data":                     "data",
		"/home/me/Photos/":                        "Photos",
		"relative/dir":                            "dir",
		"Docs":                                    "Docs",
		// Roots and nothing: the archive's own name is used instead.
		`C:\`: "",
		"C:":  "",
		"/":   "",
		"":    "",
		".":   "",
	}
	for in, want := range cases {
		if got := originalFolderName(in); got != want {
			t.Errorf("originalFolderName(%q) = %q, want %q", in, got, want)
		}
	}
}
