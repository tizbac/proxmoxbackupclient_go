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
and Backup Sets added in the app by an administrator (adding or changing Backup Sets needs
an elevated app while job protection is on, which is the default).

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
  when one machine should own it. This needs a file exported by version 0.9.2 or later
  (older files do not say which computer they came from).
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
msiexec /i ProxmoxBackupClient-0.9.2-x64.msi /qn AUTOSTART=0 CONFIGFILE="\\server\deploy\pbc-settings.json" /l*v C:\Windows\Temp\pbc-install.log
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
