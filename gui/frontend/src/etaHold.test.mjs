// node src/etaHold.test.mjs
import assert from 'node:assert/strict'
import { holdEta, finishAt, ETA_HOLD_MS } from './etaHold.js'

const t0 = 1_000_000
// The first estimate is shown at once.
let h = holdEta(null, 300, t0)
assert.deepEqual(h, { eta: 300, at: t0, key: '' })
// New estimates within 10 s are not shown: the held one stays, and so does
// its forecast finish.
for (const [dt, eta] of [[1000, 280], [5000, 310], [9999, 250]]) {
  h = holdEta(h, eta, t0 + dt)
  assert.equal(h.eta, 300)
  assert.equal(finishAt(h), t0 + 300_000)
}
// At 10 s the latest estimate replaces it.
h = holdEta(h, 240, t0 + ETA_HOLD_MS)
assert.deepEqual(h, { eta: 240, at: t0 + 10000, key: '' })
assert.equal(finishAt(h), t0 + 10000 + 240_000)
// Done (0) shows at once.
assert.equal(holdEta(h, 0, t0 + 11000).eta, 0)
// No estimate (null) clears it.
assert.deepEqual(holdEta(h, null, t0 + 11000), { eta: null, at: 0, key: '' })
// A new stage starts its own estimate at once.
const s = holdEta({ eta: 50, at: t0, key: 'acls' }, 900, t0 + 2000, 'verifying')
assert.deepEqual(s, { eta: 900, at: t0 + 2000, key: 'verifying' })
console.log('etaHold: all passed')
