# Proxmox Backup Client: Windows installer (MSI)

This folder builds the Windows installer, `ProxmoxBackupClient-<version>-x64.msi`.
It uses WiX Toolset 3 (tested with 3.14).

For step-by-step build and test instructions see [BUILD_INSTRUCTIONS.md](BUILD_INSTRUCTIONS.md).
For silent installs on many computers see [DEPLOYMENT.md](DEPLOYMENT.md).

## Files

| File | What it is |
| ---- | ---------- |
| `wix/build-msi.ps1` | The build script. Use this one. |
| `wix/Product.wxs` | The default brand ("Proxmox Backup Client"): name, exe names, UpgradeCode and Programs and Features links. |
| `wix/ProductBody.wxi` | The installer itself, shared by every brand: files, service, shortcut, dialogs, uninstall actions. |
| `wix/NimbusBackup.wxs`, `wix/AcmeBackup.wxs` | Example brand files (placeholder URLs). Not built by `build-msi.ps1`. |
| `wix/License.rtf` | The licence page shown during install. |
| `icons/<ExeName>.ico` | The icon for each brand, picked by the brand's `ExeName`. |

## What the MSI installs

- `C:\Program Files\ProxmoxBackupClient\ProxmoxBackupClient.exe` (the app) and
  `ProxmoxBackupClientSVC.exe` (the background service). The folder can be changed
  on the "Destination Folder" page.
- The Windows service `ProxmoxBackupClientSVC`, display name "Proxmox Backup Client Service".
  It runs as LocalSystem, starts automatically at boot and is started at the end of the install.
  The app finds it and hands it backups and schedules ("Run as Service" shows as on).
- A Start menu entry "Proxmox Backup Client".
- One entry in Programs and Features, with this fork's links (about, help, updates, issues).
- When `AUTOSTART=1` (the default): a `Run` entry under HKCU for the account that ran
  the install, which starts the app minimised to the tray at sign-in (`--minimized`).

The settings are not part of the MSI. The app and the service keep them in
`C:\ProgramData\ProxmoxBackupClient` (servers and tokens, Backup Sets, keys, reports, logs).
The service gives that folder its own permissions when it starts: SYSTEM and Administrators
full control, local administrator accounts read and create, nobody else.

## Upgrade

Installing a newer MSI replaces the installed one in place (MajorUpgrade, same UpgradeCode).
A rebuilt MSI of the same version also replaces it. The installer stops the service and
closes any running copy of the app first, then starts the new service. The settings in
ProgramData are not touched. Installing an older version over a newer one is refused.

## Removal and the settings

On removal the service is stopped and deleted, and the programs and the Start menu entry go.
`C:\ProgramData\ProxmoxBackupClient` is kept unless the property `KEEP_CONFIG` is `0`.

- Programs and Features > Change (Windows 11 Settings > Apps > Modify), then Remove, shows a
  page that asks "Keep configuration" or "Delete configuration". Keep is the default. Running
  the MSI again and choosing Remove does the same.
- Programs and Features > Uninstall runs without the installer's own pages, so the question
  is not asked and the settings are kept.
- Silent: `msiexec /x <msi> /qn` keeps the settings; add `KEEP_CONFIG=0` to delete them.

The silent paths have been tested; the pages have not been clicked through yet (see [../MSI_UNINSTALL_TEST.md](../MSI_UNINSTALL_TEST.md)).

"Delete configuration" removes what it can: files the app holds open (its logs, if a copy of
it is running from another folder) stay.

## Public properties

| Property | Default | Effect |
| -------- | ------- | ------ |
| `INSTALLFOLDER` | `C:\Program Files\ProxmoxBackupClient` | Where the two programs go. |
| `AUTOSTART` | `1` | `0` leaves out the sign-in Run entry. |
| `KEEP_CONFIG` | `1` | On removal, `0` deletes `C:\ProgramData\ProxmoxBackupClient`. |

There is no property that loads settings (no `CONFIGFILE` or similar). Servers and Backup
Sets are added in the app after the install.

## Branded builds

A brand is one `.wxs` file that sets `ProductName`, `ExeName`, `ServiceExeName`,
`VendorKey`, `Manufacturer`, `UpgradeCode` and the four Programs and Features links, then
includes `ProductBody.wxi`. The programs are installed under the brand's exe names, and
the app picks its brand from its own exe name at run time (`gui/brand.go`). Every brand
keeps its settings in `C:\ProgramData\ProxmoxBackupClient`, and the removal page and
`KEEP_CONFIG=0` act on that folder.

To make one: copy `Product.wxs`, give it a new UpgradeCode, add `icons/<ExeName>.ico`,
build the two programs under the brand's names, and run candle and light on the new file
the way `build-msi.ps1` does (it only builds `Product.wxs`).

Never change the UpgradeCode of an existing brand: an upgrade would then install beside the
old version instead of replacing it.

## Troubleshooting

- **"version '...' must be x.y.z"**: MSI versions are three numbers. Pass `-Version 0.9.0`
  for a `-rc1` or `dev-...` build.
- **"gui\build\bin\... is missing"**: build both programs first (see BUILD_INSTRUCTIONS.md).
- **candle.exe not found**: unpack WiX 3.14 to `C:\Dev\tools\wix314`, pass `-WixDir`, or
  put it on PATH.
- **The service does not start**: look at `C:\ProgramData\ProxmoxBackupClient\service-service.log`.
  Check that `ProxmoxBackupClientSVC.exe` came from the service build (`-tags service`), not
  a copy of the app.
- **Install log**: `msiexec /i <msi> /l*v install.log`.
