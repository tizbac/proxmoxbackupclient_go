package machinebackup

import (
	"bytes"
	"errors"
	"machinebackuplib"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// runMain runs this program (Main(), through TestMain) with args, with no
// PBS_* variables from the environment, and returns its output and exit code.
func runMain(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "PBS_") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	cmd.Env = append(cmd.Env, "MACHINEBACKUP_RUN_MAIN=1")
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	case err != nil:
		t.Fatal(err)
	}
	return o.String(), e.String(), code
}

// tizbac/proxmoxbackupclient_go#55: each missing option is named; exit 1.
func TestCLIMissingOptionsNamed(t *testing.T) {
	_, stderr, code := runMain(t)
	want := "Missing options: -baseurl, -authid and -secret, or -pbsusername, -datastore, -backupdev"
	if code != 1 || !strings.Contains(stderr, want) {
		t.Errorf("exit %d, stderr:\n%s\nwant %q", code, stderr, want)
	}
}

func TestCLIBackslashLine(t *testing.T) {
	_, stderr, code := runMain(t, `\`, "-backupdev", "/dev/sda")
	if code != 1 || !strings.Contains(stderr, "continues a command in bash") {
		t.Errorf("exit %d, stderr:\n%s", code, stderr)
	}
}

// A mistyped option exits 1, not flag's 2 ("another backup job is running").
func TestCLIUnknownOptionExit1(t *testing.T) {
	_, stderr, code := runMain(t, "-backupdevice", "/dev/sda")
	if code != 1 || !strings.Contains(stderr, "-backupdevice") {
		t.Errorf("exit %d, stderr:\n%s", code, stderr)
	}
}

func TestCLIVersion(t *testing.T) {
	stdout, _, code := runMain(t, "-version")
	if code != 0 || !strings.Contains(stdout, " dev") || !strings.Contains(stdout, "(go") {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
}

func TestCLIHelp(t *testing.T) {
	stdout, _, code := runMain(t, "-help")
	if code != 0 || !strings.Contains(stdout, "Usage: ") || !strings.Contains(stdout, "-backupdev") {
		t.Errorf("exit %d, stdout:\n%s", code, stdout)
	}
}

// -liveread is taken on every system; on Windows, where VSS always reads a
// volume in use, the run says it has no effect. The run then fails to
// connect (nothing listens on port 1), which is not what is tested here.
func TestCLILiveReadFlag(t *testing.T) {
	stdout, stderr, code := runMain(t, "-liveread", "-baseurl", "https://127.0.0.1:1", "-certfingerprint", "00:11",
		"-authid", "a@pbs!t", "-secret", "s", "-datastore", "ds", "-backupdev", "nosuchdisk.img")
	if code == 2 {
		t.Skip("another backup job holds the process lock on this machine")
	}
	if strings.Contains(stderr, "flag provided but not defined") || strings.Contains(stderr, "Missing options") {
		t.Fatalf("-liveread not taken: exit %d\n%s", code, stderr)
	}
	note := strings.Contains(stdout, "-liveread has no effect on Windows")
	if note != (runtime.GOOS == "windows") {
		t.Errorf("Windows note shown: %v on %s\n%s", note, runtime.GOOS, stdout)
	}
}

// -systray is still accepted, so old scripts run, and says it does nothing.
// The run then fails to connect (nothing listens on port 1).
func TestCLISysTrayRetired(t *testing.T) {
	stdout, stderr, code := runMain(t, "-systray", "-baseurl", "https://127.0.0.1:1", "-certfingerprint", "00:11",
		"-authid", "a@pbs!t", "-secret", "s", "-datastore", "ds", "-backupdev", "nosuchdisk.img")
	if code == 2 {
		t.Skip("another backup job holds the process lock on this machine")
	}
	if strings.Contains(stderr, "flag provided but not defined") {
		t.Fatalf("-systray refused\n%s", stderr)
	}
	if !strings.Contains(stdout, "-systray does nothing") {
		t.Errorf("no word that -systray does nothing\n%s", stdout)
	}
}

func TestProblems(t *testing.T) {
	full := machinebackuplib.Config{BaseURL: "https://pbs:8007", AuthID: "a@pbs!t", Secret: "s", Datastore: "ds",
		BackupDevices: []string{"/dev/sda"}, BackupType: "host"}
	if p := problems(&full); len(p) != 0 || !full.Valid() {
		t.Errorf("complete config: %q, Valid %v", p, full.Valid())
	}
	for name, c := range map[string]struct {
		edit func(*machinebackuplib.Config)
		want string
	}{
		"ticket":         {func(c *machinebackuplib.Config) { c.AuthID, c.Secret, c.Ticket = "", "", "t" }, ""},
		"no secret":      {func(c *machinebackuplib.Config) { c.Secret = "" }, "Missing options: -secret (for -authid)"},
		"no devices":     {func(c *machinebackuplib.Config) { c.BackupDevices = nil }, "Missing options: -backupdev"},
		"vm":             {func(c *machinebackuplib.Config) { c.BackupType = "vm" }, ""},
		"bad type":       {func(c *machinebackuplib.Config) { c.BackupType = "VM" }, `-type is "VM": use host or vm`},
		"compression":    {func(c *machinebackuplib.Config) { c.CompressionLevel = "best" }, ""},
		"bad level":      {func(c *machinebackuplib.Config) { c.CompressionLevel = "max" }, `-compression is "max": use none, fastest, default, better or best`},
		"no compression": {func(c *machinebackuplib.Config) { c.CompressionLevel = "none" }, ""},
		"smtp no host":   {func(c *machinebackuplib.Config) { c.SMTP = &machinebackuplib.SMTPConfig{Port: "25"} }, `smtp: "host" and "port" are needed`},
	} {
		cfg := full
		c.edit(&cfg)
		got := strings.Join(problems(&cfg), " | ")
		if got != c.want {
			t.Errorf("%s: %q, want %q", name, got, c.want)
		}
		// No problem must mean the library takes the settings too.
		if got == "" && !cfg.Valid() {
			t.Errorf("%s: no problems, but Valid() is false", name)
		}
	}
}
