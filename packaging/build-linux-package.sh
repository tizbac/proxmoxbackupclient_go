#!/bin/sh
# build-linux-package.sh - build one Linux package of salvanza inside a
# standard container, the same way on a developer's machine and in GitHub
# Actions (.github/workflows/build-and-release.yml, job linux-packages).
#
# Usage: packaging/build-linux-package.sh <deb|rpm|arch> [output-dir]
#
#   deb   ubuntu:22.04, the oldest base supported (Ubuntu 22.04, Linux Mint
#         21, Debian 12 and newer): packaging/debian/build-deb.sh
#   rpm   fedora:44: rpmbuild with packaging/fedora/salvanza.spec
#   arch  archlinux: make pkgarch (packaging/arch/PKGBUILD); x86_64 only,
#         there is no official Arch Linux image for arm64
#
# Environment:
#   PBS_ARCH     amd64 or arm64 (default: the host's). A foreign architecture
#                runs under QEMU emulation (binfmt_misc), which works but is
#                several times slower; GitHub builds arm64 on arm64 runners.
#   PBS_VERSION  package version. Default: from a v* tag on HEAD (v2.0.0 gives
#                2.0.0, v2.0.0-rc.1 gives 2.0.0~rc.1, which sorts before 2.0.0),
#                else gui/wails.json productVersion with ~dev.<short sha>.
#                The Arch pkgver is always productVersion (the PKGBUILD checks
#                it); a build that is not productVersion itself gets the rest
#                in its file name instead (salvanza-2.0.0-1-x86_64-rc.1...).
#   PBS_GO       Go release series for the deb build (default 1.25): Ubuntu
#                22.04's own Go is too old, so the latest 1.25.x comes from
#                go.dev, checked against its published SHA-256.
#
# Needs only docker (and git, for the default version) on the host. The source
# is the working tree as it is; the committed frontend (gui/frontend/dist) is
# used, as by the other packaging scripts. Result: one package in output-dir
# (default dist/), owned by the calling user.
set -eu

KIND=${1:-}
case "$KIND" in
    deb | rpm | arch) : ;;
    *) echo "usage: $0 <deb|rpm|arch> [output-dir]" >&2; exit 2 ;;
esac

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
OUT_DIR=${2:-$ROOT/dist}
mkdir -p "$OUT_DIR"
OUT_DIR=$(cd "$OUT_DIR" && pwd)

host_arch() {
    case "$(uname -m)" in
        x86_64 | amd64) echo amd64 ;;
        aarch64 | arm64) echo arm64 ;;
        *) uname -m ;;
    esac
}
ARCH=${PBS_ARCH:-$(host_arch)}
case "$ARCH" in
    amd64 | arm64) : ;;
    *) echo "error: unsupported architecture: $ARCH (amd64 or arm64)" >&2; exit 2 ;;
esac
if [ "$KIND" = arch ] && [ "$ARCH" != amd64 ]; then
    echo "error: the Arch package is built for x86_64 only (no official Arch Linux arm64 image)" >&2
    exit 2
fi

BASE=$(sed -n 's/.*"productVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$ROOT/gui/wails.json" | head -n1)
[ -n "$BASE" ] || { echo "error: no productVersion in gui/wails.json" >&2; exit 1; }
SHA=$(git -C "$ROOT" rev-parse --short=7 HEAD 2>/dev/null || echo "")
VERSION=${PBS_VERSION:-}
if [ -z "$VERSION" ]; then
    TAG=$(git -C "$ROOT" tag --points-at HEAD 2>/dev/null | grep -E "^v$BASE([-+].*)?$" | head -n1 || true)
    if [ -n "$TAG" ]; then
        VERSION=$(echo "${TAG#v}" | tr '-' '~')
    elif [ -n "$SHA" ]; then
        VERSION="$BASE~dev.$SHA"
    else
        VERSION=$BASE
    fi
fi
case "$VERSION" in
    *-* | *[[:space:]]*) echo "error: version '$VERSION' has a hyphen or space (use ~ for a prerelease)" >&2; exit 2 ;;
esac
GO_SERIES=${PBS_GO:-1.25}

case "$KIND" in
    deb) IMAGE=ubuntu:22.04 ;;
    rpm) IMAGE=fedora:44 ;;
    arch) IMAGE=archlinux:latest ;;
esac

echo "==> $KIND for $ARCH, version $VERSION, in $IMAGE"

# The source goes in read-only; the build works on a copy inside the
# container, so nothing in the checkout changes (make pkgarch edits the
# PKGBUILD) and no root-owned files are left behind.
docker run --rm -i --platform "linux/$ARCH" \
    -v "$ROOT":/src:ro \
    -v "$OUT_DIR":/out \
    -e KIND="$KIND" -e VERSION="$VERSION" -e BASE="$BASE" -e SHA="$SHA" \
    -e GO_SERIES="$GO_SERIES" -e HOST_UID="$(id -u)" -e HOST_GID="$(id -g)" \
    "$IMAGE" sh -s <<'CONTAINER'
set -eu
copy_source() {
    mkdir -p "$1"
    tar -C /src --exclude=./.git --exclude=./dist --exclude=./gui/build/bin \
        --exclude=node_modules --exclude='./packaging/arch/src' --exclude='./packaging/arch/pkg' \
        --exclude='./packaging/arch/*.pkg.tar*' --exclude='./packaging/arch/*.tar.gz' \
        -cf - . | tar -C "$1" -xf -
}
# The package must bring the snapshot module installer and the source it builds.
check_installer() {
    n=$(printf '%s\n' "$1" | grep -cE 'usr/share/salvanza/elastio-snap/(install-elastio-snap\.sh|kernel7-strscpy\.patch|source/src/elastio-snap\.h)$' || true)
    echo "==> snapshot module installer in the package: $n of 3"
    [ "$n" = 3 ]
}

case "$KIND" in
deb)
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    apt-get install -y -qq --no-install-recommends ca-certificates curl jq gcc libc6-dev pkg-config \
        libgtk-3-dev libwebkit2gtk-4.1-dev dpkg-dev binutils file >/dev/null
    garch=$(dpkg --print-architecture)
    rel=$(curl -fsSL 'https://go.dev/dl/?mode=json&include=all' |
        jq -r --arg s "go$GO_SERIES." --arg a "$garch" '
            [.[] | select(.stable and (.version | startswith($s)))][0]
            | .files[] | select(.os == "linux" and .arch == $a and .kind == "archive")
            | "\(.filename) \(.sha256)"')
    [ -n "$rel" ] || { echo "error: no Go $GO_SERIES.x for linux-$garch on go.dev" >&2; exit 1; }
    set -- $rel
    curl -fsSL -o /tmp/go.tgz "https://go.dev/dl/$1"
    echo "$2  /tmp/go.tgz" | sha256sum -c - >/dev/null
    tar -C /usr/local -xzf /tmp/go.tgz
    export PATH=/usr/local/go/bin:$PATH GOFLAGS=-buildvcs=false GOTOOLCHAIN=local
    echo "==> $(go version)"
    copy_source /build
    cd /build
    PBS_VERSION="$VERSION" sh packaging/debian/build-deb.sh /tmp/pkg
    p=$(ls /tmp/pkg/*.deb)
    check_installer "$(dpkg-deb -c "$p" | awk '{print $NF}')"
    ;;
rpm)
    dnf -y -q install golang gcc gtk3-devel webkit2gtk4.1-devel rpm-build tar >/dev/null
    export GOFLAGS=-buildvcs=false
    echo "==> $(go version)"
    W=/root/rpmbuild
    mkdir -p "$W"/BUILD "$W"/BUILDROOT "$W"/RPMS "$W"/SOURCES "$W"/SPECS "$W"/SRPMS
    copy_source "/tmp/src/salvanza-$VERSION"
    tar -C /tmp/src -czf "$W/SOURCES/salvanza-$VERSION.tar.gz" "salvanza-$VERSION"
    sed "s/^Version:.*$/Version:       $VERSION/" "/tmp/src/salvanza-$VERSION/packaging/fedora/salvanza.spec" \
        > "$W/SPECS/salvanza.spec"
    rpmbuild --define "_topdir $W" -bb "$W/SPECS/salvanza.spec"
    mkdir -p /tmp/pkg
    cp "$W"/RPMS/*/salvanza-*.rpm /tmp/pkg/
    p=$(ls /tmp/pkg/*.rpm)
    check_installer "$(rpm -qlp "$p")"
    ;;
arch)
    pacman -Syu --noconfirm --needed -q base-devel go gtk3 webkit2gtk-4.1 >/dev/null
    useradd -m builder
    copy_source /build
    chown -R builder: /build
    echo "==> $(go version)"
    su builder -c 'cd /build && GOFLAGS=-buildvcs=false make pkgarch'
    mkdir -p /tmp/pkg
    for p in /build/packaging/arch/salvanza-*.pkg.tar.zst; do
        name=$(basename "$p" .pkg.tar.zst)
        # pkgver is productVersion; a dev or prerelease build says which it is
        # in the file name, as the .deb and .rpm do in their version.
        case "$VERSION" in "$BASE~"*) name="$name-${VERSION#"$BASE~"}" ;; esac
        cp "$p" "/tmp/pkg/$name.pkg.tar.zst"
    done
    p=$(ls /tmp/pkg/*.pkg.tar.zst)
    check_installer "$(tar -tf "$p")"
    ;;
esac
cp /tmp/pkg/* /out/
chown "$HOST_UID:$HOST_GID" /out/"$(basename "$p")"
echo "==> built: $(basename "$p")"
CONTAINER
