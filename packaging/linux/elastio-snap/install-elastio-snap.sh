#!/usr/bin/env bash
#
# install-elastio-snap.sh - build and install the elastio-snap snapshot module
# through DKMS, so machine backups of a running Linux computer can read each
# mounted partition through a snapshot.
#
# What it does:
#   1. Takes the elastio-snap source bundled beside it (source/, Axcient's
#      pinned, tested commit), or fetches another version with --ref/--repo.
#   2. Applies kernel7-strscpy.patch (Linux 7 removed strncpy). The patch is
#      skipped when the source already contains it, or no longer calls
#      strncpy at all. If it neither applies nor is already there, the script
#      stops before changing anything, rather than build a half-patched module.
#   3. Installs the build tools and kernel headers (apt, dnf or pacman), puts the source
#      in /usr/src and builds it with DKMS for the running kernel and every
#      other installed kernel with headers; DKMS rebuilds it for every new
#      kernel. DKMS runs without a terminal, so Ubuntu's own Secure Boot
#      prompt during the build never waits for an answer.
#   4. With Secure Boot on: checks that the key DKMS signed the module with is
#      enrolled, and if not, asks the firmware to enrol it at the next start
#      (mokutil, with a one-time password the user types again then).
#   5. Loads the module now and at every start (/etc/modules-load.d).
#
# The app's "Install it for me" and "salvanza-cli install-snapshot-module" run
# this script. Exit codes: 0 installed and loaded; 10 installed, but the
# computer must restart so the user can approve the signing key (Secure Boot);
# anything else, failed (the output says why).
#
# Safe to run again: an identical source already built for the running kernel
# is left alone. Debian, Ubuntu, Linux Mint, Fedora, the RHEL family, and
# Arch Linux and its derivatives. Needs root.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PATCH_FILE="$SCRIPT_DIR/kernel7-strscpy.patch"
BUNDLED_SRC="$SCRIPT_DIR/source"

DEFAULT_REPO="https://github.com/Axcient/elastio-snap.git"
# Axcient/elastio-snap branch develop, version 0.12.11, the commit bundled in
# source/. Tested with the patch on Fedora 44 (kernel 7.2) and Linux Mint 21.3
# (kernel 5.15), November 2026.
PINNED_REF="971ba56a607022efc72cb3567c3cc29b2e4d10cf"
UPSTREAM_PR="https://github.com/Axcient/elastio-snap/pull/138"

MODULE="elastio-snap"
MODULE_SYS="elastio_snap"
LOAD_CONF="/etc/modules-load.d/elastio-snap.conf"
# Left while a key waits for approval at the next start; /run is emptied by
# the restart, so the app stops saying "restart to finish" once it has.
MOK_PENDING="/run/elastio-snap-mok-pending"
EXIT_NEEDS_RESTART=10

REPO=""
REF=""
NO_PATCH=0
WITH_ELIOCTL=0
MOK_PASSWORD_STDIN=0

usage() {
  cat <<EOF
Usage: sudo $0 [options]

  --mok-password-stdin  with Secure Boot on, read the one-time password for
                   enrolling the signing key from the first line of standard
                   input (8 to 16 characters); without it the password is
                   asked for on the terminal
  --ref REF        fetch and build this elastio-snap commit, tag or branch
                   instead of the bundled source ($PINNED_REF)
  --repo URL       git repository to fetch from (default: $DEFAULT_REPO)
  --no-patch       build the source as it is, without the kernel 7 patch
                   (only for kernels before 7)
  --with-elioctl   also build the module's own tool, elioctl, into /usr/local/bin
                   (with --ref only: the bundled source has the module alone)
  -h, --help       this help
EOF
  exit "${1:-1}"
}

say()  { printf '%s\n' "$*"; }
step() { printf '\n### %s\n' "$*"; }
die()  { printf 'error: %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --mok-password-stdin) MOK_PASSWORD_STDIN=1; shift ;;
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

# The bundled source unless another version was asked for (or it is missing,
# in a copy of the script taken on its own).
FETCH=0
if [ -n "$REF" ] || [ -n "$REPO" ] || [ ! -f "$BUNDLED_SRC/src/elastio-snap.h" ]; then
  FETCH=1
  REPO="${REPO:-$DEFAULT_REPO}"
  REF="${REF:-$PINNED_REF}"
fi
[ "$WITH_ELIOCTL" = "0" ] || [ "$FETCH" = "1" ] \
  || die "--with-elioctl needs --ref: the bundled source has the module alone"

# The password is read before anything is built, so a run from the app never
# waits on standard input later.
MOK_PW=""
if [ "$MOK_PASSWORD_STDIN" = "1" ]; then
  IFS= read -r MOK_PW || true
  case "${#MOK_PW}" in
    8|9|10|11|12|13|14|15|16) ;;
    *) die "the one-time password must be 8 to 16 characters" ;;
  esac
fi

KVER="$(uname -r)"

# ---------------------------------------------------------------- packages --

FAMILY=""
if [ -r /etc/os-release ]; then
  # shellcheck disable=SC1091
  . /etc/os-release
  case " ${ID:-} ${ID_LIKE:-} " in
    *" debian "*|*" ubuntu "*)                FAMILY=debian ;;
    *" fedora "*|*" rhel "*|*" centos "*)     FAMILY=fedora ;;
    *" arch "*)                               FAMILY=arch ;;
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
      # Standard input stays out of it: a package script that asks something
      # (shim's Secure Boot prompt) gets no answer and moves on.
      if [ "$APT_UPDATED" = "0" ]; then
        DEBIAN_FRONTEND=noninteractive apt-get -qq update < /dev/null
        APT_UPDATED=1
      fi
      DEBIAN_FRONTEND=noninteractive apt-get -y -qq install "${missing[@]}" > /dev/null < /dev/null
      ;;
    fedora)
      for p in "$@"; do
        rpm -q "$p" > /dev/null 2>&1 || missing+=("$p")
      done
      [ "${#missing[@]}" -eq 0 ] && return 0
      say "installing: ${missing[*]}"
      if command -v dnf > /dev/null 2>&1; then
        dnf -y -q install "${missing[@]}" < /dev/null
      else
        yum -y -q install "${missing[@]}" < /dev/null
      fi
      ;;
    arch)
      for p in "$@"; do
        pacman -Q "$p" > /dev/null 2>&1 || missing+=("$p")
      done
      [ "${#missing[@]}" -eq 0 ] && return 0
      say "installing: ${missing[*]}"
      # No -Sy: Arch does not support refreshing the package lists without
      # updating the system, so a stale list is the user's update to make.
      pacman -S --needed --noconfirm "${missing[@]}" < /dev/null \
        || die "pacman could not install ${missing[*]}: update the system (sudo pacman -Syu), restart, then run this again"
      ;;
    *)
      for p in "$@"; do
        command -v "$p" > /dev/null 2>&1 || missing+=("$p")
      done
      [ "${#missing[@]}" -eq 0 ] && return 0
      die "this distribution is not Debian, Fedora or Arch based: install ${missing[*]} yourself, then run this again"
      ;;
  esac
}

# secure_boot_on: the firmware's SecureBoot variable is 1 (its fifth byte;
# the first four are the variable's attributes).
secure_boot_on() {
  local var
  var="$(ls /sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c 2> /dev/null || true)"
  [ -n "$var" ] || return 1
  [ "$(od -An -t u1 -j 4 -N 1 "$var" 2> /dev/null | tr -d ' ')" = "1" ]
}

# started_by_shim: the computer started through shim, whose key list
# (MokListRT) is what mokutil enrols into. Without shim (Secure Boot with the
# owner's own keys, as sbctl sets up on Arch) an enrolment request is never
# read, and the restart would come back without the module.
started_by_shim() {
  [ -e /sys/firmware/efi/mok-variables/MokListRT ] \
    || [ -n "$(ls /sys/firmware/efi/efivars/MokListRT-605dab50-e046-4300-abb6-3dd810dd8b23 2> /dev/null || true)" ]
}

# ---------------------------------------------------------------- 1. source --

WORK="$(mktemp -d /tmp/elastio-snap-src.XXXXXX)"
trap 'rm -rf "$WORK"' EXIT
SRC="$WORK/elastio-snap"

if [ "$FETCH" = "0" ]; then
  step "1/6 elastio-snap source (bundled)"
  install_packages patch
  cp -a "$BUNDLED_SRC" "$SRC"
  COMMIT="$PINNED_REF"
  REPO_NOTE="bundled with this installer ($DEFAULT_REPO)"
else
  step "1/6 fetch elastio-snap ($REF)"
  install_packages git patch
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
  REPO_NOTE="$REPO"
fi
[ -f "$SRC/src/elastio-snap.h" ] && [ -f "$SRC/dist/elastio-snap-dkms-conf" ] \
  || die "$COMMIT has no src/elastio-snap.h or dist/elastio-snap-dkms-conf: too old, or not elastio-snap"
VER="$(sed -n 's/.*ELASTIO_SNAP_VERSION "\([^"]*\)".*/\1/p' "$SRC/src/elastio-snap.h" | head -n1)"
[ -n "$VER" ] || die "no ELASTIO_SNAP_VERSION in src/elastio-snap.h: is this elastio-snap?"
say "commit $COMMIT, version $VER"

# ------------------------------------------------------ 2. kernel 7 patch --

step "2/6 kernel 7 patch"
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
  - Run without --ref to build the bundled, tested version ($PINNED_REF).
  - Or check whether Axcient has released the fix: $UPSTREAM_PR
    If it has, this patch is no longer needed.
  - On a kernel before 7, --no-patch builds this version unpatched.
EOF
  exit 1
fi
say "patch: $PATCH_STATE"

# ------------------------------------------------- 3. build tools and DKMS --

step "3/6 build tools and kernel headers for $KVER"
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
  arch)
    # Each kernel package (linux, linux-lts, linux-zen, Manjaro's linux612 ...)
    # has its headers in <name>-headers. Those of every installed kernel, so
    # DKMS's pacman hook rebuilds the module whichever kernel is updated.
    [ -e "/usr/lib/modules/$KVER/vmlinuz" ] \
      || die "the running kernel ($KVER) is no longer installed, as after a system update: restart, then run this again"
    pkgs=(gcc make dkms)
    for vmlinuz in /usr/lib/modules/*/vmlinuz; do
      kpkg="$(pacman -Qqo "$vmlinuz" 2> /dev/null || true)"
      [ -n "$kpkg" ] || continue
      case " ${pkgs[*]} " in *" $kpkg-headers "*) ;; *) pkgs+=("$kpkg-headers") ;; esac
    done
    install_packages "${pkgs[@]}"
    ;;
  *)
    install_packages make gcc dkms
    ;;
esac
if [ ! -d "/lib/modules/$KVER/build" ]; then
  [ "$FAMILY" != "arch" ] || die "the headers installed are not those of the running kernel ($KVER): update the system (sudo pacman -Syu), restart, then run this again"
  die "no kernel headers for $KVER in /lib/modules/$KVER/build"
fi

SECURE_BOOT=0
if secure_boot_on; then
  SECURE_BOOT=1
  say "Secure Boot is on: the module must be signed with an enrolled key"
  started_by_shim || die "Secure Boot is on, but this computer does not start through shim, so mokutil cannot enrol a key: sign the module with your own Secure Boot key, or turn Secure Boot off (see the GUI manual, Secure Boot)"
  install_packages mokutil openssl
fi

# The source as DKMS will see it, with a note of where it came from.
STAGE="$WORK/stage"
cp -a "$SRC/src" "$STAGE"
# DKMS runs it as PRE_BUILD; a copy made on Windows can lose its mode.
chmod 0755 "$STAGE/genconfig.sh"
sed "s/@MODULE_VERSION@/$VER/" "$SRC/dist/elastio-snap-dkms-conf" > "$STAGE/dkms.conf"
cat > "$STAGE/SOURCE.txt" <<EOF
Installed by install-elastio-snap.sh
repository: $REPO_NOTE
commit: $COMMIT
kernel 7 patch: $PATCH_STATE
EOF

step "4/6 DKMS: $MODULE/$VER for $KVER"
DKMS_SRC="/usr/src/$MODULE-$VER"
# DKMS without a terminal. On Ubuntu, "dkms install" with Secure Boot on runs
# update-secureboot-policy --enroll-key, which asks for its own Secure Boot
# password through debconf: run from the app, with no terminal, it waited for
# ever (found 2026-11-06 on Ubuntu 24.04). Without one it leaves the key
# alone, and step 5 enrols it with the user's password.
dkms_q() { DEBIAN_FRONTEND=noninteractive dkms "$@" < /dev/null; }
dkms_state() { dkms status "$MODULE/$VER" -k "$KVER" 2> /dev/null | grep -E "^${MODULE}[/,]" || true; }

# Other elastio-snap versions in DKMS would fight over the same module file.
while read -r other; do
  [ -n "$other" ] && [ "$other" != "$VER" ] || continue
  say "removing $MODULE/$other from DKMS"
  dkms_q remove "$MODULE/$other" --all > /dev/null || die "dkms remove $MODULE/$other failed"
done < <(dkms status 2> /dev/null | sed -n "s|^${MODULE}[/,] *\([^,:]*\).*|\1|p" | sort -u)

if [ -d "$DKMS_SRC" ] && diff -r -q "$STAGE" "$DKMS_SRC" > /dev/null 2>&1; then
  say "$DKMS_SRC already holds this source"
else
  if [ -n "$(dkms status "$MODULE/$VER" 2> /dev/null)" ]; then
    say "replacing the different $MODULE/$VER source in DKMS"
    dkms_q remove "$MODULE/$VER" --all > /dev/null || die "dkms remove $MODULE/$VER failed"
  fi
  rm -rf "$DKMS_SRC"
  cp -a "$STAGE" "$DKMS_SRC"
fi

if dkms_state | grep -q 'installed'; then
  say "already built and installed for $KVER"
else
  [ -n "$(dkms status "$MODULE/$VER" 2> /dev/null)" ] || dkms_q add "$MODULE/$VER" > /dev/null
  if ! dkms_q install "$MODULE/$VER" -k "$KVER"; then
    die "the DKMS build failed: see /var/lib/dkms/$MODULE/$VER/build/make.log"
  fi
fi
# Every other installed kernel with its headers gets the module too: an
# update may have installed a newer kernel that has not started yet, and the
# next start (which Secure Boot needs anyway) would then come up without the
# module (found 2026-11-06 on Ubuntu 24.04: 6.8.0-149 installed in the
# background, the restart started it, no elastio-snap).
for kdir in /lib/modules/*/build; do
  k="$(basename "$(dirname "$kdir")")"
  [ "$k" != "$KVER" ] && [ -d "$kdir" ] || continue
  dkms status "$MODULE/$VER" -k "$k" 2> /dev/null | grep -q 'installed' && continue
  say "building it for the installed kernel $k too"
  dkms_q install "$MODULE/$VER" -k "$k" > /dev/null 2>&1 \
    || say "warning: the build for $k failed (/var/lib/dkms/$MODULE/$VER/build/make.log); after starting $k, run this again"
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

# ----------------------------------------------------------- 5. Secure Boot --

# hexid: an id as plain upper-case hex, without colons, spaces or leading
# zeros, so a module's sig_key and a certificate's serial or key id compare.
hexid() { tr -d ': \t\n' | tr '[:lower:]' '[:upper:]' | sed 's/^0*//'; }

# signing_cert: the certificate of the key DKMS signed the module with, found
# among the places DKMS keeps them (its own setting, Ubuntu's shim-signed key,
# DKMS's default) by matching the module's sig_key against each one's serial
# number and key id.
signing_cert() {
  local ko sig c conf cands=()
  ko="$(modinfo -k "$KVER" -n "$MODULE" 2> /dev/null || true)"
  [ -n "$ko" ] || return 1
  sig="$(modinfo -F sig_key "$ko" 2> /dev/null | hexid)"
  [ -n "$sig" ] || return 2
  for conf in /etc/dkms/framework.conf /etc/dkms/framework.conf.d/*.conf; do
    [ -r "$conf" ] || continue
    c="$(sed -n 's/^[[:space:]]*mok_certificate=["]\{0,1\}\([^"]*\)["]\{0,1\}[[:space:]]*$/\1/p' "$conf" | tail -n1)"
    [ -n "$c" ] && cands+=("$c")
  done
  cands+=(/var/lib/shim-signed/mok/MOK.der /var/lib/dkms/mok.pub)
  for c in "${cands[@]}"; do
    [ -f "$c" ] || continue
    if [ "$(openssl x509 -inform DER -in "$c" -noout -serial 2> /dev/null | sed 's/^serial=//' | hexid)" = "$sig" ] \
      || [ "$(openssl x509 -inform DER -in "$c" -noout -ext subjectKeyIdentifier 2> /dev/null | sed 1d | hexid)" = "$sig" ]; then
      printf '%s\n' "$c"
      return 0
    fi
  done
  return 3
}

step "5/6 Secure Boot"
NEEDS_RESTART=0
if [ "$SECURE_BOOT" = "0" ]; then
  say "Secure Boot is off: nothing to do"
  rm -f "$MOK_PENDING"
else
  set +e
  CERT="$(signing_cert)"
  rc=$?
  set -e
  case "$rc" in
    0) say "the module is signed with $CERT" ;;
    1) die "modinfo cannot find the $MODULE module just built for $KVER" ;;
    2) die "the module is not signed, so Secure Boot will refuse it. This version of DKMS does not sign modules: update the system (DKMS 3 signs them), or see the GUI manual, Secure Boot." ;;
    *) die "cannot find the key the module was signed with (see the GUI manual, Secure Boot)" ;;
  esac
  KEY_STATE="$(mokutil --test-key "$CERT" 2>&1 || true)"
  if printf '%s' "$KEY_STATE" | grep -q 'already enrolled'; then
    say "the key is enrolled"
    rm -f "$MOK_PENDING"
  else
    if printf '%s' "$KEY_STATE" | grep -q 'enrollment request'; then
      if [ -z "$MOK_PW" ] && [ ! -t 0 ]; then
        say "the key is already waiting to be enrolled at the next start"
        NEEDS_RESTART=1
      else
        # A new password replaces the one asked for before, which may be
        # forgotten by now.
        say "the key is already waiting to be enrolled: asking again with the new password"
        mokutil --revoke-import > /dev/null 2>&1 || true
      fi
    fi
    if [ "$NEEDS_RESTART" = "0" ]; then
      if [ -n "$MOK_PW" ]; then
        printf '%s\n%s\n' "$MOK_PW" "$MOK_PW" | mokutil --import "$CERT" > /dev/null \
          || die "mokutil could not ask the firmware to enrol $CERT"
      elif [ -t 0 ]; then
        say "Choose a one-time password (8 to 16 letters and digits). You type it once more"
        say "after the restart, to approve the key."
        mokutil --import "$CERT" || die "mokutil could not ask the firmware to enrol $CERT"
      else
        die "Secure Boot is on and the signing key is not enrolled: run this on a terminal, or give a one-time password with --mok-password-stdin"
      fi
      say "the firmware enrols the key at the next start, once approved"
      NEEDS_RESTART=1
    fi
    : > "$MOK_PENDING"
  fi
fi

# ------------------------------------------------------------ 6. load it --

step "6/6 load now and at every start"
if [ "$(cat "$LOAD_CONF" 2> /dev/null)" != "$MODULE" ]; then
  printf '%s\n' "$MODULE" > "$LOAD_CONF"
  say "wrote $LOAD_CONF"
else
  say "$LOAD_CONF already there"
fi

if [ "$NEEDS_RESTART" = "1" ]; then
  cat <<EOF

Nearly done: elastio-snap $VER is built and installed through DKMS, but
Secure Boot loads it only once its signing key is approved. To approve it:
  1. Restart the computer.
  2. A blue screen, "Perform MOK management", appears: press a key within
     10 seconds (otherwise the computer starts as usual and nothing is
     enrolled; run this again to ask again).
  3. Choose "Enroll MOK", then "Continue", then "Yes".
  4. Type the one-time password, then choose "Reboot".
The module then loads at every start.
EOF
  exit "$EXIT_NEEDS_RESTART"
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
The app's machine backup screen says "Snapshot module loaded: elastio-snap"
(start the app again if it was open while this ran).
EOF
