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
	number     int // the partition's number on the disk (sda2: 2)
	majmin     string
	startByte  uint64
	endByte    uint64
	mountpoint string
	fstype     string // "ext4", "xfs", "vfat"... ("" when unknown)
	skip       bool   // left out by the Backup Set: not snapshotted or read
}

// label names the partition: "/dev/sda2 (/home, ext4)" or "/dev/sda1".
func (p diskPartition) label() string {
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
		if n, err := readUintFile(filepath.Join(pdir, "partition")); err == nil {
			p.number = int(n)
		}
		if data, err := os.ReadFile(filepath.Join(pdir, "dev")); err == nil {
			p.majmin = strings.TrimSpace(string(data))
			p.mountpoint, p.fstype = mountForMajMin(p.majmin)
			if p.fstype == "" {
				p.fstype = udevFileSystem(p.majmin)
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

// markSkippedLinux sets skip on the partitions skips name. It returns lines
// for the run log, as markSkipped does on Windows.
func markSkippedLinux(dev string, parts []diskPartition, skips []PartitionSkip) []string {
	if len(skips) == 0 {
		return nil
	}
	ranges := make([]partRange, len(parts))
	for i, p := range parts {
		ranges[i] = partRange{start: p.startByte, end: p.endByte, number: p.number}
	}
	skip, msgs := skipsMatch(dev, ranges, skips)
	for i := range parts {
		if skip[i] {
			parts[i].skip = true
			msgs = append(msgs, leavingOut(dev, parts[i].label(), parts[i].endByte-parts[i].startByte))
		}
	}
	return msgs
}

// snapshotMountpoints are the mounted partitions to snapshot: a partition
// left out is not read, so it needs no snapshot. A disk whose mounted
// partitions are all left out is read without one.
func snapshotMountpoints(parts []diskPartition) []string {
	mountpoints := make([]string, 0)
	for _, p := range parts {
		if p.mountpoint != "" && !p.skip {
			mountpoints = append(mountpoints, p.mountpoint)
		}
	}
	return mountpoints
}

func backupWholeDisk(client *pbscommon.PBSClient, dev string, index int, progressCallback ProgressCallback, jp *JobProgress, readFree, liveRead bool, skips []PartitionSkip) (bool, int64, error) {
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

	for _, msg := range markSkippedLinux(dev, parts, skips) {
		log.Print(msg)
		if progressCallback != nil {
			progressCallback(0, msg)
		}
	}
	mountpoints := snapshotMountpoints(parts)

	log.Printf("Whole-disk backup of %s (%s, %d partitions, %d mounted and backed up)",
		dev, BytesToString(total), len(parts), len(mountpoints))

	fidxName := fmt.Sprintf("drive-sata%d.img.fidx", index)

	// Nothing to snapshot (no mounted partition left to back up, or Use
	// snapshots turned off): the disk is read straight away, so the card
	// says it is reading rather than taking a snapshot.
	if len(mountpoints) == 0 || liveRead {
		jp.setPhase("data")
	}

	err = withSnapshots(mountpoints, liveRead, func(snapshots map[string]snapshot.SnapShot) error {
		return streamStitchedDisk(client, dev, fidxName, uint64(total), parts, snapshots, progressCallback, jp, readFree)
	})
	if err != nil {
		return true, 0, err
	}
	return true, total, nil
}

// liveReadHint ends the message of a backup that stops because a mounted
// disk cannot be snapshotted, saying how to read it live instead, in the app
// and with proxmoxbackup-machine.
const liveReadHint = "or turn off Use snapshots for this Backup Set (-liveread on the command line) and accept a backup that may be inconsistent"

// withSnapshots runs read with block snapshots of mountpoints. With nothing
// mounted there is nothing to snapshot, so read runs straight away. Asking
// the snapshot package anyway failed the backup of a spare or unmounted disk
// on a machine without elastio-snap or dattobd, although no snapshot would
// have been taken; upstream reads such a disk too (0bb391c). A partition in
// use without being mounted (swap, an LVM or RAID member) is read raw either
// way, as before.
//
// liveRead (the set's Use snapshots turned off, and accepted) reads the
// mounted partitions live too. Upstream also fell back to a live read when
// the snapshot failed; here that needs the user's say-so, so a scheduled
// backup never turns inconsistent unnoticed.
func withSnapshots(mountpoints []string, liveRead bool, read func(map[string]snapshot.SnapShot) error) error {
	if len(mountpoints) == 0 {
		log.Print("No partition of this disk that is backed up is mounted: reading it without a snapshot")
		return read(map[string]snapshot.SnapShot{})
	}
	if liveRead {
		log.Printf("WARNING: reading mounted %v live, without a snapshot (Use snapshots is off for this Backup Set, or -liveread): files that change during the backup may be inconsistent in the image", mountpoints)
		return read(map[string]snapshot.SnapShot{})
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("a snapshot of mounted %v needs root: run the backup as root or through the service, %s", mountpoints, liveReadHint)
	}
	if _, ok := snapshot.DetectControl(); !ok {
		return fmt.Errorf("a snapshot of mounted %v needs elastio-snap or dattobd loaded: install one, %s", mountpoints, liveReadHint)
	}
	return snapshot.CreateVSSSnapshot(mountpoints, false, read)
}

type diskSegment struct {
	start   uint64
	end     uint64
	snapDev string
	cowFile string // the snapshot's copy-on-write file on this partition
	rawFS   bool   // an unmounted ext/XFS partition, read raw with its free-space map
	skip    bool   // a partition the Backup Set leaves out: zeros, nothing read
}

// diskRegions are the parts of the disk not read raw as they are: the
// partitions left out, those read through their snapshot, and unmounted
// ext/XFS ones read with their free-space map.
func diskRegions(parts []diskPartition, total uint64, snapshots map[string]snapshot.SnapShot) []diskSegment {
	regions := make([]diskSegment, 0)
	for _, p := range parts {
		end := p.endByte
		if end > total {
			end = total
		}
		if p.skip {
			regions = append(regions, diskSegment{start: p.startByte, end: end, skip: true})
			continue
		}
		if p.mountpoint == "" {
			// Not mounted, so nothing changes it while it is read: its own
			// free-space map is safe to use.
			if p.fstype == "ext2" || p.fstype == "ext3" || p.fstype == "ext4" || p.fstype == "xfs" {
				regions = append(regions, diskSegment{start: p.startByte, end: end, rawFS: true})
			}
			continue
		}
		sn, ok := snapshots[p.mountpoint]
		if !ok || !sn.Valid || sn.ObjectPath == "" || !isBlockDevice(sn.ObjectPath) {
			log.Printf("Note: no snapshot for %s (%s), reading it raw (crash-consistent)", p.device, p.mountpoint)
			continue
		}
		regions = append(regions, diskSegment{start: p.startByte, end: end, snapDev: sn.ObjectPath, cowFile: sn.CowFile})
		log.Printf("Partition %s (%s) @ %s -> snapshot %s",
			p.device, p.mountpoint, BytesToString(int64(p.startByte)), sn.ObjectPath)
	}
	return regions
}

func streamStitchedDisk(client *pbscommon.PBSClient, dev, fidxName string, total uint64, parts []diskPartition, snapshots map[string]snapshot.SnapShot, progressCallback ProgressCallback, jp *JobProgress, readFree bool) error {
	segments := assembleSegments(total, diskRegions(parts, total, snapshots))

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
	readerDone := make(chan struct{})

	go func() {
		defer close(readerDone)
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
	// The reader holds the snapshot devices open until it returns (it stops at
	// its next block once the upload has ended). Returning before then left
	// them open while the snapshots were destroyed, which failed with
	// "resource busy" after an upload error (seen 2026-10-24 on a 4G link).
	<-readerDone
	return uploadErr
}

// partitionLabelAt names the partition holding byte off of the disk:
// "/dev/sda2 (/home)", "/dev/sda1", or "unpartitioned space".
func partitionLabelAt(parts []diskPartition, off uint64) string {
	for _, p := range parts {
		if off >= p.startByte && off < p.endByte {
			if p.skip {
				return p.label() + ", left out"
			}
			return p.label()
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
			segments = append(segments, diskSegment{start: s, end: r.end, snapDev: r.snapDev, cowFile: r.cowFile, rawFS: r.rawFS, skip: r.skip})
			cursor = r.end
		}
	}
	if cursor < total {
		segments = append(segments, diskSegment{start: cursor, end: total})
	}
	return segments
}

func writeSegments(dev string, segments []diskSegment, total uint64, ch chan []byte, uploadDone <-chan struct{}, progressCallback ProgressCallback, jp *JobProgress, readFree bool) error {
	disk, err := os.Open(dev)
	if err != nil {
		return err
	}
	defer disk.Close()
	return copySegments(disk, dev, segments, total, ch, uploadDone, progressCallback, jp, readFree)
}

// copySegments is writeSegments reading the disk through disk.
func copySegments(disk io.ReaderAt, dev string, segments []diskSegment, total uint64, ch chan []byte, uploadDone <-chan struct{}, progressCallback ProgressCallback, jp *JobProgress, readFree bool) error {
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

	// Every segment's source and free-space map first, so that the progress
	// counts only what will be read from the first block on
	// (JobProgress.ToSkip), with the segments left out.
	srcs := make([]io.ReaderAt, len(segments))
	useds := make([]*usedMap, len(segments))
	for i, seg := range segments {
		length := seg.end - seg.start
		if seg.skip {
			jp.addToSkip(length)
			continue
		}
		srcs[i] = disk
		switch {
		case seg.snapDev != "":
			sf, err := os.Open(seg.snapDev)
			if err != nil {
				return err
			}
			defer sf.Close()
			srcs[i] = sf
			useds[i] = segmentUsedMap(sf, int64(length), seg.cowFile, readFree, "snapshot "+seg.snapDev)
		case seg.rawFS:
			useds[i] = segmentUsedMap(io.NewSectionReader(disk, int64(seg.start), int64(length)), int64(length), "", readFree, fmt.Sprintf("%s at %d", dev, seg.start))
		}
		if useds[i] != nil {
			jp.addToSkip(useds[i].skippable(length))
		}
	}

	block := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
	for i, seg := range segments {
		length := seg.end - seg.start
		if seg.skip {
			// Left out by the Backup Set: zeros, and nothing read.
			if !leaveOut(length, emitZeros, jp) {
				return ErrCancelled
			}
			log.Printf("%s at %d: left out, %s written as zeros without being read", dev, seg.start, BytesToString(int64(length)))
			continue
		}
		var bad uint64
		src, srcOff, name := srcs[i], seg.start, dev
		if seg.snapDev != "" {
			srcOff, name = 0, "snapshot "+seg.snapDev
		}
		// onBad gets offsets in src; the report wants them on the disk.
		onBad := func(off, n uint64) {
			bad += n
			jp.noteUnreadable(dev, seg.start+(off-srcOff), n)
		}
		used := useds[i]
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
			return ErrCancelled
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
			return ErrCancelled
		}
		buffer = buffer[n:]
	}
	return nil
}
