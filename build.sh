#!/bin/bash

export CGO_ENABLED=0
export GOOS=windows
export GOARCH=amd64

go build -o pbsdirectorybackup.exe -ldflags="-s -w" -trimpath ./directorybackup
go build -o pbsmachinebackup.exe -ldflags="-s -w" -trimpath ./machinebackup
