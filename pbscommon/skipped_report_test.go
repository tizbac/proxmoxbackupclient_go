package pbscommon

import (
	"fmt"
	"strings"
	"testing"
)

// Issue #149: a Windows system drive. The backup meets hundreds of junction
// points and system files before the first file it cannot read, so the first
// 50 of everything named none of the 31,789 unreadable ones.
func TestSkippedReportNamesEveryUnreadableFile(t *testing.T) {
	var skipped, unreadable []string
	for i := 0; i < 400; i++ {
		skipped = append(skipped, fmt.Sprintf(`Junction point (skipped): C:\Users\u\link%d`, i))
	}
	skipped = append(skipped, `System file (auto-excluded): C:\pagefile.sys`)
	for i := 0; i < 300; i++ {
		m := fmt.Sprintf(`Cannot open file: C:\Data\f%d.db (Error: open C:\Data\f%d.db: Access is denied.)`, i, i)
		if i%100 == 0 {
			m = fmt.Sprintf(`Cannot open file: C:\Data\f%d.db (Error: open C:\Data\f%d.db: The process cannot access the file because it is being used by another process.)`, i, i)
		}
		skipped = append(skipped, m)
		unreadable = append(unreadable, m)
	}

	lines := SkippedReport(skipped, unreadable)
	all := strings.Join(lines, "\n")
	if lines[0] != "=== COULD NOT BE READ (300) ===" {
		t.Errorf("the unreadable files come first: got %q", lines[0])
	}
	for _, m := range unreadable {
		if !strings.Contains(all, m) {
			t.Fatalf("not named in the log: %s", m)
		}
	}
	for _, want := range []string{
		"       297  Cannot open file: Access is denied.",
		"         3  Cannot open file: The process cannot access the file because it is being used by another process.",
		"=== END COULD NOT BE READ ===",
		"=== LEFT OUT BY DESIGN (401) ===",
		"       400  Junction point (skipped)",
		"         1  System file (auto-excluded)",
		`  [50] Junction point (skipped): C:\Users\u\link49`,
		"  ... and 351 more, not listed here (of the kinds counted above)",
		"=== END LEFT OUT BY DESIGN ===",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, firstLines(lines, 12))
		}
	}
	// The most frequent reason first.
	if !strings.Contains(lines[2], "297") {
		t.Errorf("most frequent reason first: got %q", lines[2])
	}
	if strings.Contains(all, "link50") {
		t.Error("only the first 50 items left out by design are named")
	}
}

func TestSkippedReportOnlyOneKind(t *testing.T) {
	if got := SkippedReport(nil, nil); len(got) != 0 {
		t.Errorf("nothing left out: got %v", got)
	}
	// Five sockets in a home folder: no "could not be read" part.
	var sockets []string
	for i := 0; i < 5; i++ {
		sockets = append(sockets, fmt.Sprintf("socket (skipped, holds no file data): /home/u/.s%d", i))
	}
	all := strings.Join(SkippedReport(sockets, nil), "\n")
	if strings.Contains(all, "COULD NOT BE READ") || !strings.Contains(all, "=== LEFT OUT BY DESIGN (5) ===") ||
		!strings.Contains(all, "         5  socket (skipped, holds no file data)") || strings.Contains(all, "more, not listed") {
		t.Errorf("by design only:\n%s", all)
	}
	// Only read errors: no "by design" part.
	bad := []string{"Cannot read directory: /root/x (Error: open /root/x: permission denied)"}
	all = strings.Join(SkippedReport(bad, bad), "\n")
	if strings.Contains(all, "BY DESIGN") || !strings.Contains(all, "  [1] "+bad[0]) ||
		!strings.Contains(all, "         1  Cannot read directory: permission denied") {
		t.Errorf("read errors only:\n%s", all)
	}
}

// Past the limit the log says how many are not named and where they are.
func TestSkippedReportLimit(t *testing.T) {
	var unreadable []string
	for i := 0; i < 7; i++ {
		unreadable = append(unreadable, fmt.Sprintf("Cannot stat file: /d/f%d (Error: lstat /d/f%d: input/output error)", i, i))
	}
	lines := skippedReport(unreadable, unreadable, 5, 50)
	all := strings.Join(lines, "\n")
	if !strings.Contains(all, "=== COULD NOT BE READ (7) ===") || !strings.Contains(all, "  [5] ") || strings.Contains(all, "  [6] ") ||
		!strings.Contains(all, "  ... and 2 more, not listed here (the status file stored with the snapshot names every one)") {
		t.Errorf("limit of 5:\n%s", all)
	}
	if !strings.Contains(all, "         7  Cannot stat file: input/output error") {
		t.Errorf("the count by reason covers every file, listed or not:\n%s", all)
	}
}

// A file named twice among the read errors is taken out of the by-design
// list twice, and a by-design item is kept whatever else is there.
func TestWithoutUnreadable(t *testing.T) {
	skipped := []string{"A", "junction", "A", "B", "socket"}
	got := withoutUnreadable(skipped, []string{"A", "A", "B"})
	if strings.Join(got, ",") != "junction,socket" {
		t.Errorf("got %v", got)
	}
	if got := withoutUnreadable(skipped, nil); len(got) != 5 {
		t.Errorf("no read errors: got %v", got)
	}
}

func TestReadErrorReason(t *testing.T) {
	for msg, want := range map[string]string{
		`Cannot open file: C:\a b\c.txt (Error: open C:\a b\c.txt: Access is denied.)`:                                                       "Cannot open file: Access is denied.",
		`Cannot stat directory: /srv/x (Error: lstat /srv/x: permission denied)`:                                                             "Cannot stat directory: permission denied",
		`Cannot read symlink: /srv/l (Error: readlink /srv/l: invalid argument)`:                                                             "Cannot read symlink: invalid argument",
		`Read failed after 10 of 4096 bytes, zero-padded to declared size (content incomplete): /d/f (Error: read /d/f: input/output error)`: "Read failed part way through the file: input/output error",
		`File shrank during backup, zero-padded to declared size (content inconsistent): /d/f`:                                               "File shrank during backup",
		`File grew during backup, tail past 4096 bytes not captured (content incomplete): /d/f`:                                              "File grew during backup",
		`Cannot open file: /d/odd (Error: some error with no path)`:                                                                          "Cannot open file: some error with no path",
		`something else entirely`: "something else entirely",
	} {
		if got := ReadErrorReason(msg); got != want {
			t.Errorf("%s\n  got  %q\n  want %q", msg, got, want)
		}
	}
}

func TestSkipKind(t *testing.T) {
	for msg, want := range map[string]string{
		`Junction point (skipped): C:\Users\u\Application Data`:          "Junction point (skipped)",
		`System folder (auto-excluded): C:\$Recycle.Bin`:                 "System folder (auto-excluded)",
		`fifo (skipped, holds no file data): /run/x`:                     "fifo (skipped, holds no file data)",
		`Metadata collect failed for file C:\x\y.txt: Access is denied.`: "Permissions or attributes not recorded (the item itself is in the backup)",
		`Metadata collect failed for dir C:\x: Access is denied.`:        "Permissions or attributes not recorded (the item itself is in the backup)",
	} {
		if got := skipKind(msg); got != want {
			t.Errorf("%s: got %q, want %q", msg, got, want)
		}
	}
}

func firstLines(lines []string, n int) string {
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
