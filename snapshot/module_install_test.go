package snapshot

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDistroFamily(t *testing.T) {
	cases := map[string]string{
		"NAME=\"Linux Mint\"\nID=linuxmint\nID_LIKE=\"ubuntu debian\"\n": "debian",
		"ID=ubuntu\nID_LIKE=debian\n":                                    "debian",
		"ID=debian\n":                                                    "debian",
		"ID=fedora\n":                                                    "fedora",
		"ID=\"rocky\"\nID_LIKE=\"rhel centos fedora\"\n":                 "fedora",
		"ID=arch\n":                                                      "arch",
		"ID=manjaro\nID_LIKE=arch\n":                                     "arch",
		"ID=endeavouros\nID_LIKE=arch\n":                                 "arch",
		"ID=archarm\nID_LIKE=arch\n":                                     "arch",
		"ID=opensuse-tumbleweed\nID_LIKE=\"opensuse suse\"\n":            "",
		"": "",
		// Only ID and ID_LIKE count, not a name that happens to say debian.
		"NAME=debian-like\nID=void\n": "",
	}
	for in, want := range cases {
		if got := distroFamily(in); got != want {
			t.Errorf("distroFamily(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSecureBootOn(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, b []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if !secureBootOn(write("on", []byte{6, 0, 0, 0, 1})) {
		t.Error("attributes then 1: want on")
	}
	if secureBootOn(write("off", []byte{6, 0, 0, 0, 0})) {
		t.Error("attributes then 0: want off")
	}
	if secureBootOn(write("short", []byte{6, 0, 0, 0})) {
		t.Error("no value byte: want off")
	}
	if secureBootOn(filepath.Join(dir, "missing")) {
		t.Error("no variable: want off")
	}
}

func TestModuleInstallerPath(t *testing.T) {
	if got := ModuleInstallerPath("salvanza"); filepath.ToSlash(got) != "/usr/share/salvanza/elastio-snap/install-elastio-snap.sh" {
		t.Errorf("got %q", got)
	}
}

func TestInspectModuleInstallMissingScript(t *testing.T) {
	st := InspectModuleInstall(filepath.Join(t.TempDir(), "none.sh"))
	if st.Available {
		t.Fatal("no installer: want not available")
	}
	want := ModuleInstallNoInstaller
	if runtime.GOOS != "linux" {
		want = ModuleInstallNotLinux
	}
	if st.Reason != want {
		t.Errorf("reason %q, want %q", st.Reason, want)
	}
}

func TestModuleInstallCommand(t *testing.T) {
	m := ModuleInstall{Script: "/usr/share/salvanza/elastio-snap/install-elastio-snap.sh", MokPassword: "abcd1234"}
	cmd, err := m.Command(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cmd.Args, " "); got != "/bin/bash "+m.Script+" --mok-password-stdin" {
		t.Errorf("args %q", got)
	}
	// The password goes on standard input only, never on the command line.
	for _, a := range cmd.Args {
		if strings.Contains(a, m.MokPassword) {
			t.Fatalf("password on the command line: %q", cmd.Args)
		}
	}
	in, _ := io.ReadAll(cmd.Stdin)
	if string(in) != "abcd1234\n" {
		t.Errorf("stdin %q", in)
	}

	plain := ModuleInstall{Script: m.Script, Stdin: strings.NewReader("tty")}
	cmd, err = plain.Command(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Args) != 2 || cmd.Stdin != plain.Stdin {
		t.Errorf("without a password: args %q, stdin passed through %v", cmd.Args, cmd.Stdin == plain.Stdin)
	}

	if _, err := (ModuleInstall{Script: "install-elastio-snap.sh"}).Command(context.Background(), false); err == nil {
		t.Error("relative path: want an error")
	}
}

func TestModuleInstallResult(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	exit := func(code string) error { return exec.Command("sh", "-c", "exit "+code).Run() }
	if restart, err := ModuleInstallResult(nil); restart || err != nil {
		t.Errorf("exit 0: %v %v", restart, err)
	}
	if restart, err := ModuleInstallResult(exit("10")); !restart || err != nil {
		t.Errorf("exit 10: want restart, got %v %v", restart, err)
	}
	if restart, err := ModuleInstallResult(exit("1")); restart || err == nil || !strings.Contains(err.Error(), "exit code 1") {
		t.Errorf("exit 1: %v %v", restart, err)
	}
	if _, err := ModuleInstallResult(errors.New("no bash")); err == nil {
		t.Error("start failure: want an error")
	}
}
