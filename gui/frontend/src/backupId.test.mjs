// node src/backupId.test.mjs
import assert from 'node:assert/strict'
import { suggestBackupId, slugId, setsSharingGroup } from './backupId.js'

const cases = [
  ['Fileserver - Data', 'FILESERVER', [], 'fileserver-data'],
  ['Fileserver - Office Accounts', 'fileserver', [], 'fileserver-office-accounts'],
  ['Office Accounts', 'fileserver', [], 'fileserver-office-accounts'],
  ['Data (fileserver)', 'fileserver', [], 'fileserver-data'],
  ['Photos on FileServer drive F', 'fileserver', [], 'fileserver-photos-on-drive-f'],
  ['Fileserver', 'fileserver', [], 'fileserver'],
  ['', 'fileserver', [], 'fileserver'],
  ['Documents', 'OFFICE-PC2', [], 'office-pc2-documents'],
  ['Office PC2 docs', 'OFFICE-PC2', [], 'office-pc2-docs'],
  ['Café Ümlaut files', 'laptop', [], 'laptop-cafe-umlaut-files'],
  ['Fileserver - Data', 'fileserver', ['fileserver-data'], 'fileserver-data-2'],
  ['Fileserver - Data', 'fileserver', ['FILESERVER-DATA', 'fileserver-data-2'], 'fileserver-data-3'],
  ['Fileserver - Data (copy)', 'fileserver', ['fileserver-data'], 'fileserver-data-copy'],
  // A slip in the computer's name in the set's name still counts as it
  ['Fileserve - XXX Home Made', 'FILESERVER', [], 'fileserver-xxx-home-made'],
  ['Fliesevrer - Data', 'fileserver', [], 'fileserver-data'],
  ['Fileservr Data', 'fileserver', [], 'fileserver-data'],
  ['Ofifce photos', 'office', [], 'office-photos'],
  // ...but not a different word, nor a near miss of a short name
  ['File Servers', 'fileserver', [], 'fileserver-file-servers'],
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
  { id: 'a', name: 'Data', backupId: 'fileserver', pbsServerId: '' },
  { id: 'b', name: 'Office Accounts', backupId: 'fileserver', pbsServerId: 'pbs1' },
  { id: 'c', name: 'Trading', backupId: 'fileserver', pbsServerId: 'pbs2' },
  { id: 'd', name: 'Machine', backupId: '', backupType: 'machine' },
  { id: 'e', name: 'Machine as VM', backupId: 'fileserver', backupType: 'machine', machineAsVm: true },
  { id: 'f', name: 'Case', backupId: 'FileServer' },
  { id: 'g', name: 'Own', backupId: 'fileserver-own' },
]
const sharing = (id) => setsSharingGroup(sets.find(s => s.id === id), sets, 'pbs1', 'fileserver').map(s => s.id)
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
assert.deepEqual(setsSharingGroup({ backupId: 'fileserver-own' }, sets, 'pbs1', 'fileserver').map(s => s.id), ['g'])
console.log(`backupId: ${cases.length + 2 + 8} checks passed`)
