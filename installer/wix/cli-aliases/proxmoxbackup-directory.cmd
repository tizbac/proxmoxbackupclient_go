@rem proxmoxbackup-directory: its earlier name; it is now salvanza-cli backup-folder.
@"%~dp0salvanza-cli.exe" backup-folder %*
@exit /b %ERRORLEVEL%
