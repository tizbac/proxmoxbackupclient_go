#!/usr/bin/env bash
#
# End-to-end test suite for Salvanza against a real
# Proxmox Backup Server.
#
# Every test but the last performs a backup with one of OUR CLIs and then
# restores it with the OFFICIAL proxmox-backup-client, which is the real
# interoperability contract: if the official client can read and reproduce our
# archives byte-for-byte, our writer is correct. The last test is the reverse,
# for our reader: an encrypted backup made by the official client, restored by
# our own code.
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

	make_symlinks "$src"
	# The comparison leaves this name out at the root (lib.sh); the fixture
	# must not have its own file there, or a difference in it would be hidden.
	[ ! -e "$src/$BACKUP_META" ] || die "the fixture must not contain $BACKUP_META"
	log "directory fixture: $(find "$src" | wc -l) entries ($(find "$src" -type l | wc -l) symlinks), $(du -sh "$src" | cut -f1)"
}

# Symbolic links: a Linux backup stores them as links (writeSymlink in
# pbscommon/pxar.go) and the restore must recreate each link with its target
# text unchanged, not the file or folder it points to. assert_tree_identical
# compares them (tree_links).
make_symlinks() {
	local src="$1"
	ln -s hello.txt "$src/link-to-file"                      # relative, to a file
	ln -s ../hello.txt "$src/subdir/link-up"                 # relative, out of a folder
	ln -s subdir/nested "$src/link-to-dir"                   # to a folder: must not be followed
	ln -s does-not-exist "$src/dangling-link"                # target missing
	ln -s /etc/hostname "$src/subdir/absolute-link"          # absolute target
	ln -s "file with spaces & unïcode.txt" "$src/subdir/lïnk with spaces"
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

# unique_text <file> <MiB>: text that compresses well and that no earlier run
# or fixture has held, so a backup of it has chunks PBS does not hold yet.
# Tests that count what a backup uploaded (chunk_magic_count) need that: a
# chunk the datastore already has is not sent again.
unique_text() {
	python3 - "$1" "$2" <<'PY'
import os, sys
seed = os.urandom(16).hex()
line = "%s line %%09d of a fixture of the end-to-end suite\n" % seed
want = int(sys.argv[2]) * 1024 * 1024
with open(sys.argv[1], "w") as f:
    n = size = 0
    while size < want:
        s = line % n
        f.write(s)
        size += len(s)
        n += 1
PY
}

# Fixtures of the compression "none" test and of the official-client backup
# test: each its own unique data (see unique_text).
make_extra_fixtures() {
	mkdir -p "$WORK/src-none/sub"
	unique_text "$WORK/src-none/words.txt" 3
	printf 'a small file\n' >"$WORK/src-none/sub/small.txt"
	unique_text "$WORK/none.img" 9
	truncate -s $((9 * 1024 * 1024)) "$WORK/none.img"
	NONE_ARCHIVE="$(slugify "$WORK/none.img").fidx"

	cp -a "$WORK/src" "$WORK/src-official"
	unique_text "$WORK/src-official/words.txt" 3
	unique_text "$WORK/official.img" 9
	truncate -s $((9 * 1024 * 1024)) "$WORK/official.img"

	mkdir -p "$WORK/src-plain/sub"
	unique_text "$WORK/src-plain/words.txt" 3
	printf 'a small file\n' >"$WORK/src-plain/sub/small.txt"
	unique_text "$WORK/plain.img" 9
	truncate -s $((9 * 1024 * 1024)) "$WORK/plain.img"
	PLAIN_ARCHIVE="$(slugify "$WORK/plain.img").fidx"
	log "extra fixtures: src-none, none.img (archive '$NONE_ARCHIVE'), src-official, official.img, src-plain, plain.img (archive '$PLAIN_ARCHIVE')"
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
	assert_backup_meta "$WORK/restore-ns" "$(cd "$WORK/src" && pwd)" "$snap"

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
	# In a subshell, so a failed assertion (die -> exit 1) fails just this
	# test and the suite goes on to report every other one.
	if ( "$@" ); then
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
	assert_backup_meta "$WORK/restore-dir" "$(cd "$WORK/src" && pwd)" e2e-dir
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
		-keyfile "$KEYFILE" -compress-encrypted >"$WORK/log-dir-enc.txt" 2>&1 \
		|| { cat "$WORK/log-dir-enc.txt"; return 1; }

	# With -compress-encrypted an encrypted backup compresses before it
	# encrypts, as the official client does: the fixture has data that
	# compresses, so the datastore must now hold compressed encrypted chunks
	# (no test before this one used a key).
	local n
	n="$(chunk_magic_count "$MAGIC_ENCR_COMPR")"
	[ "$n" -gt 0 ] || die "the encrypted backup stored no compressed encrypted chunk"
	ok "encrypted backup: $n chunk(s) stored compressed, then encrypted"

	# Negative check: the key fingerprint is embedded in the manifest, so an
	# unkeyed restore must be refused.
	pbc_restore_must_fail host/e2e-dir-enc backup.pxar.didx restore-dir-enc-nokey

	pbc_restore host/e2e-dir-enc backup.pxar.didx restore-dir-enc --keyfile key.pem
	assert_tree_identical "$WORK/src" "$WORK/restore-dir-enc" "encrypted directory backup"
	assert_backup_meta "$WORK/restore-dir-enc" "$(cd "$WORK/src" && pwd)" e2e-dir-enc
}

# ===========================================================================
# Test 4: encrypted block backup (.fidx) -> official client restore --keyfile
# ===========================================================================
test_block_backup_encrypted() {
	our_machine_backup "$WORK/disk.img" -backup-id e2e-block-enc \
		-keyfile "$KEYFILE" -compress-encrypted >"$WORK/log-block-enc.txt" 2>&1 \
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
	# The note of the version that wrote the snapshot (pbscommon/
	# snapshot_writer.go) must come back from PBS, which rebuilds the rest of
	# the manifest at /finish. An unencrypted snapshot is in the format every
	# version reads, so it names a version and nothing else.
	python3 - "$WORK/manifest-dir.json" <<-'PY' || return 1
		import json, sys
		m = json.load(open(sys.argv[1]))
		w = m.get("unprotected", {}).get("salvanza-client")
		if not isinstance(w, dict) or not w.get("version"):
		    sys.exit(f"unprotected['salvanza-client'] names no version: {w!r}")
		if "format" in w or "min-version" in w:
		    sys.exit(f"an unencrypted snapshot names a format: {w!r}")
		print(f"  written by version {w['version']}")
	PY
	ok "unencrypted manifest: PBS kept the note of the version that wrote it"

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
		# An encrypted snapshot names the format with compressed encrypted
		# chunks and the first version that reads it, for a client too old
		# to know the format (pbscommon/snapshot_writer.go). The official
		# client restored this manifest and checked its signature above,
		# so the note does not disturb either.
		w = m.get("unprotected", {}).get("salvanza-client")
		if not isinstance(w, dict) or not w.get("version"):
		    sys.exit(f"unprotected['salvanza-client'] names no version: {w!r}")
		if w.get("format") != 1 or w.get("min-version") != "2.1.0":
		    sys.exit(f"an encrypted snapshot must name format 1 and min-version 2.1.0: {w!r}")
		print(f"  {len(files)} archives, all crypt-mode=encrypt; manifest signed; key set matches proxmox-backup")
		print(f"  written by version {w['version']}, format {w['format']}, needs {w['min-version']} or later")
	PY
	ok "encrypted manifest: signed, all archives crypt-mode=encrypt, PBS-compatible key set, writer note kept"
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

# ===========================================================================
# Test 9: -compression none -> nothing stored compressed, official restore
# ===========================================================================
# assert_stored_as <what> <magic that must not grow> <its count before>
#                  <magic that must grow> <its count before>
assert_stored_as() {
	local what="$1" never="$2" never0="$3" grown="$4" grown0="$5"
	local never1 grown1
	never1="$(chunk_magic_count "$never")"
	grown1="$(chunk_magic_count "$grown")"
	assert_eq "$never0" "$never1" "$what: chunks stored compressed"
	[ "$grown1" -gt "$grown0" ] || die "$what: no chunk was stored uncompressed (before $grown0, after $grown1)"
	ok "$what: $((grown1 - grown0)) chunk(s) stored, none of them compressed"
}

test_compression_none() {
	local c0 u0

	step "our CLI: folder backup with -compression none"
	c0="$(chunk_magic_count "$MAGIC_COMPRESSED")"
	u0="$(chunk_magic_count "$MAGIC_UNCOMPRESSED")"
	our_directory_backup "$WORK/src-none" -backup-id e2e-none -compression none \
		>"$WORK/log-none.txt" 2>&1 \
		|| { cat "$WORK/log-none.txt"; return 1; }
	assert_stored_as "folder backup, compression none" "$MAGIC_COMPRESSED" "$c0" "$MAGIC_UNCOMPRESSED" "$u0"
	pbc_restore host/e2e-none backup.pxar.didx restore-none
	assert_tree_identical "$WORK/src-none" "$WORK/restore-none" "folder backup, compression none"

	step "our CLI: encrypted folder backup with -compression none"
	c0="$(chunk_magic_count "$MAGIC_ENCR_COMPR")"
	u0="$(chunk_magic_count "$MAGIC_ENCRYPTED")"
	our_directory_backup "$WORK/src-none" -backup-id e2e-none-enc -compression none \
		-keyfile "$KEYFILE" -compress-encrypted >"$WORK/log-none-enc.txt" 2>&1 \
		|| { cat "$WORK/log-none-enc.txt"; return 1; }
	assert_stored_as "encrypted folder backup, compression none" "$MAGIC_ENCR_COMPR" "$c0" "$MAGIC_ENCRYPTED" "$u0"
	pbc_restore host/e2e-none-enc backup.pxar.didx restore-none-enc --keyfile key.pem
	assert_tree_identical "$WORK/src-none" "$WORK/restore-none-enc" "encrypted folder backup, compression none"

	step "our CLI: encrypted block backup with -compression none"
	c0="$(chunk_magic_count "$MAGIC_ENCR_COMPR")"
	u0="$(chunk_magic_count "$MAGIC_ENCRYPTED")"
	our_machine_backup "$WORK/none.img" -backup-id e2e-block-none-enc -compression none \
		-keyfile "$KEYFILE" -compress-encrypted >"$WORK/log-block-none-enc.txt" 2>&1 \
		|| { cat "$WORK/log-block-none-enc.txt"; return 1; }
	assert_stored_as "encrypted block backup, compression none" "$MAGIC_ENCR_COMPR" "$c0" "$MAGIC_ENCRYPTED" "$u0"
	pbc_restore host/e2e-block-none-enc "$NONE_ARCHIVE" restore-none-enc.img --keyfile key.pem
	assert_bytes_identical "$WORK/none.img" "$WORK/restore-none-enc.img" "encrypted block backup, compression none"
}

# ===========================================================================
# Test 10: encrypted backup by the OFFICIAL client -> restore by our own reader
# ===========================================================================
# The official client compresses a chunk or blob before it encrypts it
# (ENCR_COMPR). Our own backups do the same from version 2.1, and tests 3 and 4
# prove the official client reads those; this one proves our reader reads the
# official client's, in each place a restore decodes data: a folder archive
# (dynamic index), the same folder as a split archive, a disk image (fixed
# index) and a blob. There is no restore command to run, so the reading is
# done by a Go test of pbscommon that only runs when it is given a server
# (TestLiveRestoreOfficial, pbscommon/zz_restore_livetest_test.go).
test_official_backup_our_restore() {
	pbc_available || { warn "no official client -- skipping"; return 0; }
	command -v go >/dev/null 2>&1 || die "go is required: the restore is done by a Go test of pbscommon"

	local n0 n1
	n0="$(chunk_magic_count "$MAGIC_ENCR_COMPR")"
	step "official client: encrypted backup of a folder, a disk image and a blob"
	pbc backup src.pxar:src-official disk.img:official.img notes.conf:src-official/words.txt \
		--keyfile key.pem --backup-id e2e-official-enc >"$WORK/log-official-enc.txt" 2>&1 \
		|| { cat "$WORK/log-official-enc.txt"; return 1; }
	n1="$(chunk_magic_count "$MAGIC_ENCR_COMPR")"
	[ "$n1" -gt "$n0" ] || die "the official client stored no compressed encrypted chunk, so this test would prove nothing"
	ok "official client stored $((n1 - n0)) compressed encrypted chunk(s)"

	step "official client: the same folder as a split archive (--change-detection-mode data)"
	pbc backup src.pxar:src-official --change-detection-mode data \
		--keyfile key.pem --backup-id e2e-official-split >"$WORK/log-official-split.txt" 2>&1 \
		|| { cat "$WORK/log-official-split.txt"; return 1; }

	# The cases for the Go test: which snapshots, and what each archive must
	# come back as. The token secret goes in from the environment, into a
	# file only this user can read, in the workdir that is removed at the end.
	pbc snapshot list --output-format json >"$WORK/snapshots.json" \
		|| { pbc snapshot list; return 1; }
	rm -rf "$WORK/restore-official" "$WORK/restore-official-split"
	python3 - "$WORK" "$PBS_BASEURL" "$PBS_FINGERPRINT" "$PBS_DATASTORE" "$PBS_AUTHID" <<-'PY' || return 1
		import hashlib, json, os, sys
		work, url, fingerprint, datastore, authid = sys.argv[1:6]
		snaps = json.load(open(os.path.join(work, "snapshots.json")))
		def when(backup_id):
		    t = [s["backup-time"] for s in snaps if s["backup-id"] == backup_id]
		    if len(t) != 1:
		        sys.exit(f"snapshot {backup_id}: found {len(t)} of them, want 1")
		    return t[0]
		def files(backup_id):
		    return [f["filename"] if isinstance(f, dict) else f
		            for s in snaps if s["backup-id"] == backup_id for f in s.get("files", [])]
		def sha(rel):
		    h = hashlib.sha256()
		    with open(os.path.join(work, rel), "rb") as f:
		        for block in iter(lambda: f.read(1 << 20), b""):
		            h.update(block)
		    return h.hexdigest()
		enc, split = "e2e-official-enc", "e2e-official-split"
		for backup_id, names in ((enc, ["src.pxar.didx", "disk.img.fidx", "notes.conf.blob"]),
		                         (split, ["src.mpxar.didx", "src.ppxar.didx"])):
		    missing = [n for n in names if n not in files(backup_id)]
		    if missing:
		        sys.exit(f"snapshot {backup_id} has no {missing}: it holds {files(backup_id)}")
		def case(name, backup_id, kind, archive, **more):
		    return dict(name=name, type="host", id=backup_id, time=when(backup_id), kind=kind, archive=archive, **more)
		cfg = {
		    "url": url, "fingerprint": fingerprint, "datastore": datastore, "namespace": "",
		    "authid": authid, "secret": os.environ["PBS_TOKEN_SECRET"],
		    "keyfile": os.path.join(work, "key.pem"),
		    "cases": [
		        case("folder", enc, "pxar", "src.pxar.didx", dest=os.path.join(work, "restore-official")),
		        case("split folder", split, "pxar", "src.mpxar.didx", payload="src.ppxar.didx",
		             dest=os.path.join(work, "restore-official-split")),
		        case("disk image", enc, "image", "disk.img.fidx", sha256=sha("official.img")),
		        case("blob", enc, "blob", "notes.conf.blob", sha256=sha("src-official/words.txt")),
		    ],
		}
		fd = os.open(os.path.join(work, "restore-cases.json"), os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
		with os.fdopen(fd, "w") as f:
		    json.dump(cfg, f)
	PY

	step "our reader: restoring the official client's encrypted archives"
	( cd "$REPO_ROOT/pbscommon" && PBS_RESTORE_CASES="$WORK/restore-cases.json" \
		go test -count=1 -run '^TestLiveRestoreOfficial$' -v . ) >"$WORK/log-our-restore.txt" 2>&1 \
		|| { cat "$WORK/log-our-restore.txt"; return 1; }
	# A test that was skipped also exits 0: it must have run and passed.
	grep -q -- '--- PASS: TestLiveRestoreOfficial ' "$WORK/log-our-restore.txt" \
		|| { cat "$WORK/log-our-restore.txt"; die "TestLiveRestoreOfficial did not run"; }
	grep -- '--- PASS' "$WORK/log-our-restore.txt" || true
	ok "our reader read the disk image and the blob: sha256 as the source"

	assert_tree_identical "$WORK/src-official" "$WORK/restore-official" "official encrypted folder backup, our restore"
	assert_tree_identical "$WORK/src-official" "$WORK/restore-official-split" "official encrypted split folder backup, our restore"
}

# ===========================================================================
# Test 11: encrypted backup without -compress-encrypted -> stored as 2.0 did
# ===========================================================================
# Without -compress-encrypted an encrypted backup is written as version 2.0
# and earlier wrote it, the only form those versions restore: no chunk
# compressed before encryption, whatever -compression says. Its own data, so
# nothing it sends is already in the datastore in the other form.
test_encrypted_uncompressed_default() {
	local c0 u0

	step "our CLI: encrypted folder backup without -compress-encrypted"
	c0="$(chunk_magic_count "$MAGIC_ENCR_COMPR")"
	u0="$(chunk_magic_count "$MAGIC_ENCRYPTED")"
	our_directory_backup "$WORK/src-plain" -backup-id e2e-plain-enc \
		-keyfile "$KEYFILE" >"$WORK/log-plain-enc.txt" 2>&1 \
		|| { cat "$WORK/log-plain-enc.txt"; return 1; }
	assert_stored_as "encrypted folder backup, no -compress-encrypted" "$MAGIC_ENCR_COMPR" "$c0" "$MAGIC_ENCRYPTED" "$u0"
	grep -q 'written uncompressed' "$WORK/log-plain-enc.txt" \
		|| die "the run did not say its encrypted chunks are written uncompressed"
	pbc_restore host/e2e-plain-enc backup.pxar.didx restore-plain-enc --keyfile key.pem
	assert_tree_identical "$WORK/src-plain" "$WORK/restore-plain-enc" "encrypted folder backup, no -compress-encrypted"

	step "our CLI: encrypted block backup without -compress-encrypted, -compression best"
	c0="$(chunk_magic_count "$MAGIC_ENCR_COMPR")"
	u0="$(chunk_magic_count "$MAGIC_ENCRYPTED")"
	our_machine_backup "$WORK/plain.img" -backup-id e2e-block-plain-enc -compression best \
		-keyfile "$KEYFILE" >"$WORK/log-block-plain-enc.txt" 2>&1 \
		|| { cat "$WORK/log-block-plain-enc.txt"; return 1; }
	assert_stored_as "encrypted block backup, no -compress-encrypted" "$MAGIC_ENCR_COMPR" "$c0" "$MAGIC_ENCRYPTED" "$u0"
	grep -q 'written uncompressed' "$WORK/log-block-plain-enc.txt" \
		|| die "the run did not say its encrypted chunks are written uncompressed"
	pbc_restore host/e2e-block-plain-enc "$PLAIN_ARCHIVE" restore-plain-enc.img --keyfile key.pem
	assert_bytes_identical "$WORK/plain.img" "$WORK/restore-plain-enc.img" "encrypted block backup, no -compress-encrypted"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
	printf '%s\n' "$C_BOLD"
	cat <<-'BANNER'
		===============================================================
		 Salvanza -- end-to-end test suite
		 backup with our CLIs, restore with the official client
		 (and, last, the reverse)
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
	assert_file_exists "$REPO_ROOT/dist/salvanza-cli"

	section "Preparing fixtures"
	make_directory_fixture
	make_block_fixture
	make_extra_fixtures

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
	run_test "9. compression none -> official restore"             test_compression_none
	run_test "10. official encrypted backup -> our restore"        test_official_backup_our_restore
	run_test "11. encrypted, no -compress-encrypted -> as 2.0"     test_encrypted_uncompressed_default

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