package main

import (
	"runtime"
	"testing"
)

func TestPosixLocaleTag(t *testing.T) {
	for in, want := range map[string]string{
		"en_GB.UTF-8": "en-GB",
		"en_US.utf8":  "en-US",
		"de_DE@euro":  "de-DE",
		"fr_CA":       "fr-CA",
		"de":          "de",
		"C":           "",
		"C.UTF-8":     "",
		"POSIX":       "",
		"":            "",
	} {
		if got := posixLocaleTag(in); got != want {
			t.Errorf("posixLocaleTag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRegionalLocale(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Every Windows user has a regional format.
		if got := regionalLocale(); len(got) < 2 {
			t.Fatalf("regionalLocale() = %q, want a locale name such as en-GB", got)
		}
		return
	}
	for _, c := range []struct{ all, time, lang, want string }{
		{"", "", "en_GB.UTF-8", "en-GB"},
		{"", "en_GB.UTF-8", "en_US.UTF-8", "en-GB"},
		{"de_DE.UTF-8", "en_GB.UTF-8", "en_US.UTF-8", "de-DE"},
		{"", "", "", ""},
	} {
		t.Setenv("LC_ALL", c.all)
		t.Setenv("LC_TIME", c.time)
		t.Setenv("LANG", c.lang)
		if got := regionalLocale(); got != c.want {
			t.Errorf("LC_ALL=%q LC_TIME=%q LANG=%q: regionalLocale() = %q, want %q", c.all, c.time, c.lang, got, c.want)
		}
	}
}
