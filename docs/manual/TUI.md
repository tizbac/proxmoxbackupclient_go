# The text window: user manual

`salvanza-cli tui` is the desktop app's Backup Sets, running backups, history and restore in a terminal. Use it on a server without a desktop, over SSH, or whenever a text screen is quicker than the app. It works with the computer's background service, the same one the desktop app uses, so what you do here shows in the app and the other way round.

It is new after version 2.0.

## Contents

1. [Start it](#1-start-it)
2. [The screens](#2-the-screens)
3. [Backup Sets](#3-backup-sets)
4. [Back up folders now](#4-back-up-folders-now)
5. [Restore](#5-restore)
6. [Keys](#6-keys)
7. [Scripts and checks](#7-scripts-and-checks)
8. [What it does not do yet](#8-what-it-does-not-do-yet)

## 1. Start it

Run it as **administrator** (Windows) or **root** (Linux), with the Salvanza service running:

```
salvanza-cli tui
```

The service gives its API token only to administrators and root. A launcher that already has the token can pass it in the environment variable `SALVANZA_API_TOKEN`; `-token-file <file>` names the token file when it is not in the service's settings folder, and `-url <address>` the service's address when it is not `http://127.0.0.1:18765`.

`salvanza-cli tui -demo` shows invented data and needs no service: a good way to look around.

If the window says "the Salvanza service is not running or not reachable", start the service ([GUI manual, section 15](GUI.md#15-background-service-tray-and-closing-the-window)). If it says the service refused the token, you are not running as administrator or root.

The window needs a terminal at least 80 columns wide and 24 lines high; it uses more room when there is any. Closing it does not stop backups: they run in the service.

## 2. The screens

The top line shows whether the service answers, its version and how many backups run. The numbers 1 to 6, the function keys F1 to F6 and **Tab** switch screens.

| Screen | Shows |
|---|---|
| 1 Dashboard | The Backup Sets with their next and last run and result, the backups running now with progress bars, and the latest runs |
| 2 Backup Sets | The same list with the selected set's details beside it |
| 3 History | Every run recorded, newest first. **Enter** opens the run's log |
| 4 Messages | The Message Log |
| 5 Servers | The Proxmox Backup Servers set up on this computer, without their logins |
| 6 Restore | Browse a backup and restore files ([section 5](#5-restore)) |

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

The editor has three steps like the app's: **General** (name, backup ID, folders, server), **Schedule** (on a schedule or manual; every N minutes, hours, days, weeks or months; time of day; days of the week) and **Options** (change detection, VSS, files to leave out, e-mail when it fails). **PgDn** and **PgUp** move between the steps, **Ctrl+S** saves and **Esc** cancels. The service checks what you save, as for the app, and says what is wrong. A set that has several schedules keeps the others: only the first is edited here. The disks of a machine set are chosen in the app.

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

## 6. Keys

| Key | Does |
|---|---|
| **Tab**, **1** to **6**, **F1** to **F6** | Switch screen |
| **Up**, **Down**, **PgUp**, **PgDn**, **Home**, **End** | Move in a list |
| **g** | Refresh now (the window also refreshes every 2 seconds) |
| **?** | Help |
| **q**, **Ctrl+C** | Quit |

The mouse works in terminals that pass it on.

## 7. Scripts and checks

`salvanza-cli tui -check` connects, prints what the service reports (its version, the Backup Sets, history and server counts) and exits with 0, or with 1 when the service did not answer. It needs no terminal, so it suits a health check or a script. Two options go further and change or read things, so use them on a test machine:

* `-check -write` makes a temporary manual Backup Set, changes it and deletes it again, to prove the service takes what the editor sends.
* `-check -restore-check` restores a few small files of the newest readable snapshot into a temporary folder and deletes it.

## 8. What it does not do yet

* Undelete and Roll back, Reports, Preferences and the setting up of servers and encryption keys are in the app only (or in the service's command line, see the [GUI manual](GUI.md#undelete-and-roll-back-from-the-command-line)).
* Encrypted snapshots are refused with a message: restore them with the app, which holds the key.
* A disk image (a machine backup) is not restored here: use the [bare-metal restore](BMR.md).
* Machine Backup Sets can be edited here but not made.
