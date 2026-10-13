# User guide: several PBS servers

## What it is for

You can set up several Proxmox Backup Servers (or several datastores on one server) and choose, for each Backup Set and each one-off backup, which one it goes to.

### Examples

#### Fast and big storage
```
C:\Users (30 GB, important) -> PBS on SSD, daily
D:\Media (500 GB, archive)  -> PBS on large disks, weekly
```

#### Main server and disaster recovery
```
All folders -> PBS Production (main)
             + PBS DR (offsite, a second Backup Set)
```

#### One datastore per kind of data
```
Documents -> datastore "documents"
Videos    -> datastore "media"
Code      -> datastore "dev-backups"
```

Each datastore is a separate server entry, even when the URL is the same.

---

## Step by step

### Step 1: open the server list

Open **Preferences** (side menu, or Tools > Preferences...) and the **Account Information** tab. The **PBS Configuration** section lists the configured servers. On first start, with no server yet, Preferences opens here by itself.

### Step 2: add a server

Click **+ Add server**. The editor has four tabs.

**Server**

| Field | Example | Required | Notes |
|---|---|---|---|
| Server name | `Fast SSD` | Yes | Shown in the app |
| Server ID | `pbs-ssd` | No | Only when adding. Left empty, one is generated |
| PBS server URL | `https://pbs-ssd.local:8007` | Yes | Full URL with the port |
| Datastore | `ssd-fast` | Yes | |
| Namespace | `clients` | No | |
| SSL certificate fingerprint | `AA:BB:CC:...` | No | Left empty, Test offers the server's fingerprint for you to confirm, then pins it |
| Description | `SSD storage` | No | |

**User / Password** or **API Token**: fill in one of the two. A token is an Authentication ID such as `backup@pbs!laptop` and its secret. Give it the DatastoreBackup role on the datastore (DatastoreReader as well to restore snapshots someone else made). Avoid `root@pam`.

**Encryption** (optional): Browse to a PBS key file or Generate a new one. Backups to this server are then encrypted before they leave the computer. **Without the key file an encrypted snapshot cannot be restored: keep a copy away from this computer.** For a key with a passphrase choose **Ask me** (once per app session; scheduled backups fail until it has been entered) or **Remember on this computer** (needed for scheduled backups and the background service).

Save the server. The first server you add becomes the default.

### Step 3: test the connection

Each server in the list has a status: **Not tested**, **Testing...**, **Online** or **Offline**. Click **Test** to check it. If the server uses a self-signed certificate and no fingerprint is set, the app shows the fingerprint it found; compare it with the one on the PBS dashboard (Show Fingerprint) before you confirm.

### Step 4: choose the default server

The default server is marked **⭐ DEFAULT**. Click **☆ Default** on another server to make it the default. The default is preselected for new backups and in Restore.

### Step 5: pick the server for each backup

- **Backup Sets:** edit the set, **Destination** tab, **Destination PBS server**.
- **One-off backups:** **Destination PBS server** on the form.
- **Restore:** **Source PBS server** at the top of the Restore page.
- **Undelete and Roll back** use the server of the Backup Set you pick.

A Backup Set saved without a server uses whatever is the default when it runs.

---

## Managing servers

### Edit a server

Click **Edit**, change the fields, save. The token secret and password are never shown again; leave them empty to keep the stored ones. A changed URL, datastore or credentials applies to the next backup of every Backup Set that uses the server.

### Delete a server

Click **Delete** and confirm. If it was the default, another server becomes the default. Backup Sets that point at the deleted server fail until you choose another server in their Destination tab. The snapshots on the server are not touched.

### Two Backup Sets, two servers

1. Add `PBS Production` (`https://pbs-prod.local:8007`, datastore `backup-prod`) and make it the default.
2. Add `PBS DR` (`https://pbs-dr.remote:8007`, datastore `backup-dr`).
3. Create a Backup Set "Daily" with Destination `PBS Production`, daily at 02:00.
4. Clone it as "DR", set Destination to `PBS DR`, Saturdays at 03:00.

The two sets can keep the same Backup ID, because they are on different servers. Two sets on the **same** server with the same Backup ID share one backup group: the editor warns about this and both cards show a **SHARED ID** badge.

### Moving settings to another computer

Preferences > **Export / Import Settings** exports the servers and Backup Sets to a file, with or without the secrets, and imports them on another computer.

---

## FAQ

### What happened to my old single-server settings?

A configuration from before several servers were supported is converted on first start into one server with the ID `default`, named "Main PBS Server", which becomes the default. Your backups keep working.

### Can two servers use the same datastore name?

Yes. Each entry has its own URL and datastore.

### How do I see which server a backup used?

A Backup Set shows its server in its Destination tab. **Reports** list each run.

### A server shows Offline

1. Check the network: ping, firewall, VPN.
2. Check the URL: `https://`, port 8007.
3. Check the credentials: user and realm, or token ID and secret, and the token's permissions.
4. Log in to the PBS web interface with the same account.

---

## Troubleshooting

### "PBS server 'xxx' not found"

The app's message for a server ID that does not exist (versions up to 0.8.2 gave it in French: "serveur PBS 'xxx' introuvable"). A Backup Set still points at a deleted server: edit the set and choose a server in the Destination tab.

### "no PBS server specified and no default server"

No server is the default and there is more than one. Click **☆ Default** on one of them.

---

See also section 4 of the [GUI manual](docs/manual/GUI.md#4-adding-a-pbs-server). For how this is built, see [MULTI_PBS_GUIDE.md](MULTI_PBS_GUIDE.md).
