# Proxmox Backup Client - TODO

What is still open, as of 2026-10-16 (after v0.9.1). Finished work is in [CHANGELOG.md](CHANGELOG.md). The old, much longer list (with its design notes, findings and audit trail) is kept in [docs/TODO-HISTORY.md](docs/TODO-HISTORY.md); every item still open there has been carried over here.

## Release and distribution

- [ ] **Code signing** for the exe files and the MSI (no Authenticode certificate yet, so SmartScreen and Defender warn). Options (checked 2026-10-13): SignPath Foundation (free for open source, signs builds from CI, so needs the release workflow below), a Certum open-source certificate (from about 50 EUR a year), or a Sectigo/DigiCert certificate (roughly 150 to 400 GBP a year, key on a token or cloud HSM). Azure Artifact Signing ($9.99 a month) takes individuals only in the US and Canada. Then check the signature shows in the file properties and a fresh Windows 11 install gives no SmartScreen warning.
- [ ] **Release workflow in CI** (`.github/workflows/release.yml`): build Windows, the Linux packages and the BMR ISO in one run and attest them (build provenance needs GitHub Actions, not a personal machine). Today releases are built and uploaded by hand.
- [ ] **VirusTotal scan** of the release files (manual upload or an API key).
- [ ] **MSI: click through the removal pages** on a desktop: Programs and Features > Change, Remove, keep or delete, Back, Cancel (tests 3, 4, 5 and 7 in [MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)). The silent paths passed on 2026-10-13.

## Upstream (tizbac/proxmoxbackupclient_go)

- [ ] **Merge the fork into upstream** (Mick is a collaborator from 2026-10-26): on a short-lived `develop` branch, in stages (engine and CLIs, GUI, then the rename, packaging and docs), each merged to master once tested; small fixes go straight to master. Then retire the fork. #121 to #130 were merged on 2026-10-26; the VSS handle leak (a84e50b) and the junctions fix (8354c96) were pushed to master on 2026-11-01.
- [ ] **#131** (VSS gave no device on Server 2025 26100.1742): fix ready on `pr/vss-no-device-v2`, held until it is tested on a VM installed from the original 26100.1742 release ISO, as SYSTEM.
- [ ] **Neutral default icon** ready on `pr/neutral-default-icon`, to go in with the rename, with the GUI header logo.

## Background service

- [ ] **Live test on Linux: Restart as administrator** (pkexec) from a desktop session, and clicking Install in the root app.
- [ ] Pause backup in the tray: try it by hand with the mouse. Tested on winclient on 2026-10-16 by sending the tray's own messages, for a run of the app and a run of the service: greyed out when idle, Pause and Resume from the tray, the window and the tray following each other, the tooltip, and greyed out again after the run.
- [ ] Maybe: a "resume at" time for a paused backup.

## Backing up

- [ ] Maybe: options per schedule (the monthly one does a full read, or goes to another server). Several schedules per set were built on 2026-10-13; Mick put this off.
- [ ] **Very large folders in split backups** are split one level deep only (`gui/backup_analysis.go`): a 700 GB subfolder becomes one 700 GB part. Recursive splitting down to a size limit was planned; less pressing since change detection reuses unchanged files. Decide whether it is still wanted.
- [ ] **VSS: about 21 handles left behind per snapshot** (Windows; was 33 before 2026-10-21). All in creating and releasing the shadow copy (none in deleting it, in the VSS object itself, or in `vssadmin`), mostly handles to threads that have ended, plus events and semaphores. A service with a daily backup gains about 7,700 a year. Measure with `TestLiveVSSHandles` (`PBS_VSS_VOL`, `PBS_VSS_HANDLES`). Tried without effect: FreeWriterMetadata after the snapshot, and CoInitializeSecurity as Microsoft's vshadow sample sets it (the VSS 8194 "IVssWriterCallback access denied" events on every snapshot stayed too; they may be connected).
- [ ] Maybe: read files that only SYSTEM may read (Windows Defender's folders in Program Files, protected files in System32) with backup semantics (SeBackupPrivilege) when the app runs elevated or as the service, as backup programs do. Today such a set ends as partial (the 2026-10-21 load test: System32 8 files, Program Files 1 folder).
- [ ] Maybe: Reports shows a partial run (some files unreadable, the rest backed up) as failed, next to "Backup completed in ...: N files skipped". A status of its own (warning) would say it better.
- [ ] Offered, not agreed: an empty-block marker in machine backups, skipping the 4 MiB copy for blank chunks in `emitZeros`/`push` (about 3 to 4 minutes off a 27 minute laptop run).
- [ ] **Smaller fixed chunk size for machine backups** (1 MiB or 512 KiB instead of 4 MiB), agreed with tizbac on 2026-11-01 as the way to cut the changed data per backup, rather than dirty-block tracking through the USN journal (too much room for silent corruption). After the merge work. Measured on 2026-11-01 from three Windows VMs' daily backups on the production PBS (200 changed 4 MiB chunks each, old and new decoded and compared): the changed data at 1 MiB is 48 to 68% of the 4 MiB figure, at 512 KiB 35 to 49%, at 256 KiB 26 to 36%, flattening below that (4 KiB 16 to 24%). Cost: 4x or 8x more chunks (index size, GC, verify, upload requests). First test on pbs-test whether PBS takes and verifies a fidx with another chunk size, whether PVE restores it (qmrestore, live restore) and whether file browsing works: the fidx header carries the chunk size, but 4 MiB may be assumed elsewhere (`PBS_FIXED_CHUNK_SIZE` here). Scripts: `leak\dirty_chunks.py`, `leak\subblock_dirty.py` in the 2026-11-01 scratch archive. tizbac's ideas (2026-11-01): a parameter per disk, small chunks for the system disk only, while a disk holding a big shared folder could even use 8 MiB; and warn about scheduled defragmentation (it moves blocks, so they all count as changed) and help turn it off. On an SSD Windows' Optimize Drives only trims, which moves nothing, so the warning is for hard disks. Before building, check PBS and PVE's source for places that assume 4 MiB (he suggests an LLM pass over the code).

## Restoring

- [ ] **Disk images: BitLocker volumes** cannot be browsed (they would need the recovery key).
- [ ] Maybe: an "Other backup..." choice in the Undelete and Roll back set picker (server, backup ID, target folder) for another machine's backups.

## Bare-metal restore ISO

- [ ] Boot-stick exclusion, still to try on real hardware: a real USB SSD. Done: the ISO as a fixed SATA disk on a UEFI VM (2026-10-13), a real Ventoy stick with UEFI and BIOS, in Ventoy's normal and GRUB2 modes (2026-10-15, v0.9.1 ISO), and the ISO written raw to a USB disk reporting itself non-removable on a VM with two internal disks, UEFI and BIOS (2026-10-21). Check the key-file step lists the Ventoy data partition (exFAT) so the key can live on the same stick.
- [ ] Restore and start a Windows machine with the next release ISO, as was done for a Debian 13 machine on 2026-10-21 (UEFI and BIOS, same size and larger disk, test ISO `-test-20261021`).

## Security

- [ ] Paper key printing from the Linux app run as root (`proxmoxbackupclient-root`): untested (needs the password for pkexec). As the normal user it printed (2026-10-18).
- [ ] Key import with a real webcam: tested only with a camera feed played to WebView2 (Windows) and WebKit's test camera (Linux), as the test machines have none. Also untested: the camera in the Linux app run as root, and building the Linux app against webkit2gtk-4.1 (`-tags webkit2_41`, the .deb's choice where its headers exist), as linuxclient only has the 4.0 headers; the camera code uses the same pkg-config lines as Wails.
- [ ] Restore of an encrypted snapshot from the Proxmox VE storage configuration (key given to PVE): untested.

## Languages and docs

- [ ] Native speakers to look over the app's strings in the languages its users actually use. All 17 were reviewed against the English on 2026-10-13 (meaning, one term per concept, menu names, plurals, quotation marks); a native ear still catches what is correct but unidiomatic. The feature lists on the About page (`featuresList`, `techList`) were not part of that review, nor were the Backup Set More menu's new labels (37df34d, 2026-10-15).
- [ ] The 12 translated READMEs were regenerated from the English one on 2026-10-13 (condensed, machine-translated): keep them in step when the English README changes (in step again on 2026-10-19: the v0.9.1 paragraph, `-no-mtime` and Import... added), and have native speakers look them over.

## Command-line tools


## Tests

- [ ] Run the Docker end-to-end suite (`testing/e2e`) in CI: `e2e.yml` runs on pushes and pull requests to `main`/`develop` (or by hand), not this repository's `master`, so it has never run here; `build-and-release.yml` has the same branches (part of the CI work).

## Parked

Kept, not being worked on (Mick, 2026-10-11).
- **App froze** (high CPU, unresponsive) after a machine backup on rigel, 26/27 September. Never seen again. Notes in the history file ("UNRESOLVED: app froze").
- **BMR wizard language choice**: the wizard is English only by design (it skips Clonezilla's language prompt for speed); a picker would need a full string table for our own prompts.
- **Speed limit per Backup Set**: a cap slows every run even when the link is free; revisit only for a real case such as a slow site link. Pause covers "the server is busy now".
- **A separate bare-metal restore CLI** (`cmd/proxmoxbackupclient-restore` for a Linux live USB): the BMR ISO's wizard covers bare-metal restore; build it only if a no-GUI rescue path is wanted.
- **Retention per Backup Set in the client**: retention is the server's job (the datastore prune job, now with `keep-hourly`), the usual backup token (DatastoreBackup) cannot prune, and a client prune would fight the datastore job unless the set had its own namespace. Parked by Mick, 2026-10-14.
- **Machine backups: skipping free space in partitions without a drive letter** (EFI, recovery), which are read whole. Looked at 2026-10-17: the EFI partition is FAT32, which VSS cannot snapshot; the recovery partition could be snapshotted by its volume GUID, at the cost of one more snapshot set every run, and skipping its free space without a snapshot could zero clusters written during the backup. Together they are about 1 GB, read in seconds and nearly all reused. Not worth the risk.
- **Is deepthought's backup CPU-bound?** Moot since change detection skips unchanged files; reopen only if a full read is too slow.
