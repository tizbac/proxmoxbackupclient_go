package main

import (
	"strings"
	"testing"
	"time"
)

func TestSearchHasGlob(t *testing.T) {
	cases := map[string]bool{
		"":            false,
		"plain.txt":   false,
		"Prix*":       true,
		"*":           true,
		"a?b":         true,
		"a*b?c":       true,
		"[abc]":       false,
		"back\\slash": false,
		"  ":          false,
	}
	for q, want := range cases {
		if got := hasGlob(q); got != want {
			t.Errorf("hasGlob(%q) = %v, want %v", q, got, want)
		}
	}
}

func TestSearchCompileGlobAnchorsAndEscapes(t *testing.T) {
	re, err := compileGlob("Prix*")
	if err != nil {
		t.Fatalf("compileGlob: %v", err)
	}
	for _, m := range []string{"prix", "PRIX", "prix.exe", "Prix-old"} {
		if !re.MatchString(m) {
			t.Errorf("glob Prix* did not match %q", m)
		}
	}
	// * is "anything after", so prix2 matches; what must not match is a
	// base name where the literal prefix is absent or longer.
	for _, m := range []string{"", "aprix", "le prix", "pri", "xprix"} {
		if re.MatchString(m) {
			t.Errorf("glob Prix* wrongly matched %q", m)
		}
	}

	// Everything that is not * or ? must match literally, so regexp
	// metacharacters a user types in a filename must not be interpreted.
	re, err = compileGlob("a+b(c)[d].txt")
	if err != nil {
		t.Fatalf("compileGlob: %v", err)
	}
	if !re.MatchString("a+b(c)[d].txt") {
		t.Error("regexp metacharacters were not matched literally")
	}
	if re.MatchString("aab(c)d.txt") {
		t.Error("+ was treated as a regexp operator")
	}

	// ? matches exactly one character.
	re, err = compileGlob("a?c")
	if err != nil {
		t.Fatalf("compileGlob: %v", err)
	}
	if !re.MatchString("abc") || !re.MatchString("aZc") {
		t.Error("? did not match a single character")
	}
	if re.MatchString("ac") || re.MatchString("abbc") {
		t.Error("? matched the wrong number of characters")
	}
}

func TestSearchBuildMatcherName(t *testing.T) {
	m, err := buildMatcher(SearchModeName, "report")
	if err != nil {
		t.Fatalf("buildMatcher: %v", err)
	}
	// Only the base name is considered, so directories are ignored entirely.
	cases := map[string]bool{
		"docs/report.pdf":           true,
		"docs/REPORT.PDF":           true,
		"deep/nested/my_report.doc": true,
		"reports":                   true,
		"":                          false,
		"a/rep/ort.txt":             false, // split across the base name
		"x/reporters/inner_report":  true,
		"docs/nothing.txt":          false,
		"a/report.txt/b.txt":        false, // "b.txt" is the base name
	}
	for path, want := range cases {
		if got := m(path); got != want {
			t.Errorf("name mode %q -> %v, want %v", path, got, want)
		}
	}
}

func TestSearchBuildMatcherNameDoesNotFoldAccents(t *testing.T) {
	// Matching is a plain lowercase+Contains, so "report" must not match
	// "réport" and vice versa. Asserted explicitly because it is the kind of
	// thing a future unicode-aware normalisation would silently change.
	ascii, err := buildMatcher(SearchModeName, "report")
	if err != nil {
		t.Fatalf("buildMatcher: %v", err)
	}
	if ascii("unicode/réport.txt") {
		t.Error(`"report" matched "réport"`)
	}
	accented, err := buildMatcher(SearchModeName, "réport")
	if err != nil {
		t.Fatalf("buildMatcher: %v", err)
	}
	if !accented("unicode/réport.txt") {
		t.Error(`"réport" did not match "réport"`)
	}
	if accented("unicode/report.txt") {
		t.Error(`"réport" matched "report"`)
	}
}

func TestSearchBuildMatcherNameGlob(t *testing.T) {
	m, err := buildMatcher(SearchModeName, "*.txt")
	if err != nil {
		t.Fatalf("buildMatcher: %v", err)
	}
	if !m("dir/notes.txt") {
		t.Error("*.txt did not match dir/notes.txt")
	}
	if m("dir/notes.txt.bak") {
		t.Error("*.txt matched a longer extension")
	}
	if m("dir/sub") {
		t.Error("*.txt matched a directory-less path with no .txt")
	}

	// The glob is anchored to the base name, so a directory component must not
	// be able to satisfy it.
	if m("notes.txt/sibling") {
		t.Error("glob matched against something other than the base name")
	}
}

func TestSearchBuildMatcherPath(t *testing.T) {
	m, err := buildMatcher(SearchModePath, "etc/ssh")
	if err != nil {
		t.Fatalf("buildMatcher: %v", err)
	}
	if !m("etc/ssh/sshd_config") {
		t.Error("path substring did not match")
	}
	if !m("var/ETC/SSH/sshd_config") {
		t.Error("path mode should be case-insensitive")
	}
	if m("elsewhere") {
		t.Error("path substring matched an unrelated path")
	}
	if m("srv/ssh") {
		t.Error("path substring matched a path that does not contain it")
	}
	// Plain substring semantics, not segment-aware: the query is not required to
	// land on a path boundary. Documented behaviour, matched here so a change
	// to segment-aware matching is a deliberate one.
	if !m("etc/sshtest") {
		t.Error("path mode no longer does a plain substring match")
	}
}

func TestSearchBuildMatcherPathGlob(t *testing.T) {
	m, err := buildMatcher(SearchModePath, "etc/*/config")
	if err != nil {
		t.Fatalf("buildMatcher: %v", err)
	}
	if !m("etc/ssh/config") {
		t.Error("path glob did not match")
	}
	if m("etc/ssh/other") {
		t.Error("path glob matched the wrong leaf")
	}
}

func TestSearchBuildMatcherRegex(t *testing.T) {
	m, err := buildMatcher(SearchModeRegex, `^v([0-9]+)\.`)
	if err != nil {
		t.Fatalf("buildMatcher: %v", err)
	}
	if !m("logs/v12.log") {
		t.Error("regex did not match")
	}
	if !m("logs/V12.LOG") {
		t.Error("regex should be case-insensitive by default")
	}
	// Only the base name is tested, so a directory that happens to satisfy the
	// pattern does not make the entry match.
	if m("v12/notes.txt") {
		t.Error("regex matched against the directory instead of the base name")
	}
}

func TestSearchBuildMatcherRejectsBadInput(t *testing.T) {
	if _, err := buildMatcher(SearchModeRegex, "("); err == nil {
		t.Error("expected an error for an invalid regexp")
	} else if !strings.Contains(err.Error(), "expression régulière invalide") {
		t.Errorf("error = %q, want it to mention the regexp", err)
	}
	if _, err := buildMatcher(SearchMatchMode("nope"), "x"); err == nil {
		t.Error("expected an error for an unknown mode")
	} else if !strings.Contains(err.Error(), "mode de recherche inconnu") {
		t.Errorf("error = %q, want it to mention the mode", err)
	}
}

func TestSearchBuildMatcherEmptyModeBehavesAsName(t *testing.T) {
	withDefault, err := buildMatcher("", "notes")
	if err != nil {
		t.Fatalf("buildMatcher: %v", err)
	}
	explicit, err := buildMatcher(SearchModeName, "notes")
	if err != nil {
		t.Fatalf("buildMatcher: %v", err)
	}
	for _, p := range []string{"a/notes.txt", "notes", "x/y/z", ""} {
		if withDefault(p) != explicit(p) {
			t.Errorf("empty mode disagrees with SearchModeName on %q", p)
		}
	}
}

func TestSearchBuildMatcherEmptyQueryMatchesEverything(t *testing.T) {
	// SearchFilesInline rejects a blank query, but buildMatcher itself must not
	// be handed one without panicking.
	for _, mode := range []SearchMatchMode{SearchModeName, SearchModePath, SearchModeRegex, ""} {
		m, err := buildMatcher(mode, "")
		if err != nil {
			t.Fatalf("buildMatcher(%q, \"\"): %v", mode, err)
		}
		if !m("anything/at/all.txt") {
			t.Errorf("mode %q: empty query did not match everything", mode)
		}
	}
}

func TestSearchJoinOriginPath(t *testing.T) {
	unix := &BackupMeta{OS: "linux", OriginalPath: "/home/tiziano/data/"}
	cases := []struct {
		name string
		meta *BackupMeta
		arch string
		want string
	}{
		{"unix nested", unix, "a/b.txt", "/home/tiziano/data/a/b.txt"},
		{"unix no trailing slash", &BackupMeta{OS: "linux", OriginalPath: "/home/tiziano/data"}, "b.txt", "/home/tiziano/data/b.txt"},
		{"unix root archive path", unix, "", "/home/tiziano/data"},
		{"windows meta", &BackupMeta{OS: "windows", OriginalPath: `C:\Users\tiz\data\`}, "a/b.txt", `C:\Users\tiz\data\a\b.txt`},
		{"backslash implies windows", &BackupMeta{OS: "", OriginalPath: `D:\x\`}, "p/q", `D:\x\p\q`},
		{"no meta", nil, "a.txt", ""},
		{"empty original path", &BackupMeta{OS: "linux", OriginalPath: ""}, "a.txt", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := joinOriginPath(tc.meta, tc.arch); got != tc.want {
				t.Errorf("joinOriginPath = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSearchJoinOriginPathDoesNotDoubleSeparate(t *testing.T) {
	meta := &BackupMeta{OS: "linux", OriginalPath: "/srv/backup///"}
	if got, want := joinOriginPath(meta, "x.txt"), "/srv/backup/x.txt"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSearchFmtTime(t *testing.T) {
	if got := fmtTime(time.Time{}); got != "∞" {
		t.Errorf("fmtTime(zero) = %q, want ∞", got)
	}
	at := time.Date(2026, 3, 1, 12, 30, 0, 0, time.UTC)
	if got, want := fmtTime(at), "2026-03-01"; got != want {
		t.Errorf("fmtTime = %q, want %q", got, want)
	}
}

func TestSearchFilesInlineValidatesOptions(t *testing.T) {
	good := SearchOptions{
		BaseURL:   "https://pbs.example.com:8007",
		AuthID:    "root@pam!test",
		Secret:    "s3cret",
		Datastore: "backup",
		Query:     "notes",
	}
	cases := []struct {
		name    string
		mutate  func(o *SearchOptions)
		wantErr string
	}{
		{"no base url", func(o *SearchOptions) { o.BaseURL = "" }, "paramètres de connexion"},
		{"no authid", func(o *SearchOptions) { o.AuthID = "" }, "paramètres de connexion"},
		{"no secret", func(o *SearchOptions) { o.Secret = "" }, "paramètres de connexion"},
		{"no datastore", func(o *SearchOptions) { o.Datastore = "" }, "datastore requis"},
		{"blank query", func(o *SearchOptions) { o.Query = "   " }, "terme de recherche requis"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := good
			tc.mutate(&o)
			res, err := SearchFilesInline(o)
			if err == nil {
				t.Fatalf("expected an error, got result %+v", res)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestSearchFilesInlineAcceptsTicketAuth(t *testing.T) {
	// Ticket-only auth must pass validation; it then fails later on the network
	// call, which proves validation let it through.
	o := SearchOptions{
		BaseURL:   "https://127.0.0.1:1",
		Ticket:    "PVE:root@pam",
		Datastore: "backup",
		Query:     "notes",
	}
	if _, err := SearchFilesInline(o); err == nil {
		t.Fatal("expected the snapshot listing to fail")
	} else if strings.Contains(err.Error(), "paramètres de connexion") {
		t.Errorf("ticket auth was rejected as missing credentials: %v", err)
	}
}

func TestSearchFilesInlineRejectsBadQueryBeforeNetwork(t *testing.T) {
	o := SearchOptions{
		BaseURL:   "https://127.0.0.1:1",
		AuthID:    "root@pam!test",
		Secret:    "s3cret",
		Datastore: "backup",
		Query:     "(",
		Mode:      SearchModeRegex,
	}
	_, err := SearchFilesInline(o)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "liste des snapshots") {
		t.Errorf("an invalid regexp reached the network call: %v", err)
	}
}

func TestSearchCancelFlagIsResettable(t *testing.T) {
	CancelFileSearch()
	if !searchCancelled.Load() {
		t.Fatal("CancelFileSearch did not set the flag")
	}
	// SearchFilesInline clears the flag, but only after validation passes, so
	// drive it through the same entry point the GUI uses.
	_, _ = SearchFilesInline(SearchOptions{
		BaseURL:   "https://127.0.0.1:1",
		AuthID:    "root@pam!test",
		Secret:    "s3cret",
		Datastore: "backup",
		Query:     "notes",
	})
	if searchCancelled.Load() {
		t.Error("SearchFilesInline left the cancel flag set")
	}
}
