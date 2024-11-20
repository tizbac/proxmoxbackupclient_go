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
	newchunk *atomic.Uint64 
	reusechunk *atomic.Uint64
	knownChunks *hashmap.Map[string, bool]
}

type DidxEntry struct {
	offset uint64
	digest []byte
}

func (c *ChunkState) Init(newchunk *atomic.Uint64 , reusechunk *atomic.Uint64, knownChunks *hashmap.Map[string, bool] ) {
	c.assignments = make([]string, 0)
	c.assignments_offset = make([]uint64, 0)
	c.pos = 0
	c.chunkcount = 0
	c.chunkdigests = sha256.New()
	c.current_chunk = make([]byte, 0)
	c.C = Chunker{}
	c.C.New(1024 * 1024 * 4)
	c.reusechunk = reusechunk
	c.newchunk = newchunk
	c.knownChunks = knownChunks
}

func (c *ChunkState) HandleData(b []byte, client *PBSClient){
	chunkpos := c.C.Scan(b)

	if chunkpos == 0 {
		//No break happened, just append data 
		c.current_chunk = append(c.current_chunk, b...)
	} else {

		for chunkpos > 0 {
			//Append data until break position
			c.current_chunk = append(c.current_chunk, b[:chunkpos]...)

			h := sha256.New()
			// TODO: error handling inside callback
			h.Write(c.current_chunk)
			bindigest := h.Sum(nil)
			shahash := hex.EncodeToString(bindigest)

			if _, ok := c.knownChunks.GetOrInsert(shahash, true); !ok {
				fmt.Printf("New chunk[%s] %d bytes\n", shahash, len(c.current_chunk))
				c.newchunk.Add(1)

				client.UploadCompressedChunk(c.wrid, shahash, c.current_chunk)
			} else {
				fmt.Printf("Reuse chunk[%s] %d bytes\n", shahash, len(c.current_chunk))
				c.reusechunk.Add(1)
			}

			// TODO: error handling inside callback
			binary.Write(c.chunkdigests, binary.LittleEndian, (c.pos + uint64(len(c.current_chunk))))
			// TODO: error handling inside callback
			c.chunkdigests.Write(h.Sum(nil))

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
}

func (c *ChunkState) Eof(client *PBSClient) {
	//Here we write the remainder of data for which cyclic hash did not trigger
	
	if len(c.current_chunk) > 0 {
		h := sha256.New()
		_, err := h.Write(c.current_chunk)
		if err != nil {
			panic(err)
		}

		shahash := hex.EncodeToString(h.Sum(nil))
		binary.Write(c.chunkdigests, binary.LittleEndian, (c.pos + uint64(len(c.current_chunk))))
		c.chunkdigests.Write(h.Sum(nil))

		if _, ok := c.knownChunks.GetOrInsert(shahash, true); !ok {
			fmt.Printf("New chunk[%s] %d bytes\n", shahash, len(c.current_chunk))
			client.UploadCompressedChunk(c.wrid, shahash, c.current_chunk)
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
		client.AssignChunks(c.wrid, c.assignments[k:k2], c.assignments_offset[k:k2])
	}

	client.CloseDynamicIndex(c.wrid, hex.EncodeToString(c.chunkdigests.Sum(nil)), c.pos, c.chunkcount)
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
	if runtime.GOOS == "windows" {
		go systray.Run(func() {
			systray.SetIcon(ICON)
			systray.SetTooltip("PBSGO Backup running")
			beeep.Notify("Proxmox Backup Go", "Backup started", "")
		},
			func() {

			})
	}
	

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
	streamChunk.Init(newchunk, reusechunk, knownChunks)

	streamChunk.wrid, err = client.CreateDynamicIndex(filename)
	if err != nil {
		return err
	}
	B := make([]byte, 65536)
	for {
		
		n, err := stream.Read(B)
		
		b := B[:n]
		
		streamChunk.HandleData(b, client)

		if err == io.EOF {
			break
		}
	}

	streamChunk.Eof(client)

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
	pxarChunk.Init(newchunk, reusechunk, knownChunks)

	pcat1Chunk := ChunkState{}
	pcat1Chunk.Init(newchunk, reusechunk, knownChunks)

	pxarChunk.wrid, err = client.CreateDynamicIndex(archive.archivename)
	if err != nil {
		return err
	}
	pcat1Chunk.wrid, err = client.CreateDynamicIndex("catalog.pcat1.didx")
	if err != nil {
		return err
	}

	archive.writeCB = func(b []byte) {
		

		if pxarOut != "" {
			// TODO: error handling inside callback
			f.Write(b)
		}

		pxarChunk.HandleData(b, client)

		//
	}

	archive.catalogWriteCB = func(b []byte) {
		pcat1Chunk.HandleData(b, client)
	}

	//This is the entry point of backup job which will start streaming with the PCAT and PXAR write callback
	//Data to be hashed and eventuall uploaded

	archive.WriteDir(backupdir, "", true)

	
	pxarChunk.Eof(client)
	pcat1Chunk.Eof(client)

	

	err = client.UploadManifest()
	if err != nil {
		return err
	}

	return client.Finish()
}
