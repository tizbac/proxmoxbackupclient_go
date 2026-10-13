# AGENTS.md

This file provides essential context for agents working in this repository to avoid common mistakes and ramp up quickly.

## Project Structure

This is a Go-based backup client for Proxmox Backup Server with multiple binaries:
- `proxmoxbackup-directory` - Directory (pxar) backup CLI tool, from `directorybackup/`
- `proxmoxbackup-machine` - Machine (whole-disk image) backup CLI tool, Windows (VSS) and Linux (elastio-snap/dattobd), from `machinebackup/`
- `proxmoxbackup-nbd` - NBD server CLI tool (Linux-only), from `nbd/`
- `ProxmoxBackupClient` - GUI application (Wails v2 + React), from `gui/`
- `ProxmoxBackupClientSVC` - the background service: the same `gui/` package built with `-tags service`. It also runs `-list-sets`, `-undelete` and `-rollback` from the command line (on Linux the GUI binary takes these too)

Shared code: `pbscommon` (PBS protocol, pxar, chunking, encryption), `clientcommon`, `machinebackuplib`, `snapshot` (VSS, elastio-snap/dattobd), `pkg/retry`, `pkg/security`. Packaging lives in `installer/wix/` (MSI) and `packaging/` (.deb, .rpm, Arch, Linux launchers and the systemd unit). User manuals are in `docs/manual/`.

## Build System

- `go.work` covers `gui`, `pbscommon`, `clientcommon`, `machinebackuplib`, `machinebackup`, `snapshot` and `nbd`. `directorybackup`, `pkg/retry` and `pkg/security` are standalone modules: build `directorybackup` with a temporary work file that adds it, or test them with `GOWORK=off`
- GUI: `wails build` in `gui/` (Wails v2.13.0). The built frontend in `gui/frontend/dist` is committed, so Node.js is only needed to change the frontend
- Service: `go build -tags service` in `gui/`
- MSI: `installer/wix/build-msi.ps1` (WiX 3) after building the app and the service into `gui/build/bin`
- Linux packages: `packaging/debian/build-deb.sh` and `packaging/fedora/build-rpm.sh`
- The version comes from `gui/wails.json` (`productVersion`) and is passed in with `-ldflags "-X main.appVersion=..."`
- See [BUILD.md](BUILD.md) for the exact commands

## Key Commands

- `make test` - Run all tests with race detector, module by module
- `make test-e2e` - End-to-end tests against a real PBS in Docker (see `testing/e2e/README.md`)
- `make deb`, `make pkgfedora` - Build the .deb or the .rpm
- `make lint` - Run golangci-lint
- `make security-check` - Run security checks with gosec
- `make cli` - Build the CLI tools (`cli-directory` runs `go mod tidy`, which can change `go.mod`)

The Makefile's `all`, `gui`, `service`, `cross-gui-windows` and `release` targets still use the old Nimbus Backup names and a `cmd/service` folder that does not exist; they do not work as they stand. The root `build.bat`, `build_cli.bat`, `build_gui.bat`, `build_gui.sh` and `build.sh` are also out of date.

## Important Notes

- GUI build requires `wails` CLI tool (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0`) on Windows; the Linux packages build the GUI with plain `go build -tags production`
- Build process uses security hardening flags (`-trimpath`, `-s -w`)
- Settings live in `C:\ProgramData\ProxmoxBackupClient\` on Windows; on Linux in `~/.proxmox-backup-guardian/`, or `/var/lib/proxmoxbackupclient/` once the service is installed
- Cross-compilation uses GOOS and GOARCH environment variables
- Live tests are the `gui/zz_*_livetest*_test.go` files; each is skipped unless its environment variables are set (for example `PBS_LIVE_TEST=1`; the skip message names them), and most need a real PBS

## Testing

- `make test` runs `go test -race -covermode=atomic -coverprofile=coverage.out ./...` in each module
- Coverage report generation: `go tool cover -html=coverage.out -o coverage.html`
- Test coverage is collected in a `coverage.out` file per module

## Development Setup

- Install frontend dependencies with `npm install` in `gui/frontend/`
- GUI development mode: `wails dev` in `gui/` (or `make gui-dev`)

## Platform-Specific Considerations

- NBD tool is Linux-only (uses Linux ioctl)
- Machine backup works on Windows (VSS) and Linux; on Linux it refuses to run without root and a loaded elastio-snap or dattobd kernel module
- The system tray and toast notifications are Windows-only
- On Windows, VSS, machine backups and changing Backup Sets (job protection) need an elevated GUI; it offers Restart as administrator (pkexec on Linux)
- The background service runs as LocalSystem on Windows (`ProxmoxBackupClientSVC`) and as root under systemd on Linux (`proxmoxbackupclient.service`); it listens on `127.0.0.1:18765` for the GUI
- Cross-compilation to Windows requires `GOOS=windows` environment variable
