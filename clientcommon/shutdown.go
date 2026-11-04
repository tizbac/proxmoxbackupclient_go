package clientcommon

import (
	"flag"
	"fmt"
	"os"

	"pbscommon"
)

// AfterRun is what the backup tools do once a backup has run: -shutdown-after,
// or "shutdown-after" in the config file, the app's "Turn off the computer
// after the backup".
type AfterRun struct {
	ShutdownAfter bool `json:"shutdown-after,omitempty"`
}

// ShutdownAfterFlag defines -shutdown-after on fs. The function it returns,
// called after parsing, copies the flag over a when it was given, so it
// overrides the config file (-shutdown-after=false too) and, left out, keeps
// the file's value.
func ShutdownAfterFlag(fs *flag.FlagSet) func(a *AfterRun) {
	on := fs.Bool("shutdown-after", false, "Turn the computer off once the backup has run, whether it worked or not, after the e-mail if one is sent (Windows: through the Windows API, or shutdown.exe if that fails; Linux: shutdown -h now). Not when the run stops before it starts (wrong options, another backup running). Needs the right to shut down: an administrator, SYSTEM or root")
	return func(a *AfterRun) {
		fs.Visit(func(f *flag.Flag) {
			if f.Name == "shutdown-after" {
				a.ShutdownAfter = *on
			}
		})
	}
}

// shutdownComputer is pbscommon.ShutdownComputer; tests replace it.
var shutdownComputer = pbscommon.ShutdownComputer

// ShutdownAfterRun turns the computer off when a asks, once the backup has run
// and its e-mail has gone; call it just before the tool exits. It says what
// it does. A failure is reported on stderr and does not change the exit code:
// the backup's result is what the exit code reports.
func ShutdownAfterRun(a AfterRun) {
	if !a.ShutdownAfter {
		return
	}
	fmt.Println("Turning the computer off, as -shutdown-after asks.")
	method, err := shutdownComputer("Shutting down after a backup, as -shutdown-after asks.")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot turn the computer off: "+err.Error())
		return
	}
	fmt.Println("Shutdown requested through " + method + ".")
}
