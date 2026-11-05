#!/usr/bin/env bash
# Regenerates the per-brand .ico files.
#
#   ./generate.sh
#
# Outputs:
#   installer/icons/<brand>-gui.ico                    : the installer's and shortcuts' icons
#   gui/build/windows/brands/<brandkey>.ico            : per-brand system-tray icons
#     (embedded into the GUI binary via gui/icon.go and selected at runtime
#      from the executable's name, so one binary ships every brand)
#
# Sources:
#   installer/icons/src/default*.svg — the default (unbranded) tile: save and
#                                 restore arrows over a server. default.svg is
#                                 used for 48 px and up, default-32.svg for 24
#                                 and 32 px, default-16.svg is drawn pixel by pixel
#   installer/icons/src/*.svg   — glyph-only square tiles for brands whose
#                                 logo is a wide banner (drawn from
#                                 gui/frontend/public/brands/*.svg)
#   installer/icons/etitechbackup-gui.ico : used as-is
#
# Requires: rsvg-convert (librsvg2-bin), ImageMagick 7 (magick)
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p ../../gui/build/windows/brands
BRANDS=../../gui/build/windows/brands

for b in nimbusbackup; do
  # Named as WiX's $(var.ExeName).ico refers to it
  rsvg-convert -w 1024 "src/$b.svg" -o "/tmp/$b.png"
  magick "/tmp/$b.png" -define icon:auto-resize=256,128,64,48,32,16 "$b-gui.ico"
  cp "$b-gui.ico" "$BRANDS/$b.ico"
done

# Default tile: no Proxmox mark (their media kit rules out using it as an app
# icon). Each size is rendered from the artwork drawn for it rather than scaled
# down from 256, so the tray icon stays readable at 16 px.
for s in 256 128 64 48; do rsvg-convert -w $s -h $s src/default.svg -o /tmp/default-$s.png; done
for s in 32 24; do rsvg-convert -w $s -h $s src/default-32.svg -o /tmp/default-$s.png; done
rsvg-convert -w 16 -h 16 src/default-16.svg -o /tmp/default-16.png
magick /tmp/default-{256,128,64,48,32,24,16}.png salvanza-gui.ico
cp salvanza-gui.ico "$BRANDS/salvanza.ico"
# Also the exe's resource icon (window/taskbar/Alt-Tab), shared by every
# branded rename of the binary — better than Wails' placeholder icon.
cp salvanza-gui.ico ../../gui/build/windows/icon.ico

# Etitech: tray tile is the same artwork as the installer icon.
cp etitechbackup-gui.ico "$BRANDS/etitechbackup.ico"

file "$BRANDS"/*.ico *.ico
