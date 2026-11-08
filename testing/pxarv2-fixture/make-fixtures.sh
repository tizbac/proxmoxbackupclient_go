#!/bin/bash
# Writes the snapshots TestPXARv2OfficialInterop (gui/zz_pxarv2_official_livetest_test.go)
# reads, with the OFFICIAL proxmox-backup-client, and the expected hashes:
#
#   host/pxarv2-legacy  legacy
#   host/pxarv2-split   data, metadata (unchanged), metadata (after changes)
#
# Run it where proxmox-backup-client is installed (the test PBS itself will
# do), with PBS_REPOSITORY, PBS_PASSWORD and PBS_FINGERPRINT set for the test
# datastore. It prints each snapshot's time: put those in pxarv2Snapshots in
# the test, and copy out/*.sha256 to gui/testdata/pxarv2/.
set -e
here=$(cd "$(dirname "$0")" && pwd)
F=${FIXTURE_DIR:-/var/tmp/pxfix}
rm -rf "$F" && mkdir -p "$F/out"
python3 "$here/fixture_gen.py" "$F/fixture"
cd "$F/fixture"
hashes() { find . -type f -print0 | sort -z | xargs -0 sha256sum > "$F/out/$1.sha256"; }
snap() { # label backup-id change-detection-mode
  hashes "$1"
  proxmox-backup-client backup "fixture.pxar:$F/fixture" --backup-id "$2" --change-detection-mode "$3" > "$F/out/$1.log" 2>&1
  t=$(proxmox-backup-client snapshot list "host/$2" --output-format json |
    python3 -c 'import json,sys; print(max(s["backup-time"] for s in json.load(sys.stdin)))')
  echo "$1 $2 $t"
  sleep 2
}
snap legacy pxarv2-legacy legacy
snap data pxarv2-split data
snap metadata1 pxarv2-split metadata
python3 "$here/fixture_gen.py" "$F/fixture" mutate
snap metadata2 pxarv2-split metadata
echo "hashes in $F/out"
