package pbscommon

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- listing -------------------------------------------------------------

func TestPXARListEntries(t *testing.T) {
	pr := NewPXARReader(standardPXAR(t))
	entries, err := pr.ListEntries()
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}

	got := make(map[string]PXARTreeEntry, len(entries))
	var order []string
	for _, e := range entries {
		got[e.Path] = e
		order = append(order, e.Path)
	}

	want := []string{
		"hello.txt",
		"empty-dir",
		"sub",
		"sub/other.txt",
		"sub/nested",
		"sub/nested/deep.txt",
		"notes",
		"notes/file with spaces & ünicode.txt",
	}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries, want %d\ngot: %v", len(entries), len(want), order)
	}
	for i, w := range want {
		if order[i] != w {
			t.Errorf("entry %d = %q, want %q (full order: %v)", i, order[i], w, order)
		}
	}

	if e := got["hello.txt"]; e.IsDir || e.Size != uint64(len("hello world\n")) {
		t.Errorf("hello.txt = %+v, want a 12-byte regular file", e)
	}
	if e := got["empty-dir"]; !e.IsDir {
		t.Errorf("empty-dir should be reported as a directory, got %+v", e)
	}
	if e := got["sub/nested/deep.txt"]; e.Mode&0777 != 0644 || e.ModTime != 1600000123 {
		t.Errorf("deep.txt mode/mtime = %#o/%d, want 0644/1600000123", e.Mode, e.ModTime)
	}
}

func TestPXARListEntriesIsRepeatable(t *testing.T) {
	// walk() rewinds internally, so listing twice must not be affected by the
	// reader having advanced.
	pr := NewPXARReader(standardPXAR(t))
	first, err := pr.ListEntries()
	if err != nil {
		t.Fatalf("first ListEntries: %v", err)
	}
	second, err := pr.ListEntries()
	if err != nil {
		t.Fatalf("second ListEntries: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("listing is not repeatable: %d then %d entries", len(first), len(second))
	}
	for i := range first {
		if first[i].Path != second[i].Path {
			t.Fatalf("entry %d differs between runs: %q vs %q", i, first[i].Path, second[i].Path)
		}
	}
}

func TestPXAREmptyArchive(t *testing.T) {
	// Root ENTRY + GOODBYE, nothing inside.
	empty := makePXAR(t)
	entries, err := NewPXARReader(empty).ListEntries()
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries from an empty archive, want 0", len(entries))
	}
}

func TestPXARReaderAtMatchesInMemory(t *testing.T) {
	data := standardPXAR(t)
	fromMem, err := NewPXARReader(data).ListEntries()
	if err != nil {
		t.Fatalf("NewPXARReader: %v", err)
	}
	fromFile, err := NewPXARReaderAt(bytes.NewReader(data), int64(len(data))).ListEntries()
	if err != nil {
		t.Fatalf("NewPXARReaderAt: %v", err)
	}
	if len(fromMem) != len(fromFile) {
		t.Fatalf("entry count differs: %d vs %d", len(fromMem), len(fromFile))
	}
	for i := range fromMem {
		if fromMem[i].Path != fromFile[i].Path || fromMem[i].Size != fromFile[i].Size {
			t.Errorf("entry %d differs: %+v vs %+v", i, fromMem[i], fromFile[i])
		}
	}
}

// --- walk() hardening ----------------------------------------------------

// The sizes in a PXAR header come straight from archive bytes, i.e. from
// whatever is stored on the server. walk() runs without a recover() in the
// listing/search paths, so a hostile or corrupt size must produce an error
// rather than a panic or a hang.
func TestPXARWalkRejectsHostileSizes(t *testing.T) {
	good := pxarEntryPayload(t, IFREG|0644, 1600000000)

	cases := []struct {
		name string
		data []byte
	}{
		{
			"header size below the 16-byte header",
			pxarRawEntry(t, PXAR_FILENAME, 8, nil),
		},
		{
			"size far beyond the archive",
			pxarRawEntry(t, PXAR_FILENAME, 1<<62, nil),
		},
		{
			"size overflowing int64 once 16 is subtracted",
			pxarRawEntry(t, PXAR_PAYLOAD, 1<<63, nil),
		},
		{
			"payload larger than the remaining bytes",
			append(pxarSection(t, PXAR_ENTRY, good),
				pxarRawEntry(t, PXAR_PAYLOAD, 1<<40, nil)...),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("walk panicked: %v", r)
				}
			}()
			if _, err := NewPXARReader(tc.data).ListEntries(); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

func TestPXARWalkSkipsUnknownRecordTypes(t *testing.T) {
	// Symlinks, xattrs, ACLs and hardlinks all have their own PXAR record
	// types. They must be stepped over, not treated as an error or mistaken
	// for a file.
	data := makePXAR(t, pxarFile("a.txt", []byte("A\n")))
	var buf []byte
	buf = append(buf, pxarSection(t, 0xdeadbeefdeadbeef, make([]byte, 12))...)
	buf = append(buf, data...)

	entries, err := NewPXARReader(buf).ListEntries()
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "a.txt" {
		t.Errorf("unknown record types must be skipped transparently, got %+v", entries)
	}
}

func TestPXARWalkPropagatesCallbackError(t *testing.T) {
	sentinel := errors.New("stop right here")
	err := NewPXARReader(standardPXAR(t)).walk(func(PXARTreeEntry, *io.SectionReader) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("callback error = %v, want %v", err, sentinel)
	}
}

// --- ReadVirtualFile -----------------------------------------------------

func TestPXARReadVirtualFile(t *testing.T) {
	pr := NewPXARReader(standardPXAR(t))
	data, err := pr.ReadVirtualFile("hello.txt")
	if err != nil {
		t.Fatalf("ReadVirtualFile: %v", err)
	}
	if string(data) != "hello world\n" {
		t.Errorf("content = %q, want %q", data, "hello world\n")
	}
}

func TestPXARReadVirtualFileNotFound(t *testing.T) {
	_, err := NewPXARReader(standardPXAR(t)).ReadVirtualFile("nope.txt")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want os.ErrNotExist", err)
	}
}

// Only archive-root files are addressable this way: sub-directory entries have
// a slash in their path and must not be matched by their basename.
func TestPXARReadVirtualFileIgnoresNestedPaths(t *testing.T) {
	pr := NewPXARReader(standardPXAR(t))
	if _, err := pr.ReadVirtualFile("deep.txt"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("nested file should not be reachable by basename, got %v", err)
	}
}

// --- path helpers --------------------------------------------------------

func TestPXARNormalizeIncludes(t *testing.T) {
	cases := []struct {
		in, want []string
	}{
		{nil, []string{}},
		{[]string{}, []string{}},
		{[]string{"a"}, []string{"a"}},
		{[]string{"/a/b/"}, []string{"a/b"}},
		{[]string{`\a\b`}, []string{"a/b"}},
		{[]string{""}, []string{}},
		{[]string{"/"}, []string{}},
		{[]string{"a", "  ", "b/"}, []string{"a", "  ", "b"}}, // only slashes are trimmed
		{[]string{"keep/case/ünïcode"}, []string{"keep/case/ünïcode"}},
	}
	for _, tc := range cases {
		got := NormalizeIncludes(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("NormalizeIncludes(%v) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("NormalizeIncludes(%v)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestPXARIsUnsafeArchivePath(t *testing.T) {
	unsafe := []string{
		"/etc/passwd",
		"/",
		"../escape",
		"a/../../escape",
		"a/b/..",
		`\windows\system32`,
		`\\server\share`,
		`C:/windows`,
		"c:relative",
	}
	for _, p := range unsafe {
		if !isUnsafeArchivePath(p) {
			t.Errorf("isUnsafeArchivePath(%q) = false, want true", p)
		}
	}
	safe := []string{
		"",
		"a",
		"a/b",
		"a.b/c.d",
		"..hidden",
		"a..b",
		"dir with spaces/f.txt",
	}
	for _, p := range safe {
		if isUnsafeArchivePath(p) {
			t.Errorf("isUnsafeArchivePath(%q) = true, want false", p)
		}
	}
	// ".." alone is a single segment equal to "..", so it must be unsafe; the
	// intent of the case above is only that *names* starting with dots are fine.
	if !isUnsafeArchivePath("..") {
		t.Error(`isUnsafeArchivePath("..") = false, want true`)
	}
}

func TestPXARPathMatches(t *testing.T) {
	cases := []struct {
		path     string
		includes []string
		want     bool
	}{
		{"a/b.txt", nil, true},
		{"a/b.txt", []string{}, true},
		{"a/b.txt", []string{"a"}, true},         // ancestor selected
		{"a/b.txt", []string{"a/b.txt"}, true},   // exact
		{"a/b.txt", []string{"a/b.txt/c"}, true}, // descendant of selection
		{"a/b.txt", []string{"other"}, false},
		{"a/b.txt", []string{"ab"}, false}, // must not match on a bare prefix
		{"a/b.txt", []string{"a/c"}, false},
		{"", []string{"a"}, false},
	}
	for _, tc := range cases {
		if got := pathMatches(tc.path, tc.includes); got != tc.want {
			t.Errorf("pathMatches(%q, %v) = %v, want %v", tc.path, tc.includes, got, tc.want)
		}
	}
}

// --- extraction ----------------------------------------------------------

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return string(b)
}

func TestPXARExtractAll(t *testing.T) {
	dest := t.TempDir()
	got, err := NewPXARReader(standardPXAR(t)).ExtractAll(dest)
	if err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}
	if len(got) != 8 {
		t.Errorf("extracted %d entries, want 8", len(got))
	}

	if s := readFile(t, filepath.Join(dest, "hello.txt")); s != "hello world\n" {
		t.Errorf("hello.txt = %q", s)
	}
	if s := readFile(t, filepath.Join(dest, "sub", "other.txt")); s != "other\n" {
		t.Errorf("sub/other.txt = %q", s)
	}
	if s := readFile(t, filepath.Join(dest, "sub", "nested", "deep.txt")); s != "deep\n" {
		t.Errorf("sub/nested/deep.txt = %q", s)
	}
	if s := readFile(t, filepath.Join(dest, "notes", "file with spaces & ünicode.txt")); s != "unicode\n" {
		t.Errorf("unicode file content = %q", s)
	}

	// Empty directories must survive the round trip, unlike symlinks.
	for _, d := range []string{"empty-dir", "sub", "sub/nested", "notes"} {
		info, err := os.Stat(filepath.Join(dest, filepath.FromSlash(d)))
		if err != nil {
			t.Errorf("directory %s missing: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", d)
		}
	}

	for _, e := range got {
		if e.Skipped {
			t.Errorf("unexpected skip of %s: %s", e.Path, e.SkipReason)
		}
	}
}

func TestPXARExtractAllPreservesModTime(t *testing.T) {
	dest := t.TempDir()
	if _, err := NewPXARReader(standardPXAR(t)).ExtractAll(dest); err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}
	info, err := os.Stat(filepath.Join(dest, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(time.Unix(1600000123, 0)) {
		t.Errorf("mtime = %v, want %v", info.ModTime(), time.Unix(1600000123, 0))
	}
}

func TestPXARExtractFilteredSelectsSubtree(t *testing.T) {
	dest := t.TempDir()
	got, err := NewPXARReader(standardPXAR(t)).ExtractFiltered(dest, []string{"/sub/"}, false)
	if err != nil {
		t.Fatalf("ExtractFiltered: %v", err)
	}

	if s := readFile(t, filepath.Join(dest, "sub", "other.txt")); s != "other\n" {
		t.Errorf("sub/other.txt = %q", s)
	}
	if s := readFile(t, filepath.Join(dest, "sub", "nested", "deep.txt")); s != "deep\n" {
		t.Errorf("sub/nested/deep.txt = %q", s)
	}
	if _, err := os.Stat(filepath.Join(dest, "hello.txt")); !os.IsNotExist(err) {
		t.Errorf("hello.txt should not have been extracted")
	}
	if _, err := os.Stat(filepath.Join(dest, "notes")); !os.IsNotExist(err) {
		t.Errorf("notes/ should not have been extracted")
	}
	for _, e := range got {
		if e.Skipped {
			t.Errorf("unexpected skip of %s: %s", e.Path, e.SkipReason)
		}
	}
}

func TestPXARExtractWithoutOverwriteKeepsExistingFile(t *testing.T) {
	dest := t.TempDir()
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "hello.txt"), []byte("PRE-EXISTING"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := NewPXARReader(standardPXAR(t)).ExtractFiltered(dest, []string{"hello.txt"}, false)
	if err != nil {
		t.Fatalf("ExtractFiltered: %v", err)
	}
	if s := readFile(t, filepath.Join(dest, "hello.txt")); s != "PRE-EXISTING" {
		t.Errorf("existing file was clobbered: %q", s)
	}
	var sawSkip bool
	for _, e := range got {
		if e.Skipped && e.Expected {
			sawSkip = true
			if e.SkipReason != "already exists" {
				t.Errorf("SkipReason = %q, want %q", e.SkipReason, "already exists")
			}
		}
	}
	if !sawSkip {
		t.Error("expected an Expected=true skip for the pre-existing file")
	}
}

func TestPXARExtractWithOverwriteReplaces(t *testing.T) {
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "hello.txt"), []byte("PRE-EXISTING"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPXARReader(standardPXAR(t)).ExtractFiltered(dest, []string{"hello.txt"}, true); err != nil {
		t.Fatalf("ExtractFiltered: %v", err)
	}
	if s := readFile(t, filepath.Join(dest, "hello.txt")); s != "hello world\n" {
		t.Errorf("content = %q, want the archive content", s)
	}
}

// A hostile archive must not be able to write outside the destination, whatever
// the rewriter does: the entry path itself is validated first.
func TestPXARExtractRefusesPathTraversal(t *testing.T) {
	base := t.TempDir()
	dest := filepath.Join(base, "dest")

	data := makePXAR(t,
		pxarFile("../pwned.txt", []byte("escaped\n")),
		pxarFile("a/../../also-pwned.txt", []byte("escaped\n")),
		pxarFile(`C:\\windows\\evil.txt`, []byte("escaped\n")),
		// Absolute names, emitted verbatim: the walker must still see them.
		pxarNamedFile(t, "/abs-pwned.txt", []byte("escaped\n")),
		pxarFile("safe.txt", []byte("ok\n")),
	)

	got, err := NewPXARReader(data).ExtractAll(dest)
	if err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}

	// Nothing at all may land outside dest.
	if err := filepath.Walk(base, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(dest, p)
		if rerr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Errorf("traversal escaped the destination: %s", p)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if s := readFile(t, filepath.Join(dest, "safe.txt")); s != "ok\n" {
		t.Errorf("safe.txt = %q, want the archive content", s)
	}

	// Every hostile entry must be reported as refused rather than silently
	// dropped, so the user can see what the archive tried to do.
	refused := map[string]bool{}
	for _, e := range got {
		if e.Skipped && strings.Contains(e.SkipReason, "unsafe archive path") {
			refused[e.Path] = true
		}
	}
	for _, want := range []string{
		"../pwned.txt",
		"a/../../also-pwned.txt",
		"/abs-pwned.txt",
		`C:\\windows\\evil.txt`,
		// Intermediate directory records of the traversal are hostile too.
		"..",
		"a/..",
		"a/../..",
	} {
		if !refused[want] {
			t.Errorf("entry %q was not refused; refused set: %v", want, refused)
		}
	}
}

func TestPXARExtractWithRewriter(t *testing.T) {
	dest := t.TempDir()
	// Rewrite every entry into a single subdirectory, the way a restore into
	// an alternate destination root would.
	rewriter := func(p string) string { return filepath.Join(dest, "restored", filepath.FromSlash(p)) }

	got, err := NewPXARReader(standardPXAR(t)).ExtractWithRewriter(rewriter, nil, false)
	if err != nil {
		t.Fatalf("ExtractWithRewriter: %v", err)
	}
	if s := readFile(t, filepath.Join(dest, "restored", "sub", "nested", "deep.txt")); s != "deep\n" {
		t.Errorf("rewritten path content = %q", s)
	}
	// ArchivePath must stay relative to the archive root so callers can match
	// restored files against archive-side metadata such as ACL side-cars.
	for _, e := range got {
		if e.Skipped {
			continue
		}
		if filepath.IsAbs(e.ArchivePath) {
			t.Errorf("ArchivePath %q must be archive-relative", e.ArchivePath)
		}
		if !strings.HasPrefix(e.Path, filepath.Join(dest, "restored")) {
			t.Errorf("Path %q was not rewritten", e.Path)
		}
	}
}

// An empty rewriter result means "drop this entry" (e.g. the flat-selection
// ancestor case) — not a skip and not an error.
func TestPXARExtractRewriterCanDropEntries(t *testing.T) {
	dest := t.TempDir()
	got, err := NewPXARReader(standardPXAR(t)).ExtractWithRewriter(
		func(p string) string {
			if strings.HasPrefix(p, "notes/") {
				return ""
			}
			return filepath.Join(dest, filepath.FromSlash(p))
		}, nil, false)
	if err != nil {
		t.Fatalf("ExtractWithRewriter: %v", err)
	}
	for _, e := range got {
		if e.Skipped {
			t.Errorf("dropping an entry must not be reported as a skip: %+v", e)
		}
	}
	// The dropped child is gone; the parent directory record ("notes", which does
	// not match the "notes/" prefix) is legitimately still created.
	if _, err := os.Stat(filepath.Join(dest, "notes", "file with spaces & ünicode.txt")); !os.IsNotExist(err) {
		t.Error("dropped entry should not have been written")
	}
	if s := readFile(t, filepath.Join(dest, "hello.txt")); s != "hello world\n" {
		t.Errorf("hello.txt = %q", s)
	}
}

func TestPXARExtractWithRewriterRequiresRewriter(t *testing.T) {
	if _, err := NewPXARReader(standardPXAR(t)).ExtractWithRewriter(nil, nil, false); err == nil {
		t.Fatal("expected an error for a nil rewriter")
	}
}

func TestPXARExtractLeavesNoTempFiles(t *testing.T) {
	// Content is written to a sibling temp file and atomically renamed, so a
	// clean run must not leave anything behind.
	dest := t.TempDir()
	if _, err := NewPXARReader(standardPXAR(t)).ExtractAll(dest); err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}
	var leftovers []string
	filepath.Walk(dest, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.Contains(info.Name(), ".tmp") || strings.HasPrefix(info.Name(), ".") {
			leftovers = append(leftovers, p)
		}
		return nil
	})
	if len(leftovers) > 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
}

func TestPXARExtractEmptyFile(t *testing.T) {
	dest := t.TempDir()
	if _, err := NewPXARReader(makePXAR(t, pxarFile("zero.txt", nil))).ExtractAll(dest); err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}
	p := filepath.Join(dest, "zero.txt")
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("zero-byte file was not created: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("size = %d, want 0", info.Size())
	}
}
