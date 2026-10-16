# Proxmox Backup Client: Windows and Linux client for Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)
[![Ko-fi](https://img.shields.io/badge/Ko--fi-buy%20me%20a%20coffee-FF5E5B?logo=ko-fi&logoColor=white)](https://ko-fi.com/mickbeeby)

> **Built on the work of others.** This fork builds on Tiziano Bacocco's (tizbac) original [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) and on RDEM Systems' Nimbus Backup GUI client, which was merged into it and which this app's GUI grew from.
>
> ☕ The client is free and stays free. If it saves you some time, you can [buy me a coffee on Ko-fi](https://ko-fi.com/mickbeeby). It is entirely optional and buys no support or feature promises.

**Proxmox Backup Client is an open-source (GPL-3.0) backup client for Proxmox Backup Server (PBS), for Windows and Linux.** It backs up folders as browsable file archives and whole machines as bootable disk images, on schedules or on demand, and brings back anything from a single file to a whole machine, onto the same computer, a replacement, or a Proxmox VE virtual machine.

<p align="center">
  <a href="docs/screenshots/overview-how-it-works.png"><img src="docs/screenshots/overview-how-it-works.png" width="400" alt="How it works: Windows and Linux machines back up through the client to Proxmox Backup Server, then restore as files, onto bare metal, or into a Proxmox VE VM"></a>
  <a href="docs/screenshots/overview-screenshots.png"><img src="docs/screenshots/overview-screenshots.png" width="400" alt="Screenshots of the app: Backup Sets, Reports, schedules, a whole PC as a VM, backup progress and restore"></a>
</p>

I build it for my own homelab, to make PBS the single backup system for a mix of Windows and Linux servers, desktops and laptops, in place of Backup for Workgroups, rsync mirrors and Clonezilla/RescueZilla images. It began in September 2026 as a fork of [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) and has grown well beyond it: over 350 commits since, with 18 fixes and features sent back to the original project ([list below](#-contributions-to-the-original-project)).

**You are welcome to use it as it is.** It is officially **alpha software**, used at your own risk, with no guarantee or warranty of any kind. I test every release myself and it is in regular use on my own machines, but see [Status, bug reports and feature requests](#-status-bug-reports-and-feature-requests) before relying on it.

The suite:

- **Proxmox Backup Client GUI**: the app for Windows and Linux. Backup Sets with several schedules each, folder and whole-disk backups with consistent snapshots (VSS on Windows, elastio-snap/dattobd on Linux), restore of files from folder and machine backups, Undelete and Roll back, several PBS servers, client-side encryption, and a background service so schedules run with nobody signed in.
- **`proxmoxbackup-directory`**: command-line folder (pxar) backups with deduplication and change detection.
- **`proxmoxbackup-machine`**: command-line live whole-disk backups (fidx) of Windows and Linux machines; the image is bootable after a restore.
- **`proxmoxbackup-nbd`**: attaches a disk backup as an NBD device (Linux).
- **A bare-metal restore ISO** built on Clonezilla Live, with a guided restore from power-on to reboot.

> Keywords: proxmox backup client windows · PBS client · Windows VSS backup · physical to virtual (P2V) · bare-metal restore · Proxmox Backup Server interface.

## ⭐ What it does, with measured results

| | Result |
|---|---|
| **Incremental folder backups** | A 480.9 GB file server share of 761,866 files: first run 4 h 17 min, each later run 10 to 15 minutes. Unchanged files are not even opened (the official client's Metadata change detection). |
| **Fast machine backups** | Free space, the paging file and the hibernation file are not read, and blank disk space is not hashed. A 1 TB laptop disk with 373 GB used: 1 h 23 min to 1 h 44 min before, 38 minutes now. A 50 GB test disk: 3 min 36 s, now 1 min 15 s. |
| **Files back from machine backups** | Browse a disk image like a folder and restore single files, from NTFS, FAT, exFAT, ext2/3/4 and XFS. A laptop's 127 GB C: drive with 430,000 files lists in 4 to 15 seconds; nothing is mounted or downloaded first. |
| **Physical machine into a virtual machine** | Back up a PC or server, then start it as a Proxmox VE VM: restored through Proxmox VE itself, or with the restore ISO. [See below](#-physical-machines-into-virtual-machines-p2v). |
| **Bare-metal restore** | Boot the ISO (USB stick, CD or PXE), enter the PBS details, pick the backup: a 32 GB Windows machine was back at its login screen 8 minutes after power-on. |
| **Undelete** | After 39 GB were deleted from a 761,000-file share, a 7-day search found exactly those files in about a minute, and put them back. |
| **Roll back** | Put a set's folders back as they were at a chosen backup, with a preview first and Undo afterwards. |
| **Schedules** | Several per Backup Set, from every few minutes to once a year: "every day at 02:00" plus "the last day of every month" or "the 2nd Tuesday", each shown in plain words with its next 3 runs. |
| **Restore fidelity** | NTFS permissions, attributes, alternate data streams and extended attributes, creation and folder times on Windows; modes, owners, ACLs, extended attributes and symlinks on Linux. An optional pass re-reads what was restored and compares it with the backup. |
| **Works with Proxmox's own tools** | Backups made here restore byte-identically with the official `proxmox-backup-client`, and the other way round. [Details](#-compatible-with-the-official-proxmox-client-in-both-directions). |

## 🧩 What it adds over the original project

Beyond the [18 changes merged into the original project](#-contributions-to-the-original-project), this client adds:

**Backups**
- Incremental folder backups with the official client's change detection (Data and Metadata modes, split pxar v2 archives), with a full read every N runs.
- Machine backups that skip free space, the paging file and the hibernation file (Windows NTFS; Linux ext2/3/4 and XFS), write unreadable sectors as zeros and carry on, and show blocks, disk, partition and chunk counts as they run.
- Pause, Resume and Stop for folder and machine backups, also for backups the background service runs.
- Alternate data streams, NTFS extended attributes and creation times in Windows folder backups.

**Schedules and Backup Sets**
- Several schedules per Backup Set, from every few minutes to once a year (the nth weekday, the last day of the month), each in plain words with its next 3 runs.
- A background service on Linux as well as Windows, and Restart as administrator on Linux.
- Quick or full Run Now, a VSS / Snapshot badge, a SHARED ID warning, set names written to PBS as snapshot comments, email alerts, post-backup actions, and settings export and import.

**Restores**
- Single files back from machine backups (NTFS, FAT, exFAT, ext2/3/4, XFS), without mounting or downloading the image.
- Undelete and Roll back, for folder and machine Backup Sets, in the GUI and from the command line.
- An optional verify pass after a restore, Windows creation times, and a snapshot tree that loads one folder at a time, so very large backups open at once.
- A guided bare-metal restore ISO that also restores into an empty VM, and tested physical-to-virtual routes ([P2V](#-physical-machines-into-virtual-machines-p2v)).
- Byte-identical compatibility with the official `proxmox-backup-client` in both directions, including its Data and Metadata (split archive) snapshots.

**The app**
- A rebuilt interface: menu bar, side menu, Reports with the full backup and restore history, a Message Log, five colour themes and a custom colour, tray and toast notifications.
- 18 interface languages, every one reviewed (the original has 5).
- Settings folder locked to administrators (Windows) or its owner (Linux), and a service API token renewed at every start.
- MSI, `.deb` and `.rpm` packages, and the user manuals for the GUI, the command line and bare-metal restore.

## 🔄 Physical machines into virtual machines (P2V)

A machine backup of a physical PC or server can become a Proxmox VE virtual machine, two ways:

**1. Restore it in Proxmox VE.** Set the Backup Set (or one-off backup) to **Backup as: Proxmox VE virtual machine**. The backup is stored as `vm/<number>` with a small VM configuration, so Proxmox VE lists it with its other VM backups: open the PBS storage in Proxmox VE, select the backup, **Restore**, pick a VM ID and storage. The VM gets one SATA disk and the right firmware for the disk (OVMF for UEFI/GPT, SeaBIOS for BIOS/MBR); add a network card before starting it, and an EFI disk (Pre-Enroll keys unticked) if a UEFI one does not start.

**2. Restore it with the bare-metal restore ISO into an empty VM.** Create a VM with the same firmware as the original machine (OVMF with an EFI disk for UEFI/GPT, SeaBIOS for BIOS/MBR), a SATA disk at least as large as the original and an Intel E1000 network card, boot it from the ISO and run the restore. This works for `host` and `vm` backups alike, and the same backup still restores onto bare metal.

Tested routes:

| From | To | Route | Result |
|---|---|---|---|
| Windows 10 laptop, UEFI/GPT | Proxmox VE VM | Restore ISO | Started at the user's desktop |
| Windows Server, UEFI/GPT | Proxmox VE VM | Proxmox VE restore | Started without changes |
| Xubuntu desktop, UEFI/GPT | Proxmox VE VM | Proxmox VE restore, and the restore ISO | Started at the desktop |
| Linux, BIOS/MBR | Proxmox VE VM | Restore ISO, and Proxmox VE restore | Started at the login prompt |
| Linux Mint, encrypted backup | Proxmox VE VM | Restore ISO, key from a USB stick | Started |
| Linux PC | the same PC with a new disk | Restore ISO | Started |

Windows sets up the new virtual hardware on first start ("Getting devices ready"). The [bare-metal restore manual](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines) has the firmware table, drivers and step-by-step instructions.

## 📖 User manuals

| Manual | Covers |
|---|---|
| **[GUI manual](docs/manual/GUI.md)** | Install, PBS servers, Backup Sets and schedules, incremental backups, progress, restore, Undelete, Roll back, Reports, logs, Preferences, the background service, troubleshooting |
| **[Command line manual](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: every flag, config files, exit codes, scheduling; Undelete and Roll back from the command line |
| **[Bare-metal restore manual](docs/manual/BMR.md)** | The restore ISO, step by step; `vm` backups onto Proxmox VE; physical to virtual; different hardware |
| [Restore scenarios](docs/RESTORE_GUIDE.md) | Which restore to use when |
| [Manuals index](docs/manual/README.md) | An overview of the three manuals |

## 🆕 New in v0.9.0: several schedules per Backup Set, installers, a Linux service

**v0.9.1** fixes the bare-metal restore ISO: the restore entry is back in its BIOS boot menu, a Ventoy stick is recognised as the boot disk, and a target disk smaller than the original is refused before anything is written. It also notes a failed notification e-mail in the snapshot's PBS comment.

- **Several schedules per Backup Set**, from every few minutes to once a year: "every day at 02:00" plus "every month on the last day" or "the 2nd Tuesday", each schedule in plain words with its next 3 runs. Existing sets keep their schedule.
- **Folder backups keep alternate data streams and NTFS extended attributes** (Windows), such as the "downloaded from the internet" mark, and a restore puts them back. Known Limitations now lists none.
- **Installers**: a **Windows MSI** (app and background service under Program Files, Start menu entry, in-place upgrades, keep or delete the settings on removal) and **Linux packages**, `.deb` (Debian, Ubuntu, Mint) and `.rpm` (Fedora), with the app, the three command-line tools, the manuals and menu entries.
- **A background service on Linux** (`proxmoxbackupclient.service`): scheduled Backup Sets run as root, also when nobody is logged in. Turn it on in Preferences > Advanced > Run as Service, as on Windows. **Restart as administrator** now works on Linux too (through pkexec).
- **Pause, Resume and Stop for backups the service runs**, with their progress card in the open app.
- **A warning when two Backup Sets share a backup group** (same server, type and Backup ID), with a **SHARED ID** badge on both cards.
- **Settings locked down**: on Windows `C:\ProgramData\ProxmoxBackupClient` is now open to administrators only; on Linux the settings folder is private to its owner.
- **Files back from machine backups of every common file system**: FAT, exFAT, ext2/3/4 and XFS as well as NTFS. Windows image restores also put back permissions and alternate data streams.
- **Linux machine backups skip free space** (ext2/3/4, XFS), as Windows ones do since v0.8.0.
- **Undelete and Roll back for Linux machine Backup Sets**, and **from the command line** for any Backup Set: `-list-sets`, `-undelete`, `-rollback` (section 12 of the [command line manual](docs/manual/CLI.md)).
- The command-line tools print errors to stderr instead of showing message boxes, and `proxmoxbackup-machine` sends its notification e-mail.
- **The bare-metal restore ISO** never offers the stick it booted from as the disk to restore onto, and the manual explains matching the firmware (UEFI or BIOS) when a physical machine becomes a VM.
- **Every language reviewed** (17 translations, about 1,850 strings), and dates and numbers follow the app's language.

Earlier releases: **v0.8** faster machine backups (free space skipped, bad sectors survived), files back from machine backups, Pause and Resume; **v0.7** Undelete and Roll back; **v0.6** incremental folder backups. The [CHANGELOG](CHANGELOG.md) has every change.

## 🤝 Compatible with the official Proxmox client, in both directions

Verified with `proxmox-backup-client` 3.4.9 and PBS 4.2:

- **Legacy, Data and Metadata snapshots made by this client restore byte-identically with the official client**, from both the GUI and the command line, made on Windows or Linux, and PBS 4.2's file browser opens them. On Linux the restored files also keep their exact mode, nanosecond modification time and symlink targets.
- **Legacy, Data and Metadata snapshots made by the official client restore byte-identically with this client**, fully or selectively.
- Same split archive format (pxar version 2), same change-detection modes, same encryption key file format (`proxmox-backup-client key create` keys work here, and keys generated here work there), same `PBS_REPOSITORY` style environment variables.

So your backups are never tied to this client: Proxmox's own tools restore every snapshot it makes, and it restores every snapshot the official client makes. The only extra is one small file this client adds at the root of a folder snapshot, `.proxmox_backup_client_meta.json` (the original path), which the official client restores as an ordinary file.

> ⚠️ **Disclaimer:** This project is **not affiliated in any way** with **Proxmox Server Solutions GmbH**. "Proxmox", the Proxmox logo and related names are the property of their respective owners; here they are used **only** to state compatibility. See [proxmox.com](https://www.proxmox.com/) for their products.

## 📦 Download and install

👉 **[Download the latest release](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| You have | Use |
|---|---|
| Windows | The **MSI installer** (`ProxmoxBackupClient-<version>-x64.msi`) installs the app and the background service under Program Files, with a Start menu entry. Or unzip `ProxmoxBackupClient-v<version>-windows-amd64.zip` (the app and `ProxmoxBackupClientSVC.exe`; keep them in the same folder) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Another Linux | Unpack `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (needs the WebKitGTK 4.1 runtime) |
| Command line only | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` or `-linux-amd64.tar.gz` |
| A machine to restore | `ProxmoxBackupClient-GO-BMR-Restore-v<version>.iso`, the bare-metal restore ISO |

The MSI, `.deb` and `.rpm` are new in v0.9.0: releases up to v0.8.1 have only the zip and tar.gz files. [BUILD.md](BUILD.md) describes building them yourself.

The Linux packages install two menu entries: **Proxmox Backup Client** runs the app as you (folder backups of your own files), and **Proxmox Backup Client (administrator)** runs it as root, asking for your password through polkit, for machine backups and block snapshots. The background service is installed but not turned on; see [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **Windows shows "virus detected" (e.g. `Trojan:Win32/Sabsik.FL.A!ml`) or a SmartScreen warning?**
> This is a **known false positive** for Go/Wails applications: it is *not* a virus. The `!ml` suffix indicates a machine-learning model detection that flags *unsigned and uncommon* executables. Check the download against `SHA256SUMS.txt` (below).

### 🔎 Verifying a download

Every release includes a `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-0.9.0-x64.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
```

On Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`.

> ℹ️ **No code signing, CI-signed attestation, or VirusTotal scan yet.** Releases are built and uploaded manually, not yet through a CI pipeline that could produce a verifiable build-provenance attestation. See [TODO.md](TODO.md) if you'd like to help set that up.

## ✨ Features

### GUI
- **🌍 Multilingual**: 18 interface languages: French, English, Italian, German, Polish, Spanish, Bulgarian, Czech, Greek, Hungarian, Latvian, Lithuanian, Dutch, Portuguese, Romanian, Slovak, Turkish and Ukrainian
- Native menu bar and side menu (Backup, Restore, Undelete, Roll back, Reports, Message Log, Preferences, About)
- **Backup Sets**: reusable named jobs with **several schedules each** (every few minutes to yearly, or manual), a directory-vs-machine type badge, a VSS / Snapshot badge, "Run Now" (quick or full read for Metadata sets), and a **SHARED ID** warning when two sets would write into one backup group
- **One-off backups** of folders or whole disks
- **Reports** page with full backup and restore history, and a capped **Message Log** for diagnostics
- **Preferences** dialog with five colour themes (Amber, Blue, Green, Red, Dark) plus a custom colour option
- **Incremental folder backups** (change detection, Metadata mode): unchanged files are reused without being read, as in the official client, with an optional full read every N runs
- **Machine (whole-disk) backups** on Windows and Linux that skip free space and survive bad sectors, stored as `host` or as a Proxmox VE `vm` backup
- **Physical to virtual**: a machine backup restores into a Proxmox VE VM, through Proxmox VE or the restore ISO
- **Restore**: snapshot browsing, file search (wildcards, regular expressions, full paths) and selective restore, with NTFS/POSIX ACLs, alternate data streams, folder timestamps, Windows creation times and an optional verify pass
- **Files back from machine backups**: browse and restore files from NTFS, FAT, exFAT, ext2/3/4 and XFS volumes in a disk image
- **Undelete** and **Roll back**: find files a Backup Set's backups still have but the disk does not, and put them back; or put a set's folders back as they were at a chosen backup, with a preview first and Undo afterwards. Folder and machine sets. See the [GUI manual](docs/manual/GUI.md#10-undelete)
- **Pause, Resume and Stop** a running backup
- **Run as Service**: install, start, stop or remove the background service from Preferences > Advanced, so schedules run without anyone signed in (Windows and Linux)
- Live progress card: start time and forecast finish, files done of total, processing speed (data and files per second) and actual upload speed, current file, and why a run is reading every file
- VSS (Volume Shadow Copy) on Windows, elastio-snap/dattobd block snapshots on Linux
- Email notifications and post-backup actions, Backup Set cloning, settings export and import
- Several PBS servers, with certificate fingerprint pinning (trust on first use), API token or user/password login
- **Client-side encryption**: per-server PBS key file (AES-256-GCM, manifest signed), optional passphrase protection, see [Client-side encryption](#-client-side-encryption)
- Administrator protection for changing Backup Sets, with Restart as administrator
- Debug logging for diagnostics

### CLI tools
- `proxmoxbackup-directory`: folder (pxar) backups with deduplication and `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: live whole-disk backups of Windows (VSS) and Linux (elastio-snap/dattobd) machines (fidx, incremental, free space skipped unless `-readfreespace`, `-type vm` for a Proxmox VE backup)
- `proxmoxbackup-nbd`: NBD server for restoring disk backups (Linux)
- `ProxmoxBackupClientSVC` (Windows) and `proxmoxbackupclient` (Linux packages): `-list-sets`, `-undelete` and `-rollback` for Backup Sets defined in the GUI

The first three accept `-keyfile` and `-keyfile-passphrase` for encrypted snapshots. Full reference: [command line manual](docs/manual/CLI.md).

### 📸 Screenshots

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets**: named jobs with a badge for the schedule that runs next (DAILY, WEEKLY, MONTHLY, +N for the others) and a directory/machine type badge, Run Now on each

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

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**Schedules**: several per set, from every few minutes to once a year ("every month on the last day", "the 2nd Tuesday"), with the next 3 runs

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Backup as a Proxmox VE VM**: choose the virtual machine type and a numeric ID; the PBS backup ID is shown as you type

</td>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Backup Sets as Proxmox VE VMs**: a set stored as a `vm` backup carries a `MACHINE (VM)` badge

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/oneoff-as-vm.png" width="280">](docs/screenshots/oneoff-as-vm.png)
**One-off backups**: the same option on the one-off form

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**Selective restore**: tick individual files/folders from the snapshot tree, restore in place or elsewhere

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Reports**: full run history with a detail panel (duration, chunks, folders backed up)

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-message-log.png" width="280">](docs/screenshots/gui-message-log.png)
**Message Log**: a capped, timestamped diagnostic feed across every backup and restore

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**Multi-PBS**: manage several PBS servers, test connectivity, pick a default

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-edit-server.png" width="280">](docs/screenshots/gui-preferences-edit-server.png)
**Add/edit a server**: URL, user/password or API token, datastore and namespace

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-theme-amber.png" width="280">](docs/screenshots/gui-theme-amber.png)
**Themes**: five built-in colour palettes plus a custom colour, recolouring the whole app live (see the strip below)

</td>
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

**System folders:** `System Volume Information` (VSS storage, can reach 100+ GB) and `$RECYCLE.BIN` wherever they are; `Recovery` only at the root of a drive; `.pbs-rollback` (Roll back's safety folder).
**System files:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` and `DumpStack.log.tmp`.

**Why this matters:** a drive may show 1.03 TB used while real files are ~141 GB. Without exclusions the backup would include VSS snapshots (wasted space and time); with them, the size matches the real data.

**Recommendation:** use a **folder** backup with these exclusions for file-level backups; use a **machine** backup in a separate Backup Set for bare-metal restore and P2V (it is a whole-disk image; free space and the paging and hibernation files go in as zeros).

### Security & quality
- Optional client-side encryption of every chunk, with a signed manifest
- Settings folder open to administrators only on Windows, private to its owner on Linux
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

**Bare-metal restore:** the restore ISO asks for the key (none, a key file on a USB stick, or a typed path) and for the passphrase of a protected key. The key is never written to the ISO. See [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md).

**Known limitations**
- A key generated by the GUI is written with a protected ACL (current user, SYSTEM and Administrators only). Key files you keep elsewhere keep whatever permissions their folder gives them.
- `config.json` holds the PBS tokens, passwords and any remembered passphrase in plain text. From v0.9.0 its folder is open to administrators only (Windows) or to its owner only (Linux); up to v0.8.2 it was readable by local Users on Windows, so restrict it yourself on a shared machine.
- Restoring an encrypted snapshot needs the same key file; there is no recovery if it is lost.
- The Ask-key flow and the service are separate processes: use Remember for anything that runs through the service.

## Faster folder backups: change detection

In the default Legacy mode every folder backup reads every file, so a run takes as long as reading the whole folder even when nothing changed. Backup Sets (and the CLI) can use the change-detection modes of the official `proxmox-backup-client` instead. Metadata mode is in daily use on a 480.9 GB, 761,866-file server, where a run with few changes takes 10 to 15 minutes instead of over 4 hours.

| Mode | Reads | Stored as |
|---|---|---|
| **Legacy** (default) | every file | one `<name>.pxar.didx` per folder, as before |
| **Data** | every file | a split archive: `<name>.mpxar.didx` (metadata) + `<name>.ppxar.didx` (file contents) |
| **Metadata** | only new and changed files | the same split archive; files whose size and modification time (to the nanosecond), mode and owner match the previous backup are taken from it without being read |

In **Metadata** mode the first run reads everything (there is nothing to compare with yet); later runs read only what changed. A file changed in a way that keeps its size and modification time is not noticed, which is why a set can also **read every file every N runs** (a Data run): that run catches such changes and drops the padding that reuse leaves in the archive. A file that could not be read completely (zero-padded after a read error) is always read again on the next run. Reuse only happens from a previous backup encrypted the same way, with the same key.

Because unchanged data is neither read nor sent, it also cuts traffic to a remote or offsite PBS to little more than what actually changed.

**GUI:** edit a Backup Set, **Destination** tab, **Change detection**. It applies to folder sets only: machine (full disk) backups are disk images and are not affected, so bare-metal and Proxmox VE restores work exactly as before. One-off backups use Legacy. Only runs that finish successfully count towards the next full read; the Backup Sets list shows "Next full read in N runs". The progress card says when and why a run is reading every file. **Reports** shows each run's mode and how many unchanged files were reused.

**One Backup ID per set.** Metadata mode compares with the newest snapshot in the backup group. If two sets (or two machines) use the same Backup ID on the same server, each run compares with the other's snapshot and reads everything. The Backup Set editor warns about this.

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

The above command will look at Disk 0, detect all mounted partitions, take a VSS snapshot of these, and then create a bootable backup image of the whole disk as fidx. Free space and the paging and hibernation files go into the image as zeros without being read (on Linux, ext2/3/4 and XFS free space too); `-readfreespace` reads everything instead, for a forensic copy. A sector that cannot be read goes into the image as zeros, and the run lists where. Add `-type vm` to store the backup as a Proxmox VE VM backup ([P2V](#-physical-machines-into-virtual-machines-p2v)).

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

The GUI's Restore page opens a machine backup like a folder backup: each volume in the image (NTFS, FAT, exFAT, ext2/3/4 or XFS) is a folder you can browse and restore from. Nothing is mounted.

On Linux you can also attach a disk backup as a block device with the nbd tool.

In order to use nbd please first do `modprobe nbd max_part=0`.

For unknown reasons, using `max_part != 0` causes an infinite partition probe loop.

The NBD tool will connect any fixed disk backup, regardless of it being a VM or host backup (that being said, it also works for PVE backups).

To use it use a command line similar to this:
`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

If you omit `-path`, a terminal UI will show up allowing you to select the fidx file.

> Do not use this on a machine running anything important: a corrupt file system can crash the OS, which is why Proxmox VE uses a QEMU instance for this.

Unmount everything on the nbd disk before stopping `proxmoxbackup-nbd`, or the partition is likely to stay busy and impossible to unmount.

If you get a `Device or resource busy` error, you have to force disconnect by running `nbd-client -d /dev/nbd0` or simply reboot.

### Restore to a physical machine or a VM

The bare-metal restore ISO has a fully automated **"PBS Bare Metal Restore"** boot menu entry: see [Bare-metal restore ISO](#-bare-metal-restore-iso) below. It is the first, default-on-timeout entry and skips Clonezilla's own language and keyboard prompts: enter your PBS connection details, pick a snapshot, confirm the detected target disk, and it handles network setup, NBD attach, the restore itself and an automatic reboot with no further input. The same ISO restores into an empty virtual machine for [P2V](#-physical-machines-into-virtual-machines-p2v).

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
  -no-mtime                  Record 1970-01-01 as every file and folder time (advanced; see the CLI manual)
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

This allows leveraging buzhash for dedup even when using tar for example, or the sql dump itself.

## Known Issues

Windows Defender antimalware being active will slow the backup down up to 25% of attainable speed.

On Windows the command-line tools take a process lock: a second backup started while one is running prints "Another backup job is running" and exits. On Linux there is no such lock, so do not schedule two CLI runs at the same time.

## 📋 Prerequisites

- Windows 10/11 or Windows Server (64-bit), with administrator rights for VSS and machine backups
- or Linux (the packages are for Debian, Ubuntu, Mint and Fedora), with the WebKitGTK runtime for the GUI, and root plus the elastio-snap or dattobd kernel module for consistent machine backups
- Network access to a Proxmox Backup Server
- For P2V: Proxmox VE with the PBS datastore added as storage, or any hypervisor that can boot the restore ISO

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

### 💿 Bare-metal restore ISO

The restore ISO is a stock Clonezilla Live ISO patched with the `pbsnbd` / `machinebackup` binaries plus two menu entries. It boots from CD, from a USB stick (written with `dd`, or from a Ventoy stick), over PXE, and on UEFI or BIOS machines:

- **`ocs-pbs-bare-metal-restore`**: the first, default-on-timeout boot menu entry, **"Proxmox Backup Client Go - PBS Bare Metal Restore"**. It skips Clonezilla's language and keyboard prompts and its generic wizard, and goes straight from PBS credentials to a completed, auto-rebooted restore: DHCP first (a static-IP prompt only if that fails), NTP sync, an encryption key from a USB stick if the backup is encrypted, and target disk detection that never offers the stick it booted from. On success it shows a completion message and reboots. It restores Windows and Linux backups alike (the restore is at block level), onto bare metal or into an empty VM.
- **`ocs-pbs-nbd`**: a **"pbs-nbd"** entry in Clonezilla's own mode-selection menu. Prompts for your PBS connection details and the snapshot to restore, attaches it to `/dev/nbd0` in the background, then hands off to Clonezilla's normal disk-restore wizard for you to drive by hand.

Also fixed on the restore side: a `zstd.NewReader` deadlock in the PBS chunk-fetch path that hung full-machine restores (contributed to the original project as [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85)), a "first restore pass fails with 'no partition', rerun succeeds" bug caused by Clonezilla's own `is_disk_without_part_and_fs()` racing udev, and BIOS/MBR machines that did not start after a restore (the data between the MBR and the first partition is now restored too).

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

Full details (why a full ISO rebuild instead of an in-place swap, prerequisites, menu flow, verification) in **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**; using it in the **[bare-metal restore manual](docs/manual/BMR.md)**.

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

## 🔁 Contributions to the original project

Fixes and features of general use are offered to [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) as pull requests. Eighteen have been merged:

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
  - [PR #105](https://github.com/tizbac/proxmoxbackupclient_go/pull/105): Restore: put back every file time, also 1970 and earlier, and folder times

## 🧪 Status, bug reports and feature requests

**This is alpha software, provided as is, without guarantee or warranty of any kind, and used entirely at your own risk.** I take no responsibility for lost or damaged data, including source files.

It is built for my own homelab. Every release is tested by me before it goes out, and it is in regular use there: scheduled folder and machine backups of Windows and Linux machines, with restores, Undelete, Roll back, bare-metal restores and P2V restores tested along the way. No backup is proven until you have restored from it, so test your own restores, including a bare-metal one, before relying on it.

- **Bug reports are welcome**: open an [issue](https://github.com/mjb-is/proxmoxbackupclient_go/issues) with what you did, what happened and the log (Message Log, or the files in the settings folder).
- **Feature requests**: I add features that help my own use, so I am not taking requests for others. Please send them to the original project, [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go/issues). Pull requests that fix bugs, or that would clearly help my use too, are welcome.

Bug reports and fixes are the most useful help. If you would like to say thanks another way, you can [buy me a coffee on Ko-fi](https://ko-fi.com/mickbeeby).

## 🙏 Credits and history

The original command-line client was written by Tiziano Bacocco (tizbac) from 2023. RDEM Systems' graphical client was later merged into it, and with it the first GUI, multi-PBS support and the Windows service. This fork started in September 2026 to fill the gaps that kept PBS from being my only backup system; its releases run from v0.3.0, and fixes of general use go back to the original project.

- The original client, [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go), by Tiziano Bacocco (tizbac) and its contributors.
- RDEM Systems' Nimbus Backup GUI client, the starting point for this app's GUI.
- The workflow and feel of Backup for Workgroups (BFW), which Backup Sets, Undelete and Roll back follow.
- [Clonezilla Live](https://clonezilla.org/), which the bare-metal restore ISO is built on.

## 📄 License

GPLv3: see the [LICENSE](LICENSE) file.

---

**Changes since the fork © 2026 mjb-is, building on the work of Tiziano Bacocco (tizbac), the Proxmox Backup Client GO contributors and RDEM Systems (© 2024-2026).**

☕ If this client is useful to you, you can [buy me a coffee on Ko-fi](https://ko-fi.com/mickbeeby). Thank you.
