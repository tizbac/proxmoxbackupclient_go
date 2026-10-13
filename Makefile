# Proxmox Backup Client: build, package and test.
# The same commands as BUILD.md; run from the repository root.

.PHONY: help all cli cli-directory cli-machine cli-nbd gui gui-dev service \
	deb pkgfedora msi test test-e2e lint security-check clean install-deps

# Version from gui/wails.json (productVersion)
VERSION := $(shell grep '"productVersion"' gui/wails.json | cut -d'"' -f4)
GOOS    := $(shell go env GOOS)
EXE     := $(shell go env GOEXE)

BUILD_DIR := dist
CLI_FLAGS := -trimpath -ldflags "-s -w"

help:
	@echo "Proxmox Backup Client $(VERSION)"
	@echo ""
	@echo "  all            the command-line tools, the GUI and the service"
	@echo "  cli            proxmoxbackup-directory, -machine and (Linux) -nbd in $(BUILD_DIR)/"
	@echo "  gui            the GUI, gui/build/bin/ProxmoxBackupClient$(EXE) (wails build)"
	@echo "  service        the background service, gui/build/bin/ProxmoxBackupClientSVC$(EXE)"
	@echo "  gui-dev        the GUI in wails dev mode"
	@echo "  deb            the .deb (packaging/debian/build-deb.sh)"
	@echo "  pkgfedora      the .rpm, in a fedora:44 container (packaging/fedora/build-rpm.sh)"
	@echo "  msi            the Windows installer (installer/wix/build-msi.ps1; build gui and service first)"
	@echo "  test           go test -race in every module"
	@echo "  test-e2e       back up against a real PBS in Docker (testing/e2e/README.md)"
	@echo "  lint, security-check, clean, install-deps"

all: cli gui service

# Command-line tools. directorybackup is not in go.work: build it with a
# temporary work file that adds it.
ifeq ($(GOOS),linux)
cli: cli-directory cli-machine cli-nbd
else
cli: cli-directory cli-machine
endif

cli-directory:
	@mkdir -p $(BUILD_DIR)
	{ cat go.work; printf '\nuse ./directorybackup\n'; } > go.tmpwork
	GOWORK=$(CURDIR)/go.tmpwork CGO_ENABLED=0 go build $(CLI_FLAGS) -o $(BUILD_DIR)/proxmoxbackup-directory$(EXE) ./directorybackup; \
		rc=$$?; rm -f go.tmpwork go.tmpwork.sum; exit $$rc

cli-machine:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(CLI_FLAGS) -o $(BUILD_DIR)/proxmoxbackup-machine$(EXE) ./machinebackup

# NBD is Linux only (Linux ioctls)
cli-nbd:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(CLI_FLAGS) -o $(BUILD_DIR)/proxmoxbackup-nbd ./nbd

gui:
	cd gui && wails build -clean -ldflags "-X main.appVersion=$(VERSION)"

gui-dev:
	cd gui && wails dev

# Build after gui: wails build -clean empties gui/build/bin.
service:
	@mkdir -p gui/build/bin
	cd gui && go build -tags service -trimpath -ldflags "-s -w -X main.appVersion=$(VERSION)" \
		-o build/bin/ProxmoxBackupClientSVC$(EXE) .

deb:
	sh packaging/debian/build-deb.sh $(BUILD_DIR)

pkgfedora:
	sh packaging/fedora/build-rpm.sh $(BUILD_DIR)

msi:
	powershell -NoProfile -ExecutionPolicy Bypass -File installer/wix/build-msi.ps1 -Version $(VERSION)

# Every module that contains Go code and therefore unit tests.
#
# go.work only lists a subset of them: directorybackup, pkg/retry and
# pkg/security are standalone modules and MUST be built with GOWORK=off,
# otherwise the go tool refuses to run ("directory prefix . does not contain
# modules listed in go.work").
WORKSPACE_MODULES  := pbscommon clientcommon machinebackuplib machinebackup snapshot nbd gui
STANDALONE_MODULES := directorybackup pkg/retry pkg/security

test:
	@rc=0; \
	for m in $(WORKSPACE_MODULES); do \
		echo "==> $$m"; \
		( cd $$m && go test -race -covermode=atomic -coverprofile=coverage.out ./... ) || rc=1; \
	done; \
	for m in $(STANDALONE_MODULES); do \
		echo "==> $$m"; \
		( cd $$m && GOWORK=off go test -race -covermode=atomic -coverprofile=coverage.out ./... ) || rc=1; \
	done; \
	echo "Coverage:"; \
	for m in $(WORKSPACE_MODULES); do \
		[ -f $$m/coverage.out ] || continue; \
		printf '  %-22s %s\n' "$$m" "$$( cd $$m && go tool cover -func=coverage.out 2>/dev/null | tail -n1 | tr -s ' \t' ' ' )"; \
	done; \
	for m in $(STANDALONE_MODULES); do \
		[ -f $$m/coverage.out ] || continue; \
		printf '  %-22s %s\n' "$$m" "$$( cd $$m && GOWORK=off go tool cover -func=coverage.out 2>/dev/null | tail -n1 | tr -s ' \t' ' ' )"; \
	done; \
	exit $$rc

# End-to-end tests: backs up with our CLIs against a real PBS in Docker and
# restores with the official proxmox-backup-client. Needs Docker.
test-e2e:
	./testing/e2e/run.sh

security-check:
	@which gosec || go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -severity high -confidence high ./...

lint:
	@which golangci-lint || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run ./...

# gui/frontend/dist is committed and stays.
clean:
	rm -rf $(BUILD_DIR) gui/build/bin go.tmpwork go.tmpwork.sum
	find . -name coverage.out -delete

install-deps:
	go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
	cd gui/frontend && npm install
