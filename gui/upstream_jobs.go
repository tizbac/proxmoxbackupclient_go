package main

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
