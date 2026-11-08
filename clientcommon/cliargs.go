package clientcommon

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

// ToolName is the name the program was started by, without .exe, for the
// usage and messages: a release also ships the tools under older names.
func ToolName() string {
	name := filepath.Base(os.Args[0])
	if ext := filepath.Ext(name); strings.EqualFold(ext, ".exe") {
		name = strings.TrimSuffix(name, ext)
	}
	return name
}

// VersionString is what -version prints: the tool's name, the version the
// build stamped in (main.version, set by make and goreleaser) or else the
// commit it was built from, and the Go version and platform.
func VersionString(tool, version string) string {
	if version == "" {
		version = "dev"
		if bi, ok := debug.ReadBuildInfo(); ok {
			var rev, dirty string
			for _, s := range bi.Settings {
				switch s.Key {
				case "vcs.revision":
					rev = s.Value
				case "vcs.modified":
					if s.Value == "true" {
						dirty = "-dirty"
					}
				}
			}
			if len(rev) >= 8 {
				version = "dev-" + rev[:8] + dirty
			}
		}
	}
	return fmt.Sprintf("%s %s (%s, %s/%s)", tool, version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// PrintUsage writes -help's text: how to run the tool, what it does, its
// options and where the manual is.
func PrintUsage(w io.Writer, fs *flag.FlagSet, tool, summary string) {
	_, _ = fmt.Fprintf(w, "Usage: %s [options]\n\n%s\n\nOptions:\n", tool, summary)
	out := fs.Output()
	fs.SetOutput(w)
	fs.PrintDefaults()
	fs.SetOutput(out)
	_, _ = io.WriteString(w, "\nOptions can also come from a JSON file (-config) and the PBS_* environment\nvariables. The command line manual is docs/manual/CLI.md in the source.\n")
}

// ParseFlags parses a backup tool's command line (ParseArgs). -help or -h
// prints the usage on standard output and exits 0. Anything wrong is printed
// on standard error with how to get the usage, and exits 1: flag's own exit
// code for a bad option is 2, which the tools use for "another backup job is
// running".
func ParseFlags(fs *flag.FlagSet, args []string, summary string, pathFlags ...string) {
	err := ParseArgs(fs, args, pathFlags...)
	if errors.Is(err, flag.ErrHelp) {
		PrintUsage(os.Stdout, fs, ToolName(), summary)
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\nRun %s -help for the options.\n", err, ToolName())
		os.Exit(1)
	}
}

// ParseArgs parses args into fs and returns flag.ErrHelp for -help or -h,
// or an error saying what is wrong:
//
//   - an option the tool does not have, or a value that does not parse;
//   - an argument that is not an option. Parsing stops at the first one, so
//     every option after it would be left out silently. The usual causes get
//     a hint: a \ ending a line outside bash (tizbac/proxmoxbackupclient_go#55,
//     where only the program's name was run and every option was "missing"),
//     and a value after a switch such as -novss;
//   - on Windows, a quote in the value of one of pathFlags. A path ending in
//     \ inside quotes ("C:\") passes a quote and everything after it.
func ParseArgs(fs *flag.FlagSet, args []string, pathFlags ...string) error {
	return parseArgs(fs, args, runtime.GOOS, pathFlags)
}

func parseArgs(fs *flag.FlagSet, args []string, goos string, pathFlags []string) error {
	fs.Init(fs.Name(), flag.ContinueOnError)
	fs.SetOutput(io.Discard) // the caller prints the error, once
	fs.Usage = func() {}
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Empty arguments at the end are let through: a batch file's "%1" with
	// nothing given passes one, and before this check they did no harm.
	// One with options after it still stops the run, as those were lost.
	if fs.NArg() > 0 && !allEmpty(fs.Args()) {
		return strayArgError(fs, args)
	}
	if goos == "windows" {
		var err error
		fs.Visit(func(f *flag.Flag) {
			if err == nil && contains(pathFlags, f.Name) && strings.Contains(f.Value.String(), `"`) {
				err = fmt.Errorf("-%s came through as [%s], with a quote in it: on Windows a path ending in \\ inside quotes, such as \"C:\\\", passes a quote and everything after it. Leave out the quotes (C:\\), double the \\ (\"C:\\\\\"), or leave out the last \\ (\"D:\\Data\")",
					f.Name, f.Value.String())
			}
		})
		return err
	}
	return nil
}

// strayArgError explains the first argument that is not an option.
func strayArgError(fs *flag.FlagSet, args []string) error {
	stray := fs.Arg(0)
	msg := fmt.Sprintf("unexpected argument %q: it and every option after it were not read.", stray)
	if stray == `\` {
		return errors.New(msg + " A \\ at the end of a line continues a command in bash and other Unix shells only: in PowerShell end the line with a backtick (`), in cmd with ^, or put the whole command on one line")
	}
	if i := len(args) - fs.NArg(); i > 0 {
		prev := strings.TrimLeft(args[i-1], "-")
		if f := fs.Lookup(prev); f != nil && isBoolFlag(f) {
			return fmt.Errorf("%s -%s is a switch and takes no value: write -%s on its own, or -%s=%s", msg, f.Name, f.Name, f.Name, stray)
		}
	}
	return errors.New(msg + " A value with spaces needs quotes, and an option with several values needs its name before each one")
}

func allEmpty(args []string) bool {
	for _, a := range args {
		if a != "" {
			return false
		}
	}
	return true
}

func isBoolFlag(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
