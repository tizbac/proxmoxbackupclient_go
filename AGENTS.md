# AGENTS.md

This file provides essential context for agents working in this repository to avoid common mistakes and ramp up quickly.

## Project Structure

This is Salvanza for Proxmox Backup Server (formerly Proxmox Backup Client), a Go-based backup client for Proxmox Backup Server, module path `github.com/salvanza/salvanza_client`. Project site https://salvanza.org/, repository https://github.com/salvanza/salvanza_client. Binaries:
- `salvanza-cli` - the command line, from `cli/`: one program with the subcommands `backup-folder` (folder/pxar backups, the library in `directorybackup/`), `backup-disk` (whole-disk image backups, Windows VSS and Linux elastio-snap/dattobd, `machinebackup/`) and `attach-nbd` (NBD server, Linux only, `nbd/`). Started under an earlier tool name (`proxmoxbackup-directory`, `-machine`, `-nbd`, `pbsdirectorybackup`, ...) it runs that subcommand; the Linux packages install those names as links and the MSI as `.cmd` files (`installer/wix/cli-aliases/`)
- `salvanza-gui` - GUI application (Wails v2 + React), from `gui/`
- `salvanza-svc` - the background service: the same `gui/` package built with `-tags service`. It also runs `-list-sets`, `-undelete` and `-rollback` from the command line (on Linux the GUI binary takes these too)
- `pbsnbd` - the nbd code on its own (`nbd/cmd/pbsnbd`), for the bare-metal restore ISO only

The brand comes from the program's file name (`gui/brand.go`, `gui/identity.go`): `salvanza-gui`/`salvanza-svc` is Salvanza, `nimbusbackup-*` Nimbus Backup and `etitechbackup-*` Etitech Backup, each with its own folders (`C:\ProgramData\<Folder>`, `~/.<folder>`). Settings of the names before the rename (`ProxmoxBackupClient`, `~/.proxmox-backup-guardian`, `/var/lib/proxmoxbackupclient`) are taken over once by `adoptFolder`.

Shared code: `pbscommon` (PBS protocol, pxar, chunking, encryption), `clientcommon`, `machinebackuplib`, `snapshot` (VSS, elastio-snap/dattobd), `pkg/retry`, `pkg/security`. Packaging lives in `installer/wix/` (MSI) and `packaging/` (.deb, .rpm, Arch, Linux launchers and the systemd unit). User manuals are in `docs/manual/`.

## Build System

- `go.work` covers `gui`, `gui/api`, `cli`, `pbscommon`, `clientcommon`, `machinebackuplib`, `machinebackup`, `snapshot` and `nbd`. `directorybackup`, `pkg/retry` and `pkg/security` are standalone modules: test them with `GOWORK=off`
- Command line: `GOWORK=off go build` in `cli/` (its `go.mod` points at the sibling modules), or `make cli`
- GUI: `wails build` in `gui/` (Wails v2.13.0). The built frontend in `gui/frontend/dist` is committed, so Node.js is only needed to change the frontend
- Service: `go build -tags service` in `gui/`
- MSI: `installer/wix/build-msi.ps1` (WiX 3) after building the app, the service and `salvanza-cli.exe` into `gui/build/bin`
- Linux packages: `packaging/debian/build-deb.sh` and `packaging/fedora/build-rpm.sh`
- The version comes from `gui/wails.json` (`productVersion`) and is passed in with `-ldflags "-X main.appVersion=..."`
- See [BUILD.md](BUILD.md) for the exact commands

## Key Commands

- `make test` - Run all tests with race detector, module by module
- `make test-e2e` - End-to-end tests against a real PBS in Docker (see `testing/e2e/README.md`)
- `make deb`, `make pkgfedora`, `make pkgarch` - Build the .deb, the .rpm or the Arch package (`pkgarch` on Arch)
- `make lint` - Run golangci-lint
- `make security-check` - Run security checks with gosec
- `make cli` - Build `salvanza-cli` into `dist/`
- `make gui`, then `make service` - The GUI and the background service into `gui/build/bin` (`make all` does the command line, the GUI and the service)
- `make msi` - The Windows installers, one per brand, built on Linux with Docker and Wine (`installer/wix/build_msi_linux.sh`); on Windows use `installer/wix/build-msi.ps1` (see BUILD.md)
- `make clonezilla` - The bare-metal recovery ISO (see PATCH-CLONEZILLA.md); not part of `make all`

## Important Notes

- GUI build requires `wails` CLI tool (`go install github.com/wailsapp/wails/v2/cmd/wails@$(WAILS_VERSION)`) on Windows; the Linux packages build the GUI with plain `go build -tags production`. Never install it with `@latest`: `WAILS_VERSION` in the Makefile is pinned to v2.13.0 (matching `gui/go.mod`), because newer releases such as v2.17+ declare `go 1.26`
- Build process uses security hardening flags (`-trimpath`, `-s -w`)
- The app's Help page embeds copies of `docs/manual/GUI.md`, `CLI.md`, `BMR.md` and `docs/RESTORE_GUIDE.md` from `gui/docs/` (go:embed cannot reach outside `gui/`). Edit the files in `docs/`, then copy them over by hand (no build step does it); `TestEmbeddedManualsMatchDocs` fails while they differ, and `node src/manuals.test.mjs` in `gui/frontend` checks every link between the manuals
- With the background service, the computer's settings live in its folder, `C:\ProgramData\Salvanza\` on Windows or `/var/lib/salvanza/` on Linux, which only the service, SYSTEM/root and elevated administrators can open; the app reaches them through the service's API (`gui/api/INTEGRATION.md`, `gui/service_connect.go`). Without a service, and for its own files, the app uses `~/.salvanza/` (`getConfigDir`). A branded build uses its brand's folders instead
- Cross-compilation uses GOOS and GOARCH environment variables
- Live tests are the `gui/zz_*_livetest*_test.go` files; each is skipped unless its environment variables are set (for example `PBS_LIVE_TEST=1`; the skip message names them), and most need a real PBS

## Windows MSI builds (`make msi`)

- `make msi` runs `installer/wix/build_msi_linux.sh`, which builds the GUI and
  service inside a `golang:1.25` Docker container, then runs WiX under Wine.
  The host needs Docker only; Go, Node, and Wine are not used.
- The Wails CLI inside that container is pinned by `WAILS_CLI_VERSION` in
  `installer/wix/build_msi_linux.sh`. Do not change it to `@latest`: newer
  Wails releases (v2.17+) declare `go 1.26`, while the container image ships
  Go 1.25 with `GOTOOLCHAIN=local`, so the build aborts with
  `requires go >= 1.26.0`.
- Keep `WAILS_CLI_VERSION` (script) and `WAILS_VERSION` (Makefile) in sync with
  each other and compatible with `gui/go.mod` and the `golang:*` image. When
  bumping either, actually run `make msi` before committing.
- The script reuses `gui/build/bin/ProxmoxBackupClient.exe` and
  `ProxmoxBackupClientSVC.exe` when they are newer than their sources, and
  rebuilds them otherwise. Never stage host-built (Linux ELF) binaries: the
  service must be a Windows PE or the installed service fails with 193/1053.
- MSIs are written to `dist/<Brand>-<version>.msi` for the brands listed in
  `installer/wix/ProductBody.wxi`.
- The WiX/Wine image is rebuilt on every run (`docker build`), so Dockerfile
  changes take effect without a manual `docker build`.
- Build outputs (`gui/frontend/wailsjs/`, `gui/frontend/dist/`,
  `gui/build/bin/`) are regenerated and may be rewritten by the build; do not
  commit mode-only or generated churn unless the Go API actually changed.

## Testing

- `make test` runs `go test -race -covermode=atomic -coverprofile=coverage.out ./...` in each module
- Coverage report generation: `go tool cover -html=coverage.out -o coverage.html`
- Test coverage is collected in a `coverage.out` file per module

## Development Setup

- Install frontend dependencies with `npm install` in `gui/frontend/`
- GUI development mode: `wails dev` in `gui/` (or `make gui-dev`)

## Platform-Specific Considerations

- `salvanza-cli attach-nbd` is Linux-only (uses Linux ioctl); elsewhere it stops with a message
- Machine backup works on Windows (VSS) and Linux; on Linux it refuses to run without root and a loaded elastio-snap or dattobd kernel module
- The system tray and toast notifications are Windows-only
- Without the service, VSS and machine backups need an elevated GUI; it offers Restart as administrator (pkexec on Linux). With the service, the app needs the service's token, which it gets once through UAC or pkexec (Connect)
- The background service runs as LocalSystem on Windows (`salvanza-svc`) and as root under systemd on Linux (`salvanza.service`); it listens on `127.0.0.1:18765` for the GUI, which sends its token in the `X-Salvanza-Token` header (`X-Proxmox-Client-Token` is still accepted)
- Each brand's installer and service remove the earlier installs (Proxmox Backup Client and the other brands): see `installer/wix/ProductBody.wxi` and `gui/legacy_services_windows.go`
- Cross-compilation to Windows requires `GOOS=windows` environment variable
