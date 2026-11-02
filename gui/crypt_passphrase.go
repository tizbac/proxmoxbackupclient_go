package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"pbscommon"
)

// errPassphraseRequired is returned (wrapped) when a passphrase-protected key
// file has to be unlocked and there is neither a stored passphrase nor a way to
// ask the user, which is the case for scheduled jobs running as a service.
var errPassphraseRequired = errors.New("ENCRYPTION_PASSPHRASE_REQUIRED")

const maxPassphraseAttempts = 5

var (
	sessionPassphraseMu sync.Mutex
	sessionPassphrases  = map[string]string{}

	// passphrasePrompter asks the user for the passphrase of the key file at
	// path and returns it already verified. It stays nil in the service build
	// (and before the GUI window exists), where nobody can answer.
	passphrasePrompter func(path, fingerprint, lastErr string) (string, error)

	// promptMu serialises prompts, so two actions that need the same key (a
	// backup and a listing, say) ask once, not twice.
	promptMu sync.Mutex
)

func keyCacheID(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(path)
}

func sessionPassphrase(path string) (string, bool) {
	sessionPassphraseMu.Lock()
	defer sessionPassphraseMu.Unlock()
	p, ok := sessionPassphrases[keyCacheID(path)]
	return p, ok
}

func rememberSessionPassphrase(path, passphrase string) {
	sessionPassphraseMu.Lock()
	defer sessionPassphraseMu.Unlock()
	sessionPassphrases[keyCacheID(path)] = passphrase
}

func forgetSessionPassphrase(path string) {
	sessionPassphraseMu.Lock()
	defer sessionPassphraseMu.Unlock()
	delete(sessionPassphrases, keyCacheID(path))
}

// unlockProtectedKey unwraps a passphrase-protected key file: the stored
// passphrase first, then the one entered earlier this session, then (when
// interactive and a prompter is wired up) by asking the user.
func unlockProtectedKey(path, stored string, keyCfg *pbscommon.KeyConfig, interactive bool) (*pbscommon.CryptConfig, error) {
	var lastErr error
	if stored != "" {
		crypt, err := keyCfg.CryptConfig([]byte(stored))
		if err == nil {
			return crypt, nil
		}
		lastErr = fmt.Errorf("stored passphrase does not unlock the key: %w", err)
	}
	if p, ok := sessionPassphrase(path); ok {
		crypt, err := keyCfg.CryptConfig([]byte(p))
		if err == nil {
			return crypt, nil
		}
		forgetSessionPassphrase(path)
		lastErr = err
	}
	if !interactive {
		return nil, fmt.Errorf("%w: encryption key file %s is passphrase-protected and no passphrase is stored for it (unattended runs need the passphrase stored in the key settings)", errPassphraseRequired, path)
	}
	if passphrasePrompter == nil {
		msg := "no passphrase is stored for it and nobody is available to enter one (scheduled jobs need the passphrase stored in the key settings)"
		if lastErr != nil {
			msg = lastErr.Error()
		}
		return nil, fmt.Errorf("%w: encryption key file %s is passphrase-protected, %s", errPassphraseRequired, path, msg)
	}

	promptMu.Lock()
	defer promptMu.Unlock()
	// Another action may have been answered while this one waited for the lock.
	if p, ok := sessionPassphrase(path); ok {
		if crypt, err := keyCfg.CryptConfig([]byte(p)); err == nil {
			return crypt, nil
		}
	}
	lastMsg := ""
	if lastErr != nil {
		lastMsg = lastErr.Error()
	}
	for attempt := 0; attempt < maxPassphraseAttempts; attempt++ {
		p, err := passphrasePrompter(path, keyCfg.Fingerprint, lastMsg)
		if err != nil {
			return nil, err
		}
		crypt, err := keyCfg.CryptConfig([]byte(p))
		if err == nil {
			rememberSessionPassphrase(path, p)
			return crypt, nil
		}
		lastMsg = err.Error()
	}
	return nil, fmt.Errorf("encryption key file %s: too many wrong passphrases", path)
}

// checkKeyUsableByService fails early when the server's key is passphrase
// protected and no passphrase is stored for it. A backup routed through the
// background service runs as another process that cannot show a prompt, and
// the session passphrase lives only in this GUI process.
func checkKeyUsableByService(keyFile, storedPassphrase string) error {
	if keyFile == "" || storedPassphrase != "" {
		return nil
	}
	keyCfg, err := pbscommon.LoadKeyConfig(keyFile)
	if err != nil || keyCfg.KDF == nil {
		// Unreadable keys are reported by the service itself with the real reason.
		return nil
	}
	return fmt.Errorf("%w: this backup runs through the background service, which cannot ask for the passphrase of the encryption key %s. Open the server's Encryption tab and choose \"Remember on this computer\" for this key", errPassphraseRequired, keyFile)
}

// verifyKeyPassphrase reports whether passphrase unlocks the key file at path.
func verifyKeyPassphrase(path, passphrase string) error {
	keyCfg, err := pbscommon.LoadKeyConfig(path)
	if err != nil {
		return err
	}
	if keyCfg.KDF == nil {
		return nil
	}
	_, err = keyCfg.CryptConfig([]byte(passphrase))
	return err
}

// mergeStoredPassphrase decides what passphrase to persist after the frontend
// saved settings. The frontend never receives the stored one, so an empty
// value means "keep it". Changing the key file drops it, since it belonged to
// the old key.
func mergeStoredPassphrase(prevPath, prevPassphrase, newPath, typed string, clear bool) string {
	switch {
	case clear:
		return ""
	case typed != "":
		return typed
	case prevPath != newPath:
		return ""
	default:
		return prevPassphrase
	}
}

// ---- interactive prompt, GUI build only wires passphrasePrompter ----

type pendingPassphrase struct {
	path string
	ch   chan passphraseAnswer
}

type passphraseAnswer struct {
	passphrase string
	cancel     bool
}

var (
	pendingMu       sync.Mutex
	pendingPrompts  = map[string]*pendingPassphrase{}
	pendingSequence int
)

const passphrasePromptTimeout = 10 * time.Minute

// beginPassphrasePrompt registers a prompt and returns its id and the channel
// the answer arrives on. emit tells the frontend to show the dialog.
func beginPassphrasePrompt(path, fingerprint, lastErr string, emit func(payload map[string]any)) (string, chan passphraseAnswer) {
	pendingMu.Lock()
	pendingSequence++
	id := fmt.Sprintf("pp-%d", pendingSequence)
	p := &pendingPassphrase{path: path, ch: make(chan passphraseAnswer, 1)}
	pendingPrompts[id] = p
	pendingMu.Unlock()
	emit(map[string]any{"id": id, "path": path, "fingerprint": fingerprint, "error": lastErr})
	return id, p.ch
}

func endPassphrasePrompt(id string) {
	pendingMu.Lock()
	delete(pendingPrompts, id)
	pendingMu.Unlock()
}

// waitPassphrase blocks until the frontend answers or the prompt times out.
func waitPassphrase(id string, ch chan passphraseAnswer) (string, error) {
	defer endPassphrasePrompt(id)
	select {
	case ans := <-ch:
		if ans.cancel {
			return "", errors.New("passphrase entry cancelled")
		}
		return ans.passphrase, nil
	case <-time.After(passphrasePromptTimeout):
		return "", errors.New("timed out waiting for the encryption key passphrase")
	}
}

// submitPassphrase verifies an answer from the frontend. A wrong passphrase
// returns an error and leaves the prompt open so the dialog can show it and
// let the user retry.
func submitPassphrase(id, passphrase string, cancel bool) error {
	pendingMu.Lock()
	p := pendingPrompts[id]
	pendingMu.Unlock()
	if p == nil {
		return errors.New("this passphrase request is no longer pending")
	}
	if cancel {
		select {
		case p.ch <- passphraseAnswer{cancel: true}:
		default:
		}
		return nil
	}
	if err := verifyKeyPassphrase(p.path, passphrase); err != nil {
		return err
	}
	select {
	case p.ch <- passphraseAnswer{passphrase: passphrase}:
	default:
	}
	return nil
}
