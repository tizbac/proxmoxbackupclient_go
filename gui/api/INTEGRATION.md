# Local service API

The background service (`ProxmoxBackupClientSVC`, Windows service or systemd unit) serves a
small HTTP API on `127.0.0.1:18765`. The app uses it to hand backups to the service and to
follow and control the service's runs. This package holds both sides:

| File | |
| ---- | --- |
| `server.go` | the routes (`Server`, started by `gui/service.go`) and the interfaces the service's `App` implements |
| `client.go` | the client the app uses (`Client`) |
| `types.go` | request and response bodies |
| `auth.go` | the token file and the checks applied to every request |
| `mode.go` | `ModeDetector`: service mode or standalone |

## How the app uses it

At startup the app calls `GET /status` (`ModeDetector.DetectMode`). If it answers 200 the
app is in **service mode**: it does not run its own scheduler, it sends Run Now and one-off
backups to the service, and it follows the service's runs. Otherwise it is **standalone**
and runs backups and schedules itself. Preferences > Advanced > Run as Service switches
between the two (`gui/service_switch.go`).

In service mode the app:

- starts a backup with `POST /backup` or `POST /backup/machine`, then polls
  `GET /backup/status/{job_id}` every 3 seconds until `complete` is true;
- lists `GET /backup/runs` every 3 seconds and shows a progress card for a run it did not
  start (a scheduled one), with Pause, Resume and Stop sent to `/backup/pause`,
  `/backup/resume` and `/backup/stop`;
- asks `GET /status` whether the service is busy (`active_jobs`);
- sends a certificate pin to `POST /pbs/fingerprint`, because only the service may write
  `config.json`.

The `/jobs` routes are served but the app does not call them: it reads and writes the
Backup Sets file itself.

## Authentication

Every request must carry the header

```
X-Proxmox-Client-Token: <contents of the token file>
```

The token file is `api-token` in the settings folder:

| | Path | Who can read it |
| --- | --- | --- |
| Windows | `C:\ProgramData\ProxmoxBackupClient\api-token` | the folder's permissions: SYSTEM, Administrators, local administrator accounts (read) |
| Linux | `/var/lib/proxmoxbackupclient/api-token` | root, and the sudo or wheel group (0640 in a 2750 root-owned folder) |

The service writes a new random token (32 bytes, hex) each time it starts, before the API
starts (`NewToken`, an atomic replace), after it has locked the settings folder down. A
token copied earlier, or a file planted before the service first ran, stops working at the
next restart. The client reads the file on every request, so a running app picks up a new
token without a restart. If the token cannot be written the server keeps an empty token
and refuses every request.

Checks on every request (`authMiddleware`):

- request body at most 1 MiB;
- a request with an `Origin` header gets **403** `forbidden` (browsers send it; the app
  does not);
- a missing or wrong token gets **401** `unauthorized` (constant-time compare).

## Errors

Errors are JSON with the HTTP status repeated in `code`:

```json
{ "error": "Invalid request: unexpected EOF", "code": 400 }
```

A wrong method gets **405** `Method not allowed`.

## Routes

All bodies are JSON. Field names are exactly as shown.

### GET /status

Whether the service is up, and what it runs with.

```json
{
  "running": true,
  "version": "0.9.0",
  "active_jobs": 0,
  "configuration": {
    "hostname": "PC01",
    "baseurl": "https://pbs.example:8007",
    "datastore": "backup",
    "certfingerprint": "...",
    "backup-id": "PC01",
    "encryption_key_file": "C:\\ProgramData\\ProxmoxBackupClient\\keys\\encryption-key.json"
  }
}
```

`active_jobs` counts runs on the API that are still running. `configuration` never holds
secrets (no token secret, no passphrase). `last_backup` exists in the type but is never set.

### POST /backup

Starts a folder backup. Answers at once; the backup runs in the background.

Request (`BackupRequest`):

```json
{
  "backup_type": "directory",
  "backup_id": "PC01",
  "backup_dirs": ["C:\\Users\\me\\Documents"],
  "drive_letters": [],
  "exclude_list": ["*.tmp"],
  "use_vss": true,
  "compression": "fastest",
  "pbs_server_id": "",
  "comment": "",
  "change_detection_mode": ""
}
```

- `backup_id`: empty means the computer's name, as for a Backup Set without one (before
  0.9.2 an empty one was refused with 400 `backup_id is required`).
- `compression`: `fastest` (default when empty), `default`, `better` or `best`.
- `pbs_server_id`: empty means the default server.
- `comment` goes to the snapshot's manifest comment as is.
- `change_detection_mode`: `""`, `legacy`, `data` or `metadata`, for a directory Backup Set run.

Before starting, the service reloads `config.json`, so changes made in the app apply.

Response (`BackupResponse`):

```json
{ "success": true, "message": "Backup started successfully (running in background)", "job_id": "backup-1791234567-12" }
```

### POST /backup/machine

Starts a machine (disk image) backup. Same request and response as `/backup`. The disk
devices to back up go in `drive_letters`, as the app's disk picker lists them (for example
`\\.\PhysicalDrive0`); `backup_dirs`, `exclude_list` and `change_detection_mode` are not
used.

### GET /backup/status/{job_id}

The state of one run (`BackupProgress`). **404** `Job not found` for an unknown ID.

```json
{
  "job_id": "backup-1791234567-12",
  "running": true,
  "progress": 37.5,
  "message": "...",
  "success": false,
  "complete": false,
  "start_time": "2026-10-13T02:00:00+01:00",
  "paused": false
}
```

- `progress` is 0 to 100. When `complete` is true, `success` says how it ended and `error`
  holds the reason of a failure.
- `name` and `scheduled: true` are set on a run the service started from a schedule.
- `paused` and `paused_since` (Unix ms) are added while a running backup is paused.
- An entry is removed 10 minutes after its run ends.

### GET /backup/runs

All runs the API knows: those in progress and those that ended in the last 10 minutes,
oldest first. Each is a `BackupProgress` as above.

```json
{ "runs": [ { "job_id": "scheduled-1791230000-3", "name": "Documents", "scheduled": true, "running": true, "progress": 12, "message": "...", "success": false, "complete": false, "start_time": "..." } ] }
```

Run IDs start with `backup-` for a run the app asked for and `scheduled-` for one the
service started.

### POST /backup/pause, POST /backup/resume, POST /backup/stop

Act on the backup the service is running, whoever started it. No request body.

- pause: the backup waits at its next safe point (between files, between 64 KiB blocks of
  a large file, or between disk image blocks). The PBS session stays open.
- resume: it carries on.
- stop: it is cancelled after the current file. Reports record it as cancelled.

Response (`RunControlResponse`), the pause state afterwards:

```json
{ "paused": true, "paused_since": 1791234600000 }
```

**409** with `no backup is running` when pause or stop finds nothing to act on.
**501** if the service's handler does not implement `RunController`.

### GET /jobs

The Backup Sets, in a short form (`JobsResponse`):

```json
{ "jobs": [ { "id": "...", "name": "Documents", "backup_type": "directory", "schedule": "02:00", "last_run": "...", "next_run": "...", "status": "idle" } ] }
```

`schedule` is the daily time field. `status` is always `idle` (not tracked).

### POST /jobs/create, POST or PUT /jobs/update

Add or change a Backup Set. The body is one Backup Set in the same form as in
`scheduled_jobs.json` (the `ScheduledJob` struct in `gui/scheduler.go`, camelCase field
names such as `scheduleTime` and `backupDirs`).

Response: `{ "success": true, "message": "Job created successfully" }` (or `updated`).
A failure is **500** `Failed to create job: <reason>`.

Through the API these are refused (`JOBS_ADMIN_REQUIRED`) whenever job protection applies:
on Windows while the "require administrator" policy is on (the default), and on Linux
always.

### DELETE or POST /jobs/delete/{id}

Deletes a Backup Set. Response `{ "success": true, "message": "Job deleted successfully" }`.
**400** without an ID; refused like create and update under job protection.

### POST /pbs/fingerprint

Pins a PBS server's certificate fingerprint in `config.json`, written by the service.

Request: `{ "id": "<pbs server id>", "fingerprint": "AA:BB:..." }`. Both are required
(**400** otherwise). The fingerprint is validated. Response:
`{ "success": true, "message": "Fingerprint pinned successfully" }`; **500** with the reason
if it cannot be saved.

## Example

PowerShell, as an administrator on Windows:

```powershell
$h = @{ 'X-Proxmox-Client-Token' = (Get-Content C:\ProgramData\ProxmoxBackupClient\api-token -Raw).Trim() }
Invoke-RestMethod http://127.0.0.1:18765/status -Headers $h
Invoke-RestMethod http://127.0.0.1:18765/backup/runs -Headers $h
```

Linux, as root or a member of sudo or wheel:

```sh
curl -s -H "X-Proxmox-Client-Token: $(cat /var/lib/proxmoxbackupclient/api-token)" http://127.0.0.1:18765/status
```
