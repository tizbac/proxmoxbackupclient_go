import test from 'node:test'
import assert from 'node:assert/strict'
import { openSteps, clampStep, restoreReached, undeleteReached, rollbackReached } from './stepRules.mjs'

test('a step opens only when every step before it is reached', () => {
  assert.deepEqual(openSteps([true, true, false, true, true]), [true, true, false, false, false])
  assert.deepEqual(openSteps([true, false, true]), [true, false, false])
  assert.deepEqual(openSteps([true, true, true]), [true, true, true])
})

test('step 1 is always open', () => {
  assert.deepEqual(openSteps([false, false]), [true, false])
})

test('clampStep keeps an open step and falls back to the last open one before it', () => {
  const open = [true, true, true, false, false]
  assert.equal(clampStep(3, open), 3)
  assert.equal(clampStep(5, open), 3)
  assert.equal(clampStep(4, open), 3)
  assert.equal(clampStep(1, open), 1)
  assert.equal(clampStep(0, open), 1)
  assert.equal(clampStep(9, [true, true]), 2)
})

test('Restore: changing the source closes everything after it', () => {
  const all = restoreReached({ listed: true, snapshot: true, files: 3, destination: true })
  assert.deepEqual(openSteps(all), [true, true, true, true, true])
  // A new server or backup ID clears the list and the snapshot.
  const cleared = restoreReached({ listed: false, snapshot: false, files: 0, destination: true })
  assert.deepEqual(openSteps(cleared), [true, false, false, false, false])
  assert.equal(clampStep(5, openSteps(cleared)), 1)
})

test('Restore: files and a destination are needed in turn', () => {
  assert.deepEqual(openSteps(restoreReached({ listed: true, snapshot: true, files: 0, destination: true })),
    [true, true, true, false, false])
  assert.deepEqual(openSteps(restoreReached({ listed: true, snapshot: true, files: 2, destination: false })),
    [true, true, true, true, false])
})

test('Restore: a search hit opens straight up to the destination', () => {
  // handleRestoreHit lists nothing, but chooses the snapshot and the file.
  const open = openSteps(restoreReached({ listed: true, snapshot: true, files: 1, destination: true }))
  assert.equal(clampStep(4, open), 4)
})

test('Undelete and Roll back', () => {
  assert.deepEqual(openSteps(undeleteReached({ scanned: true, files: 0 })), [true, true, false])
  assert.deepEqual(openSteps(undeleteReached({ scanned: false, files: 5 })), [true, false, false])
  assert.deepEqual(openSteps(rollbackReached({ set: true, backup: true, preview: false })), [true, true, true, false])
  assert.deepEqual(openSteps(rollbackReached({ set: true, backup: false, preview: true })), [true, true, false, false])
})
