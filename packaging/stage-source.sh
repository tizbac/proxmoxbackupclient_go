#!/bin/sh
# stage-source.sh - build pbsgo-<version>.tar.gz from the CURRENT working tree.
#
# The packaging targets (packaging/arch/PKGBUILD, packaging/fedora) consume this
# tarball instead of downloading a commit from GitHub, so a package always
# contains exactly the source it was built from - uncommitted changes included.
# Packaging from a tree that has no .git directory at all works too.
#
# Usage: packaging/stage-source.sh [output]
#        output: a tarball path or an existing directory
#        (default: <repo>/dist/pbsgo-<version>.tar.gz)
#
# The tarball always contains a single top-level directory pbsgo-<version>,
# which is what %setup (RPM) and $_srcroot (PKGBUILD) expect.
set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/.." && pwd)

VERSION=$(sed -n 's/.*"productVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
    "$ROOT/gui/wails.json")
[ -n "$VERSION" ] || {
    echo "error: cannot read productVersion from gui/wails.json" >&2
    exit 1
}

# Default: <repo>/dist/pbsgo-<version>.tar.gz. An existing directory (or a
# path ending in /) means "stage into it".
OUT=${1:-$ROOT/dist/pbsgo-$VERSION.tar.gz}
case "$OUT" in
    */) OUT="${OUT}pbsgo-$VERSION.tar.gz" ;;
esac
[ -d "$OUT" ] && OUT="$OUT/pbsgo-$VERSION.tar.gz"
case "$OUT" in
    /*) : ;;
    *)  OUT="$PWD/$OUT" ;;
esac
mkdir -p "$(dirname "$OUT")"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT INT TERM
STAGE="$WORK/pbsgo-$VERSION"

echo "==> staging source tree ($VERSION)"
mkdir -p "$STAGE"

# Copy the working tree into the top-level directory. Everything is taken as-is
# (tracked, modified and not-yet-committed files alike); only VCS metadata,
# build outputs and packaging leftovers are dropped. Patterns starting with
# './' are anchored at the repository root, the others match at any depth.
#
# gui/build is BOTH: gui/build/windows/icon.ico is tracked and embedded
# (gui/icon.go: //go:embed build/windows/icon.ico), while gui/build/bin is the
# wails output - drop the latter, never the former.
tar -C "$ROOT" \
    --exclude='./.git' \
    --exclude='./.go' \
    --exclude='./dist' \
    --exclude='./gui/build/bin' \
    --exclude='./gui/gui' \
    --exclude='./packaging/arch/src' \
    --exclude='./packaging/arch/pkg' \
    --exclude='./packaging/arch'/*.pkg.tar* \
    --exclude='node_modules' \
    --exclude='vendor' \
    --exclude='coverage.out' \
    --exclude='coverage.html' \
    --exclude='coverage.txt' \
    --exclude='*.syso' \
    --exclude='./gui/versioninfo.json' \
    --exclude='*.orig' \
    --exclude='*.rej' \
    --exclude='.DS_Store' \
    -cf - . | tar -C "$STAGE" -xf -

# Never ship a tarball inside the tarball: a previous staging output (dist/,
# packaging/arch/, or wherever the caller pointed us) must not be re-packed.
find "$STAGE" -type f -name 'pbsgo-*.tar.gz' -delete

tar -C "$WORK" -czf "$OUT" "pbsgo-$VERSION"

# Sanity check: no tracked file may be swallowed by an exclusion pattern
# (gui/icon.go embeds gui/build/windows/icon.ico, for instance). Skipped when
# packaging from a tree without .git.
if [ -e "$ROOT/.git" ] && command -v git >/dev/null 2>&1; then
    tar -tzf "$OUT" | sed "s|^pbsgo-$VERSION/||" | grep -v '/$' | LC_ALL=C sort \
        > "$WORK/in-tarball"
    git -C "$ROOT" ls-files | LC_ALL=C sort > "$WORK/tracked"
    missing=$(LC_ALL=C comm -23 "$WORK/tracked" "$WORK/in-tarball")
    if [ -n "$missing" ]; then
        echo "error: tracked files missing from $OUT:" >&2
        echo "$missing" >&2
        exit 1
    fi
fi

BYTES=$(wc -c <"$OUT" | tr -d ' ')
if command -v sha256sum >/dev/null 2>&1; then
    SUM=$(sha256sum "$OUT" | cut -d' ' -f1)
    echo "==> $OUT ($BYTES bytes)"
    echo "==> sha256 $SUM"
else
    echo "==> $OUT ($BYTES bytes)"
fi
