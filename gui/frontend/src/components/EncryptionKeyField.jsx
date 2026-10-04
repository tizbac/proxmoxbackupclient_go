import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from '../i18n/i18nContext'

// Wails bindings are resolved by App.jsx on window.go; this component receives
// them as props so it stays testable/usable when the runtime is absent (the dev
// server, or a plain browser) — every backend call is optional here.
function resolve(name, prop) {
  if (prop) return prop
  if (window.go && window.go.main && window.go.main.App) return window.go.main.App[name]
  return undefined
}

/**
 * EncryptionKeyField edits the path to a PBS encryption key file.
 *
 * The GUI only ever sees the PATH (never key material) and only supports
 * passphrase-less key files, because it has no console to prompt on. The
 * fingerprint is surfaced on every change so the user can confirm the key that
 * will actually be used, and an unusable path is reported inline instead of
 * failing later during a backup or a restore.
 */
export default function EncryptionKeyField({ value, onChange, className = '' }) {
  const { t } = useTranslation()
  const [info, setInfo] = useState(null)
  const [busy, setBusy] = useState(false)

  const inspect = useCallback(resolve('InspectEncryptionKeyFile'), [])
  const generate = useCallback(resolve('GenerateEncryptionKeyFile'), [])
  const openExisting = useCallback(resolve('OpenEncryptionKeyDialog'), [])
  const openNew = useCallback(resolve('OpenEncryptionKeySaveDialog'), [])

  // Re-inspect whenever the path changes. inspect() is synchronous in Go and
  // cheap (one stat + a small JSON read), and doing it here means the user
  // always sees the fingerprint of the key that will be used.
  useEffect(() => {
    if (!inspect) {
      setInfo(null)
      return
    }
    try {
      setInfo(inspect(value || ''))
    } catch {
      setInfo(null)
    }
  }, [value, inspect])

  const handleBrowse = async () => {
    if (!openExisting) return
    try {
      const p = await openExisting()
      if (p) onChange(p)
    } catch (err) {
      console.error('encryption key browse failed', err)
    }
  }

  const handleGenerate = async () => {
    if (!generate || !openNew) return
    setBusy(true)
    try {
      let path = await openNew()
      if (!path) return
      const msg = t('encryptionKeyConfirm').replace('{path}', path)
      if (!window.confirm(msg)) return
      const result = await generate(path)
      onChange(path)
      // Trust the backend's own view: it only returns a usable key file.
      if (result) setInfo(result)
      window.alert(t('encryptionKeyCreated').replace('{path}', path).replace('{fp}', result?.fingerprint || '?'))
    } catch (err) {
      window.alert(t('encryptionKeyGenerateFailed').replace('{err}', err))
    } finally {
      setBusy(false)
    }
  }

  const reason = info && !info.usable ? info.reason : ''

  return (
    <div className={`form-group ${className}`.trim()}>
      <label>{t('encryptionKey')}</label>
      <div style={{ display: 'flex', gap: '8px', alignItems: 'stretch' }}>
        <input
          type="text"
          value={value || ''}
          onChange={(e) => onChange(e.target.value)}
          placeholder={t('phEncryptionKey')}
          spellCheck={false}
        />
        <button type="button" className="btn btn-secondary" onClick={handleBrowse} disabled={!openExisting || busy}>
          {t('encryptionKeyBrowse')}
        </button>
        <button type="button" className="btn btn-secondary" onClick={handleGenerate} disabled={!generate || busy}>
          {t('encryptionKeyGenerate')}
        </button>
        <button
          type="button"
          className="btn btn-secondary"
          onClick={() => onChange('')}
          disabled={busy || !(value || '')}
        >
          {t('encryptionKeyClear')}
        </button>
      </div>

      <div style={{ marginTop: '6px', fontSize: '12px', lineHeight: '1.4' }}>
        <div style={{ color: '#666' }}>{t('encryptionKeyHint')}</div>
        {!(value || '') && <div style={{ color: '#a06000' }}>{t('encryptionKeyUnset')}</div>}
        {info && info.usable && info.fingerprint && (
          <div style={{ color: '#1a7f37', wordBreak: 'break-all' }}>
            {t('encryptionKeyOk').replace('{fp}', info.fingerprint)}
          </div>
        )}
        {reason && (
          <div style={{ color: '#c62828', wordBreak: 'break-word' }}>{reason}</div>
        )}
      </div>
    </div>
  )
}