package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cornelk/hashmap"
	"pbscommon"
	"retry"
	"security"
	"snapshot"
)

// BackupOptions contains all parameters for a backup operation
type BackupOptions struct {
	BaseURL         string
	AuthID          string
	Secret          string
	Datastore       string
	Namespace       string
	CertFingerprint string
	BackupDirs      []string // Multiple directories or drives to backup
	BackupID        string
	BackupType      string // "host" for directory, "vm" for machine
	UseVSS          bool
	Compression     string   // Compression level: "fastest", "default", "better", "best"
	ExcludeList     []string // User-configured exclusion patterns applied by the PXAR writer (H-04)
	DisableSplit    bool     // When true, never auto-split regardless of size
	SplitSizeBytes  uint64   // Auto-split threshold and per-bin target; 0 = default (SplitThreshold)
	OnProgress      func(percent float64, message string)
	OnComplete      func(success bool, message string)
	// OnResult delivers the full structured result (Group 0 contract). It is
	// additive: OnComplete keeps firing with the success bool for existing
	// consumers. OnResult is the source the sidecar (Group 1) and rich history read.
	OnResult func(*BackupStatus)
	// OnStats delivers structured live progress so the GUI can show real
	// statistics instead of parsing them out of the progress message string.
	OnStats func(*BackupProgressStats)
}

// isFatalSessionError returns true for errors that make the current PBS session
// unusable. These indicate the H2 connection was lost; the session state on the
// server is gone, and all subsequent operations on this session will fail.
// The only recovery is a fresh Connect() with a new backup-time.
func isFatalSessionError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "PBS session cannot be resumed") ||
		strings.Contains(s, "connection lost") ||
		strings.Contains(s, "unexpected EOF") ||
		strings.Contains(s, "writer '") && strings.Contains(s, "not registered")
}

// Global backup locks per destination (BaseURL + Datastore)
var (
	backupLocks      = make(map[string]*sync.Mutex)
	backupLocksMutex sync.Mutex
)

// getBackupLock returns a mutex for the given backup destination
func getBackupLock(baseURL, datastore string) *sync.Mutex {
	key := baseURL + "|" + datastore
	backupLocksMutex.Lock()
	defer backupLocksMutex.Unlock()

	if _, exists := backupLocks[key]; !exists {
		backupLocks[key] = &sync.Mutex{}
	}
	return backupLocks[key]
}

// calculateDirSizeCtx scans a directory recursively and returns total size in bytes.
//
// It is hardened for whole-drive roots like C:\ in two ways:
//   - It never descends into directory junctions / reparse points. On a system
//     drive these loop (e.g. C:\ProgramData\Application Data -> C:\ProgramData,
//     C:\Documents and Settings -> C:\Users), which can make a naive walk run
//     effectively forever. The pxar writer already skips them (pxar.go WriteDir),
//     so they don't belong in the size estimate either.
//   - It honors ctx, so a pathologically large or slow drive (e.g. one where
//     antivirus scans every file open) can't stall the caller indefinitely; on
//     deadline it returns the partial size gathered so far.
//
// Returns size and error if access was denied at the root (needs VSS) or if ctx
// fired (partial size is still returned alongside the context error).
func calculateDirSizeCtx(ctx context.Context, path string) (uint64, error) {
	var totalSize uint64
	var accessDenied bool

	err := filepath.WalkDir(path, func(filePath string, d os.DirEntry, err error) error {
		if ctx.Err() != nil {
			// Deadline/cancel: stop walking and report what we have so far.
			return ctx.Err()
		}
		if err != nil {
			// Check if it's an access denied error
			if strings.Contains(err.Error(), "Access is denied") ||
				strings.Contains(err.Error(), "permission denied") {
				accessDenied = true
				// Stop walking this path, but continue with others
				if filePath == path {
					// Root path denied - cannot scan at all
					return fmt.Errorf("access denied to root path")
				}
				return filepath.SkipDir
			}
			return nil // Skip other errors
		}
		// Never follow junctions / symlinks (see doc comment): on Windows
		// ReadDir reports mount points with ModeSymlink, so skip the whole
		// subtree rather than recursing into a loop.
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return nil // file vanished or became unreadable: ignore for sizing
			}
			totalSize += uint64(info.Size())
		}
		return nil
	})

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		// Partial size is still useful for the split heuristic.
		return totalSize, err
	}
	if accessDenied && totalSize == 0 {
		return 0, fmt.Errorf("access denied: %s", path)
	}

	return totalSize, err
}

type ChunkState struct {
	assignments         []string
	assignmentsOffset  []uint64
	pos                 uint64
	wrid                uint64
	chunkcount          uint64
	chunkdigests        hash.Hash
	currentChunk       []byte
	C                   pbscommon.Chunker
	newchunk            *atomic.Uint64
	reusechunk          *atomic.Uint64
	failedchunk         *atomic.Uint64     // Track failed chunk uploads
	knownChunks         *hashmap.Map[string, bool]
	onProgress          func(float64, string)
	onStats             func(*BackupProgressStats) // Structured live stats for the GUI (nil for the catalog stream)
	currentDir          string                     // Directory currently being archived, for the stats payload
	lastProgressReport  uint64
	lastProgressPercent float64            // Track last reported percentage to prevent backwards progress
	totalSize           *atomic.Uint64     // Total size, updated by background scan
	uploadErrors        []string           // Collect upload errors to report at the end
	errorsMutex         sync.Mutex         // Protect uploadErrors slice
}

func (c *ChunkState) Init(newchunk *atomic.Uint64, reusechunk *atomic.Uint64, failedchunk *atomic.Uint64, knownChunks *hashmap.Map[string, bool], onProgress func(float64, string), totalSize *atomic.Uint64, onStats func(*BackupProgressStats), currentDir string) {
	c.assignments = make([]string, 0)
	c.assignmentsOffset = make([]uint64, 0)
	c.pos = 0
	c.chunkcount = 0
	c.chunkdigests = sha256.New()
	c.currentChunk = make([]byte, 0)
	c.C = pbscommon.Chunker{}
	// Chunk size avg = 4MB → max = 16MB (PBS hard limit on chunk size)
	// Was 8MB (max=32MB) as workaround for "Invalid string length" errors,
	// but the real cause was broken JSON encoding in CreateDynamicIndex (fixed in 16fbba8)
	c.C.New(1024 * 1024 * 4)
	c.reusechunk = reusechunk
	c.newchunk = newchunk
	c.failedchunk = failedchunk
	c.knownChunks = knownChunks
	c.onProgress = onProgress
	c.onStats = onStats
	c.currentDir = currentDir
	c.lastProgressReport = 0
	c.lastProgressPercent = 0.0
	c.totalSize = totalSize
	c.uploadErrors = make([]string, 0)
}

func (c *ChunkState) HandleData(b []byte, client *pbscommon.PBSClient) error {
	chunkpos := c.C.Scan(b)

	if chunkpos == 0 {
		c.currentChunk = append(c.currentChunk, b...)
	} else {
		for chunkpos > 0 {
			c.currentChunk = append(c.currentChunk, b[:chunkpos]...)

			h := sha256.New()
			if _, err := h.Write(c.currentChunk); err != nil {
				return fmt.Errorf("failed to hash chunk: %w", err)
			}
			bindigest := h.Sum(nil)
			shahash := hex.EncodeToString(bindigest)

			if _, known := c.knownChunks.Get(shahash); !known {
				writeBackupLog(fmt.Sprintf("New chunk[%s] %d bytes", shahash, len(c.currentChunk)))

				// Retry chunk upload with exponential backoff
				chunkData := c.currentChunk // Capture for closure
				retryConfig := retry.DefaultConfig()
				retryConfig.MaxAttempts = 5 // More retries for chunk uploads
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()

				err := retry.DoWithJitter(ctx, retryConfig, retry.DefaultRetryable, func() error {
					return client.UploadDynamicCompressedChunk(c.wrid, shahash, chunkData)
				})
				if err != nil {
					errMsg := fmt.Sprintf("⚠️  Failed to upload chunk %s after %d retries: %v", shahash, retryConfig.MaxAttempts, err)
					writeBackupLog(errMsg)
					c.failedchunk.Add(1)

					// Collect error for final report
					c.errorsMutex.Lock()
					c.uploadErrors = append(c.uploadErrors, errMsg)
					c.errorsMutex.Unlock()

					// C3 fail-closed: a chunk we could not upload must NOT be indexed.
					// Indexing it would close a dynamic index referencing a digest that
					// is absent from the datastore — an unrestorable snapshot. Abort the
					// writer now (whether the error is fatal or transient); WriteDir's EOF,
					// CloseDynamicIndex, manifest and Finish are never reached for this dir.
					return fmt.Errorf("chunk upload failed, aborting to avoid committing a corrupt snapshot: %w", err)
				}
				// Mark the chunk known ONLY after a confirmed upload, so a failed chunk
				// can never make a later identical chunk be skipped as "reused".
				c.knownChunks.Set(shahash, true)
				c.newchunk.Add(1)
			} else {
				writeBackupLog(fmt.Sprintf("Reuse chunk[%s] %d bytes", shahash, len(c.currentChunk)))
				c.reusechunk.Add(1)
			}

			if err := binary.Write(c.chunkdigests, binary.LittleEndian, (c.pos + uint64(len(c.currentChunk)))); err != nil {
				return fmt.Errorf("failed to write chunk offset: %w", err)
			}
			if _, err := c.chunkdigests.Write(h.Sum(nil)); err != nil {
				return fmt.Errorf("failed to write chunk digest: %w", err)
			}

			c.assignmentsOffset = append(c.assignmentsOffset, c.pos)
			c.assignments = append(c.assignments, shahash)
			c.pos += uint64(len(c.currentChunk))
			c.chunkcount += 1

			// Report progress every 10 MB
			if c.onProgress != nil && c.pos-c.lastProgressReport > 10*1024*1024 {
				c.lastProgressReport = c.pos
				sizeMB := c.pos / (1024 * 1024)

				// Build progress message with chunk stats
				var msg string
				failed := c.failedchunk.Load()
				if failed > 0 {
					msg = fmt.Sprintf("Traité: %d MB (New: %d, Reused: %d, ⚠️ Failed: %d chunks)",
						sizeMB, c.newchunk.Load(), c.reusechunk.Load(), failed)
				} else {
					msg = fmt.Sprintf("Traité: %d MB (New: %d, Reused: %d chunks)",
						sizeMB, c.newchunk.Load(), c.reusechunk.Load())
				}

				// Calculate progress based on total size if available
				var progress float64
				totalSize := c.totalSize.Load()
				if totalSize > 0 {
					// Progress from 10% to 90% based on bytes processed
					progress = 0.1 + (float64(c.pos)/float64(totalSize))*0.8
					if progress > 0.9 {
						progress = 0.9
					}
					if failed > 0 {
						msg = fmt.Sprintf("Traité: %d / %d MB (New: %d, Reused: %d, ⚠️ Failed: %d chunks)",
							sizeMB, totalSize/(1024*1024), c.newchunk.Load(), c.reusechunk.Load(), failed)
					} else {
						msg = fmt.Sprintf("Traité: %d / %d MB (New: %d, Reused: %d chunks)",
							sizeMB, totalSize/(1024*1024), c.newchunk.Load(), c.reusechunk.Load())
					}
				} else {
					// No total size yet, show indeterminate progress
					progress = 0.1 + float64(sizeMB%100)/1000.0 // Slowly increment from 10%
					if progress > 0.5 {
						progress = 0.5
					}
				}

				// Never report backwards progress - totalSize can increase during backup
				if progress < c.lastProgressPercent {
					progress = c.lastProgressPercent
				}
				c.lastProgressPercent = progress

				c.onProgress(progress, msg)

				// Structured live stats for the GUI (same cadence as the message).
				if c.onStats != nil {
					c.onStats(&BackupProgressStats{
						Percent:      progress,
						BytesDone:    c.pos,
						BytesTotal:   totalSize,
						NewChunks:    c.newchunk.Load(),
						ReusedChunks: c.reusechunk.Load(),
						FailedChunks: failed,
						CurrentDir:   c.currentDir,
						Message:      msg,
					})
				}
			}

			c.currentChunk = make([]byte, 0)
			b = b[chunkpos:]
			chunkpos = c.C.Scan(b)
		}
		c.currentChunk = append(c.currentChunk, b...)
	}
	return nil
}

func (c *ChunkState) EOF(client *pbscommon.PBSClient) error {
	if len(c.currentChunk) > 0 {
		h := sha256.New()
		if _, err := h.Write(c.currentChunk); err != nil {
			return fmt.Errorf("failed to hash final chunk: %w", err)
		}

		shahash := hex.EncodeToString(h.Sum(nil))
		if err := binary.Write(c.chunkdigests, binary.LittleEndian, (c.pos + uint64(len(c.currentChunk)))); err != nil {
			return fmt.Errorf("failed to write final chunk offset: %w", err)
		}
		if _, err := c.chunkdigests.Write(h.Sum(nil)); err != nil {
			return fmt.Errorf("failed to write final chunk digest: %w", err)
		}

		if _, known := c.knownChunks.Get(shahash); !known {
			writeBackupLog(fmt.Sprintf("New chunk[%s] %d bytes", shahash, len(c.currentChunk)))

			// Retry final chunk upload with exponential backoff
			chunkData := c.currentChunk
			retryConfig := retry.DefaultConfig()
			retryConfig.MaxAttempts = 5
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			err := retry.DoWithJitter(ctx, retryConfig, retry.DefaultRetryable, func() error {
				return client.UploadDynamicCompressedChunk(c.wrid, shahash, chunkData)
			})
			if err != nil {
				errMsg := fmt.Sprintf("⚠️  Failed to upload final chunk %s after %d retries: %v", shahash, retryConfig.MaxAttempts, err)
				writeBackupLog(errMsg)
				c.failedchunk.Add(1)

				c.errorsMutex.Lock()
				c.uploadErrors = append(c.uploadErrors, errMsg)
				c.errorsMutex.Unlock()

				// C3 fail-closed (see HandleData): never index an unconfirmed chunk —
				// abort before the chunk is appended to the index and before EOF closes it.
				return fmt.Errorf("final chunk upload failed, aborting to avoid committing a corrupt snapshot: %w", err)
			}
			// Mark known only after a confirmed upload.
			c.knownChunks.Set(shahash, true)
			c.newchunk.Add(1)
		} else {
			writeBackupLog(fmt.Sprintf("Reuse chunk[%s] %d bytes", shahash, len(c.currentChunk)))
			c.reusechunk.Add(1)
		}
		c.assignmentsOffset = append(c.assignmentsOffset, c.pos)
		c.assignments = append(c.assignments, shahash)
		c.pos += uint64(len(c.currentChunk))
		c.chunkcount += 1
	}

	// Assign chunks in batches with retry
	retryConfig := retry.DefaultConfig()
	retryConfig.MaxAttempts = 5
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	for k := 0; k < len(c.assignments); k += 128 {
		k2 := k + 128
		if k2 > len(c.assignments) {
			k2 = len(c.assignments)
		}

		// Capture loop variables for closure
		batchStart, batchEnd := k, k2
		assignments := c.assignments[batchStart:batchEnd]
		offsets := c.assignmentsOffset[batchStart:batchEnd]

		err := retry.DoWithJitter(ctx, retryConfig, retry.DefaultRetryable, func() error {
			return client.AssignDynamicChunks(c.wrid, assignments, offsets)
		})
		if err != nil {
			return fmt.Errorf("failed to assign chunks (batch %d-%d) after retries: %w", batchStart, batchEnd, err)
		}
	}

	// Close index with retry
	digest := hex.EncodeToString(c.chunkdigests.Sum(nil))
	err := retry.DoWithJitter(ctx, retryConfig, retry.DefaultRetryable, func() error {
		return client.CloseDynamicIndex(c.wrid, digest, c.pos, c.chunkcount)
	})
	if err != nil {
		return fmt.Errorf("failed to close dynamic index after retries: %w", err)
	}
	return nil
}

// formatDuration formats a duration in a human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}

// RunBackupInline performs a backup without external binaries
func RunBackupInline(opts BackupOptions) (returnErr error) {
	// Per-run log file: each backup run gets its own dedicated log file.
	// Must be set up BEFORE any writeBackupLog call so logs land in the right place.
	runLogID := opts.BackupID
	if runLogID == "" {
		if h, err := os.Hostname(); err == nil {
			runLogID = h
		} else {
			runLogID = "backup"
		}
	}
	runLogger := StartBackupRunLog(runLogID)
	defer EndBackupRunLog(runLogger)

	// CRITICAL: Panic recovery to prevent silent goroutine death (scheduler launches backups in goroutines)
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("CRITICAL: Backup panic in RunBackupInline: %v", r)
			writeBackupLog(errMsg)
			// Get stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			writeBackupLog(fmt.Sprintf("Stack trace:\n%s", buf[:n]))
			returnErr = fmt.Errorf("backup panic: %v", r)
		}
	}()

	userName := "<unknown>"
	if cu, err := user.Current(); err == nil {
		userName = cu.Username
	}
	writeBackupLog(fmt.Sprintf("==== starting version %s - user %s - %s/%s ====",
		appVersion, userName, runtime.GOOS, runtime.GOARCH))

	// Validate options. Log the resolved target (secret presence only, never the
	// secret) so a misconfiguration is diagnosable from the backup log alone — a
	// prod report showed only "Validating backup options" then nothing because an
	// empty multi-PBS resolution failed here silently.
	writeBackupLog(fmt.Sprintf("[DEBUG] Validating backup options: target=%q datastore=%q ns=%q authid=%q secret=%v dirs=%d backupID=%q",
		opts.BaseURL, opts.Datastore, opts.Namespace, opts.AuthID, opts.Secret != "", len(opts.BackupDirs), opts.BackupID))
	if opts.BaseURL == "" || opts.AuthID == "" || opts.Secret == "" {
		var missing []string
		if opts.BaseURL == "" {
			missing = append(missing, "BaseURL")
		}
		if opts.AuthID == "" {
			missing = append(missing, "AuthID")
		}
		if opts.Secret == "" {
			missing = append(missing, "Secret")
		}
		errMsg := fmt.Sprintf("PBS connection parameters required (missing: %s) — check the selected/default PBS server in the config",
			strings.Join(missing, ", "))
		writeBackupLog("[ERROR] " + errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	writeBackupLog("[DEBUG] Options validated")

	if len(opts.BackupDirs) == 0 {
		return fmt.Errorf("at least one backup directory or drive required")
	}

	// Get hostname for backup-id generation
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unnamed-backup"
	}

	// Splitting a backup into smaller parts is now an explicit, opt-in choice the
	// user makes in the GUI (the split plan is built and orchestrated there via
	// CreateBackupSplitPlan, which issues one StartBackup per part). RunBackupInline
	// therefore no longer sizes the directories or auto-splits by total size: that
	// pre-pass was what made a whole-drive backup of C:\ hang before it ever began,
	// and it also ran on every scheduled run. A scheduled/recurring backup is always
	// a normal (unsplit) backup — exactly the intended "split the first seed, full
	// afterwards" behavior.

	// Each selected directory is its OWN backup group (its own
	// backup-id), so PBS retention treats successive runs of the same folder as one
	// series. Previously all selected folders shared one backup-id (derived from the
	// first), landing as separate snapshots in a single group — which makes prune
	// keep/drop the wrong folders. A single selected directory keeps the caller's
	// backup-id (which may have been set explicitly, e.g. by a scheduled job).
	if len(opts.BackupDirs) <= 1 {
		return runBackupInlineInternal(opts)
	}

	// Each folder runs as its own internal backup, but the per-folder terminal
	// callbacks are SUPPRESSED and aggregated into a single OnComplete/OnResult for
	// the whole multi-folder run. Otherwise the first folder's OnComplete would
	// finalize (and, in API mode, tear down) the job before the rest ran — losing
	// later folders' status and breaking the honest-result contract. Live progress
	// (OnProgress/OnStats) still flows per folder.
	aggStart := time.Now()
	agg := &BackupStatus{Outcome: OutcomeVerifiedSuccess, BackupID: opts.BackupID, BackupTime: aggStart.Unix()}
	var perDirErrors []string

	// Each folder gets a distinct backup-id DERIVED FROM the base id (the caller's
	// custom backup-id, or the hostname). Deriving it as "<base>_<path>" keeps the
	// base a substring of every child id, so restore's substring search on the
	// configured backup-id still discovers all of a multi-folder backup's groups.
	baseID := opts.BackupID
	if baseID == "" {
		baseID = hostname
	}

	for _, dir := range opts.BackupDirs {
		dirOpts := opts
		dirOpts.BackupDirs = []string{dir}
		dirOpts.BackupID = GenerateBackupID(baseID, dir)
		dirOpts.OnComplete = nil // suppress per-folder terminal callback; aggregated below
		var dirStatus *BackupStatus
		dirOpts.OnResult = func(s *BackupStatus) { dirStatus = s }

		writeBackupLog(fmt.Sprintf("[Grouping] Backing up %s as its own group %s", dir, dirOpts.BackupID))
		derr := runBackupInlineInternal(dirOpts)

		if dirStatus != nil {
			agg.merge(dirStatus)
		} else if derr != nil {
			agg.Outcome = OutcomeFailed
		}
		if derr != nil {
			errMsg := fmt.Sprintf("backup of %s failed: %v", dir, derr)
			writeBackupLog(errMsg)
			perDirErrors = append(perDirErrors, errMsg)
			// Continue with the remaining folders — one bad folder must not skip the rest.
		}
	}

	agg.DurationSec = time.Since(aggStart).Seconds()
	if len(perDirErrors) > 0 {
		agg.Message = fmt.Sprintf("%d/%d dossiers en échec:\n%s", len(perDirErrors), len(opts.BackupDirs), strings.Join(perDirErrors, "\n"))
	} else {
		agg.Message = fmt.Sprintf("Backup de %d dossiers terminé (%d new, %d reused chunks)", len(opts.BackupDirs), agg.NewChunks, agg.ReusedChunks)
	}

	if opts.OnComplete != nil {
		opts.OnComplete(agg.Success(), agg.Message)
	}
	if opts.OnResult != nil {
		opts.OnResult(agg)
	}
	if len(perDirErrors) > 0 {
		return fmt.Errorf("%s", agg.Message)
	}
	return nil
}

// runBackupInlineInternal is the actual backup implementation (called by RunBackupInline)
func runBackupInlineInternal(opts BackupOptions) (returnErr error) {
	// CRITICAL: Panic recovery for split jobs
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("CRITICAL: Backup panic in runBackupInlineInternal: %v", r)
			writeBackupLog(errMsg)
			// Get stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			writeBackupLog(fmt.Sprintf("Stack trace:\n%s", buf[:n]))
			returnErr = fmt.Errorf("backup panic: %v", r)
		}
	}()

	startTime := time.Now()

	// Acquire backup lock for this destination to prevent concurrent backups
	backupLock := getBackupLock(opts.BaseURL, opts.Datastore)
	writeBackupLog(fmt.Sprintf("[Backup Lock] Waiting for lock on %s/%s (prevents concurrent backups)", opts.BaseURL, opts.Datastore))

	// Notify that we're waiting if OnProgress is set
	if opts.OnProgress != nil {
		opts.OnProgress(0, "Waiting for previous backup to complete...")
	}

	backupLock.Lock()
	writeBackupLog(fmt.Sprintf("[Backup Lock] ✓ Lock acquired for %s/%s - starting backup", opts.BaseURL, opts.Datastore))
	defer func() {
		backupLock.Unlock()
		writeBackupLog(fmt.Sprintf("[Backup Lock] ✓ Lock released for %s/%s", opts.BaseURL, opts.Datastore))
	}()

	// Generate backup ID from path if not specified
	writeBackupLog("[DEBUG] Generating backup ID if needed")
	if opts.BackupID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unnamed-backup"
		}

		// Generate backup-id from first directory path: hostname_DRIVE_PATH
		if len(opts.BackupDirs) > 0 {
			opts.BackupID = GenerateBackupID(hostname, opts.BackupDirs[0])
		} else {
			opts.BackupID = hostname
		}
	}
	writeBackupLog(fmt.Sprintf("[DEBUG] BackupID set to: %s", opts.BackupID))

	// Default to "host" type for directory backups
	if opts.BackupType == "" {
		opts.BackupType = "host"
	}

	// Progress callback wrapper
	progress := func(pct float64, msg string) {
		writeBackupLog(fmt.Sprintf("Backup progress: %.1f%% - %s", pct*100, msg))
		if opts.OnProgress != nil {
			opts.OnProgress(pct, msg)
		}
	}

	// Check if all backup directories exist
	writeBackupLog(fmt.Sprintf("[DEBUG] Checking %d backup directories exist", len(opts.BackupDirs)))
	for idx, dir := range opts.BackupDirs {
		writeBackupLog(fmt.Sprintf("[DEBUG] Checking directory %d/%d: %s", idx+1, len(opts.BackupDirs), dir))
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			errMsg := fmt.Sprintf("Backup directory does not exist: %s", dir)
			writeBackupLog(errMsg)
			if opts.OnComplete != nil {
				opts.OnComplete(false, errMsg)
			}
			if opts.OnResult != nil {
				opts.OnResult(&BackupStatus{
					Outcome:     OutcomeFailed,
					BackupID:    opts.BackupID,
					DurationSec: time.Since(startTime).Seconds(),
					Message:     errMsg,
				})
			}
			return fmt.Errorf("%s", errMsg)
		}
	}
	writeBackupLog("[DEBUG] All directories checked, calling progress(0.05)")

	progress(0.05, "Connecting to PBS...")
	writeBackupLog("[DEBUG] After progress(0.05), before connection log")

	// Debug: log connection parameters with sanitized credentials
	writeBackupLog(fmt.Sprintf("PBS Connection: URL=%s, AuthID=%s, Secret=%s, Datastore=%s, BackupID=%s",
		security.SanitizeURL(opts.BaseURL),
		opts.AuthID,
		security.SanitizeSecret(opts.Secret),
		opts.Datastore,
		opts.BackupID))

	writeBackupLog("[DEBUG] Creating PBS client struct")

	// Parse compression level (default to fastest if empty or invalid)
	compressionLevel := pbscommon.ParseCompressionLevel(opts.Compression)
	writeBackupLog(fmt.Sprintf("[DEBUG] Compression level: %s", compressionLevel))

	// Create PBS client
	client := &pbscommon.PBSClient{
		BaseURL:          opts.BaseURL,
		CertFingerPrint:  opts.CertFingerprint,
		AuthID:           opts.AuthID,
		Secret:           opts.Secret,
		Datastore:        opts.Datastore,
		Namespace:        opts.Namespace,
		Insecure:         opts.CertFingerprint != "",
		CompressionLevel: compressionLevel,
		Manifest: pbscommon.BackupManifest{
			BackupID: opts.BackupID,
		},
	}

	writeBackupLog("[DEBUG] PBS client created, starting directory backup loop")

	// Backup each directory
	var newchunk atomic.Uint64
	var reusechunk atomic.Uint64
	var failedchunk atomic.Uint64
	var totalSize atomic.Uint64
	var dirErrors []string
	var dirResults []DirResult
	// Run-level accumulators: client.{ReadErrors,SkippedFiles,ExcludedFiles} are
	// reset by each directory's Connect(), so we capture them per directory before
	// the next one wipes them — otherwise a read error in an earlier folder of a
	// multi-folder run / split bin is lost and the run can falsely report
	// verified_success (v2-H-02 / M-01).
	var allReadErrors, allSkipped, allExcluded []string
	successfulDirs := 0

	// Retry policy for session-lost failures: PBS keeps the BackupGroup lock until
	// it detects the dead TCP connection server-side (observed ~16 min in prod).
	// Any retry sooner than that hits 400 "while creating locked backup group".
	// We wait sessionLostRetryWait between attempts so the lock has time to expire.
	const maxDirAttempts = 2
	const sessionLostRetryWait = 25 * time.Minute

	for idx, dir := range opts.BackupDirs {
		writeBackupLog(fmt.Sprintf("Starting backup of directory %d/%d: %s", idx+1, len(opts.BackupDirs), dir))

		// Each directory becomes its own PBS session (Connect → upload → Finish).
		// On session-lost we wait for PBS to release the group lock, then retry once.
		// PBS dedupes chunks, so retrying is cheap for the already-uploaded data.
		var err error
		var dirBytes uint64
		for attempt := 1; attempt <= maxDirAttempts; attempt++ {
			dirBytes, err = backupDirectory(client, &newchunk, &reusechunk, &failedchunk, dir, opts.UseVSS, progress, opts.OnStats, opts.ExcludeList)
			if err == nil {
				break
			}
			// Tear down the abandoned PBS session before retrying or moving to the
			// next directory: leaving it open holds the backup-group lock (~16 min
			// until TCP keepalive reaps it) so every later directory in this run
			// then fails with "locked backup group". [B-4]
			client.Close()
			if !isFatalSessionError(err) {
				// Non-recoverable error (e.g. access denied) — don't retry
				break
			}
			if attempt < maxDirAttempts {
				writeBackupLog(fmt.Sprintf("Directory %s: session lost on attempt %d/%d: %v",
					dir, attempt, maxDirAttempts, err))
				writeBackupLog(fmt.Sprintf("Waiting %s for PBS to release backup group lock before retry...",
					sessionLostRetryWait))

				waitUntil := time.Now().Add(sessionLostRetryWait)
				for {
					remaining := time.Until(waitUntil)
					if remaining <= 0 {
						break
					}
					progress(0, fmt.Sprintf("Session PBS perdue, attente %s avant retry (lock PBS en cours de libération)...",
						remaining.Round(time.Second)))
					sleepFor := 30 * time.Second
					if remaining < sleepFor {
						sleepFor = remaining
					}
					time.Sleep(sleepFor)
				}
				writeBackupLog(fmt.Sprintf("Wait complete, retrying directory %s with fresh connection", dir))
			}
		}

		// Capture this directory's lists before the next directory's Connect() resets
		// them on the shared client (v2-H-02 / M-01).
		allReadErrors = append(allReadErrors, client.ReadErrors...)
		allSkipped = append(allSkipped, client.SkippedFiles...)
		allExcluded = append(allExcluded, client.ExcludedFiles...)

		if err != nil {
			errMsg := fmt.Sprintf("Backup failed for %s: %v", dir, err)
			writeBackupLog(errMsg)
			dirErrors = append(dirErrors, errMsg)
			dirResults = append(dirResults, DirResult{Path: dir, OK: false, Error: err.Error()})
			continue // Don't abort — try remaining directories
		}

		// Finalize this directory's PBS session immediately so it's committed on the server
		finishCfg := retry.DefaultConfig()
		finishCfg.MaxAttempts = 5
		finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		finishErr := retry.DoWithJitter(finishCtx, finishCfg, retry.DefaultRetryable, func() error {
			return client.Finish()
		})
		finishCancel()
		if finishErr != nil {
			client.Close() // release the session/group lock on a failed finalize too [B-4]
			errMsg := fmt.Sprintf("Failed to finalize backup for %s: %v", dir, finishErr)
			writeBackupLog(errMsg)
			dirErrors = append(dirErrors, errMsg)
			dirResults = append(dirResults, DirResult{Path: dir, OK: false, Error: finishErr.Error()})
			continue
		}
		writeBackupLog(fmt.Sprintf("Directory %d/%d finalized: %s", idx+1, len(opts.BackupDirs), dir))
		// Accumulate the real archived byte count so TotalBytes / "X MB backed up"
		// and the [RESULT] support line are no longer always zero. [B-3]
		totalSize.Add(dirBytes)
		dirResults = append(dirResults, DirResult{Path: dir, OK: true})
		successfulDirs++
	}

	// If NO directory was backed up successfully, fail the whole backup
	if successfulDirs == 0 {
		errMsg := fmt.Sprintf("All %d directories failed:\n%s", len(opts.BackupDirs), strings.Join(dirErrors, "\n"))
		writeBackupLog(errMsg)
		status := &BackupStatus{
			Outcome:          OutcomeFailed,
			BackupID:         opts.BackupID,
			BackupTime:       client.Manifest.BackupTime,
			DurationSec:      time.Since(startTime).Seconds(),
			TotalBytes:       totalSize.Load(),
			NewChunks:        newchunk.Load(),
			ReusedChunks:     reusechunk.Load(),
			FailedChunks:     failedchunk.Load(),
			Directories:      dirResults,
			ExcludedByPolicy: excludedToIssues(allExcluded),
			SkippedReadError: skippedToIssues(allReadErrors),
			Message:          errMsg,
		}
		if opts.OnComplete != nil {
			opts.OnComplete(false, errMsg)
		}
		if opts.OnResult != nil {
			opts.OnResult(status)
		}
		return fmt.Errorf("%s", errMsg)
	}

	// Calculate backup duration and size
	duration := time.Since(startTime)
	totalSizeMB := float64(totalSize.Load()) / (1024 * 1024)

	// Build completion message with duration, size, and chunk stats
	failed := failedchunk.Load()
	partial := len(dirErrors) > 0
	var completionMsg, progressMsg string
	switch {
	case partial:
		completionMsg = fmt.Sprintf("⚠️  Backup partiel en %s: %d/%d dossiers OK, %.1f MB (%d new, %d reused chunks)\nErreurs:\n%s",
			formatDuration(duration), successfulDirs, len(opts.BackupDirs), totalSizeMB, newchunk.Load(), reusechunk.Load(), strings.Join(dirErrors, "\n"))
		progressMsg = fmt.Sprintf("Backup partiel : %d/%d dossiers OK", successfulDirs, len(opts.BackupDirs))
	case failed > 0:
		completionMsg = fmt.Sprintf("⚠️  Backup completed with errors in %s: %.1f MB backed up (%d new, %d reused, %d FAILED chunks)",
			formatDuration(duration), totalSizeMB, newchunk.Load(), reusechunk.Load(), failed)
		progressMsg = fmt.Sprintf("Backup completed with %d failed chunks", failed)
	default:
		completionMsg = fmt.Sprintf("Backup completed in %s: %.1f MB backed up (%d new, %d reused chunks)",
			formatDuration(duration), totalSizeMB, newchunk.Load(), reusechunk.Load())
		progressMsg = "Backup completed"
	}

	progress(1.0, progressMsg)

	if len(allSkipped) > 0 {
		completionMsg += fmt.Sprintf("\n⚠️  %d fichiers/dossiers ignorés (accès refusé ou junction points)", len(allSkipped))
		writeBackupLog(fmt.Sprintf("=== SKIPPED FILES/DIRECTORIES (%d) ===", len(allSkipped)))

		// Log first 50 skipped files in detail
		maxLog := 50
		if len(allSkipped) < maxLog {
			maxLog = len(allSkipped)
		}
		for i := 0; i < maxLog; i++ {
			writeBackupLog(fmt.Sprintf("  [%d] %s", i+1, allSkipped[i]))
		}
		if len(allSkipped) > 50 {
			writeBackupLog(fmt.Sprintf("  ... and %d more (see full list in GUI)", len(allSkipped)-50))
		}
		writeBackupLog("=== END SKIPPED FILES ===")
	}

	writeBackupLog(completionMsg)

	// Determine the authoritative 4-level outcome (v2-H-02 / F-01):
	//   failed  — a chunk upload failed (would corrupt the index) or no dir committed
	//   partial — some directories failed, OR files were unreadable / changed during
	//             read (genuine read errors, not expected system auto-excludes)
	//   success_with_policy_exclusions — complete except files the user excluded
	//   verified_success — fully complete
	hasReadIssues := len(allReadErrors) > 0
	var outcome BackupOutcome
	switch {
	case failed > 0:
		outcome = OutcomeFailed
	case partial:
		outcome = OutcomePartial
	case hasReadIssues:
		outcome = OutcomePartial
	case len(allExcluded) > 0:
		outcome = OutcomeSuccessWithExclusions
	default:
		outcome = OutcomeVerifiedSuccess
	}

	status := &BackupStatus{
		Outcome:          outcome,
		BackupID:         opts.BackupID,
		BackupTime:       client.Manifest.BackupTime,
		DurationSec:      duration.Seconds(),
		TotalBytes:       totalSize.Load(),
		NewChunks:        newchunk.Load(),
		ReusedChunks:     reusechunk.Load(),
		FailedChunks:     failed,
		Directories:      dirResults,
		ExcludedByPolicy: excludedToIssues(allExcluded),
		SkippedReadError: skippedToIssues(allReadErrors),
		Message:          completionMsg,
	}

	// One machine-greppable result line for support (pairs with the start-of-run
	// target log): outcome label + the structured counters behind completionMsg.
	writeBackupLog(fmt.Sprintf("[RESULT] outcome=%s dirs_ok=%d/%d new=%d reused=%d failed=%d read_errors=%d excluded=%d bytes=%d duration=%s",
		status.Outcome, successfulDirs, len(opts.BackupDirs), status.NewChunks, status.ReusedChunks,
		status.FailedChunks, len(status.SkippedReadError), len(status.ExcludedByPolicy), status.TotalBytes, duration))

	// Additive (choice A): OnComplete keeps its (success, message) contract for
	// existing consumers; OnResult carries the full structured status for the
	// sidecar (Group 1) and rich history.
	if opts.OnComplete != nil {
		opts.OnComplete(status.Success(), completionMsg)
	}
	if opts.OnResult != nil {
		opts.OnResult(status)
	}

	// Contract (Group 0 / audit H-03): a partial or failed backup must NOT return a
	// nil error, otherwise the API fallback and the scheduler record it as success.
	if !status.Success() {
		return fmt.Errorf("%s", completionMsg)
	}
	return nil
}

func backupDirectory(client *pbscommon.PBSClient, newchunk, reusechunk, failedchunk *atomic.Uint64, backupdir string, usevss bool, progress func(float64, string), onStats func(*BackupProgressStats), excludeList []string) (uint64, error) {
	writeBackupLog(fmt.Sprintf("Starting backup of %s", backupdir))
	originalPath := backupdir

	if usevss {
		var bytesArchived uint64
		err := snapshot.CreateVSSSnapshot([]string{backupdir}, func(snaps map[string]snapshot.SnapShot) error {
			for _, snap := range snaps {
				backupdir = snap.FullPath
				break
			}
			var e error
			bytesArchived, e = backupReal(client, newchunk, reusechunk, failedchunk, backupdir, originalPath, usevss, progress, onStats, excludeList)
			return e
		})
		return bytesArchived, err
	}

	return backupReal(client, newchunk, reusechunk, failedchunk, backupdir, originalPath, usevss, progress, onStats, excludeList)
}

func backupReal(client *pbscommon.PBSClient, newchunk, reusechunk, failedchunk *atomic.Uint64, backupdir string, originalPath string, vssUsed bool, progress func(float64, string), onStats func(*BackupProgressStats), excludeList []string) (returnBytes uint64, returnErr error) {
	// Panic recovery - critical to prevent silent crashes during backup
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("CRITICAL: Backup panic occurred: %v", r)
			writeBackupLog(errMsg)
			// Get stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			writeBackupLog(fmt.Sprintf("Stack trace:\n%s", buf[:n]))
			returnErr = fmt.Errorf("backup panic: %v", r)
		}
	}()

	client.Connect(false, "host")
	knownChunks := hashmap.New[string, bool]()

	// Start background scan to calculate total size (drives the progress %). This
	// is non-blocking — the backup streams in parallel — but bound it with the same
	// runaway guard so a whole-drive root can't leave a goroutine walking forever
	// (junctions are already skipped); a partial size just makes the % approximate.
	totalSize := &atomic.Uint64{}
	go func() {
		writeBackupLog(fmt.Sprintf("Starting background size calculation for: %s", backupdir))
		ctx, cancel := context.WithTimeout(context.Background(), analysisSizeBudget)
		defer cancel()
		size, err := calculateDirSizeCtx(ctx, backupdir)
		if err != nil {
			writeBackupLog(fmt.Sprintf("WARNING: Size calculation had errors: %v", err))
		}
		totalSize.Store(size)
		writeBackupLog(fmt.Sprintf("Total size calculated: %d MB", size/(1024*1024)))
	}()

	archive := &pbscommon.PXARArchive{}
	archive.ArchiveName = "backup.pxar.didx"
	archive.ExcludeList = excludeList
	archive.ExcludeRoot = originalPath // logical root for VSS-safe absolute-pattern matching

	// Inject backup metadata into the PXAR archive root
	hostname, _ := os.Hostname()
	metaJSON, err := GenerateBackupMeta(client.Manifest.BackupID, originalPath, hostname, vssUsed)
	if err != nil {
		writeBackupLog(fmt.Sprintf("WARNING: Failed to generate backup metadata: %v", err))
	} else {
		archive.VirtualFiles = map[string][]byte{
			BackupMetaFilename: metaJSON,
		}
	}

	// NTFS metadata collector: captures ACLs/owner/attrs for every entry during
	// the walk. Implementation is Windows-only; no-op on other platforms.
	// The collected data is serialized after the walk and uploaded as a blob
	// in the same backup session (see below, after Eof).
	ntfsCollector := NewNTFSMetaCollector(backupdir, hostname)
	archive.MetaCollector = ntfsCollector

	previousDidx, err := client.DownloadPreviousToBytes(archive.ArchiveName)
	if err != nil {
		// This is normal for first backup - no previous backup exists
		writeBackupLog(fmt.Sprintf("No previous backup found (first backup?): %v", err))
		previousDidx = []byte{}
	} else {
		writeBackupLog(fmt.Sprintf("Downloaded previous DIDX: %d bytes", len(previousDidx)))
	}

	for _, shahash := range pbscommon.ParsePreviousDIDXChunkDigests(previousDidx) {
		knownChunks.Set(shahash, true)
	}

	writeBackupLog(fmt.Sprintf("Known chunks: %d", knownChunks.Len()))

	pxarChunk := ChunkState{}
	pxarChunk.Init(newchunk, reusechunk, failedchunk, knownChunks, progress, totalSize, onStats, backupdir)

	pcat1Chunk := ChunkState{}
	pcat1Chunk.Init(newchunk, reusechunk, failedchunk, knownChunks, nil, totalSize, nil, "")

	pxarChunk.wrid, err = client.CreateDynamicIndex(archive.ArchiveName)
	if err != nil {
		return 0, err
	}
	pcat1Chunk.wrid, err = client.CreateDynamicIndex("catalog.pcat1.didx")
	if err != nil {
		return 0, err
	}

	archive.WriteCB = func(b []byte) error {
		return pxarChunk.HandleData(b, client)
	}

	archive.CatalogWriteCB = func(b []byte) error {
		return pcat1Chunk.HandleData(b, client)
	}

	if _, err = archive.WriteDir(backupdir, "", true); err != nil {
		return 0, fmt.Errorf("failed to write directory archive: %w", err)
	}

	// Map VSS shadow-copy paths back to the original logical root so the status
	// lists are meaningful to the user (no-op for non-VSS backups, where
	// backupdir == originalPath). Reused for both the aggregate status and sidecar.
	logicalSkipped := toLogicalPaths(archive.SkippedFiles, backupdir, originalPath)
	logicalExcluded := toLogicalPaths(archive.ExcludedFiles, backupdir, originalPath)
	logicalReadErrors := toLogicalPaths(archive.ReadErrors, backupdir, originalPath)

	// Collect skipped files from archive
	if len(logicalSkipped) > 0 {
		writeBackupLog(fmt.Sprintf("Backup completed with %d skipped files/directories", len(logicalSkipped)))
		client.SkippedFiles = append(client.SkippedFiles, logicalSkipped...)
	}

	// Genuine read errors / content instability drive the outcome (v2-H-02).
	if len(logicalReadErrors) > 0 {
		client.ReadErrors = append(client.ReadErrors, logicalReadErrors...)
	}

	// Collect files excluded by user policy (H-04), kept distinct from errors.
	if len(logicalExcluded) > 0 {
		writeBackupLog(fmt.Sprintf("%d files/directories excluded by user policy", len(logicalExcluded)))
		client.ExcludedFiles = append(client.ExcludedFiles, logicalExcluded...)
	}

	// Guard: if WriteDir produced 0 data, the backup dir was effectively empty or inaccessible
	if pxarChunk.pos == 0 && len(pxarChunk.currentChunk) == 0 {
		return 0, fmt.Errorf("backup produced 0 bytes for %s — directory may be empty, inaccessible, or all files were excluded", backupdir)
	}

	if err = pxarChunk.EOF(client); err != nil {
		return 0, err
	}
	if err = pcat1Chunk.EOF(client); err != nil {
		return 0, err
	}

	// Serialize NTFS metadata collected during the walk and upload it as a
	// blob in the same backup session. The blob is listed in the PBS manifest
	// alongside backup.pxar.didx and catalog.pcat1.didx so restore tools can
	// fetch it without extracting the whole archive. Best-effort: a failure
	// here does NOT fail the backup — the file data is already safe.
	if aclBytes, aclErr := ntfsCollector.Finalize(); aclErr != nil {
		writeBackupLog(fmt.Sprintf("WARNING: failed to serialize NTFS metadata: %v", aclErr))
	} else if len(aclBytes) > 0 {
		entries, uniqueSDDLs, metaErrs := ntfsCollector.Stats()
		writeBackupLog(fmt.Sprintf("NTFS metadata: %d entries, %d unique SDDLs, %d errors, %d bytes gzipped",
			entries, uniqueSDDLs, metaErrs, len(aclBytes)))
		if upErr := client.UploadBlob(BackupAclsFilename, aclBytes); upErr != nil {
			writeBackupLog(fmt.Sprintf("WARNING: failed to upload NTFS metadata blob: %v", upErr))
		}
	}

	// Persist a per-snapshot status sidecar (files excluded by policy + files
	// skipped on read errors) as a manifest blob, so the GUI can list them without
	// restoring the archive. Best-effort: a failure here does NOT fail the backup.
	sidecar := &BackupSidecar{
		FormatVersion:    1,
		BackupID:         client.Manifest.BackupID,
		Directory:        originalPath,
		GeneratedAt:      time.Now().Unix(),
		ExcludedByPolicy: excludedToIssues(logicalExcluded),
		SkippedReadError: skippedToIssues(logicalReadErrors),
	}
	if sidecarBytes, sErr := json.Marshal(sidecar); sErr != nil {
		writeBackupLog(fmt.Sprintf("WARNING: failed to serialize status sidecar: %v", sErr))
	} else if upErr := client.UploadBlob(BackupStatusFilename, sidecarBytes); upErr != nil {
		writeBackupLog(fmt.Sprintf("WARNING: failed to upload status sidecar: %v", upErr))
	}

	// Upload manifest with retry
	retryConfig := retry.DefaultConfig()
	retryConfig.MaxAttempts = 5
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err = retry.DoWithJitter(ctx, retryConfig, retry.DefaultRetryable, func() error {
		return client.UploadManifest()
	})
	if err != nil {
		return 0, fmt.Errorf("failed to upload manifest after retries: %w", err)
	}

	// pxarChunk.pos is the true archived byte count for this directory (more
	// accurate than the background size estimate, which only drives the %).
	return pxarChunk.pos, nil
}
