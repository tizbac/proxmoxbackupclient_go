%global debug_package %{nil}

# Version is set by packaging/fedora/build-rpm.sh (gui/wails.json or PBS_VERSION)
Name:          proxmoxbackupclient
Version:       0.0.1
Release:       1%{?dist}
Summary:       Backup client for Proxmox Backup Server (app and command line)

License:       GPL-3.0-or-later
URL:           https://github.com/mjb-is/proxmoxbackupclient_go
Source0:       proxmoxbackupclient-%{version}.tar.gz

BuildRequires: golang
BuildRequires: gcc
BuildRequires: gtk3-devel
BuildRequires: webkit2gtk4.1-devel
Requires:      polkit

%description
Backs folders and whole disks up to a Proxmox Backup Server and restores
them, with deduplication, encryption, change detection, Undelete and
Roll back:
 - proxmoxbackupclient: the app (folder backups as the current user);
   proxmoxbackupclient-root starts it as root for machine (whole-disk)
   backups and block snapshots
 - proxmoxbackup-directory: folder backups from the command line
 - proxmoxbackup-machine: whole-disk backups from the command line
 - proxmoxbackup-nbd: attach a disk image from the server as /dev/nbdN
 - proxmoxbackupclient.service: the background service that runs
   scheduled Backup Sets as root (installed, not enabled; turn it on in
   the app, Preferences > Advanced > Run as Service)
Snapshots of mounted file systems use elastio-snap when it is installed.

%prep
%setup -q

%build
export CGO_ENABLED=0
# directorybackup is not part of go.work, and its go.mod has no replace
# lines for the local modules: build it with a temporary work file.
{ cat go.work; printf '\nuse ./directorybackup\n'; } > go.rpmwork
GOWORK=$PWD/go.rpmwork go build -trimpath \
    -ldflags "-s -w -X main.version=%{version}" -o directorybackup/proxmoxbackup-directory ./directorybackup
go build -trimpath -ldflags "-s -w -X main.version=%{version}" -o machinebackup/proxmoxbackup-machine ./machinebackup
go build -trimpath -ldflags "-s -w -X main.version=%{version}" -o nbd/proxmoxbackup-nbd ./nbd
# The app needs cgo (GTK, WebKit). "production" links the desktop frontend;
# "webkit2_41" selects webkit2gtk-4.1.
export CGO_ENABLED=1
( cd gui && go build -trimpath -tags production,webkit2_41 \
    -ldflags "-s -w -X main.appVersion=%{version}" -o ProxmoxBackupClient . )
( cd gui && CGO_ENABLED=0 go build -trimpath -tags service \
    -ldflags "-s -w -X main.appVersion=%{version}" -o ProxmoxBackupClientSVC . )

%install
rm -rf %{buildroot}
install -d -m 0755 %{buildroot}%{_prefix}/lib/proxmoxbackupclient \
                  %{buildroot}%{_bindir} \
                  %{buildroot}%{_datadir}/applications \
                  %{buildroot}%{_datadir}/icons/hicolor/256x256/apps \
                  %{buildroot}%{_prefix}/lib/systemd/system

install -m 0755 directorybackup/proxmoxbackup-directory %{buildroot}%{_bindir}/
install -m 0755 machinebackup/proxmoxbackup-machine %{buildroot}%{_bindir}/
install -m 0755 nbd/proxmoxbackup-nbd %{buildroot}%{_bindir}/
install -m 0755 gui/ProxmoxBackupClient %{buildroot}%{_prefix}/lib/proxmoxbackupclient/
install -m 0755 gui/ProxmoxBackupClientSVC %{buildroot}%{_prefix}/lib/proxmoxbackupclient/
install -m 0644 packaging/linux/proxmoxbackupclient.service %{buildroot}%{_prefix}/lib/systemd/system/
install -m 0755 packaging/linux/proxmoxbackupclient %{buildroot}%{_bindir}/
install -m 0755 packaging/linux/proxmoxbackupclient-root %{buildroot}%{_bindir}/
install -m 0644 packaging/linux/proxmoxbackupclient.desktop %{buildroot}%{_datadir}/applications/
install -m 0644 packaging/linux/proxmoxbackupclient-root.desktop %{buildroot}%{_datadir}/applications/
install -m 0644 gui/Icon.png %{buildroot}%{_datadir}/icons/hicolor/256x256/apps/proxmoxbackupclient.png

%files
%license LICENSE
%doc README.md CHANGELOG.md docs/manual
%{_bindir}/proxmoxbackup-directory
%{_bindir}/proxmoxbackup-machine
%{_bindir}/proxmoxbackup-nbd
%{_bindir}/proxmoxbackupclient
%{_bindir}/proxmoxbackupclient-root
%{_prefix}/lib/proxmoxbackupclient/ProxmoxBackupClient
%{_prefix}/lib/proxmoxbackupclient/ProxmoxBackupClientSVC
%{_prefix}/lib/systemd/system/proxmoxbackupclient.service
%{_datadir}/applications/proxmoxbackupclient.desktop
%{_datadir}/applications/proxmoxbackupclient-root.desktop
%{_datadir}/icons/hicolor/256x256/apps/proxmoxbackupclient.png

# The service is installed but not enabled (the app turns it on). An upgrade
# restarts a running service; removal stops and disables it. The settings in
# /var/lib/proxmoxbackupclient are kept.
%post
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload || :
    [ "$1" -gt 1 ] && systemctl try-restart proxmoxbackupclient.service || :
fi

%preun
if [ "$1" -eq 0 ] && [ -d /run/systemd/system ]; then
    systemctl disable --now proxmoxbackupclient.service >/dev/null 2>&1 || :
fi

%postun
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload || :
fi

%changelog
* Mon Oct 12 2026 proxmoxbackupclient_go (mjb-is fork) <noreply@github.com> - 0.0.1-1
- Package named proxmoxbackupclient, the fork's tool names, shared launchers
