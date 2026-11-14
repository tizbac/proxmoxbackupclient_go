// node src/runsOfSet.test.mjs
import assert from 'node:assert/strict'
import { isSetRun, runsOfSet, shortRunError } from './runsOfSet.js'

const ids = (history, job) => runsOfSet(history, job).map(h => h.id)

// A run is its set's by the set's ID, whatever either is called.
const history = [
  { id: 'r6', name: 'Test', jobId: 'new', status: 'success' },
  { id: 'r5', name: 'Test', jobId: 'old', status: 'failed' },
  { id: 'r4', name: 'Test', jobId: '-', status: 'failed' },
  { id: 'r3', name: 'Before rename', jobId: 'new', status: 'failed' },
  { id: 'r2', name: 'Test', trigger: 'oneoff', status: 'failed' },
  { id: 'r1', name: 'Test', kind: 'restore', status: 'failed' },
]
// A new set named as a deleted one shows none of the deleted set's runs,
// and a renamed set keeps its own.
assert.deepEqual(ids(history, { id: 'new', name: 'Test' }), ['r6', 'r3'])
assert.deepEqual(ids(history, { id: 'new', name: 'Renamed again' }), ['r6', 'r3'])
assert.deepEqual(ids(history, { id: 'old', name: 'Test' }), ['r5'])
assert.deepEqual(ids(history, { id: 'other', name: 'Test' }), [])

// An entry not yet tied to a set goes by its name; a one-off or a restore
// with the set's name never counts.
const older = [
  { id: 'o3', name: 'Documents', status: 'failed' },
  { id: 'o2', name: 'Documents', trigger: 'oneoff', status: 'failed' },
  { id: 'o1', name: 'Photos', status: 'success' },
]
assert.deepEqual(ids(older, { id: 'd', name: 'Documents' }), ['o3'])
assert.deepEqual(ids(older, { id: 'p', name: 'Photos' }), ['o1'])
assert.deepEqual(ids(null, { id: 'd', name: 'Documents' }), [])
assert.deepEqual(ids(older, null), [])
assert.equal(isSetRun({ name: 'x', trigger: 'manual' }), true)
assert.equal(isSetRun({ name: 'x', kind: 'restore' }), false)
assert.equal(isSetRun(undefined), false)

// The error on the card: what came back, not the way there.
assert.equal(
  shortRunError('Error: failed to create shared catalog index: HTTP request failed: Post "https://pbs.example.com:8007//dynamic_index": PBS refused the backup session: HTTP 400 Bad Request - backup owner check failed (a@pbs != b@pbs)'),
  'PBS refused the backup session: HTTP 400 Bad Request - backup owner check failed (a@pbs != b@pbs)')
assert.equal(
  shortRunError('failed to start: Get "https://pbs:8007/api2/json/version": dial tcp 192.0.2.1:8007: connect: no route to host'),
  'dial tcp 192.0.2.1:8007: connect: no route to host')
// The last request named is the one that failed.
assert.equal(
  shortRunError('Get "https://a/x": retried: Post "https://a/y": context deadline exceeded'),
  'context deadline exceeded')
// The first line only, and no "Error: " in front.
assert.equal(shortRunError('Error: PBS refused the login\nHTTP/1.1 401\nBody: no'), 'PBS refused the login')
assert.equal(shortRunError('\n  Backup cancelled by user  \nmore'), 'Backup cancelled by user')
// Nothing to cut: as it is.
assert.equal(shortRunError('2 files could not be read: in use'), '2 files could not be read: in use')
// A request clause with nothing after it is kept whole.
assert.equal(shortRunError('Post "https://a/y": '), 'Post "https://a/y":')
assert.equal(shortRunError(''), '')
assert.equal(shortRunError(undefined), '')
// Twice running gives the same answer (the pattern keeps no position).
assert.equal(shortRunError('Post "https://a/y": refused'), 'refused')
assert.equal(shortRunError('Post "https://a/y": refused'), 'refused')
console.log('runsOfSet: ok')
