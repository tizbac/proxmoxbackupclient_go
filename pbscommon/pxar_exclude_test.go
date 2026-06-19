package pbscommon

import "testing"

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
			if got := isExcluded(c.rel, c.base, root, c.patterns); got != c.want {
				t.Errorf("isExcluded(%q, %q, %q, %v) = %v, want %v",
					c.rel, c.base, root, c.patterns, got, c.want)
			}
		})
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
	}
	for _, c := range cases {
		if got := relExcludePath(c.root, c.full); got != c.want {
			t.Errorf("relExcludePath(%q, %q) = %q, want %q", c.root, c.full, got, c.want)
		}
	}
}
