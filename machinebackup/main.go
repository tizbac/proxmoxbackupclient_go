package main

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"log"
	"os"
	"pbscommon"
	"snapshot"
	"strings"
	"sync/atomic"

	"github.com/cornelk/hashmap"
	"github.com/shirou/gopsutil/disk"
)

var defaultMailSubjectTemplate = "Backup {{.Status}}"
var defaultMailBodyTemplate = `{{if .Success}}Backup complete ({{.FromattedDuration}})
Chunks New {{.NewChunks}}, Reused {{.ReusedChunks}}.{{else}}Error occurred while working, backup may be not completed.
Last error is: {{.ErrorStr}}{{end}}`

var didxMagic = []byte{28, 145, 78, 165, 25, 186, 179, 205}

type ChunkState struct {
	assignments        []string
	assignments_offset []uint64
	pos                uint64
	wrid               uint64
	chunkcount         uint64
	chunkdigests       hash.Hash
	current_chunk      []byte
	C                  pbscommon.Chunker
	newchunk           *atomic.Uint64
	reusechunk         *atomic.Uint64
	knownChunks        *hashmap.Map[string, bool]
}

func (c *ChunkState) Init(newchunk *atomic.Uint64, reusechunk *atomic.Uint64, knownChunks *hashmap.Map[string, bool]) {
	c.assignments = make([]string, 0)
	c.assignments_offset = make([]uint64, 0)
	c.pos = 0
	c.chunkcount = 0
	c.chunkdigests = sha256.New()
	c.current_chunk = make([]byte, 0)
	c.C = pbscommon.Chunker{}
	c.C.New(1024 * 1024 * 4)
	c.reusechunk = reusechunk
	c.newchunk = newchunk
	c.knownChunks = knownChunks
}

func main() {
	/*	var newchunk *atomic.Uint64 = new(atomic.Uint64)
		var reusechunk *atomic.Uint64 = new(atomic.Uint64)*/

	/*cfg := loadConfig()

	if ok := cfg.valid(); !ok {
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
	}

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

	//insecure := cfg.CertFingerprint != ""

	/*client := &pbscommon.PBSClient{
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
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("Failed to retrieve hostname:", err)
		hostname = "unknown"
	}

	begin := time.Now()*/

	partitions, err := disk.Partitions(false) // false means don't include virtual partitions
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
	}
}
