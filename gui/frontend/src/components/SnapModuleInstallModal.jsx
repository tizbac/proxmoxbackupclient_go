import { useState, useEffect, useRef } from 'react'
import { useTranslation } from '../i18n/i18nContext'

// "Install it for me" (Linux): explains what the snapshot module installer
// does, asks for a one-time password when Secure Boot is on (the user types it
// again at the next start to approve the module's signing key), runs
// InstallSnapshotModule with its output shown as it comes ("snapmodule:line"),
// and ends on one of: loaded, restart needed (with the steps), or failed.
// status is GetSystemInfo's snap_install; onFinished refreshes it.
export const MOK_PASSWORD_RE = /^[A-Za-z0-9]{8,16}$/

export function RestartSteps({ tl }) {
  return (
    <ol style={{ margin: '6px 0 0 18px', padding: 0, lineHeight: 1.6 }}>
      <li>{tl('snapInstallRestartStep1', 'Restart the computer.')}</li>
      <li>{tl('snapInstallRestartStep2', 'A blue screen, "Perform MOK management", appears: press a key within 10 seconds.')}</li>
      <li>{tl('snapInstallRestartStep3', 'Choose "Enroll MOK", then "Continue", then "Yes".')}</li>
      <li>{tl('snapInstallRestartStep4', 'Type the one-time password, then choose "Reboot".')}</li>
    </ol>
  )
}

export default function SnapModuleInstallModal({ status = {}, onClose, onFinished, onOpenManual }) {
  const { t } = useTranslation()
  const tl = (key, fallback, params) => { const v = t(key, params); return v === key ? fallback : v }
  const [phase, setPhase] = useState('intro') // intro, running, loaded, restart, failed
  const [pw, setPw] = useState('')
  const [pw2, setPw2] = useState('')
  const [note, setNote] = useState('')
  const [error, setError] = useState('')
  const [lines, setLines] = useState([])
  const logRef = useRef(null)
  const secureBoot = !!status.secure_boot

  useEffect(() => {
    const on = window.runtime?.EventsOn
    if (!on) return
    const off = on('snapmodule:line', (line) => setLines(prev => [...prev, String(line)]))
    return () => { if (off) off() }
  }, [])
  useEffect(() => {
    if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight
  }, [lines])

  const pwOk = !secureBoot || (MOK_PASSWORD_RE.test(pw) && pw === pw2)
  const pwHint = !secureBoot || pw === '' ? ''
    : !MOK_PASSWORD_RE.test(pw) ? tl('snapInstallPasswordRule', '8 to 16 letters and digits.')
      : pw2 !== '' && pw !== pw2 ? tl('snapInstallPasswordMismatch', 'The two passwords differ.') : ''

  const start = async () => {
    const fn = window.go?.main?.App?.InstallSnapshotModule
    if (!fn || !pwOk) return
    setLines([])
    setNote('')
    setError('')
    setPhase('running')
    try {
      const result = await fn(secureBoot ? pw : '')
      setPhase(result === 'restart' ? 'restart' : 'loaded')
    } catch (e) {
      const msg = String(e)
      if (msg.includes('ELEVATION_CANCELLED')) {
        setNote(tl('snapInstallDeclined', 'No administrator password was given, so nothing was installed.'))
        setPhase('intro')
      } else {
        setError(msg)
        setPhase('failed')
      }
    }
    if (onFinished) onFinished()
  }

  const busy = phase === 'running'
  const close = () => { if (!busy && onClose) onClose() }

  return (
    <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.45)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1100 }}>
      <form className="card" style={{ maxWidth: '620px', width: '92%', maxHeight: '90vh', overflowY: 'auto', margin: 0 }}
        onSubmit={(e) => { e.preventDefault(); if (phase === 'intro') start() }}>
        <h3 style={{ marginTop: 0 }}>{tl('snapInstallTitle', 'Install the snapshot module')}</h3>

        {phase === 'intro' && (
          <>
            <p style={{ lineHeight: 1.5, marginTop: 0 }}>
              {tl('snapInstallIntro', "This builds elastio-snap for this computer's Linux kernel and loads it, now and at every start. It first installs the build tools and kernel headers from your distribution, a download of a few hundred MB, so it needs the internet and takes a few minutes. You are asked for an administrator's password.")}
            </p>
            {secureBoot && (
              <div className="info-box" style={{ marginBottom: '12px' }}>
                <strong>{tl('snapInstallSecureBootTitle', 'Secure Boot is on')}</strong>
                <div style={{ marginTop: '4px', lineHeight: 1.5 }}>
                  {tl('snapInstallSecureBootBody', 'Secure Boot loads the module only once you approve its signing key, which you do once, at the next restart. Choose a one-time password for that. You type it again on a blue screen after the restart, which may read the keyboard as an American (US) one.')}
                </div>
                <div className="form-group" style={{ marginTop: '10px', marginBottom: '6px' }}>
                  <label>{tl('snapInstallPassword', 'One-time password')}</label>
                  <input type="password" value={pw} onChange={(e) => setPw(e.target.value)} autoComplete="new-password" spellCheck={false} maxLength={16} />
                </div>
                <div className="form-group no-bottom-margin">
                  <label>{tl('snapInstallPasswordAgain', 'Type it again')}</label>
                  <input type="password" value={pw2} onChange={(e) => setPw2(e.target.value)} autoComplete="new-password" spellCheck={false} maxLength={16} />
                </div>
                <div style={{ fontSize: '12px', marginTop: '6px', color: pwHint ? '#c62828' : '#666' }}>
                  {pwHint || tl('snapInstallPasswordRule', '8 to 16 letters and digits.')}
                </div>
              </div>
            )}
            {note && <div style={{ color: '#c62828', fontSize: '13px', marginBottom: '8px' }}>{note}</div>}
          </>
        )}

        {phase !== 'intro' && (
          <>
            {busy && <p style={{ marginTop: 0 }}>⏳ {tl('snapInstallRunning', 'Installing. This takes a few minutes: keep the app open.')}</p>}
            {phase === 'loaded' && (
              <div className="info-box snap-module-ok" style={{ marginBottom: '10px' }}>
                ✅ {tl('snapInstallDone', 'The snapshot module is installed and loaded. Mounted disks are now read through a snapshot.')}
              </div>
            )}
            {phase === 'restart' && (
              <div className="info-box" style={{ marginBottom: '10px' }}>
                <strong>{tl('snapInstallRestartTitle', 'Nearly done: restart to approve the key')}</strong>
                <RestartSteps tl={tl} />
                <div style={{ marginTop: '6px' }}>{tl('snapInstallRestartAfter', 'The module then loads at every start. Until the restart, machine backups cannot use snapshots.')}</div>
              </div>
            )}
            {phase === 'failed' && (
              <div className="info-box snap-module-missing" style={{ marginBottom: '10px' }}>
                ⚠️ {tl('snapInstallFailed', `The install did not finish: ${error}`, { error })}
                <div style={{ marginTop: '4px' }}>{tl('snapInstallFailedHint', 'The output below says why. The installation guide in Help has the steps to do it by hand.')}</div>
              </div>
            )}
            <pre ref={logRef} style={{ background: '#1e1e1e', color: '#d4d4d4', fontSize: '12px', padding: '8px', borderRadius: '4px', height: '240px', overflow: 'auto', whiteSpace: 'pre-wrap', wordBreak: 'break-word', margin: 0 }}>
              {lines.join('\n')}
            </pre>
          </>
        )}

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '16px' }}>
          {phase === 'failed' && onOpenManual && (
            <button type="button" className="btn btn-secondary" style={{ margin: 0 }} onClick={() => { close(); onOpenManual() }}>
              {tl('snapModuleHowTo', 'How to install')}
            </button>
          )}
          {phase === 'intro' && (
            <>
              <button type="button" className="btn btn-secondary" style={{ margin: 0 }} onClick={close}>{tl('snapInstallCancel', 'Cancel')}</button>
              <button type="submit" className="btn" style={{ margin: 0 }} disabled={!pwOk}>{tl('snapInstallStart', 'Install')}</button>
            </>
          )}
          {phase !== 'intro' && (
            <button type="button" className="btn" style={{ margin: 0 }} onClick={close} disabled={busy}>{tl('snapInstallClose', 'Close')}</button>
          )}
        </div>
      </form>
    </div>
  )
}
