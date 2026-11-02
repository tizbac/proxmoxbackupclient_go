package main

import (
	"errors"
	"path/filepath"
	"testing"
)

func withPrompter(t *testing.T, f func(path, fp, lastErr string) (string, error)) {
	t.Helper()
	old := passphrasePrompter
	passphrasePrompter = f
	t.Cleanup(func() { passphrasePrompter = old })
}

func TestStoredPassphraseUnlocksWithoutPrompting(t *testing.T) {
	path := writePassphraseKeyFile(t, t.TempDir(), "k.json", "hunter2")
	forgetSessionPassphrase(path)
	withPrompter(t, func(string, string, string) (string, error) {
		t.Fatal("must not prompt when a passphrase is stored")
		return "", nil
	})
	c := &Config{EncryptionKeyFile: path, EncryptionKeyPassphrase: "hunter2"}
	if err := c.loadCryptConfig(); err != nil {
		t.Fatalf("loadCryptConfig: %v", err)
	}
	if c.Crypt == nil {
		t.Fatal("Crypt not set")
	}
}

func TestPromptedPassphraseIsAskedOnceThenRemembered(t *testing.T) {
	path := writePassphraseKeyFile(t, t.TempDir(), "k.json", "hunter2")
	forgetSessionPassphrase(path)
	asked := 0
	withPrompter(t, func(_, fp, _ string) (string, error) {
		asked++
		if fp == "" {
			t.Error("prompter should be told the key fingerprint")
		}
		return "hunter2", nil
	})
	for i := 0; i < 3; i++ {
		c := &Config{EncryptionKeyFile: path}
		if err := c.loadCryptConfig(); err != nil || c.Crypt == nil {
			t.Fatalf("load %d: %v", i, err)
		}
	}
	if asked != 1 {
		t.Fatalf("asked %d times, want 1", asked)
	}
}

func TestWrongPassphraseIsAskedAgainWithTheError(t *testing.T) {
	path := writePassphraseKeyFile(t, t.TempDir(), "k.json", "hunter2")
	forgetSessionPassphrase(path)
	var errs []string
	answers := []string{"nope-nope", "hunter2"}
	withPrompter(t, func(_, _, lastErr string) (string, error) {
		errs = append(errs, lastErr)
		a := answers[0]
		answers = answers[1:]
		return a, nil
	})
	c := &Config{EncryptionKeyFile: path}
	if err := c.loadCryptConfig(); err != nil {
		t.Fatalf("loadCryptConfig: %v", err)
	}
	if len(errs) != 2 || errs[0] != "" || errs[1] == "" {
		t.Fatalf("prompt errors %q, want first empty then a reason", errs)
	}
}

func TestCancelledPromptFailsTheAction(t *testing.T) {
	path := writePassphraseKeyFile(t, t.TempDir(), "k.json", "hunter2")
	forgetSessionPassphrase(path)
	withPrompter(t, func(string, string, string) (string, error) { return "", errors.New("cancelled") })
	c := &Config{EncryptionKeyFile: path}
	if err := c.loadCryptConfig(); err == nil {
		t.Fatal("expected an error after cancel")
	}
}

func TestWrongStoredPassphraseFallsBackToPrompt(t *testing.T) {
	path := writePassphraseKeyFile(t, t.TempDir(), "k.json", "hunter2")
	forgetSessionPassphrase(path)
	withPrompter(t, func(_, _, lastErr string) (string, error) {
		if lastErr == "" {
			t.Error("the prompt should explain the stored passphrase failed")
		}
		return "hunter2", nil
	})
	c := &Config{EncryptionKeyFile: path, EncryptionKeyPassphrase: "stale-stale"}
	if err := c.loadCryptConfig(); err != nil {
		t.Fatalf("loadCryptConfig: %v", err)
	}
}

func TestSaveTimeUnlockNeverPrompts(t *testing.T) {
	path := writePassphraseKeyFile(t, t.TempDir(), "k.json", "hunter2")
	forgetSessionPassphrase(path)
	withPrompter(t, func(string, string, string) (string, error) {
		t.Fatal("saving settings must not prompt")
		return "", nil
	})
	c := &Config{EncryptionKeyFile: path}
	if err := c.unlockCrypt(false); !errors.Is(err, errPassphraseRequired) {
		t.Fatalf("got %v, want errPassphraseRequired", err)
	}
}

func TestMergeStoredPassphrase(t *testing.T) {
	cases := []struct {
		name                          string
		prevPath, prevPass, newPath   string
		typed                         string
		clear                         bool
		want                          string
	}{
		{"keep when blank", "a", "old", "a", "", false, "old"},
		{"typed replaces", "a", "old", "a", "new", false, "new"},
		{"clear wins", "a", "old", "a", "", true, ""},
		{"clear beats typed", "a", "old", "a", "new", true, ""},
		{"new key path drops it", "a", "old", "b", "", false, ""},
		{"new key path keeps typed", "a", "old", "b", "new", false, "new"},
	}
	for _, c := range cases {
		if got := mergeStoredPassphrase(c.prevPath, c.prevPass, c.newPath, c.typed, c.clear); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestSanitizedHidesStoredPassphrase(t *testing.T) {
	c := &Config{EncryptionKeyPassphrase: "hunter2", PBSServers: map[string]*PBSServer{"x": {ID: "x", EncryptionKeyPassphrase: "p4ssw0rd"}}}
	cp := c.sanitized()
	if cp.EncryptionKeyPassphrase != "" || !cp.EncryptionKeyPassphraseSet {
		t.Fatalf("config not sanitized: %+v", cp)
	}
	if s := cp.PBSServers["x"]; s.EncryptionKeyPassphrase != "" || !s.EncryptionKeyPassphraseSet {
		t.Fatalf("server not sanitized: %+v", s)
	}
}

func TestPassphrasePromptRoundTrip(t *testing.T) {
	path := writePassphraseKeyFile(t, t.TempDir(), "k.json", "hunter2")
	var gotID string
	emitted := make(chan struct{}, 1)
	id, ch := beginPassphrasePrompt(path, "fp", "", func(p map[string]any) {
		gotID = p["id"].(string)
		emitted <- struct{}{}
	})
	<-emitted
	if gotID != id {
		t.Fatalf("event id %q != %q", gotID, id)
	}
	if err := submitPassphrase(id, "wrong-pass", false); err == nil {
		t.Fatal("a wrong passphrase must be refused and leave the prompt open")
	}
	if err := submitPassphrase(id, "hunter2", false); err != nil {
		t.Fatalf("right passphrase refused: %v", err)
	}
	pass, err := waitPassphrase(id, ch)
	if err != nil || pass != "hunter2" {
		t.Fatalf("got %q, %v", pass, err)
	}
	if err := submitPassphrase(id, "hunter2", false); err == nil {
		t.Fatal("a finished prompt must not accept another answer")
	}
}

func TestIsUnattendedRun(t *testing.T) {
	a := &App{}
	if a.isUnattendedRun("") || a.isUnattendedRun("missing") {
		t.Fatal("empty or unknown key must not be unattended")
	}
	cases := map[string]bool{"scheduled": true, "startup": true, "manual": false}
	for trigger, want := range cases {
		key := a.registerPendingPostActions(ScheduledJob{ID: "j"}, trigger)
		if got := a.isUnattendedRun(key); got != want {
			t.Fatalf("trigger %q: got %v want %v", trigger, got, want)
		}
		if a.takePendingPostActions(key) == nil {
			t.Fatalf("trigger %q: isUnattendedRun must not consume the entry", trigger)
		}
	}
}

func TestCheckKeyUsableByService(t *testing.T) {
	dir := t.TempDir()
	protected := writePassphraseKeyFile(t, dir, "p.json", "hunter2")
	if err := checkKeyUsableByService(protected, ""); !errors.Is(err, errPassphraseRequired) {
		t.Fatalf("protected key without stored passphrase: want errPassphraseRequired, got %v", err)
	}
	if err := checkKeyUsableByService(protected, "hunter2"); err != nil {
		t.Fatalf("stored passphrase: %v", err)
	}
	if err := checkKeyUsableByService("", ""); err != nil {
		t.Fatalf("no key configured: %v", err)
	}
	if err := checkKeyUsableByService(filepath.Join(dir, "missing.json"), ""); err != nil {
		t.Fatalf("unreadable key is the service's to report: %v", err)
	}
}
