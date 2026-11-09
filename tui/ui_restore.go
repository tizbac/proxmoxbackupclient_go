package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// restoreView is the Restore screen: five steps like the desktop app's
// (Source, Snapshot, Files, Destination, Restore). Later steps are reached
// from the earlier ones; Esc goes one step back.
type restoreView struct {
	a      *App
	root   *tview.Flex
	steps  *tview.TextView
	pages  *tview.Pages
	step   int
	server *tview.DropDown
	ids    []string // server ids of the dropdown, "" first

	groupsTable *tview.Table
	snapTable   *tview.Table
	tree        *tview.TreeView
	treeInfo    *tview.TextView
	destForm    *tview.Form
	dest        *tview.InputField
	overwrite   *tview.Checkbox
	progress    *tview.TextView

	mu       sync.Mutex
	groups   []Group
	group    Group
	snaps    []SnapshotInfo
	snap     SnapshotInfo
	children map[string][]CatalogItem
	selected map[string]bool
	running  bool
	// the key's passphrase, once asked for, for this snapshot
	passphrase string
}

func (a *App) buildRestore() tview.Primitive {
	r := &restoreView{a: a, children: map[string][]CatalogItem{}, selected: map[string]bool{}}
	a.rs = r
	r.steps = tview.NewTextView().SetDynamicColors(true)
	r.pages = tview.NewPages()

	r.server = tview.NewDropDown().SetLabel(" Server ").SetFieldBackgroundColor(colSel).SetFieldTextColor(tcell.ColorWhite)
	r.server.SetLabelColor(colDim)
	r.groupsTable = newTable()
	r.groupsTable.SetTitle(" Backups on the server ").SetTitleAlign(tview.AlignLeft)
	r.groupsTable.SetSelectedFunc(func(row, _ int) { r.pickGroup(row) })
	r.groupsTable.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyTab {
			a.app.SetFocus(r.server)
			return nil
		}
		return ev
	})
	r.server.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyTab {
			a.app.SetFocus(r.groupsTable)
			return nil
		}
		return ev
	})
	step1 := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(r.server, 1, 0, false).AddItem(r.groupsTable, 0, 1, true)

	r.snapTable = newTable()
	r.snapTable.SetTitle(" Snapshots ").SetTitleAlign(tview.AlignLeft)
	r.snapTable.SetSelectedFunc(func(row, _ int) { r.pickSnapshot(row) })

	r.tree = tview.NewTreeView()
	r.tree.SetBorder(true).SetTitle(" Files ").SetTitleAlign(tview.AlignLeft)
	r.tree.SetGraphics(false)
	r.treeInfo = textBox("Selection")
	r.tree.SetInputCapture(r.treeKeys)
	r.tree.SetSelectedFunc(func(n *tview.TreeNode) { r.expand(n, !n.IsExpanded()) })
	step3 := tview.NewFlex().AddItem(r.tree, 0, 3, true).AddItem(r.treeInfo, 34, 0, false)

	r.dest = tview.NewInputField().SetLabel("Restore into ").SetFieldWidth(60)
	r.overwrite = tview.NewCheckbox().SetLabel("Overwrite files that exist ")
	r.destForm = styleForm(tview.NewForm()).AddFormItem(r.dest).AddFormItem(r.overwrite)
	r.destForm.AddButton("Restore", r.start).AddButton("Back", func() { r.show(2) })
	r.destForm.SetTitle(" Where to put the files ")
	r.destForm.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEscape {
			r.show(2)
			return nil
		}
		return ev
	})

	r.progress = textBox("Restore")
	r.progress.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if (ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyEnter) && !r.isRunning() {
			r.show(0)
			return nil
		}
		return ev
	})

	r.pages.AddPage("0", step1, true, true)
	r.pages.AddPage("1", r.snapTable, true, false)
	r.pages.AddPage("2", step3, true, false)
	r.pages.AddPage("3", r.destForm, true, false)
	r.pages.AddPage("4", r.progress, true, false)
	r.root = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(r.steps, 1, 0, false).AddItem(r.pages, 0, 1, true)

	// Esc: one step back (the screen's own keys come first)
	r.snapTable.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEscape {
			r.show(0)
			return nil
		}
		return ev
	})
	r.renderSteps()
	return r.root
}

// typing says whether a text field has the keyboard (the destination step).
func (r *restoreView) typing() bool { return r.step == 3 }

func (r *restoreView) isRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *restoreView) renderSteps() {
	names := []string{"Source", "Snapshot", "Files", "Destination", "Restore"}
	var b strings.Builder
	b.WriteString(" ")
	for i, n := range names {
		switch {
		case i == r.step:
			fmt.Fprintf(&b, "[black:#%06x] %d %s [-:-] ", colHead.Hex(), i+1, n)
		case i < r.step:
			fmt.Fprintf(&b, "%s %d %s [-:-] ", tag(colOK), i+1, n)
		default:
			fmt.Fprintf(&b, "%s %d %s [-:-] ", tag(colDim), i+1, n)
		}
	}
	r.steps.SetText(b.String())
}

func (r *restoreView) show(step int) {
	r.step = step
	r.pages.SwitchToPage(fmt.Sprint(step))
	r.renderSteps()
	a := r.a
	switch step {
	case 0:
		a.app.SetFocus(r.groupsTable)
	case 1:
		a.app.SetFocus(r.snapTable)
	case 2:
		a.app.SetFocus(r.tree)
	case 3:
		a.app.SetFocus(r.destForm)
	case 4:
		a.app.SetFocus(r.progress)
	}
	a.renderChrome()
}

// enter is called when the Restore tab is shown: fill the servers and groups.
func (r *restoreView) enter() {
	a := r.a
	a.mu.Lock()
	options := []string{"(default server)"}
	r.ids = []string{""}
	for _, s := range a.servers {
		options = append(options, s.Name)
		r.ids = append(r.ids, s.ID)
	}
	a.mu.Unlock()
	r.server.SetOptions(options, func(string, int) { r.loadGroups() })
	r.server.SetCurrentOption(0)
	if r.step == 0 {
		r.loadGroups()
	}
	r.show(r.step)
}

func (r *restoreView) serverID() string {
	i, _ := r.server.GetCurrentOption()
	if i >= 0 && i < len(r.ids) {
		return r.ids[i]
	}
	return ""
}

func (r *restoreView) loadGroups() {
	r.groupsTable.Clear()
	header(r.groupsTable, "Backup", "Backups", "Latest")
	r.groupsTable.SetCell(1, 0, cell("reading the server...", colDim))
	srv := r.serverID()
	go func() {
		groups, err := r.a.be.RestoreGroups(srv)
		r.a.app.QueueUpdateDraw(func() {
			r.groupsTable.Clear()
			header(r.groupsTable, "Backup", "Backups", "Latest")
			if err != nil {
				r.groupsTable.SetCell(1, 0, cell(describeErr(err), colErr))
				return
			}
			r.mu.Lock()
			r.groups = groups
			r.mu.Unlock()
			for i, g := range groups {
				r.groupsTable.SetCell(i+1, 0, cell(g.Type+"/"+g.ID, colText))
				r.groupsTable.SetCell(i+1, 1, cell(fmt.Sprint(g.Count), colDim))
				r.groupsTable.SetCell(i+1, 2, cell(when(g.Latest), colDim))
			}
			if len(groups) == 0 {
				r.groupsTable.SetCell(1, 0, cell("nothing is stored on this server yet", colDim))
			} else {
				r.groupsTable.Select(1, 0)
			}
		})
	}()
}

func (r *restoreView) pickGroup(row int) {
	r.mu.Lock()
	if row < 1 || row > len(r.groups) {
		r.mu.Unlock()
		return
	}
	r.group = r.groups[row-1]
	g := r.group
	r.mu.Unlock()
	r.snapTable.Clear()
	header(r.snapTable, "Taken", "Size", "Archives", "Encrypted")
	r.snapTable.SetCell(1, 0, cell("reading the snapshots...", colDim))
	r.show(1)
	srv := r.serverID()
	go func() {
		snaps, err := r.a.be.RestoreSnapshots(srv, g)
		r.a.app.QueueUpdateDraw(func() {
			r.snapTable.Clear()
			header(r.snapTable, "Taken", "Size", "Archives", "Encrypted")
			if err != nil {
				r.snapTable.SetCell(1, 0, cell(describeErr(err), colErr))
				return
			}
			r.mu.Lock()
			r.snaps = snaps
			r.mu.Unlock()
			for i, s := range snaps {
				enc, c := "no", colDim
				if s.Encrypted {
					enc, c = "yes", colWarn
				}
				arch := fmt.Sprint(len(s.Archives))
				if len(s.Archives) == 0 {
					arch = "none (disk image)"
				}
				r.snapTable.SetCell(i+1, 0, cell(s.Time.Format("Mon 2 Jan 2006 15:04"), colText))
				r.snapTable.SetCell(i+1, 1, cell(human(float64(s.Size)), colDim))
				r.snapTable.SetCell(i+1, 2, cell(arch, colDim))
				r.snapTable.SetCell(i+1, 3, cell(enc, c))
			}
			if len(snaps) > 0 {
				r.snapTable.Select(1, 0)
			}
		})
	}()
}

func (r *restoreView) pickSnapshot(row int) {
	r.mu.Lock()
	if row < 1 || row > len(r.snaps) {
		r.mu.Unlock()
		return
	}
	r.snap = r.snaps[row-1]
	r.passphrase = ""
	s := r.snap
	r.mu.Unlock()
	if len(s.Archives) == 0 {
		r.a.errorBox("This snapshot is a disk image, not files. Restore it with the bare-metal restore.")
		return
	}
	root := tview.NewTreeNode(tag(colDim) + "reading the file list...").SetSelectable(false)
	r.tree.SetRoot(root).SetCurrentNode(root)
	r.mu.Lock()
	r.selected = map[string]bool{} // the ticks of the snapshot before do not carry over
	r.mu.Unlock()
	r.refreshInfo()
	r.show(2)
	r.readCatalog(s, "")
}

// readCatalog reads the file list; an encrypted snapshot whose key needs a
// passphrase asks for it, and tries again with it.
func (r *restoreView) readCatalog(s SnapshotInfo, passphrase string) {
	srv := r.serverID()
	go func() {
		items, err := r.a.be.RestoreCatalog(srv, s, passphrase)
		r.a.app.QueueUpdateDraw(func() {
			switch {
			case errors.Is(err, ErrNeedPassphrase):
				r.askPassphrase("", func(p string) { r.readCatalog(s, p) })
			case err != nil && passphrase != "" && !errors.Is(err, ErrNoKey) && strings.Contains(err.Error(), "could not be unlocked"):
				r.askPassphrase("That passphrase did not unlock the key. Try again.", func(p string) { r.readCatalog(s, p) })
			case err != nil:
				r.a.errorBox(describeErr(err))
				r.show(1)
			default:
				r.mu.Lock()
				r.passphrase = passphrase
				r.mu.Unlock()
				r.loadTree(items)
			}
		})
	}()
}

// askPassphrase shows a masked field for the key's passphrase; Esc goes back
// to the snapshot list.
func (r *restoreView) askPassphrase(note string, then func(string)) {
	a := r.a
	field := tview.NewInputField().SetLabel("Passphrase ").SetFieldWidth(40).SetMaskCharacter('*')
	form := styleForm(tview.NewForm()).AddFormItem(field)
	text := "This backup is encrypted. Enter the passphrase of the key."
	if note != "" {
		text = note
	}
	form.AddTextView("", tag(colDim)+text, 56, 2, true, false)
	form.SetTitle(" Encrypted backup ")
	closeAsk := func() {
		a.pages.RemovePage("passphrase")
		a.modal = false
	}
	form.AddButton("Unlock", func() {
		p := field.GetText()
		closeAsk()
		if p == "" {
			r.show(1)
			return
		}
		then(p)
	})
	form.AddButton("Cancel", func() {
		closeAsk()
		r.show(1)
	})
	form.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEscape {
			closeAsk()
			r.show(1)
			return nil
		}
		return ev
	})
	box := tview.NewFlex().AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).AddItem(nil, 0, 1, false).AddItem(form, 11, 0, true).AddItem(nil, 0, 1, false), 66, 0, true).
		AddItem(nil, 0, 1, false)
	a.modal = true
	a.pages.AddPage("passphrase", box, true, true)
	a.app.SetFocus(form)
}

func parentOf(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return ""
}

func (r *restoreView) loadTree(items []CatalogItem) {
	r.mu.Lock()
	r.children = map[string][]CatalogItem{}
	r.selected = map[string]bool{}
	for _, it := range items {
		r.children[parentOf(it.Path)] = append(r.children[parentOf(it.Path)], it)
	}
	for k := range r.children {
		c := r.children[k]
		sort.SliceStable(c, func(i, j int) bool {
			if c[i].Dir != c[j].Dir {
				return c[i].Dir
			}
			return strings.ToLower(c[i].Path) < strings.ToLower(c[j].Path)
		})
	}
	snap := r.snap
	r.mu.Unlock()
	root := tview.NewTreeNode("").SetSelectable(false).SetColor(colHead)
	root.SetText(fmt.Sprintf("%s%s  %s", tag(colHead), snap.Time.Format("2 Jan 2006 15:04"), snap.BackupID))
	r.addChildren(root, "")
	for _, n := range root.GetChildren() {
		n.SetExpanded(true) // the archive nodes open
		r.addChildren(n, n.GetReference().(string))
	}
	r.tree.SetRoot(root)
	if kids := root.GetChildren(); len(kids) > 0 {
		r.tree.SetCurrentNode(kids[0])
	}
	r.refreshLabels()
	r.refreshInfo()
}

func (r *restoreView) addChildren(n *tview.TreeNode, path string) {
	if len(n.GetChildren()) > 0 {
		return
	}
	r.mu.Lock()
	kids := r.children[path]
	r.mu.Unlock()
	for _, it := range kids {
		child := tview.NewTreeNode("").SetReference(it.Path).SetSelectable(true).
			SetSelectedTextStyle(tcell.StyleDefault.Background(colSel).Foreground(tcell.ColorWhite))
		child.SetExpanded(false)
		n.AddChild(child)
	}
}

func (r *restoreView) expand(n *tview.TreeNode, open bool) {
	ref, _ := n.GetReference().(string)
	if ref == "" {
		return
	}
	r.mu.Lock()
	isDir := false
	for _, it := range r.children[parentOf(ref)] {
		if it.Path == ref {
			isDir = it.Dir
		}
	}
	r.mu.Unlock()
	if !isDir {
		return
	}
	if open {
		r.addChildren(n, ref)
	}
	n.SetExpanded(open)
	r.refreshLabels()
}

// covered says whether path is ticked itself or inside a ticked folder.
func (r *restoreView) covered(path string) (direct, inherited bool) {
	if r.selected[path] {
		return true, false
	}
	for p := parentOf(path); p != ""; p = parentOf(p) {
		if r.selected[p] {
			return false, true
		}
	}
	return false, false
}

func (r *restoreView) refreshLabels() {
	r.mu.Lock()
	defer r.mu.Unlock()
	var walk func(n *tview.TreeNode)
	walk = func(n *tview.TreeNode) {
		for _, c := range n.GetChildren() {
			ref, _ := c.GetReference().(string)
			var it CatalogItem
			for _, x := range r.children[parentOf(ref)] {
				if x.Path == ref {
					it = x
				}
			}
			name := ref[strings.LastIndex(ref, "/")+1:]
			if parentOf(ref) == "" {
				name = archiveBase(name)
			}
			box, col := "[ ]", colText
			if d, inh := r.covered(ref); d {
				box, col = "[x]", colOK
			} else if inh {
				box, col = "[x]", colDim
			}
			icon, size := "  ", ""
			if it.Dir {
				icon = "+ "
				if c.IsExpanded() {
					icon = "- "
				}
			} else {
				size = tag(colDim) + "  " + human(float64(it.Size))
			}
			c.SetText(fmt.Sprintf("%s%s[-] %s%s%s", tag(col), box, icon, tview.Escape(name), size))
			walk(c)
		}
	}
	walk(r.tree.GetRoot())
}

func (r *restoreView) includes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for p, on := range r.selected {
		if !on {
			continue
		}
		skip := false
		for q := parentOf(p); q != ""; q = parentOf(q) {
			if r.selected[q] {
				skip = true
			}
		}
		if !skip {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func (r *restoreView) refreshInfo() {
	inc := r.includes()
	var b strings.Builder
	if len(inc) == 0 {
		b.WriteString(tag(colDim) + "Nothing ticked.\nRestoring now restores the whole snapshot.\n")
	} else {
		fmt.Fprintf(&b, "%s%d item(s) ticked[-]\n", tag(colOK), len(inc))
		for i, p := range inc {
			if i >= 8 {
				fmt.Fprintf(&b, "%s...[-]\n", tag(colDim))
				break
			}
			shown := archiveBase(p) + "  (the whole backup)"
			if i := strings.Index(p, "/"); i >= 0 {
				shown = p[i+1:]
			}
			b.WriteString(tview.Escape(shown) + "\n")
		}
	}
	b.WriteString("\n" + tag(colKey) + "Space[-] tick  " + tag(colKey) + "a[-] all  " + tag(colKey) + "c[-] clear\n" +
		tag(colKey) + "Right/Left[-] open, close\n" + tag(colKey) + "PgDn[-] next step  " + tag(colKey) + "Esc[-] back")
	r.treeInfo.SetText(b.String())
}

func (r *restoreView) treeKeys(ev *tcell.EventKey) *tcell.EventKey {
	node := r.tree.GetCurrentNode()
	switch {
	case ev.Key() == tcell.KeyEscape:
		r.show(1)
		return nil
	case ev.Key() == tcell.KeyPgDn:
		r.toDestination()
		return nil
	case ev.Key() == tcell.KeyRight && node != nil:
		r.expand(node, true)
		return nil
	case ev.Key() == tcell.KeyLeft && node != nil:
		r.expand(node, false)
		return nil
	case ev.Rune() == ' ' && node != nil:
		if ref, _ := node.GetReference().(string); ref != "" {
			r.mu.Lock()
			r.selected[ref] = !r.selected[ref]
			r.mu.Unlock()
			r.refreshLabels()
			r.refreshInfo()
		}
		return nil
	case ev.Rune() == 'a':
		r.mu.Lock()
		r.selected = map[string]bool{}
		for _, it := range r.children[""] {
			r.selected[it.Path] = true
		}
		r.mu.Unlock()
		r.refreshLabels()
		r.refreshInfo()
		return nil
	case ev.Rune() == 'c':
		r.mu.Lock()
		r.selected = map[string]bool{}
		r.mu.Unlock()
		r.refreshLabels()
		r.refreshInfo()
		return nil
	}
	return ev
}

func (r *restoreView) toDestination() {
	if r.dest.GetText() == "" {
		home, _ := os.UserHomeDir()
		r.dest.SetText(filepath.Join(home, "Salvanza restore "+time.Now().Format("2006-01-02 1504")))
	}
	r.show(3)
}

func (r *restoreView) start() {
	dest := strings.TrimSpace(r.dest.GetText())
	if dest == "" {
		r.a.errorBox("Choose a folder to restore into.")
		return
	}
	inc := r.includes()
	r.mu.Lock()
	r.running = true
	snap := r.snap
	r.mu.Unlock()
	r.progress.SetText(fmt.Sprintf(" %sRestoring into %s ...[-]\n", tag(colHead), tview.Escape(dest)))
	r.show(4)
	srv := r.serverID()
	overwrite := r.overwrite.IsChecked()
	r.mu.Lock()
	pass := r.passphrase
	r.mu.Unlock()
	go func() {
		count := 0
		n, err := r.a.be.RestoreFiles(srv, snap, inc, dest, overwrite, pass, func(file string) {
			count++
			c := count
			r.a.app.QueueUpdateDraw(func() {
				r.progress.SetText(fmt.Sprintf(" %sRestoring into %s[-]\n\n %s%d files so far[-]\n %s%s[-]", tag(colHead), tview.Escape(dest), tag(colOK), c, tag(colDim), tview.Escape(file)))
			})
		})
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
		r.a.app.QueueUpdateDraw(func() {
			if err != nil {
				r.progress.SetText(fmt.Sprintf(" %sThe restore stopped:[-] %s\n\n %sEsc[-] to go back", tag(colErr), tview.Escape(describeErr(err)), tag(colKey)))
				return
			}
			r.progress.SetText(fmt.Sprintf(" %sRestore finished.[-]\n\n %d file(s) restored into\n %s\n\n %sEsc[-] for a new restore", tag(colOK), n, tview.Escape(dest), tag(colKey)))
			r.a.setFlash(fmt.Sprintf("restored %d files", n))
		})
	}()
}
