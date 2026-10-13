# Proxmox Backup Client: Windows and Linux client for Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client is an open-source (GPL-3.0) backup client for Proxmox Backup Server (PBS), working on Windows and Linux.**

It is a **suite of tools** for backing up to PBS:

- **Proxmox Backup Client GUI** (based on the Nimbus Backup GUI from RDEM Systems): a graphical client for backing up Windows and Linux servers and workstations to PBS, with consistent snapshots (VSS on Windows, elastio-snap/dattobd on Linux), Backup Sets with schedules, folder and whole-disk backups, restore, Undelete and Roll back, several PBS servers, and a background service on Windows and Linux.
- **`proxmoxbackup-directory`**: command-line tool for folder (pxar) backups with deduplication and change detection.
- **`proxmoxbackup-machine`**: command-line tool for live whole-disk backups (fidx) of Windows (VSS) and Linux (elastio-snap/dattobd) machines; the image is bootable after a bare-metal restore.
- **`proxmoxbackup-nbd`**: attaches a disk backup as an NBD device for restoring (Linux).
- A **bare-metal restore ISO** built on Clonezilla Live.

> Keywords: proxmox backup client windows · PBS client · Windows VSS backup · immutable offsite backups · Proxmox Backup Server interface.

## 📖 User manuals

| Manual | Covers |
|---|---|
| **[GUI manual](docs/manual/GUI.md)** | Install, PBS servers, Backup Sets and schedules, incremental backups, progress, restore, Undelete, Roll back, Reports, logs, Preferences, the background service, troubleshooting |
| **[Command line manual](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: every flag, config files, exit codes, scheduling; Undelete and Roll back from the command line |
| **[Bare-metal restore manual](docs/manual/BMR.md)** | The restore ISO, step by step; `vm` snapshots onto Proxmox VE; different hardware |
| [Restore scenarios](docs/RESTORE_GUIDE.md) | Which restore to use when |
| [Manuals index](docs/manual/README.md) | An overview of the three manuals |

## 🆕 On master, for the next release

These are in the [CHANGELOG](CHANGELOG.md) under [Unreleased] and not yet in a release:

- **Installers**: a **Windows MSI** (app and background service under Program Files, Start menu entry, in-place upgrades, keep or delete the settings on removal) and **Linux packages**, `.deb` (Debian, Ubuntu, Mint) and `.rpm` (Fedora), with the app, the three command-line tools, the manuals and menu entries.
- **A background service on Linux** (`proxmoxbackupclient.service`): scheduled Backup Sets run as root, also when nobody is logged in. Turn it on in Preferences > Advanced > Run as Service, as on Windows. **Restart as administrator** now works on Linux too (through pkexec).
- **Pause, Resume and Stop for backups the service runs**, with their progress card in the open app.
- **A warning when two Backup Sets share a backup group** (same server, type and Backup ID), with a **SHARED ID** badge on both cards.
- **Settings locked down**: on Windows `C:\ProgramData\ProxmoxBackupClient` is now open to administrators only; on Linux the settings folder is private to its owner.
- **Files back from machine backups of every common file system**: FAT, exFAT, ext2/3/4 and XFS as well as NTFS. Windows image restores also put back permissions and alternate data streams.
- **Linux machine backups skip free space** (ext2/3/4, XFS), as Windows ones do since v0.8.0.
- **Undelete and Roll back for Linux machine Backup Sets**, and **from the command line** for any Backup Set: `-list-sets`, `-undelete`, `-rollback` (section 12 of the [command line manual](docs/manual/CLI.md)).
- The command-line tools print errors to stderr instead of showing message boxes, and `proxmoxbackup-machine` sends its notification e-mail.

## 💽 New in v0.8.0: faster machine backups, files back from them, Pause/Resume

- **Machine backups skip free space** (and the paging and hibernation files) on Windows, so a half-empty disk is read in about half the time, and a **bad sector** no longer stops the backup: it goes into the image as zeros and the run says where.
- The machine backup progress card shows blocks of total, the disk and partition, New / Reused / blank chunks and the upload rate.
- **Restore files from a machine backup**: each NTFS volume in the image is a folder you can browse, search and tick. Nothing is mounted or downloaded first; only the file table and then the chosen files' data are read (a 127 GB laptop C: with 430,000 files lists in 4 to 15 seconds). So one machine backup per laptop covers both a bare-metal restore and "I need that file back".
- **Undelete and Roll back** work on machine Backup Sets (Windows), on the folders you choose for the set (by default Desktop, Documents, Pictures, Music, Videos and Downloads).
- **Pause and Resume** a running folder or machine backup, a **VSS / Snapshot ON-OFF badge** on each set card, and Windows **creation times** backed up and restored.
- **v0.8.2** (tagged, not published as a release; the change ships with the next one): blank disk space is no longer hashed block by block, so a mostly empty disk goes through in about a third of the time (a 50 GB test disk: 3m36s, now 1m15s).

See the [CHANGELOG](CHANGELOG.md).

## 🗑️ New in v0.7.0: Undelete and Roll back

Two new pages in the GUI, for folder Backup Sets:

- **Undelete** finds the files that are in a set's backups but no longer on disk, in the newest backup or every backup of the last 7 or 30 days, shows them as a folder tree with when each went missing (and "probably moved to ..." when the same file is elsewhere), and restores the ticked ones where they were or to another folder. After deleting 39 GB from a 761,000-file share, a 7-day search found exactly those files in about a minute.
- **Roll back** puts a set's folders back as they were at a chosen backup. It previews what would change (missing, changed, added since), then restores. Every file it replaces or removes is first moved aside on the same drive, so **Undo** puts everything back.

Only the backups' file lists and the folders' listings are read; no file is opened until it is restored. Also new: in-app confirmation dialogs, a Next run time on each Backup Set card, a suggested Backup ID per set (`deepthought-data`), unique set names, and the background service included in the Windows download (Preferences > Advanced > Run as Service). See the [GUI manual](docs/manual/GUI.md#10-undelete) and the [CHANGELOG](CHANGELOG.md).

## ⚡ New in v0.6.0: incremental folder backups

Folder backups can now skip files that have not changed, using the same **change detection** as the official `proxmox-backup-client`. In **Metadata** mode the client compares each file's size, modification time (to the nanosecond), mode and owner with the previous snapshot. Unchanged files are not even opened: their data is reused from the previous snapshot's chunks. Only new and changed files are read and uploaded.

Measured on a real file server (480.9 GB, 761,866 files on USB disks):

| Run | Time | Read | New chunks uploaded |
|---|---|---|---|
| First Metadata run (nothing to compare with) | 4 h 17 min | every file | 135,549 |
| Following runs | about 10 to 15 min (quicker when the last run was minutes ago) | only changed files; all 761,866 unchanged files reused | 4 to 40 |

Because unchanged data is neither read nor sent, it also cuts traffic to a remote or offsite PBS to little more than what actually changed.

To use it: edit a folder **Backup Set**, **Destination** tab, **Change detection: Metadata**. Set **Read every file every N runs** so that every Nth run is a full read, which catches the rare change that keeps a file's size and time (for example 42 for a set that runs every 4 hours, roughly weekly). Give each set its **own Backup ID**: Metadata mode compares with the newest snapshot in the group, so two sets sharing an ID make each other read everything. Command line: `-change-detection-mode metadata`. Legacy stays the default; details in [Faster folder backups: change detection](#faster-folder-backups-change-detection) and the [GUI manual](docs/manual/GUI.md#7-incremental-folder-backups-change-detection).

## 🤝 Compatible with the official Proxmox client, in both directions

Verified with `proxmox-backup-client` 3.4.9 and PBS 4.2:

- **Legacy, Data and Metadata snapshots made by this client restore byte-identically with the official client**, from both the GUI and the command line, made on Windows or Linux, and PBS 4.2's file browser opens them. On Linux the restored files also keep their exact mode, nanosecond modification time and symlink targets.
- **Legacy, Data and Metadata snapshots made by the official client restore byte-identically with this client**, fully or selectively.
- Same split archive format (pxar version 2), same change-detection modes, same encryption key file format (`proxmox-backup-client key create` keys work here, and keys generated here work there), same `PBS_REPOSITORY` style environment variables.

So your backups are never tied to this client: Proxmox's own tools restore every snapshot it makes, and it restores every snapshot the official client makes. The only extra is one small file this client adds at the root of a folder snapshot, `.proxmox_backup_client_meta.json` (the original path), which the official client restores as an ordinary file.

> ⚠️ **Disclaimer:** This project is **not affiliated in any way** with **Proxmox Server Solutions GmbH**. "Proxmox", the Proxmox logo and related names are the property of their respective owners; here they are used **only** to state compatibility. See [proxmox.com](https://www.proxmox.com/) for their products.

## 🔧 About this fork

This is **mjb-is's fork** of the upstream project. Repo: **https://github.com/mjb-is/proxmoxbackupclient_go**. Focus areas: creating functionality and flow that would allow PBS to be the single server-based backup system in my homelab, which currently uses a mix of Proxmox Backup Server with Proxmox VE VM and Linux host backups, Backup for Workgroups for Windows servers file-based and bare-metal recovery, FastFileSync/Rsync incremental file mirrors, and CloneZilla/RescueZilla. Hardening the Windows/Linux GUI client and building a fully automated bare-metal restore path on top of Clonezilla. Highlights over upstream:

- **Rebuilt GUI**: a native File/View/Tools/Help menu bar, a left side menu (Backup, Restore, Undelete, Roll back, Reports, Message Log, Preferences, About), "Backup Sets" with daily, interval or manual triggers, a type badge and a one-click "Run Now", a Reports page with full backup history, a capped Message Log, and a Preferences dialog with five colour themes (Amber, Blue, Green, Red, Dark) plus a custom colour option. Eighteen interface languages (French, English, Italian, German, Polish, Spanish, Bulgarian, Czech, Greek, Hungarian, Latvian, Lithuanian, Dutch, Portuguese, Romanian, Slovak, Turkish and Ukrainian), with no hardcoded language left, so the whole application displays in the selected language.
- **A real PBS/NBD chunk-fetch reliability fix**: regular hangs on full machine backup restore (4 fails to 1 success typical) came from a `zstd.NewReader` call in the reader path started with a live `io.Reader` instead of `nil`, silently spinning up an unused background streaming-decode goroutine that could deadlock a bare-metal restore mid-transfer. Fixed upstream too, merged as [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85).
- **A fully automated "PBS Bare Metal Restore" wizard** for the Clonezilla live ISO: see [below](#clonezilla-live-iso-bare-metal-restore). Test restore of a 32GiB Windows machine from power-on into the boot ISO, through to a restored Windows login, in just 8 minutes; similarly, a 40GiB Linux Mint restore in 10 minutes to the login prompt. Both tested on isolated VMs, still to be tested on bare metal.
- **Restore fidelity and feedback**: NTFS ACLs and DOS attributes are re-applied on Windows, and POSIX ACLs and extended attributes on Linux (both verified live). Folder timestamps are restored as well as file ones. Linux folder backups also record real modes, owners and symlinks, and a root restore re-applies them. An optional "Verify after restore" pass re-reads what was written and compares it with the snapshot. Selective restore uses the archive's own index instead of scanning it, the snapshot tree loads one folder at a time, and the restore progress bar tracks bytes actually written, with a stage label under it and the run recorded in Reports.
- **Backup Set workflow**: clone a set, a Backup Set's name is written to PBS as the snapshot comment (one-off backups get an optional comment field), email notifications with on-failure options, post-backup actions (shut down, minimise to the tray or exit the app, run an application), settings export and import between machines, a queue that names what is waiting behind the active job, and Stop, Pause and Resume on the progress card.
- **Backup behaviour**: an optional multi-threaded read-ahead for backups (experimental), the file currently being archived is shown during directory backups, network shares are no longer snapshotted with VSS, and mapped drives are visible in the folder picker even when the app is elevated. A tray tooltip and Windows toast notifications report backup and restore activity.
- **Machine backup as a Proxmox VE "vm" snapshot.** The one-off backup form and Backup Sets can store a machine backup as `vm/<id>` so Proxmox VE can list and restore it (pick a target Storage in the restore dialog; a restored Windows UEFI/GPT disk is verified to boot). The ID is numeric, with an offset into a reserved range by default so it cannot clash with a real VM. The generated VM config is minimal (SATA disk, OVMF for GPT disks, no TPM or Secure Boot keys, no drivers), so expect a bare-bones VM. This also works as a physical-to-virtual conversion: a `vm` snapshot restores onto Proxmox VE and boots (verified with a Windows Server UEFI disk), and the same snapshot can still be restored onto bare metal with the Clonezilla bare-metal restore, which lists `vm` snapshots alongside `host` ones (verified over PXE onto a blank VM). A Backup Set stores its numeric ID, so every scheduled run lands in the same `vm/<id>` group; a set with a non-numeric ID is refused at save time. Host backup remains the default. Restoring these snapshots onto Proxmox VE and onto bare metal has been tested repeatedly with no failures to date.

Anything of general use gets sent upstream as a PR rather than kept fork-only; day-to-day fork-specific work stays here. Seventeen pull requests from this fork have been merged into the original project so far, and one ([#105](https://github.com/tizbac/proxmoxbackupclient_go/pull/105), file and folder times on restore) is open:

  - [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85): Fix zstd decoder goroutine deadlock in `GetChunkData` (hung full-machine restores)
  - [PR #86](https://github.com/tizbac/proxmoxbackupclient_go/pull/86): Fully automated PBS Bare Metal Restore boot entry for the Clonezilla ISO
  - [PR #87](https://github.com/tizbac/proxmoxbackupclient_go/pull/87): Linux snapshots: drive elastio-snap/dattobd through ioctl instead of `elioctl`/`dbdctl`
  - [PR #88](https://github.com/tizbac/proxmoxbackupclient_go/pull/88): Restore: re-apply captured NTFS ACLs (Windows) and POSIX ACLs and xattrs (Linux)
  - [PR #90](https://github.com/tizbac/proxmoxbackupclient_go/pull/90): `machinebackuplib`: make `vm` backups restorable and bootable on Proxmox VE
  - [PR #91](https://github.com/tizbac/proxmoxbackupclient_go/pull/91): GUI: back machines up as `host`, not `vm`, by default
  - [PR #92](https://github.com/tizbac/proxmoxbackupclient_go/pull/92): `patch-clonezilla`: skip patches whose change is already in the base ISO
  - [PR #93](https://github.com/tizbac/proxmoxbackupclient_go/pull/93): `patch-clonezilla`: add the PBS bare-metal restore entry to `isolinux.cfg`
  - [PR #94](https://github.com/tizbac/proxmoxbackupclient_go/pull/94): Restore the data between the MBR and the first partition in the BMR flow (BIOS/MBR machines boot after restore)
  - [PR #95](https://github.com/tizbac/proxmoxbackupclient_go/pull/95): Fix reading passphrase-protected key files made by `proxmox-backup-client`
  - [PR #96](https://github.com/tizbac/proxmoxbackupclient_go/pull/96): `directorybackup`: back up several directories in one run
  - [PR #97](https://github.com/tizbac/proxmoxbackupclient_go/pull/97): Docs: point elastio-snap at the maintained Axcient fork
  - [PR #98](https://github.com/tizbac/proxmoxbackupclient_go/pull/98): `pbscommon`: skip the key file permission check on Windows
  - [PR #99](https://github.com/tizbac/proxmoxbackupclient_go/pull/99): Restore encrypted backups from the Clonezilla BMR ISO
  - [PR #102](https://github.com/tizbac/proxmoxbackupclient_go/pull/102): `snapshot`: delete our VSS shadow copies after every backup, through the VSS API
  - [PR #103](https://github.com/tizbac/proxmoxbackupclient_go/pull/103): `machinebackup`: skip hashing blank (all-zero) chunks
  - [PR #104](https://github.com/tizbac/proxmoxbackupclient_go/pull/104): CLIs: report errors on stderr, never in a message box

The original author may choose to merge anything else from this repo as they see fit under GPL.

## 📦 Download and install

👉 **[Download the latest release](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| You have | Use |
|---|---|
| Windows | The **MSI installer** (`ProxmoxBackupClient-<version>-x64.msi`) installs the app and the background service under Program Files, with a Start menu entry. Or unzip `ProxmoxBackupClient-v<version>-windows-amd64.zip` (the app and `ProxmoxBackupClientSVC.exe`; keep them in the same folder) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Another Linux | Unpack `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (needs the WebKitGTK runtime) |
| Command line only | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` or `-linux-amd64.tar.gz` |

The MSI, `.deb` and `.rpm` are new on master: releases up to v0.8.1 have only the zip and tar.gz files. Until a release carries the installers, build them as described in [BUILD.md](BUILD.md).

The Linux packages install two menu entries: **Proxmox Backup Client** runs the app as you (folder backups of your own files), and **Proxmox Backup Client (administrator)** runs it as root, asking for your password through polkit, for machine backups and block snapshots. The background service is installed but not turned on; see [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **Windows shows "virus detected" (e.g. `Trojan:Win32/Sabsik.FL.A!ml`) or a SmartScreen warning?**
> This is a **known false positive** for Go/Wails applications: it is *not* a virus. The `!ml` suffix indicates a machine-learning model detection that flags *unsigned and uncommon* executables. Upstream has more detail: [why this happens and how to verify their download](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Verifying a download

Every release from this fork includes a `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-v0.8.1-windows-amd64.zip -Algorithm SHA256   # compare with SHA256SUMS.txt
```

On Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`.

> ℹ️ **No code signing, CI-signed attestation, or VirusTotal scan yet** for this fork's own builds (upstream's own releases have some of these; see their repo). These are built and uploaded manually, not yet through a CI pipeline that could produce a verifiable build-provenance attestation. See [TODO.md](TODO.md) if you'd like to help set that up.

## ✨ Features

### GUI: Proxmox Backup Client GUI (recommended)
- **🌍 Multilingual**: 18 interface languages: French, English, Italian, German, Polish, Spanish, Bulgarian, Czech, Greek, Hungarian, Latvian, Lithuanian, Dutch, Portuguese, Romanian, Slovak, Turkish and Ukrainian
- Native menu bar and side menu (Backup, Restore, Undelete, Roll back, Reports, Message Log, Preferences, About)
- **Backup Sets**: reusable named jobs with daily / interval / manual scheduling, a directory-vs-machine type badge, a VSS / Snapshot badge, "Run Now" (quick or full read for Metadata sets), and a **SHARED ID** warning when two sets would write into one backup group
- **One-off backups** of folders or whole disks
- **Reports** page with full backup and restore history, and a capped **Message Log** for diagnostics
- **Preferences** dialog with five colour themes (Amber, Blue, Green, Red, Dark) plus a custom colour option
- **Incremental folder backups** (change detection, Metadata mode): unchanged files are reused without being read, as in the official client, with an optional full read every N runs
- **Machine (whole-disk) backups** on Windows and Linux that skip free space and survive bad sectors, stored as `host` or as a Proxmox VE `vm` snapshot
- **Restore**: snapshot browsing, file search (wildcards, regular expressions, full paths) and selective restore, with NTFS/POSIX ACL restore, folder timestamps, Windows creation times and an optional verify pass
- **Files back from machine backups**: browse and restore files from NTFS volumes in a disk image (FAT, exFAT, ext2/3/4 and XFS on master)
- **Undelete** and **Roll back** (from v0.7.0): find files a Backup Set's backups still have but the disk does not, and put them back; or put a set's folders back as they were at a chosen backup, with a preview first and Undo afterwards. Folder and machine sets. See the [GUI manual](docs/manual/GUI.md#10-undelete)
- **Pause, Resume and Stop** a running backup
- **Run as Service**: install, start, stop or remove the background service from Preferences > Advanced, so schedules run without anyone signed in (Windows from v0.7.0; Linux on master)
- Live progress card: start time and forecast finish, files done of total, processing speed (data and files per second) and actual upload speed, current file, and why a run is reading every file
- VSS (Volume Shadow Copy) on Windows, elastio-snap/dattobd block snapshots on Linux
- Email notifications and post-backup actions, Backup Set cloning, settings export and import
- Several PBS servers, with certificate fingerprint pinning (trust on first use), API token or user/password login
- **Client-side encryption**: per-server PBS key file (AES-256-GCM, manifest signed), optional passphrase protection, see [Client-side encryption](#-client-side-encryption)
- Administrator protection for changing Backup Sets (Windows), with Restart as administrator
- Debug logging for diagnostics

### CLI tools
- `proxmoxbackup-directory`: folder (pxar) backups with deduplication and `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: live whole-disk backups of Windows (VSS) and Linux (elastio-snap/dattobd) machines (fidx, incremental, free space skipped unless `-readfreespace`)
- `proxmoxbackup-nbd`: NBD server for restoring disk backups (Linux)
- `ProxmoxBackupClientSVC` (Windows) and `proxmoxbackupclient` (Linux packages): `-list-sets`, `-undelete` and `-rollback` for Backup Sets defined in the GUI (on master)

The first three accept `-keyfile` and `-keyfile-passphrase` for encrypted snapshots. Full reference: [command line manual](docs/manual/CLI.md).

### 📸 Screenshots

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets**: named jobs with a repeat/manual trigger badge and a directory/machine type badge, Run Now on each

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Live backup progress**: throughput, ETA, and new-vs-reused chunk counts as it runs

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restore**: pick a PBS server and backup ID, list snapshots or search across them by filename

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Backup Sets as Proxmox VE VMs**: a set stored as a `vm` snapshot carries a `MACHINE (VM)` badge

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Backup as**: choose the Proxmox VE virtual machine type and a numeric ID; the PBS backup ID is shown as you type

</td>
<td width="33%">

[<img src="docs/screenshots/oneoff-as-vm.png" width="280">](docs/screenshots/oneoff-as-vm.png)
**One-off backups**: the same option on the one-off form
</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**Selective restore**: tick individual files/folders from the snapshot tree, restore in place or elsewhere

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Reports**: full run history with a detail panel (duration, chunks, folders backed up)

</td>
<td width="33%">

[<img src="docs/screenshots/gui-message-log.png" width="280">](docs/screenshots/gui-message-log.png)
**Message Log**: a capped, timestamped diagnostic feed across every backup and restore

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**Multi-PBS**: manage several PBS servers, test connectivity, pick a default

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-edit-server.png" width="280">](docs/screenshots/gui-preferences-edit-server.png)
**Add/edit a server**: URL, user/password or API token, datastore and namespace

</td>
<td width="33%">

[<img src="docs/screenshots/gui-theme-amber.png" width="280">](docs/screenshots/gui-theme-amber.png)
**Themes**: five built-in colour palettes plus a custom colour, recolouring the whole app live (see the strip below)

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-advanced.png" width="280">](docs/screenshots/gui-preferences-advanced.png)
**Backup options**: explained honestly: parallel restore extraction is experimental and measured *slower* on our own test hardware

</td>
</tr>
</table>

**🎨 Colour themes**: the Preferences dialog itself recolours with each pick, not just a swatch:

<table>
<tr>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-amber.png" width="180">](docs/screenshots/gui-theme-amber.png)
Amber

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-blue.png" width="180">](docs/screenshots/gui-theme-blue.png)
Classic Blue

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-green.png" width="180">](docs/screenshots/gui-theme-green.png)
Green

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-red.png" width="180">](docs/screenshots/gui-theme-red.png)
Red

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-dark.png" width="180">](docs/screenshots/gui-theme-dark.png)
Dark

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-custom.png" width="180">](docs/screenshots/gui-theme-custom.png)
Custom

</td>
</tr>
</table>

**🌍 Multilingual**: the Reports page in French, Italian and Spanish (and 15 more, including English, German and Polish):

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-lang-french.png" width="280">](docs/screenshots/gui-lang-french.png)
Français

</td>
<td width="33%">

[<img src="docs/screenshots/gui-lang-italian.png" width="280">](docs/screenshots/gui-lang-italian.png)
Italiano

</td>
<td width="33%">

[<img src="docs/screenshots/gui-lang-spanish.png" width="280">](docs/screenshots/gui-lang-spanish.png)
Español

</td>
</tr>
</table>

### Smart system exclusions (folder backups)
Folder backups leave out these on their own:

**System folders:** `System Volume Information` (VSS storage, can reach 100+ GB) and `$RECYCLE.BIN` wherever they are; `Recovery` only at the root of a drive (on master; earlier versions skipped a folder of that name at any depth); `.pbs-rollback` (Roll back's safety folder).
**System files:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` and `DumpStack.log.tmp`.

**Why this matters:** a drive may show 1.03 TB used while real files are ~141 GB. Without exclusions the backup would include VSS snapshots (wasted space and time); with them, the size matches the real data.

**Recommendation:** use a **folder** backup with these exclusions for file-level backups; use a **machine** backup in a separate Backup Set for bare-metal restore (it is a whole-disk image; free space and the paging and hibernation files go in as zeros).

### Security & quality
- Optional client-side encryption of every chunk, with a signed manifest
- Settings folder open to administrators only on Windows, private to its owner on Linux (on master)
- The local service API listens on 127.0.0.1 only and needs a token, renewed every time the service starts
- Input validation and credential sanitization
- Path traversal prevention
- Retry logic with exponential backoff
- Error handling and unit tests in every module, plus opt-in live tests and end-to-end tests against a real PBS

## 🔐 Client-side encryption

Snapshots can be encrypted before they leave the machine, using a Proxmox Backup Server key file (the format `proxmox-backup-client key create` writes, so keys are interchangeable with the official client). Every chunk is encrypted with AES-256-GCM and the manifest is signed. The PBS server never sees the key.

**GUI:** open **Preferences > Account Information**, edit a server and use its **Encryption** tab. Browse to an existing key file or **Generate...** a new one (the Advanced option protects it with a passphrase). The tab shows the key fingerprint. Reports show whether each backup was encrypted. **Without the key file a snapshot cannot be restored: keep a copy somewhere safe, away from this machine.**

**Passphrase-protected keys (scrypt or PBKDF2):** choose how the app unlocks the key:
- **Ask me**: the passphrase is asked once per app session and kept in memory only. Scheduled and startup backups never prompt: they fail fast, with a clear message, until the passphrase has been entered in that session. A backup run through the background service also fails fast for an Ask key.
- **Remember on this computer**: the passphrase is stored in the config like the PBS token, so anyone who can read `config.json` can unlock the key. This is the choice for unattended scheduled jobs and for the background service.

Restore asks for the passphrase when you select an encrypted snapshot.

**CLI:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` is only needed for a protected key and is prompted for when omitted. `proxmoxbackup-machine` and `proxmoxbackup-nbd` take the same two flags; `proxmoxbackup-nbd` needs them to attach an encrypted disk image and checks the first chunk before touching the NBD device, so a wrong key fails immediately.

**Bare-metal restore:** the Clonezilla ISO asks for the key (none, a key file on a USB stick, or a typed path) and for the passphrase of a protected key. The key is never written to the ISO. See [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md).

**Known limitations**
- A key generated by the GUI is written with a protected ACL (current user, SYSTEM and Administrators only). Key files you keep elsewhere keep whatever permissions their folder gives them.
- `config.json` holds the PBS tokens, passwords and any remembered passphrase in plain text. On master its folder is open to administrators only (Windows) or to its owner only (Linux); up to v0.8.2 it was readable by local Users on Windows, so restrict it yourself on a shared machine.
- Restoring an encrypted snapshot needs the same key file; there is no recovery if it is lost.
- The Ask-key flow and the service are separate processes: use Remember for anything that runs through the service.

## Faster folder backups: change detection

In the default Legacy mode every folder backup reads every file, so a run takes as long as reading the whole folder even when nothing changed. Backup Sets (and the CLI) can use the change-detection modes of the official `proxmox-backup-client` instead. Metadata mode is in daily use on a 480.9 GB, 761,866-file server, where a run with few changes takes 10 to 15 minutes instead of over 4 hours (see [New in v0.6.0](#-new-in-v060-incremental-folder-backups)).

| Mode | Reads | Stored as |
|---|---|---|
| **Legacy** (default) | every file | one `<name>.pxar.didx` per folder, as before |
| **Data** | every file | a split archive: `<name>.mpxar.didx` (metadata) + `<name>.ppxar.didx` (file contents) |
| **Metadata** | only new and changed files | the same split archive; files whose size and modification time (to the nanosecond), mode and owner match the previous backup are taken from it without being read |

In **Metadata** mode the first run reads everything (there is nothing to compare with yet); later runs read only what changed. A file changed in a way that keeps its size and modification time is not noticed, which is why a set can also **read every file every N runs** (a Data run): that run catches such changes and drops the padding that reuse leaves in the archive. A file that could not be read completely (zero-padded after a read error) is always read again on the next run. Reuse only happens from a previous backup encrypted the same way, with the same key.

**GUI:** edit a Backup Set, **Destination** tab, **Change detection**. It applies to folder sets only: machine (full disk) backups are disk images and are not affected, so bare-metal (Clonezilla) and Proxmox VE restores work exactly as before. One-off backups use Legacy. Only runs that finish successfully count towards the next full read; the Backup Sets list shows "Next full read in N runs". The progress card says when and why a run is reading every file. **Reports** shows each run's mode and how many unchanged files were reused.

**One Backup ID per set.** Metadata mode compares with the newest snapshot in the backup group. If two sets (or two machines) use the same Backup ID on the same server, each run compares with the other's snapshot and reads everything. On master the Backup Set editor warns about this.

**CLI:** `-change-detection-mode legacy|data|metadata`, or `"change-detection-mode"` in the JSON config. There is no CLI "every N runs" flag: schedule an occasional run with `-change-detection-mode data` instead.

**Before switching a set to Data or Metadata,** update every machine you restore from to v0.6.0 or later: builds before split-archive support cannot read split archives (they show the folders without their files). The official `proxmox-backup-client` restores them byte-identically (tested with 3.4.9), and PBS 4.2's file browser opens them. The first split backup of a folder shares little with its older Legacy backups, so it costs roughly one more full copy of that folder on the datastore until the old snapshots are pruned.

## 🚀 Quick start (GUI)

1. Install it: the MSI on Windows, the `.deb` or `.rpm` on Linux, or unpack the zip or tar.gz from the [releases](https://github.com/mjb-is/proxmoxbackupclient_go/releases) (see [Download and install](#-download-and-install))
2. Start it. On Windows, **Run as administrator** for VSS, machine backups and changing Backup Sets. On Linux, folder backups of your own files work as you; use **Proxmox Backup Client (administrator)** (or run it as root) for machine backups and block snapshots
3. Add your PBS server in **Preferences > Account Information** and **Test** it
4. Create a Backup Set (or a one-off backup) and select the folders or disks to back up
5. Run it. To have schedules run with nobody signed in, turn on **Preferences > Advanced > Run as Service**

The [GUI manual](docs/manual/GUI.md) walks through every step.

## Full machine live backup

`proxmoxbackup-machine` (and the GUI's machine backups) back up a complete Windows 10/11 or Windows Server system, or a Linux machine, without any downtime.

The command syntax is mostly the same as for folder backups, except that `-backupdir` is replaced by `-backupdev` (repeat it for several disks).
For example an invocation could be:

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

The above command will look at Disk 0, detect all mounted partitions, take a VSS snapshot of these, and then create a bootable backup image of the whole disk as fidx. Free space and the paging and hibernation files go into the image as zeros without being read (Linux ext2/3/4 and XFS free space too, on master); `-readfreespace` reads everything instead, for a forensic copy.

The next backup will be incremental; hashing has been parallelized so speeds of 1 GB/sec can be easily reached.

### Linux machine backup prerequisites (VSS equivalent)

Windows machine backup uses VSS (hence "Run as administrator" above); Linux has no VSS, so this
project uses the `elastio-snap` kernel module (a fork of `dattobd`) for the same job: a
point-in-time, consistent snapshot of a live block device while it keeps being written to. The original
`elastio/elastio-snap` repository is archived; the maintained version is
[`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap), which is what to install. `dattobd`
itself is also supported if it is already loaded.

Both of the following are required, independently, or a Linux machine backup refuses to run:

1. **The kernel module must be installed and loaded.** `elastio-snap`'s documented repository-package
   install (see [its INSTALL.md](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md)) currently
   points at a moved/broken URL for at least Ubuntu 22.04 (confirmed 2026-09-24: the repo package
   404s after a redirect). Building from source works reliably instead:
   ```bash
   sudo apt-get install linux-headers-$(uname -r) build-essential   # Debian/Ubuntu
   git clone --depth 1 https://github.com/Axcient/elastio-snap.git
   cd elastio-snap
   sudo make
   sudo make install
   sudo depmod -a     # required - `make install` only copies the .ko, it never
                       # runs depmod, so a bare `modprobe elastio-snap` right
                       # after a fresh install fails with "not found in
                       # directory" even though the file is right there
                       # (confirmed live 2026-09-24)
   sudo modprobe elastio-snap
   lsmod | grep elastio-snap   # confirms it loaded
   ```
   This builds and loads it for the current boot only; consult your distro's docs to load it automatically
   on every boot (e.g. an `/etc/modules-load.d/` entry) if you want that.

2. **The backup process itself must run as root (EUID 0).** Even with the module loaded, `CreateVSSSnapshot`
   (the Linux snapshot path despite the Windows-centric name) explicitly refuses to run as a non-root user:
   a consistent whole-disk snapshot needs the module's root-only control interface, so this is a hard
   requirement, not something a `disk`-group membership or similar can substitute for. For a machine backup, use
   the **Proxmox Backup Client (administrator)** menu entry, the background service, or run the GUI or
   `proxmoxbackup-machine` with `sudo` (or as root); a plain file/directory backup has no such requirement.

If a machine backup fails with `backup device /dev/sdX: open /dev/sdX: permission denied`, that's usually
just needing your user in the `disk` group for basic read access (`sudo usermod -aG disk $USER`, then log
out/in), but that alone is NOT enough for a real snapshot; you'll still hit the root requirement above
immediately after.

### File restore from a disk backup

The GUI's Restore page opens a machine backup like a folder backup: each volume in the image is a folder you can browse and restore from (NTFS from v0.8.0; FAT, exFAT, ext2/3/4 and XFS on master). Nothing is mounted.

On Linux you can also attach a disk backup as a block device with the nbd tool.

In order to use nbd please first do `modprobe nbd max_part=0`.

For unknown reasons, using `max_part != 0` causes an infinite partition probe loop.

The NBD tool will connect any fixed disk backup, regardless of it being a VM or host backup (that being said, it also works for PVE backups).

To use it use a command line similar to this:
`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

If you omit `-path`, a terminal UI will show up allowing you to select the fidx file.

> Beware to not use this on a machine running important stuff (a corrupt filesystem can crash the OS potentially, that's why Proxmox VE uses a QEMU instance for this).

Also be very sure to have unmounted anything on the nbd disk before stopping pbsnbd, if not you will likely end up with a busy unmountable partition. If someone has an indication of how to recover from that, please tell me.

If you get a `Device or resource busy` error, you have to force disconnect by running `nbd-client -d /dev/nbd0` or simply reboot.

### Restore to physical machine

This fork ships a patched Clonezilla Live ISO with a fully automated **"PBS Bare Metal Restore"** boot menu entry: see [Clonezilla live ISO (bare-metal restore)](#clonezilla-live-iso-bare-metal-restore) below. It's a separate, first-position, default-on-timeout entry that skips Clonezilla's own language/keyboard prompts entirely: enter your PBS connection details, pick a snapshot, confirm the auto-detected target disk, and it handles network setup, NBD attach, the restore itself and an automatic reboot with no further input.

The older, semi-manual route also still ships on the same ISO, as its own entry in Clonezilla's normal mode-selection menu ("pbs-nbd"): it walks you through PBS connection details and attaches the chosen snapshot to `/dev/nbd0` in the background, then leaves you to drive the actual restore through Clonezilla's own standard disk-restore wizard by hand.

## Usage: Directory Backup

The full reference, with examples for Windows and Linux, exit codes and scheduling, is in the **[command line manual](docs/manual/CLI.md)**. A typical command:

```shell
proxmoxbackup-directory.exe -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -backupdir "C:\path\to\backup" -datastore "datastorename"
```

```
proxmoxbackup-directory
  -baseurl string            PBS URL, example: https://192.168.1.10:8007
  -certfingerprint string    Certificate fingerprint (asked for interactively when omitted)
  -authid string             API token ID (user@realm!token)
  -secret string             API token secret
  -pbsusername string        PBS user for ticket login (with -pbspassword; overrides -authid/-secret)
  -pbspassword string        Password for -pbsusername (asked for when omitted)
  -datastore string          Datastore name
  -namespace string          Namespace (optional)
  -backup-id string          Backup ID (default: the hostname)
  -backupdir string          Source directory; repeat for several (each becomes group <id>_<path>)
  -change-detection-mode     legacy (default), data or metadata
  -novss                     Do not snapshot the source (VSS on Windows, elastio-snap/dattobd on Linux)
  -keyfile string            PBS encryption key file
  -keyfile-passphrase string Passphrase for a protected key (asked for when omitted)
  -backupstream string       Back up standard input as <name>.didx instead of a directory
  -pxarout string            Also write the archive to a local file (debugging)
  -mail-host, -mail-port, -mail-username, -mail-password, -mail-insecure,
  -mail-from, -mail-to, -mail-subject-template, -mail-body-template
                             Email report (optional)
  -config string             JSON config file; flags given on the command line override it
```

The `PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD` and `PBS_FINGERPRINT` environment variables of the official client are read too.

For JSON configuration a JSON example is provided, fill in only the needed fields.

Note on mail templating:
[Go's templating engine](https://pkg.go.dev/text/template) is used for mail subjects and bodies, please refer to the documentation for the syntax.
The following variables are available for templating:

- `.NewChunks`: number of new chunks created
- `.ReusedChunks`: number of chunks reused
- `.Datastore`: datastore name
- `.Error`: the error, if any (`.ErrorStr` gives it as text, empty when there is none)
- `.ReadErrors`: the files or folders that could not be read; `.ReadErrorCount` is how many
- `.Hostname`: hostname of the machine
- `.StartTime`: time the backup started
- `.EndTime`: time the backup ended
- `.Duration`: duration of the backup
- `.FromattedDuration`: formatted duration of the backup (the name is spelt this way in the code)
- `.Success`: true when the backup finished with no errors and nothing unreadable
- `.Partial`: true when a snapshot was saved but some files could not be read
- `.Status`: `Success`, `Partial` or `Failed`

## Stream Backup

This allows backing up a stream instead of a pxar, allowing endless possibilities. For example you can invoke:

```
mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]
```

This allows leveraging buzhash for dedup even when using tar for example, or the sql dump itself. And if someone wants to attempt it, it should be possible with some hack to pipe the DISM command to generate a WIM image to this and have a full host backup.

## Known Issues

Windows Defender antimalware being active will slow the backup down up to 25% of attainable speed.

On Windows the command-line tools take a process lock: a second backup started while one is running prints "Another backup job is running" and exits. On Linux there is no such lock, so do not schedule two CLI runs at the same time.

## 📋 Prerequisites

- Windows 10/11 or Windows Server (64-bit), with administrator rights for VSS and machine backups
- or Linux (the packages are for Debian, Ubuntu, Mint and Fedora), with the WebKitGTK runtime for the GUI, and root plus the elastio-snap or dattobd kernel module for consistent machine backups
- Network access to a Proxmox Backup Server

## 🔨 Building from source

**[BUILD.md](BUILD.md)** has the full instructions: the GUI and the background service, the command-line tools, the MSI and the Linux packages. In short, with Go 1.25 or later and the Wails CLI v2.13:

```bash
cd gui
wails build                                    # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe   # the background service
```

```bash
packaging/debian/build-deb.sh     # .deb
packaging/fedora/build-rpm.sh     # .rpm (in Docker)
```

```powershell
installer\wix\build-msi.ps1       # MSI (WiX 3), after building the app and the service
```

The built frontend (`gui/frontend/dist`) is committed, so Node.js is only needed to change the frontend (`npm install` and `npm run build` in `gui/frontend`, or `wails dev` for hot reload). Tests: `make test`.

## 🔧 Advanced use & guides

### Multi-PBS (multiple PBS servers)

Configure several PBS servers and pick the target per Backup Set or one-off backup (e.g. `C:\Users` to a fast SSD PBS daily, `D:\` to a big-data PBS weekly, plus a DR server).

- **[User guide](MULTI_PBS_USER_GUIDE.md)**: adding and testing servers, the default server, choosing a server per backup, FAQ and troubleshooting.
- **[Implementation guide](MULTI_PBS_GUIDE.md)**: data model, automatic migration from a single-PBS config, backend methods.

Legacy single-PBS configuration is automatically migrated to a `default` server on first load.

### Clonezilla live ISO (bare-metal restore)

The rescue workflow is built by patching a stock Clonezilla Live ISO with the `pbsnbd` / `machinebackup` binaries plus two menu entries (boots from CD, USB via `dd`, and UEFI):

- **`ocs-pbs-nbd`**: a **"pbs-nbd"** entry added to Clonezilla's own mode-selection menu. Prompts for your PBS connection details and the snapshot to restore, attaches it to `/dev/nbd0` in the background, then hands off to Clonezilla's normal disk-restore wizard for you to drive by hand.
- **`ocs-pbs-bare-metal-restore`**: a new, first-position, default-on-timeout boot menu entry: **"Proxmox Backup Client Go - PBS Bare Metal Restore"**. Skips Clonezilla's language/keyboard prompts and its own generic wizard entirely, and walks straight from PBS credentials to a completed, auto-rebooted restore: DHCP is tried first (falling back to a static-IP prompt only if it fails), NTP sync runs by default, the target disk is auto-detected, and on success the machine shows a completion message and reboots on its own. Works identically for Windows- and Linux-sourced backups (Clonezilla restores at the raw block level, so it's OS-agnostic). Built to its own ISO filename alongside the manual-wizard ISO, so both stay available.

Also fixed on the Clonezilla side, independent of either menu entry: a `zstd.NewReader` deadlock in the PBS chunk-fetch path (merged upstream as [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85); the other upstream PRs are listed [above](#-about-this-fork)) and a 100%-reproducible "first restore pass fails with 'no partition', rerun succeeds" bug, root-caused to Clonezilla's own `is_disk_without_part_and_fs()` (in `ocs-functions`) racing against udev rather than any NBD-attach timing.

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

Full details (why a full ISO rebuild instead of an in-place swap, prerequisites, menu flow, verification) in **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**.

### Building the Windows GUI

**Native Windows**: see **[BUILD.md](BUILD.md)** for the toolchain (Go, MinGW, Wails) and the commands for the app, the service and the MSI.

**Docker (when building on Linux).** `./build_gui_windows_docker.sh` builds the Windows app as `ProxmoxBackupClientGO.exe`, with proper WebView2 support, in a disposable `golang` container (installs mingw and Wails, builds the frontend, runs `wails build`). It does not build the background service or the MSI.

### Feature status, changelog & internal docs

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**: what is done, partial or not done, and since which version.
- **[CHANGELOG.md](CHANGELOG.md)**: per-version change history, and [Unreleased] for what is on master.
- **[TODO.md](TODO.md)**: open roadmap and ideas.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**: what each release contains.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**: MSI uninstall dialog (keep/delete configuration) and its test plan.
- **[AGENTS.md](AGENTS.md)**: repository layout and conventions, for contributors and coding agents.

## 🌐 Tech Support in Italy

Being said that the project and **ALL** its contributions will remain forever public and licensed under GPLv3 license, main sponsor of this project and also of the latest machine backup features, E.T.I. Srl ( https://etitech.net ) can provide, to who needs it, support for Proxmox deployments in general and specifically Windows backup tools.

There will never be a "community" & "enterprise" different edition, solely tech support will be an independent service.
Any customization that you are going to ask, even if development may be paid, will be released as GPLv3 like the whole project is.

## 🖥️ GUI attribution

The **Proxmox Backup Client GUI** is based on the **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)**, developed and maintained by **[RDEM Systems](https://www.rdem-systems.com/)**.

The GUI (originally a fork of this project) has been merged back into this repository: the entire codebase, including the GUI and all its features, remains open-source under the GPLv3 license. RDEM Systems sponsors the GUI development and provides commercial support for it.

## ⚠️ Warning

This software is provided "as is". Although we aim for reliability, we decline any responsibility for loss of or damage to data. Always test your backups and verify restoration before relying on them in production.

The software is still alpha quality and we take no responsibility for any kind of damage or data loss, even of source files.

## 📄 License

GPLv3: see the [LICENSE](LICENSE) file.

## 🏷️ Branding

Every contributor who has contributed **at least 5 commits** that add functionality or fixes, has the right to have their branding data added for commercial use.

The only conditions are that the company that the branding points to is **not** running any of the following:

- Malware campaigns
- Businesses promoting war (this applies to any country, including western ones)
- Scams
- Data theft
- Child trafficking
- Violence
- Discrimination
- Drugs
- Any activity generally recognized as illegal

If any complaint shows up at any of the contributors, we will try to get in touch; if no valid explanation is given, we will **immediately terminate** that benefit.

The **GPLv3 license remains active**, and you will still be free to fork the project and build your own executables.

## 🤝 Contribute

The GUI is now fully implemented, but contributions are still welcome, especially:

1. Encryption: native review of the machine-translated encryption strings, and a key-management story beyond a key file (for example a Windows DPAPI or Credential Manager store)
2. Physical-to-virtual (P2V) migration, restoring a bare-metal backup into a virtual machine (working for the basics via the `vm` snapshot type, verified with a Windows UEFI disk; still missing a NIC, TPM, virtio drivers, CPU/RAM taken from the real machine, and non-SATA disk buses)
3. Async upload / multicore upload of chunks (multicore compression is already implemented for machine backup)
4. Proxmox side patch to add another kind of entry to pxar format with Windows security descriptors in it
5. Support for Windows symlinks
6. Anything interesting you can come up with :)

## About Proxmox Backup Client GO contributors

Proxmox Backup Client GO contributors develop and maintain this project. The software relies on the NTP/NTS infrastructure and the [11 public NTS servers](https://github.com/jauderho/nts-servers) listed in the community reference.

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
