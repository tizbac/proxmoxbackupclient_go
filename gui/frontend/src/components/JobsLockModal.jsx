// Shown when a Backup Set change is refused because scheduled job protection
// is on and this copy is not running as administrator. A modal rather than a
// fading status line: the change did NOT happen, so it must be acknowledged.
export default function JobsLockModal({ t, onClose, onRestartAsAdmin }) {
  return (
    <div
      style={{
        position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.55)',
        display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1200,
      }}
      onClick={onClose}
      role="alertdialog"
      aria-modal="true"
      aria-labelledby="jobs-lock-title"
    >
      <div
        style={{
          maxWidth: '480px', width: '90%', background: '#fff', borderRadius: '6px',
          overflow: 'hidden', boxShadow: '0 12px 44px rgba(0,0,0,.4)', border: '1px solid #9b2c2c',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div
          id="jobs-lock-title"
          style={{
            background: '#c53030', color: '#fff', padding: '12px 18px',
            fontSize: '16px', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '10px',
          }}
        >
          <span aria-hidden="true">🔒</span>
          <span>{t('jobsLockModalTitle')}</span>
        </div>
        <div style={{ padding: '18px', color: '#2d3748', lineHeight: 1.55, fontSize: '14px' }}>
          {t('jobsAdminRequired')}
        </div>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', padding: '0 18px 18px' }}>
          <button className="btn btn-secondary" onClick={onRestartAsAdmin}>{t('jobsPolicyElevateBtn')}</button>
          <button className="btn" onClick={onClose} autoFocus>{t('prefsOK')}</button>
        </div>
      </div>
    </div>
  )
}
