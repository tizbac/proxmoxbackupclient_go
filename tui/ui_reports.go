package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rivo/tview"
)

var reportPeriods = []struct {
	name string
	days int
}{{"all recorded runs", 0}, {"the last 7 days", 7}, {"the last 30 days", 30}}

func (a *App) buildReports() tview.Primitive {
	a.repTable = newTable()
	a.repTable.SetTitle(" Reports ").SetTitleAlign(tview.AlignLeft)
	a.repFail = textBox("Recent failures")
	return tview.NewFlex().SetDirection(tview.FlexRow).AddItem(a.repTable, 0, 3, true).AddItem(a.repFail, 10, 0, false)
}

type setStats struct {
	Name           string
	Runs, OK, Fail int
	Last           HistoryEntry
}

// reportStats groups the history by Backup Set for the period.
func reportStats(hist []HistoryEntry, days int, now time.Time) ([]setStats, []HistoryEntry) {
	by := map[string]*setStats{}
	var failures []HistoryEntry
	for _, h := range hist {
		if days > 0 && h.At.Before(now.AddDate(0, 0, -days)) {
			continue
		}
		s := by[h.Name]
		if s == nil {
			s = &setStats{Name: h.Name}
			by[h.Name] = s
		}
		s.Runs++
		switch strings.ToLower(h.Status) {
		case "success":
			s.OK++
		case "failed":
			s.Fail++
			failures = append(failures, h)
		}
		if h.At.After(s.Last.At) {
			s.Last = h
		}
	}
	out := make([]setStats, 0, len(by))
	for _, s := range by {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	sort.Slice(failures, func(i, j int) bool { return failures[i].At.After(failures[j].At) })
	return out, failures
}

func (a *App) renderReports() {
	a.mu.Lock()
	hist := a.hist
	p := reportPeriods[a.repPeriod]
	a.mu.Unlock()
	stats, failures := reportStats(hist, p.days, time.Now())
	a.repTable.Clear()
	a.repTable.SetTitle(fmt.Sprintf(" Reports: %s   (w changes the period) ", p.name))
	header(a.repTable, "Backup Set", "Runs", "OK", "Failed", "Success", "Last run", "Last result")
	totalRuns, totalOK := 0, 0
	for i, s := range stats {
		rate := "-"
		if s.OK+s.Fail > 0 {
			rate = fmt.Sprintf("%d%%", s.OK*100/(s.OK+s.Fail))
		}
		failCol := colDim
		if s.Fail > 0 {
			failCol = colErr
		}
		a.repTable.SetCell(i+1, 0, cell(s.Name, colText))
		a.repTable.SetCell(i+1, 1, cell(fmt.Sprint(s.Runs), colDim))
		a.repTable.SetCell(i+1, 2, cell(fmt.Sprint(s.OK), colOK))
		a.repTable.SetCell(i+1, 3, cell(fmt.Sprint(s.Fail), failCol))
		a.repTable.SetCell(i+1, 4, cell(rate, colText))
		a.repTable.SetCell(i+1, 5, cell(when(s.Last.At), colDim))
		a.repTable.SetCell(i+1, 6, cell(statusWord(s.Last.Status), statusColour(s.Last.Status)))
		totalRuns += s.Runs
		totalOK += s.OK
	}
	if len(stats) == 0 {
		a.repTable.SetCell(1, 0, cell("No runs recorded in this period.", colDim))
	} else {
		n := len(stats) + 1
		a.repTable.SetCell(n, 0, cell("All sets", colHead))
		a.repTable.SetCell(n, 1, cell(fmt.Sprint(totalRuns), colHead))
		a.repTable.SetCell(n, 2, cell(fmt.Sprint(totalOK), colHead))
		a.repTable.SetCell(n, 3, cell(fmt.Sprint(len(failures)), colHead))
	}
	var b strings.Builder
	for i, f := range failures {
		if i >= 7 {
			break
		}
		fmt.Fprintf(&b, " %s%-17s[-] [white::b]%-24s[-:-:-] %s%s[-]\n", tag(colDim), when(f.At), f.Name, tag(colErr), tview.Escape(f.Message))
	}
	if b.Len() == 0 {
		b.WriteString(" " + tag(colOK) + "No failed runs in this period.")
	}
	a.repFail.SetText(b.String())
}

// ---- log files ---------------------------------------------------------------

func (a *App) buildLogs() tview.Primitive {
	a.logsTable = newTable()
	a.logsTable.SetTitle(" Log files ").SetTitleAlign(tview.AlignLeft)
	a.logsTable.SetSelectedFunc(func(row, _ int) { a.openLogFile(row) })
	return a.logsTable
}

func (a *App) loadLogFiles() {
	go func() {
		files, err := a.be.LogFiles()
		a.app.QueueUpdateDraw(func() {
			a.mu.Lock()
			a.logFiles, a.logErr = files, err
			a.mu.Unlock()
			a.renderLogs()
		})
	}()
}

func (a *App) renderLogs() {
	keep, _ := a.logsTable.GetSelection()
	a.logsTable.Clear()
	header(a.logsTable, "Written", "Kind", "Backup ID", "Size", "File")
	a.mu.Lock()
	files, err := a.logFiles, a.logErr
	a.mu.Unlock()
	if err != nil {
		a.logsTable.SetCell(1, 0, cell(describeErr(err), colErr))
		return
	}
	for i, f := range files {
		a.logsTable.SetCell(i+1, 0, cell(when(f.Modified), colDim))
		a.logsTable.SetCell(i+1, 1, cell(f.Kind, colText))
		a.logsTable.SetCell(i+1, 2, cell(f.BackupID, colText))
		a.logsTable.SetCell(i+1, 3, cell(human(float64(f.Size)), colDim))
		a.logsTable.SetCell(i+1, 4, cell(f.Name, colDim))
	}
	if len(files) > 0 {
		if keep < 1 || keep > len(files) {
			keep = 1
		}
		a.logsTable.Select(keep, 0)
	}
}

func (a *App) openLogFile(row int) {
	a.mu.Lock()
	if row < 1 || row > len(a.logFiles) {
		a.mu.Unlock()
		return
	}
	f := a.logFiles[row-1]
	a.mu.Unlock()
	a.viewLog("Log file: "+f.Name, func(filter string, hide bool) ([]string, error) {
		return a.be.ReadLogFile(f, filter, hide)
	})
}
