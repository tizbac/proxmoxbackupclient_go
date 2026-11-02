# Linux packages

`packaging/debian/build-deb.sh` builds `proxmoxbackupclient_<version>_<arch>.deb`
(Debian, Ubuntu, Mint; amd64 or arm64, the build host's). `packaging/fedora/build-rpm.sh`
builds the RPM (and a source RPM) in a Fedora container. The version comes from
`gui/wails.json` (`productVersion`), or from `PBS_VERSION`. Both packages use the
launchers, menu entries and systemd unit in `packaging/linux/`, and install the same files.

## .deb build host

- Go toolchain (the Wails CLI is not needed: the app is built with `go build`)
- `dpkg-dev` (dpkg-deb, dpkg-architecture), on a Debian or Ubuntu system: the runtime
  dependencies are worked out with `dpkg -S` from the libraries the app links
- GTK3 and WebKitGTK headers: `libgtk-3-dev` and `libwebkit2gtk-4.1-dev`
  (4.0 is used when 4.1 is not installed)
- the frontend in `gui/frontend/dist` (committed)

```sh
packaging/debian/build-deb.sh [output-dir]      # default: dist/
PBS_VERSION=2.0.0 packaging/debian/build-deb.sh
```

## .rpm

Needs only git and docker on the host; Go, GTK and WebKit are installed in the
container (`fedora:44`, or `PBS_FEDORA_IMAGE`). It packages the committed tree
(`git archive HEAD`), so commit first.

```sh
packaging/fedora/build-rpm.sh [output-dir]      # default: dist/
```

## What is installed

| Path | |
| ---- | --- |
| `/usr/bin/proxmoxbackupclient` | starts the app as the current user (folder backups of your files; with `-list-sets`, `-undelete` or `-rollback`, those commands without a window) |
| `/usr/bin/proxmoxbackupclient-root` | starts the app as root, for machine (whole-disk) backups and snapshots; without the background service it uses your own settings, with it the service's; asks for the password with pkexec (sudo from a terminal) |
| `/usr/bin/proxmoxbackup-directory` | folder backups from the command line |
| `/usr/bin/proxmoxbackup-machine` | whole-disk backups from the command line |
| `/usr/bin/proxmoxbackup-nbd` | attach a disk image from the server as `/dev/nbdN` (root) |
| `/usr/lib/proxmoxbackupclient/ProxmoxBackupClient` | the app itself |
| `/usr/lib/proxmoxbackupclient/ProxmoxBackupClientSVC` | the background service program |
| `/usr/lib/systemd/system/proxmoxbackupclient.service` | its systemd unit, enabled and started on a fresh install |
| `/usr/share/applications/proxmoxbackupclient.desktop` | menu entry "Proxmox Backup Client" |
| `/usr/share/applications/proxmoxbackupclient-root.desktop` | menu entry "Proxmox Backup Client (administrator)" |
| `/usr/share/icons/hicolor/256x256/apps/proxmoxbackupclient.png` | icon |
| `/usr/share/doc/proxmoxbackupclient/` | README, CHANGELOG, the manuals (and `copyright` in the .deb) |

The .deb recommends `pkexec | policykit-1` and suggests `elastio-snap-dkms`; the .rpm
requires `polkit`.

## The two menu entries

- **Proxmox Backup Client** runs the app as you. Folder backups of your own files work
  this way. With the background service running, it asks once for administrator rights
  (pkexec) to connect to the service (below).
- **Proxmox Backup Client (administrator)** runs `proxmoxbackupclient-root`. It asks for
  your password through polkit (pkexec), or with sudo when started from a terminal, and
  runs the app as root with your `HOME`. Without the background service it uses your
  settings (`~/.proxmox-backup-guardian`); with it, it reads the service's token itself and
  connects without a second prompt. When it closes it hands your settings folder and your
  log folder (`~/.local/state/proxmoxbackupclient`) back to you. Use it for machine
  backups, block snapshots, and for turning the background service on or off.

## The background service

Without the service, scheduled Backup Sets run only while the app is open (there is no
tray icon on Linux, so its window has to stay open). The service runs them as root, also
when nobody is logged in.

A fresh install enables and starts the service, as the Windows installer does. On first
start it creates `/var/lib/proxmoxbackupclient`, root's alone: owner root, group root,
folders 0700 and files 0600. It sets these again each time it starts, which also undoes
what earlier versions did (group `sudo`, `wheel` or `admin`, mode 2750, files readable by
the group). An upgrade keeps your choice: a service you turned off stays off.

To turn it on later (for example after turning it off), use the administrator app:
Preferences > Advanced > Run as Service. That:

1. copies your settings (servers, keys, Backup Sets, history) to
   `/var/lib/proxmoxbackupclient`, unless that folder already holds settings; your own
   `~/.proxmox-backup-guardian` is left as it was;
2. runs `systemctl enable --now proxmoxbackupclient.service`; the service then locks the
   folder down as above.

From then on the service keeps the settings, and the app works on them through the
service's local API. The API needs the service's token
(`/var/lib/proxmoxbackupclient/api-token`, root, 0600). The app gets it by running a copy
of itself as root through pkexec: you are asked for your password (an administrator's, if
your account is not in the sudo or wheel group) the first time the app's window is in front
of you, and once more each time the service replaces the token (every 24 hours). The app
keeps the token in memory only. Until it is connected its window shows a **Connect**
button and no settings. Without pkexec (polkit) the app cannot connect. The administrator
app, running as root, reads the token itself.

The same Preferences section also starts, stops and removes the service. Removing it stops
and disables the unit; the settings stay in `/var/lib/proxmoxbackupclient`, and the app
goes back to your own settings and runs their schedules while it is open.

The service logs to `/var/log/proxmoxbackupclient/`. See section 15 of the GUI manual.

## Install, upgrade and removal

| | .deb | .rpm |
| --- | --- | --- |
| Install (also after a removal that kept the package's files) | `preinst` marks it fresh; `postinst`: `systemctl daemon-reload`, then `enable --now` | `%post`: the same |
| Upgrade | `postinst`: `systemctl try-restart`, so a running service restarts on the new version and a stopped one stays stopped | `%post`: the same |
| Removal | `prerm remove`: `systemctl disable --now`; `postrm`: `daemon-reload` | `%preun` (last version removed): `disable --now`; `%postun`: `daemon-reload` |

`/var/lib/proxmoxbackupclient` is never deleted by the package, also not on
`apt purge`. Delete it by hand if you want the settings gone. The systemd steps are
skipped when systemd is not running (for example in a container).

## Snapshots

Machine backups of a disk with mounted file systems need a block snapshot kernel module,
elastio-snap (`elastio-snap-dkms`) or dattobd, see the main README. Without one loaded the
backup refuses to run rather than copy file systems that are changing underneath it.
Partitions that are not mounted are read directly.
