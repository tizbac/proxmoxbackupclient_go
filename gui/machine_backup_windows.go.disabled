//go:build windows
// +build windows

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"pbscommon"
	"regexp"
	"snapshot"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"

	"crypto/sha256"
	"encoding/hex"
	"math"
	"slices"
	"sync"

	"github.com/cornelk/hashmap"
	"golang.org/x/sys/windows"
)

type DISK_EXTENT struct {
	DiskNumber     uint32
	StartingOffset int64
	ExtentLength   int64
}

type VOLUME_DISK_EXTENTS struct {
	NumberOfDiskExtents uint32
	Extents             [16]DISK_EXTENT
}

type PARTITION_STYLE uint32

const (
	PartitionStyleMBR PARTITION_STYLE = 0
	PartitionStyleGPT PARTITION_STYLE = 1
)

type PARTITION_INFORMATION_EX struct {
	PartitionStyle     PARTITION_STYLE
	Partitionordinal   uint16
	StartingOffset     uint64
	PartitionLength    uint64
	PartitionNumber    uint32
	RewritePartition   bool
	IsServicePartition bool
	Padding            [112]byte
}

type GET_LENGTH_INFORMATION struct {
	Length int64
}

type DRIVE_LAYOUT_INFORMATION_EX struct {
	PartitionStyle uint32
	PartitionCount uint32
	PlaceHolder    [36]byte
	PartitionEntry [128]PARTITION_INFORMATION_EX
}

const IOCTL_DISK_GET_DRIVE_LAYOUT_EX = 0x00070050
const IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS = 0x00560000
const IOCTL_DISK_GET_LENGTH_INFO = 0x0007405C

var (
	modkernel32                      = windows.NewLazySystemDLL("kernel32.dll")
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

type Partition struct {
	StartByte   uint64
	EndByte     uint64
	RequiresVSS bool
	Skip        bool
	Letter      string
}

// PhysicalDiskInfo contains information about a physical disk
type PhysicalDiskInfo struct {
	DiskNumber int      `json:"diskNumber"`
	SizeBytes  int64    `json:"sizeBytes"`
	SizeText   string   `json:"sizeText"`
	Letters    []string `json:"letters"`
	Label      string   `json:"label"`
	Path       string   `json:"path"`
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

		hVol, err := windows.CreateFile(
			windows.StringToUTF16Ptr(volName[:len(volName)-1]),
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
			}

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

func GetDiskLength(path string) (int64, error) {
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

// ListPhysicalDisks returns a list of available physical disks with their information
func ListPhysicalDisks() ([]PhysicalDiskInfo, error) {
	writeDebugLog("Enumerating physical disks...")

	// Get volume to disk mapping
	vols, err := enumVolumeDiskOffset()
	if err != nil {
		return nil, fmt.Errorf("Failed to enumerate volumes: %v", err)
	}

	// Group letters by disk number
	diskLetters := make(map[int][]string)
	for _, v := range vols {
		diskNum := int(v.DiskNumber)
		for _, letter := range v.Letters {
			// Extract just the drive letter (e.g., "C:" from "C:\")
			letter = strings.TrimRight(letter, "\\")
			if !contains(diskLetters[diskNum], letter) {
				diskLetters[diskNum] = append(diskLetters[diskNum], letter)
			}
		}
	}

	// Try to enumerate disks 0-9
	disks := make([]PhysicalDiskInfo, 0)
	for i := 0; i < 10; i++ {
		diskPath := fmt.Sprintf("\\\\.\\PhysicalDrive%d", i)
		size, err := GetDiskLength(diskPath)
		if err != nil {
			// Disk doesn't exist or can't be accessed
			continue
		}

		letters := diskLetters[i]
		if letters == nil {
			letters = []string{}
		}
		sort.Strings(letters)

		// Build label
		label := fmt.Sprintf("Disque %d", i)
		if len(letters) > 0 {
			label += fmt.Sprintf(" (%s)", strings.Join(letters, ", "))
		}
		label += fmt.Sprintf(" - %s", BytesToString(size))

		disks = append(disks, PhysicalDiskInfo{
			DiskNumber: i,
			SizeBytes:  size,
			SizeText:   BytesToString(size),
			Letters:    letters,
			Label:      label,
			Path:       diskPath,
		})

		writeDebugLog(fmt.Sprintf("Found: %s", label))
	}

	return disks, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

type MachineChunkState struct {
	assignments        []string
	indexHashData      map[uint64][]byte
	assignmentsOffset  []uint64
	processedSize      uint64
	wrid               uint64
	chunkcount         uint64
	currentChunk       []byte
	newchunk           *atomic.Uint64
	reusechunk         *atomic.Uint64
	knownChunks        *hashmap.Map[string, bool]
}

func (c *MachineChunkState) Init(newchunk *atomic.Uint64, reusechunk *atomic.Uint64, knownChunks *hashmap.Map[string, bool]) {
	c.assignments = make([]string, 0)
	c.assignmentsOffset = make([]uint64, 0)
	c.processedSize = 0
	c.chunkcount = 0
	c.indexHashData = make(map[uint64][]byte)
	c.currentChunk = make([]byte, 0)
	c.reusechunk = reusechunk
	c.newchunk = newchunk
	c.knownChunks = knownChunks
}

func uploadWorker(client *pbscommon.PBSClient, filename string, totalSize uint64, ch chan []byte, progress func(float64, string)) error {
	var newchunk *atomic.Uint64 = new(atomic.Uint64)
	var reusechunk *atomic.Uint64 = new(atomic.Uint64)
	knownChunks := hashmap.New[string, bool]()

	knownChunks2, err := client.GetKnownSha265FromFIDX(filename)
	if err == nil {
		knownChunks = knownChunks2
		writeDebugLog(fmt.Sprintf("Loaded %d known chunks from previous backup", knownChunks.Len()))
	} else {
		writeDebugLog(fmt.Sprintf("No previous backup found: %v", err))
	}

	CS := MachineChunkState{}
	CS.Init(newchunk, reusechunk, knownChunks)
	wrid, err := client.CreateFixedIndex(pbscommon.FixedIndexCreateReq{
		ArchiveName: filename,
		Size:        int64(totalSize),
	})
	if err != nil {
		return err
	}

	var assignmentMutex sync.Mutex

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
			_, _ = h.Write(seg.Data)

			shahash := hex.EncodeToString(h.Sum(nil))

			assignmentMutex.Lock()
			CS.indexHashData[seg.Pos] = h.Sum(nil)
			digests[int64(seg.Pos)] = h.Sum(nil)

			_, exists := knownChunks.GetOrInsert(shahash, true)
			assignmentMutex.Unlock()

			if exists {
				reusechunk.Add(1)
			} else {
				err = client.UploadFixedCompressedChunk(wrid, shahash, seg.Data)
				if err != nil {
					errch <- err
					break
				}
				newchunk.Add(1)
			}

			assignmentMutex.Lock()
			CS.assignments = append(CS.assignments, shahash)
			CS.assignmentsOffset = append(CS.assignmentsOffset, seg.Pos)
			CS.processedSize += uint64(len(seg.Data))
			CS.chunkcount++

			// Update progress
			percent := float64(CS.processedSize) / float64(totalSize)
			totalChunks := int(math.Ceil(float64(totalSize) / float64(pbscommon.PBS_FIXED_CHUNK_SIZE)))
			msg := fmt.Sprintf("Chunk %d/%d (New: %d, Reused: %d)", CS.chunkcount, totalChunks, newchunk.Load(), reusechunk.Load())
			if progress != nil {
				progress(0.1+percent*0.85, msg)
			}

			if CS.processedSize > totalSize {
				errch <- fmt.Errorf("Fatal: tried to backup more data than specified size!")
				break
			}
			assignmentMutex.Unlock()
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

	// Assign chunks
	for k := 0; k < len(CS.assignments); k += 128 {
		k2 := k + 128
		if k2 > len(CS.assignments) {
			k2 = len(CS.assignments)
		}
		err = client.AssignFixedChunks(wrid, CS.assignments[k:k2], CS.assignmentsOffset[k:k2])
		if err != nil {
			return err
		}
	}

	chunkdigests := sha256.New()
	positions := make([]uint64, 0, len(CS.indexHashData))
	for k := range CS.indexHashData {
		positions = append(positions, k)
	}
	slices.Sort(positions)
	for _, P := range positions {
		_, _ = chunkdigests.Write(CS.indexHashData[P])
	}

	err = client.CloseFixedIndex(wrid, hex.EncodeToString(chunkdigests.Sum(nil)), CS.processedSize, CS.chunkcount)
	if err != nil {
		return err
	}
	return nil
}

func backupWindowsDisk(client *pbscommon.PBSClient, index int, progress func(float64, string)) (int64, error) {
	writeDebugLog(fmt.Sprintf("Starting backup of PhysicalDrive%d", index))

	parts := make([]Partition, 0)
	ch := make(chan []byte)
	diskdev := fmt.Sprintf("\\\\.\\PhysicalDrive%d", index)

	volumeHandle, err := syscall.CreateFile(
		syscall.StringToUTF16Ptr(diskdev),
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("Failed to open %s: %v", diskdev, err)
	}
	defer syscall.CloseHandle(volumeHandle)

	var volumeDiskExtents DRIVE_LAYOUT_INFORMATION_EX
	var bytesReturned uint32

	err = syscall.DeviceIoControl(
		volumeHandle,
		IOCTL_DISK_GET_DRIVE_LAYOUT_EX,
		nil,
		0,
		(*byte)(unsafe.Pointer(&volumeDiskExtents)),
		uint32(unsafe.Sizeof(volumeDiskExtents)),
		&bytesReturned,
		nil,
	)

	if err != nil {
		return 0, fmt.Errorf("Failed to get drive layout: %v", err)
	}

	vols, err := enumVolumeDiskOffset()
	if err != nil {
		return 0, fmt.Errorf("Failed to enumerate volumes: %v", err)
	}

	for i := 0; i < int(volumeDiskExtents.PartitionCount); i++ {
		E := volumeDiskExtents.PartitionEntry[i]
		if E.PartitionNumber == 0 {
			continue
		}
		writeDebugLog(fmt.Sprintf("Partition %d: offset=%s, length=%s",
			E.PartitionNumber, BytesToString(int64(E.StartingOffset)), BytesToString(int64(E.PartitionLength))))

		var letter string = ""
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

	snapshotPaths := make([]string, 0)
	for _, p := range parts {
		if p.RequiresVSS {
			snapshotPaths = append(snapshotPaths, fmt.Sprintf("%s:\\\\", p.Letter))
		}
	}

	total, err := GetDiskLength(diskdev)
	if err != nil {
		return 0, err
	}

	writeDebugLog(fmt.Sprintf("Total disk size: %s", BytesToString(total)))

	return total, snapshot.CreateVSSSnapshot(snapshotPaths, func(snapshots map[string]snapshot.SnapShot) error {
		// Fill gaps between partitions
		newparts := make([]Partition, 0)
		var curpos uint64 = 0
		for _, P := range parts {
			if P.StartByte != curpos {
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

		F, err := os.Open(diskdev)
		if err != nil {
			return fmt.Errorf("Failed to open disk: %v", err)
		}
		defer F.Close()

		go func() {
			buffer := make([]byte, 0)
			for idx, P := range parts {
				writeDebugLog(fmt.Sprintf("Processing partition %d: %s to %s",
					idx, BytesToString(int64(P.StartByte)), BytesToString(int64(P.EndByte))))

				if !P.RequiresVSS {
					_, err := F.Seek(int64(P.StartByte), io.SeekStart)
					if err != nil {
						writeDebugLog(fmt.Sprintf("Failed to seek: %v", err))
						close(ch)
						return
					}

					block := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
					pos := P.StartByte
					for pos < P.EndByte {
						nbytes, err := F.Read(block[:min(uint64(len(block)), P.EndByte-pos)])
						if err != nil {
							writeDebugLog(fmt.Sprintf("Failed to read: %v", err))
							close(ch)
							return
						}
						buffer = append(buffer, block[:nbytes]...)

						if len(buffer) >= pbscommon.PBS_FIXED_CHUNK_SIZE {
							ch <- buffer[:pbscommon.PBS_FIXED_CHUNK_SIZE]
							buffer = buffer[pbscommon.PBS_FIXED_CHUNK_SIZE:]
						}
						pos += uint64(nbytes)
					}
					if pos != P.EndByte {
						writeDebugLog(fmt.Sprintf("Failed to read partition entirely %d/%d", pos, P.EndByte))
					}
				} else {
					snap, ok := snapshots[P.Letter+":\\"]
					if !ok {
						writeDebugLog(fmt.Sprintf("Cannot find snapshot for letter %s", P.Letter))
						close(ch)
						return
					}

					snapshotFile, err := os.Open(strings.TrimRight(snap.ObjectPath, "\\"))
					if err != nil {
						writeDebugLog(fmt.Sprintf("Failed to open snapshot: %v", err))
						close(ch)
						return
					}
					defer snapshotFile.Close()

					pos := P.StartByte
					l, err := GetDiskLength(strings.TrimRight(snap.ObjectPath, "\\"))
					if err != nil {
						writeDebugLog(fmt.Sprintf("Failed to get snapshot length: %v", err))
						close(ch)
						return
					}

					if uint64(P.EndByte) != uint64(P.StartByte)+uint64(l) {
						log.Printf("VSS snapshot is smaller than partition, will pad with zeros")
					}

					npad := P.EndByte - (uint64(P.StartByte) + uint64(l))
					block := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)

					for {
						nbytes, err := snapshotFile.Read(block)
						if err == io.EOF {
							if pos != P.EndByte {
								npad = P.EndByte - pos
							}
							break
						}
						if pos >= P.EndByte {
							writeDebugLog("Fatal: Went outside partition space")
							close(ch)
							return
						}
						if err != nil {
							writeDebugLog(fmt.Sprintf("Failed to read snapshot: %v", err))
							close(ch)
							return
						}
						pos += uint64(nbytes)
						buffer = append(buffer, block[:nbytes]...)
						if len(buffer) >= pbscommon.PBS_FIXED_CHUNK_SIZE {
							ch <- buffer[:pbscommon.PBS_FIXED_CHUNK_SIZE]
							buffer = buffer[pbscommon.PBS_FIXED_CHUNK_SIZE:]
						}
					}

					// Padding
					block = make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
					for npad > 0 {
						sl := block[:min(pbscommon.PBS_FIXED_CHUNK_SIZE, npad)]
						buffer = append(buffer, sl...)
						pos += uint64(len(sl))
						if len(buffer) >= pbscommon.PBS_FIXED_CHUNK_SIZE {
							ch <- buffer[:pbscommon.PBS_FIXED_CHUNK_SIZE]
							buffer = buffer[pbscommon.PBS_FIXED_CHUNK_SIZE:]
						}
						npad -= uint64(len(sl))
					}
					if pos != P.EndByte {
						writeDebugLog(fmt.Sprintf("Failed to read partition entirely %d/%d", pos, P.EndByte))
					}
				}
			}

			// Flush remaining buffer
			for len(buffer) > 0 {
				if len(buffer) > pbscommon.PBS_FIXED_CHUNK_SIZE {
					ch <- buffer[:pbscommon.PBS_FIXED_CHUNK_SIZE]
					buffer = buffer[pbscommon.PBS_FIXED_CHUNK_SIZE:]
				} else {
					ch <- buffer
					buffer = buffer[:0]
				}
			}

			close(ch)
		}()

		return uploadWorker(client, fmt.Sprintf("drive-sata%d.img.fidx", index), uint64(total), ch, progress)
	})
}

// RunMachineBackup performs a full physical disk backup
func RunMachineBackup(opts BackupOptions) error {
	writeDebugLog("Starting machine backup")

	// Validate options
	if opts.BaseURL == "" || opts.AuthID == "" || opts.Secret == "" {
		return fmt.Errorf("PBS connection parameters required")
	}

	if len(opts.BackupDirs) == 0 {
		return fmt.Errorf("At least one physical drive required")
	}

	// Create PBS client
	client := &pbscommon.PBSClient{
		BaseURL:         opts.BaseURL,
		CertFingerPrint: opts.CertFingerprint,
		AuthID:          opts.AuthID,
		Secret:          opts.Secret,
		Datastore:       opts.Datastore,
		Namespace:       opts.Namespace,
		Insecure:        opts.CertFingerprint != "",
		Manifest: pbscommon.BackupManifest{
			BackupID: opts.BackupID,
		},
	}

	progress := func(pct float64, msg string) {
		writeDebugLog(fmt.Sprintf("Machine backup progress: %.1f%% - %s", pct*100, msg))
		if opts.OnProgress != nil {
			opts.OnProgress(pct, msg)
		}
	}

	progress(0.05, "Connecting to PBS...")
	client.Connect(false, "vm")

	// Parse and backup each physical drive
	for _, dev := range opts.BackupDirs {
		if !strings.HasPrefix(dev, "\\\\.\\PhysicalDrive") {
			return fmt.Errorf("Invalid physical drive path: %s", dev)
		}

		re := regexp.MustCompile(`PhysicalDrive(\d+)$`)
		matches := re.FindStringSubmatch(dev)
		if len(matches) < 2 {
			return fmt.Errorf("Failed to parse drive number from: %s", dev)
		}

		idx, err := strconv.ParseInt(matches[1], 10, 32)
		if err != nil {
			return fmt.Errorf("Invalid drive number: %v", err)
		}

		progress(0.10, fmt.Sprintf("Backing up PhysicalDrive%d...", idx))
		_, err = backupWindowsDisk(client, int(idx), progress)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to backup PhysicalDrive%d: %v", idx, err)
			writeDebugLog(errMsg)
			if opts.OnComplete != nil {
				opts.OnComplete(false, errMsg)
			}
			return fmt.Errorf(errMsg)
		}
	}

	progress(0.95, "Finalizing backup...")
	err := client.UploadManifest()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to upload manifest: %v", err)
		writeDebugLog(errMsg)
		if opts.OnComplete != nil {
			opts.OnComplete(false, errMsg)
		}
		return fmt.Errorf(errMsg)
	}

	err = client.Finish()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to finalize backup: %v", err)
		writeDebugLog(errMsg)
		if opts.OnComplete != nil {
			opts.OnComplete(false, errMsg)
		}
		return fmt.Errorf(errMsg)
	}

	progress(1.0, "Backup completed")
	writeDebugLog("Machine backup completed successfully")

	if opts.OnComplete != nil {
		opts.OnComplete(true, "Machine backup completed successfully")
	}

	return nil
}
