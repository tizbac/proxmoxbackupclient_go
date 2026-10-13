# Proxmox Backup Client: feature status (2026-10-13)

## Overview

**Latest release:** v0.9.0 ([releases](https://github.com/mjb-is/proxmoxbackupclient_go/releases)). v0.8.2 was tagged but not published; its change is in v0.9.0.
**Platforms:** Windows 10/11 and Windows Server (GUI, service, CLI); Linux (GUI, service, CLI, .deb and .rpm packages).
**Open work:** [TODO.md](TODO.md).

"Since" is the first version with the feature.

---

## Done

### GUI (Wails v2 + React)
| Feature | Since | Notes |
|---|---|---|
| Menu bar and side menu (Backup, Restore, Undelete, Roll back, Reports, Message Log, Preferences, About) | before 0.5.0 | |
| 18 interface languages | 0.5.0 | Most are machine-translated; see TODO.md |
| Five colour themes plus a custom colour | before 0.5.0 | |
| Reports (backup and restore history) and Message Log | before 0.5.0 | |
| Progress card: forecast finish, files, processing and upload rates, current file, New/Reused chunks | 0.6.0 | Machine backups get disk rows from 0.8.0 |
| In-app confirmation dialogs | 0.7.0 | |
| Settings export and import | 0.5.0 | |

### Folder backups
| Feature | Since | Notes |
|---|---|---|
| Several folders per backup, exclusion patterns, automatic system exclusions | before 0.5.0 | "Recovery" is left out only at the root of a drive (0.9.0) |
| VSS (Windows) and elastio-snap/dattobd snapshots (Linux) | before 0.5.0 | Network shares are read live |
| Change detection: Legacy, Data, Metadata (split pxar, as the official client) | 0.6.0 | Metadata reuses unchanged files without reading them; optional full read every N runs |
| Run now: quick or full read | 0.9.0 | |
| A disk dropping out: wait up to 2 minutes and carry on; unreadable files zero-padded and reported | 0.6.0 | |
| NTFS permissions and attributes (Windows), mode, owner, symlinks, POSIX ACLs and xattrs (Linux) | 0.5.0 | |
| Creation times (Windows) | 0.8.0 | |
| Pause and Resume | 0.8.0 | Also for runs the service does (0.9.0) |
| Auto-split of a large first backup into smaller parts | before 0.5.0 | One level deep only |
| Optional multi-threaded read-ahead | 0.5.0 | Experimental |

### Machine (disk image) backups
| Feature | Since | Notes |
|---|---|---|
| Whole-disk image (fidx), Windows with VSS, Linux with elastio-snap/dattobd | before 0.5.0 | Linux needs root and a loaded elastio-snap or dattobd module, or the backup refuses to run |
| Store as `host/<id>` or as a Proxmox VE `vm/<number>` | 0.5.0 | |
| Skip free space and the paging, hibernation and swap files (Windows) | 0.8.0 | |
| Skip free space (Linux: ext2/3/4 and XFS) and leave out the snapshot store | 0.9.0 | |
| Blank blocks recognised without hashing | 0.8.2 | |
| Bad sectors written as zeros, run reported as partial | 0.8.0 | |
| VSS / Snapshot badge on each Backup Set card | 0.8.0 | Amber "PARTLY" or "N/A" for network folders (0.9.0) |

### Restore
| Feature | Since | Notes |
|---|---|---|
| List snapshots, browse, search (wildcards, regular expression, full path), selective restore | before 0.5.0 | |
| Split (Data/Metadata) archives, including the official client's | 0.6.0 | |
| Permissions and folder times put back; optional verify after restore | 0.5.0 | |
| Files from a machine backup: NTFS | 0.8.0 | Nothing is mounted or downloaded first |
| Files from a machine backup: FAT12/16/32, exFAT, ext2/3/4, XFS; NTFS permissions and alternate data streams | 0.9.0 | BitLocker volumes cannot be browsed |
| Folder backups record alternate data streams (up to 1 MB each) and NTFS extended attributes; a restore puts them back | 0.9.0 | Windows; verified live (classic and metadata change detection) |

### Undelete and Roll back
| Feature | Since | Notes |
|---|---|---|
| Undelete and Roll back for folder Backup Sets, with preview and Undo | 0.7.0 | |
| For Windows machine Backup Sets | 0.8.0 | |
| For Linux machine Backup Sets (ext4, XFS) | 0.9.0 | |
| From the command line: `-list-sets`, `-undelete`, `-rollback` | 0.9.0 | `ProxmoxBackupClientSVC` on Windows, `proxmoxbackupclient` on Linux |

### Backup Sets and scheduling
| Feature | Since | Notes |
|---|---|---|
| Daily, interval and manual triggers, weekdays, run at startup | before 0.5.0 | |
| Several schedules per Backup Set: every N minutes, hours, days, weeks, months (a day, the last day, the Nth weekday) or years, with the next 3 runs | 0.9.0 | Repeats run on the clock; a day a month lacks runs on the nearest earlier day |
| Clone, unique names, suggested Backup ID, Next run line | 0.7.0 | |
| Warning and SHARED ID badge when two sets share a backup group | 0.9.0 | |
| Email notifications and post-backup actions | 0.5.0 | Minimise to tray when finished (0.9.0) |
| Administrator protection for changing Backup Sets (Windows) | 0.6.0 | |

### Background service
| Feature | Since | Notes |
|---|---|---|
| Windows service (`ProxmoxBackupClientSVC`, LocalSystem), Run as Service in Preferences | 0.7.0 | Local API on 127.0.0.1:18765 with a token |
| Linux service (`proxmoxbackupclient.service`, root), shared `/var/lib/proxmoxbackupclient` | 0.9.0 | Restart as administrator through pkexec |
| Progress, Pause, Resume and Stop for runs the service does | 0.9.0 | The same progress card rows as for the app's own runs |
| Settings folder open to administrators only (Windows); 0700 (Linux) | 0.9.0 | |
| Orphaned VSS shadow copies cleared at start | before 0.5.0 | |

### Servers and encryption
| Feature | Since | Notes |
|---|---|---|
| Several PBS servers, a default, a server per Backup Set and per one-off backup | before 0.5.0 | See [MULTI_PBS_USER_GUIDE.md](MULTI_PBS_USER_GUIDE.md) |
| API token or user/password login; certificate fingerprint pinning (trust on first use) | before 0.5.0 | |
| Client-side encryption with a PBS key file, passphrase Ask or Remember | 0.5.0 | Same key format as `proxmox-backup-client` |

### Install and packaging
| Feature | Since | Notes |
|---|---|---|
| Zip (Windows) and tar.gz (Linux) downloads with SHA256SUMS | before 0.5.0 | |
| Windows installer (MSI) with the service, Start menu entry, keep or delete settings on removal | 0.9.0 | |
| .deb and .rpm packages with the service and two menu entries | 0.9.0 | |

### Command line and bare-metal restore
| Feature | Since | Notes |
|---|---|---|
| `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd` | before 0.5.0 | See [docs/manual/CLI.md](docs/manual/CLI.md) |
| Errors on stderr instead of message boxes; `proxmoxbackup-machine` sends its e-mail | 0.9.0 | |
| Clonezilla ISO with an automated PBS Bare Metal Restore entry, encrypted backups included | 0.5.0 | See [docs/manual/BMR.md](docs/manual/BMR.md) |

### Compatibility and tests
| Feature | Since | Notes |
|---|---|---|
| Snapshots restore byte-identically with the official client, both ways (tested with 3.4.9, PBS 4.2) | 0.6.0 | |
| Unit tests per module (`make test`), opt-in live tests, end-to-end tests against PBS in Docker (`make test-e2e`) | | |

---

## Partial or not yet tested

- **Linux Restart as administrator** (pkexec) from a desktop session: not live-tested.
- **Encryption key on machine backups the service runs**: fixed, not tried with a key configured.
- **System tray and toast notifications**: Windows only.

## Not done

- **Code signing** of the exe files and the MSI.
- **Release builds in CI** with build provenance; releases are built by hand.
- **Silent MSI install with a ready-made config** (no `CONFIGFILE` property).
- **Secrets encrypted at rest** (DPAPI or similar): tokens, passwords and remembered passphrases are plain text in `config.json`, inside a folder only administrators can read.
- **Retention (prune) per Backup Set** in the client (PBS prune jobs cover it per datastore or namespace).
- **Bandwidth limiting.**

See [TODO.md](TODO.md) for the full list.

## Dropped

- Machine UUID (the hostname is enough).
- Heartbeat to a remote API (no SaaS backend).
- go-msi (WiX does the job).
