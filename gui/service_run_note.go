package main

import (
	"fmt"
	"os"
	"time"

	"pbscommon"
)

// groupNoter adds a note to the snapshot a Backup Set run made when the run
// happened in the background service but the app sends the e-mail (Run Now
// in service mode): the app does not hold that snapshot, so it finds it on
// the server, as the newest one of the set's backup group taken since the
// run started.
type groupNoter struct {
	a      *App
	job    ScheduledJob
	since  time.Time
	pbsCfg func() (*Config, error)
}

// newGroupNoter returns a noter for job's run that started at since.
func (a *App) newGroupNoter(job ScheduledJob, since time.Time) *groupNoter {
	return &groupNoter{a: a, job: job, since: since, pbsCfg: func() (*Config, error) { return a.resolvePBS(job.PBSServerID) }}
}

// snapshotGroup is the PBS backup type and ID job's snapshots go under.
func snapshotGroup(job ScheduledJob) (backupType, backupID string) {
	backupType = "host"
	if job.BackupType == "machine" {
		backupType = machineSnapshotType(scheduledMachineBackupType(job))
	}
	backupID = job.BackupID
	if backupID == "" {
		backupID, _ = os.Hostname()
	}
	return backupType, backupID
}

// newestSnapshotSince picks the newest snapshot taken at or after since (a
// minute's leeway for the clocks of this computer and the server).
func newestSnapshotSince(snaps []pbscommon.BackupManifest, since time.Time) *pbscommon.BackupManifest {
	var best *pbscommon.BackupManifest
	for i := range snaps {
		s := &snaps[i]
		if s.BackupTime < since.Add(-time.Minute).Unix() {
			continue
		}
		if best == nil || s.BackupTime > best.BackupTime {
			best = s
		}
	}
	return best
}

func (g *groupNoter) AddNote(note string) error {
	cfg, err := g.pbsCfg()
	if err != nil {
		return err
	}
	client := &pbscommon.PBSClient{
		BaseURL: cfg.BaseURL, CertFingerPrint: cfg.CertFingerprint, AuthID: cfg.AuthID, Secret: cfg.Secret,
		Ticket: cfg.Ticket, CSRFToken: cfg.CSRFToken, Datastore: cfg.Datastore, Namespace: cfg.Namespace,
		Insecure: cfg.CertFingerprint != "",
	}
	backupType, backupID := snapshotGroup(g.job)
	snaps, err := client.ListGroupSnapshots(backupType, backupID)
	if err != nil {
		return fmt.Errorf("listing %s/%s: %w", backupType, backupID, err)
	}
	snap := newestSnapshotSince(snaps, g.since)
	if snap == nil {
		return fmt.Errorf("no snapshot of %s/%s since %s", backupType, backupID, g.since.Format(time.RFC3339))
	}
	client.Manifest = pbscommon.BackupManifest{BackupType: backupType, BackupID: backupID, BackupTime: snap.BackupTime}
	// A set's snapshots carry its name as their comment (the listing does
	// not return it, and the run that set it is in another process).
	if g.job.Name != "" {
		note = g.job.Name + " | " + note
	}
	return client.SetComment(note)
}
