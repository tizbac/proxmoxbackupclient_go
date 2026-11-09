package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/salvanza/salvanza_client/gui/api"
)

// The palette follows the desktop app's blue.
var (
	colBg      = tcell.NewRGBColor(18, 23, 30)
	colPanel   = tcell.NewRGBColor(24, 31, 40)
	colBorder  = tcell.NewRGBColor(58, 74, 92)
	colAccent  = tcell.NewRGBColor(15, 98, 168)
	colSel     = tcell.NewRGBColor(38, 79, 120)
	colText    = tcell.NewRGBColor(214, 221, 230)
	colDim     = tcell.NewRGBColor(111, 130, 150)
	colHead    = tcell.NewRGBColor(108, 195, 255)
	colKey     = tcell.NewRGBColor(255, 209, 102)
	colOK      = tcell.NewRGBColor(91, 214, 138)
	colWarn    = tcell.NewRGBColor(255, 180, 84)
	colErr     = tcell.NewRGBColor(255, 107, 107)
	tabNames   = []string{"Dashboard", "Backup Sets", "History", "Messages", "Servers", "Restore", "Reports", "Logs", "Preferences"}
	tabShort   = []string{"Dash", "Sets", "Hist", "Msgs", "Servers", "Restore", "Reports", "Logs", "Prefs"}
	tabPages   = []string{"dash", "sets", "hist", "msgs", "srv", "rest", "rep", "logs", "prefs"}
	refreshRun = 2 * time.Second
	refreshAll = 10 * time.Second
)

// App is the text-mode window.
type App struct {
	app *tview.Application
	be  Backend

	pages  *tview.Pages
	header *tview.TextView
	tabs   *tview.TextView
	status *tview.TextView
	footer *tview.TextView

	dashSets   *tview.Table
	dashRuns   *tview.TextView
	dashRecent *tview.TextView
	setsTable  *tview.Table
	setDetail  *tview.TextView
	histTable  *tview.Table
	histDetail *tview.TextView
	msgsTable  *tview.Table
	srvTable   *tview.Table
	srvDetail  *tview.TextView
	rs         *restoreView
	pf         *prefsView
	keyCache   map[string]KeyInfo
	repTable   *tview.Table
	repFail    *tview.TextView
	repPeriod  int
	logsTable  *tview.Table
	logFiles   []LogFile
	logErr     error

	mu      sync.Mutex
	svc     *api.StatusResponse
	svcErr  error
	sets    []Set
	runs    []api.BackupProgress
	hist    []HistoryEntry
	msgs    []MessageEntry
	servers []Server
	tab     int
	modal   bool
	flash   string
	flashAt time.Time
	version string
}

// NewApp builds the window over a backend. Call Run to show it.
func NewApp(be Backend, version string) *App {
	a := &App{app: tview.NewApplication(), be: be, version: version}
	a.build()
	return a
}

func applyStyles() {
	tview.Styles.PrimitiveBackgroundColor = colBg
	tview.Styles.ContrastBackgroundColor = colPanel
	tview.Styles.MoreContrastBackgroundColor = colSel
	tview.Styles.BorderColor = colBorder
	tview.Styles.TitleColor = colHead
	tview.Styles.GraphicsColor = colBorder
	tview.Styles.PrimaryTextColor = colText
	tview.Styles.SecondaryTextColor = colDim
	tview.Styles.TertiaryTextColor = colHead
	tview.Styles.InverseTextColor = colBg
	tview.Styles.ContrastSecondaryTextColor = colDim
}

func tag(c tcell.Color) string { return fmt.Sprintf("[#%06x]", c.Hex()) }

func (a *App) build() {
	applyStyles()
	a.header = tview.NewTextView().SetDynamicColors(true)
	a.header.SetBackgroundColor(colAccent)
	a.tabs = tview.NewTextView().SetDynamicColors(true)
	a.status = tview.NewTextView().SetDynamicColors(true)
	a.footer = tview.NewTextView().SetDynamicColors(true)
	a.footer.SetBackgroundColor(colPanel)

	a.pages = tview.NewPages()
	a.pages.AddPage("dash", a.buildDashboard(), true, true)
	a.pages.AddPage("sets", a.buildSets(), true, false)
	a.pages.AddPage("hist", a.buildHistory(), true, false)
	a.pages.AddPage("msgs", a.buildMessages(), true, false)
	a.pages.AddPage("srv", a.buildServers(), true, false)
	a.pages.AddPage("rest", a.buildRestore(), true, false)
	a.pages.AddPage("rep", a.buildReports(), true, false)
	a.pages.AddPage("logs", a.buildLogs(), true, false)
	a.pages.AddPage("prefs", a.buildPrefs(), true, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 1, 0, false).
		AddItem(a.tabs, 1, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.status, 1, 0, false).
		AddItem(a.footer, 1, 0, false)
	a.app.SetRoot(root, true).EnableMouse(true)
	a.app.SetInputCapture(a.keys)
	a.switchTab(0)
}

func newTable() *tview.Table {
	t := tview.NewTable().SetSelectable(true, false).SetFixed(1, 0).SetSeparator(' ')
	t.SetSelectedStyle(tcell.StyleDefault.Background(colSel).Foreground(tcell.ColorWhite))
	t.SetBorder(true)
	return t
}

func header(t *tview.Table, cols ...string) {
	for i, c := range cols {
		t.SetCell(0, i, tview.NewTableCell(" "+c).SetTextColor(colHead).SetSelectable(false).SetAttributes(tcell.AttrBold).SetExpansion(1))
	}
}

func cell(text string, c tcell.Color) *tview.TableCell {
	return tview.NewTableCell(" " + text).SetTextColor(c).SetExpansion(1)
}

func textBox(title string) *tview.TextView {
	t := tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetWordWrap(true)
	t.SetBorder(true).SetTitle(" " + title + " ").SetTitleAlign(tview.AlignLeft)
	return t
}

// ---- layout ---------------------------------------------------------------

func (a *App) buildDashboard() tview.Primitive {
	a.dashSets = newTable()
	a.dashSets.SetTitle(" Backup Sets ").SetTitleAlign(tview.AlignLeft)
	a.dashRuns = textBox("Running now")
	a.dashRecent = textBox("Recent")
	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.dashSets, 0, 3, true).
		AddItem(a.dashRuns, 9, 0, false).
		AddItem(a.dashRecent, 7, 0, false)
}

func (a *App) buildSets() tview.Primitive {
	a.setsTable = newTable()
	a.setsTable.SetTitle(" Backup Sets ").SetTitleAlign(tview.AlignLeft)
	a.setDetail = textBox("Details")
	a.setsTable.SetSelectionChangedFunc(func(int, int) { a.renderSetDetail() })
	a.setsTable.SetSelectedFunc(func(int, int) { a.editSet() })
	a.dashSets.SetSelectedFunc(func(int, int) { a.editSet() })
	return tview.NewFlex().
		AddItem(a.setsTable, 0, 3, true).
		AddItem(a.setDetail, 0, 2, false)
}

func (a *App) buildHistory() tview.Primitive {
	a.histTable = newTable()
	a.histTable.SetTitle(" History ").SetTitleAlign(tview.AlignLeft)
	a.histDetail = textBox("Run")
	a.histTable.SetSelectionChangedFunc(func(int, int) { a.renderHistDetail() })
	a.histTable.SetSelectedFunc(func(int, int) { a.openLog() })
	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.histTable, 0, 3, true).
		AddItem(a.histDetail, 7, 0, false)
}

func (a *App) buildMessages() tview.Primitive {
	a.msgsTable = newTable()
	a.msgsTable.SetTitle(" Message Log ").SetTitleAlign(tview.AlignLeft)
	return a.msgsTable
}

func (a *App) buildServers() tview.Primitive {
	a.srvTable = newTable()
	a.srvTable.SetTitle(" PBS servers ").SetTitleAlign(tview.AlignLeft)
	a.srvDetail = textBox("Server")
	a.srvTable.SetSelectionChangedFunc(func(int, int) { a.renderSrvDetail() })
	a.srvTable.SetSelectedFunc(func(int, int) { a.editServer() })
	return tview.NewFlex().
		AddItem(a.srvTable, 0, 3, true).
		AddItem(a.srvDetail, 0, 2, false)
}

// ---- keys ------------------------------------------------------------------

// typingNow says whether a text field has the keyboard: the destination step
// of Restore and the Preferences forms. Characters then belong to the field.
func (a *App) typingNow() bool {
	return (a.tab == 5 && a.rs != nil && a.rs.typing()) || a.tab == 8
}

// ownsTab says whether Tab belongs to the screen (moving between its fields)
// instead of switching screens; the function keys and 1 to 9 still switch.
func (a *App) ownsTab() bool {
	return (a.tab == 5 && a.rs != nil && (a.rs.step == 0 || a.rs.step == 3)) || a.tab == 8
}

func (a *App) keys(ev *tcell.EventKey) *tcell.EventKey {
	if a.modal {
		return ev
	}
	switch ev.Key() {
	case tcell.KeyTab:
		if a.ownsTab() {
			return ev
		}
		a.switchTab((a.tab + 1) % len(tabPages))
		return nil
	case tcell.KeyBacktab:
		if a.ownsTab() {
			return ev
		}
		a.switchTab((a.tab + len(tabPages) - 1) % len(tabPages))
		return nil
	case tcell.KeyF1, tcell.KeyF2, tcell.KeyF3, tcell.KeyF4, tcell.KeyF5, tcell.KeyF6, tcell.KeyF7, tcell.KeyF8, tcell.KeyF9:
		a.switchTab(int(ev.Key() - tcell.KeyF1))
		return nil
	case tcell.KeyCtrlC:
		a.app.Stop()
		return nil
	case tcell.KeyEscape:
		return ev
	}
	if ev.Key() != tcell.KeyRune || a.typingNow() {
		return ev
	}
	sets := a.tab == 0 || a.tab == 1
	switch ev.Rune() {
	case 'q', 'Q':
		a.app.Stop()
		return nil
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		a.switchTab(int(ev.Rune() - '1'))
		return nil
	case '?':
		a.help()
		return nil
	case 'g', 'R':
		go a.refresh(true)
		if a.tab == 7 {
			a.loadLogFiles()
		}
		a.setFlash("refreshing")
		return nil
	}
	switch {
	case sets:
		switch ev.Rune() {
		case 'r':
			a.actRun()
		case 'p':
			a.actPause()
		case 's':
			a.actStop()
		case 'e':
			a.actToggle()
		case 'n':
			a.newSet()
		case 'd':
			a.actDelete()
		case 'b':
			a.oneOff()
		case 'u':
			a.undeleteSet()
		case 'k':
			a.rollbackSet()
		case 'E':
			a.editSet()
		default:
			return ev
		}
		return nil
	case a.tab == 4:
		switch ev.Rune() {
		case 'n':
			a.newServer()
		case 'd':
			a.deleteServer()
		case 't':
			a.testServer()
		case 'u':
			a.defaultServer()
		case 'f':
			a.pinServer()
		case 'E':
			a.editServer()
		default:
			return ev
		}
		return nil
	case a.tab == 6 && ev.Rune() == 'w':
		a.mu.Lock()
		a.repPeriod = (a.repPeriod + 1) % len(reportPeriods)
		a.mu.Unlock()
		a.renderReports()
		return nil
	}
	return ev
}

func (a *App) switchTab(i int) {
	a.tab = i
	a.pages.SwitchToPage(tabPages[i])
	switch i {
	case 0:
		a.app.SetFocus(a.dashSets)
	case 1:
		a.app.SetFocus(a.setsTable)
	case 2:
		a.app.SetFocus(a.histTable)
	case 3:
		a.app.SetFocus(a.msgsTable)
	case 4:
		a.app.SetFocus(a.srvTable)
	case 5:
		a.rs.enter()
	case 6:
		a.renderReports()
		a.app.SetFocus(a.repTable)
	case 7:
		a.loadLogFiles()
		a.app.SetFocus(a.logsTable)
	case 8:
		a.pf.enter()
	}
	a.renderChrome()
}

// ---- chrome ------------------------------------------------------------------

func (a *App) setFlash(s string) {
	a.mu.Lock()
	a.flash, a.flashAt = s, time.Now()
	a.mu.Unlock()
	a.renderChrome()
}

func (a *App) renderChrome() {
	a.mu.Lock()
	svc, err, flash, flashAt, tab := a.svc, a.svcErr, a.flash, a.flashAt, a.tab
	running := 0
	for _, r := range a.runs {
		if r.Running {
			running++
		}
	}
	a.mu.Unlock()

	state := tag(colDim) + "connecting"
	switch {
	case err != nil:
		state = tag(colErr) + "no service: " + describeErr(err)
	case svc != nil:
		state = tag(colOK) + "service running"
		if running > 0 {
			state += fmt.Sprintf("[white]   %d backup(s) running", running)
		}
		if svc.Version != "" {
			state += tag(colDim) + "   v" + svc.Version
		}
	}
	mode := ""
	if a.be.Name() == "demo" {
		mode = tag(colKey) + "  [DEMO DATA]"
	}
	a.header.SetText(fmt.Sprintf("[white::b] Salvanza Backup%s[-:-:-]   %s", mode, state))
	var b strings.Builder
	names := tabNames
	if _, _, w, _ := a.tabs.GetRect(); w > 0 && w < 105 {
		names = tabShort // the long names do not fit an 80 column terminal
	}
	for i, n := range names {
		if i == tab {
			fmt.Fprintf(&b, "[black:#%06x] %d %s [-:-] ", colHead.Hex(), i+1, n)
		} else {
			fmt.Fprintf(&b, "%s %d %s [-:-] ", tag(colDim), i+1, n)
		}
	}
	a.tabs.SetText(b.String())

	if flash != "" && time.Since(flashAt) < 6*time.Second {
		a.status.SetText(" " + tag(colWarn) + flash)
	} else {
		a.status.SetText("")
	}
	hint := func(k, d string) string { return tag(colKey) + "[::b]" + k + "[-:-:-]" + tag(colDim) + " " + d + "  " }
	f := " "
	switch tab {
	case 0, 1:
		f += hint("r", "run now") + hint("p", "pause/resume") + hint("s", "stop") + hint("e", "enable/disable") +
			hint("Enter", "edit") + hint("n", "new") + hint("d", "delete") + hint("b", "one-off backup") +
			hint("u", "undelete") + hint("k", "roll back")
	}
	if tab == 2 {
		f += hint("Enter", "open the run's log")
	}
	switch tab {
	case 4:
		f += hint("Enter", "edit") + hint("n", "new") + hint("d", "delete") + hint("t", "test") + hint("u", "make default") + hint("f", "pin fingerprint")
	case 5:
		f += hint("Enter", "choose") + hint("Space", "tick") + hint("PgDn", "next step") + hint("Esc", "back")
	case 6:
		f += hint("w", "change the period")
	case 7:
		f += hint("Enter", "open the log") + hint("g", "refresh")
	case 8:
		f += hint("Ctrl+S", "save") + hint("PgDn", "next page")
	}
	f += hint("Tab", "next") + hint("?", "help") + hint("q", "quit")
	a.footer.SetText(f)
}

// ---- data ---------------------------------------------------------------------

// Run shows the window and blocks until it is closed.
func (a *App) Run() error {
	go a.loop()
	return a.app.Run()
}

func (a *App) loop() {
	a.refresh(true)
	fast := time.NewTicker(refreshRun)
	slow := time.NewTicker(refreshAll)
	defer fast.Stop()
	defer slow.Stop()
	for {
		select {
		case <-fast.C:
			a.refresh(false)
		case <-slow.C:
			a.refresh(true)
		}
	}
}

// refresh reads from the backend (never on the UI goroutine) and redraws.
func (a *App) refresh(all bool) {
	svc, serr := a.be.Status()
	runs, rerr := a.be.Runs()
	var sets []Set
	var hist []HistoryEntry
	var msgs []MessageEntry
	var servers []Server
	if all && serr == nil {
		sets, _ = a.be.Sets()
		hist, _ = a.be.History()
		msgs, _ = a.be.Messages()
		servers, _ = a.be.Servers()
	}
	a.app.QueueUpdateDraw(func() {
		a.mu.Lock()
		a.svc, a.svcErr = svc, serr
		if serr == nil && rerr == nil {
			a.runs = runs
		}
		if all && serr == nil {
			a.sets, a.hist, a.msgs, a.servers = sets, hist, msgs, servers
		}
		a.mu.Unlock()
		a.renderAll()
	})
}

func (a *App) renderAll() {
	a.renderChrome()
	a.renderDashSets()
	a.renderRuns()
	a.renderRecent()
	a.renderSets()
	a.renderHist()
	a.renderMsgs()
	a.renderServers()
	a.renderReports()
}

// ---- helpers --------------------------------------------------------------------

func when(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	now := time.Now()
	d0 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	days := int(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Sub(d0).Hours() / 24)
	hm := t.Format("15:04")
	switch {
	case days == 0:
		return "today " + hm
	case days == 1:
		return "tomorrow " + hm
	case days == -1:
		return "yesterday " + hm
	case days > 1 && days < 7:
		return t.Format("Mon") + " " + hm
	}
	return t.Format("2 Jan") + " " + hm
}

func bar(pct float64, width int, c tcell.Color) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	n := int(pct / 100 * float64(width))
	return tag(c) + strings.Repeat("█", n) + tag(colBorder) + strings.Repeat("░", width-n) + "[-]"
}

func human(n float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for n >= 1000 && i < len(units)-1 {
		n /= 1000
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", n, units[i])
	}
	return fmt.Sprintf("%.1f %s", n, units[i])
}

func statusColour(s string) tcell.Color {
	switch strings.ToLower(s) {
	case "success", "ok":
		return colOK
	case "failed", "error":
		return colErr
	case "partial", "warning", "cancelled", "canceled":
		return colWarn
	}
	return colDim
}

func statusWord(s string) string {
	switch strings.ToLower(s) {
	case "success":
		return "OK"
	case "failed":
		return "Failed"
	case "":
		return "-"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// lastResult is the most recent history entry of a set, for the Result column.
func (a *App) lastResult(set Set) (HistoryEntry, bool) {
	for _, h := range a.hist { // newest first
		if h.Name == set.Name {
			return h, true
		}
	}
	return HistoryEntry{}, false
}

func (a *App) runOf(set Set) (api.BackupProgress, bool) {
	for _, r := range a.runs {
		if r.Running && r.Name == set.Name {
			return r, true
		}
	}
	return api.BackupProgress{}, false
}

// ---- dashboard + sets -------------------------------------------------------------

func (a *App) fillSets(t *tview.Table) {
	keep, _ := t.GetSelection()
	t.Clear()
	header(t, "Name", "Type", "Schedule", "Next run", "Last run", "Result")
	a.mu.Lock()
	n := len(a.sets)
	for i, s := range a.sets {
		name := s.Name
		nameCol := colText
		if !s.Enabled {
			nameCol = colDim
			name += "  (off)"
		}
		next := when(s.NextRun)
		if s.Manual {
			next = "manual"
		}
		if !s.Enabled {
			next = "disabled"
		}
		res, resCol := "-", colDim
		if r, ok := a.runOf(s); ok {
			res, resCol = fmt.Sprintf("running %.0f%%", r.Progress), colHead
			if r.Paused {
				res = "paused"
				resCol = colWarn
			}
		} else if h, ok := a.lastResult(s); ok {
			res, resCol = statusWord(h.Status), statusColour(h.Status)
		}
		t.SetCell(i+1, 0, cell(name, nameCol))
		t.SetCell(i+1, 1, cell(s.Type, colDim))
		t.SetCell(i+1, 2, cell(s.Schedule, colText))
		t.SetCell(i+1, 3, cell(next, colText))
		t.SetCell(i+1, 4, cell(when(s.LastRun), colDim))
		t.SetCell(i+1, 5, cell(res, resCol))
	}
	a.mu.Unlock()
	// Select calls the selection callbacks, which take the lock themselves.
	if keep < 1 {
		keep = 1
	}
	if keep > n {
		keep = n
	}
	if n > 0 {
		t.Select(keep, 0)
	}
}

func (a *App) renderDashSets() { a.fillSets(a.dashSets) }

func (a *App) renderSets() {
	a.fillSets(a.setsTable)
	a.renderSetDetail()
}

func (a *App) selectedSet() (Set, bool) {
	t := a.dashSets
	if a.tab == 1 {
		t = a.setsTable
	}
	row, _ := t.GetSelection()
	a.mu.Lock()
	defer a.mu.Unlock()
	if row < 1 || row > len(a.sets) {
		return Set{}, false
	}
	return a.sets[row-1], true
}

func (a *App) renderSetDetail() {
	s, ok := a.selectedSet()
	a.setDetail.Clear()
	if !ok {
		a.setDetail.SetText(tag(colDim) + "No Backup Sets.")
		return
	}
	var b strings.Builder
	line := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, "%s%-11s[-] %s\n", tag(colDim), k, v)
		}
	}
	line("Name", "[white::b]"+s.Name+"[-:-:-]")
	line("Type", s.Type)
	line("Backup ID", s.BackupID)
	line("Server", s.ServerID)
	state := tag(colOK) + "enabled"
	if !s.Enabled {
		state = tag(colDim) + "disabled"
	}
	line("State", state)
	line("Schedule", s.Schedule)
	line("Next run", when(s.NextRun))
	line("Last run", when(s.LastRun))
	line("Encryption", a.setEncryption(s))
	if len(s.Sources) > 0 {
		b.WriteString(tag(colDim) + "Sources[-]\n")
		for _, src := range s.Sources {
			b.WriteString("  " + tview.Escape(src) + "\n")
		}
	}
	a.setDetail.SetText(b.String())
}

// ---- running + recent ----------------------------------------------------------------

func (a *App) renderRuns() {
	a.mu.Lock()
	runs := append([]api.BackupProgress(nil), a.runs...)
	a.mu.Unlock()
	var b strings.Builder
	shown := 0
	for _, r := range runs {
		if !r.Running && !r.Complete {
			continue
		}
		shown++
		name := r.Name
		if name == "" {
			name = r.JobID
		}
		switch {
		case r.Running && r.Paused:
			fmt.Fprintf(&b, " [white::b]%-16s[-:-:-] %s %s%3.0f%%  paused\n", name, bar(r.Progress, 30, colWarn), tag(colWarn), r.Progress)
		case r.Running:
			speed := ""
			if r.Stats != nil {
				if s := num(r.Stats, "sentBytes"); s > 0 && !parseTime(r.StartTime).IsZero() {
					el := time.Since(parseTime(r.StartTime)).Seconds()
					if el > 1 {
						speed = fmt.Sprintf("   %s/s sent", human(s/el))
					}
				}
			}
			fmt.Fprintf(&b, " [white::b]%-16s[-:-:-] %s %s%3.0f%%%s%s\n", name, bar(r.Progress, 30, colHead), tag(colHead), r.Progress, tag(colDim), speed)
			detail := r.Message
			if r.Stats != nil {
				if fd, ft := num(r.Stats, "filesDone"), num(r.Stats, "filesTotal"); fd > 0 {
					detail = fmt.Sprintf("%.0f of %.0f files", fd, ft)
					if ft == 0 {
						detail = fmt.Sprintf("%.0f files", fd)
					}
				}
				if d := num(r.Stats, "bytesDone"); d > 0 {
					detail += "   " + human(d)
					if t := num(r.Stats, "bytesTotal"); t > 0 {
						detail += " of " + human(t)
					}
				}
				if dir, _ := r.Stats["currentDir"].(string); dir != "" {
					detail += "   " + dir
				}
			}
			if r.File != "" {
				detail = r.File
			}
			fmt.Fprintf(&b, "   %s%s[-]\n", tag(colDim), tview.Escape(detail))
		case r.Complete && r.Success:
			fmt.Fprintf(&b, " [white::b]%-16s[-:-:-] %sfinished OK[-]  %s%s[-]\n", name, tag(colOK), tag(colDim), tview.Escape(r.Message))
		default:
			fmt.Fprintf(&b, " [white::b]%-16s[-:-:-] %sfailed[-]  %s%s[-]\n", name, tag(colErr), tag(colDim), tview.Escape(firstNonEmpty(r.Error, r.Message)))
		}
	}
	if shown == 0 {
		b.WriteString(" " + tag(colDim) + "Nothing is running.")
	}
	a.dashRuns.SetText(b.String())
}

func (a *App) renderRecent() {
	a.mu.Lock()
	hist := a.hist
	a.mu.Unlock()
	var b strings.Builder
	for i, h := range hist {
		if i >= 5 {
			break
		}
		fmt.Fprintf(&b, " %s%-17s[-] [white]%-16s[-] %s%-7s[-] %s%s[-]\n", tag(colDim), when(h.At), h.Name, tag(statusColour(h.Status)), statusWord(h.Status), tag(colDim), tview.Escape(h.Message))
	}
	if b.Len() == 0 {
		b.WriteString(" " + tag(colDim) + "No runs recorded yet.")
	}
	a.dashRecent.SetText(b.String())
}

// ---- history, messages, servers ---------------------------------------------------

func (a *App) renderHist() {
	keep, _ := a.histTable.GetSelection()
	a.histTable.Clear()
	header(a.histTable, "When", "Set", "Result", "Message")
	a.mu.Lock()
	for i, h := range a.hist {
		a.histTable.SetCell(i+1, 0, cell(when(h.At), colDim))
		a.histTable.SetCell(i+1, 1, cell(h.Name, colText))
		a.histTable.SetCell(i+1, 2, cell(statusWord(h.Status), statusColour(h.Status)))
		a.histTable.SetCell(i+1, 3, cell(h.Message, colText))
	}
	n := len(a.hist)
	a.mu.Unlock()
	if n > 0 {
		if keep < 1 || keep > n {
			keep = 1
		}
		a.histTable.Select(keep, 0)
	}
	a.renderHistDetail()
}

func (a *App) renderHistDetail() {
	row, _ := a.histTable.GetSelection()
	a.mu.Lock()
	defer a.mu.Unlock()
	if row < 1 || row > len(a.hist) {
		a.histDetail.SetText("")
		return
	}
	h := a.hist[row-1]
	var b strings.Builder
	fmt.Fprintf(&b, "%sWhen[-]       %s\n%sSet[-]        [white::b]%s[-:-:-]   %s%s[-]\n", tag(colDim), h.At.Format("Mon 2 Jan 2006 15:04"), tag(colDim), h.Name, tag(colDim), h.BackupID)
	fmt.Fprintf(&b, "%sResult[-]     %s%s[-]\n", tag(colDim), tag(statusColour(h.Status)), statusWord(h.Status))
	if h.Encryption != "" {
		fmt.Fprintf(&b, "%sEncryption[-] %s\n", tag(colDim), h.Encryption)
	}
	fmt.Fprintf(&b, "%sMessage[-]    %s", tag(colDim), tview.Escape(h.Message))
	a.histDetail.SetText(b.String())
}

func (a *App) renderMsgs() {
	keep, _ := a.msgsTable.GetSelection()
	a.msgsTable.Clear()
	header(a.msgsTable, "When", "Level", "Message")
	a.mu.Lock()
	n := len(a.msgs)
	for i, m := range a.msgs {
		c := statusColour(m.Level)
		if strings.EqualFold(m.Level, "info") {
			c = colDim
		}
		a.msgsTable.SetCell(i+1, 0, cell(when(m.At), colDim))
		a.msgsTable.SetCell(i+1, 1, cell(strings.ToUpper(m.Level), c))
		a.msgsTable.SetCell(i+1, 2, cell(m.Message, colText))
	}
	a.mu.Unlock()
	if n > 0 {
		if keep < 1 || keep > n {
			keep = 1
		}
		a.msgsTable.Select(keep, 0)
	}
}

func (a *App) renderServers() {
	keep, _ := a.srvTable.GetSelection()
	a.srvTable.Clear()
	header(a.srvTable, "Name", "Address", "Datastore", "Login")
	a.mu.Lock()
	n := len(a.servers)
	for i, s := range a.servers {
		name := s.Name
		if s.Default {
			name += "  (default)"
		}
		a.srvTable.SetCell(i+1, 0, cell(name, colText))
		a.srvTable.SetCell(i+1, 1, cell(s.URL, colText))
		a.srvTable.SetCell(i+1, 2, cell(s.Datastore, colText))
		a.srvTable.SetCell(i+1, 3, cell(s.Login, colDim))
	}
	a.mu.Unlock()
	if n > 0 {
		if keep < 1 || keep > n {
			keep = 1
		}
		a.srvTable.Select(keep, 0)
	}
	a.renderSrvDetail()
}

func (a *App) renderSrvDetail() {
	row, _ := a.srvTable.GetSelection()
	a.mu.Lock()
	defer a.mu.Unlock()
	if row < 1 || row > len(a.servers) {
		a.srvDetail.SetText(tag(colDim) + "No servers.")
		return
	}
	s := a.servers[row-1]
	enc := "no"
	if s.KeyFile != "" {
		enc = tag(colOK) + "yes"
	} else if s.Encrypted {
		enc = tag(colOK) + "yes, with a key"
	}
	var b strings.Builder
	line := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, "%s%-12s[-] %s\n", tag(colDim), k, v)
		}
	}
	line("Name", "[white::b]"+s.Name+"[-:-:-]")
	line("Address", s.URL)
	line("Datastore", s.Datastore)
	line("Namespace", s.Namespace)
	line("Login", s.Login)
	line("Encrypted", enc)
	line("Fingerprint", s.Fingerprint)
	if s.KeyFile != "" {
		b.WriteString("\n")
		line("Key file", tview.Escape(s.KeyFile))
		pass := "no, asked for when needed"
		if s.PassphraseStored {
			pass = "yes"
		}
		if k, ok := a.keyCache[s.KeyFile]; ok {
			switch {
			case !k.Exists || !k.Usable:
				line("Key", tag(colErr)+tview.Escape(firstNonEmpty(k.Reason, "not usable")))
			default:
				line("Key", "fingerprint "+k.Fingerprint)
				if k.PassphraseProtected {
					line("Passphrase", "the key is protected; stored: "+pass)
				} else {
					line("Passphrase", "the key has none")
				}
				if k.OpenToOthers {
					line("Access", tag(colWarn)+"other accounts can read the key file (restrict it in the editor)")
				}
			}
		} else {
			line("Passphrase stored", pass)
			path := s.KeyFile
			go a.loadKeyInfo(path)
		}
	}
	a.srvDetail.SetText(b.String())
}

// ---- actions --------------------------------------------------------------------------

func (a *App) actRun() {
	s, ok := a.selectedSet()
	if !ok || (a.tab != 0 && a.tab != 1) {
		return
	}
	go func() {
		err := a.be.RunSet(s.ID)
		msg := "started " + s.Name
		if err != nil {
			msg = describeErr(err)
		}
		a.app.QueueUpdateDraw(func() { a.setFlash(msg) })
		a.refresh(false)
	}()
}

// activeRun is the running backup the pause and stop keys act on: the selected
// set's when it is running, otherwise the only one running.
func (a *App) activeRun() (api.BackupProgress, bool) {
	s, ok := a.selectedSet()
	a.mu.Lock()
	defer a.mu.Unlock()
	var running []api.BackupProgress
	for _, r := range a.runs {
		if r.Running {
			if ok && r.Name == s.Name {
				return r, true
			}
			running = append(running, r)
		}
	}
	if len(running) == 1 {
		return running[0], true
	}
	return api.BackupProgress{}, false
}

func (a *App) actPause() {
	r, ok := a.activeRun()
	if !ok {
		a.setFlash("no backup is running")
		return
	}
	action := "pause"
	if r.Paused {
		action = "resume"
	}
	go func() {
		err := a.be.Control(action, r.JobID)
		msg := action + "d " + r.Name
		if action == "pause" {
			msg = "paused " + r.Name
		}
		if err != nil {
			msg = describeErr(err)
		}
		a.app.QueueUpdateDraw(func() { a.setFlash(msg) })
		a.refresh(false)
	}()
}

func (a *App) actStop() {
	r, ok := a.activeRun()
	if !ok {
		a.setFlash("no backup is running")
		return
	}
	a.confirm(fmt.Sprintf("Stop the running backup of %s?\nNo snapshot is kept.", r.Name), func() {
		go func() {
			err := a.be.Control("stop", r.JobID)
			msg := "stopped " + r.Name
			if err != nil {
				msg = describeErr(err)
			}
			a.app.QueueUpdateDraw(func() { a.setFlash(msg) })
			a.refresh(false)
		}()
	})
}

func (a *App) actToggle() {
	s, ok := a.selectedSet()
	if !ok || (a.tab != 0 && a.tab != 1) {
		return
	}
	go func() {
		err := a.be.SetEnabled(s.ID, !s.Enabled)
		word := "enabled"
		if s.Enabled {
			word = "disabled"
		}
		msg := word + " " + s.Name
		if err != nil {
			msg = describeErr(err)
		}
		a.app.QueueUpdateDraw(func() { a.setFlash(msg) })
		a.refresh(true)
	}()
}

// ---- dialogs -------------------------------------------------------------------------

func (a *App) confirm(text string, yes func()) {
	m := tview.NewModal().SetText(text).AddButtons([]string{"Yes", "No"})
	m.SetBackgroundColor(colPanel).SetTextColor(colText)
	m.SetButtonBackgroundColor(colAccent)
	prev := a.modal // a form under the question (the server editor, say) stays open
	m.SetDoneFunc(func(i int, label string) {
		a.pages.RemovePage("modal")
		a.modal = prev
		if prev {
			if _, front := a.pages.GetFrontPage(); front != nil {
				a.app.SetFocus(front)
			}
		} else {
			a.switchTab(a.tab)
		}
		if label == "Yes" {
			yes()
		}
	})
	a.modal = true
	a.pages.AddPage("modal", m, true, true)
	a.app.SetFocus(m)
}

func (a *App) help() {
	t := tview.NewTextView().SetDynamicColors(true)
	t.SetBorder(true).SetTitle(" Keys ").SetBackgroundColor(colPanel)
	k := func(key, d string) string {
		return fmt.Sprintf(" %s[::b]%-12s[-:-:-] %s\n", tag(colKey), key, d)
	}
	t.SetText("\n" +
		k("Tab, 1-9", "switch screen (F1 to F9 too)") +
		k("Up, Down", "move in a list (PgUp, PgDn, Home, End)") +
		k("r", "run the selected Backup Set now") +
		k("p", "pause or resume the running backup") +
		k("s", "stop the running backup (asks first)") +
		k("e", "enable or disable the selected Backup Set") +
		k("Enter", "edit the selected Backup Set (History: open its log)") +
		k("n, d", "new Backup Set, delete the selected one (asks first)") +
		k("b", "back up folders now, without a Backup Set") +
		k("u, k", "Undelete files of the selected set, Roll the set back") +
		k("6", "Restore: browse a backup and restore files") +
		k("Servers", "n new, Enter edit, d delete, t test, u default, f pin") +
		k("7, 8, 9", "Reports (w: period), Log files, Preferences") +
		k("g", "refresh now (it also refreshes every 2 s)") +
		k("q, Ctrl+C", "quit; running backups go on in the service") +
		"\n " + tag(colDim) + "Press Esc to close.")
	t.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEscape || ev.Rune() == 'q' || ev.Rune() == '?' {
			a.pages.RemovePage("help")
			a.modal = false
			a.switchTab(a.tab)
			return nil
		}
		return ev
	})
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).AddItem(nil, 0, 1, false).AddItem(t, 20, 0, true).AddItem(nil, 0, 1, false), 64, 0, true).
		AddItem(nil, 0, 1, false)
	a.modal = true
	a.pages.AddPage("help", flex, true, true)
	a.app.SetFocus(t)
}
