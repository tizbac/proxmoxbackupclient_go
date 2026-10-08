# Bare-metal restore: user manual

This manual covers restoring a whole machine from a machine (full disk) backup. It applies to v0.6.0. For file and folder restores use the GUI's Restore page ([GUI.md](GUI.md#9-restore)).

## Contents

1. [Which restore do you need?](#1-which-restore-do-you-need)
2. [Before disaster strikes](#2-before-disaster-strikes)
3. [Get the ISO and make the boot media](#3-get-the-iso-and-make-the-boot-media)
4. [Automated restore, step by step](#4-automated-restore-step-by-step)
5. [The manual pbs-nbd entry](#5-the-manual-pbs-nbd-entry)
6. [Restoring a vm snapshot onto Proxmox VE](#6-restoring-a-vm-snapshot-onto-proxmox-ve)
7. [Restoring onto different hardware (P2V and replacement machines)](#7-restoring-onto-different-hardware-p2v-and-replacement-machines)
8. [Troubleshooting](#8-troubleshooting)

## 1. Which restore do you need?

| Situation | Use |
|---|---|
| Some files or folders deleted or damaged, machine still works | GUI Restore page ([GUI.md](GUI.md#9-restore)) |
| Disk dead or system unbootable, same or similar machine | Automated bare-metal restore ([section 4](#4-automated-restore-step-by-step)) |
| Machine gone, move it into a Proxmox VE VM | `vm` snapshot restored from Proxmox VE ([section 6](#6-restoring-a-vm-snapshot-onto-proxmox-ve)) |
| Copy a few files out of a disk image | Manual `pbs-nbd` entry, or `proxmoxbackup-nbd` on Linux ([section 5](#5-the-manual-pbs-nbd-entry)) |
| Ransomware or corruption | Any of the above, from a snapshot taken **before** the problem |

A bare-metal restore needs a **machine** backup: a Backup Set or one-off backup with Backup type **Machine (full disk)**, or `proxmoxbackup-machine`. Folder backups cannot be restored this way.

## 2. Before disaster strikes

* Make a machine backup of the system disk on a schedule, and keep folder backups of your data as well.
* Write down the PBS URL, a PBS **user name and password** that can read the datastore, the datastore name and any namespace. The restore ISO logs in with a user and password; it does not take API tokens. Make a dedicated PBS user (for example `restore@pbs`) with the **DatastoreReader** role on the datastore if you do not want to use an administrator account.
* If your backups are encrypted, keep a copy of the key file (and its passphrase) on a USB stick, away from the machine.
* Make the boot media now and test it once on a spare machine or VM.
* Note your machine's boot menu key (often F12, F11, F8, Esc or Del).

## 3. Get the ISO and make the boot media

The ISO is a Clonezilla Live image with this project's restore tools added.

1. Download `ProxmoxBackupClient-GO-BMR-Restore-<version>.iso` from the [releases page](https://github.com/mjb-is/proxmoxbackupclient_go/releases). In the GUI, **Tools > Download Bare Metal Restore ISO…** opens that page. A release that did not change the ISO links to the one from an earlier release.
2. Check it against that release's `SHA256SUMS.txt`.
3. Write it to a USB stick (everything on the stick is erased):
   * Linux: `sudo dd if=ProxmoxBackupClient-GO-BMR-Restore-<version>.iso of=/dev/sdX bs=4M conv=fsync status=progress`
   * Windows: a tool that writes the image as is, for example Rufus in "DD Image" mode, or balenaEtcher.

   Or burn it to a DVD, or attach it to a VM as a CD.

### A multi-purpose rescue stick with Ventoy (recommended)

Writing the ISO as above gives the whole stick to one image. [Ventoy](https://www.ventoy.net) is a better way to keep a recovery stick: it prepares the stick once, and from then on you simply **copy ISO files onto it** like ordinary files. At boot Ventoy shows a menu of every ISO on the stick, so one stick can carry this restore ISO next to a Windows installer, a Linux live system, memtest and so on. Updating to a newer restore ISO is a matter of copying the new file over.

1. Download Ventoy from [ventoy.net](https://www.ventoy.net) and install it to the stick: **Ventoy2Disk.exe** on Windows, `Ventoy2Disk.sh` or VentoyGUI on Linux. This erases the stick once.
2. Copy `ProxmoxBackupClient-GO-BMR-Restore-<version>.iso` onto the stick's large data partition (it shows up as a normal drive). Do not unpack it.
3. Boot from the stick and pick the ISO in Ventoy's menu, then carry on as in [section 4](#4-automated-restore-step-by-step).

Ventoy starts on legacy BIOS and UEFI machines. With Secure Boot on, its first start asks you to enrol Ventoy's key once (follow its on-screen steps), or turn Secure Boot off for the restore.

**Check the target disk.** Booted from a Ventoy stick (or a USB SSD that reports itself as a fixed disk), the restore can list the stick next to the machine's own disk and ask which to restore to. Pick the machine's disk; the confirmation screen shows each disk's size before anything is written. Test the stick once on a spare machine or VM, as with any boot media.

The ISO boots on both legacy BIOS and UEFI machines. It can also be network-booted like any Clonezilla Live image; setting that up is not covered here.

You can build your own ISO from a stock Clonezilla Live ISO with `patch-clonezilla.sh`. See [PATCH-CLONEZILLA.md](../../PATCH-CLONEZILLA.md).

## 4. Automated restore, step by step

This **overwrites the whole target disk**. Disconnect any disk you want to keep if you can.

### 4.1 Boot

Plug in the boot media, power on and pick it from the boot menu. The first menu entry, **"Proxmox Backup Client Go - PBS Bare Metal Restore"**, is the default: press Enter or let it start on its own. It skips Clonezilla's language and keyboard questions (it uses English with a UK keyboard layout).

### 4.2 Network

DHCP is tried on every network adapter. If no address comes up, Clonezilla's own network screen appears so you can set a static address. The clock is set from the internet automatically. A wired connection is more reliable than Wi-Fi in the live system.

### 4.3 PBS details

Four screens, each checked before moving on:

| Screen | Example | Rule |
|---|---|---|
| PBS server URL | `https://pbs.example.com:8007` | Must start with `https://` (already filled in) |
| PBS username | `restore@pbs` | Must include the realm (`@pam`, `@pbs`) |
| Password | | Must not be empty |
| PBS datastore | `backup` or `backup/clients` | Text after the first `/` is the namespace |

These are remembered for the rest of the boot session (in `/tmp/pbsnbd-credentials`, readable by root only), so a second attempt starts with them filled in.

### 4.4 Encryption key

"Were these backups made with an encryption key?"

| Answer | What happens |
|---|---|
| No, the backups are not encrypted | Carry on |
| Yes, the key file is on a USB stick or other disk | Pick the partition. It is mounted read-only, the `*.json` files on it are listed (or the only one is used), the key is copied to `/tmp`, and the stick is unmounted. You can unplug it |
| Yes, I will type the path of the key file | Type the full path |

For a passphrase-protected key a hidden box asks for the passphrase. A wrong passphrase is asked for again, up to 3 attempts. The key is never written to the ISO or the target disk.

### 4.5 Pick the snapshot

Up to three menus; a menu with only one choice is skipped:

1. **Which machine**: `host/<id>` or `vm/<id>`, with the PBS comment (the Backup Set name) if there is one.
2. **Which date**: the backups of that machine, newest first.
3. **Which disk image**: for a backup of more than one disk, for example `drive-sata0.img.fidx`.

The chosen image is attached as `/dev/nbd0`. If it cannot be attached (wrong key, server unreachable), a message shows the last lines of the attach log and the flow stops.

### 4.6 Pick and confirm the target disk

The target is the one local disk that is not removable, not optical and not the attached image. With exactly one such disk it is chosen for you. With several, a menu asks, showing each size. The target should be at least as large as the original disk.

A confirmation screen shows the source and target with their sizes:

> Source (PBS snapshot) : /dev/nbd0 (...) Target (THIS MACHINE) : /dev/sda (...) EVERYTHING on /dev/sda will be PERMANENTLY OVERWRITTEN. Continue?

Clonezilla then asks twice more before it writes. Answer `y` to each.

### 4.7 The restore

Clonezilla copies the partition table, every partition and the data between the partition table and the first partition. That last part matters on legacy BIOS disks, where GRUB keeps part of itself there; without it the disk would stop at a bare `GRUB` prompt. UEFI (GPT) disks restore their EFI system partition like any other partition.

Restores of 32 to 40 GiB Windows and Linux test machines took 8 to 11 minutes from power-on to the login prompt on a local network. Time depends on disk size, network and PBS speed.

### 4.8 Finish

* **Success:** "Restore is complete. The system will now reboot..." Remove the boot media when prompted.
* **Failure:** the exit code and the log folder are shown, and the machine does not reboot. Look in the log folder, then run the restore again.

### After the restore

* The restored system is exactly as it was at the snapshot: files, settings and permissions.
* If Windows does not start, boot a Windows installation USB and use **Repair your computer > Troubleshoot > Startup Repair**. This is more likely if the disk controller or boot mode (UEFI or BIOS) differs from the original machine.
* Open the GUI and check that the Backup Sets run again.

## 5. The manual pbs-nbd entry

Clonezilla's own main menu (reached by choosing the normal Clonezilla entries at boot) has an extra mode, **pbs-nbd**. It asks for the same PBS details, key and snapshot as section 4, attaches the image to `/dev/nbd0` in the background, and returns you to Clonezilla. Then either:

* use Clonezilla's own device-to-device wizard with `nbd0` as the source, for a restore where you choose every option yourself, or
* open a shell, `mount -o ro /dev/nbd0p1 /mnt` (after `partprobe /dev/nbd0`), and copy files out.

Running pbs-nbd again stops the previous background attach first.

On any Linux machine the same is done with `proxmoxbackup-nbd` ([CLI.md](CLI.md#6-proxmoxbackup-nbd-attach-a-disk-image-linux)).

## 6. Restoring a vm snapshot onto Proxmox VE

A machine backup saved with **Backup as: Proxmox VE virtual machine** (or `proxmoxbackup-machine -type vm`) is stored as `vm/<number>`. Proxmox VE sees it as an ordinary VM backup.

1. In Proxmox VE, add the PBS datastore as storage (**Datacenter > Storage > Add > Proxmox Backup Server**), with the same namespace.
2. Open that storage, **Backups**, select the `vm/<number>` snapshot, **Restore**.
3. Choose a free VM ID and a target **Storage**, and restore.

The VM is minimal: one SATA disk, OVMF (UEFI) firmware for GPT disks, no network card, no TPM, no Secure Boot keys. Add a network card and anything else the guest needs before starting it. A Windows Server UEFI disk restored this way booted without further changes.

The same `vm` snapshot can also be restored onto bare metal with the ISO; it appears in the machine menu as `vm/<number>`.

## 7. Restoring onto different hardware (P2V and replacement machines)

A disk image carries the drivers of the machine it came from.

* **Linux** usually starts on different hardware.
* **Windows** may stop with `INACCESSIBLE_BOOT_DEVICE` if the disk controller is very different. The SATA disk that a `vm` restore creates avoids the most common case, because Windows has built-in SATA drivers. Install VirtIO drivers inside the VM afterwards if you want to switch to faster virtual hardware.
* When a whole-disk restore will not start, the reliable route is a fresh Windows install followed by a folder restore of your data with the GUI.

## 8. Troubleshooting

| Message or symptom | What to check |
|---|---|
| "Unable to 'modprobe nbd max_part=0'" | The ISO's kernel lacks the module. Rebuild from a current Clonezilla ISO |
| "Could not list the backups" | URL, port, user and realm, password, datastore name. The PBS user needs the DatastoreReader role (or higher) on the datastore |
| "No backups found in datastore" | Datastore and namespace (text after the first `/`), and the user's rights on it |
| "That passphrase did not unlock the key" | Retype the passphrase; 3 attempts in total |
| "No disk partition with a filesystem was found" | Plug in the key USB stick, then start the flow again |
| "/dev/nbd0 could not be attached" | Wrong key file for an encrypted backup, or the server stopped answering. The message shows the attach log |
| "No local target disk found" | The new disk is missing, unsupported by the live system, or seen as removable. Check cabling and BIOS settings |
| Several disks and unsure which is the target | Compare sizes in the menu; if still unsure, check disk models and serials in the firmware setup first, or disconnect the others |
| Clonezilla reports the target is too small | Use a disk at least as large as the original |
| Stops at a `GRUB` prompt after restore | Should not happen with this ISO (it restores the area GRUB uses). Make sure you used the "PBS Bare Metal Restore" entry, not a hand-run restore without Clonezilla's `-j2` option |
| Windows will not start after restore | Startup Repair from Windows installation media (see "After the restore") |
