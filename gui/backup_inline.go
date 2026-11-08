package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// formatByteSize auto-scales a byte count (KB/MB/GB/TB) for the "Processed:"
// progress message below — that message used to hardcode a ÷1024/1024 MB
// divisor unconditionally, unreadable well past 1000 (a real 478GB job
// showed "Processed: 489472 / 489472 MB"). Mirrors the frontend's own
// formatBytes (App.jsx) in spirit; kept as a separate, simpler Go
// implementation since a debug-log string has no need to match its exact
// decimal-place formatting.
//
// Both units are shown, decimal first and binary in brackets, the same
// convention as the GUI's speed ("357.9 GB (333.3 GiB)"): this used to divide
// by 1024 and label the result GB, so figures disagreed with PBS (decimal)
// and looked wrong next to Windows Explorer (binary, labelled GB).
//
// The binary figure uses the decimal one's unit step, so the two read as the
// same amount: "1.1 GB (1.0 GiB)", not "1.1 GB (1004.3 MiB)".
func formatByteSize(bytes uint64) string {
	if bytes < 1000 {
		return fmt.Sprintf("%d B", bytes)
	}
	dec, bin := []string{"B", "KB", "MB", "GB", "TB"}, []string{"B", "KiB", "MiB", "GiB", "TiB"}
	v, u := float64(bytes), 0
	for v >= 999.95 && u < len(dec)-1 { // 999.95 would print as 1000.0
		v /= 1000
		u++
	}
	b := float64(bytes)
	for i := 0; i < u; i++ {
		b /= 1024
	}
	return fmt.Sprintf("%.1f %s (%.1f %s)", v, dec[u], b, bin[u])
}

// BackupOptions contains all parameters for a backup operation
type BackupOptions struct {
	Ctx context.Context // Cancel to request a graceful stop between backup steps
	// Run is the run this backup is (backup_runs.go): its Stop, Pause and log
	// file. The caller registers it before queueing; nil makes one here.
	Run             *backupRun
	BaseURL         string
	AuthID          string
	Secret          string
	Ticket          string // PBS session ticket (u/p login); preferred over AuthID/Secret when set
	CSRFToken       string
	Datastore       string
	Namespace       string
	CertFingerprint string
	BackupObjects   []string // Multiple directories or drives to backup
	BackupID        string
	BackupType      string // "host" for directory, "vm" for machine
	Kind            string // "disk", "directory", or "machine"
	UseVSS          bool
	Compression     string   // Compression level: "fastest", "default", "better", "best"
	ExcludeList     []string // User-configured exclusion patterns applied by the PXAR writer (H-04)
	DisableSplit    bool     // When true, never auto-split regardless of size
	SplitSizeBytes  uint64   // Auto-split threshold and per-bin target; 0 = default (SplitThreshold)
	// PrefetchWorkers > 0 enables per-directory background read-ahead in
	// pbscommon.PXARArchive (see its own doc comment) — 0 (the default,
	// matching Config.ParallelBackupRead off) is today's fully synchronous
	// read path. Machine/whole-disk backups don't use PXARArchive at all, so
	// this only affects directory-type backups.
	PrefetchWorkers int
	// ChangeDetectionMode is "" / "legacy" (one .pxar.didx per folder, the
	// default), "data" or "metadata" (split .mpxar.didx + .ppxar.didx; see
	// pbscommon/change_detection.go). metadata behaves like data until reuse
	// of unchanged files is implemented.
	ChangeDetectionMode string
	// ScheduledFullRead marks a run of a metadata-mode Backup Set turned into
	// a full read by its "full read every N runs" setting, so the card can
	// say why every file is being read.
	ScheduledFullRead bool
	// FullReadRequested: a metadata-mode set's run started with Run now
	// (full read).
	FullReadRequested bool
	// Comment is sent as-is as the PBS snapshot's manifest comment (shows up
	// next to the snapshot in PBS's own web UI). Callers set this to the
	// triggering Backup Set's own name — see startBackupDirect/
	// startMachineBackupDirect, which reuse the exact same name (or
	// "Manual backup - X"/"Backup machine - X" fallback for a genuine
	// one-off) that Reports already labels the run with, rather than
	// inventing a second, separate description field.
	Comment string
	// Crypt encrypts every chunk and signs the manifest. Nil means a plain
	// snapshot. Populated from Config.Crypt (Config.EncryptionKeyFile).
	Crypt *pbscommon.CryptConfig
	// ReadFreeSpace (machine backups): read free space too, a forensic copy.
	ReadFreeSpace bool
	// LiveRead (Linux machine backups): read mounted disks without a
	// snapshot, as the set's owner accepted (ScheduledJob.LiveRead).
	LiveRead bool
	// SkipPartitions (machine backups): the partitions to leave out of each
	// disk, keyed by its device (ScheduledJob.SkipPartitions).
	SkipPartitions map[string][]machinebackuplib.PartitionSkip
	// OnSnapshot receives, before OnComplete, a way to add a note to the
	// snapshot a successful run made (the post-backup actions use it when
	// the notification e-mail cannot be sent).
	OnSnapshot func(snapshotNoter)
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
	// OnFile reports the file currently being archived (directory backups only,
	// sampled about twice a second and only when it changed).
	OnFile func(path string)

	// ShareLogins are the settings' network share logins (share_logins.go),
	// used for folders on \\server\share when the service runs the backup.
	ShareLogins map[string]*ShareLogin
}

// The rules for a lost connection to PBS (retry.SessionLost, retry.Reconnectable
// and the waits before running a backup again) are in pkg/retry, shared with
// the command-line tools.

// skippedSummary is the plain-text tail of a folder backup's completion
// message for what it skipped: a warning for the files it could not read,
// and a plain note for the links, sockets, pipes and system files it leaves
// out on purpose (on 24/10 a Linux home folder's five "access denied" were
// all sockets). The backup log lists each one either way.
func skippedSummary(skipped, unreadable int) string {
	s := ""
	if unreadable > 0 {
		s += fmt.Sprintf("\n⚠️  %d files/folders could not be read (the backup log lists them)", unreadable)
	}
	if byDesign := skipped - unreadable; byDesign > 0 {
		s += fmt.Sprintf("\n%d items left out by design: links, sockets, pipes and system files (the backup log lists them)", byDesign)
	}
	return s
}

// waitOrCancel waits for d, or until ctx is cancelled (false).
func waitOrCancel(ctx context.Context, d time.Duration) bool {
	if ctx == nil {
		time.Sleep(d)
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// cardRefresh is how often a running backup sends the progress card its
// figures (and a folder backup its current file): 10 times a second, so the
// numbers roll instead of ticking. Each update is one small event and a
// redraw; measured on an i5-7200U during a machine backup, the whole display
// side took about 3% of one core at 5 events a second. Speeds and the finish
// forecast are averaged over 10 s and 2 min in the frontend, so a faster
// cadence does not make them jumpier.
const cardRefresh = 100 * time.Millisecond

// Stop, Pause and each run's log are per run now (backup_runs.go), and which
// backups may run side by side is decided before a run starts
// (operation_queue.go). There used to be one cancel, one pause and one lock
// per server and datastore here, which kept every folder backup to the same
// datastore one at a time.

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
	size, _, err := calculateDirSizeFilesCtx(ctx, path)
	return size, err
}

// calculateDirSizeFilesCtx is calculateDirSizeCtx that also counts the files,
// for the backup card's "Files: done / total".
func calculateDirSizeFilesCtx(ctx context.Context, path string) (uint64, uint64, error) {
	var totalSize, totalFiles uint64
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
			totalFiles++
		}
		return nil
	})

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		// Partial size is still useful for the split heuristic.
		return totalSize, totalFiles, err
	}
	if accessDenied && totalSize == 0 {
		return 0, 0, fmt.Errorf("access denied: %s", path)
	}

	return totalSize, totalFiles, err
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

// uploadJob is one chunk handed from the (single-threaded) boundary search in
// HandleData/EOF to the worker pool started by Init: its index entry, which
// the worker completes with the digest, and its data.
type uploadJob struct {
	entry *chunkEntry
	data  []byte
}

// chunkEntry is one chunk of the index being built: where it starts in the
// stream, its length and its digest.
type chunkEntry struct {
	offset, size uint64
	digest       [32]byte
}

// chunkBufferSize is the room a chunk's buffer is made with: the chunker's
// average, so most chunks are built without the buffer growing and being
// copied (a chunk can reach four times this, and then grows).
const chunkBufferSize = 4 << 20

// jobProgress is shared across every directory in one backup attempt so the
// live progress bar reflects the WHOLE job, not just whichever directory is
// currently being archived. Found live 2026-09-26: backed up one local
// folder and one network share in the same Backup Set — the displayed
// running total stayed at the size of whichever directory's own background
// scan happened to have populated a PRIVATE, per-directory total, even
// though the two folders together came to more. sizeEstimate and
// bytesDoneBase both only ever grow within one attempt (a retried attempt
// gets a fresh jobProgress, same lifetime as the pre-existing totalSize
// accumulator this replaces), so the percentage this feeds is monotonic by
// construction; lastPercent is the belt-and-braces guard against a momentary
// dip right at a directory boundary (sizeEstimate growing slightly out of
// step with bytesDoneBase as each directory's own scan completes at its own
// pace).
type jobProgress struct {
	sizeEstimate  atomic.Uint64 // sum of every directory's own background-scanned size, growing as each dir's scan completes
	bytesDoneBase atomic.Uint64 // bytes already archived by directories that finished before the one currently in progress

	currentFile atomic.Value // string: logical path of the file most recently started

	// Metadata change detection totals across every directory of the job.
	reusedFiles atomic.Uint64
	reusedBytes atomic.Uint64

	lastPercentMu sync.Mutex
	lastPercent   float64

	// Live figures for the backup card (see emitStats). curPos is the data
	// position of the directory in progress, so bytesDoneBase+curPos is the
	// job's bytes done; filesEstimate counts files as sizeEstimate counts
	// bytes; scansDone counts finished background scans out of scansWanted.
	// prevBytes/prevFiles total the previous snapshot of each directory that
	// metadata mode compares with: a total known straight away, used until
	// the scans finish.
	curPos        atomic.Uint64
	filesEstimate atomic.Uint64
	filesDone     atomic.Uint64
	uploadedBytes atomic.Uint64
	scansDone     atomic.Int32
	scansWanted   atomic.Int32
	prevBytes     atomic.Uint64
	prevFiles     atomic.Uint64
	currentDir    atomic.Value // string
	phase         atomic.Value // string: "connecting", "data", "finishing"
	readReason    atomic.Value // string, see BackupProgressStats.ReadReason
	// anyReuse is set once a directory compares with a usable previous
	// snapshot; a split run without it read every file.
	anyReuse atomic.Bool
}

// totals returns the job's byte and file totals: the background scans' once
// they have all finished, otherwise the larger of what they have so far and
// the previous snapshots' totals (0 while neither is known).
func (p *jobProgress) totals() (bytes, files uint64) {
	bytes, files = p.sizeEstimate.Load(), p.filesEstimate.Load()
	if p.scansDone.Load() < p.scansWanted.Load() {
		if b := p.prevBytes.Load(); b > bytes {
			bytes = b
		}
		if f := p.prevFiles.Load(); f > files {
			files = f
		}
	}
	return bytes, files
}

// percent is the job's progress so far (0..1), as last reported; 0 for nil.
func (p *jobProgress) percent() float64 {
	if p == nil {
		return 0
	}
	p.lastPercentMu.Lock()
	defer p.lastPercentMu.Unlock()
	return p.lastPercent
}

func loadString(v *atomic.Value) string {
	s, _ := v.Load().(string)
	return s
}

type ChunkState struct {
	// entries is the index being built, in the stream's order: each chunk's
	// offset and size, recorded by the walk, and its digest, which a worker
	// fills in (or the walk, for a chunk reused without its data). The index
	// list and its checksum are made from it once every worker has finished
	// (EOF), so they come out as if the walk had hashed each chunk.
	entries            []*chunkEntry
	pos                uint64
	wrid               uint64
	chunkcount         uint64
	currentChunk       []byte
	C                  pbscommon.Chunker
	newchunk           *atomic.Uint64
	reusechunk         *atomic.Uint64
	failedchunk        *atomic.Uint64 // Track failed chunk uploads
	knownChunks        *haxmap.Map[string, bool]
	onProgress         func(float64, string)
	onStats            func(*BackupProgressStats) // Structured live stats for the GUI (nil for the catalog stream)
	currentDir         string                     // Directory currently being archived, for the stats payload
	lastProgressReport uint64
	progress           *jobProgress // Shared job-wide totals — see jobProgress's doc comment
	uploadErrors       []string     // Collect upload errors to report at the end
	errorsMutex        sync.Mutex   // Protect uploadErrors slice

	// The worker pool (added 2026-09-24 for uploads, see chunkUploadWorkers).
	// HandleData/EOF only find the chunk boundaries and record each chunk's
	// place in the index; a worker hashes the chunk, checks it against the
	// chunks PBS has, and compresses and sends it if new. Hashing used to be
	// done by the walk itself, and SHA-256 was the largest part of the time
	// one stream took (most of all where Go has no fast SHA-256: a CPU
	// without AVX2, or a VM given a generic CPU type), so a stream ran at one
	// core's hashing speed with the other cores idle.
	uploadJobs chan uploadJob
	uploadWG   sync.WaitGroup
	closeOnce  sync.Once
	fatalErrMu sync.Mutex
	fatalErr   error

	// cancelCtx is the Stop button's context: it ends the upload retries,
	// and chunks still queued when it is cancelled are not sent (upstream
	// #117). nil = never cancelled.
	cancelCtx context.Context
	// logCtx names the run this stream belongs to, for its log lines when
	// cancelCtx is not set (the catalog stream).
	logCtx context.Context
}

// logContext is the context whose run's log file this stream writes to.
func (c *ChunkState) logContext() context.Context {
	if c.cancelCtx != nil {
		return c.cancelCtx
	}
	return c.logCtx
}

// retryParent is the context the chunk upload retries derive from: the Stop
// button's one when set, so a cancel ends the retries and their waits.
func (c *ChunkState) retryParent() context.Context {
	if c.cancelCtx != nil {
		return c.cancelCtx
	}
	return context.Background()
}

// stopped reports whether the Stop button's context has been cancelled.
func (c *ChunkState) stopped() bool {
	return c.cancelCtx != nil && c.cancelCtx.Err() != nil
}

// latchFatal records err as the first fatal error, if none is recorded yet.
func (c *ChunkState) latchFatal(err error) {
	c.fatalErrMu.Lock()
	if c.fatalErr == nil {
		c.fatalErr = err
	}
	c.fatalErrMu.Unlock()
}

// closeUploads stops the upload workers taking new jobs; they end after the
// one they are on. EOF calls it and then waits for them; a backup that ends
// with an error (a Stop, a failed walk) calls it without waiting, so the
// workers do not stay behind once the session is dropped.
func (c *ChunkState) closeUploads() {
	if c.uploadJobs == nil {
		return
	}
	c.closeOnce.Do(func() { close(c.uploadJobs) })
}

func (c *ChunkState) Init(client *pbscommon.PBSClient, newchunk *atomic.Uint64, reusechunk *atomic.Uint64, failedchunk *atomic.Uint64, knownChunks *haxmap.Map[string, bool], onProgress func(float64, string), progress *jobProgress, onStats func(*BackupProgressStats), currentDir string) {
	c.entries = nil
	c.pos = 0
	c.chunkcount = 0
	c.currentChunk = make([]byte, 0, chunkBufferSize)
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
	c.progress = progress
	c.uploadErrors = make([]string, 0)
	c.startUploadWorkers(client)
}

// startUploadWorkers launches the worker pool. c.wrid isn't set yet at this
// point (CreateDynamicIndex runs after Init, before any real data flows) —
// safe because each worker only reads c.wrid when a job arrives, and the
// channel send that delivers the first job happens-after c.wrid is written,
// per the Go memory model's channel-synchronization guarantee. A worker
// writes the digest into the job's entry; the walk reads the entries only
// after EOF has waited for every worker.
func (c *ChunkState) startUploadWorkers(client *pbscommon.PBSClient) {
	c.uploadJobs = make(chan uploadJob, chunkUploadWorkers*2)
	c.uploadWG.Add(chunkUploadWorkers)
	for i := 0; i < chunkUploadWorkers; i++ {
		go func() {
			defer c.uploadWG.Done()
			for job := range c.uploadJobs {
				// Stopped: send nothing more. The session is dropped, so
				// the walk only needs to see the cancel (checkFatal).
				if c.stopped() {
					c.latchFatal(c.cancelCtx.Err())
					continue
				}
				// After a failed chunk the snapshot is not finished:
				// hash and send nothing more.
				if c.checkFatal() != nil {
					continue
				}
				// The digest a chunk is published under: sha256(plaintext),
				// or sha256(plaintext || id_key) when the snapshot is
				// encrypted.
				job.entry.digest = client.ChunkDigest(job.data)
				shahash := hex.EncodeToString(job.entry.digest[:])
				// GetOrSet marks the digest known BEFORE upload confirms,
				// atomically, so a duplicate of this chunk elsewhere in the
				// stream (common, runs of zeros for one), in another worker,
				// is never submitted twice while the first upload is still in
				// flight (the pattern machinebackuplib's uploadWorker uses).
				// This is safe even though the upload could still fail: on
				// any upload failure c.fatalErr is latched and EOF aborts
				// before AssignDynamicChunks/CloseDynamicIndex, so an
				// optimistically "known" chunk from a failed run never ends
				// up referenced by a finalized snapshot (see checkFatal/EOF).
				if _, known := c.knownChunks.GetOrSet(shahash, true); known {
					runLog(c.logContext(), fmt.Sprintf("Reuse chunk[%s] %d bytes", shahash, len(job.data)))
					c.reusechunk.Add(1)
					continue
				}
				runLog(c.logContext(), fmt.Sprintf("New chunk[%s] %d bytes", shahash, len(job.data)))
				retryConfig := retry.DefaultConfig()
				retryConfig.MaxAttempts = 5
				// Each attempt is bounded by the upload's own watch
				// (pbscommon/upload_stall.go): given up once it stops
				// moving, never while it moves. So the attempts do not share
				// one time limit: with 5 minutes for all of them, a first
				// attempt that stalled used it all and the chunk was never
				// sent again.
				attempt := 0
				err := retry.DoWithJitter(c.retryParent(), retryConfig, retry.DefaultRetryable, func() error {
					attempt++
					uerr := client.UploadDynamicCompressedChunk(c.wrid, shahash, job.data)
					if uerr != nil && attempt < retryConfig.MaxAttempts && retry.DefaultRetryable(uerr) && !c.stopped() {
						runLog(c.logContext(), fmt.Sprintf("⚠️  Chunk %s: upload attempt %d of %d failed, sending it again: %v", shahash, attempt, retryConfig.MaxAttempts, uerr))
					}
					return uerr
				})
				if err != nil && c.stopped() {
					// Ended by Stop, not a failed chunk.
					c.latchFatal(c.cancelCtx.Err())
					continue
				}
				if err != nil {
					errMsg := fmt.Sprintf("⚠️  Failed to upload chunk %s after %d retries: %v", shahash, retryConfig.MaxAttempts, err)
					runLog(c.logContext(), errMsg)
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
				if c.progress != nil {
					c.progress.uploadedBytes.Add(uint64(len(job.data)))
				}
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

// processChunk records c.currentChunk (the scanner's current break) in the
// index and hands it to the worker pool, which hashes it and sends it if PBS
// does not have it, so HandleData's caller (the pxar walk) moves on to the
// next boundary at once. c.currentChunk is a new buffer afterwards. Shared by
// HandleData's loop, the boundary calls and EOF's final tail chunk.
func (c *ChunkState) processChunk(client *pbscommon.PBSClient) error {
	// A worker may have already latched a fatal error from an earlier chunk
	// (uploads happen in the background — see uploadJobs) — check before
	// hashing/queuing more work so a failure stops new uploads promptly
	// instead of only after HandleData's next call.
	if err := c.checkFatal(); err != nil {
		return err
	}

	// Its place in the index now, its digest from the worker that hashes
	// it; the next chunk starts in a new buffer.
	e := &chunkEntry{offset: c.pos, size: uint64(len(c.currentChunk))}
	c.uploadJobs <- uploadJob{entry: e, data: c.currentChunk}
	c.currentChunk = make([]byte, 0, chunkBufferSize)
	return c.recordChunk(e)
}

// recordChunk appends a chunk to the index being built and reports progress.
func (c *ChunkState) recordChunk(e *chunkEntry) error {
	size := e.size
	c.entries = append(c.entries, e)
	c.pos += size
	c.chunkcount += 1

	// The data stream that drives progress (onStats set: the payload stream
	// of a split archive, the archive itself otherwise) publishes its
	// position for the backup card's live figures (emitStats).
	if c.onStats != nil && c.progress != nil {
		c.progress.curPos.Store(c.pos)
	}

	// Report progress every 10 MB
	if c.onProgress != nil && c.pos-c.lastProgressReport > 10*1024*1024 {
		c.lastProgressReport = c.pos

		// Calculate progress against the WHOLE JOB's total, not just this one
		// directory's — bytesDone/totalSize both sum across every directory in
		// the job (see jobProgress's doc comment). c.pos alone is only this
		// directory's own bytes; bytesDoneBase adds back what earlier
		// directories in the same job already contributed.
		failed := c.failedchunk.Load()
		bytesDone := c.progress.bytesDoneBase.Load() + c.pos
		totalSize, _ := c.progress.totals()

		// The percentage is the share of the data done, capped at 99% until
		// the run has finished: it used to be squeezed into 10%..90%, so
		// 442.2 of 447.7 GB (98.8%) showed as "89%" (seen live 2026-10-07).
		// With no total yet it holds where it is; the card shows the total
		// as "Calculating..." meanwhile.
		c.progress.lastPercentMu.Lock()
		progress := c.progress.lastPercent
		if totalSize > 0 {
			progress = float64(bytesDone) / float64(totalSize)
			if progress > 0.99 {
				progress = 0.99
			}
		}
		// Never report backwards progress. This guard is on the SHARED
		// jobProgress, not this one directory's ChunkState, precisely so it
		// still holds across a directory boundary: totalSize can grow as each
		// directory's own background size-scan completes at its own pace,
		// which without this could otherwise show as a momentary dip right
		// when a new directory starts.
		if progress < c.progress.lastPercent {
			progress = c.progress.lastPercent
		}
		c.progress.lastPercent = progress
		c.progress.lastPercentMu.Unlock()

		processed := formatByteSize(bytesDone)
		if totalSize > 0 {
			processed += " / " + formatByteSize(totalSize)
		}
		var msg string
		if failed > 0 {
			msg = fmt.Sprintf("Processed: %s (New: %d, Reused: %d, ⚠️ Failed: %d chunks)",
				processed, c.newchunk.Load(), c.reusechunk.Load(), failed)
		} else {
			msg = fmt.Sprintf("Processed: %s (New: %d, Reused: %d chunks)",
				processed, c.newchunk.Load(), c.reusechunk.Load())
		}
		c.onProgress(progress, msg)
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

			b = b[chunkpos:]
			chunkpos = c.C.Scan(b)
		}
		c.currentChunk = append(c.currentChunk, b...)
	}
	return nil
}

// SuggestBoundary cuts the current chunk here if that gives a chunk within
// the chunker's normal size bounds (average/4 to average*4), the same rule as
// the official client's payload chunker. Used at file starts in a split
// archive's payload stream so unchanged files line up on chunk boundaries.
func (c *ChunkState) SuggestBoundary(client *pbscommon.PBSClient) error {
	n := uint64(len(c.currentChunk))
	if n < c.C.MinSize() || n > c.C.MaxSize() {
		return nil
	}
	if err := c.processChunk(client); err != nil {
		return err
	}
	c.C.Reset()
	return nil
}

// ForceBoundary ends the chunk in progress here, whatever its size, so the
// next byte starts a new chunk. Metadata change detection needs this before
// it appends reused chunks.
func (c *ChunkState) ForceBoundary(client *pbscommon.PBSClient) error {
	if len(c.currentChunk) > 0 {
		if err := c.processChunk(client); err != nil {
			return err
		}
	}
	c.C.Reset()
	return nil
}

// InjectKnownChunk appends a chunk of the previous snapshot's index without
// its data. The chunk must be one the server registered for this session,
// i.e. listed in the previous index downloaded through /previous.
func (c *ChunkState) InjectKnownChunk(digest [32]byte, size uint64) error {
	if err := c.checkFatal(); err != nil {
		return err
	}
	if len(c.currentChunk) != 0 {
		return fmt.Errorf("internal error: reused chunk appended while %d bytes are pending", len(c.currentChunk))
	}
	shahash := hex.EncodeToString(digest[:])
	if _, known := c.knownChunks.Get(shahash); !known {
		return fmt.Errorf("internal error: reused chunk %s is not in the previous index", shahash)
	}
	c.reusechunk.Add(1)
	return c.recordChunk(&chunkEntry{offset: c.pos, size: size, digest: digest})
}

// indexList returns the index in the stream's order, as PBS takes it (each
// chunk's digest and offset), and the index checksum: SHA-256 over each
// chunk's end offset (u64 little endian) and digest. Call it only once every
// worker has finished.
func (c *ChunkState) indexList() (digests []string, offsets []uint64, csum string) {
	h := sha256.New()
	var end [8]byte
	for _, e := range c.entries {
		binary.LittleEndian.PutUint64(end[:], e.offset+e.size)
		h.Write(end[:])
		h.Write(e.digest[:])
		digests = append(digests, hex.EncodeToString(e.digest[:]))
		offsets = append(offsets, e.offset)
	}
	return digests, offsets, hex.EncodeToString(h.Sum(nil))
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
	c.closeUploads()
	c.uploadWG.Wait()
	if err := c.checkFatal(); err != nil {
		return err
	}

	// Assign chunks in batches with retry
	retryConfig := retry.DefaultConfig()
	retryConfig.MaxAttempts = 5
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	allDigests, allOffsets, digest := c.indexList()
	for k := 0; k < len(allDigests); k += 128 {
		k2 := k + 128
		if k2 > len(allDigests) {
			k2 = len(allDigests)
		}

		// Capture loop variables for closure
		batchStart, batchEnd := k, k2
		assignments := allDigests[batchStart:batchEnd]
		offsets := allOffsets[batchStart:batchEnd]

		err := retry.DoWithJitter(ctx, retryConfig, retry.DefaultRetryable, func() error {
			return client.AssignDynamicChunks(c.wrid, assignments, offsets)
		})
		if err != nil {
			return fmt.Errorf("failed to assign chunks (batch %d-%d) after retries: %w", batchStart, batchEnd, err)
		}
	}

	// Close index with retry
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

	// The run's own Stop and Pause (the caller registered it before it
	// queued; a caller without one gets one here). Its context carries the
	// run and this log file down to everything the backup starts.
	run := opts.Run
	if run == nil {
		run = newBackupRun(opts.Comment, opts.Kind, opts.BackupID)
		defer run.end()
	}
	if opts.Ctx == nil {
		opts.Ctx = run.ctx
	}
	opts.Ctx = withRun(opts.Ctx, run, runLogger)
	run.logger.Store(runLogger)
	defer run.logger.Store(nil)

	// CRITICAL: Panic recovery to prevent silent goroutine death (scheduler launches backups in goroutines)
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("CRITICAL: Backup panic in RunBackupInline: %v", r)
			runLog(opts.Ctx, errMsg)
			// Get stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			runLog(opts.Ctx, fmt.Sprintf("Stack trace:\n%s", buf[:n]))
			returnErr = fmt.Errorf("backup panic: %v", r)
		}
	}()

	userName := "<unknown>"
	if cu, err := user.Current(); err == nil {
		userName = cu.Username
	}
	runLog(opts.Ctx, fmt.Sprintf("==== starting version %s - user %s - %s/%s ====",
		appVersion, userName, runtime.GOOS, runtime.GOARCH))
	if others := len(activeBackupRuns()) - 1; others > 0 {
		runLog(opts.Ctx, fmt.Sprintf("Running alongside %d other backup(s) in this process", others))
	}

	// Validate options. Log the resolved target (secret presence only, never the
	// secret) so a misconfiguration is diagnosable from the backup log alone — a
	// prod report showed only "Validating backup options" then nothing because an
	// empty multi-PBS resolution failed here silently.
	runLog(opts.Ctx, fmt.Sprintf("[DEBUG] Validating backup options: target=%q datastore=%q ns=%q authid=%q secret=%v dirs=%d backupID=%q",
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
		runLog(opts.Ctx, "[ERROR] "+errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	runLog(opts.Ctx, "[DEBUG] Options validated")

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
	// behaviour this project set out to replace. That splitting is now retired in favor of PBS's
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
			runLog(opts.Ctx, errMsg)
			// Get stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			runLog(opts.Ctx, fmt.Sprintf("Stack trace:\n%s", buf[:n]))
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

	// Generate backup ID from path if not specified
	runLog(opts.Ctx, "[DEBUG] Generating backup ID if needed")
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
	runLog(opts.Ctx, fmt.Sprintf("[DEBUG] BackupID set to: %s", opts.BackupID))

	// Default to "host" type for directory backups
	if opts.BackupType == "" {
		opts.BackupType = "host"
	}

	// Progress callback wrapper
	progress := func(pct float64, msg string) {
		runLog(opts.Ctx, fmt.Sprintf("Backup progress: %.1f%% - %s", pct*100, msg))
		if opts.OnProgress != nil {
			opts.OnProgress(pct, msg)
		}
	}

	// Folders on network shares: connect with the servers' saved logins
	// before anything reads them (share_logins.go; the service only).
	releaseShares, shareErrs := connectShares(opts.ShareLogins, opts.BackupObjects)
	defer releaseShares()
	for _, err := range shareErrs {
		runLog(opts.Ctx, fmt.Sprintf("Network share: %v", err))
	}

	// Check if all backup directories exist
	runLog(opts.Ctx, fmt.Sprintf("[DEBUG] Checking %d backup directories exist", len(opts.BackupObjects)))
	for idx, dir := range opts.BackupObjects {
		runLog(opts.Ctx, fmt.Sprintf("[DEBUG] Checking directory %d/%d: %s", idx+1, len(opts.BackupObjects), dir))
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			errMsg := fmt.Sprintf("Backup directory does not exist: %s", dir)
			// A Backup Set saved before mapped drives were kept as the
			// share's path (resolveMappedDrives) names the user's letter,
			// which the service does not have.
			if serviceBuild && len(dir) >= 2 && dir[1] == ':' && !driveExists(dir[:2]) {
				errMsg += fmt.Sprintf(" (the background service has no drive %s: if it is a drive mapped to a network share, edit the Backup Set and save it again, which keeps the share's own path)", strings.ToUpper(dir[:2]))
			}
			errKey, errParams := MsgBackupDirNotExist, msgParams{"dir": dir}
			runLog(opts.Ctx, errMsg)
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
	runLog(opts.Ctx, "[DEBUG] All directories checked, calling progress(0.05)")

	progress(0, "Connecting to PBS...")
	runLog(opts.Ctx, "[DEBUG] After progress(0.05), before connection log")

	// Debug: log connection parameters with sanitized credentials
	runLog(opts.Ctx, fmt.Sprintf("PBS Connection: URL=%s, AuthID=%s, Secret=%s, Datastore=%s, BackupID=%s",
		security.SanitizeURL(opts.BaseURL),
		opts.AuthID,
		security.SanitizeSecret(opts.Secret),
		opts.Datastore,
		opts.BackupID))

	runLog(opts.Ctx, "[DEBUG] Creating PBS client struct")

	// Parse compression level (default to fastest if empty or invalid)
	compressionLevel := pbscommon.ParseCompressionLevel(opts.Compression)
	runLog(opts.Ctx, fmt.Sprintf("[DEBUG] Compression level: %s", compressionLevel))

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
		Crypt:            opts.Crypt,
		Manifest: pbscommon.BackupManifest{
			BackupID: opts.BackupID,
			Comment:  opts.Comment,
		},
	}

	runLog(opts.Ctx, "[DEBUG] PBS client created, starting single-session multi-archive backup")

	hostname, _ := os.Hostname()

	// Counters and per-directory result accumulators for the whole run. Chunk
	// counters (newchunk/reusechunk/failedchunk) intentionally live OUTSIDE the
	// attempt loop below and keep accumulating across a whole-job retry: chunks
	// uploaded in a failed attempt are real work PBS already has (content-
	// addressed dedup), not phantom double-counting — only the FINAL committed
	// attempt's directories ever contribute to jobProg.bytesDoneBase / the
	// reported Directories list. jobProg.bytesDoneBase doubles as both the
	// running total fed to BackupStatus.TotalBytes below AND the live
	// progress-bar baseline every directory's own ChunkState reads from (see
	// jobProgress's doc comment) — same one accumulator, same lifetime the
	// old totalSize this replaces had.
	var newchunk atomic.Uint64
	var reusechunk atomic.Uint64
	var failedchunk atomic.Uint64
	jobProg := &jobProgress{}
	jobProg.phase.Store("connecting")
	// emitStats sends the backup card's live figures. It runs on a timer
	// rather than from the chunk path so the card keeps a steady cadence
	// (the GUI measures speeds over the last few seconds of these samples)
	// whether data is flowing fast, slowly, or not at all.
	emitStats := func() {
		if opts.OnStats == nil {
			return
		}
		bytesTotal, filesTotal := jobProg.totals()
		jobProg.lastPercentMu.Lock()
		pct := jobProg.lastPercent
		jobProg.lastPercentMu.Unlock()
		opts.OnStats(&BackupProgressStats{
			Percent:       pct,
			BytesDone:     jobProg.bytesDoneBase.Load() + jobProg.curPos.Load(),
			BytesTotal:    bytesTotal,
			NewChunks:     newchunk.Load(),
			ReusedChunks:  reusechunk.Load(),
			FailedChunks:  failedchunk.Load(),
			CurrentDir:    loadString(&jobProg.currentDir),
			FilesDone:     jobProg.filesDone.Load(),
			FilesTotal:    filesTotal,
			UploadedBytes: jobProg.uploadedBytes.Load(),
			SentBytes:     client.SentBytes(),
			Phase:         loadString(&jobProg.phase),
			ReadReason:    loadString(&jobProg.readReason),
			ChangeMode:    opts.ChangeDetectionMode,
		})
	}
	if opts.OnFile != nil || opts.OnStats != nil {
		tickerDone := make(chan struct{})
		tickerExited := make(chan struct{})
		var stopOnce sync.Once
		stopTicker := func() {
			stopOnce.Do(func() { close(tickerDone) })
			<-tickerExited
		}
		defer stopTicker()
		// The ticker must be silent before the run is reported complete: a
		// tick after it ("finishing", this run's start time) landed on the
		// card of the next queued backup, which then showed "Finishing" from
		// its first second (seen 2026-10-08, Data set then Full Machine).
		if onComplete := opts.OnComplete; onComplete != nil {
			opts.OnComplete = func(success bool, message string, key MessageKey, params msgParams) {
				stopTicker()
				onComplete(success, message, key, params)
			}
		}
		go func() {
			defer close(tickerExited)
			t := time.NewTicker(cardRefresh)
			defer t.Stop()
			last := ""
			for {
				select {
				case <-tickerDone:
					return
				case <-t.C:
					if opts.OnFile != nil {
						if p, _ := jobProg.currentFile.Load().(string); p != "" && p != last {
							last = p
							opts.OnFile(p)
						}
					}
					emitStats()
				}
			}
		}()
	}
	var dirErrors []string
	var dirResults []DirResult
	var allReadErrors, allSkipped, allExcluded []string
	successfulDirs := 0

	// failRun builds the failed BackupStatus, fires the callbacks and returns the
	// error — shared by every "whole run failed" exit below (all directories
	// failed, session lost and retries exhausted, manifest upload failed,
	// finalize failed) so they all report identically.
	failRun := func(errMsg string, key MessageKey, params msgParams) error {
		runLog(opts.Ctx, errMsg)
		status := &BackupStatus{
			Outcome:          OutcomeFailed,
			BackupID:         opts.BackupID,
			BackupTime:       client.Manifest.BackupTime,
			DurationSec:      time.Since(startTime).Seconds(),
			TotalBytes:       jobProg.bytesDoneBase.Load(),
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
	//   - A SESSION-FATAL error (retry.SessionLost — the underlying HTTP/2
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
	const maxJobAttempts = retry.FolderRerunAttempts
	const sessionLostRetryWait = retry.FolderRerunWait

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

		// Launch every directory's background size scan CONCURRENTLY, right
		// now, rather than each directory only starting its own scan once its
		// own turn in the loop below arrives (that's what backupReal used to
		// do). Found live 2026-09-26: with the sequential-per-directory
		// timing, jobProg.sizeEstimate only ever reflected whichever
		// directories had ALREADY been reached — a job backing up a large
		// network share first and a small local folder second showed the
		// network share's own size as "total" for its entire run (correct,
		// since the local folder's scan genuinely hadn't started yet, so its
		// size genuinely wasn't known), only growing to the true combined
		// total once the second directory's turn began. It looked like
		// "two passes ... rather than the combined progress" even though the
		// numbers themselves were accurate at each moment. Scanning every
		// directory up front instead means the combined total is normally
		// already fully known well before the first directory even finishes,
		// so the percentage bar climbs smoothly across the whole job from
		// the start.
		jobProg.scansWanted.Add(int32(len(opts.BackupObjects)))
		for _, dir := range opts.BackupObjects {
			dir := dir
			go func() {
				runLog(opts.Ctx, fmt.Sprintf("Starting background size calculation for: %s", dir))
				// The run's context: the scan ends with the run (or its Stop).
				ctx, cancel := context.WithTimeout(opts.Ctx, analysisSizeBudget)
				defer cancel()
				size, files, err := calculateDirSizeFilesCtx(ctx, dir)
				if err != nil {
					runLog(opts.Ctx, fmt.Sprintf("WARNING: Size calculation had errors for %s: %v", dir, err))
				}
				jobProg.sizeEstimate.Add(size)
				jobProg.filesEstimate.Add(files)
				jobProg.scansDone.Add(1)
				runLog(opts.Ctx, fmt.Sprintf("Size calculated for %s: %d MB in %d files", dir, size/(1024*1024), files))
			}()
		}

		runLog(opts.Ctx, fmt.Sprintf("[Session] Connecting (attempt %d/%d) for %d selected folder(s)",
			jobAttempt, maxJobAttempts, len(opts.BackupObjects)))
		client.Connect(false, "host")

		// ONE catalog dynamic index for the whole attempt, named exactly
		// "catalog.pcat1.didx" (PBS's web-UI browse feature hardcodes that
		// literal name server-side — see sharedCatalogCoordinator's doc
		// comment above backupDirectory for the full story). Every directory
		// below writes its catalog data into this same index; it is
		// finalized once, after the loop, not per directory.
		catalogChunk := ChunkState{logCtx: opts.Ctx}
		catalogChunk.Init(client, &newchunk, &reusechunk, &failedchunk, haxmap.New[string, bool](), nil, jobProg, nil, "")
		var catalogIndexErr error
		catalogChunk.wrid, catalogIndexErr = client.CreateDynamicIndex("catalog.pcat1.didx")
		if catalogIndexErr != nil {
			client.Close()
			if jobAttempt >= maxJobAttempts {
				return failRun(fmt.Sprintf("failed to create shared catalog index: %v", catalogIndexErr),
					MsgCatalogCreateFailed, msgParams{"error": catalogIndexErr.Error()})
			}
			runLog(opts.Ctx, fmt.Sprintf("Failed to create shared catalog index (attempt %d/%d): %v — retrying whole job",
				jobAttempt, maxJobAttempts, catalogIndexErr))
			continue
		}
		sharedCatalog := &sharedCatalogCoordinator{chunk: &catalogChunk}

		switch {
		case opts.FullReadRequested:
			jobProg.readReason.Store("full_read_requested")
		case opts.ScheduledFullRead:
			jobProg.readReason.Store("full_read")
		case opts.ChangeDetectionMode == pbscommon.ChangeDetectionData:
			jobProg.readReason.Store("data")
		default:
			jobProg.readReason.Store("")
		}
		jobProg.phase.Store("data")

		var sessionFatal error
		for idx, dir := range opts.BackupObjects {
			if opts.Ctx != nil && opts.Ctx.Err() != nil {
				runLog(opts.Ctx, fmt.Sprintf("Cancellation requested — stopping before backup of %s", dir))
				client.Close()
				if opts.OnComplete != nil {
					opts.OnComplete(false, "Backup cancelled by user", MsgBackupCancelled, nil)
				}
				return fmt.Errorf("backup cancelled by user")
			}
			runLog(opts.Ctx, fmt.Sprintf("Starting archive %d/%d: %s", idx+1, len(opts.BackupObjects), dir))

			archiveBase := archiveBaseName(dir, usedArchiveNames)
			dirBytes, aclMeta, dirSkipped, dirExcluded, dirReadErrors, err :=
				backupDirectory(opts.Ctx, client, &newchunk, &reusechunk, &failedchunk, dir, archiveBase, opts.UseVSS, progress, jobProg, opts.OnStats, opts.ExcludeList, sharedCatalog, opts.PrefetchWorkers, opts.ChangeDetectionMode)

			allSkipped = append(allSkipped, dirSkipped...)
			allExcluded = append(allExcluded, dirExcluded...)
			allReadErrors = append(allReadErrors, dirReadErrors...)

			if err != nil {
				// Cancellation mid-directory (PXARArchive.WriteDir's own
				// per-file check, pbscommon/pxar.go) — must abort the WHOLE
				// job here, not just this one directory: falling through to
				// the generic "directory-local error, keep going" branch
				// below would silently swallow Stop and move on to the next
				// directory instead, exactly the bug this fixes.
				if errors.Is(err, context.Canceled) || errors.Is(err, pbscommon.ErrWriteCancelled) {
					runLog(opts.Ctx, fmt.Sprintf("Cancellation requested — stopping mid-backup of %s", dir))
					client.Close()
					if opts.OnComplete != nil {
						opts.OnComplete(false, "Backup cancelled by user", MsgBackupCancelled, nil)
					}
					return fmt.Errorf("backup cancelled by user")
				}
				if retry.SessionLost(err) {
					runLog(opts.Ctx, fmt.Sprintf("Session lost while backing up %s: %v", dir, err))
					sessionFatal = err
					dirErrors = append(dirErrors, fmt.Sprintf("backup of %s failed: %v (session lost)", dir, err))
					dirResults = append(dirResults, DirResult{Path: dir, OK: false, Error: err.Error()})
					break // shared connection is dead — abort the rest of this attempt
				}
				// Say why when the share's saved login did not connect.
				if serr := shareFailure(shareErrs, dir); serr != nil {
					err = fmt.Errorf("%w (%v)", err, serr)
				}
				errMsg := fmt.Sprintf("Backup failed for %s: %v", dir, err)
				runLog(opts.Ctx, errMsg)
				dirErrors = append(dirErrors, errMsg)
				dirResults = append(dirResults, DirResult{Path: dir, OK: false, Error: err.Error()})
				continue // directory-local error — best-effort, keep going
			}

			if aclMeta != nil {
				combinedACLs.Archives[archiveBase] = aclMeta
			}
			jobProg.bytesDoneBase.Add(dirBytes)
			jobProg.curPos.Store(0)
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
			runLog(opts.Ctx, fmt.Sprintf("Waiting %s for PBS to release the backup group lock before retrying the whole job...",
				sessionLostRetryWait))
			waitUntil := time.Now().Add(sessionLostRetryWait)
			for {
				if opts.Ctx != nil && opts.Ctx.Err() != nil {
					runLog(opts.Ctx, "Cancellation requested during session-lost wait — aborting")
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
			runLog(opts.Ctx, "Wait complete, retrying the whole job with a fresh connection")
			continue
		}

		// Session stayed healthy for the whole attempt (whether every directory
		// succeeded or this is a best-effort partial run) — finalize it once,
		// covering every archive written above, and we're done: no whole-job
		// retry for directory-local errors (matches the old design, which also
		// never retried a non-fatal per-directory error).

		jobProg.phase.Store("finishing")
		jobProg.lastPercentMu.Lock()
		if jobProg.lastPercent < 0.99 && jobProg.lastPercent > 0 {
			jobProg.lastPercent = 0.99
		}
		finishPct := jobProg.lastPercent
		jobProg.lastPercentMu.Unlock()
		progress(finishPct, "Finishing: closing the catalog and uploading the manifest...")

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
			aclBytes, dropped, aclErr := combinedACLs.SerializeCapped(pbscommon.MaxFileMetaBlobBytes)
			if dropped > 0 {
				runLog(opts.Ctx, fmt.Sprintf("WARNING: file metadata too large for one PBS blob: the content of %d alternate data streams was left out (permissions and attributes kept)", dropped))
			}
			if aclErr != nil {
				runLog(opts.Ctx, fmt.Sprintf("WARNING: failed to serialize combined NTFS metadata: %v", aclErr))
			} else if upErr := client.UploadBlob(BackupAclsFilename, aclBytes); upErr != nil {
				runLog(opts.Ctx, fmt.Sprintf("WARNING: failed to upload combined NTFS metadata blob: %v", upErr))
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
			runLog(opts.Ctx, fmt.Sprintf("WARNING: failed to serialize status sidecar: %v", sErr))
		} else if upErr := client.UploadBlob(BackupStatusFilename, sidecarBytes); upErr != nil {
			runLog(opts.Ctx, fmt.Sprintf("WARNING: failed to upload status sidecar: %v", upErr))
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

		runLog(opts.Ctx, fmt.Sprintf("Session finalized: %d/%d directories committed", successfulDirs, len(opts.BackupObjects)))

		// PBS's own "Comment" column reads from the dedicated notes API, not
		// from anything in the manifest we just uploaded (verified live
		// against the test PBS — index.json.blob's Comment field is never
		// surfaced by ListSnapshots or the GUI). Best-effort: the backup
		// already succeeded, so a failure here is logged, not fatal.
		if opts.Comment != "" {
			if commentErr := client.SetComment(opts.Comment); commentErr != nil {
				runLog(opts.Ctx, fmt.Sprintf("WARNING: failed to set PBS snapshot comment: %v", commentErr))
			}
		}
		break
	}

	// Calculate backup duration and size
	duration := time.Since(startTime)
	// Decimal MB, as the "MB" in these messages says (it was MiB: a 30.2 GB
	// set read "28832.6 MB"). The figure stays in MB so the translated
	// messages, and Reports entries saved before, keep their unit.
	totalSizeMB := float64(jobProg.bytesDoneBase.Load()) / 1e6

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

	// The raw byte counts, so Reports and the Message Log can show the size
	// and the data uploaded as fields of their own, in both units.
	completionParams["bytes"] = jobProg.bytesDoneBase.Load()
	completionParams["uploadedBytes"] = jobProg.uploadedBytes.Load()
	completionParams["sentBytes"] = client.SentBytes()

	// skipped rides along in every completion key's params (0 when there were
	// none) so the frontend can splice in the "N files skipped" clause itself
	// — see msgcodes.go's MsgSkippedFilesNote doc comment — without losing
	// that detail when a localized message_key is rendered instead of Message.
	completionParams["skipped"] = len(allSkipped)
	// The part of those it could not read (the rest are links, sockets,
	// pipes and system files it leaves out on purpose), so only these get a
	// warning sign.
	completionParams["unreadable"] = len(allReadErrors)
	// Same for the change-detection outcome, so the history entry written from
	// OnComplete can show it (Reports).
	changeMode := opts.ChangeDetectionMode
	if pbscommon.SplitArchiveMode(changeMode) {
		completionParams["changeDetection"] = changeMode
		completionParams["reusedFiles"] = jobProg.reusedFiles.Load()
		completionParams["reusedBytes"] = jobProg.reusedBytes.Load()
		completionParams["readEveryFile"] = !jobProg.anyReuse.Load()
		if changeMode == pbscommon.ChangeDetectionMetadata {
			runLog(opts.Ctx, fmt.Sprintf("Metadata change detection: %d unchanged files (%s) reused without reading in this job",
				jobProg.reusedFiles.Load(), formatByteSize(jobProg.reusedBytes.Load())))
		}
	}

	if len(allSkipped) > 0 {
		completionMsg += skippedSummary(len(allSkipped), len(allReadErrors))
		runLog(opts.Ctx, fmt.Sprintf("=== SKIPPED FILES/DIRECTORIES (%d) ===", len(allSkipped)))

		// Log first 50 skipped files in detail
		maxLog := 50
		if len(allSkipped) < maxLog {
			maxLog = len(allSkipped)
		}
		for i := 0; i < maxLog; i++ {
			runLog(opts.Ctx, fmt.Sprintf("  [%d] %s", i+1, allSkipped[i]))
		}
		if len(allSkipped) > 50 {
			runLog(opts.Ctx, fmt.Sprintf("  ... and %d more (see full list in GUI)", len(allSkipped)-50))
		}
		runLog(opts.Ctx, "=== END SKIPPED FILES ===")
	}

	runLog(opts.Ctx, completionMsg)

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
		TotalBytes:       jobProg.bytesDoneBase.Load(),
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
	if pbscommon.SplitArchiveMode(changeMode) {
		status.ChangeDetection = changeMode
		status.ReusedFiles = jobProg.reusedFiles.Load()
		status.ReusedFileBytes = jobProg.reusedBytes.Load()
		status.ReadEveryFile = !jobProg.anyReuse.Load()
	}

	// One machine-greppable result line for support (pairs with the start-of-run
	// target log): outcome label + the structured counters behind completionMsg.
	runLog(opts.Ctx, fmt.Sprintf("[RESULT] outcome=%s dirs_ok=%d/%d new=%d reused=%d failed=%d read_errors=%d excluded=%d bytes=%d new_data=%d transferred=%d duration=%s",
		status.Outcome, successfulDirs, len(opts.BackupObjects), status.NewChunks, status.ReusedChunks,
		status.FailedChunks, len(status.SkippedReadError), len(status.ExcludedByPolicy), status.TotalBytes,
		jobProg.uploadedBytes.Load(), client.SentBytes(), duration))

	if status.Success() && opts.OnSnapshot != nil {
		opts.OnSnapshot(&clientNoter{client: client})
	}

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

// clientNoter adds a note to the PBS comment of the snapshot a folder backup
// made, after the comment it was given (as machinebackuplib.BackupResult does).
type clientNoter struct{ client *pbscommon.PBSClient }

func (c *clientNoter) AddNote(note string) error {
	if comment := c.client.Manifest.Comment; comment != "" {
		note = comment + " | " + note
	}
	return c.client.SetComment(note)
}

// runMachineBackupInline handles machine backup using machinebackuplib
func runMachineBackupInline(opts BackupOptions) error {
	startTime := time.Now()

	// Create machine backup config from opts
	cfg := &machinebackuplib.Config{
		BaseURL:          opts.BaseURL,
		CertFingerprint:  opts.CertFingerprint,
		AuthID:           opts.AuthID,
		Secret:           opts.Secret,
		Ticket:           opts.Ticket,
		CSRFToken:        opts.CSRFToken,
		Datastore:        opts.Datastore,
		Namespace:        opts.Namespace,
		BackupID:         opts.BackupID,
		BackupType:       opts.BackupType,
		BackupDevices:    opts.BackupObjects,
		Comment:          opts.Comment,
		Crypt:            opts.Crypt,
		CompressionLevel: opts.Compression,
		ReadFreeSpace:    opts.ReadFreeSpace,
		LiveRead:         opts.LiveRead,
		SkipPartitions:   opts.SkipPartitions,
		Progress:         &machinebackuplib.JobProgress{},
	}
	if opts.ReadFreeSpace {
		runLog(opts.Ctx, "Forensic copy: free space and the paging/hibernation files are read too")
	}
	if opts.LiveRead && runtime.GOOS == "linux" {
		runLog(opts.Ctx, "WARNING: Use snapshots is off for this Backup Set: mounted disks are read live and the image may be inconsistent")
	}
	jp := cfg.Progress

	// counts is the "New / Reused / blank / uploaded" part of the log lines.
	counts := func() string {
		s := fmt.Sprintf("new %d, reused %d (%d blank), new/changed data %s, transferred %s, free space not read %s",
			jp.NewChunks.Load(), jp.ReusedChunks.Load(), jp.BlankChunks.Load(), formatByteSize(jp.UploadedBytes.Load()),
			formatByteSize(jp.SentBytes()), formatByteSize(jp.FreeSkipped.Load()))
		if n := jp.LeftOut.Load(); n > 0 {
			s += ", partitions left out " + formatByteSize(n)
		}
		return s
	}

	// Progress callback wrapper. Returning true (user pressed Stop, which
	// cancels opts.Ctx) makes the backup abort without committing the index.
	// The disk reader calls it after every block, so it is also where a
	// paused machine backup waits. The log gets a block line only once per
	// whole percent (it used to get all ~30,000 of a 128 GB disk); other
	// messages (snapshot, disk complete) are always logged.
	// Block lines also reach OnProgress (a GUI event and a debug log line
	// each) at most four times a second; the card's figures come from
	// emitStats below.
	lastLoggedPct := -1
	var lastBlockEmit time.Time
	progress := func(pct float64, msg string) bool {
		block := strings.Contains(msg, ": Block ")
		if !block {
			runLog(opts.Ctx, fmt.Sprintf("Backup progress: %.1f%% - %s", pct*100, msg))
		} else if p := int(pct * 100); p != lastLoggedPct {
			lastLoggedPct = p
			runLog(opts.Ctx, fmt.Sprintf("Backup progress: %d%% - %s - %s", p, msg, counts()))
		}
		if opts.OnProgress != nil && (!block || time.Since(lastBlockEmit) >= 250*time.Millisecond) {
			if block {
				lastBlockEmit = time.Now()
			}
			// 100% would take the progress card down while the image is
			// still being closed; the completion event finishes it.
			opts.OnProgress(min(pct, 0.99), msg)
		}
		if runPause(opts.Ctx).Wait(opts.Ctx) != nil {
			return true
		}
		return opts.Ctx != nil && opts.Ctx.Err() != nil
	}

	// While waiting to start again after a lost connection: when, and which
	// attempt comes next (see the retry loop below).
	var retryAt atomic.Int64
	var retryAttempt atomic.Int32

	// emitStats sends the card's live figures on a timer, as folder backups
	// do, so speeds are measured over a steady cadence.
	emitStats := func() {
		if opts.OnStats == nil {
			return
		}
		w := jp.Where()
		if at := retryAt.Load(); at > 0 {
			w.Phase = "retry_wait"
		}
		total := jp.BytesTotal.Load()
		done := jp.BytesDone.Load()
		// The percentage counts what is read: free space and partitions
		// left out go in as zeros at once (JobProgress.ReadProgress).
		readDone, readTotal := jp.ReadProgress()
		pct := 0.0
		if readTotal > 0 {
			pct = float64(readDone) / float64(readTotal)
		}
		opts.OnStats(&BackupProgressStats{
			Percent:       pct,
			BytesDone:     done,
			BytesTotal:    total,
			NewChunks:     jp.NewChunks.Load(),
			ReusedChunks:  jp.ReusedChunks.Load(),
			UploadedBytes: jp.UploadedBytes.Load(),
			SentBytes:     jp.SentBytes(),
			Phase:         w.Phase,
			Kind:          "machine",
			BlankChunks:   jp.BlankChunks.Load(),
			Disk:          w.Disk,
			DiskModel:     w.DiskModel,
			DiskNo:        w.DiskNo,
			DiskCount:     w.DiskCount,
			DiskBytes:     w.DiskSize,
			DiskDone:      w.DiskPos,
			Partition:     w.Partition,
			Unreadable:    jp.UnreadableBytes.Load(),
			ReadDone:      readDone,
			ReadTotal:     readTotal,
			RetryAt:       retryAt.Load(),
			Attempt:       int(retryAttempt.Load()),
			Attempts:      len(retry.ReconnectWaits) + 1,
		})
	}
	tickerDone := make(chan struct{})
	tickerExited := make(chan struct{})
	go func() {
		defer close(tickerExited)
		t := time.NewTicker(cardRefresh)
		defer t.Stop()
		for {
			select {
			case <-tickerDone:
				return
			case <-t.C:
				emitStats()
			}
		}
	}()
	// The ticker must be silent before the run is reported complete, or a
	// late tick lands on the card of the next queued backup.
	stopTicker := func() {
		select {
		case <-tickerDone:
		default:
			close(tickerDone)
		}
		<-tickerExited
	}
	defer stopTicker()

	// Perform machine backup. A lost connection (a mobile link dropping, PBS
	// restarting) ends the PBS session, and with it every chunk sent in it, so
	// the run starts again with a new session and new snapshots after a wait
	// that grows each time, as a folder backup does. The wait also lets PBS
	// release the backup group's lock; an attempt made while it still holds it
	// fails at once and waits again.
	var result *machinebackuplib.BackupResult
	var err error
	for attempt := 1; ; attempt++ {
		result, err = machinebackuplib.Backup(cfg, progress)
		if err == nil || attempt > len(retry.ReconnectWaits) || !retry.Reconnectable(err) ||
			(opts.Ctx != nil && opts.Ctx.Err() != nil) {
			break
		}
		wait := retry.ReconnectWaits[attempt-1]
		at := time.Now().Add(wait)
		runLog(opts.Ctx, fmt.Sprintf("Connection to the server lost on attempt %d of %d (%s): %v. Starting again at %s with a new session; chunks sent in the lost one are sent again.",
			attempt, len(retry.ReconnectWaits)+1, counts(), err, at.Format("15:04:05")))
		jp.Reset()
		retryAt.Store(at.UnixMilli())
		retryAttempt.Store(int32(attempt + 1))
		if !waitOrCancel(opts.Ctx, wait) {
			retryAt.Store(0)
			break // Stop pressed: reported as cancelled below
		}
		retryAt.Store(0)
	}
	stopTicker()
	if err == nil && result != nil && opts.OnSnapshot != nil {
		opts.OnSnapshot(result)
	}
	runLog(opts.Ctx, fmt.Sprintf("[RESULT] machine backup: %s of %s read, %s", formatByteSize(jp.BytesDone.Load()), formatByteSize(jp.BytesTotal.Load()), counts()))
	if err != nil && opts.Ctx != nil && opts.Ctx.Err() != nil {
		// Stop is the user's choice, not a failure: reported as a folder
		// backup's Stop is.
		runLog(opts.Ctx, "Cancellation requested: machine backup stopped, no snapshot kept")
		if opts.OnComplete != nil {
			opts.OnComplete(false, "Backup cancelled by user", MsgBackupCancelled, nil)
		}
		return fmt.Errorf("backup cancelled by user")
	}
	if err != nil {
		// Handle error case
		errMsg := fmt.Sprintf("Machine backup failed: %v", err)
		errParams := msgParams{"error": err.Error()}
		runLog(opts.Ctx, errMsg)

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
	newC, reusedC, blankC := jp.NewChunks.Load(), jp.ReusedChunks.Load(), jp.BlankChunks.Load()
	read, uploaded := formatByteSize(jp.BytesTotal.Load()), formatByteSize(jp.UploadedBytes.Load())
	completionMsg := fmt.Sprintf("Machine backup completed in %s: %s read (%d new, %d reused chunks, %d of them blank space; %s uploaded)",
		formatDuration(duration), read, newC, reusedC, blankC, uploaded)
	completionParams := msgParams{"duration": formatDuration(duration), "read": read, "new": newC, "reused": reusedC, "blank": blankC, "uploaded": uploaded,
		// The raw byte counts behind read and uploaded, for Reports and the Message Log.
		"readBytes": jp.BytesTotal.Load(), "uploadedBytes": jp.UploadedBytes.Load(), "sentBytes": jp.SentBytes()}
	completionKey, outcome := MsgMachineBackupCompletedStats, OutcomeVerifiedSuccess

	// Sectors the disk could not read are zeros in the image: the backup is
	// usable but not whole, so say so and where, as a partial folder backup does.
	if badBytes := jp.UnreadableBytes.Load(); badBytes > 0 {
		var where []string
		for _, r := range jp.Unreadable() {
			where = append(where, fmt.Sprintf("%s at %s (%s)", r.Disk, formatByteSize(r.Offset), formatByteSize(r.Length)))
		}
		for _, w := range where {
			runLog(opts.Ctx, "Unreadable, zeros in the image: "+w)
		}
		completionMsg = fmt.Sprintf("⚠️ Machine backup completed in %s, but %s of the disk could not be read and is zeros in the image (%s). %s read (%d new, %d reused chunks; %s uploaded)",
			formatDuration(duration), formatByteSize(badBytes), strings.Join(where, ", "), read, newC, reusedC, uploaded)
		completionKey, outcome = MsgMachineBackupUnreadable, OutcomePartial
		completionParams["bad"] = formatByteSize(badBytes)
		completionParams["where"] = strings.Join(where, ", ")
	}

	if opts.OnComplete != nil {
		opts.OnComplete(true, completionMsg, completionKey, completionParams)
	}
	if opts.OnResult != nil {
		opts.OnResult(&BackupStatus{
			Outcome:       outcome,
			BackupID:      opts.BackupID,
			DurationSec:   duration.Seconds(),
			NewChunks:     newC,
			ReusedChunks:  reusedC,
			TotalBytes:    jp.BytesTotal.Load(),
			Message:       completionMsg,
			MessageKey:    completionKey,
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
func backupDirectory(ctx context.Context, client *pbscommon.PBSClient, newchunk, reusechunk, failedchunk *atomic.Uint64, backupdir string, archiveBase string, usevss bool, progress func(float64, string), jobProg *jobProgress, onStats func(*BackupProgressStats), excludeList []string, catalog *sharedCatalogCoordinator, prefetchWorkers int, changeMode string) (uint64, *BackupFileMeta, []string, []string, []string, error) {
	runLog(ctx, fmt.Sprintf("Starting backup of %s", backupdir))
	originalPath := backupdir

	// A network/remote path (a UNC share, or a drive letter mapped to one)
	// can't be snapshotted — VSS on Windows and the elastio-snap/dattobd
	// block-device snapshot on Linux both operate on a real local volume,
	// which a network share isn't. Found live 2026-09-26: a Backup Set
	// pointed at \\FILESERVER\backup with VSS on failed outright
	// ("VSS_VOLUME_SUPPORT - snapshots are not supported for drive ...").
	// Checked per-directory (not once for the whole job) so a job mixing
	// local and network paths still gets a real snapshot for the local
	// ones — only the network path(s) fall back to a live, unsnapshotted
	// read, same as if VSS had never been requested for that one directory.
	if usevss && !snapshot.PathSupportsSnapshot(backupdir) {
		runLog(ctx, fmt.Sprintf("Skipping snapshot for %s: network/remote paths don't support volume snapshots — backing up live instead", backupdir))
		usevss = false
	}

	if usevss {
		// VSS setup (checking writer status, creating the shadow copy) can take
		// a few seconds with nothing else to report — without this the UI is
		// left showing the previous, now-stale "Connecting to PBS..." message
		// for the whole pause. Real chunk-level progress from backupReal below
		// naturally overwrites this once the snapshot is ready. It keeps the
		// job's percentage: it used to send 0, so the bar fell back to 0% at
		// each folder after the first (seen live 2026-11-05, 90% to 0% for the
		// 14 s of the third folder's snapshot, then on to 92.7%).
		if progress != nil {
			progress(jobProg.percent(), "Creating VSS snapshot...")
		}

		var bytesArchived uint64
		var aclMeta *BackupFileMeta
		var skipped, excluded, readErrs []string
		err := snapshot.CreateVSSSnapshotCtx(ctx, []string{backupdir}, true, func(snaps map[string]snapshot.SnapShot) error {
			for _, snap := range snaps {
				backupdir = snap.FullPath
				break
			}
			var e error
			bytesArchived, aclMeta, skipped, excluded, readErrs, e = backupReal(ctx, client, newchunk, reusechunk, failedchunk, backupdir, originalPath, archiveBase, usevss, progress, jobProg, onStats, excludeList, catalog, prefetchWorkers, changeMode)
			return e
		})
		return bytesArchived, aclMeta, skipped, excluded, readErrs, err
	}

	return backupReal(ctx, client, newchunk, reusechunk, failedchunk, backupdir, originalPath, archiveBase, usevss, progress, jobProg, onStats, excludeList, catalog, prefetchWorkers, changeMode)
}

// reuseStatsHook, when set (tests only), receives each archive's metadata
// change detection statistics.
var reuseStatsHook func(archiveName string, st pbscommon.ReuseStats)

// openableDir reports why dir cannot be opened and listed, or nil.
func openableDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Readdirnames(1); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func backupReal(ctx context.Context, client *pbscommon.PBSClient, newchunk, reusechunk, failedchunk *atomic.Uint64, backupdir string, originalPath string, archiveBase string, vssUsed bool, progress func(float64, string), jobProg *jobProgress, onStats func(*BackupProgressStats), excludeList []string, catalog *sharedCatalogCoordinator, prefetchWorkers int, changeMode string) (returnBytes uint64, returnMeta *BackupFileMeta, returnSkipped, returnExcluded, returnReadErrors []string, returnErr error) {
	// Panic recovery - critical to prevent silent crashes during backup
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("CRITICAL: Backup panic occurred: %v", r)
			runLog(ctx, errMsg)
			// Get stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			runLog(ctx, fmt.Sprintf("Stack trace:\n%s", buf[:n]))
			returnErr = fmt.Errorf("backup panic: %v", r)
		}
	}()

	// The folder must open before any of its archives does. PBS finishes no
	// snapshot with an index left open (and the manifest lists it with no
	// checksum), so a folder that failed after its index was created took the
	// whole snapshot down with it, the other folders' archives too (found
	// 2026-11-06: a share the service may not open ended in "Invalid string
	// length" on the manifest). Failing here leaves the other folders to be
	// kept as a partial backup.
	if err := openableDir(backupdir); err != nil {
		return 0, nil, nil, nil, nil, fmt.Errorf("cannot open %s: %w", originalPath, err)
	}

	// NOTE: client.Connect() is called ONCE by the caller for the whole
	// multi-archive session (runBackupInlineInternal) — not per directory here.
	knownChunks := haxmap.New[string, bool]()

	// This directory's own contribution to jobProg.sizeEstimate was already
	// launched, alongside every OTHER directory in this job, right when the
	// attempt started (see the comment there for why) — nothing to scan here
	// anymore.

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
	// Split modes write the archive as a metadata stream (named here, and
	// recorded in the catalog under this name) plus a payload stream.
	split := pbscommon.SplitArchiveMode(changeMode)
	archiveName := archiveBase + ".pxar.didx"
	payloadName := ""
	if split {
		archiveName = archiveBase + ".mpxar.didx"
		payloadName = archiveBase + ".ppxar.didx"
		runLog(ctx, fmt.Sprintf("Change detection: %s mode, split archive %s + %s", changeMode, archiveName, payloadName))
	}

	jobProg.currentDir.Store(originalPath)
	archive := &pbscommon.PXARArchive{}
	archive.ArchiveName = archiveName
	archive.Split = split
	archive.ExcludeList = excludeList
	archive.Ctx = ctx
	archive.Pause = runPause(ctx)
	archive.ExcludeRoot = originalPath
	archive.OnFile = func(p string) {
		if vssUsed && strings.HasPrefix(p, backupdir) {
			p = originalPath + p[len(backupdir):]
		}
		jobProg.currentFile.Store(p)
		jobProg.filesDone.Add(1)
	}
	archive.PrefetchWorkers = prefetchWorkers // logical root for VSS-safe absolute-pattern matching
	// Source disk dropping out and coming back mid-backup (source_wait.go).
	archive.OnNotice = func(msg string) { runLog(ctx, msg) }

	// Inject backup metadata into the PXAR archive root
	hostname, _ := os.Hostname()
	metaJSON, err := GenerateBackupMeta(client.Manifest.BackupID, originalPath, hostname, vssUsed)
	if err != nil {
		runLog(ctx, fmt.Sprintf("WARNING: Failed to generate backup metadata: %v", err))
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
		runLog(ctx, fmt.Sprintf("No previous backup found (first backup?): %v", err))
		previousDidx = []byte{}
	} else {
		runLog(ctx, fmt.Sprintf("Downloaded previous DIDX: %d bytes", len(previousDidx)))
	}

	for _, shahash := range pbscommon.ParsePreviousDIDXChunkDigests(previousDidx) {
		knownChunks.Set(shahash, true)
	}

	runLog(ctx, fmt.Sprintf("Known chunks: %d", knownChunks.Len()))

	pxarChunk := ChunkState{cancelCtx: ctx}
	// payloadChunk carries the file contents of a split archive and drives
	// progress (its position is the archived data); pxarChunk is then the
	// small metadata stream and reports no progress of its own.
	var payloadChunk *ChunkState
	// A run that ends with an error (a Stop, a failed walk) never reaches
	// EOF: let the upload workers go, the caller drops the session.
	defer func() {
		if returnErr != nil {
			pxarChunk.closeUploads()
			if payloadChunk != nil {
				payloadChunk.closeUploads()
			}
		}
	}()
	if split {
		pxarChunk.Init(client, newchunk, reusechunk, failedchunk, knownChunks, nil, jobProg, nil, "")

		payloadKnown := haxmap.New[string, bool]()
		prevPayload, perr := client.DownloadPreviousToBytes(payloadName)
		if perr != nil {
			runLog(ctx, fmt.Sprintf("No previous payload index for %s: %v", payloadName, perr))
		}
		for _, shahash := range pbscommon.ParsePreviousDIDXChunkDigests(prevPayload) {
			payloadKnown.Set(shahash, true)
		}
		runLog(ctx, fmt.Sprintf("Known payload chunks: %d", payloadKnown.Len()))
		payloadChunk = &ChunkState{cancelCtx: ctx}
		payloadChunk.Init(client, newchunk, reusechunk, failedchunk, payloadKnown, progress, jobProg, onStats, backupdir)
		payloadChunk.wrid, err = client.CreateDynamicIndex(payloadName)
		if err != nil {
			return 0, nil, nil, nil, nil, err
		}
		archive.PayloadWriteCB = func(b []byte) error {
			return payloadChunk.HandleData(b, client)
		}
		archive.OnPayloadFileStart = func() error {
			return payloadChunk.SuggestBoundary(client)
		}
		if changeMode == pbscommon.ChangeDetectionMetadata {
			// Reuse unchanged files from the previous snapshot without
			// reading them. Anything unusable about the previous snapshot
			// just means every file is read, as in data mode.
			prev, prevTime, perr := client.LoadPreviousSplitArchive(archiveName, payloadName, prevPayload)
			if perr != nil {
				runLog(ctx, fmt.Sprintf("Metadata change detection: no usable previous snapshot for %s (%v), reading every file", archiveName, perr))
				if !jobProg.anyReuse.Load() {
					jobProg.readReason.Store("no_previous")
				}
			} else {
				runLog(ctx, fmt.Sprintf("Metadata change detection: comparing with snapshot %s (%d files)",
					time.Unix(prevTime, 0).UTC().Format(time.RFC3339), len(prev.Files)))
				jobProg.anyReuse.Store(true)
				jobProg.readReason.Store("")
				// The previous snapshot's totals: the card's total until
				// the background scans finish (jobProgress.totals).
				var prevBytes uint64
				for _, f := range prev.Files {
					prevBytes += f.Size
				}
				jobProg.prevBytes.Add(prevBytes)
				jobProg.prevFiles.Add(uint64(len(prev.Files)))
				archive.Reuse = pbscommon.NewPayloadReuse(prev,
					func() error { return payloadChunk.ForceBoundary(client) },
					payloadChunk.InjectKnownChunk)
			}
		}
	} else {
		pxarChunk.Init(client, newchunk, reusechunk, failedchunk, knownChunks, progress, jobProg, onStats, backupdir)
	}

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
	if archive.Reuse != nil {
		st := archive.Reuse.Stats
		if reuseStatsHook != nil {
			reuseStatsHook(archiveName, st)
		}
		jobProg.reusedFiles.Add(st.Files)
		jobProg.reusedBytes.Add(st.Bytes)
		runLog(ctx, fmt.Sprintf("Metadata change detection: %d unchanged files (%s) reused without reading, %d chunks reused, %s padding; %d unchanged files read anyway to limit padding",
			st.Files, formatByteSize(st.Bytes), st.Chunks, formatByteSize(st.Padding), st.OverPadding))
	}

	// Map VSS shadow-copy paths back to the original logical root so the status
	// lists are meaningful to the user (no-op for non-VSS backups, where
	// backupdir == originalPath). Reused for both the aggregate status and sidecar.
	logicalSkipped := toLogicalPaths(archive.SkippedFiles, backupdir, originalPath)
	logicalExcluded := toLogicalPaths(archive.ExcludedFiles, backupdir, originalPath)
	logicalReadErrors := toLogicalPaths(archive.ReadErrors, backupdir, originalPath)

	if len(logicalSkipped) > 0 {
		runLog(ctx, fmt.Sprintf("Backup completed with %d skipped files/directories", len(logicalSkipped)))
	}
	if len(logicalExcluded) > 0 {
		runLog(ctx, fmt.Sprintf("%d files/directories excluded by user policy", len(logicalExcluded)))
	}

	// Guard: if WriteDir produced 0 data, the backup dir was effectively empty or inaccessible
	if pxarChunk.pos == 0 && len(pxarChunk.currentChunk) == 0 {
		return 0, nil, logicalSkipped, logicalExcluded, logicalReadErrors, fmt.Errorf("backup produced 0 bytes for %s — directory may be empty, inaccessible, or all files were excluded", backupdir)
	}

	// A Stop that lands after the last archive write: stop before closing
	// the indexes, so the session is dropped with nothing committed (#117).
	if ctx != nil && ctx.Err() != nil {
		return 0, nil, logicalSkipped, logicalExcluded, logicalReadErrors, ctx.Err()
	}
	if err = pxarChunk.EOF(client); err != nil {
		return 0, nil, logicalSkipped, logicalExcluded, logicalReadErrors, err
	}
	archivedBytes := pxarChunk.pos
	if payloadChunk != nil {
		if err = payloadChunk.EOF(client); err != nil {
			return 0, nil, logicalSkipped, logicalExcluded, logicalReadErrors, err
		}
		archivedBytes += payloadChunk.pos
	}
	// The shared catalog index is NOT closed here — it stays open across every
	// directory in this attempt and is finalized once, by the caller, after
	// the whole per-directory loop (see runBackupInlineInternal).

	// Finalize (but do not upload) this directory's NTFS metadata; the caller
	// aggregates every directory's metadata into one combined blob for the
	// whole snapshot.
	var aclMeta *BackupFileMeta
	if m, aclErr := ntfsCollector.FinalizeRaw(); aclErr != nil {
		runLog(ctx, fmt.Sprintf("WARNING: failed to finalize NTFS metadata: %v", aclErr))
	} else if m != nil && len(m.Entries) > 0 {
		entries, uniqueSDDLs, metaErrs := ntfsCollector.Stats()
		runLog(ctx, fmt.Sprintf("NTFS metadata: %d entries, %d unique SDDLs, %d errors", entries, uniqueSDDLs, metaErrs))
		if streams, skipped, eas := ntfsCollector.StreamStats(); streams+skipped+eas > 0 {
			runLog(ctx, fmt.Sprintf("Alternate data streams: %d recorded, %d by name only (too large or unreadable); extended attributes: %d", streams, skipped, eas))
		}
		aclMeta = m
	}

	// pxarChunk.pos is the true archived byte count for this directory (more
	// accurate than the background size estimate, which only drives the %).
	return archivedBytes, aclMeta, logicalSkipped, logicalExcluded, logicalReadErrors, nil
}
