# Patching a Clonezilla Live ISO

This documents how [`patch-clonezilla.sh`](./patch-clonezilla.sh) injects extra
binaries into a stock Clonezilla Live ISO **without breaking boot**, so the
resulting ISO boots from CD, USB (`dd`) and UEFI exactly like the original —
just with our ticket-enabled `pbsnbd` / `machinebackup` binaries shipped inside
the live rootfs.

## Why a full rebuild (and not an in-place patch)

The naive approach is to swap only the squashfs inside the existing ISO:

```
xorriso -indev in.iso -outdev out.iso \
  -update new.squashfs /live/filesystem.squashfs -boot_image any keep -commit
```

This **does not work**. `isolinux` keeps a *Boot Information Table* (BIT) inside
its `isolinux.bin` that records the LBA of the El Torito catalog and a checksum.
`-boot_image any keep` copies the *original* `isolinux.bin` with its *original*
BIT. As soon as the squashfs grows and the filesystem shifts, that BIT points at
the wrong location, and the kernel loader aborts with:

```
isolinux: Image checksum error, sorry...
```

The fix is to **rebuild the whole ISO from scratch** in `xorriso -as mkisofs`
mode. During a from-scratch build xorriso re-emits a fresh BIT
(`-boot-info-table`) and a valid isohybrid MBR, so the checksums are correct by
construction.

## Pipeline

`patch-clonezilla.sh` does the following (all in a `mktemp -d` workdir that is
removed on exit):

1. **Extract** the whole ISO filesystem to a directory tree (`7z`).
2. **Detect** the squashfs compression + block size (`unsquashfs -s`) so the
   rebuilt image is byte-compatible.
3. **Expand** the rootfs (`unsquashfs`), `install` each input file into
   `usr/local/sbin/`, and **rebuild** the squashfs with matching parameters
   (`mksquashfs -all-root -b <block> -comp xz -Xbcj x86`). Swap it back into the
   tree at `/live/filesystem.squashfs`.
4. **Rebuild the ISO** from the tree with `xorriso -as mkisofs`, preserving:
   - the volume id (auto-read from the source ISO),
   - the isohybrid MBR (embedded from `isohdpfx.bin`),
   - the BIOS El Torito boot image (`isolinux.bin`) **with `-boot-info-table`**,
   - the UEFI El Torito boot image (`efi.img`).
5. **Verify** the result and refuse to leave a half-baked ISO behind:
   - the squashfs extracted out of the new ISO must match the rebuilt one (md5),
   - `-report_el_torito` must list `boot-info-table` for the BIOS image,
   - the MBR must end in the `0x55AA` signature.

## Prerequisites

- `xorriso` (>= 1.5), `unsquashfs`/`mksquashfs` (squashfs-tools), `7z`
  (7-zip), `isoinfo` (genisoimage) — for reading the volume id.
- `patch` — to apply the unified-diff menu patches.
- An isohybrid MBR prefix, normally
  `/usr/lib/ISOLINUX/isohdpfx.bin` (from the `syslinux`/`isolinux` packages).

## Usage

```
./patch-clonezilla.sh -o OUTPUT_ISO INPUT_ISO [FILE]...
```

Example — add the two ticket-enabled binaries plus the PBS-NBD helper (menu
entry for it is applied as a unified diff from `clonezilla-patch/patches/`):

```
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd
```

To point the diff-based patching at another patch directory, pass `-p PATCH_DIR`
(or set the `PATCH_DIR` env var).

Every file listed after the input ISO is installed into `usr/local/sbin/` inside
the live rootfs. All Clonezilla-specific paths/sizes are env-overridable
(see `ISO_SQFS`, `INSTALL_DIR`, `ISO_BIOS_IMG`, `ISO_UEFI_IMG`, `ISO_MBR_SRC`,
`ISO_PARTITION_*`, …); sensible Clonezilla defaults are built in, so the common
case needs no overrides.

## Unified-diff menu patching (diff-based, never replace)

Besides injecting files, the patcher applies **unified diffs** to the extracted
filesystem with the `patch` command — upstream files are modified in place and
never wholesale-replaced, so a stock ISO whose files drifted from the reference
version fails loudly instead of being silently overwritten.

Each `*.patch` in `PATCH_DIR` (default `<script dir>/clonezilla-patch/patches`)
is tried with `--dry-run` against the extracted rootfs and the ISO tree; the
target path in the diff header decides where it belongs. A patch that does not
apply cleanly aborts the run.

Prerequisite for patching: the `patch` utility.

## Included menu option: attach a PBS backup via NBD

- Helper `/usr/local/sbin/ocs-pbs-nbd` (`clonezilla-patch/ocs-pbs-nbd`) is
  injected into the live rootfs.
- Patch `clonezilla-patch/patches/0001-clonezilla-main-menu-pbs-nbd.patch`
  adds a **"pbs-nbd"** entry to the Clonezilla main menu (`/usr/sbin/clonezilla`,
  the mode-selection dialog) that launches the helper.

When the user picks it, the helper:

1. Starts the network exactly like a remote share (sshfs/nfs/samba) does:
   `network_config_if_necessary`, using the Clonezilla `$DIA` (dialog/whiptail)
   TUI — the usual `ocs-live-netcfg` wizard runs if no NIC is configured.
2. If a previous `pbsnbd` instance is still running, warns the user with a
   dialog ("already running, it will be stopped") and stops it (SIGINT first
   for a clean NBD disconnect, SIGKILL if it does not go away), so re-running
   the flow always starts from a clean slate. Instances launched by the flow
   survive a return to the main menu (`setsid`, own session, stdin from
   `/dev/null`).
3. `modprobe nbd max_part=0` (so the kernel does not probe partitions on the
   remote `.fidx` image).
4. Asks, via `$DIA` input-boxes, for:
   - PBS server URL — the `https://` prefix is pre-filled in the box;
   - username (a realm must be present, e.g. `root@pam` or `root@pbs`) and
     password;
   - datastore. A namespace may be appended after a slash:
     `datastore/ns1/ns2` means datastore `datastore` and **everything after the
     first `/` is passed to `pbsnbd -namespace`** (`ns1/ns2`).
   The last used server/username/password/datastore are kept in
   `/tmp/pbsnbd-credentials` (mode 600) and pre-fill the boxes on the next run.
5. Runs `/usr/local/sbin/pbsnbd -list`, which prints the available backups as
   `type/id/time/file.fidx[#comment]` lines sorted by date, **newest first**
   (the trailing `#comment`, if the snapshot has one, is included) and presents
   a **three-level** `$DIA` menu:
   (a) the `<kind>/<backupid>` (e.g. `vm/100`, `ct/50`; the snapshot comment,
   if any, is shown next to it), (b) the backup by date (with file count),
   (c) the `.fidx` file to attach. Menus with a single entry are skipped.
6. Passes the chosen `-list` line verbatim (including any trailing `#comment`,
   which `pbsnbd -path` strips) to `pbsnbd -path` (with the same
   `-baseurl -username -password -datastore [-namespace]` arguments) and
   **forks it in the background**, so the Clonezilla main menu / shell stays
   usable while `/dev/nbd0` attaches. Rerunning the flow warns and stops the
   previous background instance first.

## Generated ISO build (for reference)

Step 4 expands to (values shown are the Clonezilla defaults):

```
xorriso -as mkisofs -o OUT \
  -V 3.3.3-15-amd64 -J -R \
  -isohybrid-mbr /usr/lib/ISOLINUX/isohdpfx.bin \
  -partition_offset 16 -partition_cyl_align on \
  -partition_hd_cyl 64 -partition_sec_hd 32 \
  -iso_mbr_part_type 0x17 \
  -b syslinux/isolinux.bin -c syslinux/boot.cat \
    -no-emul-boot -boot-load-size 4 -boot-info-table \
  -eltorito-alt-boot -e boot/grub/efi.img \
    -no-emul-boot -boot-load-size 6912 \
  <tree>
```

Note the option-name quirks for xorriso 1.5 `-as mkisofs`:
- partition options use underscores (`-partition_offset`, `-iso_mbr_part_type`),
- there is **no** `-mbr_force_bootable` standalone option — the bootable flag
  comes from the embedded `isohdpfx.bin` MBR, so do not pass it.

## Verifying the result

```
# BIT applied + BIOS/UEFI images present:
xorriso -indev OUT -report_el_torito plain

# MBR signature 55 AA at byte 510:
xxd -s 510 -l 2 OUT

# binaries inside the shipped squashfs:
xorriso -indev OUT -osirrox on extract /live/filesystem.squashfs /tmp/f.squashfs -commit
unsquashfs -ll /tmp/f.squashfs | grep -E 'usr/local/sbin/(pbsnbd|machinebackup)'
```

A good ISO reports `boot-info-table isohybrid-suitable` for the BIOS image, has
both BIOS (`/syslinux/isolinux.bin`) and UEFI (`/boot/grub/efi.img`) images, and
ends its MBR with `55aa`.
