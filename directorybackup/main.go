package main

import (
	"clientcommon"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"
	"pbscommon"
	"runtime"
	"snapshot"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cornelk/hashmap"
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
	knownChunks        *hashmap.Map[string, bool]
}


func (c *ChunkState) Init(newchunk *atomic.Uint64, reusechunk *atomic.Uint64, knownChunks *hashmap.Map[string, bool]) {
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

			h := sha256.New()
			if _, err := h.Write(c.current_chunk); err != nil {
				return fmt.Errorf("failed to hash chunk: %w", err)
			}
			bindigest := h.Sum(nil)
			shahash := hex.EncodeToString(bindigest)

			if _, ok := c.knownChunks.GetOrInsert(shahash, true); !ok {
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
			if _, err := c.chunkdigests.Write(h.Sum(nil)); err != nil {
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

func (c *ChunkState) Eof(client *pbscommon.PBSClient) error {
	//Here we write the remainder of data for which cyclic hash did not trigger

	if len(c.current_chunk) > 0 {
		h := sha256.New()
		if _, err := h.Write(c.current_chunk); err != nil {
			return fmt.Errorf("failed to hash final chunk: %w", err)
		}

		shahash := hex.EncodeToString(h.Sum(nil))
		if err := binary.Write(c.chunkdigests, binary.LittleEndian, (c.pos + uint64(len(c.current_chunk)))); err != nil {
			return fmt.Errorf("failed to write final chunk offset: %w", err)
		}
		if _, err := c.chunkdigests.Write(h.Sum(nil)); err != nil {
			return fmt.Errorf("failed to write final chunk digest: %w", err)
		}

		if _, ok := c.knownChunks.GetOrInsert(shahash, true); !ok {
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
		}
		os.Exit(1)
	}

	L := clientcommon.Locking{}

	lock_ok := L.AcquireProcessLock()
	if !lock_ok {

		dialog.Error("Backup jobs need to run exclusively, please wait until the previous job has finished")
		os.Exit(2)
	}
	defer L.ReleaseProcessLock()

	insecure := cfg.CertFingerprint != ""

	client := &pbscommon.PBSClient{
		BaseURL:         cfg.BaseURL,
		CertFingerPrint: cfg.CertFingerprint, //"ea:7d:06:f9:87:73:a4:72:d0:e8:05:a4:b3:3d:95:d7:0a:26:dd:6d:5c:ca:e6:99:83:e4:11:3b:5f:10:f4:4b",
		AuthID:          cfg.AuthID,
		Secret:          cfg.Secret,
		Datastore:       cfg.Datastore,
		Namespace:       cfg.Namespace,
		Insecure:        insecure,
		Manifest: pbscommon.BackupManifest{
			BackupID: cfg.BackupID,
		},
	}
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("Failed to retrieve hostname:", err)
		hostname = "unknown"
	}

	begin := time.Now()
	var readErrors []string
	if cfg.BackupSourceDir != "" {
		readErrors, err = backup(client, newchunk, reusechunk, cfg.PxarOut, cfg.BackupSourceDir, cfg.UseVSS)
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

func backup_stream(client *pbscommon.PBSClient, newchunk, reusechunk *atomic.Uint64, filename string, stream io.Reader) error {
	knownChunks := hashmap.New[string, bool]()
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

func backup_real(client *pbscommon.PBSClient, newchunk, reusechunk *atomic.Uint64, pxarOut string, backupdir string) ([]string, error) {
	client.Connect(false, "host")
	knownChunks := hashmap.New[string, bool]()

	archive := &pbscommon.PXARArchive{}
	archive.ArchiveName = "backup.pxar.didx"

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

	if err = pxarChunk.Eof(client); err != nil {
		return nil, err
	}
	if err = pcat1Chunk.Eof(client); err != nil {
		return nil, err
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
	var err error
	var readErrors []string
	if usevss {
		err = snapshot.CreateVSSSnapshot(([]string{backupdir}), func(snaps map[string]snapshot.SnapShot) error {
			// Get first snapshot from map (Go 1.22 compatible)
			for _, snap := range snaps {
				backupdir = snap.FullPath
				break
			}
			//Remove VSS snapshot on windows, on linux for now NOP
			var e error
			readErrors, e = backup_real(client, newchunk, reusechunk, pxarOut, backupdir)
			return e

		})
	} else {
		readErrors, err = backup_real(client, newchunk, reusechunk, pxarOut, backupdir)
	}

	if err != nil {
		return readErrors, err
	}

	// Commit the snapshot even on a partial (read-error) run so the data that
	// was readable is retained; the partial status is reported via readErrors.
	return readErrors, client.Finish()
}
