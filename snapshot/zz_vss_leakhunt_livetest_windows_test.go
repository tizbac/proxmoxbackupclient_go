//go:build windows

package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/st-matskevich/go-vss"
	"golang.org/x/sys/windows"
)

// Opt-in (PBS_VSS_VOL and PBS_VSS_HUNT=<snapshots>; run elevated): finds
// where the handles a snapshot leaves behind come from. For each way of
// taking snapshots it reports, over that many snapshots, how the process's
// handles changed by object type, how many of its thread handles belong to
// threads that have ended (and which module started those threads), and how
// its live threads changed by the module that started them.
//
//   - as the backups take them: CreateVSSSnapshot, which locks a thread and
//     starts and ends COM for every snapshot
//   - on one long-lived thread with COM started once, as a requester that
//     keeps one VSS thread would
func TestLiveVSSLeakHunt(t *testing.T) {
	vol := os.Getenv("PBS_VSS_VOL")
	n, _ := strconv.Atoi(os.Getenv("PBS_VSS_HUNT"))
	if vol == "" || n <= 0 {
		t.Skip("set PBS_VSS_VOL and PBS_VSS_HUNT")
	}
	whole := func() {
		if err := CreateVSSSnapshot([]string{vol}, false, func(map[string]SnapShot) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	whole() // the first loads the VSS libraries
	whole()

	hunt := func(name string, step func()) {
		before := takeProcessCensus(t)
		for i := 0; i < n; i++ {
			step()
		}
		after := takeProcessCensus(t)
		t.Logf("=== %s x%d: handles %d -> %d (%+.1f each), live threads %d -> %d",
			name, n, before.handles, after.handles, float64(after.handles-before.handles)/float64(n), before.threads, after.threads)
		t.Logf("    handle types: %s", diffCounts(before.byType, after.byType))
		t.Logf("    thread handles to ended threads, by starting module: %s", diffCounts(before.deadThreadHandles, after.deadThreadHandles))
		t.Logf("    live threads by starting module: %s", diffCounts(before.liveThreads, after.liveThreads))
	}

	hunt("CreateVSSSnapshot (COM per snapshot)", whole)

	// One locked thread for every VSS call, COM started once.
	jobs := make(chan func())
	done := make(chan struct{})
	go func() {
		defer close(done)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		uninit, err := initCOM()
		if err != nil {
			t.Error(err)
			for range jobs {
			}
			return
		}
		defer uninit()
		for f := range jobs {
			f()
		}
	}()
	onThread := func(f func()) {
		finished := make(chan struct{})
		jobs <- func() { f(); close(finished) }
		<-finished
	}
	oneThread := func() {
		onThread(func() {
			s, err := createShadow(vol, vssTimeoutMs)
			if err != nil {
				t.Error(err)
				return
			}
			if err := s.release(); err != nil {
				t.Error(err)
			}
			deleteShadow(s.Id)
		})
	}
	oneThread()
	hunt("one COM thread for all", oneThread)
	close(jobs)
	<-done
}

// Opt-in (PBS_VSS_VOL, PBS_VSS_REFS=1; run elevated): the VSS object's
// reference count after each step of createShadow and release, the way they
// make and free a shadow copy. VSS stops the thread it starts for the object,
// and closes the handles that go with it, only when the count reaches zero.
func TestLiveVSSRefCounts(t *testing.T) {
	vol := os.Getenv("PBS_VSS_VOL")
	if vol == "" || os.Getenv("PBS_VSS_REFS") != "1" {
		t.Skip("set PBS_VSS_VOL and PBS_VSS_REFS=1")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	uninit, err := initCOM()
	if err != nil {
		t.Fatal(err)
	}
	defer uninit()
	c, err := newVSSComponents()
	if err != nil {
		t.Fatal(err)
	}
	refs := func(step string) {
		c.AddRef()
		t.Logf("%-40s refs %d", step, c.Release())
	}
	refs("newVSSComponents")
	must := func(step string, err error) {
		if err != nil {
			t.Fatalf("%s: %v", step, err)
		}
		refs(step)
	}
	must("SetContext", c.SetContext(vss.VSS_CTX_BACKUP))
	must("SetBackupState", c.SetBackupState(false, false, vss.VSS_BT_COPY, false))
	// PBS_VSS_NOSTATUS=1 leaves out GatherWriterStatus and FreeWriterStatus.
	a, err := c.GatherWriterMetadata()
	must("GatherWriterMetadata", waitVSS("gather", a, err, vssTimeoutMs))
	must("FreeWriterMetadata", c.FreeWriterMetadata())
	var provider ole.GUID
	if _, err := c.IsVolumeSupported(provider, vol); err != nil {
		t.Fatal(err)
	}
	refs("IsVolumeSupported")
	var setID, snapID ole.GUID
	must("StartSnapshotSet", c.StartSnapshotSet(&setID))
	must("AddToSnapshotSet", c.AddToSnapshotSet(vol, provider, &snapID))
	a, err = c.PrepareForBackup()
	must("PrepareForBackup", waitVSS("prepare", a, err, vssTimeoutMs))
	a, err = c.DoSnapshotSet()
	must("DoSnapshotSet", waitVSS("snapshot", a, err, vssTimeoutMs))
	d, err := queriedSnapshotDetails(c, snapID)
	if err != nil || d.DeviceObject == "" {
		t.Fatalf("Query: device %q, %v", d.DeviceObject, err)
	}
	refs("Query")
	a, err = c.BackupComplete()
	must("BackupComplete", waitVSS("complete", a, err, vssTimeoutMs))
	if os.Getenv("PBS_VSS_NOSTATUS") != "1" {
		a, err = c.GatherWriterStatus()
		must("GatherWriterStatus", waitVSS("status", a, err, vssTimeoutMs))
		must("FreeWriterStatus", c.FreeWriterStatus())
	}
	t.Logf("%-40s refs %d (0 frees the object)", "final Release", c.Release())
	deleteShadow(snapID.String())
}

// processCensus is what takeProcessCensus counts in this process.
type processCensus struct {
	handles           int
	threads           int
	byType            map[string]int // open handles by object type
	deadThreadHandles map[string]int // handles to ended threads, by the module that started them
	liveThreads       map[string]int // live threads by the module that started them
}

var (
	ntdll                    = windows.NewLazySystemDLL("ntdll.dll")
	procNtQueryObject        = ntdll.NewProc("NtQueryObject")
	procNtQueryInfoThread    = ntdll.NewProc("NtQueryInformationThread")
	kernel32Hunt             = windows.NewLazySystemDLL("kernel32.dll")
	procGetModuleHandleExW   = kernel32Hunt.NewProc("GetModuleHandleExW")
	procGetExitCodeThreadHnt = kernel32Hunt.NewProc("GetExitCodeThread")
)

const (
	processHandleInformation   = 51 // PROCESSINFOCLASS ProcessHandleInformation
	objectTypeInformation      = 2  // OBJECT_INFORMATION_CLASS
	threadQuerySetWin32StartAd = 9  // THREADINFOCLASS ThreadQuerySetWin32StartAddress
	stillActive                = 259
)

// handleEntry is PROCESS_HANDLE_TABLE_ENTRY_INFO (64-bit).
type handleEntry struct {
	HandleValue      uintptr
	HandleCount      uintptr
	PointerCount     uintptr
	GrantedAccess    uint32
	ObjectTypeIndex  uint32
	HandleAttributes uint32
	Reserved         uint32
}

func takeProcessCensus(t *testing.T) processCensus {
	t.Helper()
	c := processCensus{byType: map[string]int{}, deadThreadHandles: map[string]int{}, liveThreads: map[string]int{}}
	self := windows.CurrentProcess()

	// The handle table.
	buf := make([]byte, 1<<20)
	var got uint32
	for {
		err := windows.NtQueryInformationProcess(self, processHandleInformation, unsafe.Pointer(&buf[0]), uint32(len(buf)), &got)
		if err == nil {
			break
		}
		if len(buf) > 64<<20 {
			t.Fatalf("handle table: %v", err)
		}
		buf = make([]byte, len(buf)*2)
	}
	count := int(*(*uintptr)(unsafe.Pointer(&buf[0])))
	entries := unsafe.Slice((*handleEntry)(unsafe.Pointer(&buf[2*unsafe.Sizeof(uintptr(0))])), count)
	typeNames := map[uint32]string{}
	for _, e := range entries {
		name, ok := typeNames[e.ObjectTypeIndex]
		if !ok {
			name = objectTypeName(windows.Handle(e.HandleValue))
			typeNames[e.ObjectTypeIndex] = name
		}
		c.byType[name]++
		c.handles++
		if name == "Thread" {
			var code uint32
			if r, _, _ := procGetExitCodeThreadHnt.Call(e.HandleValue, uintptr(unsafe.Pointer(&code))); r != 0 && code != stillActive {
				c.deadThreadHandles[threadStartModule(windows.Handle(e.HandleValue))]++
			}
		}
	}

	// The live threads.
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		t.Fatalf("thread snapshot: %v", err)
	}
	defer windows.CloseHandle(snap)
	pid := windows.GetCurrentProcessId()
	var te windows.ThreadEntry32
	te.Size = uint32(unsafe.Sizeof(te))
	for err = windows.Thread32First(snap, &te); err == nil; err = windows.Thread32Next(snap, &te) {
		if te.OwnerProcessID != pid {
			continue
		}
		c.threads++
		h, oerr := windows.OpenThread(windows.THREAD_QUERY_INFORMATION, false, te.ThreadID)
		if oerr != nil {
			c.liveThreads["(cannot open)"]++
			continue
		}
		c.liveThreads[threadStartModule(h)]++
		windows.CloseHandle(h)
	}
	return c
}

// objectTypeName is the object type of h ("Thread", "Event", "ALPC Port").
func objectTypeName(h windows.Handle) string {
	buf := make([]byte, 4096)
	var got uint32
	r, _, _ := procNtQueryObject.Call(uintptr(h), objectTypeInformation, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&got)))
	if r != 0 {
		return fmt.Sprintf("(type? 0x%x)", r)
	}
	us := (*windows.NTUnicodeString)(unsafe.Pointer(&buf[0]))
	return us.String()
}

// threadStartModule names the module whose code a thread started in
// ("vssapi.dll", "rpcrt4.dll", the test binary for Go's own threads).
func threadStartModule(h windows.Handle) string {
	var start uintptr
	r, _, _ := procNtQueryInfoThread.Call(uintptr(h), threadQuerySetWin32StartAd, uintptr(unsafe.Pointer(&start)), unsafe.Sizeof(start), 0)
	if r != 0 {
		return "(start unknown)"
	}
	var mod windows.Handle
	const fromAddress, unchangedRefcount = 0x4, 0x2
	if ok, _, _ := procGetModuleHandleExW.Call(fromAddress|unchangedRefcount, start, uintptr(unsafe.Pointer(&mod))); ok == 0 {
		return fmt.Sprintf("(no module %#x)", start)
	}
	name := make([]uint16, windows.MAX_PATH)
	n, err := windows.GetModuleFileName(mod, &name[0], uint32(len(name)))
	if err != nil || n == 0 {
		return "(module name?)"
	}
	return strings.ToLower(filepath.Base(windows.UTF16ToString(name[:n])))
}

// diffCounts lists what changed between two counts, largest change first.
func diffCounts(before, after map[string]int) string {
	type d struct {
		k string
		v int
	}
	var ds []d
	for k := range union(before, after) {
		if v := after[k] - before[k]; v != 0 {
			ds = append(ds, d{k, v})
		}
	}
	if len(ds) == 0 {
		return "no change"
	}
	sort.Slice(ds, func(i, j int) bool { return abs(ds[i].v) > abs(ds[j].v) })
	parts := make([]string, len(ds))
	for i, x := range ds {
		parts[i] = fmt.Sprintf("%s %+d", x.k, x.v)
	}
	return strings.Join(parts, ", ")
}

func union(a, b map[string]int) map[string]struct{} {
	u := map[string]struct{}{}
	for k := range a {
		u[k] = struct{}{}
	}
	for k := range b {
		u[k] = struct{}{}
	}
	return u
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
