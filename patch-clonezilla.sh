#!/usr/bin/env bash
#
# patch-clonezilla.sh - inject files into a Clonezilla live ISO while preserving boot.
#
# Rebuilds the ISO's squashfs rootfs with extra files installed, then writes it
# back into a copy of the ISO, keeping El Torito (BIOS+UEFI) and the isohybrid MBR.

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: patch-clonezilla.sh -o OUTPUT_ISO INPUT_ISO [FILE]...

  INPUT_ISO     Stock Clonezilla live ISO to patch.
  -o OUTPUT_ISO Destination ISO to write.
  FILE...       Files to install into the squashfs (default dir: usr/local/sbin).

Env overrides:
  ISO_SQFS      squashfs path inside the ISO   (default /live/filesystem.squashfs)
  INSTALL_DIR   install dir inside the squashfs (default usr/local/sbin)
EOF
  exit "${1:-1}"
}

SQFS_PATH="${ISO_SQFS:-/live/filesystem.squashfs}"
INSTALL_DIR="${INSTALL_DIR:-usr/local/sbin}"

OUT=""
POSITIONAL=()
while [ $# -gt 0 ]; do
  case "$1" in
    -o)        OUT="${2:-}"; shift 2 ;;
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

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
SRC_SQFS="$WORK/filesystem.squashfs"
NEW_SQFS="$WORK/filesystem.new"
ROOTFS="$WORK/rootfs"

echo "### [1/5] extract squashfs ($SQFS_PATH) from ISO"
xorriso -indev "$IN" -osirrox on extract "$SQFS_PATH" "$SRC_SQFS" -commit >/dev/null 2>&1

echo "### [2/5] detect squashfs compression / block size"
SUMMARY="$(unsquashfs -s "$SRC_SQFS")"
COMP="$(printf '%s\n' "$SUMMARY" | awk '{if (tolower($1)=="compression") print $2}')"
BLOCK="$(printf '%s\n' "$SUMMARY" | awk '{if (tolower($1)=="block" && tolower($2)=="size") print $3}')"
[ -n "$COMP" ] && [ -n "$BLOCK" ] || { echo "error: could not detect squashfs format" >&2; exit 1; }
echo "        compression=$COMP  block_size=$BLOCK"

echo "### [3/5] expand rootfs and inject ${#FILES[@]} file(s) into $INSTALL_DIR/"
mkdir -p "$ROOTFS"
# unsquashfs returns 2 when run unprivileged (cannot preserve security.* xattrs /
# full ownership). Files are still created, so accept rc 0 or 2 and verify by content.
set +e
unsquashfs -f -no-xattrs -d "$ROOTFS" "$SRC_SQFS" >/dev/null 2>&1
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

echo "### [4/5] rebuild squashfs (matching compression/block size)"
if [ "$COMP" = "xz" ]; then
  ( cd "$ROOTFS" && mksquashfs . "$NEW_SQFS" -all-root -b "$BLOCK" -comp xz -Xbcj x86 >/dev/null 2>&1 )
else
  ( cd "$ROOTFS" && mksquashfs . "$NEW_SQFS" -all-root -b "$BLOCK" -comp "$COMP" >/dev/null 2>&1 )
fi
echo "        rebuilt squashfs: $(stat -c%s "$NEW_SQFS") bytes"

echo "### [5/5] write squashfs into ISO (El Torito + isohybrid MBR kept)"
rm -f "$OUT"
xorriso -indev "$IN" -outdev "$OUT" -update "$NEW_SQFS" "$SQFS_PATH" -boot_image any keep -commit >/dev/null 2>&1

echo "--- verification ---"
NEW_MD5="$(md5sum "$NEW_SQFS" | cut -d' ' -f1)"
VERIFY_SQFS="$WORK/verify.squashfs"
xorriso -indev "$OUT" -osirrox on extract "$SQFS_PATH" "$VERIFY_SQFS" -commit >/dev/null 2>&1
OUT_MD5="$(md5sum "$VERIFY_SQFS" | cut -d' ' -f1)"
echo "  rebuilt squashfs md5 : $NEW_MD5"
echo "  iso squashfs    md5 : $OUT_MD5"
[ "$NEW_MD5" = "$OUT_MD5" ] || { echo "error: squashfs in ISO does not match rebuilt image" >&2; exit 1; }
BOOTREFS="$(xorriso -indev "$OUT" -report_el_torito plain 2>/dev/null | grep -c 'El Torito' || true)"
echo "  El Torito references : $BOOTREFS"
echo "  output ISO           : $OUT ($(stat -c%s "$OUT") bytes)"
echo "OK: wrote $OUT"
