# Proxmox Backup Client: roadmap

What is still open after 2.0. Finished work is in [CHANGELOG.md](CHANGELOG.md).

## Release and distribution

- [ ] **Authenticode signing** of the programs and the MSIs, through SignPath Foundation (free for open source, signs builds from CI). Pending; until then SmartScreen and Defender may warn, and the checksums and the build attestation are the proof of origin. Once signed, check that the signature shows in the file properties and that a fresh Windows 11 install gives no SmartScreen warning.
- [ ] **Build the Linux packages and the restore ISO in the release workflow.** Today the release workflows build and attest the MSIs and the command-line archives; the `.deb`, `.rpm`, Arch package and ISO are built with `make deb`, `make pkgfedora`, `make pkgarch` and `make clonezilla` (or the separate ISO workflow).
- [ ] **Rename the unbranded client** to a name without the Proxmox mark (see the trademark notice in the README), with its own icon and header logo.
- [ ] **One set of names for the command-line tools.** `make cli`, the release's `proxmoxbackup-cli-*` downloads and the manuals use `proxmoxbackup-directory`, `proxmoxbackup-machine` and `proxmoxbackup-nbd`, but `.goreleaser.yaml` (the `proxmoxbackupclient_go_*` archives) still builds `pbsdirectorybackup`, `pbsmachinebackup`, `directorybackup` and `pbsnbd`, so a release ships the same tools under two sets of names. Keep one; if the old names stay for existing scripts, say so in the CLI manual.
- [ ] Maybe: **one command-line tool with subcommands**, `proxmoxbackup directory`, `proxmoxbackup machine` and `proxmoxbackup nbd`, in place of three programs. The old names keep working as links to it, which choose the subcommand from the name they were started by (as BusyBox does), and without a subcommand the kind of backup follows from the options: `-backupdir` (or `-backupstream`) for folders, `-backupdev` for disks, from the command line or the config file. Later, one run could back up both.
- [ ] **MSI removal pages**: click through Programs and Features > Change, then Remove, keep or delete the settings, Back and Cancel, on a desktop (tests 3, 4, 5 and 7 in [MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)). The silent paths pass.

## Background service

- [ ] **The service could also run each user's own Backup Sets.** Sets made with **Use my own settings** run only while that user's app is open or in the tray; the service could run them too, so that they also run when the app is closed or the user is signed out.
- [ ] **macOS has no check of who answers on the service's port.** On Windows and Linux the app checks that the program on `127.0.0.1:18765` is the service before it sends anything; on macOS it does not yet.
- [ ] Test **Restart as administrator** (pkexec) from a Linux desktop session, and **Install** in the app run as root.
- [ ] Test **Pause backup** in the Windows tray by hand with the mouse, for a run of the app and one of the service.
- [ ] Maybe: a "resume at" time for a paused backup.

## Backing up

- [ ] **Linux ACLs inside the archive, as the official client does.** pxar has entries for POSIX ACLs, extended attributes and file capabilities, and the official client writes them. This client defines them (`pbscommon/pxar.go`) but has never written them, and its reader skips them, so on Linux the official client restores this client's backups without their ACLs, and this client restores the official client's without theirs. Plan: a Backup Options choice, **Switch to the new Linux ACL method** or **Retain the legacy Linux ACL method**. The new method writes the entries (also in the metadata archive of split backups) and the reader applies them; older snapshots keep restoring from `proxmox-client-acls.json.gz.blob`. Each snapshot is complete on its own, so nothing is uploaded again, but the first run after switching reads every file, since Metadata change detection sees each file's metadata as changed; the option says so. Measure that run in testing. Windows keeps the blob, as pxar has no place for NTFS permissions.
- [ ] **Smaller fixed chunks for machine backups** (1 MiB or 512 KiB instead of 4 MiB), to cut the data each backup sends for a changed block, rather than tracking dirty blocks through the USN journal. Measured on daily backups of three Windows VMs: the changed data at 1 MiB is 48 to 68% of the 4 MiB figure, at 512 KiB 35 to 49%. The cost is 4 to 8 times more chunks (index size, garbage collection, verify, upload requests). First check that PBS takes and verifies a fidx with another chunk size, that Proxmox VE restores it (also live restore) and that file browsing works: the fidx header carries the chunk size, but 4 MiB may be assumed elsewhere (`PBS_FIXED_CHUNK_SIZE` here). Ideas: a chunk size per disk (small for the system disk, larger for a disk of big shared files), and a warning about scheduled defragmentation of hard disks, which moves blocks so they all count as changed.
- [ ] **VSS leaves about 21 handles behind per snapshot** (Windows), all in creating and releasing the shadow copy, mostly handles to threads that have ended. A service with a daily backup gains about 7,700 a year. Measure with `TestLiveVSSHandles` (`PBS_VSS_VOL`, `PBS_VSS_HANDLES`). Tried without effect: FreeWriterMetadata after the snapshot, and CoInitializeSecurity as Microsoft's vshadow sample sets it.
- [ ] **Very large folders in split backups** are split one level deep only (`gui/backup_analysis.go`): a 700 GB subfolder becomes one 700 GB part. Decide whether recursive splitting is still wanted now that change detection reuses unchanged files.
- [ ] **`proxmoxbackup-machine -systray` does nothing**: the tray icon was taken out of the command-line tools (it misbehaved, above all with nobody signed in) and has not come back. Retire the flag (accept it with a note, so scripts keep working), or bring back a Windows tray for runs made with someone signed in.
- [ ] Maybe: read files that only SYSTEM may read (Windows Defender's folders in Program Files, protected files in System32) with backup semantics (SeBackupPrivilege) when the app runs elevated or as the service. Today such a set ends as partial.
- [ ] Maybe: a status of its own (warning) in Reports for a partial run (some files unreadable, the rest backed up), which shows as failed today.
- [ ] Maybe: options per schedule (the monthly schedule does a full read, or goes to another server).
- [ ] Maybe: an empty-block marker in machine backups, skipping the 4 MiB copy for blank chunks (a few minutes off a half-hour laptop run).

## Restoring

- [ ] **Optionally format a left-out partition on restore.** A partition left out of a machine backup comes back empty and unformatted, in its place and at its size; the restore could offer to put a file system on it.
- [ ] **Disk images: BitLocker volumes** cannot be browsed (they would need the recovery key).
- [ ] Maybe: an "Other backup..." choice in the Undelete and Roll back set picker (server, backup ID, target folder) for another machine's backups.

## Bare-metal restore ISO

- [ ] Boot-stick exclusion on a real USB SSD (tested so far: the ISO as a fixed SATA disk, a Ventoy stick with UEFI and BIOS, and a USB disk that reports itself non-removable on a VM). Check that the key-file step lists a Ventoy stick's exFAT data partition, so the key can live on the same stick.
- [ ] Restore and start a Windows machine with the 2.0 ISO, as was done for a Debian 13 machine (UEFI and BIOS, same size and larger disk).

## Security

- [ ] Paper key printing from the Linux app run as root (`proxmoxbackupclient-root`): untested.
- [ ] Key import with a real webcam: tested only with a camera feed played to WebView2 (Windows) and WebKit's test camera (Linux). Also untested: the camera in the Linux app run as root.
- [ ] Restore of an encrypted snapshot through the Proxmox VE storage configuration (key given to Proxmox VE): untested.

## Languages and docs

- [ ] **Kernel 7 and the snapshot modules.** elastio-snap 0.12.11 does not build on kernel 7 (no `strncpy`); the project carries `packaging/linux/elastio-snap/kernel7-strscpy.patch`, applied by `install-elastio-snap.sh`, and has offered it to Axcient as [pull request 138](https://github.com/Axcient/elastio-snap/pull/138). When Axcient merges and releases it: move the script's pinned commit to that release, drop the patch (and the patch step from the script), and update "Linux snapshots: installing elastio-snap or dattobd" in the GUI manual. dattobd 0.12.2 does not build on Fedora 44 at all (kernels 6.19 and 7.2). The Secure Boot steps in the manual were not tried on a machine with Secure Boot on.
- [ ] **The translated READMEs want a native speaker's review.** They are made from the English README.
- [ ] Native speakers to look over the app's strings in the languages its users use. All 17 were reviewed against the English (meaning, one term per concept, menu names, plurals, quotation marks), but a native ear still catches what is correct but unidiomatic. The About page's feature lists were not part of that review.

## Parked

Kept, not being worked on.

- **Bare-metal restore wizard in other languages**: the wizard is English only by design (it skips Clonezilla's language prompt for speed); a picker would need a full string table for its own prompts.
- **Speed limit per Backup Set**: the speed limits in Preferences apply to everything running; a limit per set would slow its runs even when the link is free. Revisit for a real case such as a slow site link.
- **A separate bare-metal restore command-line tool** for a Linux live USB: the restore ISO's wizard covers bare-metal restore; build it only if a rescue path without a GUI is wanted.
- **Retention per Backup Set in the client**: retention is the server's job (the datastore's prune job), the usual backup token cannot prune, and a client prune would fight the datastore's job unless the set had its own namespace.
- **Skipping free space in partitions without a drive letter** (EFI, recovery), which are read whole. The EFI partition is FAT32, which VSS cannot snapshot, and skipping free space without a snapshot could zero clusters written during the backup. Together they are about 1 GB, read in seconds and nearly all reused. Not worth the risk.
