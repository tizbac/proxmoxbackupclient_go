# Proxmox Backup Client - TODO

What is still open, as of 2026-10-13. Finished work is in [CHANGELOG.md](CHANGELOG.md). The old, much longer list (with its design notes, findings and audit trail) is kept in [docs/TODO-HISTORY.md](docs/TODO-HISTORY.md); every item still open there has been carried over here.

## Release and distribution

- [ ] **Publish v0.9.1.** Built on 2026-10-15 (tag v0.9.1 local only): push master and the tag, publish the GitHub release with the assets and notes, then deploy to the machines.
- [ ] **Code signing** for the exe files and the MSI (no Authenticode certificate yet, so SmartScreen and Defender warn). Options (checked 2026-10-13): SignPath Foundation (free for open source, signs builds from CI, so needs the release workflow below), a Certum open-source certificate (from about 50 EUR a year), or a Sectigo/DigiCert certificate (roughly 150 to 400 GBP a year, key on a token or cloud HSM). Azure Artifact Signing ($9.99 a month) takes individuals only in the US and Canada. Then check the signature shows in the file properties and a fresh Windows 11 install gives no SmartScreen warning.
- [ ] **Release workflow in CI** (`.github/workflows/release.yml`): build Windows, the Linux packages and the BMR ISO in one run and attest them (build provenance needs GitHub Actions, not a personal machine). Today releases are built and uploaded by hand.
- [ ] **VirusTotal scan** of the release files (manual upload or an API key).
- [ ] **MSI: click through the removal pages** on a desktop: Programs and Features > Change, Remove, keep or delete, Back, Cancel (tests 3, 4, 5 and 7 in [MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)). The silent paths passed on 2026-10-13.
- [ ] **Silent MSI install with a ready-made config** for Group Policy or Intune: a `CONFIGFILE` property and a custom action that copies it into `C:\ProgramData\ProxmoxBackupClient`, a template config, and a deployment guide. Not built: `installer/wix` has no such property.

## Upstream (tizbac/proxmoxbackupclient_go)

- [ ] **#105 open**: restore puts back 1970 and earlier file times and folder times, plus `-no-mtime` (CLI, asked for by tizbac). Waiting on review.
- [ ] Candidate ports, each a fresh small branch off upstream/master (our commits no longer apply as patches): `proxmoxbackup-machine` sends its mail (fork 5e02a28); exclude "Recovery" only at the drive root (d749941); restore permissions when the owner cannot be set, and keep a folder's protected (non-inheriting) DACL (5c1ef56).

## Background service

- [ ] **Live test: the encryption key on machine backups the service runs** (fixed in 2f46844, not tried with a key configured).
- [ ] **Live test on Linux: Restart as administrator** (pkexec) from a desktop session, and clicking Install in the root app.
- [ ] Maybe: Pause from the tray menu, and a "resume at" time.

## Backing up

- [ ] Maybe: options per schedule (the monthly one does a full read, or goes to another server). Several schedules per set were built on 2026-10-13; Mick put this off.
- [ ] **Retention per Backup Set in the client** (keep last N / hourly / daily / weekly / monthly), applied by a group prune after each good run, so Undelete and Roll back keep more restore points. Only works where the datastore's own prune job leaves that group alone (own namespace, or looser datastore rules). Show "Rollback points kept: ..." on the card. (The PBS-side `keep-hourly 48` is already in place.)
- [ ] Maybe: NTFS extended attributes in files restored from a machine (disk image) backup (the image reader does not read `$EA`; folder backups record them since v0.9.0, and a bare-metal restore keeps them).
- [ ] **Very large folders in split backups** are split one level deep only (`gui/backup_analysis.go`): a 700 GB subfolder becomes one 700 GB part. Recursive splitting down to a size limit was planned; less pressing since change detection reuses unchanged files. Decide whether it is still wanted.
- [ ] Machine backups: a GUI option for a forensic copy (read free space too; the CLI has `-readfreespace`).
- [ ] Machine backups: volumes without a drive letter (EFI, recovery) are still read whole (small).
- [ ] Offered, not agreed: an empty-block marker in machine backups, skipping the 4 MiB copy for blank chunks in `emitZeros`/`push` (about 3 to 4 minutes off a 27 minute laptop run).

## Restoring

- [ ] **Disk images: a changed partition layout** since a snapshot (a partition moved) makes that snapshot's folder list fail with a clear error; could fall back to finding the volume by its NTFS serial number.
- [ ] **Disk images: BitLocker volumes** cannot be browsed (they would need the recovery key).
- [ ] **Quick test: Proxmox VE File Restore** on a `vm/9000xxx` snapshot this client wrote, to compare with our own disk-image browsing.
- [ ] Maybe: an "Other backup..." choice in the Undelete and Roll back set picker (server, backup ID, target folder) for another machine's backups.

## Bare-metal restore ISO

- [ ] Boot-stick exclusion: the 2026-10-13 test ISO, booted as a fixed SATA disk on a UEFI VM, left that disk out and named it under "Not eligible as target" (Mick). Still to try: a real Ventoy stick (BIOS and UEFI) and a BIOS VM with two internal disks (`patch-clonezilla.sh` on VM140) and test from a dd-written stick, a Ventoy stick (BIOS and UEFI) and a USB SSD, on a VM with one and with two internal disks. Check the key-file step lists the Ventoy data partition (exFAT) so the key can live on the same stick.
- [ ] Test a bare-metal boot with a passphrase-protected key.
- [ ] **Publish a new ISO.** The only published one (v0.5.0) predates the removal of the Linux snapshot store file (`.pbs_snapshot_N.cow`) after a restore; BMR.md tells its users to delete it by hand.

## Security

- [ ] **Secrets stored in plain text** in `config.json`: PBS tokens, passwords and remembered key passphrases. The settings folder is now open to administrators only (eb91d17, CHANGELOG [Unreleased] > Security), so this is defence in depth: DPAPI (`CryptProtectData` with the machine scope, so the SYSTEM service can read them) on Windows with automatic migration of old configs, and a Linux equivalent or at least the 0700/0640 folder as now.
- [ ] Key files kept outside the settings folder keep whatever permissions they had (keys the app generates get a protected ACL).
- [ ] An "Export recovery key" button (print or save the key with its fingerprint).
- [ ] Restore of an encrypted snapshot from the Proxmox VE storage configuration (key given to PVE): untested.

## Languages and docs

- [ ] Native speakers to look over the app's strings in the languages its users actually use. All 17 were reviewed against the English on 2026-10-13 (meaning, one term per concept, menu names, plurals, quotation marks); a native ear still catches what is correct but unidiomatic. The feature lists on the About page (`featuresList`, `techList`) were not part of that review.
- [ ] The 12 translated READMEs were regenerated from the English one on 2026-10-13 (condensed, machine-translated): keep them in step when the English README changes, and have native speakers look them over.

## Command-line tools

- [ ] `proxmoxbackup-directory` still prints a few early errors (config file, fingerprint, login, key file) to stdout instead of stderr.

## Tests

- [ ] `testing/e2e/run.sh` still says symbolic links are skipped.

- [ ] Load test: five Backup Sets at once, check memory stays bounded.
- [ ] The opt-in end-to-end test (`PBS_E2E=1`) still skips folder modification times; enable that comparison next time it runs.

## Parked

Kept, not being worked on (Mick, 2026-10-11).
- **App froze** (high CPU, unresponsive) after a machine backup on rigel, 26/27 September. Never seen again. Notes in the history file ("UNRESOLVED: app froze").
- **BMR wizard language choice**: the wizard is English only by design (it skips Clonezilla's language prompt for speed); a picker would need a full string table for our own prompts.
- **Speed limit per Backup Set**: a cap slows every run even when the link is free; revisit only for a real case such as a slow site link. Pause covers "the server is busy now".
- **A separate bare-metal restore CLI** (`cmd/proxmoxbackupclient-restore` for a Linux live USB): the BMR ISO's wizard covers bare-metal restore; build it only if a no-GUI rescue path is wanted.
- **Is deepthought's backup CPU-bound?** Moot since change detection skips unchanged files; reopen only if a full read is too slow.
