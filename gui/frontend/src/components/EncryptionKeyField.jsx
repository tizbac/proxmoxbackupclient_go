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
 * The GUI only ever sees the PATH (never key material). A passphrase-protected
 * key file is unlocked either by asking once per session (a modal, see
 * PassphraseModal) or by a passphrase stored in the config on this computer,
 * which is what lets scheduled jobs run unattended. The fingerprint is surfaced on every change so the user can confirm the key that
 * will actually be used, and an unusable path is reported inline instead of
 * failing later during a backup or a restore.
 */
export default function EncryptionKeyField({
  value,
  onChange,
  passphraseSet = false,
  passphrase = '',
  onPassphraseChange,
  clearStored = false,
  onClearStoredChange,
  className = '',
}) {
  const { t } = useTranslation()
  const [info, setInfo] = useState(null)
  const [busy, setBusy] = useState(false)
  const [mode, setMode] = useState(passphraseSet && !clearStored ? 'remember' : 'ask')
  const [verifyMsg, setVerifyMsg] = useState(null)

  const inspect = useCallback(resolve('InspectEncryptionKeyFile'), [])
  const generate = useCallback(resolve('GenerateEncryptionKeyFile'), [])
  const openExisting = useCallback(resolve('OpenEncryptionKeyDialog'), [])
  const openNew = useCallback(resolve('OpenEncryptionKeySaveDialog'), [])
  const verify = useCallback(resolve('VerifyEncryptionKeyPassphrase'), [])

  // Re-inspect whenever the path changes. Wails bindings always return a
  // Promise, so the result has to be awaited; a stale answer from a previous
  // path must not overwrite the current one.
  useEffect(() => {
    if (!inspect) {
      setInfo(null)
      return undefined
    }
    let live = true
    Promise.resolve()
      .then(() => inspect(value || ''))
      .then((r) => { if (live) setInfo(r || null) })
      .catch(() => { if (live) setInfo(null) })
    return () => { live = false }
  }, [value, inspect])

  // A different key file or a freshly loaded server resets the unlock choice to
  // what the backend actually holds (it drops a stored passphrase on path change).
  useEffect(() => {
    setMode(passphraseSet && !clearStored ? 'remember' : 'ask')
    setVerifyMsg(null)
  }, [value, passphraseSet]) // eslint-disable-line react-hooks/exhaustive-deps

  const chooseMode = (m) => {
    setMode(m)
    setVerifyMsg(null)
    if (m === 'ask') {
      if (onPassphraseChange) onPassphraseChange('')
      if (onClearStoredChange) onClearStoredChange(!!passphraseSet)
    } else if (onClearStoredChange) {
      onClearStoredChange(false)
    }
  }

  const handleVerify = async () => {
    if (!verify) return
    if (!passphrase) {
      setVerifyMsg({ ok: false, text: t('encryptionPassphraseEnter') })
      return
    }
    setBusy(true)
    try {
      const err = await verify(value || '', passphrase)
      setVerifyMsg(err ? { ok: false, text: String(err) } : { ok: true, text: t('encryptionPassphraseVerified') })
    } catch (e) {
      setVerifyMsg({ ok: false, text: String(e) })
    } finally {
      setBusy(false)
    }
  }

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

      {info && info.passphrase_protected && (
        <div style={{ marginTop: '10px', fontSize: '13px' }}>
          <div style={{ fontWeight: 600, marginBottom: '4px' }}>{t('encryptionPassphraseTitle')}</div>
          <div style={{ color: '#666', fontSize: '12px', marginBottom: '6px' }}>{t('encryptionPassphraseIntro')}</div>
          <label style={{ display: 'flex', gap: '6px', alignItems: 'flex-start', marginBottom: '4px', fontWeight: 'normal' }}>
            <input
              type="radio"
              name="encryption-unlock-mode"
              checked={mode === 'ask'}
              onChange={() => chooseMode('ask')}
              style={{ width: 'auto', marginTop: '3px' }}
            />
            <span>{t('encryptionPassphraseAsk')}</span>
          </label>
          <label style={{ display: 'flex', gap: '6px', alignItems: 'flex-start', marginBottom: '4px', fontWeight: 'normal' }}>
            <input
              type="radio"
              name="encryption-unlock-mode"
              checked={mode === 'remember'}
              onChange={() => chooseMode('remember')}
              style={{ width: 'auto', marginTop: '3px' }}
            />
            <span>{t('encryptionPassphraseRemember')}</span>
          </label>
          {mode === 'ask' && (
            <div style={{ color: '#a06000', fontSize: '12px' }}>{t('encryptionPassphraseAskNote')}</div>
          )}
          {mode === 'remember' && (
            <div style={{ marginTop: '6px' }}>
              <div style={{ display: 'flex', gap: '8px', alignItems: 'stretch' }}>
                <input
                  type="password"
                  value={passphrase || ''}
                  onChange={(e) => {
                    setVerifyMsg(null)
                    if (onPassphraseChange) onPassphraseChange(e.target.value)
                  }}
                  placeholder={passphraseSet ? t('encryptionPassphraseKeep') : t('encryptionPassphrasePlaceholder')}
                  autoComplete="new-password"
                  spellCheck={false}
                />
                <button type="button" className="btn btn-secondary" onClick={handleVerify} disabled={!verify || busy}>
                  {t('encryptionPassphraseVerify')}
                </button>
              </div>
              <div style={{ color: '#a06000', fontSize: '12px', marginTop: '4px' }}>{t('encryptionPassphraseRememberNote')}</div>
              {verifyMsg && (
                <div style={{ color: verifyMsg.ok ? '#1a7f37' : '#c62828', fontSize: '12px', marginTop: '4px', wordBreak: 'break-word' }}>
                  {verifyMsg.text}
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  )
}