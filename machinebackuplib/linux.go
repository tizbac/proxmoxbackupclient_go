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
)

const sectorSize = 512

type diskPartition struct {
	device     string
	startByte  uint64
	endByte    uint64
	mountpoint string
	fstype     string // "ext4", "xfs", "vfat"... ("" when unknown)
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

// mountForMajMin returns where a block device (major:minor) is mounted,
// preferring a mount of its root, and that mount's file system type.
func mountForMajMin(majmin string) (string, string) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", ""
	}
	defer f.Close()

	best, bestFS := "", ""
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
		fs := ""
		for i := 5; i+1 < len(fields); i++ {
			if fields[i] == "-" {
				fs = fields[i+1]
				break
			}
		}
		if best == "" || (!bestIsRoot && root == "/") {
			best, bestFS = mp, fs
			bestIsRoot = root == "/"
		}
	}
	return best, bestFS
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
			majmin := strings.TrimSpace(string(data))
			p.mountpoint, p.fstype = mountForMajMin(majmin)
			if p.fstype == "" {
				p.fstype = udevFileSystem(majmin)
			}
		}
		parts = append(parts, p)
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].startByte < parts[j].startByte })
	return parts, nil
}

// segmentUsedMap is the used map of a snapshotted partition: its file
// system's free space (unless readFree) and the snapshot's own
// copy-on-write file, which lives on the partition and must not go into
// the image (a restored disk would carry a stale file of a tenth of the
// file system). nil means read everything.
func segmentUsedMap(src io.ReaderAt, length int64, cowFile string, readFree bool, name string) *usedMap {
	var m *usedMap
	if !readFree {
		var err error
		if m, err = LinuxFreeSpaceMap(src, length); err != nil {
			log.Printf("%s: free space will be read (%v)", name, err)
			m = nil
		}
	}
	if cowFile == "" {
		return m
	}
	ext, err := fileExtents(cowFile)
	if err != nil || len(ext) == 0 {
		log.Printf("%s: snapshot store %s not left out (%v)", name, cowFile, err)
		return m
	}
	if m == nil {
		m = newUsedMap(4096, (length+4095)/4096)
	}
	var n int64
	for _, e := range ext {
		m.FreeBytes(e[0], e[1])
		n += e[1]
	}
	log.Printf("%s: snapshot store %s (%s) left out of the image", name, cowFile, BytesToString(n))
	return m
}

func isBlockDevice(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeDevice != 0 && fi.Mode()&os.ModeCharDevice == 0
}

func backupWholeDisk(client *pbscommon.PBSClient, dev string, index int, progressCallback ProgressCallback, jp *JobProgress, readFree bool) (bool, int64, error) {
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

	err = snapshot.CreateVSSSnapshot(mountpoints, false, func(snapshots map[string]snapshot.SnapShot) error {
		return streamStitchedDisk(client, dev, fidxName, uint64(total), parts, snapshots, progressCallback, jp, readFree)
	})
	if err != nil {
		return true, 0, err
	}
	return true, total, nil
}

type diskSegment struct {
	start   uint64
	end     uint64
	snapDev string
	cowFile string // the snapshot's copy-on-write file on this partition
}

func streamStitchedDisk(client *pbscommon.PBSClient, dev, fidxName string, total uint64, parts []diskPartition, snapshots map[string]snapshot.SnapShot, progressCallback ProgressCallback, jp *JobProgress, readFree bool) error {
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
		regions = append(regions, diskSegment{start: p.startByte, end: end, snapDev: sn.ObjectPath, cowFile: sn.CowFile})
		log.Printf("Partition %s (%s) @ %s -> snapshot %s",
			p.device, p.mountpoint, BytesToString(int64(p.startByte)), sn.ObjectPath)
	}

	segments := assembleSegments(total, regions)

	// The progress card names the partition being read; the reader only
	// reports a fraction of the disk, so look the offset up here.
	if jp != nil {
		inner := progressCallback
		progressCallback = func(fraction float64, message string) bool {
			jp.setPartition(partitionLabelAt(parts, uint64(fraction*float64(total))))
			if inner == nil {
				return false
			}
			return inner(fraction, message)
		}
	}

	ch := make(chan []byte)
	errCh := make(chan error, 1)
	uploadDone := make(chan struct{})

	go func() {
		err := writeSegments(dev, segments, total, ch, uploadDone, progressCallback, jp, readFree)
		// Error first, then close: uploadWorker's non-blocking check of
		// errCh runs after ch is drained and must find the value there.
		errCh <- err
		close(ch)
	}()

	// uploadWorker is the sole consumer of errCh (it returns the reader error
	// through its own return value), so this function must not read errCh
	// itself — a second receive would block forever once the writer has
	// exited.
	var uploadErr error
	go func() {
		defer close(uploadDone)
		uploadErr = uploadWorker(client, fidxName, total, ch, errCh, jp)
	}()

	<-uploadDone
	return uploadErr
}

// partitionLabelAt names the partition holding byte off of the disk:
// "/dev/sda2 (/home)", "/dev/sda1", or "unpartitioned space".
func partitionLabelAt(parts []diskPartition, off uint64) string {
	for _, p := range parts {
		if off >= p.startByte && off < p.endByte {
			var in []string
			for _, s := range []string{p.mountpoint, p.fstype} {
				if s != "" {
					in = append(in, s)
				}
			}
			if len(in) > 0 {
				return p.device + " (" + strings.Join(in, ", ") + ")"
			}
			return p.device
		}
	}
	return "unpartitioned space"
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
			segments = append(segments, diskSegment{start: s, end: r.end, snapDev: r.snapDev, cowFile: r.cowFile})
			cursor = r.end
		}
	}
	if cursor < total {
		segments = append(segments, diskSegment{start: cursor, end: total})
	}
	return segments
}

func writeSegments(dev string, segments []diskSegment, total uint64, ch chan []byte, uploadDone <-chan struct{}, progressCallback ProgressCallback, jp *JobProgress, readFree bool) (retErr error) {
	buffer := make([]byte, 0, pbscommon.PBS_FIXED_CHUNK_SIZE*2)

	var pos uint64 = 0

	report := func() bool {
		if progressCallback == nil {
			return false
		}
		return progressCallback(float64(pos)/float64(total), blockMessage(dev, pos, total))
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
		src, srcOff, name := io.ReaderAt(disk), seg.start, dev
		if seg.snapDev != "" {
			sf, err := os.Open(seg.snapDev)
			if err != nil {
				return err
			}
			defer sf.Close()
			src, srcOff, name = sf, 0, "snapshot "+seg.snapDev
		}
		// onBad gets offsets in src; the report wants them on the disk.
		onBad := func(off, n uint64) {
			bad += n
			jp.noteUnreadable(dev, seg.start+(off-srcOff), n)
		}
		var used *usedMap
		if seg.snapDev != "" {
			used = segmentUsedMap(src, int64(length), seg.cowFile, readFree, name)
		}
		var err error
		if used == nil {
			err = readResilient(src, srcOff, length, block, sectorSize, emit, emitZeros, onBad)
		} else {
			// As on Windows: a 4 MiB window with no used block is zeros,
			// the rest is read with its free blocks zeroed.
			var skipped uint64
			for w := uint64(0); w < length && err == nil; w += pbscommon.PBS_FIXED_CHUNK_SIZE {
				n := min(uint64(pbscommon.PBS_FIXED_CHUNK_SIZE), length-w)
				if used.anyUsed(int64(w), int64(n)) {
					at := int64(w)
					masked := func(data []byte) bool {
						used.zeroFree(data, at)
						at += int64(len(data))
						return emit(data)
					}
					maskedZeros := func(k uint64) bool {
						at += int64(k)
						return emitZeros(k)
					}
					err = readResilient(src, srcOff+w, n, block, sectorSize, masked, maskedZeros, onBad)
				} else if emitZeros(n) {
					skipped += n
					jp.FreeSkipped.Add(n)
				} else {
					err = errCopyStopped
				}
			}
			log.Printf("%s: %s of free space not read", name, BytesToString(int64(skipped)))
		}
		if bad > 0 {
			log.Printf("[31;1mWarning: %s had %d unreadable sector(s) (~%s), zero-filled in the image[0m",
				name, bad/sectorSize, BytesToString(int64(bad)))
		}
		if err == errCopyStopped {
			return errCancelled
		}
		if err != nil {
			return fmt.Errorf("read %s at segment %d: %w", name, seg.start, err)
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
