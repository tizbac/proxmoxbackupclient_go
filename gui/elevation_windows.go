//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
)

// requestElevationWindows relaunches this GUI elevated (UAC, verb "runas") and
// stops the current, unelevated copy — the Windows counterpart of the Linux
// pkexec/sudo relaunch, so both platforms share the same "ask now, or keep
// running unprivileged" contract. The manifest is asInvoker, which is what
// makes a refusal possible at all: declined UAC simply leaves this instance
// running with its current (unelevated) privileges, and unavailable elevation
// surfaces the same way.
func requestElevationWindows() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	// Drop the single-instance mutex BEFORE launching: the elevated copy runs
	// CheckSingleInstance at startup and would otherwise find THIS instance,
	// activate it and exit — nothing would ever come up elevated.
	ReleaseSingleInstance()

	if _, err := shellExecuteRunas(exe, ""); err != nil {
		// Nothing was launched: re-acquire the lock, we stay the running
		// instance in whatever privileges we already have.
		CheckSingleInstance()
		if errors.Is(err, ErrElevationDeclined) {
			return fmt.Errorf("UAC prompt was declined: still running without administrator rights: %w", err)
		}
		return err
	}

	// The elevated copy takes over from here; leaving avoids two windows
	// fighting over the tray/mutex/config.
	os.Exit(0)
	return nil // unreachable, kept for the compiler
}
