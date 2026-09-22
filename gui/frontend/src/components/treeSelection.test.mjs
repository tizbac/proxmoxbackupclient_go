// Standalone Node test for the tricky part of DirectoryTree's selection logic
// (no React/DOM/Wails needed — pure functions only). Run with:
//   node src/components/treeSelection.test.mjs
import assert from 'node:assert/strict'
import { isChecked, checkState, computeReincludeExcludes } from './treeSelection.js'

// Fixture:
//   C:\Data
//   ├── A
//   │   ├── x.txt
//   │   └── B
//   │       ├── y.txt
//   │       └── C
//   │           └── z.txt
//   └── D
// Field names deliberately match the real Go DirEntry JSON shape (is_dir,
// snake_case — see gui/dirlist.go) rather than a JS-idiomatic isDir. A prior
// version of this fixture used isDir and never caught that DirectoryTree.jsx
// was reading node.isDir while the real backend sends is_dir — the mismatch
// only showed up when Mick ran the real GUI, since this test's own fixture
// and the mock used for browser-based verification both used the wrong
// (consistent with each other, but wrong) field name. Lesson: a hand-written
// mock/fixture can hide a serialization-boundary bug precisely because it
// never crosses the real boundary.
const childrenByPath = {
  'c:/data': [
    { name: 'A', path: 'C:\\Data\\A', is_dir: true },
    { name: 'D', path: 'C:\\Data\\D', is_dir: true },
  ],
  'c:/data/a': [
    { name: 'x.txt', path: 'C:\\Data\\A\\x.txt', is_dir: false },
    { name: 'B', path: 'C:\\Data\\A\\B', is_dir: true },
  ],
  'c:/data/a/b': [
    { name: 'y.txt', path: 'C:\\Data\\A\\B\\y.txt', is_dir: false },
    { name: 'C', path: 'C:\\Data\\A\\B\\C', is_dir: true },
  ],
}

let passed = 0
function check(label, actual, expected) {
  assert.deepEqual(actual, expected, label)
  passed++
  console.log(`ok - ${label}`)
}

// Whole tree included, nothing excluded yet.
check('root included, nothing excluded -> D checked', isChecked('C:\\Data\\D', ['C:\\Data'], []), true)
check('root included, nothing excluded -> A/B/C checked', isChecked('C:\\Data\\A\\B\\C', ['C:\\Data'], []), true)
check('no covering root -> unchecked', isChecked('C:\\Other', ['C:\\Data'], []), false)

// Whole folder A excluded.
const afterExcludeA = ['C:\\Data\\A']
check('A excluded -> A unchecked', isChecked('C:\\Data\\A', ['C:\\Data'], afterExcludeA), false)
check('A excluded -> A\\B (descendant) unchecked', isChecked('C:\\Data\\A\\B', ['C:\\Data'], afterExcludeA), false)
check('A excluded -> D (sibling) still checked', isChecked('C:\\Data\\D', ['C:\\Data'], afterExcludeA), true)

// Re-include the grandchild C two levels down inside excluded A — this is the
// actual algorithm under test: it must punch a hole down through B without
// re-including A's or B's OTHER contents.
const afterReinclude = computeReincludeExcludes(childrenByPath, afterExcludeA, 'C:\\Data\\A\\B\\C', 'C:\\Data\\A')
check(
  'reinclude A\\B\\C: excludes A\\x.txt and A\\B\\y.txt, drops the A exclude',
  [...afterReinclude].sort(),
  ['C:\\Data\\A\\B\\y.txt', 'C:\\Data\\A\\x.txt'].sort()
)
check('reinclude A\\B\\C: C itself is checked again', isChecked('C:\\Data\\A\\B\\C', ['C:\\Data'], afterReinclude), true)
check('reinclude A\\B\\C: A itself reads checked (parent shown solid; carve-out is deeper)', isChecked('C:\\Data\\A', ['C:\\Data'], afterReinclude), true)
check('reinclude A\\B\\C: A\\x.txt stays excluded', isChecked('C:\\Data\\A\\x.txt', ['C:\\Data'], afterReinclude), false)
check('reinclude A\\B\\C: A\\B\\y.txt stays excluded', isChecked('C:\\Data\\A\\B\\y.txt', ['C:\\Data'], afterReinclude), false)
check('reinclude A\\B\\C: D untouched', isChecked('C:\\Data\\D', ['C:\\Data'], afterReinclude), true)

// Safety: a missing intermediate listing must stop the walk rather than guess.
const partial = computeReincludeExcludes({ 'c:/data/a': childrenByPath['c:/data/a'] }, afterExcludeA, 'C:\\Data\\A\\B\\C', 'C:\\Data\\A')
check('missing listing for B -> walk stops after A, only excludes x.txt', [...partial].sort(), ['C:\\Data\\A\\x.txt'])

// Tri-state checkbox display (checkState) — added after Mick compared our
// tree picker against BFW's own and pointed out a checked child folder gave
// its unchecked parent no visual indication anything under it was selected
// (BFW shows a partial/mixed checkbox for exactly this case).
check('no root at all -> unchecked (not mixed)', checkState('C:\\Other', ['C:\\Data'], []), false)
check('fully covered by a root, no excludes -> true (not mixed)', checkState('C:\\Data\\D', ['C:\\Data'], []), true)
check(
  'child independently checked, parent never checked -> parent reads mixed',
  checkState('C:\\Data\\A', ['C:\\Data\\A\\B'], []),
  'mixed'
)
check(
  'child independently checked, parent never checked -> the child itself still reads true',
  checkState('C:\\Data\\A\\B', ['C:\\Data\\A\\B'], []),
  true
)
check(
  'root with a nested exclude -> the root itself reads mixed, not solid true',
  checkState('C:\\Data\\A', ['C:\\Data'], ['C:\\Data\\A\\B']),
  'mixed'
)
check(
  'root with a nested exclude -> a sibling with nothing excluded under it still reads true',
  checkState('C:\\Data\\D', ['C:\\Data'], ['C:\\Data\\A\\B']),
  true
)
check(
  'excluded folder itself (not just a descendant) -> false, not mixed',
  checkState('C:\\Data\\A', ['C:\\Data'], afterExcludeA),
  false
)

console.log(`\n${passed} checks passed`)
