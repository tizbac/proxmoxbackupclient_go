# Nimbus Backup - Unified Build System
# Builds both CLI and GUI applications

.PHONY: all cli gui clean test help install-deps

# Version from wails.json
VERSION := $(shell grep '"productVersion"' gui/wails.json | cut -d'"' -f4)

# Build directories
BUILD_DIR := dist
CLI_DIR := cmd
GUI_DIR := gui

# Binary names
CLI_DIR_BIN := proxmoxbackup-directory
CLI_MACHINE_BIN := proxmoxbackup-machine
CLI_NBD_BIN := proxmoxbackup-nbd
SERVICE_BIN := NimbusBackupSVC
GUI_BIN := NimbusBackup

# Go build flags (security hardening)
GO_FLAGS := -trimpath -buildmode=pie
LDFLAGS := -s -w -X main.version=$(VERSION) \
	-extldflags '-static-pie -Wl,-z,relro,-z,now'

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
	cd directorybackup && go mod tidy && go build $(GO_FLAGS) -ldflags="$(LDFLAGS)" \
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

# Service Build (Standalone Windows Service)
service:
	@echo "🔧 Building Backup Service..."
	@mkdir -p $(BUILD_DIR)
	@mkdir -p gui/build/bin
	cd cmd/service && go mod tidy && go build $(GO_FLAGS) -ldflags="$(LDFLAGS)" \
		-o ../../gui/build/bin/$(SERVICE_BIN)$(shell go env GOEXE)
	@cp gui/build/bin/$(SERVICE_BIN)$(shell go env GOEXE) $(BUILD_DIR)/ || true
	@echo "✅ Built: gui/build/bin/$(SERVICE_BIN) (for MSI)"
	@echo "✅ Built: $(BUILD_DIR)/$(SERVICE_BIN)"

# GUI Build
gui:
	@echo "🎨 Building GUI application (version $(VERSION))..."
	cd gui && wails build -clean -platform $(shell go env GOOS)/$(shell go env GOARCH) \
		-ldflags "-X main.appVersion=$(VERSION)"
	@mkdir -p $(BUILD_DIR)
	@cp gui/build/bin/$(GUI_BIN)$(shell go env GOEXE) $(BUILD_DIR)/
	@echo "✅ Built: $(BUILD_DIR)/$(GUI_BIN)"

# GUI Development mode
gui-dev:
	@echo "🚀 Starting GUI in development mode..."
	cd gui && wails dev

# Tests
test:
	@echo "🧪 Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "📊 Coverage report:"
	go tool cover -func=coverage.out

test-coverage:
	go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report generated: coverage.html"

# Security checks
security-check:
	@echo "🔒 Running security checks..."
	@which gosec || go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -severity high -confidence high ./...

# Linting
lint:
	@echo "🔍 Running linters..."
	@which golangci-lint || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run ./...

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
