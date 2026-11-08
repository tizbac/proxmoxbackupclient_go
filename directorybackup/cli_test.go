package directorybackup

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
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
	cmd.Env = append(cmd.Env, "DIRECTORYBACKUP_RUN_MAIN=1")
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

// tizbac/proxmoxbackupclient_go#55: no options at all names each one that is
// needed, and exits 1.
func TestCLIMissingOptionsNamed(t *testing.T) {
	_, stderr, code := runMain(t)
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	want := "Missing options: -baseurl, -authid and -secret, or -pbsusername, -datastore, -backupdir (or -backupstream)"
	if !strings.Contains(stderr, want) || !strings.Contains(stderr, "-help") {
		t.Errorf("stderr:\n%s\nwant %q and a pointer to -help", stderr, want)
	}
}

// The command of #55 as it reaches the program from cmd or PowerShell: the
// name and a \, the options on lines of their own that never get there.
func TestCLIBackslashLine(t *testing.T) {
	_, stderr, code := runMain(t, `\`)
	if code != 1 || !strings.Contains(stderr, "continues a command in bash") {
		t.Errorf("exit %d, stderr:\n%s", code, stderr)
	}
}

// A mistyped option exits 1, not flag's 2 (2 is "another backup job is
// running").
func TestCLIUnknownOptionExit1(t *testing.T) {
	_, stderr, code := runMain(t, "-backupdirr", "/x")
	if code != 1 || !strings.Contains(stderr, "-backupdirr") {
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
	if code != 0 || !strings.Contains(stdout, "Usage: ") || !strings.Contains(stdout, "-backupdir") || !strings.Contains(stdout, "-version") {
		t.Errorf("exit %d, stdout:\n%s", code, stdout)
	}
}

func TestMissingOptions(t *testing.T) {
	full := Config{BaseURL: "https://pbs:8007", AuthID: "a@pbs!t", Secret: "s", Datastore: "ds", BackupSourceDir: "/data"}
	if m := full.missing(); len(m) != 0 {
		t.Errorf("complete config: missing %v", m)
	}
	for name, c := range map[string]struct {
		edit func(*Config)
		want string
	}{
		"no secret":     {func(c *Config) { c.Secret = "" }, "-secret (for -authid)"},
		"no authid":     {func(c *Config) { c.AuthID = "" }, "-authid (for -secret)"},
		"user login":    {func(c *Config) { c.AuthID, c.Secret, c.PBSUsername = "", "", "u@pbs" }, ""},
		"stream":        {func(c *Config) { c.BackupSourceDir, c.BackupStreamName = "", "db.sql" }, ""},
		"no source":     {func(c *Config) { c.BackupSourceDir = "" }, "-backupdir (or -backupstream)"},
		"no datastore":  {func(c *Config) { c.Datastore = "" }, "-datastore"},
		"no server URL": {func(c *Config) { c.BaseURL = "" }, "-baseurl"},
	} {
		cfg := full
		c.edit(&cfg)
		got := strings.Join(cfg.missing(), ", ")
		if got != c.want {
			t.Errorf("%s: missing %q, want %q", name, got, c.want)
		}
	}
}

// Every problem is listed, not only the first.
func TestProblemsListsAll(t *testing.T) {
	c := Config{ChangeDetectionMode: "fast", SMTP: &SMTPConfig{}}
	p := c.problems()
	if len(p) != 3 || !strings.HasPrefix(p[0], "Missing options:") || !strings.Contains(p[1], "fast") || !strings.Contains(p[2], "smtp") {
		t.Errorf("problems %q", p)
	}
}
