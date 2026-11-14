package main

import (
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
)

// captureStderr runs f and returns what it wrote to os.Stderr, which is where
// writeDebugLog goes in a test (no log file is open).
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	func() {
		defer func() { os.Stderr = old }()
		f()
	}()
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// A Backup Set's before or after command can hold a password or a token, and
// every user of the computer may read the service's logs (#141): the log says
// that it ran and that it failed, and neither the command nor what it printed.
func TestPostBackupAppLogsNeitherTheCommandNorItsOutput(t *testing.T) {
	command := "echo printed-hunter2-out && echo url-token-s3cr3t >/dev/null && exit 3"
	if runtime.GOOS == "windows" {
		command = "echo printed-hunter2-out & echo url-token-s3cr3t >NUL & exit 3"
	}
	log := captureStderr(t, func() { runPostBackupApp(command, "Nightly", "after") })
	for _, secret := range []string{"hunter2", "s3cr3t", "echo"} {
		if strings.Contains(log, secret) {
			t.Errorf("the log holds %q:\n%s", secret, log)
		}
	}
	for _, want := range []string{`running after-backup application for job "Nightly"`, "failed", "which are not logged"} {
		if !strings.Contains(log, want) {
			t.Errorf("the log lacks %q:\n%s", want, log)
		}
	}
}
