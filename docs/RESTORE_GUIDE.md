# Restore scenarios

Which restore to use, and where the steps are. Applies to v0.9.0 (see the [changelog](../CHANGELOG.md)). The steps themselves are in the [GUI manual's Restore chapter](manual/GUI.md#9-restore) and in [BMR.md](manual/BMR.md); this page only points the way.

| Scenario | Method | Steps |
|---|---|---|
| A file or folder deleted or damaged | GUI selective restore: tick just what you need | [GUI.md, Restore](manual/GUI.md#9-restore) |
| Files deleted from a Backup Set's folders, and you do not know exactly which | GUI **Undelete**: lists what is in the backups but gone from disk | [GUI.md, Undelete](manual/GUI.md#10-undelete) |
| A Backup Set's folders should be as they were at a certain backup | GUI **Roll back**: preview first, Undo afterwards | [GUI.md, Roll back](manual/GUI.md#11-roll-back) |
| A whole folder backup back to where it came from | GUI, **Restore to original path** | [GUI.md, Restore](manual/GUI.md#94-choose-where-and-how) |
| Data onto a new or replacement computer | GUI on the new computer, Backup ID field cleared to list every backup, **Restore to alternate path** | [GUI.md, Restore](manual/GUI.md#91-find-the-snapshot) |
| Find a file when you do not know which backup has it | GUI **Search for a file** | [GUI.md, Search](manual/GUI.md#92-search-for-a-file) |
| A few files out of a machine (disk image) backup | GUI Restore: the image's volumes open like folders (NTFS, FAT, exFAT, ext2/3/4, XFS) | [GUI.md, files from a machine backup](manual/GUI.md#96-files-from-a-machine-backup) |
| Ransomware or mass corruption | Any method, from a snapshot taken **before** the problem. Disconnect the machine from the network first | below |
| Dead system disk, same or similar machine | Bare-metal restore ISO | [BMR.md](manual/BMR.md#4-automated-restore-step-by-step) |
| Physical machine into a Proxmox VE VM | `vm` machine backup restored from Proxmox VE | [BMR.md, Proxmox VE](manual/BMR.md#6-restoring-a-vm-snapshot-onto-proxmox-ve) |
| Very different hardware | Fresh OS install, then a GUI restore of the data (from a folder backup, or the files of a machine backup) | [BMR.md, different hardware](manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines) |
| Mount a disk image read-only (Linux, or from the ISO) | `pbs-nbd` on the ISO, or `proxmoxbackup-nbd` on Linux, then mount read-only | [BMR.md, pbs-nbd](manual/BMR.md#5-the-manual-pbs-nbd-entry) |
| Undelete or Roll back without the GUI (a script, a server) | `ProxmoxBackupClientSVC -undelete` / `-rollback` | [CLI.md, Undelete and Roll back](manual/CLI.md#12-undelete-and-roll-back-for-a-backup-set) |
| A folder snapshot without this client | The official `proxmox-backup-client restore` (Legacy, Data and Metadata snapshots verified) | [CLI.md, compatibility](manual/CLI.md#11-compatibility-with-the-official-proxmox-client) |

## Ransomware and corruption

1. Disconnect the affected machine from the network.
2. Find the last good snapshot: Reports and the snapshot dates help, and Search can show when a file last looked normal.
3. Prefer restoring to an alternate path first and checking the files, then move them into place. Roll back's preview also shows what changed since a backup, without writing anything.
4. For a system that cannot be trusted any more, restore the system disk with the bare-metal ISO from a snapshot before the infection, then restore data the same way.
5. Change the passwords and PBS tokens the machine used.

## What a restore brings back

From a folder backup:

* File contents, verified against the snapshot if you tick **Verify after restore**.
* Modification times of files and folders, and on Windows creation times (backups from v0.8.0 on).
* With **Restore permissions/ACLs**: on Windows owner, group, permissions (DACL) and the Hidden, System, Archive and ReadOnly attributes (run as administrator so any recorded owner can be set); on Linux POSIX ACLs and extended attributes, plus owner and mode when restoring as root.
* With **Restore alternate data streams and extended attributes** (Windows, on by default): the data attached to files and folders, such as Zone.Identifier, up to 1 MB per stream, and NTFS extended attributes. Folder backups record them from v0.9.0 on.
* Where a file has two names (a hard link), only one is restored.

From a machine backup the files always go to another folder; the GUI manual lists what comes back ([section 9.6](manual/GUI.md#96-files-from-a-machine-backup)).

Restoring permissions onto a network share applies only what the share's file system supports. Restore to a local NTFS disk if permissions matter.
