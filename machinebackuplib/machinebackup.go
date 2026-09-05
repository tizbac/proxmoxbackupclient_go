package machinebackuplib

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"math"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"pbscommon"

	"github.com/alphadose/haxmap"
	"github.com/google/uuid"
)

// ProgressCallback function type for reporting progress
// Return true to cancel the backup operation
type ProgressCallback func(percentage float64, message string) bool



var defaultMailSubjectTemplate = "Backup {{.Status}}"
var defaultMailBodyTemplate = `{{if .Success}}Backup complete ({{.FromattedDuration}})
Chunks New {{.NewChunks}}, Reused {{.ReusedChunks}}.{{else}}Error occurred while working, backup may be not completed.
Last error is: {{.ErrorStr}}{{end}}`

var didxMagic = []byte{28, 145, 78, 165, 25, 186, 179, 205}



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
	knownChunks        *haxmap.Map[string, bool]
}

type Partition struct {
	StartByte   uint64
	EndByte     uint64
	RequiresVSS bool
	Skip        bool
	Letter      string
}

func (c *ChunkState) Init(newchunk *atomic.Uint64, reusechunk *atomic.Uint64, knownChunks *haxmap.Map[string, bool]) {
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
	knownChunks := haxmap.New[string, bool]()

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
			if _, err := h.Write(seg.Data); err != nil {
				errch <- fmt.Errorf("failed to hash chunk at position %d: %w", seg.Pos, err)
				break
			}

			shahash := hex.EncodeToString(h.Sum(nil))
			//binary.Write(CS.chunkdigests, binary.LittleEndian, (CS.pos + uint64(nread)))

			assignment_mutex.Lock()
			CS.index_hash_data[seg.Pos] = h.Sum(nil)
			digests[int64(seg.Pos)] = h.Sum(nil)

			_, exists := knownChunks.GetOrSet(shahash, true)
			assignment_mutex.Unlock()

			if exists {
				reusechunk.Add(1)
			} else {
				err = client.UploadFixedCompressedChunk(wrid, shahash, seg.Data)
				if err != nil {
					errch <- fmt.Errorf("failed to upload chunk %s: %w", shahash, err)
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
			percentage := float64(CS.processed_size) / float64(total_size) * 100
fmt.Printf("Chunk %d/%d/%d - Progress: %.2f%%\n", CS.chunkcount, int(math.Ceil(float64(total_size)/float64(pbscommon.PBS_FIXED_CHUNK_SIZE))), reusechunk.Load(), percentage)
			
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
	// Collect map keys (Go 1.22 compatible)
	positions := make([]uint64, 0, len(CS.index_hash_data))
	for pos := range CS.index_hash_data {
		positions = append(positions, pos)
	}
	slices.Sort(positions)
	for _, P := range positions {
		if _, err := chunkdigests.Write(CS.index_hash_data[P]); err != nil {
			return fmt.Errorf("failed to write chunk digest for position %d: %w", P, err)
		}
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

func BackupFileDevice(client *pbscommon.PBSClient, filename string, progressCallback ProgressCallback) error {
	slug := Slugify(filename)

	f, err := os.Open(filename)

	if err != nil {
		return err
	}

	size, err := f.Seek(0, io.SeekEnd)
	var b int64 = 0
	var totread int64 = 0
	if err != nil {
		return err
	}
	ch := make(chan []byte)
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			errCh <- fmt.Errorf("failed to seek to start: %w", err)
			return
		}
		for {
			block := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE) //PBS block size is fixed 4MB
			nread, err := f.Read(block)
			if err == io.EOF {
				break
			} else if err != nil {
				errCh <- fmt.Errorf("failed to read block: %w", err)
				return
			}

			ch <- block[:nread]
			totread = totread + int64(nread)
			progressCallback((float64(totread)/float64(size)), fmt.Sprintf("%s: Block %d", filename, b))
			b++
		}
		errCh <- nil
	}()

	uploadErr := uploadWorker(client, slug+".fidx", uint64(size), ch)
	readErr := <-errCh
	if readErr != nil {
		return readErr
	}
	return uploadErr
}

type BackupDisk struct {
	Index int
	Size  int64
}

// BackupResult represents the result of a backup operation
type BackupResult struct {
	Disks []BackupDisk
}

// Backup performs a machine backup using the provided configuration
func Backup(cfg *Config, progressCallback ProgressCallback) (*BackupResult, error) {
	// Validate configuration
	if !cfg.Valid() {
		return nil, fmt.Errorf("invalid configuration")
	}

	client := &pbscommon.PBSClient{
		BaseURL:         cfg.BaseURL,
		CertFingerPrint: cfg.CertFingerprint, //"ea:7d:06:f9:87:73:a4:72:d0:e8:05:a4:b3:3d:95:d7:0a:26:dd:6d:5c:ca:e6:99:83:e4:11:3b:5f:10:f4:4b",
		AuthID:          cfg.AuthID,
		Secret:          cfg.Secret,
		Username:        cfg.PBSUsername,
		Password:        cfg.PBSPassword,
		Datastore:       cfg.Datastore,
		Namespace:       cfg.Namespace,
		Insecure:        cfg.CertFingerprint != "",
		Manifest: pbscommon.BackupManifest{
			BackupID: cfg.BackupID,
		},
	}
	if client.Username != "" {
		if err := client.ObtainTicket(); err != nil {
			return nil, fmt.Errorf("ticket login failed: %w", err)
		}
	}

	//Physical drive paths will be like  "\\\\.\\PhysicalDrive0"
	client.Connect(false, cfg.BackupType)
	disks := make([]BackupDisk, 0)

	// Calculate total size of all disks
	var totalSize uint64 = 0
	for _, dev := range cfg.BackupDevices {
		if strings.HasPrefix(dev, "\\\\.\\PhysicalDrive") {
			// For physical drives, get the disk size
			re := regexp.MustCompile(`PhysicalDrive(\d+)$`)
			matches := re.FindStringSubmatch(dev)
			idx, _ := strconv.ParseInt(matches[1], 10, 32)
			
			// Get disk size using platform-specific function
			size, err := GetDiskSize(fmt.Sprintf("\\\\.\\PhysicalDrive%d", idx))
			if err != nil {
				return nil, fmt.Errorf("failed to get disk size for %s: %v", dev, err)
			}
			totalSize += uint64(size)
		} else {
			// For file devices, get file size
			info, err := os.Stat(dev)
			if err != nil {
				return nil, fmt.Errorf("failed to get file size for %s: %v", dev, err)
			}
			totalSize += uint64(info.Size())
		}
	}

	// Track progress for each device
	currentProcessedSize := uint64(0)
	
	for _, dev := range cfg.BackupDevices {
		if strings.HasPrefix(dev, "\\\\.\\PhysicalDrive") {
			re := regexp.MustCompile(`PhysicalDrive(\d+)$`)
			matches := re.FindStringSubmatch(dev)
			idx, _ := strconv.ParseInt(matches[1], 10, 32)
			
			size, err := BackupWindowsDisk(client, int(idx), progressCallback)
			if err != nil {
				return nil, fmt.Errorf("backup disk %s %v", dev, err)
			}
			
			disks = append(disks, BackupDisk{
				Index: int(idx),
				Size:  size,
			})
			
			// Update progress for this disk
			currentProcessedSize += uint64(size)
			if progressCallback != nil && totalSize > 0 {
				percentage := float64(currentProcessedSize) / float64(totalSize) * 100
				if progressCallback(percentage, fmt.Sprintf("Backup complete for disk %s", dev)) {
					return nil, fmt.Errorf("backup cancelled by user")
				}
			}
		} else {
			err := BackupFileDevice(client, dev, progressCallback)
			if err != nil {
				return nil, fmt.Errorf("backup device %s %v", dev, err)
			}
			
			// For file devices, get the file size to update progress
			info, err := os.Stat(dev)
			if err != nil {
				return nil, fmt.Errorf("failed to get file size for %s: %v", dev, err)
			}
			
			currentProcessedSize += uint64(info.Size())
			if progressCallback != nil && totalSize > 0 {
				percentage := float64(currentProcessedSize) / float64(totalSize) * 100
				if progressCallback(percentage, fmt.Sprintf("Backup complete for file %s", dev)) {
					return nil, fmt.Errorf("backup cancelled by user")
				}
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
			return nil, fmt.Errorf("parse VM config template %v", err)
		}
		vmid, err := strconv.ParseInt(cfg.BackupID, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parse VM ID %v", err)
		}
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("get hostname: %v", err)
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
		if err := tmpl.Execute(&wr, cfgt); err != nil {
			return nil, fmt.Errorf("execute VM config template: %v", err)
		}
		if err := client.UploadBlob("qemu-server.conf.blob", wr.Bytes()); err != nil {
			return nil, fmt.Errorf("upload VM config blob: %v", err)
		}
	}

	if err := client.UploadManifest(); err != nil {
		return nil, fmt.Errorf("upload manifest: %v", err)
	}
	if err := client.Finish(); err != nil {
		return nil, fmt.Errorf("finish: %v", err)
	}

	
	return &BackupResult{
		Disks: disks,
	}, nil
}

// Helper method to create a default configuration
func NewDefaultConfig() *Config {
	return &Config{
		BackupType: "host",
	}
}