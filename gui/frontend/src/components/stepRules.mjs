// Which steps of a wizard page (Restore, Undelete, Roll back) can be opened.
// Each step needs everything before it: `reached` lists, step by step,
// whether what that step needs is there (step 1 always is). A step is open
// when it and every step before it are reached, so clearing an early choice
// closes everything after it.

// openSteps returns one boolean per step: may it be shown?
export function openSteps(reached) {
  const open = []
  let ok = true
  for (let i = 0; i < reached.length; i++) {
    ok = ok && (i === 0 || !!reached[i])
    open.push(ok)
  }
  return open
}

// clampStep returns the step to show: `current` when it is open, else the
// last open step before it (an early choice changed, so later steps closed).
export function clampStep(current, open) {
  let s = Math.min(Math.max(1, current), open.length)
  while (s > 1 && !open[s - 1]) s--
  return s
}

// restoreReached: what each Restore step needs. listed: the snapshots are
// listed; snapshot: one is chosen; files: at least one entry is ticked;
// destination: where they go is valid; the Options step needs nothing more.
export function restoreReached({ listed, snapshot, files, destination }) {
  return [true, !!listed, !!snapshot, !!snapshot && files > 0, !!snapshot && files > 0 && !!destination]
}

// undeleteReached: the scan has run on the chosen set; files are chosen.
export function undeleteReached({ scanned, files }) {
  return [true, !!scanned, !!scanned && files > 0]
}

// rollbackReached: a set is chosen (its backups listed); a backup is chosen;
// its preview is there; the Roll back step needs the preview too.
export function rollbackReached({ set, backup, preview }) {
  return [true, !!set, !!set && !!backup, !!set && !!backup && !!preview]
}
