%global debug_package %{nil}

# Version is set by packaging/fedora/build-rpm.sh (gui/wails.json or PBS_VERSION)
Name:          salvanza
Version:       0.0.1
Release:       1%{?dist}
Summary:       Salvanza, backup for Proxmox Backup Server (app and command line)

License:       GPL-3.0-or-later
URL:           https://salvanza.org/
Source0:       salvanza-%{version}.tar.gz

BuildRequires: golang
BuildRequires: gcc
BuildRequires: gtk3-devel
BuildRequires: webkit2gtk4.1-devel
# pkexec: the app asks for an administrator's password through it to connect
# to the service, whose API token is root's alone.
Requires:      polkit
# Replaces the project's earlier packages: pbsgo (1.2.0-rc packaging) and
# proxmoxbackupclient (the name before Salvanza). rpm wants a version on
# Obsoletes; neither reached 3.0.
Obsoletes:     pbsgo < 3.0.0
Obsoletes:     proxmoxbackupclient < 3.0.0

%description
Backs folders and whole disks up to a Proxmox Backup Server and restores
them, with deduplication, encryption, change detection, Undelete and
Roll back:
 - salvanza-gui: the app (folder backups as the current user);
   salvanza-gui-root starts it as root for machine (whole-disk)
   backups and block snapshots
 - salvanza-cli backup-folder: folder backups from the command line
 - salvanza-cli backup-disk: whole-disk backups from the command line
 - salvanza-cli attach-nbd: attach a disk image from the server as /dev/nbdN
 - salvanza.service: the background service that runs scheduled Backup
   Sets as root (enabled on a fresh install; turned on and off in the app,
   Preferences > Advanced > Run as Service)
It replaces the proxmoxbackupclient package and takes its settings over.
Snapshots of mounted file systems use elastio-snap when it is installed.

%prep
%setup -q

%build
export CGO_ENABLED=0
# go.work lists every module, cli included.
go build -trimpath -ldflags "-s -w -X main.version=%{version}" -o cli/salvanza-cli ./cli
# The app needs cgo (GTK, WebKit). "production" links the desktop frontend;
# "webkit2_41" selects webkit2gtk-4.1.
export CGO_ENABLED=1
( cd gui && go build -trimpath -tags production,webkit2_41 \
    -ldflags "-s -w -X main.appVersion=%{version}" -o salvanza-gui . )
( cd gui && CGO_ENABLED=0 go build -trimpath -tags service \
    -ldflags "-s -w -X main.appVersion=%{version}" -o salvanza-svc . )

%install
rm -rf %{buildroot}
install -d -m 0755 %{buildroot}%{_prefix}/lib/salvanza \
                  %{buildroot}%{_libexecdir}/salvanza \
                  %{buildroot}%{_bindir} \
                  %{buildroot}%{_datadir}/applications \
                  %{buildroot}%{_datadir}/icons/hicolor/256x256/apps \
                  %{buildroot}%{_prefix}/lib/systemd/system

install -m 0755 cli/salvanza-cli %{buildroot}%{_bindir}/
# The earlier names keep working: salvanza-cli picks the command from the
# name it was started by.
for old in proxmoxbackup-directory proxmoxbackup-machine proxmoxbackup-nbd; do
    ln -s salvanza-cli %{buildroot}%{_bindir}/$old
done
install -m 0755 gui/salvanza-gui %{buildroot}%{_prefix}/lib/salvanza/
# The service program in libexec: labelled bin_t, so SELinux runs the service
# as unconfined_service_t (in /usr/lib it is lib_t and stays in init_t).
install -m 0755 gui/salvanza-svc %{buildroot}%{_libexecdir}/salvanza/
install -m 0644 packaging/linux/salvanza.service %{buildroot}%{_prefix}/lib/systemd/system/
install -m 0755 packaging/linux/salvanza-gui %{buildroot}%{_bindir}/
install -m 0755 packaging/linux/salvanza-gui-root %{buildroot}%{_bindir}/
install -m 0644 packaging/linux/salvanza.desktop %{buildroot}%{_datadir}/applications/
install -m 0644 gui/Icon.png %{buildroot}%{_datadir}/icons/hicolor/256x256/apps/salvanza.png

%files
%license LICENSE
%doc README.md CHANGELOG.md docs/manual
%{_bindir}/salvanza-cli
%{_bindir}/proxmoxbackup-directory
%{_bindir}/proxmoxbackup-machine
%{_bindir}/proxmoxbackup-nbd
%{_bindir}/salvanza-gui
%{_bindir}/salvanza-gui-root
%dir %{_prefix}/lib/salvanza
%{_prefix}/lib/salvanza/salvanza-gui
%dir %{_libexecdir}/salvanza
%{_libexecdir}/salvanza/salvanza-svc
%{_prefix}/lib/systemd/system/salvanza.service
%{_datadir}/applications/salvanza.desktop
%{_datadir}/icons/hicolor/256x256/apps/salvanza.png

# A fresh install enables and starts the service, so the Backup Sets run on
# schedule with nobody logged in; an upgrade keeps the user's choice (Run as
# Service) and restarts a running service. Removal stops and disables it. The
# settings in /var/lib/salvanza are kept.
#
# Installed in place of proxmoxbackupclient, the service keeps that package's
# choice. rpm sets this package up before it erases the old one, so %post
# still sees whether the old service is enabled; the old package's erase then
# stops it, and %posttrans starts this one (both on port 18765, so not
# before).
%post
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload || :
    if [ "$1" -eq 1 ]; then
        mkdir -p /run/salvanza
        if [ -f %{_prefix}/lib/systemd/system/proxmoxbackupclient.service ] && \
           ! systemctl -q is-enabled proxmoxbackupclient.service 2>/dev/null; then
            : # the earlier service was off: it stays off
        else
            touch /run/salvanza/enable-service
        fi
    else
        systemctl try-restart salvanza.service || :
    fi
fi

%posttrans
if [ -e /run/salvanza/enable-service ]; then
    rm -f /run/salvanza/enable-service
    systemctl enable --now salvanza.service || :
fi

%preun
if [ "$1" -eq 0 ] && [ -d /run/systemd/system ]; then
    systemctl disable --now salvanza.service >/dev/null 2>&1 || :
fi

%postun
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload || :
fi

%changelog
* Thu Nov 05 2026 Salvanza contributors <noreply@github.com> - 2.0.0-1
- Renamed to salvanza: salvanza-gui, salvanza-svc, salvanza-cli (with the
  earlier command names as links); replaces proxmoxbackupclient
* Mon Oct 12 2026 proxmoxbackupclient_go (mjb-is fork) <noreply@github.com> - 0.0.1-1
- Package named proxmoxbackupclient, the fork's tool names, shared launchers
