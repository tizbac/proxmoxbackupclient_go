# Proxmox Backup Client - TODO

What is still open, as of 2026-10-16 (after v0.9.1). Finished work is in [CHANGELOG.md](CHANGELOG.md). The old, much longer list (with its design notes, findings and audit trail) is kept in [docs/TODO-HISTORY.md](docs/TODO-HISTORY.md); every item still open there has been carried over here.

## Release and distribution

- [ ] **Code signing** for the exe files and the MSI (no Authenticode certificate yet, so SmartScreen and Defender warn). Options (checked 2026-10-13): SignPath Foundation (free for open source, signs builds from CI, so needs the release workflow below), a Certum open-source certificate (from about 50 EUR a year), or a Sectigo/DigiCert certificate (roughly 150 to 400 GBP a year, key on a token or cloud HSM). Azure Artifact Signing ($9.99 a month) takes individuals only in the US and Canada. Then check the signature shows in the file properties and a fresh Windows 11 install gives no SmartScreen warning.
- [ ] **Release workflow in CI** (`.github/workflows/release.yml`): build Windows, the Linux packages and the BMR ISO in one run and attest them (build provenance needs GitHub Actions, not a personal machine). Today releases are built and uploaded by hand.
- [ ] **VirusTotal scan** of the release files (manual upload or an API key).
- [ ] **MSI: click through the removal pages** on a desktop: Programs and Features > Change, Remove, keep or delete, Back, Cancel (tests 3, 4, 5 and 7 in [MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)). The silent paths passed on 2026-10-13.
- [ ] **Silent MSI install with a ready-made config** for Group Policy or Intune: a `CONFIGFILE` property and a custom action that copies it into `C:\ProgramData\ProxmoxBackupClient`, a template config, and a deployment guide. Not built: `installer/wix` has no such property.

## Upstream (tizbac/proxmoxbackupclient_go)

- [ ] **#113 open**: the bare-metal restore wizard never offers the boot stick (Ventoy, USB SSD) and refuses a disk smaller than the backed up one as the target. Waiting on review. (#102 to #112 are merged.)
- [ ] Candidate port, a fresh small branch off upstream/master (our commits no longer apply as patches): restore permissions when the owner cannot be set, and keep a folder's protected (non-inheriting) DACL (5c1ef56).

## Background service

- [ ] **Live test on Linux: Restart as administrator** (pkexec) from a desktop session, and clicking Install in the root app.
- [ ] Pause backup in the tray: try it by hand with the mouse. Tested on winclient on 2026-10-16 by sending the tray's own messages, for a run of the app and a run of the service: greyed out when idle, Pause and Resume from the tray, the window and the tray following each other, the tooltip, and greyed out again after the run.
- [ ] Maybe: a "resume at" time for a paused backup.

## Backing up

- [ ] Maybe: options per schedule (the monthly one does a full read, or goes to another server). Several schedules per set were built on 2026-10-13; Mick put this off.
- [ ] Maybe: NTFS extended attributes in files restored from a machine (disk image) backup (the image reader does not read `$EA`; folder backups record them since v0.9.0, and a bare-metal restore keeps them).
- [ ] **Very large folders in split backups** are split one level deep only (`gui/backup_analysis.go`): a 700 GB subfolder becomes one 700 GB part. Recursive splitting down to a size limit was planned; less pressing since change detection reuses unchanged files. Decide whether it is still wanted.
- [ ] Live test of a forensic machine set (2026-10-17): the run log says "Forensic copy", "free space not read" stays 0, and a deleted file's data is in the image.
- [ ] Offered, not agreed: an empty-block marker in machine backups, skipping the 4 MiB copy for blank chunks in `emitZeros`/`push` (about 3 to 4 minutes off a 27 minute laptop run).

## Restoring

- [ ] **Disk images: a changed partition layout** since a snapshot (a partition moved) makes that snapshot's folder list fail with a clear error; could fall back to finding the volume by its NTFS serial number.
- [ ] **Disk images: BitLocker volumes** cannot be browsed (they would need the recovery key).
- [ ] **Quick test: Proxmox VE File Restore** on a `vm/9000xxx` snapshot this client wrote, to compare with our own disk-image browsing.
- [ ] Maybe: an "Other backup..." choice in the Undelete and Roll back set picker (server, backup ID, target folder) for another machine's backups.

## Bare-metal restore ISO

- [ ] Boot-stick exclusion, still to try: a dd-written stick, a real USB SSD, and a VM with two internal disks. Done: the ISO as a fixed SATA disk on a UEFI VM (2026-10-13), and a real Ventoy stick with UEFI and BIOS, in Ventoy's normal and GRUB2 modes (2026-10-15, v0.9.1 ISO). Check the key-file step lists the Ventoy data partition (exFAT) so the key can live on the same stick.
- [ ] A full restore with the v0.9.1 ISO onto a target the same size or larger (the smaller-disk refusal was tested; the path that goes ahead was not booted with this ISO).
- [ ] Rebuild the ISO with the wizards that pass the PBS password and key passphrase to `pbsnbd` in the environment (2026-10-16; `pbsnbd` itself tested on linuxclient), boot it, and check `ps` shows neither during a restore with a protected key.

## Security

- [ ] Live test on winclient of the 2026-10-17 security batch with the service installed: an old plain `config.json` converted to `dpapi:` on first start, backups by the app and by the service still log in, a remembered passphrase still unlocks a protected key in a service run, and Print a paper key printed to a real printer (or PDF) and scanned back.
- [ ] Restore of an encrypted snapshot from the Proxmox VE storage configuration (key given to PVE): untested.

## Languages and docs

- [ ] Native speakers to look over the app's strings in the languages its users actually use. All 17 were reviewed against the English on 2026-10-13 (meaning, one term per concept, menu names, plurals, quotation marks); a native ear still catches what is correct but unidiomatic. The feature lists on the About page (`featuresList`, `techList`) were not part of that review, nor were the Backup Set More menu's new labels (37df34d, 2026-10-15).
- [ ] The 12 translated READMEs were regenerated from the English one on 2026-10-13 (condensed, machine-translated): keep them in step when the English README changes (they do not have the v0.9.1 paragraph yet), and have native speakers look them over.

## Command-line tools

- [ ] Live test of the mail-failed note from the app and the service (2026-10-17, unit-tested only): a set with an e-mail on completion and an SMTP server that refuses, then the snapshot's comment on PBS.

## Tests

- [ ] Maybe: symbolic links in the Docker end-to-end fixture (`testing/e2e`), with `assert_tree_identical` comparing link targets.
- [ ] Load test: five Backup Sets at once, check memory stays bounded.

## Parked

Kept, not being worked on (Mick, 2026-10-11).
- **App froze** (high CPU, unresponsive) after a machine backup on rigel, 26/27 September. Never seen again. Notes in the history file ("UNRESOLVED: app froze").
- **BMR wizard language choice**: the wizard is English only by design (it skips Clonezilla's language prompt for speed); a picker would need a full string table for our own prompts.
- **Speed limit per Backup Set**: a cap slows every run even when the link is free; revisit only for a real case such as a slow site link. Pause covers "the server is busy now".
- **A separate bare-metal restore CLI** (`cmd/proxmoxbackupclient-restore` for a Linux live USB): the BMR ISO's wizard covers bare-metal restore; build it only if a no-GUI rescue path is wanted.
- **Retention per Backup Set in the client**: retention is the server's job (the datastore prune job, now with `keep-hourly`), the usual backup token (DatastoreBackup) cannot prune, and a client prune would fight the datastore job unless the set had its own namespace. Parked by Mick, 2026-10-14.
- **Machine backups: skipping free space in partitions without a drive letter** (EFI, recovery), which are read whole. Looked at 2026-10-17: the EFI partition is FAT32, which VSS cannot snapshot; the recovery partition could be snapshotted by its volume GUID, at the cost of one more snapshot set every run, and skipping its free space without a snapshot could zero clusters written during the backup. Together they are about 1 GB, read in seconds and nearly all reused. Not worth the risk.
- **Is deepthought's backup CPU-bound?** Moot since change detection skips unchanged files; reopen only if a full read is too slow.
