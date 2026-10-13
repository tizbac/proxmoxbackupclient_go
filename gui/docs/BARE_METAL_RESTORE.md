# Bare Metal Restore Guide

**What this is for:** the machine this client was backing up is gone (dead disk, stolen
laptop, failed motherboard swap, whatever) and you have a new or repaired disk to put the
whole machine back on, from a machine (full disk) backup on your Proxmox Backup Server.

The restore copies the backed-up disk image block for block, so it works the same for any
operating system on the disk: Windows, Linux or anything else.

This is a **whole-disk** restore. It overwrites everything on the target disk with the
exact contents of a backed-up snapshot. It is not for recovering a handful of deleted
files: use the client's own **Restore** page for that instead, while the machine is still
alive and bootable.

---

## Before you start

- [ ] The replacement disk is physically installed in the machine.
- [ ] You have the **Bare Metal Restore ISO**, written to a USB stick or burned to a disc.
      Get it from Tools → Download Bare Metal Restore ISO… in the client, or directly from
      [this fork's releases page](https://github.com/mjb-is/proxmoxbackupclient_go/releases).
      Recommended: a [Ventoy](https://www.ventoy.net) stick. Prepare the stick with Ventoy
      once, then just copy the ISO file onto it, next to any other rescue ISOs you keep;
      Ventoy's boot menu lists them all.
- [ ] The machine can reach your Proxmox Backup Server over the network. Wired is more
      reliable than Wi-Fi for this: the live system's Wi-Fi support is limited.
- [ ] You know the PBS server's URL, a PBS **username and password** with access to the
      datastore (for example `root@pam`, or a user such as `restore@pbs` with the
      DatastoreReader role), and the datastore name (plus namespace, if the backup lives
      in one). The ISO logs in with a username and password only: **API tokens do not
      work here**, even though the client itself may use one.
- [ ] If the backups are **encrypted**, you have the key file (for example on a USB
      stick) and its passphrase, if it has one. Without the key an encrypted backup
      cannot be restored.
- [ ] You're restoring the **correct** snapshot. If the failure might have been caused by
      something that corrupted recent data (ransomware, for example), pick a date from
      before that, not just the latest one.

**This process is destructive.** Everything on the target disk is permanently overwritten.
Double-check you're pointed at the *new/replacement* disk, not something else in the
machine you meant to keep (a second data drive, for example), and not the USB stick or
USB SSD you booted from.

---

## Step by step

### 1. Boot from the USB/disc

Plug in the restore media and power on, using your machine's boot-menu key (often F12,
F11, Esc or Del; check your hardware's manual if unsure) to boot from it instead of the
internal disk.

The very first menu you see already defaults to **"Proxmox Backup Client Go - PBS Bare
Metal Restore"**: just let it boot on its own, or press Enter on it. This skips straight
past the language/keyboard prompt (it uses English with a UK keyboard) and Clonezilla's
own menu entirely; there is nothing else to pick here.

*(There's also a manual "pbs-nbd" option further into Clonezilla's own menu, for attaching
a snapshot without an automatic whole-disk overwrite. It is useful if you want to mount it
and pull out individual files/folders instead. That path is not covered by this guide.)*

### 2. Network comes up on its own

DHCP is attempted automatically on every network adapter. If your network doesn't use
DHCP, or no address comes up, Clonezilla's own network setup screen appears so you can
configure it manually. The system clock is synced automatically too, with no prompt
either way.

### 3. Enter your PBS details

You'll be asked for four things, one screen at a time:

1. **PBS server URL**: the full address, for example `https://pbs.example.com:8007`
   (`https://` is already filled in for you).
2. **Username**: must include the realm, for example `root@pam` or `restore@pbs`
   (`root@pam` is filled in). Not an API token.
3. **Password**: the password of that user.
4. **Datastore**: the datastore name, for example `backup`. If the backup is inside a
   namespace, append it after a slash: `backup/clients` means datastore `backup`,
   namespace `clients`.

Each screen checks what you typed and lets you try again if something looks wrong
(missing `https://`, missing `@realm`, an empty password). These are remembered for the
rest of this boot session if you need to restart the flow.

### 4. Encryption key

Next you're asked **"Were these backups made with an encryption key?"**:

- **No, the backups are not encrypted**: carry on.
- **Yes, the key file is on a USB stick or other disk**: pick the partition that holds
  it. It is mounted read-only, the `.json` files on it are listed (if there is only one,
  it is used), the key is copied into memory and the stick is unmounted again, so you can
  unplug it.
- **Yes, I will type the path of the key file**: type the full path.

If the key is protected by a passphrase, a hidden box asks for it. A wrong passphrase is
asked for again, three attempts in total. The key is never written to the target disk.

### 5. Pick the snapshot to restore

You'll see up to three menus, each skipped automatically if there's only one option:

1. **Which machine**: `host/<backup ID>` (for example your computer's name) or
   `vm/<number>`, shown with its PBS comment if it has one, so you can tell jobs apart if
   several look similar.
2. **Which date**: every backup run for that machine.
3. **Which disk image**: if that backup covered more than one disk, pick the right one.

If the image cannot be attached (wrong key file, server not answering), a message shows
the last lines of the attach log and the flow stops.

### 6. Confirm the restore target

The wizard looks for the disk to restore *onto*: a local disk that is not marked as
removable and is not the snapshot you just attached. If there's exactly one candidate,
it's chosen automatically. If there's more than one, you're asked to pick, with each
disk's size shown, since guessing wrong here is unrecoverable.

**Watch out when you boot from a Ventoy stick or a USB SSD:** some of these report
themselves as fixed disks, so the boot drive itself can be offered as a target, or even
chosen automatically if the machine's own disk is not seen. Check the device and size
before you continue.

Either way, you'll see **one confirmation screen** showing the source snapshot and the
target disk, with their sizes, before anything is written:

> Source (PBS snapshot): ... Target (THIS MACHINE): ...
> EVERYTHING on [target] will be PERMANENTLY OVERWRITTEN. Continue?

Read it carefully. This is the last point where nothing has been touched yet. Clonezilla
then asks twice more before it writes: answer `y` to each.

### 7. The restore runs

Once confirmed, the disk image streams from PBS straight onto the target disk: the
partition table, every partition, and the boot code between the partition table and the
first partition. There's no further input needed: leave it running. Depending on the
backup's size and your network speed, this can take anywhere from a few minutes to well
over an hour.

### 8. Finish and reboot

- **On success:** a message confirms the restore is complete and the machine reboots on
  its own. Remove the USB stick/disc when prompted, before it boots back into it again.
- **On failure:** you'll see the exit code and where the log files are, and the machine
  will *not* reboot automatically. Check the log before trying again.

---

## After the restore

- The restored disk boots with everything as it was at snapshot time: files, settings and
  permissions, whatever the operating system.
- If it doesn't boot cleanly (rare, but more likely if the new disk's controller or the
  boot mode, UEFI or legacy BIOS, differs from the original machine), repair the boot
  loader with the operating system's own tools:
  - **Windows:** boot from a Windows installation USB and use its "Repair your computer"
    → Startup Repair option.
  - **Linux:** boot a live Linux system and reinstall the boot loader (usually GRUB) the
    way your distribution describes.
- Reconnect the machine to your network and confirm this backup client picks up its
  scheduled Backup Sets again as normal.

---

## Troubleshooting

**"Could not list the backups"**: check the URL and port, the username with its realm,
the password and the datastore name. The user needs at least the DatastoreReader role on
the datastore. An API token will not work here.

**"No backups found in datastore"**: double-check the datastore name and namespace
(everything after the first `/`), and that the user actually has access to that
datastore on the PBS server.

**"That passphrase did not unlock the key"**: retype it; you have three attempts.

**"/dev/nbd0 could not be attached"**: usually the wrong key file for an encrypted
backup, or the server stopped answering. The message shows the attach log.

**It can't find a target disk**: the new disk isn't showing up as a plain local disk (it
may be missing, not yet initialized, or connected over an interface the live system
doesn't have a driver for). Check the physical connection first.

**Multiple disks, not sure which is which**: the picker shows each candidate's size. If
that's not enough to tell them apart, check disk models and serials in the machine's
BIOS/UEFI setup first, or disconnect the disks you want to keep, then come back.

**It finished but the system won't boot**: see "After the restore" above for repairing
the boot loader; this is a boot-configuration issue, not a sign the restore itself
failed.

**Restoring after ransomware or corruption**: pick a snapshot from *before* the
infection, not the most recent one. If you're not sure how far back is safe, check this
client's own Reports/backup history from another machine first.

---

*This guide describes this fork's fully automated PBS Bare Metal Restore flow. Restoring
onto genuinely different hardware (a replacement machine with a different chipset or
storage controller) may need more: Linux usually starts, but Windows may stop with
`INACCESSIBLE_BOOT_DEVICE`. If a restored Windows disk will not start on the new
hardware, the reliable route is a fresh Windows install plus a normal file-level restore
of your data.*
