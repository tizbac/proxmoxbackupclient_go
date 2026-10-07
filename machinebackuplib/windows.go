//go:build windows
// +build windows

package machinebackuplib

import (
	"fmt"
	"io"
	"log"
	"os"
	"pbscommon"
	"snapshot"
	"strings"
	"syscall"
	"unsafe"

	"github.com/tawesoft/golib/v2/dialog"
	"golang.org/x/sys/windows"
)

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
	Length     uint64
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
						Length:     uint64(extent.ExtentLength),
						Letters:    make([]string, 0),
					}

					r1, _, _ := procGetVolumePathNamesForVolumeW.Call(
						uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(volName))),
						uintptr(unsafe.Pointer(&buffer2[0])),
						uintptr(len(buffer2)),
						uintptr(unsafe.Pointer(&returnLength)),
					)

					if r1 != 0 {
						// GetVolumePathNamesForVolumeNameW fills buffer2 (UTF-16)
						// with a REG_MULTI_SZ: consecutive null-terminated wide
						// strings ending in an empty string. returnLength is the
						// count of wide chars written. The previous loop scanned
						// the raw IOCTL byte buffer with byte indices, which only
						// ever recovered the first character of the first path.
						end := int(returnLength)
						if end > len(buffer2) {
							end = len(buffer2)
						}
						j := 0
						for j < end && buffer2[j] != 0 {
							start := j
							for j < end && buffer2[j] != 0 {
								j++
							}
							if p := windows.UTF16ToString(buffer2[start:j]); p != "" {
								v.Letters = append(v.Letters, p)
							}
							j++ // skip the null terminator
						}
					} else {
						// Could not map this volume to a path: leave it without a
						// letter and keep enumerating the rest (this previously
						// aborted the whole enumeration, silently dropping VSS for
						// every later volume).
						fmt.Printf("%s : GetVolumePathNamesForVolumeNameW failed\n", volName)
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

// GetDiskSize returns the size of a disk device
func GetDiskSize(path string) (int64, error) {
	return GetDiskLength(path)
}

func BackupWindowsDisk(client *pbscommon.PBSClient, index int, progressCallback ProgressCallback) (int64, error) {
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
		if E.PartitionNumber == 0 {
			continue //Windows API sometimes wrongly returns a partition that is effectively null, probably in case of MBR it is fixed 4 partitions anyway
		}
		fmt.Printf("Part: %d %s %s\n", E.PartitionNumber, BytesToString(int64(E.StartingOffset)), BytesToString(int64(E.PartitionLength)))
		p := Partition{
			StartByte: uint64(E.StartingOffset),
			EndByte:   uint64(E.StartingOffset + E.PartitionLength),
			Skip:      false,
		}
		// An active volume (one with a drive letter) that overlaps this
		// partition MUST be read through a VSS snapshot: reading the raw disk
		// underneath a mounted NTFS volume is not crash-consistent. Overlap
		// (rather than exact offset match) also covers volumes whose extent
		// starts slightly off the partition start.
		for _, V := range vols {
			if V.DiskNumber != int32(index) {
				continue
			}
			volEnd := V.Offset + V.Length
			if p.EndByte <= V.Offset || p.StartByte >= volEnd {
				continue // no overlap
			}
			// Only an exact drive-letter mount ("X:\") gives a usable VSS
			// letter. Folder mount points ("C:\Mount\Data\") have no own
			// letter; backing them up raw by partition offset yields the
			// correct data (if not crash-consistent), whereas taking the host
			// volume's letter as theirs would snapshot the wrong volume.
			for _, mountPath := range V.Letters {
				if len(mountPath) == 3 && mountPath[1] == ':' && mountPath[2] == '\\' {
					p.RequiresVSS = true
					if p.Letter == "" {
						p.Letter = string(mountPath[0])
					}
					break
				}
			}
		}
		parts = append(parts, p)
	}

	// Enforce VSS coverage: every drive-letter volume on this disk must fall
	// inside a partition entry we will read through its snapshot. If one is
	// not, abort instead of silently streaming a live volume raw.
	for _, V := range vols {
		if V.DiskNumber != int32(index) {
			continue
		}
		hasLetter := false
		for _, mountPath := range V.Letters {
			if len(mountPath) == 3 && mountPath[1] == ':' && mountPath[2] == '\\' {
				hasLetter = true
				break
			}
		}
		if !hasLetter {
			continue
		}
		volEnd := V.Offset + V.Length
		covered := false
		for i := range parts {
			if parts[i].StartByte < volEnd && parts[i].EndByte > V.Offset {
				covered = true
				break
			}
		}
		if !covered {
			return 0, fmt.Errorf("refusing to back up %s: active volume at offset %s (mount %s) is not covered by any partition entry, cannot guarantee a VSS-consistent read",
				diskdev, BytesToString(int64(V.Offset)), strings.Join(V.Letters, "; "))
		}
	}

	snapshot_paths := make([]string, 0)

	for _, p := range parts {
		if p.RequiresVSS {
			snapshot_paths = append(snapshot_paths, fmt.Sprintf("%s:\\\\", p.Letter))
		}
	}

	total, err := GetDiskLength(diskdev)
	if err != nil {
		return 0, err
	}

	// Visible "working" phase before the snapshot step: on a live Windows
	// system CreateVSSSnapshot can take a while (it runs the VSS writers),
	// and without this the GUI progress bar appears frozen at 0%.
	if progressCallback != nil {
		progressCallback(0, fmt.Sprintf("%s: creating VSS snapshot", diskdev))
	}

	return total, snapshot.CreateVSSSnapshot(snapshot_paths, false, func(snapshots map[string]snapshot.SnapShot) error {
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

		F, err := os.Open(diskdev)
		if err != nil {
			return err
		}
		defer F.Close()

		var b int64 = 0
		buffer := make([]byte, 0)

		errCh := make(chan error, 1)
		uploadDone := make(chan struct{})

		// Report read progress for this disk; the callback maps it onto the
		// whole job. Returning true stops the backup (user pressed Stop).
		report := func(pos uint64) bool {
			if progressCallback == nil {
				return false
			}
			return progressCallback(float64(pos)/float64(total), fmt.Sprintf("%s: Block %d", diskdev, b))
		}

		sendChunk := func(p []byte) bool {
			if len(p) == 0 {
				return true
			}
			select {
			case ch <- p:
				return true
			case <-uploadDone:
				return false // uploader stopped; drop the data
			}
		}

		push := func(data []byte) bool {
			buffer = append(buffer, data...)
			for len(buffer) >= pbscommon.PBS_FIXED_CHUNK_SIZE {
				chunk := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
				copy(chunk, buffer[:pbscommon.PBS_FIXED_CHUNK_SIZE])
				if !sendChunk(chunk) {
					return false
				}
				buffer = buffer[pbscommon.PBS_FIXED_CHUNK_SIZE:]
			}
			return true
		}

		flush := func() bool {
			for len(buffer) > 0 {
				n := len(buffer)
				if n > pbscommon.PBS_FIXED_CHUNK_SIZE {
					n = pbscommon.PBS_FIXED_CHUNK_SIZE
				}
				chunk := make([]byte, n)
				copy(chunk, buffer[:n])
				if !sendChunk(chunk) {
					return false
				}
				buffer = buffer[n:]
			}
			return true
		}

		readPartition := func(P Partition) error {
			pos := P.StartByte
			block := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
			if !P.RequiresVSS {
				if _, err := F.Seek(int64(P.StartByte), io.SeekStart); err != nil {
					return fmt.Errorf("seek to offset %d in %s: %w", P.StartByte, diskdev, err)
				}
				for pos < P.EndByte {
					nbytes, rerr := F.Read(block[:min(uint64(len(block)), P.EndByte-pos)])
					if nbytes > 0 {
						if !push(block[:nbytes]) {
							return errUploadAborted
						}
						pos += uint64(nbytes)
					}
					if report(pos) {
						return errCancelled
					}
					b++
					if rerr == io.EOF {
						break
					}
					if rerr != nil {
						return fmt.Errorf("read %s at offset %d: %w", diskdev, pos, rerr)
					}
				}
			} else {
				snap, ok := snapshots[P.Letter+":\\"]
				if !ok {
					return fmt.Errorf("cannot find snapshot for letter %s", P.Letter)
				}
				snapshot_file, err := os.Open(strings.TrimRight(snap.ObjectPath, "\\"))
				if err != nil {
					return err
				}
				defer snapshot_file.Close()

				l, err := GetDiskLength(strings.TrimRight(snap.ObjectPath, "\\"))
				if err != nil {
					return err
				}
				if uint64(P.StartByte)+uint64(l) > P.EndByte {
					log.Print("Harmless warning: VSS snapshot is larger than partition, will read only up to the partition end")
				}

				// Read the shadow up to the partition end. Clamping each read
				// to P.EndByte-pos means pos never overshoots, so a shadow that
				// is exactly the partition size ends cleanly on EOF, a smaller
				// shadow leaves a positive pad, and a larger one is simply
				// truncated at the partition boundary.
				for pos < P.EndByte {
					nbytes, rerr := snapshot_file.Read(block[:min(uint64(len(block)), P.EndByte-pos)])
					if report(pos) {
						return errCancelled
					}
					b++
					if nbytes > 0 {
						if !push(block[:nbytes]) {
							return errUploadAborted
						}
						pos += uint64(nbytes)
					}
					if rerr == io.EOF {
						break
					}
					if rerr != nil {
						return fmt.Errorf("read snapshot %s at offset %d: %w", snap.Id, pos, rerr)
					}
				}

				// Pad with zeros for the bytes the shadow didn't cover (FS
				// smaller than the partition). npad is computed from how much
				// was actually read, avoiding the unsigned underflow the old
				// pre-computed npad hit when the shadow was >= the partition.
				npad := P.EndByte - pos
				zeroBlk := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
				for npad > 0 {
					log.Printf("Padding %d", npad)
					m := min(uint64(pbscommon.PBS_FIXED_CHUNK_SIZE), npad)
					if !push(zeroBlk[:m]) {
						return errUploadAborted
					}
					pos += m
					npad -= m
				}
			}
			if pos != P.EndByte {
				return fmt.Errorf("failed to read partition entirely %d/%d", pos, P.EndByte)
			}
			return nil
		}

		go func() {
			var rerr error
			defer func() {
				close(ch)
				errCh <- rerr
			}()
			for idx, P := range parts {
				fmt.Printf("Partition: %d\n", idx)
				if e := readPartition(P); e != nil {
					rerr = e
					break
				}
			}
			if rerr == nil && !flush() {
				rerr = errUploadAborted
			}
		}()

		var uploadErr error
		go func() {
			defer close(uploadDone)
			uploadErr = uploadWorker(client, fmt.Sprintf("drive-sata%d.img.fidx", index), uint64(total), ch, errCh)
		}()

		readErr := <-errCh
		<-uploadDone
		if uploadErr != nil {
			return uploadErr
		}
		return readErr
	})
}

func SysTraySetup() {
	//TODO
}

// backupWholeDisk is Linux-only (see linux.go). On Windows whole disks are
// handled through the \\\\.\\PhysicalDriveN path, so this reports "not handled".
func backupWholeDisk(client *pbscommon.PBSClient, dev string, index int, useSnapshot bool, progressCallback ProgressCallback) (bool, int64, error) {
	return false, 0, nil
}
