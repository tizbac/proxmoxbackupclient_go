#!/bin/sh
# build-deb.sh - build the pbsgo Debian package (.deb).
#
# Usage: packaging/debian/build-deb.sh [output-dir]     (default: dist/)
#
# Build host requirements:
#   - Go toolchain
#   - dpkg-dev (dpkg-deb) and a Debian/Ubuntu userland (dpkg -S)
#   - GTK3 + webkit2gtk-4.1 dev libraries (cgo headers for the GUI)
#   - the GUI frontend must already be built (gui/frontend/dist);
#     run `cd gui/frontend && npm install && npm run build` first if needed
#
# Result: <output-dir>/pbsgo_<version>_<arch>.deb containing
#   /usr/bin/pbsgo            directory/stream backup CLI
#   /usr/bin/pbsgo-machine    whole-machine (raw disk) backup CLI
#   /usr/bin/pbsgo-nbd        fidx-to-NBD restore tool (run as root)
#   /usr/bin/pbsgo-gui        GUI launcher (current user, probes service + token fetch)
#   /usr/bin/pbsgo-gui-root   GUI launcher (elevated, for machine backup)
#   /usr/lib/pbsgo/pbsgo-gui  GUI application binary
#   /usr/lib/pbsgo/pbsgo-service  Service binary (systemd)
#   /usr/lib/systemd/system/pbsgo.service  systemd unit (NOT enabled by default)
#   /usr/share/applications/pbsgo-gui.desktop
set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
OUT_DIR=${1:-$ROOT/dist}
PKG=pbsgo

wails_field() {
    sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" "$ROOT/gui/wails.json" | head -n1
}

# Read version from git tag (exact match) or git short SHA
# Falls back to wails.json if not in a git repo
if [ -f "$ROOT/scripts/get-version.sh" ]; then
    VERSION=$(bash "$ROOT/scripts/get-version.sh" "$ROOT")
else
    VERSION=$(wails_field productVersion)
fi
[ -n "$VERSION" ] || { echo "error: cannot determine version" >&2; exit 1; }
GUI_OUTPUT=$(wails_field outputfilename)
[ -n "$GUI_OUTPUT" ] || GUI_OUTPUT=ProxmoxBackupClient

ARCH=$(go env GOARCH)
case "$ARCH" in
    amd64) DEBARCH=amd64 ;;
    arm64) DEBARCH=arm64 ;;
    *) echo "error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT INT TERM
STAGE=$WORK/root
mkdir -p "$STAGE/DEBIAN" \
         "$STAGE/usr/bin" \
         "$STAGE/usr/lib/pbsgo" \
         "$STAGE/usr/lib/systemd/system" \
         "$STAGE/usr/share/applications" \
         "$STAGE/usr/share/icons/hicolor/256x256/apps"

CLI_LDFLAGS="-s -w -X main.version=$VERSION -extldflags '-static-pie -Wl,-z,relro,-z,now'"

echo "==> building CLI tools"
(cd "$ROOT/directorybackup" && \
    GOWORK=off go mod tidy >/dev/null && \
    GOWORK=off go build -trimpath -buildmode=pie -ldflags="$CLI_LDFLAGS" -o "$STAGE/usr/bin/pbsgo" .)
(cd "$ROOT/machinebackup" && \
    go build -trimpath -buildmode=pie -ldflags="$CLI_LDFLAGS" -o "$STAGE/usr/bin/pbsgo-machine" .)
(cd "$ROOT/nbd" && \
    go build -trimpath -buildmode=pie -ldflags="$CLI_LDFLAGS" -o "$STAGE/usr/bin/pbsgo-nbd" .)

echo "==> building GUI"
if [ -f "$ROOT/gui/frontend/dist/index.html" ]; then
    :
else
    echo "error: gui/frontend/dist/index.html not found; build the frontend first (cd gui/frontend && npm install && npm run build)" >&2
    exit 1
fi
# The desktop frontend (webkit/gtk cgo) is only linked with the "production"
# build tag; "webkit2_41" selects webkit2gtk-4.1 (wails defaults to the
# EOL webkit2gtk-4.0 otherwise).
GO_TAGS=production
WAILS_EXTRA_TAGS=""
if command -v pkg-config >/dev/null 2>&1 && pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
    GO_TAGS="$GO_TAGS,webkit2_41"
    WAILS_EXTRA_TAGS="-tags webkit2_41"
fi
if command -v wails >/dev/null 2>&1; then
    # shellcheck disable=SC2086
    (cd "$ROOT/gui" && wails build -clean -platform "linux/$ARCH" $WAILS_EXTRA_TAGS -ldflags "-X main.appVersion=$VERSION")
    cp "$ROOT/gui/build/bin/$GUI_OUTPUT" "$STAGE/usr/lib/pbsgo/pbsgo-gui"
else
    (cd "$ROOT/gui" && go build -tags "$GO_TAGS" -ldflags "-s -w -X main.appVersion=$VERSION" -o "$STAGE/usr/lib/pbsgo/pbsgo-gui" .)
fi

echo "==> building service binary (systemd)"
(cd "$ROOT/gui" && GOWORK=off go build -tags service -trimpath -buildmode=pie \
    -ldflags="-s -w -X main.appVersion=$VERSION" \
    -o "$STAGE/usr/lib/pbsgo/pbsgo-service" .)

echo "==> installing launchers, service unit, and metadata"
install -m 0755 "$SCRIPT_DIR/pbsgo-gui" "$STAGE/usr/bin/pbsgo-gui"
install -m 0755 "$SCRIPT_DIR/pbsgo-gui-root" "$STAGE/usr/bin/pbsgo-gui-root"
install -m 0644 "$SCRIPT_DIR/pbsgo-gui.desktop" "$STAGE/usr/share/applications/pbsgo-gui.desktop"
install -m 0644 "$ROOT/packaging/systemd/pbsgo.service" "$STAGE/usr/lib/systemd/system/pbsgo.service"

# Maintainer scripts
install -m 0755 "$SCRIPT_DIR/preinst" "$STAGE/DEBIAN/preinst"
install -m 0755 "$SCRIPT_DIR/postinst" "$STAGE/DEBIAN/postinst"
install -m 0755 "$SCRIPT_DIR/prerm" "$STAGE/DEBIAN/prerm"
install -m 0755 "$SCRIPT_DIR/postrm" "$STAGE/DEBIAN/postrm"
# Prefer the 1024px icon wails generates; fall back to the tracked
# 256px source icon on clean checkouts where gui/build/ does not exist.
if [ -f "$ROOT/gui/build/appicon.png" ]; then
    install -m 0644 "$ROOT/gui/build/appicon.png" "$STAGE/usr/share/icons/hicolor/256x256/apps/pbsgo.png"
else
    install -m 0644 "$ROOT/gui/Icon.png" "$STAGE/usr/share/icons/hicolor/256x256/apps/pbsgo.png"
fi

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
    # Add a dependency for every directly required (NEEDED) shared library,
    # using the package that owns the library file on this system.
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
for bin in "$STAGE"/usr/bin/pbsgo "$STAGE"/usr/bin/pbsgo-machine \
           "$STAGE"/usr/bin/pbsgo-nbd "$STAGE"/usr/lib/pbsgo/pbsgo-gui \
           "$STAGE"/usr/lib/pbsgo/pbsgo-service; do
    scan_deps "$bin"
done
if [ -z "$DEPS" ]; then
    echo "warning: no shared-library dependencies detected; using fallback" >&2
    DEPS="libwebkit2gtk-4.1-0, libgtk-3-0t64 | libgtk-3-0"
fi
echo "    runtime deps: $DEPS"

awk -v v="$VERSION" -v a="$DEBARCH" -v d="$DEPS" '
function rep(s, ph, val,    i) {
    i = index(s, ph)
    if (i > 0) return substr(s, 1, i - 1) val rep(substr(s, i + length(ph)), ph, val)
    return s
}
{
    line = rep($0, "@VERSION@", v)
    line = rep(line, "@ARCH@", a)
    line = rep(line, "@DEPENDS@", d)
    print line
}' "$SCRIPT_DIR/control.in" > "$STAGE/DEBIAN/control"

echo "==> building package"
mkdir -p "$OUT_DIR"
DEB_PATH="$OUT_DIR/${PKG}_${VERSION}_${DEBARCH}.deb"
rm -f "$DEB_PATH"
dpkg-deb --build --root-owner-group "$STAGE" "$DEB_PATH"
echo "==> done: $DEB_PATH"
dpkg-deb --info "$DEB_PATH"
