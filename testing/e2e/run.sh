#!/usr/bin/env bash
#
# End-to-end test suite for proxmoxbackupclient_go against a real
# Proxmox Backup Server.
#
# Every test performs a backup with one of OUR CLIs and then restores it with
# the OFFICIAL proxmox-backup-client, which is the real interoperability
# contract: if the official client can read and reproduce our archives
# byte-for-byte, our writer is correct.
#
# Usage:
#   ./testing/e2e/run.sh              # run everything
#   E2E_KEEP=1 ./testing/e2e/run.sh   # keep workdir + container for debugging
#
# See testing/e2e/README.md for the full documentation.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=testing/e2e/lib.sh
source "$SCRIPT_DIR/lib.sh"

PASSED=()
FAILED=()

# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------
make_directory_fixture() {
	local src="$WORK/src"
	mkdir -p "$src/subdir/nested" "$src/empty-dir"

	printf 'hello from the e2e suite\n' >"$src/hello.txt"
	printf 'nested payload\nsecond line\n' >"$src/subdir/nested/deep.txt"
	printf 'a sibling file\n' >"$src/subdir/other.txt"

	# A file with spaces and non-ASCII characters in its name exercises the
	# pxar entry encoding and the restore path rewriting.
	printf 'spaces and unicode in the name\n' >"$src/subdir/file with spaces & unïcode.txt"

	# 2 MiB of incompressible data: spans several 4 MiB fixed chunks so the
	# block backup is genuinely multi-chunk.
	dd if=/dev/urandom of="$src/random.bin" bs=1M count=2 status=none

	# 1 MiB of zeros: highly compressible, exercises the zstd path and dedup.
	dd if=/dev/zero of="$src/zeros.bin" bs=1M count=1 status=none

	# A 3 MiB repeating pattern: compresses but is not uniform.
	python3 -c "
import sys
open(sys.argv[1], 'wb').write(bytes(range(256)) * 4096 * 3)
" "$src/pattern.bin"

	# NOTE: symlinks are deliberately NOT part of the fixture. Our pxar
	# writer skips them (pbscommon/pxar_reader.go, walk()), so they are not
	# restorable today. See README.md "Known gaps".
	log "directory fixture: $(find "$src" | wc -l) entries, $(du -sh "$src" | cut -f1)"
}

make_block_fixture() {
local img="$WORK/disk.img"
	# ~25 MiB of deliberately mixed entropy so that neither the compressor nor
	# the fixed chunker degenerates:
	#   8 MiB random + 4 MiB 'A' + 12 MiB random + 1 MiB repeating pattern
	#
	# Built with python rather than `tr </dev/zero | dd count=N`: dd exits after
	# N blocks and closes the pipe, so tr dies of SIGPIPE and -- with
	# `set -o pipefail` that aborts the whole suite.
	python3 - "$img" <<'PY'
import os, sys
MB = 1024 * 1024
img = sys.argv[1]
regions = [(0, 8, "rand"), (8, 4, "A"), (12, 12, "rand"), (24, 1, "pattern")]
with open(img, "wb") as f:
    for off_mb, len_mb, kind in regions:
        f.seek(off_mb * MB)
        if kind == "rand":
            f.write(os.urandom(len_mb * MB))
        elif kind == "A":
            f.write(b"A" * (len_mb * MB))
        else:
            f.write(bytes(range(256)) * (len_mb * MB // 256))
    f.truncate(25 * MB)
PY

	# Our machine backup names the block archive Slugify(<device path>)+".fidx",
	# where Slugify lives in machinebackuplib/machinebackup.go:
	#
	#	lowercase -> drop '/', ' ' and '_' -> drop everything outside
	#	[a-z0-9-] -> squeeze runs of '-' -> trim leading/trailing '-'
	#
	# This shell function mirrors that algorithm step for step. The Go side is
	# pinned by TestSlugify in machinebackuplib/slugify_test.go, so if the two
	# ever drift apart the Go test fails there.
	BLOCK_ARCHIVE="$(slugify "$img").fidx"
	log "block fixture: $(stat -c%s "$img") bytes, expected archive '$BLOCK_ARCHIVE'"
}

make_keyfile() {
	KEYFILE="$WORK/key.pem"
	if pbc_available; then
		# Creating the key with the OFFICIAL client is itself an
		# interoperability assertion: our client then has to parse it.
		step "creating an encryption key with the official client"
		pbc key create --kdf none key.pem >/dev/null || die "key create failed"
		# The client writes the key 0600. Assert we can actually read it,
		# otherwise the failure resurfaces much later as a confusing
		# "permission denied" inside tests 3 and 4.
		[ -r "$KEYFILE" ] || die "the key file '$KEYFILE' is not readable by $(id -un) (mode $(stat -c%a "$KEYFILE"), owner $(stat -c%U "$KEYFILE"))"
	else
		step "creating an encryption key by hand (no official client)"
		local key
		key="$(head -c 32 /dev/urandom | base64 -w0)"
		cat >"$KEYFILE" <<-EOF
			{"kdf":null,"created":"2026-01-01T00:00:00Z","modified":"2026-01-01T00:00:00Z","data":"${key}","fingerprint":"unused"}
		EOF
		chmod 600 "$KEYFILE"
	fi
	assert_file_exists "$KEYFILE"
	ok "encryption key: $KEYFILE ($(stat -c%s "$KEYFILE") bytes)"
}

# Namespaces are a server-side object: a backup may only name a namespace that
# already exists. This test covers the whole round trip -- create the namespace,
# back up into it with our own CLI, list it back with our own client, and
# restore it with the official client using --ns.
#
# It exists because a version probe once suppressed the `ns` parameter on
# anything newer than PBS 2.x, silently writing namespaced backups into the
# root namespace instead. See NamespaceNotFoundErr in pbscommon/pbsapi.go.
test_namespace_roundtrip() {
	[ -n "$PBS_NAMESPACE" ] || { warn "PBS_NAMESPACE is empty -- skipping"; return 0; }

	local snap="e2e-ns"
	step "our CLI: backing up into namespace '$PBS_NAMESPACE'"
	our_directory_backup "$WORK/src" -backup-id "$snap" -namespace "$PBS_NAMESPACE" \
		>"$WORK/log-ns.txt" 2>&1 \
		|| { cat "$WORK/log-ns.txt"; return 1; }

	# The official client must be able to see it in that namespace...
	step "official client: listing namespace '$PBS_NAMESPACE'"
	pbc namespace list >"$WORK/ns-list.txt" 2>&1 \
		|| { cat "$WORK/ns-list.txt"; return 1; }
	grep -q "$PBS_NAMESPACE" "$WORK/ns-list.txt" \
		|| { cat "$WORK/ns-list.txt"; return 1; }

	# ...and restore from it.
	step "official client: restoring from namespace '$PBS_NAMESPACE'"
	pbc_restore "host/$snap" backup.pxar.didx restore-ns --ns "$PBS_NAMESPACE"
	assert_tree_identical "$WORK/src" "$WORK/restore-ns" "namespaced directory backup"

	# Guard against the exact regression above: the snapshot must NOT also be
	# visible in the root namespace. Tests 1-5 do create root-namespace
	# snapshots, so listing the root namespace is non-empty -- what matters is
	# that *this* snapshot is absent from it.
	step "our CLI: the snapshot must not leak into the root namespace"
	pbc snapshot list >"$WORK/root-list.txt" 2>&1 \
		|| { cat "$WORK/root-list.txt"; return 1; }
	if grep -q "$snap" "$WORK/root-list.txt"; then
		cat "$WORK/root-list.txt"
		return 1
	fi
	ok "namespaced snapshot is absent from the root namespace listing"
}

# ---------------------------------------------------------------------------
# Test runner
# ---------------------------------------------------------------------------
run_test() {
	local name="$1"; shift
	printf '\n%s--- %s ---%s\n' "$C_BOLD" "$name" "$C_RESET"
	if "$@"; then
		PASSED+=("$name")
		printf '%s%s✔ %s%s\n' "$C_BOLD" "$C_GREEN" "$name" "$C_RESET"
	else
		FAILED+=("$name")
		printf '%s%s✘ %s%s\n' "$C_BOLD" "$C_RED" "$name" "$C_RESET"
		return 0
	fi
}

# ===========================================================================
# Test 1: directory backup -> official client restore
# ===========================================================================
test_directory_backup() {
	our_directory_backup "$WORK/src" -backup-id e2e-dir  >"$WORK/log-dir.txt" 2>&1 \
		|| { cat "$WORK/log-dir.txt"; return 1; }

	pbc_restore host/e2e-dir backup.pxar.didx restore-dir
	assert_tree_identical "$WORK/src" "$WORK/restore-dir" "directory backup"
}

# ===========================================================================
# Test 2: block/file backup (.fidx) -> official client restore
# ===========================================================================
test_block_backup() {
	our_machine_backup "$WORK/disk.img" -backup-id e2e-block >"$WORK/log-block.txt" 2>&1 \
		|| { cat "$WORK/log-block.txt"; return 1; }

	pbc_restore host/e2e-block "$BLOCK_ARCHIVE" restore-block.img
	assert_bytes_identical "$WORK/disk.img" "$WORK/restore-block.img" "block backup"
}

# ===========================================================================
# Test 3: encrypted directory backup -> official client restore --keyfile
# ===========================================================================
test_directory_backup_encrypted() {
	our_directory_backup "$WORK/src" -backup-id e2e-dir-enc \
		-keyfile "$KEYFILE" >"$WORK/log-dir-enc.txt" 2>&1 \
		|| { cat "$WORK/log-dir-enc.txt"; return 1; }

	# Negative check: the key fingerprint is embedded in the manifest, so an
	# unkeyed restore must be refused.
	pbc_restore_must_fail host/e2e-dir-enc backup.pxar.didx restore-dir-enc-nokey

	pbc_restore host/e2e-dir-enc backup.pxar.didx restore-dir-enc --keyfile key.pem
	assert_tree_identical "$WORK/src" "$WORK/restore-dir-enc" "encrypted directory backup"
}

# ===========================================================================
# Test 4: encrypted block backup (.fidx) -> official client restore --keyfile
# ===========================================================================
test_block_backup_encrypted() {
	our_machine_backup "$WORK/disk.img" -backup-id e2e-block-enc \
		-keyfile "$KEYFILE" >"$WORK/log-block-enc.txt" 2>&1 \
		|| { cat "$WORK/log-block-enc.txt"; return 1; }

	pbc_restore_must_fail host/e2e-block-enc "$BLOCK_ARCHIVE" restore-block-enc.img
	pbc_restore host/e2e-block-enc "$BLOCK_ARCHIVE" restore-block-enc.img --keyfile key.pem
	assert_bytes_identical "$WORK/disk.img" "$WORK/restore-block-enc.img" "encrypted block backup"
}

# ===========================================================================
# Test 5: the official client can read our manifest and enumerate our snapshots
# ===========================================================================
test_manifest_interop() {
	local snap
	snap="$(pbc snapshot list --output-format json)" || {
		pbc snapshot list
		return 1
	}

	local found=0
	for want in e2e-dir e2e-dir-enc e2e-block e2e-block-enc; do
		case "$snap" in
			*"\"$want\""*) found=$((found + 1)) ;;
			*) printf '  snapshot %s not listed by the official client:\n' "$want" >&2
			   pbc snapshot list >&2
			   return 1 ;;
		esac
	done
	ok "official client lists all $found of our snapshots"

	# The manifest blob must be readable (and, when encrypted, verifiable).
	pbc_restore host/e2e-dir index.json.blob manifest-dir.json
	if ! grep -q '"backup-type"' "$WORK/manifest-dir.json"; then
		die "restored index.json.blob does not look like a manifest"
	fi
	ok "official client restored our index.json.blob (unencrypted snapshot)"

	pbc_restore host/e2e-dir-enc index.json.blob manifest-enc.json --keyfile key.pem
	if ! grep -q '"key-fingerprint"' "$WORK/manifest-enc.json"; then
		die "the encrypted manifest has no key-fingerprint"
	fi
	# Every archive in an encrypted snapshot must be marked crypt-mode=encrypt.
	# (index.json.blob itself is never in the signed `files` array -- PBS
	# re-serialises the manifest at /finish and the official client lists the
	# manifest blob separately as crypt-mode=sign-only.)
	python3 - "$WORK/manifest-enc.json" <<-'PY' || return 1
		import json, sys
		m = json.load(open(sys.argv[1]))
		files = m.get("files", [])
		if len(files) < 2:
		    sys.exit(f"expected >=2 archives in the encrypted manifest, found {len(files)}")
		bad = [f["filename"] for f in files if f.get("crypt-mode") != "encrypt"]
		if bad:
		    sys.exit(f"archives not marked crypt-mode=encrypt: {bad}")
		# PBS drops every key that proxmox-backup's BackupManifest does not
		# declare when it re-serialises the manifest at /finish. Any extra key
		# we sign would be missing from the stored copy and the signature
		# check would fail, so pin the exact key set here.
		want = {"backup-id", "backup-time", "backup-type", "files", "signature", "unprotected"}
		got = set(m)
		if got != want:
		    sys.exit(f"manifest key set mismatch: extra={got-want} missing={want-got}")
		if not m.get("signature"):
		    sys.exit("encrypted manifest carries no signature")
		fprint = set(files[0])
		if fprint != {"crypt-mode", "csum", "filename", "size"}:
		    sys.exit(f"unexpected file entry keys: {sorted(fprint)}")
		print(f"  {len(files)} archives, all crypt-mode=encrypt; manifest signed; key set matches proxmox-backup")
	PY
	ok "encrypted manifest: signed, all archives crypt-mode=encrypt, PBS-compatible key set"
}

# ===========================================================================
# Test 6: PBS itself can verify every chunk and archive we uploaded
# ===========================================================================
test_server_side_verify() {
	log "running proxmox-backup-manager verify (this reads every chunk back)"
	if ! pbs_mgr verify "$PBS_DATASTORE" 2>&1 | tee "$WORK/log-verify.txt"; then
		return 1
	fi
	grep -q 'TASK OK' "$WORK/log-verify.txt" \
		|| warn "verify output did not contain a success marker (see $WORK/log-verify.txt)"
	ok "PBS verified the datastore"
}

# ===========================================================================
# Test 7: dedup -- a second identical backup must reuse chunks
# ===========================================================================
test_dedup_reuse() {
	local out
	out="$(our_directory_backup "$WORK/src" -backup-id e2e-dir-dedup 2>&1)" \
		|| { printf '%s\n' "$out"; return 1; }
	if ! printf '%s' "$out" | grep -qi 'reuse'; then
		printf '%s\n' "$out" >&2
		die "the second identical backup did not report any chunk reuse"
	fi
	ok "second identical directory backup reused chunks from the first"

	out="$(our_machine_backup "$WORK/disk.img" -backup-id e2e-block-dedup 2>&1)" \
		|| { printf '%s\n' "$out"; return 1; }
	ok "second identical block backup completed (fixed index is content-addressed)"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
	printf '%s\n' "$C_BOLD"
	cat <<-'BANNER'
		===============================================================
		 proxmoxbackupclient_go -- end-to-end test suite
		 backup with our CLIs, restore with the official client
		===============================================================
	BANNER
	printf '%s' "$C_RESET"

	command -v docker >/dev/null 2>&1 || die "docker is required"
	docker info >/dev/null 2>&1 || die "the docker daemon is not reachable"
	command -v python3 >/dev/null 2>&1 || die "python3 is required (used for fixtures and JSON assertions)"

	log "workdir: $WORK"
	log "PBS:     $PBS_IMAGE on $PBS_BASEURL"

	if [ "$E2E_SKIP_BUILD" != "1" ]; then
		section "Building our CLIs"
		make -C "$REPO_ROOT" cli || die "make cli failed"
	fi
	assert_file_exists "$REPO_ROOT/dist/proxmoxbackup-directory"
	assert_file_exists "$REPO_ROOT/dist/proxmoxbackup-machine"

	section "Preparing fixtures"
	make_directory_fixture
	make_block_fixture

	detect_pbc
	start_pbs
	configure_pbs
	make_keyfile

	run_test "1. directory backup  -> official restore"            test_directory_backup
	run_test "2. block backup (.fidx) -> official restore"         test_block_backup
	run_test "3. encrypted directory backup -> official restore"   test_directory_backup_encrypted
	run_test "4. encrypted block backup -> official restore"       test_block_backup_encrypted
	run_test "5. manifest interop (snapshot list + index.json)"    test_manifest_interop
	run_test "6. PBS server-side verify"                           test_server_side_verify
	run_test "7. chunk dedup on a second identical backup"         test_dedup_reuse
	run_test "8. namespace round-trip (-namespace / --ns)"         test_namespace_roundtrip

	section "Summary"
	log "passed: ${#PASSED[@]}"
	for t in "${PASSED[@]+"${PASSED[@]}"}"; do ok "$t"; done
	if [ "${#FAILED[@]}" -gt 0 ]; then
		log "failed: ${#FAILED[@]}"
		for t in "${FAILED[@]}"; do printf '%s  FAIL%s  %s\n' "$C_RED" "$C_RESET" "$t"; done
		die "end-to-end suite failed"
	fi
	printf '%s%s\n' "$C_GREEN" "ALL END-TO-END TESTS PASSED" "$C_RESET"
}

cleanup() {
	local rc=$?
	if [ "$E2E_KEEP" = "1" ]; then
		warn "E2E_KEEP=1 -- leaving container '$PBS_CONTAINER' and workdir '$WORK' in place"
		docker logs --tail 30 "$PBS_CONTAINER" 2>/dev/null || true
	else
		stop_pbs
		rm -rf "$WORK"
	fi
	exit "$rc"
}
trap cleanup EXIT

main