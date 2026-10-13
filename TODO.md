# Proxmox Backup Client - TODO

What is still open, as of 2026-10-13. Finished work is in [CHANGELOG.md](CHANGELOG.md). The old, much longer list (with its design notes, findings and audit trail) is kept in [docs/TODO-HISTORY.md](docs/TODO-HISTORY.md); every item still open there has been carried over here.

## Release and distribution

- [ ] **Next release.** The last release is v0.8.1 (0.8.2 was deployed, not released). Master now also holds the MSI and Linux packages, the Linux background service, Pause/Resume/Stop for service runs and the shared Backup ID warning (CHANGELOG [Unreleased]). Pick the version (the docs assume v0.9.0), tag, release, then deploy to the machines.
- [ ] **Code signing** for the exe files and the MSI (no Authenticode certificate yet, so SmartScreen and Defender warn). Options: Azure Trusted Signing (about 108 EUR a year), SignPath, or a DigiCert/Sectigo certificate. Then check the signature shows in the file properties and a fresh Windows 11 install gives no SmartScreen warning.
- [ ] **Release workflow in CI** (`.github/workflows/release.yml`): build Windows, the Linux packages and the BMR ISO in one run and attest them (build provenance needs GitHub Actions, not a personal machine). Today releases are built and uploaded by hand.
- [ ] **VirusTotal scan** of the release files (manual upload or an API key).
- [ ] **MSI: the keep-settings question on removal** is probably never shown from Programs and Features: the MSI sets `ARPNOMODIFY=1`, so Uninstall there runs without the dialog; only `msiexec /i` then Remove shows it. Test, then either make the dialog reachable or reword the CHANGELOG and installer docs.
- [ ] **Branded MSIs would delete the wrong settings folder**: the "delete settings" step and its dialog text in `installer/wix/ProductBody.wxi` use `%ProgramData%\<ExeName>`, but the app always uses `C:\ProgramData\ProxmoxBackupClient`.
- [ ] **CI's MSI step** calls candle and light directly without `-arch x64` or `-sice:ICE61`; switch it to `installer/wix/build-msi.ps1`, and drop the stale `installer/build.bat` and `config.example.json`.
- [ ] **Dead build files**: the Makefile's `all`, `gui`, `service`, `cross-gui-windows` and `release` targets use the old Nimbus names and a `cmd/service` folder that does not exist; the root `build.bat`, `build_cli.bat`, `build_gui.bat`, `build_gui.sh` and `build.sh` no longer work either. BUILD.md has the commands that do. Fix or delete them.
- [ ] `packaging/arch/PKGBUILD` still builds upstream's old "pbsgo" package: update it to the .deb/.rpm layout or drop it.
- [ ] **Silent MSI install with a ready-made config** for Group Policy or Intune: a `CONFIGFILE` property and a custom action that copies it into `C:\ProgramData\ProxmoxBackupClient`, a template config, and a deployment guide. Not built: `installer/wix` has no such property.

## Upstream (tizbac/proxmoxbackupclient_go)

- [ ] **#105 open**: restore puts back 1970 and earlier file times and folder times, plus `-no-mtime` (CLI, asked for by tizbac). Waiting on review.
- [ ] Candidate ports, each a fresh small branch off upstream/master (our commits no longer apply as patches): `proxmoxbackup-machine` sends its mail (fork 5e02a28); exclude "Recovery" only at the drive root (d749941); restore permissions when the owner cannot be set, and keep a folder's protected (non-inheriting) DACL (5c1ef56).

## Background service

- [ ] **Stats rows for runs the service does**: Files, Processing, Upload and New/Reused stay empty on the card for a run handed to the service or started by its schedule. The percentage and stage line already arrive (`/backup/runs`); add the stats to the API's run entry and have `watchServiceRuns` emit them as the app's own runs do.
- [ ] **Live test: the encryption key on machine backups the service runs** (fixed in 2f46844, not tried with a key configured).
- [ ] **Live test on Linux: Restart as administrator** (pkexec) from a desktop session, and clicking Install in the root app.
- [ ] **Bug: "When the backup finishes: closes (the app exits)" stops the service.** For a set the service runs, `runPostBackupActions` (`gui/email_notifications.go`) calls `os.Exit(0)` in the service process. Skip it there (minimise already checks `isServiceProcess`), and maybe say in the editor that it applies to the app only.
- [ ] **Two service names on Windows**: `ProxmoxBackupClientSVC -service install` registers the service as `ProxmoxBackupClient`, while the MSI and Preferences > Run as Service use `ProxmoxBackupClientSVC`. Check and make them one name (installing both ways would give two services).
- [ ] Maybe: Pause from the tray menu, and a "resume at" time.

## Backing up

- [ ] **Several schedules per Backup Set** (Mick, 2026-10-11). Today a set has one trigger (daily at a time, every N minutes in a window, or manual) plus weekdays, so monthly or yearly is impossible.
  - Model: `Schedules []Schedule` per set, the old fields migrated into one entry on load. A schedule repeats every N hours (at minute :MM, optional window), days (at HH:MM, optional weekdays), weeks (which weekdays, at HH:MM), months (day D, "last day", or "the 2nd Tuesday", at HH:MM) or years (date and time), with an optional start date and an enabled tick.
  - UI: Manual / Scheduled radio; a list of schedules in plain words ("Every month on the last day at 02:00") with Edit, Delete and Add schedule; the editor shows only the fields the unit needs and a live "Next 3 runs" preview. The card shows the earliest next run.
  - Behaviour: earliest across schedules; two due together run once; catch-up as today; "full read every N runs" counts runs. Later maybe a per-schedule override (the monthly one does a full read, or goes to another server).
  - The service reads the same model. Tests for month ends, 29 February, DST changes, "last Friday". 18 languages, manual, CHANGELOG.
- [ ] **Retention per Backup Set in the client** (keep last N / hourly / daily / weekly / monthly), applied by a group prune after each good run, so Undelete and Roll back keep more restore points. Only works where the datastore's own prune job leaves that group alone (own namespace, or looser datastore rules). Show "Rollback points kept: ..." on the card. (The PBS-side `keep-hourly 48` is already in place.)
- [ ] **Alternate data streams in folder backups** (Zone.Identifier and the like) are not captured, so they cannot come back; machine-image restores already put them back. Needs a field in `FileMetaEntry`, a collector, and the restore step; the "Restore alternate data streams" option is still reserved (`RestoreADS`).
- [ ] **Very large folders in split backups** are split one level deep only (`gui/backup_analysis.go`): a 700 GB subfolder becomes one 700 GB part. Recursive splitting down to a size limit was planned; less pressing since change detection reuses unchanged files. Decide whether it is still wanted.
- [ ] Machine backups: a GUI option for a forensic copy (read free space too; the CLI has `-readfreespace`).
- [ ] Machine backups: volumes without a drive letter (EFI, recovery) are still read whole (small).
- [ ] Offered, not agreed: an empty-block marker in machine backups, skipping the 4 MiB copy for blank chunks in `emitZeros`/`push` (about 3 to 4 minutes off a 27 minute laptop run).

## Restoring

- [ ] **Disk images: a changed partition layout** since a snapshot (a partition moved) makes that snapshot's folder list fail with a clear error; could fall back to finding the volume by its NTFS serial number.
- [ ] **Disk images: BitLocker volumes** cannot be browsed (they would need the recovery key).
- [ ] **Quick test: Proxmox VE File Restore** on a `vm/9000xxx` snapshot this client wrote, to compare with our own disk-image browsing.
- [ ] The **Restore modification times** tick box does nothing: modification times are always restored (`restore_inline.go`). Remove it, or make it work.
- [ ] Maybe: an "Other backup..." choice in the Undelete and Roll back set picker (server, backup ID, target folder) for another machine's backups.

## Bare-metal restore ISO

- [ ] **Never offer the boot stick as the target disk.** The wizard skips only nbd, loop, sr, ram and removable devices, so a Ventoy stick (device-mapper) or a USB SSD that reports non-removable becomes a candidate. Also skip `dm-*`, `zram*` and the disk behind the live medium (`findmnt -no SOURCE /run/live/medium` or `/dev/mapper/ventoy`, its parent via `lsblk -no PKNAME`, a dm device's `slaves/`).
- [ ] Rebuild the ISO (`patch-clonezilla.sh` on VM140) and test from a dd-written stick, a Ventoy stick (BIOS and UEFI) and a USB SSD, on a VM with one and with two internal disks. Check the key-file step lists the Ventoy data partition (exFAT) so the key can live on the same stick.
- [ ] Test a bare-metal boot with a passphrase-protected key.
- [ ] **Publish a new ISO.** The only published one (v0.5.0) predates the removal of the Linux snapshot store file (`.pbs_snapshot_N.cow`) after a restore; BMR.md tells its users to delete it by hand.

## Security

- [ ] **Secrets stored in plain text** in `config.json`: PBS tokens, passwords and remembered key passphrases. The settings folder is now open to administrators only (eb91d17, CHANGELOG [Unreleased] > Security), so this is defence in depth: DPAPI (`CryptProtectData` with the machine scope, so the SYSTEM service can read them) on Windows with automatic migration of old configs, and a Linux equivalent or at least the 0700/0640 folder as now.
- [ ] Key files kept outside the settings folder keep whatever permissions they had (keys the app generates get a protected ACL).
- [ ] An "Export recovery key" button (print or save the key with its fingerprint).
- [ ] Restore of an encrypted snapshot from the Proxmox VE storage configuration (key given to PVE): untested.

## Languages and docs

- [ ] Native-speaker review of the machine-translated strings (the 17 languages other than English and French), the encryption texts first.
- [ ] The 12 translated READMEs were regenerated from the English one on 2026-10-13 (condensed, machine-translated): keep them in step when the English README changes, and have native speakers look them over.
- [ ] `MachineBackupConfig` (machine backup settings) has hard-coded English labels; move them to the translations.
- [ ] Some Go error messages are still in French and reach the window in every language: "serveur PBS '%s' introuvable" (`gui/config.go`, 4 places), "fichier de cle de chiffrement illisible" (`gui/config.go`), and the key messages in `gui/crypt_key.go` ("fichier introuvable ou illisible", "cle illisible", "le fichier %s existe deja..."). Put them in English (MULTI_PBS_GUIDE.md and MULTI_PBS_USER_GUIDE.md quote the first one).
- [ ] Screenshots in `docs/screenshots` date from late September (old side menu, no More menu or VSS badge, a BETA banner on Restore, the old Preferences tabs); only `gui-backup-progress.png` is current. Retake them with the UI harness or a real run. The `nimbus-*.png` files are upstream's.

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
