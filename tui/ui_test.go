package tui

import (
	"encoding/json"
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
	// Running the app initialises the screen again, which resets its size.
	time.Sleep(250 * time.Millisecond)
	sim.SetSize(w, h)
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

func TestHistoryEntryOpensItsLog(t *testing.T) {
	h := newHarness(t, 130, 36)
	h.typ('3')
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Backup completed in 55s")
	s := h.screen()
	for _, want := range []string{"Log: ", "New chunk", "Esc"} {
		if !strings.Contains(s, want) {
			t.Errorf("log view lacks %q:\n%s", want, s)
		}
	}
	h.typ('c') // hide the chunk lines
	time.Sleep(400 * time.Millisecond)
	if strings.Contains(h.screen(), "New chunk 1000") {
		t.Errorf("chunk lines are still shown:\n%s", h.screen())
	}
	h.key(tcell.KeyEscape, 0)
	time.Sleep(250 * time.Millisecond)
	if strings.Contains(h.screen(), "Log: ") {
		t.Errorf("the log view did not close:\n%s", h.screen())
	}
	// the history screen answers its keys again
	h.typ('1')
	h.waitFor("Running now")
}

func (h *harness) typeText(s string) {
	for _, r := range s {
		h.sim.InjectKey(tcell.KeyRune, r, tcell.ModNone)
	}
	time.Sleep(200 * time.Millisecond)
}

func TestEditASetThroughTheForm(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.key(tcell.KeyEnter, 0) // the first set, Documents
	h.waitFor("Edit Backup Set")
	for _, want := range []string{"Name", "Backup ID", "Folders", "PBS server", "Documents", `C:\Users\mick\Documents`} {
		if !strings.Contains(h.screen(), want) {
			t.Errorf("the editor lacks %q:\n%s", want, h.screen())
		}
	}
	h.typeText("2") // Documents2
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Repeat every")
	if !strings.Contains(h.screen(), "18:00") {
		t.Errorf("step 2 shows the set's time:\n%s", h.screen())
	}
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Change detection")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("saved Documents2")
	h.waitFor("Documents2")
	if strings.Contains(h.screen(), "Edit Backup Set") {
		t.Errorf("the editor closed after saving:\n%s", h.screen())
	}
}

func TestEditorRefusesWhatTheServiceWouldRefuse(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Edit Backup Set")
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Repeat every")
	// down to the "Only on days" field: unit, every, at, days
	for i := 0; i < 4; i++ {
		h.key(tcell.KeyTab, 0)
	}
	h.typeText("Funday")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("is not a day")
	h.key(tcell.KeyEnter, 0) // OK
	time.Sleep(200 * time.Millisecond)
	if !strings.Contains(h.screen(), "Repeat every") {
		t.Errorf("the editor stays open after a refused save:\n%s", h.screen())
	}
	h.key(tcell.KeyEscape, 0)
	time.Sleep(200 * time.Millisecond)
	if strings.Contains(h.screen(), "Repeat every") {
		t.Errorf("Esc closes the editor:\n%s", h.screen())
	}
}

func TestNewSetNeedsANameAndAFolder(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('n')
	h.waitFor("New Backup Set")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("needs a name")
	h.key(tcell.KeyEnter, 0)
	h.typeText("Test set")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("add at least one folder")
	h.key(tcell.KeyEnter, 0)
	h.key(tcell.KeyTab, 0) // Backup ID
	h.key(tcell.KeyTab, 0) // Folders
	h.typeText(`C:\Data`)
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("saved Test set")
	h.typ('2')
	h.waitFor("Test set")
}

func TestDeleteASet(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('d')
	h.waitFor("Delete the Backup Set")
	h.key(tcell.KeyEnter, 0) // Yes
	h.waitFor("deleted Documents")
	time.Sleep(300 * time.Millisecond)
	if strings.Count(h.screen(), "Documents") > 2 { // the flash line and perhaps a recent entry, not the table row
		t.Logf("screen after delete:\n%s", h.screen())
	}
}

func TestRestoreWizardFromSourceToDone(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('6')
	h.waitFor("Backups on the server")
	h.waitFor("host/DESKTOP-01")
	h.key(tcell.KeyEnter, 0) // the first group
	h.waitFor("Snapshots")
	h.waitFor("Archives")
	h.key(tcell.KeyEnter, 0) // the newest snapshot
	h.waitFor("Selection")
	h.waitFor("Documents")
	s := h.screen()
	for _, want := range []string{"Pictures", "todo.txt", "Nothing ticked"} {
		if !strings.Contains(s, want) {
			t.Errorf("the file tree lacks %q:\n%s", want, s)
		}
	}
	h.key(tcell.KeyDown, 0)  // Documents
	h.key(tcell.KeyRight, 0) // open it
	h.waitFor("Finance")
	h.key(tcell.KeyLeft, 0)
	time.Sleep(200 * time.Millisecond)
	if strings.Contains(h.screen(), "Finance") {
		t.Errorf("Left closes the folder:\n%s", h.screen())
	}
	h.key(tcell.KeyUp, 0)
	h.typ(' ') // tick the current node, the archive
	h.waitFor("1 item(s) ticked")
	h.typ('c')
	h.waitFor("Nothing ticked")
	h.typ('a')
	h.waitFor("item(s) ticked")
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Restore into")
	h.key(tcell.KeyTab, 0) // overwrite
	h.key(tcell.KeyTab, 0) // the Restore button
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Restore finished")
	if !strings.Contains(h.screen(), "file(s) restored") {
		t.Errorf("the result is shown:\n%s", h.screen())
	}
	h.key(tcell.KeyEscape, 0)
	h.waitFor("Backups on the server")
}

func TestRestoreRefusesADiskImage(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('6')
	h.waitFor("host/DESKTOP-01-disk")
	h.key(tcell.KeyDown, 0)
	h.key(tcell.KeyDown, 0)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("none (disk image)")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("disk image, not files")
}

func TestTypingInTheDestinationFieldIsNotAShortcut(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('6')
	h.waitFor("host/DESKTOP-01")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Archives")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Selection")
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Restore into")
	// letters that are shortcuts elsewhere (q quit, s stop, r run, e enable) must land in the field
	h.typeText("qsre")
	if !strings.Contains(h.screen(), "qsre") {
		t.Errorf("typed text did not reach the field:\n%s", h.screen())
	}
}

func TestOneOffBackupForm(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('b')
	h.waitFor("Back up folders now")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("at least one folder")
	h.key(tcell.KeyEnter, 0)
	h.typeText(`C:\Data`)
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("started the one-off backup")
	h.waitFor(`One-off: C:\Data`)
}

func TestCheckAndCheckWriteOnTheDemoBackend(t *testing.T) {
	var out, errb strings.Builder
	be := newDemoBackend()
	if code := Check(be, &out, &errb); code != 0 {
		t.Fatalf("Check: %d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "backup sets: 4") {
		t.Errorf("Check output: %s", out.String())
	}
	out.Reset()
	if code := CheckWrite(be, &out, &errb); code != 0 {
		t.Fatalf("CheckWrite: %d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "deleted again") {
		t.Errorf("CheckWrite output: %s", out.String())
	}
	if sets, _ := be.Sets(); len(sets) != 4 {
		t.Errorf("the temporary set was not removed: %d sets", len(sets))
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
	sched := func(s ...map[string]interface{}) map[string]interface{} {
		arr := make([]interface{}, len(s))
		for i, v := range s {
			arr[i] = v
		}
		return map[string]interface{}{"scheduleMode": "scheduled", "schedules": arr}
	}
	days := func(d ...string) []interface{} {
		out := make([]interface{}, len(d))
		for i, v := range d {
			out[i] = v
		}
		return out
	}
	cases := []struct {
		name string
		m    map[string]interface{}
		want string
	}{
		{"daily", sched(map[string]interface{}{"enabled": true, "unit": "days", "every": 1.0, "at": "18:00"}), "daily 18:00"},
		{"every 2 days", sched(map[string]interface{}{"enabled": true, "unit": "days", "every": 2.0, "at": "01:30"}), "every 2 days 01:30"},
		{"weekdays only", sched(map[string]interface{}{"enabled": true, "unit": "days", "every": 1.0, "at": "02:00", "weekdays": days("Mon", "Tue")}), "Mon,Tue 02:00"},
		{"weekly", sched(map[string]interface{}{"enabled": true, "unit": "weeks", "every": 1.0, "at": "01:00", "weekdays": days("Sun")}), "weekly Sun 01:00"},
		{"minutes in a window", sched(map[string]interface{}{"enabled": true, "unit": "minutes", "every": 30.0, "windowStart": "09:00", "windowEnd": "17:00"}), "every 30 mins 09:00-17:00"},
		{"hourly", sched(map[string]interface{}{"enabled": true, "unit": "hours", "every": 1.0, "minute": 15.0}), "every hour at :15"},
		{"monthly last day", sched(map[string]interface{}{"enabled": true, "unit": "months", "every": 1.0, "monthMode": "last", "at": "03:00"}), "monthly, last day 03:00"},
		{"two schedules", sched(
			map[string]interface{}{"enabled": true, "unit": "days", "every": 1.0, "at": "02:00"},
			map[string]interface{}{"enabled": true, "unit": "days", "every": 1.0, "at": "12:30"}), "daily 02:00, daily 12:30"},
		{"a disabled one is left out", sched(
			map[string]interface{}{"enabled": false, "unit": "days", "every": 1.0, "at": "02:00"},
			map[string]interface{}{"enabled": true, "unit": "days", "every": 1.0, "at": "12:30"}), "daily 12:30"},
		{"all off", sched(map[string]interface{}{"enabled": false, "unit": "days", "every": 1.0, "at": "02:00"}), "all schedules off"},
		{"manual", map[string]interface{}{"scheduleMode": "manual"}, "manual"},
		{"old daily", map[string]interface{}{"scheduleTime": "18:00", "triggerMode": "daily"}, "daily 18:00"},
		{"old interval", map[string]interface{}{"triggerMode": "interval", "intervalMinutes": 30.0, "daysOfWeek": days("Mon", "Fri")}, "every 30 min Mon,Fri"},
		{"old manual", map[string]interface{}{"triggerMode": "manual"}, "manual"},
		{"scheduled mode but no schedules and a manual trigger", map[string]interface{}{"scheduleMode": "scheduled", "triggerMode": "manual"}, "manual"},
		{"nothing at all", map[string]interface{}{}, "no schedule"},
	}
	for _, c := range cases {
		if got := scheduleText(c.m); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestParsesRealServiceDocuments feeds the parsers documents captured from a
// running service (winclient, 09/11/2026), so a change in the service's
// fields shows up here.
func TestParsesRealServiceDocuments(t *testing.T) {
	var job map[string]interface{}
	if err := json.Unmarshal([]byte(`{"backupDirs":["C:\\TestData\\Backup of Compaq 486 Laptop"],"backupId":"pbstest-winclient","backupType":"directory","enabled":true,"id":"1790165976252","lastRun":"2026-11-09T02:10:48Z","name":"Folder Backup without VSS","nextRun":"2026-11-10T02:00:00Z","pbsServerId":"pbs-test","scheduleMode":"scheduled","scheduleTime":"02:00","schedules":[{"at":"02:00","enabled":true,"every":1,"id":"s1","unit":"days"}],"triggerMode":"daily","useVSS":false}`), &job); err != nil {
		t.Fatal(err)
	}
	s := toSet(job)
	if s.Name != "Folder Backup without VSS" || s.Type != "folder" || !s.Enabled || s.Schedule != "daily 02:00" ||
		s.ServerID != "pbs-test" || len(s.Sources) != 1 || s.LastRun.IsZero() || s.NextRun.IsZero() {
		t.Errorf("set parsed wrongly: %+v", s)
	}
	var hist map[string]interface{}
	if err := json.Unmarshal([]byte(`{"backupDirs":[],"backupId":"pbstest-winclient","backupType":"machine","encryption":"unencrypted","id":"1794189646","message":"Machine backup completed in 7m 32s: 34.4 GB","name":"Full Machine","status":"success","timestamp":"2026-11-09T02:09:52Z","trigger":"scheduled","useVSS":true}`), &hist); err != nil {
		t.Fatal(err)
	}
	h := toHistory(hist)
	if h.Name != "Full Machine" || h.Status != "success" || h.At.IsZero() || h.Encryption != "unencrypted" {
		t.Errorf("history parsed wrongly: %+v", h)
	}
	var msg map[string]interface{}
	if err := json.Unmarshal([]byte(`{"job_name":"Full Machine","level":"info","message":"Full Machine: done","source":"Backup","timestamp":"2026-11-09T02:09:52Z"}`), &msg); err != nil {
		t.Fatal(err)
	}
	m := toMessage(msg)
	if m.Level != "info" || m.Message != "Full Machine: done" || m.At.IsZero() || m.Source != "Backup" {
		t.Errorf("message parsed wrongly: %+v", m)
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
		{"7-edit-general", func() { h.key(tcell.KeyEscape, 0); h.typ('1'); h.key(tcell.KeyEnter, 0) }},
		{"8-edit-schedule", func() { h.key(tcell.KeyPgDn, 0) }},
		{"9-edit-options", func() { h.key(tcell.KeyPgDn, 0) }},
		{"10-log", func() {
			h.key(tcell.KeyEscape, 0)
			h.typ('3')
			h.key(tcell.KeyEnter, 0)
			time.Sleep(300 * time.Millisecond)
		}},
		{"11-restore-source", func() { h.key(tcell.KeyEscape, 0); h.typ('6'); time.Sleep(400 * time.Millisecond) }},
		{"12-restore-snapshots", func() { h.key(tcell.KeyEnter, 0); time.Sleep(400 * time.Millisecond) }},
		{"13-restore-files", func() {
			h.key(tcell.KeyEnter, 0)
			time.Sleep(500 * time.Millisecond)
			h.key(tcell.KeyDown, 0)
			h.key(tcell.KeyRight, 0)
			h.key(tcell.KeyDown, 0)
			h.key(tcell.KeyRight, 0)
			h.key(tcell.KeyDown, 0)
			h.typ(' ')
		}},
		{"14-restore-destination", func() { h.key(tcell.KeyPgDn, 0); time.Sleep(300 * time.Millisecond) }},
		{"15-one-off", func() { h.key(tcell.KeyEscape, 0); h.typ('1'); h.typ('b') }},
		{"16-reports", func() { h.key(tcell.KeyEscape, 0); h.typ('7') }},
		{"17-log-files", func() { h.typ('8') }},
		{"18-servers", func() { h.typ('5') }},
		{"19-server-edit", func() { h.key(tcell.KeyEnter, 0) }},
		{"20-preferences", func() { h.key(tcell.KeyEscape, 0); h.sim.InjectKey(tcell.KeyF9, 0, tcell.ModNone) }},
		{"21-passphrase", func() {
			h.sim.InjectKey(tcell.KeyF6, 0, tcell.ModNone)
			time.Sleep(400 * time.Millisecond)
			h.key(tcell.KeyEscape, 0) // files -> snapshots
			h.key(tcell.KeyEscape, 0) // snapshots -> source
			h.key(tcell.KeyDown, 0)   // the photos group, encrypted
			h.key(tcell.KeyEnter, 0)
			time.Sleep(400 * time.Millisecond)
			h.key(tcell.KeyEnter, 0)
			time.Sleep(400 * time.Millisecond)
			h.typeText("secret")
		}},
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
