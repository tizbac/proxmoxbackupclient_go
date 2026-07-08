#!/bin/bash

# Build script for Proxmox Backup Guardian GUI
# Builds separate GUI binary with Fyne

set -e

echo "🔨 Building Proxmox Backup Guardian GUI..."

cd gui

cd frontend
npm install
npm run build
cd ..

# Install Fyne dependencies
echo "📦 Installing Fyne..."
go get fyne.io/fyne/v2@latest

# Check if running on Debian Trixie
is_debian_trixie=false
if [ -f /etc/debian_version ]; then
    # Get Debian version
    debian_version=$(cat /etc/debian_version 2>/dev/null | cut -d'.' -f1)
    if [ "$debian_version" = "13" ]; then
        is_debian_trixie=true
    fi
fi

# Build for current platform
echo "🏗️  Building GUI binary..."
if [ "$is_debian_trixie" = true ]; then
    echo "🐧 Detected Debian Trixie, adding webkit2_41 tag..."
    go build -tags "desktop,production,webkit2_41" -o ../proxmox-backup-gui .
else
    go build -tags "desktop,production" -o ../proxmox-backup-gui .
fi

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
