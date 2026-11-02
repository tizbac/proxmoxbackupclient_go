//go:build !windows

package main

import "os"

// regionalLocale is the locale for dates, in the POSIX order: LC_ALL, then
// LC_TIME, then LANG.
func regionalLocale() string {
	for _, v := range []string{"LC_ALL", "LC_TIME", "LANG"} {
		if s := os.Getenv(v); s != "" {
			return posixLocaleTag(s)
		}
	}
	return ""
}
