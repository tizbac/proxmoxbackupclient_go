# Build Instructions

This builds the GUI, the background service, the command-line tools, the Windows installer (MSI) and the Linux packages (.deb, .rpm).

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

`go.work` lists `gui`, `pbscommon`, `clientcommon`, `machinebackuplib`, `machinebackup`, `snapshot` and `nbd`. `directorybackup`, `pkg/retry` and `pkg/security` are separate modules and are not in it. To build `directorybackup` from the repository root, use a temporary work file that adds it (the packaging scripts do the same):

```sh
{ cat go.work; printf '\nuse ./directorybackup\n'; } > go.tmpwork
GOWORK=$PWD/go.tmpwork CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/proxmoxbackup-directory ./directorybackup
rm go.tmpwork go.tmpwork.sum
```

## GUI and service (Windows)

From `gui/`:

```powershell
wails build -clean -ldflags "-X main.appVersion=0.10.0"
go build -tags service -trimpath -ldflags "-s -w -X main.appVersion=0.10.0" -o build\bin\ProxmoxBackupClientSVC.exe
```

This gives `gui\build\bin\ProxmoxBackupClient.exe` (the app) and `gui\build\bin\ProxmoxBackupClientSVC.exe` (the background service, which also takes `-list-sets`, `-undelete` and `-rollback`). Keep them in the same folder. Use the version from `gui/wails.json` (`productVersion`).

The app's Help page shows the manuals from `gui/docs/`, copies of `docs/manual/GUI.md`, `CLI.md`, `BMR.md` and `docs/RESTORE_GUIDE.md`. After editing a manual, copy it over first (`make manuals`, which `make gui` runs); the GUI tests fail while the copies differ.

To build the Windows GUI on Linux, `./build_gui_windows_docker.sh` runs `wails build` in a `golang:1.25` container and writes `ProxmoxBackupClientGO.exe`. It does not build the service.

## Command-line tools

From the repository root, with `GOOS` set to `windows` or `linux`:

```sh
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/proxmoxbackup-machine ./machinebackup
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/proxmoxbackup-nbd ./nbd        # Linux only
```

and `proxmoxbackup-directory` as shown under [The workspace](#the-workspace). Add `.exe` to the names on Windows.

## Windows installer (MSI)

Build the app and the service first (above), then:

```powershell
cd installer\wix
.\build-msi.ps1                       # version from gui\wails.json
.\build-msi.ps1 -Version 0.10.0 -WixDir C:\Dev\tools\wix314
```

The result is `installer\wix\out\ProxmoxBackupClient-<version>-x64.msi`. The version must be three numbers (`x.y.z`). WiX is looked for in `-WixDir`, then on `PATH`.

## Linux packages

```sh
packaging/debian/build-deb.sh [output-dir]     # .deb, on Debian, Ubuntu or Mint; default output dist/
packaging/fedora/build-rpm.sh [output-dir]     # .rpm, in a fedora:44 Docker container
```

Both take the version from `gui/wails.json`, or from `PBS_VERSION`. The .deb build host needs Go, `dpkg-dev` and the GTK3 and WebKitGTK dev packages. Build release .debs on Ubuntu 22.04 (a container is fine) with `libwebkit2gtk-4.1-dev`: the package then depends on WebKitGTK 4.1 and glibc 2.34, which Ubuntu 22.04 and 24.04, Mint 21 and Debian 12 and 13 all have. A .deb built against WebKitGTK 4.0 (Mint 21's dev package) does not install on Debian 13 or Ubuntu 24.04, and one built on a newer system needs a newer glibc than Mint 21 has; the .rpm build host needs only git and Docker. See [packaging/debian/README.md](packaging/debian/README.md) for what the packages install. `make deb` and `make pkgfedora` call the same scripts.

## Tests

`make test` runs `go test -race` in every module (the separate modules with `GOWORK=off`). `make test-e2e` backs up with the CLIs against a real PBS in Docker and restores with the official client; see `testing/e2e/README.md`.

## Makefile

On Linux (or with GNU make and a POSIX shell on Windows) the Makefile runs the same commands: `make cli` (the command-line tools in `dist/`), `make gui`, then `make service` (in that order: `wails build -clean` empties `gui/build/bin`), `make all` for all three, `make deb`, `make pkgfedora`, `make msi` (Windows), `make test`, `make test-e2e`, `make clean`. `make help` lists them. The version comes from `gui/wails.json`.
