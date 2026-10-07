%global debug_package %{nil}

# Version is overridden by packaging/fedora/build-rpm.sh from gui/wails.json
Name:          pbsgo
Version:       0.2.119
Release:       1%{?dist}
Summary:       Backup client for Proxmox Backup Server (CLI + GUI)

License:       GPLv3+
URL:           https://github.com/tizbac/proxmoxbackupclient_go
Source0:       pbsgo-%{version}.tar.gz

BuildRequires: golang
BuildRequires: gcc
BuildRequires: gtk3-devel
BuildRequires: webkit2gtk4.1-devel
BuildRequires: systemd

# pkexec/sudo: used by the pbsgo-gui launcher for one-time token-fetch
# elevation (soft requirements - the launcher degrades to standalone mode)
Recommends: polkit sudo

%description
Go-based backup client for Proxmox Backup Server:
 - pbsgo: directory and stream backup with deduplication
 - pbsgo-machine: whole-machine (raw disk) backup, using VSS on
   Windows and block-level snapshotting on Linux
 - pbsgo-nbd: mount fidx images as read-only NBD block devices
 - pbsgo-gui: graphical interface; machine (whole-disk) backup
   requires root, use the pbsgo-gui-root launcher for that
 - pbsgo-service: systemd service for scheduled backups (NOT enabled by default)
The GUI frontend is embedded in the binary, no Node.js is needed at
runtime.

%prep
%setup -q

%build
# directorybackup is intentionally not part of go.work
( cd directorybackup && GOWORK=off go build -trimpath \
    -ldflags "-s -w -X main.version=%{version}" -o pbsgo . )
( cd machinebackup && go build -trimpath \
    -ldflags "-s -w -X main.version=%{version}" -o pbsgo-machine . )
( cd nbd && go build -trimpath \
    -ldflags "-s -w -X main.version=%{version}" -o pbsgo-nbd . )
# the wails desktop frontend needs the production build tag; webkit2_41
# selects webkit2gtk-4.1 (wails defaults to the EOL webkit2gtk-4.0)
( cd gui && go build -tags production,webkit2_41 \
    -ldflags "-s -w -X main.appVersion=%{version}" -o pbsgo-gui . )
# service binary for systemd
( cd gui && GOWORK=off go build -tags service -trimpath \
    -ldflags "-s -w -X main.appVersion=%{version}" -o pbsgo-service . )

%install
rm -rf %{buildroot}
install -d -m 0755 %{buildroot}%{_libdir}/pbsgo \
                  %{buildroot}%{_bindir} \
                  %{buildroot}%{_datadir}/applications \
                  %{buildroot}%{_datadir}/icons/hicolor/256x256/apps \
                  %{buildroot}%{_unitdir}

install -m 0755 directorybackup/pbsgo      %{buildroot}%{_bindir}/pbsgo
install -m 0755 machinebackup/pbsgo-machine %{buildroot}%{_bindir}/pbsgo-machine
install -m 0755 nbd/pbsgo-nbd              %{buildroot}%{_bindir}/pbsgo-nbd
install -m 0755 gui/pbsgo-gui              %{buildroot}%{_libdir}/pbsgo/pbsgo-gui
install -m 0755 gui/pbsgo-service          %{buildroot}%{_libdir}/pbsgo/pbsgo-service

install -m 0644 gui/Icon.png \
    %{buildroot}%{_datadir}/icons/hicolor/256x256/apps/pbsgo.png

# systemd unit (NOT enabled by default - user must explicitly enable);
# %install runs with cwd = the extracted source dir, so packaging/ is here
# (not ../packaging/)
install -m 0644 packaging/systemd/pbsgo.service %{buildroot}%{_unitdir}/pbsgo.service

cat > %{buildroot}%{_datadir}/applications/pbsgo-gui.desktop <<'EOF'
[Desktop Entry]
Name=Proxmox Backup Client
Comment=Backup client for Proxmox Backup Server
Exec=pbsgo-gui
Icon=pbsgo
Terminal=false
Type=Application
Categories=System;Utility;
EOF
chmod 0644 %{buildroot}%{_datadir}/applications/pbsgo-gui.desktop

# pbsgo-gui launcher: probes service, does pkexec token fetch if needed
cat > %{buildroot}%{_bindir}/pbsgo-gui <<'EOF'
#!/bin/sh
# pbsgo-gui: launch the Proxmox Backup Client GUI as the current user.
# Probes the local service (127.0.0.1:18765). If the service is running but the
# GUI cannot read its token file (/var/lib/pbsgo/api-token, root-only), this
# launcher performs a ONE-TIME pkexec elevation to fetch the token, exports it
# as PBSGO_API_TOKEN, and execs the GUI. On any failure it exports
# PBSGO_TOKEN_FETCH_FAILED=1 so the GUI skips its own 401->elevated prompt.
#
# Note: machine (whole-disk) backup needs root privileges. To run the
# GUI elevated, use pbsgo-gui-root instead.

set -eu

GUI_BIN=@LIBDIR@/pbsgo/pbsgo-gui
TOKEN_FILE=/var/lib/pbsgo/api-token
SERVICE_PORT=18765

# Probe service reachability via /dev/tcp (bash builtin, no netcat needed).
# Returns 0 if reachable, 1 if connection refused, 2 if other error.
probe_service() {
    # Use a short timeout with bash's /dev/tcp redirection
    if timeout 1 bash -c "exec 3<>/dev/tcp/127.0.0.1/$SERVICE_PORT" 2>/dev/null; then
        exec 3<&-
        exec 3>&-
        return 0
    fi
    return 1
}

# Attempt one elevated token fetch via pkexec (fallback sudo).
# Writes the token to a temp file, then exports PBSGO_API_TOKEN.
elevated_fetch() {
    local handoff
    handoff=$(mktemp /tmp/pbsgo-token-XXXXXX) || return 1
    chmod 600 "$handoff"

    # Launch the GUI binary elevated with the token-fetch child flag
    if command -v pkexec >/dev/null 2>&1; then
        pkexec "$GUI_BIN" --elevated-token-fetch "$handoff" >/dev/null 2>&1 || true
    elif command -v sudo >/dev/null 2>&1; then
        sudo --non-interactive "$GUI_BIN" --elevated-token-fetch "$handoff" >/dev/null 2>&1 || true
    else {
        echo "pbsgo-gui: neither pkexec nor sudo available for token fetch" >&2
        rm -f "$handoff"
        return 1
    }; fi

    # Wait briefly for the child to write the handoff file (max ~30s)
    local deadline
    deadline=$(($(date +%s) + 30))
    while [ $(date +%s) -lt $deadline ]; do
        if [ -s "$handoff" ]; then
            # Read token into env var and clean up
            PBSGO_API_TOKEN=$(cat "$handoff" 2>/dev/null | tr -d '[:space:]')
            export PBSGO_API_TOKEN
            rm -f "$handoff"
            [ -n "$PBSGO_API_TOKEN" ] && return 0
            return 1
        fi
        sleep 0.5
    done
    rm -f "$handoff"
    return 1
}

# Main launcher logic
if probe_service; then
    # Service is reachable. Try to read the token file directly (works if
    # the user is root, or if the token has been made readable).
    if [ -r "$TOKEN_FILE" ]; then
        PBSGO_API_TOKEN=$(cat "$TOKEN_FILE" 2>/dev/null | tr -d '[:space:]')
        export PBSGO_API_TOKEN
    else
        # Token not readable: attempt ONE elevated fetch
        if ! elevated_fetch; then
            # Failed: tell the GUI not to prompt again
            export PBSGO_TOKEN_FETCH_FAILED=1
        fi
    fi
fi

# If the service is NOT reachable, we let the GUI start in standalone mode
# (it will detect this itself via its own probe). No token env is exported.

exec "$GUI_BIN" "$@"
EOF
sed -i "s|@LIBDIR@|%{_libdir}|g" %{buildroot}%{_bindir}/pbsgo-gui
chmod 0755 %{buildroot}%{_bindir}/pbsgo-gui

cat > %{buildroot}%{_bindir}/pbsgo-gui-root <<'EOF'
#!/bin/sh
# pbsgo-gui-root: launch the Proxmox Backup Client GUI with root
# privileges, which machine (whole-disk) backup requires.
if [ "$(id -u)" = "0" ]; then
    exec @LIBDIR@/pbsgo/pbsgo-gui "$@"
fi

if command -v sudo >/dev/null 2>&1; then
    if [ -n "${DISPLAY:-}" ]; then
        # Keep the X11 session reachable from the elevated process.
        exec sudo env "DISPLAY=$DISPLAY" "XAUTHORITY=${XAUTHORITY:-$HOME/.Xauthority}" @LIBDIR@/pbsgo/pbsgo-gui "$@"
    fi
    exec sudo @LIBDIR@/pbsgo/pbsgo-gui "$@"
fi

exec pkexec @LIBDIR@/pbsgo/pbsgo-gui "$@"
EOF
sed -i "s|@LIBDIR@|%{_libdir}|g" %{buildroot}%{_bindir}/pbsgo-gui-root
chmod 0755 %{buildroot}%{_bindir}/pbsgo-gui-root

%pre
# Stop the service on upgrade ($1 >= 2). Deliberately NOT disabling it:
# the user's enablement must survive upgrades (and fresh installs never
# enable anything - see %post).
if [ "$1" -gt 1 ]; then
    systemctl stop pbsgo 2>/dev/null || true
fi

%post
# Reload systemd daemon to pick up the new unit file
systemctl daemon-reload 2>/dev/null || true
# IMPORTANT: Do NOT enable the service by default (no systemctl enable).
# The user must explicitly enable/start it: sudo systemctl enable --now pbsgo

%preun
# Stop and disable the service
if [ "$1" = 0 ]; then  # remove
    systemctl stop pbsgo 2>/dev/null || true
    systemctl disable pbsgo 2>/dev/null || true
fi

%postun
# Reload systemd to remove the unit
systemctl daemon-reload 2>/dev/null || true

%files
%license LICENSE
%doc README.md CHANGELOG.md
%dir %{_libdir}/pbsgo
%{_bindir}/pbsgo
%{_bindir}/pbsgo-machine
%{_bindir}/pbsgo-nbd
%{_bindir}/pbsgo-gui
%{_bindir}/pbsgo-gui-root
%{_libdir}/pbsgo/pbsgo-gui
%{_libdir}/pbsgo/pbsgo-service
%{_unitdir}/pbsgo.service
%{_datadir}/applications/pbsgo-gui.desktop
%{_datadir}/icons/hicolor/256x256/apps/pbsgo.png

%changelog
* Mon Sep 07 2026 Proxmox Backup Client contributors <noreply@localhost> - 0.2.119-1
- Initial RPM package