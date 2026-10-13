# Deploying the MSI to many computers

This covers silent installs through Group Policy, Intune or a script. Everything here
uses standard `msiexec` options and the MSI's own properties. See [README.md](README.md)
for what the MSI installs.

## What a silent install does and does not do

It does:

- install the app and the service in `C:\Program Files\ProxmoxBackupClient`;
- install and start the service `ProxmoxBackupClientSVC` (LocalSystem, automatic);
- add the Start menu entry and the Programs and Features entry;
- upgrade an older version in place.

It does not configure anything. There is no `CONFIGFILE` property or other way to pass
settings to the MSI. After the install, each computer needs its PBS server, token and
Backup Sets added in the app, by an administrator (adding or changing Backup Sets needs an
elevated app while job protection is on, which is the default).

## Commands

Run as an administrator, or as SYSTEM (GPO, Intune).

```powershell
# Install or upgrade
msiexec /i ProxmoxBackupClient-0.9.0-x64.msi /qn /norestart /l*v C:\Windows\Temp\pbc-install.log

# Without the sign-in Run entry
msiexec /i ProxmoxBackupClient-0.9.0-x64.msi /qn AUTOSTART=0

# Other install folder
msiexec /i ProxmoxBackupClient-0.9.0-x64.msi /qn INSTALLFOLDER="D:\Apps\ProxmoxBackupClient"

# Remove, keeping C:\ProgramData\ProxmoxBackupClient
msiexec /x ProxmoxBackupClient-0.9.0-x64.msi /qn

# Remove and delete the settings
msiexec /x ProxmoxBackupClient-0.9.0-x64.msi /qn KEEP_CONFIG=0
```

`AUTOSTART=1` (the default) writes the Run entry under HKCU of the account that runs the
install. Under GPO or Intune that is SYSTEM, so the entry does nothing useful for the
signed-in user. Use `AUTOSTART=0` there.

Install exit code 0 means success; 3010 means success with a restart needed.

## Group Policy

Computer Configuration > Policies > Software Settings > Software Installation, new
package, "Assigned", pointing at the MSI on a share that Domain Computers can read. To set
`AUTOSTART=0`, use a transform (.mst) made with Orca or a similar tool.

## Intune

Add it as a Line-of-business app (the MSI as is) or as a Win32 app (packed with the
Win32 Content Prep Tool), with:

- Install: `msiexec /i ProxmoxBackupClient-0.9.0-x64.msi /qn AUTOSTART=0`
- Uninstall: `msiexec /x {ProductCode} /qn`
- Detection: the MSI product code, or the file
  `C:\Program Files\ProxmoxBackupClient\ProxmoxBackupClientSVC.exe`.

The product code changes with every build (`Product Id="*"`). The UpgradeCode
`F15C307F-1053-46B7-BCB0-2FAAC3627ED6` stays the same for all versions.

## Checking a computer

```powershell
Get-Service ProxmoxBackupClientSVC                     # Running, Automatic
Test-Path "C:\Program Files\ProxmoxBackupClient\ProxmoxBackupClient.exe"
Get-Content C:\ProgramData\ProxmoxBackupClient\service-service.log -Tail 50
```

`service-service.log` and `backup-service.log` are the service's logs. Each backup also
gets its own `backup-<time>-<id>.log`. These files are readable only by administrators.

## Security notes

- `C:\ProgramData\ProxmoxBackupClient\config.json` holds the PBS API token secrets. The
  service restricts the folder to SYSTEM, Administrators and local administrator accounts
  (read and create) every time it starts. A standard user who opens the app is told the
  settings are open to administrators only.
- The service's local API listens on 127.0.0.1:18765 only and needs the token in
  `C:\ProgramData\ProxmoxBackupClient\api-token`, which the service replaces each time it
  starts. See [../gui/api/INTEGRATION.md](../gui/api/INTEGRATION.md).
- The programs are not code signed. SmartScreen or Defender may warn. Check the SHA-256
  that `build-msi.ps1` prints, or the release's checksums.

## Monitoring

There is no central dashboard. Each computer's Reports page and its notification e-mails
(set per Backup Set) show the results of its backups.
