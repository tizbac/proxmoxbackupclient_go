#!/bin/sh
# build-deb.sh - build the salvanza Debian/Ubuntu package (.deb).
#
# Usage: packaging/debian/build-deb.sh [output-dir]     (default: dist/)
#        PBS_VERSION=2.0.0 packaging/debian/build-deb.sh   (default: gui/wails.json)
#
# Build host requirements:
#   - Go toolchain; wails CLI optional (plain go build is used without it)
#   - dpkg-dev (dpkg-deb) and a Debian/Ubuntu userland (dpkg -S)
#   - GTK3 + webkit2gtk (4.1 preferred, 4.0 accepted) dev libraries
#   - the GUI frontend already built (gui/frontend/dist, committed in git)
#
# Result: <output-dir>/salvanza_<version>_<arch>.deb containing
#   /usr/bin/salvanza-cli          the command line: backup-folder, backup-disk,
#                                  attach-nbd (root)
#   /usr/bin/proxmoxbackup-{directory,machine,nbd}  its earlier names (symlinks)
#   /usr/bin/salvanza-gui          the app, as the current user
#   /usr/bin/salvanza-gui-root     the app as root (machine backups)
#   /usr/lib/salvanza/salvanza-gui
#   /usr/libexec/salvanza/salvanza-svc  the background service
#   /usr/lib/systemd/system/salvanza.service  (enabled on a fresh install)
#   /usr/share/applications/salvanza.desktop
#   /usr/share/icons/hicolor/256x256/apps/salvanza.png
#   /usr/share/doc/salvanza/{copyright,README.md,CHANGELOG.md.gz,manual/}
#
# It replaces the proxmoxbackupclient and pbsgo packages; the service takes
# their settings over on its first start (gui/config_dir_unix.go).
set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
OUT_DIR=${1:-$ROOT/dist}
PKG=salvanza
LIN="$ROOT/packaging/linux"

wails_field() {
    sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" "$ROOT/gui/wails.json" | head -n1
}

VERSION=${PBS_VERSION:-$(wails_field productVersion)}
[ -n "$VERSION" ] || { echo "error: no version (PBS_VERSION or gui/wails.json productVersion)" >&2; exit 1; }
GUI_OUTPUT=$(wails_field outputfilename)
[ -n "$GUI_OUTPUT" ] || GUI_OUTPUT=salvanza-gui

ARCH=$(go env GOARCH)
case "$ARCH" in
    amd64) DEBARCH=amd64 ;;
    arm64) DEBARCH=arm64 ;;
    *) echo "error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT INT TERM
STAGE=$WORK/root
DOC=$STAGE/usr/share/doc/$PKG
mkdir -p "$STAGE/DEBIAN" \
         "$STAGE/usr/bin" \
         "$STAGE/usr/lib/$PKG" \
         "$STAGE/usr/libexec/$PKG" \
         "$STAGE/usr/share/applications" \
         "$STAGE/usr/share/icons/hicolor/256x256/apps" \
         "$DOC/manual"

CLI_LDFLAGS="-s -w -X main.version=$VERSION"

echo "==> building the command line"
# go.work lists every module, cli included.
(cd "$ROOT" && CGO_ENABLED=0 go build -trimpath -ldflags="$CLI_LDFLAGS" -o "$STAGE/usr/bin/salvanza-cli" ./cli)
# The earlier names keep working: salvanza-cli picks the command from the
# name it was started by.
for old in proxmoxbackup-directory proxmoxbackup-machine proxmoxbackup-nbd; do
    ln -s salvanza-cli "$STAGE/usr/bin/$old"
done

echo "==> building the app"
[ -f "$ROOT/gui/frontend/dist/index.html" ] || {
    echo "error: gui/frontend/dist/index.html not found; build the frontend first (cd gui/frontend && npm install && npm run build)" >&2
    exit 1
}
# The desktop frontend (webkit/gtk cgo) is only linked with the "production"
# build tag; "webkit2_41" selects webkit2gtk-4.1 when its headers are there
# (wails uses 4.0 otherwise).
GO_TAGS=production
if command -v pkg-config >/dev/null 2>&1 && pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
    GO_TAGS="$GO_TAGS,webkit2_41"
fi
(cd "$ROOT/gui" && go build -trimpath -tags "$GO_TAGS" -ldflags "-s -w -X main.appVersion=$VERSION" -o "$STAGE/usr/lib/$PKG/$GUI_OUTPUT" .)

echo "==> building the background service"
# In /usr/libexec, as in the .rpm and the Arch package (they share the unit).
(cd "$ROOT/gui" && CGO_ENABLED=0 go build -trimpath -tags service -ldflags "-s -w -X main.appVersion=$VERSION" -o "$STAGE/usr/libexec/$PKG/salvanza-svc" .)

echo "==> installing launchers, menu entries and docs"
install -m 0755 "$LIN/salvanza-gui" "$STAGE/usr/bin/salvanza-gui"
install -m 0755 "$LIN/salvanza-gui-root" "$STAGE/usr/bin/salvanza-gui-root"
# One menu entry: the app asks for administrator rights itself when it needs
# them (connecting to the service; Restart as administrator runs the root
# launcher above).
install -m 0644 "$LIN/salvanza.desktop" "$STAGE/usr/share/applications/"
mkdir -p "$STAGE/usr/lib/systemd/system"
install -m 0644 "$LIN/salvanza.service" "$STAGE/usr/lib/systemd/system/"
# gui/Icon.png is the 256 px icon (gui/build/appicon.png is the same at 1024 px).
install -m 0644 "$ROOT/gui/Icon.png" "$STAGE/usr/share/icons/hicolor/256x256/apps/$PKG.png"
install -m 0644 "$ROOT/README.md" "$DOC/README.md"
gzip -9n -c "$ROOT/CHANGELOG.md" > "$DOC/CHANGELOG.md.gz"
chmod 0644 "$DOC/CHANGELOG.md.gz"
for m in "$ROOT"/docs/manual/*.md; do install -m 0644 "$m" "$DOC/manual/"; done
cat > "$DOC/copyright" <<EOF
Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/
Upstream-Name: salvanza_client
Source: https://github.com/salvanza/salvanza_client

Files: *
License: GPL-3.0-or-later
 On Debian systems the full text is in /usr/share/common-licenses/GPL-3.
EOF

echo "==> computing dependencies"
NATIVE_ARCH=$(dpkg --print-architecture 2>/dev/null || echo amd64)
MULTIARCH=$(dpkg-architecture -qDEB_HOST_MULTIARCH 2>/dev/null || echo x86_64-linux-gnu)
DEPS=""
add_dep() {
    case " $DEPS, " in
        *" $1, "*) return 0 ;;
    esac
    [ -n "$DEPS" ] && DEPS="$DEPS, "
    DEPS="$DEPS$1"
}
# Pick the package that owns the runtime file for a SONAME on this system:
# native architecture, standard multiarch dir, exact file name, prefer a
# runtime package over a -dev one.
pick_pkg() {
    dpkg -S "$1" 2>/dev/null | awk -F: -v s="$1" -v arch="$NATIVE_ARCH" -v ma="$MULTIARCH" '
        function base(p) { sub(/.*\//, "", p); return p }
        {
            n = split($0, f, ":")
            if (n == 2)      { pkg = f[1]; a = arch; path = f[2] }
            else if (n >= 3) { pkg = f[1]; a = f[2]; path = f[3]; for (i = 4; i <= n; i++) path = path ":" f[i] }
            else next
            sub(/^ +/, "", path)
            if (a != arch) next
            if (index(path, "/" ma "/") == 0) next
            if (base(path) != s) next
            if (pkg ~ /-dev$/) { fb = pkg; next }
            print pkg; found = 1; exit
        }
        END { if (!found) print fb }
    '
}
scan_deps() {
    for soname in $(readelf -d "$1" 2>/dev/null | sed -n 's/.*NEEDED.*\[\(.*\)\].*/\1/p'); do
        case "$soname" in
            *linux-vdso*|*ld-linux*) continue ;;
        esac
        pkg=$(pick_pkg "$soname" | head -n1) || true
        if [ -n "$pkg" ]; then
            add_dep "$pkg"
        else
            echo "warning: no owning package for $soname; skipping" >&2
        fi
    done
}
scan_deps "$STAGE/usr/lib/$PKG/$GUI_OUTPUT"
if [ -z "$DEPS" ]; then
    echo "warning: no shared-library dependencies detected; using fallback" >&2
    DEPS="libwebkit2gtk-4.1-0, libgtk-3-0t64 | libgtk-3-0"
fi
echo "    runtime deps: $DEPS"

SIZE=$(du -sk "$STAGE" | cut -f1)
awk -v v="$VERSION" -v a="$DEBARCH" -v d="$DEPS" -v sz="$SIZE" '
function rep(s, ph, val,    i) {
    i = index(s, ph)
    if (i > 0) return substr(s, 1, i - 1) val rep(substr(s, i + length(ph)), ph, val)
    return s
}
{
    line = rep($0, "@VERSION@", v)
    line = rep(line, "@ARCH@", a)
    line = rep(line, "@DEPENDS@", d)
    line = rep(line, "@SIZE@", sz)
    print line
}' "$SCRIPT_DIR/control.in" > "$STAGE/DEBIAN/control"

# A fresh install enables and starts the service, so the Backup Sets run
# on schedule with nobody logged in (as the Windows MSI does by default).
# An upgrade keeps what the user chose in Preferences > Advanced > Run as
# Service and restarts a running service; removal stops and disables it.
# /var/lib/salvanza (the Backup Sets and keys) is kept, also on purge.
#
# Installed in place of the proxmoxbackupclient package, the service keeps
# that package's choice: dpkg (or apt) removes the old package before this
# one is set up, and its removal stops and disables the old service, so
# the journal is what still tells whether it was running a moment ago.
# With an earlier install there (earlier_install) and the old service not
# stopped in the last 5 minutes, the service stays off (Preferences >
# Advanced > Run as Service turns it on).
#
# "Fresh" is told in preinst: dpkg calls it with "install" for a first
# install and for one after a removal that kept the package's files (where
# postinst would see the old version and take it for an upgrade), and with
# "upgrade" for an upgrade. It leaves a marker that postinst acts on.
cat > "$STAGE/DEBIAN/preinst" <<'EOF'
#!/bin/sh
set -e
if [ "$1" = install ]; then
    mkdir -p /run/salvanza && touch /run/salvanza/fresh-install || true
fi
exit 0
EOF
cat > "$STAGE/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
fresh=/run/salvanza/fresh-install
# old_service_was_on: the proxmoxbackupclient service was stopped in the
# last 5 minutes, which its package's removal does when it ran.
old_service_was_on() {
    command -v journalctl >/dev/null 2>&1 || return 1
    journalctl -q -o cat -u proxmoxbackupclient.service --since "-5 min" 2>/dev/null | grep -q '^Stopp'
}
# earlier_install: the client was installed here before the rename. The
# service's folder exists only once its service has run, so also the old
# package as dpkg still knows it (removed, its files kept) and a user's own
# settings: on a machine whose old service was never turned on, the service
# was switched on, and the app then showed the service's empty settings
# instead of the user's (found 2026-11-05).
earlier_install() {
    [ -d /var/lib/proxmoxbackupclient ] || [ -d /var/lib/pbsgo ] && return 0
    for d in /root/.proxmox-backup-guardian /home/*/.proxmox-backup-guardian; do
        [ -d "$d" ] && return 0
    done
    dpkg-query -W -f='${db:Status-Status}\n' proxmoxbackupclient pbsgo 2>/dev/null |
        grep -qE '^(installed|config-files|half-installed|half-configured|unpacked)$'
}
if [ "$1" = configure ] && [ -d /run/systemd/system ]; then
    systemctl daemon-reload || true
    if [ -e "$fresh" ]; then
        if earlier_install && [ ! -d /var/lib/salvanza ] && ! old_service_was_on; then
            echo "salvanza: the earlier service was off; it stays off (Preferences > Advanced > Run as Service)"
        else
            systemctl enable --now salvanza.service || true
        fi
    else
        systemctl try-restart salvanza.service || true
    fi
fi
rm -f "$fresh"
exit 0
EOF
cat > "$STAGE/DEBIAN/prerm" <<'EOF'
#!/bin/sh
set -e
if [ "$1" = remove ] && [ -d /run/systemd/system ]; then
    systemctl disable --now salvanza.service >/dev/null 2>&1 || true
fi
exit 0
EOF
cat > "$STAGE/DEBIAN/postrm" <<'EOF'
#!/bin/sh
set -e
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload || true
fi
exit 0
EOF
chmod 0755 "$STAGE/DEBIAN/preinst" "$STAGE/DEBIAN/postinst" "$STAGE/DEBIAN/prerm" "$STAGE/DEBIAN/postrm"

echo "==> building the package"
# Nothing group- or world-writable, whatever the build user's umask.
chmod -R go-w "$STAGE"
mkdir -p "$OUT_DIR"
DEB_PATH="$OUT_DIR/${PKG}_${VERSION}_${DEBARCH}.deb"
rm -f "$DEB_PATH"
dpkg-deb --build --root-owner-group "$STAGE" "$DEB_PATH"
echo "==> done: $DEB_PATH"
dpkg-deb --info "$DEB_PATH"
