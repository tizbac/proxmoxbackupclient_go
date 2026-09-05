# Proxmox Backup Client — Windows client for Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/tizbac/proxmoxbackupclient_go)](https://github.com/tizbac/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-github-orange)](https://github.com/tizbac/proxmoxbackupclient_go)

**Proxmox Backup Client is an open-source (GPL-3.0) backup client for Proxmox Backup Server (PBS), working on Windows and Linux.**

It is a **suite of tools** for backing up to PBS:

- **Proxmox Backup Client GUI** (based on the Nimbus Backup GUI from RDEM Systems) — modern graphical interface for backing up Windows servers and workstations to PBS: consistent VSS snapshots, scheduled jobs, file and disk modes, snapshot browsing/restoration, multi-PBS support and a Windows service mode.
- **`proxmoxbackup-directory`** — command-line tool for directory (PXAR) backups with deduplication.
- **`proxmoxbackup-machine`** — command-line tool for full live Windows machine backups (FIDX, VSS, incremental).
- **`proxmoxbackup-nbd`** — NBD server for restoring disk backups (Linux).

> Keywords: proxmox backup client windows · PBS client · Windows VSS backup · immutable offsite backups · Proxmox Backup Server interface.

> ⚠️ **Disclaimer:** This project is **not affiliated in any way** with **Proxmox Server Solutions GmbH**. "Proxmox", the Proxmox logo and related names are the property of their respective owners; here they are used **only** to state compatibility. See [proxmox.com](https://www.proxmox.com/) for their products.

## 📦 Download

👉 **[Download the latest release](https://github.com/tizbac/proxmoxbackupclient_go/releases)**

> ⚠️ **Windows shows "virus detected" (e.g. `Trojan:Win32/Sabsik.FL.A!ml`) or a SmartScreen warning?**
> This is a **known false positive** for Go/Wails applications — it is *not* a virus. The `!ml` suffix indicates a machine-learning model detection that flags *unsigned and uncommon* executables.
> See [why this happens and how to verify the download](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Verifying any download

Every release provides SHA-256 checksums and a **signed provenance attestation** (cryptographic proof that the binary was produced by this repository's CI, from a precise commit):

```powershell
Get-FileHash .\ProxmoxBackupClient.exe -Algorithm SHA256   # compare with SHA256SUMS.txt
gh attestation verify .\ProxmoxBackupClient.exe --repo tizbac/proxmoxbackupclient_go
```

**VirusTotal — 0 detections.** Independent multi-engine reports of recent MSI installers:
[0.2.108](https://www.virustotal.com/gui/file/6e8fb7ce9af740d470e947addb8daba4331c0b88e8bfdec9e0697ea8f7f29e9e/detection) ·
[0.2.107](https://www.virustotal.com/gui/file/6fd6c6fa77e0305c129ef882a3745100aa6033187a6d52a4af94149ab6b666d2/detection) ·
[0.2.106](https://www.virustotal.com/gui/file/ad6e56700ed9df8e088906e38cee2e2882fc7045f4e39269de0e379a01784ad7/detection)

> ℹ️ **Code signing:** Windows binaries are **not yet Authenticode-signed** (an OSS certificate via [SignPath Foundation](https://signpath.org) is pending). In the meantime, provenance is established through the attestation and checksums above.

## ✨ Features

### GUI — Proxmox Backup Client GUI (recommended)
- **🌍 Multilingual** — French, English, Italian, German and Polish interfaces
- User-friendly configuration with connection test
- Real-time backup progress with throughput and time remaining
- VSS (Volume Shadow Copy) support for consistent backups
- Multi-folder backups, file and disk modes
- Snapshot browsing, file search (wildcards) and restoration
- Multi-PBS server support with certificate fingerprint pinning (TOFU)
- Windows service mode + scheduled backups
- Backup cancel, history (last 6) and rerun
- Debug logging for diagnostics

### CLI tools
- `proxmoxbackup-directory` — directory (PXAR) backups with deduplication
- `proxmoxbackup-machine` — full live Windows machine backups (FIDX, VSS, incremental)
- `proxmoxbackup-nbd` — NBD server for restoring disk backups (Linux)

### 📸 Screenshots

![Server configuration](docs/screenshots/nimbus-gui-liste-servers.png)
*Multi-PBS server management with status indicators*

![Add server form](docs/screenshots/nimbus-gui-add-server-form.png)
*Simple server configuration with connection test*

![One-shot backup](docs/screenshots/nimbus-gui-one-shot-backup.png)
*Real-time backup progress with ETA and throughput*

### Smart system exclusions (file mode)
When backing up an entire drive (e.g. `D:\`), the GUI automatically excludes:

**System folders:** `System Volume Information` (VSS storage, can reach 100+ GB), `$RECYCLE.BIN`, `Recovery`.
**System files:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys`.

**Why this matters:** a drive may show 1.03 TB used while real files are ~141 GB. Without exclusions the backup would include VSS snapshots (wasted space and time); with them, the size matches the real data.

**Recommendation:** use **file mode** (default) with auto-exclusions for file-level backups; use **disk mode** in a separate job for bare-metal restoration (includes everything).

### Security & quality
- Input validation and credential sanitization
- Path traversal prevention
- Retry logic with exponential backoff
- Comprehensive error handling and tests, 100% lint compliance

## 🚀 Quick start (GUI)

1. Download `ProxmoxBackupClient.exe` (or the `.msi`) from the releases
2. Launch it with administrator rights (required for VSS)
3. Configure your PBS connection and test it
4. Select the folders to back up
5. Start the backup

## NEW! — Full machine live backup

New functionality has been added that now allows backing up a complete Windows 10/11 system and their respective server versions without any downtime.

The command syntax is mostly the same, except `-backupdir string`.

In the case of machine backup executable there's in place of backupdir, `-backupdev`.
For example an invocation could be:

`machinebackup.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

The above command will look at Disk 0, detect all mounted partitions, take a VSS snapshot of these, and then create a bootable backup image of the whole disk as FIDX.

The next backup will be incremental, hashing has been parallelized so speeds of 1 GB/sec can be easily reached.

### File restore — NEW!

File restore is possible by using the nbd tool.

In order to use nbd please first do `modprobe nbd max_part=0`.

For unknown reasons, using `max_part != 0` causes an infinite partition probe loop.

The NBD tool will connect any fixed disk backup, regardless of it being a VM or host backup (that being said, it also works for PVE backups).

To use it use a command line similar to this:
`./pbsnbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

If you omit `-path`, a terminal UI will show up allowing you to select the fidx file.

> Beware to not use this on a machine running important stuff (a corrupt filesystem can crash the OS potentially, that's why Proxmox VE uses a QEMU instance for this).

Also be very sure to have unmounted anything on the nbd disk before stopping pbsnbd, if not you will likely end up with a busy unmountable partition. If someone has an indication of how to recover from that, please tell me.

If you get a `Device or resource busy` error, you have to force disconnect by running `nbd-client -d /dev/nbd0` or simply reboot.

### Restore to physical machine

A live CD / PXE boot system will be released that will allow logging in to a PBS server, selecting the backup, and launching clonezilla. For now the best way is spinning up a clonezilla live and copying to it the nbd server executable; before proceeding with clonezilla, on another tty, you launch `pbsnbd`.

I suggest also copying over command line parameters such as authid, baseurl, fingerprint etc, they are a pain in the... to hand type!

Once pbsnbd is up and running, you can use the clonezilla disk-to-local-disk option.

## Usage — Directory Backup

A typical command would look like:

```shell
proxmoxbackupgo.exe -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -backupdir "C:\path\to\backup" -datastore "datastorename"
```

```
proxmoxbackupgo.exe
  -authid string
        Authentication ID (PBS Api token)
  -secret string
        Secret for authentication
  -backupdir string
        Backup source directory, must not be symlink
  -baseurl string
        Base URL for the proxmox backup server, example: https://192.168.1.10:8007
  -certfingerprint string
        Certificate fingerprint for SSL connection, example: ea:7d:06:f9...
  -datastore string
        Datastore name
  -namespace string
        Namespace (optional)
  -backup-id string
        Backup ID (optional - if not specified, the hostname is used as the default for host-type backups)
  -pxarout string
        Output PXAR archive for debug purposes (optional)
  -backupstream string  ***NEW***
        Filename for stream backup
  -mail-host string
        mail notification system: mail server host(optional)
  -mail-port string
        mail notification system: mail server port(optional)
  -mail-username string
        mail notification system: mail server username(optional)
  -mail-password string
        mail notification system: mail server password(optional)
  -mail-insecure bool
        mail notification system: allow insecure communications(optional)
  -mail-from string
        mail notification system: sender mail(optional)
  -mail-to string
        mail notification system: receiver mail(optional)

  -mail-subject-template string
        mail notification system: mail subject template(optional)
  -mail-body-template string
        mail notification system: mail body template(optional)

  -config string
        Path to JSON config file. If this flag is provided all the others will override the loaded config file
```

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
mysqldump yourdatabase | ./proxmoxbackupgo -backupstream yourdatabase.sql [other options]
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
- Go 1.22 or later
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

The rescue workflow is built by patching a stock Clonezilla Live ISO with the `pbsnbd` / `machinebackup` binaries plus a **pbs-nbd** entry in the Clonezilla main menu (boots from CD, USB via `dd`, and UEFI):

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd
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
- **[FIXES_SUMMARY.md](FIXES_SUMMARY.md)** — GUI fix notes (directory vs machine-backup mode switching).

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

1. Encryption support (still missing)
2. Physical-to-virtual (P2V) migration, restoring a bare-metal backup into a virtual machine (still incomplete)
3. Async upload / multicore upload of chunks (multicore compression is already implemented for machine backup)
4. Proxmox side patch to add another kind of entry to pxar format with Windows security descriptors in it
5. Support for Windows symlinks
6. Anything interesting you can come up with :)

## About Proxmox Backup Client GO contributors

Proxmox Backup Client GO contributors develop and maintain this project. The software relies on the NTP/NTS infrastructure and the [11 public NTS servers](https://github.com/jauderho/nts-servers) listed in the community reference.

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**