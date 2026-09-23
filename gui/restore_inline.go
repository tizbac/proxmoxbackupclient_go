package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"pbscommon"
)

// RestoreMode picks where extracted files land on disk.
//
// Original restores back to the original filesystem location captured in the
// backup metadata sidecar (requires hostname + OS to match). The two Alternate
// modes write under opts.DestPath: Abs preserves the archive's full directory
// layout, Flat strips the longest common prefix of the user's selection so a
// single file lands at dest/<basename>.
type RestoreMode string

const (
	RestoreModeOriginal      RestoreMode = "original"
	RestoreModeAlternateAbs  RestoreMode = "alternate_abs"
	RestoreModeAlternateFlat RestoreMode = "alternate_flat"
)

// RestoreOptions contains all parameters for a restore operation.
//
// IncludePaths is the list of archive-relative paths to extract. Empty means
// "extract everything in the snapshot". Selecting a directory implies all
// descendants. Paths use forward slashes (archive style); backslashes are
// accepted and normalized.
//
	// RestoreACLs / RestoreADS / RestoreTimestamps are reserved for the upcoming
	// NTFS sidecar work — accepted today so the API surface is stable, but only
	// RestoreTimestamps has any effect (always-on: mtime is restored). The other
	// two are no-ops until the per-file .proxmox_meta sidecar lands.
type RestoreOptions struct {
	BaseURL         string
	AuthID          string
	Secret          string
	Ticket          string // PBS session ticket (u/p login); preferred over AuthID/Secret when set
	CSRFToken       string
	Datastore       string
	Namespace       string
	CertFingerprint string
	BackupID        string
	SnapshotTime    time.Time
	DestPath        string

	// Mode selects the destination policy. Empty defaults to alternate_abs
	// (legacy behaviour: dest + full archive path).
	Mode RestoreMode

	// AllowCrossHost permits an in-place restore even when the snapshot was
	// taken on a different machine. Honored only in RestoreModeOriginal.
	AllowCrossHost bool

	IncludePaths      []string
	Overwrite         bool
	RestoreACLs       bool // reserved — requires NTFS sidecar
	RestoreADS        bool // reserved — requires NTFS sidecar
	RestoreTimestamps bool // mtime is always restored; flag kept for symmetry

	// Ctx, when set, bounds the whole restore: cancelling it aborts an
	// in-flight chunk fetch immediately (see DIDXReaderAt.SetContext) instead
	// of only stopping between chunks. RestoreSnapshotInline sets this itself
	// via newRestoreContext() when the caller leaves it nil, mirroring how
	// BackupOptions.Ctx/newBackupContext work for backups (added 2026-09-22
	// after a live restore hung forever with no way to cancel it — see
	// GetChunkData's doc comment for the underlying bug).
	Ctx context.Context

	OnProgress func(percent float64, message string)

	// OnStats delivers structured live progress (bytes transferred) so the GUI
	// can show a real transfer rate, mirroring BackupOptions.OnStats. Restore
	// has no chunk-reuse/failure concept (every needed chunk is downloaded
	// fresh from the reader's perspective), so this is narrower than
	// BackupProgressStats — just bytes done/total and which archive is
	// currently being fetched.
	OnStats func(*RestoreProgressStats)
}

// RestoreProgressStats is the structured payload behind RestoreOptions.OnStats.
// BytesDone/BytesTotal are estimated from chunk-fetch counts against the
// archive's total size (chunks vary somewhat in size, so this is an
// approximation, not an exact byte count) — good enough for a progress UI's
// transfer-rate display, the same standard BackupProgressStats is held to.
type RestoreProgressStats struct {
	BytesDone      uint64 `json:"bytes_done"`
	BytesTotal     uint64 `json:"bytes_total"`
	CurrentArchive string `json:"current_archive,omitempty"`
}

var (
	currentRestoreCancelMutex sync.Mutex
	currentRestoreCancel      context.CancelFunc
)

// CancelRestore requests a graceful stop of the currently running restore.
// Mirrors CancelBackup exactly. Exported to the GUI.
func (a *App) CancelRestore() error {
	currentRestoreCancelMutex.Lock()
	defer currentRestoreCancelMutex.Unlock()
	if currentRestoreCancel != nil {
		currentRestoreCancel()
		writeDebugLog("CancelRestore: cancellation requested for running restore")
		return nil
	}
	writeDebugLog("CancelRestore: no restore running (or already cancelled)")
	return nil
}

// newRestoreContext returns a fresh cancellable context and registers its
// cancel function as the current in-flight restore. Mirrors newBackupContext.
func newRestoreContext() (context.Context, context.CancelFunc) {
	currentRestoreCancelMutex.Lock()
	defer currentRestoreCancelMutex.Unlock()
	if currentRestoreCancel != nil {
		currentRestoreCancel() // cancel any previous run before replacing
	}
	ctx, cancel := context.WithCancel(context.Background())
	currentRestoreCancel = cancel
	return ctx, cancel
}

// doneRestoreContext clears the shared cancellation token when a run finishes.
func doneRestoreContext() {
	currentRestoreCancelMutex.Lock()
	defer currentRestoreCancelMutex.Unlock()
	currentRestoreCancel = nil
}

// SnapshotInfo contains information about a backup snapshot.
type SnapshotInfo struct {
	BackupType string
	BackupID   string
	BackupTime time.Time
	Size       int64
	Owner      string
	Protected  bool
	Files      []string
}

// SnapshotEntry is a single file or directory inside a snapshot, suitable for
// driving a tree view in the GUI.
type SnapshotEntry struct {
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    uint64 `json:"size"`
	ModTime int64  `json:"mtime"`
}

// ListSnapshotsInline lists available snapshots from PBS.
// SECURITY: Only lists snapshots from the specified PBS server/datastore/namespace
// to prevent cross-server snapshot access.
func ListSnapshotsInline(baseURL, authID, secret, ticket, csrf, datastore, namespace, certFingerprint, backupID string) ([]SnapshotInfo, error) {
	writeBackupLog(fmt.Sprintf("Listing snapshots for backup ID: %s on %s/%s/%s", backupID, baseURL, datastore, namespace))

	client := &pbscommon.PBSClient{
		BaseURL:          baseURL,
		CertFingerPrint:  certFingerprint,
		AuthID:           authID,
		Secret:           secret,
		Ticket:           ticket,
		CSRFToken:        csrf,
		Datastore:        datastore,
		Namespace:        namespace,
		Insecure:         certFingerprint != "",
		CompressionLevel: pbscommon.CompressionFastest,
		Manifest: pbscommon.BackupManifest{
			BackupID: backupID,
		},
	}

	manifests, err := client.ListSnapshots()
	if err != nil {
		writeBackupLog(fmt.Sprintf("Failed to list snapshots: %v", err))
		return nil, fmt.Errorf("failed to list snapshots: %v", err)
	}

	result := make([]SnapshotInfo, 0)
	for _, m := range manifests {
		// Partial match supports split backups: searching "JDS-SRV-1" matches
		// "JDS-SRV-1_D_DATA" or "JDS-SRV-1_PART-A".
		if backupID != "" && !strings.Contains(m.BackupID, backupID) {
			continue
		}

		info := SnapshotInfo{
			BackupType: m.BackupType,
			BackupID:   m.BackupID,
			BackupTime: time.Unix(m.BackupTime, 0),
			Size:       m.Size,
			Owner:      m.Owner,
			Protected:  m.Protected,
			Files:      make([]string, 0, len(m.Files)),
		}
		for _, f := range m.Files {
			info.Files = append(info.Files, f.Filename)
		}
		result = append(result, info)
	}

	writeBackupLog(fmt.Sprintf("Found %d snapshots", len(result)))
	return result, nil
}

// resolveArchiveNames returns the real data-archive filenames (".pxar.didx")
// actually present in this snapshot's manifest, in manifest order.
//
// This exists because every "backup.pxar.didx" default and hardcoded literal
// in this file predates the 2026-09-21 single-session multi-archive backend
// restructure: before that, a directory backup always produced exactly one
// archive, always named literally "backup.pxar.didx", so a hardcoded default
// was harmless. It no longer is — every archive since that restructure is
// named after the source folder (archiveBaseName in backup_inline.go), so
// opening "backup.pxar.didx" directly (as opposed to going through the
// catalog, which stores the real name and doesn't care what it's called)
// fails with a dynamic-index-not-found error. Browsing/listing/meta-reading
// were unaffected because assembleSnapshotTree's catalog fast path never
// needed the literal name to be real — only RestoreSnapshotInline's own final
// extraction step opened an index directly by that hardcoded name.
//
// Filters out non-pxar manifest entries (the ACL/status blobs, index.json
// itself) by suffix — no other file type in this fork's manifests ends in
// ".pxar.didx".
func resolveArchiveNames(opts RestoreOptions) ([]string, error) {
	client := &pbscommon.PBSClient{
		BaseURL:          opts.BaseURL,
		CertFingerPrint:  opts.CertFingerprint,
		AuthID:           opts.AuthID,
		Secret:           opts.Secret,
		Ticket:           opts.Ticket,
		CSRFToken:        opts.CSRFToken,
		Datastore:        opts.Datastore,
		Namespace:        opts.Namespace,
		Insecure:         opts.CertFingerprint != "",
		CompressionLevel: pbscommon.CompressionFastest,
	}
	manifests, err := client.ListSnapshots()
	if err != nil {
		return nil, fmt.Errorf("failed to look up snapshot manifest: %w", err)
	}
	target := opts.SnapshotTime.Unix()
	for _, m := range manifests {
		if m.BackupID != opts.BackupID || m.BackupTime != target {
			continue
		}
		names := make([]string, 0, len(m.Files))
		for _, f := range m.Files {
			if strings.HasSuffix(f.Filename, ".pxar.didx") {
				names = append(names, f.Filename)
			}
		}
		if len(names) == 0 {
			return nil, fmt.Errorf("snapshot manifest has no data archive (.pxar.didx) — nothing to restore")
		}
		return names, nil
	}
	return nil, fmt.Errorf("snapshot %s/%s not found while resolving its archive name",
		opts.BackupID, opts.SnapshotTime.Format(time.RFC3339))
}

// withSnapshotReader opens a snapshot archive over a LAZY chunk-backed reader and
// hands it to fn. Chunks are fetched from PBS on demand (with an LRU cache) as fn
// reads, instead of downloading and reassembling the whole archive into a temp
// file first. This removes the "free %TEMP% space == archive size" requirement
// and lets selective restore skip the chunks of files the caller did not select.
// The PBS reader session stays open for the duration of fn.
func withSnapshotReader(opts RestoreOptions, archiveName, logTag string, progress func(done, total int, bytesDone, bytesTotal int64), cancel func() bool, fn func(*pbscommon.PXARReader) error) error {
	if archiveName == "" {
		archiveName = "backup.pxar.didx"
	}
	if opts.BaseURL == "" || !((opts.AuthID != "" && opts.Secret != "") || opts.Ticket != "") {
		return fmt.Errorf("PBS connection parameters required")
	}
	if opts.BackupID == "" {
		return fmt.Errorf("backup ID required")
	}
	if opts.Datastore == "" {
		return fmt.Errorf("datastore required")
	}

	client := &pbscommon.PBSClient{
		BaseURL:          opts.BaseURL,
		CertFingerPrint:  opts.CertFingerprint,
		AuthID:           opts.AuthID,
		Secret:           opts.Secret,
		Datastore:        opts.Datastore,
		Namespace:        opts.Namespace,
		Insecure:         opts.CertFingerprint != "",
		CompressionLevel: pbscommon.CompressionFastest,
		Manifest: pbscommon.BackupManifest{
			BackupID:   opts.BackupID,
			BackupTime: opts.SnapshotTime.Unix(),
		},
	}
	client.Connect(true, "host")
	defer client.Close()

	// archiveSize is set right below, before any chunk fetch can happen
	// (NewDIDXReaderAt only downloads the index and returns — chunks are
	// fetched lazily by fn's later reads) — safe for this closure to capture.
	var archiveSize int64
	ra, size, err := client.NewDIDXReaderAt(archiveName, 64, func(fetched, total int) {
		if fetched == total || fetched%32 == 0 {
			writeBackupLog(fmt.Sprintf("%s: fetched %d/%d chunks of %s", logTag, fetched, total, archiveName))
		}
		if progress != nil {
			var bytesDone int64
			if total > 0 {
				bytesDone = int64(float64(archiveSize) * float64(fetched) / float64(total))
			}
			progress(fetched, total, bytesDone, archiveSize)
		}
	})
	if err != nil {
		writeBackupLog(fmt.Sprintf("Failed to open snapshot reader (%s): %v", logTag, err))
		return fmt.Errorf("failed to open snapshot archive: %w", err)
	}
	archiveSize = size
	// Bounds each individual chunk fetch (chunkFetchTimeout) and lets a
	// cancelled opts.Ctx abort one that's already in flight — see
	// GetChunkData's doc comment. Safe to call even when opts.Ctx is nil
	// (SetContext falls back to context.Background()).
	ra.SetContext(opts.Ctx)
	// Combine the caller's own cancel predicate (if any, e.g. a cross-snapshot
	// search's own stop button) with the restore context so EITHER aborts
	// between-chunk reads immediately, not just a hung in-flight one.
	ra.SetCancelCheck(func() bool {
		if cancel != nil && cancel() {
			return true
		}
		return opts.Ctx != nil && opts.Ctx.Err() != nil
	})

	return fn(pbscommon.NewPXARReaderAt(ra, size))
}

// listSnapshotViaCatalog lists a snapshot's file tree from the compact
// catalog.pcat1.didx instead of walking the multi-GB data archive. The catalog
// holds only names, sizes and mtimes, so a full listing fetches a few MB rather
// than re-reading the whole backup — the difference between seconds and hours
// on large datastores.
//
// Returns ok=false (with no error) when the snapshot has no usable catalog
// (legacy snapshots predating catalog upload, or a parse failure) so the caller
// can fall back to the PXAR walk. meta is read separately and cheaply from the
// start of the data archive; it is best-effort and may be nil.
func listSnapshotViaCatalog(opts RestoreOptions, cancel func() bool) (entries []SnapshotEntry, meta *BackupMeta, ok bool) {
	client := &pbscommon.PBSClient{
		BaseURL:          opts.BaseURL,
		CertFingerPrint:  opts.CertFingerprint,
		AuthID:           opts.AuthID,
		Secret:           opts.Secret,
		Datastore:        opts.Datastore,
		Namespace:        opts.Namespace,
		Insecure:         opts.CertFingerprint != "",
		CompressionLevel: pbscommon.CompressionFastest,
		Manifest: pbscommon.BackupManifest{
			BackupID:   opts.BackupID,
			BackupTime: opts.SnapshotTime.Unix(),
		},
	}
	client.Connect(true, "host")
	defer client.Close()

	ra, size, err := client.NewDIDXReaderAt("catalog.pcat1.didx", 64, nil)
	if err != nil {
		writeBackupLog(fmt.Sprintf("Catalog unavailable for %s@%d (%v), falling back to data-archive walk",
			opts.BackupID, opts.SnapshotTime.Unix(), err))
		return nil, nil, false
	}
	ra.SetContext(opts.Ctx)
	ra.SetCancelCheck(func() bool {
		if cancel != nil && cancel() {
			return true
		}
		return opts.Ctx != nil && opts.Ctx.Err() != nil
	})

	buf := make([]byte, size)
	if _, rerr := ra.ReadAt(buf, 0); rerr != nil && rerr != io.EOF {
		writeBackupLog(fmt.Sprintf("Catalog read failed for %s@%d: %v, falling back", opts.BackupID, opts.SnapshotTime.Unix(), rerr))
		return nil, nil, false
	}

	catEntries, perr := pbscommon.ParseCatalog(buf)
	if perr != nil {
		writeBackupLog(fmt.Sprintf("Catalog parse failed for %s@%d: %v, falling back", opts.BackupID, opts.SnapshotTime.Unix(), perr))
		return nil, nil, false
	}

	// Every entry is now prefixed by its own archive's RAW name (e.g.
	// "c__testdata_backup_of_compaq_486_laptop.pxar.didx/BLOCKB") — see
	// ParseCatalog's doc comment. Collect the distinct top-level archive
	// names so they can be remapped to a human-readable one derived from each
	// archive's own meta sidecar, instead of showing the internal slug (or,
	// for a genuinely multi-folder job, showing several archives' contents
	// unlabelled and merged together — the actual bug this fixes).
	archiveNameSet := make(map[string]bool)
	for _, e := range catEntries {
		if e.IsDir && !strings.Contains(e.Path, "/") {
			archiveNameSet[e.Path] = true
		}
	}
	archiveNames := make([]string, 0, len(archiveNameSet))
	for name := range archiveNameSet {
		archiveNames = append(archiveNames, name)
	}
	displayNames, metaByArchive := resolveArchiveDisplayNames(opts, archiveNames, cancel)

	entries = make([]SnapshotEntry, 0, len(catEntries))
	for _, e := range catEntries {
		archiveName, rest, _ := strings.Cut(e.Path, "/")
		display := displayNames[archiveName]
		if display == "" {
			display = archiveName
		}
		path := display
		if rest != "" {
			path = display + "/" + rest
		}
		entries = append(entries, SnapshotEntry{Path: path, IsDir: e.IsDir, Size: e.Size, ModTime: e.ModTime})
	}

	// Origin banner meta: best-effort, whichever archive has one. A genuinely
	// per-archive banner (for a real multi-folder job) would need real UI
	// work beyond tonight's scope — this keeps the existing single-banner
	// behavior, now sourced from resolveArchiveDisplayNames' own reads
	// instead of a separate readSnapshotMetaCheap call.
	for _, name := range archiveNames {
		if m := metaByArchive[name]; m != nil {
			meta = m
			break
		}
	}

	writeBackupLog(fmt.Sprintf("Catalog listing for %s@%d: %d entries via fast path (%d catalog bytes, %d archive(s))",
		opts.BackupID, opts.SnapshotTime.Unix(), len(entries), size, len(archiveNames)))
	return entries, meta, true
}

// resolveArchiveDisplayNames maps each archive's own raw name (as stored in
// the manifest/catalog, e.g. "c__testdata_....pxar.didx") to a human-readable
// display name derived from that archive's own meta sidecar
// (filepath.Base(OriginalPath) — the originally-selected folder's own name,
// which PXAR's archive format itself never records; see WriteDir's toplevel
// doc comment). Falls back to the raw archive name when no meta / no
// OriginalPath is available (legacy snapshots, or a best-effort read
// failure). Collisions (two archives whose original folders share a
// basename) are disambiguated with a " (2)", " (3)", ... suffix, walked in
// sorted archive-name order so the mapping is deterministic — this exact
// function is called again at restore time (RestoreSnapshotInline) to
// translate the GUI's displayed selection back to real archive-relative
// paths, so both calls must produce an identical mapping for the same
// snapshot.
func resolveArchiveDisplayNames(opts RestoreOptions, archiveNames []string, cancel func() bool) (displayNames map[string]string, metaByArchive map[string]*BackupMeta) {
	sorted := append([]string(nil), archiveNames...)
	sort.Strings(sorted)

	displayNames = make(map[string]string, len(sorted))
	metaByArchive = make(map[string]*BackupMeta, len(sorted))
	used := make(map[string]bool, len(sorted))

	for _, archiveName := range sorted {
		meta := readArchiveMetaCheap(opts, archiveName, cancel)
		metaByArchive[archiveName] = meta

		name := archiveName
		if meta != nil && meta.OriginalPath != "" {
			if base := filepath.Base(filepath.Clean(meta.OriginalPath)); base != "" && base != "." && base != string(filepath.Separator) {
				name = base
			}
		}
		final := name
		for n := 2; used[final]; n++ {
			final = fmt.Sprintf("%s (%d)", name, n)
		}
		used[final] = true
		displayNames[archiveName] = final
	}
	return displayNames, metaByArchive
}

// readArchiveMetaCheap reads only the meta sidecar from a specific archive.
// A multi-folder job's snapshot has one archive per originally-selected
// folder, each carrying its OWN meta sidecar (GenerateBackupMeta is called
// per-directory at backup time — see backup_inline.go) — needed to reconstruct
// each archive's own original folder name on restore (see
// wrapRewriterForArchive) and for browse-tree display (see
// resolveArchiveDisplayNames). PXARReader.ReadVirtualFile stops walking as
// soon as the root-level sidecar is found, and since it is written before any
// real file content the walk fetches just the first chunk(s). Best-effort:
// any failure returns nil.
func readArchiveMetaCheap(opts RestoreOptions, archiveName string, cancel func() bool) *BackupMeta {
	var meta *BackupMeta
	err := withSnapshotReader(opts, archiveName, "MetaCheap", nil, cancel, func(reader *pbscommon.PXARReader) error {
		meta = tryReadBackupMeta(reader)
		return nil
	})
	if err != nil {
		writeBackupLog(fmt.Sprintf("Cheap meta read failed for %s@%d archive=%s: %v", opts.BackupID, opts.SnapshotTime.Unix(), archiveName, err))
		return nil
	}
	return meta
}

// assembleSnapshotTree returns a snapshot's full file tree plus its meta
// sidecar, preferring the compact catalog and falling back to walking the data
// archive only for snapshots without a usable catalog. Callers are responsible
// for caching the result.
//
// The catalog only describes the default data archive (backup.pxar.didx), so
// the fast path is taken only for that archive; any other archiveName goes
// straight to the walk.
func assembleSnapshotTree(opts RestoreOptions, archiveName, logTag string, cancel func() bool) ([]SnapshotEntry, *BackupMeta, error) {
	if archiveName == "" {
		archiveName = "backup.pxar.didx"
	}
	if archiveName == "backup.pxar.didx" {
		if entries, meta, ok := listSnapshotViaCatalog(opts, cancel); ok {
			return entries, meta, nil
		}
		// A cancelled catalog read surfaces as ok=false (treated as "no catalog").
		// Bail before the expensive data-archive walk instead of falling back.
		if cancel != nil && cancel() {
			return nil, nil, pbscommon.ErrReadCancelled
		}
	}

	var entries []SnapshotEntry
	var meta *BackupMeta
	err := withSnapshotReader(opts, archiveName, logTag, nil, cancel, func(reader *pbscommon.PXARReader) error {
		es, lerr := reader.ListEntries()
		if lerr != nil {
			return fmt.Errorf("failed to parse archive: %v", lerr)
		}
		entries = make([]SnapshotEntry, 0, len(es))
		for _, e := range es {
			entries = append(entries, SnapshotEntry{Path: e.Path, IsDir: e.IsDir, Size: e.Size, ModTime: e.ModTime})
		}
		meta = tryReadBackupMeta(reader)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return entries, meta, nil
}

// buildSnapshotCacheKey is the canonical cache key for a given snapshot.
// Centralized so list/meta/restore all hit the same envelope.
func buildSnapshotCacheKey(opts RestoreOptions) snapshotCacheKey {
	return snapshotCacheKey{
		PBSID:      opts.BaseURL,
		Datastore:  opts.Datastore,
		Namespace:  opts.Namespace,
		BackupType: "host", // this client only ever creates host-type snapshots
		BackupID:   opts.BackupID,
		SnapshotAt: opts.SnapshotTime.Unix(),
	}
}

// ListSnapshotContentsInline downloads a snapshot's PXAR archive and returns
// its tree of entries (files + directories) without extracting anything to disk.
// Used by the GUI to power the restore navigation tree.
//
// Results are cached locally per snapshot — a snapshot's contents are immutable
// once written, so the cache never goes stale, only ages out. Set forceRefresh
// to bypass the cache (e.g. for a manual "Reload" button).
//
// As a side effect, the snapshot's `.proxmox_backup_client_meta.json` sidecar is parsed
// and cached too, so a subsequent ReadSnapshotMetaInline call is free.
//
// archiveName defaults to "backup.pxar.didx" when empty.
func ListSnapshotContentsInline(opts RestoreOptions, archiveName string, forceRefresh bool) ([]SnapshotEntry, error) {
	if archiveName == "" {
		archiveName = "backup.pxar.didx"
	}
	writeBackupLog(fmt.Sprintf("Listing contents: backupID=%s snapshot=%s archive=%s force=%v",
		opts.BackupID, opts.SnapshotTime.Format(time.RFC3339), archiveName, forceRefresh))

	cacheKey := buildSnapshotCacheKey(opts)
	if !forceRefresh {
		if cached, ok := loadSnapshotTreeCache(cacheKey); ok {
			writeBackupLog(fmt.Sprintf("Restore cache hit: %d entries (skipping download)", len(cached.Entries)))
			return cached.Entries, nil
		}
	}

	result, meta, err := assembleSnapshotTree(opts, archiveName, "Listing", nil)
	if err != nil {
		return nil, err
	}
	writeBackupLog(fmt.Sprintf("Listed %d entries in snapshot", len(result)))

	// Best-effort cache write — a failure here just means the next listing pays
	// the assembly cost again. Meta is cached alongside so a later
	// GetSnapshotMeta call doesn't re-download anything.
	if werr := saveSnapshotTreeCache(cacheKey, result, meta); werr != nil {
		writeBackupLog(fmt.Sprintf("Restore cache write failed: %v", werr))
	}

	return result, nil
}

// tryReadBackupMeta extracts the .proxmox_backup_client_meta.json sidecar from an
// already-parsed archive. Returns nil on any failure (legacy snapshots,
// corrupted JSON, missing file) — meta is informational, never fatal.
func tryReadBackupMeta(reader *pbscommon.PXARReader) *BackupMeta {
	raw, err := reader.ReadVirtualFile(BackupMetaFilename)
	if err != nil {
		// os.ErrNotExist is expected for legacy snapshots created before the
		// sidecar shipped — log at debug volume only.
		writeBackupLog(fmt.Sprintf("Backup meta sidecar not available: %v", err))
		return nil
	}
	var meta BackupMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		writeBackupLog(fmt.Sprintf("Backup meta sidecar malformed: %v", err))
		return nil
	}
	return &meta
}

// ReadSnapshotMetaInline returns the .proxmox_backup_client_meta.json sidecar stored
// at the root of a snapshot, or nil with a non-nil error when no sidecar is
// present (legacy snapshots created before the sidecar shipped).
//
// Hits the local cache first — if ListSnapshotContentsInline has run for this
// snapshot, the meta is already there and no download is performed. Otherwise
// the archive is downloaded + assembled (same cost as a listing).
func ReadSnapshotMetaInline(opts RestoreOptions, forceRefresh bool) (*BackupMeta, error) {
	if opts.BaseURL == "" || opts.AuthID == "" || opts.Secret == "" {
		return nil, fmt.Errorf("PBS connection parameters required")
	}
	if opts.BackupID == "" {
		return nil, fmt.Errorf("backup ID required")
	}
	if opts.Datastore == "" {
		return nil, fmt.Errorf("datastore required")
	}

	cacheKey := buildSnapshotCacheKey(opts)
	if !forceRefresh {
		if cached, ok := loadSnapshotTreeCache(cacheKey); ok {
			if cached.Meta != nil {
				writeBackupLog("Snapshot meta: cache hit")
				return cached.Meta, nil
			}
			// Cache exists but predates meta capture (or this snapshot has no
			// sidecar). Don't bother re-downloading just to confirm — caller
			// can pass forceRefresh=true explicitly if they want to retry.
			writeBackupLog("Snapshot meta: cache hit without meta — no sidecar in this snapshot")
			return nil, nil
		}
	}

	writeBackupLog(fmt.Sprintf("Snapshot meta: reading for backupID=%s snapshot=%s",
		opts.BackupID, opts.SnapshotTime.Format(time.RFC3339)))

	// Assemble via the catalog fast path when possible; this also returns the
	// tree, so refresh the full listing cache while we have it.
	entries, meta, err := assembleSnapshotTree(opts, "backup.pxar.didx", "Meta", nil)
	if err != nil {
		return nil, err
	}
	if werr := saveSnapshotTreeCache(cacheKey, entries, meta); werr != nil {
		writeBackupLog(fmt.Sprintf("Restore cache write failed: %v", werr))
	}

	return meta, nil
}

// buildPathRewriter validates the requested mode against the snapshot metadata
// (when needed) and returns a rewriter that maps archive paths to filesystem
// paths on this host.
//
// Validation rules:
//   - original: requires a meta sidecar with OriginalPath. OS must match
//     runtime.GOOS (no Windows-on-Linux). Hostname must match unless
//     opts.AllowCrossHost is set.
//   - alternate_abs / alternate_flat: requires opts.DestPath.
//   - flat with no IncludePaths is equivalent to abs — flat needs a selection
//     to derive a common root from.
func buildPathRewriter(opts RestoreOptions, meta *BackupMeta) (pbscommon.PathRewriter, error) {
	mode := opts.Mode
	if mode == "" {
		mode = RestoreModeAlternateAbs
	}

	switch mode {
	case RestoreModeOriginal:
		if meta == nil {
			return nil, fmt.Errorf("in-place restore not possible: this snapshot has no metadata (.proxmox_backup_client_meta.json missing), choose \"alternate location\" instead")
		}
		if meta.OriginalPath == "" {
			return nil, fmt.Errorf("in-place restore not possible: the original path is not set in the metadata")
		}
		if meta.OS != "" && meta.OS != runtime.GOOS {
			return nil, fmt.Errorf("in-place restore not possible: backup was made on %s, current machine is %s", meta.OS, runtime.GOOS)
		}
		if !opts.AllowCrossHost {
			localHost, err := os.Hostname()
			if err != nil {
				return nil, fmt.Errorf("could not read local hostname: %w", err)
			}
			if meta.Hostname != "" && !equalHostnames(meta.Hostname, localHost) {
				return nil, fmt.Errorf("in-place restore blocked: backup from %q, current machine %q — tick \"force cross-host\" if intentional", meta.Hostname, localHost)
			}
		}
		// Materialize the original root once, with native separators.
		root := filepath.Clean(meta.OriginalPath)
		return func(archivePath string) string {
			if archivePath == "" {
				return root
			}
			return filepath.Join(root, filepath.FromSlash(archivePath))
		}, nil

	case RestoreModeAlternateAbs:
		if opts.DestPath == "" {
			return nil, fmt.Errorf("destination folder required")
		}
		dest := opts.DestPath
		return func(archivePath string) string {
			return filepath.Join(dest, filepath.FromSlash(archivePath))
		}, nil

	case RestoreModeAlternateFlat:
		if opts.DestPath == "" {
			return nil, fmt.Errorf("destination folder required")
		}
		// Empty selection means "restore everything" — flat is meaningless,
		// fall back to abs so the user gets a sensible result instead of
		// hundreds of files colliding at the dest root.
		if len(opts.IncludePaths) == 0 {
			dest := opts.DestPath
			return func(archivePath string) string {
				return filepath.Join(dest, filepath.FromSlash(archivePath))
			}, nil
		}
		prefix := commonAncestorDir(pbscommon.NormalizeIncludes(opts.IncludePaths))
		dest := opts.DestPath
		return func(archivePath string) string {
			// Drop archive entries above the selection root (e.g. parent
			// directory entries the walker emits as scaffolding). The walker
			// also matches ancestors of includes for mkdir scaffolding —
			// skip those so they don't pollute the flat root.
			if archivePath == "" {
				return ""
			}
			if prefix == "" {
				return filepath.Join(dest, filepath.FromSlash(archivePath))
			}
			if archivePath == prefix {
				return ""
			}
			rel := strings.TrimPrefix(archivePath, prefix+"/")
			if rel == archivePath {
				// Not a descendant of the common prefix → ancestor scaffolding,
				// drop silently.
				return ""
			}
			return filepath.Join(dest, filepath.FromSlash(rel))
		}, nil

	default:
		return nil, fmt.Errorf("unknown restore mode: %q", string(mode))
	}
}

// wrapRewriterForArchive extends an alternate_abs rewriter so restored files
// land under dest/<original folder name>/... instead of dest/... directly.
//
// PXAR's own archive format has no concept of a named root: WriteDir(toplevel)
// on the originally-selected folder writes only that folder's CONTENTS as the
// archive's top level, never the folder's own name (see WriteDir's own doc
// comment). Without this, an alternate_abs restore of a single selected folder
// silently drops that folder's own name — e.g. selecting "Backup of Compaq 486
// Laptop" restores BLOCKB/CPC/DEFENDER/... directly under dest, with no
// "Backup of Compaq 486 Laptop" wrapper — which is genuinely surprising versus
// what the user selected. Each archive's own meta sidecar still has the
// original folder's full path (captured per-directory at backup time), so
// this reconstructs the name from there instead of touching the archive
// format itself.
//
// Only applies to alternate_abs. Original mode needs no help — meta.OriginalPath
// IS already the absolute restore root there. Alternate_flat's entire purpose
// is to NOT preserve structure, so adding a wrapper back in would fight its
// own point; it keeps its existing behavior unchanged.
//
// wrapper is the SAME name resolveArchiveDisplayNames computed for this
// archive (including its collision-disambiguation suffix, e.g. "Data (2)")
// — deliberately not re-derived here from meta.OriginalPath independently,
// which would let two archives whose original folders share a basename
// silently write into the same output subfolder.
func wrapRewriterForArchive(base pbscommon.PathRewriter, wrapper string) pbscommon.PathRewriter {
	if wrapper == "" {
		return base
	}
	return func(archivePath string) string {
		if archivePath == "" {
			return base(wrapper)
		}
		return base(wrapper + "/" + archivePath)
	}
}

// commonAncestorDir returns the longest path prefix shared by all includes,
// using forward-slash archive convention. Returns "" when the includes have
// no common directory (e.g. "a/x.txt" and "b/y.txt").
func commonAncestorDir(includes []string) string {
	if len(includes) == 0 {
		return ""
	}
	if len(includes) == 1 {
		// Single selection: parent dir is the common root. A single file
		// `a/b/c.txt` becomes flat as `c.txt`; a single dir `a/b/c` becomes
		// flat as `c/...` because the trim prefix is `a/b`.
		return parentDir(includes[0])
	}
	parts := strings.Split(includes[0], "/")
	for _, p := range includes[1:] {
		ps := strings.Split(p, "/")
		max := len(parts)
		if len(ps) < max {
			max = len(ps)
		}
		i := 0
		for i < max && parts[i] == ps[i] {
			i++
		}
		parts = parts[:i]
		if len(parts) == 0 {
			return ""
		}
	}
	return strings.Join(parts, "/")
}

func parentDir(archivePath string) string {
	i := strings.LastIndex(archivePath, "/")
	if i < 0 {
		return ""
	}
	return archivePath[:i]
}

// equalHostnames compares hostnames tolerant of case and trailing dot/domain
// (so "WIN-A" == "win-a" == "WIN-A.local").
func equalHostnames(a, b string) bool {
	norm := func(s string) string {
		s = strings.ToLower(s)
		if dot := strings.Index(s, "."); dot >= 0 {
			s = s[:dot]
		}
		return s
	}
	return norm(a) == norm(b)
}

// RestoreSnapshotInline restores a snapshot from PBS.
// SECURITY: Only restores from the configured PBS server/datastore/namespace.
// Snapshots from other servers will fail with HTTP 404.
//
// When opts.IncludePaths is non-empty, only the matching files and directories
// are extracted. Otherwise the whole snapshot is restored.
func RestoreSnapshotInline(opts RestoreOptions) error {
	// Wire a shared, user-cancellable context for this run (Stop/Cancel
	// button) — mirrors RunBackupInline's identical setup. Also bounds every
	// individual chunk fetch (see GetChunkData's doc comment for why that
	// matters on its own, independent of whether anyone clicks cancel).
	restoreCtx, restoreCancel := newRestoreContext()
	defer doneRestoreContext()
	if opts.Ctx == nil {
		opts.Ctx = restoreCtx
	}
	_ = restoreCancel

	mode := opts.Mode
	if mode == "" {
		mode = RestoreModeAlternateAbs
	}
	writeBackupLog(fmt.Sprintf("Starting restore: snapshot=%s, mode=%s, dest=%s, includes=%d, overwrite=%v, allowCrossHost=%v from %s/%s/%s",
		opts.SnapshotTime.Format("2006-01-02T15:04:05Z"), mode, opts.DestPath, len(opts.IncludePaths), opts.Overwrite, opts.AllowCrossHost,
		opts.BaseURL, opts.Datastore, opts.Namespace))

	progress := func(pct float64, msg string) {
		writeBackupLog(fmt.Sprintf("Restore progress: %.1f%% - %s", pct*100, msg))
		if opts.OnProgress != nil {
			opts.OnProgress(pct, msg)
		}
	}

	if opts.BaseURL == "" || !((opts.AuthID != "" && opts.Secret != "") || opts.Ticket != "") {
		return fmt.Errorf("PBS connection parameters required")
	}
	if opts.BackupID == "" {
		return fmt.Errorf("backup ID required")
	}
	if opts.Datastore == "" {
		return fmt.Errorf("datastore required for security")
	}
	// DestPath is required for alternate modes only — original reads the
	// target from the backup metadata sidecar.
	if mode != RestoreModeOriginal && opts.DestPath == "" {
		return fmt.Errorf("destination path required")
	}

	// In-place restores need the sidecar up front to validate before we burn
	// time on the multi-GB download. Cheap if the snapshot was listed first.
	var meta *BackupMeta
	if mode == RestoreModeOriginal {
		var err error
		meta, err = ReadSnapshotMetaInline(opts, false)
		if err != nil {
			return fmt.Errorf("lecture du sidecar pour restauration in-place: %w", err)
		}
		// Validate immediately so the user sees the cross-host / cross-OS
		// refusal before any chunk is downloaded.
		if _, err := buildPathRewriter(opts, meta); err != nil {
			return err
		}
		// In-place implies overwrite. Files in OriginalPath are by definition
		// candidates for replacement; skipping them would be confusing.
		opts.Overwrite = true
		writeBackupLog(fmt.Sprintf("In-place target: %s (host=%s, os=%s)", meta.OriginalPath, meta.Hostname, meta.OS))
	}

	progress(0.05, "Preparing restore...")

	// Build the rewriter before downloading so a misconfiguration (missing
	// dest, cross-host refusal) fails fast. For alternate modes meta is unused;
	// for in-place, meta was read above before download.
	rewriter, err := buildPathRewriter(opts, meta)
	if err != nil {
		return err
	}

	// Resolve the snapshot's REAL archive name(s) instead of assuming the old
	// single-archive "backup.pxar.didx" literal (see resolveArchiveNames' doc
	// comment) — one job's snapshot can hold several named archives since the
	// multi-archive backend restructure, and each needs its own extraction
	// pass. Ordinary single-folder backups (the common case) just resolve to
	// one name and loop once, exactly as before.
	archiveNames, err := resolveArchiveNames(opts)
	if err != nil {
		return err
	}
	writeBackupLog(fmt.Sprintf("Resolved %d archive(s) for this snapshot: %v", len(archiveNames), archiveNames))

	progress(0.20, "Downloading backup archive...")
	// AssembleDIDXToFile downloads the .didx index and reassembles the actual
	// PXAR stream chunk-by-chunk into a temp file (bounded memory), then we walk
	// it from disk and stream each file payload to its destination.
	// Same display names the browse tree showed (resolveArchiveDisplayNames
	// is deterministic given the same archive list, so this reproduces
	// exactly what listSnapshotViaCatalog computed when the user was
	// selecting paths) — needed to translate the GUI's wrapper-prefixed
	// selection back to real archive-relative paths below.
	displayNames, _ := resolveArchiveDisplayNames(opts, archiveNames, nil)

	var extracted []pbscommon.PXARExtractedFile
	includesToCheck := pbscommon.NormalizeIncludes(opts.IncludePaths)
	anyIncludesMatched := false
	archiveSpan := 0.60 / float64(len(archiveNames))
	for i, archiveName := range archiveNames {
		archiveBase := 0.20 + float64(i)*archiveSpan
		wrapper := displayNames[archiveName]

		// Translate this archive's slice of opts.IncludePaths back to real
		// archive-relative paths. The GUI's browse tree now shows every path
		// prefixed with the SAME display-name wrapper (see
		// resolveArchiveDisplayNames) — but the archive itself only ever
		// contains raw, unprefixed paths (PXAR's own root is always unnamed),
		// so passing the wrapped paths straight to ExtractWithRewriter would
		// never match anything via pathMatches, silently restoring nothing.
		// This applies to every restore mode, not just alternate_abs.
		var archiveIncludes []string
		referencedThisArchive := len(includesToCheck) == 0 // empty overall selection = everything, in every archive, unchanged from before this fix
		for _, inc := range includesToCheck {
			norm := strings.ReplaceAll(inc, "\\", "/")
			switch {
			case norm == wrapper:
				archiveIncludes = append(archiveIncludes, "")
				referencedThisArchive = true
			case strings.HasPrefix(norm, wrapper+"/"):
				archiveIncludes = append(archiveIncludes, strings.TrimPrefix(norm, wrapper+"/"))
				referencedThisArchive = true
			}
		}
		if !referencedThisArchive {
			// Nothing in the user's selection touches this archive at all —
			// skip it entirely. Passing an empty archiveIncludes here instead
			// would be wrong: ExtractWithRewriter treats an empty list as
			// "extract everything" (the pre-existing, unchanged convention
			// for a genuinely empty overall selection), which would silently
			// restore an archive the user never selected anything from.
			writeBackupLog(fmt.Sprintf("Skipping archive %s (%s): nothing in the selection references it", archiveName, wrapper))
			continue
		}

		// Reconstruct THIS archive's own originally-selected folder name on
		// disk (alternate_abs only — see wrapRewriterForArchive's doc
		// comment). Each archive has its own meta sidecar, so a multi-folder
		// job wraps each folder under its own name, not just a single global
		// one.
		archiveRewriter := rewriter
		if mode == RestoreModeAlternateAbs {
			archiveRewriter = wrapRewriterForArchive(rewriter, wrapper)
		}

		var extractedHere []pbscommon.PXARExtractedFile
		err = withSnapshotReader(opts, archiveName, "Restore", func(done, total int, bytesDone, bytesTotal int64) {
			// Map this archive's chunk progress to its own slice of the
			// 0.20–0.80 overall bar, so N archives fill it evenly instead of
			// each one restarting from 0.20.
			if total == 0 {
				return
			}
			pct := archiveBase + archiveSpan*(float64(done)/float64(total))
			progress(pct, fmt.Sprintf("Downloading %s (%d/%d chunks)", archiveName, done, total))
			if opts.OnStats != nil {
				opts.OnStats(&RestoreProgressStats{
					BytesDone:      uint64(bytesDone),
					BytesTotal:     uint64(bytesTotal),
					CurrentArchive: archiveName,
				})
			}
		}, nil, func(reader *pbscommon.PXARReader) error {
			progress(archiveBase+archiveSpan*0.9, fmt.Sprintf("Extracting %s...", archiveName))
			var eerr error
			extractedHere, eerr = reader.ExtractWithRewriter(archiveRewriter, archiveIncludes, opts.Overwrite)
			if eerr != nil {
				writeBackupLog(fmt.Sprintf("PXAR extraction failed for %s: %v", archiveName, eerr))
				return fmt.Errorf("failed to extract archive %s: %v", archiveName, eerr)
			}
			return nil
		})
		if err != nil {
			return err
		}

		// v2-H-09's match check (below the loop) must use the SAME rewriter an
		// archive was actually extracted with, against this archive's own
		// STRIPPED includes (archiveIncludes) — not the raw, wrapper-prefixed
		// includesToCheck, which would double the wrapper (archiveRewriter
		// already re-adds it) and never match anything real. This loop can
		// hand different archives different (wrapped) rewriters and different
		// stripped includes, so the check has to happen per-archive, right
		// here, against archiveRewriter + archiveIncludes + extractedHere.
		if !anyIncludesMatched {
			for _, inc := range archiveIncludes {
				want := archiveRewriter(inc)
				if want == "" {
					continue
				}
				for _, f := range extractedHere {
					if f.Path == want || strings.HasPrefix(f.Path, want+string(os.PathSeparator)) {
						anyIncludesMatched = true
						break
					}
				}
				if anyIncludesMatched {
					break
				}
			}
		}

		extracted = append(extracted, extractedHere...)
	}

	successCount := 0
	skipCount := 0      // deliberate skips (e.g. already exists with overwrite off)
	errorSkipCount := 0 // genuine failures (open/write/rename/mkdir)
	dirCount := 0
	for _, f := range extracted {
		if f.Skipped {
			if f.Expected {
				skipCount++
				writeBackupLog(fmt.Sprintf("SKIPPED (expected): %s - %s", f.Path, f.SkipReason))
			} else {
				errorSkipCount++
				writeBackupLog(fmt.Sprintf("SKIPPED (error): %s - %s", f.Path, f.SkipReason))
			}
		} else if f.IsDir {
			dirCount++
		} else {
			successCount++
		}
	}

	writeBackupLog(fmt.Sprintf("Extraction complete: %d files, %d dirs, %d skipped (expected), %d failed",
		successCount, dirCount, skipCount, errorSkipCount))
	progress(0.95, fmt.Sprintf("Extracted %d files", successCount))

	if opts.RestoreACLs || opts.RestoreADS {
		// Reserved options — sidecar metadata isn't written by the backup yet.
		// Log the request so it shows up in support transcripts.
		writeBackupLog("NOTE: ACL/ADS restore requested but not yet implemented (NTFS sidecar pending)")
	}

	progress(1.0, "Restore completed")

	// v2-H-09: a selective restore is a success only if at least one REQUESTED path
	// (the include itself or a descendant) was actually present in the snapshot.
	// Ancestor directories are auto-created for context and must NOT count, otherwise
	// a stale/mis-cased leaf under existing folders would falsely report success with
	// the file never materialized. anyIncludesMatched was computed inside the
	// per-archive loop above, against each archive's OWN (possibly wrapped)
	// rewriter and its own extracted results — using the single unwrapped
	// `rewriter` here instead was the bug that made a genuinely successful
	// alternate_abs restore report "matched no files" (the wrapper folder name
	// sits between dest and the archive path, so the unwrapped want-path was
	// never a prefix of the real, wrapped, extracted path).
	if len(includesToCheck) > 0 && !anyIncludesMatched {
		return fmt.Errorf("restore matched no files for the requested path(s): %s",
			strings.Join(opts.IncludePaths, ", "))
	}
	// Only genuine failures fail the restore. Files left in place because
	// overwrite was disabled are the requested behaviour, not an error.
	if errorSkipCount > 0 {
		return fmt.Errorf("restore completed with %d failed files (see logs)", errorSkipCount)
	}
	return nil
}
