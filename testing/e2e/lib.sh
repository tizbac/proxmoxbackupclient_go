#!/usr/bin/env bash
# Shared helpers for the end-to-end test suite.
# Sourced by run.sh -- not meant to be executed directly.
# shellcheck shell=bash

# ---------------------------------------------------------------------------
# Configuration (every value overridable from the environment)
# ---------------------------------------------------------------------------
PBS_IMAGE="${PBS_IMAGE:-ayufan/proxmox-backup-server:latest}"
PBS_CONTAINER="${PBS_CONTAINER:-pbs-e2e}"
PBS_PORT="${PBS_PORT:-8007}"
PBS_BASEURL="https://127.0.0.1:${PBS_PORT}"
PBS_DATASTORE="${PBS_DATASTORE:-e2e}"
PBS_USER="${PBS_USER:-e2e@pbs}"
PBS_TOKEN_NAME="${PBS_TOKEN_NAME:-e2etoken}"
PBS_AUTHID="${PBS_USER}!${PBS_TOKEN_NAME}"

# Namespace used by the namespace round-trip test. PBS namespaces are a
# server-side object: they must exist before a backup may name them.
PBS_NAMESPACE="${PBS_NAMESPACE:-e2e}"

# Official Proxmox client.
#
# By default the suite *builds* the client from Proxmox's own Debian repository
# (see testing/e2e/official-client/Dockerfile) and runs it from a container.
# Setting PBC_BIN explicitly overrides that and uses a local binary instead,
# which is handy when testing a locally built client.
if [ -n "${PBC_BIN:-}" ]; then
	PBC_BIN_EXPLICIT=1
else
	PBC_BIN="proxmox-backup-client"
	PBC_BIN_EXPLICIT=""
fi
PBC_IMAGE="${PBC_IMAGE:-proxmoxbackupclient-e2e-client:latest}"

# Set to 1 to skip the "restore with the official client" assertions.
E2E_SKIP_OFFICIAL="${E2E_SKIP_OFFICIAL:-0}"
# Set to 1 to keep the workdir and the container for post-mortem inspection.
E2E_KEEP="${E2E_KEEP:-0}"
# Skip `make cli` and reuse whatever is already in dist/.
E2E_SKIP_BUILD="${E2E_SKIP_BUILD:-0}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK="${WORK:-$(mktemp -d -t pbs-e2e-XXXXXX)}"

# Filled in by start_pbs / configure_pbs.
PBS_FINGERPRINT=""
PBS_TOKEN_SECRET=""

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------
if [ -t 1 ]; then
	C_RESET=$'\033[0m'; C_RED=$'\033[31m'; C_GREEN=$'\033[32m'
	C_YELLOW=$'\033[33m'; C_BLUE=$'\033[34m'; C_BOLD=$'\033[1m'
else
	C_RESET=""; C_RED=""; C_GREEN=""; C_YELLOW=""; C_BLUE=""; C_BOLD=""
fi

log()  { printf '%s[e2e]%s %s\n' "$C_BLUE" "$C_RESET" "$*"; }
ok()   { printf '%s  PASS%s  %s\n' "$C_GREEN" "$C_RESET" "$*"; }
warn() { printf '%s  WARN%s  %s\n' "$C_YELLOW" "$C_RESET" "$*" >&2; }
die()  { printf '%s  FAIL%s  %s\n' "$C_RED" "$C_RESET" "$*" >&2; exit 1; }

section() {
	printf '\n%s%s== %s ==%s\n' "$C_BOLD" "$C_BLUE" "$*" "$C_RESET"
}

step() {
	printf '%s->%s %s\n' "$C_BLUE" "$C_RESET" "$*"
}

# ---------------------------------------------------------------------------
# Assertions
# ---------------------------------------------------------------------------
assert_eq() {
	local want="$1" got="$2" what="$3"
	[ "$want" = "$got" ] || die "$what: expected '$want', got '$got'"
	return 0
}

assert_file_exists() {
	[ -f "$1" ] || die "expected file '$1' to exist"
	return 0
}

assert_dir_exists() {
	[ -d "$1" ] || die "expected directory '$1' to exist"
	return 0
}

# Sorted "<sha256>  <relpath>" listing of every regular file under $1.
tree_manifest() {
	( cd "$1" && find . -type f -print0 | LC_ALL=C sort -z | xargs -0 -r sha256sum )
}

# Sorted relative listing of every directory under $1 (excludes the root).
tree_dirs() {
	( cd "$1" && find . -mindepth 1 -type d -print | LC_ALL=C sort )
}

# assert_tree_identical <dir-a> <dir-b> <label>
assert_tree_identical() {
	local a="$1" b="$2" label="$3"
	assert_dir_exists "$a"
	assert_dir_exists "$b"

	local d_manifest d_dirs
	d_manifest="$(mktemp)"; d_dirs="$(mktemp)"
	if ! diff -u <(tree_manifest "$a") <(tree_manifest "$b") >"$d_manifest"; then
		printf '%s' "$d_manifest" >&2
		rm -f "$d_manifest" "$d_dirs"
		die "$label: restored files differ from the source"
	fi
	if ! diff -u <(tree_dirs "$a") <(tree_dirs "$b") >"$d_dirs"; then
		printf '%s' "$d_dirs" >&2
		rm -f "$d_manifest" "$d_dirs"
		die "$label: restored directory tree differs from the source"
	fi
	rm -f "$d_manifest" "$d_dirs"
	ok "$label: $(tree_manifest "$a" | wc -l) file(s) byte-identical, tree layout identical"
	return 0
}

# assert_bytes_identical <file-a> <file-b> <label>
assert_bytes_identical() {
	local a="$1" b="$2" label="$3"
	assert_file_exists "$a"
	assert_file_exists "$b"
	local sa sb
	sa="$(sha256sum "$a" | cut -d' ' -f1)"
	sb="$(sha256sum "$b" | cut -d' ' -f1)"
	[ "$sa" = "$sb" ] || die "$label: sha256 mismatch (source=$sa restored=$sb)"
	ok "$label: byte-identical ($(stat -c%s "$a") bytes, sha256=${sa:0:16}...)"
	return 0
}

# ---------------------------------------------------------------------------
# PBS server control
# ---------------------------------------------------------------------------
pbs_mgr() {
	docker exec "$PBS_CONTAINER" proxmox-backup-manager "$@"
}

# Poll an arbitrary command until it succeeds or the timeout expires.
retry() {
	local timeout="$1"; shift
	local waited=0
	until "$@" >/dev/null 2>&1; do
		if [ "$waited" -ge "$timeout" ]; then
			return 1
		fi
		sleep 2
		waited=$((waited + 2))
	done
	return 0
}

pbs_ready() {
	pbs_mgr version >/dev/null 2>&1
}

start_pbs() {
	section "Starting PBS container"

	if docker ps -a --format '{{.Names}}' | grep -qx "$PBS_CONTAINER" \
	   || docker volume ls --format '{{.Name}}' | grep -qx "${PBS_CONTAINER}-data"; then
		log "removing pre-existing container/volumes for '$PBS_CONTAINER'"
		remove_pbs
	fi

	log "pulling $PBS_IMAGE"
	docker pull -q "$PBS_IMAGE" >/dev/null || die "cannot pull $PBS_IMAGE"

	# NOTE: there is no official Proxmox PBS image; PBS deliberately does not
	# ship one. ayufan/proxmox-backup-server is the community image that Proxmox
	# staff point at on their forum. /run must be a tmpfs or the API server
	# refuses to start.
	#
	# The datastore/config/log trees are *named volumes* rather than host
	# bind mounts: the container runs as root and would leave root-owned files
	# behind that the (possibly non-root) invoking user cannot clean up.
	docker run -d \
		--name "$PBS_CONTAINER" \
		-p "127.0.0.1:${PBS_PORT}:8007" \
		--tmpfs /run:rw,mode=1777 \
		-v "${PBS_CONTAINER}-etc:/etc/proxmox-backup" \
		-v "${PBS_CONTAINER}-data:/var/lib/proxmox-backup" \
		-v "${PBS_CONTAINER}-log:/var/log/proxmox-backup" \
		-e TZ=UTC \
		"$PBS_IMAGE" >/dev/null || die "failed to start PBS container"

	log "waiting for the PBS API to come up (can take ~20s on first boot)"
	retry 180 pbs_ready || {
		docker logs --tail 80 "$PBS_CONTAINER" >&2 || true
		die "PBS did not become ready within 180s"
	}
	ok "PBS is up: $(pbs_mgr version | head -n1)"
}

configure_pbs() {
	section "Configuring datastore, user and API token"

	step "creating datastore '$PBS_DATASTORE'"
	# `datastore create` prints a chunkstore progress bar; silence it.
	pbs_mgr datastore create "$PBS_DATASTORE" "/var/lib/proxmox-backup/store-${PBS_DATASTORE}" >/dev/null \
		|| die "datastore create failed"

	step "creating user '$PBS_USER'"
	pbs_mgr user create "$PBS_USER" >/dev/null || die "user create failed"

	# PBS >= 3 accepts exactly ONE role per call.
	#
	# The role has to be granted to the *token* auth-id as well as to the user,
	# otherwise API calls made with the token fail with
	#   "permission check failed - missing Datastore.Audit|Datastore.Backup".
	# The token ACL can only be set once the token exists, so the token has to
	# be generated first ("no such API token" otherwise).
	step "granting DatastoreAdmin to the user"
	pbs_mgr acl update "/datastore/${PBS_DATASTORE}" DatastoreAdmin --auth-id "$PBS_USER" >/dev/null \
		|| die "acl update (user) failed"

	step "generating API token '$PBS_AUTHID'"
	local token_json
	token_json="$(pbs_mgr user generate-token "$PBS_USER" "$PBS_TOKEN_NAME")" \
		|| die "user generate-token failed"
	PBS_TOKEN_SECRET="$(printf '%s' "$token_json" | sed -n 's/.*"value"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
	[ -n "$PBS_TOKEN_SECRET" ] || die "could not parse the API token secret from: $token_json"

	step "granting DatastoreAdmin to the API token"
	pbs_mgr acl update "/datastore/${PBS_DATASTORE}" DatastoreAdmin --auth-id "$PBS_AUTHID" >/dev/null \
		|| die "acl update (token) failed"

	# The client-facing certificate is proxy.pem in PBS 4.x (there is no
	# known_hosts/master.pem any more). `cert info` prints
	# "Fingerprint (sha256): 20:9d:...".
	# NOTE: BOTH our client and the official client want this colon-hex SHA-256
	# form. Plain 64-char hex is rejected by the official client.
	PBS_FINGERPRINT="$(pbs_mgr cert info | grep -i 'Fingerprint (sha256)' | head -n1 | sed 's/^.*: //' | tr -d '[:space:]')"
	[ -n "$PBS_FINGERPRINT" ] || die "could not determine the server certificate fingerprint"

	export PBS_REPOSITORY="${PBS_AUTHID}@127.0.0.1:${PBS_PORT}:${PBS_DATASTORE}"
	export PBS_FINGERPRINT PBS_TOKEN_SECRET
	export PBS_PASSWORD="$PBS_TOKEN_SECRET"

	# ACL changes are not visible to the API immediately; retry until the
	# official client (or any authenticated API call) succeeds.
	step "waiting for the datastore ACL to propagate"
	if [ "$E2E_SKIP_OFFICIAL" = "1" ]; then
		retry 60 bash -c "true" || true
	else
		retry 90 pbc snapshot list || die "the datastore ACL never became effective"
	fi

	ok "datastore='$PBS_DATASTORE' authid='$PBS_AUTHID' fingerprint='${PBS_FINGERPRINT:0:23}...'"

	# Namespaces are a server-side object and have to exist before a backup is
	# allowed to name one -- otherwise the server answers the HTTP/2 upgrade
	# handshake with "404 namespace not found".
	#
	# `proxmox-backup-manager` has no "namespace" verb in PBS 4.x, so this goes
	# through the official client, which does (namespace create/list/delete).
	if [ -n "$PBS_NAMESPACE" ]; then
		step "creating namespace '$PBS_NAMESPACE'"
		pbc namespace create "$PBS_NAMESPACE" >/dev/null \
			|| die "could not create namespace '$PBS_NAMESPACE'"
		ok "namespace '$PBS_NAMESPACE' created"
	fi
}

remove_pbs() {
	# -v only drops *anonymous* volumes, so the named etc/data/log volumes have
	# to go explicitly. Leaving them behind makes the next run fail with
	# "datastore 'e2e' already exists".
	docker rm -f "$PBS_CONTAINER" >/dev/null 2>&1 || true
	docker volume rm -f \
		"${PBS_CONTAINER}-etc" \
		"${PBS_CONTAINER}-data" \
		"${PBS_CONTAINER}-log" >/dev/null 2>&1 || true
}

stop_pbs() {
	remove_pbs
}

# ---------------------------------------------------------------------------
# Official Proxmox client wrapper
#
# pbc <subcommand> [args...]   -- runs with $WORK as the working directory so
# that all paths in the tests can be written relative to it.
# ---------------------------------------------------------------------------
PBC_MODE=""

detect_pbc() {
	if [ "$E2E_SKIP_OFFICIAL" = "1" ]; then
		PBC_MODE="skipped"
		warn "E2E_SKIP_OFFICIAL=1 -- official-client assertions will not run"
		return 0
	fi

	# An explicit PBC_BIN always wins; useful when debugging against a client
	# built from source.
	if [ -n "${PBC_BIN_EXPLICIT:-}" ] && command -v "$PBC_BIN" >/dev/null 2>&1; then
		PBC_MODE="local"
		log "using local official client: $(command -v "$PBC_BIN") ($("$PBC_BIN" version 2>&1 | head -n1))"
		return 0
	fi

	# Otherwise build the client from Proxmox's own Debian repository. This is
	# the default on purpose: the suite asserts behaviour that only a *current*
	# client has. A distro-packaged client is routinely years behind (3.2.7 on
	# the machine this was developed on) and the pre-built community images are
	# older still (2.4.4), which would quietly remove namespaces from the
	# matrix without anything failing.
	if ! command -v docker >/dev/null 2>&1; then
		die "docker is required to build the official client from the Proxmox Debian repository"
	fi
	step "building the official proxmox-backup-client from download.proxmox.com"
	docker build -q -t "$PBC_IMAGE" "$REPO_ROOT/testing/e2e/official-client" >/dev/null \
		|| die "failed to build $PBC_IMAGE from testing/e2e/official-client"
	PBC_MODE="docker"
	local ver
	ver="$(docker run --rm --entrypoint proxmox-backup-client "$PBC_IMAGE" version 2>&1 | head -n1)"
	[ -n "$ver" ] || die "built $PBC_IMAGE but it cannot report a version"
	ok "official client (from download.proxmox.com): $ver"
}

pbc() {
	if [ "$PBC_MODE" = "local" ]; then
		( cd "$WORK" && "$PBC_BIN" "$@" )
	elif [ "$PBC_MODE" = "docker" ]; then
		# Run as the invoking uid:gid, otherwise files the client creates in
		# the mounted workdir (most importantly the 0600 key file from
		# `key create`) end up owned by root and our own CLIs -- running as
		# the unprivileged user that invoked this script -- cannot read them.
		# --network host so PBS_REPOSITORY's 127.0.0.1:<port> resolves.
		docker run --rm --network host \
			--user "$(id -u):$(id -g)" \
			-v "$WORK:/e2e" -w /e2e \
			-e PBS_REPOSITORY -e PBS_FINGERPRINT -e PBS_PASSWORD \
			"$PBC_IMAGE" "$@"
	else
		die "official client not available"
	fi
}

pbc_available() { [ "$PBC_MODE" != "skipped" ]; }

# pbc_restore <snapshot> <archive> <target> [extra args...]
pbc_restore() {
	local snapshot="$1" archive="$2" target="$3"; shift 3
	local target_abs="$WORK/$target"
	# The official client creates blob/index targets with O_EXCL, so a leftover
	# file from a previous run would make the restore fail for the wrong reason.
	rm -rf "$target_abs"
	pbc restore "$snapshot" "$archive" "$target" "$@"
}

# Assert that an official-client operation that is *supposed* to fail really
# does fail. Used for the negative encryption tests.
pbc_restore_must_fail() {
	local snapshot="$1" archive="$2" target="$3"; shift 3
	local target_abs="$WORK/$target"
	rm -rf "$target_abs"
	if pbc restore "$snapshot" "$archive" "$target" "$@" >"$WORK/expected-failure.log" 2>&1; then
		cat "$WORK/expected-failure.log" >&2
		die "$snapshot $archive: expected the restore to FAIL without the key, but it succeeded"
	fi
	ok "$snapshot $archive: refused without the encryption key (as expected)"
}

# ---------------------------------------------------------------------------
# Our own CLI wrappers
# ---------------------------------------------------------------------------

# slugify <path> -- mirror of machinebackuplib.Slugify (machinebackup.go).
#
#	lowercase -> drop '/', ' ' and '_' -> drop everything outside [a-z0-9-]
#	-> squeeze runs of '-' -> trim leading/trailing '-'
#
# The Go implementation is pinned by TestSlugify in
# machinebackuplib/slugify_test.go; keep the two in step.
slugify() {
	printf '%s' "$1" \
		| tr 'A-Z' 'a-z' \
		| tr -d '/ _' \
		| sed -e 's/[^a-z0-9-]//g' -e 's/--*/-/g' -e 's/^-//' -e 's/-$//'
}
our_directory_backup() {
	local dir="$1"; shift
	( cd "$REPO_ROOT" && timeout 900 "$REPO_ROOT/dist/proxmoxbackup-directory" \
		-baseurl "$PBS_BASEURL" \
		-certfingerprint "$PBS_FINGERPRINT" \
		-authid "$PBS_AUTHID" \
		-secret "$PBS_TOKEN_SECRET" \
		-datastore "$PBS_DATASTORE" \
		-backupdir "$dir" \
		-novss \
		"$@" )
}

our_machine_backup() {
	local dev="$1"; shift
	( cd "$REPO_ROOT" && timeout 900 "$REPO_ROOT/dist/proxmoxbackup-machine" \
		-baseurl "$PBS_BASEURL" \
		-certfingerprint "$PBS_FINGERPRINT" \
		-authid "$PBS_AUTHID" \
		-secret "$PBS_TOKEN_SECRET" \
		-datastore "$PBS_DATASTORE" \
		-type host \
		-backupdev "$dev" \
		"$@" )
}