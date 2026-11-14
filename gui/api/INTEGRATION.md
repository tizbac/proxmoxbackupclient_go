# Local service API

The background service of Salvanza for Proxmox Backup Server (`salvanza-svc`, Windows
service `salvanza-svc` or systemd unit `salvanza.service`; a branded build's has its own
name) serves a
small HTTP API on `127.0.0.1:18765`. The app uses it to hand backups to the service and to
follow and control the service's runs. This package holds both sides:

| File | |
| ---- | --- |
| `server.go` | the routes (`Server`, started by `gui/service.go`) and the interfaces the service's `App` implements |
| `client.go` | the client the app uses (`Client`) |
| `types.go` | request and response bodies |
| `auth.go` | the token file and the checks applied to every request |
| `peer.go` | the app's check that it is the service answering, on each connection |
| `mode.go` | `ModeDetector`: service mode or standalone |

## How the app uses it

At startup the app calls `GET /status` (`Client.ProbeStatus`, `gui/service_connect.go`).
If the service answers (200, or 401 when the app has no token yet), or is installed but not
answering yet, the app is in **service mode**: the service keeps the computer's settings,
Backup Sets, history and Message Log, runs every backup, and the app works on them only
through this API. Otherwise it is **standalone** and keeps its own settings in
`~/.salvanza`. Preferences > Advanced > Run as Service switches between the
two (`gui/service_switch.go`). A user who cannot connect may choose **Use my own settings**
(standalone with the service running).

In service mode the app:

- starts a backup with `POST /backup` or `POST /backup/machine`, then polls
  `GET /backup/status/{job_id}` every 3 seconds until `complete` is true;
- lists `GET /backup/runs` every 3 seconds and shows a progress card for a run it did not
  start (a scheduled one), with Pause, Resume and Stop sent to `/backup/pause`,
  `/backup/resume` and `/backup/stop`;
- while it holds no token (not connected yet, or after its window has been in the tray for
  15 minutes: `gui/token_drop.go`), reads `GET /status/public` every 3 seconds instead, for
  the tray (how many backups run and wait, whether one failed);
- asks `GET /status` whether the service is busy (`active_jobs`);
- reads the settings with `GET /config` (no secrets: `*_set` markers instead) and changes
  them with `POST /call/{name}`, which runs one of the app's own setters in the service
  (the list is `apiCalls` in `gui/service_api.go`);
- gets what one restore, browse or search needs from `POST /pbs/access`;
- reads and adds history (`/history`, `/history/append`) and Message Log entries
  (`/messages`), lists and controls Backup Sets (`/jobs/full`, `/jobs/enabled`,
  `/call/RunScheduledJobNowRead`), exports and imports settings (`/settings/export`,
  `/settings/import`) and stores keys (`/keys/save`);
- reads the service's logs for its Logs page, Save logs and the service log folder button
  (`/logs`, `/logs/read`, `/logs/run`, `/logs/bundle`), with or without the token, so a user
  who is not an administrator, or who works on their own settings, still sees them;
- sends a certificate pin to `POST /pbs/fingerprint`.

Each of these routes is served when the service has the handler and answers **501**
otherwise (`gui/api/server_state.go`).

## Authentication

Every request except `GET /status/public`, `GET /servers/public` and the four log requests
(`GET /logs`, `POST /logs/read`, `GET /logs/run`, `POST /logs/bundle`) must carry the header

```
X-Salvanza-Token: <contents of the token file>
```

The header's name before the rename to Salvanza, `X-Proxmox-Client-Token`, is still
accepted for now, so scripts written for Proxmox Backup Client keep working; the app sends
`X-Salvanza-Token`.

The token file is `api-token` in the service's settings folder (a branded build's own
folder, such as `C:\ProgramData\NimbusBackup`):

| | Path | Who can read it |
| --- | --- | --- |
| Windows | `C:\ProgramData\Salvanza\api-token` | SYSTEM (full control) and the Administrators group (read), protected; a program without elevation cannot |
| Linux | `/var/lib/salvanza/api-token` | root only (0600 in a 0700 root-owned folder) |

A launcher that already has the token can give it in the environment variable
`SALVANZA_API_TOKEN` (`PBSGO_API_TOKEN`, its earlier name, is still read); the app itself
never sets it. `SALVANZA_TOKEN_ROTATION` and `SALVANZA_TOKEN_GRACE` (durations such as
`24h` and `10m`; a rotation of `0` turns it off) change the service's rotation interval and
grace window below; the earlier `PBSGO_TOKEN_ROTATION` and `PBSGO_TOKEN_GRACE` are still
read.

The service writes a new random token (32 bytes, hex) each time it starts, before the API
starts (`NewToken`, an atomic replace), after it has locked the settings folder down, and
replaces it every 24 hours (`TokenRotator`; the replaced one is accepted for 10 more
minutes). A token copied earlier, or a file planted before the service first ran, stops
working at the next start. If the token cannot be written the server keeps an empty token
and refuses every request.

The app, run without elevation, cannot read the file: it starts a copy of itself elevated
(UAC on Windows, pkexec on Linux) with `--elevated-token-fetch <file>`, which writes the
token into a file the app created and exits (`gui/token_elevated.go`). The app keeps the
token in memory only and asks again only when the user connects (`gui/service_connect.go`).
An elevated app, or one run as root, reads the file itself.

While the app holds the token, any program running as the same user could read it from the
app's memory. So the app holds it only while its window is in use: once the window has been
hidden in the tray for 15 minutes, the app drops the token (an elevated app also stops
reading the file) and is not connected until its window is shown again, when it asks once
more (`gui/token_drop.go`). Meanwhile the tray reads `GET /status/public`, which needs no
token.

The app also checks the service before it sends anything. While the service is stopped,
another program could listen on `127.0.0.1:18765` and pose as it, to collect the token or a
PBS secret typed into Preferences. So on each new connection to that address, before a byte
is sent, the app looks up who holds the other end of that connection (`peer.go`):

| | The other end must be |
| --- | --- |
| Windows | the service's own process: the process that owns the socket in the system's TCP table (`GetExtendedTcpTable`) has the process ID the Service Control Manager gives for the running service (`gui/service_peer_windows.go`) |
| Linux | root's (uid 0), from `/proc/net/tcp` and `/proc/net/tcp6`: the connection's socket and every socket listening on port 18765 (until it is accepted, many kernels show a connection as root's whoever listens); the service runs as root, and only root can listen as root |
| macOS, others | not checked yet |

Anything else is refused: the connection is closed unused, the debug log gets a line such as
`something other than the background service is answering on 127.0.0.1:18765 (process 4321
(python.exe)): not connecting`, and the app behaves as if the service were not running.
Other programs using the API are not affected; they can make the same check.

Checks on every request (`authMiddleware`):

- request body at most 8 MiB;
- a request with an `Origin` header gets **403** `forbidden` (browsers send it; the app
  does not);
- a missing or wrong token gets **401** `unauthorized` (constant-time compare), except on
  `GET /status/public`, `GET /servers/public` and the four log requests (any other method
  there, and any other path under `/logs`, needs the token too);
- one of those requests without the token must name this computer in its `Host` header
  (`127.0.0.1` or any `127.x`, `::1` or `localhost`, with or without a port), or it gets
  **403** `forbidden`. A web page that points its own host name at 127.0.0.1 (DNS
  rebinding) sends that name, so it is refused. With the token the `Host` does not matter.

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
  "version": "2.0.0",
  "active_jobs": 0,
  "configuration": {
    "hostname": "PC01",
    "baseurl": "https://pbs.example:8007",
    "datastore": "backup",
    "certfingerprint": "...",
    "backup-id": "PC01",
    "encryption_key_file": "C:\\ProgramData\\Salvanza\\keys\\encryption-key.json"
  }
}
```

`active_jobs` counts runs on the API that are still running. `configuration` never holds
secrets (no token secret, no passphrase). `last_backup` exists in the type but is never set.

### GET /status/public

One of the routes that need no token (`public_status.go`). Any program on the computer may read
it, so it says only whether backups run and whether one failed: no Backup Set names, paths,
servers or messages. The app's tray uses it while the app holds no token. Every field is
always present:

```json
{
  "running": true,
  "version": "2.0.0",
  "backups_running": 1,
  "backups_waiting": 0,
  "last_run_failed": false,
  "last_run_ended": "2026-11-02T21:04:05Z"
}
```

- `running` is always true (the service answered).
- `version` is the service's build version, as in `GET /status`.
- `backups_running` and `backups_waiting` count the service's backups in progress and those
  queued behind another (their message starts with `Queued: `).
- `last_run_failed` is true when the most recent run of at least one Backup Set failed
  (from the run history, matched to the current Backup Sets by each run's set ID, so a
  deleted set's runs do not count for a new set given its name; restores and one-off
  backups do not count).
- `last_run_ended` is when the most recent backup ended (RFC 3339), `""` when none is
  recorded.

The service reads the history for the last two fields at most once every 5 seconds. The
`Origin` check still applies, and so does the app's peer check: the app reads the route only
from the real service. An earlier service without the route answers it **401**.

### GET /servers/public

Another route that needs no token (`public_servers.go`): the computer's PBS servers,
without their logins. A user who works on their own settings (Use my own settings) holds
no token; the app's Preferences offer them these servers, so they can add one with their
own API token or username and password instead of typing its address, datastore and
fingerprint again. So the list says where each server is and how it is reached, and
nothing that logs in to it: no token ID, username, secret, password, passphrase or key
file. Sorted by name; every field is always present:

```json
[
  {
    "id": "office",
    "name": "Office PBS",
    "base_url": "https://pbs.example.com:8007",
    "datastore": "main",
    "namespace": "office",
    "fingerprint": "aa:bb:...",
    "login": "token",
    "encrypted": true
  }
]
```

- `login` is how the computer logs in to the server: `token` (an API token) or `password`
  (a username and password). The app opens that tab of the server form for the user's own.
- `encrypted` is true when the computer's backups to the server are encrypted. The key is
  not shared.

With no servers the answer is `[]`. The `Origin` check and the app's peer check apply as to
`GET /status/public`. An earlier service without the route answers it **401**.

### GET /logs, POST /logs/read, GET /logs/run, POST /logs/bundle

The service's own logs, for any program on the computer, with or without the token
(`public_logs.go`). The logs are in the service's settings folder, which only the service
and an administrator may read, and a user who is not an administrator still needs to see
why a backup failed and to attach the logs to a bug report. Only log files are served
(`service-*.log`, `backup-*.log` and their rotated and compressed parts), never the
settings, the token or a key, and the settings folder stays locked.

What any user of the computer can read through these: the folders and files the service
backs up, the PBS servers, datastores and login names. Never a secret: the log calls mask
those, server addresses are logged without any `user:password@` part, and a Backup Set's
before and after commands are not logged, nor what they print.

- `GET /logs` lists the logs, newest first: `[{ "folder": 0, "name": "service-service.log",
  "kind": "app", "size": 1234, "modified": 1794528558000, "compressed": false,
  "service": true }]`. A run log also has `backupId` and `started` (Unix ms).
- `POST /logs/read` gives part of one: request `{ "name": "...", "filter": "", "hideChunks":
  false, "lines": 2000, "around": "" }` (`filter`: only lines holding it, any case;
  `around`: a time, `2006-01-02 15:04:05` or its first part, for the lines around it instead
  of the end); answer `{ "lines": [...], "matching": 0, "first": 0, "focus": -1 }`. **400**
  for a name that is not one of the service's logs.
- `GET /logs/run?backup_id=<id>&at=<Unix ms>` finds the log of that run, as one entry of the
  list; **404** when there is none.
- `POST /logs/bundle` with `{ "days": 14 }` (0 to 365; 0 means the default 14) answers
  `{ "data": "<zip, base64>" }`: the logs of those days, newest first, up to a size limit,
  with an `info.txt`.

They are answered one at a time, and an answer has 2 minutes to reach the caller, so a
program that asks in a loop, or asks and does not read, cannot fill the service's memory or
hold the others up for longer. The `Origin` and `Host` checks apply. An earlier service
answers them **401** without the token.

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
  "compression": "",
  "pbs_server_id": "",
  "comment": "",
  "change_detection_mode": ""
}
```

- `backup_id`: empty means the computer's name, as for a Backup Set without one (version
  1.2 and earlier refused an empty one with 400 `backup_id is required`).
- `compression`: `none`, `fastest`, `default` (zstd level 3), `better` or `best` for this run; empty
  means the computer's Backup Options setting, which is `default` unless changed. Earlier
  builds took empty as `fastest`. An encrypted backup is compressed before it is encrypted.
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
- stop: it is cancelled at once, part way through a file if need be (a folder backup checks
  before every 64 KiB block it reads), and also while paused. No snapshot is kept. Reports
  record it as cancelled.

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

Only a caller holding the token reaches these, so they are not gated further (the earlier
"require administrator" policy is gone).

### DELETE or POST /jobs/delete/{id}

Deletes a Backup Set. Response `{ "success": true, "message": "Job deleted successfully" }`.
**400** without an ID.

### POST /pbs/fingerprint

Pins a PBS server's certificate fingerprint in `config.json`, written by the service.

Request: `{ "id": "<pbs server id>", "fingerprint": "AA:BB:..." }`. Both are required
(**400** otherwise). The fingerprint is validated. Response:
`{ "success": true, "message": "Fingerprint pinned successfully" }`; **500** with the reason
if it cannot be saved.

## Example

PowerShell, run as administrator (elevated) on Windows:

```powershell
$h = @{ 'X-Salvanza-Token' = (Get-Content C:\ProgramData\Salvanza\api-token -Raw).Trim() }
Invoke-RestMethod http://127.0.0.1:18765/status -Headers $h
Invoke-RestMethod http://127.0.0.1:18765/backup/runs -Headers $h
```

Linux, as root:

```sh
curl -s -H "X-Salvanza-Token: $(cat /var/lib/salvanza/api-token)" http://127.0.0.1:18765/status
```

The public status and the service's logs need neither elevation nor the token:

```sh
curl -s http://127.0.0.1:18765/status/public
curl -s http://127.0.0.1:18765/logs
```
