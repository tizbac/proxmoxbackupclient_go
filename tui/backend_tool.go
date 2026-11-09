package tui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Undelete and Roll back run on the service program's command line
// (salvanza-svc -undelete / -rollback, in the GUI manual): the same engine as
// the desktop app's pages, on the Backup Sets the service runs. The window
// shows its output as it comes. Undoing a roll back stays in the desktop app.

// serviceProgramName is the file name of the service program.
func serviceProgramName() string {
	if runtime.GOOS == "windows" {
		return "salvanza-svc.exe"
	}
	return "salvanza-svc"
}

// findServiceProgram finds salvanza-svc: the SALVANZA_SVC variable, next to
// this program, the install folders, then the search path.
func findServiceProgram() (string, error) {
	name := serviceProgramName()
	var tries []string
	if p := os.Getenv("SALVANZA_SVC"); p != "" {
		tries = append(tries, p)
	}
	if exe, err := os.Executable(); err == nil {
		tries = append(tries, filepath.Join(filepath.Dir(exe), name))
	}
	if runtime.GOOS == "windows" {
		for _, env := range []string{"ProgramFiles", "ProgramW6432"} {
			if d := os.Getenv(env); d != "" {
				tries = append(tries, filepath.Join(d, "Salvanza", name))
			}
		}
	} else {
		tries = append(tries, "/usr/libexec/salvanza/"+name, "/usr/lib/salvanza/"+name, "/usr/local/libexec/salvanza/"+name)
	}
	for _, t := range tries {
		if st, err := os.Stat(t); err == nil && !st.IsDir() {
			return t, nil
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%s was not found next to this program or in the install folders; set SALVANZA_SVC to its path", name)
}

// RunSetTool runs the service program with args, calling line for each line it
// prints (progress, results and errors alike), and returns an error when it
// exits with a failure.
func (b *serviceBackend) RunSetTool(args []string, line func(string)) error {
	path, err := findServiceProgram()
	if err != nil {
		return err
	}
	return runTool(path, args, line)
}

func runTool(path string, args []string, line func(string)) error {
	cmd := exec.Command(path, args...) // #nosec G204 -- the service program found above, with arguments built by the window
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		return err
	}
	var last string
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			s := strings.TrimRight(sc.Text(), "\r")
			if strings.TrimSpace(s) == "" {
				continue
			}
			last = s
			line(s)
		}
	}()
	werr := cmd.Wait()
	_ = pw.Close()
	<-done
	if werr != nil {
		var ee *exec.ExitError
		if errors.As(werr, &ee) && last != "" {
			return errors.New(last)
		}
		return werr
	}
	return nil
}

// The demo plays the same scenes with invented files.

func (d *demoBackend) RunSetTool(args []string, line func(string)) error {
	has := func(f string) bool {
		for _, a := range args {
			if a == f {
				return true
			}
		}
		return false
	}
	pause := func() { time.Sleep(60 * time.Millisecond) }
	switch {
	case has("-undelete"):
		line(`Looking for files of "Documents" that are in its backups but not on disk...`)
		pause()
		line("  C:/Users/demo/Documents/budget-2026.xlsx  (84 KiB, deleted since the last backup)")
		line("  C:/Users/demo/Documents/notes/old-plan.docx  (31 KiB, deleted since the last backup)")
		line("  C:/Users/demo/Documents/scan.pdf  (2.1 MiB, deleted since the last backup)  [probably moved to D:/Scans/scan.pdf]")
		line("3 files (2.2 MiB) from 1 snapshot(s)")
		if has("-dry-run") {
			return nil
		}
		pause()
		line("  50%  budget-2026.xlsx")
		pause()
		line(" 100%  notes/old-plan.docx")
		line("Restored 2 files (115 KiB); 0 differences found when checked against the backup.")
	case has("-rollback"):
		line(`Comparing "Documents" with its backup of 2026-11-09 02:10...`)
		pause()
		line("  missing now: 2 files (115 KiB)")
		line("  changed since: 5 files (3.4 MiB)")
		line("  added since: 1 files (12 KiB)")
		line("  unchanged: 1204")
		line("Policy replace: 7 files restored or replaced, 0 removed; everything replaced or removed is kept in C:/Users/demo/.salvanza-rollback for Undo.")
		if has("-dry-run") {
			return nil
		}
		pause()
		line("  60%  restoring 7 files")
		pause()
		line("Rolled back 7 files; the replaced files are kept for Undo in the desktop app.")
	default:
		return errors.New("the demo has no such tool")
	}
	return nil
}
