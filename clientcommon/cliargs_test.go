package clientcommon

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

// testFlags is a small set like a backup tool's: a path, a list and a switch.
func testFlags() (*flag.FlagSet, *string, *bool) {
	fs := flag.NewFlagSet("tool", flag.ExitOnError)
	dir := fs.String("backupdir", "", "folder")
	novss := fs.Bool("novss", false, "no snapshot")
	fs.String("secret", "", "secret")
	return fs, dir, novss
}

func TestParseArgsGood(t *testing.T) {
	fs, dir, novss := testFlags()
	if err := parseArgs(fs, []string{"-backupdir", `C:\Data`, "-novss"}, "windows", []string{"backupdir"}); err != nil {
		t.Fatal(err)
	}
	if *dir != `C:\Data` || !*novss {
		t.Errorf("backupdir %q, novss %v", *dir, *novss)
	}
}

func TestParseArgsHelp(t *testing.T) {
	for _, h := range []string{"-help", "-h", "--help"} {
		fs, _, _ := testFlags()
		if err := parseArgs(fs, []string{h}, "linux", nil); !errors.Is(err, flag.ErrHelp) {
			t.Errorf("%s: %v, want flag.ErrHelp", h, err)
		}
	}
}

// An option the tool does not have is an error, not an exit of flag's own
// (code 2, which means "another backup job is running" here).
func TestParseArgsUnknownOption(t *testing.T) {
	fs, _, _ := testFlags()
	err := parseArgs(fs, []string{"-backupdirr", "x"}, "linux", nil)
	if err == nil || !strings.Contains(err.Error(), "backupdirr") {
		t.Errorf("err %v, want it to name -backupdirr", err)
	}
}

// tizbac/proxmoxbackupclient_go#55: a command written with bash's \ line
// endings, run where \ does not continue the line. Parsing stops at the \, so
// every option after it is lost; it must be an error that says why.
func TestParseArgsBackslashLine(t *testing.T) {
	fs, dir, _ := testFlags()
	err := parseArgs(fs, []string{`\`, "-backupdir", "/data"}, "windows", nil)
	if err == nil {
		t.Fatal("no error for a stray \\")
	}
	for _, want := range []string{`"\\"`, "every option after it", "backtick", "^"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q not in %q", want, err)
		}
	}
	if *dir != "" {
		t.Errorf("backupdir %q read after the stray argument", *dir)
	}
}

// A value after a switch ends the options there too: -novss true.
func TestParseArgsValueAfterSwitch(t *testing.T) {
	fs, _, _ := testFlags()
	err := parseArgs(fs, []string{"-novss", "true", "-secret", "s"}, "linux", nil)
	if err == nil || !strings.Contains(err.Error(), "-novss is a switch") || !strings.Contains(err.Error(), "-novss=true") {
		t.Errorf("err %v, want the switch hint", err)
	}
}

// A batch file's empty "%1" at the end did no harm before and still does
// none; an empty argument with options after it stops the run, as those
// options were dropped.
func TestParseArgsEmptyArgs(t *testing.T) {
	fs, dir, _ := testFlags()
	if err := parseArgs(fs, []string{"-backupdir", "/a", "", ""}, "windows", nil); err != nil || *dir != "/a" {
		t.Errorf("trailing empty arguments: %v, backupdir %q", err, *dir)
	}
	fs, _, _ = testFlags()
	if err := parseArgs(fs, []string{"", "-backupdir", "/a"}, "windows", nil); err == nil {
		t.Error("an empty argument before options: no error")
	}
}

func TestParseArgsOtherStray(t *testing.T) {
	fs, _, _ := testFlags()
	err := parseArgs(fs, []string{"-backupdir", "/a", "/b"}, "linux", nil)
	if err == nil || !strings.Contains(err.Error(), `"/b"`) || !strings.Contains(err.Error(), "its name before each one") {
		t.Errorf("err %v", err)
	}
}

// On Windows "C:\" passes C:" and swallows what follows into the same
// argument; a path option with a quote in it says so. Elsewhere a quote is a
// legal file name character, and other options may hold one.
func TestParseArgsQuoteInPath(t *testing.T) {
	args := []string{"-backupdir", `C:" -novss`}
	fs, _, _ := testFlags()
	err := parseArgs(fs, args, "windows", []string{"backupdir"})
	if err == nil || !strings.Contains(err.Error(), `[C:" -novss]`) || !strings.Contains(err.Error(), `"C:\\"`) {
		t.Errorf("windows: err %v", err)
	}
	fs, _, _ = testFlags()
	if err := parseArgs(fs, args, "linux", []string{"backupdir"}); err != nil {
		t.Errorf("linux: %v", err)
	}
	fs, _, _ = testFlags()
	if err := parseArgs(fs, []string{"-secret", `a"b`}, "windows", []string{"backupdir"}); err != nil {
		t.Errorf("a quote in a secret: %v", err)
	}
}

func TestPrintUsage(t *testing.T) {
	fs, _, _ := testFlags()
	var b bytes.Buffer
	PrintUsage(&b, fs, "proxmoxbackup-directory", "Backs up folders.")
	for _, want := range []string{"Usage: proxmoxbackup-directory [options]", "Backs up folders.", "-backupdir", "-novss", "-config"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("%q not in the usage:\n%s", want, b.String())
		}
	}
}

func TestVersionString(t *testing.T) {
	if v := VersionString("tool", "2.0.0"); !strings.HasPrefix(v, "tool 2.0.0 (go") {
		t.Errorf("stamped: %q", v)
	}
	if v := VersionString("tool", ""); !strings.HasPrefix(v, "tool dev") {
		t.Errorf("unstamped: %q", v)
	}
}
