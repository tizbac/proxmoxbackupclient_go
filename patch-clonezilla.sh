#!/usr/bin/env bash
#
# patch-clonezilla.sh - inject files into a Clonezilla live ISO while preserving boot.
#
# Why a full rebuild instead of an in-place update?
#   An in-place `xorriso -update ... -boot_image any keep` leaves the isolinux
#   Boot Information Table (BIT) pointing at the old El Torito layout. Once the
#   squashfs grows the BIT goes stale and isolinux aborts with
#   "isolinux: Image checksum error". Rebuilding the ISO from scratch in
#   `xorriso -as mkisofs` mode makes xorriso re-emit a fresh BIT
#   (-boot-info-table) and a valid isohybrid MBR, so the patched ISO still boots
#   from CD, USB (dd) and UEFI.

set -euo pipefail

# Directory this script lives in (so the menu patches can be found even when
# the script is invoked from another cwd).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  cat <<'EOF'
Usage: patch-clonezilla.sh -o OUTPUT_ISO INPUT_ISO [FILE]...

  INPUT_ISO     Stock Clonezilla live ISO to patch.
  -o OUTPUT_ISO Destination ISO to write.
  FILE...       Files to install into the squashfs rootfs (default dir: usr/local/sbin).

  -p PATCH_DIR  Directory containing unified diffs (*.patch) to apply with
                `patch` on the extracted filesystem (rootfs or ISO tree).
                Diff-based: upstream files are patched, never replaced.
                (default: <script dir>/clonezilla-patch/patches)

Env overrides:
  ISO_SQFS              squashfs path inside the ISO        (default /live/filesystem.squashfs)
  INSTALL_DIR           dir inside the squashfs to use      (default usr/local/sbin)
  PATCH_DIR             see -p above
  ISO_VOLUME            volume id (auto-detected from INPUT_ISO if unset)
  ISO_BIOS_IMG          BIOS El Torito image path           (default syslinux/isolinux.bin)
  ISO_BIOS_CAT          El Torito catalog path              (default syslinux/boot.cat)
  ISO_BIOS_LOAD         BIOS boot-load-size                 (default 4)
  ISO_UEFI_IMG          UEFI El Torito image path           (default boot/grub/efi.img)
  ISO_UEFI_LOAD         UEFI boot-load-size                 (default 6912)
  ISO_MBR_SRC           isohybrid MBR prefix file           (default /usr/lib/ISOLINUX/isohdpfx.bin)
  ISO_PARTITION_OFFSET  partition start LBA                 (default 16)
  ISO_PARTITION_HD_CYL  heads per cylinder                  (default 64)
  ISO_PARTITION_SEC_HD  sectors per head                    (default 32)
  ISO_PARTITION_TYPE    MBR partition type                  (default 0x17)
EOF
  exit "${1:-1}"
}

SQFS_PATH="${ISO_SQFS:-/live/filesystem.squashfs}"
INSTALL_DIR="${INSTALL_DIR:-usr/local/sbin}"
PATCH_DIR="${PATCH_DIR:-$SCRIPT_DIR/clonezilla-patch/patches}"
PATCH_DIR_SET="0"
BIOS_IMG="${ISO_BIOS_IMG:-syslinux/isolinux.bin}"
BIOS_CAT="${ISO_BIOS_CAT:-syslinux/boot.cat}"
BIOS_LOAD="${ISO_BIOS_LOAD:-4}"
UEFI_IMG="${ISO_UEFI_IMG:-boot/grub/efi.img}"
UEFI_LOAD="${ISO_UEFI_LOAD:-6912}"
MBR_SRC="${ISO_MBR_SRC:-/usr/lib/ISOLINUX/isohdpfx.bin}"
PART_OFFSET="${ISO_PARTITION_OFFSET:-16}"
PART_HD_CYL="${ISO_PARTITION_HD_CYL:-64}"
PART_SEC_HD="${ISO_PARTITION_SEC_HD:-32}"
PART_TYPE="${ISO_PARTITION_TYPE:-0x17}"

OUT=""
POSITIONAL=()
while [ $# -gt 0 ]; do
  case "$1" in
    -o)        OUT="${2:-}"; shift 2 ;;
    -p|--patch-dir) PATCH_DIR="${2:-}"; PATCH_DIR_SET="1"; shift 2 ;;
    -h|--help) usage 0 ;;
    -*)        echo "unknown option: $1" >&2; usage ;;
    *)         POSITIONAL+=("$1"); shift ;;
  esac
done
[ -n "$OUT" ] || { echo "error: -o OUTPUT_ISO is required" >&2; usage; }
[ ${#POSITIONAL[@]} -ge 1 ] || { echo "error: INPUT_ISO is required" >&2; usage; }
IN="${POSITIONAL[0]}"
if [ ${#POSITIONAL[@]} -ge 2 ]; then FILES=( "${POSITIONAL[@]:1}" ); else FILES=(); fi
[ -f "$IN" ] || { echo "error: input ISO not found: $IN" >&2; exit 1; }
[ ${#FILES[@]} -gt 0 ] || { echo "error: at least one FILE to inject is required" >&2; usage; }
for f in "${FILES[@]}"; do [ -f "$f" ] || { echo "error: not a file: $f" >&2; exit 1; }; done
[ -f "$MBR_SRC" ] || { echo "error: isohybrid MBR prefix not found: $MBR_SRC" >&2; exit 1; }
command -v 7z >/dev/null 2>&1 || { echo "error: 7z not found (required to extract the ISO tree)" >&2; exit 1; }

# Apply a set of unified diffs (*.patch) to the extracted filesystem with
# `patch`. Each patch is tried against the extracted rootfs and the ISO tree
# (the target path from the diff header decides where it belongs). A patch
# must apply cleanly (--dry-run first); otherwise we abort so a stock ISO
# whose upstream files changed is never silently mis-patched.
apply_unified_patches() {
  local p base t applied
  command -v patch >/dev/null 2>&1 || { echo "error: 'patch' not found (required to apply menu patches)" >&2; return 1; }
  if [ -z "$PATCH_DIR" ] || [ ! -d "$PATCH_DIR" ]; then
    if [ "$PATCH_DIR_SET" = "1" ]; then
      echo "error: PATCH_DIR does not exist: ${PATCH_DIR:-<unset>}" >&2
      return 1
    fi
    echo "  no patches in ${PATCH_DIR:-<unset>}, skipping menu patching"
    return 0
  fi
  local n=0
  while IFS= read -r p; do
    [ -n "$p" ] || continue
    base="$PATCH_DIR/$p"
    applied=0
    for t in "$ROOTFS" "$TREE"; do
      if patch -d "$t" -p1 -N --dry-run --forward -s < "$base" >/dev/null 2>&1; then
        if ! patch -d "$t" -p1 -N --forward -s < "$base" >/dev/null 2>&1; then
          echo "error: failed to apply patch $base to $t" >&2
          return 1
        fi
        echo "        + patched $p -> ${t}${t:+/}"
        n=$((n+1))
        applied=1
        break
      fi
    done
    if [ "$applied" != "1" ]; then
      echo "error: patch $base does not apply cleanly to the extracted filesystem (upstream file changed?)" >&2
      return 1
    fi
  done < <(LC_ALL=C find "$PATCH_DIR" -maxdepth 1 -name '*.patch' -type f -printf '%f\n' 2>/dev/null | sort)
  [ "$n" -ge 1 ] && echo "        applied $n unified-diff patch(es)"
  return 0
}

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
TREE="$WORK/tree"
ROOTFS="$WORK/rootfs"
NEW_SQFS="$WORK/filesystem.new"
SQFS="$TREE$SQFS_PATH"

# Volume id: explicit env wins, else read it back from the source ISO.
VOL="${ISO_VOLUME:-}"
if [ -z "$VOL" ]; then
  VOL="$(isoinfo -d -i "$IN" 2>/dev/null | grep -i 'volume id' | sed -E 's/.*volume id:[[:space:]]*//' | tr -d '[:space:]')"
fi
[ -n "$VOL" ] || VOL="CLONEZILLA"

echo "### [1/5] extract full ISO filesystem (7z)"
mkdir -p "$TREE"
7z x -o"$TREE" -y -bb1 "$IN" >/dev/null
[ -f "$SQFS" ] || { echo "error: $SQFS_PATH not found in ISO" >&2; exit 1; }

echo "### [2/5] detect squashfs compression / block size"
SUM="$(unsquashfs -s "$SQFS")"
COMP="$(printf '%s\n' "$SUM" | awk '{if (tolower($1)=="compression") print $2}')"
BLOCK="$(printf '%s\n' "$SUM" | awk '{if (tolower($1)=="block" && tolower($2)=="size") print $3}')"
[ -n "$COMP" ] && [ -n "$BLOCK" ] || { echo "error: could not detect squashfs format" >&2; exit 1; }
echo "        compression=$COMP  block_size=$BLOCK"

echo "### [3/5] expand rootfs, inject ${#FILES[@]} file(s) into $INSTALL_DIR/, rebuild squashfs"
mkdir -p "$ROOTFS"
# unsquashfs returns 2 when run unprivileged (cannot preserve security.* xattrs);
# files are still created, so accept rc 0 or 2 and validate by content count.
set +e
fakeroot unsquashfs -f -no-xattrs -d "$ROOTFS" "$SQFS" >/dev/null 2>&1
RC=$?
set -e
[ "$RC" -eq 0 ] || [ "$RC" -eq 2 ] || { echo "error: unsquashfs failed (rc=$RC)" >&2; exit 1; }
# find exits non-zero on unreadable entries; isolate so pipefail/-e cannot abort.
NFILES="$( { find "$ROOTFS" -mindepth 1 2>/dev/null | wc -l; } || true )"
[ "${NFILES:-0}" -ge 1000 ] || { echo "error: expansion incomplete (only ${NFILES:-0} entries)" >&2; exit 1; }
echo "        expanded: $NFILES filesystem entries"
mkdir -p "$ROOTFS/$INSTALL_DIR"
for f in "${FILES[@]}"; do
  install -m 0755 "$f" "$ROOTFS/$INSTALL_DIR/"
  echo "        + $INSTALL_DIR/$(basename "$f")  ($(md5sum "$f" | cut -d' ' -f1))"
done
# Apply any unified diffs (e.g. the PBS-NBD main-menu option) diff-based,
# so upstream files are patched in place instead of being replaced.
apply_unified_patches
if [ "$COMP" = "xz" ]; then
  ( cd "$ROOTFS" && fakeroot mksquashfs . "$NEW_SQFS" -all-root -b "$BLOCK" -comp xz -Xbcj x86 >/dev/null 2>&1 )
else
  ( cd "$ROOTFS" && fakeroot mksquashfs . "$NEW_SQFS" -all-root -b "$BLOCK" -comp "$COMP" >/dev/null 2>&1 )
fi
[ -s "$NEW_SQFS" ] || { echo "error: squashfs rebuild failed" >&2; exit 1; }
cp -f "$NEW_SQFS" "$SQFS"
echo "        rebuilt squashfs: $(stat -c%s "$NEW_SQFS") bytes"

echo "### [4/5] rebuild ISO (mkisofs mode: -boot-info-table + isohybrid MBR)"
rm -f "$OUT"
xorriso -as mkisofs -o "$OUT" \
  -V "$VOL" -J -R \
  -isohybrid-mbr "$MBR_SRC" \
  -partition_offset "$PART_OFFSET" -partition_cyl_align on \
  -partition_hd_cyl "$PART_HD_CYL" -partition_sec_hd "$PART_SEC_HD" \
  -iso_mbr_part_type "$PART_TYPE" \
  -b "$BIOS_IMG" -c "$BIOS_CAT" -no-emul-boot -boot-load-size "$BIOS_LOAD" -boot-info-table \
  -eltorito-alt-boot -e "$UEFI_IMG" -no-emul-boot -boot-load-size "$UEFI_LOAD" \
  "$TREE"

echo "### [5/5] verify"
NEW_MD5="$(md5sum "$NEW_SQFS" | cut -d' ' -f1)"
VSQFS="$WORK/verify.squashfs"
xorriso -indev "$OUT" -osirrox on extract "$SQFS_PATH" "$VSQFS" -commit >/dev/null 2>&1
ISO_MD5="$(md5sum "$VSQFS" | cut -d' ' -f1)"
[ "$NEW_MD5" = "$ISO_MD5" ] || { echo "error: squashfs in ISO does not match rebuilt image" >&2; exit 1; }
BITREFS="$(xorriso -indev "$OUT" -report_el_torito plain 2>/dev/null | grep -c 'boot-info-table' || true)"
[ "${BITREFS:-0}" -ge 1 ] || { echo "error: boot-info-table was not applied to the BIOS image" >&2; exit 1; }
SIG="$(dd if="$OUT" bs=1 skip=510 count=2 2>/dev/null | xxd -p)"
[ "$SIG" = "55aa" ] || { echo "error: MBR 0x55AA signature missing" >&2; exit 1; }
echo "  rebuilt squashfs md5 : $NEW_MD5"
echo "  iso squashfs    md5 : $ISO_MD5"
echo "  boot-info-table refs : $BITREFS"
echo "  MBR signature       : $SIG"
echo "  output ISO          : $OUT ($(stat -c%s "$OUT") bytes)"
echo "OK: wrote $OUT"
