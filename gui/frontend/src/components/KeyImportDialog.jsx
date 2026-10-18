import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from '../i18n/i18nContext'

// KeyImportDialog brings back an encryption key kept away from this computer:
// a copy of the key file, or a paper key read from its QR code (with the
// camera, or from a scan or photo) or typed in from its text. The backend
// reads and checks the key (key_import.go); the dialog shows its fingerprint
// to compare with the paper, asks for the passphrase of a protected key, and
// saves it into the keys folder (SaveImportedKey), never over a file.
//
// The camera view sends a frame to the backend about four times a second
// until one holds the key's QR code. On Linux the app's web view only allows
// the camera after PrepareCamera, and only until CameraClosed.
//
// Like ConfirmDialog, Tab stays inside the dialog and Escape closes it;
// focus goes back to what had it (the Import... button) on close.

function api(name) {
  if (window.go && window.go.main && window.go.main.App) return window.go.main.App[name]
  return undefined
}

const tile = {
  display: 'flex', gap: '12px', alignItems: 'flex-start', width: '100%', textAlign: 'left',
  margin: 0, padding: '12px 14px', border: '1px solid #cbd5e1', borderRadius: '6px',
  background: '#fff', color: '#2d3748', cursor: 'pointer', fontSize: '14px', lineHeight: 1.4,
}

export default function KeyImportDialog({ onClose, onImported }) {
  const { t } = useTranslation()
  const [step, setStep] = useState('choose') // choose | camera | text | confirm
  const [busy, setBusy] = useState(false) // false, true, or what is going on (shown)
  const [msg, setMsg] = useState(null) // { ok, text }
  const [key, setKey] = useState(null) // KeyImport from the backend
  const [source, setSource] = useState('')
  const [text, setText] = useState('')
  const [pass, setPass] = useState('')
  const [cameras, setCameras] = useState(null) // null: still looking
  const firstRef = useRef(null)
  const boxRef = useRef(null)
  const busyRef = useRef(false)

  // The camera: on Linux the web view must allow it first; then count the
  // cameras so the choice can say there is none. Closed with the dialog,
  // once PrepareCamera has answered, so the close cannot overtake it.
  useEffect(() => {
    let live = true
    const prepare = api('PrepareCamera')
    const ready = Promise.resolve().then(() => (prepare ? prepare() : 0)).catch(() => 0)
    ready
      .then(() => (navigator.mediaDevices && navigator.mediaDevices.enumerateDevices ? navigator.mediaDevices.enumerateDevices() : []))
      .then((devs) => { if (live) setCameras(devs.filter((d) => d.kind === 'videoinput').length) })
      .catch(() => { if (live) setCameras(0) })
    return () => {
      live = false
      const closed = api('CameraClosed')
      if (closed) ready.then((id) => closed(id)).catch(() => {})
    }
  }, [])

  // Focus goes back where it was when the dialog closes.
  useEffect(() => {
    const was = document.activeElement
    return () => { if (was && document.contains(was)) was.focus() }
  }, [])

  // The first choice has the focus on the choose step. Elsewhere, when the
  // control that had it has gone (the camera tile, the passphrase box after
  // Check), the dialog takes it, so Escape and Tab keep working.
  useEffect(() => { if (step === 'choose' && firstRef.current) firstRef.current.focus() }, [step])
  useEffect(() => {
    const box = boxRef.current
    if (box && !box.contains(document.activeElement)) box.focus()
  })

  const onKeyDown = (e) => {
    if (e.key === 'Escape') {
      e.preventDefault()
      if (!busy) onClose()
      return
    }
    if (e.key !== 'Tab' || !boxRef.current) return
    const items = [...boxRef.current.querySelectorAll('button:not([disabled]), input:not([disabled]), textarea:not([disabled])')]
    if (items.length === 0) { e.preventDefault(); return }
    const i = items.indexOf(document.activeElement)
    const next = e.shiftKey ? (i <= 0 ? items.length - 1 : i - 1) : (i === items.length - 1 ? 0 : i + 1)
    e.preventDefault()
    items[next].focus()
  }

  const problemText = useCallback((ki) => {
    const say = {
      no_qr: t('keyImportNoQR'),
      not_key: t('keyImportNotKey'),
      damaged: t('keyImportDamaged'),
      unsupported_image: t('keyImportBadPicture'),
      wrong_passphrase: t('keyImportWrongPassphrase'),
    }[ki.problem] || ki.problem
    // The technical reason helps with a picture or text that is not a key;
    // for a damaged key it would only show a second, wrong fingerprint.
    return ki.detail && ki.problem !== 'damaged' ? `${say} (${ki.detail})` : say
  }, [t])

  // found takes what the backend read: a key goes to the confirm step, a
  // problem is shown where the user is.
  const found = (ki, from) => {
    if (!ki || (!ki.key_text && !ki.problem)) return false // cancelled, or nothing yet
    if (ki.problem) {
      setMsg({ ok: false, text: problemText(ki) })
      return false
    }
    setKey(ki)
    setSource(from)
    setPass('')
    setMsg(null)
    setStep('confirm')
    return true
  }

  // run guards a backend call against a second click and shows what failed;
  // label, when given, says what is going on meanwhile.
  const run = async (label, fn) => {
    if (busyRef.current) return
    busyRef.current = true
    setBusy(label || true)
    setMsg(null)
    try {
      await fn()
    } catch (err) {
      setMsg({ ok: false, text: t('keyImportFailed').replace('{err}', () => String(err)) })
    } finally {
      busyRef.current = false
      setBusy(false)
    }
  }

  const fromFile = () => run(null, async () => { found(await api('ReadImportKeyFile')(), 'file') })
  // The picture is chosen first, so "Reading the picture" is only shown
  // once there is one: a large photo takes a second or two.
  const fromPicture = () => run(null, async () => {
    const path = await api('ChooseImportKeyPicture')()
    if (!path) return
    setBusy(t('keyImportReading'))
    found(await api('ReadImportKeyPicture')(path), 'picture')
  })
  const fromText = () => run(null, async () => { found(await api('ReadImportKeyText')(text), 'text') })

  const verify = () => run(null, async () => {
    const ki = await api('VerifyImportedKey')(key.key_text, pass)
    if (ki.problem) {
      setMsg({ ok: false, text: problemText({ ...ki, detail: '' }) })
      return
    }
    setKey(ki)
    setMsg({ ok: true, text: t('keyImportUnlocked') })
  })

  const save = () => run(null, async () => {
    const result = await api('SaveImportedKey')(key.key_text, key.passphrase_protected ? pass : '')
    onImported(result)
  })

  const back = () => { setStep('choose'); setKey(null); setMsg(null); setPass('') }
  const canSave = key && (!key.passphrase_protected || key.checked)

  return (
    <div
      style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1250, padding: '16px' }}
      onMouseDown={(e) => { if (e.target === e.currentTarget && !busy) onClose() }}
      onKeyDown={onKeyDown}
    >
      <div ref={boxRef} tabIndex={-1} role="dialog" aria-modal="true" aria-labelledby="key-import-title"
        style={{ maxWidth: '640px', width: '100%', background: '#fff', borderRadius: '8px', overflow: 'hidden', boxShadow: '0 12px 44px rgba(0,0,0,.35)', outline: 'none' }}>
        <div id="key-import-title" style={{ background: 'var(--accent)', color: '#fff', padding: '12px 18px', fontSize: '16px', fontWeight: 600, display: 'flex', gap: '10px' }}>
          <span aria-hidden="true">🔑</span><span>{t('keyImportTitle')}</span>
        </div>
        <div style={{ padding: '18px', color: '#2d3748', fontSize: '14px', lineHeight: 1.5, maxHeight: '70vh', overflowY: 'auto' }}>
          {step === 'choose' && (
            <>
              <p style={{ margin: '0 0 12px' }}>{t('keyImportIntro')}</p>
              <div style={{ display: 'grid', gap: '8px' }}>
                <button ref={firstRef} type="button" style={tile} onClick={fromFile} disabled={!!busy}>
                  <span aria-hidden="true" style={{ fontSize: '22px' }}>💾</span>
                  <span><strong>{t('keyImportFile')}</strong><br /><span style={{ color: '#64748b', fontSize: '13px' }}>{t('keyImportFileDesc')}</span></span>
                </button>
                <button type="button" style={{ ...tile, ...(cameras === 0 ? { opacity: 0.55, cursor: 'default' } : {}) }}
                  onClick={() => { setMsg(null); setStep('camera') }} disabled={!!busy || !cameras}>
                  <span aria-hidden="true" style={{ fontSize: '22px' }}>📷</span>
                  <span><strong>{t('keyImportCamera')}</strong><br /><span style={{ color: '#64748b', fontSize: '13px' }}>{cameras === 0 ? t('keyImportNoCamera') : t('keyImportCameraDesc')}</span></span>
                </button>
                <button type="button" style={tile} onClick={fromPicture} disabled={!!busy}>
                  <span aria-hidden="true" style={{ fontSize: '22px' }}>🖼️</span>
                  <span><strong>{t('keyImportPicture')}</strong><br /><span style={{ color: '#64748b', fontSize: '13px' }}>{t('keyImportPictureDesc')}</span></span>
                </button>
                <button type="button" style={tile} onClick={() => { setMsg(null); setStep('text') }} disabled={!!busy}>
                  <span aria-hidden="true" style={{ fontSize: '22px' }}>⌨️</span>
                  <span><strong>{t('keyImportText')}</strong><br /><span style={{ color: '#64748b', fontSize: '13px' }}>{t('keyImportTextDesc')}</span></span>
                </button>
              </div>
            </>
          )}

          {step === 'camera' && (
            <CameraView t={t} onKey={(ki) => found(ki, 'camera')} problemText={problemText} />
          )}

          {step === 'text' && (
            <>
              <label htmlFor="key-import-text" style={{ display: 'block', fontWeight: 600, marginBottom: '6px' }}>{t('keyImportTextLabel')}</label>
              <textarea
                id="key-import-text"
                value={text}
                onChange={(e) => { setText(e.target.value); setMsg(null) }}
                rows={9}
                spellCheck={false}
                autoFocus
                placeholder={'{\n  "data": "...",\n  "fingerprint": "..."\n}'}
                style={{ width: '100%', boxSizing: 'border-box', fontFamily: 'Consolas, monospace', fontSize: '12px' }}
              />
            </>
          )}

          {step === 'confirm' && key && (
            <>
              <p style={{ margin: '0 0 10px' }}>{t('keyImportFound')}</p>
              <dl style={{ display: 'grid', gridTemplateColumns: 'max-content 1fr', gap: '6px 12px', margin: '0 0 12px', fontSize: '13px' }}>
                <dt style={{ color: '#64748b' }}>{t('paperKeyFingerprint')}</dt>
                <dd style={{ margin: 0, fontFamily: 'Consolas, monospace', fontWeight: 700, fontSize: '15px', wordBreak: 'break-all' }}>{key.fingerprint || '?'}</dd>
                {key.created && (<><dt style={{ color: '#64748b' }}>{t('paperKeyCreated')}</dt><dd style={{ margin: 0 }}>{key.created}</dd></>)}
                {key.hint && (<><dt style={{ color: '#64748b' }}>{t('keyImportHint')}</dt><dd style={{ margin: 0 }}>{key.hint}</dd></>)}
                {key.source_path && (<><dt style={{ color: '#64748b' }}>{t(source === 'picture' ? 'keyImportPicture' : 'paperKeyFile')}</dt><dd style={{ margin: 0, wordBreak: 'break-all' }}>{key.source_path}</dd></>)}
              </dl>
              {!key.passphrase_protected && (
                <p style={{ margin: '0 0 8px', fontWeight: 600, color: key.checked ? '#1a7f37' : '#a06000' }}>
                  {key.checked ? `✓ ${t('keyImportIntact')}` : t('keyImportUnchecked')}
                </p>
              )}
              {key.passphrase_protected && !key.checked && (
                <>
                  <p id="key-import-needs-pass" style={{ margin: '0 0 6px' }}>{t('keyImportNeedsPassphrase')}</p>
                  <div style={{ display: 'flex', gap: '8px' }}>
                    <input type="password" value={pass} onChange={(e) => { setPass(e.target.value); setMsg(null) }}
                      aria-label={t('keyImportPassphrase')} aria-describedby="key-import-needs-pass"
                      onKeyDown={(e) => { if (e.key === 'Enter' && pass) { e.preventDefault(); verify() } }}
                      placeholder={t('keyImportPassphrase')} autoComplete="off" spellCheck={false} autoFocus style={{ flex: '1 1 auto' }} />
                    <button type="button" className="btn btn-secondary" style={{ margin: 0 }} onClick={verify} disabled={!pass || !!busy}>{t('keyImportCheck')}</button>
                  </div>
                </>
              )}
              {source === 'picture' && !key.passphrase_protected && (
                <p style={{ margin: '10px 0 0', fontSize: '13px', background: '#fff4d6', border: '1px solid #e0b84c', padding: '8px 10px', borderRadius: '4px' }}>
                  {t('keyImportPhotoWarning')}
                </p>
              )}
            </>
          )}

          {typeof busy === 'string' && <p role="status" style={{ margin: '12px 0 0', fontSize: '13px', color: '#64748b' }}>{busy}…</p>}
          {msg && (
            <p role={msg.ok ? 'status' : 'alert'} style={{ margin: '12px 0 0', fontSize: '13px', fontWeight: 600, wordBreak: 'break-word', color: msg.ok ? '#1a7f37' : '#c62828', background: msg.ok ? '#eef8f0' : '#fdecea', padding: '8px 10px', borderRadius: '4px' }}>
              {msg.text}
            </p>
          )}
        </div>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', padding: '0 18px 18px', flexWrap: 'wrap' }}>
          {step !== 'choose' && (
            <button type="button" className="btn btn-secondary" style={{ margin: 0, marginRight: 'auto' }} onClick={back} disabled={!!busy}>{t('keyImportBack')}</button>
          )}
          <button type="button" className="btn btn-secondary" style={{ margin: 0 }} onClick={onClose} disabled={!!busy}>{t('keyImportCancel')}</button>
          {step === 'text' && (
            <button type="button" className="btn" style={{ margin: 0 }} onClick={fromText} disabled={!text.trim() || !!busy}>{t('keyImportCheck')}</button>
          )}
          {step === 'confirm' && (
            <button type="button" className="btn" style={{ margin: 0 }} onClick={save} disabled={!canSave || !!busy}>{t('keyImportSave')}</button>
          )}
        </div>
      </div>
    </div>
  )
}

// CameraView shows the camera with a square to hold the QR code in, and
// sends frames to the backend until one holds a key (onKey). A QR code that
// is not a key is said under the picture, and the camera keeps looking.
function CameraView({ t, onKey, problemText }) {
  const videoRef = useRef(null)
  const [state, setState] = useState('starting') // starting | live | failed
  const [note, setNote] = useState('')

  useEffect(() => {
    let live = true
    let stream = null
    let timer = null
    const canvas = document.createElement('canvas')
    const read = api('ReadImportKeyCameraFrame')

    const stop = () => {
      live = false
      if (timer) clearTimeout(timer)
      if (stream) stream.getTracks().forEach((tr) => tr.stop())
    }

    const tick = async () => {
      if (!live) return
      const v = videoRef.current
      if (v && v.videoWidth && read) {
        canvas.width = v.videoWidth
        canvas.height = v.videoHeight
        canvas.getContext('2d').drawImage(v, 0, 0)
        try {
          const ki = await read(canvas.toDataURL('image/jpeg', 0.92))
          if (!live) return
          if (ki && ki.key_text) {
            stop()
            onKey(ki)
            return
          }
          // A frame without a QR code leaves the note alone: the code comes
          // and goes as the page moves, and the note would blink.
          if (ki && ki.problem) setNote(problemText(ki))
        } catch (err) {
          if (live) setNote(String(err))
        }
      }
      if (live) timer = setTimeout(tick, 250)
    }

    ;(async () => {
      try {
        if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) throw Object.assign(new Error('no media devices'), { name: 'NotFoundError' })
        stream = await navigator.mediaDevices.getUserMedia({ video: { width: { ideal: 1280 }, height: { ideal: 720 } }, audio: false })
        if (!live) { stream.getTracks().forEach((tr) => tr.stop()); return }
        const v = videoRef.current
        v.srcObject = stream
        await v.play()
        setState('live')
        timer = setTimeout(tick, 300)
      } catch (err) {
        if (!live) return
        setState('failed')
        const name = err && err.name
        if (name === 'NotAllowedError' || name === 'SecurityError') setNote(t('keyImportCameraDenied'))
        else if (name === 'NotFoundError' || name === 'OverconstrainedError') setNote(t('keyImportNoCamera'))
        else setNote(t('keyImportCameraFailed').replace('{err}', () => String((err && err.message) || err)))
      }
    })()
    return stop
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <>
      <p style={{ margin: '0 0 10px' }}>{t('keyImportCameraHint')}</p>
      <div style={{ position: 'relative', background: '#111', borderRadius: '6px', overflow: 'hidden', aspectRatio: '16 / 9', display: state === 'failed' ? 'none' : 'block' }}>
        {/* Mirrored, like a mirror: moving the page left moves it left. */}
        <video ref={videoRef} muted playsInline aria-label={t('keyImportCamera')}
          style={{ width: '100%', height: '100%', objectFit: 'contain', transform: 'scaleX(-1)', display: 'block' }} />
        <div aria-hidden="true" style={{ position: 'absolute', top: '50%', left: '50%', height: '70%', aspectRatio: '1 / 1', transform: 'translate(-50%, -50%)', border: '3px solid rgba(255,255,255,.85)', borderRadius: '8px', boxShadow: '0 0 0 2000px rgba(0,0,0,.25)' }} />
      </div>
      {state === 'starting' && <p role="status" style={{ margin: '8px 0 0', fontSize: '13px', color: '#64748b' }}>{t('keyImportCameraStarting')}</p>}
      {note && (
        <p role="alert" style={{ margin: '8px 0 0', fontSize: '13px', fontWeight: 600, color: '#c62828', wordBreak: 'break-word' }}>{note}</p>
      )}
    </>
  )
}
