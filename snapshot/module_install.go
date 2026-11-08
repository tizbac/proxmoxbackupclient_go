package snapshot

// The snapshot module installer: the app's "Install it for me" and
// "salvanza-cli install-snapshot-module" run the elastio-snap installer the
// Linux packages put in /usr/share/<product>/elastio-snap
// (packaging/linux/elastio-snap/install-elastio-snap.sh, with the module's
// source bundled beside it). It installs the build tools and kernel headers,
// builds the module with DKMS, deals with Secure Boot and loads it.

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
)

// ModuleInstallExitNeedsRestart is the installer's exit code when the module
// is installed but Secure Boot loads it only after a restart, at which the
// user approves its signing key.
const ModuleInstallExitNeedsRestart = 10

// moduleInstallPendingFile is left by the installer while a signing key waits
// for that approval; /run is emptied by the restart.
const moduleInstallPendingFile = "/run/elastio-snap-mok-pending"

// secureBootVar is the firmware's SecureBoot variable (EFI global variable
// GUID).
const secureBootVar = "/sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c"

// Why the installer cannot be offered (ModuleInstallStatus.Reason).
const (
	ModuleInstallNotLinux     = "not-linux"
	ModuleInstallNoInstaller  = "no-installer"
	ModuleInstallDistribution = "distribution"
)

// ModuleInstallStatus is what the app shows before the install.
type ModuleInstallStatus struct {
	// Available: the installer is there and supports this distribution.
	Available bool `json:"available"`
	// Reason says why not (ModuleInstallNotLinux, ...) when !Available.
	Reason string `json:"reason,omitempty"`
	// Script is the installer's path.
	Script string `json:"script,omitempty"`
	// SecureBoot: the firmware has Secure Boot on, so the user chooses a
	// one-time password to approve the signing key with after a restart.
	SecureBoot bool `json:"secure_boot"`
	// RestartPending: an install has asked for the key to be enrolled, and
	// the computer has not restarted since.
	RestartPending bool `json:"restart_pending"`
}

// ModuleInstallerPath is where the packages of the product named fileBase
// ("salvanza") put the installer. It is a Linux path on every OS, so path,
// not filepath, builds and checks it.
func ModuleInstallerPath(fileBase string) string {
	return path.Join("/usr/share", fileBase, "elastio-snap", "install-elastio-snap.sh")
}

// InspectModuleInstall reports whether the installer at script can run on
// this computer, and the Secure Boot state.
func InspectModuleInstall(script string) ModuleInstallStatus {
	st := ModuleInstallStatus{Script: script}
	if runtime.GOOS != "linux" {
		st.Reason = ModuleInstallNotLinux
		return st
	}
	st.SecureBoot = secureBootOn(secureBootVar)
	if _, err := os.Stat(moduleInstallPendingFile); err == nil {
		st.RestartPending = true
	}
	if fi, err := os.Stat(script); err != nil || !fi.Mode().IsRegular() {
		st.Reason = ModuleInstallNoInstaller
		return st
	}
	osRelease, _ := os.ReadFile("/etc/os-release")
	if distroFamily(string(osRelease)) == "" {
		st.Reason = ModuleInstallDistribution
		return st
	}
	st.Available = true
	return st
}

// secureBootOn reads the firmware's SecureBoot variable: four bytes of
// attributes, then 1 when it is on. No variable (BIOS start, no efivarfs)
// means off.
func secureBootOn(path string) bool {
	b, err := os.ReadFile(path) // #nosec G304 -- the fixed efivarfs path
	return err == nil && len(b) >= 5 && b[4] == 1
}

// distroFamily is "debian", "fedora" or "arch" for the distributions the
// installer knows (as its own test of /etc/os-release), else "".
func distroFamily(osRelease string) string {
	var ids []string
	for _, line := range strings.Split(osRelease, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || (k != "ID" && k != "ID_LIKE") {
			continue
		}
		ids = append(ids, strings.Fields(strings.Trim(v, `"'`))...)
	}
	for _, id := range ids {
		switch id {
		case "debian", "ubuntu":
			return "debian"
		}
	}
	for _, id := range ids {
		switch id {
		case "fedora", "rhel", "centos":
			return "fedora"
		}
	}
	for _, id := range ids {
		if id == "arch" {
			return "arch"
		}
	}
	return ""
}

// ModuleInstall is one run of the installer, as root.
type ModuleInstall struct {
	Script string
	// MokPassword, when set, is the one-time password for Secure Boot,
	// handed to the installer on its standard input. Otherwise Stdin is the
	// installer's (mokutil asks on the terminal if it needs one).
	MokPassword string
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

// ErrModuleInstallNeedsRoot: the installer runs as root only.
var ErrModuleInstallNeedsRoot = errors.New("installing the snapshot module needs root")

// Run runs the installer. needsRestart: installed, but Secure Boot loads the
// module only once the user has approved its key at the next start.
func (m ModuleInstall) Run(ctx context.Context) (needsRestart bool, err error) {
	if os.Geteuid() != 0 {
		return false, ErrModuleInstallNeedsRoot
	}
	cmd, err := m.Command(ctx, false)
	if err != nil {
		return false, err
	}
	return ModuleInstallResult(cmd.Run())
}

// Command is the installer's command line with its input and output. elevate
// runs it through pkexec, whose password dialog then names the installer
// itself (the packages install it executable); otherwise bash runs it, which
// needs root already.
func (m ModuleInstall) Command(ctx context.Context, elevate bool) (*exec.Cmd, error) {
	if !path.IsAbs(m.Script) {
		return nil, fmt.Errorf("the installer's path is not absolute: %q", m.Script)
	}
	var flags []string
	if m.MokPassword != "" {
		flags = append(flags, "--mok-password-stdin")
	}
	var cmd *exec.Cmd
	if elevate {
		pkexec, err := exec.LookPath("pkexec")
		if err != nil {
			return nil, errors.New("asking for administrator rights needs pkexec (polkit), which is not installed")
		}
		// #nosec G204 -- pkexec asks for an administrator's password before it
		// runs the installer the package put in place
		cmd = exec.CommandContext(ctx, pkexec, append([]string{m.Script}, flags...)...)
	} else {
		// #nosec G204 -- bash runs the installer the package put in place (or
		// the one an administrator named on the command line)
		cmd = exec.CommandContext(ctx, "/bin/bash", append([]string{m.Script}, flags...)...)
	}
	cmd.Stdout, cmd.Stderr = m.Stdout, m.Stderr
	if m.MokPassword != "" {
		cmd.Stdin = strings.NewReader(m.MokPassword + "\n")
	} else {
		cmd.Stdin = m.Stdin
	}
	return cmd, nil
}

// ModuleInstallResult turns the installer's exit (cmd.Run or cmd.Wait) into
// Run's answer.
func ModuleInstallResult(err error) (bool, error) {
	if err == nil {
		return false, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ee.ExitCode() == ModuleInstallExitNeedsRestart {
			return true, nil
		}
		return false, fmt.Errorf("the snapshot module installer stopped (exit code %d): its output says why", ee.ExitCode())
	}
	return false, fmt.Errorf("cannot run the snapshot module installer: %w", err)
}

// ScanLines calls line for each line read from r until it ends; for the
// installer's output on its way to the app.
func ScanLines(r io.Reader, line func(string)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line(sc.Text())
	}
}
