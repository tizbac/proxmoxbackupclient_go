// Which part of a long list of backups to show (Restore's snapshots, Roll
// back's backups): the newest first, a page at a time of the chosen size (5,
// 10, 25, 50 or all), optionally only those at or before a time. The page
// with the selected one can be found, so a choice made earlier (or by Roll
// back's date box) is brought into view rather than left on another page.

export const SHOW_CHOICES = ['5', '10', '25', '50', 'all']
export const DEFAULT_SHOW = '10'

// newestFirst: a copy, newest first by its unix time.
export function newestFirst(items) {
  return [...(items || [])].sort((a, b) => (b.unix || 0) - (a.unix || 0))
}

// showChoice: a stored choice as one of SHOW_CHOICES. Choices from before the
// pages ("fit", "100") become the nearest one.
export function showChoice(v) {
  if (SHOW_CHOICES.includes(v)) return v
  if (v === '100') return '50'
  return DEFAULT_SHOW
}

// pageSize: how many a page holds for the choice (Infinity for all).
export function pageSize(show) {
  const v = showChoice(show)
  return v === 'all' ? Infinity : parseInt(v, 10)
}

function matchingItems(items, before) {
  const all = items || []
  return before > 0 ? all.filter(s => (s.unix || 0) <= before) : all
}

// pageItems: items newest first; size: from pageSize; page: 0-based, kept
// within the pages there are; before: unix seconds, 0 for no limit. from and
// to number the shown items from 1 (both 0 when none).
export function pageItems(items, { size = Infinity, page = 0, before = 0 } = {}) {
  const all = items || []
  const list = matchingItems(all, before)
  const per = size > 0 ? size : Infinity
  const pages = per === Infinity || list.length === 0 ? 1 : Math.ceil(list.length / per)
  const p = Math.min(Math.max(0, Math.floor(page) || 0), pages - 1)
  const start = per === Infinity ? 0 : p * per
  const shown = per === Infinity ? list : list.slice(start, start + per)
  return {
    shown, matching: list.length, total: all.length, page: p, pages,
    from: shown.length ? start + 1 : 0, to: start + shown.length,
  }
}

// pageOf: the page holding the item isSelected picks out, or -1 when no
// matching item is selected.
export function pageOf(items, { size = Infinity, before = 0, isSelected } = {}) {
  if (!isSelected) return -1
  const i = matchingItems(items, before).findIndex(isSelected)
  if (i < 0) return -1
  return size > 0 && size !== Infinity ? Math.floor(i / size) : 0
}

// unixFromLocal: a datetime-local value ("2026-11-07T14:30") as unix seconds,
// 0 when empty or not a date.
export function unixFromLocal(value) {
  if (!value) return 0
  const ms = new Date(value).getTime()
  return Number.isFinite(ms) ? Math.floor(ms / 1000) : 0
}
