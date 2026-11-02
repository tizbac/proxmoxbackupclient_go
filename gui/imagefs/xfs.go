package imagefs

import (
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"github.com/masahiro331/go-xfs-filesystem/xfs"
)

// XFS is an XFS volume, read through github.com/masahiro331/go-xfs-filesystem
// (the reader Trivy uses). That reader gives names, sizes, permissions and
// modification times, but not owners or symbolic link targets, and reads a
// file from its start only.
type XFS struct {
	mu    sync.Mutex // the reader seeks one shared section reader
	fs    *xfs.FileSystem
	label string
}

// OpenXFS reads the allocation groups of the XFS volume r holds.
func OpenXFS(r io.ReaderAt, size int64) (*XFS, error) {
	b := make([]byte, 120)
	_, _ = r.ReadAt(b, 0)
	f, err := xfs.NewFS(*io.NewSectionReader(r, 0, size), nil)
	if err != nil {
		return nil, fmt.Errorf("XFS: %w", err)
	}
	label := ""
	// sb_fname: 12 bytes at offset 108 of the superblock.
	for _, c := range b[108:120] {
		if c == 0 {
			break
		}
		label += string(rune(c))
	}
	return &XFS{fs: f, label: label}, nil
}

func (v *XFS) FSName() string        { return "XFS" }
func (v *XFS) Label() string         { return v.label }
func (v *XFS) CaseInsensitive() bool { return false }

// List returns every file, folder and symbolic link, sorted by path.
func (v *XFS) List(cancel func() bool, progress func(done, total int64)) ([]Entry, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	var out []Entry
	err := fs.WalkDir(v.fs, "/", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if cancel != nil && cancel() {
			return ErrCancelled
		}
		p = strings.TrimPrefix(p, "/")
		if p == "" {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		m := info.Mode()
		e := Entry{Path: p, MTime: Filetime(info.ModTime()), Mode: uint32(m.Perm())}
		switch {
		case m.IsDir():
			e.IsDir = true
			e.Mode |= ModeDir
		case m&fs.ModeSymlink != 0:
			e.Symlink = true
			e.Mode |= ModeSymlink
		case m.IsRegular():
			e.Size = uint64(info.Size())
			e.Mode |= ModeRegular
		default:
			return nil
		}
		out = append(out, e)
		if progress != nil && len(out)%1000 == 0 {
			progress(int64(len(out)), 0)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Open opens a file entry from List.
func (v *XFS) Open(e Entry) (FileReader, error) {
	return &xfsFile{v: v, path: e.Path, size: int64(e.Size)}, nil
}

// xfsFile reads a file in order: ReadAt at the position the last read ended
// carries on, an earlier one starts again from the beginning.
type xfsFile struct {
	v    *XFS
	path string
	size int64
	f    fs.File
	pos  int64
}

func (f *xfsFile) Size() int64         { return f.size }
func (f *xfsFile) Extents() [][2]int64 { return nil }

func (f *xfsFile) ReadAt(p []byte, off int64) (int, error) {
	if off >= f.size {
		return 0, io.EOF
	}
	f.v.mu.Lock()
	defer f.v.mu.Unlock()
	if f.f == nil || off < f.pos {
		if f.f != nil {
			f.f.Close()
		}
		file, err := f.v.fs.Open(f.path)
		if err != nil {
			return 0, err
		}
		f.f, f.pos = file, 0
	}
	if off > f.pos {
		if _, err := io.CopyN(io.Discard, f.f, off-f.pos); err != nil {
			return 0, err
		}
		f.pos = off
	}
	want := p
	if rem := f.size - off; int64(len(want)) > rem {
		want = want[:rem]
	}
	n, err := io.ReadFull(f.f, want)
	f.pos += int64(n)
	if err == io.ErrUnexpectedEOF || (err == nil && n < len(p)) {
		err = io.EOF
	}
	return n, err
}
