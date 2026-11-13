package clientcommon

import (
	"io"
	"os"
	"strings"
	"testing"

	"pbscommon"
)

// captureStdout runs f and returns what it printed on standard output.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	defer func() { os.Stdout = old }()
	f()
	_ = w.Close()
	os.Stdout = old
	return <-done
}

// An encrypted run of the command-line tools writes what version 2.0 wrote,
// and says so, unless it is asked to compress; a run without a key has
// nothing to say and stays without one. (What the two settings write is
// tested with the key itself, in pbscommon.)
func TestBackupCrypt(t *testing.T) {
	out := captureStdout(t, func() {
		if got := BackupCrypt(nil, false); got != nil {
			t.Error("no key became a key")
		}
		if got := BackupCrypt(nil, true); got != nil {
			t.Error("no key became a key with -compress-encrypted")
		}
	})
	if out != "" {
		t.Errorf("a run without a key printed %q", out)
	}

	crypt, err := LoadCryptConfig(officialPBKDF2Key, officialPBKDF2Passphrase)
	if err != nil {
		t.Fatal(err)
	}

	var off, on *pbscommon.CryptConfig
	out = captureStdout(t, func() { off = BackupCrypt(crypt, false) })
	if off == nil || off.Compresses() {
		t.Fatal("without -compress-encrypted the run compresses encrypted chunks")
	}
	for _, want := range []string{"uncompressed", "version 2.0 and earlier", "-compress-encrypted", "2.1 or later"} {
		if !strings.Contains(out, want) {
			t.Errorf("the line of a run without -compress-encrypted lacks %q: %q", want, out)
		}
	}
	if strings.Count(out, "\n") != 1 {
		t.Errorf("want one line, got %q", out)
	}

	out = captureStdout(t, func() { on = BackupCrypt(crypt, true) })
	if on == nil || !on.Compresses() {
		t.Fatal("-compress-encrypted does not compress")
	}
	if out != "" {
		t.Errorf("-compress-encrypted printed %q", out)
	}
	if off.Fingerprint() != officialPBKDF2Fingerprint || on.Fingerprint() != officialPBKDF2Fingerprint {
		t.Error("the key changed with the option")
	}
	// the key the caller holds is left as it was
	if !crypt.Compresses() {
		t.Error("BackupCrypt changed the key it was given")
	}
}
