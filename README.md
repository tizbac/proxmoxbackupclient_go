This software implements a proxmox backup client software for windows, backup only as of now
Works on linux too especially for development

The software is still alpha quality and i take no responsability for any kind of damage or data loss even of source files.

Contributions are welcome especially 

1. GUI with tray icon to show backup progress and backup taking place
2. Encryption support
3. A GUI way of configuring it and maybe create a json job file similiar freefilesync does
4. Async upload / compress and multicore upload + compression of chunks
5. Proxmox side patch to add another kind of entry to pxar format with Windows security descriptors in it
6. Support for windows symlinks
7. Anything interesting you can come up with :)

Usage - Directory Backup
=====

A typical command would look like:

```shell
proxmoxbackupgo.exe -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -backupdir "C:\path\to\backup" -datastore "datastorename"
```

```
proxmoxbackupgo.exe
  -authid string
        Authentication ID (PBS Api token)
  -secret string
        Secret for authentication
  -backupdir string
        Backup source directory, must not be symlink
  -baseurl string
        Base URL for the proxmox backup server, example: https://192.168.1.10:8007
  -certfingerprint string
        Certificate fingerprint for SSL connection, example: ea:7d:06:f9...
  -datastore string
        Datastore name
  -namespace string
        Namespace (optional)
  -backup-id string
        Backup ID (optional - if not specified, the hostname is used as the default for host-type backups)
  -pxarout string
        Output PXAR archive for debug purposes (optional)
  -backupstream string  ***NEW***
        Filename for stream backup
  -mail-host string
        mail notification system: mail server host(optional)
  -mail-port string
        mail notification system: mail server port(optional)
  -mail-username string
        mail notification system: mail server username(optional)
  -mail-password string
        mail notification system: mail server password(optional)
  -mail-insecure bool
        mail notification system: allow insecure communications(optional)
  -mail-from string
        mail notification system: sender mail(optional)
  -mail-to string
        mail notification system: receiver mail(optional)

  -mail-subject-template string
        mail notification system: mail subject template(optional)
  -mail-body-template string
        mail notification system: mail body template(optional)

  -config string
        Path to JSON config file. If this flag is provided all the others will override the loaded config file
```

For JSON configuration a JSON example is provided, fill in only the needed fields.

Note on mail templating:
[Go's templating engine](https://pkg.go.dev/text/template) is used for mail subjects and bodies, please refer to the documentation for the syntax.
The following variables are available for templating:

- `.NewChunks`: number of new chunks created
- `.ReusedChunks`: number of chunks reused
- `.Datastore`: datastore name
- `.Error`: error message if any
- `.Hostname`: hostname of the machine
- `.StartTime`: time the backup started
- `.EndTime`: time the backup ended
- `.Duration`: duration of the backup
- `.FromattedDuration`: formatted duration of the backup
- `.Success`: a boolean telling whether the backup was successful 
- `.Status`: string representation of the backup status [SUCCESS, FAILURE]

Stream Backup
=============

This allows backing up a stream instead of a PXAR, allows endless possibilities for example you can invoke 

```
mysqldump yourdatabase | ./proxmoxbackupgo -backupstream yourdatabase.sql [other options]
```

This allows leveraging buzhash for dedup even when using tar for example, or the sql dump itself, and if someone wants to attempt it should be possible with some hack to pipe DISM command to generate WIM image to this and have full host backup

Known Issues
============

Windows defender antimalware being active will slow backup down up to 25% of attainable speed 

~~There's as of now no mechanism to prevent two instances being launched at same time which will screw up VSS and backup~~
If you using windows planning utility it should theoretically prevent two instances starting at same time when originating from same job

# NEW! - Full machine live backup

New funcionality has been added that now allows backing up a complete Windows 10/11 system and their respective server versions without any downtime.

The command syntax is mostly the same , except `-backupdir string`

In the case of machine backup executable there's in place of backupdir, `-backupdev`
For example an invocation could be

`machinebackup.exe -authid 'yourapikey' -backupdev \\\\.\PhysicalDisk0 -baseurl https://yourpbs:8007 -certfingerprint "xx:xx:xx..." -datastore zfs -secret 'L4m3r' -backup-id "testfull1"`

The above command will look at Disk 0 , detect all mounted partition, take VSS snapshot of these, and then create a bootable backup image of whole disk as FIDX.

Next backup will be incremental, hashing has been paralleled so speeds of 1 gbyte/sec can be easily reached.

### File restore

File restore at the moment is not possible but will be soon by marking the backup as vm backup instead of host.

### Restore to physical machine - Current

Restoring to physical machine for now involves downloading from PBS gui the fidx file ( actually the whole image will be downloaded ), and using dd to restore it.

### Restore to physical machine - Future

A live cd / PXE boot system will be released that will allow logging in to a PBS server, selecting the backup, and launching clonezilla.
Two options will be offered, one with physical interaction, and another one spawning a web gui allowing an IT technician to do the restore fully remotely once the machine is network booted.
The restore process will involve using Proxmox's  patched qemu with pbs driver exposing the backup directly to clonezilla and thus allowing efficient disk2disk restore.

# Tech Support in Italy

Being said that the project and **ALL** it's contribution will remain forever public and licensed under GPLv3 license, main sponsor of this project and also latest machine backup features, E.T.I. Srl ( https://etitech.net ) can provide to who needs it support for Proxmox deployments in general and specifically Windows backup tools.

There will never be a "community" & "enterprise" different edition, solely tech support will be an independent service.
Any customization that you are going to ask, even if development may be paid it will be released as GPLv3 like whole project is.



