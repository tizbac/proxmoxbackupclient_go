@echo off
REM Build script for Nimbus Backup MSI installer
REM Requires WiX Toolset 3.x or 4.x installed

echo Building Nimbus Backup MSI Installer...
echo.

REM Check if WiX is installed
where candle.exe >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: WiX Toolset not found in PATH
    echo Please install WiX Toolset from https://wixtoolset.org/
    exit /b 1
)

REM Clean previous build
if exist *.wixobj del *.wixobj
if exist *.wixpdb del *.wixpdb
if exist *.msi del *.msi

REM Compile WiX source
echo [1/2] Compiling WiX source...
candle.exe Product.wxs -ext WixUIExtension -ext WixUtilExtension
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: Compilation failed
    exit /b 1
)

REM Link and create MSI
echo [2/2] Creating MSI package...
light.exe Product.wixobj -ext WixUIExtension -ext WixUtilExtension -out NimbusBackup.msi
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: Linking failed
    exit /b 1
)

echo.
echo SUCCESS! MSI created: NimbusBackup.msi
echo.
pause
