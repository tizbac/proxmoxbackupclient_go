# The text window: user manual

`salvanza-cli tui` is the desktop app's Backup Sets, running backups, history, servers, preferences and restore in a terminal. Use it on a server without a desktop, over SSH, or whenever a text screen is quicker than the app. It works with the computer's background service, the same one the desktop app uses, so what you do here shows in the app and the other way round.

It is new after version 2.0.

## Contents

1. [Start it](#1-start-it)
2. [The screens](#2-the-screens)
3. [Backup Sets](#3-backup-sets)
4. [Back up folders now](#4-back-up-folders-now)
5. [Restore](#5-restore)
6. [Servers and encryption keys](#6-servers-and-encryption-keys)
7. [Reports and log files](#7-reports-and-log-files)
8. [Preferences](#8-preferences)
9. [Keys](#9-keys)
10. [Scripts and checks](#10-scripts-and-checks)
11. [What it does not do yet](#11-what-it-does-not-do-yet)

## 1. Start it

Run it as **administrator** (Windows) or **root** (Linux), with the Salvanza service running:

```
salvanza-cli tui
```

The service gives its API token only to administrators and root. A launcher that already has the token can pass it in the environment variable `SALVANZA_API_TOKEN`; `-token-file <file>` names the token file when it is not in the service's settings folder, and `-url <address>` the service's address when it is not `http://127.0.0.1:18765`.

`salvanza-cli tui -demo` shows invented data and needs no service: a good way to look around. Everything works in it, but nothing is stored anywhere.

If the window says "the Salvanza service is not running or not reachable", start the service ([GUI manual, section 15](GUI.md#15-background-service-tray-and-closing-the-window)). If it says the service refused the token, you are not running as administrator or root.

The window needs a terminal at least 80 columns wide and 24 lines high; it uses more room when there is any. Closing it does not stop backups: they run in the service.

## 2. The screens

The top line shows whether the service answers, its version and how many backups run. The numbers 1 to 9, the function keys F1 to F9 and **Tab** switch screens. (On Preferences and on the typing steps of Restore the characters belong to the field you are in, so use the function keys there.)

| Screen | Shows |
|---|---|
| 1 Dashboard | The Backup Sets with their next and last run and result, the backups running now with progress bars, and the latest runs |
| 2 Backup Sets | The same list with the selected set's details beside it |
| 3 History | Every run recorded, newest first. **Enter** opens the run's log |
| 4 Messages | The Message Log |
| 5 Servers | The Proxmox Backup Servers set up on this computer, and their management ([section 6](#6-servers)) |
| 6 Restore | Browse a backup and restore files ([section 5](#5-restore)) |
| 7 Reports | Runs, successes and failures for each Backup Set ([section 7](#7-reports-and-log-files)) |
| 8 Logs | The service's log files |
| 9 Preferences | Backup options and the e-mail account ([section 8](#8-preferences)) |

On a narrow terminal the screen names in the top bar are shortened.

## 3. Backup Sets

On the Dashboard or the Backup Sets screen:

| Key | Does |
|---|---|
| **r** | Runs the selected set now |
| **p** | Pauses the running backup, or resumes it |
| **s** | Stops the running backup, after asking. No snapshot is kept |
| **e** | Turns the set on or off |
| **Enter** | Edits the set |
| **n** | Makes a new folder set |
| **d** | Deletes the set, after asking. Its backups on the server stay |
| **u** | Undelete: puts back files that are in the set's backups but gone from the disk ([below](#undelete-and-roll-back)) |
| **k** | Roll back: puts the set's folders back as an earlier backup had them ([below](#undelete-and-roll-back)) |

The editor has three steps like the app's: **General** (name, backup ID, folders, server), **Schedule** (on a schedule or manual; every N minutes, hours, days, weeks or months; time of day; days of the week) and **Options** (change detection, VSS, files to leave out, e-mail when it fails). **PgDn** and **PgUp** move between the steps, **Ctrl+S** saves and **Esc** cancels. The service checks what you save, as for the app, and says what is wrong. A set that has several schedules keeps the others: only the first is edited here. The disks of a machine set are chosen in the app.

### Undelete and Roll back

Both work on the selected Backup Set through the service program (`salvanza-svc`, the same engine as the desktop app and the [command line in the GUI manual](GUI.md#undelete-and-roll-back-from-the-command-line)), so they need the same account as the rest of this window: administrator or root. The window finds the program next to itself, in the install folders or on the search path; the variable `SALVANZA_SVC` names it when it is somewhere else. Its output appears in the window as it comes; **Esc** closes the window (work that has started goes on).

**Undelete (u).** Choose how far back to look (0 is the newest backup only; N also looks in the last N days) and, if you like, a folder to restore under instead of where the files were. **Look** lists the files that are in the backups but not on disk and changes nothing. **Restore** puts them back, never over a file that has come back. Files that look moved are listed, not restored. The restored files are checked against the backup unless you turn that off.

**Roll back (k).** Choose the backup (the newest, or the newest at or before a local time written `2026-10-10 14:30`) and what to do: **replace** (restore missing and changed files, keep files added since), **missing** (only files that are gone) or **exact** (also remove files added since). **Preview** shows the counts and changes nothing. **Roll back** shows the same counts and then waits: press **y** to go ahead, **Esc** to cancel. Everything replaced or removed is first moved to a safety folder; **undoing a roll back is done in the desktop app**.

## 4. Back up folders now

**b** opens a form for a one-off backup without a Backup Set: the folders (one per line), backup ID, server, compression, VSS, files to leave out and a comment. **Ctrl+S** starts it. It then shows under Running now like any other backup.

## 5. Restore

Screen 6 follows the app's five steps.

1. **Source.** Pick the server (**Tab** moves between the server and the list) and then a backup group, such as `host/DESKTOP-01`.
2. **Snapshot.** Pick one of its snapshots.
3. **Files.** The snapshot's files appear as a tree from its catalog. **Right** opens a folder and **Left** closes it, **Space** ticks a file or folder (a ticked folder takes everything in it), **a** ticks everything and **c** clears the ticks. Nothing ticked restores the whole snapshot. **PgDn** goes on.
4. **Destination.** The folder to restore into (a new one named with the date is suggested) and whether to overwrite files that exist. Choose **Restore**.
5. **Restore.** The files are written as they are read from the server. **Esc** starts over when it has finished.

A snapshot of several folders restores each into a folder of its own below the destination.

**Encrypted backups.** The server's encryption key is the one the service has stored for it (set up under [Servers and encryption keys](#6-servers-and-encryption-keys) or in the app). If the key has a passphrase and the service has not stored it, the window asks for it once, masked, when you open the snapshot, and uses it for the restore. A wrong passphrase is explained and asked for again. A server with no key set up says so.

## 6. Servers and encryption keys

Screen 5 lists the servers; the details of the selected one show beside the list, never its login.

| Key | Does |
|---|---|
| **Enter** | Edits the server |
| **n** | Adds a server |
| **d** | Deletes it, after asking. Its backups stay on it |
| **t** | Tests the connection (address, login and datastore) |
| **u** | Makes it the default server |
| **f** | Reads the certificate the server presents and pins its fingerprint, after you have compared it with the one in the Proxmox Backup Server (Dashboard, Show Fingerprint) |

The editor has three steps. **Server**: name, address (`https://pbs.example.com:8007`), fingerprint, datastore, namespace, description. **Fetch fingerprint** fills the fingerprint in from the server; compare it with the one in the Proxmox Backup Server before you save. **Login**: an API token (token ID and secret) or a user name and password. On an existing server leave a secret or password empty to keep the stored one. The service checks the server on saving, as for the app. **Encryption** is the third step ([below](#encryption-keys)).

### Encryption keys

A server's backups are encrypted with the key whose file is named in the server's **Encryption** step; with no key file the backups are not encrypted. The key file stays in the service's keys folder. The window never shows a key, only its fingerprint and where the file is.

| Button | Does |
|---|---|
| **Generate key** | Makes a new key (without a passphrase, as the app does) in the keys folder and puts it in the **Key file** field. It never overwrites an existing file. A server that has a key already asks first, because a new key cannot read what the old one encrypted |
| **Import key file** | Reads a key file from anywhere on this computer (a USB stick, a backup copy), checks it, shows its **fingerprint** to compare with your notes or paper copy, and stores a copy in the keys folder. A key without a passphrase must match its own fingerprint; a protected key is checked when you give its passphrase, which also lets it be stored. An existing key file is never overwritten |
| **Inspect** | Shows the key file's fingerprint, creation date, whether it has a passphrase, and whether other accounts can read it |
| **Restrict access** | Limits who can read the key file to the service and administrators (or its owner on Linux) |

**Key passphrase** stores the passphrase of a protected key in the service so scheduled backups can run alone; leave it empty to keep what is stored, or tick **Forget the stored passphrase**. Without a stored passphrase the window asks for it when it needs the key, for a restore. The server's details on the Servers screen, and each Backup Set's details, say whether backups are encrypted, with which key, and warn when the key file can be read by other accounts.

Save the server to use a new key. **Keep a copy of the key file somewhere safe: without it the backups cannot be restored.**

Printing the key as a **paper key**, and reading a paper key back (from its QR code or typed text), are done in the desktop app only, on purpose: the window never puts the key itself on screen.

## 7. Reports and log files

**Reports** (7) shows, for each Backup Set, how many runs there were, how many succeeded and failed, the success rate and the last result, with the latest failures and their reasons below. **w** changes the period: all recorded runs, the last 7 days, the last 30 days.

**Logs** (8) lists the service's log files. **Enter** opens one. In a log, **/** filters the lines (type a word and press Enter; an empty filter shows everything), **c** leaves out the chunk lines, **Esc** goes back, and the arrow keys, **PgUp**, **PgDn**, **Home** and **End** scroll. History's **Enter** opens the log of that run the same way.

## 8. Preferences

Screen 9 has four pages, **Backup options**, **E-mail**, **Network shares** and **Settings file**, switched with **PgDn** and **PgUp** (or the buttons). **Ctrl+S** saves, **Revert** reads the stored values again.

* **Backup options:** speed limits for upload, download and disk reading (MB/s, 0 for none), upload threads (0 for automatic), the chunk timeout (30 to 1800 seconds, 0 for the default of 90), compression, how many backups run at once, restoring files in parallel, and reading folders ahead while backing up, with the number of workers.
* **E-mail:** the account Backup Sets use to e-mail their results: server, port, user name, password (leave empty to keep the stored one), whether to accept an untrusted certificate, the sender address. **Save and send test message** saves the account and sends a short message to the address you give.

* **Network shares:** the login the service uses to reach another computer's share (`\\server\share`) when the backups run. Give the server, user name and password (leave the password empty to keep the stored one); a folder on the share can be tried before the login is saved. **Remove login** takes it away, **Check folder** tells whether the backups can read a folder on a share, with the saved login. The password is never shown.
* **Settings file:** **Export** writes the Backup Sets, servers and preferences to a new file (never over an existing one; with **Include passwords and secrets** ticked the file is readable only by you and must be kept safe). **Import** merges such a file into this computer's settings, after asking, and says what it added.

Only what you changed is sent, through the same settings the app uses, so the service's own checks and limits apply and tell you what is wrong.

## 9. Keys

| Key | Does |
|---|---|
| **Tab**, **1** to **9**, **F1** to **F9** | Switch screen |
| **Up**, **Down**, **PgUp**, **PgDn**, **Home**, **End** | Move in a list |
| **g** | Refresh now (the window also refreshes every 2 seconds) |
| **?** | Help |
| **q**, **Ctrl+C** | Quit |

Each screen's own keys are in its section and in the bar at the bottom. The mouse works in terminals that pass it on.

## 10. Scripts and checks

`salvanza-cli tui -check` connects, prints what the service reports (its version, the Backup Sets, history and server counts) and exits with 0, or with 1 when the service did not answer. It needs no terminal, so it suits a health check or a script. Some options go further and change or read things, so use them on a test machine:

* `-check -write` makes a temporary manual Backup Set and a temporary server, changes and deletes them, changes one preference and puts it back, and reads a log file, to prove the service takes what the window sends.
* `-check -encryption` proves the whole encryption chain against a real Proxmox Backup Server (named in the environment variables `TUI_CHECK_PBS_URL`, `_AUTHID`, `_SECRET`, `_DATASTORE`, `_FINGERPRINT`): it makes a key, lets a temporary server use it, backs a folder up, restores it with the server's key and compares the files, then removes the server and the test backups. The key file it made stays in the keys folder; its path is printed as `KEYFILE=`.
* `-check -recover` (same environment variables) backs a folder up with a temporary Backup Set, deletes one file, changes one and adds one, then proves Undelete (the file comes back, and the look changes nothing) and Roll back with **exact** (the changed file is restored, the added one removed, the preview changes nothing), and removes everything it made. It needs the service program, as above.
* `-check -write` also saves, lists and removes a share login, and exports the settings to a temporary file and imports them back.
* `-check -restore-check` restores a few small files of the newest readable snapshot into a temporary folder and deletes it. `-restore-group <backup ID>` tries only that backup, which also shows why an encrypted one cannot be read.

## 11. What it does not do yet

* **Undoing a roll back**, and deleting the safety copies it keeps, are in the app only.
* The **paper key** (printing it, reading it back) is in the app only, by design.
* A disk image (a machine backup) is not restored here: use the [bare-metal restore](BMR.md).
* Machine Backup Sets can be edited here but not made.
* The window is in English only.
