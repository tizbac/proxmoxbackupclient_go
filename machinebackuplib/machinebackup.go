package machinebackuplib

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

// ProgressCallback function type for reporting progress.
// percentage is the fraction of the WHOLE job completed so far (0.0 - 1.0),
// not of a single device, so multi-disk jobs show one continuous bar.
// Return true to cancel the backup operation.
type ProgressCallback func(percentage float64, message string) bool

var (
	errCancelled     = errors.New("backup cancelled by user")
	errUploadAborted = errors.New("upload aborted")
	defaultMailSubjectTemplate = "Backup {{.Status}}"
	defaultMailBodyTemplate    = `{{if .Success}}Backup complete ({{.FromattedDuration}})
Chunks New {{.NewChunks}}, Reused {{.ReusedChunks}}.{{else}}Error occurred while working, backup may be not completed.
Last error is: {{.ErrorStr}}{{end}}`
	didxMagic = []byte{28, 145, 78, 165, 25, 186, 179, 205}
)

type ChunkState struct {
	assignments        []string
	index_hash_data    map[uint64][]byte
	assignments_offset []uint64
	processed_size     uint64
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

// deviceSizeBytes returns the size in bytes of a backup device.
//
// os.Stat alone is not enough: on Linux a block device (and the
// /dev/disk/by-id symlink to it) reports st_size == 0. That made the job-wide
// progress fraction divide by a zero total, collapse to NaN and clamp to 0%,
// pinning the GUI progress bar at 0 for the whole run even though bytes were
// clearly flowing. Regular files use stat; block devices and Windows
// PhysicalDrive paths ask the platform for the real length.
func deviceSizeBytes(dev string) (uint64, error) {
	if strings.HasPrefix(dev, `\\.\PhysicalDrive`) {
		re := regexp.MustCompile(`PhysicalDrive(\d+)$`)
		matches := re.FindStringSubmatch(dev)
		if len(matches) < 2 {
			return 0, fmt.Errorf("invalid physical drive path %q", dev)
		}
		idx, err := strconv.ParseInt(matches[1], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid physical drive index in %q: %w", dev, err)
		}
		size, err := GetDiskSize(fmt.Sprintf(`\\.\PhysicalDrive%d`, idx))
		if err != nil {
			return 0, fmt.Errorf("failed to get disk size for %s: %w", dev, err)
		}
		if size <= 0 {
			return 0, fmt.Errorf("device %s reported a non-positive size (%d)", dev, size)
		}
		return uint64(size), nil
	}

	// Regular files: stat is exact and avoids opening the file.
	if info, err := os.Stat(dev); err == nil && info.Mode().IsRegular() {
		return uint64(info.Size()), nil
	}

	// Block devices / other special files.
	size, err := GetDiskSize(dev)
	if err != nil {
		return 0, fmt.Errorf("failed to get size of %s: %w", dev, err)
	}
	if size <= 0 {
		return 0, fmt.Errorf("device %s reported a non-positive size (%d)", dev, size)
	}
	return uint64(size), nil
}

// jobProgressFraction maps a single device's completion fraction (0.0-1.0) to
// the fraction of the whole job. An unknown total (0 bytes) reports 0 instead
// of a NaN so the caller can never render a bogus percentage.
func jobProgressFraction(baseSize, devSize uint64, fraction float64, totalSize uint64) float64 {
	if totalSize == 0 {
		return 0
	}
	whole := (float64(baseSize) + fraction*float64(devSize)) / float64(totalSize)
	if math.IsNaN(whole) || math.IsInf(whole, 0) {
		return 0
	}
	return whole
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

// uploadWorker streams fixed-size chunks from ch into a PBS fixed index and
// commits it (assign + close) when all data has been processed. readErrCh,
// when non-nil, carries the reader's terminal error (buffered, exactly one
// send): a reader failure or user cancellation makes uploadWorker abort
// WITHOUT committing the index, so a cancelled/partial run never ends up as a
// "complete" backup on the server.
//
// uploadWorker is the ONLY consumer of readErrCh: it is the sole reader of
// that channel and reports the reader's error through its own return value.
// Letting the caller read the channel a second time deadlocks — the value is
// buffered, so whoever arrives first wins, and the loser's receive blocks
// forever once the reader goroutine has exited.
func uploadWorker(client *pbscommon.PBSClient, filename string, total_size uint64, ch chan []byte, readErrCh <-chan error) error {
	var newchunk = new(atomic.Uint64)
	var reusechunk = new(atomic.Uint64)
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
			// Digest of the *plaintext*, in whichever scheme this snapshot
			// uses: plain sha256, or sha256(plaintext || id_key) when an
			// encryption key is configured. This is the value the fixed index
			// stores and the value the chunk store is keyed by, so it must be
			// derived the same way on every restore.
			chunkdigest := client.ChunkDigest(seg.Data)
			shahash := hex.EncodeToString(chunkdigest[:])
			//binary.Write(CS.chunkdigests, binary.LittleEndian, (CS.pos + uint64(nread)))

			assignment_mutex.Lock()
			CS.index_hash_data[seg.Pos] = chunkdigest[:]
			digests[int64(seg.Pos)] = chunkdigest[:]

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
				errch <- fmt.Errorf("fatal: tried to backup more data than specified size")
				break
			}
			if total_size > 0 {
				percentage := float64(CS.processed_size) / float64(total_size) * 100
				if math.IsNaN(percentage) || math.IsInf(percentage, 0) {
					percentage = 0
				}
				fmt.Printf("Chunk %d/%d/%d - Progress: %.2f%%\n", CS.chunkcount, int(math.Ceil(float64(total_size)/float64(pbscommon.PBS_FIXED_CHUNK_SIZE))), reusechunk.Load(), percentage)
			}

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

	// The reader reported an error (or the user cancelled): the data in this
	// fixed index is partial, so leave it unclosed instead of committing it.
	// Consuming readErrCh here — and only here — is what makes the ownership
	// note on uploadWorker hold; see also the deterministic-ordering note on
	// the reader goroutine in BackupFileDevice.
	if readErrCh != nil {
		select {
		case rerr := <-readErrCh:
			if rerr != nil {
				return rerr
			}
		default:
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

// Slugify turns a device path into the name of the block archive that stores
// it, i.e. the .fidx file is called Slugify(<device>) + ".fidx".
//
// The name has to be stable: it is what identifies the archive inside a
// snapshot, and every backup of the same device must land on the same name.
//
// Everything outside [a-z0-9] is dropped, runs of '-' are collapsed into a
// single '-' and leading/trailing '-' are trimmed.
func Slugify(input string) string {
	s := strings.ToLower(input)
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "_", "")
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	s = reg.ReplaceAllString(s, "")
	regDash := regexp.MustCompile(`-+`)
	// Collapse the run down to a single '-'. Replacing with "" would delete
	// the dashes outright, which also made the Trim below dead code.
	s = regDash.ReplaceAllString(s, "-")
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
	// Buffered so the reader goroutine never blocks on the send, and so the
	// single value is guaranteed to be sitting in the buffer by the time
	// uploadWorker's non-blocking receive runs: the value is sent before the
	// deferred close(ch), and uploadWorker can only finish draining ch once
	// close(ch) has happened.
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		var rerr error
		defer func() { errCh <- rerr }()
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			rerr = fmt.Errorf("failed to seek to start: %w", err)
			return
		}
		for {
			block := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE) //PBS block size is fixed 4MB
			nread, err := f.Read(block)
			if err == io.EOF {
				break
			} else if err != nil {
				rerr = fmt.Errorf("failed to read block: %w", err)
				break
			}

			ch <- block[:nread]
			totread = totread + int64(nread)
			// Returning true stops the backup (user pressed Stop).
			if progressCallback != nil {
				var pct float64
				if size <= 0 {
					pct = 0
				} else {
					pct = float64(totread) / float64(size)
					if math.IsNaN(pct) || math.IsInf(pct, 0) {
						pct = 0
					}
				}
				if progressCallback(pct, fmt.Sprintf("%s: Block %d", filename, b)) {
					rerr = errCancelled
					break
				}
			}
			b++
		}
	}()

	// uploadWorker owns errCh and surfaces both the upload error and the
	// reader error through its return value, so there is deliberately no
	// second receive here.
	return uploadWorker(client, slug+".fidx", uint64(size), ch, errCh)
}

type BackupDisk struct {
	Index int
	Size  int64
	GPT   bool // disk carries a GPT, so a VM booting it needs UEFI (OVMF)
}

// diskHasGPT reports whether the device starts with a GPT: the "EFI PART"
// header sits in LBA 1, which is byte 512 on 512-byte-sector disks and byte
// 4096 on 4Kn ones. Failure to read counts as "no", i.e. the BIOS default.
func diskHasGPT(dev string) bool {
	f, err := os.Open(dev)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 8192)
	if _, err := io.ReadFull(f, buf); err != nil {
		return false
	}
	return gptSignatureIn(buf)
}

func gptSignatureIn(head []byte) bool {
	for _, off := range []int{512, 4096} {
		if len(head) >= off+8 && string(head[off:off+8]) == "EFI PART" {
			return true
		}
	}
	return false
}

// bootDiskIsGPT looks at sata0, the disk the generated config boots from:
// the lowest-indexed backed-up disk.
func bootDiskIsGPT(disks []BackupDisk) bool {
	best := -1
	for i, d := range disks {
		if best < 0 || d.Index < disks[best].Index {
			best = i
		}
	}
	return best >= 0 && disks[best].GPT
}

// BackupResult represents the result of a backup operation
type BackupResult struct {
	Disks []BackupDisk
}

// qemuConfigData is the input to qemuConfigTemplate.
type qemuConfigData struct {
	VMGenId string
	VMID    int64
	VMName  string
	Disks   []BackupDisk
	OS      string
	SMBIOS  string
	UEFI    bool
}

// VMID is not a field of BackupDisk, so inside the {{range .Disks}} block it
// has to be reached through $ (the root data) rather than through dot.
//
// The "#qmdump#map:<drive>:<devname>:<storage>:<format>:" line per disk is what
// makes Proxmox VE restore the disk image at all: PVE's restore only allocates
// and fills drives declared by these lines (found 2026-10-02: without them a
// restore ends "TASK OK" in a second, copies the config verbatim and writes no
// data). Storage is left empty so the storage chosen in the restore dialog
// applies; PVE falls back to "local" (no images) if none is chosen. The sata
// line is rewritten by PVE with the newly allocated volume.
//
// UEFI adds "bios: ovmf" for GPT boot disks (verified 2026-10-03: a Windows
// GPT disk gives "no bootable device" under the default SeaBIOS and boots
// under OVMF). No efidisk0 is written: PVE's restore only fills drives that
// have an image in the archive, and without one PVE starts OVMF with a
// temporary efivars disk, which is enough for Windows to boot.
var qemuConfigTemplate = template.Must(template.New("qemuconfig").Parse(`{{if .UEFI}}bios: ovmf
{{end}}boot: order=sata0
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
sata{{.Index}}: local:{{$.VMID}}/vm-{{$.VMID}}-disk-{{.Index}}.raw,cache=writeback,discard=on,size={{.Size}}
#qmdump#map:sata{{.Index}}:drive-sata{{.Index}}::raw:
{{end}}
vmgenid: {{.VMGenId}}
`))

func renderQemuConfig(data qemuConfigData) ([]byte, error) {
	var wr bytes.Buffer
	if err := qemuConfigTemplate.Execute(&wr, data); err != nil {
		return nil, fmt.Errorf("execute VM config template: %v", err)
	}
	return wr.Bytes(), nil
}

// Backup performs a machine backup using the provided configuration
func Backup(cfg *Config, progressCallback ProgressCallback) (*BackupResult, error) {
	// Validate configuration
	if !cfg.Valid() {
		return nil, fmt.Errorf("invalid configuration")
	}
	// A "vm" backup needs a numeric VMID for the generated VM config; reject a
	// bad ID now rather than after every disk has been transferred.
	if cfg.BackupType == "vm" {
		if _, err := strconv.ParseInt(cfg.BackupID, 10, 32); err != nil {
			return nil, fmt.Errorf("backup type \"vm\" needs a numeric VM ID (e.g. 100) as the backup ID, got %q: use a numeric ID, or use backup type \"host\" if a VM-type snapshot is not required", cfg.BackupID)
		}
	}

	client := &pbscommon.PBSClient{
		BaseURL:         cfg.BaseURL,
		CertFingerPrint: cfg.CertFingerprint, //"ea:7d:06:f9:87:73:a4:72:d0:e8:05:a4:b3:3d:95:d7:0a:26:dd:6d:5c:ca:e6:99:83:e4:11:3b:5f:10:f4:4b",
		AuthID:          cfg.AuthID,
		Secret:          cfg.Secret,
		Username:        cfg.PBSUsername,
		Password:        cfg.PBSPassword,
		Ticket:          cfg.Ticket,
		CSRFToken:       cfg.CSRFToken,
		Datastore:       cfg.Datastore,
		Namespace:       cfg.Namespace,
		Insecure:        cfg.CertFingerprint != "",
		// Nil for a plain snapshot, in which case pbscommon keeps the
		// pre-encryption sha256 + magic/CRC32 framing.
		Crypt: cfg.Crypt,
		Manifest: pbscommon.BackupManifest{
			BackupID: cfg.BackupID,
		},
	}
	// A pre-obtained session ticket (GUI login) wins; otherwise exchange
	// username/password for one.
	if client.Ticket == "" && client.Username != "" {
		if err := client.ObtainTicket(); err != nil {
			return nil, fmt.Errorf("ticket login failed: %w", err)
		}
	}
	if progressCallback != nil {
		if progressCallback(0, "Connecting to Proxmox Backup Server...") {
			return nil, errCancelled
		}
	}

	//Physical drive paths will be like  "\\\\.\\PhysicalDrive0"
	client.Connect(false, cfg.BackupType)
	disks := make([]BackupDisk, 0)

	// Calculate total size of all disks
	var totalSize uint64 = 0
	sizes := make([]uint64, len(cfg.BackupDevices))
	for i, dev := range cfg.BackupDevices {
		size, err := deviceSizeBytes(dev)
		if err != nil {
			return nil, err
		}
		sizes[i] = size
		totalSize += size
	}

	// Track progress for each device
	currentProcessedSize := uint64(0)

	for i, dev := range cfg.BackupDevices {
		// Wrap the job-wide callback so a device can keep reporting its own
		// 0..1 read fraction and it maps to the fraction of the whole job.
		baseSize := currentProcessedSize
		devSize := sizes[i]
		deviceCallback := func(fraction float64, message string) bool {
			if progressCallback == nil {
				return false
			}
			return progressCallback(jobProgressFraction(baseSize, devSize, fraction, totalSize), message)
		}
		if strings.HasPrefix(dev, "\\\\.\\PhysicalDrive") {
			re := regexp.MustCompile(`PhysicalDrive(\d+)$`)
			matches := re.FindStringSubmatch(dev)
			idx, _ := strconv.ParseInt(matches[1], 10, 32)

			isGPT := diskHasGPT(dev)
			size, err := BackupWindowsDisk(client, int(idx), deviceCallback)
			if err != nil {
				return nil, fmt.Errorf("backup disk %s %v", dev, err)
			}

			disks = append(disks, BackupDisk{
				Index: int(idx),
				Size:  size,
				GPT:   isGPT,
			})

			currentProcessedSize += uint64(size)
			if progressCallback != nil && totalSize > 0 {
				pct := jobProgressFraction(baseSize, devSize, 1.0, totalSize)
				if progressCallback(pct, fmt.Sprintf("Backup complete for disk %s", dev)) {
					return nil, errCancelled
				}
			}
		} else {
			// On Linux a whole block device (e.g. /dev/sda) is backed up as a
			// consistent, stitched full-disk image (partition table + every
			// partition, mounted ones snapshotted). Anything else falls back
			// to a plain raw read of the device/file.
			isGPT := diskHasGPT(dev)
			handled, size, err := backupWholeDisk(client, dev, i, cfg.UseSnapshot, deviceCallback)
			if err != nil {
				return nil, fmt.Errorf("backup device %s: %v", dev, err)
			}
			if handled {
				disks = append(disks, BackupDisk{
					Index: i,
					Size:  size,
					GPT:   isGPT,
				})
			} else if err := BackupFileDevice(client, dev, deviceCallback); err != nil {
				return nil, fmt.Errorf("backup device %s: %v", dev, err)
			}

			// Update progress: the whole-disk size when handled, otherwise the file size
			processed := int64(0)
			if handled {
				processed = size
			} else {
				ps, perr := deviceSizeBytes(dev)
				if perr == nil {
					processed = int64(ps)
				}
			}
			currentProcessedSize += uint64(processed)
			if progressCallback != nil && totalSize > 0 {
				pct := jobProgressFraction(baseSize, devSize, 1.0, totalSize)
				if progressCallback(pct, fmt.Sprintf("Backup complete for device %s", dev)) {
					return nil, errCancelled
				}
			}
		}
	}

	if cfg.BackupType == "vm" {
		vmid, err := strconv.ParseInt(cfg.BackupID, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parse VM ID %v", err)
		}
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("get hostname: %v", err)
		}
		cfgt := qemuConfigData{
			VMGenId: uuid.New().String(),
			VMID:    vmid,
			Disks:   disks,
			VMName:  hostname,
			SMBIOS:  uuid.New().String(), //TODO extract from real machine
			UEFI:    bootDiskIsGPT(disks),
		}
		if runtime.GOOS == "windows" { // TODO Improve
			cfgt.OS = "win11"
		} else {
			cfgt.OS = "l26"
		}
		wr, err := renderQemuConfig(cfgt)
		if err != nil {
			return nil, err
		}
		if err := client.UploadBlob("qemu-server.conf.blob", wr); err != nil {
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
