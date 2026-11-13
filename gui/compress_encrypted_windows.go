//go:build windows

package main

import "golang.org/x/sys/windows/registry"

// The installer's note that it replaced a client that already wrote
// compressed encrypted backups: Nimbus Backup 0.4.1 to 0.4.5 as RDEM Systems
// released it (installer/wix/ProductBody.wxi, RDEMNIMBUS04). Its settings
// folder cannot be told from the one this project's 1.2 left, which wrote the
// plain form, so the installer, which sees the product and its version, says.
// Overridable so tests can point at a throwaway HKCU key instead of HKLM.
var (
	upgradeNoteRoot = registry.LOCAL_MACHINE
	upgradeNotePath = `SOFTWARE\` + folderName() + `\Upgrade`
)

const replacedCompressingClientValue = "ReplacedCompressingClient"

// replacedCompressingClient: did this install replace a client that wrote
// compressed encrypted backups? Missing value or any read problem = no.
func replacedCompressingClient() bool {
	k, err := registry.OpenKey(upgradeNoteRoot, upgradeNotePath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue(replacedCompressingClientValue)
	return err == nil && v != 0
}
