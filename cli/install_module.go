package main

// install-snapshot-module: builds, installs and loads the elastio-snap
// snapshot module with the installer the Linux packages bundle
// (snapshot.ModuleInstall), as the app's "Install it for me" does.

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"

	"snapshot"
)

func installSnapshotModule() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	script := fs.String("script", snapshot.ModuleInstallerPath("salvanza"), "the installer to run")
	pwStdin := fs.Bool("mok-password-stdin", false, "with Secure Boot on, read the one-time password for the signing key from the first line of standard input (8 to 16 characters); without it the password is asked for on the terminal")
	fs.Usage = func() {
		out := fs.Output()
		_, _ = fmt.Fprintf(out, "Usage: sudo %s [options]\n\n", os.Args[0])
		_, _ = fmt.Fprintln(out, "Builds the elastio-snap snapshot module for the running kernel with DKMS,")
		_, _ = fmt.Fprintln(out, "installs it and loads it now and at every start, so machine backups read")
		_, _ = fmt.Fprintln(out, "mounted disks through a snapshot. It installs the build tools and kernel")
		_, _ = fmt.Fprintln(out, "headers first (apt or dnf). With Secure Boot on, the signing key is approved")
		_, _ = fmt.Fprintln(out, "once at the next start: exit code 10 says a restart is needed for that.")
		_, _ = fmt.Fprintln(out, "Debian, Ubuntu, Linux Mint, Fedora and the RHEL family.")
		_, _ = fmt.Fprintln(out)
		fs.PrintDefaults()
	}
	_ = fs.Parse(os.Args[1:])

	if runtime.GOOS != "linux" {
		fmt.Fprintln(os.Stderr, "install-snapshot-module is for Linux: Windows has its own snapshots (VSS)")
		os.Exit(1)
	}
	st := snapshot.InspectModuleInstall(*script)
	switch st.Reason {
	case snapshot.ModuleInstallNoInstaller:
		fmt.Fprintf(os.Stderr, "the installer is not at %s: this package does not include it (see the GUI manual's section \"Linux snapshots: installing elastio-snap or dattobd\")\n", *script)
		os.Exit(1)
	case snapshot.ModuleInstallDistribution:
		fmt.Fprintln(os.Stderr, "the installer knows Debian, Ubuntu, Linux Mint, Fedora and the RHEL family only: see the GUI manual's section \"Linux snapshots: installing elastio-snap or dattobd\" for the manual steps")
		os.Exit(1)
	}

	m := snapshot.ModuleInstall{Script: *script, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	if *pwStdin {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			fmt.Fprintln(os.Stderr, "-mok-password-stdin: no password on standard input")
			os.Exit(1)
		}
		m.MokPassword = trimNewline(line)
		if m.MokPassword == "" {
			fmt.Fprintln(os.Stderr, "-mok-password-stdin: the password is empty")
			os.Exit(1)
		}
	}
	needsRestart, err := m.Run(context.Background())
	switch {
	case errors.Is(err, snapshot.ErrModuleInstallNeedsRoot):
		fmt.Fprintln(os.Stderr, "run it as root: sudo salvanza-cli install-snapshot-module")
		os.Exit(1)
	case err != nil:
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	case needsRestart:
		os.Exit(snapshot.ModuleInstallExitNeedsRestart)
	}
}

// trimNewline removes the line end a password typed or piped in carries.
func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
