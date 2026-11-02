import { useTranslation } from '../i18n/i18nContext'

// missingDisks are disks of the Backup Set being edited that are not
// connected now ({ path, id }): the set's runs fail until they are back.
// systemInfo (GetSystemInfo) drives two Linux boxes from upstream: which
// snapshot module is loaded, and whether this window needs restarting as
// administrator to read the disks (onRestartAsAdmin); the disks are greyed
// out until then.
//
// Under each ticked disk its partitions are listed. In a Backup Set
// (setSkipParts given) each can be unticked to leave it out of the set
// (skipParts, ScheduledJob.skipPartitions): it is not read and goes into the
// image as zeros. A one-off backup reads whole disks, so there the ticks are
// greyed out. diskIdFor gives the identity the set saves for a disk.
function MachineBackupConfig({ backupType, physicalDisks, setSelectedDrives, selectedDrives, missingDisks = [], systemInfo = {}, onRestartAsAdmin,
  skipParts = [], setSkipParts, diskIdFor = (d) => d.unique_id || '' }) {
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

  // A partition is "1.2 GB" or "100 MB": a whole line per partition is too much.
  const shortSize = (bytes) => {
    const fmt = (v, d) => { try { return v.toLocaleString(language, { maximumFractionDigits: d }) } catch { return v.toFixed(d) } }
    return bytes >= 1e9 ? `${fmt(bytes / 1e9, 1)} GB` : `${fmt(bytes / 1e6, 0)} MB`
  }
  const partLabel = (p) => [tl('partitionNumber', `Partition ${p.number}`, { n: p.number }), p.mount, p.file_system || p.type, shortSize(p.size)].filter(Boolean).join(' · ')
  // How the run log names it if the partition table changes (in English, as the log is).
  const logLabel = (p) => `partition ${p.number}` + ([p.mount, p.file_system || p.type].filter(Boolean).length ? ` (${[p.mount, p.file_system || p.type].filter(Boolean).join(', ')})` : '')
  // Only an identity the disk carries itself is saved (as the backend does).
  const stableId = (id) => (id && !id.startsWith('size:') ? id : '')
  const skipsOn = (drive) => {
    const id = stableId(diskIdFor(drive))
    return skipParts.filter(s => s.diskId
      ? s.diskId === id || drive.unique_id === s.diskId || (drive.other_ids || []).includes(s.diskId)
      : !id && s.device === drive.device_path)
  }
  const sameStart = (s, p) => s.start === p.start && s.size === p.size
  const setPartBackedUp = (drive, p, backedUp) => {
    const mine = skipsOn(drive).filter(s => sameStart(s, p))
    const rest = skipParts.filter(s => !mine.includes(s))
    setSkipParts(backedUp ? rest : [...rest, { diskId: stableId(diskIdFor(drive)), device: drive.device_path, start: p.start, size: p.size, number: p.number, label: logLabel(p) }])
  }
  const setOnlyTip = tl('machinePartsSetOnly', 'Only a Backup Set can leave partitions out.')

  const renderParts = (drive) => {
    if (drive.partitions == null) {
      return <div className="drive-parts"><div className="drive-parts-hint">{tl('machinePartsUnknown', 'The partitions of this disk could not be read: the whole disk is backed up.')}</div></div>
    }
    if (drive.partitions.length === 0) return null
    const skips = skipsOn(drive)
    // Saved skips the disk no longer has (its partition table changed).
    const gone = setSkipParts ? skips.filter(s => !drive.partitions.some(p => sameStart(s, p))) : []
    return (
      <div className="drive-parts">
        {setSkipParts && <div className="drive-parts-hint">{tl('machinePartsHint', 'Untick a partition to leave it out of this set. It is not read and goes into the image as zeros, which take almost no space.')}</div>}
        <ul className="drive-parts-list">
          {drive.partitions.map(p => {
            const out = !!setSkipParts && skips.some(s => sameStart(s, p))
            const needed = p.role === 'system' || p.role === 'boot'
            return (
              <li key={p.start} className={'drive-part' + (out ? ' left-out' : '')}>
                <label className={setSkipParts ? undefined : 'locked'} title={setSkipParts ? undefined : setOnlyTip}>
                  <input
                    type="checkbox"
                    checked={!out}
                    disabled={!setSkipParts || needsElevation}
                    onChange={(e) => setPartBackedUp(drive, p, e.target.checked)}
                  />
                  <span className="drive-part-name">{partLabel(p)}</span>
                  {p.role === 'system' && <span className="drive-badge drive-badge-system">{t('machineDiskBadgeSystem')}</span>}
                  {p.role === 'boot' && <span className="drive-badge">{t('machineDiskBadgeBoot')}</span>}
                </label>
                {out && <div className="drive-part-note">{tl('machinePartLeftOut', 'Left out: it goes into the image empty and comes back unformatted when restored.')}</div>}
                {out && needed && <div className="drive-part-warn">⚠️ {tl('machinePartNoStart', 'The disk restored from this backup will not start without this partition.')}</div>}
              </li>
            )
          })}
        </ul>
        {gone.map(s => (
          <div key={`${s.start}-${s.size}`} className="drive-part-warn">
            {tl('machinePartGone', `${s.label} is no longer on this disk, so it is backed up. Saving the set forgets it.`, { label: s.label || `partition ${s.number}` })}
          </div>
        ))}
      </div>
    )
  }

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
              <div className="drive-entry" key={drive.device_path}>
                <label className="drive-item">
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
                {selectedDrives.includes(drive.device_path) && renderParts(drive)}
              </div>
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
