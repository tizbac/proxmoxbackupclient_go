# AGENTS.md

This file provides essential context for agents working in this repository to avoid common mistakes and ramp up quickly.

## Project Structure

This is a Go-based backup client for Proxmox Backup Server with multiple binaries:
- `proxmoxbackup-directory` - Directory backup CLI tool
- `proxmoxbackup-machine` - Machine backup CLI tool (Windows)
- `proxmoxbackup-nbd` - NBD server CLI tool (Linux-only)
- `ProxmoxBackupClient` - GUI application (Wails-based)

## Build System

- Uses Makefile for building all components
- Build artifacts are placed in `dist/` directory
- Cross-compilation is supported for Windows, Linux, and macOS
- GUI uses Wails framework with React frontend
- Uses `wails build` for GUI builds

## Key Commands

- `make` or `make all` - Build everything (CLI + GUI + Service)
- `make cli` - Build all CLI tools
- `make gui` - Build GUI application
- `make service` - Build Windows Service
- `make test` - Run all tests with race detector
- `make lint` - Run golangci-lint
- `make security-check` - Run security checks with gosec
- `make msi` - Build the per-brand Windows MSI installers (Docker required)
- `make release` - Prepare release packages

## Important Notes

- The GUI requires Node.js for frontend build (npm install)
- GUI build requires the `wails` CLI. NEVER install it with `@latest`; use the
  pinned `WAILS_VERSION` in the Makefile:
  `go install github.com/wailsapp/wails/v2/cmd/wails@$(WAILS_VERSION)`
- Build process uses security hardening flags
- Windows service requires admin privileges
- Cross-compilation uses GOOS and GOARCH environment variables
- CLI tools are built from respective directories: `directorybackup/`, `machinebackup/`, `nbd/`
- GUI is built from `gui/` directory using wails build

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

- Tests are run with `go test -v -race -coverprofile=coverage.out ./...`
- Coverage report generation: `go tool cover -html=coverage.out -o coverage.html`
- Test coverage is collected in `coverage.out` file

## Development Setup

- Run `make dev-setup` to install dependencies and set up dev environment
- Install frontend dependencies with `npm install` in `gui/frontend/`
- GUI development mode: `make gui-dev` or `wails dev` in gui directory

## Platform-Specific Considerations

- NBD tool is Linux-only (uses Linux ioctl)
- Machine backup is Windows-only
- GUI requires Windows admin privileges for manifest settings
- Cross-compilation to Windows requires `GOOS=windows` environment variable