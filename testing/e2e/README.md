# End-to-end test suite

Automated end-to-end tests that back up data with **our** CLIs, restore it with
the **official** `proxmox-backup-client`, and verify the result is byte-identical.

The official client is the interoperability contract: if it can read our archives
and reproduce the original bytes, our chunking, blob framing, index format and
manifest signing are all correct.

## Running

```bash
./testing/e2e/run.sh              # everything
make test-e2e                      # same thing, via the Makefile
E2E_KEEP=1 ./testing/e2e/run.sh    # keep the container + workdir for debugging
```

From GitHub Actions, see [`../../.github/workflows/e2e.yml`](../../.github/workflows/e2e.yml).

### Requirements

| Requirement | Notes |
|---|---|
| Docker | A PBS container is started for the duration of the run. |
| `python3` | Used to build fixtures and to assert on JSON. |
| `proxmox-backup-client` | The official client. If it is not on `PATH`, the suite automatically falls back to the `fdrake/proxmox-backup-client` container image. |
| `make`, Go | Used to build `dist/proxmoxbackup-directory` and `dist/proxmoxbackup-machine` (skip with `E2E_SKIP_BUILD=1`). |

### Configuration

Every setting is an environment variable with a sensible default:

| Variable | Default | Purpose |
|---|---|---|
| `PBS_IMAGE` | `ayufan/proxmox-backup-server:latest` | PBS container image. **There is no official Proxmox PBS image** — PBS deliberately ships none — so we use the community image Proxmox staff point at on their forum. |
| `PBS_CONTAINER` | `pbs-e2e` | Container name (removed on exit). |
| `PBS_PORT` | `8007` | Host port, bound to `127.0.0.1` only. |
| `PBS_DATASTORE` | `e2e` | Datastore created inside PBS. |
| `PBS_USER` / `PBS_TOKEN_NAME` | `e2e@pbs` / `e2etoken` | API user and token. A dedicated token is generated per run; no admin password is needed. |
| `PBC_BIN` | `proxmox-backup-client` | Path to the official client. |
| `PBC_IMAGE` | `fdrake/proxmox-backup-client:latest` | Fallback official-client image. |
| `E2E_SKIP_OFFICIAL` | `0` | Set to `1` to skip every official-client assertion (useful for a quick smoke run). |
| `E2E_SKIP_BUILD` | `0` | Set to `1` to reuse the binaries already in `dist/`. |
| `E2E_KEEP` | `0` | Set to `1` to keep the workdir and container. |
| `WORK` | `mktemp -d` | Working directory. |

## What is covered

| # | Test | Assertion |
|---|---|---|
| 1 | Directory backup → official restore | Every file and directory is byte-identical (`sha256` manifest diff + directory-tree diff). |
| 2 | Block/file backup (`.fidx`) → official restore | The restored image is byte-identical to the source. |
| 3 | **Encrypted** directory backup → official restore `--keyfile` | Restores identically; and the unkeyed restore is *refused*. |
| 4 | **Encrypted** block backup → official restore `--keyfile` | Restores identically; and the unkeyed restore is *refused*. |
| 5 | Manifest interop | The official client lists our snapshots, restores our `index.json.blob`, and the manifest has a valid signature, a `key-fingerprint`, `crypt-mode: encrypt` on every archive, and exactly the key set that proxmox-backup declares. |
| 6 | PBS server-side verify | `proxmox-backup-manager verify` reads every chunk and archive back and reports `TASK OK`. |
| 7 | Dedup | A second identical backup reports chunk reuse. |

Tests 3 and 4 use a key file created by the **official** client, which doubles
as an assertion that our key-file parser is compatible.

## Interop notes discovered while building this

These cost real debugging time and are easy to regress, so they are worth
knowing before changing anything in `pbscommon`:

1. **Certificate fingerprint format.** Both our client and the official client
   want the **colon-hex SHA-256** of the server certificate (`20:9d:...`).
   Plain 64-char hex is rejected by the official client. The value comes from
   `proxmox-backup-manager cert info` inside the container; the certificate is
   regenerated for every container, so it must never be hard-coded.

2. **PBS ≥ 3.0 has no namespaces.** The HTTP/2 upgrade handshake schema does
   not accept an `ns` parameter; sending one makes PBS return `404 namespace not
   found`. `pbsapi.go` probes `GET /api2/json/version` and only sends `ns` to
   version-2 servers.

3. **PBS rewrites the manifest.** At `/finish` PBS parses `index.json.blob`
   into its *own* `BackupManifest` struct and re-serialises it, dropping every
   key that struct does not declare. Because the official client verifies the
   signature against that stored copy, **our manifest must contain exactly the
   keys proxmox-backup declares** — no more. Adding e.g. a `comment` field
   silently breaks every encrypted restore with `wrong signature in manifest`.

4. **The manifest blob is never encrypted**, only signed, and it is not part of
   its own signed `files` array. PBS adds it back at listing time as
   `crypt-mode: sign-only`.

5. **API tokens need their own ACL entry.** Granting a datastore role to the
   user is not enough; it must also be granted to the `<user>!<token>` auth-id,
   and the change takes a couple of seconds to propagate.

## Known gaps

* **Symlinks are not restored.** The pxar writer skips them
  (`pbscommon/pxar_reader.go`, `walk()`), so the fixtures deliberately contain
  none.
* **No restore CLI exists in this project.** Restore is GUI-only
  (`gui/restore_inline.go`) or delegated to `proxmoxbackup-nbd` for block
  devices. That is why every restore assertion here goes through the official
  client; `docs/RESTORE_GUIDE.md` lists the planned `proxmoxbackupclient-restore`
  binary as "À DÉVELOPPER".
* **`-novss` is mandatory for the directory backup on Linux**; the default
  consistent-snapshot path requires root.
* **Restore is only asserted through the official client**, because this project
  ships no restore CLI. The suite therefore deliberately does *not* test the GUI
  or `proxmoxbackup-nbd` restore paths.

## Harness gotchas

Things that cost time while building this suite and that are easy to trip over
again:

* **`set -o pipefail` turns a SIGPIPE into a suite abort.** Building the block
  fixture as `tr '\0' 'A' </dev/zero | dd bs=1M count=4` makes `dd` exit after
  four blocks and close the pipe, `tr` die of SIGPIPE, and the whole run stop
  with exit 141. The fixture is therefore generated by a single Python process.
* **Never hand-type the certificate fingerprint.** A typo produces the very
  misleading `certificate fingerprint does not match expected fingerprint`.
  `configure_pbs` derives it from `proxmox-backup-manager cert info`.
* **The official client creates its output file with `O_EXCL`**, so every
  restore target is deleted before the call.
* **`acl update` needs exactly one role, and the token ACL must be set after
  the token exists.** Granting `DatastoreAdmin` to the user alone still fails at
  runtime with `permission check failed - missing Datastore.Audit|Datastore.Backup`.
* **When the official client runs from a container, it runs as the invoking
  user** (`--user "$(id -u):$(id -g)"`). As root it would write a root-owned
  `0600` key file into the mounted work directory that our CLIs, running as the
  unprivileged host user, could not read.
* **`docker rm -v` does not delete named volumes**; `remove_pbs()` deletes them
  explicitly, otherwise the next run fails with `datastore 'e2e' already exists`.

## Relationship to `make test`

`make test` is the offline unit-test loop (`go test -race` over every module)
and `make test-e2e` is this suite. They are kept separate on purpose: the E2E
suite needs Docker and ~10 minutes, and would otherwise dominate every local
`make test`. The CI workflow runs them as two independent jobs.