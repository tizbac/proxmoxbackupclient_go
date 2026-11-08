import React, { useEffect, useState } from 'react'
import { SHOW_CHOICES, showChoice } from './listPaging.mjs'

// The controls for a long list of backups (Restore's snapshots, Roll back's
// backups): « ‹ "Showing 11-20 of 1,382" › » to step through the pages, how
// many a page holds (5, 10, 25, 50 or all) and, where it is given, an "at or
// before" date. The page size is kept for the next time. Which items a page
// holds comes from listPaging.mjs.

const SHOW_KEY = 'backupListShow'

// The chosen size is one setting for both lists: kept in localStorage, and
// handed straight to the other list's page while the app runs.
let currentShow = null
const showListeners = new Set()
function readShow() {
  if (currentShow === null) {
    try {
      currentShow = showChoice(localStorage.getItem(SHOW_KEY))
    } catch { currentShow = showChoice(null) }
  }
  return currentShow
}

// useListShow: the chosen size and its setter.
export function useListShow() {
  const [show, setShow] = useState(readShow)
  useEffect(() => {
    showListeners.add(setShow)
    return () => { showListeners.delete(setShow) }
  }, [])
  const choose = (v) => {
    currentShow = showChoice(v)
    try { localStorage.setItem(SHOW_KEY, currentShow) } catch { /* ignore */ }
    showListeners.forEach(fn => fn(currentShow))
  }
  return [show, choose]
}

// ListPageNav: « ‹ Showing 11-20 of 120 › ». page is pageItems()'s result;
// onPage(n) goes to page n (0-based). The arrows are greyed on the first or
// last page, and all four when everything fits on one.
export function ListPageNav({ tl, page, onPage }) {
  const fmt = (n) => n.toLocaleString()
  const count = page.matching > 0
    ? tl('listShowingRange', 'Showing {from}-{to} of {total}')
      .replace('{from}', fmt(page.from)).replace('{to}', fmt(page.to)).replace('{total}', fmt(page.matching))
    : tl('listShowing', 'Showing {n} of {total}').replace('{n}', '0').replace('{total}', '0')
  const atFirst = page.page <= 0
  const atLast = page.page >= page.pages - 1
  const arrow = (label, text, to, disabled) => (
    <button type="button" className="btn btn-secondary list-page-btn" disabled={disabled}
      onClick={() => onPage(to)} title={label} aria-label={label}>{text}</button>
  )
  return (
    <span className="list-page-nav">
      {arrow(tl('listFirstPage', 'First page'), '«', 0, atFirst)}
      {arrow(tl('listPrevPage', 'Previous page'), '‹', page.page - 1, atFirst)}
      <span className="list-pager-count" aria-live="polite">{count}</span>
      {arrow(tl('listNextPage', 'Next page'), '›', page.page + 1, atLast)}
      {arrow(tl('listLastPage', 'Last page'), '»', page.pages - 1, atLast)}
    </span>
  )
}

// ListPagerBar: the page arrows and count, the page size, and the date box.
// before / onBefore: the datetime-local value of the date box and its setter
// (no box without them); beforeLabel names it.
export function ListPagerBar({ tl, page, onPage, show, onShow, before, onBefore, beforeLabel, idPrefix }) {
  const sizes = { all: tl('listShowAll', 'All') }
  return (
    <div className="list-pager">
      <ListPageNav tl={tl} page={page} onPage={onPage} />
      <label className="list-pager-field" htmlFor={`${idPrefix}-show`}>
        {tl('listShow', 'Show')}
        <select id={`${idPrefix}-show`} value={show} onChange={(e) => onShow(e.target.value)}>
          {SHOW_CHOICES.map(v => <option key={v} value={v}>{sizes[v] || v}</option>)}
        </select>
      </label>
      {onBefore && (
        <span className="list-pager-field">
          <label className="list-pager-field" htmlFor={`${idPrefix}-before`}>
            {beforeLabel || tl('listAtOrBefore', 'At or before:')}
            <input id={`${idPrefix}-before`} type="datetime-local" value={before} onChange={(e) => onBefore(e.target.value)} />
          </label>
          {before && (
            <button type="button" className="wiz-link" onClick={() => onBefore('')} title={tl('listClearDate', 'Clear the date')}
              aria-label={tl('listClearDate', 'Clear the date')}>✕</button>
          )}
        </span>
      )}
    </div>
  )
}

// ListPagerFoot: the page arrows again under a list that runs to more than
// one page, so the next page is in reach without going back up.
export function ListPagerFoot({ tl, page, onPage }) {
  if (page.pages <= 1) return null
  return (
    <div className="list-pager list-pager-foot">
      <ListPageNav tl={tl} page={page} onPage={onPage} />
    </div>
  )
}
