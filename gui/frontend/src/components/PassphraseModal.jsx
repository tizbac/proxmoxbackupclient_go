import { useState, useEffect, useRef } from 'react'
import { useTranslation } from '../i18n/i18nContext'

// Global prompt for a passphrase-protected encryption key. The backend emits
// `encryption:passphrase-required` in the middle of a backup, restore or
// listing and blocks until SubmitEncryptionPassphrase answers it. A wrong
// passphrase comes back as a message and the modal stays open for a retry.
export default function PassphraseModal({ EventsOn }) {
  const { t } = useTranslation()
  const [req, setReq] = useState(null)
  const [passphrase, setPassphrase] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const inputRef = useRef(null)

  useEffect(() => {
    if (!EventsOn) return
    const unsub = EventsOn('encryption:passphrase-required', (data) => {
      setReq(data || {})
      setPassphrase('')
      setError((data && data.error) || '')
      setBusy(false)
    })
    return () => { if (unsub) unsub() }
  }, [EventsOn])

  useEffect(() => {
    if (req && inputRef.current) inputRef.current.focus()
  }, [req])

  if (!req) return null

  const submit = async (cancel) => {
    const fn = window.go?.main?.App?.SubmitEncryptionPassphrase
    if (!fn) {
      setReq(null)
      return
    }
    setBusy(true)
    try {
      const err = await fn(req.id, cancel ? '' : passphrase, !!cancel)
      if (err) {
        setError(String(err))
        setPassphrase('')
        setBusy(false)
        return
      }
      setReq(null)
      setPassphrase('')
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
        style={{ maxWidth: '460px', width: '90%', margin: 0 }}
        onSubmit={(e) => { e.preventDefault(); if (!busy && passphrase) submit(false) }}
      >
        <h3 style={{ marginTop: 0 }}>{t('encryptionPromptTitle')}</h3>
        <p style={{ color: '#4a5568', lineHeight: 1.5, marginTop: 0 }}>{t('encryptionPromptBody')}</p>
        <div style={{ fontSize: '12px', color: '#666', wordBreak: 'break-all', marginBottom: '10px' }}>
          <div>{req.path}</div>
          {req.fingerprint && <div>{t('encryptionPromptFingerprint').replace('{fp}', req.fingerprint)}</div>}
        </div>
        <input
          ref={inputRef}
          type="password"
          value={passphrase}
          onChange={(e) => setPassphrase(e.target.value)}
          placeholder={t('encryptionPassphrasePlaceholder')}
          autoComplete="off"
          spellCheck={false}
          disabled={busy}
        />
        {error && (
          <div style={{ color: '#c62828', fontSize: '13px', marginTop: '8px', wordBreak: 'break-word' }}>{error}</div>
        )}
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '16px' }}>
          <button type="button" className="btn btn-secondary" onClick={() => submit(true)} disabled={busy}>
            {t('encryptionPromptCancel')}
          </button>
          <button type="submit" className="btn" disabled={busy || !passphrase}>
            {t('encryptionPromptUnlock')}
          </button>
        </div>
      </form>
    </div>
  )
}
