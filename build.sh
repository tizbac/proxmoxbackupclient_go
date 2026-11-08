#!/bin/bash

export CGO_ENABLED=0
export GOOS=windows
export GOARCH=amd64

# The command line: salvanza-cli backup-folder / backup-disk.
go build -o salvanza-cli.exe -ldflags="-s -w" -trimpath ./cli
