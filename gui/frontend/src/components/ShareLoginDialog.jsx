import { useState, useEffect, useRef } from 'react'
import { useTranslation } from '../i18n/i18nContext'

// The login the background service connects to a file server's shares with
// (Preferences > Network Shares; share_logins.go). The service reaches a
// share as this computer's own account, which a share that needs a user name
// and password refuses. The login is tried on a folder of the server before
// it is saved, so a wrong password is found here and not at the next backup.
//
// req: { server, path, username, editServer, change } or null. onClose(saved)
// is called when it is saved or cancelled.
export default function ShareLoginDialog({ req, onClose }) {
  const { t } = useTranslation()
  const [server, setServer] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [testPath, setTestPath] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const firstRef = useRef(null)

  useEffect(() => {
    if (!req) return
    setServer(req.server || '')
    setUsername(req.username || '')
    setPassword('')
    setTestPath(req.path || '')
    setError('')
    setBusy(false)
  }, [req])

  useEffect(() => {
    if (req && firstRef.current) firstRef.current.focus()
  }, [req])

  if (!req) return null

  // A folder to try the login on: the one given, or the server's name alone
  // is not enough (a login is tried on a share), so it is asked for.
  const cleanServer = server.trim().replace(/^[\\/]+/, '')
  const canSave = cleanServer && username.trim() && (password || req.change) && !busy

  const save = async () => {
    const fn = window.go?.main?.App?.SetShareLogin
    if (!fn) { onClose(false); return }
    setBusy(true)
    setError('')
    try {
      await fn(cleanServer, username.trim(), password, testPath.trim())
      setPassword('')
      onClose(true)
    } catch (e) {
      setError(String(e))
      setBusy(false)
    }
  }

  return (
    <div
      style={{
        position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.45)',
        display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1100,
      }}
    >
      <form
        className="card"
        style={{ maxWidth: '520px', width: '90%', margin: 0 }}
        onSubmit={(e) => { e.preventDefault(); if (canSave) save() }}
      >
        <h3 style={{ marginTop: 0 }}>{t('shareLoginTitle')}</h3>
        <p style={{ color: '#4a5568', lineHeight: 1.5, marginTop: 0 }}>
          {t('shareLoginBody', { server: cleanServer ? `\\\\${cleanServer}` : t('sharesServer') })}
        </p>
        <div className="form-group">
          <label htmlFor="shareServer">{t('sharesServer')}</label>
          <input
            id="shareServer"
            ref={req.editServer ? firstRef : null}
            type="text"
            value={server}
            onChange={(e) => setServer(e.target.value)}
            placeholder="fileserver"
            disabled={busy || !req.editServer}
            spellCheck={false}
          />
        </div>
        <div className="form-group">
          <label htmlFor="shareUser">{t('sharesUser')}</label>
          <input
            id="shareUser"
            ref={req.editServer ? null : firstRef}
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder={t('shareLoginUserPlaceholder')}
            autoComplete="off"
            spellCheck={false}
            disabled={busy}
          />
        </div>
        <div className="form-group">
          <label htmlFor="sharePassword">{t('shareLoginPassword')}</label>
          <input
            id="sharePassword"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={req.change ? t('shareLoginPasswordKeep') : ''}
            autoComplete="off"
            disabled={busy}
          />
        </div>
        <div className="form-group no-bottom-margin">
          <label htmlFor="shareTestPath">{t('shareLoginTestFolder')}</label>
          <input
            id="shareTestPath"
            type="text"
            value={testPath}
            onChange={(e) => setTestPath(e.target.value)}
            placeholder={cleanServer ? `\\\\${cleanServer}\\share\\folder` : '\\\\fileserver\\share\\folder'}
            spellCheck={false}
            disabled={busy || !!req.path}
          />
          <div style={{ fontSize: '12px', color: '#666', marginTop: '4px' }}>{t('shareLoginTestHint')}</div>
        </div>
        {error && (
          <div style={{ color: '#c62828', fontSize: '13px', marginTop: '8px', wordBreak: 'break-word' }}>{error}</div>
        )}
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '16px' }}>
          <button type="button" className="btn btn-secondary" onClick={() => onClose(false)} disabled={busy} style={{ margin: 0 }}>
            {t('shareLoginCancel')}
          </button>
          <button type="submit" className="btn" disabled={!canSave} style={{ margin: 0 }}>
            {busy ? `⏳ ${t('shareLoginTesting')}` : t('shareLoginSave')}
          </button>
        </div>
      </form>
    </div>
  )
}
