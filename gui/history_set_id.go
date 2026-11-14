package main

// A run in the history used to be tied to its Backup Set by the set's name
// alone. A new set given the name of a deleted one then showed the deleted
// set's last failure and last success as its own, and a renamed set lost its
// runs (issue #143). Each backup of a set now records the set's ID
// (JobHistory.JobID), and everything that asks "which runs are this set's"
// goes through runOfSet.

// historyNoSet is the JobID of a backup that belongs to no Backup Set there
// is now: its set was deleted or renamed before runs recorded the ID. It can
// never equal a set's ID, so a later set with the same name does not take it.
const historyNoSet = "-"

// isSetRun reports whether h is a backup a Backup Set ran, as against a
// restore or a one-off backup from the Backup page.
func isSetRun(h JobHistory) bool {
	return h.Kind == "" && h.Trigger != "oneoff"
}

// runOfSet reports whether history entry h is a run of Backup Set job.
// An entry with no JobID was written by a version that did not record it and
// has not been through stampHistorySetIDs yet (which runs at every start), so
// until then the name is all there is to go on.
func runOfSet(h JobHistory, job ScheduledJob) bool {
	if !isSetRun(h) {
		return false
	}
	if h.JobID != "" {
		return h.JobID == job.ID
	}
	return h.Name == job.Name
}

// stampHistorySetIDs gives each Backup Set run in history that has no JobID
// the ID of the set in jobs with exactly its name, or historyNoSet when no
// set has that name, and reports whether it changed anything. It runs once
// per entry: from then on the entry follows its set through a rename and is
// not picked up by a later set of the same name.
//
// The name is the only link an older entry has, so a set that already
// replaced a deleted set of the same name before this ran still gets the
// deleted set's entries; they age out as new runs are recorded.
func stampHistorySetIDs(history []JobHistory, jobs []ScheduledJob) bool {
	idByName := make(map[string]string, len(jobs))
	for _, j := range jobs {
		if j.ID != "" {
			idByName[j.Name] = j.ID
		}
	}
	changed := false
	for i := range history {
		h := &history[i]
		if h.JobID != "" || !isSetRun(*h) {
			continue
		}
		h.JobID = historyNoSet
		if id, ok := idByName[h.Name]; ok {
			h.JobID = id
		}
		changed = true
	}
	return changed
}
