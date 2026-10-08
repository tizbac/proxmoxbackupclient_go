#!/bin/sh
# build-rpm.sh - build the pbsgo RPM inside a Fedora Docker container.
#
# Usage: packaging/fedora/build-rpm.sh [output-dir]     (default: dist/)
#
# Requirements: docker, plus network access from the container (dnf +
# Go module downloads). The host only needs a POSIX shell, tar and
# docker; Go, GTK and WebKit are installed inside the container.
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

# Read version from git tag (exact match) or git short SHA
# Falls back to wails.json if not in a git repo
if [ -f "$ROOT/scripts/get-version.sh" ]; then
    VERSION=$(bash "$ROOT/scripts/get-version.sh" "$ROOT")
else
    VERSION=$(sed -n 's/.*"productVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$ROOT/gui/wails.json")
fi
[ -n "$VERSION" ] || { echo "error: cannot determine version" >&2; exit 1; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT INT TERM

echo "==> staging source tree ($VERSION)"
# Built from the CURRENT working tree (uncommitted changes included) rather
# than a commit fetched from GitHub: what is packaged is what was built.
sh "$ROOT/packaging/stage-source.sh" "$WORK/pbsgo-$VERSION.tar.gz"

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
