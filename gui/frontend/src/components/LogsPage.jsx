import { useEffect, useRef, useState } from 'react'

// View Full Logs: the log files the app and the background service write,
// newest first, and the one picked, a page at a time (the end of it, or the
// lines around a time when opened from Reports or the Message Log). Chunk
// lines ("New chunk[...]", one per 4 MiB) are hidden by default: a run's log
// is mostly those.

const call = (fn, ...a) => {
  const app = window.go && window.go.main && window.go.main.App
  return app && app[fn] ? app[fn](...a) : Promise.reject(new Error('not available'))
}

const pad = (n) => String(n).padStart(2, '0')

// A local time as the log lines write it ("2026-10-31 23:26:02").
export const logStamp = (ms) => {
  const d = new Date(ms)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

// Where to open the log of a Reports entry or a Message Log line: the run's
// own log when there is one, else the backup (or app) log around its time.
// Returns {folder, name, around, notice} for LogsPage's request, or null.
export async function logRequestFor({ backupId, at, kind, notice }) {
  if (backupId && kind !== 'restore' && kind !== 'app') {
    try {
      const f = await call('FindRunLog', backupId, at)
      if (f && f.name) return { folder: f.folder, name: f.name, around: '', notice: '' }
    } catch { /* none: the shared log below */ }
  }
  let files = []
  try { files = (await call('ListLogFiles')) || [] } catch { return null }
  const want = kind === 'app' ? 'app' : 'backup'
  // The current file of that kind (not a rotated part), the app's own first.
  const current = files.filter(f => f.kind === want && /\.log$/.test(f.name)).sort((a, b) => (a.service === b.service ? 0 : a.service ? 1 : -1))
  const f = current[0] || files.find(x => x.kind === want)
  if (!f) return null
  return { folder: f.folder, name: f.name, around: logStamp(at), notice: notice || '' }
}

export default function LogsPage({ t, language, active, request, onOpenFolder, formatBytes }) {
  const [files, setFiles] = useState([])
  const [sel, setSel] = useState(null) // {folder, name}
  const [view, setView] = useState(null)
  const [filter, setFilter] = useState('')
  const [applied, setApplied] = useState('')
  const [hideChunks, setHideChunks] = useState(true)
  const [lines, setLines] = useState(2000)
  const [around, setAround] = useState('')
  const [notice, setNotice] = useState('')
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState('')
  const [reload, setReload] = useState(0)
  const boxRef = useRef(null)
  const focusRef = useRef(null)

  const loadFiles = () => call('ListLogFiles').then(f => { setFiles(f || []); return f || [] }).catch(e => { setErr(String(e)); return [] })

  useEffect(() => {
    if (!active) return
    loadFiles().then(f => { if (!sel && !request && f.length > 0) setSel({ folder: f[0].folder, name: f[0].name }) })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active])

  useEffect(() => {
    if (!request) return
    setSel({ folder: request.folder, name: request.name })
    setAround(request.around || '')
    setNotice(request.notice || '')
    setLines(2000)
    loadFiles()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [request])

  useEffect(() => {
    if (!sel) return
    setLoading(true)
    setErr('')
    call('ReadLog', { folder: sel.folder, name: sel.name, filter: applied, hideChunks, lines, around })
      .then(v => setView(v))
      .catch(e => { setErr(String(e)); setView(null) })
      .finally(() => setLoading(false))
  }, [sel, applied, hideChunks, lines, around, reload])

  // The line asked for in the middle, else the end of the log.
  useEffect(() => {
    if (focusRef.current) {
      focusRef.current.scrollIntoView({ block: 'center' })
    } else if (boxRef.current) {
      boxRef.current.scrollTop = boxRef.current.scrollHeight
    }
  }, [view])

  const pick = (f) => { setSel({ folder: f.folder, name: f.name }); setAround(''); setNotice(''); setLines(2000) }

  const label = (f) => {
    const base = f.kind === 'run' ? t('logsKindRun', { id: f.backupId })
      : f.kind === 'backup' ? t('logsKindBackup')
        : t('logsKindApp')
    return f.service ? `${base} (${t('logsService')})` : base
  }
  const when = (f) => new Date(f.kind === 'run' && f.started ? f.started : f.modified).toLocaleString(language)
  const selFile = sel && files.find(f => f.folder === sel.folder && f.name === sel.name)
  const lineStyle = (s) => /\[ERROR\]|FAILED|Failed|failed|Cannot |CRITICAL|panic/.test(s) ? { color: '#c0392b' }
    : /WARNING|⚠️|Skipped|skipped/.test(s) ? { color: '#a06000' } : undefined

  return (
    <div>
      <h2>{t('logsTitle')}</h2>
      {files.length === 0 ? (
        <p style={{ color: '#718096' }}>{t('logsNone')}</p>
      ) : (
        <div style={{ display: 'flex', gap: '16px', alignItems: 'flex-start' }}>
          <div style={{ flex: '0 0 280px', border: '1px solid #e2e8f0', borderRadius: '8px', overflow: 'hidden' }}>
            <div style={{ maxHeight: '620px', overflowY: 'auto' }}>
              {files.map((f, i) => {
                const on = sel && sel.folder === f.folder && sel.name === f.name
                return (
                  <div key={`${f.folder}/${f.name}`} onClick={() => pick(f)} title={f.name}
                    style={{ padding: '9px 12px', cursor: 'pointer', borderTop: i === 0 ? 'none' : '1px solid #e2e8f0', backgroundColor: on ? '#eff6ff' : '#fff' }}>
                    <div style={{ fontSize: '13px', fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{label(f)}</div>
                    <div style={{ fontSize: '12px', color: '#718096' }}>{when(f)} · {formatBytes(f.size)}{f.compressed ? ' (gz)' : ''}</div>
                  </div>
                )
              })}
            </div>
          </div>
          <div style={{ flex: '1 1 auto', minWidth: 0 }}>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', alignItems: 'center', marginBottom: '8px' }}>
              <input type="text" value={filter} placeholder={t('logsFilter')} style={{ flex: '1 1 220px', minWidth: 0 }}
                onChange={e => setFilter(e.target.value)} onKeyDown={e => { if (e.key === 'Enter') { setAround(''); setApplied(filter) } }} />
              <button className="btn btn-secondary" onClick={() => { setAround(''); setApplied(filter) }}>{t('logsApply')}</button>
              <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '13px' }}>
                <input type="checkbox" checked={hideChunks} onChange={e => setHideChunks(e.target.checked)} />
                {t('logsHideChunks')}
              </label>
              <button className="btn btn-secondary" onClick={() => { loadFiles(); setReload(n => n + 1) }}>{t('logsRefresh')}</button>
              {onOpenFolder && <button className="btn btn-secondary" onClick={onOpenFolder}>{t('logsOpenFolder')}</button>}
            </div>
            {notice && <div className="info-box" style={{ marginBottom: '8px' }}>ℹ️ {notice}</div>}
            {err && <div className="status error visible" style={{ marginBottom: '8px' }}>❌ {err}</div>}
            {selFile && (
              <div style={{ fontSize: '12px', color: '#718096', marginBottom: '6px' }}>
                {selFile.name}
                {view && <> · {t('logsShowing', { shown: view.lines.length.toLocaleString(language), total: view.matching.toLocaleString(language) })}</>}
                {loading && <> · {t('logsLoading')}</>}
              </div>
            )}
            {view && view.first > 0 && (
              <button className="btn btn-secondary" style={{ marginBottom: '6px' }} onClick={() => setLines(n => Math.min(n * 4, 200000))}>
                {t('logsShowMore')}
              </button>
            )}
            <div ref={boxRef} style={{ height: '560px', overflow: 'auto', border: '1px solid #e2e8f0', borderRadius: '6px', backgroundColor: '#fbfbfc', padding: '8px', fontFamily: 'Consolas, "DejaVu Sans Mono", monospace', fontSize: '12px', lineHeight: 1.45 }}>
              {view && view.lines.map((s, i) => (
                <div key={i} ref={i === view.focus ? focusRef : null}
                  style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all', ...(i === view.focus ? { backgroundColor: '#fff3bf' } : {}), ...lineStyle(s) }}>
                  {s}
                </div>
              ))}
              {view && view.lines.length === 0 && <span style={{ color: '#718096' }}>{t('logsNoLines')}</span>}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
