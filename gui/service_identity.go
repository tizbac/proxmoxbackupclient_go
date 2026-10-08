package main

import "strings"

// serviceIdentityForExeBase maps a service executable's base name (as the WiX
// installer names it: $(var.ServiceExeName).exe) to the (SCM name, display
// name) pair the service has to register under.
//
// The SCM name is the executable base name verbatim, because
// installer/wix/ProductBody.wxi registers the service as
// Name="$(var.ServiceExeName)" and ships the binary as
// $(var.ServiceExeName).exe. Registering anything else makes
// StartServiceCtrlDispatcher fail with ERROR_FAILED_SERVICE_CONTROLLER_CONNECT
// (1061) and Windows reports "service failed to start".
//
// The display name mirrors the installer's DisplayName="$(var.ProductName) Service",
// resolving the brand from the exe base without the "SVC" suffix.
func serviceIdentityForExeBase(exeBase string) (name, displayName string) {
	name = exeBase
	brandKey := strings.TrimSuffix(exeBase, "SVC")
	return name, ResolveBrand(brandKey).Title + " Service"
}
