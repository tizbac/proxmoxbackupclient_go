package pbscommon

import (
	"strings"
	"testing"
)

// TestIsExcluded covers the user-exclusion matcher (H-04): separatorless globs
// match the basename anywhere; patterns with a separator (incl. absolute ones,
// relativized against the backup root) are ANCHORED and never match a nested
// same-named entry. Matching is case-insensitive and separator-agnostic.
func TestIsExcluded(t *testing.T) {
	const root = `C:\Users\Alice`

	cases := []struct {
		name     string
		rel      string
		base     string
		patterns []string
		want     bool
	}{
		{"basename glob match", "docs/x.tmp", "x.tmp", []string{"*.tmp"}, true},
		{"basename glob nomatch", "docs/x.log", "x.log", []string{"*.tmp"}, false},
		{"basename literal anywhere", "a/b/node_modules", "node_modules", []string{"node_modules"}, true},
		{"basename case-insensitive", "x/Y.TMP", "Y.TMP", []string{"*.tmp"}, true},
		{"absolute anchored direct child", "Temp", "Temp", []string{`C:\Users\Alice\Temp`}, true},
		{"absolute anchored subtree", "Temp/cache/f", "f", []string{`C:\Users\Alice\Temp`}, true},
		{"absolute anchored NOT basename-anywhere", "projects/Temp", "Temp", []string{`C:\Users\Alice\Temp`}, false},
		{"path glob match", "logs/a.tmp", "a.tmp", []string{"logs/*.tmp"}, true},
		{"path glob does not cross separator", "logs/sub/a.tmp", "a.tmp", []string{"logs/*.tmp"}, false},
		{"empty patterns never match", "anything", "anything", nil, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isExcluded(c.rel, c.base, root, c.patterns, false); got != c.want {
				t.Errorf("isExcluded(%q, %q, %q, %v) = %v, want %v",
					c.rel, c.base, root, c.patterns, got, c.want)
			}
		})
	}
}

// A backup of / with or without a snapshot: absolute and relative patterns
// with a separator match, where they used to match nothing.
func TestIsExcludedRootOfFileSystem(t *testing.T) {
	for _, c := range []struct {
		walkRoot, entry, pattern string
		want                     bool
	}{
		// Without a snapshot, walked from / itself.
		{"/", "/var/cache", "/var/cache", true},
		{"/", "/var/log/a.log", "var/log/*.log", true},
		// Through a snapshot of /, walked from its mount.
		{"/tmp/.snap/1", "/tmp/.snap/1/var/cache", "/var/cache", true},
		{"/tmp/.snap/1", "/tmp/.snap/1/srv/var/cache", "/var/cache", false},
	} {
		rel := relExcludePath(c.walkRoot, c.entry)
		base := c.entry[strings.LastIndex(c.entry, "/")+1:]
		if got := isExcluded(rel, base, "/", []string{c.pattern}, false); got != c.want {
			t.Errorf("walked from %s: %s with %q = %v, want %v", c.walkRoot, c.entry, c.pattern, got, c.want)
		}
	}
}

// The example of issue #139: leave everything out except .pst files, still
// looking into every folder (written as the official client's .pxarexclude has
// it, and with ** for any depth).
func TestExcludeNegationIssue139(t *testing.T) {
	const root = "/data"
	for _, patterns := range [][]string{
		{"*", "!**.pst", "!**/"},
		{"*", "!*.pst", "!**/"},
		{"*", `!**\`, "!*.pst"}, // as the GUI manual shows it
	} {
		for _, c := range []struct {
			rel   string
			isDir bool
			want  bool
		}{
			{"mail.pst", false, false},
			{"Outlook/Archive/old.PST", false, false},
			{"Outlook", true, false},
			{"Outlook/Archive", true, false},
			{"notes.txt", false, true},
			{"Outlook/Archive/old.ost", false, true},
		} {
			base := c.rel[strings.LastIndex(c.rel, "/")+1:]
			if got := isExcluded(c.rel, base, root, patterns, c.isDir); got != c.want {
				t.Errorf("%q with %q (dir %v) = %v, want %v", c.rel, patterns, c.isDir, got, c.want)
			}
		}
	}
}

// The last pattern that matches decides; ! brings an entry back; without !
// any match still leaves the entry out, as before.
func TestExcludeLastMatchWins(t *testing.T) {
	const root = `C:\Data`
	for _, c := range []struct {
		name     string
		rel      string
		isDir    bool
		patterns []string
		want     bool
	}{
		{"exclude then include", "a/keep.log", false, []string{"*.log", "!keep.log"}, false},
		{"include then exclude", "a/keep.log", false, []string{"!keep.log", "*.log"}, true},
		{"include only: nothing to bring back", "a/x.log", false, []string{"!*.log"}, false},
		{"no ! patterns: any match, as before", "a/x.tmp", false, []string{"*.log", "*.tmp", "node_modules"}, true},
		{"anchored include brings a subfolder back", "logs/keep", true, []string{"logs/*", "!logs/keep"}, false},
		{"its siblings stay out", "logs/old", true, []string{"logs/*", "!logs/keep"}, true},
		{"absolute include", "temp/keep", true, []string{`C:\Data\Temp`, `!C:\Data\Temp\keep`}, false},
		{"blank and lone ! are skipped", "x.tmp", false, []string{"", "!", "*.tmp"}, true},
	} {
		base := c.rel[strings.LastIndex(c.rel, "/")+1:]
		if got := isExcluded(c.rel, base, root, c.patterns, c.isDir); got != c.want {
			t.Errorf("%s: %q with %q = %v, want %v", c.name, c.rel, c.patterns, got, c.want)
		}
	}
}

// A pattern ending in / matches folders only, and stays anchored as it was.
func TestExcludeFoldersOnly(t *testing.T) {
	const root = "/data"
	for _, c := range []struct {
		rel     string
		isDir   bool
		pattern string
		want    bool
	}{
		{"cache", true, "cache/", true},
		{"cache", false, "cache/", false},  // a file called cache stays
		{"x/cache", true, "cache/", false}, // anchored, as "cache/" always was
		{"build", true, "*/", true},
		{"build.log", false, "*/", false},
		{"a/b", true, "**/", true},
		{"a/b/c.txt", false, "**/", false},
		{"cache", true, `cache\`, true},
	} {
		base := c.rel[strings.LastIndex(c.rel, "/")+1:]
		if got := isExcluded(c.rel, base, root, []string{c.pattern}, c.isDir); got != c.want {
			t.Errorf("%q (dir %v) with %q = %v, want %v", c.rel, c.isDir, c.pattern, got, c.want)
		}
	}
}

// ** as a whole segment of a path pattern matches any number of folders.
func TestExcludeDoubleStar(t *testing.T) {
	const root = "/data"
	for _, c := range []struct {
		rel, pattern string
		want         bool
	}{
		{"cache", "**/cache", true},
		{"a/b/cache", "**/cache", true},
		{"a/b/cache2", "**/cache", false},
		{"logs/a.tmp", "logs/**/*.tmp", true},
		{"logs/x/y/a.tmp", "logs/**/*.tmp", true},
		{"other/x/a.tmp", "logs/**/*.tmp", false},
		{"logs/sub/a.tmp", "logs/*.tmp", false}, // a single * still does not cross a folder
		{"a/b/c", "a/**", true},
		{"a", "a/**", true},
		{"x.pst", "**.pst", true}, // within a name, ** is *
	} {
		base := c.rel[strings.LastIndex(c.rel, "/")+1:]
		if got := isExcluded(c.rel, base, root, []string{c.pattern}, false); got != c.want {
			t.Errorf("%q with %q = %v, want %v", c.rel, c.pattern, got, c.want)
		}
	}
}

// TestRelExcludePath checks that paths are reduced relative to the backup root so
// absolute patterns and VSS-walked entries compare on the same (root-relative) basis.
func TestRelExcludePath(t *testing.T) {
	cases := []struct {
		root, full, want string
	}{
		{`C:\Users\Alice`, `C:\Users\Alice\Temp`, "temp"},
		{`C:\Users\Alice`, `C:\Users\Alice`, ""},
		{`C:\Users\Alice`, `D:\Other`, "d:/other"}, // not under root: returned normalized
		{`\\?\GLOBALROOT\Device\Shadow1`, `\\?\GLOBALROOT\Device\Shadow1\Temp`, "temp"},
		// A backup of / (a whole Linux system): every path is under the root.
		{"/", "/var/cache", "var/cache"},
		{"/", "/", ""},
		{"/", "var/log/*.log", "var/log/*.log"}, // a relative pattern stays as it is
		{`C:\`, `C:\Users`, "users"},
		{"", "/var/cache", "/var/cache"}, // no root given: unchanged
	}
	for _, c := range cases {
		if got := relExcludePath(c.root, c.full); got != c.want {
			t.Errorf("relExcludePath(%q, %q) = %q, want %q", c.root, c.full, got, c.want)
		}
	}
}
