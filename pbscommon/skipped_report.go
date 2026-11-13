package pbscommon

import (
	"fmt"
	"sort"
	"strings"
)

// What a folder backup left out, written out for its log.
//
// The log used to give the first 50 of everything a backup left out, in the
// order it met them. On a Windows system drive the first few hundred are
// junction points and system files, left out on purpose, so a backup that
// could not read 31,789 files named none of them (issue #149). The files it
// could not read now come first, each one by name, with a count for each
// reason; the items left out on purpose follow, a count for each kind and a
// short sample.

// UnreadableLogLimit is how many of the files a backup could not read are
// named in its log. A backup that could read nothing at all would otherwise
// write a log as long as the folder's file list; the status file stored with
// the snapshot (BackupStatusFilename) names every one.
const UnreadableLogLimit = 50000

// byDesignLogLimit is how many of the items left out on purpose are named.
const byDesignLogLimit = 50

// reasonLogLimit is how many reasons, or kinds, get a line of their own.
const reasonLogLimit = 20

// SkippedReport returns the lines for a backup's log: skipped is everything
// it left out (PXARArchive.SkippedFiles), unreadable the part of those it
// could not read (PXARArchive.ReadErrors). Nothing left out: no lines.
func SkippedReport(skipped, unreadable []string) []string {
	return skippedReport(skipped, unreadable, UnreadableLogLimit, byDesignLogLimit)
}

func skippedReport(skipped, unreadable []string, maxUnreadable, maxByDesign int) []string {
	var lines []string
	if len(unreadable) > 0 {
		lines = append(lines, fmt.Sprintf("=== COULD NOT BE READ (%d) ===", len(unreadable)))
		lines = append(lines, "  By reason:")
		lines = append(lines, countLines(unreadable, ReadErrorReason, "reasons")...)
		lines = append(lines, listLines(unreadable, maxUnreadable,
			"the status file stored with the snapshot names every one")...)
		lines = append(lines, "=== END COULD NOT BE READ ===")
	}
	if byDesign := withoutUnreadable(skipped, unreadable); len(byDesign) > 0 {
		lines = append(lines, fmt.Sprintf("=== LEFT OUT BY DESIGN (%d) ===", len(byDesign)))
		lines = append(lines, "  By kind:")
		lines = append(lines, countLines(byDesign, skipKind, "kinds")...)
		lines = append(lines, listLines(byDesign, maxByDesign, "of the kinds counted above")...)
		lines = append(lines, "=== END LEFT OUT BY DESIGN ===")
	}
	return lines
}

// withoutUnreadable is skipped less the messages in unreadable (each read
// error is in both lists), in the same order.
func withoutUnreadable(skipped, unreadable []string) []string {
	if len(unreadable) == 0 {
		return skipped
	}
	left := make(map[string]int, len(unreadable))
	for _, u := range unreadable {
		left[u]++
	}
	out := make([]string, 0, len(skipped))
	for _, s := range skipped {
		if left[s] > 0 {
			left[s]--
			continue
		}
		out = append(out, s)
	}
	return out
}

// listLines numbers the first max of msgs, and says how many more there are.
func listLines(msgs []string, max int, rest string) []string {
	n := len(msgs)
	if n > max {
		n = max
	}
	lines := make([]string, 0, n+1)
	for i := 0; i < n; i++ {
		lines = append(lines, fmt.Sprintf("  [%d] %s", i+1, msgs[i]))
	}
	if more := len(msgs) - n; more > 0 {
		lines = append(lines, fmt.Sprintf("  ... and %d more, not listed here (%s)", more, rest))
	}
	return lines
}

// countLines gives one line for each key of msgs with how many have it, the
// most frequent first, and one line for the rest past reasonLogLimit.
func countLines(msgs []string, key func(string) string, what string) []string {
	counts := map[string]int{}
	for _, m := range msgs {
		counts[key(m)]++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	var lines []string
	other := 0
	for i, k := range keys {
		if i >= reasonLogLimit {
			other += counts[k]
			continue
		}
		lines = append(lines, fmt.Sprintf("  %8d  %s", counts[k], k))
	}
	if other > 0 {
		lines = append(lines, fmt.Sprintf("  %8d  other %s", other, what))
	}
	return lines
}

// ReadErrorReason is a read error message without the file it names, so that
// messages for the same cause count together: "Cannot open file: C:\a.txt
// (Error: open C:\a.txt: Access is denied.)" gives "Cannot open file: Access
// is denied.".
func ReadErrorReason(msg string) string {
	what := msg
	if i := strings.Index(msg, ": "); i >= 0 {
		what = msg[:i]
	}
	// These three carry byte counts, which differ from file to file.
	for _, p := range []string{"Read failed after", "File shrank during backup", "File grew during backup"} {
		if strings.HasPrefix(what, p) {
			what = p
			if p == "Read failed after" {
				what = "Read failed part way through the file"
			}
		}
	}
	const mark = "(Error: "
	if i := strings.LastIndex(msg, mark); i >= 0 && strings.HasSuffix(msg, ")") {
		cause := msg[i+len(mark) : len(msg)-1]
		// "open <path>: Access is denied.": the cause is what follows the path.
		if j := strings.LastIndex(cause, ": "); j >= 0 {
			cause = cause[j+2:]
		}
		if cause != "" {
			what += ": " + cause
		}
	}
	return what
}

// skipKind is the kind of an item left out on purpose: its message without
// the path ("Junction point (skipped)").
func skipKind(msg string) string {
	if strings.HasPrefix(msg, "Metadata collect failed") {
		return "Permissions or attributes not recorded (the item itself is in the backup)"
	}
	if i := strings.Index(msg, ": "); i >= 0 {
		return msg[:i]
	}
	return msg
}
