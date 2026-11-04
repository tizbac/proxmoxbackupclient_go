package clientcommon

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"testing"
)

// -shutdown-after given overrides the config file, either way; left out, the
// file's "shutdown-after" stands.
func TestShutdownAfterFlag(t *testing.T) {
	for _, c := range []struct {
		file string
		args []string
		want bool
	}{
		{`{}`, nil, false},
		{`{}`, []string{"-shutdown-after"}, true},
		{`{"shutdown-after": true}`, nil, true},
		{`{"shutdown-after": true}`, []string{"-shutdown-after=false"}, false},
		{`{"shutdown-after": false}`, []string{"-shutdown-after"}, true},
	} {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		apply := ShutdownAfterFlag(fs)
		if err := fs.Parse(c.args); err != nil {
			t.Fatal(err)
		}
		var a AfterRun
		if err := json.Unmarshal([]byte(c.file), &a); err != nil {
			t.Fatal(err)
		}
		apply(&a)
		if a.ShutdownAfter != c.want {
			t.Errorf("file %s, args %v: %v, want %v", c.file, c.args, a.ShutdownAfter, c.want)
		}
	}
}

// ShutdownAfterRun asks for a shutdown only when told to, and a refusal is
// reported, not fatal.
func TestShutdownAfterRun(t *testing.T) {
	calls := 0
	var fail error
	old := shutdownComputer
	shutdownComputer = func(string) (string, error) { calls++; return "a test", fail }
	t.Cleanup(func() { shutdownComputer = old })

	ShutdownAfterRun(AfterRun{})
	if calls != 0 {
		t.Errorf("asked for a shutdown without -shutdown-after")
	}
	ShutdownAfterRun(AfterRun{ShutdownAfter: true})
	if calls != 1 {
		t.Errorf("%d shutdown requests with -shutdown-after, want 1", calls)
	}
	fail = errors.New("refused")
	ShutdownAfterRun(AfterRun{ShutdownAfter: true}) // must return, not exit
	if calls != 2 {
		t.Errorf("%d shutdown requests, want 2", calls)
	}
}
