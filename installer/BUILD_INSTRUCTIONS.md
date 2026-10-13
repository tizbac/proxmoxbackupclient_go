# Building the MSI

The MSI holds two programs from `gui/build/bin`: the app and the background service.
Build both, then run `build-msi.ps1`.

## 1. Tools

- Go and the Wails CLI (`wails`), as for any build of the app.
- WiX Toolset 3: the `wix314-binaries.zip` from https://github.com/wixtoolset/wix3/releases
  is enough. `build-msi.ps1` looks in `C:\Dev\tools\wix314` first, then on PATH.

## 2. Build the app

```powershell
cd gui
wails build -clean -platform windows/amd64 -ldflags "-X main.appVersion=0.9.0"
```

This writes `gui\build\bin\ProxmoxBackupClient.exe`. `-clean` empties `gui\build\bin`, so
build the app before the service.

## 3. Build the service

From the repository root:

```powershell
go build -tags service -ldflags "-X main.appVersion=0.9.0" -o gui\build\bin\ProxmoxBackupClientSVC.exe ./gui
```

The `service` build tag makes the service program (no window, local API on
127.0.0.1:18765). A copy of the app renamed to `...SVC.exe` does not work as a service.

## 4. Build the MSI

```powershell
cd installer\wix
.\build-msi.ps1 -Version 0.9.0
```

Options:

| Option | Default |
| ------ | ------- |
| `-Version x.y.z` | `productVersion` from `gui\wails.json`. Must be three numbers. |
| `-WixDir <folder>` | `C:\Dev\tools\wix314`, then PATH. |
| `-OutDir <folder>` | `installer\wix\out` (ignored by git). |

The result is `installer\wix\out\ProxmoxBackupClient-<version>-x64.msi`. The script prints
its size and SHA-256. It runs `candle -arch x64 -dProductVersion=<version>` on `Product.wxs`
and `light` with the WixUI and WixUtil extensions. ICE61 is suppressed on purpose, because
same-version upgrades are allowed.

For a branded installer, see "Branded builds" in [README.md](README.md).

## 5. Test it

As an administrator:

```powershell
# Install silently and log
msiexec /i .\out\ProxmoxBackupClient-0.9.0-x64.msi /qn /l*v install.log

# Check the service
Get-Service ProxmoxBackupClientSVC | Select-Object Name, Status, StartType

# Check the programs
Get-ChildItem "C:\Program Files\ProxmoxBackupClient"

# Check the settings folder is locked down (no BUILTIN\Users)
(Get-Acl C:\ProgramData\ProxmoxBackupClient).Access | Select-Object IdentityReference, FileSystemRights
```

Then start the app from the Start menu. Preferences > Advanced > Run as Service should show
the service as installed and running.

Upgrade test: build again with a higher `-Version` and install it over the first. There
should still be one entry in Programs and Features, with the new version, and the service
should be running.

Remove:

```powershell
msiexec /x .\out\ProxmoxBackupClient-0.9.0-x64.msi /qn                 # keeps the settings
msiexec /x .\out\ProxmoxBackupClient-0.9.0-x64.msi /qn KEEP_CONFIG=0   # deletes them
```

Service logs are in `C:\ProgramData\ProxmoxBackupClient` (`service-service.log`,
`backup-service.log`, and one `backup-<time>-<id>.log` per run).

## CI

`.github/workflows/build-and-release.yml` also builds an MSI, with `build-msi.ps1` and the
WiX Toolset that Chocolatey installs, and uploads it as `ProxmoxBackupClient.msi`.
