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
- An isohybrid MBR prefix, normally
  `/usr/lib/ISOLINUX/isohdpfx.bin` (from the `syslinux`/`isolinux` packages).

## Usage

```
./patch-clonezilla.sh -o OUTPUT_ISO INPUT_ISO [FILE]...
```

Example — add the two ticket-enabled binaries:

```
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup
```

Every file listed after the input ISO is installed into `usr/local/sbin/` inside
the live rootfs. All Clonezilla-specific paths/sizes are env-overridable
(see `ISO_SQFS`, `INSTALL_DIR`, `ISO_BIOS_IMG`, `ISO_UEFI_IMG`, `ISO_MBR_SRC`,
`ISO_PARTITION_*`, …); sensible Clonezilla defaults are built in, so the common
case needs no overrides.

## The generated ISO build (for reference)

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
