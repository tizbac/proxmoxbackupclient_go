#!/bin/bash
# Build Windows GUI using Docker with the Wails framework
# The GUI is a Wails application (NOT Fyne), so it must be built
# with `wails build`, not fyne-cross.
#
# The whole repository is mounted into the container so that the
# `replace` directives in gui/go.mod (../machinebackuplib, ../pbscommon,
# ../pkg/retry, ../pkg/security, ../snapshot, ../clientcommon) resolve
# correctly.

set -e

echo "🐳 Building Windows GUI with Docker (Wails)..."
echo "This will produce a Windows .exe with proper WebView2 support"
echo ""
echo "📦 Output: ${PWD}/ProxmoxBackupClientGO.exe"

OUTPUT_NAME="ProxmoxBackupClientGO.exe"
OUTPUT_PATH="${PWD}/${OUTPUT_NAME}"
CONTAINER_NAME="nimbusbackup-gui-build"

cleanup() {
    docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Run the build in a disposable container. We mount the entire repository
# (not just gui/) so the local module replacements resolve correctly.
docker run --rm \
    --name "${CONTAINER_NAME}" \
    -v "$(pwd):/build" \
    -w /build \
    golang:1.25 \
    bash -euo pipefail -c '
        set -e
        echo "📦 Installing build dependencies..."
        apt-get update -qq
        DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
            gcc \
            gcc-mingw-w64-x86-64 \
            nodejs \
            npm \
        >/dev/null

        echo "📦 Installing Wails CLI..."
        go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
        export PATH="$(go env GOPATH)/bin:$PATH"

        echo "📦 Building frontend (JS)..."
        cd /build/gui/frontend
        npm install >/dev/null 2>&1
        npm run build
        cd /build

        echo "🔨 Building Windows AMD64 binary..."
        VERSION=$(grep -o '"productVersion"[^,]*' /build/gui/wails.json | head -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')
        cd /build/gui
        wails build -clean -platform windows/amd64 \
            -ldflags "-X main.appVersion=${VERSION:-dev}"
        cd /build

        if [ -f /build/gui/build/bin/NimbusBackup.exe ]; then
            echo "✅ Build successful!"
            cp /build/gui/build/bin/NimbusBackup.exe /build/ProxmoxBackupClientGO.exe
        else
            echo "❌ Build failed - binary not found"
            ls -la /build/gui/build/bin/ 2>/dev/null || echo "No output bin directory found"
            exit 1
        fi
    '

echo ""
if [ -f "${OUTPUT_PATH}" ]; then
    echo "Binary created:"
    ls -lh "${OUTPUT_PATH}"
    echo ""
    echo "📦 Location: ${OUTPUT_PATH}"
    echo ""
    echo "✅ This binary includes proper Windows WebView2 support"
    echo ""
    echo "To test on Windows:"
    echo "  1. Transfer ${OUTPUT_PATH} to Windows"
    echo "  2. Double-click to run"
else
    echo "❌ Build failed - binary not found"
    exit 1
fi
