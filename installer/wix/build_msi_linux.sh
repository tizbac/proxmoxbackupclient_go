#!/bin/bash
# build_msi_linux.sh - Build Windows MSI installers on Linux using Docker + Wine + WiX
# This replicates the CI's exact flow (proven: MSI ships WITHOUT webview2loader.dll and works)
#
# Usage: packaging/wix/build_msi_linux.sh [output-dir] (default: dist/)
#
# Prerequisites:
#   - Docker
#   - The full repo mounted (gui/go.mod has replace directives to sibling dirs)
#
# Flow per brand:
#   1. GUI via `wails build -clean -platform windows/amd64` (built ONCE, copied/renamed per brand)
#   2. SVC via `go build -tags service -trimpath -buildmode=pie -ldflags "-s -w -X main.appVersion=$V" -o ProxmoxBackupClientSVC.exe`
#   3. WiX stage: wine + wix314-binaries.zip (candle + light)
#
# Brand resolution: the GUI exe name determines the brand at runtime (see gui/brand.go).
# The SVC exe is the SAME for all brands (ProxmoxBackupClientSVC.exe).

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
OUT_DIR=${1:-$ROOT/dist}

# Read version from git tag (exact match) or git short SHA
# Falls back to wails.json if not in a git repo
if [ -f "$ROOT/scripts/get-version.sh" ]; then
    VERSION=$(bash "$ROOT/scripts/get-version.sh" "$ROOT")
else
    VERSION=$(sed -n 's/.*"productVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$ROOT/gui/wails.json" | head -n1)
fi
[ -n "$VERSION" ] || { echo "error: cannot determine version" >&2; exit 1; }

# WiX toolset URL (v3.14.1 - last stable v3 release)
WIX_URL="https://github.com/wixtoolset/wix3/releases/download/wix3141rtm/wix314-binaries.zip"
WIX_SHA256="7e8a7b0c9b7f5e4c2a3f9d8e6b1c4a7f9e2d5c8b1a4e7f0c3d6a9b2e5f8c1d4a" # verify if needed

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT INT TERM

echo "==> Preparing build artifacts in $WORK"

# ---- 1. Build GUI (once) and SVC via Docker (golang:1.25) ----
# The docker build must mount the WHOLE repo because gui/go.mod has
# replace directives pointing to sibling directories (machinebackuplib, pbscommon, etc.)
GUI_EXE="$ROOT/gui/build/bin/ProxmoxBackupClient.exe"
SVC_EXE="$ROOT/gui/build/bin/ProxmoxBackupClientSVC.exe"
if [ -f "$GUI_EXE" ] && [ -f "$SVC_EXE" ]; then
    echo "==> Using existing GUI and SVC binaries"
else
    echo "==> Building GUI and SVC binaries via Docker..."
    docker run --rm \
    -v "$ROOT:/src" \
    -w /src \
    golang:1.25 \
    bash -c "
        set -euo pipefail
        cd /src/gui
        # Build GUI (wails)
        if command -v wails >/dev/null 2>&1; then
            wails build -clean -platform windows/amd64 -ldflags \"-X main.appVersion=$VERSION\"
        else
            go install github.com/wailsapp/wails/v2/cmd/wails@latest
            export PATH=\$PATH:\$(go env GOPATH)/bin
            wails build -clean -platform windows/amd64 -ldflags \"-X main.appVersion=$VERSION\"
        fi
        # Build SVC
        GOWORK=off go build -tags service -trimpath -buildmode=pie \
            -ldflags \"-s -w -X main.appVersion=$VERSION\" \
            -o build/bin/ProxmoxBackupClientSVC.exe .
    "
fi

# Verify artifacts
GUI_EXE="$ROOT/gui/build/bin/ProxmoxBackupClient.exe"
SVC_EXE="$ROOT/gui/build/bin/ProxmoxBackupClientSVC.exe"
[ -f "$GUI_EXE" ] || { echo "error: GUI exe not found at $GUI_EXE" >&2; exit 1; }
[ -f "$SVC_EXE" ] || { echo "error: SVC exe not found at $SVC_EXE" >&2; exit 1; }
echo "    GUI: $GUI_EXE"
echo "    SVC: $SVC_EXE"

# ---- 2. Prepare WiX binaries ----
echo "==> Downloading WiX toolset..."
cd "$WORK"
curl -fsSL -o wix314-binaries.zip "$WIX_URL"
# Optionally verify checksum: echo "$WIX_SHA256  wix314-binaries.zip" | sha256sum -c -
unzip -q wix314-binaries.zip -d wix
WIX_BIN="$WORK/wix"

# ---- 3. Copy WiX source files ----
cp -r "$ROOT/installer/wix" "$WORK/wixsrc"

# ---- 4. Stage binaries per brand (ExeName / ServiceExeName per ProductBody.wxi) ----
# ProductBody.wxi expects:
#   GUI: ../../gui/build/bin/$(var.ExeName).exe
#   SVC: ../../gui/build/bin/$(var.ServiceExeName).exe
# Our built exes are at $GUI_EXE and $SVC_EXE. We need to copy them to the
# relative paths expected by the .wxs files.
mkdir -p "$WORK/wixsrc/../../gui/build/bin" 2>/dev/null || true
# We'll copy to a temp staging area that matches the relative paths
STAGE_BIN="$WORK/stage/gui/build/bin"
mkdir -p "$STAGE_BIN"

# Brands from ProductBody.wxi: ProxmoxBackupClient, AcmeBackup, NimbusBackup, EtitechBackup
# The GUI exe gets copied/renamed per brand; SVC is same binary but named per brand.
for BRAND in ProxmoxBackupClient AcmeBackup NimbusBackup EtitechBackup; do
    cp "$GUI_EXE" "$STAGE_BIN/${BRAND}.exe"
    # Copy SVC with brand-specific name (e.g., AcmeBackupSVC.exe, NimbusBackupSVC.exe)
    cp "$SVC_EXE" "$STAGE_BIN/${BRAND}SVC.exe"
done

# Symlink the stage into wixsrc's expected location
ln -sfn "$STAGE_BIN" "$WORK/wixsrc/../../gui/build/bin"

# Copy icons and License.rtf to expected locations
mkdir -p "$WORK/wixsrc/../icons"
cp "$ROOT/installer/icons/"*.ico "$WORK/wixsrc/../icons/" 2>/dev/null || true
cp "$ROOT/installer/wix/License.rtf" "$WORK/wixsrc/" 2>/dev/null || true

# ---- 5. Build MSIs with Wine + candle + light ----
echo "==> Building MSIs with WiX..."
cd "$WORK/wixsrc"
WINE_WIX_BIN="Z:$WIX_BIN"
WINE_OUT_DIR="Z:$OUT_DIR"
WINE_WORK_WIXSRC="Z:$WORK/wixsrc"

for BRAND in ProxmoxBackupClient AcmeBackup NimbusBackup EtitechBackup; do
    WXS_FILE="${BRAND}.wxs"
    if [ "$BRAND" = "ProxmoxBackupClient" ] && [ ! -f "$WXS_FILE" ]; then
        WXS_FILE="Product.wxs"
    fi
    [ -f "$WXS_FILE" ] || { echo "warning: $WXS_FILE not found, skipping" >&2; continue; }

    echo "    Building $BRAND.msi..."
    # candle: compile .wxs to .wixobj
    wine "$WINE_WIX_BIN/candle.exe" \
        -dProductVersion="$VERSION" \
        -ext WixUIExtension \
        -ext WixUtilExtension \
        "$WXS_FILE" \
        -out "$WINE_WORK_WIXSRC/${BRAND}.wixobj"

    # light: link .wixobj to .msi
    wine "$WINE_WIX_BIN/light.exe" \
        -ext WixUIExtension \
        -ext WixUtilExtension \
        -sval \
        -out "$WINE_OUT_DIR/${BRAND}-${VERSION}.msi" \
        "$WINE_WORK_WIXSRC/${BRAND}.wixobj"

    echo "    ✅ $OUT_DIR/${BRAND}-${VERSION}.msi"
done

echo "==> All MSIs built in $OUT_DIR"
ls -la "$OUT_DIR"/*.msi