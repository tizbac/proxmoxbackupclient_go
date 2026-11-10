// salvanza-cli is the command line of Salvanza for Proxmox Backup Server: one
// program with a subcommand for each job, which were separate tools before
// the rename (proxmoxbackup-directory, proxmoxbackup-machine,
// proxmoxbackup-nbd). Each subcommand takes the options its tool took.
//
// Started under one of the old names (a link or a copy named
// proxmoxbackup-directory, or the .cmd stand-ins the Windows installer puts
// beside it), it runs that tool's subcommand, so scripts written for the old
// tools keep working.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"clientcommon"
	"directorybackup"
	"machinebackup"
	tui "salvanza-tui"
	"snapshot"
)

// version is set by the build (-ldflags "-X main.version=..."); -version
// prints it, as each subcommand's -version does.
var version string

// command is one subcommand: its name, a line for the help, what runs it,
// and the names the program had before (aliases).
type command struct {
	name, summary string
	run           func()
	aliases       []string
}

func commands() []command {
	return []command{
		{"backup-folder", "back up folders, or standard input, as a host snapshot", directorybackup.Main,
			[]string{"proxmoxbackup-directory", "pbsdirectorybackup", "directorybackup"}},
		{"backup-disk", "back up whole disks as a host or vm snapshot", machinebackup.Main,
			[]string{"proxmoxbackup-machine", "pbsmachinebackup", "machinebackup"}},
		{"attach-nbd", "attach a disk image from PBS as a read-only /dev/nbdN (Linux)", attachNBD,
			[]string{"proxmoxbackup-nbd", "pbsnbd"}},
		{"install-snapshot-module", "build and load elastio-snap for disk snapshots (Linux, root)", installSnapshotModule, nil},
		{"tui", "text window for Backup Sets, running backups and history (administrator or root)", runTUI, nil},
	}
}

// runTUI is the text-mode window: it talks to the local Salvanza service.
func runTUI() {
	os.Exit(tui.Run(os.Args[1:], os.Stdout, os.Stderr, version))
}

func main() {
	directorybackup.SetVersion(version)
	machinebackup.SetVersion(version)

	// Started under an old tool's name: that tool's subcommand.
	if c, ok := byAlias(programName(os.Args[0])); ok {
		start(c)
		return
	}

	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(1)
	}
	sub := os.Args[1]
	switch sub {
	case "-version", "--version", "version":
		fmt.Println(clientcommon.VersionString("salvanza-cli", version))
		return
	case "help", "-help", "--help", "-h":
		if len(os.Args) > 2 {
			if c, ok := byName(os.Args[2]); ok {
				os.Args = []string{"salvanza-cli " + c.name, "-help"}
				c.run()
				return
			}
		}
		usage(os.Stdout)
		return
	}
	c, ok := byName(sub)
	if !ok {
		fmt.Fprintf(os.Stderr, "salvanza-cli: unknown command %q\n\n", sub)
		usage(os.Stderr)
		os.Exit(1)
	}
	// The subcommand sees its own options only, and names itself
	// "salvanza-cli <subcommand>" in its help and messages.
	os.Args = append([]string{"salvanza-cli " + c.name}, os.Args[2:]...)
	start(c)
}

// start runs a subcommand. The two backups remove their snapshots when they
// are stopped with Ctrl+C or a signal (Linux; attach-nbd has its own
// handling, and install-snapshot-module takes none).
func start(c command) {
	if c.name == "backup-folder" || c.name == "backup-disk" {
		snapshot.CleanupOnSignal()
	}
	c.run()
}

// programName is the name the program was started under, lower case and
// without its folder or .exe. Both separators count, whichever system this
// is, so a Windows path names the same program everywhere.
func programName(arg0 string) string {
	n := arg0
	if i := strings.LastIndexAny(n, `/\`); i >= 0 {
		n = n[i+1:]
	}
	return strings.TrimSuffix(strings.ToLower(n), ".exe")
}

func byName(name string) (command, bool) {
	for _, c := range commands() {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}

func byAlias(name string) (command, bool) {
	for _, c := range commands() {
		for _, a := range c.aliases {
			if a == name {
				return c, true
			}
		}
	}
	return command{}, false
}

func usage(w io.Writer) {
	var b strings.Builder
	b.WriteString("Usage: salvanza-cli <command> [options]\n\n")
	b.WriteString("Salvanza for Proxmox Backup Server, on the command line.\n\nCommands:\n")
	for _, c := range commands() {
		_, _ = fmt.Fprintf(&b, "  %-24s %s\n", c.name, c.summary)
	}
	b.WriteString("\n'salvanza-cli <command> -help' lists a command's options; 'salvanza-cli -version' prints the version.\n")
	b.WriteString("The tools' earlier names (proxmoxbackup-directory, -machine, -nbd) run the same commands.\n")
	_, _ = io.WriteString(w, b.String())
}
