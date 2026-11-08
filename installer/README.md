# Salvanza for Proxmox Backup Server: Windows installer (MSI)

> ⚠️ **Trademark & affiliation notice.** This is an **independent community project**, not an official Proxmox product: it is **not affiliated with, endorsed by, or sponsored by Proxmox Server Solutions GmbH**. **Proxmox®** is a registered trademark of Proxmox Server Solutions GmbH in the EU, the U.S. and other countries, and it is used here **only** to describe compatibility. Following Proxmox's own trademark guidance (registered marks must not be used in product or app names, for example "XY App for Proxmox Virtual Environment" rather than "Proxmox XY App"), **the non-branded client has been renamed**: the product formerly called *Proxmox Backup Client* is now **Salvanza for Proxmox Backup Server**, a name that does not contain the Proxmox mark. See the [Proxmox media kit](https://www.proxmox.com/en/about/company-details/media-kit) for the exact rules.

This folder builds the Windows installers: one MSI per brand (Salvanza, Nimbus Backup,
Etitech Backup). It uses WiX Toolset 3 (tested with 3.14). The release workflow
(`.github/workflows/build-and-release.yml`) builds every brand's MSI;
`wix/build_msi_linux.sh` does the same in Docker with Wine, and `wix/build-msi.ps1` builds
the default brand on Windows, `Salvanza-<version>-x64.msi`.

For step-by-step build and test instructions see [BUILD_INSTRUCTIONS.md](BUILD_INSTRUCTIONS.md).
For silent installs on many computers see [DEPLOYMENT.md](DEPLOYMENT.md).

## Files

| File | What it is |
| ---- | ---------- |
| `wix/build-msi.ps1` | Builds the default brand's MSI on Windows. |
| `wix/build_msi_linux.sh` | Builds every brand's MSI in Docker (WiX under Wine). |
| `wix/Product.wxs` | The default brand, "Salvanza for Proxmox Backup Server": name, program names, folder, UpgradeCode and Programs and Features links. |
| `wix/ProductBody.wxi` | The installer itself, shared by every brand: files, service, command line, shortcut, dialogs, upgrade and uninstall actions. |
| `wix/NimbusBackup.wxs`, `wix/EtitechBackup.wxs` | The other brands. |
| `wix/cli-aliases/*.cmd` | The command line's earlier names (`proxmoxbackup-directory.cmd` and so on), installed beside `salvanza-cli.exe`. |
| `wix/License.rtf` | The licence page shown during install. |
| `icons/<ExeName>.ico` | The icon for each brand, picked by the brand's `ExeName` (`salvanza-gui.ico`, `nimbusbackup-gui.ico`, `etitechbackup-gui.ico`). |

## What the MSI installs

- `C:\Program Files\Salvanza\salvanza-gui.exe` (the app), `salvanza-svc.exe` (the
  background service) and `salvanza-cli.exe` (the command line, the same in every brand).
  The folder can be changed on the "Destination Folder" page.
- The install folder on the system `PATH`, so `salvanza-cli` runs from any new terminal,
  and beside it `.cmd` files under the command line's earlier names
  (`proxmoxbackup-directory`, `proxmoxbackup-machine`, `pbsdirectorybackup`,
  `pbsmachinebackup`, `directorybackup`), so scripts written for the earlier tools keep
  working.
- The Windows service `salvanza-svc`, display name "Salvanza for Proxmox Backup Server Service".
  It runs as LocalSystem, starts automatically at boot and is started at the end of the install.
  The app finds it and hands it backups and schedules ("Run as Service" shows as on).
- A Start menu entry "Salvanza for Proxmox Backup Server". The app asks for administrator
  rights itself when it connects to the service, and offers Restart as administrator (the
  app's `-admin` switch, a UAC prompt) where an operation needs the app elevated. An upgrade
  removes the "(Administrator)" shortcuts earlier builds installed.
- When `DESKTOPSHORTCUTS=1` (the default): the same shortcut on the all-users desktop.
- One entry in Programs and Features, with the brand's links (about: https://salvanza.org/,
  help: https://github.com/salvanza/salvanza_client, updates: its releases page, contact:
  its issues page).
- When `AUTOSTART=1`: a `Run` entry under HKLM, which starts the app minimised to the tray
  when any user signs in (`--minimized`). It is off by default: the "Start at sign-in" page,
  after "Destination Folder", has the tick box **Start Salvanza for Proxmox Backup Server
  when a user signs in**, unticked unless the installed version had it on. The app does not
  ask for administrator rights at sign-in; it asks the first time its window is opened
  (below). The choice is kept in `HKLM\Software\<VendorKey>\<ExeName>`
  (`HKLM\Software\Salvanza\salvanza-gui`), value `AutoStart`, so an upgrade or a repair
  keeps it unless `AUTOSTART` is given on the command line. An upgrade from Proxmox Backup
  Client, or from a version without that value, keeps the sign-in start the earlier install
  had (its remembered choice, or its `Run` entry).

The settings are not part of the MSI. The service keeps them in
`C:\ProgramData\Salvanza` (servers and tokens, Backup Sets, keys, reports, its logs). It
gives that folder its own permissions when it starts: SYSTEM and Administrators full
control, nobody else, so only the service and a program started as administrator can open
it. Earlier builds also let each local administrator account read the folder without
elevation; the service removes those entries.

The app reaches the settings through the service's local API. It gets the service's token
by asking for administrator rights once (UAC; a standard user gives an administrator's name
and password), keeps it in memory, and asks again when the service replaces it (every 24
hours). Until it is connected the app shows no settings and a **Connect** button. Only a
small helper is elevated for that; the app itself stays a normal program. An app already
running as administrator connects without a prompt. Once its window has been in the tray
for 15 minutes, the app lets go of the token and asks again when the window is next
shown; meanwhile the tray says how many backups run and whether one failed. The app's own
logs are in each user's `%LOCALAPPDATA%\Salvanza\Logs`.

## Upgrade

Installing a newer MSI replaces the installed one in place (MajorUpgrade, same UpgradeCode).
A rebuilt MSI of the same version also replaces it. The installer stops the service and
closes any running copy of the app first, then starts the new service. The settings in
ProgramData are not touched. Installing an older version over a newer one is refused.

### Upgrading from Proxmox Backup Client, and other brands

Every brand's MSI also removes, the same way, any install of the other products of this
project: Proxmox Backup Client (the name before the rename, including the earlier fork's
0.9 and 0.10 MSIs), and the other brands. So installing Salvanza over Proxmox Backup Client,
or over Nimbus Backup, replaces it; two never sit side by side. Their removal keeps their
settings (it is part of an upgrade), and the installer closes their apps (`ProxmoxBackupClient.exe`,
`nimbusbackup-gui.exe` and so on) first.

When the new service first starts, it:

- stops and removes the services of earlier installs (`ProxmoxBackupClientSVC`,
  `ProxmoxBackupClient`, `NimbusBackupSVC`, `EtitechBackupSVC`) and the other brands'
  services, also copies installed without an MSI;
- if `C:\ProgramData\Salvanza` has no settings yet, moves the newest of
  `C:\ProgramData\ProxmoxBackupClient` and the other brands' folders there (or copies it,
  if it cannot be moved), and keeps a copy of the settings files in
  `C:\ProgramData\Salvanza\pre-salvanza-<date>`.

The app does the same for each user's own folders the first time that user starts it:
`%APPDATA%\ProxmoxBackupClient`, `%LOCALAPPDATA%\ProxmoxBackupClient` and
`%USERPROFILE%\.proxmox-backup-guardian` become `%APPDATA%\Salvanza`,
`%LOCALAPPDATA%\Salvanza` and `%USERPROFILE%\.salvanza`.

An option to keep another brand and install beside it is on the [roadmap](../TODO.md).

## Removal and the settings

On removal the service is stopped and deleted, and the programs and the Start menu entry go.
`C:\ProgramData\Salvanza` is kept unless the property `KEEP_CONFIG` is `0`.

- Programs and Features > Change (Windows 11 Settings > Apps > Modify), then Remove, shows a
  page that asks "Keep configuration" or "Delete configuration". Keep is the default. Running
  the MSI again and choosing Remove does the same.
- Programs and Features > Uninstall runs without the installer's own pages, so the question
  is not asked and the settings are kept.
- Silent: `msiexec /x <msi> /qn` keeps the settings; add `KEEP_CONFIG=0` to delete them.

The silent paths and the pages have been tested on builds from before the rename (see
[../MSI_UNINSTALL_TEST.md](../MSI_UNINSTALL_TEST.md)).

"Delete configuration" removes what it can: files the app holds open (its logs, if a copy of
it is running from another folder) stay.

## Public properties

| Property | Default | Effect |
| -------- | ------- | ------ |
| `INSTALLFOLDER` | `C:\Program Files\Salvanza` | Where the programs go. |
| `SERVICEAUTOSTART` | `1` | `0` installs the service as Manual and leaves it stopped. |
| `AUTOSTART` | `0` | `1` adds the sign-in Run entry (the "Start at sign-in" tick box). Not given, an upgrade or repair keeps the installed version's choice. |
| `DESKTOPSHORTCUTS` | `1` | `0` leaves out the desktop shortcut (the Start menu entry stays). |
| `CONFIGFILE` | (none) | Full path of a settings file saved with Preferences > Export, applied before the service starts. See [DEPLOYMENT.md](DEPLOYMENT.md). |
| `KEEP_CONFIG` | `1` | On removal, `0` deletes `C:\ProgramData\Salvanza` (the brand's folder). |

Silent install needs only msiexec's own `/quiet`, for example
`msiexec /i Salvanza-<version>-x64.msi /quiet SERVICEAUTOSTART=0`.
Exit codes: `0` success, `3010` success with a restart needed.

## Branded builds

A brand is one `.wxs` file that sets `ProductName`, `ExeName`, `ServiceExeName`, `Folder`,
`VendorKey`, `Manufacturer`, `UpgradeCode` and the four Programs and Features links, then
includes `ProductBody.wxi`. The programs are installed as `<FileBase>-gui.exe` and
`<FileBase>-svc.exe`, and the app picks its brand from its own program name at run time
(`gui/brand.go`), so `ExeName`, `ServiceExeName` and `Folder` must match the brand's
`FileBase` and `Folder` there:

| Brand | Programs | Service | Folders |
| ----- | -------- | ------- | ------- |
| Salvanza | `salvanza-gui.exe`, `salvanza-svc.exe` | `salvanza-svc` | `C:\Program Files\Salvanza`, `C:\ProgramData\Salvanza` |
| Nimbus Backup | `nimbusbackup-gui.exe`, `nimbusbackup-svc.exe` | `nimbusbackup-svc` | `C:\Program Files\NimbusBackup`, `C:\ProgramData\NimbusBackup` |
| Etitech Backup | `etitechbackup-gui.exe`, `etitechbackup-svc.exe` | `etitechbackup-svc` | `C:\Program Files\EtitechBackup`, `C:\ProgramData\EtitechBackup` |

Every brand installs the same `salvanza-cli.exe`. The removal page and `KEEP_CONFIG=0`
act on the brand's own settings folder.

To make one: add the brand to `brandCatalog` in `gui/brand.go`, copy `Product.wxs`, give it
a new UpgradeCode, add its UpgradeCode to the list of products every installer replaces in
`ProductBody.wxi`, add `icons/<ExeName>.ico`, build the programs under the brand's names,
and run candle and light on the new file the way `build-msi.ps1` does (it only builds
`Product.wxs`).

Never change the UpgradeCode of an existing brand: an upgrade would then install beside the
old version instead of replacing it.

## Troubleshooting

- **"version '...' must be x.y.z"**: MSI versions are three numbers. Pass `-Version 2.0.0`
  for a `-rc1` or `dev-...` build.
- **"gui\build\bin\... is missing"**: build the three programs first (see BUILD_INSTRUCTIONS.md).
- **candle.exe not found**: unpack WiX 3.14 to `C:\Dev\tools\wix314`, pass `-WixDir`, or
  put it on PATH.
- **The service does not start**: look at `C:\ProgramData\Salvanza\service-service.log`.
  Check that `salvanza-svc.exe` came from the service build (`-tags service`), not a copy
  of the app.
- **Install log**: `msiexec /i <msi> /l*v install.log`.
