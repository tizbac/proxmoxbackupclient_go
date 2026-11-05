# Building the MSI

The MSI holds three programs from `gui/build/bin`: the app (`salvanza-gui.exe`), the
background service (`salvanza-svc.exe`) and the command line (`salvanza-cli.exe`). Build
all three, then run `build-msi.ps1`.

## 1. Tools

- Go and the Wails CLI (`wails`), as for any build of the app.
- WiX Toolset 3: the `wix314-binaries.zip` from https://github.com/wixtoolset/wix3/releases
  is enough. `build-msi.ps1` looks in `C:\Dev\tools\wix314` first, then on PATH.

## 2. Build the app

```powershell
cd gui
wails build -clean -platform windows/amd64 -ldflags "-X main.appVersion=2.0.0"
```

This writes `gui\build\bin\salvanza-gui.exe`. `-clean` empties `gui\build\bin`, so
build the app before the service and the command line.

## 3. Build the service

From the repository root:

```powershell
go build -tags service -ldflags "-X main.appVersion=2.0.0" -o gui\build\bin\salvanza-svc.exe ./gui
```

The `service` build tag makes the service program (no window, local API on
127.0.0.1:18765). A copy of the app renamed to `salvanza-svc.exe` does not work as a service.

## 4. Build the command line

From `cli\`:

```powershell
$env:GOWORK = 'off'
go build -trimpath -ldflags "-s -w -X main.version=2.0.0" -o ..\gui\build\bin\salvanza-cli.exe .
```

The MSI installs it with the `.cmd` files in `installer\wix\cli-aliases\` (the command
line's earlier names).

## 5. Build the MSI

```powershell
cd installer\wix
.\build-msi.ps1 -Version 2.0.0
```

Options:

| Option | Default |
| ------ | ------- |
| `-Version x.y.z` | `productVersion` from `gui\wails.json`. Must be three numbers. |
| `-WixDir <folder>` | `C:\Dev\tools\wix314`, then PATH. |
| `-OutDir <folder>` | `installer\wix\out` (ignored by git). |

The result is `installer\wix\out\Salvanza-<version>-x64.msi`. The script prints
its size and SHA-256. It runs `candle -arch x64 -dProductVersion=<version>` on `Product.wxs`
and `light` with the WixUI and WixUtil extensions. ICE61 is suppressed on purpose, because
same-version upgrades are allowed.

For a branded installer, see "Branded builds" in [README.md](README.md).

## 6. Test it

As an administrator:

```powershell
# Install silently and log
msiexec /i .\out\Salvanza-2.0.0-x64.msi /qn /l*v install.log

# Check the service
Get-Service salvanza-svc | Select-Object Name, Status, StartType

# Check the programs
Get-ChildItem "C:\Program Files\Salvanza"

# Check the command line is on the PATH (in a new terminal)
salvanza-cli version

# Check the settings folder is locked down (no BUILTIN\Users)
(Get-Acl C:\ProgramData\Salvanza).Access | Select-Object IdentityReference, FileSystemRights
```

Then start the app from the Start menu. Preferences > Advanced > Run as Service should show
the service as installed and running.

Upgrade test: build again with a higher `-Version` and install it over the first. There
should still be one entry in Programs and Features, with the new version, and the service
should be running.

Upgrade from Proxmox Backup Client: install the Salvanza MSI on a computer with Proxmox
Backup Client and some settings. Afterwards there should be one entry in Programs and
Features (Salvanza for Proxmox Backup Server), no `ProxmoxBackupClientSVC` service, and
`C:\ProgramData\Salvanza` should hold the earlier settings and a `pre-salvanza-<date>` copy
(test 8 in [../MSI_UNINSTALL_TEST.md](../MSI_UNINSTALL_TEST.md)).

Remove:

```powershell
msiexec /x .\out\Salvanza-2.0.0-x64.msi /qn                 # keeps the settings
msiexec /x .\out\Salvanza-2.0.0-x64.msi /qn KEEP_CONFIG=0   # deletes them
```

Service logs are in `C:\ProgramData\Salvanza` (`service-service.log`,
`backup-service.log`, and one `backup-<time>-<id>.log` per run).

## CI

A local build with `build-msi.ps1` gives one MSI, `Salvanza-<version>-x64.msi`.

`.github/workflows/build-and-release.yml` does not use `build-msi.ps1`. It installs the WiX
Toolset with Chocolatey and runs `candle` and `light` itself, once per brand, giving one
MSI per brand (Salvanza, Nimbus Backup, Etitech Backup). `make msi` on Linux
(`build_msi_linux.sh`, Docker and Wine) builds the same three, as `<brand>-<version>.msi`
(for example `Salvanza-2.0.0.msi` and `NimbusBackup-2.0.0.msi`).
