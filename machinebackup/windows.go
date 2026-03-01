//go:build windows
// +build windows

package main

import (
	"fmt"
	"io"
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
		if E.PartitionNumber == 0 {
			continue //Windows API sometimes wrongly returns a partition that is effectively null, probably in case of MBR it is fixed 4 partitions anyway
		}
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
					block := make([]byte, PBS_FIXED_CHUNK_SIZE)
					pos := P.StartByte
					for pos < P.EndByte {
						nbytes, err := F.Read(block[:min(uint64(len(block)), P.EndByte-pos)])
						if err != nil {
							panic(err)
						}
						buffer = append(buffer, block[:nbytes]...)

						if len(buffer) >= PBS_FIXED_CHUNK_SIZE {
							ch <- buffer[:PBS_FIXED_CHUNK_SIZE]
							buffer = buffer[PBS_FIXED_CHUNK_SIZE:]
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
					pos := P.StartByte
					block := make([]byte, PBS_FIXED_CHUNK_SIZE)
					for {
						nbytes, err := snapshot_file.Read(block)
						if err == io.EOF {
							break
						}
						if pos >= P.EndByte {
							panic(fmt.Errorf("Fatal: Went outside partition space while reading VSS snapshot"))
						}
						if err != nil {
							panic(err)
						}
						pos += uint64(nbytes)
						buffer = append(buffer, block[:nbytes]...)
						if len(buffer) >= PBS_FIXED_CHUNK_SIZE {
							ch <- buffer[:PBS_FIXED_CHUNK_SIZE]
							buffer = buffer[PBS_FIXED_CHUNK_SIZE:]
						}
					}
				}
			}
			if len(buffer) > 0 {
				ch <- buffer
			}

			close(ch)
		}()

		return uploadWorker(client, fmt.Sprintf("windisk%d.fidx", index), uint64(total), ch)

	})
}

func sysTraySetup() {
	//TODO
}
