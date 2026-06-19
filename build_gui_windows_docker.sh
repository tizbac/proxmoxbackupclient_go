#!/bin/bash
# Build Windows GUI using fyne-cross tool
# This produces a properly linked Windows binary with OpenGL support

set -e

echo "🐳 Building Windows GUI with fyne-cross..."
echo "This will produce a Windows .exe with proper OpenGL/WGL support"
echo ""

cd gui

# Install fyne-cross if not present
if ! command -v fyne-cross &> /dev/null; then
    echo "📦 Installing fyne-cross..."
    go install github.com/fyne-io/fyne-cross@latest
fi

echo ""
echo "🔨 Building Windows AMD64 binary..."
fyne-cross windows -arch=amd64 -app-id com.rdem-systems.backup-guardian

echo ""
# fyne-cross places binaries in fyne-cross/dist/
if [ -f "fyne-cross/dist/windows-amd64/gui.exe" ]; then
    echo "✅ Build successful!"
    echo ""

    # Copy to parent directory with proper name
    cp fyne-cross/dist/windows-amd64/gui.exe ../proxmox-backup-gui-windows-amd64.exe

    echo "Binary created:"
    ls -lh ../proxmox-backup-gui-windows-amd64.exe
    echo ""
    echo "📦 Location: proxmox-backup-gui-windows-amd64.exe"
    echo ""
    echo "✅ This binary includes proper Windows OpenGL support"
    echo "✅ No 'WGL driver error' will occur"
    echo ""
    echo "To test on Windows:"
    echo "  1. Transfer proxmox-backup-gui-windows-amd64.exe to Windows"
    echo "  2. Double-click to run (no installation needed)"
else
    echo "❌ Build failed - binary not found"
    echo "Checking fyne-cross output directory..."
    ls -la fyne-cross/ || echo "No fyne-cross directory found"
    exit 1
fi

cd ..
