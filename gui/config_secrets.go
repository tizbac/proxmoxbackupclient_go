package main

import (
	"fmt"
	"strings"
	"sync"
)

// The secrets in config.json (PBS tokens and passwords, remembered key
// passphrases, the SMTP password) are written sealed on Windows: encrypted
// with DPAPI in the machine scope, so the background service (SYSTEM) and
// the app (an administrator) can both read them, and a copy of the file
// taken off this computer (a backup of ProgramData, a support zip) cannot.
// It is not a defence against an administrator of this computer, who can
// unseal them the same way the app does; the settings folder being open to
// administrators only is what keeps other local accounts out.
//
// A sealed value is "dpapi:" plus base64. Anything without the prefix is
// plain text (an older config, or Linux, where nothing is sealed and the
// 0600/0640 settings folder is the protection), and is sealed the next time
// the file is written.
const sealedPrefix = "dpapi:"

func isSealed(s string) bool { return strings.HasPrefix(s, sealedPrefix) }

// sealSecret returns s sealed for config.json. Empty and already sealed
// values are returned as they are.
func sealSecret(s string) (string, error) {
	if s == "" || isSealed(s) || !canSealSecrets {
		return s, nil
	}
	return sealPlatform(s)
}

// openSecret returns the plain text of a value read from config.json. A
// value that cannot be unsealed (the file was copied from another computer)
// is returned still sealed, with the error: kept like that, a later save
// writes it back unchanged instead of losing it, and logging in with it
// fails with a clear error from the server.
func openSecret(s string) (string, error) {
	if !isSealed(s) {
		return s, nil
	}
	plain, err := openPlatform(s)
	if err != nil {
		return s, err
	}
	return plain, nil
}

// secretFields lists every secret of a config, with where it lives, so
// sealing and unsealing cannot miss one.
func (c *Config) secretFields() map[string]*string {
	f := map[string]*string{
		"secret":                    &c.Secret,
		"encryption_key_passphrase": &c.EncryptionKeyPassphrase,
		"smtp_password":             &c.SMTPPassword,
	}
	for id, s := range c.PBSServers {
		if s == nil {
			continue
		}
		f["pbs_servers."+id+".secret"] = &s.Secret
		f["pbs_servers."+id+".password"] = &s.Password
		f["pbs_servers."+id+".encryption_key_passphrase"] = &s.EncryptionKeyPassphrase
	}
	return f
}

// sealedCopy returns a copy of c for writing to disk, with every secret
// sealed. c itself keeps the plain text the rest of the app works with.
func (c *Config) sealedCopy() (*Config, error) {
	cp := *c
	if c.PBSServers != nil {
		cp.PBSServers = make(map[string]*PBSServer, len(c.PBSServers))
		for id, s := range c.PBSServers {
			if s == nil {
				continue
			}
			sc := *s
			cp.PBSServers[id] = &sc
		}
	}
	for name, p := range cp.secretFields() {
		sealed, err := sealSecret(*p)
		if err != nil {
			return nil, fmt.Errorf("sealing %s: %w", name, err)
		}
		*p = sealed
	}
	return &cp, nil
}

// openSecrets unseals every secret of a config just read from disk. It
// reports whether any secret was plain text and could have been sealed, so
// the caller can write the file back sealed.
func (c *Config) openSecrets() (hadPlain bool) {
	for name, p := range c.secretFields() {
		if *p == "" {
			continue
		}
		if !isSealed(*p) {
			hadPlain = true
			continue
		}
		plain, err := openSecret(*p)
		if err != nil {
			writeDebugLog(fmt.Sprintf("config: cannot unseal %s (was config.json copied from another computer?): %v", name, err))
		}
		*p = plain
	}
	return hadPlain && canSealSecrets
}

// sealOnLoadOnce makes the first LoadConfig of a process write back a
// config that still holds plain-text secrets, so an upgrade seals them
// without waiting for the next settings change. Only once: a process that
// cannot write the file (the app while the service owns it) tries once.
var sealOnLoadOnce sync.Once
