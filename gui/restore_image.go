package main

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"pbscommon"

	"github.com/tizbac/proxmoxbackupclient_go/gui/imagefs"
)

// Files from a machine (disk image) backup: a snapshot whose only archives
// are drive-*.img.fidx is browsed and restored by reading the partition
// table and NTFS volumes inside the image (gui/imagefs) straight from the
// fixed index, fetching only the chunks that hold what is being read. The
// tree shows one folder per NTFS volume ("Partition 3 - Windows (NTFS, 127.3
// GB)"); restore writes to another folder, never over the live disk.

// snapshotDetails remembers each listed snapshot's type and manifest files,
// keyed by server, datastore, namespace, ID and time, so the browse and
// restore calls (which carry only an ID and a time) can tell a vm/ snapshot
// or a disk image when they meet one.
var snapshotDetails sync.Map

func snapshotDetailsKey(baseURL, datastore, namespace, backupID string, unix int64) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d", baseURL, datastore, namespace, backupID, unix)
}

func rememberSnapshot(baseURL, datastore, namespace string, info SnapshotInfo) {
	k := snapshotDetailsKey(baseURL, datastore, namespace, info.BackupID, info.BackupTime.Unix())
	// A host and a vm snapshot with the same ID and time: keep the one with
	// folder archives, which is what the restore tree has always shown.
	if prev, ok := snapshotDetails.Load(k); ok && hasDataArchive(prev.(SnapshotInfo).Files) && !hasDataArchive(info.Files) {
		return
	}
	snapshotDetails.Store(k, info)
}

func hasDataArchive(files []string) bool {
	for _, f := range files {
		if isDataArchive(f) {
			return true
		}
	}
	return false
}

// imageArchives lists a snapshot's disk images (drive-*.img.fidx), sorted.
func imageArchives(files []string) []string {
	var out []string
	for _, f := range files {
		if strings.HasSuffix(f, ".img.fidx") {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// fillSnapshotKind sets opts.BackupType and the manifest file list from the
// snapshot list (listing the snapshots once if this one has not been seen).
// Best effort: on failure the options stay as they were ("host", no files),
// which is how every folder snapshot behaved before.
func fillSnapshotKind(opts *RestoreOptions) {
	if opts.snapshotFiles != nil || opts.BackupID == "" {
		return
	}
	k := snapshotDetailsKey(opts.BaseURL, opts.Datastore, opts.Namespace, opts.BackupID, opts.SnapshotTime.Unix())
	v, ok := snapshotDetails.Load(k)
	if !ok {
		if _, err := ListSnapshotsInline(opts.BaseURL, opts.AuthID, opts.Secret, opts.Ticket, opts.CSRFToken,
			opts.Datastore, opts.Namespace, opts.CertFingerprint, opts.BackupID); err != nil {
			writeBackupLog(fmt.Sprintf("Snapshot details for %s@%d not found: %v", opts.BackupID, opts.SnapshotTime.Unix(), err))
			return
		}
		if v, ok = snapshotDetails.Load(k); !ok {
			return
		}
	}
	info := v.(SnapshotInfo)
	if opts.BackupType == "" {
		opts.BackupType = info.BackupType
	}
	opts.snapshotFiles = append([]string{}, info.Files...)
}

func (o RestoreOptions) backupType() string {
	if o.BackupType == "" {
		return "host"
	}
	return o.BackupType
}

// isImageSnapshot: the snapshot holds disk images and no folder archive.
func isImageSnapshot(opts RestoreOptions) bool {
	return len(imageArchives(opts.snapshotFiles)) > 0 && !hasDataArchive(opts.snapshotFiles)
}

// imageVolume is one NTFS volume of a disk-image snapshot.
type imageVolume struct {
	Display string // the folder name the tree shows it under
	Archive string // drive-sata0.img.fidx
	Start   int64  // partition offset in the image
	Size    int64
	Entries []imagefs.Entry // sorted by path, with MFT record numbers for restore
}

// imageListing keeps the volumes of the last image snapshot listed, so a
// restore right after browsing does not read the master file tables again.
var imageListing struct {
	sync.Mutex
	key  snapshotCacheKey
	vols []imageVolume
}

// imageSession is an open reader session on a disk-image snapshot.
type imageSession struct {
	client  *pbscommon.PBSClient
	readers map[string]*pbscommon.DIDXReaderAt
	sizes   map[string]int64
}

func openImageSession(opts RestoreOptions, cancel func() bool) (*imageSession, error) {
	client := &pbscommon.PBSClient{
		BaseURL: opts.BaseURL, CertFingerPrint: opts.CertFingerprint, AuthID: opts.AuthID, Secret: opts.Secret,
		Ticket: opts.Ticket, CSRFToken: opts.CSRFToken, Datastore: opts.Datastore, Namespace: opts.Namespace,
		Insecure: opts.CertFingerprint != "", Crypt: opts.Crypt,
		Manifest: pbscommon.BackupManifest{BackupID: opts.BackupID, BackupTime: opts.SnapshotTime.Unix()},
	}
	client.Connect(true, opts.backupType())
	s := &imageSession{client: client, readers: map[string]*pbscommon.DIDXReaderAt{}, sizes: map[string]int64{}}
	for _, a := range imageArchives(opts.snapshotFiles) {
		ra, size, err := client.NewFIDXReaderAt(a, 32, nil)
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("open disk image %s: %w", a, err)
		}
		ra.SetContext(opts.Ctx)
		ra.SetCancelCheck(cancel)
		s.readers[a], s.sizes[a] = ra, size
	}
	return s, nil
}

func (s *imageSession) close() { s.client.Close() }

// volumeName is the folder a volume is shown and restored under:
// "Partition 3 - Windows (NTFS, 127.3 GB)", with "Disk 2 " in front when the
// snapshot has more than one disk.
func volumeName(p imagefs.Partition, label string, disk, disks int) string {
	name := fmt.Sprintf("Partition %d", p.Number)
	if label = strings.TrimSpace(strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(label)); label != "" {
		name += " - " + label
	}
	name += " (NTFS, " + imagefs.HumanBytes(p.Size) + ")"
	if disks > 1 {
		name = fmt.Sprintf("Disk %d %s", disk, name)
	}
	return name
}

// openImageVolumes finds the NTFS volumes in every disk of the session and,
// when list is set, lists their files. Partitions with another file system
// (EFI FAT32, BitLocker, Linux) are logged and left out.
func openImageVolumes(s *imageSession, opts RestoreOptions, list bool, cancel func() bool) ([]imageVolume, map[string]*imagefs.NTFS, error) {
	archives := imageArchives(opts.snapshotFiles)
	var vols []imageVolume
	opened := map[string]*imagefs.NTFS{}
	var skipped []string
	for di, a := range archives {
		ra := s.readers[a]
		parts, err := imagefs.ReadPartitions(ra, s.sizes[a])
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", a, err)
		}
		for _, p := range parts {
			if p.FS != "ntfs" {
				skipped = append(skipped, a+" "+p.Label())
				continue
			}
			vol, err := imagefs.OpenNTFS(io.NewSectionReader(ra, p.Start, p.Size))
			if err != nil {
				skipped = append(skipped, fmt.Sprintf("%s %s (%v)", a, p.Label(), err))
				continue
			}
			v := imageVolume{Display: volumeName(p, vol.Label(), di+1, len(archives)), Archive: a, Start: p.Start, Size: p.Size}
			if list {
				start := time.Now()
				last := time.Time{}
				v.Entries, err = vol.List(cancel, func(done, total int64) {
					if time.Since(last) >= 2*time.Second {
						last = time.Now()
						writeBackupLog(fmt.Sprintf("Listing %s: %d of %d MB of the file table read", v.Display, done>>20, total>>20))
					}
				})
				if err != nil {
					return nil, nil, fmt.Errorf("%s: %w", v.Display, err)
				}
				v.Entries = withoutSystemFolders(v.Entries)
				writeBackupLog(fmt.Sprintf("Listed %s: %d entries in %v", v.Display, len(v.Entries), time.Since(start).Round(time.Millisecond)))
			}
			vols = append(vols, v)
			opened[v.Display] = vol
		}
	}
	if len(skipped) > 0 {
		writeBackupLog("Disk image partitions not browsable (only NTFS is read): " + strings.Join(skipped, "; "))
	}
	if len(vols) == 0 {
		return nil, nil, fmt.Errorf("this disk image has no NTFS volume to browse (found: %s)", strings.Join(skipped, "; "))
	}
	return vols, opened, nil
}

// withoutSystemFolders leaves out "System Volume Information" (shadow copy
// storage, restore points: often gigabytes, never wanted back, and never in a
// folder backup), keeping the entries' order.
func withoutSystemFolders(entries []imagefs.Entry) []imagefs.Entry {
	const svi = "system volume information"
	out := entries[:0]
	for _, e := range entries {
		p := strings.ToLower(e.Path)
		if p == svi || strings.HasPrefix(p, svi+"/") {
			continue
		}
		out = append(out, e)
	}
	return out
}

// listImageSnapshot is assembleSnapshotTree for a disk-image snapshot.
func listImageSnapshot(opts RestoreOptions, cancel func() bool) ([]SnapshotEntry, error) {
	s, err := openImageSession(opts, cancel)
	if err != nil {
		return nil, err
	}
	defer s.close()
	vols, _, err := openImageVolumes(s, opts, true, cancel)
	if err != nil {
		return nil, err
	}
	imageListing.Lock()
	imageListing.key, imageListing.vols = buildSnapshotCacheKey(opts), vols
	imageListing.Unlock()

	var out []SnapshotEntry
	for _, v := range vols {
		out = append(out, SnapshotEntry{Path: v.Display, IsDir: true})
		for _, e := range v.Entries {
			out = append(out, SnapshotEntry{Path: v.Display + "/" + e.Path, IsDir: e.IsDir, Size: e.Size, ModTime: imagefs.UnixSeconds(e.MTime)})
		}
	}
	return out, nil
}

// pickEntries returns the entries a selection of rel (a volume-relative
// path, "" for the whole volume) covers: the entry itself and, for a folder,
// everything below it. entries are sorted by path.
func pickEntries(entries []imagefs.Entry, rel string) []imagefs.Entry {
	if rel == "" {
		return entries
	}
	var out []imagefs.Entry
	i := sort.Search(len(entries), func(i int) bool { return entries[i].Path >= rel })
	if i < len(entries) && entries[i].Path == rel {
		out = append(out, entries[i])
	}
	prefix := rel + "/"
	j := sort.Search(len(entries), func(i int) bool { return entries[i].Path >= prefix })
	for ; j < len(entries) && strings.HasPrefix(entries[j].Path, prefix); j++ {
		out = append(out, entries[j])
	}
	return out
}

// restoreImageSnapshot is RestoreSnapshotInline for a disk-image snapshot.
// Files go to another folder (abs: dest/<volume>/<path>; flat: below the
// selection's common folder), or, for a caller that gives each volume a root
// (ArchiveRoots keyed by volume name, Mode original), under that root.
// Modification and creation times are restored; with RestoreACLs the
// read-only, hidden, system and archive attributes too. Permissions (ACLs)
// are not read from the image yet.
func restoreImageSnapshot(opts RestoreOptions, mode RestoreMode, progress func(float64, string), stage func(string, string)) error {
	if mode == RestoreModeOriginal && opts.ArchiveRoots == nil {
		return fmt.Errorf("files from a disk image are restored to another folder: choose \"alternate location\"")
	}
	var rewriter pbscommon.PathRewriter
	if mode != RestoreModeOriginal {
		var err error
		if rewriter, err = buildPathRewriter(opts, nil); err != nil {
			return err
		}
	}
	cancelled := func() bool { return opts.Ctx != nil && opts.Ctx.Err() != nil }

	stage("locating", "")
	progress(0.03, "Opening the disk image...")
	s, err := openImageSession(opts, cancelled)
	if err != nil {
		return err
	}
	defer s.close()

	// Volumes and their file lists: from the browse just done, else read now.
	key := buildSnapshotCacheKey(opts)
	imageListing.Lock()
	var vols []imageVolume
	if imageListing.key == key {
		vols = imageListing.vols
	}
	imageListing.Unlock()
	_, opened, err := openImageVolumes(s, opts, false, cancelled)
	if err != nil {
		return err
	}
	if vols == nil {
		progress(0.04, "Reading the disk image's file tables...")
		if vols, opened, err = openImageVolumes(s, opts, true, cancelled); err != nil {
			return err
		}
	}

	// What to restore, and where each item goes. Three kinds of selection:
	//   - IncludePaths from the Restore tree: "<volume>/<path>", to another
	//     folder through the usual rewriter (abs or flat);
	//   - ArchiveIncludes keyed by a machine set's folder base ("drive-sata0
	//     .img.fidx@<offset>:<folder>", Undelete and Roll back): paths
	//     relative to that folder, back into it (ArchiveRoots, Mode original)
	//     or to dest/<folder name>/...;
	//   - ArchiveIncludes keyed by a volume name, like the tree.
	// No selection means everything.
	type pick struct {
		vol   *imageVolume
		entry imagefs.Entry
		dest  string
	}
	var picks []pick
	seen := map[string]bool{}
	add := func(v *imageVolume, rel string, dest func(e imagefs.Entry) string) {
		for _, e := range pickEntries(v.Entries, rel) {
			if k := v.Display + "/" + e.Path; !seen[k] {
				seen[k] = true
				if d := dest(e); d != "" {
					picks = append(picks, pick{v, e, d})
				}
			}
		}
	}
	viaRewriter := func(v *imageVolume) func(imagefs.Entry) string {
		return func(e imagefs.Entry) string {
			if rewriter == nil {
				return ""
			}
			return rewriter(v.Display + "/" + e.Path)
		}
	}
	volumeAt := func(archive string, offset int64) *imageVolume {
		for i := range vols {
			if vols[i].Archive == archive && vols[i].Start == offset {
				return &vols[i]
			}
		}
		return nil
	}
	includes := pbscommon.NormalizeIncludes(opts.IncludePaths)
	switch {
	case opts.ArchiveIncludes != nil:
		for key, rels := range opts.ArchiveIncludes {
			if ib, ok := parseImageBase(key); ok {
				v := volumeAt(ib.archive, ib.offset)
				if v == nil {
					return fmt.Errorf("%s: no NTFS volume at %d in the disk image", ib.archive, ib.offset)
				}
				sub := exactCasePath(v.Entries, ib.sub)
				root := opts.ArchiveRoots[key]
				folderName := v.Display
				if sub != "" {
					folderName = sub[strings.LastIndexByte(sub, '/')+1:]
				}
				dest := func(e imagefs.Entry) string {
					rel := e.Path
					if sub != "" {
						rel = strings.TrimPrefix(e.Path, sub+"/")
					}
					if mode == RestoreModeOriginal {
						if root == "" {
							return ""
						}
						return filepath.Join(filepath.Clean(root), filepath.FromSlash(rel))
					}
					return rewriter(folderName + "/" + rel)
				}
				for _, rel := range rels {
					rel = strings.Trim(strings.ReplaceAll(rel, "\\", "/"), "/")
					if sub != "" {
						rel = sub + "/" + rel
					}
					add(v, rel, dest)
				}
				continue
			}
			for i := range vols {
				if vols[i].Display != key {
					continue
				}
				v := &vols[i]
				dest := viaRewriter(v)
				if mode == RestoreModeOriginal {
					root := opts.ArchiveRoots[key]
					dest = func(e imagefs.Entry) string {
						if root == "" {
							return ""
						}
						return filepath.Join(filepath.Clean(root), filepath.FromSlash(e.Path))
					}
				}
				for _, rel := range rels {
					add(v, strings.Trim(strings.ReplaceAll(rel, "\\", "/"), "/"), dest)
				}
			}
		}
	case len(includes) == 0:
		for i := range vols {
			add(&vols[i], "", viaRewriter(&vols[i]))
		}
	default:
		for i := range vols {
			v := &vols[i]
			for _, inc := range includes {
				inc = strings.ReplaceAll(inc, "\\", "/")
				if inc == v.Display {
					add(v, "", viaRewriter(v))
				} else if strings.HasPrefix(inc, v.Display+"/") {
					add(v, strings.TrimPrefix(inc, v.Display+"/"), viaRewriter(v))
				}
			}
		}
	}
	if len(picks) == 0 {
		return fmt.Errorf("restore matched no files for the requested path(s): %s", strings.Join(opts.IncludePaths, ", "))
	}

	// Open the files in MFT order (their records are read in sequence),
	// then let read-ahead run over exactly the clusters they occupy.
	type job struct {
		pick
		dest string
		f    *imagefs.File
		at   int64 // first byte on the disk, for reading in disk order
	}
	var jobs []job
	var dirs []job
	var total int64
	sort.SliceStable(picks, func(i, j int) bool {
		if picks[i].vol != picks[j].vol {
			return picks[i].vol.Display < picks[j].vol.Display
		}
		return picks[i].entry.Record < picks[j].entry.Record
	})
	ranges := map[string][][2]int64{}
	var extracted []pbscommon.PXARExtractedFile
	for n, p := range picks {
		if cancelled() {
			return fmt.Errorf("restore cancelled")
		}
		dest := p.dest
		if p.entry.IsDir {
			dirs = append(dirs, job{pick: p, dest: dest})
			continue
		}
		f, err := opened[p.vol.Display].Open(p.entry.Record, int64(p.entry.Size))
		if err != nil {
			extracted = append(extracted, pbscommon.PXARExtractedFile{Path: dest, ArchivePath: p.vol.Display + "/" + p.entry.Path,
				Size: p.entry.Size, Skipped: true, SkipReason: err.Error()})
			continue
		}
		j := job{pick: p, dest: dest, f: f, at: -1}
		for _, x := range f.Extents() {
			abs := [2]int64{p.vol.Start + x[0], p.vol.Start + x[1]}
			ranges[p.vol.Archive] = append(ranges[p.vol.Archive], abs)
			if j.at < 0 {
				j.at = abs[0]
			}
		}
		total += int64(p.entry.Size)
		jobs = append(jobs, j)
		if n%2000 == 0 {
			progress(0.05, fmt.Sprintf("Locating files in the disk image: %d of %d", n, len(picks)))
		}
	}
	for a, rs := range ranges {
		n := s.readers[a].LimitPrefetchToRanges(rs)
		writeBackupLog(fmt.Sprintf("Image restore: %d files, %s, in %d chunks of %s", len(jobs), formatByteSize(uint64(total)), n, a))
	}
	sort.SliceStable(jobs, func(i, k int) bool { return jobs[i].at < jobs[k].at })

	extractEnd := 0.95
	if opts.VerifyAfterRestore {
		extractEnd = 0.80
	}
	stage("transferring", "")

	for _, d := range dirs {
		if err := os.MkdirAll(d.dest, 0o755); err != nil {
			extracted = append(extracted, pbscommon.PXARExtractedFile{Path: d.dest, IsDir: true, Skipped: true, SkipReason: err.Error()})
			continue
		}
		extracted = append(extracted, pbscommon.PXARExtractedFile{Path: d.dest, ArchivePath: d.vol.Display + "/" + d.entry.Path,
			IsDir: true, ModTime: imagefs.UnixSeconds(d.entry.MTime), ModNanos: filetimeNanos(d.entry.MTime)})
	}

	var written int64
	lastEmit := time.Time{}
	buf := make([]byte, 1<<20)
	for _, j := range jobs {
		if cancelled() {
			return fmt.Errorf("restore cancelled")
		}
		if opts.OnFile != nil {
			opts.OnFile(j.vol.Display + "/" + j.entry.Path)
		}
		ef := writeImageFile(opts, j.dest, j.f, j.entry, buf, func(n int64) {
			written += n
			if time.Since(lastEmit) >= 250*time.Millisecond {
				lastEmit = time.Now()
				frac := 1.0
				if total > 0 {
					frac = float64(written) / float64(total)
				}
				progress(0.05+(extractEnd-0.05)*frac, fmt.Sprintf("Restoring from the disk image: %s of %s written", formatByteSize(uint64(written)), formatByteSize(uint64(total))))
				if opts.OnStats != nil {
					opts.OnStats(&RestoreProgressStats{BytesDone: uint64(written), BytesTotal: uint64(total), CurrentArchive: j.vol.Display})
				}
			}
		})
		ef.ArchivePath = j.vol.Display + "/" + j.entry.Path
		extracted = append(extracted, ef)
	}
	if opts.OnStats != nil {
		opts.OnStats(&RestoreProgressStats{BytesDone: uint64(written), BytesTotal: uint64(total)})
	}

	// Creation times (and attributes) of the folders; their modification
	// times are set by finishExtraction, deepest first, after everything.
	for _, d := range dirs {
		if d.entry.Created > 0 {
			_ = applyCreationTime(d.dest, d.entry.Created)
		}
	}

	errorSkipCount, verifyFailures, err := finishExtraction(opts, extracted, extractEnd, progress, stage)
	if err != nil {
		return err
	}
	if opts.RestoreACLs {
		writeBackupLog("NOTE: permissions (ACLs) are not restored from a disk image yet; attributes and times are")
	}
	progress(1.0, "Restore completed")
	if errorSkipCount > 0 {
		return fmt.Errorf("restore completed with %d failed files (see logs)", errorSkipCount)
	}
	if len(verifyFailures) > 0 {
		return fmt.Errorf("restore finished but %d restored file(s) failed verification against the snapshot (see logs)", len(verifyFailures))
	}
	return nil
}

// writeImageFile writes one file from the image to dest through a .part
// file, then sets its times (and attributes with RestoreACLs). A file that
// exists is left alone unless Overwrite. Errors are reported in the result
// (Skipped), as the pxar extractor does, so one bad file does not stop the rest.
func writeImageFile(opts RestoreOptions, dest string, f *imagefs.File, e imagefs.Entry, buf []byte, onWrite func(int64)) pbscommon.PXARExtractedFile {
	res := pbscommon.PXARExtractedFile{Path: dest, Size: e.Size, ModTime: imagefs.UnixSeconds(e.MTime), ModNanos: filetimeNanos(e.MTime)}
	skip := func(expected bool, why string) pbscommon.PXARExtractedFile {
		res.Skipped, res.Expected, res.SkipReason = true, expected, why
		return res
	}
	if _, err := os.Lstat(dest); err == nil && !opts.Overwrite {
		return skip(true, "already exists")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return skip(false, err.Error())
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".proxmox-*.part")
	if err != nil {
		return skip(false, err.Error())
	}
	var h hash.Hash
	var w io.Writer = tmp
	if opts.VerifyAfterRestore {
		h = sha256.New()
		w = io.MultiWriter(tmp, h)
	}
	_, cerr := io.CopyBuffer(w, &progressReader{r: io.NewSectionReader(f, 0, f.Size()), onRead: onWrite}, buf)
	if err := tmp.Close(); cerr == nil {
		cerr = err
	}
	if cerr != nil {
		os.Remove(tmp.Name())
		return skip(false, cerr.Error())
	}
	clearReadOnly(dest)
	if err := os.Rename(tmp.Name(), dest); err != nil {
		os.Remove(tmp.Name())
		return skip(false, err.Error())
	}
	mt := time.Unix(res.ModTime, int64(res.ModNanos))
	if err := os.Chtimes(dest, mt, mt); err != nil {
		writeBackupLog(fmt.Sprintf("Time not set on %s: %v", dest, err))
	}
	if e.Created > 0 {
		_ = applyCreationTime(dest, e.Created)
	}
	if opts.RestoreACLs {
		applyFileAttributes(dest, e.Attrs)
	}
	if h != nil {
		res.SHA256 = h.Sum(nil)
	}
	return res
}

// filetimeNanos is the sub-second part of a FILETIME, in nanoseconds.
func filetimeNanos(ft int64) uint32 {
	if ft <= 0 {
		return 0
	}
	return uint32((ft % 10000000) * 100)
}

type progressReader struct {
	r      io.Reader
	onRead func(int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 && p.onRead != nil {
		p.onRead(int64(n))
	}
	return n, err
}

// exactCasePath returns sub as the volume spells it (NTFS names are
// case-insensitive; a folder typed in a set may differ in case), or sub
// unchanged when the volume has no such folder.
func exactCasePath(entries []imagefs.Entry, sub string) string {
	if sub == "" {
		return ""
	}
	want := strings.ToLower(sub)
	for _, e := range entries {
		if e.IsDir && len(e.Path) == len(sub) && strings.ToLower(e.Path) == want {
			return e.Path
		}
	}
	return sub
}
