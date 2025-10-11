set CGO_ENABLED=1
set GOOS=windows
set GOEXPERIMENT=nodwarf5
go build -o proxmoxbackupgomachine_cli.exe ./machinebackup
