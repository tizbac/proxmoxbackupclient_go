import { useState, useEffect } from 'react'

function MachineBackupConfig({ config, setConfig, backupType, setBackupType, physicalDisks, setSelectedDrives, selectedDrives }) {
  const [isWindows, setIsWindows] = useState(false)

  useEffect(() => {
    // Check if we're on Windows (can be done via system info or by checking for Windows-specific APIs)
    // This is a placeholder - actual implementation would check system info
    setIsWindows(true) // For now, assuming Windows for this implementation
  }, [])

  const handleDriveSelect = (drivePath) => {
    if (selectedDrives.includes(drivePath)) {
      setSelectedDrives(selectedDrives.filter(d => d !== drivePath))
    } else {
      setSelectedDrives([...selectedDrives, drivePath])
    }
  }

  const handleSelectAll = () => {
    setSelectedDrives(physicalDisks.map(d => d.path))
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
            <button onClick={handleSelectAll}>Select All</button>
            <button onClick={handleDeselectAll}>Deselect All</button>
          </div>
          <div className="drives-list">
            {physicalDisks.map((drive) => (
              <div key={drive.device_path} className="drive-item">
                <label className="checkbox-label">
                  <input
                    type="checkbox"
                    id={drive.device_path}
                    checked={selectedDrives.includes(drive.device_path)}
                    onChange={() => handleDriveSelect(drive.device_path)}
                  />
                  <span className="label-text">
                    {drive.device_path} ({(drive.size / (1024 * 1024 * 1024)).toFixed(2)} GB) - {drive.model}
                  </span>
                </label>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

export default MachineBackupConfig