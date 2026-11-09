package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/salvanza/salvanza_client/gui/api"
)

// machineSel is the disks of a machine Backup Set or one-off disk backup: the
// device paths, the identity recorded for each (at the same index, so the
// service finds the disk again if the system numbers it differently), and the
// partitions left out (not read, zeros in the image).
type machineSel struct {
	paths []string
	ids   []string
	skips []api.PartitionSkip
}

// selOf reads the disks of a Backup Set document.
func selOf(m map[string]interface{}) machineSel {
	s := machineSel{paths: strList(m, "driveLetters")}
	ids, _ := m["diskIds"].([]interface{})
	for i := range s.paths {
		id := ""
		if i < len(ids) {
			id, _ = ids[i].(string)
		}
		s.ids = append(s.ids, id)
	}
	if arr, ok := m["skipPartitions"].([]interface{}); ok {
		for _, v := range arr {
			p, _ := v.(map[string]interface{})
			if p == nil {
				continue
			}
			s.skips = append(s.skips, api.PartitionSkip{
				DiskID: str(p, "diskId"), Device: str(p, "device"), Start: uint64(num(p, "start")),
				Size: uint64(num(p, "size")), Number: int(num(p, "number")), Label: str(p, "label"),
			})
		}
	}
	return s
}

// apply writes the disks into a Backup Set document.
func (s machineSel) apply(doc map[string]interface{}) {
	paths := make([]interface{}, 0, len(s.paths))
	ids := make([]interface{}, 0, len(s.paths))
	for i, p := range s.paths {
		paths = append(paths, p)
		id := ""
		if i < len(s.ids) {
			id = s.ids[i]
		}
		ids = append(ids, id)
	}
	skips := make([]interface{}, 0, len(s.skips))
	for _, k := range s.skips {
		m := map[string]interface{}{"device": k.Device, "start": float64(k.Start), "size": float64(k.Size)}
		if k.DiskID != "" {
			m["diskId"] = k.DiskID
		}
		if k.Number != 0 {
			m["number"] = float64(k.Number)
		}
		if k.Label != "" {
			m["label"] = k.Label
		}
		skips = append(skips, m)
	}
	doc["driveLetters"], doc["diskIds"], doc["skipPartitions"] = paths, ids, skips
}

// summary is the one line the forms show for the disks.
func (s machineSel) summary() string {
	if len(s.paths) == 0 {
		return "no disk chosen"
	}
	out := strings.Join(s.paths, ", ")
	if len(s.skips) > 0 {
		out += fmt.Sprintf("; %d partition(s) left out", len(s.skips))
	}
	return out
}

type pickRow struct {
	disk, part int // part -1 is the disk itself
}

type diskPicker struct {
	a     *App
	disks []Disk
	rows  []pickRow
	table *tview.Table
	note  *tview.TextView
	on    []bool            // by disk: backed up
	left  map[[3]int64]bool // by disk, partition start and size: left out
	idsAt []string          // by disk: the identity to record
	lost  machineSel        // disks the set names that are not listed any more
	skips bool              // partitions can be left out (a set, not a one-off)
	done  func(machineSel)
	quit  func()
}

// pickDisks lists the disks, with what the set backs up now ticked, and calls
// done with the new choice on Enter. A disk the set names that the system no
// longer lists is kept, and said so.
func (a *App) pickDisks(now machineSel, withSkips bool, done func(machineSel), cancel func()) {
	go func() {
		disks, err := a.be.Disks()
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.errorBox("The disks could not be listed: " + describeErr(err) + "\nListing them needs administrator rights (root on Linux).")
				cancel()
				return
			}
			p := &diskPicker{a: a, disks: disks, skips: withSkips, done: done, quit: cancel, left: map[[3]int64]bool{}}
			p.load(now)
			p.show()
		})
	}()
}

// load ticks the disks of now, finding each by the identity it recorded and
// else by its device path; skips are matched the same way.
func (p *diskPicker) load(now machineSel) {
	p.on = make([]bool, len(p.disks))
	p.idsAt = make([]string, len(p.disks))
	for i, path := range now.paths {
		id := ""
		if i < len(now.ids) {
			id = now.ids[i]
		}
		found := -1
		for j, d := range p.disks {
			if stableID(id) != "" && d.hasID(id) {
				found = j
				break
			}
		}
		if found < 0 {
			for j, d := range p.disks {
				if d.DevicePath == path && (stableID(id) == "" || stableID(d.UniqueID) == "") {
					found = j
					break
				}
			}
		}
		if found < 0 {
			p.lost.paths = append(p.lost.paths, path)
			p.lost.ids = append(p.lost.ids, id)
			continue
		}
		p.on[found] = true
		p.idsAt[found] = id
	}
	for _, k := range now.skips {
		for j, d := range p.disks {
			match := false
			if k.DiskID != "" {
				match = d.hasID(k.DiskID)
			} else {
				match = d.DevicePath == k.Device
			}
			if match {
				p.left[[3]int64{int64(j), int64(k.Start), int64(k.Size)}] = true
			}
		}
	}
}

func (p *diskPicker) label(d Disk) string {
	var b strings.Builder
	name := fmt.Sprintf("Disk %d", d.DiskNumber)
	if strings.HasPrefix(d.DevicePath, "/") {
		name = d.DevicePath
	}
	b.WriteString(name)
	if d.Model != "" {
		b.WriteString("  " + strings.TrimSpace(d.Model))
	}
	if len(d.DriveLetters) > 0 {
		b.WriteString("  " + strings.Join(d.DriveLetters, " "))
	}
	if d.IsBootDisk {
		b.WriteString("  [boot]")
	}
	if d.IsSystemDisk {
		b.WriteString("  [system]")
	}
	return b.String()
}

func (p *diskPicker) partLabel(pt Partition) string {
	parts := []string{fmt.Sprintf("Partition %d", pt.Number)}
	for _, s := range []string{pt.Mount, pt.FileSystem, pt.Type} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "  ")
}

func (p *diskPicker) draw() {
	p.table.Clear()
	p.rows = nil
	cell := func(r, c int, text string, col tcell.Color) {
		p.table.SetCell(r, c, tview.NewTableCell(text).SetTextColor(col).SetSelectedStyle(onColour(colSel)))
	}
	r := 0
	for i, d := range p.disks {
		mark := "( )"
		col := colWhite
		if p.on[i] {
			mark, col = "(*)", colOK
		}
		cell(r, 0, mark, col)
		cell(r, 1, p.label(d), col)
		cell(r, 2, human(float64(d.Size)), colDim)
		p.rows = append(p.rows, pickRow{i, -1})
		r++
		if !p.skips {
			continue
		}
		for j, pt := range d.Partitions {
			text, c := "backed up", colDim
			if p.left[[3]int64{int64(i), int64(pt.Start), int64(pt.Size)}] {
				text, c = "LEFT OUT", colWarn
			}
			if !p.on[i] {
				text, c = "", colDim
			}
			cell(r, 0, "   "+text, c)
			cell(r, 1, "   "+p.partLabel(pt), colDim)
			cell(r, 2, human(float64(pt.Size)), colDim)
			p.rows = append(p.rows, pickRow{i, j})
			r++
		}
	}
	p.summary()
}

func (p *diskPicker) summary() {
	n, size := 0, int64(0)
	for i, d := range p.disks {
		if p.on[i] {
			n++
			size += d.Size
		}
	}
	text := fmt.Sprintf("%d disk(s) chosen, %s", n, human(float64(size)))
	if len(p.lost.paths) > 0 {
		text += tag(colWarn) + "\nNot listed any more, kept in the set: " + strings.Join(p.lost.paths, ", ") + "[-]"
	}
	p.note.SetText(text)
}

func (p *diskPicker) toggle() {
	row, _ := p.table.GetSelection()
	if row < 0 || row >= len(p.rows) {
		return
	}
	pr := p.rows[row]
	if pr.part < 0 {
		p.on[pr.disk] = !p.on[pr.disk]
		if p.on[pr.disk] && p.idsAt[pr.disk] == "" {
			p.idsAt[pr.disk] = p.disks[pr.disk].UniqueID
		}
	} else {
		if !p.on[pr.disk] {
			p.a.setFlash("tick the disk first")
			return
		}
		pt := p.disks[pr.disk].Partitions[pr.part]
		k := [3]int64{int64(pr.disk), int64(pt.Start), int64(pt.Size)}
		p.left[k] = !p.left[k]
		if p.left[k] && pt.Role != "" {
			p.a.setFlash("the disk restored from this backup will not start without this partition")
		}
	}
	p.draw()
	p.table.Select(row, 0)
}

// result is the choice: the ticked disks (the recorded identity kept, else the
// disk's own), then the disks that are not listed any more, and the left-out
// partitions of the ticked disks.
func (p *diskPicker) result() machineSel {
	var out machineSel
	for i, d := range p.disks {
		if !p.on[i] {
			continue
		}
		id := p.idsAt[i]
		if id == "" || !d.hasID(id) {
			id = d.UniqueID
		}
		out.paths = append(out.paths, d.DevicePath)
		out.ids = append(out.ids, id)
		for _, pt := range d.Partitions {
			if p.left[[3]int64{int64(i), int64(pt.Start), int64(pt.Size)}] {
				out.skips = append(out.skips, api.PartitionSkip{
					DiskID: stableID(id), Device: d.DevicePath, Start: pt.Start, Size: pt.Size, Number: pt.Number, Label: p.partLabel(pt),
				})
			}
		}
	}
	out.paths = append(out.paths, p.lost.paths...)
	out.ids = append(out.ids, p.lost.ids...)
	return out
}

func (p *diskPicker) close() {
	p.a.pages.RemovePage("disks")
}

func (p *diskPicker) show() {
	p.table = tview.NewTable().SetSelectable(true, false)
	p.table.SetBorder(true).SetBackgroundColor(colBg).SetTitle(" Choose the disks ")
	p.note = tview.NewTextView().SetDynamicColors(true)
	hint := "Space ticks a disk"
	if p.skips {
		hint += " or leaves a partition out (it is not read; zeros go into the image)"
	}
	help := tview.NewTextView().SetDynamicColors(true).SetText(tag(colDim) + hint + ".   Enter done   Esc cancel[-]")
	p.draw()
	p.table.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch {
		case ev.Key() == tcell.KeyEscape:
			p.close()
			p.quit()
			return nil
		case ev.Key() == tcell.KeyEnter || ev.Key() == tcell.KeyCtrlS:
			sel := p.result()
			p.close()
			p.done(sel)
			return nil
		case ev.Key() == tcell.KeyRune && ev.Rune() == ' ':
			p.toggle()
			return nil
		}
		return ev
	})
	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.table, 0, 1, true).AddItem(p.note, 2, 0, false).AddItem(help, 1, 0, false)
	flex := tview.NewFlex().AddItem(nil, 0, 1, false).AddItem(body, 100, 0, true).AddItem(nil, 0, 1, false)
	p.a.modal = true
	p.a.pages.AddPage("disks", flex, true, false)
	p.a.pages.SwitchToPage("disks")
	p.a.app.SetFocus(p.table)
}
