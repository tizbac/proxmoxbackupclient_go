# Proxmox Backup Client: release notes

What changed in each version, and what is new on master since the last release, is in **[CHANGELOG.md](CHANGELOG.md)**. This page only lists what a release contains and where to read more.

## What a release contains

Every release on the [releases page](https://github.com/mjb-is/proxmoxbackupclient_go/releases) has a `SHA256SUMS.txt`. Up to v0.8.1 a release has:

| File | Contents |
|---|---|
| `ProxmoxBackupClient-v<version>-windows-amd64.zip` | The GUI and `ProxmoxBackupClientSVC.exe`, the background service. Keep both in the same folder |
| `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` | The GUI for Linux |
| `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` | `proxmoxbackup-directory.exe`, `proxmoxbackup-machine.exe` |
| `ProxmoxBackupClient-CLI-v<version>-linux-amd64.tar.gz` | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd` |

The bare-metal restore ISO is attached to the release that last changed it (v0.5.0 at the time of writing).

New since v0.8.2 and not yet in a release (see [Unreleased] in the CHANGELOG):

| File | Contents |
|---|---|
| `ProxmoxBackupClient-<version>-x64.msi` | Windows installer: the app and the background service under Program Files, a Start menu entry, in-place upgrades, and a keep or delete settings choice on removal. Built with `installer/wix/build-msi.ps1` |
| `proxmoxbackupclient_<version>_<arch>.deb` | Debian, Ubuntu, Mint: the app, the three command-line tools, the manuals, two menu entries and the systemd service (installed, not enabled). Built with `packaging/debian/build-deb.sh` |
| `proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` | Fedora: the same contents. Built with `packaging/fedora/build-rpm.sh` |

Builds are made by hand and are not code signed. Check them against `SHA256SUMS.txt`.

## Where to read more

- [README.md](README.md): features, install, quick start
- [docs/manual/GUI.md](docs/manual/GUI.md), [docs/manual/CLI.md](docs/manual/CLI.md), [docs/manual/BMR.md](docs/manual/BMR.md): the manuals
- [FEATURES_STATUS.md](FEATURES_STATUS.md): what works, what is partial, what is not done
