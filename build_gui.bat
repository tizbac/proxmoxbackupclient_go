@echo off
REM Build script for Proxmox Backup Guardian GUI (Windows)
REM The GUI is a Wails application; on Windows it renders through WebView2.

echo ========================================
echo Building Proxmox Backup Guardian GUI
echo ========================================
echo.

cd gui

echo [1/2] Checking Go installation...
go version
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: Go is not installed or not in PATH
    echo Download from: https://go.dev/dl/
    pause
    exit /b 1
)

echo.
echo [2/2] Building GUI binary with WebView2 support...
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
