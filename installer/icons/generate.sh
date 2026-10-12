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
#   installer/icons/src/*.svg   — glyph-only square tiles for brands whose
#                                 logo is a wide banner (drawn from
#                                 gui/frontend/public/brands/*.svg)
#   installer/icons/ProxmoxBackupClient.ico, EtitechBackup.ico — used as-is
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

# Proxmox tray tile: center the existing wide scene on a black square. Its
# corners are opaque black, so the padding is seamless.
magick ProxmoxBackupClient.ico -background black -gravity center \
  -extent 256x256 -define icon:auto-resize=256,128,64,48,32,16 \
  "$BRANDS/proxmoxbackupclient.ico"

# Etitech: tray tile is the same artwork as the installer icon.
cp EtitechBackup.ico "$BRANDS/etitechbackup.ico"

file "$BRANDS"/*.ico *.ico
