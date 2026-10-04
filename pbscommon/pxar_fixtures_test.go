package pbscommon

import (
	"encoding/binary"
	"strings"
	"testing"
)

// pxarEntry describes one file or directory to place in a synthetic archive.
type pxarEntry struct {
	// name is the archive-relative path, forward slashes. A trailing slash is
	// stripped; directories are detected by isDir.
	name  string
	isDir bool
	mode  uint64
	mtime uint64
	data  []byte
	// verbatim keeps name as-is instead of splitting it into parent/basename,
	// for fixtures that must exercise hostile archive names.
	verbatim bool
}

func pxarDir(name string) pxarEntry {
	return pxarEntry{name: name, isDir: true, mode: IFDIR | 0755, mtime: 1600000000}
}

func pxarFile(name string, data []byte) pxarEntry {
	return pxarEntry{name: name, mode: IFREG | 0644, mtime: 1600000123, data: data}
}

// pxarSection builds one PXAR record: a 16-byte little-endian header
// (type, total record size) followed by the payload.
func pxarSection(t *testing.T, typ uint64, payload []byte) []byte {
	t.Helper()
	out := make([]byte, 16+len(payload))
	binary.LittleEndian.PutUint64(out[0:8], typ)
	binary.LittleEndian.PutUint64(out[8:16], uint64(16+len(payload)))
	copy(out[16:], payload)
	return out
}

// pxarEntryPayload builds the 40-byte PXARFileEntry body that walk() parses:
// mode(u64) | flags(u64) | uid(u32) | gid(u32) | mtime.secs(u64) |
// mtime.nanos(u32) | padding(u32).
func pxarEntryPayload(t *testing.T, mode, mtime uint64) []byte {
	t.Helper()
	out := make([]byte, 40)
	binary.LittleEndian.PutUint64(out[0:8], mode)
	binary.LittleEndian.PutUint64(out[8:16], 0)      // flags
	binary.LittleEndian.PutUint32(out[16:20], 0)     // uid
	binary.LittleEndian.PutUint32(out[20:24], 0)     // gid
	binary.LittleEndian.PutUint64(out[24:32], mtime) // mtime.secs
	binary.LittleEndian.PutUint32(out[32:36], 0)     // mtime.nanos
	binary.LittleEndian.PutUint32(out[36:40], 0)     // padding
	return out
}

// pxarNamedFile emits a file whose FILENAME record is written verbatim, so a
// test can construct names that the normal basename-based builder cannot produce
// (absolute paths, drive-rooted paths, UNC paths).
func pxarNamedFile(t *testing.T, name string, data []byte) pxarEntry {
	t.Helper()
	return pxarEntry{name: name, mode: IFREG | 0644, mtime: 1600000000, data: data, verbatim: true}
}

// pxarNode is a node of the directory tree the fixture builder emits.
type pxarNode struct {
	entry    pxarEntry
	children []*pxarNode
}

// pxarTree turns archive-relative paths into a tree, preserving the caller's
// order. Directories that only exist implicitly (nobody listed them, but a file
// lives underneath) are synthesised and inserted just before their first child,
// because a real PXAR archive carries a FILENAME+ENTRY record for every
// directory it walks into — that is how the walker knows to descend.
func pxarTree(entries []pxarEntry) []*pxarNode {
	root := &pxarNode{}
	index := map[string]*pxarNode{}

	// ensureDir resolves path, creating it and any missing ancestors, and
	// returns the node that children of path must be appended to.
	var ensureDir func(path string) *pxarNode
	ensureDir = func(path string) *pxarNode {
		if path == "" {
			return root
		}
		if n, ok := index[path]; ok {
			return n
		}
		parentPath, base := "", path
		if i := strings.LastIndex(path, "/"); i >= 0 {
			parentPath, base = path[:i], path[i+1:]
		}
		n := &pxarNode{entry: pxarEntry{name: base, isDir: true, mode: IFDIR | 0755, mtime: 1600000000}}
		ensureDir(parentPath).children = append(ensureDir(parentPath).children, n)
		index[path] = n
		return n
	}

	for _, e := range entries {
		name := e.name
		for len(name) > 0 && name[len(name)-1] == '/' {
			name = name[:len(name)-1]
		}
		parentPath, base := "", name
		if !e.verbatim {
			if i := strings.LastIndex(name, "/"); i >= 0 {
				parentPath, base = name[:i], name[i+1:]
			}
		}
		parent := ensureDir(parentPath)

		// A PXAR FILENAME record holds only the basename: walk() rebuilds the
		// full path by joining the FILENAME onto the directory stack it
		// maintains via GOODBYE markers.
		if e.isDir {
			if n, ok := index[name]; ok {
				// Already created implicitly by an earlier child; keep the
				// children and adopt the explicit metadata.
				n.entry.mode, n.entry.mtime = e.mode, e.mtime
				continue
			}
			n := &pxarNode{entry: pxarEntry{name: base, isDir: true, mode: e.mode, mtime: e.mtime}}
			parent.children = append(parent.children, n)
			index[name] = n
			continue
		}

		parent.children = append(parent.children, &pxarNode{
			entry: pxarEntry{name: base, mode: e.mode, mtime: e.mtime, data: e.data},
		})
	}
	return root.children
}

// makePXAR serialises entries into a PXAR archive: a root directory ENTRY,
// then FILENAME + ENTRY per entry (+ PAYLOAD for files), with a GOODBYE closing
// every directory so the walker pops its path stack.
func makePXAR(t *testing.T, entries ...pxarEntry) []byte {
	t.Helper()

	var emit func(nodes []*pxarNode) []byte
	emit = func(nodes []*pxarNode) []byte {
		var out []byte
		for _, n := range nodes {
			out = append(out, pxarSection(t, PXAR_FILENAME, []byte(n.entry.name))...)
			out = append(out, pxarSection(t, PXAR_ENTRY, pxarEntryPayload(t, n.entry.mode, n.entry.mtime))...)
			if n.entry.isDir {
				out = append(out, emit(n.children)...)
				out = append(out, pxarSection(t, PXAR_GOODBYE, nil)...)
			} else {
				out = append(out, pxarSection(t, PXAR_PAYLOAD, n.entry.data)...)
			}
		}
		return out
	}

	out := pxarSection(t, PXAR_ENTRY, pxarEntryPayload(t, IFDIR|0755, 1600000000))
	out = append(out, emit(pxarTree(entries))...)
	out = append(out, pxarSection(t, PXAR_GOODBYE, nil)...)
	return out
}

// pxarRawEntry injects a verbatim record, used to build hostile archives.
func pxarRawEntry(t *testing.T, typ uint64, declaredSize uint64, payload []byte) []byte {
	t.Helper()
	out := make([]byte, 16+len(payload))
	binary.LittleEndian.PutUint64(out[0:8], typ)
	binary.LittleEndian.PutUint64(out[8:16], declaredSize)
	copy(out[16:], payload)
	return out
}

// standardPXAR is the fixture most tests share:
//
//	hello.txt
//	empty-dir/
//	sub/other.txt
//	sub/nested/deep.txt
//	notes/file with spaces & ünicode.txt
func standardPXAR(t *testing.T) []byte {
	t.Helper()
	return makePXAR(t,
		pxarFile("hello.txt", []byte("hello world\n")),
		pxarDir("empty-dir"),
		pxarFile("sub/other.txt", []byte("other\n")),
		pxarFile("sub/nested/deep.txt", []byte("deep\n")),
		pxarFile("notes/file with spaces & ünicode.txt", []byte("unicode\n")),
	)
}
