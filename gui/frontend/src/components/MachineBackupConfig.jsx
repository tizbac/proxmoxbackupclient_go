import { useTranslation } from '../i18n/i18nContext'

// missingDisks are disks of the Backup Set being edited that are not
// connected now ({ path, id }): the set's runs fail until they are back.
// systemInfo (GetSystemInfo) drives two Linux boxes from upstream: which
// snapshot module is loaded, and whether this window needs restarting as
// administrator to read the disks (onRestartAsAdmin); the disks are greyed
// out until then.
function MachineBackupConfig({ backupType, physicalDisks, setSelectedDrives, selectedDrives, missingDisks = [], systemInfo = {}, onRestartAsAdmin }) {
  const { t, language } = useTranslation()
  const tl = (key, fallback, params) => { const v = t(key, params); return v === key ? fallback : v }
  const handleDriveSelect = (drivePath) => {
    if (selectedDrives.includes(drivePath)) {
      setSelectedDrives(selectedDrives.filter(d => d !== drivePath))
    } else {
      setSelectedDrives([...selectedDrives, drivePath])
    }
  }

  const handleSelectAll = () => {
    setSelectedDrives(physicalDisks.map(d => d.device_path))
  }

  const handleDeselectAll = () => {
    setSelectedDrives([])
  }

  // Ensure this component is only rendered when backupType is 'machine'
  if (backupType !== 'machine') return null

  const size = (bytes) => {
    const fmt = (v) => { try { return v.toLocaleString(language, { minimumFractionDigits: 1, maximumFractionDigits: 1 }) } catch { return v.toFixed(1) } }
    return `${fmt(bytes / 1e9)} GB (${fmt(bytes / 2 ** 30)} GiB)`
  }

  const linux = systemInfo.os === 'linux'
  const module = systemInfo.snapshot_module
  const needsElevation = !!systemInfo.needs_local_elevation

  return (
    <div className="machine-backup-config">
      <h3>{t('machineDisksTitle')}</h3>

      {linux && module && (
        <div className="info-box" style={{ marginBottom: '12px', borderColor: '#90caf9', background: '#e7f3ff' }}>
          {tl('snapModuleActive', `Snapshot module loaded: ${module}. Mounted disks are read through a snapshot.`, { module })}
        </div>
      )}
      {linux && !module && (
        <div className="info-box" style={{ marginBottom: '12px', borderColor: '#ffc107', background: '#fff3cd' }}>
          ⚠️ {tl('snapModuleMissing', 'No snapshot module loaded (elastio-snap or dattobd). A mounted disk cannot be snapshotted: install one, or turn off Use snapshots for the set and accept a backup that may be inconsistent.')}
        </div>
      )}
      {needsElevation && (
        <div className="info-box" style={{ marginBottom: '12px', borderColor: '#f5c6cb', background: '#f8d7da', color: '#721c24' }}>
          <strong>⚠️ {tl('elevNeededTitle', 'Administrator rights needed')}</strong>
          <div style={{ marginTop: '4px' }}>
            {tl('elevNeededHint', 'Reading disks needs root, and the service is not running. Restart this app as administrator, or turn on Run as Service in Preferences.')}
          </div>
          {onRestartAsAdmin && (
            <button className="btn" style={{ marginTop: '10px' }} onClick={onRestartAsAdmin}>{t('jobsPolicyElevateBtn')}</button>
          )}
        </div>
      )}

      <div className="form-group">
        <label>{t('machineDisksSelect')}</label>
        <div className="drive-selection">
          <div className="drive-actions">
            <button className="btn" onClick={handleSelectAll} disabled={needsElevation}>{t('machineDisksSelectAll')}</button>
            <button className="btn btn-secondary" onClick={handleDeselectAll} disabled={needsElevation}>{t('machineDisksDeselectAll')}</button>
          </div>
          <div className="drives-list" style={needsElevation ? { opacity: 0.6, pointerEvents: 'none' } : undefined}>
            {physicalDisks.length === 0 && (
              <div style={{ padding: '12px', color: '#718096' }}>{t('machineDisksNone')}</div>
            )}
            {physicalDisks.map((drive) => (
              <label className="drive-item" key={drive.device_path}>
                <input
                  type="checkbox"
                  checked={selectedDrives.includes(drive.device_path)}
                  onChange={() => handleDriveSelect(drive.device_path)}
                  disabled={needsElevation}
                />
                <span className="drive-device">
                  {drive.device_path}
                  {drive.drive_letters && drive.drive_letters.length > 0 && (
                    <span style={{ color: '#718096' }}>{` (${drive.drive_letters.join(' ')})`}</span>
                  )}
                </span>
                <span className="drive-size">{size(drive.size)}</span>
                <span className="drive-model">{drive.model}</span>
                {drive.is_boot_disk && <span className="drive-badge">{t('machineDiskBadgeBoot')}</span>}
                {drive.is_system_disk && <span className="drive-badge drive-badge-system">{t('machineDiskBadgeSystem')}</span>}
              </label>
            ))}
          </div>
          {missingDisks.map(m => (
            <div key={m.path} className="info-box" style={{ marginTop: '10px', borderColor: '#ef4444', background: '#fef2f2', color: '#991b1b' }}>
              {t('machineDiskNotConnected', { path: m.path, id: m.id })}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

export default MachineBackupConfig
