// Pure selection-logic for DirectoryTree, kept separate from the React/Fluent
// wiring so it can be unit-tested directly (see treeSelection.test.mjs) — this
// is the trickiest part of the tree picker (the re-include-a-subfolder
// algorithm) and the one most worth verifying without a human clicking
// through the real GUI.

// Windows paths arrive with backslashes; everything here compares
// case-insensitively on forward slashes so "C:\Users" and "c:/users" (or a
// trailing slash either way) are treated as the same path.
export function normPath(p) {
  return p.replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase()
}

// True when `path` IS `root` or sits anywhere under it.
export function isUnder(root, path) {
  const r = normPath(root)
  const p = normPath(path)
  return p === r || p.startsWith(r + '/')
}

// Effective checked state for one path, derived purely from the two lists the
// parent form owns: `roots` (fully-included backup directories — these ARE
// opts.BackupObjects) and `excludes` (absolute paths excluded under one of
// those roots — these get appended to the existing, already-wired
// `excludeList`). A path is checked if some root covers it and neither it nor
// any folder between it and that root has been excluded.
export function isChecked(path, roots, excludes) {
  const p = normPath(path)
  const root = roots.find((r) => isUnder(r, p))
  if (!root) return false
  return !excludes.some((e) => isUnder(e, p))
}

// Tri-state checkbox render value for one path: true (fully included),
// false (not included at all), or 'mixed' (some but not all of its subtree
// is included) — the classic Explorer/BFW convention Mick pointed out was
// missing here (a parent with a selected child looked identically empty to
// a parent with nothing selected under it at all). Two distinct ways a node
// ends up 'mixed':
//   1. It is NOT itself covered by any root, but some independently-checked
//      root lives underneath it (e.g. a child folder was checked directly
//      without ever checking this parent — the actual case in Mick's
//      screenshot: "TestData" unchecked, "Backup of Compaq 486 Laptop"
//      checked underneath it).
//   2. It IS covered by a root (isChecked would say true), but at least one
//      exclude sits at or below it — a fully-included folder with a carved-
//      out subfolder isn't ALL selected either, even though isChecked's own
//      binary semantics (deliberately) treat it as checked for backup
//      purposes. See treeSelection.test.mjs's note on this exact case.
export function checkState(path, roots, excludes) {
  const p = normPath(path)
  if (isChecked(path, roots, excludes)) {
    return excludes.some((e) => isUnder(p, e)) ? 'mixed' : true
  }
  return roots.some((r) => isUnder(p, r) && normPath(r) !== p) ? 'mixed' : false
}

// Re-include `path`, which is currently excluded only because an ANCESTOR of
// it (`ancestorExclude`) is in `excludes` — a whole-folder exclude covering
// it. Returns the new excludes array: the one ancestor-level exclude is
// replaced by excluding every SIBLING along the path down to `path` instead,
// so only `path` (and its subtree) comes back — everything else the ancestor
// exclude used to cover stays excluded.
//
// `childrenByPath` must already hold the DirEntry[] listing for every
// directory level between `ancestorExclude` and `path` (keyed by normPath) —
// true by construction in the real component, since the tree has to be
// expanded that far for this checkbox to be clickable in the first place.
// If a level is missing here (should not happen), the walk just stops early
// rather than guessing, leaving the excludes below that point as they were —
// safe (under-inclusive), never silently wrong (over-inclusive).
export function computeReincludeExcludes(childrenByPath, excludes, path, ancestorExclude) {
  const target = normPath(path)
  const next = excludes.filter((e) => e !== ancestorExclude)
  let current = ancestorExclude
  while (normPath(current) !== target) {
    const kids = childrenByPath[normPath(current)]
    if (!Array.isArray(kids)) break
    const step = kids.find((k) => isUnder(k.path, path))
    if (!step) break
    for (const sib of kids) {
      if (sib.path !== step.path) next.push(sib.path)
    }
    current = step.path
  }
  return next
}
