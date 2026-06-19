package main

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"pbscommon"
)

// maxSearchHits caps how many matches we collect before stopping, so a loose
// query (e.g. ".") can't flood the UI or exhaust memory.
const maxSearchHits = 5000

// searchCancelled is flipped by CancelFileSearch to abort an in-flight search
// between snapshots. A single global is enough: the GUI only runs one search
// at a time.
var searchCancelled atomic.Bool

// SearchMatchMode selects how Query is interpreted.
type SearchMatchMode string

const (
	// SearchModeName matches a case-insensitive substring against the file's
	// base name (the last path segment). The intuitive default.
	SearchModeName SearchMatchMode = "name"
	// SearchModeRegex matches a Go regular expression against the base name.
	// Case-insensitive by default (to match Windows expectations); add (?-i) in
	// the pattern to force case-sensitive matching.
	SearchModeRegex SearchMatchMode = "regex"
	// SearchModePath matches a case-insensitive substring against the whole
	// archive-relative path (directories included).
	SearchModePath SearchMatchMode = "path"
)

// SearchOptions parameterizes a cross-snapshot file search.
//
// HostPrefix filters which backups are scanned: every backup-id containing the
// prefix is included, so all split parts of a host (HOST_PART-A, HOST_D_DATA…)
// are searched together — the user doesn't have to know which part holds the
// file. From/To bound the snapshot period (inclusive); a zero time means
// unbounded on that side.
//
// AssembleMissing controls cost: when false only snapshots already in the local
// listing cache are searched (instant, no download). When true, snapshots not
// in cache are downloaded + assembled (slow, needs temp space) and cached for
// next time.
type SearchOptions struct {
	BaseURL         string
	AuthID          string
	Secret          string
	Datastore       string
	Namespace       string
	CertFingerprint string

	HostPrefix      string
	Query           string
	Mode            SearchMatchMode
	From            time.Time
	To              time.Time
	AssembleMissing bool

	OnProgress func(percent float64, message string)
}

// SearchHit is a single matching entry found in a snapshot.
type SearchHit struct {
	BackupID     string `json:"backup_id"`
	SnapshotTime int64  `json:"snapshot_time"` // unix seconds
	Path         string `json:"path"`          // archive-relative, forward slashes
	OriginPath   string `json:"origin_path"`   // reconstructed absolute origin, "" if no meta
	IsDir        bool   `json:"is_dir"`
	Size         uint64 `json:"size"`
	ModTime      int64  `json:"mtime"`
	FromCache    bool   `json:"from_cache"`
}

// SearchResult bundles the matches with a summary of what was (and wasn't)
// scanned, so the UI can warn when results may be incomplete.
type SearchResult struct {
	Hits               []SearchHit `json:"hits"`
	SnapshotsInRange   int         `json:"snapshots_in_range"` // snapshots within the From/To period; 0 => widen the dates
	SnapshotsSearched  int         `json:"snapshots_searched"`
	SnapshotsSkipped   int         `json:"snapshots_skipped"` // uncached and AssembleMissing=false, or failed to assemble
	SnapshotsAssembled int         `json:"snapshots_assembled"`
	Truncated          bool        `json:"truncated"` // hit maxSearchHits
	Cancelled          bool        `json:"cancelled"`
}

// CancelFileSearch requests the running search to stop at the next snapshot
// boundary. Safe to call when no search is running.
func CancelFileSearch() { searchCancelled.Store(true) }

// entryMatcher reports whether an archive entry matches the query.
type entryMatcher func(path string) bool

// hasGlob reports whether q uses shell-style wildcards (* or ?). Users coming
// from Windows naturally type patterns like "Prix*"; without this the asterisk
// was matched as a literal character and never hit anything.
func hasGlob(q string) bool { return strings.ContainsAny(q, "*?") }

// compileGlob turns a shell-style glob (* and ?) into a case-insensitive,
// anchored regexp. Every other character is escaped so it matches literally —
// the user is typing a filename pattern, not a regex.
func compileGlob(glob string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("(?i)^")
	for _, r := range glob {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

func buildMatcher(mode SearchMatchMode, query string) (entryMatcher, error) {
	baseName := func(path string) string {
		if i := strings.LastIndex(path, "/"); i >= 0 {
			return path[i+1:]
		}
		return path
	}

	switch mode {
	case SearchModeRegex:
		// Prepend (?i) so matching is case-insensitive like the other modes;
		// a user who explicitly wants case-sensitivity can override with (?-i).
		re, err := regexp.Compile("(?i)" + query)
		if err != nil {
			return nil, fmt.Errorf("expression régulière invalide: %v", err)
		}
		return func(path string) bool { return re.MatchString(baseName(path)) }, nil

	case SearchModePath:
		if hasGlob(query) {
			re, err := compileGlob(query)
			if err != nil {
				return nil, fmt.Errorf("motif de recherche invalide: %v", err)
			}
			return func(path string) bool { return re.MatchString(path) }, nil
		}
		q := strings.ToLower(query)
		return func(path string) bool { return strings.Contains(strings.ToLower(path), q) }, nil

	case SearchModeName, "":
		if hasGlob(query) {
			re, err := compileGlob(query)
			if err != nil {
				return nil, fmt.Errorf("motif de recherche invalide: %v", err)
			}
			return func(path string) bool { return re.MatchString(baseName(path)) }, nil
		}
		q := strings.ToLower(query)
		return func(path string) bool { return strings.Contains(strings.ToLower(baseName(path)), q) }, nil

	default:
		return nil, fmt.Errorf("mode de recherche inconnu: %q", string(mode))
	}
}

// joinOriginPath reconstructs the absolute on-disk location an archive entry
// came from, using the snapshot's meta sidecar. Returns "" when no usable meta
// is available (legacy snapshot).
func joinOriginPath(meta *BackupMeta, archivePath string) string {
	if meta == nil || meta.OriginalPath == "" {
		return ""
	}
	sep := "/"
	if meta.OS == "windows" || strings.Contains(meta.OriginalPath, "\\") {
		sep = "\\"
	}
	base := strings.TrimRight(meta.OriginalPath, sep)
	if archivePath == "" {
		return base
	}
	return base + sep + strings.ReplaceAll(archivePath, "/", sep)
}

// SearchFilesInline scans the snapshots of every backup-id matching HostPrefix
// within the requested period and returns entries matching Query. Cached
// listings are searched for free; uncached snapshots are assembled only when
// AssembleMissing is set. Results are newest-snapshot-first.
func SearchFilesInline(opts SearchOptions) (*SearchResult, error) {
	if opts.BaseURL == "" || opts.AuthID == "" || opts.Secret == "" {
		return nil, fmt.Errorf("paramètres de connexion PBS requis")
	}
	if opts.Datastore == "" {
		return nil, fmt.Errorf("datastore requis")
	}
	if strings.TrimSpace(opts.Query) == "" {
		return nil, fmt.Errorf("terme de recherche requis")
	}
	matcher, err := buildMatcher(opts.Mode, opts.Query)
	if err != nil {
		return nil, err
	}

	searchCancelled.Store(false)

	snaps, err := ListSnapshotsInline(opts.BaseURL, opts.AuthID, opts.Secret,
		opts.Datastore, opts.Namespace, opts.CertFingerprint, opts.HostPrefix)
	if err != nil {
		return nil, fmt.Errorf("liste des snapshots: %v", err)
	}

	type target struct {
		backupID string
		at       time.Time
	}
	targets := make([]target, 0, len(snaps))
	for _, s := range snaps {
		if !opts.From.IsZero() && s.BackupTime.Before(opts.From) {
			continue
		}
		if !opts.To.IsZero() && s.BackupTime.After(opts.To) {
			continue
		}
		targets = append(targets, target{backupID: s.BackupID, at: s.BackupTime})
	}
	// Newest first so the most likely-relevant matches surface first.
	sort.Slice(targets, func(i, j int) bool { return targets[i].at.After(targets[j].at) })

	writeBackupLog(fmt.Sprintf("Search: prefix=%q query=%q mode=%s period=[%s..%s] assembleMissing=%v -> %d snapshot(s) in range",
		opts.HostPrefix, opts.Query, opts.Mode, fmtTime(opts.From), fmtTime(opts.To), opts.AssembleMissing, len(targets)))

	result := &SearchResult{Hits: make([]SearchHit, 0, 64), SnapshotsInRange: len(targets)}
	denom := len(targets)
	if denom == 0 {
		denom = 1
	}

	for i, tg := range targets {
		if searchCancelled.Load() {
			result.Cancelled = true
			writeBackupLog("Search: cancelled by user")
			break
		}
		if opts.OnProgress != nil {
			opts.OnProgress(float64(i)/float64(denom),
				fmt.Sprintf("Recherche dans %s (%s) — %d/%d", tg.backupID, tg.at.Format("2006-01-02 15:04"), i+1, len(targets)))
		}

		ropts := RestoreOptions{
			BaseURL:         opts.BaseURL,
			AuthID:          opts.AuthID,
			Secret:          opts.Secret,
			Datastore:       opts.Datastore,
			Namespace:       opts.Namespace,
			CertFingerprint: opts.CertFingerprint,
			BackupID:        tg.backupID,
			SnapshotTime:    tg.at,
		}
		cacheKey := buildSnapshotCacheKey(ropts)

		var entries []SnapshotEntry
		var meta *BackupMeta
		fromCache := false

		if cached, ok := loadSnapshotTreeCache(cacheKey); ok {
			entries = cached.Entries
			meta = cached.Meta
			fromCache = true
			result.SnapshotsSearched++
		} else if opts.AssembleMissing {
			es, m, aerr := assembleSnapshotTree(ropts, "backup.pxar.didx", "Search", searchCancelled.Load)
			if aerr != nil {
				// Cancellation aborts the in-flight assembly mid-snapshot; treat it
				// as a user cancel, not a failed snapshot to skip past.
				if errors.Is(aerr, pbscommon.ErrReadCancelled) || searchCancelled.Load() {
					result.Cancelled = true
					writeBackupLog("Search: cancelled by user during snapshot assembly")
					break
				}
				writeBackupLog(fmt.Sprintf("Search: assemble %s@%d failed: %v", tg.backupID, tg.at.Unix(), aerr))
				result.SnapshotsSkipped++
				continue
			}
			entries = es
			meta = m
			// Don't persist a partially-read snapshot if a cancel landed during the
			// cheap meta read (meta may be nil) — let the next search re-assemble.
			if !searchCancelled.Load() {
				if serr := saveSnapshotTreeCache(cacheKey, entries, meta); serr != nil {
					writeBackupLog(fmt.Sprintf("Search: cache write failed for %s: %v", tg.backupID, serr))
				}
			}
			result.SnapshotsSearched++
			result.SnapshotsAssembled++
		} else {
			result.SnapshotsSkipped++
			continue
		}

		for _, e := range entries {
			if !matcher(e.Path) {
				continue
			}
			result.Hits = append(result.Hits, SearchHit{
				BackupID:     tg.backupID,
				SnapshotTime: tg.at.Unix(),
				Path:         e.Path,
				OriginPath:   joinOriginPath(meta, e.Path),
				IsDir:        e.IsDir,
				Size:         e.Size,
				ModTime:      e.ModTime,
				FromCache:    fromCache,
			})
			if len(result.Hits) >= maxSearchHits {
				result.Truncated = true
				break
			}
		}
		if result.Truncated {
			writeBackupLog(fmt.Sprintf("Search: hit cap of %d results, stopping", maxSearchHits))
			break
		}
	}

	if opts.OnProgress != nil {
		opts.OnProgress(1.0, fmt.Sprintf("Terminé : %d résultat(s)", len(result.Hits)))
	}
	writeBackupLog(fmt.Sprintf("Search done: %d hits, %d searched, %d assembled, %d skipped, truncated=%v cancelled=%v",
		len(result.Hits), result.SnapshotsSearched, result.SnapshotsAssembled, result.SnapshotsSkipped, result.Truncated, result.Cancelled))
	return result, nil
}

// fmtTime renders a bound for logging; "∞" marks an open end.
func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "∞"
	}
	return t.Format("2006-01-02")
}
