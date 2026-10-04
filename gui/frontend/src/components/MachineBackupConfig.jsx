function MachineBackupConfig({ backupType, physicalDisks, setSelectedDrives, selectedDrives, systemInfo, t }) {
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

  // On Linux, show snapshot module info
  const snapshotModule = systemInfo?.snapshot_module

  return (
    <div className="machine-backup-config">
      <h3>{t('machineBackupConfig')}</h3>

      {snapshotModule && (
        <div className="info-box" style={{ marginBottom: '16px', backgroundColor: '#e7f3ff', borderColor: '#90caf9' }}>
          <strong>{t('snapshotModuleActive')}: </strong>
          {t('snapshotModuleInfo', { module: snapshotModule })}
        </div>
      )}

      {!snapshotModule && systemInfo?.os === 'linux' && (
        <div className="info-box" style={{ marginBottom: '16px', backgroundColor: '#fff3cd', borderColor: '#ffc107' }}>
          <strong>⚠️ {t('snapshotModuleMissing')}</strong><br/>
          {t('snapshotModuleHint')}
        </div>
      )}

      <div className="form-group">
        <label>{t('selectDisksToBackup')}</label>
        <div className="drive-selection">
          <div className="drive-actions">
            <button className="btn" onClick={handleSelectAll}>{t('selectAll')}</button>
            <button className="btn btn-secondary" onClick={handleDeselectAll}>{t('deselectAll')}</button>
          </div>
          <div className="drives-list">
            {physicalDisks.length === 0 && (
              <div style={{ padding: '12px', color: '#718096' }}>{t('noPhysicalDisksFound')}</div>
            )}
            {physicalDisks.map((drive) => (
              <label className="drive-item" key={drive.device_path}>
                <input
                  type="checkbox"
                  checked={selectedDrives.includes(drive.device_path)}
                  onChange={() => handleDriveSelect(drive.device_path)}
                />
                <span className="drive-device">{drive.device_path}</span>
                <span className="drive-size">{(drive.size / (1024 * 1024 * 1024)).toFixed(2)} GB</span>
                <span className="drive-model">{drive.model}</span>
                {drive.is_boot_disk && <span className="drive-badge">{t('boot')}</span>}
                {drive.is_system_disk && <span className="drive-badge drive-badge-system">{t('system')}</span>}
              </label>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

export default MachineBackupConfig
