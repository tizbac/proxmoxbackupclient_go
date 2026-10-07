# Restore scenarios

Which restore to use, and where the steps are. Applies to v0.6.0. (This page replaces an early design note, written before restore existed, that described planned features and a `proxmoxbackupclient-restore` tool that was never built.)

| Scenario | Method | Steps |
|---|---|---|
| A file or folder deleted or damaged | GUI selective restore: tick just what you need | [GUI.md, Restore](manual/GUI.md#9-restore) |
| A whole folder backup back to where it came from | GUI, **Restore to original path** | [GUI.md, Restore](manual/GUI.md#94-choose-where-and-how) |
| Data onto a new or replacement computer | GUI on the new computer, Backup ID field cleared to list every backup, **Restore to alternate path** | [GUI.md, Restore](manual/GUI.md#91-find-the-snapshot) |
| Find a file when you do not know which backup has it | GUI **Search for a file** | [GUI.md, Search](manual/GUI.md#92-search-for-a-file) |
| Ransomware or mass corruption | Any method, from a snapshot taken **before** the problem. Disconnect the machine from the network first | below |
| Dead system disk, same or similar machine | Bare-metal restore ISO | [BMR.md](manual/BMR.md#4-automated-restore-step-by-step) |
| Physical machine into a Proxmox VE VM | `vm` machine backup restored from Proxmox VE | [BMR.md, Proxmox VE](manual/BMR.md#6-restoring-a-vm-snapshot-onto-proxmox-ve) |
| Very different hardware | Fresh OS install, then a GUI folder restore of the data | [BMR.md, different hardware](manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines) |
| A few files out of a disk image | `pbs-nbd` on the ISO, or `proxmoxbackup-nbd` on Linux, then mount read-only | [BMR.md, pbs-nbd](manual/BMR.md#5-the-manual-pbs-nbd-entry) |
| A folder snapshot without this client | The official `proxmox-backup-client restore` (Data and Metadata snapshots verified) | [CLI.md, compatibility](manual/CLI.md#11-compatibility-with-the-official-proxmox-client) |

## Ransomware and corruption

1. Disconnect the affected machine from the network.
2. Find the last good snapshot: Reports and the snapshot dates help, and Search can show when a file last looked normal.
3. Prefer restoring to an alternate path first and checking the files, then move them into place.
4. For a system that cannot be trusted any more, restore the system disk with the bare-metal ISO from a snapshot before the infection, then restore data the same way.
5. Change the passwords and PBS tokens the machine used.

## What a folder restore brings back

* File contents, verified against the snapshot if you tick **Verify after restore**.
* Modification times of files and folders.
* Windows: owner, group, permissions (DACL) and the Hidden, System, Archive and ReadOnly attributes. Linux: POSIX ACLs and extended attributes, plus owner and mode when restoring as root.
* Not yet: Alternate Data Streams and legacy NTFS extended attributes. Where a file has two names (a hard link), only one is restored.

Restoring permissions onto a network share applies only what the share's file system supports. Restore to a local NTFS disk if permissions matter.
