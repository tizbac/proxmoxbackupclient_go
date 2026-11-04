# Golang VSS CGO-free module
Windows API bindings for the `Volume Shadow Copy Service` in Golang for 32 and 64-bit systems. Enables the user to duplicate entire drives during runtime without any file access issues. The API bindings are accompanied by a simple CLI tool that creates and symlinks Shadow Copies of a given drive. Uses syscalls and go-ole to avoid CGo.
## Build
You can either import the vss API bindings into your project or use the CLI application. The CLI application can be built with the following command:

Cross-compilation is supported for 32 and 64-bit systems. The following commands will build the CLI application for the respective system:
```shell
make build_win32 # for 32-bit systems
make build_win64 # for 64-bit systems
```

Regular build on Windows:
```shell
make build
```

## Usage
```sh
./vss -h
```
```
usage:  vss [options]
  -D string
        Drive letter to copy (example: C:\)
  -S string
        Path of symlink folder
  -f    Creates snapshots if available shadow storage is low. Warning: Replaces older snapshots.
  -timeout int
        Snapshot creation timeout in seconds (min 180) (default 180)
```

## Docs
Official MS Docs: https://docs.microsoft.com/en-us/windows/win32/api/vss/