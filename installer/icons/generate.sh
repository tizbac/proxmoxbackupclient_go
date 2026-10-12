#!/usr/bin/env bash
# Regenerates the per-brand .ico files.
#
#   ./generate.sh
#
# Outputs:
#   installer/icons/{nimbusbackup,acmebackup}.ico      — branded ARP/shortcut icons
#   gui/build/windows/brands/<brandkey>.ico            — per-brand system-tray icons
#     (embedded into the GUI binary via gui/icon.go and selected at runtime
#      from the executable's name, so one binary ships every brand)
#
# Sources:
#   installer/icons/src/proxmox_mark.svg — official Proxmox orange mark, as
#                                 published on www.proxmox.com (favicon.svg)
#   installer/icons/src/*.svg   — glyph-only square tiles for brands whose
#                                 logo is a wide banner (drawn from
#                                 gui/frontend/public/brands/*.svg)
#   installer/icons/EtitechBackup.ico — used as-is
#
# Requires: rsvg-convert (librsvg2-bin), ImageMagick 7 (magick)
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p ../../gui/build/windows/brands
BRANDS=../../gui/build/windows/brands

declare -A EXE=([nimbusbackup]=NimbusBackup [acmebackup]=AcmeBackup)
for b in "${!EXE[@]}"; do
  # CamelCase name as referenced by WiX's $(var.ExeName).ico
  Camel="${EXE[$b]}"
  rsvg-convert -w 1024 "src/$b.svg" -o "/tmp/$b.png"
  magick "/tmp/$b.png" -define icon:auto-resize=256,128,64,48,32,16 "$Camel.ico"
  cp "$Camel.ico" "$BRANDS/$b.ico"
done

# Proxmox tile: the official orange mark from www.proxmox.com centered on the
# shared dark tile (same style as the Nimbus/Acme tiles).
rsvg-convert -w 512 -h 512 src/proxmox_mark.svg -o /tmp/proxmox_mark.png
magick -size 512x512 xc:none -fill '#0f172a' -draw 'roundrectangle 0,0 511,511 72,72' \
  \( /tmp/proxmox_mark.png -resize 300x300 \) -gravity center -composite \
  -define icon:auto-resize=256,128,64,48,32,16 ProxmoxBackupClient.ico
cp ProxmoxBackupClient.ico "$BRANDS/proxmoxbackupclient.ico"
# Also the exe's resource icon (window/taskbar/Alt-Tab), shared by every
# branded rename of the binary — better than Wails' placeholder icon.
cp ProxmoxBackupClient.ico ../../gui/build/windows/icon.ico

# Etitech: tray tile is the same artwork as the installer icon.
cp EtitechBackup.ico "$BRANDS/etitechbackup.ico"

file "$BRANDS"/*.ico *.ico
