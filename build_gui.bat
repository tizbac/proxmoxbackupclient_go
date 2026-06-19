@echo off
REM Build script for Proxmox Backup Guardian GUI (Windows)
REM This script MUST be run on a Windows machine for proper OpenGL support

echo ========================================
echo Building Proxmox Backup Guardian GUI
echo ========================================
echo.

cd gui

echo [1/3] Checking Go installation...
go version
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: Go is not installed or not in PATH
    echo Download from: https://go.dev/dl/
    pause
    exit /b 1
)

echo.
echo [2/3] Installing Fyne dependencies...
go get fyne.io/fyne/v2@latest
go mod tidy

echo.
echo [3/3] Building GUI binary with OpenGL support...
go build -ldflags="-s -w -H windowsgui" -o ..\proxmox-backup-gui.exe .

cd ..

if exist proxmox-backup-gui.exe (
    echo.
    echo ========================================
    echo ✅ Build complete!
    echo ========================================
    echo.
    echo Binary created: proxmox-backup-gui.exe
    dir proxmox-backup-gui.exe | findstr /C:"proxmox-backup-gui.exe"
    echo.
    echo To run:
    echo   .\proxmox-backup-gui.exe
    echo.
) else (
    echo.
    echo ❌ Build failed - binary not created
    pause
    exit /b 1
)
