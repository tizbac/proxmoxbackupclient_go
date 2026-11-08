import React, { useEffect, useLayoutEffect, useState } from 'react'
import { SHOW_CHOICES, MORE_STEP, fitCount } from './listPaging.mjs'

// The controls for a long list of backups (Restore's snapshots, Roll back's
// backups): "Showing 6 of 1,382", how many to show (as many as fit the
// window, 25, 100, all) and, where it is given, an "at or before" date. The
// choice of how many is kept for the next time. Which items those are comes
// from listPaging.mjs.

const SHOW_KEY = 'backupListShow'
// Below the list: its "Show more" row and the wizard's Back / Next row.
const RESERVE_PX = 140
// The "Show more" row: its button and the space above it (.list-more).
const MORE_ROW_PX = 54

// The chosen size is one setting for both lists: kept in localStorage, and
// handed straight to the other list's page while the app runs.
let currentShow = null
const showListeners = new Set()
function readShow() {
  if (currentShow === null) {
    try {
      const v = localStorage.getItem(SHOW_KEY)
      currentShow = SHOW_CHOICES.includes(v) ? v : 'fit'
    } catch { currentShow = 'fit' }
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
    currentShow = v
    try { localStorage.setItem(SHOW_KEY, v) } catch { /* ignore */ }
    showListeners.forEach(fn => fn(v))
  }
  return [show, choose]
}

function scrollParent(el) {
  for (let p = el && el.parentElement; p; p = p.parentElement) {
    const oy = getComputedStyle(p).overflowY
    if (oy === 'auto' || oy === 'scroll') return p
  }
  return null
}

// useFitCount: how many of the list's items fit between its top and the
// bottom of the window, less the rows below it, measured from the first item
// shown (its height, and for a grid its columns). 0 until measured. ref is
// the element whose children are the items; deps re-measure it.
export function useFitCount(ref, deps) {
  const [fit, setFit] = useState(0)
  useLayoutEffect(() => {
    const measure = () => {
      const box = ref.current
      const items = box ? box.children.length : 0
      const rect = box && box.getBoundingClientRect()
      // Nothing to measure while its tab or step is hidden (display: none):
      // the last count stands until it shows again.
      if (!items || !(rect.height > 0)) return
      const cs = getComputedStyle(box)
      const gap = parseFloat(cs.rowGap) || 0
      // Every track the grid has room for, the empty ones too: auto-fit
      // reports those as 0px, and counting only the filled ones would cap
      // the count at what is shown now.
      const columns = cs.display === 'grid'
        ? cs.gridTemplateColumns.split(' ').filter(Boolean).length
        : 1
      // The average row: rows differ (a disk image's has a second line).
      const filledColumns = Math.min(columns, items)
      const rows = Math.ceil(items / filledColumns)
      const rowHeight = (rect.height + gap) / rows
      // What must fit below the list: down to the wizard's Back / Next row
      // ("Show more" included), and the page's bottom padding. Not anything
      // further down, such as a status line that goes after 5 seconds.
      // RESERVE_PX stands in when there is no such row.
      const scroller = scrollParent(box)
      const viewBottom = scroller
        ? scroller.getBoundingClientRect().top + scroller.clientHeight - (parseFloat(getComputedStyle(scroller).paddingBottom) || 0)
        : window.innerHeight
      const scope = box.closest('.tab-content') || document
      const nav = [...scope.querySelectorAll('.wiz-nav')].find(el => el.getBoundingClientRect().top >= rect.bottom)
      let below = nav ? nav.getBoundingClientRect().bottom - rect.bottom : RESERVE_PX
      // Measured while every item showed, there was no "Show more" row; a
      // cut list brings it back, so its room is kept.
      const hasMore = [...scope.querySelectorAll('.list-more')].some(el => el.getBoundingClientRect().top >= rect.bottom)
      if (nav && !hasMore) below += MORE_ROW_PX
      const available = viewBottom - rect.top - below
      const n = fitCount({ available, rowHeight, columns })
      setFit(prev => (prev === n ? prev : n))
    }
    measure()
    window.addEventListener('resize', measure)
    return () => window.removeEventListener('resize', measure)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)
  return fit
}

// ListPagerBar: page is pageItems()'s result. before / onBefore: the
// datetime-local value of the date box and its setter (no box without them);
// beforeLabel names it.
export function ListPagerBar({ tl, page, show, onShow, before, onBefore, beforeLabel, idPrefix }) {
  const fmt = (n) => n.toLocaleString()
  const sizes = {
    fit: tl('listShowFit', 'As many as fit'),
    25: '25',
    100: '100',
    all: tl('listShowAll', 'All'),
  }
  return (
    <div className="list-pager">
      <span className="list-pager-count">
        {tl('listShowing', 'Showing {n} of {total}')
          .replace('{n}', fmt(page.shown.length)).replace('{total}', fmt(page.matching))}
      </span>
      <label className="list-pager-field" htmlFor={`${idPrefix}-show`}>
        {tl('listShow', 'Show')}
        <select id={`${idPrefix}-show`} value={show} onChange={(e) => onShow(e.target.value)}>
          {SHOW_CHOICES.map(v => <option key={v} value={v}>{sizes[v]}</option>)}
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

// ListMore: "Show 25 more" under the list while some are left out.
export function ListMore({ tl, page, onMore }) {
  const left = page.matching - page.shown.length
  if (left <= 0) return null
  return (
    <div className="list-more">
      <button type="button" className="btn btn-secondary" onClick={onMore}>
        {tl('listShowMore', 'Show {n} more').replace('{n}', Math.min(MORE_STEP, left).toLocaleString())}
      </button>
    </div>
  )
}
