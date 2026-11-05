package main

import (
	"bytes"
	"strings"
	"testing"
)

// The old tool names run the matching command, whatever case or .exe.
func TestAliases(t *testing.T) {
	for name, want := range map[string]string{
		"proxmoxbackup-directory":     "backup-folder",
		`C:\x\PBSDirectoryBackup.exe`: "backup-folder",
		"/usr/bin/directorybackup":    "backup-folder",
		"proxmoxbackup-machine":       "backup-disk",
		"pbsmachinebackup.exe":        "backup-disk",
		"proxmoxbackup-nbd":           "attach-nbd",
		"pbsnbd":                      "attach-nbd",
	} {
		c, ok := byAlias(programName(name))
		if !ok || c.name != want {
			t.Errorf("%s: %q %v, want %q", name, c.name, ok, want)
		}
	}
	if _, ok := byAlias(programName("salvanza-cli.exe")); ok {
		t.Error("salvanza-cli taken for an alias")
	}
}

// Every command has a summary and a unique name and aliases.
func TestCommandsListed(t *testing.T) {
	var b bytes.Buffer
	usage(&b)
	seen := map[string]bool{}
	for _, c := range commands() {
		if !strings.Contains(b.String(), c.name) || c.summary == "" {
			t.Errorf("%s missing from the help", c.name)
		}
		for _, n := range append([]string{c.name}, c.aliases...) {
			if seen[n] {
				t.Errorf("%s used twice", n)
			}
			seen[n] = true
		}
	}
}
