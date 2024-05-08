# Build Instructions

## Install Chocolatey <https://chocolatey.org/install>

- put this in an elevated "admin" powershell

```powershell
Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
```

- close the powershell
- open an elevated "admin" powershell or cmd

```powershell
choco install go
choco install mingw
```

## Test go / gcc

- close the powershell
- open a non elevated powershell or cmd

```cmd
C:\>go version
go version go1.22.2 windows/amd64

C:\>gcc --version
gcc (x86_64-posix-seh-rev0, Built by MinGW-Builds project) 13.2.0
Copyright (C) 2023 Free Software Foundation, Inc.
This is free software; see the source for copying conditions.  There is NO
warranty; not even for MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
```

## Build

- open a non elevated powershell or cmd

GUI version

```cmd
build.bat
```

CLI version

```cmd
build_cli.bat
```
