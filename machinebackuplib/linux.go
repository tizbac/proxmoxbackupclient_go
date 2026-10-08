//go:build linux
// +build linux

package machinebackuplib

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"pbscommon"
	"snapshot"
	"sort"
	"strconv"
	"strings"
	"math"
)

const sectorSize = 512

type diskPartition struct {
	device     string
	startByte  uint64
	endByte    uint64
	mountpoint string
}

func unescapeOctal(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func mountForMajMin(majmin string) string {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	best := ""
	bestIsRoot := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {

		fields := strings.Fields(sc.Text())
		if len(fields) < 5 || fields[2] != majmin {
			continue
		}
		root := fields[3]
		mp := unescapeOctal(fields[4])
		if best == "" || (!bestIsRoot && root == "/") {
			best = mp
			bestIsRoot = root == "/"
		}
	}
	return best
}

func readUintFile(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
}

func enumeratePartitions(diskName string) ([]diskPartition, error) {
	sysfs := filepath.Join("/sys/block", diskName)
	entries, err := os.ReadDir(sysfs)
	if err != nil {
		return nil, err
	}
	parts := make([]diskPartition, 0)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pdir := filepath.Join(sysfs, e.Name())

		if _, err := os.Stat(filepath.Join(pdir, "partition")); err != nil {
			continue
		}
		start, err := readUintFile(filepath.Join(pdir, "start"))
		if err != nil {
			continue
		}
		size, err := readUintFile(filepath.Join(pdir, "size"))
		if err != nil {
			continue
		}
		p := diskPartition{
			device:    "/dev/" + e.Name(),
			startByte: start * sectorSize,
			endByte:   (start + size) * sectorSize,
		}
		if data, err := os.ReadFile(filepath.Join(pdir, "dev")); err == nil {
			p.mountpoint = mountForMajMin(strings.TrimSpace(string(data)))
		}
		parts = append(parts, p)
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].startByte < parts[j].startByte })
	return parts, nil
}

func isBlockDevice(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeDevice != 0 && fi.Mode()&os.ModeCharDevice == 0
}

func backupWholeDisk(client *pbscommon.PBSClient, dev string, index int, useSnapshot bool, progressCallback ProgressCallback) (bool, int64, error) {
	if !strings.HasPrefix(dev, "/dev/") {
		return false, 0, nil
	}
	diskName := strings.TrimPrefix(dev, "/dev/")

	if fi, err := os.Stat(filepath.Join("/sys/block", diskName)); err != nil || !fi.IsDir() {
		return false, 0, nil
	}

	df, err := os.Open(dev)
	if err != nil {
		return false, 0, err
	}
	total, err := df.Seek(0, io.SeekEnd)
	df.Close()
	if err != nil {
		return false, 0, err
	}

	parts, err := enumeratePartitions(diskName)
	if err != nil {
		return false, 0, err
	}

	mountpoints := make([]string, 0)
	for _, p := range parts {
		if p.mountpoint != "" {
			mountpoints = append(mountpoints, p.mountpoint)
		}
	}

	log.Printf("Whole-disk backup of %s (%s, %d partitions, %d mounted)",
		dev, BytesToString(total), len(parts), len(mountpoints))

	fidxName := fmt.Sprintf("drive-sata%d.img.fidx", index)

	if useSnapshot && len(mountpoints) > 0 {
		// Try to create snapshots for mounted partitions
		err := snapshot.CreateVSSSnapshot(mountpoints, false, func(snapshots map[string]snapshot.SnapShot) error {
			return streamStitchedDisk(client, dev, fidxName, uint64(total), parts, snapshots, progressCallback)
		})
		if err != nil {
			return false, 0, fmt.Errorf("snapshot creation failed (dattobd/elastio-snap required for crash-consistent backup): %w", err)
		}
		return true, total, nil
	}

	// useSnapshot is false: raw disk read (crash-consistent) - this is the explicit user choice
	log.Printf("Reading raw disk %s (crash-consistent, snapshots disabled by user)", dev)
	return readRawDisk(client, dev, fidxName, uint64(total), progressCallback)
}

func readRawDisk(client *pbscommon.PBSClient, dev, fidxName string, total uint64, progressCallback ProgressCallback) (bool, int64, error) {
	// Get partitions for raw read
	parts, err := enumeratePartitions(strings.TrimPrefix(dev, "/dev/"))
	if err != nil {
		return false, 0, err
	}

	// Create a dummy snapshot map with no valid snapshots - streamStitchedDisk
	// will skip partitions without valid snapshots and read raw sectors
	emptySnapshots := make(map[string]snapshot.SnapShot)
	for _, p := range parts {
		if p.mountpoint != "" {
			emptySnapshots[p.mountpoint] = snapshot.SnapShot{}
		}
	}
	if err := streamStitchedDisk(client, dev, fidxName, total, parts, emptySnapshots, progressCallback); err != nil {
		return false, 0, err
	}
	return true, int64(total), nil
}

type diskSegment struct {
	start   uint64
	end     uint64
	snapDev string
}

func streamStitchedDisk(client *pbscommon.PBSClient, dev, fidxName string, total uint64, parts []diskPartition, snapshots map[string]snapshot.SnapShot, progressCallback ProgressCallback) error {
	regions := make([]diskSegment, 0)
	for _, p := range parts {
		if p.mountpoint == "" {
			continue
		}
		sn, ok := snapshots[p.mountpoint]
		if !ok || !sn.Valid || sn.ObjectPath == "" || !isBlockDevice(sn.ObjectPath) {
			log.Printf("Note: no snapshot for %s (%s), reading it raw (crash-consistent)", p.device, p.mountpoint)
			continue
		}
		end := p.endByte
		if end > total {
			end = total
		}
		regions = append(regions, diskSegment{start: p.startByte, end: end, snapDev: sn.ObjectPath})
		log.Printf("Partition %s (%s) @ %s -> snapshot %s",
			p.device, p.mountpoint, BytesToString(int64(p.startByte)), sn.ObjectPath)
	}

	segments := assembleSegments(total, regions)

	ch := make(chan []byte)
	errCh := make(chan error, 1)
	uploadDone := make(chan struct{})

	go func() {
		err := writeSegments(dev, segments, total, ch, uploadDone, progressCallback)
		close(ch)
		errCh <- err
	}()

	// uploadWorker is the sole consumer of errCh (it returns the reader error
	// through its own return value), so this function must not read errCh
	// itself — a second receive would block forever once the writer has
	// exited.
	var uploadErr error
	go func() {
		defer close(uploadDone)
		uploadErr = uploadWorker(client, fidxName, total, ch, errCh)
	}()

	<-uploadDone
	return uploadErr
}

func assembleSegments(total uint64, regions []diskSegment) []diskSegment {
	sort.Slice(regions, func(i, j int) bool { return regions[i].start < regions[j].start })

	segments := make([]diskSegment, 0, len(regions)*2+1)
	var cursor uint64 = 0
	for _, r := range regions {
		if r.start > cursor {
			segments = append(segments, diskSegment{start: cursor, end: r.start})
		}
		s := r.start
		if s < cursor {
			s = cursor
		}
		if r.end > s {
			segments = append(segments, diskSegment{start: s, end: r.end, snapDev: r.snapDev})
			cursor = r.end
		}
	}
	if cursor < total {
		segments = append(segments, diskSegment{start: cursor, end: total})
	}
	return segments
}

func writeSegments(dev string, segments []diskSegment, total uint64, ch chan []byte, uploadDone <-chan struct{}, progressCallback ProgressCallback) (retErr error) {
	buffer := make([]byte, 0, pbscommon.PBS_FIXED_CHUNK_SIZE*2)

	var pos uint64 = 0
	var b int64 = 0

	report := func() bool {
		if progressCallback == nil {
			return false
		}
		if total == 0 {
			// Unknown total size; report with 0% to avoid NaN
			return progressCallback(0, fmt.Sprintf("%s: Block %d", dev, b))
		}
		pct := float64(pos) / float64(total)
		if math.IsNaN(pct) || math.IsInf(pct, 0) {
			pct = 0
		}
		return progressCallback(pct, fmt.Sprintf("%s: Block %d", dev, b))
	}

	sendChunk := func(chunk []byte) bool {
		if len(chunk) == 0 {
			return true
		}
		select {
		case ch <- chunk:
		case <-uploadDone:
			return false // uploader stopped; drop the data
		}
		pos += uint64(len(chunk))
		b++
		return true
	}

	emit := func(data []byte) bool {
		buffer = append(buffer, data...)
		for len(buffer) >= pbscommon.PBS_FIXED_CHUNK_SIZE {
			chunk := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
			copy(chunk, buffer[:pbscommon.PBS_FIXED_CHUNK_SIZE])
			if !sendChunk(chunk) {
				return false
			}
			buffer = buffer[pbscommon.PBS_FIXED_CHUNK_SIZE:]
		}
		if report() {
			return false // cancelled by user
		}
		return true
	}
	zeroBlk := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
	emitZeros := func(n uint64) bool {
		for n > 0 {
			m := len(zeroBlk)
			if uint64(m) > n {
				m = int(n)
			}
			if !emit(zeroBlk[:m]) {
				return false
			}
			n -= uint64(m)
		}
		return true
	}

	disk, err := os.Open(dev)
	if err != nil {
		return err
	}
	defer disk.Close()

	block := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
	for _, seg := range segments {
		length := seg.end - seg.start
		var bad uint64
		var ok bool
		if seg.snapDev == "" {
			bad, ok = resilientCopy(disk, seg.start, length, block, emit, emitZeros)
			if bad > 0 {
				log.Printf("\033[31;1mWarning: %s had %d unreadable sector(s) (~%s), zero-filled in the image\033[0m",
					dev, bad, BytesToString(int64(bad*sectorSize)))
			}
		} else {
			sf, err := os.Open(seg.snapDev)
			if err != nil {
				return err
			}
			bad, ok = resilientCopy(sf, 0, length, block, emit, emitZeros)
			sf.Close()
			if bad > 0 {
				log.Printf("\033[31;1mWarning: snapshot %s had %d unreadable sector(s) (~%s), zero-filled in the image\033[0m",
					seg.snapDev, bad, BytesToString(int64(bad*sectorSize)))
			}
		}
		if !ok {
			return errCancelled
		}
	}

	for len(buffer) > 0 {
		n := len(buffer)
		if n > pbscommon.PBS_FIXED_CHUNK_SIZE {
			n = pbscommon.PBS_FIXED_CHUNK_SIZE
		}
		chunk := make([]byte, n)
		copy(chunk, buffer[:n])
		if !sendChunk(chunk) {
			return errCancelled
		}
		buffer = buffer[n:]
	}
	return nil
}

func resilientCopy(src io.ReaderAt, srcOffset, length uint64, block []byte, emit func([]byte) bool, emitZeros func(uint64) bool) (uint64, bool) {
	var badSectors uint64
	var pos uint64
	for pos < length {
		want := uint64(len(block))
		if want > length-pos {
			want = length - pos
		}
		n, err := src.ReadAt(block[:want], int64(srcOffset+pos))
		if n > 0 {
			if !emit(block[:n]) {
				return badSectors, false
			}
			pos += uint64(n)
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			if pos < length {
				if !emitZeros(length - pos) {
					return badSectors, false
				}
				pos = length
			}
			break
		}

		winEnd := pos + (want - uint64(n))
		for pos < winEnd {
			s := uint64(sectorSize)
			if s > winEnd-pos {
				s = winEnd - pos
			}
			m, serr := src.ReadAt(block[:s], int64(srcOffset+pos))
			if m > 0 {
				if !emit(block[:m]) {
					return badSectors, false
				}
				pos += uint64(m)
			}
			if serr == nil {
				continue
			}
			if serr == io.EOF {
				if pos < length {
					if !emitZeros(length - pos) {
						return badSectors, false
					}
					pos = length
				}
				winEnd = pos
				break
			}
			if rem := s - uint64(m); rem > 0 {
				if !emitZeros(rem) {
					return badSectors, false
				}
				pos += rem
				badSectors++
			}
		}
	}
	return badSectors, true
}
