package main

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"hash"

	"clientcommon"
	"fmt"
	"io"
	"os"
	"pbscommon"
	"runtime"
	"sync/atomic"

	"github.com/cornelk/hashmap"
	"github.com/gen2brain/beeep"
	"github.com/getlantern/systray"
	"github.com/tawesoft/golib/v2/dialog"
	"golang.org/x/sys/windows"
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

type DISK_EXTENT struct {
	DiskNumber     uint32
	StartingOffset int64 // LARGE_INTEGER in C/C++
	ExtentLength   int64 // LARGE_INTEGER in C/C++
}

type VOLUME_DISK_EXTENTS struct {
	NumberOfDiskExtents uint32
	Extents             [16]DISK_EXTENT // This is a placeholder; actual size depends on NumberOfDiskExtents
}

type PARTITION_STYLE uint32

const (
	PartitionStyleMBR PARTITION_STYLE = 0
	PartitionStyleGPT PARTITION_STYLE = 1
)

type DRIVE_LAYOUT_INFORMATION_MBR struct {
	Signature uint32
}

type DRIVE_LAYOUT_INFORMATION_GPT struct {
	DiskId windows.GUID
}

type PARTITION_INFORMATION_MBR struct {
	PartitionType byte
	BootIndicator byte
	BootPartition byte
}

type PARTITION_INFORMATION_GPT struct {
	Guid          windows.GUID
	PartitionName [36]uint16
}

type PARTITION_INFORMATION_EX struct {
	PartitionStyle     PARTITION_STYLE
	StartingOffset     uint64
	PartitionLength    uint64
	PartitionNumber    uint32
	RewritePartition   bool
	IsServicePartition bool
	DUMMYUNIONNAME     struct {
		Mbr PARTITION_INFORMATION_MBR
		Gpt PARTITION_INFORMATION_GPT
	}
}

type DRIVE_LAYOUT_INFORMATION_EX struct {
	PartitionStyle uint32
	PartitionCount uint32
	DUMMYUNIONNAME struct {
		Mbr DRIVE_LAYOUT_INFORMATION_MBR
		Gpt DRIVE_LAYOUT_INFORMATION_GPT
	}
	PartitionEntry [128]PARTITION_INFORMATION_EX
}

const IOCTL_DISK_GET_DRIVE_LAYOUT_EX = 0x00070050

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

func main() {
	var newchunk *atomic.Uint64 = new(atomic.Uint64)
	var reusechunk *atomic.Uint64 = new(atomic.Uint64)
	knownChunks := hashmap.New[string, bool]()
	CS := ChunkState{}
	CS.Init(newchunk, reusechunk, knownChunks)

	cfg := loadConfig()

	/*if ok := cfg.valid(); !ok {
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
	}*/

	//Physycal drive paths will be like  "\\\\.\\PhysicalDrive0"
	parts := make([]Partition, 0)
	if strings.HasPrefix(cfg.BackupDevice, "\\\\.\\PhysicalDrive") {
		volumeHandle, err := syscall.CreateFile(
			syscall.StringToUTF16Ptr(`\\.\C:`), // Example volume C:
			syscall.GENERIC_READ|syscall.GENERIC_WRITE,
			syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
			nil,
			syscall.OPEN_EXISTING,
			0,
			0,
		)
		if err != nil {
			dialog.Error(err.Error())
			panic(err)
		}
		defer syscall.CloseHandle(volumeHandle)
		var volumeDiskExtents DRIVE_LAYOUT_INFORMATION_EX
		var bytesReturned uint32

		// First call to get the required size (if needed)
		// ...

		// Second call with a properly sized buffer
		err = syscall.DeviceIoControl(
			volumeHandle,
			IOCTL_DISK_GET_DRIVE_LAYOUT_EX, // Define this constant
			nil,
			0,
			(*byte)(unsafe.Pointer(&volumeDiskExtents)), // Output buffer
			uint32(unsafe.Sizeof(volumeDiskExtents)),    // Size of output buffer
			&bytesReturned,
			nil,
		)

		if err != nil {
			dialog.Error(err.Error())
			panic(err)
		}
		for i := 0; i < int(volumeDiskExtents.PartitionCount); i++ {
			E := volumeDiskExtents.PartitionEntry[i]
			fmt.Printf("Part: %d %s %s\n", E.PartitionNumber, BytesToString(int64(E.StartingOffset)), BytesToString(int64(E.PartitionLength)))
			parts = append(parts, Partition{
				StartByte:   uint64(E.StartingOffset),
				EndByte:     uint64(E.StartingOffset + E.PartitionLength),
				RequiresVSS: true,
				Skip:        false,
			})
		}
	}

	return

	L := clientcommon.Locking{}

	lock_ok := L.AcquireProcessLock()
	if !lock_ok {

		dialog.Error("Backup jobs need to run exclusively, please wait until the previous job has finished")
		os.Exit(2)
	}
	defer L.ReleaseProcessLock()
	if runtime.GOOS == "windows" {
		go systray.Run(func() {
			systray.SetIcon(clientcommon.ICON)
			systray.SetTooltip("PBSGO Backup running")
			beeep.Notify("Proxmox Backup Go", "Backup started", "")
		},
			func() {

			})
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
	/*hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("Failed to retrieve hostname:", err)
		hostname = "unknown"
	}*/

	//begin := time.Now()
	F, err := os.Open(cfg.BackupDevice)
	if err != nil {
		panic(err)
	}
	pos, err := F.Seek(0, io.SeekEnd)
	if err != nil {
		panic(err)
	}
	total := pos
	_, err = F.Seek(0, io.SeekStart)
	if err != nil {
		panic(err)
	}
	client.Connect(false)
	wrid, err := client.CreateFixedIndex(pbscommon.FixedIndexCreateReq{
		ArchiveName: filepath.Base(cfg.BackupDevice) + ".fidx",
		Size:        total,
	})
	if err != nil {
		panic(err)
	}

	//Blocks are 4MB as per proxmox docs
	block := make([]byte, 4*1024*1024)
	for CS.pos < uint64(total) {
		nread, err := F.Read(block)
		if err != nil {
			panic(err)
		}
		if nread <= 0 {
			panic("Short read")
		}
		h := sha256.New()
		_, err = h.Write(block[:nread])
		if err != nil {
			panic(err)
		}

		shahash := hex.EncodeToString(h.Sum(nil))
		//binary.Write(CS.chunkdigests, binary.LittleEndian, (CS.pos + uint64(nread)))
		CS.chunkdigests.Write(h.Sum(nil))

		_, exists := knownChunks.GetOrInsert(shahash, true)

		if exists {
			reusechunk.Add(1)
		} else {
			err = client.UploadFixedCompressedChunk(wrid, shahash, block[:nread])
			if err != nil {
				panic(err)
			}
		}
		CS.assignments = append(CS.assignments, shahash)
		CS.assignments_offset = append(CS.assignments_offset, CS.pos)
		CS.pos += uint64(nread)
		CS.chunkcount++
		fmt.Printf("Chunk %d\n", CS.chunkcount)
	}

	//Avoid incurring in request entity too large by chunking assignment PUT requests in blocks of at most 128 chunks
	for k := 0; k < len(CS.assignments); k += 128 {
		k2 := k + 128
		if k2 > len(CS.assignments) {
			k2 = len(CS.assignments)
		}
		err = client.AssignFixedChunks(wrid, CS.assignments[k:k2], CS.assignments_offset[k:k2])
		if err != nil {
			panic(err)
		}
	}

	err = client.CloseFixedIndex(wrid, hex.EncodeToString(CS.chunkdigests.Sum(nil)), CS.pos, CS.chunkcount)
	if err != nil {
		panic(err)
	}
	err = client.UploadManifest()
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
