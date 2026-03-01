package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"io"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"clientcommon"
	"fmt"
	"os"
	"pbscommon"
	"runtime"
	"sync/atomic"

	"github.com/cornelk/hashmap"
	"github.com/google/uuid"
	"github.com/tawesoft/golib/v2/dialog"
)

var defaultMailSubjectTemplate = "Backup {{.Status}}"
var defaultMailBodyTemplate = `{{if .Success}}Backup complete ({{.FromattedDuration}})
Chunks New {{.NewChunks}}, Reused {{.ReusedChunks}}.{{else}}Error occurred while working, backup may be not completed.
Last error is: {{.ErrorStr}}{{end}}`

var didxMagic = []byte{28, 145, 78, 165, 25, 186, 179, 205}

const PBS_FIXED_CHUNK_SIZE = 4 * 1024 * 1024

type ChunkState struct {
	assignments        []string
	index_hash_data    map[uint64][]byte
	assignments_offset []uint64
	processed_size     uint64
	wrid               uint64
	chunkcount         uint64
	current_chunk      []byte
	C                  pbscommon.Chunker
	newchunk           *atomic.Uint64
	reusechunk         *atomic.Uint64
	knownChunks        *hashmap.Map[string, bool]
}

type Partition struct {
	StartByte   uint64
	EndByte     uint64
	RequiresVSS bool
	Skip        bool
	Letter      string
}

func (c *ChunkState) Init(newchunk *atomic.Uint64, reusechunk *atomic.Uint64, knownChunks *hashmap.Map[string, bool]) {
	c.assignments = make([]string, 0)
	c.assignments_offset = make([]uint64, 0)
	c.processed_size = 0
	c.chunkcount = 0
	c.index_hash_data = make(map[uint64][]byte)
	c.current_chunk = make([]byte, 0)
	c.C = pbscommon.Chunker{}
	c.C.New(1024 * 1024 * 4)
	c.reusechunk = reusechunk
	c.newchunk = newchunk
	c.knownChunks = knownChunks
}

func BytesToString(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%dKB", b/1024)
	}
	if b < 1024*1024*1024 {
		return fmt.Sprintf("%dMB", b/(1024*1024))
	}

	return fmt.Sprintf("%dGB", b/(1024*1024*1024))

}

func uploadWorker(client *pbscommon.PBSClient, filename string, total_size uint64, ch chan []byte) error {
	var newchunk *atomic.Uint64 = new(atomic.Uint64)
	var reusechunk *atomic.Uint64 = new(atomic.Uint64)
	knownChunks := hashmap.New[string, bool]()

	knownChunks2, err := client.GetKnownSha265FromFIDX(filename)
	if err == nil {
		knownChunks = knownChunks2
	} else {
		fmt.Printf("Cannot get previous: %s\n", err.Error())
	}

	CS := ChunkState{}
	CS.Init(newchunk, reusechunk, knownChunks)
	wrid, err := client.CreateFixedIndex(pbscommon.FixedIndexCreateReq{
		ArchiveName: filename,
		Size:        int64(total_size),
	})
	if err != nil {
		return err
	}

	var assignment_mutex sync.Mutex

	errch := make(chan error)
	digests := make(map[int64][]byte)

	type PosSeg struct {
		Pos  uint64
		Data []byte
	}

	ch2 := make(chan PosSeg)

	workerfn := func() {
		for seg := range ch2 {
			h := sha256.New()
			_, err = h.Write(seg.Data)

			shahash := hex.EncodeToString(h.Sum(nil))
			//binary.Write(CS.chunkdigests, binary.LittleEndian, (CS.pos + uint64(nread)))

			assignment_mutex.Lock()
			CS.index_hash_data[seg.Pos] = h.Sum(nil)
			digests[int64(seg.Pos)] = h.Sum(nil)

			_, exists := knownChunks.GetOrInsert(shahash, true)
			assignment_mutex.Unlock()

			if exists {
				reusechunk.Add(1)
			} else {
				err = client.UploadFixedCompressedChunk(wrid, shahash, seg.Data)
				if err != nil {
					errch <- err
					break
				}

			}
			assignment_mutex.Lock()
			CS.assignments = append(CS.assignments, shahash)
			CS.assignments_offset = append(CS.assignments_offset, seg.Pos)
			CS.processed_size += uint64(len(seg.Data))
			CS.chunkcount++
			if CS.processed_size > total_size {
				errch <- fmt.Errorf("Fatal: tried to backup more data than specified size!")
				break
			}
			fmt.Printf("Chunk %d/%d/%d\n", CS.chunkcount, int(math.Ceil(float64(total_size)/float64(PBS_FIXED_CHUNK_SIZE))), reusechunk.Load())
			assignment_mutex.Unlock()

		}
		errch <- nil
	}

	posfn := func() {
		pos := uint64(0)
		for block := range ch {

			ch2 <- PosSeg{
				Pos:  pos,
				Data: block,
			}
			pos += uint64(len(block))
		}
		close(ch2)
	}

	go posfn()

	for i := 0; i < 8; i++ {
		go workerfn()
	}
	for i := 0; i < 8; i++ {
		err := <-errch
		if err != nil {
			return err
		}
	}

	//Avoid incurring in request entity too large by chunking assignment PUT requests in blocks of at most 128 chunks
	for k := 0; k < len(CS.assignments); k += 128 {
		k2 := k + 128
		if k2 > len(CS.assignments) {
			k2 = len(CS.assignments)
		}
		err = client.AssignFixedChunks(wrid, CS.assignments[k:k2], CS.assignments_offset[k:k2])
		if err != nil {
			return err
		}
	}

	chunkdigests := sha256.New()
	positions := slices.Collect(maps.Keys(CS.index_hash_data))
	slices.Sort(positions)
	for _, P := range positions {
		chunkdigests.Write(CS.index_hash_data[P])
	}

	err = client.CloseFixedIndex(wrid, hex.EncodeToString(chunkdigests.Sum(nil)), CS.processed_size, CS.chunkcount)
	if err != nil {
		return err
	}
	return nil
}

func Slugify(input string) string {
	// Convert to lowercase
	s := strings.ToLower(input)
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "_", "")
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	s = reg.ReplaceAllString(s, "")
	regDash := regexp.MustCompile(`-+`)
	s = regDash.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")

	return s
}

//TODO: Perhaps on linux we could use that https://github.com/datto/dattobd for block devices

func backupFileDevice(client *pbscommon.PBSClient, filename string) error {
	slug := Slugify(filename)

	f, err := os.Open(filename)

	if err != nil {
		return err
	}

	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	ch := make(chan []byte)
	go func() {
		f.Seek(0, io.SeekStart)
		for {
			block := make([]byte, PBS_FIXED_CHUNK_SIZE) //PBS block size is fixed 4MB
			nread, err := f.Read(block)
			if err == io.EOF {
				break
			} else if err != nil {
				panic(err)
			}

			ch <- block[:nread]
		}
		close(ch)
	}()

	return uploadWorker(client, slug+".fidx", uint64(size), ch)
}

type BackupDisk struct {
	Index int
	Size  int64
}

func main() {

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

	if cfg.SysTray {
		sysTraySetup()
	}

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

	//Physical drive paths will be like  "\\\\.\\PhysicalDrive0"
	client.Connect(false, cfg.BackupType)
	disks := make([]BackupDisk, 0)

	for _, dev := range cfg.BackupDevices {
		if strings.HasPrefix(dev, "\\\\.\\PhysicalDrive") {

			re := regexp.MustCompile(`PhysicalDrive(\d+)$`)
			matches := re.FindStringSubmatch(dev)
			idx, _ := strconv.ParseInt(matches[1], 10, 32)
			size, err := backupWindowsDisk(client, int(idx))
			if err != nil {
				panic(err)
			}
			disks = append(disks, BackupDisk{
				Index: int(idx),
				Size:  size,
			})
		} else {
			err := backupFileDevice(client, dev)
			if err != nil {
				panic(err)
			}
		}
	}

	if cfg.BackupType == "vm" {
		type ConfigTemplate struct {
			VMGenId string
			VMID    int64
			VMName  string
			Disks   []BackupDisk
			OS      string
			SMBIOS  string
		}

		tmpl, err := template.New("qemuconfig").Parse(`boot: order=sata0
cores: 4
machine: q35
memory: 2048
name: {{.VMName}}
numa: 0
onboot: 0
ostype: {{.OS}}
scsihw: virtio-scsi-single
smbios1: uuid={{.SMBIOS}}
sockets: 1
{{range .Disks}}
sata{{.Index}}: local:{{.VMID}}/vm-{{.VMID}}-disk-{{.Index}}.raw,cache=writeback,discard=on,iothread=1,size={{.Size}}
{{end}}
vmgenid: {{.VMGenId}}
		`)
		if err != nil {
			panic(err)
		}
		vmid, err := strconv.ParseInt(cfg.BackupID, 10, 32)
		if err != nil {
			panic(err)
		}
		hostname, err := os.Hostname()
		if err != nil {
			panic(err)
		}
		wr := bytes.Buffer{}
		cfgt := ConfigTemplate{
			VMGenId: uuid.New().String(),
			VMID:    vmid,
			Disks:   disks,
			VMName:  hostname,
			SMBIOS:  uuid.New().String(), //TODO extract from real machine
		}
		if runtime.GOOS == "windows" { // TODO Improve
			cfgt.OS = "win11"
		} else {
			cfgt.OS = "l26"
		}
		tmpl.Execute(&wr, cfgt)
		client.UploadBlob("qemu-server.conf.blob", wr.Bytes())
	}

	err := client.UploadManifest()
	if err != nil {
		panic(err)
	}
	client.Finish()

	/*partitions, err := disk.Partitions(false) // false means don't include virtual partitions
	if err != nil {
		log.Fatalf("Error fetching partitions: %v", err)
	}

	// Iterate over partitions and print them
	for _, partition := range partitions {
		// Print partition information
		fmt.Printf("Device: %s\n", partition.Device)
		fmt.Printf("Mountpoint: %s\n", partition.Mountpoint)
		fmt.Printf("Filesystem type: %s\n", partition.Fstype)

		// List the corresponding drive letter for each partition
		// This is platform dependent, but it should map to the drive letter on Windows.
		// Windows typically assigns a drive letter (like C:, D:) to each partition.
		// We use partition.Mountpoint to get it, which should include the letter (e.g. "C:\").
		if partition.Mountpoint != "" {
			fmt.Printf("Drive Letter: %s\n", partition.Mountpoint)
		}
	}

	return

	SNAP := snapshot.CreateVSSSnapshot("C:\\")
	defer snapshot.VSSCleanup()
	fmt.Println("ObjectPath: " + SNAP.ObjectPath)
	file, err := os.Open(strings.TrimRight(SNAP.ObjectPath, "\\"))
	if err != nil {
		panic(err)
	}

	x := make([]byte, 1024)
	n, err := file.Read(x)
	if err != nil {
		panic(err)
	} else {
		fmt.Print(n)
	}*/

	//Windows backup logic will be as follows

	//1. Enumerate fixed non-usb disks ( SATA + NVME )
	//2. Enumerate partitions with offset and length
	//3. Start reading using PhysicalDriveX special file
	//4. If we go into a region that contains a mounted partition, if filesystem is NTFS or ReFS , take VSS snapshot and switch to the associated shadow volume file
	//4. If the partition is not mounted just keep reading, if the partition is mounted and not NTFS or ReFS for now throw a warning and write zeros
	//5. For each disk create a fixed index ( Do it in parallel maybe)

}
