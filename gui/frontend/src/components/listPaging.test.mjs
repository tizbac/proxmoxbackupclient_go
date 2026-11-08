import test from 'node:test'
import assert from 'node:assert/strict'
import { newestFirst, showChoice, pageSize, pageItems, pageOf, unixFromLocal, DEFAULT_SHOW } from './listPaging.mjs'

const snaps = [30, 10, 50, 20, 40].map(unix => ({ unix, id: String(unix) }))

test('newestFirst sorts a copy by time, newest first', () => {
  const sorted = newestFirst(snaps)
  assert.deepEqual(sorted.map(s => s.unix), [50, 40, 30, 20, 10])
  assert.deepEqual(snaps.map(s => s.unix), [30, 10, 50, 20, 40])
  assert.deepEqual(newestFirst(null), [])
})

test('showChoice keeps the choices and maps the old ones', () => {
  for (const v of ['5', '10', '25', '50', 'all']) assert.equal(showChoice(v), v)
  assert.equal(showChoice('fit'), DEFAULT_SHOW)
  assert.equal(showChoice('100'), '50')
  assert.equal(showChoice(null), DEFAULT_SHOW)
  assert.equal(showChoice('bogus'), DEFAULT_SHOW)
})

test('pageSize: the number, or Infinity for all', () => {
  assert.equal(pageSize('5'), 5)
  assert.equal(pageSize('50'), 50)
  assert.equal(pageSize('all'), Infinity)
  assert.equal(pageSize('fit'), 10)
})

test('pageItems: pages of the newest, numbered from 1', () => {
  const sorted = newestFirst(snaps)
  const p0 = pageItems(sorted, { size: 2, page: 0 })
  assert.deepEqual(p0.shown.map(s => s.unix), [50, 40])
  assert.deepEqual([p0.page, p0.pages, p0.from, p0.to, p0.matching, p0.total], [0, 3, 1, 2, 5, 5])
  const p2 = pageItems(sorted, { size: 2, page: 2 })
  assert.deepEqual(p2.shown.map(s => s.unix), [10])
  assert.deepEqual([p2.from, p2.to], [5, 5])
})

test('pageItems keeps the page within range', () => {
  const sorted = newestFirst(snaps)
  assert.equal(pageItems(sorted, { size: 2, page: 9 }).page, 2)
  assert.equal(pageItems(sorted, { size: 2, page: -3 }).page, 0)
  const all = pageItems(sorted, { size: Infinity, page: 4 })
  assert.deepEqual([all.page, all.pages, all.from, all.to], [0, 1, 1, 5])
})

test('pageItems: at or before a time, then the pages', () => {
  const sorted = newestFirst(snaps)
  const p = pageItems(sorted, { size: 2, before: 35 })
  assert.deepEqual(p.shown.map(s => s.unix), [30, 20])
  assert.deepEqual([p.matching, p.total, p.pages], [3, 5, 2])
  const none = pageItems(sorted, { size: 2, before: 5 })
  assert.deepEqual([none.shown.length, none.matching, none.pages, none.from, none.to], [0, 0, 1, 0, 0])
})

test('pageOf finds the page with the selected one', () => {
  const sorted = newestFirst(snaps)
  assert.equal(pageOf(sorted, { size: 2, isSelected: s => s.unix === 20 }), 1)
  assert.equal(pageOf(sorted, { size: 2, isSelected: s => s.unix === 50 }), 0)
  assert.equal(pageOf(sorted, { size: 2, before: 35, isSelected: s => s.unix === 10 }), 1)
  assert.equal(pageOf(sorted, { size: 2, before: 35, isSelected: s => s.unix === 40 }), -1)
  assert.equal(pageOf(sorted, { size: Infinity, isSelected: s => s.unix === 10 }), 0)
  assert.equal(pageOf(sorted, { size: 2 }), -1)
})

test('unixFromLocal reads a datetime-local value', () => {
  assert.equal(unixFromLocal(''), 0)
  assert.equal(unixFromLocal('not a date'), 0)
  assert.equal(unixFromLocal('2026-11-07T14:30'), Math.floor(new Date('2026-11-07T14:30').getTime() / 1000))
})
