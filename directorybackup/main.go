package directorybackup

import (
	"clientcommon"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"pbscommon"
	"retry"
	"runtime"
	"snapshot"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alphadose/haxmap"
)

var defaultMailSubjectTemplate = "Backup {{.Status}}"
var defaultMailBodyTemplate = `{{if .Success}}Backup complete ({{.FromattedDuration}})
Chunks New {{.NewChunks}}, Reused {{.ReusedChunks}}.{{else if .Partial}}Backup completed WITH ERRORS ({{.FromattedDuration}})
{{.ReadErrorCount}} file(s) could not be read and were skipped; the snapshot is incomplete.
Chunks New {{.NewChunks}}, Reused {{.ReusedChunks}}.{{else}}Error occurred while working, backup may be not completed.
Last error is: {{.ErrorStr}}{{end}}`

type ChunkState struct {
	// entries is the index being built, in the stream's order: each chunk's
	// offset and size, recorded by the walk, and its digest, which a worker
	// fills in (or the walk, for a chunk reused without its data). The index
	// list and its checksum are made from it once every worker has finished
	// (closeIndex), so they come out as if the walk had hashed each chunk.
	entries       []*chunkEntry
	pos           uint64
	wrid          uint64
	chunkcount    uint64
	current_chunk []byte
	C             pbscommon.Chunker
	newchunk      *atomic.Uint64
	reusechunk    *atomic.Uint64
	knownChunks   *haxmap.Map[string, bool]

	// Each chunk goes to a pool of chunkUploadWorkers while the walk carries
	// on finding the next boundary: a worker hashes it, checks it against the
	// chunks PBS has, and compresses and sends it if new. Hashing used to be
	// done by the walk itself, and SHA-256 was the largest part of the time
	// one stream took (most of all where Go has no fast SHA-256: a CPU without
	// AVX2, or a VM given a generic CPU type), so the stream ran at one
	// core's hashing speed with the other cores idle. The first upload that
	// fails is kept in fatalErr, which stops the walk at the next chunk and
	// keeps the index from being closed. ctx ends the jobs still queued, and
	// the retries of the uploads under way, once there is a failure or the
	// backup is abandoned (stopUploads).
	uploadJobs chan uploadJob
	uploadWG   sync.WaitGroup
	closeOnce  sync.Once
	ctx        context.Context
	cancel     context.CancelFunc
	fatalMu    sync.Mutex
	fatalErr   error
}

// chunkEntry is one chunk of the index: where it starts in the stream, its
// length and its digest.
type chunkEntry struct {
	offset, size uint64
	digest       [32]byte
}

// chunkUploadWorkers is how many chunks of one archive are hashed and sent to
// PBS at a time: the app's count (gui/backup_inline.go), where 8 nearly
// doubled the speed of sending new chunks one by one and more gave nothing
// further.
const chunkUploadWorkers = 8

// chunkBufferSize is the room a chunk's buffer is made with: the chunker's
// average, so most chunks are built without the buffer growing and being
// copied (a chunk can reach four times this, and then grows).
const chunkBufferSize = 4 << 20

// uploadJob is one chunk for the worker pool: its index entry, which the
// worker completes with the digest, and its data.
type uploadJob struct {
	entry *chunkEntry
	data  []byte
}

func (c *ChunkState) Init(client *pbscommon.PBSClient, newchunk *atomic.Uint64, reusechunk *atomic.Uint64, knownChunks *haxmap.Map[string, bool]) {
	c.reset(newchunk, reusechunk, knownChunks)
	// The digest a chunk is published under: sha256(plaintext) for a plain
	// snapshot, sha256(plaintext || id_key) once a key file is in play, since
	// that is what the encrypted chunk store and both index formats key on.
	c.startUploads(client.ChunkDigest, client.UploadDynamicCompressedChunk)
}

// reset is Init without the worker pool.
func (c *ChunkState) reset(newchunk *atomic.Uint64, reusechunk *atomic.Uint64, knownChunks *haxmap.Map[string, bool]) {
	c.entries = nil
	c.pos = 0
	c.chunkcount = 0
	c.current_chunk = make([]byte, 0, chunkBufferSize)
	c.C = pbscommon.Chunker{}
	c.C.New(1024 * 1024 * 4)
	c.reusechunk = reusechunk
	c.newchunk = newchunk
	c.knownChunks = knownChunks
}

// startUploads starts the worker pool, which finds each chunk's digest with
// digest and sends a new one with upload. c.wrid is set after Init, by
// CreateDynamicIndex, and before the first chunk is queued: a worker reads it
// only once it has a job, and the channel send orders the two. A worker
// writes the digest into the job's entry; the walk reads entries only after
// stopUploads has waited for every worker.
func (c *ChunkState) startUploads(digest func([]byte) [32]byte, upload func(wrid uint64, digest string, data []byte) error) {
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.uploadJobs = make(chan uploadJob, chunkUploadWorkers*2)
	c.uploadWG.Add(chunkUploadWorkers)
	for i := 0; i < chunkUploadWorkers; i++ {
		go func() {
			defer c.uploadWG.Done()
			for job := range c.uploadJobs {
				if c.ctx.Err() != nil {
					continue // failed or abandoned: send nothing more
				}
				job.entry.digest = digest(job.data)
				shahash := hex.EncodeToString(job.entry.digest[:])
				// GetOrSet marks the chunk as sent before it is sent, so a copy
				// elsewhere in the stream, in another worker, is not sent too;
				// if the send fails, the index is not closed and the snapshot
				// not finished, so nothing refers to it.
				if _, known := c.knownChunks.GetOrSet(shahash, true); known {
					chunkNotice("Reuse chunk[%s] %d bytes\n", shahash, len(job.data))
					c.reusechunk.Add(1)
					continue
				}
				chunkNotice("New chunk[%s] %d bytes\n", shahash, len(job.data))
				if err := withRetryCtx(c.ctx, "chunk upload", chunkRetryTime, func() error {
					return upload(c.wrid, shahash, job.data)
				}); err != nil {
					if c.ctx.Err() == nil { // not just ended by an earlier failure
						c.fail(fmt.Errorf("failed to upload chunk %s: %w", shahash, err))
					}
					continue
				}
				c.newchunk.Add(1)
			}
		}()
	}
}

// fail keeps err if it is the first upload failure, and ends the uploads
// still to come.
func (c *ChunkState) fail(err error) {
	c.fatalMu.Lock()
	if c.fatalErr == nil {
		c.fatalErr = err
	}
	c.fatalMu.Unlock()
	c.cancel()
}

// uploadErr is the first upload failure so far, or nil.
func (c *ChunkState) uploadErr() error {
	c.fatalMu.Lock()
	defer c.fatalMu.Unlock()
	return c.fatalErr
}

// stopUploads ends the upload pool and waits for its workers: Eof once every
// chunk is queued, and a backup that stops early (deferred), which sends
// nothing more. The workers must be gone before the session is closed or a
// new one is started on the same client.
func (c *ChunkState) stopUploads(abandon bool) {
	if c.uploadJobs == nil {
		return
	}
	if abandon {
		c.cancel()
	}
	c.closeOnce.Do(func() { close(c.uploadJobs) })
	c.uploadWG.Wait()
	c.cancel()
}

// queue adds the chunk in current_chunk to the index at the current position
// and hands it to the worker pool, which hashes it and sends it if PBS does
// not have it yet; the next chunk starts in a new buffer. The walk only finds
// the boundaries.
func (c *ChunkState) queue() error {
	if err := c.uploadErr(); err != nil {
		return err
	}
	e := &chunkEntry{offset: c.pos, size: uint64(len(c.current_chunk))}
	c.entries = append(c.entries, e)
	c.pos += e.size
	c.chunkcount++
	c.uploadJobs <- uploadJob{entry: e, data: c.current_chunk}
	c.current_chunk = make([]byte, 0, chunkBufferSize)
	return nil
}

func (c *ChunkState) HandleData(b []byte, client *pbscommon.PBSClient) error {
	if err := c.uploadErr(); err != nil {
		return err
	}
	for {
		chunkpos := c.C.Scan(b)
		if chunkpos == 0 {
			// No break in what is left: it starts the next chunk.
			c.current_chunk = append(c.current_chunk, b...)
			return nil
		}
		c.current_chunk = append(c.current_chunk, b[:chunkpos]...)
		if err := c.queue(); err != nil {
			return err
		}
		b = b[chunkpos:]
	}
}

// SuggestBoundary cuts the current chunk here when that gives a chunk within
// the normal size bounds (the official payload chunker's rule); used at file
// starts in a split archive's payload stream.
func (c *ChunkState) SuggestBoundary(client *pbscommon.PBSClient) error {
	n := uint64(len(c.current_chunk))
	if n < c.C.MinSize() || n > c.C.MaxSize() {
		return nil
	}
	if err := c.emitCurrent(client); err != nil {
		return err
	}
	c.C.Reset()
	return nil
}

// ForceBoundary ends the chunk in progress here, whatever its size (needed
// before metadata change detection appends reused chunks).
func (c *ChunkState) ForceBoundary(client *pbscommon.PBSClient) error {
	if err := c.emitCurrent(client); err != nil {
		return err
	}
	c.C.Reset()
	return nil
}

// InjectKnownChunk appends a chunk of the previous payload index without its
// data; it must be in that index, which /previous registered with the server.
func (c *ChunkState) InjectKnownChunk(digest [32]byte, size uint64) error {
	if err := c.uploadErr(); err != nil {
		return err
	}
	if len(c.current_chunk) != 0 {
		return fmt.Errorf("internal error: reused chunk appended while %d bytes are pending", len(c.current_chunk))
	}
	shahash := hex.EncodeToString(digest[:])
	if _, ok := c.knownChunks.Get(shahash); !ok {
		return fmt.Errorf("internal error: reused chunk %s is not in the previous index", shahash)
	}
	c.reusechunk.Add(1)
	c.entries = append(c.entries, &chunkEntry{offset: c.pos, size: size, digest: digest})
	c.pos += size
	c.chunkcount += 1
	return nil
}

func (c *ChunkState) Eof(client *pbscommon.PBSClient) error {
	//Here we write the remainder of data for which cyclic hash did not trigger
	if err := c.emitCurrent(client); err != nil {
		return err
	}
	// Every chunk is queued: wait for the uploads, and close the index only
	// if they all got there.
	c.stopUploads(false)
	if err := c.uploadErr(); err != nil {
		return err
	}
	return c.closeIndex(client)
}

// emitCurrent publishes the pending bytes as one chunk (no-op when empty).
func (c *ChunkState) emitCurrent(client *pbscommon.PBSClient) error {
	if len(c.current_chunk) == 0 {
		return nil
	}
	return c.queue()
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

func (c *ChunkState) closeIndex(client *pbscommon.PBSClient) error {
	assignments, assignmentOffsets, digest := c.indexList()
	//Avoid incurring in request entity too large by chunking assignment PUT requests in blocks of at most 128 chunks
	for k := 0; k < len(assignments); k += 128 {
		k2 := k + 128
		if k2 > len(assignments) {
			k2 = len(assignments)
		}
		digests, offsets := assignments[k:k2], assignmentOffsets[k:k2]
		if err := withRetry("chunk assignment", indexRetryTime, func() error {
			return client.AssignDynamicChunks(c.wrid, digests, offsets)
		}); err != nil {
			return fmt.Errorf("failed to assign chunks (batch %d-%d): %w", k, k2, err)
		}
	}

	if err := withRetry("closing the index", indexRetryTime, func() error {
		return client.CloseDynamicIndex(c.wrid, digest, c.pos, c.chunkcount)
	}); err != nil {
		return fmt.Errorf("failed to close dynamic index: %w", err)
	}
	return nil
}

// How long the retries of one request may take, as in the app: the index's
// assignments and close 10 minutes, the manifest and the session's finish 5.
// A chunk upload's attempts have no shared limit (0): each is bounded by the
// upload's own watch (pbscommon/upload_stall.go), given up once it stops
// moving and never while it moves. With 5 minutes for all of them, a first
// attempt that stalled used it all and the chunk was never sent again.
const (
	chunkRetryTime  = time.Duration(0)
	indexRetryTime  = 10 * time.Minute
	finishRetryTime = 5 * time.Minute
)

// withRetry runs one request to PBS and, if it fails in a way that passes
// (retry.DefaultRetryable: a timeout, a reset connection, a 502, 503, 504 or
// 429), tries it again, up to 5 times with growing waits, as the app does
// (tizbac/proxmoxbackupclient_go#59). Each retry is printed. The error
// returned is the request's own, so retry.SessionLost still recognises it.
func withRetry(what string, limit time.Duration, op func() error) error {
	return withRetryCtx(context.Background(), what, limit, op)
}

// withRetryCtx is withRetry ended early, without a further attempt, once
// parent is cancelled. A limit of 0 is none: only the attempts are counted.
func withRetryCtx(parent context.Context, what string, limit time.Duration, op func() error) error {
	cfg := retry.DefaultConfig()
	cfg.MaxAttempts = 5
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if limit > 0 {
		ctx, cancel = context.WithTimeout(parent, limit)
	} else {
		ctx, cancel = context.WithCancel(parent)
	}
	defer cancel()
	attempt := 0
	var last error
	err := retry.DoWithJitter(ctx, cfg, retry.DefaultRetryable, func() error {
		attempt++
		last = op()
		if last != nil && attempt < cfg.MaxAttempts && retry.DefaultRetryable(last) {
			fmt.Fprintf(os.Stderr, "%s failed (attempt %d of %d), trying again: %v\n", what, attempt, cfg.MaxAttempts, last)
		}
		return last
	})
	switch {
	case err == nil:
		return nil
	case last == nil: // ended by the time limit before a new attempt
		return err
	case attempt > 1:
		return fmt.Errorf("after %d attempts: %w", attempt, last)
	default:
		return last
	}
}

func Main() {
	newchunk := new(atomic.Uint64)
	reusechunk := new(atomic.Uint64)

	cfg := loadConfig()
	changeDetectionMode = cfg.ChangeDetectionMode
	noMTime = cfg.NoMTime
	excludeList = cfg.ExcludeList
	readAhead = cfg.ReadAhead

	// Errors go to stderr with a non-zero exit, never a message box: a box
	// waits for a click that an unattended scheduled run never gets (and on
	// Windows the run used to carry on after it with the invalid options).
	if problems := cfg.problems(); len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, p)
		}
		fmt.Fprintf(os.Stderr, "Options can also come from -config <file> and the PBS_* environment variables. Run %s -help for all of them.\n", clientcommon.ToolName())
		os.Exit(1)
	}
	if line := cfg.ApplySpeedLimits(); line != "" {
		fmt.Println(line)
	}

	// Without -certfingerprint, fetch the fingerprint the server presents and
	// ask the user to confirm it before pinning.
	fingerprint, err := clientcommon.ConfirmFingerprint(cfg.BaseURL, cfg.CertFingerprint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	cfg.CertFingerprint = fingerprint

	// Ticket login without -pbspassword: ask for it on the console.
	if cfg.PBSUsername != "" && cfg.PBSPassword == "" {
		cfg.PBSPassword, err = clientcommon.PromptPassword(fmt.Sprintf("Password for %s: ", cfg.PBSUsername))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	L := clientcommon.Locking{}

	lock_ok := L.AcquireProcessLock()
	if !lock_ok {

		fmt.Fprintln(os.Stderr, "Another backup job is running; backup jobs run one at a time. Try again when it has finished.")
		os.Exit(2)
	}
	defer L.ReleaseProcessLock()

	insecure := cfg.CertFingerprint != ""

	crypt, err := clientcommon.LoadCryptConfig(cfg.KeyFile, cfg.KeyFilePassphrase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// newClient builds a PBS client for one backup group. A multi-directory run
	// needs one per directory, because each directory is its own group.
	newClient := func(backupID string) (*pbscommon.PBSClient, error) {
		c := &pbscommon.PBSClient{
			BaseURL:         cfg.BaseURL,
			CertFingerPrint: cfg.CertFingerprint, //"ea:7d:06:f9:87:73:a4:72:d0:e8:05:a4:b3:3d:95:d7:0a:26:dd:6d:5c:ca:e6:99:83:e4:11:3b:5f:10:f4:4b",
			AuthID:          cfg.AuthID,
			Secret:          cfg.Secret,
			Username:        cfg.PBSUsername,
			Password:        cfg.PBSPassword,
			Datastore:       cfg.Datastore,
			Namespace:       cfg.Namespace,
			Insecure:        insecure,
			Crypt:           crypt,
			// Empty is default, zstd level 3, as before.
			CompressionLevel: pbscommon.ParseCompressionLevel(cfg.Compression),
			Manifest: pbscommon.BackupManifest{
				BackupID: backupID,
			},
		}
		if c.Username != "" {
			if err := c.ObtainTicket(); err != nil {
				return nil, err
			}
		}
		return c, nil
	}
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: failed to retrieve hostname:", err)
		hostname = "unknown"
	}

	dirs := cfg.Dirs()
	var client *pbscommon.PBSClient
	if len(dirs) <= 1 {
		client, err = newClient(cfg.BackupID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: ticket login failed: %v\n", err)
			os.Exit(1)
		}
	}

	// -progress: the bar is created once every prompt and login is done, so
	// nothing of the setup has to share the terminal with it. It is drawn on
	// standard error, so standard output stays as it was.
	var restoreProgress func()
	var restoreLog func()
	if cfg.Progress {
		progressBar = clientcommon.NewProgress(os.Stderr, true)
		restoreProgress = progressBar.CaptureStdout()
		if progressBar.Active() {
			oldLog := log.Writer()
			log.SetOutput(progressBar.Writer())
			restoreLog = func() { log.SetOutput(oldLog) }
		}
	}

	begin := time.Now()
	var readErrors []string
	// snapshots holds the clients of the snapshots this run made, for the
	// mail warning note below.
	var snapshots []*pbscommon.PBSClient
	if len(dirs) == 1 {
		readErrors, err = backupWithRerun(client, newchunk, reusechunk, cfg.PxarOut, dirs[0], cfg.UseVSS)
		if err == nil {
			snapshots = append(snapshots, client)
		}
	} else if len(dirs) > 1 {
		baseID := cfg.BackupID
		if baseID == "" {
			baseID = hostname
		}
		readErrors, snapshots, err = backup_many(newClient, newchunk, reusechunk, cfg.PxarOut, dirs, baseID, cfg.UseVSS)
	} else if cfg.BackupStreamName != "" {
		sn := cfg.BackupStreamName
		if !strings.HasSuffix(sn, ".didx") {
			sn += ".didx"
		}
		notice("Backing up from STDIN to %s", sn)
		err = backupStreamOnce(client, newchunk, reusechunk, sn, os.Stdin)
		if err == nil {
			snapshots = append(snapshots, client)
		}

	} else {
		panic("No backup dir or stream name specified, exiting")
	}

	end := time.Now()

	if progressBar != nil {
		// A good run is shown complete even if a few files could not be read:
		// the backup itself is over, and the final message says so.
		if err == nil {
			progressBar.Complete()
		}
		if restoreLog != nil {
			restoreLog()
		}
		restoreProgress()
		progressBar.Finish()
		progressBar = nil
	}

	mailCtx := clientcommon.MailCtx{
		NewChunks:    newchunk.Load(),
		ReusedChunks: reusechunk.Load(),
		Error:        err,
		ReadErrors:   readErrors,
		Hostname:     hostname,
		Datastore:    cfg.Datastore,
		StartTime:    begin,
		EndTime:      end,
	}

	mailBodyTemplate := defaultMailBodyTemplate
	if cfg.SMTP != nil && cfg.SMTP.Template != nil && cfg.SMTP.Template.Body != "" {
		mailBodyTemplate = cfg.SMTP.Template.Body
	}

	notice("New %d, Reused %d, backup took %s.", newchunk.Load(), reusechunk.Load(), end.Sub(begin))
	var msg string
	msg, err = mailCtx.BuildStr(mailBodyTemplate)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot use custom mail body: "+err.Error())
		msg, err = mailCtx.BuildStr(defaultMailBodyTemplate)
		if err != nil {
			// this should never happen
			panic(err)
		}
	}

	if cfg.SMTP != nil {
		var subject string

		mailSubjectTemplate := defaultMailSubjectTemplate
		if cfg.SMTP.Template != nil && cfg.SMTP.Template.Subject != "" {
			mailSubjectTemplate = cfg.SMTP.Template.Subject
		}

		subject, err = mailCtx.BuildStr(mailSubjectTemplate)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Cannot use custom mail subject: "+err.Error())
			subject, err = mailCtx.BuildStr(defaultMailSubjectTemplate)
			if err != nil {
				// this should never happen
				panic(err)
			}
		}
		// A mail problem is reported on stderr and never changes the backup's
		// own exit code, as in machinebackup.
		mailSent := true
		client, err := clientcommon.SetupMailClient(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.Insecure)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Cannot connect to the mail server: "+err.Error())
			mailSent = false
		} else {
			defer func() { _ = client.Quit() }() // the mails are sent or reported by then
			for _, ccc := range cfg.SMTP.Mails {
				if err := clientcommon.SendMail(ccc.From, ccc.To, subject, msg, client); err != nil {
					fmt.Fprintln(os.Stderr, "Cannot send the notification e-mail: "+err.Error())
					mailSent = false
				} else {
					fmt.Printf("Notification e-mail sent to %s\n", ccc.To)
				}
			}
		}
		// The mail problem also goes in the PBS comment of every snapshot this
		// run made, where it is seen when the backup is viewed. A failed backup
		// has no snapshot to note it on.
		if !mailSent {
			for _, s := range snapshots {
				if nerr := s.SetComment(clientcommon.MailFailedNote); nerr != nil {
					fmt.Fprintf(os.Stderr, "Cannot add the mail warning to the PBS comment of %s: %v\n", s.Manifest.BackupID, nerr)
				} else {
					fmt.Printf("Mail warning added to the PBS comment of %s.\n", s.Manifest.BackupID)
				}
			}
		}
	}

	// -shutdown-after: the backup has run, worked or not, and the mail has
	// gone; the exit code below still says how it went.
	clientcommon.ShutdownAfterRun(cfg.AfterRun)

	// Exit non-zero so schedulers never read a failed or incomplete backup as
	// success. mailCtx.Error is the fatal error; ReadErrors means the snapshot
	// committed but is missing unreadable files.
	if mailCtx.Error != nil {
		fmt.Fprintln(os.Stderr, "backup failed:", mailCtx.Error)
		os.Exit(1)
	}
	if len(readErrors) > 0 {
		fmt.Fprintf(os.Stderr, "backup completed with %d read error(s); snapshot is incomplete\n", len(readErrors))
		os.Exit(3)
	}

}

// backup_many backs up several directories in one run. Each directory is its own
// backup group with the id <base>_<path> (clientcommon.GenerateBackupID), so PBS
// retention and restore treat every directory as an independent series; a GUI
// Backup Set keeps its folders in one group, one archive each, instead. A
// directory that fails does not stop the others; the failures are joined into
// the returned error. The clients of the snapshots that were made are returned
// too, so a note can be added to them afterwards.
func backup_many(newClient func(backupID string) (*pbscommon.PBSClient, error), newchunk, reusechunk *atomic.Uint64, pxarOut string, dirs []string, baseID string, usevss bool) ([]string, []*pbscommon.PBSClient, error) {
	if pxarOut != "" {
		return nil, nil, fmt.Errorf("-pxarout writes a single archive and cannot be combined with several -backupdir")
	}

	ids := make(map[string]string, len(dirs))
	for _, dir := range dirs {
		id := clientcommon.GenerateBackupID(baseID, dir)
		if other, dup := ids[id]; dup {
			return nil, nil, fmt.Errorf("%q and %q would both be backed up as backup-id %q; pass distinct directories", other, dir, id)
		}
		ids[id] = dir
	}

	var readErrors []string
	var done []*pbscommon.PBSClient
	var failures []error
	for _, dir := range dirs {
		id := clientcommon.GenerateBackupID(baseID, dir)
		notice("Backing up %s as backup group %s", dir, id)

		client, err := newClient(id)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: ticket login failed: %w", dir, err))
			continue
		}
		dirReadErrors, err := backupWithRerun(client, newchunk, reusechunk, "", dir, usevss)
		// Release the session even after a failure, otherwise PBS keeps the
		// group locked until the connection times out.
		client.Close()
		readErrors = append(readErrors, dirReadErrors...)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", dir, err))
		} else {
			done = append(done, client)
		}
	}
	return readErrors, done, errors.Join(failures...)
}

// folderRerunWait is how long a folder whose session to PBS was lost waits
// before it is backed up again (retry.FolderRerunWait; a test sets it short).
var folderRerunWait = retry.FolderRerunWait

// backupWithRerun is backup, run again with a new session when the
// connection to PBS was lost (retry.SessionLost), after folderRerunWait, as
// the app does: PBS ends the session with the connection, and the wait lets
// it release the backup group. A loss in the last attempt, or any other
// error, is returned. The counts carry on across attempts, as chunks sent in
// the lost session are sent again.
func backupWithRerun(client *pbscommon.PBSClient, newchunk, reusechunk *atomic.Uint64, pxarOut, dir string, usevss bool) ([]string, error) {
	waitedSecond := false
	for attempt := 1; ; attempt++ {
		readErrors, err := backup(client, newchunk, reusechunk, pxarOut, dir, usevss)
		if err != nil && !waitedSecond && pbscommon.IsBackupTimeTaken(err) {
			client.Close()
			waitedSecond = true
			attempt--
			sameSecondNote()
			continue
		}
		if err == nil || attempt >= retry.FolderRerunAttempts || !retry.SessionLost(err) {
			return readErrors, err
		}
		client.Close()
		at := time.Now().Add(folderRerunWait)
		fmt.Fprintf(os.Stderr, "The connection to PBS was lost while backing up %s (attempt %d of %d): %v\n", dir, attempt, retry.FolderRerunAttempts, err)
		fmt.Fprintf(os.Stderr, "Backing it up again with a new session at %s, once PBS has let go of the backup group; chunks sent in the lost session are sent again.\n", at.Format("15:04:05"))
		time.Sleep(folderRerunWait)
	}
}

// sameSecondNote says why the backup starts again, and waits for the next
// second. PBS gives a snapshot the second its session starts as its backup
// time and refuses a second one of a group in the same second
// (pbscommon.BackupTimeTakenErr): two runs chained in a script, or a run
// straight after a quick one. A second refusal is returned as it is: then
// the clock is the likelier cause, and the error says so.
func sameSecondNote() {
	fmt.Fprintln(os.Stderr, "PBS already has a backup of this group from this second; starting again in the next second.")
	pbscommon.WaitNextSecond()
}

// backupStreamOnce is backup_stream, started again once if PBS refuses the
// session for a backup time already taken. That refusal comes with the
// first request, before any input has been read.
func backupStreamOnce(client *pbscommon.PBSClient, newchunk, reusechunk *atomic.Uint64, filename string, stream io.Reader) error {
	err := backup_stream(client, newchunk, reusechunk, filename, stream)
	if err != nil && pbscommon.IsBackupTimeTaken(err) {
		client.Close()
		sameSecondNote()
		err = backup_stream(client, newchunk, reusechunk, filename, stream)
	}
	return err
}

// backup_stream backs up standard input. Each request to PBS is retried in
// place (withRetry), but a lost session is not run again: the input already
// read is gone.
func backup_stream(client *pbscommon.PBSClient, newchunk, reusechunk *atomic.Uint64, filename string, stream io.Reader) error {
	knownChunks := haxmap.New[string, bool]()
	client.Connect(false, "host")
	// -progress: standard input cannot be counted beforehand, so the bar is
	// indeterminate: it slides a pulse and counts the bytes as they arrive.
	if progressBar != nil {
		progressBar.StartCopy(0, 0)
		progressBar.Current("standard input (size unknown)")
	}
	previousDidx, err := client.DownloadPreviousToBytes(filename)
	if err != nil {
		return err
	}

	notice("Downloaded previous DIDX: %d bytes", len(previousDidx))

	// Defensive parse: a truncated/short/odd-length previous index (or a sub-8-byte
	// error body) must not panic — fall back to no dedup (re-upload everything).
	prevDigests := pbscommon.ParsePreviousDIDXChunkDigests(previousDidx)
	if len(prevDigests) == 0 {
		notice("Previous index unusable or empty (%d bytes), uploading all chunks", len(previousDidx))
	}
	for _, shahash := range prevDigests {
		knownChunks.Set(shahash, true)
	}

	notice("Known chunks: %d!", knownChunks.Len())

	streamChunk := ChunkState{}
	streamChunk.Init(client, newchunk, reusechunk, knownChunks)
	defer streamChunk.stopUploads(true) // Eof stops it first on success

	streamChunk.wrid, err = client.CreateDynamicIndex(filename)
	if err != nil {
		return err
	}
	B := make([]byte, 65536)
	for {
		n, rerr := stream.Read(B)

		b := B[:n]

		if err := streamChunk.HandleData(b, client); err != nil {
			return fmt.Errorf("failed to handle stream data: %w", err)
		}
		if progressBar != nil {
			progressBar.AddBytes(int64(n))
		}

		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return fmt.Errorf("failed to read stream: %w", rerr)
		}
	}

	// Eof() already closes the dynamic index; closing it again fails the request
	// and aborts the snapshot before UploadManifest/Finish.
	if progressBar != nil {
		progressBar.Current("finishing")
	}
	if err := streamChunk.Eof(client); err != nil {
		return fmt.Errorf("failed to finalize stream: %w", err)
	}

	if err := withRetry("manifest upload", finishRetryTime, client.UploadManifest); err != nil {
		return err
	}

	return withRetry("finishing the backup", finishRetryTime, client.Finish)
}

// changeDetectionMode is the -change-detection-mode setting (see Config).
var changeDetectionMode string

// noMTime is the -no-mtime setting (see Config).
var noMTime bool

// excludeList is the -exclude setting (see Config).
var excludeList []string

// readAhead is the -read-ahead setting (see Config).
var readAhead int

func backup_real(client *pbscommon.PBSClient, newchunk, reusechunk *atomic.Uint64, pxarOut string, backupdir string, originalPath string, vssUsed bool) ([]string, error) {
	client.Connect(false, "host")
	knownChunks := haxmap.New[string, bool]()

	split := pbscommon.SplitArchiveMode(changeDetectionMode)
	archive := &pbscommon.PXARArchive{}
	archive.ArchiveName = "backup.pxar.didx"
	archive.ExcludeRoot = originalPath // the folder as named, also when read through a shadow copy
	archive.OnNotice = func(msg string) { notice("%s", msg) }
	archive.NoMTime = noMTime
	// Relative and absolute patterns are taken from ExcludeRoot, the folder as
	// named, also when it is read through a shadow copy.
	archive.ExcludeList = excludeList
	archive.PrefetchWorkers = readAhead
	if split {
		// Metadata stream; the payload stream is backup.ppxar.didx.
		archive.ArchiveName = "backup.mpxar.didx"
		archive.Split = true
	}

	// -progress: walk the folder once to know how much there is, then draw the
	// bar from the file content as it is archived. Standard input has no
	// pre-pass; its bar is indeterminate (see backup_stream).
	if progressBar != nil {
		files, bytes := countBackupEntries(backupdir, originalPath)
		notice("%d files, %s to back up", files, clientcommon.FormatBytes(bytes))
		progressBar.StartCopy(files, bytes)
		archive.OnFile = func(path string) {
			progressBar.Current(progressPath(backupdir, originalPath, path))
		}
		archive.OnData = func(n int) { progressBar.AddBytes(int64(n)) }
		archive.OnFileDone = func(string) { progressBar.AddFile() }
	}

	// Captures NTFS ACLs, attributes and creation times on Windows, extended
	// attributes incl. POSIX ACLs on Linux; uploaded as the same side-car blob
	// the GUI writes so a GUI restore can re-apply them.
	hostname, _ := os.Hostname()
	metaCollector := pbscommon.NewFileMetaCollector(backupdir, hostname)
	archive.MetaCollector = metaCollector

	// Same root-level meta file the GUI injects: lets the GUI offer "restore to
	// original path" for CLI snapshots.
	if metaJSON, merr := pbscommon.GenerateBackupMeta(client.Manifest.BackupID, originalPath, hostname, "directorybackup", vssUsed); merr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to generate backup metadata: %v\n", merr)
	} else {
		archive.VirtualFiles = map[string][]byte{pbscommon.BackupMetaFilename: metaJSON}
	}

	previousDidx, err := client.DownloadPreviousToBytes(archive.ArchiveName)
	if err != nil {
		return nil, err
	}

	notice("Downloaded previous DIDX: %d bytes", len(previousDidx))

	/*f2, _ := os.Create("test.didx")
	defer f2.Close()

	f2.Write(previous_didx)*/

	/*
		Here we download the previous dynamic index to figure out which chunks are the same of what
		we are going to upload to avoid unnecessary traffic and compression cpu usage
	*/

	// Defensive parse: a truncated/short/odd-length previous index (or a sub-8-byte
	// error body) must not panic — fall back to no dedup (re-upload everything).
	prevDigests := pbscommon.ParsePreviousDIDXChunkDigests(previousDidx)
	if len(prevDigests) == 0 {
		notice("Previous index unusable or empty (%d bytes), uploading all chunks", len(previousDidx))
	}
	for _, shahash := range prevDigests {
		knownChunks.Set(shahash, true)
	}

	notice("Known chunks: %d!", knownChunks.Len())
	f := &os.File{}
	if pxarOut != "" {
		f, err = os.Create(pxarOut)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }() // a debug copy; the backup does not depend on it
	}
	/**/

	// Each stream's upload pool is stopped when the backup ends, so none is
	// still sending when an error closes the session; Eof stops it first on
	// success.
	pxarChunk := ChunkState{}
	pxarChunk.Init(client, newchunk, reusechunk, knownChunks)
	defer pxarChunk.stopUploads(true)

	// The catalog has its own list of chunks sent: with one list, a chunk of
	// the archive could be marked as sent by the catalog's pool and the
	// archive's index closed while that send is still under way.
	pcat1Chunk := ChunkState{}
	pcat1Chunk.Init(client, newchunk, reusechunk, haxmap.New[string, bool]())
	defer pcat1Chunk.stopUploads(true)

	var payloadChunk *ChunkState
	if split {
		payloadKnown := haxmap.New[string, bool]()
		prevPayload, perr := client.DownloadPreviousToBytes("backup.ppxar.didx")
		if perr == nil {
			for _, shahash := range pbscommon.ParsePreviousDIDXChunkDigests(prevPayload) {
				payloadKnown.Set(shahash, true)
			}
		}
		notice("Known payload chunks: %d", payloadKnown.Len())
		payloadChunk = &ChunkState{}
		payloadChunk.Init(client, newchunk, reusechunk, payloadKnown)
		defer payloadChunk.stopUploads(true)
		if payloadChunk.wrid, err = client.CreateDynamicIndex("backup.ppxar.didx"); err != nil {
			return nil, err
		}
		archive.PayloadWriteCB = func(b []byte) error { return payloadChunk.HandleData(b, client) }
		archive.OnPayloadFileStart = func() error { return payloadChunk.SuggestBoundary(client) }
		if changeDetectionMode == pbscommon.ChangeDetectionMetadata && noMTime {
			notice("Metadata change detection: off with -no-mtime (no times to compare), reading every file")
		} else if changeDetectionMode == pbscommon.ChangeDetectionMetadata {
			// Reuse unchanged files from the previous snapshot without
			// reading them; an unusable previous snapshot means data mode.
			prev, prevTime, lerr := client.LoadPreviousSplitArchive(archive.ArchiveName, "backup.ppxar.didx", prevPayload)
			if lerr != nil {
				notice("Metadata change detection: no usable previous snapshot (%v), reading every file", lerr)
			} else {
				notice("Metadata change detection: comparing with snapshot %s (%d files)",
					time.Unix(prevTime, 0).UTC().Format(time.RFC3339), len(prev.Files))
				archive.Reuse = pbscommon.NewPayloadReuse(prev,
					func() error { return payloadChunk.ForceBoundary(client) },
					payloadChunk.InjectKnownChunk)
			}
		}
	}

	pxarChunk.wrid, err = client.CreateDynamicIndex(archive.ArchiveName)
	if err != nil {
		return nil, err
	}
	pcat1Chunk.wrid, err = client.CreateDynamicIndex("catalog.pcat1.didx")
	if err != nil {
		return nil, err
	}

	archive.WriteCB = func(b []byte) error {

		if pxarOut != "" {
			if _, err := f.Write(b); err != nil {
				return fmt.Errorf("failed to write to pxar output file: %w", err)
			}
		}

		if err := pxarChunk.HandleData(b, client); err != nil {
			return err
		}

		return nil
	}

	archive.CatalogWriteCB = func(b []byte) error {
		return pcat1Chunk.HandleData(b, client)
	}

	//This is the entry point of backup job which will start streaming with the PCAT and PXAR write callback
	//Data to be hashed and eventuall uploaded

	if _, err = archive.WriteDir(backupdir, "", true); err != nil {
		return nil, fmt.Errorf("failed to write directory archive: %w", err)
	}
	// What -exclude left out, by the names the user knows (not the shadow
	// copy's); an excluded folder is one entry, its contents are not walked.
	var excluded []pbscommon.FileIssue
	for _, p := range archive.ExcludedFiles {
		if vssUsed {
			p = unmapSnapshotPathString(p, backupdir, originalPath)
		}
		notice("Excluded: %s", p)
		excluded = append(excluded, pbscommon.FileIssue{Path: p, Reason: "excluded by user policy"})
	}
	if len(excluded) > 0 {
		notice("%d files and folders excluded by -exclude", len(excluded))
	}
	if archive.Reuse != nil {
		st := archive.Reuse.Stats
		notice("Metadata change detection: %d unchanged files (%d bytes) reused without reading, %d chunks reused, %d bytes padding; %d unchanged files read anyway to limit padding",
			st.Files, st.Bytes, st.Chunks, st.Padding, st.OverPadding)
	}

	if progressBar != nil {
		progressBar.Current("finishing")
	}
	if err = pxarChunk.Eof(client); err != nil {
		return nil, err
	}
	if payloadChunk != nil {
		if err = payloadChunk.Eof(client); err != nil {
			return nil, err
		}
	}
	if err = pcat1Chunk.Eof(client); err != nil {
		return nil, err
	}

	if m, merr := metaCollector.FinalizeRaw(); merr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to finalize file metadata: %v\n", merr)
	} else if m != nil {
		combined := &pbscommon.CombinedBackupFileMeta{
			Version:  pbscommon.FileMetaVersion,
			Captured: m.Captured,
			Host:     hostname,
			Archives: map[string]*pbscommon.BackupFileMeta{
				"backup": m, // archive base name, same key for classic and split archives
			},
		}
		entries, _, metaErrs := metaCollector.Stats()
		blob, dropped, serr := combined.SerializeCapped(pbscommon.MaxFileMetaBlobBytes)
		if dropped > 0 {
			fmt.Fprintf(os.Stderr, "warning: file metadata too large for one PBS blob: the content of %d alternate data streams was left out\n", dropped)
		}
		if serr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to serialize file metadata: %v\n", serr)
		} else if uerr := client.UploadBlob(pbscommon.FileMetaBlobName, blob); uerr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to upload file metadata blob: %v\n", uerr)
		} else {
			notice("File metadata: %d entries, %d errors", entries, metaErrs)
			if streams, skipped, eas := metaCollector.StreamStats(); streams+skipped+eas > 0 {
				notice("Alternate data streams: %d recorded, %d by name only; extended attributes: %d", streams, skipped, eas)
			}
		}
	}

	unreadable := byRealPath(archive.ReadErrors, vssUsed, backupdir, originalPath)
	sidecar := &pbscommon.BackupSidecar{
		FormatVersion:    1,
		BackupID:         client.Manifest.BackupID,
		Directories:      []string{originalPath},
		GeneratedAt:      time.Now().Unix(),
		ExcludedByPolicy: excluded,
		SkippedReadError: pbscommon.SkippedToIssues(unreadable),
	}
	if sb, serr := json.Marshal(sidecar); serr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to serialize status sidecar: %v\n", serr)
	} else if uerr := client.UploadBlob(pbscommon.BackupStatusFilename, sb); uerr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to upload status sidecar: %v\n", uerr)
	}

	if err := withRetry("manifest upload", finishRetryTime, client.UploadManifest); err != nil {
		return nil, err
	}
	// archive.ReadErrors lists files that could not be read and were skipped:
	// the snapshot committed but is incomplete. Surfaced as a partial result.
	return archive.ReadErrors, nil
}

func backup(client *pbscommon.PBSClient, newchunk, reusechunk *atomic.Uint64, pxarOut string, backupdir string, usevss bool) ([]string, error) {

	notice("Starting backup of %s", backupdir)
	originalPath := backupdir
	if abs, aerr := filepath.Abs(backupdir); aerr == nil {
		originalPath = abs
	}
	// Walk the folder by its absolute, cleaned path. The archive writer takes
	// entries' paths relative to the folder as given, and its children come
	// cleaned from filepath.Join, so "./data" made every -exclude pattern
	// with a separator miss.
	backupdir = originalPath
	// A network path (a share or a drive mapped to one; NFS, SMB/CIFS or FUSE
	// on Linux) has no volume to snapshot, and VSS fails on it: read it live,
	// as the app does, instead of failing the backup.
	if usevss && !snapshot.PathSupportsSnapshot(backupdir) {
		notice("Not taking a snapshot of %s: a network path has no volume to snapshot, so it is read live", backupdir)
		usevss = false
	}
	var err error
	var readErrors []string
	originalDir := backupdir
	snapshotDir := ""
	if usevss {
		err = snapshot.CreateVSSSnapshot(([]string{backupdir}), true, func(snaps map[string]snapshot.SnapShot) error {
			// Get first snapshot from map (Go 1.22 compatible)
			for _, snap := range snaps {
				snapshotDir = snap.FullPath
				break
			}
			backupdir = snapshotDir
			//Remove VSS snapshot on windows, on linux for now NOP
			var e error
			readErrors, e = backup_real(client, newchunk, reusechunk, pxarOut, backupdir, originalPath, true)
			return e

		})
		err = unmapSnapshotPath(err, snapshotDir, originalDir)
		if err != nil && snapshotDir == "" {
			// The snapshot itself could not be taken: say how to go on. On
			// Linux the error already says whether root or a module is
			// missing (snapshot.ErrNeedsRoot, ErrNoSnapshotModule) and what
			// to do; -novss is the other way.
			if runtime.GOOS == "windows" {
				err = fmt.Errorf("%w (a VSS snapshot needs administrator rights: if this is not running elevated, run it so; or add -novss to read the folder without a snapshot)", err)
			} else {
				err = fmt.Errorf("%w (or add -novss to read the folder without a snapshot)", err)
			}
		}
		for i := range readErrors {
			readErrors[i] = unmapSnapshotPathString(readErrors[i], snapshotDir, originalDir)
		}
	} else {
		readErrors, err = backup_real(client, newchunk, reusechunk, pxarOut, backupdir, originalPath, false)
	}

	if err != nil {
		return readErrors, err
	}

	// Commit the snapshot even on a partial (read-error) run so the data that
	// was readable is retained; the partial status is reported via readErrors.
	return readErrors, withRetry("finishing the backup", finishRetryTime, client.Finish)
}
