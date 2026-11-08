# User manuals

These manuals apply to version 2.0 of Salvanza for Proxmox Backup Server, formerly Proxmox Backup Client ([salvanza.org](https://salvanza.org/), source and releases on [GitHub](https://github.com/salvanza/salvanza_client); see the [changelog](../../CHANGELOG.md)). The Linux packages install them in `/usr/share/doc/salvanza/manual/`.

Salvanza backs up Windows and Linux computers to a Proxmox Backup Server (PBS). It backs up folders as browsable file archives and whole disks as bootable images, on a schedule or on demand, with optional client-side encryption. Folder backups can use change detection, so a repeat backup of a large, mostly unchanged folder reads and sends only what changed: on a 480.9 GB share of 761,866 files, the first run took 4 h 17 min and each later run about 10 minutes. Restores range from a single file, through the GUI (from folder and machine backups alike), to a whole machine onto bare metal, through a bootable Clonezilla-based ISO. Undelete and Roll back bring back what went missing from a Backup Set's folders, or put the folders back as they were at a chosen backup.

| Manual | For | Covers |
|---|---|---|
| [GUI.md](GUI.md) | Anyone using the graphical client on Windows or Linux | Install (Windows MSI; Linux .deb, .rpm, Arch package or `.tar.gz` archive), PBS servers and encryption keys, one-off backups, Backup Sets and schedules, several backups at once, incremental (change detection) backups, the progress card with Pause and Resume, restore (also single files from machine backups), Undelete, Roll back (also from the command line), Reports, logs, Preferences, the background service on Windows and Linux (also from the command line), troubleshooting |
| [CLI.md](CLI.md) | Scripting, servers without a desktop, scheduled tasks | `salvanza-cli` and its commands `backup-folder`, `backup-disk` and `attach-nbd` (and their earlier names): every flag, config files, environment variables, encryption, email reports, exit codes, Task Scheduler and cron |
| [BMR.md](BMR.md) | Recovering a machine whose disk or system is gone | Making the boot media, the automated restore, the manual pbs-nbd entry, restoring `vm` snapshots onto Proxmox VE, different hardware, troubleshooting |

For a quick decision on which restore to use, see [Restore scenarios](../RESTORE_GUIDE.md).

## Why use this client

The official `proxmox-backup-client` is a command-line tool for Linux. This client works with the same server and the same backup formats, and adds:

* **Windows.** There is no official client for Windows. Here there are a command line tool and an app, folder backups through VSS snapshots, and live whole-disk backups.
* **An app for Windows and Linux**: Backup Sets on schedules, run by a background service so nobody needs to be signed in, Restore with search, Undelete and Roll back, Reports and e-mail.
* **Whole machines**: bootable disk images, single files back from them, and a guided bare-metal restore ISO, onto the same computer, new hardware or a Proxmox VE virtual machine.
* **Windows file details kept**: NTFS permissions, attributes, alternate data streams, extended attributes and creation times are recorded with each folder backup, and this client's Restore puts them back.
* **No lock-in**: see the next section.

## Compatible with the official Proxmox client, in both directions

This client and the official `proxmox-backup-client` read each other's backups (verified with `proxmox-backup-client` 3.4.9 and PBS 4.2):

* Legacy, Data and Metadata snapshots made by this client (on Windows or Linux) restore byte-identically with the official client, and PBS's own file browser opens them.
* Legacy, Data and Metadata snapshots made by the official client restore byte-identically with this client.
* Both use the same split archive format (pxar version 2), the same change-detection modes and the same encryption key file format.

So you are not locked in: every backup made with this client can be restored with Proxmox's own tools, and anything the official client makes can be restored with this one.

Two extras ride alongside the standard archives in a folder snapshot, and the official client restores the files as normal without them: `.proxmox_backup_client_meta.json` (the original path), restored as an ordinary file, and `proxmox-client-acls.json.gz.blob`, the permissions and other file details (on Windows the NTFS permissions, attributes, creation times, alternate data streams and extended attributes; on Linux the extended attributes and POSIX ACLs), which only this client's Restore puts back. On Linux the official client keeps ACLs inside the archive itself, which this client does not yet write or read, so on Linux ACLs do not carry between the two clients; see the [roadmap](../../TODO.md).
