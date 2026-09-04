#!/usr/bin/env bash
#
# build_winpe_iso.sh — Embed Proxmox Backup Client into a Windows PE ISO
#                      and auto-launch it at start (rescue environment).
#
# Works with ANY WinPE ISO that follows the standard Microsoft layout, i.e.
# it contains a readable "sources/boot.wim" (the PE image with
# Windows/System32/startnet.cmd) and standard El-Torito BIOS+UEFI boot images.
# Tested against the archive.org "WinPE_amd64_v10.0.19041.1.iso".
#
# Requirements (Linux host):
#   7z            (p7zip-full)      ISO extraction
#   wimlib-imagex (wimtools)        WIM read/modify
#   xorriso       (xorriso)         bootable ISO rebuild
#   curl          (only if you let the script download the app exe)
#
# Usage:
#   ./build_winpe_iso.sh [OPTIONS]
#
# Options:
#   -i <path>       Base WinPE .iso (default: WinPE_base.iso in CWD)
#   -x <path>       The Windows .exe to embed (default: ProxmoxBackupClientGO.exe in CWD)
#   -o <path>       Output ISO path (default: ProxmoxBackupClient-WinPE.iso)
#   -a <launch.cmd> Optional custom launch script placed at X:\Start.cmd
#                   and invoked after wpeinit. Defaults to an embedded Start.cmd.
#   -k              Keep intermediate work dir (default: removed on success)
#   -h              Show this help
#
# The produced ISO boots into Windows PE, runs wpeinit, then runs Start.cmd
# which copies ProxmoxBackupClientGO.exe to the RAM drive and launches it.

set -euo pipefail

ISO_IN="${1:-WinPE_base.iso}"
EXE_IN="ProxmoxBackupClientGO.exe"
OUT_ISO="ProxmoxBackupClient-WinPE.iso"
LAUNCH_CMD=""
KEEP=0

usage() { sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'; exit 0; }

while getopts "i:x:o:a:kh" opt; do
    case "$opt" in
        i) ISO_IN="$OPTARG" ;;
        x) EXE_IN="$OPTARG" ;;
        o) OUT_ISO="$OPTARG" ;;
        a) LAUNCH_CMD="$OPTARG" ;;
        k) KEEP=1 ;;
        h) usage ;;
        *) usage ;;
    esac
done

for tool in 7z wimlib-imagex xorriso; do
    command -v "$tool" >/dev/null 2>&1 || { echo "ERROR: '$tool' not found. Install p7zip-full, wimtools, xorriso."; exit 1; }
done

[ -f "$ISO_IN" ]  || { echo "ERROR: base WinPE ISO not found: $ISO_IN"; exit 1; }
[ -f "$EXE_IN" ]  || { echo "ERROR: exe to embed not found: $EXE_IN"; exit 1; }

WD="$(mktemp -d ./winpe-work.XXXXXX)"
trap 'cd "$(pwd)" 2>/dev/null; [ "$KEEP" = 0 ] && rm -rf "$WD" && rm -f "$WD.iso.cat" 2>/dev/null || true' EXIT
EXT="$WD/extracted"

echo "==> Using base ISO: $ISO_IN"
echo "==> Embedding:      $EXE_IN"
echo "==> Output ISO:     $OUT_ISO"

# ---------------------------------------------------------------
# 1) Extract the ISO contents
# ---------------------------------------------------------------
echo "==> Extracting ISO..."
mkdir -p "$EXT"
7z x -y -o"$EXT" "$ISO_IN" >/dev/null

BOOTWIM="$EXT/sources/boot.wim"
[ -f "$BOOTWIM" ] || { echo "ERROR: $ISO_IN has no sources/boot.wim (not a standard WinPE ISO?)"; exit 1; }

# ---------------------------------------------------------------
# 2) Verify the WIM and locate the image to modify
# ---------------------------------------------------------------
IMG_COUNT=$(wimlib-imagex info "$BOOTWIM" 2>/dev/null | grep -i "Image Count" | awk '{print $NF}')
IMG_COUNT="${IMG_COUNT:-1}"
echo "==> boot.wim has $IMG_COUNT image(s); using image 1"

rm -f "$WD/boot.wim.new"

# ---------------------------------------------------------------
# 3) Inject the exe + Start.cmd into boot.wim (image 1)
# ---------------------------------------------------------------
# Replace system startnet.cmd so it runs wpeinit THEN our launcher.
# Generate Start.cmd (our own unless -a given).
if [ -n "$LAUNCH_CMD" ]; then
    cp "$LAUNCH_CMD" "$WD/Start.cmd"
else
    cat > "$WD/Start.cmd" <<'CMD'
@echo off
title Proxmox Backup Client - WinPE Rescue Environment
color 0B
echo ============================================================
echo  Proxmox Backup Client - WinPE Rescue Environment
echo ============================================================
set "WORKDIR=X:\ProxmoxBackup"
if not exist "%WORKDIR%" mkdir "%WORKDIR%"
for %%d in (C D E F G H I J K L M N O P Q R S T U V W X Y Z) do (
    if exist "%%d:\START_APP.exe" (
        copy /Y "%%d:\START_APP.exe" "%WORKDIR%\ProxmoxBackupClientGO.exe" >nul 2>&1
        goto :found
    )
)
:found
echo Launching Proxmox Backup Client...
start "" "%WORKDIR%\ProxmoxBackupClientGO.exe"
cmd.exe /k "title Proxmox Backup Client - Rescue Shell && cd /d %WORKDIR%"
CMD
    # Put the real forged placeholder filename -> the embedded exe name
    BN=$(basename "$EXE_IN")
    sed -i "s/START_APP.exe/$BN/g" "$WD/Start.cmd"
fi

cat > "$WD/startnet.cmd" <<'CMD'
wpeinit
for %%d in (C D E F G H I J K L M N O P Q R S T U V W) do (
    if exist "%%d:\Start.cmd" (
        %%d:\Start.cmd
        goto :end
    )
)
if exist "X:\Start.cmd" X:\Start.cmd
:end
CMD

echo "==> Injecting $BN + Start.cmd into boot.wim..."
printf 'add %s /%s\nadd %s /Start.cmd\nadd %s /Windows/System32/startnet.cmd\n' \
    "$(pwd)/$EXE_IN" "$BN" "$WD/Start.cmd" "$WD/startnet.cmd" \
    | wimlib-imagex update "$BOOTWIM" 1 >/tmp/wimupdate.log 2>&1 || {
        echo "ERROR: wimlib update failed:"; tail -20 /tmp/wimupdate.log; exit 1
    }

echo "==> Verifying injected files..."
wimlib-imagex dir "$BOOTWIM" 1 --path="/$BN"       >/dev/null || { echo "ERROR: exe missing from WIM"; exit 1; }
wimlib-imagex dir "$BOOTWIM" 1 --path="/Start.cmd" >/dev/null || { echo "ERROR: Start.cmd missing from WIM"; exit 1; }

# ---------------------------------------------------------------
# 4) Extract the hidden El-Torito boot images (BIOS + UEFI)
# ---------------------------------------------------------------
echo "==> Locating El-Torito boot images..."
# "El Torito boot img" rows carry trailing "Ldsiz LBA". BIOS image = #1,
# UEFI image = #2. The BIOS loader (etfsboot.com) is exactly Ldsiz long;
# the UEFI image (efisys.bin, a FAT floppy) is sized by the "img blks" row.
REPORT="$(xorriso -indev "$ISO_IN" -report_el_torito plain 2>/dev/null)"
ET_LBA=$( awk '/El Torito boot img :[ ]+1/{print $NF}'   <<<"$REPORT" )
ET_NBLK=$(awk '/El Torito boot img :[ ]+1/{print $(NF-1)}' <<<"$REPORT" )
EF_LBA=$( awk '/El Torito boot img :[ ]+2/{print $NF}'   <<<"$REPORT" )
EF_NBLK=$(awk '/El Torito img blks :[ ]+2/{print $NF}'    <<<"$REPORT" )

if [ -n "$ET_LBA" ] && [ -n "$ET_NBLK" ]; then
    dd if="$ISO_IN" of="$WD/etfsboot.com" bs=2048 skip="$ET_LBA" count="$ET_NBLK" status=none
    echo "    BIOS boot image: LBA $ET_LBA ($ET_NBLK blocks)"
else
    echo "    No BIOS El-Torito found; BIOS boot disabled."
    ET_NBLK=""
fi
if [ -n "$EF_LBA" ] && [ -n "$EF_NBLK" ]; then
    dd if="$ISO_IN" of="$WD/efisys.bin" bs=2048 skip="$EF_LBA" count="$EF_NBLK" status=none
    echo "    UEFI boot image: LBA $EF_LBA ($EF_NBLK blocks)"
else
    echo "    No UEFI El-Torito found; UEFI boot disabled."
    EF_NBLK=""
fi

# ---------------------------------------------------------------
# 5) Stage boot files into the tree for mkisofs/xorriso to pick up
# ---------------------------------------------------------------
if [ -n "$ET_NBLK" ]; then cp "$WD/etfsboot.com" "$EXT/etfsboot.com"; fi
if [ -n "$EF_NBLK" ]; then cp "$WD/efisys.bin"   "$EXT/efisys.bin";   fi

# ---------------------------------------------------------------
# 6) Rebuild the ISO, preserving BIOS + UEFI El-Torito boot
# ---------------------------------------------------------------
echo "==> Rebuilding ISO..."
iso_args=(
    -o "$OUT_ISO"
    -V "PBC_WINPE"
    -iso-level 3
    -J -joliet-long
    -R
    -b etfsboot.com -no-emul-boot -boot-load-size 8 -boot-info-table
    -eltorito-alt-boot
    -e efisys.bin -no-emul-boot -boot-load-size 2000 -boot-info-table
    --boot-catalog-hide
    "$EXT"
)
if [ -z "$ET_NBLK" ]; then
    # No BIOS boot image: build EFI-only
    iso_args=(
        -o "$OUT_ISO"
        -V "PBC_WINPE"
        -iso-level 3 -J -joliet-long -R
        -e efisys.bin -no-emul-boot -boot-load-size 2000
        "$EXT"
    )
fi
xorriso -as mkisofs "${iso_args[@]}" 2>/dev/null

echo "==> Verifying output boot records..."
xorriso -indev "$OUT_ISO" -report_el_torito plain 2>/dev/null \
    | grep -i "El Torito boot img" || echo "WARNING: no El-Torito boot record found in output."

echo ""
echo "SUCCESS: $OUT_ISO created"
echo "Boot it on a target machine or in QEMU:"
echo "  qemu-system-x86_64 -cdrom \"$OUT_ISO\" -m 2048 -boot d"
