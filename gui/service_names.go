package main

// linuxServiceUnit is the systemd unit of the Linux background service
// (salvanza.service), shipped by the .deb, .rpm and Arch package.
var linuxServiceUnit = product().FileBase

// Where the Linux packages put the app and the background service program:
// /usr/lib/salvanza/salvanza-gui and /usr/libexec/salvanza/salvanza-svc.
// The service program is in /usr/libexec, the place for programs that other
// programs (here systemd) run: on Fedora its files there are labelled bin_t,
// so SELinux runs the service as unconfined_service_t, the usual domain of a
// service that has no policy of its own, rather than in systemd's init_t.
var (
	packagedAppDir     = "/usr/lib/" + product().FileBase
	packagedServiceExe = "/usr/libexec/" + product().FileBase + "/" + serviceExeBase()
)
