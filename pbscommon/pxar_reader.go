package pbscommon

import (
	"bytes"
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
	pr.reset()
	var pathStack []string
	currentPath := ""
	pendingName := ""
	var pendingFileMode uint64
	var pendingFileMtime uint64
	hasPendingFile := false
	rootSeen := false

	for {
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
					if err := cb(PXARTreeEntry{
						Path:    subPath,
						IsDir:   true,
						Mode:    uint32(mode),
						ModTime: int64(mtimeSecs),
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
				if err := cb(PXARTreeEntry{
					Path:    path,
					IsDir:   false,
					Size:    uint64(contentSize),
					Mode:    uint32(pendingFileMode),
					ModTime: int64(pendingFileMtime),
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

	err := pr.walk(func(e PXARTreeEntry, payload *io.SectionReader) error {
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
		_, copyErr := io.Copy(out, payload)
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
		if e.ModTime > 0 {
			fsStart = time.Now()
			t := time.Unix(e.ModTime, 0)
			_ = os.Chtimes(fullPath, t, t)
			pr.fsOverheadNanos.Add(int64(time.Since(fsStart)))
		}
		extracted = append(extracted, PXARExtractedFile{
			Path: fullPath, ArchivePath: e.Path, Size: e.Size,
			Mode: os.FileMode(e.Mode & 0777), ModTime: e.ModTime,
		})
		return nil
	})
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

	jobs := make(chan fileJob, workers*2)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := range jobs {
				appendResult(pr.extractOneFileParallel(j.entry, j.fullPath, j.payload))
			}
		}()
	}

	walkErr := pr.walk(func(e PXARTreeEntry, payload *io.SectionReader) error {
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
		jobs <- fileJob{entry: e, fullPath: fullPath, payload: payload}
		return nil
	})

	close(jobs)
	wg.Wait()

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
	_, copyErr := io.Copy(out, payload)
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
	if e.ModTime > 0 {
		fsStart = time.Now()
		t := time.Unix(e.ModTime, 0)
		_ = os.Chtimes(fullPath, t, t)
		pr.fsOverheadNanos.Add(int64(time.Since(fsStart)))
	}
	return PXARExtractedFile{
		Path: fullPath, ArchivePath: e.Path, Size: e.Size,
		Mode: os.FileMode(e.Mode & 0777), ModTime: e.ModTime,
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
