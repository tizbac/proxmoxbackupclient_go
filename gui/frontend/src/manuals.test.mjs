// node src/manuals.test.mjs
// Every link between the manuals built into the app must land on a heading.
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { MANUALS, makeSlugger, renderManual, resolveLink, repoRef } from './manuals.js'

const s = makeSlugger()
assert.equal(s('9. Restore'), '9-restore')
assert.equal(s('9.4 Choose where and how'), '94-choose-where-and-how')
assert.equal(s('Linux machine backup prerequisites (VSS equivalent)'), 'linux-machine-backup-prerequisites-vss-equivalent')
assert.equal(s('Restore'), 'restore')
assert.equal(s('Restore'), 'restore-1')
assert.equal(s('`-no-mtime` & times'), '-no-mtime--times')

assert.equal(repoRef('0.9.1'), 'v0.9.1')
assert.equal(repoRef('dev-37df34d'), 'master')
assert.deepEqual(resolveLink('GUI.md#9-restore', 'docs/manual/CLI.md', 'v1.0.0'), { doc: 'gui', anchor: '9-restore' })
assert.deepEqual(resolveLink('manual/BMR.md', 'docs/RESTORE_GUIDE.md', 'v1.0.0'), { doc: 'bmr', anchor: '' })
assert.deepEqual(resolveLink('#7-x', 'docs/manual/BMR.md', 'v1.0.0'), { doc: 'bmr', anchor: '7-x' })
assert.deepEqual(resolveLink('../../CHANGELOG.md', 'docs/manual/GUI.md', 'v1.0.0'),
  { url: 'https://github.com/mjb-is/proxmoxbackupclient_go/blob/v1.0.0/CHANGELOG.md' })
assert.deepEqual(resolveLink('https://pbs.example/x', 'docs/manual/GUI.md', 'master'), { url: 'https://pbs.example/x' })

const docsDir = fileURLToPath(new URL('../../', import.meta.url)) // gui/
const rendered = {}
for (const m of MANUALS) {
  rendered[m.id] = renderManual(readFileSync(docsDir + m.path, 'utf8'), m.id, 'master')
  assert.ok(rendered[m.id].headings.length >= 2, `${m.id}: too few headings`)
}
let links = 0
for (const m of MANUALS) {
  for (const [, doc, anchor] of rendered[m.id].html.matchAll(/data-doc="([^"]*)" data-anchor="([^"]*)"/g)) {
    links++
    assert.ok(rendered[doc], `${m.id}: link to unknown manual ${doc}`)
    if (anchor) assert.ok(rendered[doc].html.includes(`id="${anchor}"`), `${m.id}: link to ${doc}#${anchor} finds no heading`)
  }
}
assert.ok(links > 40, `only ${links} links between manuals found`)
console.log(`manuals: ok (${links} links between manuals checked)`)
