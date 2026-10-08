import { useState, useEffect, useMemo, useRef } from 'react'
import RadioGroup from './RadioGroup'

// Undelete and Roll back pages (side menu) and the Run as Service section of
// Preferences > Advanced. Backend: gui/undelete_rollback.go,
// gui/service_control_windows.go. Helpers that live in App.jsx (formatting,
// WaitBar, Wails bindings) come in through the `ui` prop.

const fileKey = (f) => f.folder + '\u0000' + f.path

function fmtDate(unix) {
  if (!unix) return ''
  return new Date(unix * 1000).toLocaleString(undefined, { weekday: 'short', day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })
}

// Scan progress from the backend's setscan:progress events, as one line.
// Subscribed for the page's whole life: the first event can arrive before a
// re-render would have subscribed. Lines arriving while idle are ignored.
// Also returns how far through the current step it is (0..1), or null when
// the step has no count.
function useScanProgress(ui, active) {
  const [line, setLine] = useState('')
  const [fraction, setFraction] = useState(null)
  useEffect(() => {
    if (!ui.EventsOn) return undefined
    const off = ui.EventsOn('setscan:progress', (d) => {
      const tl = ui.tl
      const done = d.done || 0
      const total = d.total || 0
      const num = (n) => n.toLocaleString()
      switch (d.phase) {
        case 'snapshots':
          setLine(tl('scanPhaseSnapshots', 'Listing the set\'s snapshots...')); setFraction(null); break
        case 'index':
          setLine(tl('scanPhaseIndex', 'Reading the file list of snapshot {n} of {total}').replace('{n}', done + 1).replace('{total}', total || 1))
          setFraction(total > 0 ? done / total : null); break
        case 'walk':
          // total is what the newest backup had there: an estimate.
          setLine(total > 0
            ? tl('scanPhaseWalkCount', 'Reading the folder {folder}: {n} of about {total} files and folders').replace('{folder}', d.detail || '').replace('{n}', num(done)).replace('{total}', num(total))
            : tl('scanPhaseWalkSoFar', 'Reading the folder {folder}: {n} files and folders so far').replace('{folder}', d.detail || '').replace('{n}', num(done)))
          setFraction(total > 0 ? Math.min(1, done / total) : null); break
        case 'compare':
          setLine(tl('scanPhaseCompare', 'Comparing...')); setFraction(null); break
        default:
          setLine(''); setFraction(null)
      }
    })
    return () => { if (typeof off === 'function') off() }
  }, [])
  return [active ? line : '', setLine, active ? fraction : null]
}

export function SetPicker({ ui, jobs, value, onChange }) {
  const { tl } = ui
  const folderSets = (jobs || []).filter(j => j.backupType !== 'machine')
  return (
    <div className="form-group">
      <label htmlFor={`setpick-${ui.idSuffix || ''}`}>{tl('setPickerLabel', 'Backup Set')}</label>
      <select id={`setpick-${ui.idSuffix || ''}`} value={value || ''} onChange={(e) => onChange(e.target.value)}>
        <option value="">{tl('setPickerChoose', 'Choose a folder Backup Set...')}</option>
        {folderSets.map(j => (
          <option key={j.id} value={j.id}>{j.name} ({(j.backupDirs || []).join(', ')})</option>
        ))}
      </select>
      {folderSets.length === 0 && (
        <p style={{fontSize: '13px', color: '#718096', marginTop: '6px'}}>
          {tl('setPickerNone', 'There are no folder Backup Sets yet. Create one on the Backup page; machine sets are restored with bare-metal restore.')}
        </p>
      )}
    </div>
  )
}

// ---- Undelete -------------------------------------------------------------

function buildTree(files) {
  const roots = new Map()
  for (const f of files) {
    let root = roots.get(f.folder)
    if (!root) {
      root = { key: f.folder, name: f.folder, dirs: new Map(), files: [], count: 0, size: 0 }
      roots.set(f.folder, root)
    }
    let node = root
    root.count++; root.size += f.size
    const parts = f.path.split('/')
    for (let i = 0; i < parts.length - 1; i++) {
      let child = node.dirs.get(parts[i])
      if (!child) {
        child = { key: node.key + '/' + parts[i], name: parts[i], dirs: new Map(), files: [], count: 0, size: 0 }
        node.dirs.set(parts[i], child)
      }
      child.count++; child.size += f.size
      node = child
    }
    node.files.push(f)
  }
  return [...roots.values()]
}

function nodeFiles(node, out = []) {
  out.push(...node.files)
  for (const c of node.dirs.values()) nodeFiles(c, out)
  return out
}

function TreeNode({ ui, node, depth, expanded, toggleExpand, selected, setSelected }) {
  const { tl, formatBytesDual } = ui
  const all = useMemo(() => nodeFiles(node), [node])
  const n = all.filter(f => selected.has(fileKey(f))).length
  const boxRef = useRef(null)
  useEffect(() => { if (boxRef.current) boxRef.current.indeterminate = n > 0 && n < all.length }, [n, all.length])
  const open = expanded.has(node.key)
  const toggleAll = () => {
    const next = new Set(selected)
    if (n === all.length) all.forEach(f => next.delete(fileKey(f)))
    else all.forEach(f => next.add(fileKey(f)))
    setSelected(next)
  }
  return (
    <div>
      <div style={{display: 'flex', alignItems: 'center', gap: '6px', padding: '3px 0', paddingLeft: depth * 18}}>
        <input ref={boxRef} type="checkbox" checked={n === all.length && all.length > 0} onChange={toggleAll} style={{width: 'auto', margin: 0}} aria-label={node.name} />
        <button type="button" className="tree-toggle" onClick={() => toggleExpand(node.key)} aria-expanded={open}
          style={{background: 'none', border: 'none', padding: 0, cursor: 'pointer', fontWeight: 600, color: '#2d3748', textAlign: 'left'}}>
          {open ? '▾' : '▸'} 📁 {node.name}
        </button>
        <span style={{fontSize: '12px', color: '#718096'}}>
          {tl('undeleteNodeCount', '{n} files, {size}').replace('{n}', node.count.toLocaleString()).replace('{size}', formatBytesDual(node.size))}
        </span>
      </div>
      {open && (
        <>
          {[...node.dirs.values()].sort((a, b) => a.name.localeCompare(b.name)).map(c => (
            <TreeNode key={c.key} ui={ui} node={c} depth={depth + 1} expanded={expanded} toggleExpand={toggleExpand} selected={selected} setSelected={setSelected} />
          ))}
          {node.files.slice().sort((a, b) => a.path.localeCompare(b.path)).map(f => {
            const k = fileKey(f)
            const name = f.path.split('/').pop()
            return (
              <label key={k} style={{display: 'flex', alignItems: 'baseline', gap: '6px', padding: '2px 0', paddingLeft: (depth + 1) * 18, fontWeight: 'normal', margin: 0, cursor: 'pointer'}}>
                <input type="checkbox" checked={selected.has(k)} style={{width: 'auto', margin: 0}}
                  onChange={() => { const next = new Set(selected); if (next.has(k)) next.delete(k); else next.add(k); setSelected(next) }} />
                <span style={{fontFamily: 'Consolas, monospace', fontSize: '12px'}}>{name}</span>
                <span style={{fontSize: '12px', color: '#718096'}}>
                  {formatBytesDual(f.size)} · {f.goneByUnix
                    ? tl('undeleteGoneBetween', 'deleted between {a} and {b}').replace('{a}', fmtDate(f.snapshotUnix)).replace('{b}', fmtDate(f.goneByUnix))
                    : tl('undeleteGoneSince', 'deleted since {a}').replace('{a}', fmtDate(f.snapshotUnix))}
                </span>
                {f.movedTo && (
                  <span style={{fontSize: '12px', color: '#a06000'}}>
                    {tl('undeleteMovedTo', 'probably moved to {path}').replace('{path}', f.movedTo)}
                  </span>
                )}
              </label>
            )
          })}
        </>
      )}
    </div>
  )
}

export function UndeletePage({ ui, jobs, jobId, setJobId, restoreLoading, onStarted, restoreCard }) {
  const { t, tl, formatBytesDual, WaitBar } = ui
  const [days, setDays] = useState('0')
  const [scan, setScan] = useState(null)
  const [scanning, setScanning] = useState(null) // start time while scanning
  const [stale, setStale] = useState(false)
  const [error, setError] = useState('')
  const [filter, setFilter] = useState('')
  const [selected, setSelected] = useState(new Set())
  const [expanded, setExpanded] = useState(new Set())
  const [dest, setDest] = useState('original')
  const [destPath, setDestPath] = useState('')
  const [verify, setVerify] = useState(true)
  const [progressLine, setProgressLine, progressFraction] = useScanProgress(ui, !!scanning)
  const [, tick] = useState(0)

  useEffect(() => {
    if (!scanning) return undefined
    const id = setInterval(() => tick(n => n + 1), 1000)
    return () => clearInterval(id)
  }, [scanning])
  useEffect(() => { setScan(null); setSelected(new Set()); setError('') }, [jobId])
  useEffect(() => {
    if (!ui.EventsOn) return undefined
    const off = ui.EventsOn('restore:complete', (d) => { if (d && d.kind === 'undelete') setStale(true) })
    return () => { if (typeof off === 'function') off() }
  }, [])

  const runScan = async () => {
    if (!jobId) return
    setError(''); setScan(null); setSelected(new Set()); setStale(false); setProgressLine('')
    setScanning(Date.now())
    try {
      const res = await ui.ScanUndelete(jobId, parseInt(days, 10))
      setScan(res)
      const files = res.files || []
      setSelected(new Set(files.filter(f => !f.movedTo).map(fileKey)))
      const exp = new Set([...new Set(files.map(f => f.folder))])
      if (files.length <= 200) buildTree(files).forEach(function walk(n) { exp.add(n.key); n.dirs.forEach(walk) })
      setExpanded(exp)
    } catch (err) {
      setError(String(err))
    } finally {
      setScanning(null)
    }
  }

  const visible = useMemo(() => {
    const files = (scan && scan.files) || []
    const q = filter.trim().toLowerCase()
    return q ? files.filter(f => (f.folder + '/' + f.path).toLowerCase().includes(q)) : files
  }, [scan, filter])
  const tree = useMemo(() => buildTree(visible), [visible])
  const chosen = useMemo(() => ((scan && scan.files) || []).filter(f => selected.has(fileKey(f))), [scan, selected])
  const chosenSize = chosen.reduce((s, f) => s + f.size, 0)
  const toggleExpand = (k) => { const next = new Set(expanded); if (next.has(k)) next.delete(k); else next.add(k); setExpanded(next) }

  const start = async () => {
    if (chosen.length === 0) return
    if (dest === 'other' && !destPath) { setError(t('destinationRequired')); return }
    const msg = dest === 'original'
      ? tl('undeleteConfirmOriginal', 'Put {n} files ({size}) back where they were? Files that exist again at those places are left alone.')
      : tl('undeleteConfirmOther', 'Restore {n} files ({size}) to {path}?').replace('{path}', destPath)
    // eslint-disable-next-line no-alert
    if (!window.confirm(msg.replace('{n}', chosen.length.toLocaleString()).replace('{size}', formatBytesDual(chosenSize)))) return
    setError('')
    try {
      onStarted()
      await ui.StartUndelete(jobId, chosen.map(f => ({ archive: f.archive, path: f.path, snapshotUnix: f.snapshotUnix })), dest === 'original' ? '' : destPath, verify)
    } catch (err) {
      ui.onStartFailed(String(err))
    }
  }

  return (
    <div className="card">
      <h2 style={{marginTop: 0}}>{tl('undeleteTitle', 'Undelete')}</h2>
      <p style={{marginTop: 0, color: '#4a5568', fontSize: '14px'}}>
        {tl('undeleteIntro', 'Finds files that are in a Backup Set\'s backups but no longer on disk, and puts them back. Only folder listings are read; no file is opened.')}
      </p>
      {restoreCard}
      <SetPicker ui={{...ui, idSuffix: 'undelete'}} jobs={jobs} value={jobId} onChange={setJobId} />
      <div className="form-group">
        <label id="undelete-days-label">{tl('undeleteLookBack', 'Look back')}</label>
        <RadioGroup name="undelete-days" labelId="undelete-days-label" value={days} onChange={setDays} disabled={!!scanning}
          options={[
            { value: '0', label: tl('undeleteLatest', 'Latest backup') },
            { value: '7', label: tl('undeleteDays7', 'Last 7 days') },
            { value: '30', label: tl('undeleteDays30', 'Last 30 days') },
          ]} />
      </div>
      <div style={{display: 'flex', gap: '10px', flexWrap: 'wrap'}}>
        <button className="btn" onClick={runScan} disabled={!jobId || !!scanning || restoreLoading}>{tl('undeleteFind', 'Find deleted files')}</button>
        {scanning && <button className="btn btn-secondary" onClick={() => ui.CancelSetScan()}>{t('cancel') !== 'cancel' ? t('cancel') : 'Cancel'}</button>}
      </div>
      {scanning && <WaitBar label={progressLine || tl('undeleteScanning', 'Looking for deleted files...')} startedAt={scanning} fraction={progressFraction}
        slowHint={tl('undeleteSlowHint', 'A large set takes a few minutes: every folder is listed, and each snapshot\'s file list is read once and kept on this computer.')} />}
      {error && <div className="info-box" style={{borderColor: '#e53e3e', color: '#c53030', marginTop: '12px'}}>❌ {error}</div>}

      {scan && (
        <div style={{marginTop: '16px'}}>
          <div style={{fontSize: '14px', marginBottom: '8px'}}>
            <strong>
              {(scan.files || []).length === 0
                ? tl('undeleteNone', 'Nothing is missing: every file in the checked backups is on disk.')
                : tl('undeleteFound', '{n} files ({size}) are in the backups but not on disk.').replace('{n}', (scan.files || []).length.toLocaleString()).replace('{size}', formatBytesDual((scan.files || []).reduce((s, f) => s + f.size, 0)))}
            </strong>
            <span style={{color: '#718096'}}> {scan.snapshotsChecked === 1
              ? tl('undeleteChecked1', 'Checked the backup of {a}.').replace('{a}', fmtDate(scan.newestUnix))
              : tl('undeleteCheckedN', 'Checked {n} backups, {a} to {b}.').replace('{n}', scan.snapshotsChecked).replace('{a}', fmtDate(scan.oldestUnix)).replace('{b}', fmtDate(scan.newestUnix))}</span>
          </div>
          {stale && <div className="info-box" style={{marginBottom: '8px'}}>ℹ️ {tl('undeleteStale', 'These results are from before the last undelete. Find deleted files again to refresh them.')}</div>}
          {scan.unknown > 0 && (
            <div className="info-box" style={{marginBottom: '8px'}}>
              ⚠️ {tl('undeleteUnknown', '{n} files are under folders that could not be read now, so they are not listed:').replace('{n}', scan.unknown)} {(scan.unreadableFolders || []).slice(0, 5).join(', ')}
            </div>
          )}
          {(scan.files || []).length > 0 && (
            <>
              <div style={{display: 'flex', gap: '10px', alignItems: 'center', flexWrap: 'wrap', marginBottom: '8px'}}>
                <input type="search" value={filter} onChange={(e) => setFilter(e.target.value)} placeholder={tl('undeleteFilter', 'Filter by name or folder')} aria-label={tl('undeleteFilter', 'Filter by name or folder')} style={{maxWidth: '320px'}} />
                <button className="btn btn-secondary" onClick={() => { const next = new Set(selected); visible.forEach(f => next.add(fileKey(f))); setSelected(next) }}>{tl('selectAllShown', 'Select all shown')}</button>
                <button className="btn btn-secondary" onClick={() => { const next = new Set(selected); visible.forEach(f => next.delete(fileKey(f))); setSelected(next) }}>{tl('selectNoneShown', 'Clear shown')}</button>
              </div>
              <div style={{maxHeight: '420px', overflow: 'auto', border: '1px solid #e2e8f0', borderRadius: '6px', padding: '8px', background: '#fff'}}>
                {tree.map(n => (
                  <TreeNode key={n.key} ui={ui} node={n} depth={0} expanded={expanded} toggleExpand={toggleExpand} selected={selected} setSelected={setSelected} />
                ))}
              </div>
              <div className="form-group" style={{marginTop: '14px'}}>
                <label id="undelete-dest-label">{tl('undeleteRestoreTo', 'Restore to')}</label>
                <RadioGroup name="undelete-dest" labelId="undelete-dest-label" value={dest} onChange={setDest}
                  options={[
                    { value: 'original', label: tl('undeleteToOriginal', 'Where they were') },
                    { value: 'other', label: tl('undeleteToOther', 'Another folder') },
                  ]} />
                {dest === 'other' && (
                  <div style={{display: 'flex', gap: '8px', marginTop: '8px'}}>
                    <input type="text" value={destPath} onChange={(e) => setDestPath(e.target.value)} placeholder="D:\\Recovered" aria-label={tl('undeleteDestFolder', 'Destination folder')} />
                    <button className="btn btn-secondary" onClick={async () => { try { const d = await ui.OpenRestoreDestDialog(); if (d) setDestPath(d) } catch (e) { setError(String(e)) } }}>{t('browse') !== 'browse' ? t('browse') : 'Browse...'}</button>
                  </div>
                )}
              </div>
              <label style={{display: 'flex', alignItems: 'center', gap: '6px', fontWeight: 'normal'}}>
                <input type="checkbox" checked={verify} onChange={(e) => setVerify(e.target.checked)} style={{width: 'auto', margin: 0}} />
                {tl('verifyAfterRestore', 'Verify after restore')}
              </label>
              <div style={{marginTop: '12px'}}>
                <button className="btn" onClick={start} disabled={chosen.length === 0 || restoreLoading}>
                  {tl('undeleteStart', 'Undelete {n} files ({size})').replace('{n}', chosen.length.toLocaleString()).replace('{size}', formatBytesDual(chosenSize))}
                </button>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  )
}

// ---- Roll back --------------------------------------------------------------

function ItemList({ ui, items, total, live }) {
  const { tl, formatBytesDual } = ui
  if (!items || items.length === 0) return null
  return (
    <div style={{maxHeight: '240px', overflow: 'auto', fontFamily: 'Consolas, monospace', fontSize: '12px', background: '#fff', border: '1px solid #e2e8f0', borderRadius: '6px', padding: '6px 8px', marginTop: '6px'}}>
      {items.map((it, i) => (
        <div key={i} style={{whiteSpace: 'nowrap'}}>
          {it.folder}{it.folder.endsWith('\\') || it.folder.endsWith('/') ? '' : '\\'}{it.path.replace(/\//g, '\\')}
          <span style={{color: '#718096'}}> · {formatBytesDual(it.size)}{live ? ` (${tl('rollbackNowSize', 'now {size}').replace('{size}', formatBytesDual(it.liveSize))})` : ''}</span>
        </div>
      ))}
      {total > items.length && <div style={{color: '#718096'}}>{tl('rollbackMore', '... and {n} more').replace('{n}', (total - items.length).toLocaleString())}</div>}
    </div>
  )
}

export function RollbackPage({ ui, jobs, jobId, setJobId, restoreLoading, onStarted, restoreCard }) {
  const { t, tl, formatBytesDual, WaitBar } = ui
  const [snaps, setSnaps] = useState(null)
  const [loadingSnaps, setLoadingSnaps] = useState(null)
  const [snapUnix, setSnapUnix] = useState(0)
  const [atTime, setAtTime] = useState('')
  const [preview, setPreview] = useState(null)
  const [previewing, setPreviewing] = useState(null)
  const [policy, setPolicy] = useState('replace')
  const [backupFirst, setBackupFirst] = useState(true)
  const [verify, setVerify] = useState(true)
  const [confirmText, setConfirmText] = useState('')
  const [error, setError] = useState('')
  const [records, setRecords] = useState([])
  const [busyRecord, setBusyRecord] = useState('')
  const [progressLine, setProgressLine, progressFraction] = useScanProgress(ui, !!previewing)
  const [, tick] = useState(0)

  useEffect(() => {
    if (!loadingSnaps && !previewing) return undefined
    const id = setInterval(() => tick(n => n + 1), 1000)
    return () => clearInterval(id)
  }, [loadingSnaps, previewing])

  const loadRecords = async (id) => {
    if (!id) { setRecords([]); return }
    try { setRecords(await ui.ListRollbacks(id) || []) } catch (e) { setRecords([]) }
  }
  useEffect(() => {
    setSnaps(null); setSnapUnix(0); setPreview(null); setError(''); setAtTime(''); setConfirmText('')
    loadRecords(jobId)
    if (!jobId) return
    let cancelled = false
    setLoadingSnaps(Date.now())
    ui.ListSetSnapshots(jobId)
      .then(s => { if (!cancelled) setSnaps(s || []) })
      .catch(e => { if (!cancelled) setError(String(e)) })
      .finally(() => { if (!cancelled) setLoadingSnaps(null) })
    return () => { cancelled = true }
  }, [jobId])
  useEffect(() => {
    if (!ui.EventsOn) return undefined
    const off = ui.EventsOn('restore:complete', (d) => {
      if (d && d.kind === 'rollback') { setPreview(null); setConfirmText(''); loadRecords(jobIdRef.current) }
    })
    return () => { if (typeof off === 'function') off() }
  }, [])
  const jobIdRef = useRef(jobId)
  useEffect(() => { jobIdRef.current = jobId }, [jobId])
  useEffect(() => { setPreview(null); setConfirmText('') }, [snapUnix])

  const pickAt = (value) => {
    setAtTime(value)
    if (!value || !snaps) return
    const want = Math.floor(new Date(value).getTime() / 1000)
    const s = snaps.find(x => x.unix <= want)
    setSnapUnix(s ? s.unix : 0)
    if (!s) setError(tl('rollbackNoneBefore', 'There is no backup at or before that time.'))
    else setError('')
  }

  const runPreview = async () => {
    setError(''); setPreview(null); setProgressLine(''); setConfirmText('')
    setPreviewing(Date.now())
    try {
      setPreview(await ui.PreviewRollback(jobId, snapUnix))
    } catch (e) {
      setError(String(e))
    } finally {
      setPreviewing(null)
    }
  }

  const willRestore = preview ? preview.missingCount + (policy === 'missing' ? 0 : preview.changedCount) : 0
  const willRemove = preview && policy === 'exact' ? preview.newCount : 0
  const exactOk = policy !== 'exact' || String(preview?.newCount ?? '') === confirmText.trim()
  const nothingToDo = preview && willRestore === 0 && willRemove === 0

  const start = async () => {
    const msg = tl('rollbackConfirm', 'Roll back {set} to {when}? {restore} files will be restored or replaced and {remove} removed. Everything replaced or removed is kept first, so Undo can put it back.')
      .replace('{set}', (jobs.find(j => j.id === jobId) || {}).name || '')
      .replace('{when}', fmtDate(snapUnix)).replace('{restore}', willRestore.toLocaleString()).replace('{remove}', willRemove.toLocaleString())
    // eslint-disable-next-line no-alert
    if (!window.confirm(msg)) return
    setError('')
    try {
      onStarted()
      await ui.StartRollback(jobId, snapUnix, policy, backupFirst, policy === 'exact' ? preview.newCount : 0, verify)
    } catch (e) {
      ui.onStartFailed(String(e))
    }
  }

  const undo = async (r) => {
    // eslint-disable-next-line no-alert
    if (!window.confirm(tl('rollbackUndoConfirm', 'Undo this roll back? Files it restored are deleted and the versions it kept go back.'))) return
    setBusyRecord(r.path)
    try { ui.showStatus('✅ ' + await ui.UndoRollback(r.path), 'success') } catch (e) { ui.showStatus('❌ ' + e, 'error') }
    setBusyRecord('')
    loadRecords(jobId)
  }
  const del = async (r) => {
    // eslint-disable-next-line no-alert
    if (!window.confirm(tl('rollbackDeleteConfirm', 'Delete the versions this roll back kept ({size})? It can no longer be undone.').replace('{size}', formatBytesDual(r.bytes)))) return
    setBusyRecord(r.path)
    try { await ui.DeleteRollbackCopies(r.path) } catch (e) { ui.showStatus('❌ ' + e, 'error') }
    setBusyRecord('')
    loadRecords(jobId)
  }
  const policyName = (p) => ({
    replace: tl('rollbackPolicyReplaceShort', 'missing and changed'),
    missing: tl('rollbackPolicyMissingShort', 'missing only'),
    exact: tl('rollbackPolicyExactShort', 'exact'),
  }[p] || p)

  return (
    <div className="card">
      <h2 style={{marginTop: 0}}>{tl('rollbackTitle', 'Roll back')}</h2>
      <p style={{marginTop: 0, color: '#4a5568', fontSize: '14px'}}>
        {tl('rollbackIntro', 'Puts a Backup Set\'s folders back as they were at a chosen backup. You see what would change first, and everything it replaces or removes is kept so the roll back can be undone.')}
      </p>
      {restoreCard}
      <SetPicker ui={{...ui, idSuffix: 'rollback'}} jobs={jobs} value={jobId} onChange={setJobId} />
      {loadingSnaps && <WaitBar label={tl('rollbackListing', 'Listing the set\'s backups...')} startedAt={loadingSnaps} slowHint={t('listSlowHint') !== 'listSlowHint' ? t('listSlowHint') : ''} />}
      {error && <div className="info-box" style={{borderColor: '#e53e3e', color: '#c53030', marginTop: '12px'}}>❌ {error}</div>}

      {snaps && (
        <div className="form-group">
          <label id="rollback-snap-label">{tl('rollbackPickBackup', 'Roll back to')}</label>
          {snaps.length === 0 ? (
            <p style={{fontSize: '13px', color: '#718096'}}>{tl('rollbackNoBackups', 'This set has no backups on its server yet.')}</p>
          ) : (
            <>
              <div style={{display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap', marginBottom: '8px'}}>
                <label htmlFor="rollback-at" style={{margin: 0, fontWeight: 'normal'}}>{tl('rollbackAtTime', 'The backup at or before:')}</label>
                <input id="rollback-at" type="datetime-local" value={atTime} onChange={(e) => pickAt(e.target.value)} style={{maxWidth: '230px'}} />
              </div>
              <div role="radiogroup" aria-labelledby="rollback-snap-label" style={{maxHeight: '260px', overflow: 'auto', border: '1px solid #e2e8f0', borderRadius: '6px', background: '#fff'}}>
                {snaps.map((s, i) => (
                  <label key={s.unix} style={{display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 10px', margin: 0, fontWeight: 'normal', cursor: 'pointer', borderTop: i === 0 ? 'none' : '1px solid #edf2f7', background: snapUnix === s.unix ? '#eff6ff' : undefined}}>
                    <input type="radio" name="rollback-snap" checked={snapUnix === s.unix} onChange={() => { setSnapUnix(s.unix); setAtTime('') }} style={{width: 'auto', margin: 0}} />
                    <span>{fmtDate(s.unix)}</span>
                    <span style={{fontSize: '12px', color: '#718096'}}>{formatBytesDual(s.size)}{i === 0 ? ' · ' + tl('rollbackNewest', 'newest') : ''}{s.protected ? ' · 🔒' : ''}</span>
                  </label>
                ))}
              </div>
            </>
          )}
        </div>
      )}

      {snapUnix > 0 && (
        <div style={{display: 'flex', gap: '10px', flexWrap: 'wrap'}}>
          <button className="btn" onClick={runPreview} disabled={!!previewing || restoreLoading}>{tl('rollbackPreview', 'Show what would change')}</button>
          {previewing && <button className="btn btn-secondary" onClick={() => ui.CancelSetScan()}>{t('cancel') !== 'cancel' ? t('cancel') : 'Cancel'}</button>}
        </div>
      )}
      {previewing && <WaitBar label={progressLine || tl('rollbackComparing', 'Comparing the backup with the folders...')} startedAt={previewing} fraction={progressFraction}
        slowHint={tl('undeleteSlowHint', 'A large set takes a few minutes: every folder is listed, and each snapshot\'s file list is read once and kept on this computer.')} />}

      {preview && (
        <div style={{marginTop: '16px'}}>
          <h3 style={{marginBottom: '8px'}}>{tl('rollbackPreviewTitle', 'Compared with the backup of {when}').replace('{when}', fmtDate(preview.snapshotUnix))}</h3>
          <div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(210px, 1fr))', gap: '10px'}}>
            <div className="info-box" style={{margin: 0}}>
              <strong>{tl('rollbackMissing', 'Missing now')}</strong><br />
              {tl('rollbackFilesSize', '{n} files, {size}').replace('{n}', preview.missingCount.toLocaleString()).replace('{size}', formatBytesDual(preview.missingBytes))}
              <div style={{fontSize: '12px', color: '#4a5568'}}>{tl('rollbackMissingHint', 'In the backup, gone from disk: restored')}</div>
            </div>
            <div className="info-box" style={{margin: 0}}>
              <strong>{tl('rollbackChanged', 'Changed since')}</strong><br />
              {tl('rollbackFilesSize', '{n} files, {size}').replace('{n}', preview.changedCount.toLocaleString()).replace('{size}', formatBytesDual(preview.changedBytes))}
              <div style={{fontSize: '12px', color: '#4a5568'}}>{tl('rollbackChangedHint', 'Different size or time: replaced by the backup\'s version')}</div>
            </div>
            <div className="info-box" style={{margin: 0}}>
              <strong>{tl('rollbackAdded', 'Added since')}</strong><br />
              {tl('rollbackFilesSize', '{n} files, {size}').replace('{n}', preview.newCount.toLocaleString()).replace('{size}', formatBytesDual(preview.newBytes))}
              {preview.newDirCount > 0 ? tl('rollbackPlusFolders', ', {n} folders').replace('{n}', preview.newDirCount.toLocaleString()) : ''}
              <div style={{fontSize: '12px', color: '#4a5568'}}>{tl('rollbackAddedHint', 'Not in the backup: kept, unless you choose exact')}</div>
            </div>
            <div className="info-box" style={{margin: 0}}>
              <strong>{tl('rollbackUnchanged', 'Unchanged')}</strong><br />
              {tl('rollbackFiles', '{n} files').replace('{n}', preview.unchanged.toLocaleString())}
              <div style={{fontSize: '12px', color: '#4a5568'}}>{tl('rollbackUnchangedHint', 'Left as they are')}</div>
            </div>
          </div>
          {preview.unknown > 0 && <div className="info-box" style={{marginTop: '10px'}}>⚠️ {tl('rollbackUnknown', '{n} files are under folders that could not be read now; they are left alone.').replace('{n}', preview.unknown.toLocaleString())}</div>}
          <details style={{marginTop: '10px'}}><summary>{tl('rollbackShowMissing', 'Missing files')} ({preview.missingCount.toLocaleString()})</summary><ItemList ui={ui} items={preview.missing} total={preview.missingCount} /></details>
          <details><summary>{tl('rollbackShowChanged', 'Changed files')} ({preview.changedCount.toLocaleString()})</summary><ItemList ui={ui} items={preview.changed} total={preview.changedCount} live /></details>
          <details><summary>{tl('rollbackShowAdded', 'Files added since')} ({preview.newCount.toLocaleString()})</summary><ItemList ui={ui} items={preview.new} total={preview.newCount} /></details>

          <div className="form-group" style={{marginTop: '14px'}}>
            <label id="rollback-policy-label">{tl('rollbackWhat', 'What to do')}</label>
            <RadioGroup name="rollback-policy" labelId="rollback-policy-label" value={policy} onChange={(v) => { setPolicy(v); setConfirmText('') }} vertical
              options={[
                { value: 'replace', label: tl('rollbackPolicyReplace', 'Restore missing files and replace changed ones; keep files added since') },
                { value: 'missing', label: tl('rollbackPolicyMissing', 'Restore missing files only') },
                { value: 'exact', label: tl('rollbackPolicyExact', 'Exact roll back: also remove files added since') },
              ]} />
          </div>
          {policy === 'exact' && preview.newCount > 0 && (
            <div className="form-group">
              <label htmlFor="rollback-confirm">{tl('rollbackTypeCount', 'Type {n} to confirm that {n} files added since will be removed (kept for Undo)').replace(/\{n\}/g, preview.newCount.toLocaleString())}</label>
              <input id="rollback-confirm" type="text" inputMode="numeric" value={confirmText} onChange={(e) => setConfirmText(e.target.value)} style={{maxWidth: '160px'}} />
            </div>
          )}
          <label style={{display: 'flex', alignItems: 'center', gap: '6px', fontWeight: 'normal'}}>
            <input type="checkbox" checked={backupFirst} onChange={(e) => setBackupFirst(e.target.checked)} style={{width: 'auto', margin: 0}} />
            {tl('rollbackBackupFirst', 'Back up the set first (keeps today\'s state as a backup too)')}
          </label>
          <label style={{display: 'flex', alignItems: 'center', gap: '6px', fontWeight: 'normal'}}>
            <input type="checkbox" checked={verify} onChange={(e) => setVerify(e.target.checked)} style={{width: 'auto', margin: 0}} />
            {tl('verifyAfterRestore', 'Verify after restore')}
          </label>
          <div className="info-box" style={{marginTop: '10px'}}>
            🛟 {tl('rollbackSafetyNote', 'Before anything is replaced or removed it is moved to {folder} on the same disk, so Undo below can put it back. While the roll back runs, this set\'s scheduled backups wait.').replace('{folder}', (preview.safetyFolders || []).join(', '))}
          </div>
          <div style={{marginTop: '12px'}}>
            <button className="btn" onClick={start} disabled={restoreLoading || nothingToDo || (policy === 'exact' && preview.newCount > 0 && !exactOk)}>
              {nothingToDo ? tl('rollbackNothing', 'Nothing to roll back') : tl('rollbackStart', 'Roll back to {when}').replace('{when}', fmtDate(preview.snapshotUnix))}
            </button>
          </div>
        </div>
      )}

      {records.length > 0 && (
        <div style={{marginTop: '24px'}}>
          <h3 style={{marginBottom: '8px'}}>{tl('rollbackEarlier', 'Earlier roll backs of this set')}</h3>
          <div style={{border: '1px solid #e2e8f0', borderRadius: '6px', background: '#fff'}}>
            {records.map((r, i) => (
              <div key={r.path} style={{display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap', padding: '8px 10px', borderTop: i === 0 ? 'none' : '1px solid #edf2f7'}}>
                <div style={{flex: '1 1 300px', fontSize: '13px'}}>
                  <strong>{new Date(r.created).toLocaleString()}</strong>{' '}
                  {tl('rollbackRecordTo', 'to the backup of {when}').replace('{when}', r.snapshotId ? new Date(r.snapshotId).toLocaleString() : '?')}
                  {' · '}{policyName(r.policy)}{' · '}{r.status}
                  <div style={{color: '#718096'}}>
                    {tl('rollbackRecordCounts', 'restored {a}, replaced {b}, removed {c}; kept {size}')
                      .replace('{a}', Math.max(0, r.restored - r.moved)).replace('{b}', r.moved).replace('{c}', r.removed).replace('{size}', formatBytesDual(r.bytes))}
                  </div>
                </div>
                {r.status !== 'undone' && (
                  <button className="btn btn-secondary" onClick={() => undo(r)} disabled={!!busyRecord || restoreLoading}>{tl('rollbackUndo', 'Undo')}</button>
                )}
                <button className="btn btn-secondary" onClick={() => del(r)} disabled={!!busyRecord || restoreLoading}>{tl('rollbackDeleteKept', 'Delete kept versions')}</button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

// ---- Run as Service (Preferences > Advanced) -------------------------------

export function ServiceControl({ ui }) {
  const { tl } = ui
  const [st, setSt] = useState(null)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const refresh = async () => { try { setSt(await ui.GetServiceStatus()) } catch (e) { setError(String(e)) } }
  useEffect(() => {
    refresh()
    const id = setInterval(refresh, 5000)
    return () => clearInterval(id)
  }, [])
  if (!st) return null
  const act = async (name, fn) => {
    setBusy(name); setError('')
    try { setSt(await fn()) } catch (e) { setError(String(e)); refresh() }
    setBusy('')
  }
  const stateText = {
    running: tl('serviceRunning', 'running'), stopped: tl('serviceStopped', 'stopped'),
    starting: tl('serviceStarting', 'starting'), stopping: tl('serviceStopping', 'stopping'),
    paused: tl('servicePaused', 'paused'),
  }[st.state] || st.state
  return (
    <div className="form-group" style={{marginTop: '24px'}}>
      <h3 style={{marginBottom: '8px'}}>{tl('serviceTitle', 'Run as Service')}</h3>
      {!st.supported ? (
        <p style={{fontSize: '13px', color: '#4a5568'}}>{tl('serviceLinux', 'The background service is for Windows. On Linux, scheduled Backup Sets run while this app is open.')}</p>
      ) : (
        <>
          <p style={{fontSize: '13px', color: '#4a5568', marginTop: 0}}>
            {tl('serviceIntro', 'With the service installed, scheduled Backup Sets run in the background as Local System, also when nobody is signed in and this app is closed. Without it, they run only while this app is open.')}
          </p>
          <div style={{fontSize: '14px', marginBottom: '8px'}}>
            <strong>{tl('serviceStatusLabel', 'Status:')}</strong>{' '}
            {st.installed
              ? <>{tl('serviceInstalled', 'Installed')} ({st.name}), {stateText}{st.startType ? `, ${tl('serviceStart_' + st.startType, st.startType === 'automatic' ? 'starts with Windows' : st.startType)}` : ''}</>
              : tl('serviceNotInstalled', 'Not installed')}
          </div>
          {st.installed && (
            <div style={{fontSize: '13px', color: '#4a5568', marginBottom: '8px'}}>
              {st.serviceMode
                ? '✅ ' + tl('serviceInUse', 'This app hands backups and schedules to the service.')
                : tl('serviceNotInUse', 'This app is running the schedules itself.')}
            </div>
          )}
          {st.installed && st.otherExe && (
            <div className="info-box" style={{marginBottom: '8px'}}>⚠️ {tl('serviceOtherExe', 'The installed service runs {path}, not the copy beside this app.').replace('{path}', st.exePath)}</div>
          )}
          {!st.installed && !st.exeAvailable && (
            <div className="info-box" style={{marginBottom: '8px'}}>ℹ️ {tl('serviceNoExe', '{file} is not in this app\'s folder. It comes in the download; put it beside the app to install the service.').replace('{file}', st.expectedExe)}</div>
          )}
          {!st.isAdmin && (
            <div style={{display: 'flex', gap: '10px', alignItems: 'center', marginBottom: '8px', flexWrap: 'wrap'}}>
              <span>🔒 {tl('serviceNeedsAdmin', 'Installing, starting, stopping and removing the service need an administrator.')}</span>
              {ui.restartAsAdmin && <button className="btn btn-secondary" onClick={ui.restartAsAdmin}>{tl('jobsPolicyElevateBtn', 'Restart as administrator')}</button>}
            </div>
          )}
          <div style={{display: 'flex', gap: '10px', flexWrap: 'wrap'}}>
            {!st.installed && (
              <button className="btn" disabled={!st.isAdmin || !st.exeAvailable || !!busy} onClick={() => act('install', ui.InstallService)}>
                {busy === 'install' ? '…' : tl('serviceInstall', 'Install and start')}
              </button>
            )}
            {st.installed && st.state !== 'running' && (
              <button className="btn" disabled={!st.isAdmin || !!busy} onClick={() => act('start', ui.StartBackgroundService)}>{busy === 'start' ? '…' : tl('serviceStart', 'Start')}</button>
            )}
            {st.installed && st.state === 'running' && (
              <button className="btn btn-secondary" disabled={!st.isAdmin || !!busy} onClick={() => act('stop', ui.StopBackgroundService)}>{busy === 'stop' ? '…' : tl('serviceStop', 'Stop')}</button>
            )}
            {st.installed && (
              <button className="btn btn-secondary" disabled={!st.isAdmin || !!busy} onClick={() => {
                // eslint-disable-next-line no-alert
                if (window.confirm(tl('serviceRemoveConfirm', 'Stop and remove the background service? Backup Sets, history and settings stay; this app runs the schedules again while it is open.'))) act('remove', ui.RemoveService)
              }}>{busy === 'remove' ? '…' : tl('serviceRemove', 'Remove')}</button>
            )}
          </div>
          {error && <div className="info-box" style={{borderColor: '#e53e3e', color: '#c53030', marginTop: '10px'}}>❌ {error}</div>}
        </>
      )}
    </div>
  )
}
