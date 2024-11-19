package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cornelk/hashmap"
	"github.com/gen2brain/beeep"
	"github.com/getlantern/systray"
	"github.com/tawesoft/golib/v2/dialog"
)



var defaultMailSubjectTemplate = "Backup {{.Status}}"
var defaultMailBodyTemplate = `{{if .Success}}Backup complete ({{.FromattedDuration}})
Chunks New {{.NewChunks}}, Reused {{.ReusedChunks}}.{{else}}Error occurred while working, backup may be not completed.
Last error is: {{.ErrorStr}}{{end}}`

var didxMagic = []byte{28, 145, 78, 165, 25, 186, 179, 205}

type ChunkState struct {
	assignments        []string
	assignments_offset []uint64
	pos                uint64
	wrid               uint64
	chunkcount         uint64
	chunkdigests       hash.Hash
	current_chunk      []byte
	C                  Chunker
}

type DidxEntry struct {
	offset uint64
	digest []byte
}

func (c *ChunkState) Init() {
	c.assignments = make([]string, 0)
	c.assignments_offset = make([]uint64, 0)
	c.pos = 0
	c.chunkcount = 0
	c.chunkdigests = sha256.New()
	c.current_chunk = make([]byte, 0)
	c.C = Chunker{}
	c.C.New(1024 * 1024 * 4)
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

	L := Locking{}

	
	lock_ok := L.AcquireProcessLock()
	if !lock_ok {
		
		dialog.Error("Backup jobs need to run exclusively, please wait until the previous job has finished")
		os.Exit(2)
	}
	defer L.ReleaseProcessLock()

	go systray.Run(func() {
		systray.SetIcon(ICON)
		systray.SetTooltip("PBSGO Backup running")
		beeep.Notify("Proxmox Backup Go", "Backup started", "")
	},
		func() {

		})
	

	insecure := cfg.CertFingerprint != ""

	client := &PBSClient{
		baseurl:         cfg.BaseURL,
		certfingerprint: cfg.CertFingerprint, //"ea:7d:06:f9:87:73:a4:72:d0:e8:05:a4:b3:3d:95:d7:0a:26:dd:6d:5c:ca:e6:99:83:e4:11:3b:5f:10:f4:4b",
		authid:          cfg.AuthID,
		secret:          cfg.Secret,
		datastore:       cfg.Datastore,
		namespace:       cfg.Namespace,
		insecure:        insecure,
		manifest: BackupManifest{
			BackupID: cfg.BackupID,
		},
	}
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("Failed to retrieve hostname:", err)
		hostname = "unknown"
	}

	begin := time.Now()
	if cfg.BackupSourceDir != "" {
		err = backup(client, newchunk, reusechunk, cfg.PxarOut, cfg.BackupSourceDir)
	} else if cfg.BackupStreamName != "" {
		sn := cfg.BackupStreamName
		if ! strings.HasSuffix(sn, ".didx" ) {
			sn += ".didx"
		}
		fmt.Printf("Backing up from STDIN to %s", sn)
		err = backup_stream(client, newchunk, reusechunk, sn, os.Stdin )

	}else{
		panic("No backup dir or stream name specified, exiting")
	}

	
	end := time.Now()

	mailCtx := mailCtx{
		NewChunks:    newchunk.Load(),
		ReusedChunks: reusechunk.Load(),
		Error:        err,
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
	msg, err = mailCtx.buildStr(mailBodyTemplate)
	if err != nil {
		fmt.Println("Cannot use custom mail body: " + err.Error())
		msg, err = mailCtx.buildStr(defaultMailBodyTemplate)
		if err != nil {
			// this should never happen
			panic(err)
		}
	}
	if runtime.GOOS == "windows" {
		systray.Quit()
		beeep.Notify("Proxmox Backup Go", msg, "")
	}
	if cfg.SMTP != nil {
		var subject string

		mailSubjectTemplate := defaultMailSubjectTemplate
		if cfg.SMTP.Template != nil && cfg.SMTP.Template.Subject != "" {
			mailSubjectTemplate = cfg.SMTP.Template.Subject
		}

		subject, err = mailCtx.buildStr(mailSubjectTemplate)
		if err != nil {
			fmt.Println("Cannot use custom mail subject: " + err.Error())
			msg, err = mailCtx.buildStr(defaultMailSubjectTemplate)
			if err != nil {
				// this should never happen
				panic(err)
			}
		}
		client, err := setupClient(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.Insecure)
		if err != nil {
			fmt.Println("Cannot connect to mail server: " + err.Error())
			os.Exit(1)
		}
		defer client.Quit()
		for _, ccc := range cfg.SMTP.Mails {
			err = sendMail(ccc.From, ccc.To, subject, msg, client)
			if err != nil {
				fmt.Println("Cannot send email: " + err.Error())
				os.Exit(1)
			}
		}
	}

}

func backup_stream(client *PBSClient, newchunk, reusechunk *atomic.Uint64, filename string, stream io.Reader ) error {
	knownChunks := hashmap.New[string, bool]()
	client.Connect(false)
	previousDidx, err := client.DownloadPreviousToBytes(filename)
	if err != nil {
		return err
	}

	fmt.Printf("Downloaded previous DIDX: %d bytes\n", len(previousDidx))

	if !bytes.HasPrefix(previousDidx, didxMagic) {
		fmt.Printf("Previous index has wrong magic (%s)!\n", previousDidx[:8])

	} else {
		//Header as per proxmox documentation is fixed size of 4096 bytes,
		//then offset of type uint64 and sha256 digests follow , so 40 byte each record until EOF
		previousDidx = previousDidx[4096:]
		for i := 0; i*40 < len(previousDidx); i += 1 {
			e := DidxEntry{}
			e.offset = binary.LittleEndian.Uint64(previousDidx[i*40 : i*40+8])
			e.digest = previousDidx[i*40+8 : i*40+40]
			shahash := hex.EncodeToString(e.digest)
			fmt.Printf("Previous: %s\n", shahash)
			knownChunks.Set(shahash, true)
		}
	}

	fmt.Printf("Known chunks: %d!\n", knownChunks.Len())

	streamChunk := ChunkState{}
	streamChunk.Init()

	streamChunk.wrid, err = client.CreateDynamicIndex(filename)
	if err != nil {
		return err
	}
	B := make([]byte, 65536)
	for {
		
		n, err := stream.Read(B)
		
		b := B[:n]
		chunkpos := streamChunk.C.Scan(b)

		if chunkpos == 0 {
			streamChunk.current_chunk = append(streamChunk.current_chunk, b...)
		}

		for chunkpos > 0 {
			streamChunk.current_chunk = append(streamChunk.current_chunk, b[:chunkpos]...)

			h := sha256.New()
			// TODO: error handling inside callback
			h.Write(streamChunk.current_chunk)
			bindigest := h.Sum(nil)
			shahash := hex.EncodeToString(bindigest)

			if _, ok := knownChunks.GetOrInsert(shahash, true); !ok {
				fmt.Printf("New chunk[%s] %d bytes\n", shahash, len(streamChunk.current_chunk))
				newchunk.Add(1)

				client.UploadCompressedChunk(streamChunk.wrid, shahash, streamChunk.current_chunk)
			} else {
				fmt.Printf("Reuse chunk[%s] %d bytes\n", shahash, len(streamChunk.current_chunk))
				reusechunk.Add(1)
			}

			// TODO: error handling inside callback
			binary.Write(streamChunk.chunkdigests, binary.LittleEndian, (streamChunk.pos + uint64(len(streamChunk.current_chunk))))
			// TODO: error handling inside callback
			streamChunk.chunkdigests.Write(h.Sum(nil))

			streamChunk.assignments_offset = append(streamChunk.assignments_offset, streamChunk.pos)
			streamChunk.assignments = append(streamChunk.assignments, shahash)
			streamChunk.pos += uint64(len(streamChunk.current_chunk))
			streamChunk.chunkcount += 1

			streamChunk.current_chunk = b[chunkpos:]
			chunkpos = streamChunk.C.Scan(b[chunkpos:])
		}
		if err == io.EOF {
			break
		}
	}


	if len(streamChunk.current_chunk) > 0 {
		h := sha256.New()
		_, err = h.Write(streamChunk.current_chunk)
		if err != nil {
			return err
		}

		shahash := hex.EncodeToString(h.Sum(nil))
		binary.Write(streamChunk.chunkdigests, binary.LittleEndian, (streamChunk.pos + uint64(len(streamChunk.current_chunk))))
		streamChunk.chunkdigests.Write(h.Sum(nil))

		if _, ok := knownChunks.GetOrInsert(shahash, true); !ok {
			fmt.Printf("New chunk[%s] %d bytes\n", shahash, len(streamChunk.current_chunk))
			client.UploadCompressedChunk(streamChunk.wrid, shahash, streamChunk.current_chunk)
			newchunk.Add(1)
		} else {
			fmt.Printf("Reuse chunk[%s] %d bytes\n", shahash, len(streamChunk.current_chunk))
			reusechunk.Add(1)
		}
		streamChunk.assignments_offset = append(streamChunk.assignments_offset, streamChunk.pos)
		streamChunk.assignments = append(streamChunk.assignments, shahash)
		streamChunk.pos += uint64(len(streamChunk.current_chunk))
		streamChunk.chunkcount += 1

	}

	//Avoid incurring in request entity too large by chunking assignment PUT requests in blocks of at most 128 chunks
	for k := 0; k < len(streamChunk.assignments); k += 128 {
		k2 := k + 128
		if k2 > len(streamChunk.assignments) {
			k2 = len(streamChunk.assignments)
		}
		client.AssignChunks(streamChunk.wrid, streamChunk.assignments[k:k2], streamChunk.assignments_offset[k:k2])
	}

	client.CloseDynamicIndex(streamChunk.wrid, hex.EncodeToString(streamChunk.chunkdigests.Sum(nil)), streamChunk.pos, streamChunk.chunkcount)

	err = client.UploadManifest()
	if err != nil {
		return err
	}

	return client.Finish()
}

func backup(client *PBSClient, newchunk, reusechunk *atomic.Uint64, pxarOut string, backupdir string) error {
	knownChunks := hashmap.New[string, bool]()

	fmt.Printf("Starting backup of %s\n", backupdir)

	backupdir = createVSSSnapshot(backupdir)
	//Remove VSS snapshot on windows, on linux for now NOP
	defer VSSCleanup()

	client.Connect(false)

	archive := &PXARArchive{}
	archive.archivename = "backup.pxar.didx"

	previousDidx, err := client.DownloadPreviousToBytes(archive.archivename)
	if err != nil {
		return err
	}

	fmt.Printf("Downloaded previous DIDX: %d bytes\n", len(previousDidx))

	/*f2, _ := os.Create("test.didx")
	defer f2.Close()

	f2.Write(previous_didx)*/

	/*
		Here we download the previous dynamic index to figure out which chunks are the same of what
		we are going to upload to avoid unnecessary traffic and compression cpu usage
	*/

	if !bytes.HasPrefix(previousDidx, didxMagic) {
		fmt.Printf("Previous index has wrong magic (%s)!\n", previousDidx[:8])

	} else {
		//Header as per proxmox documentation is fixed size of 4096 bytes,
		//then offset of type uint64 and sha256 digests follow , so 40 byte each record until EOF
		previousDidx = previousDidx[4096:]
		for i := 0; i*40 < len(previousDidx); i += 1 {
			e := DidxEntry{}
			e.offset = binary.LittleEndian.Uint64(previousDidx[i*40 : i*40+8])
			e.digest = previousDidx[i*40+8 : i*40+40]
			shahash := hex.EncodeToString(e.digest)
			fmt.Printf("Previous: %s\n", shahash)
			knownChunks.Set(shahash, true)
		}
	}

	fmt.Printf("Known chunks: %d!\n", knownChunks.Len())
	f := &os.File{}
	if pxarOut != "" {
		f, err = os.Create(pxarOut)
		if err != nil {
			return err
		}
		defer f.Close()
	}
	/**/

	pxarChunk := ChunkState{}
	pxarChunk.Init()

	pcat1Chunk := ChunkState{}
	pcat1Chunk.Init()

	pxarChunk.wrid, err = client.CreateDynamicIndex(archive.archivename)
	if err != nil {
		return err
	}
	pcat1Chunk.wrid, err = client.CreateDynamicIndex("catalog.pcat1.didx")
	if err != nil {
		return err
	}

	archive.writeCB = func(b []byte) {
		chunkpos := pxarChunk.C.Scan(b)

		if chunkpos == 0 {
			pxarChunk.current_chunk = append(pxarChunk.current_chunk, b...)
		}

		for chunkpos > 0 {
			pxarChunk.current_chunk = append(pxarChunk.current_chunk, b[:chunkpos]...)

			h := sha256.New()
			// TODO: error handling inside callback
			h.Write(pxarChunk.current_chunk)
			bindigest := h.Sum(nil)
			shahash := hex.EncodeToString(bindigest)

			if _, ok := knownChunks.GetOrInsert(shahash, true); !ok {
				fmt.Printf("New chunk[%s] %d bytes\n", shahash, len(pxarChunk.current_chunk))
				newchunk.Add(1)

				client.UploadCompressedChunk(pxarChunk.wrid, shahash, pxarChunk.current_chunk)
			} else {
				fmt.Printf("Reuse chunk[%s] %d bytes\n", shahash, len(pxarChunk.current_chunk))
				reusechunk.Add(1)
			}

			// TODO: error handling inside callback
			binary.Write(pxarChunk.chunkdigests, binary.LittleEndian, (pxarChunk.pos + uint64(len(pxarChunk.current_chunk))))
			// TODO: error handling inside callback
			pxarChunk.chunkdigests.Write(h.Sum(nil))

			pxarChunk.assignments_offset = append(pxarChunk.assignments_offset, pxarChunk.pos)
			pxarChunk.assignments = append(pxarChunk.assignments, shahash)
			pxarChunk.pos += uint64(len(pxarChunk.current_chunk))
			pxarChunk.chunkcount += 1

			pxarChunk.current_chunk = b[chunkpos:]
			chunkpos = pxarChunk.C.Scan(b[chunkpos:])
		}

		if pxarOut != "" {
			// TODO: error handling inside callback
			f.Write(b)
		}
		//
	}

	archive.catalogWriteCB = func(b []byte) {
		chunkpos := pcat1Chunk.C.Scan(b)

		if chunkpos == 0 {
			pcat1Chunk.current_chunk = append(pcat1Chunk.current_chunk, b...)
		}

		var lastChunkPos uint64
		for chunkpos > 0 {
			pcat1Chunk.current_chunk = append(pcat1Chunk.current_chunk, b[:chunkpos]...)

			h := sha256.New()
			h.Write(pcat1Chunk.current_chunk)
			shahash := hex.EncodeToString(h.Sum(nil))

			fmt.Printf("Catalog: New chunk[%s] %d bytes, pos %d\n", shahash, len(pcat1Chunk.current_chunk), chunkpos)

			client.UploadCompressedChunk(pcat1Chunk.wrid, shahash, pcat1Chunk.current_chunk)
			binary.Write(pcat1Chunk.chunkdigests, binary.LittleEndian, (pcat1Chunk.pos + uint64(len(pcat1Chunk.current_chunk))))
			pcat1Chunk.chunkdigests.Write(h.Sum(nil))

			pcat1Chunk.assignments_offset = append(pcat1Chunk.assignments_offset, pcat1Chunk.pos)
			pcat1Chunk.assignments = append(pcat1Chunk.assignments, shahash)
			pcat1Chunk.pos += uint64(len(pcat1Chunk.current_chunk))
			pcat1Chunk.chunkcount += 1

			pcat1Chunk.current_chunk = b[chunkpos:]

			//lastChunkPos is here so we know when pcat1Chunk.C.Scan loops from beginnning.
			lastChunkPos = chunkpos
			chunkpos = pcat1Chunk.C.Scan(b[chunkpos:])

			if chunkpos < lastChunkPos {
				break
			}
		}
	}

	//This is the entry point of backup job which will start streaming with the PCAT and PXAR write callback
	//Data to be hashed and eventuall uploaded

	archive.WriteDir(backupdir, "", true)

	//Here we write the remainder of data for which cyclic hash did not trigger

	if len(pxarChunk.current_chunk) > 0 {
		h := sha256.New()
		_, err = h.Write(pxarChunk.current_chunk)
		if err != nil {
			return err
		}

		shahash := hex.EncodeToString(h.Sum(nil))
		binary.Write(pxarChunk.chunkdigests, binary.LittleEndian, (pxarChunk.pos + uint64(len(pxarChunk.current_chunk))))
		pxarChunk.chunkdigests.Write(h.Sum(nil))

		if _, ok := knownChunks.GetOrInsert(shahash, true); !ok {
			fmt.Printf("New chunk[%s] %d bytes\n", shahash, len(pxarChunk.current_chunk))
			client.UploadCompressedChunk(pxarChunk.wrid, shahash, pxarChunk.current_chunk)
			newchunk.Add(1)
		} else {
			fmt.Printf("Reuse chunk[%s] %d bytes\n", shahash, len(pxarChunk.current_chunk))
			reusechunk.Add(1)
		}
		pxarChunk.assignments_offset = append(pxarChunk.assignments_offset, pxarChunk.pos)
		pxarChunk.assignments = append(pxarChunk.assignments, shahash)
		pxarChunk.pos += uint64(len(pxarChunk.current_chunk))
		pxarChunk.chunkcount += 1

	}

	if len(pcat1Chunk.current_chunk) > 0 {
		h := sha256.New()
		_, err = h.Write(pcat1Chunk.current_chunk)
		if err != nil {
			return err
		}

		shahash := hex.EncodeToString(h.Sum(nil))
		binary.Write(pcat1Chunk.chunkdigests, binary.LittleEndian, (pcat1Chunk.pos + uint64(len(pcat1Chunk.current_chunk))))
		pcat1Chunk.chunkdigests.Write(h.Sum(nil))

		fmt.Printf("Catalog: New chunk[%s] %d bytes\n", shahash, len(pcat1Chunk.current_chunk))
		pcat1Chunk.assignments_offset = append(pcat1Chunk.assignments_offset, pcat1Chunk.pos)
		pcat1Chunk.assignments = append(pcat1Chunk.assignments, shahash)
		pcat1Chunk.pos += uint64(len(pcat1Chunk.current_chunk))
		pcat1Chunk.chunkcount += 1
		client.UploadCompressedChunk(pcat1Chunk.wrid, shahash, pcat1Chunk.current_chunk)
	}

	//Avoid incurring in request entity too large by chunking assignment PUT requests in blocks of at most 128 chunks
	for k := 0; k < len(pxarChunk.assignments); k += 128 {
		k2 := k + 128
		if k2 > len(pxarChunk.assignments) {
			k2 = len(pxarChunk.assignments)
		}
		client.AssignChunks(pxarChunk.wrid, pxarChunk.assignments[k:k2], pxarChunk.assignments_offset[k:k2])
	}

	client.CloseDynamicIndex(pxarChunk.wrid, hex.EncodeToString(pxarChunk.chunkdigests.Sum(nil)), pxarChunk.pos, pxarChunk.chunkcount)

	for k := 0; k < len(pcat1Chunk.assignments); k += 128 {
		k2 := k + 128
		if k2 > len(pcat1Chunk.assignments) {
			k2 = len(pcat1Chunk.assignments)
		}
		client.AssignChunks(pcat1Chunk.wrid, pcat1Chunk.assignments[k:k2], pcat1Chunk.assignments_offset[k:k2])
	}

	client.CloseDynamicIndex(pcat1Chunk.wrid, hex.EncodeToString(pcat1Chunk.chunkdigests.Sum(nil)), pcat1Chunk.pos, pcat1Chunk.chunkcount)

	err = client.UploadManifest()
	if err != nil {
		return err
	}

	return client.Finish()
}
