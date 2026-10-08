# Nimbus Backup - Unified Build System
# Builds both CLI and GUI applications

.PHONY: all cli gui deb pkgarch pkgfedora stage-source msi clonezilla clean test help install-deps

# Version from git tag (exact match) or git short SHA
# Falls back to wails.json if not in a git repo
VERSION := $(shell bash scripts/get-version.sh 2>/dev/null || grep '"productVersion"' gui/wails.json | cut -d'"' -f4)

# Build directories
BUILD_DIR := dist
CLI_DIR := cmd
GUI_DIR := gui

# Binary names
CLI_DIR_BIN := proxmoxbackup-directory
CLI_MACHINE_BIN := proxmoxbackup-machine
CLI_NBD_BIN := proxmoxbackup-nbd
SERVICE_BIN := ProxmoxBackupClientSVC
GUI_BIN := ProxmoxBackupClient

# Go build flags (security hardening)
GO_FLAGS := -trimpath -buildmode=pie
LDFLAGS := -s -w -X main.version=$(VERSION) \
	-extldflags '-static-pie -Wl,-z,relro,-z,now'

# Wails desktop frontend tags.
# The "production" tag links the real webkit/gtk cgo backend instead of the
# stub used by `wails dev`. On Linux, "webkit2_41" selects webkit2gtk-4.1,
# which is what modern distros ship; wails otherwise defaults to the EOL
# webkit2gtk-4.0 (and fails to link where 4.0 has been dropped).
# Ignored on Windows/macOS, which use WebView2/WKWebView instead.
WAILS_TAGS := production
ifeq ($(shell go env GOOS),linux)
  ifeq ($(shell pkg-config --exists webkit2gtk-4.1 2>/dev/null && echo yes),yes)
    WAILS_TAGS := $(WAILS_TAGS),webkit2_41
  else
    $(warning webkit2gtk-4.1 not found: building the GUI against webkit2gtk-4.0 (EOL). Install libwebkit2gtk-4.1-dev / webkit2gtk4.1-devel.)
  endif
endif

# Default target
all: cli gui service

help:
	@echo "Nimbus Backup Build System"
	@echo "=========================="
	@echo ""
	@echo "Targets:"
	@echo "  all          - Build everything (CLI + GUI + Service)"
	@echo "  cli          - Build all CLI tools"
	@echo "  gui          - Build GUI application"
	@echo "  service      - Build Windows Service (NimbusBackupSVC.exe)"
	@echo "  deb          - Build the Debian package (pbsgo, Linux)"
	@echo "  pkgarch      - Build the Arch package (pbsgo, on Arch)"
	@echo "  pkgfedora    - Build the Fedora RPM (docker fedora:44)"
	@echo "  stage-source - Stage the source tarball used by pkgarch/pkgfedora"
	@echo "  msi          - Build the Windows MSI installers on Linux (docker + wine)"
	@echo "  clonezilla   - Download + patch the Clonezilla live ISO (not in 'all')"
	@echo "  test         - Run all tests"
	@echo "  clean        - Remove build artifacts"
	@echo "  install-deps - Install build dependencies"
	@echo ""
	@echo "CLI Tools:"
	@echo "  cli-directory - Directory backup CLI"
	@echo "  cli-machine   - Machine backup CLI"
	@echo "  cli-nbd       - NBD server CLI"
	@echo ""
	@echo "Version: $(VERSION)"

# Install build dependencies
install-deps:
	@echo "📦 Installing dependencies..."
	go install github.com/wailsapp/wails/v2/cmd/wails@latest
	cd gui/frontend && npm install

# CLI Builds
# NBD is Linux-only (uses Linux ioctl)
ifeq ($(shell go env GOOS),linux)
cli: cli-directory cli-machine cli-nbd
	@echo "✅ All CLI tools built successfully"
else
cli: cli-directory cli-machine
	@echo "✅ CLI tools built successfully (NBD skipped - Linux only)"
endif

cli-directory:
	@echo "🔨 Building Directory Backup CLI..."
	@mkdir -p $(BUILD_DIR)
	cd directorybackup && GOWORK=off go mod tidy && GOWORK=off go build $(GO_FLAGS) -ldflags="$(LDFLAGS)" \
		-o ../$(BUILD_DIR)/$(CLI_DIR_BIN)$(shell go env GOEXE)
	@echo "✅ Built: $(BUILD_DIR)/$(CLI_DIR_BIN)"

cli-machine:
	@echo "🔨 Building Machine Backup CLI..."
	@mkdir -p $(BUILD_DIR)
	cd machinebackup && go mod tidy && go build $(GO_FLAGS) -ldflags="$(LDFLAGS)" \
		-o ../$(BUILD_DIR)/$(CLI_MACHINE_BIN)$(shell go env GOEXE)
	@echo "✅ Built: $(BUILD_DIR)/$(CLI_MACHINE_BIN)"

cli-nbd:
ifeq ($(shell go env GOOS),linux)
	@echo "🔨 Building NBD Server CLI..."
	@mkdir -p $(BUILD_DIR)
	cd nbd && go mod tidy && go build $(GO_FLAGS) -ldflags="$(LDFLAGS)" \
		-o ../$(BUILD_DIR)/$(CLI_NBD_BIN)$(shell go env GOEXE)
	@echo "✅ Built: $(BUILD_DIR)/$(CLI_NBD_BIN)"
else
	@echo "⏭️  Skipping NBD Server CLI (Linux only)"
endif

# Service Build (systemd service on Linux, Windows Service on Windows)
#
# The default target OS is windows: gui/build/bin/ProxmoxBackupClientSVC.exe is
# what the MSI packages, and a Linux ELF there cannot be started by the Windows
# SCM (error 193 / service timeout 1053). Override with `SERVICE_GOOS=linux
# make service` when a local systemd binary is wanted (the deb/rpm/arch
# packaging scripts build their own).
SERVICE_GOOS ?= windows
SERVICE_GOEXE := $(if $(filter windows,$(SERVICE_GOOS)),.exe,)
ifneq ($(SERVICE_GOOS),$(shell go env GOOS))
SERVICE_BUILD_ENV := GOOS=$(SERVICE_GOOS) GOARCH=amd64 CGO_ENABLED=0
endif

service:
	@echo "🔧 Building Backup Service ($(SERVICE_GOOS))..."
	@mkdir -p $(BUILD_DIR)
	@mkdir -p gui/build/bin
	# Build from gui/ with -tags service (gui/ IS the service package)
	cd gui && GOWORK=off $(SERVICE_BUILD_ENV) go build -tags service $(GO_FLAGS) -ldflags="-s -w -X main.appVersion=$(VERSION)" \
		-o build/bin/$(SERVICE_BIN)$(SERVICE_GOEXE) .
	@cp gui/build/bin/$(SERVICE_BIN)$(SERVICE_GOEXE) $(BUILD_DIR)/ || true
	@echo "✅ Built: gui/build/bin/$(SERVICE_BIN)$(SERVICE_GOEXE) (for MSI / systemd)"
	@echo "✅ Built: $(BUILD_DIR)/$(SERVICE_BIN)$(SERVICE_GOEXE)"

# Debian package (Linux)
deb:
	@sh packaging/debian/build-deb.sh $(BUILD_DIR)

# Arch Linux package (run on Arch, see packaging/arch/PKGBUILD).
# Stages pbsgo-<version>.tar.gz from the CURRENT working tree first, so the
# package contains the source it was built from (not a GitHub commit).
pkgarch:
	@VERSION=$$(bash scripts/get-version.sh) && \
	 sed -i "s/^pkgver=.*/pkgver=$${VERSION}/" packaging/arch/PKGBUILD && \
	 sh packaging/stage-source.sh packaging/arch && \
	 cd packaging/arch && makepkg -f -d

# Source tarball the distro packages are built from (uncommitted changes incl.)
stage-source:
	@sh packaging/stage-source.sh

# Fedora RPM (built in a fedora:44 Docker container)
pkgfedora:
	@sh packaging/fedora/build-rpm.sh $(BUILD_DIR)

# Windows MSI installers built on Linux (Docker + Wine + WiX), one per brand.
# Needs Docker; see installer/wix/build_msi_linux.sh.
msi:
	@echo "🪟 Building MSI installers (version $(VERSION))..."
	@bash installer/wix/build_msi_linux.sh $(BUILD_DIR)

# Download a stock Clonezilla live ISO, inject the PBS-NBD scripts and apply
# clonezilla-patch/patches/*.patch, producing a bootable recovery ISO.
#
# Deliberately NOT part of `all`: it needs network access plus
# 7z/xorriso/squashfs-tools/fakeroot/patch and takes several minutes.
#
#   make clonezilla                       # stable 3.3.3-37 (what the patches target)
#   CLONEZILLA_VER=3.3.4-6 make clonezilla
#   CLONEZILLA_MIRROR=<url> make clonezilla
CLONEZILLA_VER ?= 3.3.3-37
CLONEZILLA_ARCH ?= amd64
# SourceForge mirrors the official releases (osdn.net, the upstream default,
# is frequently unreachable). The path layout is <mirror>/<ver>/<iso>.
CLONEZILLA_MIRROR ?= https://downloads.sourceforge.net/project/clonezilla/clonezilla_live_stable
# Optional: URL of a sha256sum file to verify the downloaded ISO against.
# Empty = skip verification (SourceForge does not publish one).
CLONEZILLA_SHA256 ?=

CLONEZILLA_CACHE    := $(BUILD_DIR)/clonezilla
CLONEZILLA_STOCK    := $(CLONEZILLA_CACHE)/clonezilla-live-$(CLONEZILLA_VER)-$(CLONEZILLA_ARCH).iso
CLONEZILLA_OUT      := $(BUILD_DIR)/clonezilla-pbs-nbd-$(VERSION).iso
CLONEZILLA_PBSNBD   := $(CLONEZILLA_CACHE)/pbsnbd
CLONEZILLA_SCRIPTS  := clonezilla-patch/ocs-pbs-nbd clonezilla-patch/ocs-pbs-bare-metal-restore
CLONEZILLA_PATCHES  := $(wildcard clonezilla-patch/patches/*.patch)
CLONEZILLA_DEPS     := patch-clonezilla.sh $(CLONEZILLA_PATCHES) $(CLONEZILLA_SCRIPTS)

clonezilla: $(CLONEZILLA_OUT)
	@echo "✅ Built: $(CLONEZILLA_OUT)"

# Static binary, same flags as .github/workflows/build-clonezilla-iso.yml so
# it runs on any live environment regardless of the host libc.
$(CLONEZILLA_PBSNBD): $(wildcard nbd/*.go) nbd/go.mod nbd/go.sum
	@echo "🔨 Building pbsnbd (static linux/amd64)..."
	@mkdir -p $(CLONEZILLA_CACHE)
	cd nbd && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" \
		-o ../$(CLONEZILLA_PBSNBD) .

# Stock ISO, downloaded once and cached in dist/clonezilla/.
$(CLONEZILLA_STOCK):
	@echo "⬇️  Downloading Clonezilla live $(CLONEZILLA_VER)-$(CLONEZILLA_ARCH)..."
	@mkdir -p $(CLONEZILLA_CACHE)
	@set -eu; \
	  url="$(CLONEZILLA_MIRROR)/$(CLONEZILLA_VER)/clonezilla-live-$(CLONEZILLA_VER)-$(CLONEZILLA_ARCH).iso"; \
	  echo "   $$url"; \
	  curl -fL --retry 3 -o "$@.tmp" "$$url"; \
	  if [ -n "$(CLONEZILLA_SHA256)" ]; then \
	    echo "   verifying sha256"; \
	    curl -fL --retry 3 -o "$(CLONEZILLA_CACHE)/sha256sum.txt" "$(CLONEZILLA_SHA256)"; \
	    grep "clonezilla-live-$(CLONEZILLA_VER)-$(CLONEZILLA_ARCH).iso" "$(CLONEZILLA_CACHE)/sha256sum.txt" | (cd "$(CLONEZILLA_CACHE)" && sha256sum -c -); \
	  else \
	    echo "   warning: CLONEZILLA_SHA256 not set, skipping checksum verification"; \
	  fi; \
	  mv "$@.tmp" "$@"

$(CLONEZILLA_OUT): $(CLONEZILLA_STOCK) $(CLONEZILLA_PBSNBD) $(CLONEZILLA_DEPS)
	@command -v 7z >/dev/null || { echo "error: 7z not installed (apt install 7zip p7zip-full)" >&2; exit 1; }
	@command -v xorriso >/dev/null || { echo "error: xorriso not installed (apt install xorriso)" >&2; exit 1; }
	@command -v unsquashfs >/dev/null || { echo "error: unsquashfs not installed (apt install squashfs-tools)" >&2; exit 1; }
	@command -v fakeroot >/dev/null || { echo "error: fakeroot not installed (apt install fakeroot)" >&2; exit 1; }
	@test -f /usr/lib/ISOLINUX/isohdpfx.bin || { echo "error: isohybrid MBR not found (apt install isolinux)" >&2; exit 1; }
	@echo "🧩 Patching Clonezilla ISO (PBS NBD scripts + main/first-boot menus)..."
	PATCH_DIR="$(abspath clonezilla-patch/patches)" ISO_SQFS=/live/filesystem.squashfs \
		./patch-clonezilla.sh -o "$@" "$(CLONEZILLA_STOCK)" \
		"$(CLONEZILLA_PBSNBD)" $(CLONEZILLA_SCRIPTS)
	@echo "✅ Built: $@"

# GUI Build
gui:
	@echo "🎨 Building GUI application (version $(VERSION))..."
	cd gui && wails build -clean -platform $(shell go env GOOS)/$(shell go env GOARCH) \
		-tags "$(WAILS_TAGS)" \
		-ldflags "-X main.appVersion=$(VERSION)"
	@mkdir -p $(BUILD_DIR)
	@cp gui/build/bin/$(GUI_BIN)$(shell go env GOEXE) $(BUILD_DIR)/
	@echo "✅ Built: $(BUILD_DIR)/$(GUI_BIN)"

# GUI Development mode
gui-dev:
	@echo "🚀 Starting GUI in development mode..."
	cd gui && wails dev -tags "$(WAILS_TAGS)"

# Every module that contains Go code and therefore unit tests.
#
# go.work only lists a subset of them: directorybackup, pkg/retry and
# pkg/security are standalone modules and MUST be built with GOWORK=off,
# otherwise the go tool refuses to run ("directory prefix . does not contain
# modules listed in go.work").
WORKSPACE_MODULES  := pbscommon clientcommon machinebackuplib machinebackup snapshot nbd gui gui/api
STANDALONE_MODULES := directorybackup pkg/retry pkg/security

# Tests
#
# Coverage is only collected for modules that really contain test files: for a
# package WITHOUT tests `go test -coverprofile` still instruments it and shells
# out to $GOTOOLDIR/covdata, a tool the Go 1.25+ toolchain archives no longer
# ship prebuilt. That would fail the run of a module that has nothing to
# measure (machinebackup). Such modules are still compiled and vetted.
test:
	@echo "🧪 Running tests..."
	@rc=0; \
	run_module_tests() { \
		m="$$1"; gowork="$$2"; \
		if find "$$m" -name '*_test.go' -not -path '*/vendor/*' | grep -q .; then \
			echo "==> $$m"; \
			( cd "$$m" && GOWORK="$$gowork" go test -race -covermode=atomic -coverprofile=coverage.out ./... ) || rc=1; \
		else \
			echo "==> $$m (no test files: compile check only)"; \
			rm -f "$$m/coverage.out"; \
			( cd "$$m" && GOWORK="$$gowork" go test -race ./... ) || rc=1; \
		fi; \
	}; \
	for m in $(WORKSPACE_MODULES); do \
		run_module_tests "$$m" auto; \
	done; \
	for m in $(STANDALONE_MODULES); do \
		run_module_tests "$$m" off; \
	done; \
	echo "📊 Coverage:"; \
	for m in $(WORKSPACE_MODULES); do \
		[ -f $$m/coverage.out ] || continue; \
		printf '  %-22s %s\n' "$$m" "$$( cd $$m && go tool cover -func=coverage.out 2>/dev/null | tail -n1 | tr -s ' \t' ' ' )"; \
	done; \
	for m in $(STANDALONE_MODULES); do \
		[ -f $$m/coverage.out ] || continue; \
		printf '  %-22s %s\n' "$$m" "$$( cd $$m && GOWORK=off go tool cover -func=coverage.out 2>/dev/null | tail -n1 | tr -s ' \t' ' ' )"; \
	done; \
	exit $$rc

test-coverage:
	go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report generated: coverage.html"

# End-to-end tests: backs up with our CLIs against a real PBS in Docker and
# restores with the official proxmox-backup-client. Needs Docker; see
# testing/e2e/README.md.
test-e2e:
	@echo "🧪 Running end-to-end tests against PBS..."
	./testing/e2e/run.sh

.PHONY: test-e2e

# Security checks
security-check:
	@echo "🔒 Running security checks..."
	@which gosec || go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -severity high -confidence high ./...

# Linting
lint:
	@echo "🔍 Running linters..."
	@command -v golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@lint_bin=$$(command -v golangci-lint || echo "$$HOME/go/bin/golangci-lint"); \
	rc=0; \
	for m in $(WORKSPACE_MODULES); do \
		echo "==> $$m"; \
		( cd "$$m" && GOWORK=auto "$$lint_bin" run ./... ) || rc=1; \
	done; \
	for m in $(STANDALONE_MODULES); do \
		echo "==> $$m"; \
		( cd "$$m" && GOWORK=off "$$lint_bin" run ./... ) || rc=1; \
	done; \
	exit $$rc

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf gui/build
	rm -rf gui/frontend/dist
	rm -f coverage.out coverage.html
	@echo "✅ Clean complete"

# Cross-compile for all platforms
cross-compile: cross-cli-windows cross-cli-linux cross-cli-macos cross-gui-windows
	@echo "✅ All cross-compilation complete"

cross-cli-windows:
	@echo "🪟 Cross-compiling CLI for Windows..."
	@mkdir -p $(BUILD_DIR)/windows
	GOOS=windows GOARCH=amd64 cd directorybackup && go build $(GO_FLAGS) -ldflags="$(LDFLAGS)" \
		-o ../$(BUILD_DIR)/windows/$(CLI_DIR_BIN).exe
	GOOS=windows GOARCH=amd64 cd machinebackup && go build $(GO_FLAGS) -ldflags="$(LDFLAGS)" \
		-o ../$(BUILD_DIR)/windows/$(CLI_MACHINE_BIN).exe

cross-cli-linux:
	@echo "🐧 Cross-compiling CLI for Linux..."
	@mkdir -p $(BUILD_DIR)/linux
	GOOS=linux GOARCH=amd64 cd directorybackup && go build $(GO_FLAGS) -ldflags="$(LDFLAGS)" \
		-o ../$(BUILD_DIR)/linux/$(CLI_DIR_BIN)
	GOOS=linux GOARCH=amd64 cd machinebackup && go build $(GO_FLAGS) -ldflags="$(LDFLAGS)" \
		-o ../$(BUILD_DIR)/linux/$(CLI_MACHINE_BIN)

cross-cli-macos:
	@echo "🍎 Cross-compiling CLI for macOS..."
	@mkdir -p $(BUILD_DIR)/macos
	GOOS=darwin GOARCH=amd64 cd directorybackup && go build $(GO_FLAGS) -ldflags="$(LDFLAGS)" \
		-o ../$(BUILD_DIR)/macos/$(CLI_DIR_BIN)
	GOOS=darwin GOARCH=arm64 cd directorybackup && go build $(GO_FLAGS) -ldflags="$(LDFLAGS)" \
		-o ../$(BUILD_DIR)/macos/$(CLI_DIR_BIN)-arm64

cross-gui-windows:
	@echo "🪟🎨 Cross-compiling GUI for Windows..."
	cd gui && wails build -clean -platform windows/amd64
	@mkdir -p $(BUILD_DIR)/windows
	@cp gui/build/bin/$(GUI_BIN).exe $(BUILD_DIR)/windows/

# Release preparation
release: clean security-check lint test cross-compile
	@echo "📦 Preparing release v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)/release
	cd $(BUILD_DIR) && tar -czf release/nimbus-backup-cli-v$(VERSION)-linux.tar.gz linux/
	cd $(BUILD_DIR) && zip -r release/nimbus-backup-cli-v$(VERSION)-windows.zip windows/*.exe
	cd $(BUILD_DIR) && tar -czf release/nimbus-backup-cli-v$(VERSION)-macos.tar.gz macos/
	cd $(BUILD_DIR) && zip -r release/NimbusBackup-v$(VERSION)-windows.zip windows/$(GUI_BIN).exe
	@echo "✅ Release packages ready in $(BUILD_DIR)/release/"
	@ls -lh $(BUILD_DIR)/release/

# Development setup
dev-setup: install-deps
	@echo "🔧 Setting up development environment..."
	go mod download
	cd gui/frontend && npm install
	@echo "✅ Development environment ready"

.PHONY: gui-dev cross-compile cross-cli-windows cross-cli-linux cross-cli-macos cross-gui-windows release dev-setup security-check lint test-coverage
