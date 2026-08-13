# AGENTS.md

This file provides essential context for agents working in this repository to avoid common mistakes and ramp up quickly.

## Project Structure

This is a Go-based backup client for Proxmox Backup Server with multiple binaries:
- `proxmoxbackup-directory` - Directory backup CLI tool
- `proxmoxbackup-machine` - Machine backup CLI tool (Windows)
- `proxmoxbackup-nbd` - NBD server CLI tool (Linux-only)
- `NimbusBackup` - GUI application (Wails-based)

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
- `make release` - Prepare release packages

## Important Notes

- The GUI requires Node.js for frontend build (npm install)
- GUI build requires `wails` CLI tool (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- Build process uses security hardening flags
- Windows service requires admin privileges
- Cross-compilation uses GOOS and GOARCH environment variables
- CLI tools are built from respective directories: `directorybackup/`, `machinebackup/`, `nbd/`
- GUI is built from `gui/` directory using wails build

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