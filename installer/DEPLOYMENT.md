# Deploying the MSI to many computers

This covers silent installs through Group Policy, Intune or a script. Everything here
uses standard `msiexec` options and the MSI's own properties. See [README.md](README.md)
for what the MSI installs.

## What a silent install does and does not do

It does:

- install the app and the service in `C:\Program Files\ProxmoxBackupClient`;
- install and start the service `ProxmoxBackupClientSVC` (LocalSystem, automatic);
- add the Start menu entry and the Programs and Features entry;
- upgrade an older version in place;
- with `CONFIGFILE`, set up the PBS servers and Backup Sets (next section).

Without `CONFIGFILE` it configures nothing: each computer then needs its PBS server, token
and Backup Sets added in the app by an administrator (the app asks for administrator rights
before it shows or changes any setting).

## Settings for every computer (CONFIGFILE)

`CONFIGFILE` is the full path of a settings file made with **Preferences > Export**. Before
the service starts, the installer runs the service program (as SYSTEM) to merge the file
into `C:\ProgramData\ProxmoxBackupClient`, the way **Preferences > Import** does:

- PBS servers and Backup Sets are added, or replaced when one with the same ID is already
  there. Anything else already on the computer stays, and so does a token, password or key
  passphrase the computer remembers for a server the file has without one.
- The settings folder is locked down to administrators first, and tokens and passwords are
  encrypted for the computer (DPAPI) as they are written, like ones entered in the app. The
  file is read where it is; no copy of it is made.
- Backup IDs made from the name of the computer the file was exported from are made from
  each computer's own name instead: that name itself becomes an empty backup ID (the
  computer's name when it runs), and `<that name>-<set>`, as the Backup Set editor
  suggests, becomes `<this computer>-<set>`. Without that, every computer would back up into
  the same backup groups on PBS. Other backup IDs are kept, so give a set a fixed ID only
  when one machine should own it. This needs a file exported by version 2.0 or later
  (test builds' files may not say which computer they came from).
- The file's default server becomes the computer's default only if it has none.
- The Message Log says "Settings from the installer applied (PBS servers: N, Backup Sets:
  N)", or why not.

### Making the file

The easy way: set up one computer the way every computer should be, then **Preferences >
Export**, with **Include passwords/tokens in the export** ticked (without it a server new to
the computer arrives with no token, and its backups fail until one is entered).

Or start from [deploy-settings.example.json](deploy-settings.example.json): one PBS server
and a daily Backup Set of `C:\Users` with metadata change detection. Replace the server
address, its certificate fingerprint (PBS: Dashboard > Show Fingerprint), the token ID and
secret, and the datastore, and change the set to suit. Fields starting with `_` are
ignored.

The file holds the token in plain text, and every computer it is deployed to gets the
same token. Use a token made for this:

- give it only the **DatastoreBackup** role, on its own namespace if the datastore holds
  other backups. With it, each computer can back up and read back its own backup groups,
  but cannot delete snapshots, and cannot read other owners' groups;
- keep in mind that all the computers share that token, so they all own each other's
  groups: an administrator of one of them can list and restore the others' backups. Where
  that matters, deploy without the token and enter one per computer;
- keep the file where only the computers that need it can read it (on a share: Domain
  Computers, read only), and remove it once the computers are set up.

Not in the file:

- **Encryption keys.** A server's key file is named by its path on the exporting
  computer. Copy the key file to the same path on each computer first (for example with
  the same GPO), or leave encryption off in the file.
- **Machine backups stored as Proxmox VE `vm` snapshots.** Their backup ID is a number the
  editor works out for one computer, so every computer would use the same `vm/<number>`.
  Use `host` machine sets (the default) in a file meant for many computers.

### Commands

```powershell
msiexec /i ProxmoxBackupClient-2.0.0.msi /qn CONFIGFILE="\\server\deploy\pbc-settings.json" /l*v C:\Windows\Temp\pbc-install.log
```

Give the full path: a relative one is not found. The install runs as SYSTEM under GPO and
Intune, so a file on a share must be readable by the computer accounts (Domain Computers).
If the file cannot be read or is not a settings export, the install stops and is rolled
back (exit code 1603). The program and service are then removed again, but a merge that
had already happened stays.

Running the command again on a computer that has the same version installed merges the
file again (to send a changed file); the service uses the new settings from its next
backup on.

### Checking it worked

```powershell
Get-Content C:\ProgramData\ProxmoxBackupClient\service-service.log | Select-String Deploy
```

Or open the app's Message Log.

## Commands

Run as an administrator, or as SYSTEM (GPO, Intune).

```powershell
# Install or upgrade
msiexec /i ProxmoxBackupClient-2.0.0.msi /qn /norestart /l*v C:\Windows\Temp\pbc-install.log

# With the sign-in Run entry (the app starts in the tray for every user)
msiexec /i ProxmoxBackupClient-2.0.0.msi /qn AUTOSTART=1

# Other install folder
msiexec /i ProxmoxBackupClient-2.0.0.msi /qn INSTALLFOLDER="D:\Apps\ProxmoxBackupClient"

# Remove, keeping C:\ProgramData\ProxmoxBackupClient
msiexec /x ProxmoxBackupClient-2.0.0.msi /qn

# Remove and delete the settings
msiexec /x ProxmoxBackupClient-2.0.0.msi /qn KEEP_CONFIG=0
```

The app does not start at sign-in unless asked: the service runs the schedules either
way. `AUTOSTART=1` writes the Run entry under HKLM, so the app starts in the tray for every
user who signs in, also when GPO or Intune installed it as SYSTEM. The app in the tray does
not ask for administrator rights at sign-in; it asks when a user opens its window, and lets
go of the service's token again once the window has been back in the tray for 15 minutes.
Without the token the tray still shows whether backups are running and whether the last
one failed.

An upgrade without `AUTOSTART` keeps what the installed version did (the choice is
remembered under `HKLM\Software\Proxmox Backup Client\ProxmoxBackupClient`, value
`AutoStart`; a version without that value counts as on when it had the Run entry). Give
`AUTOSTART=0` or `AUTOSTART=1` to change it.

Install exit code 0 means success; 3010 means success with a restart needed.

## Group Policy

Computer Configuration > Policies > Software Settings > Software Installation, new
package, "Assigned", pointing at the MSI on a share that Domain Computers can read. To set
an option such as `AUTOSTART=1`, use a transform (.mst) made with Orca or a similar tool.

## Intune

Add it as a Line-of-business app (the MSI as is) or as a Win32 app (packed with the
Win32 Content Prep Tool), with:

- Install: `msiexec /i ProxmoxBackupClient-2.0.0.msi /qn`
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
gets its own `backup-<time>-<id>.log`. These files are readable only by SYSTEM and
elevated administrators: run PowerShell as administrator to read them. Each user's app
logs are in `%LOCALAPPDATA%\ProxmoxBackupClient\Logs`.

## Security notes

- `C:\ProgramData\ProxmoxBackupClient\config.json` holds the PBS API token secrets
  (encrypted for the computer with DPAPI). The service restricts the folder to SYSTEM and
  Administrators every time it starts, so only the service and elevated programs can open
  it. Earlier builds also let each local administrator account read it without elevation;
  the service removes those entries.
- The service's local API listens on 127.0.0.1:18765 only and needs the token in
  `C:\ProgramData\ProxmoxBackupClient\api-token` (SYSTEM full control, Administrators
  read). The service writes a new token every time it starts and replaces it every 24 hours
  while it runs. The app gets
  it by asking for administrator rights (UAC) once, keeps it in memory only while its
  window is in use (it drops it after 15 minutes in the tray), and asks again after it is
  replaced or dropped. A user who cannot give an administrator's password cannot see or
  change the settings in the app; the service still runs the Backup Sets on schedule. See
  [../gui/api/INTEGRATION.md](../gui/api/INTEGRATION.md).
- The policy **Require administrator rights to add, change or delete Backup Sets** is gone:
  with the service, every change already needs administrator rights. A `RequireAdminForJobs`
  value left under `HKLM\SOFTWARE\ProxmoxBackupClient\Policy` is ignored.
- The programs are not code signed. SmartScreen or Defender may warn. Check the SHA-256
  that `build-msi.ps1` prints, or the release's checksums.

## Monitoring

There is no central dashboard. Each computer's Reports page and its notification e-mails
(set per Backup Set) show the results of its backups.
