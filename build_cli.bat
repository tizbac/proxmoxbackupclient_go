set CGO_ENABLED=1
set GOOS=windows
set GOEXPERIMENT=nodwarf5
go build -o salvanza-cli.exe ./cli
