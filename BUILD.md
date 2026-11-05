# Build Instructions

This builds Salvanza for Proxmox Backup Server: the GUI (`salvanza-gui`), the background service (`salvanza-svc`), the command line (`salvanza-cli`), the Windows installer (MSI), the Linux packages (.deb, .rpm and Arch, with `make deb`, `make pkgfedora` and `make pkgarch`) and the bare-metal recovery ISO (`make clonezilla`).

## Tools

| Tool | Version | Needed for |
|---|---|---|
| Go | 1.25 or later (the modules say `go 1.25`) | everything |
| Wails CLI | v2.13.0 (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0`) | the GUI on Windows |
| Node.js and npm | a current LTS | only to rebuild the frontend; the built frontend in `gui/frontend/dist` is committed |
| C compiler | MinGW-w64 gcc on Windows, gcc on Linux | the GUI (cgo) |
| GTK3 and WebKitGTK dev packages | `libgtk-3-dev`, `libwebkit2gtk-4.1-dev` (4.0 also works) | the GUI on Linux |
| WiX Toolset 3 | 3.14 (`wix314-binaries.zip` is enough) | the MSI |
| `dpkg-dev` | | the .deb |
| Docker | | the .rpm, and the optional Docker build of the Windows GUI |

On Windows, Go and MinGW can come from Chocolatey (in an elevated PowerShell):

```powershell
choco install go
choco install mingw
```

Then open a new, non-elevated shell and check `go version` and `gcc --version`.

## The workspace

`go.work` lists `gui`, `gui/api`, `cli`, `pbscommon`, `clientcommon`, `machinebackuplib`, `machinebackup`, `snapshot` and `nbd`. `directorybackup`, `pkg/retry` and `pkg/security` are separate modules and are not in it. `directorybackup`, `machinebackup` and `nbd` are libraries now: the program is `cli` (`salvanza-cli`), whose `go.mod` points at all of them, so it is built with `GOWORK=off` ([below](#command-line)).

## GUI and service (Windows)

From `gui/`:

```powershell
wails build -clean -ldflags "-X main.appVersion=2.0.0"
go build -tags service -trimpath -ldflags "-s -w -X main.appVersion=2.0.0" -o build\bin\salvanza-svc.exe
```

This gives `gui\build\bin\salvanza-gui.exe` (the app) and `gui\build\bin\salvanza-svc.exe` (the background service, which also takes `-list-sets`, `-undelete` and `-rollback`). Keep them in the same folder. The app finds its brand from its file name (`gui/brand.go`): `salvanza-gui.exe` is Salvanza, and a copy named `nimbusbackup-gui.exe` or `etitechbackup-gui.exe` (with the service as `nimbusbackup-svc.exe` or `etitechbackup-svc.exe`) is that brand, with its own folders. Use the version from `gui/wails.json` (`productVersion`).

The app's Help page shows the manuals from `gui/docs/`, copies of `docs/manual/GUI.md`, `CLI.md`, `BMR.md` and `docs/RESTORE_GUIDE.md`. No build step copies them: after editing a manual in `docs/`, copy it over the one in `gui/docs/` yourself. `TestEmbeddedManualsMatchDocs` (`gui/manuals_test.go`, part of `make test`) fails while the copies differ.

To build the Windows GUI on Linux, `./build_gui_windows_docker.sh` runs `wails build` in a `golang:1.25` container. It does not build the service.

## Command line

`salvanza-cli` is one program with the commands `backup-folder`, `backup-disk` and `attach-nbd` (Linux only). From `cli/`, with `GOOS` set to `windows` or `linux`:

```sh
GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=2.0.0" -o ../dist/salvanza-cli .
```

Add `.exe` on Windows; for the MSI, write it to `gui\build\bin\salvanza-cli.exe`. `make cli` does the same for the host system. The earlier tool names ([CLI.md](docs/manual/CLI.md#earlier-names)) are not built any more: the Linux packages install `proxmoxbackup-directory`, `proxmoxbackup-machine` and `proxmoxbackup-nbd` as links to `salvanza-cli`, and the MSI installs `.cmd` files from `installer/wix/cli-aliases/`.

The bare-metal restore ISO has the nbd code on its own as `pbsnbd`, built from `nbd/cmd/pbsnbd` ([below](#bare-metal-recovery-iso)).

## Windows installer (MSI)

Build the app, the service and `salvanza-cli.exe` into `gui\build\bin` first (above), then:

```powershell
cd installer\wix
.\build-msi.ps1                       # version from gui\wails.json
.\build-msi.ps1 -Version 2.0.0 -WixDir C:\Dev\tools\wix314
```

The result is `installer\wix\out\Salvanza-<version>-x64.msi`. The version must be three numbers (`x.y.z`). WiX is looked for in `-WixDir`, then on `PATH`.

On Linux, `make msi` (`installer/wix/build_msi_linux.sh`) builds the app, the service and `salvanza-cli` in a `golang:1.25` container and runs WiX under Wine in another container, so the host needs only Docker. It writes one MSI per brand (Salvanza, Nimbus Backup, Etitech Backup), `<brand>-<version>.msi`, to `dist/`.

## Linux packages

```sh
packaging/debian/build-deb.sh [output-dir]     # .deb, on Debian, Ubuntu or Mint; default output dist/
packaging/fedora/build-rpm.sh [output-dir]     # .rpm, in a fedora:44 Docker container
```

Both take the version from `gui/wails.json`, or from `PBS_VERSION`. The .deb build host needs Go, `dpkg-dev` and the GTK3 and WebKitGTK dev packages. Build release .debs on Ubuntu 22.04 (a container is fine) with `libwebkit2gtk-4.1-dev`: the package then depends on WebKitGTK 4.1 and glibc 2.34, which Ubuntu 22.04 and 24.04, Mint 21 and Debian 12 and 13 all have. A .deb built against WebKitGTK 4.0 (Mint 21's dev package) does not install on Debian 13 or Ubuntu 24.04, and one built on a newer system needs a newer glibc than Mint 21 has; the .rpm build host needs only git and Docker. See [packaging/debian/README.md](packaging/debian/README.md) for what the packages install. `make deb` and `make pkgfedora` call the same scripts.

The Arch package is built on Arch with `make pkgarch`: it stages a source tarball from the working tree and runs `makepkg` on [packaging/arch/PKGBUILD](packaging/arch/PKGBUILD), which lists the build host's packages.

## Bare-metal recovery ISO

`make clonezilla` downloads the stock Clonezilla live ISO (version `CLONEZILLA_VER`, 3.3.3-37 by default, cached in `dist/clonezilla/`), builds `pbsnbd`, and patches it and the restore scripts into `dist/Salvanza-BMR-<version>.iso`. It needs network access, `7z`, `xorriso`, `squashfs-tools`, `fakeroot`, `patch` and the isolinux MBR, and is not part of `make all`. See [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md).

## Tests

`make test` runs `go test -race` in every module (the separate modules with `GOWORK=off`). `make test-e2e` backs up with `salvanza-cli` against a real PBS in Docker and restores with the official client; see `testing/e2e/README.md`.

## Makefile

On Linux (or with GNU make and a POSIX shell on Windows) the Makefile runs the same commands: `make cli` (`salvanza-cli` in `dist/`), `make gui`, then `make service` (in that order: `wails build -clean` empties `gui/build/bin`), `make all` for all three, `make deb`, `make pkgfedora`, `make pkgarch`, `make msi` (the Windows MSIs, built on Linux with Docker and Wine), `make clonezilla`, `make test`, `make test-e2e`, `make clean`. `make help` lists them. The version comes from `gui/wails.json`.
