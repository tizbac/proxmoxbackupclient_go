function MachineBackupConfig({ backupType, physicalDisks, setSelectedDrives, selectedDrives }) {
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

  return (
    <div className="machine-backup-config">
      <h3>Machine Backup Configuration</h3>

      <div className="form-group">
        <label>Select Disks to Backup:</label>
        <div className="drive-selection">
          <div className="drive-actions">
            <button className="btn" onClick={handleSelectAll}>Select All</button>
            <button className="btn btn-secondary" onClick={handleDeselectAll}>Deselect All</button>
          </div>
          <div className="drives-list">
            {physicalDisks.length === 0 && (
              <div style={{ padding: '12px', color: '#718096' }}>No physical disks found.</div>
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
                {drive.is_boot_disk && <span className="drive-badge">BOOT</span>}
                {drive.is_system_disk && <span className="drive-badge drive-badge-system">SYSTEM</span>}
              </label>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

export default MachineBackupConfig
