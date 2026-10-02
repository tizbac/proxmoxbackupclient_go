package pbscommon

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dchest/siphash"
)

// PXARReader reads and extracts PXAR archives.
// Supports nested directories, selective extraction, and content listing.
// Symlinks, ACLs, xattrs and devices are still skipped (read past) for now.
//
// The reader is backed by an io.ReaderAt rather than a single in-memory slice,
// so a multi-GB archive assembled to a temp file can be walked without ever
// loading the whole stream into RAM. File payloads are exposed to callers as
// section readers and streamed straight to disk during extraction.
type PXARReader struct {
	ra     io.ReaderAt
	size   int64
	offset int64

	// copyNanos/fsOverheadNanos accumulate wall-clock time spent during
	// extraction: copyNanos is time inside io.Copy (reading archive payload —
	// including any chunk-cache-miss wait — and writing it to the temp file);
	// fsOverheadNanos is everything else per file (CreateTemp, Close, Rename,
	// Chtimes). ExtractWithRewriter (sequential) writes one file at a time,
	// so for THAT path these are real wall-clock time on the critical path.
	// ExtractWithRewriterParallel shares these same counters across its
	// worker pool, so when that path is used the totals are a SUM across
	// however many workers ran concurrently, same caveat as
	// DIDXReaderAtStats' FetchNanos — found 2026-09-23 when a real restore's
	// reported "109.61s" filesystem overhead turned out to mean ~27s of real
	// time across ~4 concurrent workers, not a genuine 5x regression (the
	// real regression that run DID have was smaller but real: 28s wall-clock
	// vs 22s sequential on the same archive, measured from the
	// "Starting restore"/"Restore completed" log timestamps, not this
	// counter). Diagnostic only, added 2026-09-23.
	copyNanos       atomic.Int64
	fsOverheadNanos atomic.Int64

	// doneBytes/onProgress drive the restore progress bar by work actually
	// completed on disk: every entry the extractors finish (file closed and
	// renamed, directory created, or deliberately skipped) adds its Weight
	// (the archive bytes it occupies, see PXARTreeEntry.Weight). onProgress
	// receives the running total and may be called from several worker
	// goroutines at once, so it must be goroutine-safe.
	doneBytes  atomic.Int64
	onProgress func(doneBytes int64)

	// onFile is told the archive path of each file as its extraction starts
	// (the restore card's "current file" line). Same goroutine-safety rule
	// as onProgress.
	onFile func(path string)

	// hashFiles makes the extractors SHA-256 every file payload as it is
	// written (PXARExtractedFile.SHA256), for the optional verify-after-restore
	// pass. Off by default so a normal restore pays nothing.
	hashFiles bool
}

// SetHashFiles turns per-file SHA-256 of the extracted payloads on or off.
// Call before Extract*.
func (pr *PXARReader) SetHashFiles(on bool) { pr.hashFiles = on }

// copyPayload copies a file payload to out, returning its SHA-256 when hashing
// is enabled (nil otherwise).
func (pr *PXARReader) copyPayload(out io.Writer, payload io.Reader) ([]byte, error) {
	if !pr.hashFiles {
		_, err := io.Copy(out, payload)
		return nil, err
	}
	h := sha256.New()
	_, err := io.Copy(io.MultiWriter(out, h), payload)
	return h.Sum(nil), err
}

// SetFileCallback registers fn to be called with the archive path of each
// file as it starts being written. Call before Extract*.
func (pr *PXARReader) SetFileCallback(fn func(path string)) {
	pr.onFile = fn
}

// SetProgressCallback registers fn to be called with the cumulative archive
// bytes whose extraction has completed. Call before Extract*; not safe to
// change while an extraction is running.
func (pr *PXARReader) SetProgressCallback(fn func(doneBytes int64)) {
	pr.onProgress = fn
}

func (pr *PXARReader) entryDone(weight int64) {
	n := pr.doneBytes.Add(weight)
	if pr.onProgress != nil {
		pr.onProgress(n)
	}
}

// PXARReaderStats is a snapshot of where extraction's time went. For
// ExtractWithRewriter (sequential) these are real cumulative wall-clock time
// on the single extraction thread, directly comparable against the
// DIDXReaderAt's own FetchNanos. For ExtractWithRewriterParallel they are
// SUMMED across however many workers ran concurrently — see the copyNanos/
// fsOverheadNanos field doc for why that distinction matters in practice.
type PXARReaderStats struct {
	CopyNanos       int64 // time inside io.Copy (archive payload -> temp file, includes any cache-miss wait)
	FSOverheadNanos int64 // time in CreateTemp/Close/Rename/Chtimes, summed across every extracted file (and across workers, if the parallel path was used)
}

// Stats returns a snapshot of this reader's cumulative extraction time so far.
func (pr *PXARReader) Stats() PXARReaderStats {
	return PXARReaderStats{
		CopyNanos:       pr.copyNanos.Load(),
		FSOverheadNanos: pr.fsOverheadNanos.Load(),
	}
}

// PXARHeader represents a generic PXAR entry header.
type PXARHeader struct {
	Type uint64
	Size uint64
}

// PXARTreeEntry describes a file or directory found in a PXAR archive.
// Path uses forward slashes (archive style) and is relative to the archive root.
type PXARTreeEntry struct {
	Path    string
	IsDir   bool
	Size    uint64
	Mode    uint32
	ModTime int64
	// Weight is the number of archive bytes this entry accounts for: from the
	// end of the previous emitted entry to the end of this one (headers,
	// filename, payload). Summed over a span it equals the span's size, which
	// is what lets extraction progress be measured against the same total
	// the download side reports. Only set by walkRange.
	Weight int64
}

// PXARExtractedFile represents an extracted file (or directory) with metadata.
type PXARExtractedFile struct {
	Path string
	// ArchivePath is the entry's path inside the archive (forward slashes,
	// relative to the archive root), as opposed to Path (its destination on
	// disk after the rewriter ran). Only set for entries actually written —
	// skipped entries leave it empty, since nothing needs it. Lets a caller
	// match a restored file back to archive-relative metadata (e.g. the NTFS
	// ACL/attributes side-car, keyed by this same relative path).
	ArchivePath string
	Size        uint64
	Mode       os.FileMode
	ModTime    int64
	IsDir      bool
	Data       []byte
	Skipped    bool
	SkipReason string
	// Expected marks a deliberate, non-error skip (e.g. a file left untouched
	// because overwrite was disabled). Error skips (open/write/rename/mkdir
	// failures) leave this false so they still fail the restore.
	Expected bool
	// SHA256 is the hash of the payload as written, set only for files when
	// the reader was told to hash (SetHashFiles).
	SHA256 []byte
}

// NewPXARReader creates a new PXAR reader from an in-memory byte slice. Kept for
// callers that already hold the whole archive in RAM (small archives, tests).
// Large archives should use NewPXARReaderAt with a file-backed reader.
func NewPXARReader(data []byte) *PXARReader {
	return &PXARReader{ra: bytes.NewReader(data), size: int64(len(data))}
}

// NewPXARReaderAt creates a PXAR reader over an arbitrary io.ReaderAt (e.g. an
// *os.File holding an assembled archive). size is the total archive length in
// bytes. The reader never buffers more than one entry header / payload window
// at a time, so memory stays bounded regardless of archive size.
func NewPXARReaderAt(ra io.ReaderAt, size int64) *PXARReader {
	return &PXARReader{ra: ra, size: size}
}

func (pr *PXARReader) readHeader() (*PXARHeader, error) {
	if pr.offset+16 > pr.size {
		return nil, io.EOF
	}
	var raw [16]byte
	if _, err := pr.ra.ReadAt(raw[:], pr.offset); err != nil {
		return nil, err
	}
	return &PXARHeader{
		Type: binary.LittleEndian.Uint64(raw[0:8]),
		Size: binary.LittleEndian.Uint64(raw[8:16]),
	}, nil
}

func (pr *PXARReader) skip(n int64) { pr.offset += n }

// read returns the next n bytes and advances the cursor. Intended for small
// fixed-size sections (headers, filenames, entry structs). Payloads are NOT
// read this way — they are exposed as section readers to avoid buffering whole
// files in memory.
func (pr *PXARReader) read(n int64) ([]byte, error) {
	if pr.offset+n > pr.size {
		return nil, io.EOF
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(io.NewSectionReader(pr.ra, pr.offset, n), buf); err != nil {
		return nil, err
	}
	pr.offset += n
	return buf, nil
}

// reset rewinds the reader to the beginning so the same archive can be walked again.
func (pr *PXARReader) reset() { pr.offset = 0 }

// walkCallback is invoked for each file or directory entry encountered.
// payload is non-nil only for files — it is a section reader over the file's
// raw content. The callback may read it (e.g. to stream the file to disk) or
// ignore it; the walker advances past the payload either way.
type walkCallback func(entry PXARTreeEntry, payload *io.SectionReader) error

// walk iterates the entire PXAR archive, invoking cb for each entry.
// Correctly tracks the directory stack via PXAR_GOODBYE markers, so nested
// directories and empty directories are handled properly.
func (pr *PXARReader) walk(cb walkCallback) error {
	return pr.walkRange(cb, 0, pr.size, "", false)
}

// walkRange is walk's general form, added for the BST-based selective-restore
// fast path (ResolveArchivePathBST): once that resolves a target's byte span
// without visiting anything before it, walking the target's OWN subtree needs
// this exact same FILENAME/ENTRY/PAYLOAD/GOODBYE state machine, just starting
// partway into the stream instead of at true offset 0. Every existing caller
// goes through walk() above, which reproduces the original 0/size/""/false
// behavior exactly — this function changes nothing for them.
//
//   - startOffset/endOffset bound the scan to [startOffset, endOffset) instead
//     of the whole archive; reads never cross endOffset.
//   - initialPath seeds currentPath — the archive path of startOffset's
//     PARENT, so the first entry found (which HAS its own preceding FILENAME,
//     same as any normal nested entry) composes its path correctly via the
//     existing joinArchivePath(currentPath, pendingName) call below.
//   - rootSeen=true skips walk's own special-case for archive offset 0 (the
//     true root's ENTRY header has no preceding FILENAME and is never itself
//     emitted) — startOffset is never the true root in this function's actual
//     use, so every ENTRY found here is a normal, emitted, named entry.
func (pr *PXARReader) walkRange(cb walkCallback, startOffset, endOffset int64, initialPath string, rootSeen bool) error {
	pr.offset = startOffset
	var pathStack []string
	currentPath := initialPath
	pendingName := ""
	var pendingFileMode uint64
	var pendingFileMtime uint64
	hasPendingFile := false
	lastEmit := startOffset

	for {
		if pr.offset >= endOffset {
			break
		}
		header, err := pr.readHeader()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read header at offset %d: %w", pr.offset, err)
		}
		if header.Size < 16 {
			return fmt.Errorf("invalid header size %d at offset %d", header.Size, pr.offset)
		}
		// header.Size is controlled by the archive bytes (corruption or a hostile
		// snapshot). Bound it to the bytes remaining, compared in uint64 to avoid
		// the int64 overflow that turns a huge size negative: otherwise contentSize
		// goes negative and a later make([]byte, n) panics, or a negative skip
		// rewinds the cursor into an infinite loop. Listing/search walk this
		// without a recover(), so an unbounded value here can crash or hang the GUI.
		remaining := uint64(pr.size - pr.offset)
		if header.Size > remaining {
			return fmt.Errorf("header size %d exceeds %d remaining bytes at offset %d", header.Size, remaining, pr.offset)
		}
		contentSize := int64(header.Size) - 16

		switch header.Type {
		case PXAR_FILENAME:
			pr.skip(16)
			data, err := pr.read(contentSize)
			if err != nil {
				return fmt.Errorf("read filename: %w", err)
			}
			pendingName = string(bytes.TrimRight(data, "\x00"))

		case PXAR_ENTRY, PXAR_ENTRY_V1:
			pr.skip(16)
			data, err := pr.read(contentSize)
			if err != nil {
				return fmt.Errorf("read entry: %w", err)
			}
			// Parse PXARFileEntry payload by byte offset. We avoid binary.Read
			// on struct pointers because PXARFileEntry/MTime have unexported
			// fields that reflect.Value cannot Set, which silently zeroes mtime.
			// Layout: mode(u64) | flags(u64) | uid(u32) | gid(u32) | mtime.secs(u64) | mtime.nanos(u32) | mtime.padding(u32) = 40 bytes
			var mode uint64
			var mtimeSecs uint64
			if len(data) >= 32 {
				mode = binary.LittleEndian.Uint64(data[0:8])
				mtimeSecs = binary.LittleEndian.Uint64(data[24:32])
			}

			if mode&IFMT == IFDIR {
				// Directory entry. The first ENTRY in the archive is the root
				// (no preceding FILENAME) and is not emitted as its own entry.
				if !rootSeen {
					rootSeen = true
				} else {
					subPath := joinArchivePath(currentPath, pendingName)
					weight := pr.offset - lastEmit
					lastEmit = pr.offset
					if err := cb(PXARTreeEntry{
						Path:    subPath,
						IsDir:   true,
						Mode:    uint32(mode),
						ModTime: int64(mtimeSecs),
						Weight:  weight,
					}, nil); err != nil {
						return err
					}
					pathStack = append(pathStack, currentPath)
					currentPath = subPath
				}
				pendingName = ""
				hasPendingFile = false
			} else {
				// Regular file: defer emission until the matching PAYLOAD arrives.
				pendingFileMode = mode
				pendingFileMtime = mtimeSecs
				hasPendingFile = true
			}

		case PXAR_PAYLOAD:
			pr.skip(16)
			if pr.offset+contentSize > pr.size {
				return fmt.Errorf("read payload: %w", io.EOF)
			}
			if hasPendingFile {
				path := joinArchivePath(currentPath, pendingName)
				payload := io.NewSectionReader(pr.ra, pr.offset, contentSize)
				weight := pr.offset + contentSize - lastEmit
				lastEmit = pr.offset + contentSize
				if err := cb(PXARTreeEntry{
					Path:    path,
					IsDir:   false,
					Size:    uint64(contentSize),
					Mode:    uint32(pendingFileMode),
					ModTime: int64(pendingFileMtime),
					Weight:  weight,
				}, payload); err != nil {
					return err
				}
				hasPendingFile = false
				pendingName = ""
			}
			pr.skip(contentSize)

		case PXAR_GOODBYE:
			pr.skip(int64(header.Size))
			if len(pathStack) > 0 {
				currentPath = pathStack[len(pathStack)-1]
				pathStack = pathStack[:len(pathStack)-1]
			} else {
				currentPath = ""
			}

		default:
			// Symlinks, devices, xattrs, ACLs, FCAPs, hardlinks, quota etc.
			// These are skipped for now — they belong to file metadata that
			// the upcoming NTFS sidecar work will handle properly.
			pr.skip(int64(header.Size))
		}
	}
	return nil
}

// errBSTNotFound is returned internally when the GOODBYE binary-search-tree
// lookup can't find a requested child — either it genuinely doesn't exist,
// or something about the table didn't look as expected. Either way the
// caller (ResolveArchivePathBST's caller, walkSelective) falls back to a
// full linear walk() rather than treat this as fatal: this whole fast path
// is a pure optimization, never the only way to reach a file.
var errBSTNotFound = errors.New("pxar: entry not found via GOODBYE BST")

// siphashKey0/1 must match the exact keys WriteDir (pxar.go) hashes every
// filename with when building a directory's GOODBYE table — the BST search
// below only works if it's searching for the SAME hash the writer sorted by.
const (
	siphashKey0 = 0x83ac3f1cfbb450db
	siphashKey1 = 0xaa4f1b6879369fbd
)

// readGoodByeItem manually parses one 24-byte GoodByeItem (hash, offset, len,
// each a little-endian uint64, in that FIELD ORDER — see pxar.go's struct
// declaration) at an exact byte position. Deliberately not binary.Read on the
// struct: GoodByeItem's fields are unexported, and binary.Read needs to SET
// struct fields via reflection, which silently zeroes unexported ones
// regardless of caller package (binary.Write only needs to READ them, which
// has no such restriction — see WriteFile's own matching comment on
// PXARFileEntry for the same issue on the write... er, read side there).
func (pr *PXARReader) readGoodByeItem(pos int64) (hash, offset, length uint64, err error) {
	buf := make([]byte, 24)
	if _, err := pr.ra.ReadAt(buf, pos); err != nil {
		return 0, 0, 0, err
	}
	return binary.LittleEndian.Uint64(buf[0:8]),
		binary.LittleEndian.Uint64(buf[8:16]),
		binary.LittleEndian.Uint64(buf[16:24]),
		nil
}

// findChildSpan looks up name within the GOODBYE table of the directory
// whose content ends at spanEnd (spanEnd is always a directory's own
// exclusive end offset — see ResolveArchivePathBST), using the exact same
// balanced-BST array layout ca_make_bst (pxar.go) builds on write: a sorted
// array stored heap-style, root at index 0, children at 2i+1/2i+2, so a
// binary search by hash never needs to visit a sibling outside the search
// path. Returns [childStart, childEnd) — childStart is where the child's own
// FILENAME header begins, childEnd is the exclusive end of its entire span
// (its own content plus, if it's a directory, its own trailing GOODBYE
// table) — both computed the same way WriteDir itself tracks them on write.
func (pr *PXARReader) findChildSpan(spanEnd int64, name string) (childStart, childEnd int64, err error) {
	// The trailer is always the LAST 24 bytes of a directory's GOODBYE table,
	// and that table is always the last thing written within the directory's
	// own span — so it's always the 24 bytes immediately before spanEnd.
	trailerHash, _, goodbyeLen, err := pr.readGoodByeItem(spanEnd - 24)
	if err != nil {
		return 0, 0, fmt.Errorf("read trailer: %w", err)
	}
	if trailerHash != PXAR_GOODBYE_TAIL_MARKER {
		return 0, 0, fmt.Errorf("%w: no GOODBYE trailer at expected position", errBSTNotFound)
	}
	goodbyeHeaderPos := spanEnd - int64(goodbyeLen)
	if goodbyeHeaderPos < 0 {
		return 0, 0, fmt.Errorf("%w: GOODBYE header position negative", errBSTNotFound)
	}

	entryCount := (int64(goodbyeLen) - 16) / 24 - 1 // total entries minus the trailer itself
	if entryCount < 0 {
		return 0, 0, fmt.Errorf("%w: negative entry count", errBSTNotFound)
	}
	tableStart := goodbyeHeaderPos + 16 // past the 16-byte PXAR_GOODBYE header

	target := siphash.Hash(siphashKey0, siphashKey1, []byte(name))

	i := int64(0)
	for i < entryCount {
		itemPos := tableStart + i*24
		itemHash, itemOffset, itemLen, err := pr.readGoodByeItem(itemPos)
		if err != nil {
			return 0, 0, fmt.Errorf("read BST entry %d: %w", i, err)
		}
		switch {
		case target == itemHash:
			start := goodbyeHeaderPos - int64(itemOffset)
			return start, start + int64(itemLen), nil
		case target < itemHash:
			i = i*2 + 1
		default:
			i = i*2 + 2
		}
	}
	return 0, 0, fmt.Errorf("%w: %q", errBSTNotFound, name)
}

// ResolveArchivePathBST resolves an archive-relative path (forward slashes,
// no leading slash — same convention as PXARTreeEntry.Path) to its byte span
// using ONLY GOODBYE-table binary searches, one per path component — never
// visiting a sibling file/directory that isn't on the direct path to the
// target. Returns the target's own [start, end) span and the archive path of
// its PARENT (for walkRange's initialPath, so entries found while walking the
// target's own subtree compose correct full paths). Any failure (not found,
// or anything about a GOODBYE table not matching the expected shape) returns
// errBSTNotFound-wrapped — always meant to be a "fall back to the proven
// linear walk" signal, never a hard error surfaced to the user.
func (pr *PXARReader) ResolveArchivePathBST(path string) (targetStart, targetEnd int64, parentPath string, err error) {
	path = strings.Trim(path, "/")
	if path == "" {
		return 0, 0, "", fmt.Errorf("%w: empty path", errBSTNotFound)
	}
	components := strings.Split(path, "/")

	spanEnd := pr.size
	for idx, comp := range components {
		start, end, ferr := pr.findChildSpan(spanEnd, comp)
		if ferr != nil {
			return 0, 0, "", ferr
		}
		if idx == len(components)-1 {
			return start, end, strings.Join(components[:idx], "/"), nil
		}
		spanEnd = end
	}
	// unreachable (components is non-empty)
	return 0, 0, "", errBSTNotFound
}

// joinArchivePath joins archive paths with forward slashes, never producing
// a leading or duplicate slash.
func joinArchivePath(parent, child string) string {
	if parent == "" {
		return child
	}
	if child == "" {
		return parent
	}
	return parent + "/" + child
}

// ReadVirtualFile returns the payload of a file located at the archive root,
// matched by its exact name (e.g. ".proxmox_backup_client_meta.json"). Returns
// os.ErrNotExist if no such root-level file is present.
//
// Only root-level entries are considered — nested files of the same name are
// ignored. This matches how the writer injects sidecar files (always at root).
// Sidecar files are small, so reading the payload fully into memory is fine.
func (pr *PXARReader) ReadVirtualFile(name string) ([]byte, error) {
	var found []byte
	stopErr := errors.New("pxar: virtual file found")
	err := pr.walk(func(e PXARTreeEntry, payload *io.SectionReader) error {
		if e.IsDir {
			return nil
		}
		// Root-level files have no slash in their archive path.
		if strings.Contains(e.Path, "/") {
			return nil
		}
		if e.Path == name {
			data, rerr := io.ReadAll(payload)
			if rerr != nil {
				return rerr
			}
			found = data
			return stopErr
		}
		return nil
	})
	if err != nil && !errors.Is(err, stopErr) {
		return nil, err
	}
	if found == nil {
		return nil, os.ErrNotExist
	}
	return found, nil
}

// ListEntries returns all files and directories in the archive without extracting
// any payload. Useful for displaying a navigable tree before restore.
func (pr *PXARReader) ListEntries() ([]PXARTreeEntry, error) {
	entries := make([]PXARTreeEntry, 0, 256)
	err := pr.walk(func(e PXARTreeEntry, _ *io.SectionReader) error {
		entries = append(entries, e)
		return nil
	})
	return entries, err
}

// PathRewriter maps an archive-relative path (forward slash) to a filesystem
// path on the target host. Returning an empty string skips the entry silently
// — useful when a rewriter wants to drop entries outside the selection root in
// flat mode.
type PathRewriter func(archivePath string) string

// ExtractAll extracts the entire PXAR archive to destDir.
func (pr *PXARReader) ExtractAll(destDir string) ([]PXARExtractedFile, error) {
	return pr.ExtractFiltered(destDir, nil, false)
}

// ExtractFiltered extracts entries whose archive path matches one of includePaths
// to destDir, preserving the archive's directory layout below destDir.
//
// Equivalent to ExtractWithRewriter using the default "dest + archive_path"
// rewriter. Kept for backward compatibility — new callers should use
// ExtractWithRewriter directly when they need in-place or flat restores.
func (pr *PXARReader) ExtractFiltered(destDir string, includePaths []string, overwrite bool) ([]PXARExtractedFile, error) {
	rewriter := func(archivePath string) string {
		return filepath.Join(destDir, filepath.FromSlash(archivePath))
	}
	return pr.ExtractWithRewriter(rewriter, includePaths, overwrite)
}

// ExtractWithRewriter walks the archive once, runs include filtering, and for
// each matching entry asks the rewriter where to write it on disk. A rewriter
// returning "" tells the walker to drop that entry without recording a skip.
//
// File payloads are streamed straight from the archive to the destination file
// (io.Copy), so even multi-GB files restore with bounded memory.
//
// All filesystem decisions (mkdir parent, overwrite, mode bits, mtime) are
// centralized here so each restore mode only has to express its path policy.
func (pr *PXARReader) ExtractWithRewriter(rewriter PathRewriter, includePaths []string, overwrite bool) ([]PXARExtractedFile, error) {
	if rewriter == nil {
		return nil, fmt.Errorf("path rewriter required")
	}
	includes := NormalizeIncludes(includePaths)
	extracted := make([]PXARExtractedFile, 0, 64)

	extractEntry := func(e PXARTreeEntry, payload *io.SectionReader) error {
		if !pathMatches(e.Path, includes) {
			return nil
		}

		// Zip-slip guard: refuse any entry whose name could escape the restore
		// root once joined (absolute, drive-rooted, UNC, or containing "..").
		// rewriter(root + cleanRelative) cannot escape root, so validating the
		// entry path here protects all restore modes uniformly.
		if isUnsafeArchivePath(e.Path) {
			extracted = append(extracted, PXARExtractedFile{
				Path: e.Path, IsDir: e.IsDir,
				Skipped: true, SkipReason: "unsafe archive path refused (possible traversal)",
			})
			return nil
		}

		fullPath := rewriter(e.Path)
		if fullPath == "" {
			// Rewriter chose to drop this entry (e.g. ancestor of the flat
			// selection root). Not a skip — just not visible at the target.
			return nil
		}

		if e.IsDir {
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				extracted = append(extracted, PXARExtractedFile{
					Path: fullPath, IsDir: true,
					Skipped: true, SkipReason: fmt.Sprintf("mkdir: %v", err),
				})
				return nil
			}
			extracted = append(extracted, PXARExtractedFile{
				Path: fullPath, ArchivePath: e.Path, IsDir: true,
				Mode: os.FileMode(e.Mode & 0777), ModTime: e.ModTime,
			})
			return nil
		}

		// File: ensure parent dir exists (a parent might not have been
		// emitted yet if the user selected a deep path directly).
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			extracted = append(extracted, PXARExtractedFile{
				Path: fullPath, Size: e.Size,
				Skipped: true, SkipReason: fmt.Sprintf("mkdir parent: %v", err),
			})
			return nil
		}

		if !overwrite {
			if _, err := os.Stat(fullPath); err == nil {
				extracted = append(extracted, PXARExtractedFile{
					Path: fullPath, Size: e.Size,
					Skipped: true, Expected: true, SkipReason: "already exists",
				})
				return nil
			}
		}

		// Write to a sibling temp file then atomically rename into place. The
		// existing target stays intact until the new content is fully written —
		// critical for in-place restore, where a mid-copy failure must not leave
		// the original truncated. os.Rename replaces the target on both Unix and
		// Windows (MOVEFILE_REPLACE_EXISTING).
		//
		// Use a RANDOM, exclusive temp name via os.CreateTemp (which opens with
		// O_EXCL) instead of the predictable "<file>.proxmox-part" opened with
		// O_TRUNC: a local attacker could otherwise pre-create/guess that path
		// (v2-H-08). NOTE: a symlink/reparse point in the destination's PARENT chain
		// can still redirect the write — confining the parent chain (no-follow /
		// Windows reparse handling) is a separate hardening.
		fsStart := time.Now()
		out, err := os.CreateTemp(filepath.Dir(fullPath), filepath.Base(fullPath)+".proxmox-*.part")
		pr.fsOverheadNanos.Add(int64(time.Since(fsStart)))
		if err != nil {
			extracted = append(extracted, PXARExtractedFile{
				Path: fullPath, Size: e.Size,
				Skipped: true, SkipReason: fmt.Sprintf("open: %v", err),
			})
			return nil
		}
		tmpPath := out.Name()
		_ = out.Chmod(os.FileMode(e.Mode & 0777))
		copyStart := time.Now()
		sum, copyErr := pr.copyPayload(out, payload)
		pr.copyNanos.Add(int64(time.Since(copyStart)))
		fsStart = time.Now()
		closeErr := out.Close()
		pr.fsOverheadNanos.Add(int64(time.Since(fsStart)))
		if copyErr != nil {
			_ = os.Remove(tmpPath)
			extracted = append(extracted, PXARExtractedFile{
				Path: fullPath, Size: e.Size,
				Skipped: true, SkipReason: fmt.Sprintf("write: %v", copyErr),
			})
			return nil
		}
		if closeErr != nil {
			_ = os.Remove(tmpPath)
			extracted = append(extracted, PXARExtractedFile{
				Path: fullPath, Size: e.Size,
				Skipped: true, SkipReason: fmt.Sprintf("close: %v", closeErr),
			})
			return nil
		}
		fsStart = time.Now()
		renErr := os.Rename(tmpPath, fullPath)
		pr.fsOverheadNanos.Add(int64(time.Since(fsStart)))
		if renErr != nil {
			_ = os.Remove(tmpPath)
			extracted = append(extracted, PXARExtractedFile{
				Path: fullPath, Size: e.Size,
				Skipped: true, SkipReason: fmt.Sprintf("rename: %v", renErr),
			})
			return nil
		}
		if e.ModTime != 0 { // negative = pre-1970 mtime, still valid
			fsStart = time.Now()
			t := time.Unix(e.ModTime, 0)
			_ = os.Chtimes(fullPath, t, t)
			pr.fsOverheadNanos.Add(int64(time.Since(fsStart)))
		}
		extracted = append(extracted, PXARExtractedFile{
			Path: fullPath, ArchivePath: e.Path, Size: e.Size,
			Mode: os.FileMode(e.Mode & 0777), ModTime: e.ModTime, SHA256: sum,
		})
		return nil
	}

	// Every entry the walker hands over counts as done once extractEntry has
	// dealt with it (written, created or skipped), so progress reaches 100%
	// only when the last file is on disk.
	cb := func(e PXARTreeEntry, payload *io.SectionReader) error {
		if pr.onFile != nil && !e.IsDir && pathMatches(e.Path, includes) {
			pr.onFile(e.Path)
		}
		if err := extractEntry(e, payload); err != nil {
			return err
		}
		pr.entryDone(e.Weight)
		return nil
	}

	// Fast path: exactly one clean selection can often be resolved directly
	// via the archive's own GOODBYE binary-search-tree index
	// (ResolveArchivePathBST) instead of linearly scanning every entry before
	// it — see that function's doc comment for why this is safe (it only
	// ever trusts a GOODBYE table shape it has actually verified) and
	// pxar.go's ca_make_bst for the index format this relies on. ANY failure
	// here (not found, unexpected shape, I/O error) falls back to the proven
	// full walk below — this is a pure optimization, never the only way a
	// restore can succeed. Multiple includes keep using the full walk for
	// now: resolving several independent spans and merging their results is
	// a reasonable future extension, not needed for the common "restore one
	// folder out of a big snapshot" case this targets.
	if len(includes) == 1 {
		if targetStart, targetEnd, parentPath, ferr := pr.ResolveArchivePathBST(includes[0]); ferr == nil {
			if werr := pr.walkRange(cb, targetStart, targetEnd, parentPath, true); werr == nil {
				return extracted, nil
			}
			// Fast-path walk itself failed partway through (rare) — restart
			// clean with the full linear walk rather than return a partial,
			// possibly-confusing result.
			extracted = extracted[:0]
			pr.doneBytes.Store(0)
		}
	}

	err := pr.walk(cb)
	return extracted, err
}

// ExtractWithRewriterParallel is ExtractWithRewriter's opt-in, experimental
// twin (added 2026-09-23 — see Config.ParallelRestore): same semantics, same
// zip-slip/overwrite/include rules, but each file's slow per-file filesystem
// work (CreateTemp/Copy/Close/Rename/Chtimes — measured as the dominant cost
// once chunk-fetch prefetch made network no longer the bottleneck) runs on a
// bounded worker pool instead of one file at a time.
//
// NOT confirmed to actually be faster: a real test on the test VM (2 vCPUs,
// virtualized storage) measured 28s wall-clock for this path vs 22s for the
// sequential one on the same archive — slower, not faster, most likely
// filesystem/lock contention from concurrent CreateTemp/Rename/Chtimes
// outweighing the parallelism gain on that specific environment. This is
// exactly why the setting defaults to off and exists as an opt-in rather
// than replacing the sequential path: it may help on different hardware
// (more cores, faster/non-virtualized storage) but should not be assumed to
// help in general.
//
// This is safe because each file's payload is an independent io.SectionReader
// over the archive's underlying io.ReaderAt (see the PXAR_PAYLOAD case in
// walk) — reading it later, from a different goroutine, doesn't depend on the
// walk's own position. When that underlying reader is a DIDXReaderAt (the
// restore path), concurrent ReadAt calls are already safe by design (the
// inflight map coalesces concurrent requests for the same chunk — see
// TestDIDXReaderAt_ConcurrentReadersCoalesce).
//
// Deliberately NOT sharing code with ExtractWithRewriter: the two are kept
// fully independent so enabling this can never change what the proven
// sequential path does, and disabling it always reverts to exactly that
// path's existing, unmodified behavior.
//
// Directory creation, the archive walk itself, and the overwrite-exists
// check all stay on the walking goroutine (cheap, and a file's parent
// directory must exist before a worker can write into it) — only the slow
// per-file work is handed to workers. workers <= 0 defaults to 4.
func (pr *PXARReader) ExtractWithRewriterParallel(rewriter PathRewriter, includePaths []string, overwrite bool, workers int) ([]PXARExtractedFile, error) {
	if rewriter == nil {
		return nil, fmt.Errorf("path rewriter required")
	}
	if workers <= 0 {
		workers = 4
	}
	includes := NormalizeIncludes(includePaths)

	type fileJob struct {
		entry    PXARTreeEntry
		fullPath string
		payload  *io.SectionReader
	}

	var mu sync.Mutex
	extracted := make([]PXARExtractedFile, 0, 64)
	appendResult := func(r PXARExtractedFile) {
		mu.Lock()
		extracted = append(extracted, r)
		mu.Unlock()
	}

	var jobs chan fileJob
	var wg sync.WaitGroup
	startPool := func() {
		jobs = make(chan fileJob, workers*2)
		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func(jobs chan fileJob) {
				defer wg.Done()
				for j := range jobs {
					if pr.onFile != nil {
						pr.onFile(j.entry.Path)
					}
					appendResult(pr.extractOneFileParallel(j.entry, j.fullPath, j.payload))
					pr.entryDone(j.entry.Weight)
				}
			}(jobs)
		}
	}
	stopPool := func() {
		close(jobs)
		wg.Wait()
	}
	startPool()

	var queued bool
	extractEntry := func(e PXARTreeEntry, payload *io.SectionReader) error {
		if !pathMatches(e.Path, includes) {
			return nil
		}
		if isUnsafeArchivePath(e.Path) {
			appendResult(PXARExtractedFile{
				Path: e.Path, IsDir: e.IsDir,
				Skipped: true, SkipReason: "unsafe archive path refused (possible traversal)",
			})
			return nil
		}
		fullPath := rewriter(e.Path)
		if fullPath == "" {
			return nil
		}
		if e.IsDir {
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				appendResult(PXARExtractedFile{
					Path: fullPath, IsDir: true,
					Skipped: true, SkipReason: fmt.Sprintf("mkdir: %v", err),
				})
				return nil
			}
			appendResult(PXARExtractedFile{
				Path: fullPath, ArchivePath: e.Path, IsDir: true,
				Mode: os.FileMode(e.Mode & 0777), ModTime: e.ModTime,
			})
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			appendResult(PXARExtractedFile{
				Path: fullPath, Size: e.Size,
				Skipped: true, SkipReason: fmt.Sprintf("mkdir parent: %v", err),
			})
			return nil
		}
		if !overwrite {
			if _, err := os.Stat(fullPath); err == nil {
				appendResult(PXARExtractedFile{
					Path: fullPath, Size: e.Size,
					Skipped: true, Expected: true, SkipReason: "already exists",
				})
				return nil
			}
		}
		queued = true
		jobs <- fileJob{entry: e, fullPath: fullPath, payload: payload}
		return nil
	}
	// A queued file is counted by its worker once written; everything else
	// (directories, skips, filtered entries) is complete as soon as the
	// walker has handled it. The walker calls cb serially, so queued is not
	// shared across goroutines.
	cb := func(e PXARTreeEntry, payload *io.SectionReader) error {
		queued = false
		if err := extractEntry(e, payload); err != nil {
			return err
		}
		if !queued {
			pr.entryDone(e.Weight)
		}
		return nil
	}

	// Same GOODBYE-BST fast path as ExtractWithRewriter: a single clean
	// selection is walked inside its own span only. Without this, a selective
	// restore with parallel extraction scanned the ENTIRE archive after the
	// selection (found live 2026-10-01: 129,662-chunk walk of a 458GB snapshot
	// for a 1,698-chunk folder). Any failure falls back to the full walk.
	fastDone := false
	if len(includes) == 1 {
		if targetStart, targetEnd, parentPath, ferr := pr.ResolveArchivePathBST(includes[0]); ferr == nil {
			if werr := pr.walkRange(cb, targetStart, targetEnd, parentPath, true); werr == nil {
				fastDone = true
			} else {
				stopPool()
				mu.Lock()
				extracted = extracted[:0]
				mu.Unlock()
				pr.doneBytes.Store(0)
				startPool()
			}
		}
	}
	var walkErr error
	if !fastDone {
		walkErr = pr.walk(cb)
	}

	stopPool()

	return extracted, walkErr
}

// extractOneFileParallel does the actual CreateTemp/Copy/Close/Rename/Chtimes
// work for one file, run concurrently by ExtractWithRewriterParallel's worker
// pool. Mirrors ExtractWithRewriter's own inline file-writing body exactly
// (same temp-then-rename safety, same zip-slip protections already applied by
// the caller) — duplicated rather than shared, deliberately (see
// ExtractWithRewriterParallel's doc comment).
func (pr *PXARReader) extractOneFileParallel(e PXARTreeEntry, fullPath string, payload *io.SectionReader) PXARExtractedFile {
	fsStart := time.Now()
	out, err := os.CreateTemp(filepath.Dir(fullPath), filepath.Base(fullPath)+".proxmox-*.part")
	pr.fsOverheadNanos.Add(int64(time.Since(fsStart)))
	if err != nil {
		return PXARExtractedFile{
			Path: fullPath, Size: e.Size,
			Skipped: true, SkipReason: fmt.Sprintf("open: %v", err),
		}
	}
	tmpPath := out.Name()
	_ = out.Chmod(os.FileMode(e.Mode & 0777))

	copyStart := time.Now()
	sum, copyErr := pr.copyPayload(out, payload)
	pr.copyNanos.Add(int64(time.Since(copyStart)))

	fsStart = time.Now()
	closeErr := out.Close()
	pr.fsOverheadNanos.Add(int64(time.Since(fsStart)))

	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return PXARExtractedFile{
			Path: fullPath, Size: e.Size,
			Skipped: true, SkipReason: fmt.Sprintf("write: %v", copyErr),
		}
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return PXARExtractedFile{
			Path: fullPath, Size: e.Size,
			Skipped: true, SkipReason: fmt.Sprintf("close: %v", closeErr),
		}
	}

	fsStart = time.Now()
	renErr := os.Rename(tmpPath, fullPath)
	pr.fsOverheadNanos.Add(int64(time.Since(fsStart)))
	if renErr != nil {
		_ = os.Remove(tmpPath)
		return PXARExtractedFile{
			Path: fullPath, Size: e.Size,
			Skipped: true, SkipReason: fmt.Sprintf("rename: %v", renErr),
		}
	}
	if e.ModTime != 0 { // negative = pre-1970 mtime, still valid
		fsStart = time.Now()
		t := time.Unix(e.ModTime, 0)
		_ = os.Chtimes(fullPath, t, t)
		pr.fsOverheadNanos.Add(int64(time.Since(fsStart)))
	}
	return PXARExtractedFile{
		Path: fullPath, ArchivePath: e.Path, Size: e.Size,
		Mode: os.FileMode(e.Mode & 0777), ModTime: e.ModTime, SHA256: sum,
	}
}

// isUnsafeArchivePath reports whether a PXAR entry path could escape the restore
// destination (zip-slip). Archive paths are normalised forward-slash relative
// paths; anything absolute (unix root, UNC \\server), a Windows drive (C:/...),
// or containing a ".." segment is refused so rewriter(root + path) stays inside
// root.
func isUnsafeArchivePath(p string) bool {
	if p == "" {
		return false
	}
	q := strings.ReplaceAll(p, "\\", "/")
	if strings.HasPrefix(q, "/") {
		return true // unix-absolute, or UNC \\server\share
	}
	if len(q) >= 2 && q[1] == ':' {
		return true // windows drive-absolute, e.g. C:/...
	}
	for _, seg := range strings.Split(q, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

// NormalizeIncludes converts user-supplied include paths to the archive form
// expected by the reader: forward slashes, no leading/trailing slash, empty
// strings dropped. Exported so callers (e.g. flat-mode rewriters) can derive
// helpers from the same normalized list the walker sees.
func NormalizeIncludes(in []string) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		p = strings.ReplaceAll(p, "\\", "/")
		p = strings.Trim(p, "/")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// pathMatches returns true when path should be extracted given the include list.
// A path matches when it is one of the includes, a descendant of an include, or
// an ancestor of an include (so parent directories are created as needed).
func pathMatches(path string, includes []string) bool {
	if len(includes) == 0 {
		return true
	}
	for _, inc := range includes {
		if path == inc {
			return true
		}
		if strings.HasPrefix(path, inc+"/") {
			return true
		}
		if strings.HasPrefix(inc, path+"/") {
			return true
		}
	}
	return false
}
