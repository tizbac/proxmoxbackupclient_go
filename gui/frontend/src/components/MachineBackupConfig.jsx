import { useTranslation } from '../i18n/i18nContext'

function MachineBackupConfig({ backupType, physicalDisks, setSelectedDrives, selectedDrives }) {
  const { t, language } = useTranslation()
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

  return (
    <div className="machine-backup-config">
      <h3>{t('machineDisksTitle')}</h3>

      <div className="form-group">
        <label>{t('machineDisksSelect')}</label>
        <div className="drive-selection">
          <div className="drive-actions">
            <button className="btn" onClick={handleSelectAll}>{t('machineDisksSelectAll')}</button>
            <button className="btn btn-secondary" onClick={handleDeselectAll}>{t('machineDisksDeselectAll')}</button>
          </div>
          <div className="drives-list">
            {physicalDisks.length === 0 && (
              <div style={{ padding: '12px', color: '#718096' }}>{t('machineDisksNone')}</div>
            )}
            {physicalDisks.map((drive) => (
              <label className="drive-item" key={drive.device_path}>
                <input
                  type="checkbox"
                  checked={selectedDrives.includes(drive.device_path)}
                  onChange={() => handleDriveSelect(drive.device_path)}
                />
                <span className="drive-device">{drive.device_path}</span>
                <span className="drive-size">{size(drive.size)}</span>
                <span className="drive-model">{drive.model}</span>
                {drive.is_boot_disk && <span className="drive-badge">{t('machineDiskBadgeBoot')}</span>}
                {drive.is_system_disk && <span className="drive-badge drive-badge-system">{t('machineDiskBadgeSystem')}</span>}
              </label>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

export default MachineBackupConfig
