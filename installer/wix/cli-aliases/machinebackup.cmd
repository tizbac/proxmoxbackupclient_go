@rem machinebackup: its earlier name; it is now salvanza-cli backup-disk.
@"%~dp0salvanza-cli.exe" backup-disk %*
@exit /b %ERRORLEVEL%
