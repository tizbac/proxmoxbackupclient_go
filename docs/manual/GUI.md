# Proxmox Backup Client GUI: user manual

This manual covers the graphical client (the GUI) on Windows and Linux. It applies to v0.9.1 (see the [changelog](../../CHANGELOG.md)).

For the command line tools see [CLI.md](CLI.md). For restoring a whole machine onto bare metal see [BMR.md](BMR.md).

## Contents

1. [What the GUI does](#1-what-the-gui-does)
2. [Install and start](#2-install-and-start)
3. [The window](#3-the-window)
4. [Adding a PBS server](#4-adding-a-pbs-server)
5. [One-off backups](#5-one-off-backups)
6. [Backup Sets](#6-backup-sets)
7. [Incremental folder backups (change detection)](#7-incremental-folder-backups-change-detection)
8. [The backup progress card](#8-the-backup-progress-card)
9. [Restore](#9-restore)
10. [Undelete](#10-undelete)
11. [Roll back](#11-roll-back)
12. [Reports](#12-reports)
13. [Message Log and log files](#13-message-log-and-log-files)
14. [Preferences](#14-preferences)
15. [Background service, tray and closing the window](#15-background-service-tray-and-closing-the-window)
16. [Compatibility with the official Proxmox client](#16-compatibility-with-the-official-proxmox-client)
17. [Troubleshooting](#17-troubleshooting)

## 1. What the GUI does

The GUI backs up Windows and Linux computers to a Proxmox Backup Server (PBS) and restores from it.

* **Folder backups** store one or more folders as a file archive (pxar). You can browse them and restore single files.
* **Machine backups** store whole physical disks as images. You restore them onto bare metal with the Clonezilla ISO ([BMR.md](BMR.md)), or onto Proxmox VE if you stored them as a `vm` snapshot. You can also browse a machine backup in Restore and take single files out of it ([section 9.6](#96-files-from-a-machine-backup)).
* **Backup Sets** are named, saved jobs with a schedule. **One-off backups** run once and are not saved.
* **Change detection** (Metadata mode) makes repeat folder backups read only new and changed files. See [section 7](#7-incremental-folder-backups-change-detection).
* **Undelete** finds files that have gone from a Backup Set's folders and puts them back. **Roll back** returns a set's folders to how they were at a chosen backup, with a preview first and Undo afterwards. Both work for folder and machine sets.
* **Client-side encryption** encrypts every chunk before it leaves the machine, using a standard PBS key file.
* **A background service** (Windows and Linux) runs the schedules when nobody is signed in and the GUI is closed.

## 2. Install and start

### Windows

There are two downloads on the [releases page](https://github.com/mjb-is/proxmoxbackupclient_go/releases). Check either against `SHA256SUMS.txt`:

```powershell
Get-FileHash .\<downloaded file> -Algorithm SHA256
```

* **The installer (MSI)**, `ProxmoxBackupClient-<version>-x64.msi`. It installs the app and its background service under `C:\Program Files\ProxmoxBackupClient\`, installs the service as `ProxmoxBackupClientSVC` and starts it, and adds a Start menu entry and an entry in Programs and Features. Running a newer MSI upgrades in place. To remove it and choose whether to keep the settings (servers and tokens, Backup Sets, keys, reports and logs), use **Change** in Programs and Features (**Modify** in Windows 11 Settings > Apps), then **Remove**; plain **Uninstall** keeps them without asking.
* **The zip**, `ProxmoxBackupClient-<version>-windows-amd64.zip`. Unzip it to a folder of your choice, for example `C:\ProxmoxBackupClient`. It holds `ProxmoxBackupClient.exe` and the service program `ProxmoxBackupClientSVC.exe`; keep them together. Nothing is installed: the GUI runs the schedules itself until you install the service ([section 15](#15-background-service-tray-and-closing-the-window)).

Start **Proxmox Backup Client** (`ProxmoxBackupClient.exe`). Without the service, start it with **Run as administrator**: administrator rights are needed for VSS (consistent snapshots of files that are open) and for machine backups. With the service running, backups go through the service, which has those rights. Changing Backup Sets needs administrator rights either way while job protection is on ([section 6.8](#68-administrator-protection)).

The builds are not code signed. Windows SmartScreen or Defender may warn about an unknown or machine-learning flagged program (`!ml`). This is a known false positive for unsigned Go programs. Verify the hash instead.

Only one copy of the GUI runs at a time on Windows. Starting it again brings the running copy forward.

### Linux

* **Packages**: `.deb` (Debian, Ubuntu, Mint) and `.rpm` (Fedora), package name `proxmoxbackupclient`. They install the app in `/usr/lib/proxmoxbackupclient/`, the command line tools, the manuals, the background service (not turned on) and two menu entries:
  * **Proxmox Backup Client** (`proxmoxbackupclient`): the app as you, for folder backups.
  * **Proxmox Backup Client (administrator)** (`proxmoxbackupclient-root`): the app as root, for machine backups and block snapshots. It asks for your password (pkexec from the desktop, sudo from a terminal) and uses your own settings folder.
* **The archive**: `ProxmoxBackupClient-<version>-linux-amd64.tar.gz`. Check it against `SHA256SUMS.txt` (`sha256sum -c`), unpack it and run the `ProxmoxBackupClient` binary.

The app needs the WebKitGTK runtime that Wails programs use; most desktop distributions already have it. Folder backups run as a normal user. **Machine** backups need root. Linux has no VSS: a consistent snapshot of a mounted disk needs the `elastio-snap` (or `dattobd`) kernel module loaded and root rights. See "Linux machine backup prerequisites" in the [README](../../README.md#linux-machine-backup-prerequisites-vss-equivalent).

### Where settings are kept

| | Windows | Linux |
|---|---|---|
| Settings, servers, Backup Sets, history | `C:\ProgramData\ProxmoxBackupClient\` | `~/.proxmox-backup-guardian/`; once the background service is installed, `/var/lib/proxmoxbackupclient/` |
| Log files | `C:\ProgramData\ProxmoxBackupClient\` | `~/.local/state/proxmoxbackupclient/` (or `$XDG_STATE_HOME/proxmoxbackupclient`); the service: `/var/log/proxmoxbackupclient/` |
| Key generated by the GUI | `C:\ProgramData\ProxmoxBackupClient\keys\encryption-key.json` | `keys/encryption-key.json` in the settings folder |

The main files are `config.json` (servers, tokens, remembered passphrases, email account), `scheduled_jobs.json` (Backup Sets), `job_history.json` (Reports) and `message_log.json` (Message Log).

**Passwords in `config.json`.** On Windows the PBS tokens and passwords, remembered key passphrases and the email password are stored encrypted with Windows' own data protection (DPAPI) for this computer: they show as `"dpapi:..."` in the file. The app and the background service can read them; a copy of the file on another computer cannot. An older `config.json` is converted the first time the app or the service starts. This does not keep them from an administrator of this computer, who can read anything the service can; the folder permissions below keep other accounts out. If you copy `config.json` to another computer, enter the passwords again there (or use **Export settings** with secrets, which writes them in plain text). Going back to version 0.9.1 or older after the conversion means entering them again too. On Linux they are stored in plain text, protected by the folder permissions.

**Who can read them.** On Windows the settings folder is open to SYSTEM and Administrators (full control) and to each local administrator account (read, and create files). Standard accounts have no access at all: a standard user who opens the GUI sees "This PC's backup settings are open to its administrators only. Restart as administrator to see and run your Backup Sets." The service sets these permissions when it starts, and so does the GUI when it runs as administrator. On Linux a user's own settings folder is private to that user. The shared folder `/var/lib/proxmoxbackupclient/` belongs to root and members of the `sudo` (or `wheel`) group can read it ([section 15](#15-background-service-tray-and-closing-the-window)).

On Linux, **Proxmox Backup Client (administrator)** runs the GUI as root with your own settings folder, and hands the settings and log folders back to you when it closes. A GUI started with plain `sudo` uses root's home folder, so it has its own settings. With the background service installed every account uses the shared `/var/lib/proxmoxbackupclient/`.

## 3. The window

The left sidebar has **Backup**, **Restore**, **Undelete** and **Roll back**, a thin divider, then **Reports** and **Message Log**. **Preferences**, **Help** and **About** are at the bottom. The language picker (flag) is at the top right; 18 languages are available. The status bar at the bottom shows the version ("Version v..." for a release, "Development build ..." otherwise) and a **Known Limitations** link.

The menu bar:

| Menu | Item | What it does |
|---|---|---|
| File | Backup Now (Ctrl+B) | Opens the Backup page |
| File | Minimize to Tray | Hides the window to the notification area (Windows) |
| File | Exit (Ctrl+Q) | Quits the program |
| View | Reports, Message Log | Opens that page |
| Tools | Backup, Restore, Undelete, Roll back | Opens that page |
| Tools | Preferences… | Opens the Preferences dialog |
| Tools | Download Bare Metal Restore ISO… | Opens the latest release page in your browser |
| Tools | View Bare Metal Restore Guide | Opens the bare-metal restore manual on the Help page |
| Tools | Save Bare Metal Restore Guide… | Saves that manual as a Markdown file |
| Help | User Manual (F1) | Opens this manual on the Help page |
| Help | Command-Line Manual, Bare-Metal Restore Manual, Which Restore Do I Need? | Opens that manual on the Help page |
| Help | Known Limitations, About | Information pages |

**Help** shows the user manuals, built into the program so they work offline and match its version: this one, the command-line manual, the bare-metal restore manual and the restore scenarios ("Which restore?"). Pick one at the top; the contents list on the left jumps to a section, and links between the manuals stay in the app. **Find in this manual** searches the page (Enter for the next match, Shift+Enter for the previous one). **Save a copy…** saves the manual as a Markdown file, for example to read on a phone during a bare-metal restore; **Open on GitHub** shows it online at this version. The manuals are in English.

Anything that deletes, replaces or restores asks first in a dialog naming the action, for example **Delete this Backup Set?** with **Cancel** and **Delete**. For risky actions the dialog is red and Cancel is selected, so pressing Enter does nothing harmful; Escape or a click outside the dialog cancels.

## 4. Adding a PBS server

On first start, with no server configured, Preferences opens on **Account Information**. The Backup and Restore pages also show "No PBS server is configured." with **Open Preferences**. Later, open **Preferences > Account Information** and click **+ Add server**.

The server editor has four tabs.

### Server tab

| Field | Notes |
|---|---|
| Server name | A label for you, for example "Main PBS" |
| Server ID | Only when adding. Left empty, one is generated |
| PBS server URL | Full URL with port, for example `https://pbs.example.com:8007` |
| Datastore | The PBS datastore name |
| Namespace (optional) | A PBS namespace inside the datastore |
| SSL certificate fingerprint (optional) | The server's SHA-256 fingerprint. See below |
| Description (optional) | Free text |

**Certificate fingerprint.** PBS normally uses a self-signed certificate. If you leave the fingerprint empty, **Test** fetches the fingerprint the server presents and asks you to confirm it (**Trust this server's certificate?**). Compare it with the one shown on the PBS dashboard (Show Fingerprint), then click **Trust and connect**. The fingerprint is then pinned and must match on every connection (trust on first use).

### User / Password tab

A PBS user such as `backup@pbs`. The realm is required. The password is stored and a fresh PBS ticket is obtained for every backup.

Avoid `root@pam` or any administrator account: if this computer were compromised, that account could delete your backups. The editor shows a warning for any `@pam` user.

### API Token tab

| Field | Example |
|---|---|
| Authentication ID | `backup@pbs!laptop` |
| Secret (API Token) | the token secret PBS showed when you created it |

Create tokens in the PBS web interface under **Configuration > Access Control > API Tokens**. Give the token (or its user) the **DatastoreBackup** role on the datastore. To list and restore snapshots that another user or token made, it also needs **DatastoreReader** (or higher) on that datastore.

Fill in one of the two: User / Password or API Token. Switching to one tab clears the other.

### Encryption tab

| Control | What it does |
|---|---|
| Encryption key file (optional) | Path to a PBS key file. Empty means no encryption |
| Browse... | Pick an existing key, for example one made by `proxmox-backup-client key create` |
| Generate... | Creates a new key at a place you choose (default `keys\encryption-key.json` in the settings folder). The GUI makes keys without a passphrase; for a passphrase-protected key, create it with `proxmox-backup-client key create` and pick it with Browse... |
| Clear | Stops encrypting future backups to this server |

The tab shows the key's fingerprint. Every chunk is encrypted with AES-256-GCM and the manifest is signed. PBS never sees the key.

**Without the key file an encrypted snapshot cannot be restored.** Keep a copy somewhere safe, away from this computer (a password manager, a USB stick in a drawer). Under **Keep a copy of the key**:

| Button | What it does |
|---|---|
| 💾 Save a copy… | Saves a copy of the key file where you choose, for example on a USB stick. The copy is the same file: a passphrase-protected key stays protected |
| 🖨️ Print a paper key… | Shows the key's fingerprint and a QR code of the key file, and **Print** prints a page with both and the key file's text. To get the key back, scan the QR code (or type the text) into a file and choose that file with Browse... On Windows, when the app runs as administrator, its window cannot print, so **Print** opens the page in your web browser instead, which offers to print it: print from there and close the tab. The page is sent to the browser once, straight from the app, and is not saved as a file |

A key without a passphrase on paper is as good as the file: anyone with the page can read your encrypted backups. The paper key says so.

A key generated by the GUI is written with a protected ACL (you, SYSTEM and Administrators). A key you pick yourself keeps its permissions; if other accounts on the computer can read it, the tab says "Other accounts on this computer can read this key file." and **🔒 Restrict access** limits it to you, SYSTEM and Administrators (on Linux, to its owner). A key on a USB stick formatted FAT or exFAT has no permissions to change and is not flagged.

**Passphrase-protected keys.** For a key protected with a passphrase (scrypt or PBKDF2), choose how the GUI unlocks it:

| Choice | Behaviour |
|---|---|
| Ask me (once per app session) | You are asked (**Passphrase required**, **Unlock**) the first time a backup or restore needs it. It is kept in memory only. Scheduled and startup backups do not prompt: they fail straight away with a clear message until you have entered it in this session. Runs handed to the background service also fail with an Ask key |
| Remember on this computer | The passphrase is stored in `config.json`, encrypted like the PBS token. Needed for unattended schedules and the background service |

**Verify** checks a typed passphrase against the key.

### Server list

Each server shows its URL, datastore and namespace, and a status dot (Online, Offline, Testing..., Not tested). The buttons are **Test**, **Edit** and **Delete**. Click **☆ Default** to make a server the default. The default (⭐ DEFAULT) is preselected for new backups and for Restore.

## 5. One-off backups

On the **Backup** page click **One-off Backup…**. A one-off backup runs once and is not saved. It always uses Legacy change detection.

| Field | Notes |
|---|---|
| Backup Name (for PBS Comment) | Optional. Shown in the Comment column in PBS |
| Backup type | **Directory (specific folder)** or **Machine (full disk)** |
| Directories to backup | Folder tree with tick boxes, including mapped network drives. Several folders are allowed. **Advanced: edit list as text** takes one path per line, including UNC paths such as `\\server\share` |
| Physical disks to backup | Machine type only. Each disk shows its device, size and model, with **BOOT** and **SYSTEM** marks. **Select All** and **Deselect All** help |
| Backup as | Machine type only. See below |
| Files to exclude | Directory type only. See [exclusions](#exclusions) |
| Destination PBS server | Defaults to the default server |
| Backup ID | Defaults to the computer name. Snapshots with the same ID form one backup group in PBS |
| Use VSS (Windows Shadow Copy / Linux Snapshot) | On by default. Takes a shadow copy (Windows) or block snapshot (Linux) so open files are read consistently |
| Split this backup into multiple parts | Directory type only. For a very large first backup: it measures the folders and runs several smaller, resumable backups |

Click **Start backup**. Progress appears in the card at the top of the page ([section 8](#8-the-backup-progress-card)).

### Several folders in one backup

All folders ticked in one backup go into one snapshot, each as its own archive. In Restore they show as separate top-level folders.

### Machine backups: host or vm

**Backup as** has two choices:

* **Host backup (default).** Stored as `host/<Backup ID>`. Restore it to bare metal with the Clonezilla ISO.
* **Proxmox VE virtual machine.** Stored as `vm/<number>` so Proxmox VE lists it as a VM backup and can restore it as a VM. You type a VM ID (1 to 999999). To keep clear of your real VMs the ID is changed: **Reserved range (9000000 + ID)** (default) or **Leading zeros (000 + ID)**. The form shows the resulting "Backup ID in PBS". The VM Proxmox VE creates is bare-bones: a SATA disk, OVMF firmware for GPT disks, and no network card, TPM or drivers. The same snapshot can also be restored to bare metal with the Clonezilla ISO.

A Windows machine backup (unless the set is a forensic copy, [section 6.1](#61-source-tab)) reads free space as zeros instead of from the disk, using each NTFS volume's own allocation map, and leaves the paging, hibernation and swap files out the same way. A Linux machine backup does the same for ext2/3/4 and XFS. A half-empty disk is read in about half the time, and deleted data never reaches the backup. The image is still a complete disk. If the disk cannot read part of a partition (a bad sector), that part goes into the image as zeros and the run finishes as a partial backup that says where.

### Exclusions

One pattern per line, not case sensitive:

| Pattern | Matches |
|---|---|
| `*.tmp` | any file or folder name ending in `.tmp`, anywhere |
| `node_modules` | any file or folder with exactly that name, anywhere |
| `logs/*.tmp` | `.tmp` entries directly in `logs` under the backup folder |
| `C:\Users\Alice\Temp` | that folder and everything below it |

A pattern without a `\` or `/` matches a name anywhere in the tree. A pattern with one is anchored to the backup folder. Unticking a folder or file in the folder tree excludes just that item.

Some names are always skipped: the folders `System Volume Information`, `$RECYCLE.BIN` and `.pbs-rollback` (the Roll back safety folder) at any depth, a folder called `Recovery` only at the root of a drive (Windows' own `C:\Recovery`), and the files `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` and `DumpStack.log.tmp`. The backup log lists every skipped and excluded entry.

## 6. Backup Sets

A Backup Set is a saved backup with a name, its schedules and options. The **Backup** page lists them under **Your backup sets**. Each card shows the set's name, badges, the schedule that gives the next run and **Next run** (for scheduled sets), and **Last run**.

| Badge | Meaning |
|---|---|
| DAILY, REPEAT, WEEKLY, MONTHLY, YEARLY, MANUAL | The kind of schedule that gives the next run, with +1, +2... for the set's other schedules that are on. MANUAL: the set only runs on Run Now, or none of its schedules is on |
| DIRECTORY, MACHINE, MACHINE (VM) | The backup type; (VM) for a machine set stored as a Proxmox VE VM |
| FORENSIC | A machine set that reads free space too ([section 6.1](#61-source-tab)) |
| VSS ON, VSS OFF (Linux: SNAPSHOT ON, SNAPSHOT OFF) | Whether the set reads through a snapshot. Machine sets always show ON |
| VSS PARTLY, VSS N/A (amber) | The set asks for a snapshot, but some (PARTLY) or all (N/A) of its folders are network drives or shares, which cannot be snapshotted and are read live. Hover the badge for the list |
| SHARED ID (amber) | Another set writes into the same backup group on PBS ([section 6.4](#64-destination-tab)). Hover it for the other set's name |

A Metadata set with a full read set up also shows **Next full read in N runs** or **Next run: full read (reads every file)** after Last run.

Each card has **Run Now**, **Edit** and a **More ▾** menu:

| Button | What it does |
|---|---|
| Run Now | Starts the set now, as its schedule would. If another backup or restore is running it waits in the queue |
| Edit | Opens the editor |
| More > Run now: quick (changed files only) | Metadata folder sets only. Reuses unchanged files even when the set is due its full read; the full read stays due for the next run ([section 7](#read-every-file-every-n-runs)) |
| More > Run now: full read (every file) | Metadata folder sets only. Reads every file now and restarts the full-read count |
| More > Undelete removed files... | Opens Undelete with this set picked ([section 10](#10-undelete)) |
| More > Roll back to chosen snapshot... | Opens Roll back with this set picked ([section 11](#11-roll-back)) |
| More > Clone this Backup Set | Opens the editor with a copy, named "<name> (copy)" |
| More > Delete this Backup Set | Asks for confirmation, then deletes the set (not its snapshots) |

Click **Add New Backup Set** to create one. Give it a **Backup set name**, fill in the five tabs (**Source**, **Exclusions**, **Schedule**, **Destination**, **Alerts**), then click **Save Backup Set**. Each set needs its own name: a name another set already has (in any mix of capitals) is flagged under the name box and the set is not saved.

### 6.1 Source tab

**Backup type**, then the folders (Directory) or disks and **Backup as** (Machine), as for one-off backups ([section 5](#5-one-off-backups)).

A machine set also has **Forensic copy: read free space too**. Ticked, every sector is read, free space and the paging and hibernation files included, so deleted files can be recovered from the image later (like `proxmoxbackup-machine -readfreespace`). Unticked (the default) they go into the image as zeros without being read ([machine backups](#machine-backups-host-or-vm)). A forensic set reads the whole disk on every run and its backups take more space. Its card shows a **FORENSIC** badge.

A machine set also has **Folders for Undelete and Roll back**, one per line. Undelete and Roll back work on these folders, reading their files from the disk images. Left empty, they use your Desktop, Documents, Pictures, Music, Videos and Downloads. The folders must be on the disks the set backs up. Each folder's volume is found in an older disk image by its NTFS volume serial number, so a partition moved or resized since that backup is still found; without a match (another file system, or a volume formatted again) it is found by where its partition starts.

### 6.2 Exclusions tab

**Files to exclude (one per line, optional)**, Directory sets only. See [exclusions](#exclusions).

### 6.3 Schedule tab

**When does this set run?** **On a schedule**, or **Only when I click Run Now**. A set that only runs on Run Now keeps its schedules, so you can switch back later.

A set can have several schedules. It runs at the earliest time any of them gives; two due at the same minute give one run. The list shows each schedule in words ("Every month on the last day at 03:30") with a tick to switch it on or off, **Edit** and **Delete**. **Add schedule** adds one that runs every day at 02:00; **Weekly**, **Monthly** and **Yearly** start from those. A new set starts with every day at 02:00.

**Edit** opens the schedule under the list. **Repeat every** takes a number and a unit; the fields below it depend on the unit:

| Unit | Fields |
|---|---|
| minutes, hours | **All day**, or **Between** two times. Hours, all day: **At minute** past the hour. **Only on**: the days it runs (none picked means every day). The shortest repeat is 5 minutes |
| days | **At** a time. **Only on**: the days it runs |
| weeks | **On**: one or more days. **At** a time |
| months | **On** a day of the month, **the last day**, or **the** 1st, 2nd, 3rd, 4th or last weekday ("the 2nd Tuesday"). **At** a time |
| years | **On** a day and month. **At** a time |

Repeats run on the clock: every 2 hours between 08:00 and 18:00 runs at 08:00, 10:00 and so on to 18:00, however long each run takes. With a number above 1 for days, weeks, months or years, **Starting** sets the day the count runs from (every 2 weeks from this Monday). A day that a month or year does not have (the 31st, 29 February) runs on the nearest earlier day, and the editor says so. **Next 3 runs** lists when the schedule will run.

A schedule that cannot run is named in the list and the set is not saved: a repeat under 5 minutes, a time window that ends before it starts, or a weekly schedule with no day.

| Setting | Notes |
|---|---|
| Also run at machine startup | Under the list. Runs the set when the computer starts as well |

A summary under the settings gives the set's next run and which schedule it comes from. If the computer is off or asleep at a run time, the set runs once when it is back, however many times it missed.

Schedules run while the GUI is running (in the tray is fine) or in the installed background service ([section 15](#15-background-service-tray-and-closing-the-window)).

### 6.4 Destination tab

| Setting | Notes |
|---|---|
| Destination PBS server | Which server the set backs up to |
| Backup ID | Not shown for a vm set (the VM ID decides it). A folder set gets a suggested ID made from the computer's and the set's name (below); a machine set defaults to the computer name |
| Use VSS (Windows Shadow Copy / Linux Snapshot) | As for one-off backups |
| Change detection | **Legacy: read every file (default)**, **Data: read every file, split archive** or **Metadata: skip unchanged files (fastest)**, with **Read every file every N runs** for Metadata. See [section 7](#7-incremental-folder-backups-change-detection). Greyed out for machine sets |

**Give every Backup Set its own Backup ID.** All snapshots with the same type and ID form one group in PBS. Metadata mode compares with the newest snapshot in the group, and Legacy mode reuses chunks from it. If two sets share an ID, each run compares with the other set's snapshot and reads every file, and Undelete and Roll back list both sets' snapshots. A good pattern is `<computer>-<set>`, for example `deepthought-data`. The editor does this for you: a new folder set's Backup ID follows its name as you type it, the computer's name first and then the set's name without it. On `deepthought`, "Deepthought - Data" becomes `deepthought-data` and "Beeby Property" becomes `deepthought-beeby-property`, so every set of one computer sits together in PBS's list. An ID another set already uses gets `-2`, `-3` and so on, and typing your own ID stops the suggestion. A clone gets its own ID from the copy's name. Editing an existing set whose ID differs from the suggestion offers **Use <suggestion>**: a new ID starts a new group on PBS, so its first incremental run reads every file and its earlier backups stay under the old ID.

**Sets that share a group are flagged.** When another Backup Set writes to the same PBS server with the same type and Backup ID, the editor shows an amber note under the Backup ID naming that set, with **Use <suggestion>** for a folder set, and both sets' cards carry an amber **SHARED ID** badge. An empty Backup ID counts as the computer's name, and a set with no server chosen counts as on the default server. It is a warning only: saving is still allowed, for the rare case where sharing a group is what you want.

### 6.5 Alerts tab

| Setting | Notes |
|---|---|
| Send an e-mail when the backup completes | With a recipient address |
| Send an e-mail if the backup fails | With a recipient address |
| Run an application before the backup | Path or command |
| Run an application after the backup | Path or command |
| When the backup finishes, the app window: | **Stays as it is** (default), **Minimises to the tray** or **Closes (the app exits)**. Minimising needs the tray, so it does nothing on Linux, and it does nothing for a run the background service makes |
| Turn off the computer after the backup | Shuts the computer down when the set finishes. A warning shows while it is ticked |

The actions run in this order when the set finishes: the email, the "after" application, the window choice, then the shutdown last.

Email needs the SMTP account in **Preferences > Email** ([section 14](#14-preferences)). The tab warns if none is set up. If a successful backup's e-mail cannot be sent (the mail server is down or refuses it), "Error sending notification email, check client mail configuration." is added to the snapshot's comment on PBS, after the set's name, as the command-line tools do, so it shows in the PBS web interface.

### 6.6 What PBS sees

The set's name is written to PBS as the snapshot comment, so it shows in the PBS web interface and in the bare-metal restore menus.

### 6.7 Queue

Only one backup or restore runs at a time. Anything started meanwhile (Run Now, a schedule, a restore) waits. The progress card lists what is waiting ("1 more backup queued…").

### 6.8 Administrator protection

**Windows.** **Preferences > Advanced > Require administrator rights to add, change or delete Backup Sets** is on by default. While it is on:

* Only an administrator can add, edit, clone, delete or import Backup Sets. Anyone who can open the settings can run a set and view history.
* Without administrator rights the Backup page shows "Changing Backup Sets needs administrator rights." and a **Restart as administrator** button. A change that is refused opens **Administrator rights required** with the same button.
* Only an administrator can turn the setting off or on. It is stored in `HKLM\SOFTWARE\ProxmoxBackupClient\Policy`. A missing or unreadable value means on.

A standard (non-administrator) account cannot open the settings at all ([section 2](#where-settings-are-kept)).

**Linux.** There is no setting. Once the background service is installed the Backup Sets are in its shared folder, which only root can change, so the same banner and **Restart as administrator** appear in a GUI that is not root. **Restart as administrator** starts the GUI as root through pkexec and closes the one you had open. Before the service is installed each user keeps their own Backup Sets.

## 7. Incremental folder backups (change detection)

Folder Backup Sets can use the same change-detection modes as the official `proxmox-backup-client`. This is the biggest time saver in the client.

| Mode (as shown in the editor) | Reads | Stored as |
|---|---|---|
| Legacy: read every file (default) | every file | one `<name>.pxar.didx` per folder, as in earlier versions |
| Data: read every file, split archive | every file | a split archive: `<name>.mpxar.didx` (metadata) and `<name>.ppxar.didx` (file contents) |
| Metadata: skip unchanged files (fastest) | only new and changed files | the same split archive |

### How Metadata mode works

On each run the client reads the file list of the previous snapshot in the group. A file whose size, modification time (to the nanosecond), mode and owner match is not opened at all: its data is taken from the previous snapshot's chunks. Only new and changed files are read and uploaded.

* The **first run** in Metadata mode reads every file, because there is nothing to compare with yet.
* Later runs read only what changed. Since unchanged data is never read and never sent, this also saves bandwidth to a remote PBS.
* Reuse only happens from a previous snapshot encrypted the same way, with the same key. Changing the key reads every file once.
* A file that could only be partly read on an earlier run (zero-padded after a read error) is always read again.

Real example, a 480.9 GB data share with 761,866 files on USB disks, to a PBS on the local network:

| Run | Time | Files read | New chunks |
|---|---|---|---|
| First run (no previous snapshot) | 4 h 17 min | all | 135,549 |
| Next runs, nothing or little changed, an hour or more after the last run | about 15 min | only changed files (all 761,866 unchanged files reused) | 4 to 40 |
| The same, run again within a few minutes | about 9 to 10 min | the same | the same |
| After deleting 39 GB (136 files) | about 10 min | none | 21 (about 106 MB) |

**Why the time varies.** Even an unchanged file has to be looked up on disk (size, time, permissions). Right after a run, Windows still holds those details for every file in memory, so the next run is quick. After an hour or more they have been pushed out and come off the disk again, which on a large set of small files on USB disks adds a few minutes. Nothing is wrong when a 4-hourly run takes longer than a run started straight after another.

**Deleting files still uploads a little.** The file list (names, sizes, times, permissions) is stored apart from the file contents. Where files were removed, the affected parts of the list are written again; the contents of the files that are left are reused. The space the deleted files take on PBS is freed only once the older snapshots that still hold them are pruned and garbage collection runs.

### Read every file every N runs

A file changed in a way that keeps its size and modification time is not noticed in Metadata mode. To catch such changes, set **Read every file every N runs** (0 = never, otherwise 2 to 366). Every Nth run then reads every file, like Data mode. It also drops the small amount of unused data (padding) that reuse leaves behind.

Only runs that finish successfully count. A stopped or failed run does not bring the next full read closer. The set's card shows **Next full read in N runs**, or **Next run: full read (reads every file)**.

As a guide: for a set that runs every 4 hours, 42 means about once a week. For a daily set, 7 means once a week.

To choose for one run, use the card's **More** menu: **Run now: quick (changed files only)** skips a full read that is due (it stays due for the next run), and **Run now: full read (every file)** reads every file now and restarts the count. The progress card then says "Full read requested: reading every file". Plain **Run Now** does what the schedule would.

### Switching an existing set to Metadata

1. Update every computer you restore from to v0.6.0 or later first. Versions before split-archive support show the folders of a Data or Metadata snapshot without their files.
2. Give the set its own Backup ID if it shares one ([section 6.4](#64-destination-tab)).
3. Edit the set, **Destination** tab, choose **Metadata: skip unchanged files (fastest)**, set **Read every file every N runs**, and save.

The first split backup of a folder shares little with its older Legacy snapshots. Expect it to use roughly one more full copy of that folder on the datastore until the old Legacy snapshots are pruned and garbage collection runs.

Machine sets ignore change detection: disks are stored as images. One-off backups always use Legacy.

**Reports** show the mode of each folder run and how many unchanged files were reused.

## 8. The backup progress card

When a backup starts, a card appears at the top of the Backup page with the set's name, a percentage and a progress bar. Every row is there from the start. A value shows **Calculating...** until it is known, and **Not available** where the run cannot provide it.

The line under the bar says what the run is doing:

| Stage line | Meaning |
|---|---|
| Starting... | Connecting, taking the VSS snapshot, preparing |
| Change detection: unchanged files are reused without reading | A Metadata run, comparing with the previous snapshot |
| Scheduled full read: reading every file | This is the set's every-Nth-run full read |
| Full read requested: reading every file | You chose **Run now: full read (every file)** |
| No previous snapshot to compare with: reading every file | A Metadata run with no usable previous snapshot (first run, different key) |
| Reading every file (change detection: data) | A Data mode set |
| Reading and uploading files | A Legacy run |
| Finishing: closing the catalog and uploading the manifest... | The last step |
| Connecting to the server... / Taking a snapshot of the volumes... | A machine backup getting ready |
| Reading disk 1 of 2: CT500MX500SSD1, \\.\PhysicalDrive0 (465.8 GiB) | A machine backup reading a disk: its model, device and size |
| Finishing: closing the disk image and uploading the manifest... | The last step of a machine backup |
| Stopping after the current file... | You pressed Stop |
| ⏸ Paused for ... | You pressed Pause; the time counts up |

The rows of a folder backup:

| Row | Meaning |
|---|---|
| Started | Clock time the run started |
| Forecast finish | Clock time it is expected to end |
| Elapsed time | Time since the start |
| Time remaining | Estimate. It uses the last 2 minutes of progress and takes the longer of the bytes-based and files-based estimates, so a run of many small files is not under-estimated. Shows **Finishing...** during the last step |
| Files | Files done / total files. The total comes from the previous snapshot or a scan |
| Processing | Data processed per second over the last 10 seconds, plus files per second. Reused data counts, so this shows how fast the run is getting through the folder |
| Folder | The folder being processed |
| Upload | Data actually sent to PBS per second over the last 10 seconds. Close to zero when nearly everything is reused |
| Current file | The file being read now, updated 5 times a second |

A machine backup has the same layout with rows for a disk: **Blocks** (4 MiB blocks of the current disk, done / total), **Disk** (model, device and size, for example "CT500MX500SSD1, \\.\PhysicalDrive0 (465.8 GiB)"), and **Partition** in place of Current file (for example "C: (NTFS)", "Partition 2" or "Unpartitioned space"; on Linux "/dev/sda2 (/, ext4)").

The box underneath shows **Processed:** data done / total, in GB with GiB in brackets (1 GB = 1,000,000,000 bytes, 1 GiB = 1,073,741,824 bytes), and the number of **New** and **Reused** chunks. A machine backup adds how many of the reused chunks are blank space, and in red how much of the disk could not be read (zeros in the image). Failed chunks show in red.

**Pause** holds the run where it is, part way through a file if need be, and the button becomes **Resume**. Nothing is read or uploaded while it is paused, and the connection to PBS stays open, so Resume carries on from the same place. Stopping a backup and starting it again would read everything from the beginning. Use it when the PBS server or the network is busy with something else, or the computer is needed for a while. While a backup is paused:

- other backups and restores in this app wait, as they do behind a running backup;
- the VSS snapshot stays in place, so its shadow storage grows as files on that drive change;
- if the connection to PBS drops (the server restarts, the network goes down) the backup fails as it would while running, and has to be run again;
- Forecast finish and Time remaining show **Paused**, and pick up again from fresh measurements after Resume.

Pause works for folder and machine backups, whether the app or the background service runs them. With **Run as Service** on ([section 15](#15-background-service-tray-and-closing-the-window)), a Backup Set the service starts on its schedule shows the same card in an open app window, with its set's name, within a few seconds, and Pause, Resume and Stop act on the service's run. A window opened during the run picks it up, paused or not. On Windows the tray icon's menu has **Pause backup** too (**Resume backup** while paused), so a run can be paused without opening the window ([section 15](#15-background-service-tray-and-closing-the-window)). The card of a run the service started on its own shows the stage line and the percentage; the Files, Folder and Upload rows read **Not available**, as the service does not pass those figures on.

**Stop** ends the run after the current file, and also works while paused. Reports record it as cancelled (⏹️), not failed.

Anything waiting behind the run is listed under the card.

## 9. Restore

Open the **Restore** page. The box at the top lists what restore supports: files and folders (full tree), selective restore, modification times, creation times (Windows), several PBS servers, NTFS ACLs (Windows) or POSIX ACLs (Linux), and alternate data streams and NTFS extended attributes (Windows; folder backups record them from v0.9.0 on).

### 9.1 Find the snapshot

1. **Source PBS server**: pick the server.
2. **Backup ID to restore (optional)**: type an ID, or use the buttons beside it:
   * **▾** lists the Backup IDs your Backup Sets use (with the set names), plus this computer's name for one-off backups.
   * **✕** clears the field. An empty field lists every backup in the datastore, which is what you want when restoring onto a replacement computer.
3. Click **List available snapshots**. A moving bar shows while PBS answers. If it takes more than 15 seconds the GUI says PBS is slow to respond (a verify, garbage collection or backup may be running on the server).
4. **Available snapshots** shows date, Backup ID, type and size, in **Tile** or **List** view. 🔒 marks a protected snapshot, and a machine backup is marked "disk image". Click **Select** on one.

### 9.2 Search for a file

Click **Search for a file** to open the search panel. It searches every snapshot whose Backup ID starts with the ID in the box, over a date range.

| Setting | Notes |
|---|---|
| Search term | For example `invoice`, `*.docx`, `report-2024` |
| Type | **File name**, **Regular expression** or **Full path** |
| From / To | Date range of snapshots to search |
| Also assemble snapshots not yet browsed | Slower: downloads the file lists of snapshots not already cached |

Click **Search**. Results stop at 5,000; refine the search if you reach it. **Prepare restore** on a result selects that snapshot and file.

### 9.3 Choose what to restore

After you select a snapshot, the panel shows the snapshot's **Original source**, **Machine** and **Backed up at**, and **Snapshot contents** as a tree. Folders load one level at a time; a folder with many entries shows the first 1,000 with **Show N more**.

* Tick the files and folders to restore. **Select all** and **Select none** help. Nothing ticked means nothing is restored.
* The line under the tree shows how many entries are ticked and their total size.
* **Reload** downloads the tree again from PBS instead of the local cache.

Selective restore reads only the chunks that hold the ticked files, so restoring a few folders from a large snapshot is quick.

### 9.4 Choose where and how

| Setting | Notes |
|---|---|
| Restore to original path | Writes back to where the files came from. Overwrites existing files. Not available when the snapshot has no record of its original path, was made on another operating system, or is a disk image. If the snapshot came from another computer you must tick "Force cross-host" |
| Restore to alternate path | Writes to the **Destination folder** you choose (**Browse**) |
| Keep original directory structure | Alternate path only. On: `dest\Users\alice\doc.txt`. Off: `dest\doc.txt` (good for a single file) |
| Overwrite existing files | Always on for original path |
| Restore modification times | Sets file and folder modification times as they were in the backup. Untick it to leave restored files with the time of the restore. Creation times (Windows) are put back either way. Undelete and Roll back always put the times back |
| Verify after restore | Reads every restored file back and compares it with the snapshot. Mismatches are reported; nothing is deleted |
| Restore permissions/ACLs | Windows: owner, group, permissions (DACL) and Hidden/System/Archive/ReadOnly attributes. Linux: POSIX ACLs and extended attributes. Run as administrator (or root) for this |
| Restore alternate data streams and extended attributes | Windows, on by default. Puts back the data attached to files and folders, such as the "downloaded from the internet" mark (Zone.Identifier), and NTFS extended attributes (WSL keeps Linux owners and permissions in them). Streams over 1 MB are recorded by name only and are not restored (the restore log says how many). Backups made before v0.9.0 did not record them. Not shown on Linux |

On Windows the **creation time** (Explorer's "Date created") of every file and folder is put back too, whether or not permissions are restored, for backups made with v0.8.0 or later; older backups did not record it.

Click **▶️ Restore**.

### 9.5 The restore card

The restore card works like the backup card. Its stage line shows **Connecting and preparing restore...**, **Reading snapshot index and locating files...**, **Fetching chunks and writing files...**, then, after the data, **Setting folder timestamps...**, **Restoring ACLs and attributes...** (or **Restoring creation times...** when permissions are not being restored) and **Verifying restored files...** as they apply. The rows are Started, Forecast finish, Elapsed time, Time remaining, Speed (shown as the average once the transfer is done), Downloaded (data done / total) and Current file. **Stop** ends the restore.

The finished restore is recorded in Reports with its snapshot, destination, paths, file count, size, verification result and duration.

### 9.6 Files from a machine backup

A machine backup (a `host` or `vm` snapshot of whole disks) opens in Restore like a folder backup. Each volume in the image is a top-level folder, for example "Partition 3 - Windows (NTFS, 127.3 GB)". A disk with a file system but no partition table shows as "Whole disk", and with several disks the folders start with "Disk 2" and so on. Nothing is mounted and the image is not downloaded: the partition table and the volume's file table are read straight from the backup, then only the chunks holding the chosen files.

* **File systems**: NTFS, FAT12/16/32 (EFI partitions, memory cards), exFAT, ext2/3/4 and XFS. Swap, LVM and unknown partitions are left out. A BitLocker partition cannot be browsed, and the restore log says why.
* **Where to**: files always go to another folder (Restore to alternate path). They are never written over the live disk.
* **What comes back**: file contents, modification and creation times, and on Windows the alternate data streams (such as Zone.Identifier) and extended attributes (such as the ones WSL keeps for Linux owners and permissions) of NTFS files and folders, unless **Restore alternate data streams and extended attributes** is unticked. With **Restore permissions/ACLs** ticked, also the read-only, hidden, system and archive attributes and, on Windows, the NTFS permissions. Files from ext2/3/4 and XFS volumes keep their symbolic links and, restored on Linux, their permissions (and owners when restoring as root).
* **Speed**: a 127 GB laptop C: with 430,000 files lists in 4 to 15 seconds. Each volume's file list is kept on this computer once read, so opening the same snapshot again, even after a restart, is quick. `System Volume Information` is left out.

The same reading is what Undelete and Roll back use for machine sets.

## 10. Undelete

**Undelete** (side menu, or **More > Undelete removed files...** on a Backup Set's card) finds files that are in a Backup Set's backups but no longer on disk, and puts them back.

1. Pick the **Backup Set**. A machine set is listed with the folders it covers ([section 6.1](#61-source-tab)); their file lists are read from the disk images (NTFS on Windows; ext4 or XFS on Linux, where folders on LVM or RAID are left out with the reason).
2. Choose how far to **Look back**:

   | Choice | What is checked |
   |---|---|
   | Latest backup | Files in the newest backup that are not on disk now |
   | Last 7 days / Last 30 days | Every backup of that period. Each missing file is offered from the **newest** backup that still has it |

3. Click **Find deleted files**. The GUI reads each backup's file list (its catalog, a few MB even for hundreds of thousands of files, kept on this computer afterwards so the next search is quicker) and lists the set's folders on disk. No file is opened or read. The set's exclusions apply, so excluded files never show up.

   The bar shows where it is: **Reading the file list of snapshot 2 of 6**, then **Reading the folder F:\Data: 312,400 of about 868,592 files and folders** (the total is what the newest backup had there, so it is an estimate), with the time taken. As a guide, for a set of 760,000 files on USB disks: a backup's file list takes about 10 seconds the first time and a second or two after that, and listing the folders takes from about 30 seconds (just after a backup) to 3 or 4 minutes (disk not read for a while). A 7-day search typically takes 1 to 5 minutes.

The result is a folder tree of the missing files with a tick box on every file and folder. Each file shows its size and when it went: **deleted since** the newest backup, or **deleted between** two backups. A file whose name, size and time match a file somewhere else in the folder is marked **probably moved to ...** and is not ticked, since it most likely still exists under another name or folder. Files under a folder that cannot be read now (permissions) are not listed as missing; a note says how many.

Use the filter box to narrow the list, **Select all shown** and **Clear shown** for what is visible, then choose **Restore to**:

| Choice | What happens |
|---|---|
| Where they were | Each file goes back to its original place. A file that exists there again is left alone and reported as skipped |
| Another folder | The files go under the folder you choose, keeping their folder structure |

Tick **Verify after restore** to check every restored file against the backup, then click **Undelete N files**. The restore card shows the progress (several backups are restored one after another), Stop works as usual, and Reports records the run as **Undelete**.

Undelete also runs from the command line for any Backup Set ([CLI.md, section 12](CLI.md#12-undelete-and-roll-back-for-a-backup-set)).

## 11. Roll back

**Roll back** (side menu, or **More > Roll back to chosen snapshot...** on a Backup Set's card) puts a Backup Set's folders back as they were at a chosen backup. You see what would change first, and everything it replaces or removes is kept, so the roll back can be undone. For a machine set it works on the set's **Folders for Undelete and Roll back**, reading them from the disk images.

1. Pick the **Backup Set**. Its backups are listed newest first with their size; 🔒 marks a protected snapshot.
2. Pick a backup from the list, or enter a date and time in **The backup at or before** to pick the newest backup at or before it.
3. Click **Show what would change**. Nothing is written. The preview compares the backup with the folders now:

   | Box | Meaning |
   |---|---|
   | Missing now | In the backup, gone from disk: restored |
   | Changed since | Different size or modification time: replaced by the backup's version |
   | Added since | On disk but not in the backup: kept, unless you choose an exact roll back |
   | Unchanged | Left as they are |

   The files of each group are listed underneath (the first 2,000 of each).

4. Choose **What to do**:

   | Choice | Effect |
   |---|---|
   | Restore missing files and replace changed ones; keep files added since | The usual roll back (default) |
   | Restore missing files only | Nothing on disk is replaced; changed files keep their current version |
   | Exact roll back: also remove files added since | The folders end up exactly as in the backup. You confirm by typing the number of files that will be removed |

5. **Back up the set first** runs the Backup Set once before anything changes, so today's state is a backup too. It is on by default for a folder set and off for a machine set, whose backup is a whole-disk image and takes about as long as its usual backup. While that backup runs, the card reads "Backing up the set before the roll back: 37%". **Verify after restore** checks the restored files. Then click **Roll back to ...**.

**The safety net.** Before a file is replaced or removed it is moved, not deleted, into `.pbs-rollback` at the root of the same drive (for example `F:\.pbs-rollback\<set name>\<date-time>\`). Moving within a drive is instant, whatever the file size. The folder also holds `rollback.json`, the record of what was moved, restored and removed. `.pbs-rollback` is never backed up. Files that are open in another program cannot be moved; they are left as they are and listed in the result, with the reason.

While a roll back runs, the set's own scheduled backups wait, and Run Now on that set is refused. Reports records the run as **Roll back**.

**Earlier roll backs of this set** are listed under the page, with what each restored, replaced and removed and how much the kept versions take up:

| Button | What it does |
|---|---|
| Undo | Deletes the files the roll back restored, moves every kept version back, makes removed folders again and removes folders the roll back created if they are empty. The safety folder is removed when everything went back |
| Delete kept versions | Deletes that roll back's safety folder to free the space. It can no longer be undone |

Roll back also runs from the command line ([CLI.md, section 12](CLI.md#12-undelete-and-roll-back-for-a-backup-set)).

## 12. Reports

**Reports** lists every backup and restore run, newest first, with ✅ success, ❌ failed, ⏹️ cancelled or ⏳ running. The list also shows 🔒 Encrypted or Unencrypted, and for Metadata runs how many files were reused.

Click a run for its details:

| Detail | Notes |
|---|---|
| Status, Date / Time | |
| Mode | Automatic (scheduled), Automatic (at startup), Manual (Run Now) or Manual (one-off) |
| Type | Folders, Full machine, Restore, Undelete or Roll back |
| Encryption | Encrypted or Unencrypted |
| Change detection | Legacy, Data (split archive) or Metadata (unchanged files skipped), with "N unchanged files reused without reading, size" |
| Message | The result message, including any skipped files. A machine backup gives its counts, and for unreadable sectors where they are |
| Backup ID, VSS, Folders | |
| Snapshot, Destination, Paths, Files, Size, Verification, Duration | Restores, Undeletes and Roll backs |

A failed backup has a **Rerun** button that loads its folders and Backup ID into the Backup page.

## 13. Message Log and log files

**Message Log** is a timestamped list of backup and restore messages (ℹ️ information, ⚠️ warning, ❌ error) with their source, capped at 1,000 entries.

Log files are in `C:\ProgramData\ProxmoxBackupClient\` on Windows (Linux: [section 2](#where-settings-are-kept)). **Preferences > Message Log > View Logs** opens the folder.

| File | Contents |
|---|---|
| `backup-YYYYMMDD-HHMMSS-<backup id>.log.gz` | One per backup run, compressed when the run ends. Open with 7-Zip or `zcat` |
| `backup-gui.log` | General backup and restore log of the GUI |
| `service-gui.log` | Scheduler and general operations of the GUI |
| `backup-service.log`, `service-service.log` | The same for the background service |

Logs rotate at 10 MB, keeping 5 old files.

Every run log ends with a `[RESULT]` line, for example:

```
[RESULT] outcome=verified_success dirs_ok=1/1 new=9 reused=158838 failed=0 read_errors=0 excluded=0 bytes=480858587134 duration=10m24.4s
```

A Metadata run also logs how many unchanged files were reused, how many chunks were reused and how much padding that added. A machine backup logs one progress line per 1%.

## 14. Preferences

Open Preferences from the side menu or **Tools > Preferences…**. The dialog has **OK**, **Cancel** and **Help** at the bottom; each setting is saved as you change it (or with the tab's own **Save** button). **Help** opens this section of the manual.

| Tab | Contents |
|---|---|
| Account Information | PBS servers ([section 4](#4-adding-a-pbs-server)) |
| Backup Options | **Parallel restore extraction (experimental)**: writes several files at once during a restore. Not always faster; on our test machine it was slightly slower. **Multi-threaded backup reading (experimental)**: reads files ahead in the background during a backup, with a **Worker count:** (1 to 32, default 8). Helps when the disk is the bottleneck. Turn either off if anything behaves unexpectedly |
| Theme | Amber, Classic Blue, Green, Red, Dark, or Custom… (your own Accent color, Accent (hover) and the two gradient ends, as 6-digit hex values, with a preview). A branded build (one whose program file carries a brand name the app knows) uses the brand's own colours and has no Theme tab |
| Email | The SMTP account all Backup Sets use: SMTP server, Port, Username, Password, From address, and "Allow an unencrypted connection (not recommended)". **Save**, then **Send test email** to an address to check it ("Sending..." with a wait bar while it goes). Each set picks its own recipient on its Alerts tab |
| Export / Import Settings | **Export Settings...** saves your servers, the default server, theme, email account and Backup Sets to a `.json` file. Tick "Include passwords/tokens in the export" to include the PBS tokens and passwords, remembered key passphrases and the SMTP password, in plain text, so keep that file safe. Without it, none of them is written. Encryption key files are not included: copy them yourself (**Save a copy…** on the Encryption tab). **Import Settings...** loads such a file, for example from another computer. Servers and Backup Sets with the same ID are replaced, others are added, and nothing else is removed. Importing Backup Sets needs administrator rights while job protection is on |
| Message Log | **View Logs** opens the log folder |
| Advanced | **Scheduled job protection** ([section 6.8](#68-administrator-protection)) and **Run as Service** ([section 15](#15-background-service-tray-and-closing-the-window)) |

## 15. Background service, tray and closing the window

**Closing the window on Windows** (the X) asks whether to **Minimize to Tray** or **Exit**. Minimised, the GUI keeps running in the notification area and its schedules still run. **Exit** (also **File > Exit**) quits completely, and no schedules run from the GUI until you open it again.

The **tray icon** (Windows) menu has Show window, Pause backup, Backup status and Quit. **Pause backup** can be clicked while a backup runs, whether the app or the background service runs it, and pauses it as the progress card's Pause does ([section 8](#8-the-backup-progress-card)); it then reads **Resume backup**, and the icon's tooltip says **Backup paused**. Windows notifications report finished backups and restores.

**Linux has no tray.** Closing the window quits the app, and **Minimize to Tray** does nothing. Use the background service for schedules that run while the app is closed.

### Windows service

With the background service installed and running, the service runs the schedules and backups started from the GUI are handed to it, so scheduled Backup Sets run even when nobody is signed in and the GUI is closed. The service runs as Local System, so VSS works without starting the GUI as administrator. Encryption keys used through the service must be set to **Remember on this computer**. Without the service, the GUI runs the schedules itself while it is open. The MSI installs and starts the service for you; with the zip, install it here.

**Run as Service** (Preferences > Advanced) shows whether the service is installed, whether it is running, how it starts ("starts with Windows"), and whether the GUI is handing its work to it ("This app hands backups and schedules to the service." or "This app is running the schedules itself."). As administrator you can:

| Button | What it does |
|---|---|
| Install and start | Installs `ProxmoxBackupClientSVC.exe`, which must be in the same folder as `ProxmoxBackupClient.exe`, as an automatic Windows service named `ProxmoxBackupClientSVC`, running as Local System, restarted automatically if it stops unexpectedly. Then starts it. The GUI stops its own scheduler and hands schedules and backups to the service, so nothing runs twice |
| Start / Stop | Starts or stops the installed service. While it is stopped the GUI runs the schedules again. Stop is refused while the service is running a backup |
| Remove | Stops and uninstalls the service. Backup Sets, history and settings are not touched; the GUI runs the schedules again while it is open |

If the GUI is not running as administrator, a **Restart as administrator** button is shown. If the installed service runs a program other than the one beside this GUI, the tab says which. A service installed by an older version under the name `ProxmoxBackupClient` is recognised too.

### Linux service

The .deb and .rpm install the service program (`/usr/lib/proxmoxbackupclient/ProxmoxBackupClientSVC`) and the systemd unit `proxmoxbackupclient.service`, but do not turn it on. **Run as Service** (Preferences > Advanced, in the GUI started as administrator) does:

| Button | What it does |
|---|---|
| Install and start | Copies your Backup Sets, servers, keys and history to `/var/lib/proxmoxbackupclient/` (only if nothing is there yet; your own `~/.proxmox-backup-guardian/` is left as it was), then enables and starts the unit, which runs as root and starts with the computer. From then on the GUI and the service both use the shared folder, and the GUI hands schedules and backups to the service |
| Start / Stop | `systemctl start` / `stop`. While it is stopped the GUI runs the schedules again. Stop is refused while the service is running a backup |
| Remove | Stops and disables the unit. The settings stay in `/var/lib/proxmoxbackupclient/`, which the GUI keeps using |

The shared folder works like `C:\ProgramData\ProxmoxBackupClient\`: root owns it and can change it; members of the `sudo` (or `wheel`) group can read it, so their GUI, started normally, sees the Backup Sets, runs them through the service and shows history; other accounts get no access, and their GUI shows the "open to its administrators only" banner. Adding, editing, deleting or importing Backup Sets needs the GUI as root: **Restart as administrator** (in the banner and in Preferences) starts it with pkexec. As on Windows, the history of a backup started from a GUI that is not root is not recorded, because that GUI cannot write the shared history; scheduled runs are recorded by the service. An upgrade of the package restarts a running service. Removing the package stops and disables the service and keeps `/var/lib/proxmoxbackupclient/`; delete that folder yourself to remove the settings. Without systemd there is no service and the GUI runs the schedules while it is open.

Runs the service makes on its own schedule show in an open GUI window, with Pause, Resume and Stop ([section 8](#8-the-backup-progress-card)).

## 16. Compatibility with the official Proxmox client

This client and the official `proxmox-backup-client` read each other's backups. Tested with `proxmox-backup-client` 3.4.9 and PBS 4.2:

* Legacy, Data and Metadata snapshots made by this client (GUI and command line, Windows and Linux) restore byte-identically with the official client, and PBS 4.2's file browser opens them.
* Legacy, Data and Metadata snapshots made by the official client restore byte-identically with this client, fully or selectively.
* Both use the same split archive format (pxar version 2) and the same encryption key file format, so keys work in both.

This client adds one small file at the root of its folder snapshots, `.proxmox_backup_client_meta.json`, which records the original path. The official client restores it as an ordinary file.

## 17. Troubleshooting

**A USB or external source disk drops out during a backup.** The backup waits up to 2 minutes for the disk to come back, reopens the file and carries on, logging both events. If one file still cannot be read, only that file is affected: it is filled with zeros to its recorded size, marked incomplete in the log, and read again on the next run. If the disk does not come back within 2 minutes the backup fails. A recurring drop-out usually means a cable, port or hub problem; avoid chaining USB hubs.

**A machine backup ends as partial.** Part of a disk could not be read (bad sectors). That part is zeros in the image; the card, the Reports row and the log say where and how much. Check the disk's health.

**Listing snapshots takes a long time.** PBS reads its datastore directory to answer. On a datastore that is cold (spun-down disks) or busy with a verify, garbage collection or another backup this can take a minute or more. The GUI waits up to 120 seconds and shows the slow-server hint after 15.

**A PBS verify job fails on an empty snapshot folder.** A backup that was interrupted at the wrong moment can leave a snapshot folder on PBS with no `index.json.blob`. Verify reports it as "manifest load error ... No such file or directory" and the job ends as failed, even though every real snapshot verified. Delete that snapshot in the PBS web interface (Datastore > Content), or remove the empty folder on the PBS host.

**Restore says the owner of some files stayed the restoring account (Windows).** Setting a file's owner to another account needs the "Restore files and directories" privilege, which the GUI switches on when it runs as administrator. Run as administrator, any recorded owner comes back, even an account this computer does not know. Without it the permissions are still restored and only the owner differs; the restore log lists those files.

**Every Metadata run reads every file.** Check that no other set or computer uses the same Backup ID on the same server and namespace (look for a **SHARED ID** badge, [section 6.4](#64-destination-tab)), and that the encryption key has not changed. The stage line says why the run is reading every file.

**Scheduled backups fail with a passphrase message.** The key is set to **Ask me** and nobody has entered the passphrase since the GUI started, or the run went through the service. Use **Remember on this computer** for unattended runs.

**"Changing Backup Sets needs administrator rights."** Job protection is on (Windows), or the background service holds the Backup Sets (Linux). Click **Restart as administrator**.

**"This PC's backup settings are open to its administrators only."** You are signed in with a standard account, which cannot read the settings. Click **Restart as administrator** and give an administrator's password.

**Machine backup on Linux fails with a permission or snapshot error.** Run the GUI as root (**Proxmox Backup Client (administrator)**) and load `elastio-snap` or `dattobd` first.

**Restore of an older client version shows folders without files.** That computer has a version without split-archive support. Update it.

**Hard links.** When a file has two names (a hard link), only one name is restored.
