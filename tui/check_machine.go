package tui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/salvanza/salvanza_client/gui/api"
)

// CheckMachine proves the disk image backups through the real service: the
// disk list, a one-off disk image backup, and a disk image Backup Set that
// leaves a partition out. Both go to a real Proxmox Backup Server, and
// everything made is removed again.
//
// The server comes from TUI_CHECK_PBS_* as for CheckEncryption. TUI_CHECK_DISK
// names the disk to back up (/dev/sdb, \\.\PhysicalDrive1). It must be a
// scratch disk: the check refuses a boot or system disk, and an image of a
// large disk is slow.
func CheckMachine(be Backend, stdout, stderr io.Writer) int {
	env := func(k string) string { return strings.TrimSpace(os.Getenv("TUI_CHECK_PBS_" + k)) }
	base, authID, secret, store, fp := env("URL"), env("AUTHID"), env("SECRET"), env("DATASTORE"), env("FINGERPRINT")
	devPath := strings.TrimSpace(os.Getenv("TUI_CHECK_DISK"))
	if base == "" || authID == "" || secret == "" || store == "" || devPath == "" {
		sayln(stderr, "machine check: set TUI_CHECK_PBS_URL, _AUTHID, _SECRET, _DATASTORE (and _FINGERPRINT) and TUI_CHECK_DISK (a scratch disk)")
		return 2
	}
	stamp := time.Now().Unix()
	srvID := fmt.Sprintf("tuiselftestmach%d", stamp)
	setID := fmt.Sprintf("tuiselftestmachset%d", stamp)
	oneOffGroup := fmt.Sprintf("tui-mach-%d", stamp)
	setGroup := fmt.Sprintf("tui-machset-%d", stamp)

	disks, err := be.Disks()
	if err != nil {
		sayln(stderr, "list disks: "+describeErr(err))
		return 1
	}
	var disk *Disk
	for i := range disks {
		if disks[i].DevicePath == devPath {
			disk = &disks[i]
		}
	}
	if disk == nil {
		sayln(stderr, fmt.Sprintf("the service does not list %s (it lists %d disks)", devPath, len(disks)))
		return 1
	}
	// The service's boot flag is only a guess on Linux (every sd and nvme disk
	// has it), so the guard is what the partitions say: nothing mounted, no
	// system or boot partition.
	for _, p := range disk.Partitions {
		if p.Mount != "" || p.Role != "" || disk.IsSystemDisk {
			sayln(stderr, devPath+" holds a mounted, system or boot partition: the check only runs on a scratch disk")
			return 1
		}
	}
	if len(disk.Partitions) < 2 {
		sayln(stderr, fmt.Sprintf("%s has %d partition(s) listed; the check needs at least 2 (one is left out)", devPath, len(disk.Partitions)))
		return 1
	}
	say(stdout, "disk list: %s %s, %d partitions listed, identity %s\n", devPath, human(float64(disk.Size)), len(disk.Partitions), disk.UniqueID)

	defer func() { _ = be.DeleteSet(setID) }()
	defer func() { _ = be.DeleteServer(srvID) }()
	defer deleteGroupHTTP(base, store, authID, secret, oneOffGroup)
	defer deleteGroupHTTP(base, store, authID, secret, setGroup)
	if err := be.SaveServer(map[string]interface{}{
		"id": srvID, "name": "__tui-selftest-machine__", "baseurl": base, "datastore": store, "certfingerprint": fp,
		"authid": authID, "secret": secret,
	}, true); err != nil {
		sayln(stderr, "save server: "+describeErr(err))
		return 1
	}

	// wait polls for the group's snapshot. It shows the Message Log lines that
	// come while it waits, and gives up at once when the backup says it could
	// not reach or write to the server (and stops the run), so a refusal is
	// seen in seconds and not after the retries.
	started := time.Now()
	shown := map[string]bool{}
	wait := func(group string) ([]SnapshotInfo, bool) {
		for deadline := time.Now().Add(6 * time.Minute); time.Now().Before(deadline); {
			time.Sleep(3 * time.Second)
			if snaps, err := be.RestoreSnapshots(srvID, Group{Type: "host", ID: group}); err == nil && len(snaps) > 0 {
				return snaps, true
			}
			msgs, _ := be.Messages()
			for _, m := range msgs {
				key := m.At.Format(time.RFC3339Nano) + m.Message
				if m.At.Before(started) || shown[key] {
					continue
				}
				shown[key] = true
				say(stdout, "  message log: %s %s\n", m.Level, m.Message)
				low := strings.ToLower(m.Message)
				if strings.Contains(low, "lost on attempt") || strings.Contains(low, "refused") || strings.Contains(low, "denied") || strings.Contains(low, "permission") {
					_ = be.Control("stop", "")
					return nil, false
				}
			}
		}
		return nil, false
	}

	// 1. the one-off disk image backup
	if err := be.StartMachineBackup(api.BackupRequest{
		BackupType: "machine", BackupID: oneOffGroup, DriveLetters: []string{disk.DevicePath}, DiskIDs: []string{disk.UniqueID},
		PBSServerID: srvID, Comment: "tui self-test one-off disk backup",
	}); err != nil {
		sayln(stderr, "one-off disk backup: "+describeErr(err))
		return 1
	}
	say(stdout, "one-off disk backup of %s started\n", devPath)
	if _, ok := wait(oneOffGroup); !ok {
		sayln(stderr, "the one-off disk backup did not finish (the message log lines above say why, if there are any)")
		return 1
	}
	say(stdout, "one-off disk backup: the snapshot is on the server\n")

	// 2. a disk image Backup Set that leaves the last partition out
	left := disk.Partitions[len(disk.Partitions)-1]
	sel := machineSel{paths: []string{disk.DevicePath}, ids: []string{disk.UniqueID}, skips: []api.PartitionSkip{{
		DiskID: stableID(disk.UniqueID), Device: disk.DevicePath, Start: left.Start, Size: left.Size, Number: left.Number, Label: "self-test",
	}}}
	doc := map[string]interface{}{
		"id": setID, "name": "__tui-selftest-machine__", "backupType": "machine", "enabled": true, "backupId": setGroup,
		"backupDirs": []interface{}{}, "scheduleMode": "manual", "useVSS": false, "pbsServerId": srvID,
		"excludeList": []interface{}{}, "readFreeSpace": false, "machineAsVm": false, "imageFolders": []interface{}{},
		"schedules": []interface{}{map[string]interface{}{"id": "s1", "enabled": true, "unit": "days", "every": 1.0, "at": "03:30"}},
	}
	sel.apply(doc)
	if err := be.SaveSet(doc, true); err != nil {
		sayln(stderr, "save disk image set: "+describeErr(err))
		return 1
	}
	sets, err := be.Sets()
	if err != nil {
		sayln(stderr, "list sets: "+describeErr(err))
		return 1
	}
	var back machineSel
	for _, s := range sets {
		if s.ID == setID {
			back = selOf(s.Raw)
			if s.Type != "machine" {
				sayln(stderr, "the saved set is not a machine set: "+s.Type)
				return 1
			}
		}
	}
	if len(back.paths) != 1 || back.paths[0] != disk.DevicePath || len(back.skips) != 1 || back.skips[0].Start != left.Start {
		sayln(stderr, fmt.Sprintf("the service returned the set with other disks or skips: %+v", back))
		return 1
	}
	say(stdout, "disk image set: saved, the disk and the left-out partition %d come back as sent\n", left.Number)
	if err := be.RunSet(setID); err != nil {
		sayln(stderr, "run disk image set: "+describeErr(err))
		return 1
	}
	if _, ok := wait(setGroup); !ok {
		sayln(stderr, "the disk image set did not finish (the message log lines above say why, if there are any)")
		return 1
	}
	say(stdout, "disk image set: ran, the snapshot is on the server\n")

	// the run's log says the partition was left out, not read
	var at time.Time
	for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline) && at.IsZero(); {
		hist, _ := be.History()
		for _, h := range hist {
			if h.BackupID == setGroup && h.Status != "" && !strings.EqualFold(h.Status, "running") {
				at = h.At
			}
		}
		if at.IsZero() {
			time.Sleep(3 * time.Second)
		}
	}
	if at.IsZero() {
		sayln(stderr, "the disk image set's run is not in the history")
		return 1
	}
	lines, err := be.RunLog(setGroup, at, true)
	if err != nil {
		sayln(stderr, "run log: "+describeErr(err))
		return 1
	}
	if !strings.Contains(strings.Join(lines, "\n"), "left out") {
		sayln(stderr, fmt.Sprintf("the run log (%d lines) does not say the partition was left out", len(lines)))
		return 1
	}
	say(stdout, "disk image set: the run log says partition %d was left out and not read\n", left.Number)
	return 0
}
