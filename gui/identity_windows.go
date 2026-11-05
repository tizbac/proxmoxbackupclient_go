//go:build windows

package main

import "snapshot"

// The shadow copies' links go in the brand's own roaming folder
// (%APPDATA%\Salvanza\VSS), as the app's other files do.
func init() {
	snapshot.SetAppFolderName(folderName())
}

// legacyServiceNames are the Windows service names of the releases before
// the rename (the app's exe name plus "SVC", and the older name the service's
// own "-service install" used), which an upgrade removes.
var legacyServiceNames = []string{"ProxmoxBackupClientSVC", "ProxmoxBackupClient", "NimbusBackupSVC", "EtitechBackupSVC", "AcmeBackupSVC"}

// otherBrandServiceNames are today's service names of the other brands: an
// installer of one brand replaces any other for now, so its service removes
// theirs too.
func otherBrandServiceNames() []string {
	var names []string
	for key, b := range brandCatalog {
		if key != product().Name && b.FileBase != "" {
			names = append(names, b.FileBase+"-svc")
		}
	}
	return names
}
