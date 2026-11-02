// node src/backupId.test.mjs
import assert from 'node:assert/strict'
import { suggestBackupId, slugId, setsSharingGroup } from './backupId.js'

const cases = [
  ['Deepthought - Data', 'DEEPTHOUGHT', [], 'deepthought-data'],
  ['Deepthought - Beeby Property', 'deepthought', [], 'deepthought-beeby-property'],
  ['Beeby Property', 'deepthought', [], 'deepthought-beeby-property'],
  ['Data (deepthought)', 'deepthought', [], 'deepthought-data'],
  ['Photos on DeepThought drive F', 'deepthought', [], 'deepthought-photos-on-drive-f'],
  ['Deepthought', 'deepthought', [], 'deepthought'],
  ['', 'deepthought', [], 'deepthought'],
  ['Documents', 'MICK-YOGA520', [], 'mick-yoga520-documents'],
  ['Mick Yoga520 docs', 'MICK-YOGA520', [], 'mick-yoga520-docs'],
  ['Café Ümlaut files', 'rigel', [], 'rigel-cafe-umlaut-files'],
  ['Deepthought - Data', 'deepthought', ['deepthought-data'], 'deepthought-data-2'],
  ['Deepthought - Data', 'deepthought', ['DEEPTHOUGHT-DATA', 'deepthought-data-2'], 'deepthought-data-3'],
  ['Deepthought - Data (copy)', 'deepthought', ['deepthought-data'], 'deepthought-data-copy'],
  // A slip in the computer's name in the set's name still counts as it
  ['Deepthough - XXX Home Made', 'DEEPTHOUGHT', [], 'deepthought-xxx-home-made'],
  ['Deepthgouht - Data', 'deepthought', [], 'deepthought-data'],
  ['Deeptought Data', 'deepthought', [], 'deepthought-data'],
  ['Rigle photos', 'rigel', [], 'rigel-photos'],
  // ...but not a different word, nor a near miss of a short name
  ['Deep Thinking', 'deepthought', [], 'deepthought-deep-thinking'],
  ['Data', 'date', [], 'date-data'],
]
for (const [name, host, taken, want] of cases) {
  assert.equal(suggestBackupId(name, host, taken), want, `${name} @ ${host}`)
}
assert.equal(slugId('  --A__b..c--  '), 'a-b-c')
const long = suggestBackupId('x'.repeat(100), 'host')
assert.ok(long.length <= 60 && /^[a-z0-9][a-z0-9-]*$/.test(long), long)
// Sets that write into one PBS group (server + type + ID).
const sets = [
  { id: 'a', name: 'Data', backupId: 'deepthought', pbsServerId: '' },
  { id: 'b', name: 'Beeby Property', backupId: 'deepthought', pbsServerId: 'pbs1' },
  { id: 'c', name: 'Trading', backupId: 'deepthought', pbsServerId: 'pbs2' },
  { id: 'd', name: 'Machine', backupId: '', backupType: 'machine' },
  { id: 'e', name: 'Machine as VM', backupId: 'deepthought', backupType: 'machine', machineAsVm: true },
  { id: 'f', name: 'Case', backupId: 'DeepThought' },
  { id: 'g', name: 'Own', backupId: 'deepthought-own' },
]
const sharing = (id) => setsSharingGroup(sets.find(s => s.id === id), sets, 'pbs1', 'deepthought').map(s => s.id)
// a uses the default server (pbs1), so it meets b; the machine set's empty ID
// is the computer's name, the same group too
assert.deepEqual(sharing('a'), ['b', 'd'])
assert.deepEqual(sharing('b'), ['a', 'd'])
assert.deepEqual(sharing('d'), ['a', 'b'])
assert.deepEqual(sharing('c'), [], 'another server')
assert.deepEqual(sharing('e'), [], 'vm group, not host')
assert.deepEqual(sharing('f'), [], 'PBS IDs are case sensitive')
assert.deepEqual(sharing('g'), [])
// a set being created (no id yet) is compared with all of them
assert.deepEqual(setsSharingGroup({ backupId: 'deepthought-own' }, sets, 'pbs1', 'deepthought').map(s => s.id), ['g'])
console.log(`backupId: ${cases.length + 2 + 8} checks passed`)
