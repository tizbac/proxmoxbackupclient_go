package main

import "strings"

// migrateUpstreamJob carries a Backup Set saved by the project's app before
// the fork was merged (1.2.0-rc, its scheduled_jobs.json) over to this app's
// fields, so an upgraded computer keeps backing up to the same server, as the
// same kind, at the same times:
//   - pbs_id becomes PBSServerID;
//   - backup_kind "vm" on a machine set becomes MachineAsVM (its backup ID is
//     already the numeric VM ID upstream required);
//   - frequency "weekly" (weekdays, 0 = Sunday) and "monthly" (dayOfMonth)
//     become a weeks or months schedule at ScheduleTime. "daily", or none, is
//     what the old daily trigger already reads. A monthly day a short month
//     lacks runs on that month's last day (upstream skipped those months).
//
// The upstream keys are cleared afterwards, so the file is saved in this
// app's format. A set that already has schedules is left as it is.
func migrateUpstreamJob(job *ScheduledJob) {
	if job.PBSServerID == "" {
		job.PBSServerID = job.UpstreamPBSID
	}
	if job.UpstreamBackupKind == "vm" && job.BackupType == "machine" {
		job.MachineAsVM = true
	}
	if job.ScheduleMode == "" && job.TriggerMode == "" && job.ScheduleTime != "" {
		switch job.UpstreamFrequency {
		case "weekly":
			var days []string
			for _, d := range job.UpstreamWeekdays {
				if d >= 0 && d < len(weekdayAbbrev) {
					days = append(days, weekdayAbbrev[d])
				}
			}
			if len(days) > 0 {
				job.ScheduleMode = ScheduleModeScheduled
				job.Schedules = []Schedule{{ID: "s1", Enabled: true, Unit: "weeks", Every: 1, At: job.ScheduleTime, Weekdays: days}}
			}
		case "monthly":
			day := job.UpstreamDayOfMonth
			if day < 1 || day > 31 {
				day = 1
			}
			job.ScheduleMode = ScheduleModeScheduled
			job.Schedules = []Schedule{{ID: "s1", Enabled: true, Unit: "months", Every: 1, At: job.ScheduleTime, MonthMode: "day", MonthDay: day}}
		}
	}
	job.UpstreamFrequency, job.UpstreamWeekdays, job.UpstreamDayOfMonth = "", nil, 0
	job.UpstreamPBSID, job.UpstreamBackupKind = "", ""
}

// migrateUpstreamDiskPins gives a machine set saved by the project's 1.2.0-rc
// app the identities of its disks, so it keeps backing up the same disks:
//   - that app kept one table for the computer, each disk's identity and the
//     device path it was last seen at (config.json pinned_disks, read into
//     Config.UpstreamPinnedDisks); a device takes the identity pinned to its
//     path. A path two identities claim (one disk took another's number) gets
//     none, as there is no telling which disk the set meant;
//   - on Linux its sets hold /dev/disk/by-id paths, which name the disk
//     themselves.
//
// A device that gets no identity is backed up by its path as before, and
// gets one when the set is next saved. A set that already has identities is
// left as it is. Nothing is cleared: the table is only read, at every load,
// until the set is saved with identities of its own.
func migrateUpstreamDiskPins(job *ScheduledJob, pins map[string]string) {
	if job.BackupType != "machine" || len(job.DriveLetters) == 0 || len(job.DiskIDs) > 0 {
		return
	}
	byPath := map[string]string{}
	for id, path := range pins {
		if !isStableDiskID(id) {
			continue
		}
		p := canonicalDevicePath(path)
		if _, claimed := byPath[p]; claimed {
			byPath[p] = ""
			continue
		}
		byPath[p] = id
	}
	ids := make([]string, len(job.DriveLetters))
	found := false
	for i, dev := range job.DriveLetters {
		if name, ok := strings.CutPrefix(dev, linuxByIDDir); ok && name != "" {
			ids[i] = diskIDPrefixByID + name
		} else {
			ids[i] = byPath[canonicalDevicePath(dev)]
		}
		found = found || ids[i] != ""
	}
	if found {
		job.DiskIDs = ids
	}
}
