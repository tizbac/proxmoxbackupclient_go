set CGO_ENABLED=1
set GOOS=windows
set GOEXPERIMENT=nodwarf5
go build -o proxmoxbackupgo_cli.exe ./directorybackup
