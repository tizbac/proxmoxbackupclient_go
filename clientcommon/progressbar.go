package clientcommon

// The progress bar is the one from FastCopy
// (github.com/tizbac/FastCopy, internal/progress/progress.go), with the
// terminal width taken from golang.org/x/term so it builds on every system
// this client does, plus Complete for a backup job whose totals are not known
// up front. It draws a bordered ANSI box with a bar, counters, throughput and
// ETA that redraws in place on a terminal and collapses to one summary line
// elsewhere. Pass os.Stderr: progress is a diagnostic stream. While the box is
// active on a terminal, CaptureStdout routes what would be printed to stdout
// above it on stderr too, so the box stays readable; when stdout is redirected
// it is left alone.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// ANSI colors used by the progress bar.
const (
	colReset   = "\x1b[0m"
	colDim     = "\x1b[38;5;245m"
	colFrame   = "\x1b[38;5;240m"
	colFilled  = "\x1b[1;38;5;40m"
	colHead    = "\x1b[38;5;84m"
	colEmpty   = "\x1b[38;5;238m"
	colCyan    = "\x1b[1;38;5;39m"
	colYellow  = "\x1b[1;38;5;214m"
	colMagenta = "\x1b[1;38;5;208m"
	colBlue    = "\x1b[38;5;75m"
	colWhite   = "\x1b[38;5;250m"
)

// Progress draws a multi-line ANSI box on the terminal. It is safe to call
// from several worker goroutines.
type Progress struct {
	mu         sync.Mutex
	out        io.Writer
	report     io.Writer // plain-mode summary goes here (defaults to out)
	enabled    bool
	used       bool // something was drawn or counted: Finish reports it
	plain      bool // non-tty output: emit one summary line at the end
	width      int
	totalFiles int
	totalBytes int64
	doneFiles  int
	doneBytes  int64
	current    string
	start      time.Time
	lines      int
	last       time.Time
	pulse      int
	// Scan pre-pass state (walking the tree or indexing physical extents).
	scan      bool
	scanLabel string
	scanFiles int64
}

func NewProgress(out io.Writer, enabled bool) *Progress {
	width := 80
	if f, ok := out.(*os.File); ok {
		width = terminalWidth(f)
	}
	p := &Progress{
		out:     out,
		report:  out,
		enabled: enabled,
		width:   width,
		start:   time.Now(),
	}
	if !IsTTY(out) {
		p.plain = true
	}
	if f, ok := out.(*os.File); ok && !p.plain && !enableANSI(f) {
		p.plain = true
	}
	if p.width > 72 {
		p.width = 72
	}
	if p.width < 44 {
		p.width = 44
	}
	return p
}

// terminalWidth asks the terminal for its columns, falling back to COLUMNS/80.
func terminalWidth(f *os.File) int {
	if f != nil {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 0 {
			return w
		}
	}
	if c, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && c > 0 {
		return c
	}
	return 80
}

// SetReportWriter chooses where the one-line summary of a non-terminal run is
// written. The box and Line messages keep using the writer passed to
// NewProgress.
func (p *Progress) SetReportWriter(w io.Writer) {
	p.mu.Lock()
	p.report = w
	p.mu.Unlock()
}

// IsTTY reports whether w is a character device (a real terminal).
func IsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

// FormatBytes renders a byte count in base 1024 units.
func FormatBytes(n int64) string {
	if n < 1024 {
		return strconv.FormatInt(n, 10) + " B"
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	f := float64(n)
	for _, u := range units {
		f /= 1024
		if f < 1024 {
			return strconv.FormatFloat(f, 'f', 1, 64) + " " + u
		}
	}
	return strconv.FormatFloat(f, 'f', 1, 64) + " EB"
}

// FormatDuration renders a duration as 37s / 4m12s / 1h02m.
func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return strconv.Itoa(int(d.Seconds())) + "s"
	}
	if d < time.Hour {
		return strconv.Itoa(int(d.Minutes())) + "m" + strconv.Itoa(int(d.Seconds())%60) + "s"
	}
	return strconv.Itoa(int(d.Hours())) + "h" +
		strconv.Itoa(int(d.Minutes())%60) + "m"
}

// BeginScan starts a pre-pass whose size is not known yet: walking the tree,
// or looking up the physical extents that --fast needs. The box slides a pulse
// and counts the items visited so far, so a long scan does not look hung.
func (p *Progress) BeginScan(label string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.enabled {
		return
	}
	p.used = true
	p.scan = true
	p.scanLabel = label
	p.scanFiles = 0
	p.totalFiles = 0
	p.totalBytes = 0
	p.doneFiles = 0
	p.doneBytes = 0
	p.current = ""
	p.start = time.Now()
	p.last = time.Time{}
	if !p.plain {
		p.draw()
	}
}

// ScanStep records one more item visited during a scan pre-pass.
func (p *Progress) ScanStep(name string) {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.scanFiles++
	p.current = name
	if !p.plain && p.scan && time.Since(p.last) > 100*time.Millisecond {
		p.last = time.Now()
		p.draw()
	}
}

// StartCopy ends a scan pre-pass and sets the totals for the copy itself.
// Zero for both totals means they are not known: the bar then slides a pulse
// and shows the bytes counted so far (an indeterminate bar).
func (p *Progress) StartCopy(files int, bytes int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.used = true
	p.scan = false
	p.totalFiles = files
	p.totalBytes = bytes
	p.doneFiles = 0
	p.doneBytes = 0
	p.start = time.Now()
	p.last = time.Time{}
	if p.enabled && !p.plain {
		p.draw()
	}
}

func (p *Progress) SetTotal(files int, bytes int64) {
	p.mu.Lock()
	p.totalFiles = files
	p.totalBytes = bytes
	p.used = true
	p.mu.Unlock()
}

func (p *Progress) AddBytes(n int64) {
	p.mu.Lock()
	p.doneBytes += n
	p.used = true
	if p.enabled && !p.plain && time.Since(p.last) > 100*time.Millisecond {
		p.last = time.Now()
		p.draw()
	}
	p.mu.Unlock()
}

func (p *Progress) AddFile() {
	p.mu.Lock()
	p.doneFiles++
	p.used = true
	if p.enabled && !p.plain && time.Since(p.last) > 100*time.Millisecond {
		p.last = time.Now()
		p.draw()
	}
	p.mu.Unlock()
}

// SetBytesDone records an absolute byte counter. Reading modes use it to
// show how much of the archive has been consumed, where AddBytes would
// count the same member twice.
func (p *Progress) SetBytesDone(n int64) {
	p.mu.Lock()
	p.doneBytes = n
	p.used = true
	if p.enabled && !p.plain && time.Since(p.last) > 100*time.Millisecond {
		p.last = time.Now()
		p.draw()
	}
	p.mu.Unlock()
}

// Complete finishes the counts before Finish: a known total is shown as fully
// done, and an unknown one (standard input, a stream) becomes the bytes and
// files actually handled. It marks a job that had no figures at all as unused,
// so Finish stays quiet.
func (p *Progress) Complete() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.used {
		return
	}
	if p.scan {
		p.scan = false
	}
	if p.totalBytes <= 0 {
		p.totalBytes = p.doneBytes
	} else {
		p.doneBytes = p.totalBytes
	}
	if p.totalFiles <= 0 {
		p.totalFiles = p.doneFiles
	} else {
		p.doneFiles = p.totalFiles
	}
}

// Done reports how many members have been processed (used for checkpoints).
func (p *Progress) Done() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.doneFiles
}

// Current records the member being worked on so the box can show it.
func (p *Progress) Current(name string) {
	p.mu.Lock()
	p.current = name
	p.mu.Unlock()
}

// Active reports whether an animated box is currently drawn on the terminal.
func (p *Progress) Active() bool {
	return p.enabled && !p.plain
}

// Line prints a message just above the box so the bar keeps its place, and is
// a plain line when there is no bar. Use it for everything the tool prints
// while the bar is running, otherwise the box drifts down one line per message.
func (p *Progress) Line(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if !p.enabled || p.plain {
		fmt.Fprintln(p.out, msg)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.used = true
	if p.lines > 0 {
		fmt.Fprintf(p.out, "\x1b[%dA", p.lines)
	}
	fmt.Fprintf(p.out, "%s\x1b[K\n", msg)
	lines := p.boxLines()
	p.emit(lines)
	p.lines = len(lines)
}

// CaptureStdout routes what code writes to os.Stdout above the box while the
// box is active, so a message printed directly (a library's own status line)
// does not overwrite it. It is used only when stdout is a terminal too: when
// stdout is redirected the bar is on stderr and the two cannot collide, so the
// redirected output is left alone. The returned function closes the pipe,
// waits for the lines to be shown and puts os.Stdout back.
func (p *Progress) CaptureStdout() (restore func()) {
	if !p.Active() || !IsTTY(os.Stdout) {
		return func() {}
	}
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return func() {}
	}
	os.Stdout = w
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			p.Line("%s", sc.Text())
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			_ = w.Close()
			os.Stdout = old
			<-done
			_ = r.Close()
		})
	}
}

// Writer returns an io.Writer that shows each line above the box through
// Line. Give it to a logger while the bar is active, so log lines do not
// overwrite it.
func (p *Progress) Writer() io.Writer { return progressWriter{p} }

type progressWriter struct{ p *Progress }

func (w progressWriter) Write(b []byte) (int, error) {
	text := strings.TrimRight(string(b), "\r\n")
	for _, line := range strings.Split(text, "\n") {
		w.p.Line("%s", strings.TrimRight(line, "\r"))
	}
	return len(b), nil
}

func (p *Progress) speedMB() float64 {
	elapsed := time.Since(p.start).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return float64(p.doneBytes) / elapsed / 1e6
}

func (p *Progress) eta() time.Duration {
	if p.doneBytes <= 0 || p.totalBytes <= 0 || p.doneBytes >= p.totalBytes {
		return 0
	}
	rate := float64(p.doneBytes) / time.Since(p.start).Seconds()
	if rate <= 0 {
		return 0
	}
	return time.Duration(float64(p.totalBytes-p.doneBytes) / rate * float64(time.Second))
}

// draw must be called with p.mu held. It assumes the cursor sits one line
// below the box that was drawn last time.
func (p *Progress) draw() {
	if p.plain {
		return
	}
	lines := p.boxLines()
	if p.lines > 0 {
		// Move back to the first line of the box and repaint it in place.
		fmt.Fprintf(p.out, "\x1b[%dA", p.lines)
	}
	p.emit(lines)
	p.lines = len(lines)
}

func (p *Progress) emit(lines []string) {
	for _, l := range lines {
		fmt.Fprintf(p.out, "%s\x1b[K\n", l)
	}
}

// unknownTotals reports whether the totals were never set, in which case the
// bar cannot show a percentage and slides a pulse instead.
func (p *Progress) unknownTotals() bool {
	return p.totalFiles <= 0 && p.totalBytes <= 0
}

func (p *Progress) boxLines() []string {
	inner := p.width - 4 // content between the two vertical bars

	unknown := p.unknownTotals()
	frac := 0.0
	if !unknown {
		if p.totalBytes > 0 {
			frac = float64(p.doneBytes) / float64(p.totalBytes)
		} else {
			frac = float64(p.doneFiles) / float64(p.totalFiles)
		}
		if frac > 1 {
			frac = 1
		}
	}

	pct := fmt.Sprintf("%5.1f%%", frac*100)
	if unknown {
		pct = "  ... "
	}
	barW := inner - len(pct) - 2
	if barW < 1 {
		barW = 1
	}

	x := frac * float64(barW)
	filled := int(x)
	if filled > barW {
		filled = barW
	}
	head := ""
	empty := barW - filled
	if unknown {
		// No totals: sweep a short glow across the bar so it is still alive.
		p.pulse++
		glow := 6
		pos := (p.pulse / 4) % (barW + glow)
		filled = 0
		empty = 0
		head = ""
		for i := 0; i < barW; i++ {
			if i >= pos-glow && i < pos {
				head += "▒"
			} else {
				empty++
			}
		}
		if head == "" {
			head = "░"
			if empty > 0 {
				empty--
			}
		}
		filled = 0
	} else if rem := x - float64(filled); filled < barW && rem > 0.1 {
		if rem < 0.5 {
			head = "▒"
		} else {
			head = "▓"
		}
		empty--
	}

	bar := colFilled + strings.Repeat("█", filled) + colReset
	if head != "" {
		bar += colHead + head + colReset
	}
	if empty > 0 {
		bar += colEmpty + strings.Repeat("░", empty) + colReset
	}

	stats := colCyan + fmt.Sprintf("%d/%d files", p.doneFiles, p.totalFiles) + colReset +
		colDim + " · " + colReset +
		colYellow + FormatBytes(p.doneBytes) + colDim + " / " + colReset + colWhite + FormatBytes(p.totalBytes) + colReset +
		colDim + " · " + colReset +
		colMagenta + fmt.Sprintf("%.1f MB/s", p.speedMB()) + colReset +
		colDim + " · " + colReset +
		colBlue + "ETA " + FormatDuration(p.eta()) + colReset
	if p.scan {
		// The totals are unknown until the walk or the extent lookup is done.
		stats = colCyan + fmt.Sprintf("%d files", p.scanFiles) + colReset +
			colDim + " · " + colReset + colBlue + p.scanLabel + colReset
	} else if unknown || p.totalFiles <= 0 {
		// Extraction only knows the archive size, not the member count.
		bytes := colYellow + FormatBytes(p.doneBytes) + colReset
		eta := "ETA ?"
		if p.totalBytes > 0 {
			bytes += colDim + " / " + colReset + colWhite + FormatBytes(p.totalBytes) + colReset
			eta = "ETA " + FormatDuration(p.eta())
		}
		stats = colCyan + fmt.Sprintf("%d files", p.doneFiles) + colReset +
			colDim + " · " + colReset +
			bytes +
			colDim + " · " + colReset +
			colMagenta + fmt.Sprintf("%.1f MB/s", p.speedMB()) + colReset +
			colDim + " · " + colReset +
			colBlue + eta + colReset
	}

	file := p.current
	if file == "" {
		file = "-"
	}
	if len(file) > inner-2 {
		file = "…" + file[len(file)-inner+3:]
	}
	fileLine := colDim + "→ " + colReset + file

	top := colFrame + "╭" + strings.Repeat("─", inner+2) + "╮" + colReset
	bottom := colFrame + "╰" + strings.Repeat("─", inner+2) + "╯" + colReset

	lines := []string{
		top,
		" " + bar + " " + colYellow + pct + colReset + " ",
		" " + stats + " ",
		" " + fileLine + " ",
		bottom,
	}
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return lines
}

// Finish repaints the completed bar and leaves the terminal in a clean state.
// It returns the summary line it printed (empty when there was no bar).
func (p *Progress) Finish() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.enabled || !p.used {
		return ""
	}
	if p.plain {
		files := fmt.Sprintf("%d/%d files", p.doneFiles, p.totalFiles)
		if p.scan {
			files = fmt.Sprintf("%d files", p.scanFiles)
		} else if p.totalFiles <= 0 {
			files = fmt.Sprintf("%d files", p.doneFiles)
		}
		line := fmt.Sprintf("%s, %s in %s (%.1f MB/s)",
			files, FormatBytes(p.doneBytes),
			FormatDuration(time.Since(p.start)), p.speedMB())
		fmt.Fprintln(p.report, line)
		return line
	}
	p.draw()
	fmt.Fprintln(p.out)
	return ""
}
