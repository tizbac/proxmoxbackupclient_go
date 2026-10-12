# Linux packages

`packaging/debian/build-deb.sh` builds `proxmoxbackupclient_<version>_<arch>.deb`
(Debian, Ubuntu, Mint); `packaging/fedora/build-rpm.sh` builds the RPM in a
Fedora container. The version comes from `gui/wails.json` (`productVersion`),
or `PBS_VERSION`. Both use the launchers and menu entries in `packaging/linux/`.

## .deb build host

- Go toolchain
- `dpkg-dev` (dpkg-deb, dpkg-architecture)
- GTK3 and WebKitGTK headers: `libgtk-3-dev` and `libwebkit2gtk-4.1-dev`
  (4.0 is used when 4.1 is not installed)
- the frontend in `gui/frontend/dist` (committed)

```sh
packaging/debian/build-deb.sh [output-dir]      # default: dist/
```

## .rpm

Needs only git and docker on the host; Go, GTK and WebKit are installed in
the container (`fedora:44`, or `PBS_FEDORA_IMAGE`).

```sh
packaging/fedora/build-rpm.sh [output-dir]      # default: dist/
```

## What is installed

| Path | |
| ---- | --- |
| `/usr/bin/proxmoxbackupclient` | the app, as the current user (folder backups of your files; with `-list-sets`, `-undelete` or `-rollback`, those commands without a window) |
| `/usr/bin/proxmoxbackupclient-root` | the app as root, for machine (whole-disk) backups and snapshots, with your own settings; asks for the password with pkexec (sudo from a terminal) |
| `/usr/bin/proxmoxbackup-directory` | folder backups from the command line |
| `/usr/bin/proxmoxbackup-machine` | whole-disk backups from the command line |
| `/usr/bin/proxmoxbackup-nbd` | attach a disk image from the server as `/dev/nbdN` (root) |
| `/usr/lib/proxmoxbackupclient/ProxmoxBackupClient` | the app itself |
| `/usr/share/applications/proxmoxbackupclient.desktop`, `proxmoxbackupclient-root.desktop` | menu entries |

Scheduled backups run while the app is open (it can sit in the tray). There
is no background system service on Linux yet: a root service would need to
use each user's own settings without handing the user root, which is not
designed yet.

Snapshots of mounted file systems need the elastio-snap kernel module
(`elastio-snap-dkms`, see the main README); without it partitions are read
live.
