package main

import "sort"

// "Compress encrypted backups" is a switch on each PBS server that has an
// encryption key (PBSServer.CompressEncrypted). On, an encrypted chunk is
// compressed before it is encrypted, as the official Proxmox client writes
// it. Off, it is written as version 2.0 and older of this client wrote it,
// which is the only form those versions (and restore ISOs made with them)
// read. It only says how this computer writes: every version from 2.1 reads
// both forms.
//
// A server added with this version starts on. For the servers of settings an
// earlier version left, the first read of those settings decides
// (decideCompressEncrypted): off where the earlier version wrote the plain
// form, so that updating one computer does not stop the others, or a restore
// ISO, from restoring what it backs up from then on. The app then asks once,
// per such server, whether to turn it on (CompressEncryptedAsk).
//
// What off cannot do: PBS stores a chunk once, under a digest that is the same
// for both forms, so a chunk another computer already stored compressed with
// the same key in the same datastore is referred to, not sent again. Off keeps
// a datastore readable by 2.0 only where nothing has written compressed
// encrypted chunks with that key there.

// compressEncryptedOn is the switch as a yes or no: unset is on.
func compressEncryptedOn(v *bool) bool { return v == nil || *v }

// decideCompressEncrypted gives the switch of every server that has an
// encryption key its start value, the first time this version reads settings
// an earlier one left: off, with the question to ask, unless that version
// already wrote compressed encrypted backups (wroteCompressed), where it
// stays on. A server whose switch is already set keeps it. It returns the
// names of the servers it turned off, sorted, and nil when the settings were
// decided before.
func (c *Config) decideCompressEncrypted(wroteCompressed bool) []string {
	if c.CompressEncryptedDecided {
		return nil
	}
	c.CompressEncryptedDecided = true
	if wroteCompressed {
		return nil
	}
	if c.EncryptionKeyFile != "" && c.CompressEncrypted == nil {
		off := false
		c.CompressEncrypted = &off
	}
	var names []string
	for id, s := range c.PBSServers {
		if s == nil || s.EncryptionKeyFile == "" || s.CompressEncrypted != nil {
			continue
		}
		off := false
		s.CompressEncrypted = &off
		s.CompressEncryptedAsk = true
		name := s.Name
		if name == "" {
			name = id
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// compressEncryptedOffNotice is the Message Log line for a server whose
// switch started off; the app's one-time question says the same.
func compressEncryptedOffNotice(server string) string {
	return "Encrypted backups to " + server + " are not compressed, to stay compatible with version 2.0.0 and earlier. " +
		"We recommend turning on Compress encrypted backups (edit the server, Encryption) unless you still need to restore " +
		"with version 2.0.0 or earlier, or with a restore ISO made before 2.1."
}

// settleCompressEncrypted gives s, an update of the server existing, the
// switch and the question it is saved with. An update that does not say the
// switch (a program from before it) leaves both as they were. Else the
// question is answered once the switch is on or has been changed; with the
// switch left off it stays as the update says (the answer "keep it off" clears
// it), and an update never raises it.
func settleCompressEncrypted(existing, s *PBSServer) {
	if s.CompressEncrypted == nil {
		s.CompressEncrypted, s.CompressEncryptedAsk = existing.CompressEncrypted, existing.CompressEncryptedAsk
		return
	}
	if *s.CompressEncrypted || compressEncryptedOn(existing.CompressEncrypted) || !existing.CompressEncryptedAsk {
		s.CompressEncryptedAsk = false
	}
}

// followDefaultCompressEncrypted keeps the switch of the single server of
// settings from before there were several (BaseURL and the fields beside it,
// which a run that names no server still uses, EffectivePBS) the same as the
// default server's, which is that server in the list.
func (c *Config) followDefaultCompressEncrypted() {
	if c.BaseURL == "" {
		return
	}
	if def, err := c.GetPBSServer(""); err == nil && def != nil {
		c.CompressEncrypted = def.CompressEncrypted
	}
}

// importedCompressEncrypted gives s, a server of a settings file being merged
// in (existing is the server it replaces here, or nil), its switch where the
// file does not say it: as it is here for a server this computer has, and for
// a new one from a file written before the switch (fileHasSwitch false) off
// with the question, as for the servers of an earlier version's settings.
func importedCompressEncrypted(existing, s *PBSServer, fileHasSwitch bool) {
	if s.EncryptionKeyFile == "" || s.CompressEncrypted != nil {
		return
	}
	switch {
	case existing != nil:
		s.CompressEncrypted, s.CompressEncryptedAsk = existing.CompressEncrypted, existing.CompressEncryptedAsk
	case !fileHasSwitch:
		off := false
		s.CompressEncrypted = &off
		s.CompressEncryptedAsk = true
	}
}
