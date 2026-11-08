"""Fixture tree for the official-client interop test (TestPXARv2OfficialInterop):
pxfix_gen.py <dir> [mutate].

The original fixture's layout (a/hello.txt, a hard-linked pair a/hardlink-src.txt
and a/b/hardlink-dst.txt) plus edge cases that work on Windows too: sizes
around block and chunk edges, multi-chunk random, zeros and repeating data,
duplicates, a sparse file, 300 small files, a deep path, unicode and emoji
names, symbolic links, empty folders, old and future times. With "mutate":
an edit, an addition, a deletion, a rename and a time-only change, for the
second metadata-mode backup."""
import os
import random
import sys

root = sys.argv[1]
rng = random.Random(7070)
M = 1 << 20


def write(rel, data, mtime=None):
    p = os.path.join(root, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, "wb") as f:
        f.write(data)
    if mtime is not None:
        os.utime(p, (mtime, mtime))


if len(sys.argv) > 2 and sys.argv[2] == "mutate":
    write("a/edit-me.txt", rng.randbytes(90000))
    write("added/new-after.txt", b"added before the second metadata backup\n")
    os.remove(os.path.join(root, "a/delete-me.bin"))
    os.rename(os.path.join(root, "names/rename-me.txt"), os.path.join(root, "names/renamed.txt"))
    os.utime(os.path.join(root, "a/touch-me.bin"), (1_800_000_000, 1_800_000_000))
    with open(os.path.join(root, "big/random-48m.bin"), "r+b") as f:
        f.seek(20 * M)
        f.write(b"one changed region in the middle of a big file")
    sys.exit(0)

write("a/hello.txt", b"hello from the interop fixture\n")
write("a/hardlink-src.txt", b"hard linked content\n")
os.makedirs(os.path.join(root, "a/b"), exist_ok=True)
os.link(os.path.join(root, "a/hardlink-src.txt"), os.path.join(root, "a/b/hardlink-dst.txt"))
write("a/edit-me.txt", rng.randbytes(70000))
write("a/delete-me.bin", rng.randbytes(30000))
write("a/touch-me.bin", rng.randbytes(30000))
write("empty.txt", b"")
for n in (1, 4095, 4096, 4097, 65536, 4 * M - 1, 4 * M, 4 * M + 1, 9 * M + 3):
    write(f"sizes/f{n}.bin", rng.randbytes(n))
write("big/random-48m.bin", rng.randbytes(48 * M))
write("big/zeros-16m.bin", bytes(16 * M))
pattern = rng.randbytes(1 << 16)
write("big/repeat-16m.bin", pattern * 256)
dup = rng.randbytes(5 * M)
write("dup/a.bin", dup)
write("dup/b.bin", dup)
sp = os.path.join(root, "sparse/holes-64m.bin")
os.makedirs(os.path.dirname(sp), exist_ok=True)
with open(sp, "wb") as f:
    f.truncate(64 * M)
    f.seek(32 * M)
    f.write(b"island in the middle")
for i in range(300):
    write(f"small/d{i % 7}/s{i:03d}.txt", f"small file {i}\n".encode() * (i % 9 + 1))
deep = "deep"
for i in range(12):
    deep = os.path.join(deep, f"level{i:02d}-" + "x" * 20)
write(os.path.join(deep, "bottom.txt"), b"at the bottom\n")
for name in ("unicode/русский файл.dat", "unicode/日本語のファイル.txt", "unicode/emoji ☕🎉.txt",
             "names/with  two spaces.txt", "names/rename-me.txt", "names/.hidden"):
    write(name, name.encode() + b"\n")
os.makedirs(os.path.join(root, "links"), exist_ok=True)
for link, target in {"to-file": "../a/hello.txt", "to-dir": "../unicode", "dangling": "does-not-exist"}.items():
    os.symlink(target, os.path.join(root, "links", link))
for d in ("empty-dir", "nested-empty/a/b"):
    os.makedirs(os.path.join(root, d), exist_ok=True)
write("times/old-2001.txt", b"old\n", mtime=978_400_000)
write("times/future-2040.txt", b"future\n", mtime=2_240_000_000)
print("fixture built")
