// node src/backupId.test.mjs
import assert from 'node:assert/strict'
import { suggestBackupId, slugId } from './backupId.js'

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
]
for (const [name, host, taken, want] of cases) {
  assert.equal(suggestBackupId(name, host, taken), want, `${name} @ ${host}`)
}
assert.equal(slugId('  --A__b..c--  '), 'a-b-c')
const long = suggestBackupId('x'.repeat(100), 'host')
assert.ok(long.length <= 60 && /^[a-z0-9][a-z0-9-]*$/.test(long), long)
console.log(`backupId: ${cases.length + 2} checks passed`)
