package main

// linuxServiceUnit is the systemd unit of the Linux background service
// (proxmoxbackupclient.service), shipped by the .deb, .rpm and Arch package.
const linuxServiceUnit = "proxmoxbackupclient"

// Where the Linux packages put the app and the background service program.
// The service program is in /usr/libexec, the place for programs that other
// programs (here systemd) run: on Fedora its files there are labelled bin_t,
// so SELinux runs the service as unconfined_service_t, the usual domain of a
// service that has no policy of its own, rather than in systemd's init_t.
// Earlier packages put it beside the app; an upgrade moves it.
const (
	packagedAppDir     = "/usr/lib/proxmoxbackupclient"
	packagedServiceExe = "/usr/libexec/proxmoxbackupclient/ProxmoxBackupClientSVC"
)
