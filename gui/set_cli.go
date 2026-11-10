package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/salvanza/salvanza_client/gui/api"
)

// Command-line Undelete and Roll back, on the same engine as the GUI's pages,
// for a Backup Set defined in the GUI:
//
//	salvanza-svc -list-sets
//	salvanza-svc -undelete "<set>" [-days N] [-to DIR] [-dry-run] [-no-verify]
//	salvanza-svc -rollback "<set>" [-at "2026-10-10 14:30"] [-policy replace|missing|exact]
//	             [-backup-first] [-dry-run] [-yes] [-no-verify]
//	salvanza-svc -list-rollbacks "<set>"
//	salvanza-svc -undo-rollback "<set>" -created <time from -list-rollbacks>
//	salvanza-svc -delete-rollback-copies "<set>" -created <time from -list-rollbacks>
//
// The service program (a console program on Windows) and the Linux GUI
// program both take them; runSetCLI returns false when none of them is on
// the command line. A set is named by its name (any case) or its id.

type setOpResult struct {
	success bool
	message string
}

func runSetCLI(args []string) bool {
	cmd := ""
	for _, a := range args {
		switch strings.TrimLeft(a, "-") {
		case "list-sets", "undelete", "rollback", "list-rollbacks", "undo-rollback", "delete-rollback-copies":
			if cmd == "" {
				cmd = strings.TrimLeft(a, "-")
			}
		}
	}
	if cmd == "" {
		return false
	}
	os.Exit(setCLI(cmd, args))
	return true
}

func setCLI(cmd string, args []string) int {
	fs := flag.NewFlagSet("sets", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	listSets := fs.Bool("list-sets", false, "list the Backup Sets")
	undelete := fs.String("undelete", "", "Backup Set to undelete files for (name or id)")
	rollback := fs.String("rollback", "", "Backup Set to roll back (name or id)")
	days := fs.Int("days", 0, "undelete: also look at the snapshots of the last N days (0: the newest only)")
	to := fs.String("to", "", "undelete: restore under this folder instead of where the files were")
	at := fs.String("at", "", `roll back: to the newest backup at or before this local time ("2026-10-10 14:30"); default the newest`)
	policy := fs.String("policy", "replace", "roll back: replace (missing and changed), missing (missing only) or exact (also remove files added since)")
	backupFirst := fs.Bool("backup-first", false, "roll back: run the set once before changing anything")
	dryRun := fs.Bool("dry-run", false, "only show what would be done")
	yes := fs.Bool("yes", false, "roll back without asking")
	noVerify := fs.Bool("no-verify", false, "do not verify the restored files")
	listRollbacks := fs.String("list-rollbacks", "", "Backup Set whose earlier roll backs to list (name or id)")
	undoRollback := fs.String("undo-rollback", "", "Backup Set whose roll back to undo (name or id); needs -created")
	deleteCopies := fs.String("delete-rollback-copies", "", "Backup Set whose roll back safety copies to delete (name or id); needs -created")
	created := fs.String("created", "", "undo-rollback, delete-rollback-copies: the roll back's time as -list-rollbacks prints it")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	a := NewApp()
	a.mode = api.ModeStandalone
	a.headless = true

	jobs, err := a.GetScheduledJobs()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot read the Backup Sets:", err)
		return 1
	}
	if *listSets {
		for _, j := range jobs {
			what := strings.Join(j.BackupDirs, ", ")
			if j.BackupType == "machine" {
				what = "machine: " + strings.Join(j.DriveLetters, ", ")
			}
			fmt.Printf("%-30s  %s  (%s)\n", j.Name, j.ID, what)
		}
		return 0
	}
	name := *undelete
	switch cmd {
	case "rollback":
		name = *rollback
	case "list-rollbacks":
		name = *listRollbacks
	case "undo-rollback":
		name = *undoRollback
	case "delete-rollback-copies":
		name = *deleteCopies
	}
	var job *ScheduledJob
	for i := range jobs {
		if jobs[i].ID == name || strings.EqualFold(jobs[i].Name, name) {
			job = &jobs[i]
			break
		}
	}
	if job == nil {
		fmt.Fprintf(os.Stderr, "No Backup Set called %q (-list-sets lists them)\n", name)
		return 1
	}

	done := make(chan setOpResult, 1)
	lastLine := time.Time{}
	a.cliProgress = func(pct float64, msg string) {
		if time.Since(lastLine) >= time.Second {
			lastLine = time.Now()
			fmt.Printf("%3.0f%%  %s\n", pct*100, msg)
		}
	}
	a.cliDone = done
	wait := func() int {
		r := <-done
		fmt.Println(r.message)
		if !r.success {
			return 1
		}
		return 0
	}

	switch cmd {
	case "list-rollbacks", "undo-rollback", "delete-rollback-copies":
		return rollbackRecordsCLI(a, cmd, job, *created)

	case "undelete":
		fmt.Printf("Looking for files of %q that are in its backups but not on disk...\n", job.Name)
		scan, err := a.ScanUndelete(job.ID, *days)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Undelete scan:", err)
			return 1
		}
		var total uint64
		for _, f := range scan.Files {
			total += f.Size
			gone := "since the last backup"
			if f.GoneByUnix > 0 {
				gone = "between " + time.Unix(f.SnapshotUnix, 0).Format("2006-01-02 15:04") + " and " + time.Unix(f.GoneByUnix, 0).Format("2006-01-02 15:04")
			}
			line := fmt.Sprintf("  %s/%s  (%s, deleted %s)", f.Folder, f.Path, formatByteSize(f.Size), gone)
			if f.MovedTo != "" {
				line += "  [probably moved to " + f.MovedTo + "]"
			}
			fmt.Println(line)
		}
		fmt.Printf("%d files (%s) from %d snapshot(s)", len(scan.Files), formatByteSize(total), scan.SnapshotsChecked)
		if scan.Unknown > 0 {
			fmt.Printf("; %d more under folders that could not be read", scan.Unknown)
		}
		fmt.Println()
		if len(scan.NotOpened) > 0 {
			fmt.Printf("Not checked: %s (couldn't open as the logged in user)\n", strings.Join(scan.NotOpened, ", "))
		}
		if *dryRun || len(scan.Files) == 0 {
			return 0
		}
		var items []UndeleteItem
		for _, f := range scan.Files {
			if f.MovedTo != "" {
				continue // as in the GUI: a likely move is not restored unless picked
			}
			items = append(items, UndeleteItem{Archive: f.Archive, Path: f.Path, SnapshotUnix: f.SnapshotUnix})
		}
		if len(items) == 0 {
			fmt.Println("Nothing to restore (only probable moves).")
			return 0
		}
		if err := a.StartUndelete(job.ID, items, *to, !*noVerify); err != nil {
			fmt.Fprintln(os.Stderr, "Undelete:", err)
			return 1
		}
		return wait()

	case "rollback":
		switch *policy {
		case rollbackReplace, rollbackMissing, rollbackExact:
		default:
			fmt.Fprintf(os.Stderr, "-policy must be replace, missing or exact, not %q\n", *policy)
			return 2
		}
		snaps, err := a.ListSetSnapshots(job.ID)
		if err != nil || len(snaps) == 0 {
			fmt.Fprintln(os.Stderr, "No backups of this set to roll back to:", err)
			return 1
		}
		pick := snaps[0] // newest first
		if *at != "" {
			when, err := time.ParseInLocation("2006-01-02 15:04", *at, time.Local)
			if err != nil {
				fmt.Fprintf(os.Stderr, "-at %q: use \"YYYY-MM-DD HH:MM\"\n", *at)
				return 2
			}
			found := false
			for _, s := range snaps {
				if s.Unix <= when.Unix() {
					pick, found = s, true
					break
				}
			}
			if !found {
				fmt.Fprintln(os.Stderr, "There is no backup at or before", *at)
				return 1
			}
		}
		fmt.Printf("Comparing %q with its backup of %s...\n", job.Name, time.Unix(pick.Unix, 0).Format("2006-01-02 15:04"))
		pv, err := a.PreviewRollback(job.ID, pick.Unix)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Roll back preview:", err)
			return 1
		}
		restore := pv.MissingCount + pv.ChangedCount
		if *policy == rollbackMissing {
			restore = pv.MissingCount
		}
		remove := 0
		if *policy == rollbackExact {
			remove = pv.NewCount
		}
		fmt.Printf("  missing now: %d files (%s)\n  changed since: %d files (%s)\n  added since: %d files (%s)\n  unchanged: %d\n",
			pv.MissingCount, formatByteSize(pv.MissingBytes), pv.ChangedCount, formatByteSize(pv.ChangedBytes),
			pv.NewCount, formatByteSize(pv.NewBytes), pv.Unchanged)
		if len(pv.NotOpened) > 0 {
			fmt.Printf("  left alone: %s (couldn't open as the logged in user)\n", strings.Join(pv.NotOpened, ", "))
		}
		fmt.Printf("Policy %s: %d files restored or replaced, %d removed; everything replaced or removed is kept in %s for Undo.\n",
			*policy, restore, remove, strings.Join(pv.SafetyFolders, ", "))
		if *dryRun {
			return 0
		}
		if restore == 0 && remove == 0 {
			fmt.Println("Nothing to do.")
			return 0
		}
		if !*yes {
			fmt.Print("Type yes to roll back: ")
			line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			if strings.TrimSpace(strings.ToLower(line)) != "yes" {
				fmt.Println("Not rolled back.")
				return 1
			}
		}
		if err := a.StartRollback(job.ID, pick.Unix, *policy, *backupFirst, pv.NewCount, !*noVerify); err != nil {
			fmt.Fprintln(os.Stderr, "Roll back:", err)
			return 1
		}
		return wait()
	}
	return 2
}

// rollbackRecordsCLI lists a set's earlier roll backs (one tab-separated
// ROLLBACK line each: time, status, policy, files moved aside, removed,
// restored, bytes kept), or undoes one or deletes its safety copies, the one
// made at the -created time.
func rollbackRecordsCLI(a *App, cmd string, job *ScheduledJob, created string) int {
	records, err := a.ListRollbacks(job.ID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Roll backs:", err)
		return 1
	}
	if cmd == "list-rollbacks" {
		for _, r := range records {
			fmt.Printf("ROLLBACK\t%s\t%s\t%s\t%d\t%d\t%d\t%d\n", r.Created, r.Status, r.Policy, r.Moved, r.Removed, r.Restored, r.Bytes)
		}
		fmt.Printf("%d earlier roll back(s) of %q\n", len(records), job.Name)
		return 0
	}
	if created == "" {
		fmt.Fprintln(os.Stderr, "-created is needed: use the time -list-rollbacks prints")
		return 2
	}
	for _, r := range records {
		if r.Created != created {
			continue
		}
		if cmd == "undo-rollback" {
			msg, err := a.UndoRollback(r.Path)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Undo roll back:", err)
				return 1
			}
			fmt.Println(msg)
			return 0
		}
		if err := a.DeleteRollbackCopies(r.Path); err != nil {
			fmt.Fprintln(os.Stderr, "Delete the safety copies:", err)
			return 1
		}
		fmt.Printf("Deleted the safety copies of the roll back of %s; it can no longer be undone.\n", created)
		return 0
	}
	fmt.Fprintf(os.Stderr, "No roll back of %q was made at %s (-list-rollbacks lists them)\n", job.Name, created)
	return 1
}
