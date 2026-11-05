# Salvanza for Proxmox Backup Server 2.0: release notes

Version 2.0 is the first release under the new name: **Salvanza for Proxmox Backup Server** (Salvanza for short), formerly Proxmox Backup Client. Project site: [salvanza.org](https://salvanza.org/); source, releases and issues: [github.com/salvanza/salvanza_client](https://github.com/salvanza/salvanza_client).

Version 2.0 merges a large body of work on the GUI, the background service and the backup engine into the project. The app now has Backup Sets with several schedules each, incremental folder backups with the official client's change detection, machine backups that skip free space and can leave partitions out, several backups at once, speed limits, Undelete and Roll back, client-side encryption with keys you can keep, a guided bare-metal restore ISO that also restores into a virtual machine (P2V), and 18 interface languages. With the background service installed, the computer's PBS servers and Backup Sets belong to the service, and the app connects to it with administrator rights. The command line is one program, `salvanza-cli`, with the commands `backup-folder`, `backup-disk` and `attach-nbd`. The [README](README.md#-new-in-20) lists what is new; [CHANGELOG.md](CHANGELOG.md) has every change.

## Downloads

All files are on the [releases page](https://github.com/salvanza/salvanza_client/releases), with `SHA256SUMS.txt` and a build provenance attestation ([verifying a download](README.md#-verifying-any-download)). The Windows programs are not yet Authenticode-signed.

| For | File |
|---|---|
| Windows | `Salvanza-<version>-x64.msi`: the app (`salvanza-gui.exe`), the background service (`salvanza-svc.exe`) and the command line (`salvanza-cli.exe`), in `C:\Program Files\Salvanza`. The branded installers (`NimbusBackup-<version>-x64.msi`, `EtitechBackup-<version>-x64.msi`) are the same build under another brand |
| Debian, Ubuntu, Mint | `salvanza_<version>_amd64.deb` |
| Fedora | `salvanza-<version>-*.rpm` |
| Arch | Built from [packaging/arch/PKGBUILD](packaging/arch/PKGBUILD) with `make pkgarch` |
| Command line only | The per-platform `salvanza-cli` archives for Windows and Linux |
| A machine to restore | `Salvanza-BMR-<version>.iso`, the bare-metal restore ISO |

When a release has no Linux package, build it with `make deb`, `make pkgfedora` or `make pkgarch` ([BUILD.md](BUILD.md)).

## Upgrading

**From Proxmox Backup Client.** The upgrade is automatic. The Salvanza MSI removes an installed Proxmox Backup Client (and a Nimbus Backup or Etitech Backup install) as it would an older version of itself, and the Linux package `salvanza` replaces the `proxmoxbackupclient` and `pbsgo` packages. The earlier background service is removed. The first time Salvanza starts, it moves the earlier settings over once, from `C:\ProgramData\ProxmoxBackupClient`, `%APPDATA%\ProxmoxBackupClient`, `~/.proxmox-backup-guardian` or `/var/lib/proxmoxbackupclient`, to `C:\ProgramData\Salvanza`, `%APPDATA%\Salvanza`, `~/.salvanza` or `/var/lib/salvanza`, and keeps a dated copy of the settings files there, in `pre-salvanza-<date>`. The PBS servers, keys, Backup Sets and history carry on, and nothing changes on PBS.

| Before | Now |
|---|---|
| `ProxmoxBackupClient.exe`, `ProxmoxBackupClientSVC.exe` (service `ProxmoxBackupClientSVC`) | `salvanza-gui.exe`, `salvanza-svc.exe` (service `salvanza-svc`) |
| `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd` | `salvanza-cli backup-folder`, `salvanza-cli backup-disk`, `salvanza-cli attach-nbd` |
| Package and service `proxmoxbackupclient` | Package `salvanza`, service `salvanza.service` |
| Local API header `X-Proxmox-Client-Token`, `PBSGO_API_TOKEN`, `PBSGO_TOKEN_ROTATION`, `PBSGO_TOKEN_GRACE` | `X-Salvanza-Token`, `SALVANZA_API_TOKEN`, `SALVANZA_TOKEN_ROTATION`, `SALVANZA_TOKEN_GRACE` (the earlier names are still accepted) |

The earlier command line names still work: the Linux packages install them as links to `salvanza-cli`, and the MSI as `.cmd` files beside it, so scripts and scheduled tasks keep running ([command line manual](docs/manual/CLI.md#earlier-names)). Nimbus Backup and Etitech Backup now have their own program names (`nimbusbackup-gui`, `etitechbackup-gui`, ...) and settings folders (`NimbusBackup`, `EtitechBackup`, `~/.nimbusbackup`, `~/.etitechbackup`). The Acme Backup demonstration brand is gone.

**From 1.2.0-rc.** Install the 2.0 MSI or package over it: it upgrades in place and keeps the settings and Backup Sets, with each set's PBS server, Proxmox VE VM type and weekly or monthly schedule. On Linux the package is now called `salvanza` and replaces `pbsgo`; the service takes over the `pbsgo` settings when it has none of its own.

**From 1.1.x.** Version 1.1 had the command-line tools only; 2.0 adds the app and the background service beside them. The machine backup (now `salvanza-cli backup-disk`) sends its e-mail report and exits with code 3 when some sectors could not be read, and on Windows the commands print errors instead of showing message boxes. See the [command line manual](docs/manual/CLI.md).

**The background service.** The installers set up the background service and start it (an upgrade of a Linux package keeps a service you turned off, off). With the service, only the service reads the computer's settings folder (`C:\ProgramData\Salvanza`, `/var/lib/salvanza`), and the app asks once for administrator rights to connect to it: UAC on Windows, your password through pkexec on Linux, or an administrator's name and password on a standard account. Schedules run whether or not anyone is signed in. A user who cannot answer the prompt can choose **Use my own settings** and keep their own servers and Backup Sets, which run only while the app is open or in the tray; **Connect** later offers to copy them to the computer's. See [section 15 of the GUI manual](docs/manual/GUI.md#connecting-to-the-background-service).

**Change detection.** Before switching a Backup Set to **Data** or **Metadata** change detection, update every machine that restores from it to 2.0: earlier versions show the folders of a split archive without their files ([section 7 of the GUI manual](docs/manual/GUI.md#7-incremental-folder-backups-change-detection)).

**Passwords on Windows.** PBS tokens and passwords in `config.json` are encrypted for the computer the first time 2.0 starts. Going back to an earlier version afterwards means entering them again.

## More

- [CHANGELOG.md](CHANGELOG.md): every change in 2.0, and the history of the GUI before it.
- [GUI manual](docs/manual/GUI.md), [command line manual](docs/manual/CLI.md) and [bare-metal restore manual](docs/manual/BMR.md); the same manuals are in the app (**Help**, F1).
- [installer/DEPLOYMENT.md](installer/DEPLOYMENT.md): deploying the MSI to many computers.
