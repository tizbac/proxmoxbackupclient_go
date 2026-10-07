# User manuals

These manuals apply to **v0.6.0** of the [mjb-is fork of Proxmox Backup Client](https://github.com/mjb-is/proxmoxbackupclient_go).

Proxmox Backup Client backs up Windows and Linux computers to a Proxmox Backup Server (PBS). It backs up folders as browsable file archives and whole disks as bootable images, on a schedule or on demand, with optional client-side encryption. Folder backups can use change detection, so a repeat backup of a large, mostly unchanged folder reads and sends only what changed: on a 480.9 GB share of 761,866 files, the first run took 4 h 17 min and each later run about 10 minutes. Restores range from a single file, through the GUI, to a whole machine onto bare metal, through a bootable Clonezilla-based ISO.

| Manual | For | Covers |
|---|---|---|
| [GUI.md](GUI.md) | Anyone using the graphical client on Windows or Linux | Install, PBS servers, one-off backups, Backup Sets and schedules, incremental (change detection) backups, the progress card, restore, Reports, logs, Preferences, the Windows service, troubleshooting |
| [CLI.md](CLI.md) | Scripting, servers without a desktop, scheduled tasks | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: every flag, config files, environment variables, encryption, exit codes, Task Scheduler and cron |
| [BMR.md](BMR.md) | Recovering a machine whose disk or system is gone | Making the boot media, the automated restore, the manual pbs-nbd entry, restoring `vm` snapshots onto Proxmox VE, different hardware, troubleshooting |

For a quick decision on which restore to use, see [Restore scenarios](../RESTORE_GUIDE.md).

## Compatible with the official Proxmox client, in both directions

This client and the official `proxmox-backup-client` read each other's backups (verified with `proxmox-backup-client` 3.4.9 and PBS 4.2):

* Legacy, Data and Metadata snapshots made by this client (on Windows or Linux) restore byte-identically with the official client, and PBS's own file browser opens them.
* Legacy, Data and Metadata snapshots made by the official client restore byte-identically with this client.
* Both use the same split archive format (pxar version 2), the same change-detection modes and the same encryption key file format.

So you are not locked in: every backup made with this client can be restored with Proxmox's own tools, and anything the official client makes can be restored with this one.
