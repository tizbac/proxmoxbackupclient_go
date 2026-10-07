# Command line tools: user manual

This manual covers the command line tools. It applies to v0.6.0. For the graphical client see [GUI.md](GUI.md); for bare-metal restore see [BMR.md](BMR.md).

## Contents

1. [The tools](#1-the-tools)
2. [Connecting to PBS](#2-connecting-to-pbs)
3. [proxmoxbackup-directory: folder backups](#3-proxmoxbackup-directory-folder-backups)
4. [Incremental backups with -change-detection-mode](#4-incremental-backups-with--change-detection-mode)
5. [proxmoxbackup-machine: whole-disk backups](#5-proxmoxbackup-machine-whole-disk-backups)
6. [proxmoxbackup-nbd: attach a disk image (Linux)](#6-proxmoxbackup-nbd-attach-a-disk-image-linux)
7. [Encryption](#7-encryption)
8. [Email reports](#8-email-reports)
9. [Exit codes](#9-exit-codes)
10. [Scheduling](#10-scheduling)
11. [Compatibility with the official Proxmox client](#11-compatibility-with-the-official-proxmox-client)

## 1. The tools

The command line tools are in separate downloads on the [releases page](https://github.com/mjb-is/proxmoxbackupclient_go/releases):

| Download | Contains |
|---|---|
| `ProxmoxBackupClient-CLI-v0.6.0-windows-amd64.zip` | `proxmoxbackup-directory.exe`, `proxmoxbackup-machine.exe` |
| `ProxmoxBackupClient-CLI-v0.6.0-linux-amd64.tar.gz` | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd` |

| Tool | Purpose |
|---|---|
| `proxmoxbackup-directory` | Backs up folders (or a stream from standard input) as a `host` snapshot |
| `proxmoxbackup-machine` | Backs up whole disks as images, as a `host` or `vm` snapshot |
| `proxmoxbackup-nbd` | Linux only. Attaches a disk image from PBS as a read-only `/dev/nbdN` block device. The bare-metal restore ISO uses it as `pbsnbd` |

There is no command line restore tool for folder backups. Restore them with the GUI, or with the official `proxmox-backup-client` ([section 11](#11-compatibility-with-the-official-proxmox-client)).

The tools are static builds and need no installation. Run the backup tools as administrator (Windows) for VSS, or as root (Linux) for snapshots and disk access.

Only one backup tool runs at a time on a computer. A second one started meanwhile exits with code 2 ([section 9](#9-exit-codes)).

## 2. Connecting to PBS

All three tools take the same connection flags:

| Flag | Meaning |
|---|---|
| `-baseurl` | PBS URL with port, for example `https://192.168.1.10:8007` |
| `-certfingerprint` | The server certificate's SHA-256 fingerprint, `aa:bb:...` |
| `-authid` | API token ID, for example `backup@pbs!laptop` |
| `-secret` | API token secret |
| `-datastore` | Datastore name |
| `-namespace` | Namespace (optional) |

Instead of a token you can log in with a PBS user. The backup tools use `-pbsusername` and `-pbspassword`; `proxmoxbackup-nbd` uses `-username` and `-password`. A user login takes priority over a token. If you give a user name without a password, the backup tools ask for it on the console.

**Fingerprint.** Without `-certfingerprint` the backup tools fetch the fingerprint the server presents, print it and ask `Trust and pin this fingerprint for this run? [y/N]`. Always pass `-certfingerprint` in scripts and scheduled tasks, since nobody is there to answer. Read it from the PBS dashboard (Show Fingerprint) or with `proxmox-backup-manager cert info` on the PBS host.

**Environment variables.** The tools also read the official client's variables. A flag wins over the environment.

| Variable | Sets |
|---|---|
| `PBS_REPOSITORY` | `[[auth-id@]server[:port]:]datastore`, as in the official client. Takes priority over the next four |
| `PBS_SERVER`, `PBS_PORT` | Server and port (default port 8007) |
| `PBS_DATASTORE` | Datastore |
| `PBS_AUTH_ID` | Token ID |
| `PBS_PASSWORD` | Token secret |
| `PBS_FINGERPRINT` | Certificate fingerprint |

**JSON config file.** `proxmoxbackup-directory` and `proxmoxbackup-machine` take `-config <file>`. Flags given on the command line override values from the file. See [`config.json.example`](../../config.json.example):

```json
{
  "baseurl": "https://pbs.example.com:8007",
  "certfingerprint": "AA:BB:...",
  "authid": "backup@pbs!laptop",
  "secret": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "datastore": "backup",
  "namespace": "",
  "backup-id": "laptop-documents",
  "backupdir": "C:\\Users\\alice\\Documents",
  "change-detection-mode": "metadata",
  "keyfile": "",
  "keyfilepassphrase": ""
}
```

A config file keeps the secret off the command line and out of the process list. Restrict who can read it.

## 3. proxmoxbackup-directory: folder backups

### Flags

| Flag | Default | Meaning |
|---|---|---|
| `-backupdir <path>` | | Folder to back up. Must not be a symlink. Repeat the flag for several folders ([below](#several-folders)) |
| `-backup-id <id>` | computer name | Backup ID. Snapshots with the same ID form one group |
| `-change-detection-mode <mode>` | `legacy` | `legacy`, `data` or `metadata` ([section 4](#4-incremental-backups-with--change-detection-mode)) |
| `-novss` | VSS on | Do not snapshot the source first. Needed for file systems without VSS (VeraCrypt volumes, some network shares) and on Linux unless you run as root with `elastio-snap` or `dattobd` loaded |
| `-keyfile <file>` | | Encrypt with this PBS key file ([section 7](#7-encryption)) |
| `-keyfile-passphrase <text>` | asked | Passphrase for a protected key |
| `-backupstream <name>` | | Back up standard input as `<name>.didx` instead of a folder |
| `-pxarout <file>` | | Also write the archive to a local file (debugging). Single folder only |
| `-pbsusername`, `-pbspassword` | | User login instead of a token |
| `-config <file>` | | JSON config file |
| `-mail-...` | | Email report ([section 8](#8-email-reports)) |

Plus the connection flags in [section 2](#2-connecting-to-pbs). JSON names: `backupdir` (one folder), `backupdirs` (a list), `backup-id`, `change-detection-mode`, `usevss`, `keyfile`, `keyfilepassphrase`, `backupstreamname`, `pxarout`, `pbs-username`, `pbs-password`, `smtp`.

### Examples

Windows, one folder, API token:

```powershell
proxmoxbackup-directory.exe -baseurl https://pbs.example.com:8007 -certfingerprint AA:BB:... `
  -authid "backup@pbs!laptop" -secret "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" `
  -datastore backup -backup-id laptop-documents -backupdir "C:\Users\alice\Documents"
```

Linux, as a normal user (no snapshot):

```bash
proxmoxbackup-directory -baseurl https://pbs.example.com:8007 -certfingerprint AA:BB:... \
  -authid 'backup@pbs!server1' -secret 'xxxxxxxx-...' \
  -datastore backup -backup-id server1-home -backupdir /home -novss
```

A database dump as a stream:

```bash
mysqldump mydb | proxmoxbackup-directory -backupstream mydb.sql -backup-id db1 [connection flags]
```

### Several folders

Repeat `-backupdir`. Each folder becomes its own backup group, with the ID `<base>_<path>`, where `<base>` is `-backup-id` (or the computer name) and `<path>` is the folder path with `\`, `/`, `:` and spaces turned into safe characters. For example `-backup-id laptop -backupdir C:\Data -backupdir "D:\My Photos"` gives `laptop_C_Data` and `laptop_D_My-Photos`. One folder failing does not stop the others; the exit code reports the failure.

### What gets skipped

These names are always skipped, at any depth: the folders `System Volume Information`, `$RECYCLE.BIN` and `Recovery`, and the files `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` and `DumpStack.log.tmp`.

### Unreadable files and disk drop-outs

If a source disk drops out mid-backup ("device not ready"), the tool waits up to 2 minutes for it to come back, reopens the file and carries on. A file that still cannot be read is filled with zeros to its recorded size and flagged; the snapshot is kept and the tool exits with code 3. Other per-file read errors (a locked region, for example) are handled the same way.

## 4. Incremental backups with -change-detection-mode

| Mode | Reads | Stored as |
|---|---|---|
| `legacy` (default) | every file | `backup.pxar.didx` |
| `data` | every file | split archive `backup.mpxar.didx` + `backup.ppxar.didx` |
| `metadata` | only new and changed files | the same split archive |

In `metadata` mode the tool reads the file list of the newest snapshot in the group. A file whose size, modification time (to the nanosecond), mode and owner match is not opened: its data is reused from that snapshot's chunks. Only new and changed files are read and uploaded, so a repeat run of a large, mostly unchanged folder takes minutes instead of hours and sends very little to PBS. With no usable previous split snapshot (the first run, or a different key), the run reads every file, like `data`.

The output reports the result in this form:

```
Metadata change detection: comparing with snapshot <time> (<N> files)
Metadata change detection: <N> unchanged files (<bytes> bytes) reused without reading, <N> chunks reused, <bytes> bytes padding; <N> unchanged files read anyway to limit padding
```

Points to know:

* **Give each job its own `-backup-id`.** Metadata mode compares with the newest snapshot in the group, whichever job made it.
* **Run a full read now and then.** A change that keeps the size and modification time is not noticed. The GUI's "read every file every N runs" has no CLI flag; schedule a second task that runs the same command with `-change-detection-mode data`, for example weekly. Later `metadata` runs compare with that snapshot as normal.
* Reuse only happens from a snapshot encrypted with the same key (or both unencrypted).
* Restoring split archives needs v0.6.0 of this client (or the official client). Update every machine you restore from before switching.

## 5. proxmoxbackup-machine: whole-disk backups

### Flags

| Flag | Default | Meaning |
|---|---|---|
| `-backupdev <device>` | | Disk to back up. Repeat for several disks. Windows: `\\.\PhysicalDrive0`. Linux: `/dev/sda`, `/dev/nvme0n1` |
| `-type <host\|vm>` | `host` | `vm` stores a Proxmox VE VM snapshot (numeric `-backup-id`), which Proxmox VE can restore as a VM |
| `-backup-id <id>` | computer name | Backup ID. Must be numeric with `-type vm` |
| `-keyfile`, `-keyfile-passphrase` | | Encryption ([section 7](#7-encryption)) |
| `-systray` | off | Show a tray icon while running. Can cause problems when nobody is logged in |
| `-pbsusername`, `-pbspassword` | | User login instead of a token |
| `-config <file>` | | JSON config file (`backupdev` is a list, the type is `backuptype`) |
| `-mail-...` | | Accepted but not sent in v0.6.0 ([section 8](#8-email-reports)) |

Plus the connection flags in [section 2](#2-connecting-to-pbs).

On Windows the tool takes a VSS snapshot of every mounted partition on the disk, then stores a bootable image of the whole disk. On Linux mounted partitions are snapshotted with `elastio-snap` or `dattobd`, which must be loaded, and the tool must run as root. See "Linux machine backup prerequisites" in the [README](../../README.md#linux-machine-backup-prerequisites-vss-equivalent).

Later backups of the same disk to the same group only upload changed 4 MiB chunks.

### Examples

```powershell
proxmoxbackup-machine.exe -baseurl https://pbs.example.com:8007 -certfingerprint AA:BB:... `
  -authid "backup@pbs!laptop" -secret "..." -datastore backup `
  -backupdev \\.\PhysicalDrive0 -backup-id laptop-disk0
```

```bash
sudo proxmoxbackup-machine -baseurl https://pbs.example.com:8007 -certfingerprint AA:BB:... \
  -authid 'backup@pbs!server1' -secret '...' -datastore backup \
  -backupdev /dev/sda -type vm -backup-id 9000107
```

Restore a machine backup with the bare-metal restore ISO ([BMR.md](BMR.md)), or, for a `vm` snapshot, from Proxmox VE.

## 6. proxmoxbackup-nbd: attach a disk image (Linux)

`proxmoxbackup-nbd` attaches a disk image (`.img.fidx`) from any PBS snapshot, including Proxmox VE VM backups, as a read-only `/dev/nbdN` device. You can then mount its partitions to copy files out, or image it onto a disk.

| Flag | Default | Meaning |
|---|---|---|
| `-path <type/id/time/file>` | | The image to attach, for example `vm/107/2026-03-01T00:07:00Z/drive-scsi0.img.fidx`. A trailing `#comment` (as printed by `-list`) is ignored |
| `-list` | | Print every image as `type/id/time/file[#comment]` and exit. Needs no root |
| `-nbd <n>` | `0` | Use `/dev/nbd<n>` |
| `-username`, `-password` | | User login instead of a token |
| `-keyfile`, `-keyfile-passphrase` | | Key for an encrypted snapshot |
| `-help` | | Show the flags |

Plus `-baseurl`, `-certfingerprint`, `-authid`, `-secret`, `-datastore` and `-namespace`. Without `-path` (and without `-list`) a text menu asks for the connection details and lets you pick an image.

It must run as root. Load the module first with partition probing off; probing on an NBD device can loop:

```bash
sudo modprobe nbd max_part=0
proxmoxbackup-nbd -baseurl https://pbs.example.com:8007 -certfingerprint AA:BB:... \
  -username backup@pbs -password '...' -datastore backup -list
sudo proxmoxbackup-nbd -baseurl https://pbs.example.com:8007 -certfingerprint AA:BB:... \
  -username backup@pbs -password '...' -datastore backup \
  -path "host/laptop-disk0/2026-10-07T07:15:32Z/drive-sata0.img.fidx"
```

The tool stays in the foreground while the device is attached. Press Ctrl+C to detach. Unmount everything on the device first, or the partitions stay busy. If you see `Device or resource busy`, run `nbd-client -d /dev/nbd0` or reboot.

With `-keyfile`, the tool reads the first chunk before it attaches the device, so a missing or wrong key stops straight away with a clear message.

Do not use this on a machine running important services: a damaged file system on the attached image can upset the kernel.

## 7. Encryption

`-keyfile <file>` encrypts every chunk with AES-256-GCM and signs the manifest. The key file format is the official one: a key made with `proxmox-backup-client key create` works here, and a key made by this client (GUI **Generate...**) works with the official client.

For a passphrase-protected key (scrypt or PBKDF2), pass `-keyfile-passphrase`, or leave it out to be asked on the console. Unattended runs need the passphrase in the command or config file.

Without the key an encrypted snapshot cannot be restored. Keep a copy away from the computer.

## 8. Email reports

`proxmoxbackup-directory` can email a report at the end of every run. (`proxmoxbackup-machine` accepts the same flags and checks them, but does not send email in v0.6.0.)

| Flag | Meaning |
|---|---|
| `-mail-host`, `-mail-port` | SMTP server and port |
| `-mail-username`, `-mail-password` | SMTP login |
| `-mail-insecure` | Allow an unencrypted connection |
| `-mail-from`, `-mail-to` | Sender and recipient (the config file's `smtp.mails` list allows several pairs) |
| `-mail-subject-template`, `-mail-body-template` | Go [text/template](https://pkg.go.dev/text/template) templates |

Template fields include `.Success`, `.Status`, `.NewChunks`, `.ReusedChunks`, `.Datastore`, `.Hostname`, `.StartTime`, `.EndTime`, `.Duration`, `.FromattedDuration` (spelt like that), `.ErrorStr`, `.Partial` and `.ReadErrorCount`. The default subject is `Backup {{.Status}}`.

## 9. Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Failed: bad options, connection or login failure, key problem or backup error. For `proxmoxbackup-directory` also: the report email could not be sent |
| 2 | Another backup job is already running on this computer |
| 3 | `proxmoxbackup-directory` only: the snapshot was saved, but some files could not be read and are incomplete |

On Windows, two cases show a message box instead of only printing: missing required options (the box lists the flags) and "another job is running". A message box waits for someone to click it, so test the command by hand before scheduling it, and do not let two scheduled backups overlap.

## 10. Scheduling

### Windows Task Scheduler

1. Put the settings in a JSON file, for example `C:\ProgramData\pbs\documents.json`, and restrict it to Administrators and SYSTEM.
2. Create a task: **Run whether user is logged on or not**, **Run with highest privileges** (for VSS), and a trigger.
3. Action: program `C:\Tools\proxmoxbackup-directory.exe`, arguments `-config C:\ProgramData\pbs\documents.json`.
4. The task's "Last Run Result" shows the exit code ([section 9](#9-exit-codes)). Use `0x3` as a warning, anything else non-zero as a failure.

Or from an elevated prompt:

```powershell
schtasks /Create /TN "PBS Documents" /SC DAILY /ST 02:00 /RU SYSTEM /RL HIGHEST `
  /TR "C:\Tools\proxmoxbackup-directory.exe -config C:\ProgramData\pbs\documents.json"
```

### Linux cron

```cron
# /etc/cron.d/pbs-backup
# Daily metadata run at 02:00, full read every Sunday at 03:00
0 2 * * 1-6 root /usr/local/bin/proxmoxbackup-directory -config /etc/pbs/home.json >> /var/log/pbs-home.log 2>&1
0 3 * * 0   root /usr/local/bin/proxmoxbackup-directory -config /etc/pbs/home.json -change-detection-mode data >> /var/log/pbs-home.log 2>&1
```

`/etc/pbs/home.json` sets `"change-detection-mode": "metadata"`; the Sunday line overrides it with a flag. A systemd timer works equally well.

## 11. Compatibility with the official Proxmox client

This client and the official `proxmox-backup-client` read each other's backups. Tested with `proxmox-backup-client` 3.4.9 and PBS 4.2:

* `legacy`, `data` and `metadata` snapshots made by this client (command line and GUI, Windows and Linux) restore byte-identically with the official client, and PBS 4.2's file browser opens them. On Linux, mode, nanosecond modification time and symlink targets come back exactly too.
* `legacy`, `data` and `metadata` snapshots made by the official client restore byte-identically with this client.
* Both use the same split archive format (pxar version 2) and the same key file format.
* The modes have the same names and meaning as the official `--change-detection-mode`, and the same `PBS_*` environment variables are read.

To restore a folder snapshot made by this client with the official tool, list the snapshot's archives, then restore one:

```bash
export PBS_REPOSITORY='backup@pbs!restore@pbs.example.com:backup'
export PBS_PASSWORD='token-secret'
export PBS_FINGERPRINT='AA:BB:...'
proxmox-backup-client snapshot files "host/laptop-documents/2026-10-07T20:45:51Z"
proxmox-backup-client restore "host/laptop-documents/2026-10-07T20:45:51Z" <archive> /tmp/restore
```

Give the archive name without `.didx`: a snapshot from `proxmoxbackup-directory` holds `backup.pxar.didx` (legacy) or `backup.mpxar.didx` plus `backup.ppxar.didx` (data, metadata), so restore `backup.pxar` or `backup.mpxar`. The GUI names one archive per folder (for example `f__data.mpxar` for F:\Data), so check `snapshot files` first. Add `--keyfile` for an encrypted snapshot. The restored tree contains one extra file, `.proxmox_backup_client_meta.json`, which this client adds to record the original path.
