# MSI removal: keep or delete the settings (test plan)

When the MSI is removed, the settings folder `C:\ProgramData\ProxmoxBackupClient` is kept
unless the property `KEEP_CONFIG` is `0`. This note describes how that works and how to
test it. The installer itself is described in [installer/README.md](installer/README.md).

## What is in the settings folder

```
C:\ProgramData\ProxmoxBackupClient\
  config.json           PBS servers, API tokens, remembered passphrases, preferences
  scheduled_jobs.json   Backup Sets
  job_history.json      Reports
  message_log.json      Message Log
  keys\                 encryption keys made by the app
  api-token             local API token (the service writes a new one at each start)
  *.log, *.log.gz       logs (service-*.log, backup-*.log, one per backup run)
```

## How it works (installer/wix/ProductBody.wxi)

- Property `KEEP_CONFIG`, default `1` (keep).
- Custom action `DeleteConfigFolder`: `cmd.exe /c rmdir /s /q "%ProgramData%\<ExeName>"`,
  deferred, run as SYSTEM (`Impersonate="no"`), errors ignored.
  It runs before RemoveFiles when `(REMOVE="ALL") AND NOT UPGRADINGPRODUCTCODE AND (KEEP_CONFIG="0")`,
  so never during an upgrade.
- Dialog `UninstallDlg` ("Do you want to keep your configuration and logs?") with two radio
  buttons, "Keep configuration" (1) and "Delete configuration" (0). It is in the
  InstallUISequence, shown when `REMOVE="ALL" AND NOT UPGRADINGPRODUCTCODE`, so only during a
  removal with the full installer UI.

Programs and Features shows only "Uninstall" for this product (the WixUI_InstallDir set
sets `ARPNOMODIFY=1`). That runs the removal without the installer's own dialogs, so
`UninstallDlg` is expected not to appear there and the settings are kept. To see the
dialog, run `msiexec /i <the same msi>` on a computer where it is installed and choose Remove.

## Status

Tested on winclient with 0.8.99 and 0.8.100 (commit 5a9b779, 12 October 2026): silent
install, silent upgrade, and silent removal with the settings kept. The dialog and the
delete path have not been tested yet.

## Tests

Before each test: install the MSI, start the app, add a PBS server and one or two Backup
Sets, so the folder holds `config.json` and `scheduled_jobs.json`.

### 1. Silent removal keeps the settings (passed)

`msiexec /x <msi> /qn`

- `C:\Program Files\ProxmoxBackupClient` gone, service `ProxmoxBackupClientSVC` gone,
  Start menu entry gone, Programs and Features entry gone.
- `C:\ProgramData\ProxmoxBackupClient` still there with `config.json` and `scheduled_jobs.json`.

### 2. Silent removal with KEEP_CONFIG=0

`msiexec /x <msi> /qn KEEP_CONFIG=0 /l*v uninstall.log`

- Programs, service and entries gone as in test 1.
- `C:\ProgramData\ProxmoxBackupClient` gone.
- `uninstall.log` shows `DeleteConfigFolder` running.

### 3. Programs and Features > Uninstall

- Expected: no keep or delete question; programs gone; settings kept.

### 4. Full UI removal, keep

`msiexec /i <msi>`, then Remove, then on `UninstallDlg` leave "Keep configuration".

- The dialog appears. Afterwards the settings are kept, as in test 1.

### 5. Full UI removal, delete

As test 4, but choose "Delete configuration".

- The settings folder is gone, as in test 2.

### 6. Upgrade (passed)

Install a newer build over the installed one.

- No keep or delete question. One Programs and Features entry, with the new version.
- Service running. Settings unchanged.

### 7. Cancel

Start a full UI removal (test 4) and press Cancel on `UninstallDlg`.

- App, service and settings all still there.

## If something goes wrong

- Make a verbose log: `msiexec /x <msi> /l*v uninstall.log`, then search it for
  `DeleteConfigFolder` and `KEEP_CONFIG`.
- If the folder is not deleted with `KEEP_CONFIG=0`, check that the app or another
  program does not hold a file open in it (the action ignores errors).

## Known issue: branded builds

`DeleteConfigFolder` and the dialog text use `%ProgramData%\<ExeName>`. The app always
keeps its settings in `C:\ProgramData\ProxmoxBackupClient` (`gui/config.go`), whatever
the brand. For the default brand the two match. For a branded MSI (for example
`NimbusBackup`), `KEEP_CONFIG=0` would try to delete `C:\ProgramData\NimbusBackup` and
leave the real settings behind.
