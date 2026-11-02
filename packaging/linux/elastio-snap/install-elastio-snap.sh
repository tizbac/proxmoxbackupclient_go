#!/usr/bin/env bash
#
# install-elastio-snap.sh - build and install the elastio-snap snapshot module
# through DKMS, so machine backups of a running Linux computer can read each
# mounted partition through a snapshot.
#
# What it does:
#   1. Fetches Axcient/elastio-snap at a pinned, tested commit (or --ref).
#   2. Applies kernel7-strscpy.patch (Linux 7 removed strncpy). The patch is
#      skipped when the fetched version already contains it, or no longer calls
#      strncpy at all. If it neither applies nor is already there, the script
#      stops before changing anything, rather than build a half-patched module.
#   3. Installs the build tools and kernel headers (apt or dnf), puts the source
#      in /usr/src and builds it with DKMS, which rebuilds it for every new
#      kernel.
#   4. Loads the module now and at every start (/etc/modules-load.d).
#
# Safe to run again: an identical source already built for the running kernel
# is left alone. Debian, Ubuntu, Linux Mint, Fedora and the RHEL family.
# Needs root.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PATCH_FILE="$SCRIPT_DIR/kernel7-strscpy.patch"

DEFAULT_REPO="https://github.com/Axcient/elastio-snap.git"
# Axcient/elastio-snap branch develop, version 0.12.11. Tested with the patch
# on Fedora 44 (kernel 7.2) and Linux Mint 21.3 (kernel 5.15), November 2026.
PINNED_REF="971ba56a607022efc72cb3567c3cc29b2e4d10cf"
UPSTREAM_PR="https://github.com/Axcient/elastio-snap/pull/138"

MODULE="elastio-snap"
MODULE_SYS="elastio_snap"
LOAD_CONF="/etc/modules-load.d/elastio-snap.conf"

REPO="$DEFAULT_REPO"
REF="$PINNED_REF"
NO_PATCH=0
WITH_ELIOCTL=0

usage() {
  cat <<EOF
Usage: sudo $0 [options]

  --ref REF        elastio-snap commit, tag or branch to build
                   (default: the tested commit $PINNED_REF)
  --repo URL       git repository to fetch from (default: $DEFAULT_REPO)
  --no-patch       build the fetched source as it is, without the kernel 7 patch
                   (only for kernels before 7)
  --with-elioctl   also build the module's own tool, elioctl, into /usr/local/bin
  -h, --help       this help
EOF
  exit "${1:-1}"
}

say()  { printf '%s\n' "$*"; }
step() { printf '\n### %s\n' "$*"; }
die()  { printf 'error: %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --ref)          REF="${2:-}"; [ -n "$REF" ] || die "--ref needs a value"; shift 2 ;;
    --repo)         REPO="${2:-}"; [ -n "$REPO" ] || die "--repo needs a value"; shift 2 ;;
    --no-patch)     NO_PATCH=1; shift ;;
    --with-elioctl) WITH_ELIOCTL=1; shift ;;
    -h|--help)      usage 0 ;;
    *)              say "unknown option: $1" >&2; usage ;;
  esac
done

[ "$(id -u)" -eq 0 ] || die "run this as root, for example: sudo $0"
[ "$NO_PATCH" = "1" ] || [ -f "$PATCH_FILE" ] || die "patch not found: $PATCH_FILE"

KVER="$(uname -r)"

# ---------------------------------------------------------------- packages --

FAMILY=""
if [ -r /etc/os-release ]; then
  # shellcheck disable=SC1091
  . /etc/os-release
  case " ${ID:-} ${ID_LIKE:-} " in
    *" debian "*|*" ubuntu "*)                FAMILY=debian ;;
    *" fedora "*|*" rhel "*|*" centos "*)     FAMILY=fedora ;;
  esac
fi

APT_UPDATED=0
install_packages() {
  local missing=() p
  case "$FAMILY" in
    debian)
      for p in "$@"; do
        dpkg-query -W -f='${Status}' "$p" 2>/dev/null | grep -q 'ok installed' || missing+=("$p")
      done
      [ "${#missing[@]}" -eq 0 ] && return 0
      say "installing: ${missing[*]}"
      if [ "$APT_UPDATED" = "0" ]; then
        DEBIAN_FRONTEND=noninteractive apt-get -qq update
        APT_UPDATED=1
      fi
      DEBIAN_FRONTEND=noninteractive apt-get -y -qq install "${missing[@]}" > /dev/null
      ;;
    fedora)
      for p in "$@"; do
        rpm -q "$p" > /dev/null 2>&1 || missing+=("$p")
      done
      [ "${#missing[@]}" -eq 0 ] && return 0
      say "installing: ${missing[*]}"
      if command -v dnf > /dev/null 2>&1; then
        dnf -y -q install "${missing[@]}"
      else
        yum -y -q install "${missing[@]}"
      fi
      ;;
    *)
      for p in "$@"; do
        command -v "$p" > /dev/null 2>&1 || missing+=("$p")
      done
      [ "${#missing[@]}" -eq 0 ] && return 0
      die "this distribution is not Debian or Fedora based: install ${missing[*]} yourself, then run this again"
      ;;
  esac
}

# --------------------------------------------------------- 1. fetch source --

step "1/5 fetch elastio-snap ($REF)"
install_packages git patch

WORK="$(mktemp -d /tmp/elastio-snap-src.XXXXXX)"
trap 'rm -rf "$WORK"' EXIT
SRC="$WORK/elastio-snap"
git init -q "$SRC"
git -C "$SRC" remote add origin "$REPO"
if ! git -C "$SRC" fetch -q --depth 1 origin "$REF" 2> /dev/null; then
  # Some servers refuse a shallow fetch of a commit id: fetch everything.
  git -C "$SRC" fetch -q --tags origin '+refs/heads/*:refs/remotes/origin/*' \
    || die "cannot fetch $REPO"
  git -C "$SRC" rev-parse -q --verify "$REF^{commit}" > /dev/null 2>&1 \
    || git -C "$SRC" rev-parse -q --verify "origin/$REF^{commit}" > /dev/null 2>&1 \
    || die "$REF is not a commit, tag or branch of $REPO"
  if git -C "$SRC" rev-parse -q --verify "$REF^{commit}" > /dev/null 2>&1; then
    git -C "$SRC" -c advice.detachedHead=false checkout -q "$REF"
  else
    git -C "$SRC" -c advice.detachedHead=false checkout -q "origin/$REF"
  fi
else
  git -C "$SRC" -c advice.detachedHead=false checkout -q FETCH_HEAD
fi
COMMIT="$(git -C "$SRC" rev-parse HEAD)"
[ -f "$SRC/src/elastio-snap.h" ] && [ -f "$SRC/dist/elastio-snap-dkms-conf" ] \
  || die "$REF has no src/elastio-snap.h or dist/elastio-snap-dkms-conf: too old, or not elastio-snap"
VER="$(sed -n 's/.*ELASTIO_SNAP_VERSION "\([^"]*\)".*/\1/p' "$SRC/src/elastio-snap.h" | head -n1)"
[ -n "$VER" ] || die "no ELASTIO_SNAP_VERSION in src/elastio-snap.h: is this elastio-snap?"
say "commit $COMMIT, version $VER"

# ------------------------------------------------------ 2. kernel 7 patch --

step "2/5 kernel 7 patch"
uses_strncpy() { grep -q 'strncpy(' "$SRC"/src/*.c "$SRC"/src/*.h 2> /dev/null; }
PATCH_STATE=""
if [ "$NO_PATCH" = "1" ]; then
  PATCH_STATE="not applied (--no-patch)"
  if uses_strncpy; then
    say "warning: this source calls strncpy, so it will not build on kernel 7 or later"
  fi
elif patch -d "$SRC" -p1 -R -f -s --dry-run --fuzz=0 < "$PATCH_FILE" > /dev/null 2>&1; then
  PATCH_STATE="already in this version"
elif patch -d "$SRC" -p1 -N -f -s --dry-run --fuzz=0 < "$PATCH_FILE" > /dev/null 2>&1; then
  patch -d "$SRC" -p1 -N -f -s --fuzz=0 < "$PATCH_FILE" > /dev/null \
    || die "the patch passed its dry run but did not apply"
  uses_strncpy && die "strncpy is still used after the patch: this version needs a new patch"
  PATCH_STATE="applied"
elif ! uses_strncpy; then
  PATCH_STATE="not needed (this version no longer calls strncpy)"
else
  cat >&2 <<EOF
error: kernel7-strscpy.patch does not apply to elastio-snap $VER ($COMMIT),
and that version still calls strncpy, which kernel 7 removed.
Axcient changed the code the patch touches, so nothing was built or installed.
  - Run without --ref to build the tested version ($PINNED_REF).
  - Or check whether Axcient has released the fix: $UPSTREAM_PR
    If it has, this patch is no longer needed.
  - On a kernel before 7, --no-patch builds this version unpatched.
EOF
  exit 1
fi
say "patch: $PATCH_STATE"

# ------------------------------------------------- 3. build tools and DKMS --

step "3/5 build tools and kernel headers for $KVER"
case "$FAMILY" in
  debian)
    pkgs=(build-essential dkms "linux-headers-$KVER")
    # The headers meta-package brings the headers of each new kernel, which
    # DKMS needs to rebuild the module after an update.
    if dpkg-query -W -f='${Status}' linux-image-generic 2>/dev/null | grep -q 'ok installed'; then
      pkgs+=(linux-headers-generic)
    elif dpkg-query -W -f='${Status}' linux-image-amd64 2>/dev/null | grep -q 'ok installed'; then
      pkgs+=(linux-headers-amd64)
    fi
    install_packages "${pkgs[@]}"
    ;;
  fedora)
    pkgs=(gcc make dkms kernel-devel "kernel-devel-$KVER")
    install_packages "${pkgs[@]}" || die "could not install ${pkgs[*]} (RHEL family: DKMS comes from EPEL; an old running kernel: update and restart first)"
    ;;
  *)
    install_packages make gcc dkms
    ;;
esac
[ -d "/lib/modules/$KVER/build" ] || die "no kernel headers for $KVER in /lib/modules/$KVER/build"

# The source as DKMS will see it, with a note of where it came from.
STAGE="$WORK/stage"
cp -a "$SRC/src" "$STAGE"
sed "s/@MODULE_VERSION@/$VER/" "$SRC/dist/elastio-snap-dkms-conf" > "$STAGE/dkms.conf"
cat > "$STAGE/SOURCE.txt" <<EOF
Installed by install-elastio-snap.sh
repository: $REPO
commit: $COMMIT
kernel 7 patch: $PATCH_STATE
EOF

step "4/5 DKMS: $MODULE/$VER for $KVER"
DKMS_SRC="/usr/src/$MODULE-$VER"
dkms_state() { dkms status "$MODULE/$VER" -k "$KVER" 2> /dev/null | grep -E "^${MODULE}[/,]" || true; }

# Other elastio-snap versions in DKMS would fight over the same module file.
while read -r other; do
  [ -n "$other" ] && [ "$other" != "$VER" ] || continue
  say "removing $MODULE/$other from DKMS"
  dkms remove "$MODULE/$other" --all > /dev/null || die "dkms remove $MODULE/$other failed"
done < <(dkms status 2> /dev/null | sed -n "s|^${MODULE}[/,] *\([^,:]*\).*|\1|p" | sort -u)

OTHER_KERNELS=()
if [ -d "$DKMS_SRC" ] && diff -r -q "$STAGE" "$DKMS_SRC" > /dev/null 2>&1; then
  say "$DKMS_SRC already holds this source"
else
  if [ -n "$(dkms status "$MODULE/$VER" 2> /dev/null)" ]; then
    # Rebuild the new source for the other kernels it was installed for.
    mapfile -t OTHER_KERNELS < <(dkms status "$MODULE/$VER" 2> /dev/null \
      | sed -n "s|^${MODULE}[/,] *[^,]*, *\([^,]*\),.*: installed.*|\1|p" | grep -vxF "$KVER" || true)
    say "replacing the different $MODULE/$VER source in DKMS"
    dkms remove "$MODULE/$VER" --all > /dev/null || die "dkms remove $MODULE/$VER failed"
  fi
  rm -rf "$DKMS_SRC"
  cp -a "$STAGE" "$DKMS_SRC"
fi

if dkms_state | grep -q 'installed'; then
  say "already built and installed for $KVER"
else
  [ -n "$(dkms status "$MODULE/$VER" 2> /dev/null)" ] || dkms add "$MODULE/$VER" > /dev/null
  if ! dkms install "$MODULE/$VER" -k "$KVER"; then
    die "the DKMS build failed: see /var/lib/dkms/$MODULE/$VER/build/make.log"
  fi
fi
for k in ${OTHER_KERNELS[@]+"${OTHER_KERNELS[@]}"}; do
  [ -d "/lib/modules/$k/build" ] || continue
  say "rebuilding for the installed kernel $k"
  dkms install "$MODULE/$VER" -k "$k" > /dev/null 2>&1 || say "warning: the build for $k failed; DKMS tries again when that kernel starts"
done
dkms status "$MODULE/$VER"

# A copy installed outside DKMS (make install) can shadow the DKMS one.
mapfile -t copies < <(find "/lib/modules/$KVER" -name "$MODULE.ko*" 2> /dev/null)
if [ "${#copies[@]}" -gt 1 ]; then
  say "warning: more than one $MODULE module for $KVER:"
  printf '  %s\n' "${copies[@]}"
  say "remove the copy not made by DKMS (sudo make uninstall in its source folder, then sudo depmod -a)"
fi

if [ "$WITH_ELIOCTL" = "1" ]; then
  say "building elioctl"
  make -C "$SRC" application-static > "$WORK/elioctl.log" 2>&1 \
    || { tail -n 20 "$WORK/elioctl.log" >&2; die "building elioctl failed"; }
  install -m 0755 "$SRC/app/elioctl" /usr/local/bin/elioctl
  say "installed /usr/local/bin/elioctl"
fi

# ------------------------------------------------------------ 5. load it --

step "5/5 load now and at every start"
if [ "$(cat "$LOAD_CONF" 2> /dev/null)" != "$MODULE" ]; then
  printf '%s\n' "$MODULE" > "$LOAD_CONF"
  say "wrote $LOAD_CONF"
else
  say "$LOAD_CONF already there"
fi

NEW_SRCVER="$(modinfo -F srcversion "$MODULE" 2> /dev/null || true)"
if [ -d "/sys/module/$MODULE_SYS" ]; then
  OLD_SRCVER="$(cat "/sys/module/$MODULE_SYS/srcversion" 2> /dev/null || true)"
  if [ -n "$NEW_SRCVER" ] && [ "$OLD_SRCVER" = "$NEW_SRCVER" ]; then
    say "already loaded (this build)"
  elif grep -q '"minor"' /proc/elastio-snap-info 2> /dev/null; then
    say "warning: an older build is loaded and has snapshots in use; it was left loaded."
    say "The new build loads at the next start (or after the snapshots end: sudo rmmod $MODULE_SYS; sudo modprobe $MODULE)."
  else
    rmmod "$MODULE_SYS" || die "could not unload the older build (rmmod $MODULE_SYS)"
    modprobe "$MODULE" || die "modprobe $MODULE failed (Secure Boot? see the GUI manual, Secure Boot)"
    say "reloaded with the new build"
  fi
else
  modprobe "$MODULE" || die "modprobe $MODULE failed (Secure Boot? see the GUI manual, Secure Boot)"
  say "loaded"
fi
[ -e /dev/elastio-snap-ctl ] || die "/dev/elastio-snap-ctl is missing after loading the module"

cat <<EOF

OK: elastio-snap $VER is installed through DKMS, loaded, and loads at every start.
Check it with:
  lsmod | grep $MODULE_SYS
  dkms status
  cat /proc/elastio-snap-info
Then close the backup app and start it again: the machine backup screen
should say "Snapshot module loaded: elastio-snap".
EOF
