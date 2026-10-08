# Bare Metal Restore Guide

**What this is for:** the machine this client was backing up is gone — dead disk, stolen
laptop, failed motherboard swap, whatever — and you have a new (or repaired) disk to put
Windows back on, from a Proxmox Backup Server snapshot.

This is a **whole-disk** restore. It overwrites everything on the target disk with the
exact contents of a backed-up snapshot. It is not for recovering a handful of deleted
files — use the client's own **Restore** tab for that instead, while the machine is still
alive and bootable.

---

## Before you start

- [ ] The replacement disk is physically installed in the machine.
- [ ] You have the **Bare Metal Restore ISO**, written to a USB stick or burned to a disc.
      Get it from Tools → Download Bare Metal Restore ISO… in the client, or directly from
      [this fork's releases page](https://github.com/mjb-is/proxmoxbackupclient_go/releases).
      Recommended: a [Ventoy](https://www.ventoy.net) stick. Prepare the stick with Ventoy
      once, then just copy the ISO file onto it, next to any other rescue ISOs you keep;
      Ventoy's boot menu lists them all. If the restore then asks which disk to restore
      to, pick the machine's own disk, not the stick.
- [ ] The machine can reach your Proxmox Backup Server over the network (wired is more
      reliable than Wi-Fi for this — the live environment's Wi-Fi support is limited).
- [ ] You know the PBS server's URL, a username with access to the datastore (e.g.
      `root@pam` or a token's `user@realm`), its password, and the datastore name (plus
      namespace, if the backup lives in one).
- [ ] You're restoring the **correct** snapshot — if the failure might have been caused by
      something that corrupted recent data (e.g. ransomware), pick a date from before that,
      not just the latest one.

**This process is destructive.** Everything on the target disk is permanently overwritten.
Double-check you're pointed at the *new/replacement* disk, not something else in the
machine you meant to keep (a second data drive, a NAS mounted over the network, etc.).

---

## Step by step

### 1. Boot from the USB/disc

Plug in the restore media and power on, using your machine's boot-menu key (often F12,
F11, Esc or Del — check your hardware's manual if unsure) to boot from it instead of the
internal disk.

The very first menu you see already defaults to **"Proxmox Backup Client Go - PBS Bare
Metal Restore"** — just let it boot on its own, or press Enter on it. This skips straight
past the language/keyboard prompt and Clonezilla's own menu entirely; there is nothing
else to pick here.

*(There's also a manual "pbs-nbd" option further into Clonezilla's own menu, for attaching
a snapshot without an automatic whole-disk overwrite — useful if you want to mount it and
pull out individual files/folders instead. That path is not covered by this guide.)*

### 2. Network comes up on its own

DHCP is attempted automatically on every network adapter. If your network doesn't use
DHCP, or nothing comes up within a few seconds, Clonezilla's own network setup screen
appears so you can configure it manually. The system clock is synced automatically too —
no prompt either way.

### 3. Enter your PBS details

You'll be asked for four things, one screen at a time:

1. **PBS server URL** — the full address, e.g. `https://pbs.example.com:8007`
   (`https://` is already filled in for you).
2. **Username** — must include the realm, e.g. `root@pam` or `backup-user@pbs`.
3. **Password** — for that user/token.
4. **Datastore** — the datastore name, e.g. `backup`. If the backup is inside a namespace,
   append it after a slash: `backup/clients` means datastore `backup`, namespace `clients`.

Each screen validates what you typed and lets you try again if something looks wrong
(missing `https://`, missing `@realm`, etc.). These are remembered for the rest of this
boot session if you need to restart the flow.

### 4. Pick the snapshot to restore

You'll see up to three menus, each skipped automatically if there's only one option:

1. **Which machine** — the backup ID (e.g. your hostname), shown with its PBS comment if
   it has one, so you can tell jobs apart if several look similar.
2. **Which date** — every backup run for that machine, newest first.
3. **Which disk image** — if that backup covered more than one disk, pick the right one.

### 5. Confirm the restore target

The wizard auto-detects the disk to restore *onto* — it looks for the one local,
non-removable disk that isn't the snapshot you just attached. If there's exactly one
candidate, it's chosen automatically. If there's more than one disk in the machine, you're
asked to pick, since guessing wrong here is unrecoverable.

Either way, you'll see **one confirmation screen** showing the source snapshot and the
target disk, with their sizes, before anything is written:

> Source (PBS snapshot): ... Target (THIS MACHINE): ...
> EVERYTHING on [target] will be PERMANENTLY OVERWRITTEN. Continue?

Read it carefully. This is the last point where nothing has been touched yet.

### 6. The restore runs

Once confirmed, the disk image streams from PBS straight onto the target disk. There's no
further input needed — leave it running. Depending on the backup's size and your network
speed, this can take anywhere from a few minutes to well over an hour.

### 7. Finish and reboot

- **On success:** a message confirms the restore is complete and the machine reboots on
  its own. Remove the USB stick/disc when prompted, before it boots back into it again.
- **On failure:** you'll see the exit code and where the log file is, and the machine will
  *not* reboot automatically — check the log before trying again.

---

## After the restore

- The restored disk boots with everything as it was at snapshot time — files, permissions
  and all, not just a generic Windows install.
- If Windows doesn't boot cleanly (rare, but more likely if the new disk's controller or
  boot mode — UEFI vs. legacy BIOS — differs from the original machine), boot from a
  Windows installation USB and use its "Repair your computer" → Startup Repair option.
- Reconnect the machine to your network and confirm this backup client picks up its
  scheduled Backup Sets again as normal.

---

## Troubleshooting

**"No backups found in datastore"** — double-check the datastore name and namespace
(everything after the first `/`), and that the username/password actually has access to
that datastore on the PBS server.

**It can't find a target disk** — the new disk isn't showing up as a plain local disk (it
may be missing, not yet initialized, or connected over an interface the live environment
doesn't have a driver for). Check the physical connection first.

**Multiple disks, not sure which is which** — the picker shows each candidate's size from
`parted`. If that's not enough to tell them apart, reboot into your normal OS's BIOS/UEFI
setup and check disk order/serials there first, then come back.

**It finished but Windows won't boot** — see "After the restore" above for a Startup
Repair; this is a Windows boot-configuration issue, not a sign the restore itself failed.

**Restoring after ransomware or corruption** — pick a snapshot from *before* the infection,
not the most recent one. If you're not sure how far back is safe, check this client's own
Reports/backup history from another machine first.

---

*This guide describes this fork's fully-automated PBS Bare Metal Restore flow. It does not
cover restoring onto genuinely different hardware (a physical-to-virtual migration, or a
replacement machine with a different chipset/storage controller) — that scenario needs a
fresh Windows install plus a normal file-level restore of your data instead, since Windows
itself doesn't tolerate a whole-disk image moving to substantially different hardware.*
