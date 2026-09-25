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

	"github.com/alphadose/haxmap"
	"machinebackuplib"
	"pbscommon"
	"retry"
	"security"
	"snapshot"
)

// BackupOptions contains all parameters for a backup operation
type BackupOptions struct {
	Ctx             context.Context // Cancel to request a graceful stop between backup steps
	BaseURL         string
	AuthID          string
	Secret          string
	Ticket          string // PBS session ticket (u/p login); preferred over AuthID/Secret when set
	CSRFToken       string
	Datastore       string
	Namespace       string
	CertFingerprint string
	BackupObjects      []string // Multiple directories or drives to backup
	BackupID        string
	BackupType      string // "host" for directory, "vm" for machine
	Kind            string // "disk", "directory", or "machine"
	UseVSS          bool
	Compression     string   // Compression level: "fastest", "default", "better", "best"
	ExcludeList     []string // User-configured exclusion patterns applied by the PXAR writer (H-04)
	DisableSplit    bool     // When true, never auto-split regardless of size
	SplitSizeBytes  uint64   // Auto-split threshold and per-bin target; 0 = default (SplitThreshold)
	OnProgress func(percent float64, message string)
	// OnComplete's message is always plain, already-formatted English text
	// (backward-compatible with every existing consumer). key/params are
	// additive — see msgcodes.go's doc comment — and let a consumer that
	// wants a localized version (main.go's JobHistory-building closures) get
	// one without parsing the message string back apart. key is "" when no
	// translation exists for this particular message (rare, kept for safety
	// rather than as a real code path).
	OnComplete func(success bool, message string, key MessageKey, params msgParams)
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

// Current in-flight backup cancellation. The GUI's Stop button calls
// CancelBackup() which cancels this context at a safe point (between
// directories / between retries), so the running backup aborts gracefully
// without committing a partial snapshot.
var (
	currentBackupCancelMutex sync.Mutex
	currentBackupCancel      context.CancelFunc
)

// CancelBackup requests a graceful stop of the currently running backup.
// It is exported to the GUI and returns false if no backup is running.
func (a *App) CancelBackup() error {
	currentBackupCancelMutex.Lock()
	defer currentBackupCancelMutex.Unlock()
	if currentBackupCancel != nil {
		currentBackupCancel()
		writeDebugLog("CancelBackup: cancellation requested for running backup")
	}
	writeDebugLog("CancelBackup: no backup running (or already cancelled)")
	return nil
}

// newBackupContext returns a fresh cancellable context and registers its
// cancel function as the current in-flight backup. Callers should not defer
// Cancel() (that would also cancel the shared token); instead call
// doneBackupContext() when the run finishes.
func newBackupContext() (context.Context, context.CancelFunc) {
	currentBackupCancelMutex.Lock()
	defer currentBackupCancelMutex.Unlock()
	if currentBackupCancel != nil {
		currentBackupCancel() // cancel any previous run before replacing
	}
	ctx, cancel := context.WithCancel(context.Background())
	currentBackupCancel = cancel
	return ctx, cancel
}

// doneBackupContext clears the shared cancellation token when a run finishes.
func doneBackupContext() {
	currentBackupCancelMutex.Lock()
	defer currentBackupCancelMutex.Unlock()
	currentBackupCancel = nil
}

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

// chunkUploadWorkers bounds how many chunk uploads for one archive run
// concurrently. Benchmarked 2026-09-24 against the real test PBS with fresh
// (non-dedupable) 4MB chunks: serial uploads topped out ~26 MB/s even on a
// quiet LAN (each upload waits out its own round trip before the next chunk
// starts); 8 concurrent workers nearly doubled that to ~46-52 MB/s, and
// 16/32 workers gave no further gain (same diminishing-returns shape the
// restore-side prefetch tuning found in didx_reader.go). 8 matches
// machinebackuplib's already-proven worker count for the same reason.
const chunkUploadWorkers = 8

// uploadJob is one chunk handed from the (single-threaded) scan/hash loop in
// HandleData/EOF to the upload worker pool started by Init.
type uploadJob struct {
	digest string
	data   []byte
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
	knownChunks         *haxmap.Map[string, bool]
	onProgress          func(float64, string)
	onStats             func(*BackupProgressStats) // Structured live stats for the GUI (nil for the catalog stream)
	currentDir          string                     // Directory currently being archived, for the stats payload
	lastProgressReport  uint64
	lastProgressPercent float64            // Track last reported percentage to prevent backwards progress
	totalSize           *atomic.Uint64     // Total size, updated by background scan
	uploadErrors        []string           // Collect upload errors to report at the end
	errorsMutex         sync.Mutex         // Protect uploadErrors slice

	// Pipelined chunk upload (added 2026-09-24, see chunkUploadWorkers doc
	// comment). HandleData/EOF stay single-threaded for scanning/hashing/
	// dedup bookkeeping (position-ordered, exactly as before) and only hand
	// the actual network upload off to this pool, so the next chunk's scan
	// and upload overlap instead of waiting for each other.
	uploadJobs chan uploadJob
	uploadWG   sync.WaitGroup
	fatalErrMu sync.Mutex
	fatalErr   error
}

func (c *ChunkState) Init(client *pbscommon.PBSClient, newchunk *atomic.Uint64, reusechunk *atomic.Uint64, failedchunk *atomic.Uint64, knownChunks *haxmap.Map[string, bool], onProgress func(float64, string), totalSize *atomic.Uint64, onStats func(*BackupProgressStats), currentDir string) {
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
	c.startUploadWorkers(client)
}

// startUploadWorkers launches the upload pool. c.wrid isn't set yet at this
// point (CreateDynamicIndex runs after Init, before any real data flows) —
// safe because each worker only reads c.wrid when a job arrives, and the
// channel send that delivers the first job happens-after c.wrid is written,
// per the Go memory model's channel-synchronization guarantee.
func (c *ChunkState) startUploadWorkers(client *pbscommon.PBSClient) {
	c.uploadJobs = make(chan uploadJob, chunkUploadWorkers*2)
	c.uploadWG.Add(chunkUploadWorkers)
	for i := 0; i < chunkUploadWorkers; i++ {
		go func() {
			defer c.uploadWG.Done()
			for job := range c.uploadJobs {
				retryConfig := retry.DefaultConfig()
				retryConfig.MaxAttempts = 5
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				err := retry.DoWithJitter(ctx, retryConfig, retry.DefaultRetryable, func() error {
					return client.UploadDynamicCompressedChunk(c.wrid, job.digest, job.data)
				})
				cancel()
				if err != nil {
					errMsg := fmt.Sprintf("⚠️  Failed to upload chunk %s after %d retries: %v", job.digest, retryConfig.MaxAttempts, err)
					writeBackupLog(errMsg)
					c.failedchunk.Add(1)
					c.errorsMutex.Lock()
					c.uploadErrors = append(c.uploadErrors, errMsg)
					c.errorsMutex.Unlock()
					// C3 fail-closed (see processChunk/EOF): latch the first fatal
					// error. EOF waits for every worker to drain, checks this, and
					// skips AssignDynamicChunks/CloseDynamicIndex if it's set — so a
					// chunk we couldn't upload never ends up referenced by a
					// finalized, unrestorable snapshot.
					c.fatalErrMu.Lock()
					if c.fatalErr == nil {
						c.fatalErr = fmt.Errorf("chunk upload failed, aborting to avoid committing a corrupt snapshot: %w", err)
					}
					c.fatalErrMu.Unlock()
					continue
				}
				c.newchunk.Add(1)
			}
		}()
	}
}

// checkFatal reports the first upload failure seen by any worker so far, if
// any. Called after every chunk is scanned so a background upload failure
// stops the walk promptly instead of only being noticed at EOF.
func (c *ChunkState) checkFatal() error {
	c.fatalErrMu.Lock()
	defer c.fatalErrMu.Unlock()
	return c.fatalErr
}

// processChunk hashes/dedups/bookkeeps c.currentChunk (the scanner's current
// break) and, for a new chunk, hands it to the upload worker pool instead of
// uploading inline — HandleData's caller (the pxar walk) can move on to
// scanning the next chunk immediately instead of blocking on this one's
// network round trip. Shared by HandleData's loop and EOF's final tail
// chunk, which used to duplicate this whole block.
func (c *ChunkState) processChunk(client *pbscommon.PBSClient) error {
	// A worker may have already latched a fatal error from an earlier chunk
	// (uploads happen in the background — see uploadJobs) — check before
	// hashing/queuing more work so a failure stops new uploads promptly
	// instead of only after HandleData's next call.
	if err := c.checkFatal(); err != nil {
		return err
	}

	h := sha256.New()
	if _, err := h.Write(c.currentChunk); err != nil {
		return fmt.Errorf("failed to hash chunk: %w", err)
	}
	bindigest := h.Sum(nil)
	shahash := hex.EncodeToString(bindigest)

	// GetOrSet marks the digest known BEFORE upload confirms, atomically, so
	// a duplicate of this chunk found later in the same stream (common —
	// e.g. runs of zeros) is never submitted twice while the first upload is
	// still in flight (same pattern machinebackuplib's uploadWorker already
	// uses in production). This is safe even though the upload could still
	// fail: on any upload failure c.fatalErr is latched and EOF aborts
	// before AssignDynamicChunks/CloseDynamicIndex, so an optimistically
	// "known" chunk from a failed run never ends up referenced by a
	// finalized snapshot (see checkFatal/EOF).
	if _, known := c.knownChunks.GetOrSet(shahash, true); !known {
		writeBackupLog(fmt.Sprintf("New chunk[%s] %d bytes", shahash, len(c.currentChunk)))
		c.uploadJobs <- uploadJob{digest: shahash, data: c.currentChunk}
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
			msg = fmt.Sprintf("Processed: %d MB (New: %d, Reused: %d, ⚠️ Failed: %d chunks)",
				sizeMB, c.newchunk.Load(), c.reusechunk.Load(), failed)
		} else {
			msg = fmt.Sprintf("Processed: %d MB (New: %d, Reused: %d chunks)",
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
				msg = fmt.Sprintf("Processed: %d / %d MB (New: %d, Reused: %d, ⚠️ Failed: %d chunks)",
					sizeMB, totalSize/(1024*1024), c.newchunk.Load(), c.reusechunk.Load(), failed)
			} else {
				msg = fmt.Sprintf("Processed: %d / %d MB (New: %d, Reused: %d chunks)",
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

	return c.checkFatal()
}

func (c *ChunkState) HandleData(b []byte, client *pbscommon.PBSClient) error {
	if err := c.checkFatal(); err != nil {
		return err
	}
	chunkpos := c.C.Scan(b)

	if chunkpos == 0 {
		c.currentChunk = append(c.currentChunk, b...)
	} else {
		for chunkpos > 0 {
			c.currentChunk = append(c.currentChunk, b[:chunkpos]...)

			if err := c.processChunk(client); err != nil {
				return err
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
		if err := c.processChunk(client); err != nil {
			return err
		}
	}

	// Every remaining chunk has been handed to the upload workers (possibly
	// still in flight) — stop accepting new jobs and wait for all of them to
	// finish before touching AssignDynamicChunks/CloseDynamicIndex below.
	close(c.uploadJobs)
	c.uploadWG.Wait()
	if err := c.checkFatal(); err != nil {
		return err
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

	// Wire a shared, user-cancellable context for this run (Stop button).
	ctx, cancel := newBackupContext()
	defer doneBackupContext()
	// Callers may supply their own context; otherwise use the shared one.
	if opts.Ctx == nil {
		opts.Ctx = ctx
	}
	_ = cancel

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
		opts.BaseURL, opts.Datastore, opts.Namespace, opts.AuthID, opts.Secret != "", len(opts.BackupObjects), opts.BackupID))
	hasToken := opts.AuthID != "" && opts.Secret != ""
	hasTicket := opts.Ticket != ""
	if opts.BaseURL == "" || (!hasToken && !hasTicket) {
		var missing []string
		if opts.BaseURL == "" {
			missing = append(missing, "BaseURL")
		}
		if !hasToken && !hasTicket {
			missing = append(missing, "API token (AuthID+Secret) or session ticket")
		}
		errMsg := fmt.Sprintf("PBS connection parameters required (missing: %s) — check the selected/default PBS server in the config",
			strings.Join(missing, ", "))
		writeBackupLog("[ERROR] " + errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	writeBackupLog("[DEBUG] Options validated")

	if len(opts.BackupObjects) == 0 {
		return fmt.Errorf("at least one backup directory or drive required")
	}

	// Splitting a backup into smaller parts is now an explicit, opt-in choice the
	// user makes in the GUI (the split plan is built and orchestrated there via
	// CreateBackupSplitPlan, which issues one StartBackup per part). RunBackupInline
	// therefore no longer sizes the directories or auto-splits by total size: that
	// pre-pass was what made a whole-drive backup of C:\ hang before it ever began,
	// and it also ran on every scheduled run. A scheduled/recurring backup is always
	// a normal (unsplit) backup — exactly the intended "split the first seed, full
	// afterwards" behavior.

	// ⭐ 2026-09-21 DESIGN DECISION: true single-session multi-archive backup.
	// Previously, 2+ selected folders were split HERE into N independent
	// runBackupInlineInternal() calls, each becoming its own PBS backup GROUP
	// (own backup-id, own backup-time, own snapshot) — the "weaker" multi-folder
	// feature documented in project_windows_pbs_client_fork.md as the thing this
	// fork exists to replace. That splitting is now retired in favor of PBS's
	// native multi-archive-per-snapshot capability (proven live against the
	// isolated test PBS, 2026-09-21: `proxmox-backup-client backup
	// dirA.pxar:/tmp/testA dirB.pxar:/tmp/testB` lands as ONE snapshot containing
	// both archives). runBackupInlineInternal now owns a single PBS session
	// (one Connect() / UploadManifest() / Finish()) covering every selected
	// folder as its own named archive within ONE combined snapshot — this is
	// the actual BFW-parity goal ("one job → one snapshot"), for 1 folder or
	// many alike. See the design-decision comment on runBackupInlineInternal
	// for how a mid-session failure is handled (best-effort per-directory vs.
	// abort-and-retry the whole session).
	return runBackupInlineInternal(opts)
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

	// Handle machine backup type (new Kind field)
	if opts.Kind == "machine" {
		return runMachineBackupInline(opts)
	}
	
	// Default to directory backup for "disk" or "directory" kinds
	// (existing logic handles these cases)

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
		if len(opts.BackupObjects) > 0 {
			opts.BackupID = GenerateBackupID(hostname, opts.BackupObjects[0])
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
	writeBackupLog(fmt.Sprintf("[DEBUG] Checking %d backup directories exist", len(opts.BackupObjects)))
	for idx, dir := range opts.BackupObjects {
		writeBackupLog(fmt.Sprintf("[DEBUG] Checking directory %d/%d: %s", idx+1, len(opts.BackupObjects), dir))
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			errMsg := fmt.Sprintf("Backup directory does not exist: %s", dir)
			errKey, errParams := MsgBackupDirNotExist, msgParams{"dir": dir}
			writeBackupLog(errMsg)
			if opts.OnComplete != nil {
				opts.OnComplete(false, errMsg, errKey, errParams)
			}
			if opts.OnResult != nil {
				opts.OnResult(&BackupStatus{
					Outcome:       OutcomeFailed,
					BackupID:      opts.BackupID,
					DurationSec:   time.Since(startTime).Seconds(),
					Message:       errMsg,
					MessageKey:    errKey,
					MessageParams: errParams,
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
		Ticket:           opts.Ticket,
		CSRFToken:        opts.CSRFToken,
		Datastore:        opts.Datastore,
		Namespace:        opts.Namespace,
		Insecure:         opts.CertFingerprint != "",
		CompressionLevel: compressionLevel,
		Manifest: pbscommon.BackupManifest{
			BackupID: opts.BackupID,
		},
	}

	writeBackupLog("[DEBUG] PBS client created, starting single-session multi-archive backup")

	hostname, _ := os.Hostname()

	// Counters and per-directory result accumulators for the whole run. Chunk
	// counters (newchunk/reusechunk/failedchunk) intentionally live OUTSIDE the
	// attempt loop below and keep accumulating across a whole-job retry: chunks
	// uploaded in a failed attempt are real work PBS already has (content-
	// addressed dedup), not phantom double-counting — only the FINAL committed
	// attempt's directories ever contribute to totalSize / the reported
	// Directories list.
	var newchunk atomic.Uint64
	var reusechunk atomic.Uint64
	var failedchunk atomic.Uint64
	var totalSize atomic.Uint64
	var dirErrors []string
	var dirResults []DirResult
	var allReadErrors, allSkipped, allExcluded []string
	successfulDirs := 0

	// failRun builds the failed BackupStatus, fires the callbacks and returns the
	// error — shared by every "whole run failed" exit below (all directories
	// failed, session lost and retries exhausted, manifest upload failed,
	// finalize failed) so they all report identically.
	failRun := func(errMsg string, key MessageKey, params msgParams) error {
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
			MessageKey:       key,
			MessageParams:    params,
		}
		if opts.OnComplete != nil {
			opts.OnComplete(false, errMsg, key, params)
		}
		if opts.OnResult != nil {
			opts.OnResult(status)
		}
		return fmt.Errorf("%s", errMsg)
	}

	// ⭐ 2026-09-21 DESIGN DECISION — per-directory vs. whole-session retry policy.
	//
	// Every selected folder now shares ONE PBS session (one Connect(), N named
	// archives, one UploadManifest()+Finish()) instead of each folder owning its
	// own disposable session (the old per-folder-own-group design this replaces).
	// That changes what a mid-run failure means and what can be salvaged from it:
	//
	//   - A DIRECTORY-LOCAL error (access denied, directory vanished mid-run,
	//     "produced 0 bytes") does NOT affect the shared H2 connection — the
	//     session is still healthy, only that one directory's archive failed to
	//     write. Best-effort: skip it, log it, keep going with the remaining
	//     directories in the SAME session. The final manifest simply omits the
	//     failed directory's archive; the run is reported OutcomePartial via the
	//     existing outcome machinery below — exactly what the old per-directory
	//     model already did for one bad folder.
	//
	//   - A SESSION-FATAL error (isFatalSessionError — the underlying HTTP/2
	//     connection itself died) is different in kind. The OLD per-directory
	//     model could retry "just this folder" because each folder had its own
	//     complete, disposable session. That does not carry over to a shared
	//     session: once the connection is confirmed dead, UploadManifest() and
	//     Finish() on that SAME client would also fail (same dead H2 connection),
	//     so there is no way to partially salvage or resume it. The only
	//     recovery is a brand-new Connect() — which starts an entirely new
	//     backup-time/snapshot server-side — so a session-fatal error aborts the
	//     WHOLE combined attempt and, if attempts remain, retries the WHOLE job
	//     (every directory, from scratch) after waiting for PBS to release the
	//     backup-group lock (same ~25 min wait the old code used, now scoped to
	//     the job instead of one directory). PBS's content-addressed chunk dedup
	//     makes a full retry cheap: only chunks new since the failed attempt
	//     cost anything to re-upload.
	//
	// Net effect: "one job = one snapshot" stays true — a dead connection can
	// never leave behind a half-committed multi-folder snapshot silently missing
	// a folder the user never asked to exclude — while an ordinary single-folder
	// read error (already survivable/partial in the old design) still doesn't
	// take down folders that archived fine.
	const maxJobAttempts = 2
	const sessionLostRetryWait = 25 * time.Minute

	for jobAttempt := 1; jobAttempt <= maxJobAttempts; jobAttempt++ {
		// A retried attempt redoes every directory from scratch (see decision
		// above), so only the LAST attempt's results should be reported.
		dirErrors = nil
		dirResults = nil
		allReadErrors = nil
		allSkipped = nil
		allExcluded = nil
		successfulDirs = 0
		usedArchiveNames := make(map[string]int)
		combinedACLs := &CombinedBackupFileMeta{
			Version:  FileMetaFormatVers,
			Captured: time.Now().UTC().Format(time.RFC3339),
			Host:     hostname,
			Archives: make(map[string]*BackupFileMeta),
		}

		writeBackupLog(fmt.Sprintf("[Session] Connecting (attempt %d/%d) for %d selected folder(s)",
			jobAttempt, maxJobAttempts, len(opts.BackupObjects)))
		client.Connect(false, "host")

		// ONE catalog dynamic index for the whole attempt, named exactly
		// "catalog.pcat1.didx" (PBS's web-UI browse feature hardcodes that
		// literal name server-side — see sharedCatalogCoordinator's doc
		// comment above backupDirectory for the full story). Every directory
		// below writes its catalog data into this same index; it is
		// finalized once, after the loop, not per directory.
		catalogChunk := ChunkState{}
		catalogChunk.Init(client, &newchunk, &reusechunk, &failedchunk, haxmap.New[string, bool](), nil, &totalSize, nil, "")
		var catalogIndexErr error
		catalogChunk.wrid, catalogIndexErr = client.CreateDynamicIndex("catalog.pcat1.didx")
		if catalogIndexErr != nil {
			client.Close()
			if jobAttempt >= maxJobAttempts {
				return failRun(fmt.Sprintf("failed to create shared catalog index: %v", catalogIndexErr),
					MsgCatalogCreateFailed, msgParams{"error": catalogIndexErr.Error()})
			}
			writeBackupLog(fmt.Sprintf("Failed to create shared catalog index (attempt %d/%d): %v — retrying whole job",
				jobAttempt, maxJobAttempts, catalogIndexErr))
			continue
		}
		sharedCatalog := &sharedCatalogCoordinator{chunk: &catalogChunk}

		var sessionFatal error
		for idx, dir := range opts.BackupObjects {
			if opts.Ctx != nil && opts.Ctx.Err() != nil {
				writeBackupLog(fmt.Sprintf("Cancellation requested — stopping before backup of %s", dir))
				client.Close()
				if opts.OnComplete != nil {
					opts.OnComplete(false, "Backup cancelled by user", MsgBackupCancelled, nil)
				}
				return fmt.Errorf("backup cancelled by user")
			}
			writeBackupLog(fmt.Sprintf("Starting archive %d/%d: %s", idx+1, len(opts.BackupObjects), dir))

			archiveBase := archiveBaseName(dir, usedArchiveNames)
			dirBytes, aclMeta, dirSkipped, dirExcluded, dirReadErrors, err :=
				backupDirectory(client, &newchunk, &reusechunk, &failedchunk, dir, archiveBase, opts.UseVSS, progress, opts.OnStats, opts.ExcludeList, sharedCatalog)

			allSkipped = append(allSkipped, dirSkipped...)
			allExcluded = append(allExcluded, dirExcluded...)
			allReadErrors = append(allReadErrors, dirReadErrors...)

			if err != nil {
				if isFatalSessionError(err) {
					writeBackupLog(fmt.Sprintf("Session lost while backing up %s: %v", dir, err))
					sessionFatal = err
					dirErrors = append(dirErrors, fmt.Sprintf("backup of %s failed: %v (session lost)", dir, err))
					dirResults = append(dirResults, DirResult{Path: dir, OK: false, Error: err.Error()})
					break // shared connection is dead — abort the rest of this attempt
				}
				errMsg := fmt.Sprintf("Backup failed for %s: %v", dir, err)
				writeBackupLog(errMsg)
				dirErrors = append(dirErrors, errMsg)
				dirResults = append(dirResults, DirResult{Path: dir, OK: false, Error: err.Error()})
				continue // directory-local error — best-effort, keep going
			}

			if aclMeta != nil {
				combinedACLs.Archives[archiveBase] = aclMeta
			}
			totalSize.Add(dirBytes)
			dirResults = append(dirResults, DirResult{Path: dir, OK: true})
			successfulDirs++
		}

		if sessionFatal != nil {
			// Session-fatal: tear down the dead session before retrying or giving up.
			client.Close()
			if jobAttempt >= maxJobAttempts {
				return failRun(fmt.Sprintf("PBS session lost and retries exhausted: %v", sessionFatal),
					MsgSessionLost, msgParams{"error": sessionFatal.Error()})
			}
			writeBackupLog(fmt.Sprintf("Waiting %s for PBS to release the backup group lock before retrying the whole job...",
				sessionLostRetryWait))
			waitUntil := time.Now().Add(sessionLostRetryWait)
			for {
				if opts.Ctx != nil && opts.Ctx.Err() != nil {
					writeBackupLog("Cancellation requested during session-lost wait — aborting")
					if opts.OnComplete != nil {
						opts.OnComplete(false, "Backup cancelled by user", MsgBackupCancelled, nil)
					}
					return fmt.Errorf("backup cancelled by user")
				}
				remaining := time.Until(waitUntil)
				if remaining <= 0 {
					break
				}
				progress(0, fmt.Sprintf("PBS session lost, waiting %s before a full retry (PBS lock being released)...",
					remaining.Round(time.Second)))
				sleepFor := 30 * time.Second
				if remaining < sleepFor {
					sleepFor = remaining
				}
				time.Sleep(sleepFor)
			}
			writeBackupLog("Wait complete, retrying the whole job with a fresh connection")
			continue
		}

		// Session stayed healthy for the whole attempt (whether every directory
		// succeeded or this is a best-effort partial run) — finalize it once,
		// covering every archive written above, and we're done: no whole-job
		// retry for directory-local errors (matches the old design, which also
		// never retried a non-fatal per-directory error).

		// Finalize the ONE shared catalog now that every directory that
		// succeeded this attempt has recorded itself into it: write the
		// combined root-pointer table (one entry per successful archive) and
		// close the dynamic index. Skipped entirely if nothing succeeded — the
		// successfulDirs==0 check right below fails the whole run in that case
		// and the never-closed empty index is abandoned along with the rest of
		// the dead attempt, same as an aborted .pxar index already would be.
		if len(sharedCatalog.entries) > 0 {
			if rootErr := pbscommon.WriteSharedCatalogRoot(sharedCatalog.writeCB(client), sharedCatalog.pos, sharedCatalog.entries); rootErr != nil {
				client.Close()
				return failRun(fmt.Sprintf("failed to finalize shared catalog: %v", rootErr),
					MsgCatalogFinalizeFailed, msgParams{"error": rootErr.Error()})
			}
			if eofErr := catalogChunk.EOF(client); eofErr != nil {
				client.Close()
				return failRun(fmt.Sprintf("failed to close shared catalog index: %v", eofErr),
					MsgCatalogCloseFailed, msgParams{"error": eofErr.Error()})
			}
		}

		if len(combinedACLs.Archives) > 0 {
			if aclBytes, aclErr := combinedACLs.Serialize(); aclErr != nil {
				writeBackupLog(fmt.Sprintf("WARNING: failed to serialize combined NTFS metadata: %v", aclErr))
			} else if upErr := client.UploadBlob(BackupAclsFilename, aclBytes); upErr != nil {
				writeBackupLog(fmt.Sprintf("WARNING: failed to upload combined NTFS metadata blob: %v", upErr))
			}
		}

		sidecar := &BackupSidecar{
			FormatVersion:    1,
			BackupID:         opts.BackupID,
			Directories:      opts.BackupObjects,
			GeneratedAt:      time.Now().Unix(),
			ExcludedByPolicy: excludedToIssues(allExcluded),
			SkippedReadError: skippedToIssues(allReadErrors),
		}
		if sidecarBytes, sErr := json.Marshal(sidecar); sErr != nil {
			writeBackupLog(fmt.Sprintf("WARNING: failed to serialize status sidecar: %v", sErr))
		} else if upErr := client.UploadBlob(BackupStatusFilename, sidecarBytes); upErr != nil {
			writeBackupLog(fmt.Sprintf("WARNING: failed to upload status sidecar: %v", upErr))
		}

		// If NO directory was backed up successfully, fail the whole backup.
		if successfulDirs == 0 {
			client.Close()
			return failRun(fmt.Sprintf("All %d directories failed:\n%s", len(opts.BackupObjects), strings.Join(dirErrors, "\n")),
				MsgAllDirsFailed, msgParams{"total": len(opts.BackupObjects), "errors": strings.Join(dirErrors, "\n")})
		}

		manifestCfg := retry.DefaultConfig()
		manifestCfg.MaxAttempts = 5
		manifestCtx, manifestCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		manifestErr := retry.DoWithJitter(manifestCtx, manifestCfg, retry.DefaultRetryable, func() error {
			return client.UploadManifest()
		})
		manifestCancel()
		if manifestErr != nil {
			client.Close()
			return failRun(fmt.Sprintf("Failed to upload manifest: %v", manifestErr),
				MsgManifestUploadFailed, msgParams{"error": manifestErr.Error()})
		}

		finishCfg := retry.DefaultConfig()
		finishCfg.MaxAttempts = 5
		finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		finishErr := retry.DoWithJitter(finishCtx, finishCfg, retry.DefaultRetryable, func() error {
			return client.Finish()
		})
		finishCancel()
		if finishErr != nil {
			client.Close()
			return failRun(fmt.Sprintf("Failed to finalize backup session: %v", finishErr),
				MsgSessionFinalizeFailed, msgParams{"error": finishErr.Error()})
		}

		writeBackupLog(fmt.Sprintf("Session finalized: %d/%d directories committed", successfulDirs, len(opts.BackupObjects)))
		break
	}

	// Calculate backup duration and size
	duration := time.Since(startTime)
	totalSizeMB := float64(totalSize.Load()) / (1024 * 1024)

	// Build completion message with duration, size, and chunk stats
	failed := failedchunk.Load()
	partial := len(dirErrors) > 0
	var completionMsg, progressMsg string
	var completionKey MessageKey
	var completionParams msgParams
	switch {
	case partial:
		completionMsg = fmt.Sprintf("⚠️  Partial backup in %s: %d/%d folders OK, %.1f MB (%d new, %d reused chunks)\nErrors:\n%s",
			formatDuration(duration), successfulDirs, len(opts.BackupObjects), totalSizeMB, newchunk.Load(), reusechunk.Load(), strings.Join(dirErrors, "\n"))
		progressMsg = fmt.Sprintf("Partial backup: %d/%d folders OK", successfulDirs, len(opts.BackupObjects))
		completionKey = MsgBackupPartial
		completionParams = msgParams{
			"duration": formatDuration(duration),
			"ok":       successfulDirs,
			"total":    len(opts.BackupObjects),
			"mb":       fmt.Sprintf("%.1f", totalSizeMB),
			"new":      newchunk.Load(),
			"reused":   reusechunk.Load(),
			"errors":   strings.Join(dirErrors, "\n"),
		}
	case failed > 0:
		completionMsg = fmt.Sprintf("⚠️  Backup completed with errors in %s: %.1f MB backed up (%d new, %d reused, %d FAILED chunks)",
			formatDuration(duration), totalSizeMB, newchunk.Load(), reusechunk.Load(), failed)
		progressMsg = fmt.Sprintf("Backup completed with %d failed chunks", failed)
		completionKey = MsgBackupCompletedWithErrors
		completionParams = msgParams{
			"duration": formatDuration(duration),
			"mb":       fmt.Sprintf("%.1f", totalSizeMB),
			"new":      newchunk.Load(),
			"reused":   reusechunk.Load(),
			"failed":   failed,
		}
	default:
		completionMsg = fmt.Sprintf("Backup completed in %s: %.1f MB backed up (%d new, %d reused chunks)",
			formatDuration(duration), totalSizeMB, newchunk.Load(), reusechunk.Load())
		progressMsg = "Backup completed"
		completionKey = MsgBackupCompleted
		completionParams = msgParams{
			"duration": formatDuration(duration),
			"mb":       fmt.Sprintf("%.1f", totalSizeMB),
			"new":      newchunk.Load(),
			"reused":   reusechunk.Load(),
		}
	}

	progress(1.0, progressMsg)

	// skipped rides along in every completion key's params (0 when there were
	// none) so the frontend can splice in the "N files skipped" clause itself
	// — see msgcodes.go's MsgSkippedFilesNote doc comment — without losing
	// that detail when a localized message_key is rendered instead of Message.
	completionParams["skipped"] = len(allSkipped)

	if len(allSkipped) > 0 {
		completionMsg += fmt.Sprintf("\n⚠️  %d files/folders skipped (access denied or junction points)", len(allSkipped))
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
		MessageKey:       completionKey,
		MessageParams:    completionParams,
	}

	// One machine-greppable result line for support (pairs with the start-of-run
	// target log): outcome label + the structured counters behind completionMsg.
	writeBackupLog(fmt.Sprintf("[RESULT] outcome=%s dirs_ok=%d/%d new=%d reused=%d failed=%d read_errors=%d excluded=%d bytes=%d duration=%s",
		status.Outcome, successfulDirs, len(opts.BackupObjects), status.NewChunks, status.ReusedChunks,
		status.FailedChunks, len(status.SkippedReadError), len(status.ExcludedByPolicy), status.TotalBytes, duration))

	// Additive (choice A): OnComplete keeps its (success, message) contract for
	// existing consumers; OnResult carries the full structured status for the
	// sidecar (Group 1) and rich history.
	if opts.OnComplete != nil {
		opts.OnComplete(status.Success(), completionMsg, completionKey, completionParams)
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

// runMachineBackupInline handles machine backup using machinebackuplib
func runMachineBackupInline(opts BackupOptions) error {
	startTime := time.Now()
	
	// Create machine backup config from opts
	cfg := &machinebackuplib.Config{
		BaseURL:         opts.BaseURL,
		CertFingerprint: opts.CertFingerprint,
		AuthID:          opts.AuthID,
		Secret:          opts.Secret,
		Ticket:          opts.Ticket,
		CSRFToken:       opts.CSRFToken,
		Datastore:       opts.Datastore,
		Namespace:       opts.Namespace,
		BackupID:        opts.BackupID,
		BackupType:      opts.BackupType,
		BackupDevices:   opts.BackupObjects,
	}

	// Progress callback wrapper. Returning true (user pressed Stop, which
	// cancels opts.Ctx) makes the backup abort without committing the index.
	progress := func(pct float64, msg string) bool {
		writeBackupLog(fmt.Sprintf("Backup progress: %.1f%% - %s", pct*100, msg))
		if opts.OnProgress != nil {
			opts.OnProgress(pct, msg)
		}
		return opts.Ctx != nil && opts.Ctx.Err() != nil
	}
	
	// Perform machine backup
	_, err := machinebackuplib.Backup(cfg, progress)
	if err != nil {
		// Handle error case
		errMsg := fmt.Sprintf("Machine backup failed: %v", err)
		errParams := msgParams{"error": err.Error()}
		writeBackupLog(errMsg)

		if opts.OnComplete != nil {
			opts.OnComplete(false, errMsg, MsgMachineBackupFailed, errParams)
		}
		if opts.OnResult != nil {
			opts.OnResult(&BackupStatus{
				Outcome:       OutcomeFailed,
				BackupID:      opts.BackupID,
				DurationSec:   time.Since(startTime).Seconds(),
				Message:       errMsg,
				MessageKey:    MsgMachineBackupFailed,
				MessageParams: errParams,
			})
		}
		return fmt.Errorf("%s", errMsg)
	}

	// Success case
	duration := time.Since(startTime)
	completionMsg := fmt.Sprintf("Machine backup completed in %s", formatDuration(duration))
	completionParams := msgParams{"duration": formatDuration(duration)}

	if opts.OnComplete != nil {
		opts.OnComplete(true, completionMsg, MsgMachineBackupCompleted, completionParams)
	}
	if opts.OnResult != nil {
		opts.OnResult(&BackupStatus{
			Outcome:       OutcomeVerifiedSuccess,
			BackupID:      opts.BackupID,
			DurationSec:   duration.Seconds(),
			Message:       completionMsg,
			MessageKey:    MsgMachineBackupCompleted,
			MessageParams: completionParams,
		})
	}
	
	return nil
}

// archiveBaseName derives a stable, PBS-safe archive identifier (no extension)
// from a backup directory's path. It is a pure function of the path — STABLE
// across runs for the same directory — so PBS's previous-archive dedup lookup
// (DownloadPreviousToBytes, called per directory below) keeps finding last
// run's version even if the user reorders the selected folders between runs.
// Uniqueness WITHIN one run is guaranteed via `used` (a run-scoped counter map
// the caller resets per attempt).
func archiveBaseName(dir string, used map[string]int) string {
	clean := filepath.Clean(dir)
	var b strings.Builder
	for _, r := range clean {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_' || r == '-':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r - 'A' + 'a')
		default:
			b.WriteByte('_')
		}
	}
	name := strings.Trim(b.String(), "_")
	if name == "" {
		name = "archive"
	}
	const maxLen = 80
	if len(name) > maxLen {
		// Keep a readable prefix + a short hash of the FULL path so two long,
		// same-prefix paths still end up with distinct, stable names.
		sum := sha256.Sum256([]byte(clean))
		suffix := hex.EncodeToString(sum[:4])
		name = name[:maxLen-len(suffix)-1] + "_" + suffix
	}
	if n, exists := used[name]; exists {
		used[name] = n + 1
		name = fmt.Sprintf("%s-%d", name, n+1)
	} else {
		used[name] = 1
	}
	return name
}

// sharedCatalogCoordinator threads ONE catalog dynamic index across every
// archive written in a job attempt, so the whole snapshot ends up with ONE
// combined catalog literally named "catalog.pcat1.didx" — the exact filename
// PBS's own web-UI "browse this snapshot" feature is hardcoded server-side to
// request, regardless of how many archives the snapshot has. Before this, the
// catalog was named per-archive-uniquely to dodge a collision when it was
// still one dynamic index per directory, which meant NO snapshot (not even a
// single-folder one) could be browsed in the PBS web UI anymore (found
// 2026-09-21, confirmed live: PBS returned "unable to read dynamic index
// .../catalog.pcat1.didx — No such file or directory" for every backup taken
// with that code). See the PXARArchive field docs in pbscommon/pxar.go for
// exactly how the shared byte stream and position bookkeeping works.
//
// One instance is created per job attempt (a whole-job retry starts a fresh
// one, same as every other per-attempt state in runBackupInlineInternal).
// prepare() is called once per directory before its WriteDir, record() once
// after it succeeds, in order; after the last directory, the caller uses
// entries/pos with pbscommon.WriteSharedCatalogRoot to finish the catalog and
// closes chunk via its own EOF — never done per-directory anymore.
type sharedCatalogCoordinator struct {
	chunk   *ChunkState
	pos     uint64
	entries []pbscommon.CatalogDir
}

// prepare configures dir's PXARArchive for its place in the shared catalog
// stream: route its catalog bytes into the one shared dynamic index, defer
// the single-archive root-pointer table WriteDir would otherwise write, and —
// for every archive after the first — skip the magic header and pick up the
// running position the previous archive left off at.
func (s *sharedCatalogCoordinator) prepare(archive *pbscommon.PXARArchive, client *pbscommon.PBSClient) {
	archive.CatalogWriteCB = func(b []byte) error {
		return s.chunk.HandleData(b, client)
	}
	archive.DeferCatalogRoot = true
	if len(s.entries) > 0 {
		archive.SkipCatalogMagic = true
		archive.InitialCatalogPos = s.pos
	}
}

// record captures one archive's outcome after a successful WriteDir call —
// its CatalogDir (name + this directory's table position) and its ending
// catalog position — so the next archive's prepare() and the final
// WriteSharedCatalogRoot call have what they need.
func (s *sharedCatalogCoordinator) record(dir pbscommon.CatalogDir, archive *pbscommon.PXARArchive) {
	s.entries = append(s.entries, dir)
	s.pos = archive.CatalogPos()
}

// writeCB returns the callback WriteSharedCatalogRoot writes the final
// combined root table through — the same shared chunk every archive's
// CatalogWriteCB already writes into, so the root table lands immediately
// after the last archive's own catalog data in that one dynamic index.
func (s *sharedCatalogCoordinator) writeCB(client *pbscommon.PBSClient) pbscommon.PXAROutCB {
	return func(b []byte) error {
		return s.chunk.HandleData(b, client)
	}
}

// backupDirectory archives one directory into an already-open PBS session
// (client.Connect was already called by the caller — see the design-decision
// comment on runBackupInlineInternal) as its own named archive (archiveBase).
// Returns the archived byte count, this directory's NTFS ACL metadata (nil on
// non-Windows or if nothing was collected — aggregated by the caller into ONE
// combined blob per snapshot, not uploaded here), and the logical
// skipped/excluded/read-error path lists for this directory. catalog is the
// whole job attempt's shared catalog coordinator (see above) — every
// directory's catalog data goes through the same one.
func backupDirectory(client *pbscommon.PBSClient, newchunk, reusechunk, failedchunk *atomic.Uint64, backupdir string, archiveBase string, usevss bool, progress func(float64, string), onStats func(*BackupProgressStats), excludeList []string, catalog *sharedCatalogCoordinator) (uint64, *BackupFileMeta, []string, []string, []string, error) {
	writeBackupLog(fmt.Sprintf("Starting backup of %s", backupdir))
	originalPath := backupdir

	if usevss {
		// VSS setup (checking writer status, creating the shadow copy) can take
		// a few seconds with nothing else to report — without this the UI is
		// left showing the previous, now-stale "Connecting to PBS..." message
		// for the whole pause. Real chunk-level progress from backupReal below
		// naturally overwrites this once the snapshot is ready.
		if progress != nil {
			progress(0.05, "Initialising Shadow Copy...")
		}

		var bytesArchived uint64
		var aclMeta *BackupFileMeta
		var skipped, excluded, readErrs []string
		err := snapshot.CreateVSSSnapshot([]string{backupdir}, true, func(snaps map[string]snapshot.SnapShot) error {
			for _, snap := range snaps {
				backupdir = snap.FullPath
				break
			}
			var e error
			bytesArchived, aclMeta, skipped, excluded, readErrs, e = backupReal(client, newchunk, reusechunk, failedchunk, backupdir, originalPath, archiveBase, usevss, progress, onStats, excludeList, catalog)
			return e
		})
		return bytesArchived, aclMeta, skipped, excluded, readErrs, err
	}

	return backupReal(client, newchunk, reusechunk, failedchunk, backupdir, originalPath, archiveBase, usevss, progress, onStats, excludeList, catalog)
}

func backupReal(client *pbscommon.PBSClient, newchunk, reusechunk, failedchunk *atomic.Uint64, backupdir string, originalPath string, archiveBase string, vssUsed bool, progress func(float64, string), onStats func(*BackupProgressStats), excludeList []string, catalog *sharedCatalogCoordinator) (returnBytes uint64, returnMeta *BackupFileMeta, returnSkipped, returnExcluded, returnReadErrors []string, returnErr error) {
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

	// NOTE: client.Connect() is called ONCE by the caller for the whole
	// multi-archive session (runBackupInlineInternal) — not per directory here.
	knownChunks := haxmap.New[string, bool]()

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

	// ⚠️ Archive + catalog names are now derived from archiveBase (unique per
	// directory within the session — see archiveBaseName above), NOT the old
	// fixed "backup.pxar.didx" / "catalog.pcat1.didx". Those fixed names were
	// harmless for a single directory but would silently COLLIDE across
	// multiple CreateDynamicIndex calls in one shared session (this was the
	// actual blocker for multi-archive-per-snapshot, found 2026-09-21).
	//
	// The catalog is deliberately kept PER-DIRECTORY (not one shared catalog
	// for the whole snapshot, which is what real proxmox-backup-client does for
	// a multi-mount host backup): PXARArchive.WriteDir(toplevel=true) writes a
	// one-time catalog magic header and resets catalog_pos to 8 on every
	// toplevel call (pbscommon/pxar.go), so feeding N directories into ONE
	// shared catalog stream would re-write that header and corrupt the byte
	// offsets partway through. Solved 2026-09-21 without touching that walk
	// logic at all: PXARArchive grew SkipCatalogMagic/InitialCatalogPos/
	// DeferCatalogRoot (see pbscommon/pxar.go) so several archives can write
	// their catalog data back-to-back into ONE caller-supplied stream — the
	// position tracking a standalone WriteDir already does internally
	// (catalog_pos) just gets seeded from the previous archive instead of
	// reset to 8. catalog (the sharedCatalogCoordinator passed into this
	// function) drives that; see its doc comment above backupDirectory.
	archiveName := archiveBase + ".pxar.didx"

	archive := &pbscommon.PXARArchive{}
	archive.ArchiveName = archiveName
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

	// Metadata collector: captures ACLs/owner/attrs (Windows) or extended
	// attributes including POSIX ACLs (Linux) for every entry during the
	// walk; no-op on other platforms. Its output is returned (raw, ungzipped)
	// to the caller, which aggregates every directory's metadata into ONE
	// combined blob for the whole snapshot (see runBackupInlineInternal)
	// instead of uploading one blob per directory.
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
	pxarChunk.Init(client, newchunk, reusechunk, failedchunk, knownChunks, progress, totalSize, onStats, backupdir)

	pxarChunk.wrid, err = client.CreateDynamicIndex(archive.ArchiveName)
	if err != nil {
		return 0, nil, nil, nil, nil, err
	}

	archive.WriteCB = func(b []byte) error {
		return pxarChunk.HandleData(b, client)
	}

	// Route this archive's catalog data into the ONE shared catalog index for
	// the whole job attempt (see sharedCatalogCoordinator above backupDirectory)
	// instead of creating/closing a catalog index of its own.
	catalog.prepare(archive, client)

	catalogDir, err := archive.WriteDir(backupdir, "", true)
	if err != nil {
		return 0, nil, nil, nil, nil, fmt.Errorf("failed to write directory archive: %w", err)
	}
	catalog.record(catalogDir, archive)

	// Map VSS shadow-copy paths back to the original logical root so the status
	// lists are meaningful to the user (no-op for non-VSS backups, where
	// backupdir == originalPath). Reused for both the aggregate status and sidecar.
	logicalSkipped := toLogicalPaths(archive.SkippedFiles, backupdir, originalPath)
	logicalExcluded := toLogicalPaths(archive.ExcludedFiles, backupdir, originalPath)
	logicalReadErrors := toLogicalPaths(archive.ReadErrors, backupdir, originalPath)

	if len(logicalSkipped) > 0 {
		writeBackupLog(fmt.Sprintf("Backup completed with %d skipped files/directories", len(logicalSkipped)))
	}
	if len(logicalExcluded) > 0 {
		writeBackupLog(fmt.Sprintf("%d files/directories excluded by user policy", len(logicalExcluded)))
	}

	// Guard: if WriteDir produced 0 data, the backup dir was effectively empty or inaccessible
	if pxarChunk.pos == 0 && len(pxarChunk.currentChunk) == 0 {
		return 0, nil, logicalSkipped, logicalExcluded, logicalReadErrors, fmt.Errorf("backup produced 0 bytes for %s — directory may be empty, inaccessible, or all files were excluded", backupdir)
	}

	if err = pxarChunk.EOF(client); err != nil {
		return 0, nil, logicalSkipped, logicalExcluded, logicalReadErrors, err
	}
	// The shared catalog index is NOT closed here — it stays open across every
	// directory in this attempt and is finalized once, by the caller, after
	// the whole per-directory loop (see runBackupInlineInternal).

	// Finalize (but do not upload) this directory's NTFS metadata; the caller
	// aggregates every directory's metadata into one combined blob for the
	// whole snapshot.
	var aclMeta *BackupFileMeta
	if m, aclErr := ntfsCollector.FinalizeRaw(); aclErr != nil {
		writeBackupLog(fmt.Sprintf("WARNING: failed to finalize NTFS metadata: %v", aclErr))
	} else if m != nil && len(m.Entries) > 0 {
		entries, uniqueSDDLs, metaErrs := ntfsCollector.Stats()
		writeBackupLog(fmt.Sprintf("NTFS metadata: %d entries, %d unique SDDLs, %d errors", entries, uniqueSDDLs, metaErrs))
		aclMeta = m
	}

	// pxarChunk.pos is the true archived byte count for this directory (more
	// accurate than the background size estimate, which only drives the %).
	return pxarChunk.pos, aclMeta, logicalSkipped, logicalExcluded, logicalReadErrors, nil
}
