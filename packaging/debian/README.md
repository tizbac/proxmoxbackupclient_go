# Debian packaging (pbsgo)

> ⚠️ **Trademark & affiliation notice.** This is an **independent community project**, not an official Proxmox product: it is **not affiliated with, endorsed by, or sponsored by Proxmox Server Solutions GmbH**. **Proxmox®** is a registered trademark of Proxmox Server Solutions GmbH in the EU, the U.S. and other countries, and it is used here **only** to describe compatibility. Following Proxmox's own trademark guidance (registered marks must not be used in product or app names — e.g. "XY App for Proxmox Virtual Environment" rather than "Proxmox XY App"), **a rename of the non-branded client is underway**: the product currently called *Proxmox Backup Client* is moving to a name that does not contain the Proxmox mark. See the [Proxmox media kit](https://www.proxmox.com/en/about/company-details/media-kit) for the exact rules.

Builds `pbsgo_<version>_<arch>.deb` with `dpkg-deb`. The version is taken
from `gui/wails.json` (`productVersion`).

## Build host requirements

- Go toolchain
- `dpkg-dev` (dpkg-deb, dpkg-shlibdeps)
- `libwebkit2gtk-4.1-dev` (cgo headers to link the GUI)
- built GUI frontend in `gui/frontend/dist`
  (`cd gui/frontend && npm install && npm run build`)

## Build

```sh
packaging/debian/build-deb.sh [output-dir]     # default output dir: dist/
# or
make deb
```

## Package contents

| Path | Description |
| ---- | ----------- |
| `/usr/bin/pbsgo` | directory/stream backup CLI |
| `/usr/bin/pbsgo-machine` | whole-machine (raw disk) backup CLI |
| `/usr/bin/pbsgo-nbd` | fidx-to-NBD restore tool (run as root) |
| `/usr/bin/pbsgo-gui` | GUI launcher (current user) |
| `/usr/bin/pbsgo-gui-root` | GUI launcher via sudo/pkexec (needed for machine backup) |
| `/usr/lib/pbsgo/pbsgo-gui` | GUI application binary (frontend embedded) |
| `/usr/share/applications/pbsgo-gui.desktop` | desktop entry |
| `/usr/share/icons/hicolor/256x256/apps/pbsgo.png` | icon |

Runtime dependencies of the GUI are computed with `dpkg-shlibdeps`; when
that is not possible the script falls back to
`libwebkit2gtk-4.1-0, libgtk-3-0t64 | libgtk-3-0`.
