package clientcommon

import (
	"bytes"
	"strings"
	"testing"
)

// A run whose totals were known is reported complete, and one whose totals
// were never known (a stream) turns into the bytes it handled. Both collapse
// to one summary line when the output is not a terminal.
func TestProgressComplete(t *testing.T) {
	var b bytes.Buffer
	p := NewProgress(&b, true)
	if p.Active() {
		t.Fatal("a buffer is not a terminal: the bar should be plain")
	}
	p.StartCopy(3, 10)
	p.AddBytes(4)
	p.Complete()
	line := p.Finish()
	if !strings.Contains(line, "3/3 files") || !strings.Contains(line, "10 B") {
		t.Errorf("summary %q, want 3/3 files and 10 B", line)
	}

	b.Reset()
	p = NewProgress(&b, true)
	p.StartCopy(0, 0) // indeterminate: standard input
	p.AddBytes(123)
	p.Complete()
	line = p.Finish()
	if !strings.Contains(line, "123 B") {
		t.Errorf("stream summary %q, want the 123 bytes read", line)
	}
}

// Nothing drawn (no BeginScan, StartCopy or counts) leaves Finish silent.
func TestProgressFinishUnused(t *testing.T) {
	var b bytes.Buffer
	p := NewProgress(&b, true)
	if line := p.Finish(); line != "" || b.Len() != 0 {
		t.Errorf("Finish of an unused bar printed %q (%q)", line, b.String())
	}
}

// FormatBytes is what the bar and the callback summaries show.
func TestFormatBytes(t *testing.T) {
	for n, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KB", 1536: "1.5 KB", 1 << 20: "1.0 MB"} {
		if got := FormatBytes(n); got != want {
			t.Errorf("FormatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}
