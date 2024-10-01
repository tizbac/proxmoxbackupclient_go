#!/bin/bash

CGO_ENABLED=1
GOOS=windows
CC=x86_64-w64-mingw32-gcc

go build -o proxmoxbackupgo_cli.exe