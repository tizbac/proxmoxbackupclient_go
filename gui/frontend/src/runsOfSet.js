// Which runs in the history are a Backup Set's, and a failed run's error cut
// down to fit the set's card (issue #143).

// isSetRun: a backup a Backup Set ran, as against a restore or a one-off
// backup from the Backup page.
export function isSetRun(h) {
  return !!h && !h.kind && h.trigger !== 'oneoff'
}

// runsOfSet returns the set's own runs, in the order of history (newest
// first). A run is tied to its set by the set's ID, so it stays with the set
// through a rename and a new set given the name of a deleted one does not
// show the deleted set's runs. An entry with no jobId has not been tied to a
// set yet (the service or app does that at its next start); until then the
// name is all there is to go on. As runOfSet in history_set_id.go.
export function runsOfSet(history, job) {
  if (!job) return []
  return (history || []).filter(h => isSetRun(h) && (h.jobId ? h.jobId === job.id : h.name === job.name))
}

// A request the error names on its way to the reason: Post "https://...":
const REQUEST = /\b(?:Get|Post|Put|Delete|Head|Patch) "[^"]*": /g

// shortRunError is the error to show on a set's card, where "View this run"
// leads to the whole of it: the first line only, without the "Error: " the
// scheduler puts in front, and when the error names a request, what came back
// from it rather than the steps and the address that led there. So
//   failed to create shared catalog index: HTTP request failed: Post
//   "https://pbs:8007//dynamic_index": PBS refused the backup session: HTTP 400 ...
// reads "PBS refused the backup session: HTTP 400 ...".
export function shortRunError(message) {
  let s = String(message || '').split(/\r?\n/).map(l => l.trim()).find(l => l) || ''
  s = s.replace(/^Error:\s*/i, '')
  let cut = -1
  REQUEST.lastIndex = 0
  for (let m = REQUEST.exec(s); m !== null; m = REQUEST.exec(s)) cut = m.index + m[0].length
  const after = cut > 0 ? s.slice(cut).trim() : ''
  return after || s
}
