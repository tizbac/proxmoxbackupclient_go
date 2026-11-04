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

- Property `KEEP_CONFIG`, default `1` (keep). It is a secure property, so a choice made in
  the installer's pages reaches the elevated part of the removal.
- Custom action `DeleteConfigFolder`: `cmd.exe /c rmdir /s /q "%ProgramData%\ProxmoxBackupClient"`,
  deferred, run as SYSTEM (`Impersonate="no"`), errors ignored. It runs before RemoveFiles
  when `(REMOVE="ALL") AND NOT UPGRADINGPRODUCTCODE AND (KEEP_CONFIG="0")`, so never during
  an upgrade. The folder is the same for every brand, as in the app (`gui/config.go`).
- Page `UninstallDlg` ("Do you want to keep your configuration and logs?") with two radio
  buttons, "Keep configuration" (1) and "Delete configuration" (0). It sits in the
  maintenance wizard: Remove on the "Change, repair, or remove" page leads to it, its Next
  leads to "Ready to remove", and Back on that page returns to it.
- The installer's pages are WixUI_InstallDir's, copied as `WixUI_PbcInstallDir` without its
  `ARPNOMODIFY` property. So Programs and Features offers "Change" (Windows 11 Settings:
  "Modify"), which opens those pages. `ARPNOMODIFY` is set only while the pages run (so the
  unused Change button on them stays greyed) and cleared before the product is registered.

Programs and Features' "Uninstall" (Settings: "Uninstall") still removes without the
installer's pages, so the question is not asked there and the settings are kept. To be
asked, use "Change" or "Modify", then Remove.

## Status

- October 2026, on a Windows test machine, with an older and a newer build: silent install,
  silent upgrade, silent removal with the settings kept.
- October 2026, on the same machine: silent install (the uninstall entry has no
  `NoModify`, so "Change" shows; service `ProxmoxBackupClientSVC` running), silent removal
  with the settings kept, silent removal with `KEEP_CONFIG=0` (DeleteConfigFolder ran; every
  file went except two logs held open by a copy of the app running from another folder).
- 3 November 2026, clicked through on a Windows 10 laptop with the service and the app
  installed, settings backed up first: tests 3, 4 (with Back), 5 and 7 pass.
- Found then: removing the app while it is open or in the tray left it running with no tray
  icon and its program file gone, until ended by hand. Fixed: the app quits when Windows
  closes its tray (the tray's exit now quits the app); afterwards nothing was left running and
  the program folder was removed.
- Still open: such a removal shows "The setup was unable to automatically close all requested
  applications" (OK carries on and the removal completes). The verbose log has the app shut
  down in the user's session, and Restart Manager in session 0 ending with error 350 (restart
  required): the check runs while our own service is still running, before the package's
  StopServices; RmGetList lists only `ProxmoxBackupClientSVC`. See the roadmap.

## Tests

Before each test: install the MSI, start the app, add a PBS server and one or two Backup
Sets, so the folder holds `config.json` and `scheduled_jobs.json`.

### 1. Silent removal keeps the settings (passed)

`msiexec /x <msi> /qn`

- `C:\Program Files\ProxmoxBackupClient` gone, service `ProxmoxBackupClientSVC` gone,
  Start menu entry gone, Programs and Features entry gone.
- `C:\ProgramData\ProxmoxBackupClient` still there with `config.json` and `scheduled_jobs.json`.

### 2. Silent removal with KEEP_CONFIG=0 (passed)

`msiexec /x <msi> /qn KEEP_CONFIG=0 /l*v uninstall.log`

- Programs, service and entries gone as in test 1.
- `C:\ProgramData\ProxmoxBackupClient` gone (close the app first: files it holds open stay).
- `uninstall.log` shows `DeleteConfigFolder` running.

### 3. Programs and Features > Uninstall (passed)

- Expected: no keep or delete question; programs gone; settings kept.

### 4. Programs and Features > Change, Remove, keep (passed)

Programs and Features > Change (or Settings > Apps > Modify), Next, Remove; on the keep or
delete page leave "Keep configuration", Next, Remove.

- The page appears. Afterwards the settings are kept, as in test 1.
- Back on "Ready to remove" returns to the keep or delete page; Back there returns to
  "Change, repair, or remove".

### 5. Programs and Features > Change, Remove, delete (passed)

As test 4, but choose "Delete configuration".

- The settings folder is gone, as in test 2.

### 6. Upgrade (passed)

Install a newer build over the installed one.

- No keep or delete question. One Programs and Features entry, with the new version.
- Service running. Settings unchanged.

### 7. Cancel (passed)

Start test 4 and press Cancel on the keep or delete page.

- App, service and settings all still there.
- After "Are you sure?", Windows Installer's own "Setup Wizard was interrupted" page shows,
  saying nothing was changed; Finish closes it. That is the standard ending of a cancelled
  setup, also when installing.

## If something goes wrong

- Make a verbose log: `msiexec /x <msi> /l*v uninstall.log`, then search it for
  `DeleteConfigFolder` and `KEEP_CONFIG`.
- If the folder is not deleted with `KEEP_CONFIG=0`, check that the app or another
  program does not hold a file open in it (the action ignores errors).
