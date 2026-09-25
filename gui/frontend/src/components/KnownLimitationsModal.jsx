import { useTranslation } from '../i18n/i18nContext'

export default function KnownLimitationsModal({ onClose }) {
  const { t } = useTranslation()

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
        style={{ maxWidth: '480px', width: '90%', margin: 0 }}
        onClick={(e) => e.stopPropagation()}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
          <h3 style={{ margin: 0 }}>⚠️ {t('knownLimitations')}</h3>
          <button className="btn btn-secondary" onClick={onClose} style={{ padding: '4px 10px' }}>✕</button>
        </div>
        <p style={{ marginBottom: '14px' }}>{t('knownLimitationsIntro')}</p>
        <ul style={{ paddingLeft: '20px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
          <li>{t('limitationNtfsAcl')}</li>
        </ul>
      </div>
    </div>
  )
}
