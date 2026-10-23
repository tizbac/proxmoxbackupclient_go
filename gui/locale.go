package main

import "strings"

// GetRegionalLocale returns the computer's own format for dates and numbers
// as a BCP 47 tag ("en-GB"), or "" if it is not known. The frontend shows
// dates in it while the app's language is that same language. The web view
// only knows the display language, which on a British PC is often en-US
// (US dates) although Windows itself shows dates the British way.
func (a *App) GetRegionalLocale() string {
	return regionalLocale()
}

// posixLocaleTag turns a POSIX locale name ("en_GB.UTF-8", "de_DE@euro") into
// a BCP 47 tag ("en-GB"). "C", "POSIX" and "" give "".
func posixLocaleTag(s string) string {
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	if s == "" || s == "C" || s == "POSIX" {
		return ""
	}
	return strings.ReplaceAll(s, "_", "-")
}
