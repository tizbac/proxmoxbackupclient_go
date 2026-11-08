import test from 'node:test'
import assert from 'node:assert/strict'
import { newestFirst, pageLimit, pageItems, fitCount, unixFromLocal, FIT_FALLBACK, MORE_STEP } from './listPaging.mjs'

const snaps = [30, 10, 50, 20, 40].map(unix => ({ unix, id: String(unix) }))

test('newestFirst sorts a copy by time, newest first', () => {
  const sorted = newestFirst(snaps)
  assert.deepEqual(sorted.map(s => s.unix), [50, 40, 30, 20, 10])
  assert.deepEqual(snaps.map(s => s.unix), [30, 10, 50, 20, 40])
  assert.deepEqual(newestFirst(null), [])
})

test('pageLimit: fit uses the measured count, or the fallback before it is measured', () => {
  assert.equal(pageLimit('fit', 9), 9)
  assert.equal(pageLimit('fit', 0), FIT_FALLBACK)
  assert.equal(pageLimit('25', 9), 25)
  assert.equal(pageLimit('100', 9), 100)
  assert.equal(pageLimit('all', 9), Infinity)
  assert.equal(pageLimit('fit', 9, MORE_STEP), 9 + MORE_STEP)
  assert.equal(pageLimit('bogus', 9), MORE_STEP)
})

test('pageItems shows the newest up to the limit', () => {
  const p = pageItems(newestFirst(snaps), { limit: 2 })
  assert.deepEqual(p.shown.map(s => s.unix), [50, 40])
  assert.equal(p.matching, 5)
  assert.equal(p.total, 5)
})

test('pageItems: at or before a time, then the limit', () => {
  const p = pageItems(newestFirst(snaps), { limit: 2, before: 35 })
  assert.deepEqual(p.shown.map(s => s.unix), [30, 20])
  assert.equal(p.matching, 3)
  assert.equal(p.total, 5)
  const none = pageItems(newestFirst(snaps), { limit: 2, before: 5 })
  assert.equal(none.shown.length, 0)
  assert.equal(none.matching, 0)
})

test('pageItems never hides the selected one below the cut', () => {
  const p = pageItems(newestFirst(snaps), { limit: 2, isSelected: s => s.unix === 20 })
  assert.deepEqual(p.shown.map(s => s.unix), [50, 40, 30, 20])
  // Selected but filtered out by the date: not brought back (Restore lets
  // such a selection go when the date is set; Roll back re-picks).
  const q = pageItems(newestFirst(snaps), { limit: 2, before: 15, isSelected: s => s.unix === 40 })
  assert.deepEqual(q.shown.map(s => s.unix), [10])
})

test('fitCount: whole rows times columns, at least one row', () => {
  assert.equal(fitCount({ available: 500, rowHeight: 180, columns: 3 }), 6)
  assert.equal(fitCount({ available: 50, rowHeight: 180, columns: 3 }), 3)
  assert.equal(fitCount({ available: 600, rowHeight: 45 }), 13)
  assert.equal(fitCount({ available: 600, rowHeight: 0, columns: 3 }), FIT_FALLBACK)
})

test('unixFromLocal reads a datetime-local value', () => {
  assert.equal(unixFromLocal(''), 0)
  assert.equal(unixFromLocal('not a date'), 0)
  assert.equal(unixFromLocal('2026-11-07T14:30'), Math.floor(new Date('2026-11-07T14:30').getTime() / 1000))
})
