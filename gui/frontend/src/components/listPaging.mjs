// Which part of a long list of backups to show (Restore's snapshots, Roll
// back's backups): the newest first, as many as the chosen size ("fit" the
// window, 25, 100 or all) plus any "Show more", optionally only those at or
// before a time. The selected one is always shown, so a choice made earlier
// (or by Roll back's date box) is never hidden below the cut.

export const SHOW_CHOICES = ['fit', '25', '100', 'all']
export const MORE_STEP = 25
// Shown before the window has been measured, and when it cannot be.
export const FIT_FALLBACK = 6

// newestFirst: a copy, newest first by its unix time.
export function newestFirst(items) {
  return [...(items || [])].sort((a, b) => (b.unix || 0) - (a.unix || 0))
}

// pageLimit: how many to show for the choice, the measured fit and the
// "Show more" clicks so far.
export function pageLimit(show, fit, extra = 0) {
  let base
  if (show === 'all') base = Infinity
  else if (show === 'fit') base = fit > 0 ? Math.floor(fit) : FIT_FALLBACK
  else base = Math.max(1, parseInt(show, 10) || MORE_STEP)
  return base + Math.max(0, extra | 0)
}

// pageItems: items newest first; before: unix seconds, 0 for no limit;
// isSelected: the selected item, which stretches the page down to it.
export function pageItems(items, { limit, before = 0, isSelected } = {}) {
  const all = items || []
  const matching = before > 0 ? all.filter(s => (s.unix || 0) <= before) : all
  let n = Math.min(limit ?? Infinity, matching.length)
  if (isSelected) {
    const i = matching.findIndex(isSelected)
    if (i >= n) n = i + 1
  }
  return { shown: matching.slice(0, n), matching: matching.length, total: all.length }
}

// fitCount: whole rows of rowHeight in the available height (at least one),
// times the columns.
export function fitCount({ available, rowHeight, columns = 1 }) {
  const cols = Math.max(1, Math.floor(columns) || 1)
  if (!(rowHeight > 0)) return FIT_FALLBACK
  return Math.max(1, Math.floor(available / rowHeight)) * cols
}

// unixFromLocal: a datetime-local value ("2026-11-07T14:30") as unix seconds,
// 0 when empty or not a date.
export function unixFromLocal(value) {
  if (!value) return 0
  const ms = new Date(value).getTime()
  return Number.isFinite(ms) ? Math.floor(ms / 1000) : 0
}
