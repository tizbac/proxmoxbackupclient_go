import { useEffect, useState } from 'react'
import { useTranslation } from '../i18n/i18nContext'

const { RequestQuit, MinimizeToTray, GetServiceStatus } = window.go?.main?.App || {}

// Shown when the titlebar X is clicked, instead of silently minimizing to
// tray with no visible way back to a real quit. See main.go's beforeClose.
export default function ClosePromptModal({ onClose }) {
  const { t } = useTranslation()
  // With Run as Service on, the service keeps running the schedules after Exit.
  const [serviceMode, setServiceMode] = useState(false)
  useEffect(() => {
    GetServiceStatus?.().then(st => setServiceMode(!!st?.serviceMode)).catch(() => {})
  }, [])

  const handleExit = () => {
    RequestQuit?.()
  }
  const handleMinimize = () => {
    MinimizeToTray?.()
    onClose()
  }

  return (
    <div
      style={{
        position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.45)',
        display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000,
      }}
      onClick={onClose}
    >
      <div
        className="card"
        style={{ maxWidth: '420px', width: '90%', margin: 0 }}
        onClick={(e) => e.stopPropagation()}
      >
        <h3 style={{ marginTop: 0 }}>{t('closePromptTitle')}</h3>
        <p style={{ color: '#4a5568', lineHeight: 1.5 }}>
          {t(serviceMode ? 'closePromptBodyService' : 'closePromptBody')}
        </p>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '16px' }}>
          <button className="btn btn-secondary" onClick={handleExit}>{t('closePromptExit')}</button>
          <button className="btn" onClick={handleMinimize}>{t('closePromptMinimize')}</button>
        </div>
      </div>
    </div>
  )
}
