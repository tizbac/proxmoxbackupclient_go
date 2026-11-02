# Multi-PBS: implementation guide

## Overview

The GUI and the background service can hold **several PBS servers**. Each Backup Set and each one-off backup names the server it goes to, and Restore picks a source server. Typical uses:
- `D:\` to a large, slow PBS, weekly
- `C:\Users` to a small, fast PBS, daily
- the same folders to a production PBS and to a disaster recovery PBS (two Backup Sets)

User-facing steps are in [MULTI_PBS_USER_GUIDE.md](MULTI_PBS_USER_GUIDE.md) and section 4 of the [GUI manual](docs/manual/GUI.md#4-adding-a-pbs-server).

## Data model

### Config (`gui/config.go`)

```go
type Config struct {
    // Servers, keyed by server ID
    PBSServers map[string]*PBSServer `json:"pbs_servers,omitempty"`
    // ID of the default server
    DefaultPBSID string `json:"default_pbs_id,omitempty"`

    // Legacy single-server fields, kept so an old config.json loads.
    // Migrated into PBSServers["default"] on load (see below).
    BaseURL         string `json:"baseurl,omitempty"`
    CertFingerprint string `json:"certfingerprint,omitempty"`
    AuthID          string `json:"authid,omitempty"`
    Secret          string `json:"secret,omitempty"`
    Datastore       string `json:"datastore,omitempty"`
    Namespace       string `json:"namespace,omitempty"`
    // ... encryption key, backup, restore, e-mail and theme settings
}
```

### PBSServer (`gui/pbs_server.go`)

```go
type PBSServer struct {
    ID              string `json:"id"`   // "default", "pbs-ssd", ...
    Name            string `json:"name"` // "Fast SSD"
    BaseURL         string `json:"baseurl"`
    CertFingerprint string `json:"certfingerprint"`
    AuthID          string `json:"authid"` // API token: AuthID + Secret
    Secret          string `json:"secret"`
    Username        string `json:"username,omitempty"` // or user + password
    Password        string `json:"password,omitempty"`

    // Client-side encryption, per server
    EncryptionKeyFile            string `json:"encryption_key_file,omitempty"`
    EncryptionKeyPassphrase      string `json:"encryption_key_passphrase,omitempty"` // only with "Remember"
    EncryptionKeyPassphraseSet   bool   `json:"encryption_key_passphrase_set,omitempty"`
    ClearEncryptionKeyPassphrase bool   `json:"clear_encryption_key_passphrase,omitempty"`

    Datastore   string `json:"datastore"`
    Namespace   string `json:"namespace"`
    Description string `json:"description,omitempty"`
    IsOnline    bool   `json:"is_online,omitempty"`
    SecretSet   bool   `json:"secret_set,omitempty"`   // set on copies sent to the frontend
    PasswordSet bool   `json:"password_set,omitempty"`
}
```

A server uses **either** an API token (AuthID and Secret) **or** a username and password; adding or updating a server with a token clears the username and password. With a username, a fresh PBS ticket is obtained for every operation; tickets are never stored.

`sanitized()` strips the secret, the password and the stored passphrase before a server is handed to the frontend, and sets `SecretSet`, `PasswordSet` and `EncryptionKeyPassphraseSet` instead. On update, an empty secret or password means "keep the stored one".

The encryption key is per server. A passphrase-protected key is either asked for once per app session ("Ask") or stored in the config ("Remember", needed for scheduled backups and the service). See "Client-side encryption" in the [README](README.md#-client-side-encryption).

### Backup Sets (`gui/scheduler.go`)

```go
type ScheduledJob struct {
    // ...
    BackupID string `json:"backupId"`
    // Which configured server this set backs up to; "" means the default
    // server at the time the set runs.
    PBSServerID string `json:"pbsServerId,omitempty"`
    // ...
}
```

One-off backups and restores pass a server ID in the same way.

## Resolving a server

`App.resolvePBS(pbsID)` in `gui/pbs_server.go` is used by backups and restores, in the GUI build and the service build alike:

1. A non-empty `pbsID` is looked up in `PBSServers`; an unknown ID is an error (`PBS server '<id>' not found`).
2. An empty `pbsID` uses `EffectivePBS()`: the legacy top-level fields if `BaseURL` is set, otherwise the default server (`GetPBSServer("")`). With no default and exactly one server, that server is used; with no default and several servers, the error is `no PBS server specified and no default server`.
3. The result is a `*Config` with authentication filled in (`withAuth`). Restores then unlock the server's key (`resolveRestorePBS`).

## Migration from a single-server config

`LoadConfig()` converts an old config on first load. When `baseurl` is set and `pbs_servers` is empty:

```json
{
  "baseurl": "https://pbs.example.com:8007",
  "authid": "backup@pbs!client",
  "secret": "xxx",
  "datastore": "backup",
  "namespace": "clients"
}
```

becomes, and is saved at once:

```json
{
  "pbs_servers": {
    "default": {
      "id": "default",
      "name": "Main PBS Server",
      "baseurl": "https://pbs.example.com:8007",
      "authid": "backup@pbs!client",
      "secret": "xxx",
      "datastore": "backup",
      "namespace": "clients",
      "description": "Default PBS server (migrated from old config)"
    }
  },
  "default_pbs_id": "default",
  "baseurl": "https://pbs.example.com:8007"
}
```

The legacy fields stay in the file. A config that already has `pbs_servers` is loaded as it is.

## Server rules (`gui/config.go`)

- `AddPBSServer` validates the server (`PBSServer.Validate`: ID, name, URL, a token or a username, datastore, fingerprint format, a usable key file), refuses a duplicate ID, and makes the first server the default.
- `UpdatePBSServer` validates and replaces an existing server.
- `DeletePBSServer` removes a server; if it was the default, another remaining server becomes the default (or none, if it was the last). Backup Sets that name it fail until they are given another server.
- `SetDefaultPBS` sets `DefaultPBSID`.

## Methods exposed to the frontend (`gui/main.go`)

| Method | What it does | Returns |
|---|---|---|
| `ListPBSServers()` | All servers, sanitized | `[]*PBSServer` |
| `GetPBSServer(id)` | One server, sanitized; `""` gives the default | `*PBSServer, error` |
| `AddPBSServer(pbs)` | Adds a server | `error` |
| `UpdatePBSServer(pbs)` | Updates a server, keeping stored secrets that come back empty | `error` |
| `DeletePBSServer(id)` | Deletes a server | `error` |
| `SetDefaultPBSServer(id)` | Sets the default | `error` |
| `GetDefaultPBSID()` | The default server's ID | `string` |
| `TestPBSConnection(id)` | Connects and authenticates | `error` |
| `GetServerFingerprint(url)` | The certificate fingerprint the server presents, for trust on first use | `string, error` |
| `PinPBSServerFingerprint(id, fp)` | Stores a confirmed fingerprint on a server | `error` |

With the background service running, the GUI may not be allowed to write `config.json`; fingerprint pinning then goes through the service's local API (`/pbs/fingerprint`).

## Frontend

The server list and editor are in Preferences > Account Information (`gui/frontend/src/App.jsx`). The Backup Set editor's Destination tab and the one-off form have a **Destination PBS server** picker (`backupPBSID`, saved as `pbsServerId`); Restore has a **Source PBS server** picker. The shared Backup ID warning compares server, backup type and Backup ID, so two sets with the same ID on different servers do not warn.

## Tests

`gui/crypt_test.go` covers the per-server encryption key (`TestPBSServerToConfigCarriesTheEncryptionKey`, `TestPBSServerValidateRejectsUnusableKey`, `TestPBSServerSanitizedKeepsTheKeyPath`). `gui/config_test.go` covers saving and loading a config. There are no unit tests yet for the migration from a single-server config or for the add, delete and default rules.
