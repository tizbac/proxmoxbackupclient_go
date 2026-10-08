# Proxmox Backup Client — Windows and Linux client for Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/tizbac/proxmoxbackupclient_go)](https://github.com/tizbac/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-github-orange)](https://github.com/tizbac/proxmoxbackupclient_go)

**Proxmox Backup Client is an open-source (GPL-3.0) backup client for Proxmox Backup Server (PBS), working on Windows and Linux.**

It is a **suite of tools** for backing up to PBS:

- **Proxmox Backup Client GUI** (based on the Nimbus Backup GUI from RDEM Systems) — modern graphical interface for backing up Windows and Linux servers and workstations to PBS: consistent VSS snapshots, scheduled jobs, file and disk modes, snapshot browsing/restoration, multi-PBS support and a Windows service mode.
- **`proxmoxbackup-directory`** — command-line tool for directory (PXAR) backups with deduplication.
- **`proxmoxbackup-machine`** — command-line tool for full live machine backups (FIDX, VSS, incremental).
- **Linux machine backup** (`machinebackup` binary): live-consistent whole-disk image of a running Linux host (bootable, raw, FIDX). Mounted partitions are snapshotted point-in-time with the elastio-snap/dattobd kernel module.
- **`proxmoxbackup-nbd`** — NBD server for restoring disk backups (Linux).

> Keywords: proxmox backup client windows · PBS client · Windows VSS backup · immutable offsite backups · Proxmox Backup Server interface.

## 📖 User manuals

| Manual | Covers |
|---|---|
| **[GUI manual](docs/manual/GUI.md)** | Install, PBS servers, Backup Sets and schedules, incremental backups, progress, restore, Reports, logs, Preferences, troubleshooting |
| **[Command line manual](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: every flag, config files, exit codes, scheduling |
| **[Bare-metal restore manual](docs/manual/BMR.md)** | The restore ISO, step by step; `vm` snapshots onto Proxmox VE; different hardware |
| [Restore scenarios](docs/RESTORE_GUIDE.md) | Which restore to use when |

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

This is **mjb-is's fork** of the upstream project — repo: **https://github.com/mjb-is/proxmoxbackupclient_go**. Focus areas: creating functionality and flow that would allow PBS to be the single server-based backup system in my homelab, which currently uses a mix of Proxmox Backup Server with Proxmox VE VM and Linux host backups, Backup for Workgroups for Windows servers file-based and bare-metal recovery, FastFileSync/Rsync incremental file mirrors, and CloneZilla/RescueZilla. Hardening the Windows/Linux GUI client and building a fully automated bare-metal restore path on top of Clonezilla. Highlights over upstream:

- **Rebuilt GUI**: a refined native File/View/Tools/Help menu bar, a left sidebar (Backup, Restore, Reports, Message Log, Servers, About), a "Backup Sets" scheduler (enhanced daily / interval / manual trigger modes, with a type badge and a one-click "Run Now"), a Reports page with full backup history, a capped Message Log, and a Preferences dialog with five colour themes (Amber, Blue, Green, Red, Dark) plus a custom colour option. Eighteen interface languages now (French, English, Italian, German, Polish, Spanish, Bulgarian, Czech, Greek, Hungarian, Latvian, Lithuanian, Dutch, Portuguese, Romanian, Slovak, Turkish and Ukrainian), with removal of all hardcoded language, so the full application displays consistently in the selected language.
- **A real PBS/NBD chunk-fetch reliability fix**: Regular hangs on full machine backup restore (4 fails to 1 success typical) due to a `zstd.NewReader` call in the reader path started with a live `io.Reader` instead of `nil`, silently spinning up an unused background streaming-decode goroutine that could deadlock a bare-metal restore mid-transfer. Fixed upstream too, merged as [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85).
- **New: a fully automated "PBS Bare Metal Restore" wizard** for the Clonezilla live ISO — see [below](#clonezilla-live-iso-bare-metal-restore). Test restore of a 32GiB Windows machine from power-on into the boot ISO, through to a restored Windows login, in just 8 minutes; similarly, a 40GiB Linux Mint restore in 10 minutes to the login prompt. Both tested on isolated VMs, still to be tested on bare metal.
- **Restore fidelity and feedback**: NTFS ACLs and DOS attributes are re-applied on Windows, and POSIX ACLs and extended attributes on Linux (both verified live). Folder timestamps are restored as well as file ones. Linux folder backups also record real modes, owners and symlinks, and a root restore re-applies them. An optional "Verify after restore" pass re-reads what was written and compares it with the snapshot. Selective restore uses the archive's own index instead of scanning it, the snapshot tree loads one folder at a time, and the restore progress bar tracks bytes actually written, with a stage label under it and the run recorded in Reports.
- **Backup Set workflow**: clone a set, a Backup Set's name is written to PBS as the snapshot comment (one-off backups get an optional comment field), email notifications with on-failure options, post-backup actions (shut down, exit the app, run an application), settings export and import between machines, a queue that names what is waiting behind the active job, and a Stop button on the progress card.
- **Backup behaviour**: an optional multi-threaded read-ahead for backups (experimental), the file currently being archived is shown during directory backups, network shares are no longer snapshotted with VSS, and mapped drives are visible in the folder picker even when the app is elevated. A tray tooltip and Windows toast notifications report backup and restore activity.
- **Machine backup as a Proxmox VE "vm" snapshot.** The one-off backup form and Backup Sets can store a machine backup as `vm/<id>` so Proxmox VE can list and restore it (pick a target Storage in the restore dialog; a restored Windows UEFI/GPT disk is verified to boot). The ID is numeric, with an offset into a reserved range by default so it cannot clash with a real VM. The generated VM config is minimal (SATA disk, OVMF for GPT disks, no TPM or Secure Boot keys, no drivers), so expect a bare-bones VM. This also works as a physical-to-virtual conversion: a `vm` snapshot restores onto Proxmox VE and boots (verified with a Windows Server UEFI disk), and the same snapshot can still be restored onto bare metal with the Clonezilla bare-metal restore, which lists `vm` snapshots alongside `host` ones (verified over PXE onto a blank VM). A Backup Set stores its numeric ID, so every scheduled run lands in the same `vm/<id>` group; a set with a non-numeric ID is refused at save time. Host backup remains the default. Restoring these snapshots onto Proxmox VE and onto bare metal has been tested repeatedly with no failures to date.

Anything of general use gets sent upstream as a PR rather than kept fork-only; day-to-day fork-specific work stays here. Nine pull requests have been sent to the original project so far, and all nine have been merged:

  - [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85): Fix zstd decoder goroutine deadlock in `GetChunkData` (hung full-machine restores)
  - [PR #86](https://github.com/tizbac/proxmoxbackupclient_go/pull/86): Fully automated PBS Bare Metal Restore boot entry for the Clonezilla ISO
  - [PR #87](https://github.com/tizbac/proxmoxbackupclient_go/pull/87): Linux snapshots: drive elastio-snap/dattobd through ioctl instead of `elioctl`/`dbdctl`
  - [PR #88](https://github.com/tizbac/proxmoxbackupclient_go/pull/88): Restore: re-apply captured NTFS ACLs (Windows) and POSIX ACLs and xattrs (Linux)
  - [PR #90](https://github.com/tizbac/proxmoxbackupclient_go/pull/90): `machinebackuplib`: make `vm` backups restorable and bootable on Proxmox VE
  - [PR #91](https://github.com/tizbac/proxmoxbackupclient_go/pull/91): GUI: back machines up as `host`, not `vm`, by default
  - [PR #92](https://github.com/tizbac/proxmoxbackupclient_go/pull/92): `patch-clonezilla`: skip patches whose change is already in the base ISO
  - [PR #93](https://github.com/tizbac/proxmoxbackupclient_go/pull/93): `patch-clonezilla`: add the PBS bare-metal restore entry to `isolinux.cfg`
  - [PR #94](https://github.com/tizbac/proxmoxbackupclient_go/pull/94): Restore the data between the MBR and the first partition in the BMR flow (BIOS/MBR machines boot after restore)

The original author may choose to merge anything else from this repo as they see fit under GPL.

## 📦 Download

👉 **[Download the latest release](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

> ⚠️ **Windows shows "virus detected" (e.g. `Trojan:Win32/Sabsik.FL.A!ml`) or a SmartScreen warning?**
> This is a **known false positive** for Go/Wails applications — it is *not* a virus. The `!ml` suffix indicates a machine-learning model detection that flags *unsigned and uncommon* executables. Upstream has more detail: [why this happens and how to verify their download](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Verifying a download

Every release from this fork includes a `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-v0.6.0-windows-amd64.zip -Algorithm SHA256   # compare with SHA256SUMS.txt
```

> ℹ️ **No code signing, CI-signed attestation, or VirusTotal scan yet** for this fork's own builds (upstream's own releases have some of these — see their repo). These are built and uploaded manually, not yet through a CI pipeline that could produce a verifiable build-provenance attestation. See [TODO.md](TODO.md) if you'd like to help set that up.

## ✨ Features

### GUI — Proxmox Backup Client GUI (recommended)
- **🌍 Multilingual** — 18 interface languages: French, English, Italian, German, Polish, Spanish, Bulgarian, Czech, Greek, Hungarian, Latvian, Lithuanian, Dutch, Portuguese, Romanian, Slovak, Turkish and Ukrainian
- Native menu bar + sidebar navigation (Backup, Restore, Reports, Message Log, Servers, About)
- **Backup Sets** — reusable named jobs with daily / interval / manual scheduling, a directory-vs-machine type badge, and "Run Now"
- **Reports** page with full backup history, and a capped **Message Log** for diagnostics
- **Preferences** dialog with five colour themes (Amber, Blue, Green, Red, Dark) plus a custom colour option
- **Incremental folder backups** (change detection, Metadata mode): unchanged files are reused without being read, as in the official client, with an optional full read every N runs
- **Undelete** and **Roll back** (from v0.7.0): find files a Backup Set's backups still have but the disk does not, and put them back; or put a set's folders back as they were at a chosen backup, with a preview first and Undo afterwards. See the [GUI manual](docs/manual/GUI.md#10-undelete)
- **Run as Service** (Windows, from v0.7.0): install, start, stop or remove the background service from Preferences, so schedules run without anyone signed in
- User-friendly configuration with connection test
- Live progress card: start time and forecast finish, files done of total, processing speed (data and files per second) and actual upload speed, current file, and why a run is reading every file
- VSS (Volume Shadow Copy) support for consistent backups
- Multi-folder backups, file and disk modes
- Snapshot browsing, file search (wildcards) and selective restoration, with NTFS/POSIX ACL restore, folder timestamps and an optional verify pass
- Email notifications and post-backup actions, Backup Set cloning, settings export and import
- Machine backup as a Proxmox VE `vm` snapshot
- Multi-PBS server support with certificate fingerprint pinning (TOFU)
- **Client-side encryption** — per-server PBS key file (AES-256-GCM, manifest signed), optional passphrase protection, see [Client-side encryption](#client-side-encryption)
- Windows service mode + scheduled backups
- Backup cancel, full history and rerun
- Debug logging for diagnostics

### CLI tools
- `proxmoxbackup-directory` — directory (PXAR) backups with deduplication and `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine` — full live Windows machine backups (FIDX, VSS, incremental)
- `proxmoxbackup-nbd` — NBD server for restoring disk backups (Linux)

All three accept `-keyfile` and `-keyfile-passphrase` for encrypted snapshots. Full reference: [command line manual](docs/manual/CLI.md).

### 📸 Screenshots

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets** — named jobs with a repeat/manual trigger badge and a directory/machine type badge, Run Now on each

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Live backup progress** — throughput, ETA, and new-vs-reused chunk counts as it runs

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restore** — pick a PBS server and backup ID, list snapshots or search across them by filename

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Backup Sets as Proxmox VE VMs** — a set stored as a `vm` snapshot carries a `MACHINE (VM)` badge

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Backup as** — choose the Proxmox VE virtual machine type and a numeric ID; the PBS backup ID is shown as you type

</td>
<td width="33%">

[<img src="docs/screenshots/oneoff-as-vm.png" width="280">](docs/screenshots/oneoff-as-vm.png)
**One-off backups** — the same option on the one-off form
</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**Selective restore** — tick individual files/folders from the snapshot tree, restore in place or elsewhere

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Reports** — full run history with a detail panel (duration, chunks, folders backed up)

</td>
<td width="33%">

[<img src="docs/screenshots/gui-message-log.png" width="280">](docs/screenshots/gui-message-log.png)
**Message Log** — a capped, timestamped diagnostic feed across every backup and restore

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**Multi-PBS** — manage several PBS servers, test connectivity, pick a default

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-edit-server.png" width="280">](docs/screenshots/gui-preferences-edit-server.png)
**Add/edit a server** — URL, user/password or API token, datastore and namespace

</td>
<td width="33%">

[<img src="docs/screenshots/gui-theme-amber.png" width="280">](docs/screenshots/gui-theme-amber.png)
**Themes** — five built-in colour palettes plus a custom colour, recolouring the whole app live (see the strip below)

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-advanced.png" width="280">](docs/screenshots/gui-preferences-advanced.png)
**Advanced options** — explained honestly: parallel restore extraction is experimental and measured *slower* on our own test hardware

</td>
</tr>
</table>

**🎨 Colour themes** — the Preferences dialog itself recolours with each pick, not just a swatch:

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

**🌍 Multilingual** — the Reports page in French, Italian and Spanish (and 15 more, including English, German and Polish):

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

### Smart system exclusions (file mode)
When backing up an entire drive (e.g. `D:\`), the GUI automatically excludes:

**System folders:** `System Volume Information` (VSS storage, can reach 100+ GB), `$RECYCLE.BIN`, `Recovery`.
**System files:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys`.

**Why this matters:** a drive may show 1.03 TB used while real files are ~141 GB. Without exclusions the backup would include VSS snapshots (wasted space and time); with them, the size matches the real data.

**Recommendation:** use **file mode** (default) with auto-exclusions for file-level backups; use **disk mode** in a separate job for bare-metal restoration (includes everything).

### Security & quality
- Optional client-side encryption of every chunk, with a signed manifest
- Input validation and credential sanitization
- Path traversal prevention
- Retry logic with exponential backoff
- Comprehensive error handling and tests, 100% lint compliance

## 🔐 Client-side encryption

Snapshots can be encrypted before they leave the machine, using a Proxmox Backup Server key file (the format `proxmox-backup-client key create` writes, so keys are interchangeable with the official client). Every chunk is encrypted with AES-256-GCM and the manifest is signed. The PBS server never sees the key.

**GUI:** open **Servers**, edit a server and use its **Encryption** tab. Browse to an existing key file or **Generate...** a new one (the Advanced option protects it with a passphrase). The tab shows the key fingerprint. Reports show whether each backup was encrypted. **Without the key file a snapshot cannot be restored: keep a copy somewhere safe, away from this machine.**

**Passphrase-protected keys (scrypt or PBKDF2):** choose how the app unlocks the key:
- **Ask me** — the passphrase is asked once per app session and kept in memory only. Scheduled and startup backups never prompt: they fail fast, with a clear message, until the passphrase has been entered in that session. A backup run through the installed Windows service also fails fast for an Ask key.
- **Remember on this computer** — the passphrase is stored in the config like the PBS token, so anyone who can read `config.json` can unlock the key. This is the choice for unattended scheduled jobs and for the Windows service.

Restore asks for the passphrase when you select an encrypted snapshot.

**CLI:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` is only needed for a protected key and is prompted for when omitted. `proxmoxbackup-machine` and `proxmoxbackup-nbd` take the same two flags; `proxmoxbackup-nbd` needs them to attach an encrypted disk image and checks the first chunk before touching the NBD device, so a wrong key fails immediately.

**Bare-metal restore:** the Clonezilla ISO asks for the key (none, a key file on a USB stick, or a typed path) and for the passphrase of a protected key. The key is never written to the ISO. See [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md).

**Known limitations**
- A key generated by the GUI is written with a protected ACL (current user, SYSTEM and Administrators only). Key files you create or copy yourself keep whatever permissions their folder gives them, and `config.json` (readable by local Users on a default install, holds the PBS token and any remembered passphrase) is not tightened. Restrict them yourself on a shared machine.
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

**One Backup ID per set.** Metadata mode compares with the newest snapshot in the backup group. If two sets (or two machines) use the same Backup ID, each run compares with the other's snapshot and reads everything.

**CLI:** `-change-detection-mode legacy|data|metadata`, or `"change-detection-mode"` in the JSON config. There is no CLI "every N runs" flag: schedule an occasional run with `-change-detection-mode data` instead.

**Before switching a set to Data or Metadata,** update every machine you restore from to v0.6.0: builds before split-archive support cannot read split archives (they show the folders without their files). The official `proxmox-backup-client` restores them byte-identically (tested with 3.4.9), and PBS 4.2's file browser opens them. The first split backup of a folder shares little with its older Legacy backups, so it costs roughly one more full copy of that folder on the datastore until the old snapshots are pruned.

## 🚀 Quick start (GUI)

1. Download the Windows zip (or Linux tar.gz) from the [releases](https://github.com/mjb-is/proxmoxbackupclient_go/releases) and unpack it
2. Launch `ProxmoxBackupClient.exe` with administrator rights (required for VSS)
3. Add your PBS server in Preferences and test it
4. Create a Backup Set (or a one-off backup) and select the folders to back up
5. Run it

The [GUI manual](docs/manual/GUI.md) walks through every step.

## NEW! — Full machine live backup

New functionality has been added that now allows backing up a complete Windows 10/11 system, or Linux machine, and their respective server versions without any downtime.

The command syntax is mostly the same, except `-backupdir string`.

In the case of machine backup executable there's in place of backupdir, `-backupdev`.
For example an invocation could be:

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

The above command will look at Disk 0, detect all mounted partitions, take a VSS snapshot of these, and then create a bootable backup image of the whole disk as FIDX.

The next backup will be incremental, hashing has been parallelized so speeds of 1 GB/sec can be easily reached.

### Linux machine backup prerequisites (VSS equivalent)

Windows machine backup uses VSS (hence "launch with administrator rights" above); Linux has no VSS, so this
project uses the `elastio-snap` kernel module (a fork of `dattobd`) for the same job — a
point-in-time, consistent snapshot of a live block device while it keeps being written to. The original
`elastio/elastio-snap` repository is archived; the maintained version is
[`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap), which is what to install. `dattobd`
itself is also supported if it is already loaded.

Both of the following are required, independently, or a Linux machine backup refuses to run:

1. **The kernel module must be installed and loaded.** `elastio-snap`'s documented repository-package
   install (see [its INSTALL.md](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md)) currently
   points at a moved/broken URL for at least Ubuntu 22.04 (confirmed 2026-09-24 — the repo package
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
   (the Linux snapshot path despite the Windows-centric name) explicitly refuses to run as a non-root user —
   a consistent whole-disk snapshot needs the module's root-only control interface, so this is a hard
   requirement, not something a `disk`-group membership or similar can substitute for. Run the GUI or
   `machinebackup` with `sudo` (or as root) for a machine-type backup specifically; a plain file/directory
   backup has no such requirement.

If a machine backup fails with `backup device /dev/sdX: open /dev/sdX: permission denied`, that's usually
just needing your user in the `disk` group for basic read access (`sudo usermod -aG disk $USER`, then log
out/in) — but that alone is NOT enough for a real snapshot; you'll still hit the root requirement above
immediately after.

### File restore — NEW!

File restore is possible by using the nbd tool.

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

This fork ships a patched Clonezilla Live ISO with a fully automated **"PBS Bare Metal Restore"** boot menu entry — see [Clonezilla live ISO (bare-metal restore)](#clonezilla-live-iso-bare-metal-restore) below. It's a separate, first-position, default-on-timeout entry that skips Clonezilla's own language/keyboard prompts entirely: enter your PBS connection details, pick a snapshot, confirm the auto-detected target disk, and it handles network setup, NBD attach, the restore itself and an automatic reboot with no further input.

The older, semi-manual route also still ships on the same ISO, as its own entry in Clonezilla's normal mode-selection menu ("pbs-nbd"): it walks you through PBS connection details and attaches the chosen snapshot to `/dev/nbd0` in the background, then leaves you to drive the actual restore through Clonezilla's own standard disk-restore wizard by hand.

## Usage — Directory Backup

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
- `.Error`: error message if any
- `.Hostname`: hostname of the machine
- `.StartTime`: time the backup started
- `.EndTime`: time the backup ended
- `.Duration`: duration of the backup
- `.FromattedDuration`: formatted duration of the backup
- `.Success`: a boolean telling whether the backup was successful
- `.Status`: string representation of the backup status [SUCCESS, FAILURE]

## Stream Backup

This allows backing up a stream instead of a PXAR, allowing endless possibilities. For example you can invoke:

```
mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]
```

This allows leveraging buzhash for dedup even when using tar for example, or the sql dump itself. And if someone wants to attempt it, it should be possible with some hack to pipe the DISM command to generate a WIM image to this and have a full host backup.

## Known Issues

Windows Defender antimalware being active will slow the backup down up to 25% of attainable speed.

~~There's as of now no mechanism to prevent two instances being launched at the same time which will screw up VSS and backup~~
If you use the Windows planning utility it should theoretically prevent two instances starting at the same time when originating from the same job.

## 📋 Prerequisites

- Windows 10/11 (64-bit)
- Administrator rights (for VSS snapshots)
- Network access to a Proxmox Backup Server
- Linux works too, especially for development

## 🔨 Building from source

### Prerequisites
- Go 1.25 or later
- Node.js 20 or later
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### GUI
```bash
cd gui
npm install --prefix frontend
wails build      # or: wails dev  (hot reload)
```

### Full project (Makefile)
```bash
make install-deps   # install Wails CLI + frontend dependencies
make                # build everything (CLI + GUI + Service)
```

Artifacts are placed in the `dist/` directory. See `make help` for all targets (`cli`, `gui`, `service`, `test`, `lint`, `security-check`, `release`...).

## 🔧 Advanced use & guides

### Multi-PBS (multiple PBS servers)

Configure several PBS servers and pick the target per backup (e.g. `C:\Users` → fast SSD PBS daily, `C:\` → big-data PBS weekly, plus a DR server).

- **[User guide](MULTI_PBS_USER_GUIDE.md)** — adding/testing servers, default server, FAQ and troubleshooting.
- **[Implementation guide](MULTI_PBS_GUIDE.md)** — data model, automatic migration from single-PBS config, backend API methods.

Legacy single-PBS configuration is automatically migrated to a `default` server on first load.

### Clonezilla live ISO (bare-metal restore)

The rescue workflow is built by patching a stock Clonezilla Live ISO with the `pbsnbd` / `machinebackup` binaries plus two menu entries (boots from CD, USB via `dd`, and UEFI):

- **`ocs-pbs-nbd`** — a **"pbs-nbd"** entry added to Clonezilla's own mode-selection menu. Prompts for your PBS connection details and the snapshot to restore, attaches it to `/dev/nbd0` in the background, then hands off to Clonezilla's normal disk-restore wizard for you to drive by hand.
- **`ocs-pbs-bare-metal-restore`** — a new, first-position, default-on-timeout boot menu entry: **"Proxmox Backup Client Go - PBS Bare Metal Restore"**. Skips Clonezilla's language/keyboard prompts and its own generic wizard entirely, and walks straight from PBS credentials to a completed, auto-rebooted restore: DHCP is tried first (falling back to a static-IP prompt only if it fails), NTP sync runs by default, the target disk is auto-detected, and on success the machine shows a completion message and reboots on its own. Works identically for Windows- and Linux-sourced backups (Clonezilla restores at the raw block level, so it's OS-agnostic). Built to its own ISO filename alongside the manual-wizard ISO, so both stay available.

Also fixed on the Clonezilla side, independent of either menu entry: a `zstd.NewReader` deadlock in the PBS chunk-fetch path (merged upstream as [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85); the other eight upstream PRs are listed [above](#-about-this-fork)) and a 100%-reproducible "first restore pass fails with 'no partition', rerun succeeds" bug, root-caused to Clonezilla's own `is_disk_without_part_and_fs()` (in `ocs-functions`) racing against udev rather than any NBD-attach timing.

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

Full details (why a full ISO rebuild instead of an in-place swap, prerequisites, menu flow, verification) in **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**.

### Building the Windows GUI

**Docker (recommended, especially when building on Linux).** The one-command script builds a `ProxmoxBackupClientGO.exe` with proper WebView2 support, using a disposable `golang` container (installs mingw + Wails, builds the frontend, runs `wails build`):

```bash
./build_gui_windows_docker.sh
```

**Native Windows (Chocolatey).** See **[BUILD.md](BUILD.md)** for the complete Windows toolchain setup:

```powershell
choco install go
choco install mingw
# then, in a non-elevated shell:
build.bat          # GUI
build_cli.bat      # CLI
```

### Feature status, changelog & internal docs

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)** — per-feature status matrix (implemented / tested / roadmap).
- **[CHANGELOG.md](CHANGELOG.md)** — per-version change history.
- **[TODO.md](TODO.md)** — open roadmap and ideas.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)** — stable product state and available builds.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)** — MSI uninstall dialog (keep/delete configuration) and its test plan.

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

GPLv3 — see the [LICENSE](LICENSE) file.

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
