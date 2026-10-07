package main

import (
	"clientcommon"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"pbscommon"
	"runtime"
	"snapshot"
	"strings"
	"sync/atomic"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/tawesoft/golib/v2/dialog"
)

var defaultMailSubjectTemplate = "Backup {{.Status}}"
var defaultMailBodyTemplate = `{{if .Success}}Backup complete ({{.FromattedDuration}})
Chunks New {{.NewChunks}}, Reused {{.ReusedChunks}}.{{else if .Partial}}Backup completed WITH ERRORS ({{.FromattedDuration}})
{{.ReadErrorCount}} file(s) could not be read and were skipped; the snapshot is incomplete.
Chunks New {{.NewChunks}}, Reused {{.ReusedChunks}}.{{else}}Error occurred while working, backup may be not completed.
Last error is: {{.ErrorStr}}{{end}}`

type ChunkState struct {
	assignments        []string
	assignments_offset []uint64
	pos                uint64
	wrid               uint64
	chunkcount         uint64
	chunkdigests       hash.Hash
	current_chunk      []byte
	C                  pbscommon.Chunker
	newchunk           *atomic.Uint64
	reusechunk         *atomic.Uint64
	knownChunks        *haxmap.Map[string, bool]
}

func (c *ChunkState) Init(newchunk *atomic.Uint64, reusechunk *atomic.Uint64, knownChunks *haxmap.Map[string, bool]) {
	c.assignments = make([]string, 0)
	c.assignments_offset = make([]uint64, 0)
	c.pos = 0
	c.chunkcount = 0
	c.chunkdigests = sha256.New()
	c.current_chunk = make([]byte, 0)
	c.C = pbscommon.Chunker{}
	c.C.New(1024 * 1024 * 4)
	c.reusechunk = reusechunk
	c.newchunk = newchunk
	c.knownChunks = knownChunks
}

func (c *ChunkState) HandleData(b []byte, client *pbscommon.PBSClient) error {
	chunkpos := c.C.Scan(b)

	if chunkpos == 0 {
		//No break happened, just append data
		c.current_chunk = append(c.current_chunk, b...)
	} else {

		for chunkpos > 0 {
			//Append data until break position
			c.current_chunk = append(c.current_chunk, b[:chunkpos]...)

			// The digest has to be the one the chunk is published under:
			// sha256(plaintext) for a plain snapshot, but
			// sha256(plaintext || id_key) once a key file is in play, since
			// that is what the encrypted chunk store and both index formats
			// key on. GetChunkData re-derives it with the same key.
			bindigest := client.ChunkDigest(c.current_chunk)
			shahash := hex.EncodeToString(bindigest[:])

			if _, ok := c.knownChunks.GetOrSet(shahash, true); !ok {
				fmt.Printf("New chunk[%s] %d bytes\n", shahash, len(c.current_chunk))
				c.newchunk.Add(1)

				if err := client.UploadDynamicCompressedChunk(c.wrid, shahash, c.current_chunk); err != nil {
					return fmt.Errorf("failed to upload chunk %s: %w", shahash, err)
				}
			} else {
				fmt.Printf("Reuse chunk[%s] %d bytes\n", shahash, len(c.current_chunk))
				c.reusechunk.Add(1)
			}

			if err := binary.Write(c.chunkdigests, binary.LittleEndian, (c.pos + uint64(len(c.current_chunk)))); err != nil {
				return fmt.Errorf("failed to write chunk offset: %w", err)
			}
			if _, err := c.chunkdigests.Write(bindigest[:]); err != nil {
				return fmt.Errorf("failed to write chunk digest: %w", err)
			}

			c.assignments_offset = append(c.assignments_offset, c.pos)
			c.assignments = append(c.assignments, shahash)
			c.pos += uint64(len(c.current_chunk))
			c.chunkcount += 1

			c.current_chunk = make([]byte, 0)
			b = b[chunkpos:] //Take remainder of data
			chunkpos = c.C.Scan(b)

		}

		//No further break happened, append remaining data
		c.current_chunk = append(c.current_chunk, b...)
	}
	return nil
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
	c.current_chunk = make([]byte, 0)
	c.C.Reset()
	return nil
}

// ForceBoundary ends the chunk in progress here, whatever its size (needed
// before metadata change detection appends reused chunks).
func (c *ChunkState) ForceBoundary(client *pbscommon.PBSClient) error {
	if err := c.emitCurrent(client); err != nil {
		return err
	}
	c.current_chunk = make([]byte, 0)
	c.C.Reset()
	return nil
}

// InjectKnownChunk appends a chunk of the previous payload index without its
// data; it must be in that index, which /previous registered with the server.
func (c *ChunkState) InjectKnownChunk(digest [32]byte, size uint64) error {
	if len(c.current_chunk) != 0 {
		return fmt.Errorf("internal error: reused chunk appended while %d bytes are pending", len(c.current_chunk))
	}
	shahash := hex.EncodeToString(digest[:])
	if _, ok := c.knownChunks.Get(shahash); !ok {
		return fmt.Errorf("internal error: reused chunk %s is not in the previous index", shahash)
	}
	if err := binary.Write(c.chunkdigests, binary.LittleEndian, c.pos+size); err != nil {
		return fmt.Errorf("failed to write chunk offset: %w", err)
	}
	if _, err := c.chunkdigests.Write(digest[:]); err != nil {
		return fmt.Errorf("failed to write chunk digest: %w", err)
	}
	c.reusechunk.Add(1)
	c.assignments_offset = append(c.assignments_offset, c.pos)
	c.assignments = append(c.assignments, shahash)
	c.pos += size
	c.chunkcount += 1
	return nil
}

func (c *ChunkState) Eof(client *pbscommon.PBSClient) error {
	//Here we write the remainder of data for which cyclic hash did not trigger
	if err := c.emitCurrent(client); err != nil {
		return err
	}
	return c.closeIndex(client)
}

// emitCurrent publishes the pending bytes as one chunk (no-op when empty).
func (c *ChunkState) emitCurrent(client *pbscommon.PBSClient) error {
	if len(c.current_chunk) > 0 {
		// Same digest rule as in HandleData: plain sha256, or
		// sha256(plaintext || id_key) when the snapshot is encrypted.
		bindigest := client.ChunkDigest(c.current_chunk)
		shahash := hex.EncodeToString(bindigest[:])

		if err := binary.Write(c.chunkdigests, binary.LittleEndian, (c.pos + uint64(len(c.current_chunk)))); err != nil {
			return fmt.Errorf("failed to write final chunk offset: %w", err)
		}
		if _, err := c.chunkdigests.Write(bindigest[:]); err != nil {
			return fmt.Errorf("failed to write final chunk digest: %w", err)
		}

		if _, ok := c.knownChunks.GetOrSet(shahash, true); !ok {
			fmt.Printf("New chunk[%s] %d bytes\n", shahash, len(c.current_chunk))
			if err := client.UploadDynamicCompressedChunk(c.wrid, shahash, c.current_chunk); err != nil {
				return fmt.Errorf("failed to upload final chunk %s: %w", shahash, err)
			}
			c.newchunk.Add(1)
		} else {
			fmt.Printf("Reuse chunk[%s] %d bytes\n", shahash, len(c.current_chunk))
			c.reusechunk.Add(1)
		}
		c.assignments_offset = append(c.assignments_offset, c.pos)
		c.assignments = append(c.assignments, shahash)
		c.pos += uint64(len(c.current_chunk))
		c.chunkcount += 1

	}
	return nil
}

func (c *ChunkState) closeIndex(client *pbscommon.PBSClient) error {
	//Avoid incurring in request entity too large by chunking assignment PUT requests in blocks of at most 128 chunks
	for k := 0; k < len(c.assignments); k += 128 {
		k2 := k + 128
		if k2 > len(c.assignments) {
			k2 = len(c.assignments)
		}
		if err := client.AssignDynamicChunks(c.wrid, c.assignments[k:k2], c.assignments_offset[k:k2]); err != nil {
			return fmt.Errorf("failed to assign chunks (batch %d-%d): %w", k, k2, err)
		}
	}

	if err := client.CloseDynamicIndex(c.wrid, hex.EncodeToString(c.chunkdigests.Sum(nil)), c.pos, c.chunkcount); err != nil {
		return fmt.Errorf("failed to close dynamic index: %w", err)
	}
	return nil
}

func main() {
	var newchunk *atomic.Uint64 = new(atomic.Uint64)
	var reusechunk *atomic.Uint64 = new(atomic.Uint64)

	cfg := loadConfig()
	changeDetectionMode = cfg.ChangeDetectionMode

	if ok := cfg.valid(); !ok {
		if runtime.GOOS == "windows" {
			usage := "All options are mandatory:\n"
			flag.VisitAll(func(f *flag.Flag) {
				usage += "-" + f.Name + " " + f.Usage + "\n"
			})
			dialog.Error(usage)
		} else {
			fmt.Println("All options are mandatory")

			flag.PrintDefaults()
			os.Exit(1)
		}
	}

	// Without -certfingerprint, fetch the fingerprint the server presents and
	// ask the user to confirm it before pinning.
	fingerprint, err := clientcommon.ConfirmFingerprint(cfg.BaseURL, cfg.CertFingerprint)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	cfg.CertFingerprint = fingerprint

	// Ticket login without -pbspassword: ask for it on the console.
	if cfg.PBSUsername != "" && cfg.PBSPassword == "" {
		cfg.PBSPassword, err = clientcommon.PromptPassword(fmt.Sprintf("Password for %s: ", cfg.PBSUsername))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}

	L := clientcommon.Locking{}

	lock_ok := L.AcquireProcessLock()
	if !lock_ok {

		dialog.Error("Backup jobs need to run exclusively, please wait until the previous job has finished")
		os.Exit(2)
	}
	defer L.ReleaseProcessLock()

	insecure := cfg.CertFingerprint != ""

	crypt, err := clientcommon.LoadCryptConfig(cfg.KeyFile, cfg.KeyFilePassphrase)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
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
		fmt.Println("Failed to retrieve hostname:", err)
		hostname = "unknown"
	}

	dirs := cfg.Dirs()
	var client *pbscommon.PBSClient
	if len(dirs) <= 1 {
		client, err = newClient(cfg.BackupID)
		if err != nil {
			fmt.Printf("Error: ticket login failed: %v\n", err)
			os.Exit(1)
		}
	}

	begin := time.Now()
	var readErrors []string
	if len(dirs) == 1 {
		readErrors, err = backup(client, newchunk, reusechunk, cfg.PxarOut, dirs[0], cfg.UseVSS)
	} else if len(dirs) > 1 {
		baseID := cfg.BackupID
		if baseID == "" {
			baseID = hostname
		}
		readErrors, err = backup_many(newClient, newchunk, reusechunk, cfg.PxarOut, dirs, baseID, cfg.UseVSS)
	} else if cfg.BackupStreamName != "" {
		sn := cfg.BackupStreamName
		if !strings.HasSuffix(sn, ".didx") {
			sn += ".didx"
		}
		fmt.Printf("Backing up from STDIN to %s", sn)
		err = backup_stream(client, newchunk, reusechunk, sn, os.Stdin)

	} else {
		panic("No backup dir or stream name specified, exiting")
	}

	end := time.Now()

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

	fmt.Printf("New %d, Reused %d, backup took %s.\n", newchunk.Load(), reusechunk.Load(), end.Sub(begin))
	var msg string
	msg, err = mailCtx.BuildStr(mailBodyTemplate)
	if err != nil {
		fmt.Println("Cannot use custom mail body: " + err.Error())
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
			fmt.Println("Cannot use custom mail subject: " + err.Error())
			subject, err = mailCtx.BuildStr(defaultMailSubjectTemplate)
			if err != nil {
				// this should never happen
				panic(err)
			}
		}
		client, err := clientcommon.SetupMailClient(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.Insecure)
		if err != nil {
			fmt.Println("Cannot connect to mail server: " + err.Error())
			os.Exit(1)
		}
		defer client.Quit()
		for _, ccc := range cfg.SMTP.Mails {
			err = clientcommon.SendMail(ccc.From, ccc.To, subject, msg, client)
			if err != nil {
				fmt.Println("Cannot send email: " + err.Error())
				os.Exit(1)
			}
		}
	}

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
// backup group with the id <base>_<path> (the same ids the GUI uses), so PBS
// retention and restore treat every directory as an independent series. A
// directory that fails does not stop the others; the failures are joined into
// the returned error.
func backup_many(newClient func(backupID string) (*pbscommon.PBSClient, error), newchunk, reusechunk *atomic.Uint64, pxarOut string, dirs []string, baseID string, usevss bool) ([]string, error) {
	if pxarOut != "" {
		return nil, fmt.Errorf("-pxarout writes a single archive and cannot be combined with several -backupdir")
	}

	ids := make(map[string]string, len(dirs))
	for _, dir := range dirs {
		id := clientcommon.GenerateBackupID(baseID, dir)
		if other, dup := ids[id]; dup {
			return nil, fmt.Errorf("%q and %q would both be backed up as backup-id %q; pass distinct directories", other, dir, id)
		}
		ids[id] = dir
	}

	var readErrors []string
	var failures []error
	for _, dir := range dirs {
		id := clientcommon.GenerateBackupID(baseID, dir)
		fmt.Printf("Backing up %s as backup group %s\n", dir, id)

		client, err := newClient(id)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: ticket login failed: %w", dir, err))
			continue
		}
		dirReadErrors, err := backup(client, newchunk, reusechunk, "", dir, usevss)
		// Release the session even after a failure, otherwise PBS keeps the
		// group locked until the connection times out.
		client.Close()
		readErrors = append(readErrors, dirReadErrors...)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", dir, err))
		}
	}
	return readErrors, errors.Join(failures...)
}

func backup_stream(client *pbscommon.PBSClient, newchunk, reusechunk *atomic.Uint64, filename string, stream io.Reader) error {
	knownChunks := haxmap.New[string, bool]()
	client.Connect(false, "host")
	previousDidx, err := client.DownloadPreviousToBytes(filename)
	if err != nil {
		return err
	}

	fmt.Printf("Downloaded previous DIDX: %d bytes\n", len(previousDidx))

	// Defensive parse: a truncated/short/odd-length previous index (or a sub-8-byte
	// error body) must not panic — fall back to no dedup (re-upload everything).
	prevDigests := pbscommon.ParsePreviousDIDXChunkDigests(previousDidx)
	if len(prevDigests) == 0 {
		fmt.Printf("Previous index unusable or empty (%d bytes), uploading all chunks\n", len(previousDidx))
	}
	for _, shahash := range prevDigests {
		knownChunks.Set(shahash, true)
	}

	fmt.Printf("Known chunks: %d!\n", knownChunks.Len())

	streamChunk := ChunkState{}
	streamChunk.Init(newchunk, reusechunk, knownChunks)

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

		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return fmt.Errorf("failed to read stream: %w", rerr)
		}
	}

	// Eof() already closes the dynamic index; closing it again fails the request
	// and aborts the snapshot before UploadManifest/Finish.
	if err := streamChunk.Eof(client); err != nil {
		return fmt.Errorf("failed to finalize stream: %w", err)
	}

	err = client.UploadManifest()
	if err != nil {
		return err
	}

	return client.Finish()
}

// changeDetectionMode is the -change-detection-mode setting (see Config).
var changeDetectionMode string

func backup_real(client *pbscommon.PBSClient, newchunk, reusechunk *atomic.Uint64, pxarOut string, backupdir string, originalPath string, vssUsed bool) ([]string, error) {
	client.Connect(false, "host")
	knownChunks := haxmap.New[string, bool]()

	split := pbscommon.SplitArchiveMode(changeDetectionMode)
	archive := &pbscommon.PXARArchive{}
	archive.ArchiveName = "backup.pxar.didx"
	archive.OnNotice = func(msg string) { fmt.Println(msg) }
	if split {
		// Metadata stream; the payload stream is backup.ppxar.didx.
		archive.ArchiveName = "backup.mpxar.didx"
		archive.Split = true
	}

	// Captures extended attributes incl. POSIX ACLs on Linux (no-op elsewhere);
	// uploaded as the same side-car blob the GUI writes so a GUI restore can
	// re-apply them.
	hostname, _ := os.Hostname()
	xattrCollector := pbscommon.NewXattrCollector(backupdir, hostname)
	archive.MetaCollector = xattrCollector

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

	fmt.Printf("Downloaded previous DIDX: %d bytes\n", len(previousDidx))

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
		fmt.Printf("Previous index unusable or empty (%d bytes), uploading all chunks\n", len(previousDidx))
	}
	for _, shahash := range prevDigests {
		knownChunks.Set(shahash, true)
	}

	fmt.Printf("Known chunks: %d!\n", knownChunks.Len())
	f := &os.File{}
	if pxarOut != "" {
		f, err = os.Create(pxarOut)
		if err != nil {
			return nil, err
		}
		defer f.Close()
	}
	/**/

	pxarChunk := ChunkState{}
	pxarChunk.Init(newchunk, reusechunk, knownChunks)

	pcat1Chunk := ChunkState{}
	pcat1Chunk.Init(newchunk, reusechunk, knownChunks)

	var payloadChunk *ChunkState
	if split {
		payloadKnown := haxmap.New[string, bool]()
		prevPayload, perr := client.DownloadPreviousToBytes("backup.ppxar.didx")
		if perr == nil {
			for _, shahash := range pbscommon.ParsePreviousDIDXChunkDigests(prevPayload) {
				payloadKnown.Set(shahash, true)
			}
		}
		fmt.Printf("Known payload chunks: %d\n", payloadKnown.Len())
		payloadChunk = &ChunkState{}
		payloadChunk.Init(newchunk, reusechunk, payloadKnown)
		if payloadChunk.wrid, err = client.CreateDynamicIndex("backup.ppxar.didx"); err != nil {
			return nil, err
		}
		archive.PayloadWriteCB = func(b []byte) error { return payloadChunk.HandleData(b, client) }
		archive.OnPayloadFileStart = func() error { return payloadChunk.SuggestBoundary(client) }
		if changeDetectionMode == pbscommon.ChangeDetectionMetadata {
			// Reuse unchanged files from the previous snapshot without
			// reading them; an unusable previous snapshot means data mode.
			prev, prevTime, lerr := client.LoadPreviousSplitArchive(archive.ArchiveName, "backup.ppxar.didx", prevPayload)
			if lerr != nil {
				fmt.Printf("Metadata change detection: no usable previous snapshot (%v), reading every file\n", lerr)
			} else {
				fmt.Printf("Metadata change detection: comparing with snapshot %s (%d files)\n",
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
	if archive.Reuse != nil {
		st := archive.Reuse.Stats
		fmt.Printf("Metadata change detection: %d unchanged files (%d bytes) reused without reading, %d chunks reused, %d bytes padding; %d unchanged files read anyway to limit padding\n",
			st.Files, st.Bytes, st.Chunks, st.Padding, st.OverPadding)
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

	if m, merr := xattrCollector.FinalizeRaw(); merr != nil {
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
		entries, _, metaErrs := xattrCollector.Stats()
		if blob, serr := combined.Serialize(); serr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to serialize file metadata: %v\n", serr)
		} else if uerr := client.UploadBlob(pbscommon.FileMetaBlobName, blob); uerr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to upload file metadata blob: %v\n", uerr)
		} else {
			fmt.Printf("File metadata: %d entries, %d errors\n", entries, metaErrs)
		}
	}

	sidecar := &pbscommon.BackupSidecar{
		FormatVersion:    1,
		BackupID:         client.Manifest.BackupID,
		Directories:      []string{originalPath},
		GeneratedAt:      time.Now().Unix(),
		SkippedReadError: pbscommon.SkippedToIssues(archive.ReadErrors),
	}
	if sb, serr := json.Marshal(sidecar); serr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to serialize status sidecar: %v\n", serr)
	} else if uerr := client.UploadBlob(pbscommon.BackupStatusFilename, sb); uerr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to upload status sidecar: %v\n", uerr)
	}

	err = client.UploadManifest()
	if err != nil {
		return nil, err
	}
	// archive.ReadErrors lists files that could not be read and were skipped:
	// the snapshot committed but is incomplete. Surfaced as a partial result.
	return archive.ReadErrors, nil
}

func backup(client *pbscommon.PBSClient, newchunk, reusechunk *atomic.Uint64, pxarOut string, backupdir string, usevss bool) ([]string, error) {

	fmt.Printf("Starting backup of %s\n", backupdir)
	originalPath := backupdir
	if abs, aerr := filepath.Abs(backupdir); aerr == nil {
		originalPath = abs
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
	return readErrors, client.Finish()
}
