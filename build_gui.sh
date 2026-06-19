#!/bin/bash

# Build script for Proxmox Backup Guardian GUI
# Builds separate GUI binary with Fyne

set -e

echo "🔨 Building Proxmox Backup Guardian GUI..."

cd gui

# Install Fyne dependencies
echo "📦 Installing Fyne..."
go get fyne.io/fyne/v2@latest

# Build for current platform
echo "🏗️  Building GUI binary..."
go build -o ../proxmox-backup-gui .

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
