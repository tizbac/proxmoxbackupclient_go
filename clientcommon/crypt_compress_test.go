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

// An encrypted run of the command-line tools compresses before it encrypts,
// as proxmox-backup-client does, unless -no-compress-encrypted asks for what
// version 2.0 wrote; either way it says in one line what it writes and which
// versions restore it. A run without a key has nothing to say and stays
// without one. (What the two settings write is tested with the key itself,
// in pbscommon.)
func TestBackupCrypt(t *testing.T) {
	out := captureStdout(t, func() {
		if got := BackupCrypt(nil, false); got != nil {
			t.Error("no key became a key with -no-compress-encrypted")
		}
		if got := BackupCrypt(nil, true); got != nil {
			t.Error("no key became a key")
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
	out = captureStdout(t, func() { on = BackupCrypt(crypt, true) })
	if on == nil || !on.Compresses() {
		t.Fatal("a run that was not told otherwise does not compress encrypted chunks")
	}
	for _, want := range []string{"compressed before", "2.1 or later", "-no-compress-encrypted", "as version 2.0 did"} {
		if !strings.Contains(out, want) {
			t.Errorf("the line of a run that compresses lacks %q: %q", want, out)
		}
	}
	if strings.Count(out, "\n") != 1 {
		t.Errorf("want one line, got %q", out)
	}

	out = captureStdout(t, func() { off = BackupCrypt(crypt, false) })
	if off == nil || off.Compresses() {
		t.Fatal("-no-compress-encrypted compresses encrypted chunks")
	}
	for _, want := range []string{"uncompressed", "-no-compress-encrypted", "version 2.0 and earlier"} {
		if !strings.Contains(out, want) {
			t.Errorf("the line of a run with -no-compress-encrypted lacks %q: %q", want, out)
		}
	}
	if strings.Contains(out, "2.1 or later") {
		t.Errorf("the line of a run with -no-compress-encrypted speaks of 2.1: %q", out)
	}
	if strings.Count(out, "\n") != 1 {
		t.Errorf("want one line, got %q", out)
	}
	if off.Fingerprint() != officialPBKDF2Fingerprint || on.Fingerprint() != officialPBKDF2Fingerprint {
		t.Error("the key changed with the option")
	}
	// the key the caller holds is left as it was
	if !crypt.Compresses() {
		t.Error("BackupCrypt changed the key it was given")
	}
}
