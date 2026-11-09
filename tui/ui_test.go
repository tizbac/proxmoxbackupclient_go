package tui

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// harness runs the real UI on a simulated terminal over the demo backend.
type harness struct {
	t   *testing.T
	a   *App
	sim tcell.SimulationScreen
}

func newHarness(t *testing.T, w, h int) *harness {
	t.Helper()
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	sim.SetSize(w, h)
	a := NewApp(newDemoBackend(), "test")
	a.app.SetScreen(sim)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = a.Run()
	}()
	t.Cleanup(func() {
		a.app.Stop()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	})
	hn := &harness{t: t, a: a, sim: sim}
	hn.waitFor("Documents")
	return hn
}

func (h *harness) screen() string {
	cells, w, hgt := h.sim.GetContents()
	var b strings.Builder
	for y := 0; y < hgt; y++ {
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) == 0 {
				b.WriteByte(' ')
			} else {
				b.WriteString(string(c.Runes[:1]))
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (h *harness) waitFor(s string) {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(h.screen(), s) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	h.t.Fatalf("screen never showed %q:\n%s", s, h.screen())
}

func (h *harness) key(k tcell.Key, r rune) {
	h.sim.InjectKey(k, r, tcell.ModNone)
	time.Sleep(150 * time.Millisecond)
}

func (h *harness) typ(r rune) { h.key(tcell.KeyRune, r) }

func TestDashboardShowsSetsRunsAndRecent(t *testing.T) {
	h := newHarness(t, 130, 36)
	s := h.screen()
	for _, want := range []string{"Salvanza Backup", "[DEMO DATA]", "service running", "Backup Sets", "Documents", "Machine image",
		"Photos", "Web server", "Running now", "Nothing is running", "Recent", "Failed"} {
		if !strings.Contains(s, want) {
			t.Errorf("dashboard lacks %q:\n%s", want, s)
		}
	}
}

func TestTabsSwitchAndShowTheirScreens(t *testing.T) {
	h := newHarness(t, 130, 36)
	cases := []struct {
		r    rune
		want []string
	}{
		{'2', []string{"Details", "Backup ID", "Sources"}},
		{'3', []string{"History", "no route to host"}},
		{'4', []string{"Message Log", "WARNING", "skipped"}},
		{'5', []string{"PBS servers", "Main PBS", "Fingerprint", "pbs.example.com"}},
		{'1', []string{"Running now"}},
	}
	for _, c := range cases {
		h.typ(c.r)
		s := h.screen()
		for _, w := range c.want {
			if !strings.Contains(s, w) {
				t.Errorf("after %q the screen lacks %q:\n%s", c.r, w, s)
			}
		}
	}
	h.key(tcell.KeyTab, 0)
	h.waitFor("Details")
}

func TestRunPauseResumeStop(t *testing.T) {
	h := newHarness(t, 130, 36)
	h.typ('r') // the first set, Documents
	h.waitFor("Documents")
	h.waitFor("%")
	if !strings.Contains(h.screen(), "files") {
		t.Errorf("a running backup shows its files:\n%s", h.screen())
	}
	h.typ('p')
	h.waitFor("paused")
	h.typ('p')
	h.waitFor("Documents")
	h.typ('s')
	h.waitFor("Stop the running backup")
	h.key(tcell.KeyEnter, 0) // "Yes" is the first button
	h.waitFor("failed")
	if strings.Contains(h.screen(), "Stop the running backup") {
		t.Errorf("the confirmation closed:\n%s", h.screen())
	}
}

func TestStopCanBeCancelled(t *testing.T) {
	h := newHarness(t, 130, 36)
	h.typ('r')
	h.waitFor("%")
	h.typ('s')
	h.waitFor("Stop the running backup")
	h.key(tcell.KeyTab, 0) // to "No"
	h.key(tcell.KeyEnter, 0)
	time.Sleep(300 * time.Millisecond)
	if strings.Contains(h.screen(), "failed") && strings.Contains(h.screen(), "stopped by the user") {
		t.Errorf("the backup was stopped although No was chosen:\n%s", h.screen())
	}
}

func TestEnableDisable(t *testing.T) {
	h := newHarness(t, 130, 36)
	h.typ('e') // Documents off
	h.waitFor("(off)")
	h.typ('e')
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && strings.Contains(h.screen(), "(off)  ") && strings.Count(h.screen(), "(off)") > 1 {
		time.Sleep(50 * time.Millisecond)
	}
}

func TestHelpOpensAndCloses(t *testing.T) {
	h := newHarness(t, 130, 36)
	h.typ('?')
	h.waitFor("run the selected Backup Set now")
	h.key(tcell.KeyEscape, 0)
	time.Sleep(200 * time.Millisecond)
	if strings.Contains(h.screen(), "run the selected Backup Set now") {
		t.Errorf("help did not close:\n%s", h.screen())
	}
}

func TestNarrowTerminalStillDraws(t *testing.T) {
	h := newHarness(t, 80, 24)
	s := h.screen()
	if !strings.Contains(s, "Backup Sets") || !strings.Contains(s, "Documents") {
		t.Errorf("an 80x24 terminal shows the sets:\n%s", s)
	}
}

func TestRelativeTimes(t *testing.T) {
	now := time.Now()
	at := func(d int, h, m int) time.Time {
		return time.Date(now.Year(), now.Month(), now.Day()+d, h, m, 0, 0, now.Location())
	}
	for _, c := range []struct {
		t    time.Time
		want string
	}{
		{time.Time{}, "never"}, {at(0, 18, 0), "today 18:00"}, {at(1, 2, 5), "tomorrow 02:05"}, {at(-1, 23, 59), "yesterday 23:59"},
	} {
		if got := when(c.t); got != c.want {
			t.Errorf("when(%v) = %q, want %q", c.t, got, c.want)
		}
	}
}

func TestScheduleText(t *testing.T) {
	m := map[string]interface{}{"scheduleTime": "18:00", "triggerMode": "daily"}
	if got := scheduleText(m); got != "daily 18:00" {
		t.Errorf("daily: %q", got)
	}
	m = map[string]interface{}{"scheduleMode": "manual"}
	if got := scheduleText(m); got != "manual" {
		t.Errorf("manual: %q", got)
	}
	m = map[string]interface{}{"triggerMode": "interval", "intervalMinutes": float64(30), "daysOfWeek": []interface{}{"Mon", "Fri"}}
	if got := scheduleText(m); got != "every 30 min Mon,Fri" {
		t.Errorf("interval: %q", got)
	}
}

// TestDumpScreens writes each screen as text and as an HTML page (colours
// kept), for the mockup images, when TUI_DUMP names a folder.
func TestDumpScreens(t *testing.T) {
	dir := os.Getenv("TUI_DUMP")
	if dir == "" {
		t.Skip("TUI_DUMP not set")
	}
	_ = os.MkdirAll(dir, 0o755)
	h := newHarness(t, 130, 38)
	h.typ('r')
	h.waitFor("%")
	time.Sleep(2500 * time.Millisecond)
	shots := []struct {
		name string
		do   func()
	}{
		{"1-dashboard", func() {}},
		{"2-sets", func() { h.typ('2') }},
		{"3-history", func() { h.typ('3') }},
		{"4-messages", func() { h.typ('4') }},
		{"5-servers", func() { h.typ('5') }},
		{"6-help", func() { h.typ('1'); h.typ('?') }},
	}
	for _, s := range shots {
		s.do()
		time.Sleep(300 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(dir, s.name+".txt"), []byte(h.screen()), 0o644)
		_ = os.WriteFile(filepath.Join(dir, s.name+".html"), []byte(h.htmlScreen()), 0o644)
	}
}

func (h *harness) htmlScreen() string {
	cells, w, hgt := h.sim.GetContents()
	var b strings.Builder
	b.WriteString(`<!doctype html><meta charset=utf-8><body style="margin:0;background:#0b0f14"><div style="display:inline-block;margin:14px;border:1px solid #2b3440;border-radius:6px;overflow:hidden"><pre style="margin:0;font:15px/1.25 'Cascadia Mono',Consolas,monospace;background:#12171e">`)
	hex := func(c tcell.Color, def string) string {
		if c == tcell.ColorDefault || !c.Valid() {
			return def
		}
		r, g, bl := c.RGB()
		return fmt.Sprintf("#%02x%02x%02x", r, g, bl)
	}
	for y := 0; y < hgt; y++ {
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			fg, bg, attr := c.Style.Decompose()
			ch := " "
			if len(c.Runes) > 0 {
				ch = string(c.Runes[:1])
			}
			style := fmt.Sprintf("color:%s;background:%s", hex(fg, "#d6dde6"), hex(bg, "#12171e"))
			if attr&tcell.AttrBold != 0 {
				style += ";font-weight:bold"
			}
			fmt.Fprintf(&b, `<span style="%s">%s</span>`, style, html.EscapeString(ch))
		}
		b.WriteString("\n")
	}
	b.WriteString("</pre></div>")
	return b.String()
}
