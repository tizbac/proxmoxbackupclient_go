#!/bin/sh
# build-rpm.sh - build the pbsgo RPM inside a Fedora Docker container.
#
# Usage: packaging/fedora/build-rpm.sh [output-dir]     (default: dist/)
#
# Requirements: docker, plus network access from the container (dnf +
# Go module downloads). The host only needs git and docker; Go, GTK and
# WebKit are installed inside the container.
#
# The container image can be overridden with $PBSGO_FEDORA_IMAGE
# (default: fedora:44).
set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
OUT_DIR=${1:-$ROOT/dist}
# docker treats a bare relative path in -v as a *named volume*; make it absolute
case "$OUT_DIR" in
  /*) : ;;
  *) OUT_DIR="$PWD/$OUT_DIR" ;;
esac
IMAGE=${PBSGO_FEDORA_IMAGE:-fedora:44}

VERSION=$(sed -n 's/.*"productVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$ROOT/gui/wails.json")
[ -n "$VERSION" ] || { echo "error: cannot read productVersion from gui/wails.json" >&2; exit 1; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT INT TERM

echo "==> staging source tree ($VERSION)"
mkdir -p "$WORK/stage"
git -C "$ROOT" archive --format=tar HEAD | tar -x -C "$WORK/stage"
mkdir -p "$WORK/src"
mv "$WORK/stage" "$WORK/src/pbsgo-$VERSION"
tar -C "$WORK/src" -czf "$WORK/pbsgo-$VERSION.tar.gz" "pbsgo-$VERSION"
rmdir "$WORK/src" "$WORK/stage" 2>/dev/null || true

echo "==> preparing rpmbuild tree"
RPM=$WORK/rpmbuild
mkdir -p "$RPM"/BUILD "$RPM"/BUILDROOT "$RPM"/RPMS "$RPM"/SOURCES \
         "$RPM"/SPECS "$RPM"/SRPMS
cp "$WORK/pbsgo-$VERSION.tar.gz" "$RPM/SOURCES/"
sed "s/^Version:.*$/Version:       $VERSION/" "$SCRIPT_DIR/pbsgo.spec" \
    > "$RPM/SPECS/pbsgo.spec"

echo "==> building in container ($IMAGE)"
docker run --rm -i \
    -v "$RPM":/src:z \
    -v "$OUT_DIR":/out \
    "$IMAGE" \
    bash -s <<'DOCKER'
set -eux
# systemd: matches the spec's BuildRequires (unit dir macro + scriptlets);
# base images normally ship it, but $PBSGO_FEDORA_IMAGE overrides may not
dnf -y install golang gcc gtk3-devel webkit2gtk4.1-devel tar rpm-build systemd
export HOME=/root
cp -r /src /root/rpmbuild
rpmbuild --define "_topdir /root/rpmbuild" -ba /root/rpmbuild/SPECS/pbsgo.spec
mkdir -p /out
cp -v /root/rpmbuild/RPMS/*/*.rpm /out/
cp -v /root/rpmbuild/SRPMS/*.src.rpm /out/
DOCKER

echo "==> done:"
ls -la "$OUT_DIR"/pbsgo-*.rpm
