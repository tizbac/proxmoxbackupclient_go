package main

import (
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"regexp"
	"slices"
	"snapshot"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

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
	index_hash_data    map[uint64][]byte
	assignments_offset []uint64
	pos                uint64
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
	c.pos = 0
	c.chunkcount = 0
	c.index_hash_data = make(map[uint64][]byte)
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
	CheckSum  uint32
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
	Partitionordinal   uint16
	StartingOffset     uint64
	PartitionLength    uint64
	PartitionNumber    uint32
	RewritePartition   bool
	IsServicePartition bool
	Padding            [112]byte
	/*DUMMYUNIONNAME     struct {
		Mbr PARTITION_INFORMATION_MBR // 31
		Gpt PARTITION_INFORMATION_GPT // 72
	}*/
}

type GET_LENGTH_INFORMATION struct {
	Length int64
}

type DRIVE_LAYOUT_INFORMATION_EX struct {
	PartitionStyle uint32
	PartitionCount uint32
	/*DUMMYUNIONNAME struct {
		Mbr DRIVE_LAYOUT_INFORMATION_MBR
		Gpt DRIVE_LAYOUT_INFORMATION_GPT
	}*/
	PlaceHolder    [36]byte
	PartitionEntry [128]PARTITION_INFORMATION_EX
}

const IOCTL_DISK_GET_DRIVE_LAYOUT_EX = 0x00070050
const IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS = 0x00560000
const IOCTL_DISK_GET_LENGTH_INFO = 0x0007405C

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

var (
	modkernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procFindFirstVolumeW             = modkernel32.NewProc("FindFirstVolumeW")
	procFindNextVolumeW              = modkernel32.NewProc("FindNextVolumeW")
	procFindVolumeClose              = modkernel32.NewProc("FindVolumeClose")
	procGetVolumePathNamesForVolumeW = modkernel32.NewProc("GetVolumePathNamesForVolumeNameW")
)

type VolumeLetterAssign struct {
	DiskNumber int32
	Offset     uint64
	Letters    []string
}

func enumVolumeDiskOffset() ([]VolumeLetterAssign, error) {
	ret := make([]VolumeLetterAssign, 0)
	volumeName := make([]uint16, windows.MAX_PATH)

	r1, _, _ := procFindFirstVolumeW.Call(
		uintptr(unsafe.Pointer(&volumeName[0])),
		uintptr(len(volumeName)),
	)
	if r1 == 0 {
		return ret, nil
	}
	findHandle := windows.Handle(r1)
	defer procFindVolumeClose.Call(uintptr(findHandle))

	for {
		volName := windows.UTF16ToString(volumeName)

		fmt.Println(volName)

		hVol, err := windows.CreateFile(
			windows.StringToUTF16Ptr(volName[:len(volName)-1]), // remove trailing '\'
			windows.GENERIC_READ,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
			nil,
			windows.OPEN_EXISTING,
			0,
			0,
		)
		if err == nil {
			buffer := make([]byte, 1024)
			buffer2 := make([]uint16, 1024)
			var bytesReturned uint32

			err := windows.DeviceIoControl(
				hVol,
				IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS,
				nil,
				0,
				&buffer[0],
				uint32(len(buffer)),
				&bytesReturned,
				nil,
			)
			if err == nil {

				extents := (*VOLUME_DISK_EXTENTS)(unsafe.Pointer(&buffer[0]))

				for i := uint32(0); i < extents.NumberOfDiskExtents; i++ {
					var returnLength uint32
					extent := (*DISK_EXTENT)(unsafe.Pointer(
						uintptr(unsafe.Pointer(&extents.Extents[0])) +
							uintptr(i)*unsafe.Sizeof(DISK_EXTENT{}),
					))

					v := VolumeLetterAssign{
						DiskNumber: int32(extent.DiskNumber),
						Offset:     uint64(extent.StartingOffset),
						Letters:    make([]string, 0),
					}

					r1, _, _ := procGetVolumePathNamesForVolumeW.Call(
						uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(volName))),
						uintptr(unsafe.Pointer(&buffer2[0])),
						uintptr(len(buffer2)),
						uintptr(unsafe.Pointer(&returnLength)),
					)

					if r1 == 0 {
						return ret, nil
					}

					i := 0
					for i < len(buffer) && buffer[i] != 0 {
						start := i
						for buffer[i] != 0 {
							i++
						}
						path := windows.UTF16ToString(buffer2[start:i])
						v.Letters = append(v.Letters, path)
						i++
					}

					ret = append(ret, v)

				}

			} else {
				fmt.Printf("%s : %s\n", volName, err.Error())
			}

			//checkVolumeExtents(hVol, volName, partitionOffset)
			windows.CloseHandle(hVol)
		}

		ret, _, _ := procFindNextVolumeW.Call(
			uintptr(findHandle),
			uintptr(unsafe.Pointer(&volumeName[0])),
			uintptr(len(volumeName)),
		)
		if ret == 0 {
			break
		}
	}
	return ret, nil
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

	type UploadSeg struct {
		WRID    uint64
		Shahash string
		Block   []byte
	}
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
			digests[int64(CS.pos)] = h.Sum(nil)

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
			CS.pos += uint64(len(seg.Data))
			CS.chunkcount++
			fmt.Printf("Chunk %d/%d/%d\n", CS.chunkcount, total_size/(4*1024*1024), reusechunk.Load())
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

	err = client.CloseFixedIndex(wrid, hex.EncodeToString(chunkdigests.Sum(nil)), CS.pos, CS.chunkcount)
	if err != nil {
		return err
	}
	return nil
}

func GetDiskLength(path string) (int64, error) {
	// Open the device (e.g., \\.\PhysicalDrive0 or \\.\C:)
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(path),
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("CreateFile failed: %w", err)
	}
	defer windows.CloseHandle(handle)

	var lengthInfo GET_LENGTH_INFORMATION
	var bytesReturned uint32

	err = windows.DeviceIoControl(
		handle,
		IOCTL_DISK_GET_LENGTH_INFO,
		nil,
		0,
		(*byte)(unsafe.Pointer(&lengthInfo)),
		uint32(unsafe.Sizeof(lengthInfo)),
		&bytesReturned,
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("DeviceIoControl failed: %w", err)
	}

	return lengthInfo.Length, nil
}

func backupWindowsDisk(client *pbscommon.PBSClient, index int) error {
	parts := make([]Partition, 0)
	ch := make(chan []byte)
	diskdev := fmt.Sprintf("\\\\.\\PhysicalDrive%d", index)
	volumeHandle, err := syscall.CreateFile(
		syscall.StringToUTF16Ptr(diskdev), // Example volume C:
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

	vols, err := enumVolumeDiskOffset()
	if err != nil {
		dialog.Error(err.Error())
		panic(err)
	}
	/*var exts VOLUME_DISK_EXTENTS
	err = syscall.DeviceIoControl(
		volumeHandle,
		IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS,
		nil,
		0,
		(*byte)(unsafe.Pointer(&exts)),
		uint32(unsafe.Sizeof(exts)),
		&bytesReturned,
		nil,
	)

	if err != nil {
		dialog.Error(err.Error())
		panic(err)
	}*/

	for i := 0; i < int(volumeDiskExtents.PartitionCount); i++ {
		E := volumeDiskExtents.PartitionEntry[i]
		fmt.Printf("Part: %d %s %s\n", E.PartitionNumber, BytesToString(int64(E.StartingOffset)), BytesToString(int64(E.PartitionLength)))
		var letter string = ""
		/*for x := 0; x < int(exts.NumberOfDiskExtents); x++ {
			V := exts.Extents[x]
			if V.StartingOffset == int64(E.StartingOffset) {
				fmt.Printf("Found volume, need VSS")
			}
		}*/

		for _, V := range vols {
			if V.DiskNumber == int32(index) && V.Offset == E.StartingOffset {
				if len(V.Letters) > 0 {
					letter = V.Letters[0]
				}

			}
		}

		parts = append(parts, Partition{
			StartByte:   uint64(E.StartingOffset),
			EndByte:     uint64(E.StartingOffset + E.PartitionLength),
			RequiresVSS: letter != "",
			Skip:        false,
			Letter:      letter,
		})
	}

	snapshot_paths := make([]string, 0)

	for _, p := range parts {
		if p.RequiresVSS {
			snapshot_paths = append(snapshot_paths, fmt.Sprintf("%s:\\\\", p.Letter))
		}
	}

	total, err := GetDiskLength(diskdev)
	if err != nil {
		return err
	}

	return snapshot.CreateVSSSnapshot(snapshot_paths, func(snapshots map[string]snapshot.SnapShot) error {

		/*hostname, err := os.Hostname()
		if err != nil {
			fmt.Println("Failed to retrieve hostname:", err)
			hostname = "unknown"
		}*/

		/*parts = append([]Partition{{
			StartByte:   0,
			EndByte:     parts[0].StartByte,
			RequiresVSS: false,
			Letter:      "",
			Skip:        false,
		}}, parts...)*/

		newparts := make([]Partition, 0)
		var curpos uint64 = 0
		for _, P := range parts {
			if P.StartByte != curpos { //Add a fake partition to backup raw data between
				newparts = append(newparts, Partition{
					StartByte:   curpos,
					EndByte:     P.StartByte,
					RequiresVSS: false,
					Letter:      "",
					Skip:        false,
				})
			}
			newparts = append(newparts, P)
			curpos = P.EndByte
		}
		if curpos < uint64(total) {
			newparts = append(newparts, Partition{
				StartByte:   curpos,
				EndByte:     uint64(total),
				RequiresVSS: false,
				Letter:      "",
				Skip:        false,
			})
		}

		parts = newparts

		fmt.Printf("%+v\n", parts)

		//begin := time.Now()
		F, err := os.Open(diskdev)
		if err != nil {
			panic(err)
		}

		//Blocks are 4MB as per proxmox docs
		go func() {
			buffer := make([]byte, 0)
			for idx, P := range parts {
				fmt.Printf("Partition: %d\n", idx)
				if !P.RequiresVSS {
					F.Seek(int64(P.StartByte), io.SeekStart)
					block := make([]byte, 4*1024*1024)
					pos := P.StartByte
					for pos < P.EndByte {
						nbytes, err := F.Read(block[:min(uint64(len(block)), P.EndByte-pos)])
						if err != nil {
							panic(err)
						}
						buffer = append(buffer, block[:nbytes]...)

						if len(buffer) >= 4*1024*1024 {
							ch <- buffer[:4*1024*1024]
							buffer = buffer[4*1024*1024:]
						}
						pos += uint64(nbytes)
					}
				} else {
					snap, ok := snapshots[P.Letter+":\\"]
					if !ok {
						panic(fmt.Errorf("Cannot find snapshot for letter %s", P.Letter))
					}
					snapshot_file, err := os.Open(strings.TrimRight(snap.ObjectPath, "\\"))
					if err != nil {
						panic(err)
					}
					defer snapshot_file.Close()
					block := make([]byte, 4*1024*1024)
					for {
						nbytes, err := snapshot_file.Read(block)
						if err == io.EOF {
							break
						}
						if err != nil {
							panic(err)
						}
						buffer = append(buffer, block[:nbytes]...)
						if len(buffer) >= 4*1024*1024 {
							ch <- buffer[:4*1024*1024]
							buffer = buffer[4*1024*1024:]
						}
					}
				}
			}
			if len(buffer) > 0 {
				if len(buffer) < 4*1024*1024 {
					buffer = append(buffer, make([]byte, 4*1024*1024-len(buffer))...) //Pad to make
				}
				ch <- buffer
			}

			close(ch)
		}()

		return uploadWorker(client, fmt.Sprintf("windisk%d.fidx", index), uint64(total), ch)

	})
}

func backupFileDevice(filename string) error {
	return nil
}

func main() {

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

	//Physycal drive paths will be like  "\\\\.\\PhysicalDrive0"
	client.Connect(false)
	if strings.HasPrefix(cfg.BackupDevice, "\\\\.\\PhysicalDrive") {

		re := regexp.MustCompile(`PhysicalDrive(\d+)$`)
		matches := re.FindStringSubmatch(cfg.BackupDevice)
		idx, _ := strconv.ParseInt(matches[1], 10, 32)
		err := backupWindowsDisk(client, int(idx))
		if err != nil {
			panic(err)
		}
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
