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
PBS_VERSION=0.9.0 packaging/debian/build-deb.sh
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
| `/usr/bin/proxmoxbackupclient-root` | starts the app as root, for machine (whole-disk) backups and snapshots, with your own settings; asks for the password with pkexec (sudo from a terminal) |
| `/usr/bin/proxmoxbackup-directory` | folder backups from the command line |
| `/usr/bin/proxmoxbackup-machine` | whole-disk backups from the command line |
| `/usr/bin/proxmoxbackup-nbd` | attach a disk image from the server as `/dev/nbdN` (root) |
| `/usr/lib/proxmoxbackupclient/ProxmoxBackupClient` | the app itself |
| `/usr/lib/proxmoxbackupclient/ProxmoxBackupClientSVC` | the background service program |
| `/usr/lib/systemd/system/proxmoxbackupclient.service` | its systemd unit, installed but not enabled |
| `/usr/share/applications/proxmoxbackupclient.desktop` | menu entry "Proxmox Backup Client" |
| `/usr/share/applications/proxmoxbackupclient-root.desktop` | menu entry "Proxmox Backup Client (administrator)" |
| `/usr/share/icons/hicolor/256x256/apps/proxmoxbackupclient.png` | icon |
| `/usr/share/doc/proxmoxbackupclient/` | README, CHANGELOG, the manuals (and `copyright` in the .deb) |

The .deb recommends `pkexec | policykit-1` and suggests `elastio-snap-dkms`; the .rpm
requires `polkit`.

## The two menu entries

- **Proxmox Backup Client** runs the app as you. Folder backups of your own files work
  this way.
- **Proxmox Backup Client (administrator)** runs `proxmoxbackupclient-root`. It asks for
  your password through polkit (pkexec), or with sudo when started from a terminal, and
  runs the app as root with your `HOME`, so it uses your settings
  (`~/.proxmox-backup-guardian`). When it closes it hands that folder and your log folder
  (`~/.local/state/proxmoxbackupclient`) back to you. Use it for machine backups, block
  snapshots, and for turning the background service on or off.

## The background service

Without the service, scheduled Backup Sets run only while the app is open (there is no
tray icon on Linux, so its window has to stay open). The service runs them as root, also
when nobody is logged in.

Turn it on in the administrator app: Preferences > Advanced > Run as Service. That:

1. copies your settings (servers, keys, Backup Sets, history) to
   `/var/lib/proxmoxbackupclient`, unless that folder already holds settings; your own
   `~/.proxmox-backup-guardian` is left as it was;
2. sets that folder to owner root, group `sudo` (or `wheel`, or `admin`; root if none of
   these exist), mode 2750 (setgid, so new files keep the group); files in it are made
   readable by the group (for example 0640), never writable;
3. runs `systemctl enable --now proxmoxbackupclient.service`.

From then on every copy of the app uses `/var/lib/proxmoxbackupclient`. Members of the
sudo or wheel group can open the app normally: it reads the Backup Sets and the service's
API token, hands backups to the service and shows history. Adding, editing or deleting
Backup Sets needs the administrator app. The same Preferences section also starts, stops
and removes the service. Removing it stops and disables the unit and the app runs the
schedules again; the settings stay in `/var/lib/proxmoxbackupclient`.

The service logs to `/var/log/proxmoxbackupclient/`. See section 15 of the GUI manual.

## Install, upgrade and removal

| | .deb | .rpm |
| --- | --- | --- |
| Install | `postinst`: `systemctl daemon-reload`; the unit is not enabled or started | `%post`: the same |
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
