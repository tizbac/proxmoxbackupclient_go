package pbscommon

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// flakyFile is a real file whose reads fail once the position reaches failAt,
// like a handle on a disk that has just dropped off the bus.
type flakyFile struct {
	f      *os.File
	pos    int64
	failAt int64 // -1: never fails
	err    error
}

func (ff *flakyFile) Read(p []byte) (int, error) {
	if ff.failAt >= 0 {
		if ff.pos >= ff.failAt {
			return 0, ff.err
		}
		if room := ff.failAt - ff.pos; int64(len(p)) > room {
			p = p[:room]
		}
	}
	n, err := ff.f.Read(p)
	ff.pos += int64(n)
	return n, err
}

func (ff *flakyFile) Seek(off int64, whence int) (int64, error) {
	n, err := ff.f.Seek(off, whence)
	ff.pos = n
	return n, err
}

func (ff *flakyFile) Close() error { return ff.f.Close() }

// sourceSim drives the test seams: which opens fail, when the folder is
// unreachable, and a fake clock that sleeping advances.
type sourceSim struct {
	mu sync.Mutex
	// opens counts opens per base name; plan decides each open's behaviour.
	opens map[string]int
	plan  func(name string, open int) (openErr error, failAt int64, readErr error)
	// unreachableFor makes the folder unreachable for this many stat calls
	// once the first failure has happened (-1: forever).
	unreachableFor int
	dropped        bool
	now            time.Time
	sleeps         int
	notices        []string
}

func installSourceSim(t *testing.T, sim *sourceSim) {
	t.Helper()
	sim.opens = map[string]int{}
	sim.now = time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC)
	oldOpen, oldStat, oldSleep, oldNow := openForRead, sourceStat, sourceSleep, sourceNow
	t.Cleanup(func() { openForRead, sourceStat, sourceSleep, sourceNow = oldOpen, oldStat, oldSleep, oldNow })
	openForRead = func(path string) (io.ReadSeekCloser, error) {
		sim.mu.Lock()
		name := filepath.Base(path)
		n := sim.opens[name]
		sim.opens[name]++
		openErr, failAt, readErr := sim.plan(name, n)
		if openErr != nil || readErr != nil {
			sim.dropped = true
		}
		sim.mu.Unlock()
		if openErr != nil {
			return nil, openErr
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return &flakyFile{f: f, failAt: failAt, err: readErr}, nil
	}
	sourceStat = func(path string) (os.FileInfo, error) {
		sim.mu.Lock()
		if sim.dropped && sim.unreachableFor != 0 {
			if sim.unreachableFor > 0 {
				sim.unreachableFor--
			}
			sim.mu.Unlock()
			return nil, testDeviceGoneErr
		}
		sim.mu.Unlock()
		return os.Stat(path)
	}
	sourceSleep = func(ctx context.Context, d time.Duration) error {
		if ctx != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		sim.mu.Lock()
		sim.now = sim.now.Add(d)
		sim.sleeps++
		sim.mu.Unlock()
		return nil
	}
	sourceNow = func() time.Time { sim.mu.Lock(); defer sim.mu.Unlock(); return sim.now }
}

// sourceTree writes a.txt, big.bin (3 MB) and z.txt.
func sourceTree(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	root := t.TempDir()
	big := make([]byte, 3*1024*1024)
	rand.Read(big)
	files := map[string][]byte{"a.txt": []byte("first\n"), "big.bin": big, "z.txt": []byte("last\n")}
	for n, d := range files {
		if err := os.WriteFile(filepath.Join(root, n), d, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root, files
}

func backupWithSim(t *testing.T, root string, sim *sourceSim, ctx context.Context) (*PXARArchive, []byte, error) {
	t.Helper()
	var out bytes.Buffer
	a := &PXARArchive{ArchiveName: "test.pxar.didx", Ctx: ctx}
	a.WriteCB = func(b []byte) error { out.Write(b); return nil }
	a.OnNotice = func(m string) { sim.notices = append(sim.notices, m) }
	_, err := a.WriteDir(root, "", true)
	return a, out.Bytes(), err
}

func restoreArchive(t *testing.T, stream []byte) string {
	t.Helper()
	dest := t.TempDir()
	if _, err := NewPXARReader(stream).ExtractAll(dest); err != nil {
		t.Fatalf("archive does not restore: %v", err)
	}
	return dest
}

const failOffset = 1024*1024 + 4096

func TestSourceDropMidFileRecovers(t *testing.T) {
	root, files := sourceTree(t)
	sim := &sourceSim{unreachableFor: 3}
	sim.plan = func(name string, open int) (error, int64, error) {
		if name == "big.bin" && open == 0 {
			return nil, failOffset, testDeviceGoneErr
		}
		return nil, -1, nil
	}
	installSourceSim(t, sim)

	a, stream, err := backupWithSim(t, root, sim, nil)
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}
	if len(a.ReadErrors) != 0 {
		t.Fatalf("recovered drop still recorded read errors: %v", a.ReadErrors)
	}
	if sim.opens["big.bin"] != 2 {
		t.Errorf("big.bin opened %d times, want 2 (original + one reopen)", sim.opens["big.bin"])
	}
	if sim.sleeps < 4 {
		t.Errorf("waited %d times, want at least 4 (3 unreachable checks + the retry)", sim.sleeps)
	}
	if len(sim.notices) != 2 || !strings.Contains(sim.notices[0], "became unavailable") || !strings.Contains(sim.notices[1], "Source back") {
		t.Errorf("notices %q", sim.notices)
	}
	dest := restoreArchive(t, stream)
	for n, want := range files {
		got, _ := os.ReadFile(filepath.Join(dest, n))
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs after a recovered drop (%d bytes, want %d)", n, len(got), len(want))
		}
	}
}

func TestSourceNeverReturnsFailsBackup(t *testing.T) {
	root, _ := sourceTree(t)
	sim := &sourceSim{unreachableFor: -1}
	sim.plan = func(name string, open int) (error, int64, error) {
		if name == "big.bin" && open == 0 {
			return nil, failOffset, testDeviceGoneErr
		}
		return nil, -1, nil
	}
	installSourceSim(t, sim)

	_, _, err := backupWithSim(t, root, sim, nil)
	var lost *SourceLostError
	if !errors.As(err, &lost) {
		t.Fatalf("got %v, want a SourceLostError", err)
	}
	if elapsed := sim.now.Sub(time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC)); elapsed < sourceWaitWindow || elapsed > sourceWaitWindow+10*time.Second {
		t.Errorf("gave up after %s, want about %s", elapsed, sourceWaitWindow)
	}
	if sim.opens["z.txt"] != 0 {
		t.Errorf("walk carried on past a lost source")
	}
}

// A bad file on a disk that is still there: that file is zero-padded and
// flagged, everything else is backed up.
func TestUnreadableFileIsPaddedAndBackupContinues(t *testing.T) {
	root, files := sourceTree(t)
	sim := &sourceSim{}
	sim.plan = func(name string, open int) (error, int64, error) {
		if name == "big.bin" {
			return nil, failOffset, testDeviceGoneErr
		}
		return nil, -1, nil
	}
	installSourceSim(t, sim)

	a, stream, err := backupWithSim(t, root, sim, nil)
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}
	if len(a.ReadErrors) != 1 || !strings.Contains(a.ReadErrors[0], "Read failed after 1052672 of 3145728 bytes") {
		t.Fatalf("read errors %v", a.ReadErrors)
	}
	if got := sim.opens["big.bin"]; got != 1+sourceReachableAttempts {
		t.Errorf("big.bin opened %d times, want %d", got, 1+sourceReachableAttempts)
	}
	dest := restoreArchive(t, stream)
	got, _ := os.ReadFile(filepath.Join(dest, "big.bin"))
	want := append(append([]byte{}, files["big.bin"][:failOffset]...), make([]byte, len(files["big.bin"])-failOffset)...)
	if !bytes.Equal(got, want) {
		t.Errorf("big.bin is not the readable part plus zero padding")
	}
	for _, n := range []string{"a.txt", "z.txt"} {
		if g, _ := os.ReadFile(filepath.Join(dest, n)); !bytes.Equal(g, files[n]) {
			t.Errorf("%s not backed up after the bad file", n)
		}
	}
}

// An error that does not point at the device (a locked region, say) is not
// waited for; it used to fail the whole backup, now only that file.
func TestOtherReadErrorPadsWithoutWaiting(t *testing.T) {
	root, files := sourceTree(t)
	sim := &sourceSim{}
	sim.plan = func(name string, open int) (error, int64, error) {
		if name == "big.bin" {
			return nil, failOffset, errors.New("the process cannot access the file because another process has locked a portion of the file")
		}
		return nil, -1, nil
	}
	installSourceSim(t, sim)

	a, stream, err := backupWithSim(t, root, sim, nil)
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}
	if sim.sleeps != 0 || sim.opens["big.bin"] != 1 {
		t.Errorf("waited %d times, opened %d times; want no wait for a non-device error", sim.sleeps, sim.opens["big.bin"])
	}
	if len(a.ReadErrors) != 1 || !strings.Contains(a.ReadErrors[0], "locked a portion") {
		t.Fatalf("read errors %v", a.ReadErrors)
	}
	dest := restoreArchive(t, stream)
	if g, _ := os.ReadFile(filepath.Join(dest, "z.txt")); !bytes.Equal(g, files["z.txt"]) {
		t.Errorf("z.txt missing after the failed file")
	}
}

func TestSourceDropOnOpenRecovers(t *testing.T) {
	root, files := sourceTree(t)
	sim := &sourceSim{unreachableFor: 2}
	sim.plan = func(name string, open int) (error, int64, error) {
		if name == "z.txt" && open == 0 {
			return &os.PathError{Op: "open", Path: name, Err: testDeviceGoneErr}, 0, nil
		}
		return nil, -1, nil
	}
	installSourceSim(t, sim)

	a, stream, err := backupWithSim(t, root, sim, nil)
	if err != nil || len(a.ReadErrors) != 0 {
		t.Fatalf("err %v, read errors %v", err, a.ReadErrors)
	}
	dest := restoreArchive(t, stream)
	if g, _ := os.ReadFile(filepath.Join(dest, "z.txt")); !bytes.Equal(g, files["z.txt"]) {
		t.Errorf("z.txt not backed up after the source came back")
	}
}

func TestStopWhileWaitingForSource(t *testing.T) {
	root, _ := sourceTree(t)
	ctx, cancel := context.WithCancel(context.Background())
	sim := &sourceSim{unreachableFor: -1}
	sim.plan = func(name string, open int) (error, int64, error) {
		if name == "big.bin" && open == 0 {
			cancel() // the user presses Stop while the disk is gone
			return nil, failOffset, testDeviceGoneErr
		}
		return nil, -1, nil
	}
	installSourceSim(t, sim)

	_, _, err := backupWithSim(t, root, sim, ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want the stop to end the wait", err)
	}
}

func TestSourceUnavailableClassification(t *testing.T) {
	if !sourceUnavailable(`x`, testDeviceGoneErr) {
		t.Errorf("device error not classed as source unavailable")
	}
	if sourceUnavailable(`x`, os.ErrPermission) || sourceUnavailable(`x`, io.EOF) || sourceUnavailable(`x`, nil) {
		t.Errorf("ordinary errors classed as source unavailable")
	}
}
