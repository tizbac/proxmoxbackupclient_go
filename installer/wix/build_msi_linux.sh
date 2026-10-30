#!/bin/bash
# build_msi_linux.sh - Build Windows MSI installers on Linux using Docker + Wine + WiX
# This replicates the CI's exact flow (proven: MSI ships WITHOUT webview2loader.dll and works)
#
# Usage: installer/wix/build_msi_linux.sh [output-dir] (default: dist/)
#
# Prerequisites:
#   - Docker (everything else - GUI/SVC compilation AND wine/WiX - runs in
#     containers; the host needs neither Go, Node, nor a working wine prefix)
#   - The full repo mounted (gui/go.mod has replace directives to sibling dirs)
#
# Flow per brand:
#   1. GUI via `wails build -clean -platform windows/amd64` (built ONCE, copied/renamed per brand)
#   2. SVC via `go build -tags service -trimpath -buildmode=pie -ldflags "-s -w -X main.appVersion=$V" -o ProxmoxBackupClientSVC.exe`
#   3. WiX stage: candle + light from wix314-binaries, run under Wine INSIDE
#      Docker (installer/wix/Dockerfile.wine: Debian + wine + wine-mono + WiX)
#
# Brand resolution: the GUI exe name determines the brand at runtime (see gui/brand.go).
# The SVC exe is the SAME for all brands (ProxmoxBackupClientSVC.exe).

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
OUT_DIR=${1:-$ROOT/dist}
# Absolutize: the output dir is bind-mounted into the wine container and wine
# addresses it as Z:/out/...; docker also rejects relative -v sources. A
# relative OUT_DIR (e.g. the Makefile passing "dist") must therefore be turned
# into an absolute path before anything runs.
mkdir -p "$OUT_DIR"
OUT_DIR=$(cd "$OUT_DIR" && pwd)

# Read version from git tag (exact match) or git short SHA
# Falls back to wails.json if not in a git repo
if [ -f "$ROOT/scripts/get-version.sh" ]; then
    VERSION=$(bash "$ROOT/scripts/get-version.sh" "$ROOT")
else
    VERSION=$(sed -n 's/.*"productVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$ROOT/gui/wails.json" | head -n1)
fi
[ -n "$VERSION" ] || { echo "error: cannot determine version" >&2; exit 1; }

# Windows Installer (and therefore WiX Product/@Version) only accepts a
# numeric x.y.z[.w] tuple, each field 0..65534. A tag-less build reports
# "dev-<sha>" (and a tag may be prefixed with "v"), neither of which compiles
# (CNDL0108). For those builds derive
#
#     <wails.json productVersion>.<number of commits>
#
# so every dev MSI carries a valid AND strictly increasing ProductVersion
# (1.2.0.738), while a tagged release keeps its own numeric version.
BASE_VERSION=$(sed -n 's/.*"productVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$ROOT/gui/wails.json" | head -n1)

msi_product_version() {
    local v="$1" core p1 p2 p3 count
    # already numeric x.y.z or x.y.z.w (an optional leading "v" from a tag)
    v="${v#v}"
    if printf '%s\n' "$v" | grep -Eq '^[0-9]+(\.[0-9]+){2,3}$'; then
        printf '%s\n' "$v"
        return
    fi
    # prerelease tag (1.2.0-rc.1, 1.2.0+build): WiX only accepts the numeric
    # core, so use it rather than falling through to the commit-count scheme.
    core="${v%%[-+]*}"
    if printf '%s\n' "$core" | grep -Eq '^[0-9]+(\.[0-9]+){2}$'; then
        printf '%s\n' "$core"
        return
    fi
    IFS=. read -r p1 p2 p3 _ <<<"$BASE_VERSION"
    p1="${p1//[^0-9]/}"; p2="${p2//[^0-9]/}"; p3="${p3//[^0-9]/}"
    p1="${p1:-0}"; p2="${p2:-0}"; p3="${p3:-0}"
    p1=$((p1 > 65534 ? 65534 : p1))
    p2=$((p2 > 65534 ? 65534 : p2))
    p3=$((p3 > 65534 ? 65534 : p3))
    count=$(git -C "$ROOT" rev-list --count HEAD 2>/dev/null || printf '0')
    count=$(( ${count:-0} % 65534 ))
    printf '%s.%s.%s.%s\n' "$p1" "$p2" "$p3" "$count"
}

PRODUCT_VERSION=$(msi_product_version "$VERSION")
echo "==> Version: $VERSION (MSI ProductVersion: $PRODUCT_VERSION)"

# Toolchain image: Debian + wine + wine-mono + WiX 3.14 (see Dockerfile.wine).
# candle/light are .NET exes; running them with the HOST wine used to fail on
# machines without a working prefix ("wine: could not load kernel32.dll,
# status c0000135"), so wine only ever runs inside this image.
WINE_IMAGE="proxmoxbackup-msi-wine:1"

WORK=$(mktemp -d)

cleanup() {
    # The wine container runs as root, so a build that dies mid-way can leave
    # root-owned files in $WORK which the invoking user could not delete.
    if [ -d "${WORK:-}" ]; then
        docker run --rm -v "$WORK:/work" --entrypoint chown "$WINE_IMAGE" \
            -R "$(id -u):$(id -g)" /work >/dev/null 2>&1 || true
        rm -rf "$WORK" || true
    fi
}
trap cleanup EXIT INT TERM

echo "==> Preparing build artifacts in $WORK"

# ---- 1. Build GUI (once) and SVC via Docker (golang:1.25) ----
# The docker build must mount the WHOLE repo because gui/go.mod has
# replace directives pointing to sibling directories (machinebackuplib, pbscommon, etc.)
GUI_EXE="$ROOT/gui/build/bin/ProxmoxBackupClient.exe"
SVC_EXE="$ROOT/gui/build/bin/ProxmoxBackupClientSVC.exe"

# is_windows_pe <file> - true if the file is a Windows PE executable.
# Guards against packaging a Linux ELF as the Windows service: Windows cannot
# start it (error 193/1053) and the MSI install fails at ServiceControl.
is_windows_pe() {
    [ -f "$1" ] && file "$1" 2>/dev/null | grep -q "PE32"
}

# newest_source_mtime <paths...> - newest mtime (whole seconds) among the
# sources that end up inside the binaries. Build outputs (frontend dist,
# generated wailsjs bindings, the exes themselves) are excluded, otherwise a
# build would immediately invalidate its own result.
newest_source_mtime() {
    find "$@" -type f \
        \( -name '*.go' -o -name '*.jsx' -o -name '*.js' -o -name '*.ts' -o -name '*.tsx' \
           -o -name '*.css' -o -name '*.html' -o -name '*.json' \
           -o -name '*.png' -o -name '*.ico' -o -name '*.svg' -o -name '*.webp' \) \
        -not -path '*/node_modules/*' \
        -not -path '*/frontend/dist/*' \
        -not -path '*/frontend/wailsjs/*' \
        -not -path '*/build/bin/*' \
        -not -name '*_test.go' \
        -printf '%T@\n' 2>/dev/null | sort -rn | head -n1 | cut -d. -f1
}

# source_is_newer <binary> <paths...> - true when any source is newer than the
# binary, i.e. the binary is stale.
#
# A "is it a valid PE" check alone is NOT enough: that is how an exe built
# before gui/brand.go gained the Etitech entry kept being shipped as
# EtitechBackup.exe — the file was a perfectly valid PE, but ResolveBrand()
# inside it only knew the old brands, so the installed app silently ran with
# the default Proxmox branding instead of the brand its exe name promised.
source_is_newer() {
    local bin="$1" newest bin_mtime
    shift
    [ -f "$bin" ] || return 0 # missing counts as stale
    newest=$(newest_source_mtime "$@")
    [ -n "$newest" ] || return 1
    bin_mtime=$(stat -c %Y "$bin")
    [ "$newest" -gt "$bin_mtime" ]
}

NEED_GUI=0
NEED_SVC=0
is_windows_pe "$GUI_EXE" || NEED_GUI=1
is_windows_pe "$SVC_EXE" || NEED_SVC=1
source_is_newer "$GUI_EXE" "$ROOT/gui" && { NEED_GUI=1; echo "==> GUI sources are newer than $GUI_EXE — rebuild required"; }
source_is_newer "$SVC_EXE" "$ROOT/gui" && { NEED_SVC=1; echo "==> SVC sources are newer than $SVC_EXE — rebuild required"; }

if [ "$NEED_GUI" = 0 ] && [ "$NEED_SVC" = 0 ]; then
    echo "==> Using up-to-date GUI and SVC binaries"
else
    echo "==> Building missing/stale binaries via Docker (GUI=$NEED_GUI, SVC=$NEED_SVC)..."
    # Each artifact is rebuilt when it is missing, is not a Windows PE, or is
    # older than the sources that go into it. A wails rebuild needs npm (not
    # shipped by golang:1.25), so only an actually-stale GUI pays for it.
    docker run --rm \
    -e NEED_GUI="$NEED_GUI" \
    -e NEED_SVC="$NEED_SVC" \
    -e VERSION="$VERSION" \
    -v "$ROOT:/src" \
    -w /src \
    golang:1.25 \
    bash -c '
        set -euo pipefail
        cd /src/gui
        if [ "${NEED_GUI:-0}" = "1" ]; then
            # wails compiles the React frontend: golang:1.25 has no node/npm
            if ! command -v npm >/dev/null 2>&1; then
                echo "==> installing nodejs/npm in build container"
                apt-get update -qq
                apt-get install -y -qq --no-install-recommends nodejs npm >/dev/null
            fi
            if ! command -v wails >/dev/null 2>&1; then
                go install github.com/wailsapp/wails/v2/cmd/wails@latest
                export PATH="$PATH:$(go env GOPATH)/bin"
            fi
            wails build -clean -platform windows/amd64 -ldflags "-X main.appVersion=$VERSION"
        fi
        if [ "${NEED_SVC:-0}" = "1" ]; then
            # The bind-mounted repo is owned by a different uid than the
            # container root, so git refuses it ("dubious ownership") and VCS
            # stamping then aborts the build. Version info comes from -ldflags,
            # so stamping is disabled instead of fighting git safe.directory.
            # NOTE: this bash -c body is single quoted, keep apostrophes out.
            git config --global --add safe.directory /src 2>/dev/null || true
            # This container is Linux: GOOS=windows is mandatory, otherwise the
            # result is an ELF and the Windows SCM cannot start it (error 193,
            # service timeout 1053) - the MSI install would then fail too.
            GOOS=windows GOARCH=amd64 CGO_ENABLED=0 GOWORK=off \
                go build -tags service -trimpath -buildmode=pie -buildvcs=false \
                -ldflags "-s -w -X main.appVersion=$VERSION" \
                -o build/bin/ProxmoxBackupClientSVC.exe .
        fi
    '
    # The container runs as root on a bind mount, so everything it writes
    # (frontend dist, generated wailsjs bindings, the exes, node_modules) comes
    # out root-owned: the next local build or npm install then fails until
    # somebody remembers a sudo chown. Hand the outputs back to the invoking
    # user. Reuses the image already pulled above, so this needs no download.
    docker run --rm -v "$ROOT:/src" -w /src golang:1.25 \
        chown -R "$(id -u):$(id -g)" \
        /src/gui/build /src/gui/frontend/dist /src/gui/frontend/wailsjs \
        /src/gui/frontend/node_modules || true
fi

# Verify artifacts
GUI_EXE="$ROOT/gui/build/bin/ProxmoxBackupClient.exe"
SVC_EXE="$ROOT/gui/build/bin/ProxmoxBackupClientSVC.exe"
[ -f "$GUI_EXE" ] || { echo "error: GUI exe not found at $GUI_EXE" >&2; exit 1; }
[ -f "$SVC_EXE" ] || { echo "error: SVC exe not found at $SVC_EXE" >&2; exit 1; }
if ! is_windows_pe "$SVC_EXE"; then
    echo "error: $SVC_EXE is not a Windows PE binary (built for the wrong OS?)" >&2
    file "$SVC_EXE" >&2
    exit 1
fi
echo "    GUI: $GUI_EXE"
echo "    SVC: $SVC_EXE"

# ---- 2. Build the WiX/Wine toolchain image ----
# All tools (wine, wine-mono, candle/light) live in this image; repeated builds
# hit the docker cache and only re-run a layer when the Dockerfile changes.
echo "==> Building toolchain image $WINE_IMAGE..."
docker build -t "$WINE_IMAGE" -f "$ROOT/installer/wix/Dockerfile.wine" "$ROOT/installer/wix"

# ---- 3. Copy WiX source files ----
# Mirror the repo layout inside $WORK (installer/wix, installer/icons, gui/...):
# WiX resolves every File/@Source and Icon/@SourceFile path relative to the
# .wxs/.wxi that references it (../../gui/build/bin/..., ../icons/...), so
# keeping the same relative structure makes those paths land inside $WORK,
# which the trap above cleans up - and which concurrent builds cannot share.
# The old scheme staged the binaries OUTSIDE $WORK (in the parent of $WORK,
# i.e. a shared /tmp location) and symlinked them in: the "mkdir -p" there
# created a REAL directory first, so "ln -sfn" placed the symlink *inside* it
# (/tmp/gui/build/bin/bin) instead of replacing it, and light.exe failed with
# LGHT0103 "cannot find ../../gui/build/bin/*.exe".
mkdir -p "$WORK/installer"
cp -r "$ROOT/installer/wix" "$WORK/installer/wix"
WIXSRC="$WORK/installer/wix"

# ---- 4. Stage binaries per brand (ExeName / ServiceExeName per ProductBody.wxi) ----
# ProductBody.wxi expects:
#   GUI: ../../gui/build/bin/$(var.ExeName).exe   -> $WIXSRC/../../gui/build/bin
#   SVC: ../../gui/build/bin/$(var.ServiceExeName).exe
# Our built exes are at $GUI_EXE and $SVC_EXE. We copy them to the relative
# paths the .wxs files expect.
STAGE_BIN="$WORK/gui/build/bin"
mkdir -p "$STAGE_BIN"

# Brands from ProductBody.wxi: ProxmoxBackupClient, AcmeBackup, NimbusBackup, EtitechBackup
# The GUI exe gets copied/renamed per brand; SVC is same binary but named per brand.
for BRAND in ProxmoxBackupClient AcmeBackup NimbusBackup EtitechBackup; do
    cp "$GUI_EXE" "$STAGE_BIN/${BRAND}.exe"
    # Copy SVC with brand-specific name (e.g., AcmeBackupSVC.exe, NimbusBackupSVC.exe)
    cp "$SVC_EXE" "$STAGE_BIN/${BRAND}SVC.exe"
done

# Copy icons and License.rtf to expected locations:
#   ../icons/*.ico  -> $WIXSRC/../icons (Icon/@SourceFile in ProductBody.wxi)
#   License.rtf     -> $WIXSRC (WixUILicenseRtf)
mkdir -p "$WORK/installer/icons"
cp "$ROOT/installer/icons/"*.ico "$WORK/installer/icons/" 2>/dev/null || true
cp "$ROOT/installer/wix/License.rtf" "$WIXSRC/" 2>/dev/null || true

# ---- 5. Build MSIs with Wine + candle + light (inside Docker) ----
echo "==> Building MSIs with WiX..."
cd "$WIXSRC"

# run_wine <cmd...> - run the toolchain container. $WORK is mounted at /work,
# the output dir at /out; wine maps Z: to the container root, so Z:/work/...
# addresses the sources staged in steps 3 and 4. Nothing depends on the
# host's wine installation or prefix.
run_wine() {
    docker run --rm \
        -v "$WORK:/work" \
        -v "$OUT_DIR:/out" \
        -w /work/installer/wix \
        "$WINE_IMAGE" "$@"
}

for BRAND in ProxmoxBackupClient AcmeBackup NimbusBackup EtitechBackup; do
    WXS_FILE="${BRAND}.wxs"
    if [ "$BRAND" = "ProxmoxBackupClient" ] && [ ! -f "$WXS_FILE" ]; then
        WXS_FILE="Product.wxs"
    fi
    [ -f "$WXS_FILE" ] || { echo "warning: $WXS_FILE not found, skipping" >&2; continue; }

    echo "    Building $BRAND.msi..."
    # candle: compile .wxs to .wixobj
    run_wine wine Z:/opt/wix/candle.exe \
        -dProductVersion="$PRODUCT_VERSION" \
        -ext WixUIExtension \
        -ext WixUtilExtension \
        "Z:/work/installer/wix/$WXS_FILE" \
        -out "Z:/work/installer/wix/${BRAND}.wixobj"

    # light: link .wixobj to .msi. It resolves ProductBody.wxi's relative
    # Source paths (../../gui/build/bin/..., ../icons/...) against
    # Z:\work\installer\wix - i.e. into the mounted $WORK.
    run_wine wine Z:/opt/wix/light.exe \
        -ext WixUIExtension \
        -ext WixUtilExtension \
        -sval \
        -out "Z:/out/${BRAND}-${VERSION}.msi" \
        "Z:/work/installer/wix/${BRAND}.wixobj"

    echo "    ✅ $OUT_DIR/${BRAND}-${VERSION}.msi"
done

# The wine containers run as root; hand the results back to the invoking user
# (same pattern as the GUI/SVC build above).
docker run --rm -v "$WORK:/work" -v "$OUT_DIR:/out" --entrypoint chown \
    "$WINE_IMAGE" -R "$(id -u):$(id -g)" /work /out || true

echo "==> All MSIs built in $OUT_DIR"
ls -la "$OUT_DIR"/*.msi