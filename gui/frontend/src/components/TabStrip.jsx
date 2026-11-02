import { useLayoutEffect, useRef, useState } from 'react'

// A row of .tabhead buttons that never leaves one tab alone on a second
// row: they stay on one row when they fit, and otherwise go into as few
// evenly filled rows as fit, in columns of one width (7 tabs: 4 and 3).
// It measures the tabs' own widths, so it suits every language, and lays
// out again when the window is resized or the tabs' names change.
export default function TabStrip({ children, className = '' }) {
  const ref = useRef(null)
  const [cols, setCols] = useState(0) // 0: one row

  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return undefined
    const layout = () => {
      // Measure every tab at its natural width, on one row.
      el.classList.add('measuring')
      const widths = [...el.children].map(c => c.getBoundingClientRect().width)
      el.classList.remove('measuring')
      const style = getComputedStyle(el)
      const room = el.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight)
      const n = widths.length
      let next = 0
      if (n > 0 && widths.reduce((a, b) => a + b, 0) > room) {
        const widest = Math.max(...widths)
        next = 1
        for (let rows = 2; rows <= n; rows++) {
          const c = Math.ceil(n / rows)
          if (room / c >= widest) { next = c; break }
        }
      }
      setCols(prev => (prev === next ? prev : next))
    }
    layout()
    const resize = new ResizeObserver(layout)
    resize.observe(el)
    const names = new MutationObserver(layout)
    names.observe(el, { childList: true, subtree: true, characterData: true })
    return () => { resize.disconnect(); names.disconnect() }
  }, [])

  return (
    <div
      ref={ref}
      className={`tabstrip ${cols ? 'banked' : ''} ${className}`.trim()}
      style={cols ? { gridTemplateColumns: `repeat(${cols}, 1fr)` } : undefined}
    >
      {children}
    </div>
  )
}
