import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from '../i18n/i18nContext'
import { MANUALS, REPO, renderManual, repoRef } from '../manuals'

const { GetManual, SaveManual } = window.go?.main?.App || {}

// The Help page: the user manuals built into the app, one at a time, with a
// contents list beside them. doc/anchor/nonce come from App so a menu item or
// a Help button elsewhere can open a given section; links inside the manuals
// call onOpen the same way.
export default function ManualViewer({ doc, anchor, nonce, onOpen, appVersion }) {
  const { t, language } = useTranslation()
  const [texts, setTexts] = useState({})
  const [error, setError] = useState('')
  const [saveMsg, setSaveMsg] = useState('')
  const [find, setFind] = useState('')
  const bodyRef = useRef(null)
  const ref = repoRef(appVersion)
  const manual = MANUALS.find((m) => m.id === doc) || MANUALS[0]

  useEffect(() => {
    if (texts[manual.id] !== undefined || !GetManual) return
    GetManual(manual.id)
      .then((md) => setTexts((prev) => ({ ...prev, [manual.id]: md })))
      .catch((e) => setError(String(e)))
  }, [manual.id])

  const rendered = useMemo(
    () => (texts[manual.id] === undefined ? null : renderManual(texts[manual.id], manual.id, ref)),
    [texts, manual.id, ref])

  // Scroll to the section asked for (or the top) once the manual is on screen.
  useEffect(() => {
    if (!rendered || !bodyRef.current) return
    const el = anchor ? bodyRef.current.querySelector(`[id="${CSS.escape(anchor)}"]`) : null
    if (el) el.scrollIntoView({ block: 'start' })
    else bodyRef.current.closest('.container')?.scrollTo({ top: 0 })
  }, [rendered, anchor, nonce])

  const onBodyClick = (e) => {
    const a = e.target.closest && e.target.closest('a[data-doc]')
    if (!a) return
    e.preventDefault()
    onOpen(a.dataset.doc, a.dataset.anchor)
  }

  const onFind = (e) => {
    if (e.key !== 'Enter' || !find) return
    e.preventDefault()
    // Chromium (WebView2) and WebKitGTK both have window.find.
    window.find?.(find, false, e.shiftKey, true)
  }

  const onSave = async () => {
    setSaveMsg('')
    try {
      const path = await SaveManual?.(manual.id)
      if (path) setSaveMsg(`${t('manualSaved')} ${path}`)
    } catch (err) {
      setSaveMsg(String(err))
    }
  }

  return (
    <div className="manual-page">
      <div className="manual-toolbar">
        <h2>{t('helpTitle')}</h2>
        <div className="manual-tabs" role="tablist">
          {MANUALS.map((m) => (
            <button key={m.id} role="tab" aria-selected={m.id === manual.id}
              className={`manual-tab ${m.id === manual.id ? 'active' : ''}`}
              onClick={() => onOpen(m.id, '')}>{t(m.label)}</button>
          ))}
        </div>
        <div className="manual-actions">
          <input type="search" className="manual-find" placeholder={t('manualFind')} value={find}
            onChange={(e) => setFind(e.target.value)} onKeyDown={onFind} />
          <button className="btn btn-secondary" onClick={onSave}>{t('manualSave')}</button>
          <a className="btn btn-secondary" href={`${REPO}/blob/${ref}/${manual.path}`} target="_blank"
            rel="noopener noreferrer" data-external="true">{t('manualOnGitHub')}</a>
        </div>
      </div>
      {language !== 'en' && <div className="manual-note">{t('manualEnglishOnly')}</div>}
      {saveMsg && <div className="manual-note">{saveMsg}</div>}
      {error && <div className="status error visible">{error}</div>}

      <div className="manual-layout">
        <nav className="manual-toc" aria-label={t('manualContents')}>
          <div className="manual-toc-title">{t('manualContents')}</div>
          {rendered?.headings.map((h) => (
            <a key={h.id} href="#" data-doc={manual.id} data-anchor={h.id}
              className={h.id === anchor ? 'active' : ''} onClick={onBodyClick}>{h.text}</a>
          ))}
        </nav>
        {rendered
          ? <article ref={bodyRef} className="manual-body" onClick={onBodyClick}
              dangerouslySetInnerHTML={{ __html: rendered.html }} />
          : <article className="manual-body">{error ? '' : '…'}</article>}
      </div>
    </div>
  )
}
