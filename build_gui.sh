#!/bin/bash

# Build script for Proxmox Backup Guardian GUI
# Builds separate GUI binary with Wails

set -e

echo "🔨 Building Proxmox Backup Guardian GUI..."

cd gui

cd frontend
npm install
npm run build
cd ..

# Link against webkit2gtk-4.1 when it is available. Wails defaults to the EOL
# webkit2gtk-4.0 unless the "webkit2_41" build tag is set; distro versions that
# no longer ship 4.0 (Debian Trixie, Ubuntu 24.04+, Fedora, Arch) only have 4.1.
TAGS="desktop,production"
if command -v pkg-config >/dev/null 2>&1 && pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
    echo "🐧 Found webkit2gtk-4.1, adding webkit2_41 tag..."
    TAGS="$TAGS,webkit2_41"
else
    echo "⚠️  webkit2gtk-4.1 not found; falling back to webkit2gtk-4.0 (EOL)." >&2
    echo "   Install it (e.g. libwebkit2gtk-4.1-dev / webkit2gtk4.1-devel)." >&2
fi

# Build for current platform
echo "🏗️  Building GUI binary..."
# shellcheck disable=SC2086
go build -tags "$TAGS" -o ../proxmox-backup-gui .

cd ..

echo "✅ Build complete!"
echo ""
echo "Binaries created:"
echo "  - proxmox-backup-gui (GUI version - heavier)"
echo ""
echo "To build CLI version:"
echo "  ./build.sh"
echo ""
echo "To run GUI:"
echo "  ./proxmox-backup-gui"
