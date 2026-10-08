import { useEffect, useRef, useState } from 'react'
import { useTranslation } from '../i18n/i18nContext'

// In-app replacement for window.confirm and window.alert, which WebView2
// shows as a bare system box titled "wails.localhost". Mount <ConfirmHost />
// once; anywhere else call
//
//   if (!(await askConfirm({ title, message, confirmLabel, danger: true }))) return
//   await showNotice({ title, message })
//
// Requests queue, so two at once are shown one after the other. Escape,
// the Cancel button and a click outside answer "no"; danger dialogs start
// with the focus on Cancel so Enter never does the risky thing by accident.

let enqueue = null

export function askConfirm(opts) {
  return new Promise((resolve) => {
    if (!enqueue) { resolve(window.confirm(opts.message)); return } // eslint-disable-line no-alert
    enqueue({ ...opts, kind: 'confirm', resolve })
  })
}

export function showNotice(opts) {
  return new Promise((resolve) => {
    if (!enqueue) { window.alert(opts.message); resolve(); return } // eslint-disable-line no-alert
    enqueue({ ...opts, kind: 'notice', resolve: () => resolve() })
  })
}

export default function ConfirmHost() {
  const { t } = useTranslation()
  const tl = (key, fallback) => { const v = t(key); return v && v !== key ? v : fallback }
  const [queue, setQueue] = useState([])
  const boxRef = useRef(null)
  const returnFocus = useRef(null)

  useEffect(() => {
    enqueue = (req) => setQueue((q) => [...q, req])
    return () => { enqueue = null }
  }, [])

  const req = queue[0]

  useEffect(() => {
    if (!req) return undefined
    if (!returnFocus.current) returnFocus.current = document.activeElement
    const first = boxRef.current?.querySelector('[data-autofocus]')
    if (first) first.focus()
    return undefined
  }, [req])

  const answer = (value) => {
    if (!req) return
    req.resolve(value)
    setQueue((q) => {
      const rest = q.slice(1)
      if (rest.length === 0 && returnFocus.current) {
        const el = returnFocus.current
        returnFocus.current = null
        setTimeout(() => { if (el && document.contains(el)) el.focus() }, 0)
      }
      return rest
    })
  }

  if (!req) return null

  const isNotice = req.kind === 'notice'
  const danger = !!req.danger
  const headerBg = danger ? '#c53030' : 'var(--accent)'
  const icon = req.icon || (danger ? '⚠️' : isNotice ? 'ℹ️' : '❓')
  const title = req.title || (isNotice ? tl('noticeTitle', 'Information') : tl('confirmTitle', 'Please confirm'))
  const okLabel = req.confirmLabel || (isNotice ? tl('prefsOK', 'OK') : tl('confirmContinue', 'Continue'))
  const cancelLabel = req.cancelLabel || tl('cancel', 'Cancel')

  // Keep Tab inside the dialog and let Escape cancel.
  const onKeyDown = (e) => {
    if (e.key === 'Escape') { e.preventDefault(); answer(isNotice ? undefined : false); return }
    if (e.key !== 'Tab') return
    const items = [...boxRef.current.querySelectorAll('button')]
    if (items.length === 0) return
    const i = items.indexOf(document.activeElement)
    const next = e.shiftKey ? (i <= 0 ? items.length - 1 : i - 1) : (i === items.length - 1 ? 0 : i + 1)
    e.preventDefault()
    items[next].focus()
  }

  return (
    <div
      style={{
        position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)',
        display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1300, padding: '16px',
      }}
      onMouseDown={(e) => { if (e.target === e.currentTarget) answer(isNotice ? undefined : false) }}
      onKeyDown={onKeyDown}
    >
      <div
        ref={boxRef}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-dialog-title"
        aria-describedby="confirm-dialog-body"
        style={{
          maxWidth: '520px', width: '100%', background: '#fff', borderRadius: '8px',
          overflow: 'hidden', boxShadow: '0 12px 44px rgba(0,0,0,.35)',
        }}
      >
        <div
          id="confirm-dialog-title"
          style={{
            background: headerBg, color: '#fff', padding: '12px 18px',
            fontSize: '16px', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '10px',
          }}
        >
          <span aria-hidden="true">{icon}</span>
          <span>{title}</span>
        </div>
        <div
          id="confirm-dialog-body"
          style={{ padding: '18px', color: '#2d3748', lineHeight: 1.55, fontSize: '14px', whiteSpace: 'pre-line', overflowWrap: 'anywhere', maxHeight: '60vh', overflowY: 'auto' }}
        >
          {req.message}
          {req.details && req.details.length > 0 && (
            <dl style={{ display: 'grid', gridTemplateColumns: 'max-content 1fr', gap: '4px 14px', margin: '12px 0 0', fontSize: '13px' }}>
              {req.details.map(([k, v]) => (
                <div key={k} style={{ display: 'contents' }}>
                  <dt style={{ color: '#64748b' }}>{k}</dt>
                  <dd style={{ margin: 0, fontWeight: 500 }}>{v}</dd>
                </div>
              ))}
            </dl>
          )}
        </div>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', padding: '0 18px 18px', flexWrap: 'wrap' }}>
          {!isNotice && (
            <button className="btn btn-secondary" style={{ margin: 0 }} onClick={() => answer(false)} data-autofocus={danger ? true : undefined}>
              {cancelLabel}
            </button>
          )}
          <button
            className="btn"
            style={{ margin: 0, ...(danger ? { background: '#c53030' } : {}) }}
            onClick={() => answer(isNotice ? undefined : true)}
            data-autofocus={danger ? undefined : true}
          >
            {okLabel}
          </button>
        </div>
      </div>
    </div>
  )
}
