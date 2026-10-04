function MachineBackupConfig({ backupType, physicalDisks, setSelectedDrives, selectedDrives, systemInfo, t, RequestElevation }) {
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

  const handleElevation = async () => {
    if (RequestElevation) {
      try {
        await RequestElevation()
        // The app will relaunch elevated, so we don't need to do anything else here
      } catch (err) {
        console.error('Elevation failed:', err)
        alert(t('elevationFailed', { error: err.message || err }))
      }
    }
  }

  // Ensure this component is only rendered when backupType is 'machine'
  if (backupType !== 'machine') return null

  // On Linux, show snapshot module info
  const snapshotModule = systemInfo?.snapshot_module
  const isAdmin = systemInfo?.is_admin === true
  const isLinux = systemInfo?.os === 'linux'

  return (
    <div className="machine-backup-config">
      <h3>{t('machineBackupConfig')}</h3>

      {snapshotModule && (
        <div className="info-box" style={{ marginBottom: '16px', backgroundColor: '#e7f3ff', borderColor: '#90caf9' }}>
          <strong>{t('snapshotModuleActive')}: </strong>
          {t('snapshotModuleInfo', { module: snapshotModule })}
        </div>
      )}

      {!snapshotModule && isLinux && (
        <div className="info-box" style={{ marginBottom: '16px', backgroundColor: '#fff3cd', borderColor: '#ffc107' }}>
          <strong>⚠️ {t('snapshotModuleMissing')}</strong><br/>
          {t('snapshotModuleHint')}
        </div>
      )}

      {!isAdmin && isLinux && (
        <div className="info-box" style={{ marginBottom: '16px', backgroundColor: '#f8d7da', borderColor: '#f5c6cb' }}>
          <strong>⚠️ {t('elevationRequired')}</strong><br/>
          {t('elevationHint')}
          <div style={{ marginTop: '12px' }}>
            <button className="btn" onClick={handleElevation}>
              {t('runAsAdmin')}
            </button>
          </div>
        </div>
      )}

      <div className="form-group">
        <label>{t('selectDisksToBackup')}</label>
        <div className="drive-selection">
          <div className="drive-actions">
            <button 
              className="btn" 
              onClick={handleSelectAll} 
              disabled={!isAdmin && isLinux}
            >{t('selectAll')}</button>
            <button 
              className="btn btn-secondary" 
              onClick={handleDeselectAll} 
              disabled={!isAdmin && isLinux}
            >{t('deselectAll')}</button>
          </div>
          <div className="drives-list" style={{ opacity: (!isAdmin && isLinux) ? 0.6 : 1, pointerEvents: (!isAdmin && isLinux) ? 'none' : 'auto' }}>
            {physicalDisks.length === 0 && (
              <div style={{ padding: '12px', color: '#718096' }}>{t('noPhysicalDisksFound')}</div>
            )}
            {physicalDisks.map((drive) => (
              <label className="drive-item" key={drive.device_path} style={{ opacity: (!isAdmin && isLinux) ? 0.6 : 1 }}>
                <input
                  type="checkbox"
                  checked={selectedDrives.includes(drive.device_path)}
                  onChange={() => handleDriveSelect(drive.device_path)}
                  disabled={!isAdmin && isLinux}
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
