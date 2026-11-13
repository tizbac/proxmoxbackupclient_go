//go:build windows

package main

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// The installer's note is read from the registry: only a non-zero number
// under the value's name counts. Run against a throwaway key of the current
// user, never the computer's own.
func TestReplacedCompressingClientReadsTheInstallersNote(t *testing.T) {
	path := fmt.Sprintf(`SOFTWARE\SalvanzaTest-%d\Upgrade`, os.Getpid())
	parent := fmt.Sprintf(`SOFTWARE\SalvanzaTest-%d`, os.Getpid())
	oldRoot, oldPath := upgradeNoteRoot, upgradeNotePath
	upgradeNoteRoot, upgradeNotePath = registry.CURRENT_USER, path
	t.Cleanup(func() {
		upgradeNoteRoot, upgradeNotePath = oldRoot, oldPath
		_ = registry.DeleteKey(registry.CURRENT_USER, path)
		_ = registry.DeleteKey(registry.CURRENT_USER, parent)
	})

	if replacedCompressingClient() {
		t.Fatal("yes with no key at all")
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	if replacedCompressingClient() {
		t.Fatal("yes with the key but no value")
	}
	if err := k.SetDWordValue(replacedCompressingClientValue, 0); err != nil {
		t.Fatal(err)
	}
	if replacedCompressingClient() {
		t.Fatal("yes with the value 0")
	}
	if err := k.SetDWordValue(replacedCompressingClientValue, 1); err != nil {
		t.Fatal(err)
	}
	if !replacedCompressingClient() {
		t.Fatal("no with the value 1")
	}
	if err := k.SetStringValue(replacedCompressingClientValue, "1"); err != nil {
		t.Fatal(err)
	}
	if replacedCompressingClient() {
		t.Fatal("yes with a text value: the installer writes a number")
	}
}
