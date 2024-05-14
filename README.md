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

Usage
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
  -pxarout string
        Output PXAR archive for debug purposes (optional)
  -secret string
        Secret for authentication
  -archivename string
        Name for archive file, defaults to backup (optional)


```

Known Issues
============

Windows defender antimalware being active will slow backup down up to 25% of attainable speed 

There's as of now no mechanism to prevent two instances being launched at same time which will screw up VSS and backup
If you using windows planning utility it should theoretically prevent two instances starting at same time when originating from same job

