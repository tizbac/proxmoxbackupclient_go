import { useState, useEffect, useMemo } from 'react'
import { useTranslation } from './i18n/i18nContext'
import LanguageSwitcher from './components/LanguageSwitcher'

// Wails runtime imports (will be available when built with Wails)
let GetConfigWithHostname, SaveConfig, TestConnection, StartBackup, ListSnapshots, ListSnapshotContents, GetSnapshotMeta, RestoreSnapshot, OpenRestoreDestDialog, ListPhysicalDisks, GetVersion, EventsOn, SearchFiles, CancelSearch
let SaveScheduledJob, UpdateScheduledJob, GetScheduledJobs, DeleteScheduledJob, GetJobHistory, GetSystemInfo, GetLastBackupDirs
// Multi-PBS functions
let ListPBSServers, GetPBSServer, AddPBSServer, UpdatePBSServer, DeletePBSServer, SetDefaultPBSServer, GetDefaultPBSID, TestPBSConnection
let GetServerFingerprint, PinPBSServerFingerprint

// Check if we're running in Wails
if (window.go) {
  GetConfigWithHostname = window.go.main.App.GetConfigWithHostname
  SaveConfig = window.go.main.App.SaveConfig
  TestConnection = window.go.main.App.TestConnection
  StartBackup = window.go.main.App.StartBackup
  ListSnapshots = window.go.main.App.ListSnapshots
  ListSnapshotContents = window.go.main.App.ListSnapshotContents
  GetSnapshotMeta = window.go.main.App.GetSnapshotMeta
  RestoreSnapshot = window.go.main.App.RestoreSnapshot
  OpenRestoreDestDialog = window.go.main.App.OpenRestoreDestDialog
  SearchFiles = window.go.main.App.SearchFiles
  CancelSearch = window.go.main.App.CancelSearch
  ListPhysicalDisks = window.go.main.App.ListPhysicalDisks
  GetVersion = window.go.main.App.GetVersion
  SaveScheduledJob = window.go.main.App.SaveScheduledJob
  UpdateScheduledJob = window.go.main.App.UpdateScheduledJob
  GetScheduledJobs = window.go.main.App.GetScheduledJobs
  DeleteScheduledJob = window.go.main.App.DeleteScheduledJob
  GetJobHistory = window.go.main.App.GetJobHistory
  GetSystemInfo = window.go.main.App.GetSystemInfo
  GetLastBackupDirs = window.go.main.App.GetLastBackupDirs
  // Multi-PBS
  ListPBSServers = window.go.main.App.ListPBSServers
  GetPBSServer = window.go.main.App.GetPBSServer
  AddPBSServer = window.go.main.App.AddPBSServer
  UpdatePBSServer = window.go.main.App.UpdatePBSServer
  DeletePBSServer = window.go.main.App.DeletePBSServer
  SetDefaultPBSServer = window.go.main.App.SetDefaultPBSServer
  GetDefaultPBSID = window.go.main.App.GetDefaultPBSID
  TestPBSConnection = window.go.main.App.TestPBSConnection
  GetServerFingerprint = window.go.main.App.GetServerFingerprint
  PinPBSServerFingerprint = window.go.main.App.PinPBSServerFingerprint
}

// Wails events
if (window.runtime) {
  EventsOn = window.runtime.EventsOn
}

function App() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState('servers')
  const [hostname, setHostname] = useState('')
  const [appVersion, setAppVersion] = useState('dev')
  const [systemInfo, setSystemInfo] = useState({ mode: 'Standalone', is_admin: false, service_available: false, os: '' })
  const [config, setConfig] = useState({
    baseurl: '',
    certfingerprint: '',
    authid: '',
    secret: '',
    datastore: '',
    namespace: '',
    backupdir: '',
    'backup-id': '',
    usevss: true
  })

  // Multi-PBS states
  const [pbsServers, setPbsServers] = useState([])
  const [defaultPBSID, setDefaultPBSID] = useState('')
  const [selectedPBSID, setSelectedPBSID] = useState('')
  const [editingServer, setEditingServer] = useState(null)
  const [serverFormData, setServerFormData] = useState({
    id: '',
    name: '',
    baseurl: '',
    certfingerprint: '',
    authid: '',
    secret: '',
    datastore: '',
    namespace: '',
    description: ''
  })
  const [serverStatus, setServerStatus] = useState({}) // Map of server ID -> connection status

  const [backupType, setBackupType] = useState('directory')
  const [backupDirs, setBackupDirs] = useState('')
  const [selectedDrives, setSelectedDrives] = useState([])
  const [physicalDisks, setPhysicalDisks] = useState([])
  const [excludeList, setExcludeList] = useState('')
  const [progress, setProgress] = useState(0)
  // Opt-in: split this backup into parts (for the first backup of a large volume).
  // Off by default → no size analysis, the backup starts immediately.
  const [splitFirstBackup, setSplitFirstBackup] = useState(false)

  // Scheduling states
  const [backupMode, setBackupMode] = useState('oneshot') // 'oneshot' or 'scheduled'
  const [scheduleTime, setScheduleTime] = useState('02:00')
  const [runAtStartup, setRunAtStartup] = useState(false)
  const [scheduledJobs, setScheduledJobs] = useState([])
  const [jobHistory, setJobHistory] = useState([])
  const [editingJobId, setEditingJobId] = useState(null) // Track which job is being edited
  const [backupStats, setBackupStats] = useState({
    startTime: null,
    lastUpdate: null,
    lastPercent: 0,
    speed: 0,
    eta: null,
    // Structured live stats (from the backup:stats event)
    bytesDone: 0,
    bytesTotal: 0,
    newChunks: 0,
    reusedChunks: 0,
    failedChunks: 0,
    currentDir: ''
  })
  const [status, setStatus] = useState({ message: '', type: '', visible: false })

  const [snapshots, setSnapshots] = useState([])
  const [restoreBackupId, setRestoreBackupId] = useState('')
  const [showSnapshots, setShowSnapshots] = useState(false)
  const [restorePBSID, setRestorePBSID] = useState('')
  const [selectedSnapshot, setSelectedSnapshot] = useState(null) // { id, unix, time }
  const [snapshotMeta, setSnapshotMeta] = useState(null)         // .nimbus_backup_meta.json sidecar (null if legacy)
  const [snapshotEntries, setSnapshotEntries] = useState([])     // flat list from backend
  const [expandedDirs, setExpandedDirs] = useState(new Set())     // expanded paths in tree
  const [selectedPaths, setSelectedPaths] = useState(new Set())   // selected entry paths
  const [restoreDestPath, setRestoreDestPath] = useState('')
  // 'original' (in-place), 'alternate_abs' (preserve tree), 'alternate_flat' (strip prefix)
  const [restoreMode, setRestoreMode] = useState('alternate_abs')
  const [restoreAllowCrossHost, setRestoreAllowCrossHost] = useState(false)
  // alternate sub-mode toggle: true = abs (keep tree), false = flat. Default flat per spec.
  const [restoreKeepTree, setRestoreKeepTree] = useState(false)
  const [restoreOptions, setRestoreOptions] = useState({
    overwrite: false,
    timestamps: true,
    acls: false, // disabled in UI until NTFS sidecar lands
    ads: false   // disabled in UI until NTFS sidecar lands
  })
  const [restoreLoading, setRestoreLoading] = useState(false)
  const [restoreProgress, setRestoreProgress] = useState(0)

  // ===== file search across snapshots =====
  const [showSearch, setShowSearch] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchMode, setSearchMode] = useState('name')      // 'name' | 'regex' | 'path'
  const [searchFrom, setSearchFrom] = useState('')           // yyyy-mm-dd
  const [searchTo, setSearchTo] = useState('')               // yyyy-mm-dd
  const [searchAssembleMissing, setSearchAssembleMissing] = useState(false)
  const [searchRunning, setSearchRunning] = useState(false)
  const [searchProgress, setSearchProgress] = useState({ percent: 0, message: '' })
  const [searchResult, setSearchResult] = useState(null)     // { hits, snapshots_*, truncated, cancelled }

  // Update restoreBackupId when config or hostname changes
  useEffect(() => {
    if (!restoreBackupId && (config['backup-id'] || hostname)) {
      setRestoreBackupId(config['backup-id'] || hostname)
    }
  }, [config['backup-id'], hostname])

  // When the selected snapshot has no sidecar (legacy) or its OS doesn't match
  // the current host, in-place is impossible. Snap the mode back to alternate
  // so a stale "original" selection doesn't survive across snapshot switches.
  useEffect(() => {
    if (restoreMode !== 'original') return
    if (!snapshotMeta) { setRestoreMode('alternate_abs'); return }
    if (!snapshotMeta.original_path) { setRestoreMode('alternate_abs'); return }
    if (systemInfo.os && snapshotMeta.os && systemInfo.os !== snapshotMeta.os) {
      setRestoreMode('alternate_abs')
    }
  }, [snapshotMeta, systemInfo.os, restoreMode])

  // Sync restore PBS dropdown with default once it's loaded
  useEffect(() => {
    if (!restorePBSID && defaultPBSID) {
      setRestorePBSID(defaultPBSID)
    }
  }, [defaultPBSID])

  // Load physical disks when switching to machine mode (DISABLED FOR NOW)
  /*
  useEffect(() => {
    if (backupType === 'machine' && ListPhysicalDisks && physicalDisks.length === 0) {
      ListPhysicalDisks().then(disks => {
        setPhysicalDisks(disks)
        // Select first disk by default
        if (disks.length > 0 && selectedDrives.length === 0) {
          setSelectedDrives([disks[0].path])
        }
      }).catch(err => {
        showStatus(`❌ Erreur lors de la détection des disques: ${err}`, 'error')
      })
    }
  }, [backupType])
  */

  // Listen to backup events
  useEffect(() => {
    if (!EventsOn) return

    const unsubProgress = EventsOn('backup:progress', (data) => {
      const now = Date.now()
      const percent = Math.round(data.percent)
      setProgress(percent)
      showStatus(`🔄 ${data.message}`, 'info')

      // Calculate speed and ETA
      setBackupStats(prev => {
        const startTime = prev.startTime || now
        const lastUpdate = prev.lastUpdate || now
        const timeDiff = (now - lastUpdate) / 1000 // seconds
        const percentDiff = percent - prev.lastPercent

        // Calculate speed (percent per second)
        let speed = prev.speed
        if (timeDiff > 0 && percentDiff > 0) {
          speed = percentDiff / timeDiff
        }

        // Calculate ETA (seconds remaining)
        let eta = null
        if (speed > 0 && percent < 100) {
          const remainingPercent = 100 - percent
          eta = Math.round(remainingPercent / speed)
        }

        return {
          ...prev, // preserve structured stats (bytes/chunks) set by backup:stats
          startTime,
          lastUpdate: now,
          lastPercent: percent,
          speed,
          eta
        }
      })
    })

    // Structured live statistics (bytes + chunk counts) emitted alongside progress.
    const unsubStats = EventsOn('backup:stats', (data) => {
      setBackupStats(prev => ({
        ...prev,
        bytesDone: data.bytesDone || 0,
        bytesTotal: data.bytesTotal || 0,
        newChunks: data.newChunks || 0,
        reusedChunks: data.reusedChunks || 0,
        failedChunks: data.failedChunks || 0,
        currentDir: data.currentDir || ''
      }))
    })

    const unsubComplete = EventsOn('backup:complete', (data) => {
      setProgress(data.success ? 100 : 0)
      setBackupStats({ startTime: null, lastUpdate: null, lastPercent: 0, speed: 0, eta: null, bytesDone: 0, bytesTotal: 0, newChunks: 0, reusedChunks: 0, failedChunks: 0, currentDir: '' })
      showStatus(data.success ? '✅ ' + data.message : '❌ ' + data.message, data.success ? 'success' : 'error')

      // Add to job history
      const historyEntry = {
        id: Date.now().toString(),
        name: `Backup ${config['backup-id'] || hostname}`,
        timestamp: new Date().toISOString(),
        status: data.success ? 'success' : 'failed',
        message: data.message,
        backupDirs: backupDirs.split('\n').map(d => d.trim()).filter(d => d),
        backupId: config['backup-id'] || hostname,
        useVSS: config.usevss
      }
      setJobHistory(prev => [historyEntry, ...prev].slice(0, 20)) // Keep last 20 entries
    })

    return () => {
      if (unsubProgress) unsubProgress()
      if (unsubStats) unsubStats()
      if (unsubComplete) unsubComplete()
    }
  }, [])

  // Listen to restore events
  useEffect(() => {
    if (!EventsOn) return
    const unsubP = EventsOn('restore:progress', (data) => {
      setRestoreProgress(Math.round((data.percent || 0) * 100))
      showStatus(`🔄 ${data.message || ''}`, 'info')
    })
    const unsubC = EventsOn('restore:complete', (data) => {
      setRestoreLoading(false)
      setRestoreProgress(data.success ? 100 : 0)
      showStatus(data.success ? `✅ ${data.message}` : `❌ ${data.message}`, data.success ? 'success' : 'error')
    })
    return () => {
      if (unsubP) unsubP()
      if (unsubC) unsubC()
    }
  }, [])

  // Listen to search progress
  useEffect(() => {
    if (!EventsOn) return
    const unsub = EventsOn('search:progress', (data) => {
      setSearchProgress({ percent: Math.round((data.percent || 0) * 100), message: data.message || '' })
    })
    return () => { if (unsub) unsub() }
  }, [])

  // Split size-analysis progress (explicit-split path only) — folder-by-folder so
  // a multi-minute scan of a large volume shows movement instead of a frozen spinner.
  useEffect(() => {
    if (!EventsOn) return
    const unsub = EventsOn('analysis:progress', (data) => {
      const done = data.done || 0
      const total = data.total || 0
      const gb = ((data.bytes || 0) / (1024 * 1024 * 1024)).toFixed(1)
      showStatus(`📊 ${t('splitAnalyzing')} ${done}/${total} (${gb} GB)`, 'info')
    })
    return () => { if (unsub) unsub() }
  }, [])

  // Load config with hostname on mount
  useEffect(() => {
    const loadData = async () => {
      try {
        // Load version
        if (GetVersion) {
          const version = await GetVersion()
          setAppVersion(version || 'dev')
        }

        // Load system info (mode, admin status, service availability)
        if (GetSystemInfo) {
          const sysInfo = await GetSystemInfo()
          setSystemInfo(sysInfo || { mode: 'Standalone', is_admin: false, service_available: false })
        }

        // Load last backup directories to pre-fill the form
        if (GetLastBackupDirs) {
          const lastDirs = await GetLastBackupDirs()
          if (lastDirs && lastDirs.length > 0) {
            setBackupDirs(lastDirs.join('\n'))
          }
        }

        if (GetConfigWithHostname) {
          const data = await GetConfigWithHostname()
          if (data) {
            // Extract hostname
            const hn = data.hostname || ''
            setHostname(hn)

            // Set config (hostname is already in backup-id if needed)
            setConfig({
              baseurl: data.baseurl || '',
              certfingerprint: data.certfingerprint || '',
              authid: data.authid || '',
              secret: data.secret || '',
              datastore: data.datastore || '',
              namespace: data.namespace || '',
              backupdir: data.backupdir || '',
              'backup-id': data['backup-id'] || hn,
              usevss: data.usevss !== undefined ? data.usevss : true
            })

            // Initialize backupDirs from config if available
            if (data.backupdir) {
              setBackupDirs(data.backupdir)
            }
          }
        }
      } catch (err) {
        console.error('Failed to load config:', err)
      }
    }

    loadData()
  }, [])

  // Load scheduled jobs and history on mount
  useEffect(() => {
    const loadSchedulerData = async () => {
      try {
        if (GetScheduledJobs) {
          const jobs = await GetScheduledJobs()
          setScheduledJobs(jobs || [])
        }

        if (GetJobHistory) {
          const history = await GetJobHistory()
          setJobHistory(history || [])
        }
      } catch (err) {
        console.error('Failed to load scheduler data:', err)
      }
    }

    loadSchedulerData()

    // Refresh history every 10 seconds to update status of running jobs
    const intervalId = setInterval(() => {
      if (GetJobHistory) {
        GetJobHistory().then(history => {
          setJobHistory(history || [])
        }).catch(err => {
          console.error('Failed to refresh job history:', err)
        })
      }
    }, 10000) // 10 seconds

    return () => clearInterval(intervalId)
  }, [])

  // Load PBS servers on mount
  useEffect(() => {
    const loadPBSServers = async () => {
      try {
        if (ListPBSServers) {
          const servers = await ListPBSServers()
          setPbsServers(servers || [])
        }

        if (GetDefaultPBSID) {
          const defaultID = await GetDefaultPBSID()
          setDefaultPBSID(defaultID || '')
          setSelectedPBSID(defaultID || '')
        }
      } catch (err) {
        console.error('Failed to load PBS servers:', err)
      }
    }

    loadPBSServers()
  }, [])

  const showStatus = (message, type) => {
    setStatus({ message, type, visible: true })
    setTimeout(() => {
      setStatus(s => ({ ...s, visible: false }))
    }, 5000)
  }

  // ==================== MULTI-PBS HANDLERS ====================

  const loadPBSServers = async () => {
    try {
      if (ListPBSServers) {
        const servers = await ListPBSServers()
        setPbsServers(servers || [])
      }
      if (GetDefaultPBSID) {
        const defaultID = await GetDefaultPBSID()
        setDefaultPBSID(defaultID || '')
      }
    } catch (err) {
      console.error('Failed to load PBS servers:', err)
      showStatus(`❌ ${t('statusServerLoadError')} ${err}`, 'error')
    }
  }

  const handleAddPBSServer = async () => {
    if (!AddPBSServer) {
      showStatus('❌ Wails runtime non disponible', 'error')
      return
    }

    try {
      // Generate ID from name if not provided
      if (!serverFormData.id) {
        serverFormData.id = serverFormData.name.toLowerCase().replace(/[^a-z0-9]/g, '-')
      }

      await AddPBSServer(serverFormData)
      showStatus(`✅ ${t('statusServerAdded')}`, 'success')

      // Reset form and reload
      setServerFormData({
        id: '',
        name: '',
        baseurl: '',
        certfingerprint: '',
        authid: '',
        secret: '',
        datastore: '',
        namespace: '',
        description: ''
      })
      setEditingServer(null)
      await loadPBSServers()
    } catch (err) {
      showStatus(`❌ Erreur: ${err}`, 'error')
    }
  }

  const handleUpdatePBSServer = async () => {
    if (!UpdatePBSServer) {
      showStatus('❌ Wails runtime non disponible', 'error')
      return
    }

    try {
      await UpdatePBSServer(serverFormData)
      showStatus(`✅ ${t('statusServerUpdated')}`, 'success')

      // Reset form and reload
      setServerFormData({
        id: '',
        name: '',
        baseurl: '',
        certfingerprint: '',
        authid: '',
        secret: '',
        datastore: '',
        namespace: '',
        description: ''
      })
      setEditingServer(null)
      await loadPBSServers()
    } catch (err) {
      showStatus(`❌ Erreur: ${err}`, 'error')
    }
  }

  const handleDeletePBSServer = async (id) => {
    if (!DeletePBSServer) {
      showStatus('❌ Wails runtime non disponible', 'error')
      return
    }

    if (!confirm(t('confirmDeleteServer').replace('{id}', id))) {
      return
    }

    try {
      await DeletePBSServer(id)
      showStatus(`✅ ${t('statusServerDeleted')}`, 'success')
      await loadPBSServers()
    } catch (err) {
      showStatus(`❌ Erreur: ${err}`, 'error')
    }
  }

  const handleSetDefaultPBS = async (id) => {
    if (!SetDefaultPBSServer) {
      showStatus('❌ Wails runtime non disponible', 'error')
      return
    }

    try {
      await SetDefaultPBSServer(id)
      setDefaultPBSID(id)
      showStatus(`✅ ${t('statusServerSetDefault').replace('{id}', id)}`, 'success')
    } catch (err) {
      showStatus(`❌ Erreur: ${err}`, 'error')
    }
  }

  // True when a connection failure is an unverified-certificate error (self-signed
  // PBS with no fingerprint pinned). These are recoverable via trust-on-first-use.
  const isCertError = (err) =>
    /certificate signed by unknown authority|failed to verify certificate|x509/i.test(String(err))

  const handleTestPBSConnection = async (id) => {
    if (!TestPBSConnection) {
      showStatus('❌ Wails runtime non disponible', 'error')
      return
    }

    try {
      setServerStatus(prev => ({ ...prev, [id]: 'testing' }))
      await TestPBSConnection(id)
      setServerStatus(prev => ({ ...prev, [id]: 'online' }))
      showStatus(`✅ ${t('statusConnectionSuccess').replace('{id}', id)}`, 'success')
    } catch (err) {
      const server = pbsServers.find(s => s.id === id)
      // Self-signed cert + no fingerprint yet: offer to discover and pin it (TOFU).
      if (isCertError(err) && server && !(server.certfingerprint || '').trim() &&
          GetServerFingerprint && PinPBSServerFingerprint) {
        try {
          const fp = await GetServerFingerprint(server.baseurl)
          if (window.confirm(t('tofuConfirm').replace('{host}', server.baseurl).replace('{fp}', fp))) {
            await PinPBSServerFingerprint(id, fp)
            await loadPBSServers()
            await TestPBSConnection(id)
            setServerStatus(prev => ({ ...prev, [id]: 'online' }))
            showStatus(`✅ ${t('statusFingerprintPinned')}`, 'success')
            return
          }
        } catch (fpErr) {
          showStatus(`❌ ${t('statusFingerprintFailed')} ${fpErr}`, 'error')
          setServerStatus(prev => ({ ...prev, [id]: 'offline' }))
          return
        }
      }
      setServerStatus(prev => ({ ...prev, [id]: 'offline' }))
      showStatus(`❌ ${t('statusConnectionFailed')} ${err}`, 'error')
    }
  }

  const handleEditServer = (server) => {
    setServerFormData(server)
    setEditingServer(server.id)
  }

  const handleCancelEdit = () => {
    setServerFormData({
      id: '',
      name: '',
      baseurl: '',
      certfingerprint: '',
      authid: '',
      secret: '',
      datastore: '',
      namespace: '',
      description: ''
    })
    setEditingServer(null)
  }

  // ==================== END MULTI-PBS HANDLERS ====================

  const handleSaveConfig = async () => {
    if (!SaveConfig) {
      showStatus('❌ Wails runtime non disponible', 'error')
      return
    }

    try {
      // Trim all string values to remove whitespace (with safe fallback for undefined)
      const trimmedConfig = {
        baseurl: (config.baseurl || '').trim(),
        certfingerprint: (config.certfingerprint || '').trim(),
        authid: (config.authid || '').trim(),
        secret: (config.secret || '').trim(),
        datastore: (config.datastore || '').trim(),
        namespace: (config.namespace || '').trim(),
        backupdir: (config.backupdir || '').trim(),
        'backup-id': (config['backup-id'] || '').trim() || hostname, // Use hostname if empty
        usevss: config.usevss !== undefined ? config.usevss : true
      }
      await SaveConfig(trimmedConfig)
      setConfig(trimmedConfig)
      showStatus(`✅ ${t('statusConfigSaved')}`, 'success')
    } catch (err) {
      showStatus(`❌ Erreur : ${err}`, 'error')
    }
  }

  const handleTestConnection = async () => {
    if (!TestConnection) {
      showStatus('❌ Wails runtime non disponible', 'error')
      return
    }

    // Test with current form values (no need to save first). Declared outside the
    // try so the TOFU retest in catch can reuse these already-normalized fields.
    const testConfig = {
      baseurl: (config.baseurl || '').trim(),
      certfingerprint: (config.certfingerprint || '').trim(),
      authid: (config.authid || '').trim(),
      secret: (config.secret || '').trim(),
      datastore: (config.datastore || '').trim(),
      namespace: (config.namespace || '').trim(),
      backupdir: (config.backupdir || '').trim(),
      'backup-id': (config['backup-id'] || '').trim() || hostname, // Use hostname if empty
      usevss: config.usevss !== undefined ? config.usevss : true
    }

    try {
      await TestConnection(testConfig)
      showStatus(`✅ ${t('statusConnectionOK')}`, 'success')
    } catch (err) {
      // Self-signed cert + no fingerprint yet: discover, re-test pinned, fill the
      // form so a Save persists it (legacy single-server config).
      if (isCertError(err) && !(config.certfingerprint || '').trim() && GetServerFingerprint) {
        try {
          const fp = await GetServerFingerprint((config.baseurl || '').trim())
          if (window.confirm(t('tofuConfirm').replace('{host}', config.baseurl).replace('{fp}', fp))) {
            // Reuse the already-normalized testConfig (trimmed fields) for the pinned retest.
            await TestConnection({ ...testConfig, certfingerprint: fp })
            setConfig(prev => ({ ...prev, certfingerprint: fp }))
            showStatus(`✅ ${t('statusFingerprintPinnedSave')}`, 'success')
            return
          }
        } catch (fpErr) {
          showStatus(`❌ ${t('statusFingerprintFailed')} ${fpErr}`, 'error')
          return
        }
      }
      showStatus(`❌ ${err}`, 'error')
    }
  }

  const handleLoadConfigFile = (e) => {
    const file = e.target.files[0]
    if (!file) return

    const reader = new FileReader()
    reader.onload = (evt) => {
      try {
        const loadedConfig = JSON.parse(evt.target.result)
        setConfig(loadedConfig)
        showStatus(`✅ ${t('statusConfigLoaded')}`, 'success')
      } catch (err) {
        showStatus(`❌ ${t('statusInvalidJSON')}`, 'error')
      }
    }
    reader.readAsText(file)
  }

  // Execute split backup for large volumes (explicit opt-in via the split toggle).
  // CreateBackupSplitPlan runs the size analysis the user asked for (bounded, and
  // reporting folder-by-folder progress via the "analysis:progress" event) and
  // returns one job per part; we then run a backup per part.
  const executeSplitBackup = async (dirList) => {
    if (!window.go || !window.go.main.App.CreateBackupSplitPlan) {
      showStatus('❌ Split backup not available', 'error')
      return
    }

    try {
      showStatus(`📊 ${t('splitAnalyzing')}`, 'info')
      const splitPlan = await window.go.main.App.CreateBackupSplitPlan(
        dirList,
        config['backup-id'] || hostname,
        excludeList.split('\n').filter(l => l.trim())
      )

      // If the analysis didn't yield a real split (volume below threshold, scan
      // incomplete, or splitting disabled), the single "job" only covers the
      // enumerated subfolders and would DROP files sitting directly under the
      // selected root. Fall back to a normal full backup of the roots, which
      // always captures everything.
      if (!splitPlan || splitPlan.length <= 1) {
        showStatus(`🚀 ${t('statusBackupStarting')}`, 'info')
        setProgress(5)
        await StartBackup(
          backupType,
          dirList,
          selectedDrives,
          excludeList.split('\n').filter(l => l.trim()),
          config['backup-id'],
          config.usevss,
          ''
        )
        showStatus(`⏳ ${t('statusBackupRunning')}`, 'info')
        return
      }

      showStatus(`🔄 Lancement de ${splitPlan.length} backups partiels...`, 'info')

      // Arm a one-shot listener for the next backup:complete BEFORE starting a
      // part, so a fast completion can't be missed. The backend emits this event
      // in both standalone (OnComplete) and service mode (pollBackupProgress),
      // so awaiting it gives the part's REAL outcome instead of guessing success.
      const armBackupCompletion = () => {
        let unsub
        const promise = new Promise((resolve) => {
          unsub = EventsOn('backup:complete', (data) => {
            if (typeof unsub === 'function') unsub()
            resolve(data || { success: false, message: 'completion sans données' })
          })
        })
        return { promise, cancel: () => { if (typeof unsub === 'function') unsub() } }
      }

      // Execute split jobs sequentially, waiting for each part's real outcome.
      let succeeded = 0
      const failures = []
      for (let i = 0; i < splitPlan.length; i++) {
        const job = splitPlan[i]
        showStatus(
          `📦 Backup ${job.index}/${job.total_jobs}: ${job.size_fmt}...`,
          'info'
        )

        const completion = armBackupCompletion()
        try {
          await StartBackup(
            backupType,
            job.folders,
            selectedDrives,
            // Merge user exclusions with this job's own (a root-remainder job
            // excludes the subfolders already covered by other jobs — v2-H-01).
            [...excludeList.split('\n').filter(l => l.trim()), ...(job.exclude_list || [])],
            job.backup_id,
            config.usevss,
            ''
          )
        } catch (err) {
          // The part never started (validation / dispatch error): no completion
          // event will arrive, so cancel the wait and offer a retry.
          completion.cancel()
          const retry = window.confirm(
            `Le backup ${job.index}/${job.total_jobs} n'a pas pu démarrer:\n${err}\n\n` +
            `Réessayer cette partie ?`
          )
          if (retry) { i--; continue }
          failures.push(`Partie ${job.index}: démarrage impossible (${err})`)
          continue
        }

        // Wait for THIS part to actually finish before starting the next one.
        const result = await completion.promise
        if (result.success) {
          succeeded++
          showStatus(`✅ Backup ${job.index}/${job.total_jobs} terminé`, 'success')
        } else {
          showStatus(
            `❌ Backup ${job.index}/${job.total_jobs} échoué: ${result.message || ''}`,
            'error'
          )
          const retry = window.confirm(
            `Le backup ${job.index}/${job.total_jobs} a échoué:\n${result.message || ''}\n\n` +
            `Réessayer cette partie avant de continuer ?`
          )
          if (retry) { i--; continue }
          failures.push(`Partie ${job.index}: ${result.message || 'échec'}`)
        }
      }

      // Honest aggregate: only claim success when EVERY part actually succeeded.
      if (failures.length === 0) {
        showStatus(
          `🎉 Tous les backups partiels terminés avec succès (${succeeded}/${splitPlan.length})`,
          'success'
        )
      } else {
        showStatus(
          `⚠️ Backup partiel: ${succeeded}/${splitPlan.length} OK, ${failures.length} échec(s) :\n` +
          failures.join('\n'),
          'error'
        )
      }
    } catch (err) {
      showStatus(`❌ Erreur split backup: ${err}`, 'error')
    }
  }

  const handleStartBackup = async () => {
    if (!StartBackup) {
      showStatus('❌ Wails runtime non disponible', 'error')
      return
    }

    // Parse backup directories (one per line)
    const dirList = backupDirs.split('\n').map(d => d.trim()).filter(d => d)

    if (backupType === 'directory' && dirList.length === 0) {
      showStatus('❌ Au moins un répertoire requis', 'error')
      return
    }

    if (backupType === 'machine' && selectedDrives.length === 0) {
      showStatus('❌ Au moins un disque requis', 'error')
      return
    }

    // Splitting is now an explicit, opt-in choice (the "split this backup" toggle),
    // intended for the first backup of a large volume. When it's off we never size
    // the directories — the backup starts immediately, so a whole-drive root like
    // C:\ can no longer hang on the analysis. When it's on, executeSplitBackup runs
    // the (bounded, progress-reporting) size analysis the user asked for, then runs
    // one backup per part. Subsequent backups are left unsplit (full).
    if (splitFirstBackup && backupType === 'directory' && backupMode === 'oneshot') {
      await executeSplitBackup(dirList)
      return
    }

    // Scheduled mode - save or update job instead of executing immediately
    if (backupMode === 'scheduled') {
      if (!SaveScheduledJob || !UpdateScheduledJob) {
        showStatus('❌ Fonction de planification non disponible', 'error')
        return
      }

      const jobData = {
        id: editingJobId || Date.now().toString(),
        name: `Backup ${config['backup-id'] || hostname}`,
        scheduleTime: scheduleTime,
        runAtStartup: runAtStartup,
        backupDirs: dirList,
        backupId: config['backup-id'],
        useVSS: config.usevss,
        backupType: backupType,
        excludeList: excludeList.split('\n').filter(l => l.trim())
      }

      // Save or update to backend
      try {
        if (editingJobId) {
          // Update existing job
          await UpdateScheduledJob(jobData)
          setScheduledJobs(scheduledJobs.map(j => j.id === editingJobId ? jobData : j))
          showStatus(`✅ Backup modifié pour ${scheduleTime}`, 'success')
          setEditingJobId(null)
        } else {
          // Create new job
          await SaveScheduledJob(jobData)
          setScheduledJobs([...scheduledJobs, jobData])
          showStatus(`✅ Backup planifié pour ${scheduleTime}`, 'success')
        }
        // Reset form after save
        setScheduleTime('02:00')
        setRunAtStartup(false)
        setBackupDirs('')
      } catch (err) {
        showStatus(`❌ Erreur: ${err}`, 'error')
      }
      return
    }

    // One-shot mode - execute immediately
    showStatus(`🚀 ${t('statusBackupStarting')}`, 'info')
    setProgress(5)

    try {
      await StartBackup(
        backupType,
        dirList,
        selectedDrives,
        excludeList.split('\n').filter(l => l.trim()),
        config['backup-id'],
        config.usevss,
        ''
      )
      // Backup started in background - progress will be shown via events
      showStatus(`⏳ ${t('statusBackupRunning')}`, 'info')
    } catch (err) {
      setProgress(0)
      showStatus(`❌ ${err}`, 'error')
    }
  }

  const handleListSnapshots = async () => {
    if (!ListSnapshots) {
      showStatus('❌ Wails runtime non disponible', 'error')
      return
    }
    if (!restoreBackupId) {
      showStatus('❌ Backup ID requis', 'error')
      return
    }

    showStatus('🔍 Recherche des snapshots...', 'info')
    setSelectedSnapshot(null)
    setSnapshotEntries([])
    setSelectedPaths(new Set())
    setExpandedDirs(new Set())

    try {
      const snaps = await ListSnapshots(restorePBSID || '', restoreBackupId)
      setSnapshots(snaps || [])
      setShowSnapshots(true)
      showStatus(`✅ ${snaps.length} snapshot(s) trouvé(s)`, 'success')
    } catch (err) {
      showStatus(`❌ ${err}`, 'error')
    }
  }

  const handleSelectSnapshot = async (snap, forceRefresh = false) => {
    if (!ListSnapshotContents) {
      showStatus('❌ Wails runtime non disponible', 'error')
      return
    }
    setSelectedSnapshot(snap)
    setSnapshotMeta(null)
    setSnapshotEntries([])
    setSelectedPaths(new Set())
    setExpandedDirs(new Set())
    showStatus(`📥 ${t('loadingSnapshotContents')}`, 'info')
    const effectiveBackupId = snap.backup_id || restoreBackupId
    try {
      // Backend uses the snapshot's actual backup_id (snap.backup_id) so split
      // backups list their real contents, not the partial search term.
      const entries = await ListSnapshotContents(restorePBSID || '', effectiveBackupId, snap.unix, forceRefresh)
      setSnapshotEntries(entries || [])
      showStatus(`✅ ${(entries || []).length} ${t('entriesLoaded')}`, 'success')
    } catch (err) {
      showStatus(`❌ ${err}`, 'error')
    }
    // Meta is informational — fire-and-forget. The listing call above has
    // already populated the cache, so this is a cheap cache hit. Failure is
    // silent: legacy snapshots simply have no sidecar.
    if (GetSnapshotMeta) {
      try {
        const meta = await GetSnapshotMeta(restorePBSID || '', effectiveBackupId, snap.unix)
        if (meta) setSnapshotMeta(meta)
      } catch (_err) {
        // ignored — banner stays hidden
      }
    }
  }

  const handleReloadSnapshot = async () => {
    if (!selectedSnapshot) return
    await handleSelectSnapshot(selectedSnapshot, true)
  }

  const handleBrowseRestoreDest = async () => {
    if (!OpenRestoreDestDialog) {
      showStatus('❌ Wails runtime non disponible', 'error')
      return
    }
    try {
      const dir = await OpenRestoreDestDialog()
      if (dir) setRestoreDestPath(dir)
    } catch (err) {
      // In service mode the native picker is unavailable by design — guide the
      // user to the manual path field instead of flagging it as a failure.
      showStatus(`ℹ️ ${err}`, 'info')
    }
  }

  // ===== file search handlers =====

  // Convert a yyyy-mm-dd local date string to Unix seconds. endOfDay pushes to
  // 23:59:59 so the "To" bound is inclusive of the whole day. "" → 0 (open).
  const parseDateToUnix = (s, endOfDay = false) => {
    if (!s) return 0
    const [y, m, d] = s.split('-').map(Number)
    if (!y || !m || !d) return 0
    const dt = endOfDay ? new Date(y, m - 1, d, 23, 59, 59) : new Date(y, m - 1, d, 0, 0, 0)
    return Math.floor(dt.getTime() / 1000)
  }

  const handleSearch = async () => {
    if (!SearchFiles) {
      showStatus('❌ Wails runtime non disponible', 'error')
      return
    }
    if (!searchQuery.trim()) {
      showStatus('❌ ' + t('searchQueryRequired'), 'error')
      return
    }
    const prefix = (restoreBackupId || hostname || '').trim()
    setSearchRunning(true)
    setSearchResult(null)
    setSearchProgress({ percent: 0, message: '' })
    try {
      const res = await SearchFiles(
        restorePBSID || '',
        prefix,
        searchQuery,
        searchMode,
        parseDateToUnix(searchFrom, false),
        parseDateToUnix(searchTo, true),
        searchAssembleMissing
      )
      setSearchResult(res)
      const n = (res && res.hits) ? res.hits.length : 0
      showStatus(`✅ ${t('searchDone').replace('{n}', n)}`, 'success')
    } catch (err) {
      showStatus(`❌ ${err}`, 'error')
    } finally {
      setSearchRunning(false)
    }
  }

  const handleCancelSearch = () => {
    if (CancelSearch) {
      try { CancelSearch() } catch (_e) { /* best effort */ }
    }
  }

  // Pre-fill the restore form from a search hit: select the hit's snapshot,
  // load its contents, and tick the matched entry.
  const handleRestoreHit = async (hit) => {
    setRestoreBackupId(hit.backup_id)
    const iso = new Date(hit.snapshot_time * 1000).toISOString()
    const snap = {
      id: iso.slice(0, 19) + 'Z',
      backup_id: hit.backup_id,
      unix: hit.snapshot_time,
      time: new Date(hit.snapshot_time * 1000).toLocaleString(),
    }
    setShowSnapshots(true)
    await handleSelectSnapshot(snap)
    setSelectedPaths(new Set([hit.path]))
    const parts = hit.path.split('/')
    const dirs = new Set()
    let acc = ''
    for (let i = 0; i < parts.length - 1; i++) {
      acc = acc ? `${acc}/${parts[i]}` : parts[i]
      dirs.add(acc)
    }
    setExpandedDirs(dirs)
  }

  // inPlaceBlocker returns a translation key (or null when in-place is OK).
  // Drives the disabled state of the in-place radio + its tooltip.
  const inPlaceBlocker = () => {
    if (!snapshotMeta) return 'inPlaceNoMeta'
    if (!snapshotMeta.original_path) return 'inPlaceNoOriginalPath'
    if (systemInfo.os && snapshotMeta.os && systemInfo.os !== snapshotMeta.os) return 'inPlaceOsMismatch'
    return null
  }

  // crossHostMismatch returns true when the backup hostname differs from the
  // current machine. Comparison is case-insensitive and ignores the domain
  // suffix — same rule as backend equalHostnames.
  const crossHostMismatch = () => {
    if (!snapshotMeta || !snapshotMeta.hostname || !hostname) return false
    const norm = s => (s || '').toLowerCase().split('.')[0]
    return norm(snapshotMeta.hostname) !== norm(hostname)
  }

  const handleRestoreSnapshot = async () => {
    if (!RestoreSnapshot) {
      showStatus('❌ Wails runtime non disponible', 'error')
      return
    }
    if (!selectedSnapshot) {
      showStatus('❌ ' + t('selectSnapshotFirst'), 'error')
      return
    }

    // Resolve effective mode. The UI radio is binary (in-place / alternate);
    // the alternate sub-mode comes from the "keep tree" toggle.
    let effectiveMode = restoreMode
    if (restoreMode !== 'original') {
      effectiveMode = restoreKeepTree ? 'alternate_abs' : 'alternate_flat'
    }

    if (effectiveMode !== 'original' && !restoreDestPath) {
      showStatus('❌ ' + t('destinationRequired'), 'error')
      return
    }

    // In-place: scary, get explicit confirmation. confirm() is a stopgap until
    // we wire a real modal — for the alpha phase it's enough and the message
    // is precise about what will happen.
    if (effectiveMode === 'original') {
      const target = snapshotMeta?.original_path || '?'
      const msg = t('inPlaceConfirm').replace('{path}', target)
      // eslint-disable-next-line no-alert
      if (!window.confirm(msg)) {
        return
      }
    }

    // Empty selection = restore everything in the snapshot
    const includes = Array.from(selectedPaths)

    setRestoreLoading(true)
    setRestoreProgress(0)
    showStatus(`🔄 ${t('statusRestoring').replace('{time}', selectedSnapshot.time)}`, 'info')

    try {
      await RestoreSnapshot(
        restorePBSID || '',
        selectedSnapshot.backup_id || restoreBackupId,
        selectedSnapshot.id,
        restoreDestPath,
        effectiveMode,
        includes,
        restoreAllowCrossHost,
        restoreOptions.acls,
        restoreOptions.ads,
        restoreOptions.timestamps,
        restoreOptions.overwrite
      )
      // Completion arrives via the restore:complete event.
    } catch (err) {
      setRestoreLoading(false)
      showStatus(`❌ ${err}`, 'error')
    }
  }

  // ===== tree helpers (snapshot navigation) =====

  // Build a map childrenByDir: dir -> [entry...] from the flat list, plus a
  // set of all dir paths. Re-derived on every render — entries are tiny.
  const buildTree = (entries) => {
    const childrenByDir = new Map()
    const dirSet = new Set([''])
    childrenByDir.set('', [])
    for (const e of entries) {
      if (e.is_dir) dirSet.add(e.path)
    }
    for (const e of entries) {
      const slash = e.path.lastIndexOf('/')
      const parent = slash < 0 ? '' : e.path.substring(0, slash)
      // Some archives may emit a child without ever emitting the parent dir
      // entry. Make sure such parents still exist as buckets.
      if (!childrenByDir.has(parent)) childrenByDir.set(parent, [])
      childrenByDir.get(parent).push(e)
      if (e.is_dir && !childrenByDir.has(e.path)) childrenByDir.set(e.path, [])
    }
    // Sort each bucket: dirs first, then alphabetical
    for (const list of childrenByDir.values()) {
      list.sort((a, b) => {
        if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
        return a.path.localeCompare(b.path)
      })
    }
    return childrenByDir
  }

  const toggleDir = (path) => {
    setExpandedDirs(prev => {
      const next = new Set(prev)
      if (next.has(path)) next.delete(path)
      else next.add(path)
      return next
    })
  }

  const togglePathSelection = (path) => {
    setSelectedPaths(prev => {
      const next = new Set(prev)
      if (next.has(path)) next.delete(path)
      else next.add(path)
      return next
    })
  }

  const formatBytes = (bytes) => {
    if (!bytes || bytes < 1024) return `${bytes || 0} B`
    const units = ['KB', 'MB', 'GB', 'TB']
    let n = bytes / 1024
    let i = 0
    while (n >= 1024 && i < units.length - 1) { n /= 1024; i++ }
    return `${n.toFixed(1)} ${units[i]}`
  }

  // Reconstruct the absolute on-disk location a file came from, by joining the
  // backup's original_path (from the meta sidecar) with the archive-relative
  // path. Critical for split backups, where each part is a separate backup-id
  // and the relative tree alone doesn't tell you which physical folder/drive a
  // file belonged to. Returns null when the snapshot has no meta sidecar.
  const absOriginPath = (archivePath) => {
    const root = snapshotMeta?.original_path
    if (!root) return null
    const sep = (snapshotMeta?.os === 'windows' || root.includes('\\')) ? '\\' : '/'
    const base = root.endsWith(sep) ? root.slice(0, -1) : root
    if (!archivePath) return base
    return base + sep + archivePath.split('/').join(sep)
  }

  // Total bytes the current selection will restore. Selecting a directory pulls
  // in all its descendants, so we sum every file that is itself selected or
  // lives under a selected path. An empty selection means "restore everything",
  // so we sum the whole snapshot. Memoized — snapshots can hold 100k+ entries.
  const selectionBytes = useMemo(() => {
    if (!snapshotEntries.length) return 0
    if (selectedPaths.size === 0) {
      return snapshotEntries.reduce((sum, e) => e.is_dir ? sum : sum + (e.size || 0), 0)
    }
    const sel = Array.from(selectedPaths)
    let sum = 0
    for (const e of snapshotEntries) {
      if (e.is_dir) continue
      for (const p of sel) {
        if (e.path === p || e.path.startsWith(p + '/')) { sum += (e.size || 0); break }
      }
    }
    return sum
  }, [snapshotEntries, selectedPaths])

  // Recursive renderer driven by the children map. Depth is only used for the
  // visual indent.
  const renderTreeNode = (entry, childrenByDir, depth) => {
    const isExpanded = expandedDirs.has(entry.path)
    const isSelected = selectedPaths.has(entry.path)
    const indent = { paddingLeft: `${depth * 16}px` }
    const origin = absOriginPath(entry.path)
    const rowTitle = origin ? t('originTooltip').replace('{path}', origin) : entry.path
    return (
      <div key={entry.path}>
        <div title={rowTitle} style={{ ...indent, display: 'flex', alignItems: 'center', padding: '4px 8px', cursor: 'pointer', borderBottom: '1px solid #f1f5f9' }}>
          <input
            type="checkbox"
            checked={isSelected}
            onChange={() => togglePathSelection(entry.path)}
            style={{ marginRight: '8px' }}
          />
          {entry.is_dir ? (
            <span onClick={() => toggleDir(entry.path)} style={{ cursor: 'pointer', userSelect: 'none', marginRight: '4px' }}>
              {isExpanded ? '📂' : '📁'}
            </span>
          ) : (
            <span style={{ marginRight: '4px' }}>📄</span>
          )}
          <span
            onClick={() => entry.is_dir && toggleDir(entry.path)}
            style={{ flex: 1, cursor: entry.is_dir ? 'pointer' : 'default', fontSize: '14px' }}
          >
            {entry.path.split('/').pop() || entry.path}
          </span>
          {!entry.is_dir && (
            <span style={{ color: '#64748b', fontSize: '12px', marginLeft: '8px' }}>
              {formatBytes(entry.size)}
            </span>
          )}
        </div>
        {entry.is_dir && isExpanded && (childrenByDir.get(entry.path) || []).map(child =>
          renderTreeNode(child, childrenByDir, depth + 1)
        )}
      </div>
    )
  }

  return (
    <>
      <div className="header">
        <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
          <div>
            <h1>🛡️ {t('appTitle')}</h1>
            <p>{t('appSubtitle')}</p>
          </div>
          <LanguageSwitcher />
        </div>
      </div>

      <div className="container">
        <div className="tabs">
          <div className={`tab ${activeTab === 'servers' ? 'active' : ''}`} onClick={() => setActiveTab('servers')}>
            {t('tabServers')}
          </div>
          <div className={`tab ${activeTab === 'backup' ? 'active' : ''}`} onClick={() => setActiveTab('backup')}>
            {t('tabBackup')}
          </div>
          <div className={`tab ${activeTab === 'restore' ? 'active' : ''}`} onClick={() => setActiveTab('restore')}>
            {t('tabRestore')}
          </div>
          <div className={`tab ${activeTab === 'about' ? 'active' : ''}`} onClick={() => setActiveTab('about')}>
            {t('tabAbout')}
          </div>
        </div>

        {/* PBS Configuration Tab */}
        <div className={`tab-content ${activeTab === 'servers' ? 'active' : ''}`}>
          <h2>🖥️ {t('serversTitle')}</h2>

          {/* Show form first if no servers configured */}
          {pbsServers.length === 0 ? (
            <>
              <div className="info-box" style={{marginBottom: '20px', backgroundColor: '#eef2ff', borderLeft: '4px solid #667eea'}}>
                👋 <strong>{t('welcomeMessage')}</strong> {t('welcomeText')}<br/>
                {!config.baseurl && (
                  <>
                    <br/>
                    <strong>📦 {t('noPBSYet')}</strong><br/>
                    <a
                      href={`${t('chooseBackupUrl')}?utm_source=NimbusGui&utm_medium=tooling&utm_campaign=version-${appVersion}&utm_content=first-setup`}
                      target="_blank"
                      rel="noopener noreferrer"
                      style={{color: '#667eea', fontWeight: 'bold', textDecoration: 'underline'}}
                    >
                      {t('orderStorage')} →
                    </a>
                  </>
                )}
              </div>

              {/* Add Server Form - Prominent when no servers */}
              <div className="card">
                <h3>➕ {t('addYourServer')}</h3>
              <table style={{width: '100%', marginTop: '15px'}}>
                <thead>
                  <tr>
                    <th>{t('name')}</th>
                    <th>{t('url')}</th>
                    <th>{t('datastore')}</th>
                    <th>{t('status')}</th>
                    <th>{t('actions')}</th>
                  </tr>
                </thead>
                <tbody>
                  {pbsServers.map(server => (
                    <tr key={server.id}>
                      <td>
                        <strong>{server.name}</strong>
                        {server.id === defaultPBSID && <span style={{marginLeft: '5px', color: '#fbbf24'}}>⭐ {t('default')}</span>}
                        {server.description && <div style={{fontSize: '0.85em', color: '#999'}}>{server.description}</div>}
                      </td>
                      <td>{server.baseurl}</td>
                      <td>{server.datastore}/{server.namespace || '-'}</td>
                      <td>
                        {serverStatus[server.id] === 'testing' && <span style={{color: '#3b82f6'}}>🔄 {t('testing')}</span>}
                        {serverStatus[server.id] === 'online' && <span style={{color: '#10b981'}}>🟢 {t('online')}</span>}
                        {serverStatus[server.id] === 'offline' && <span style={{color: '#ef4444'}}>🔴 {t('offline')}</span>}
                        {!serverStatus[server.id] && <span style={{color: '#999'}}>⚪ {t('untested')}</span>}
                      </td>
                      <td>
                        <button onClick={() => handleTestPBSConnection(server.id)} style={{marginRight: '5px', padding: '5px 10px', fontSize: '0.9em'}}>
                          🔍 {t('test')}
                        </button>
                        <button onClick={() => handleEditServer(server)} style={{marginRight: '5px', padding: '5px 10px', fontSize: '0.9em'}}>
                          ✏️ {t('edit')}
                        </button>
                        {server.id !== defaultPBSID && (
                          <button onClick={() => handleSetDefaultPBS(server.id)} style={{marginRight: '5px', padding: '5px 10px', fontSize: '0.9em', backgroundColor: '#fbbf24'}}>
                            ⭐ {t('setAsDefault')}
                          </button>
                        )}
                        <button onClick={() => handleDeletePBSServer(server.id)} style={{padding: '5px 10px', fontSize: '0.9em', backgroundColor: '#ef4444', color: 'white'}}>
                          🗑️ {t('delete')}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          {/* Add/Edit Server Form */}
          <div className="card">
            <h3>{editingServer ? `✏️ ${t('editServer')}` : `➕ ${t('addYourServer')}`}</h3>

            <div className="form-group">
              <label>{t('serverName')}</label>
              <input
                type="text"
                value={serverFormData.name}
                onChange={(e) => setServerFormData({...serverFormData, name: e.target.value})}
                placeholder="SSD Rapide"
              />
            </div>

            {!editingServer && (
              <div className="form-group">
                <label>{t('serverID')}</label>
                <input
                  type="text"
                  value={serverFormData.id}
                  onChange={(e) => setServerFormData({...serverFormData, id: e.target.value})}
                  placeholder="pbs-ssd (laissez vide pour auto-génération)"
                />
              </div>
            )}

            <div className="form-group">
              <label>{t('serverURL')}</label>
              <input
                type="text"
                value={serverFormData.baseurl}
                onChange={(e) => setServerFormData({...serverFormData, baseurl: e.target.value})}
                placeholder="https://pbs-ssd.example.com:8007"
              />
            </div>

            <div className="form-group">
              <label>{t('authID')}</label>
              <input
                type="text"
                value={serverFormData.authid}
                onChange={(e) => setServerFormData({...serverFormData, authid: e.target.value})}
                placeholder="backup@pbs!token-name"
              />
            </div>

            <div className="form-group">
              <label>{t('secret')}</label>
              <input
                type="password"
                value={serverFormData.secret}
                onChange={(e) => setServerFormData({...serverFormData, secret: e.target.value})}
                placeholder={serverFormData.secret_set ? '•••••••• (laisser vide pour conserver le token actuel)' : 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'}
              />
            </div>

            <div className="form-group">
              <label>{t('datastore')}</label>
              <input
                type="text"
                value={serverFormData.datastore}
                onChange={(e) => setServerFormData({...serverFormData, datastore: e.target.value})}
                placeholder="ssd-fast"
              />
            </div>

            <div className="form-group">
              <label>{t('namespace')}</label>
              <input
                type="text"
                value={serverFormData.namespace}
                onChange={(e) => setServerFormData({...serverFormData, namespace: e.target.value})}
                placeholder="clients"
              />
            </div>

            <div className="form-group">
              <label>{t('certFingerprint')}</label>
              <input
                type="text"
                value={serverFormData.certfingerprint}
                onChange={(e) => setServerFormData({...serverFormData, certfingerprint: e.target.value})}
                placeholder="AA:BB:CC:DD:..."
              />
            </div>

            <div className="form-group">
              <label>{t('description')}</label>
              <textarea
                value={serverFormData.description}
                onChange={(e) => setServerFormData({...serverFormData, description: e.target.value})}
                placeholder="Stockage SSD pour backups critiques"
                rows="2"
              />
            </div>

            <div style={{display: 'flex', gap: '10px', marginTop: '20px'}}>
              {editingServer ? (
                <>
                  <button onClick={handleUpdatePBSServer} style={{flex: 1}}>
                    💾 {t('update')}
                  </button>
                  <button onClick={handleCancelEdit} style={{flex: 1, backgroundColor: '#999'}}>
                    ❌ {t('cancel')}
                  </button>
                </>
              ) : (
                <button onClick={handleAddPBSServer} style={{flex: 1}}>
                  ➕ {t('addFirstServer')}
                </button>
              )}
            </div>

            <div className="info-box" style={{marginTop: '20px'}}>
              💡 <strong>{t('tipTitle')}</strong> {t('tipAPIToken')}<br/>
              {t('tipAPITokenPath')}
            </div>
          </div>
            </>
          ) : (
            <>
              {/* Multi-PBS info for users with existing servers */}
              <div className="info-box" style={{marginBottom: '20px'}}>
                💡 <strong>{t('multiPBSInfo')}</strong> {t('multiPBSText')}<br/>
                {t('multiPBSExample')}
              </div>

              {/* Server List */}
              <div className="card" style={{marginBottom: '20px'}}>
                <h3>{t('configuredServers')} ({pbsServers.length})</h3>

                <table style={{width: '100%', marginTop: '15px'}}>
                  <thead>
                    <tr>
                      <th>Nom</th>
                      <th>URL</th>
                      <th>Datastore</th>
                      <th>Statut</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {pbsServers.map(server => (
                      <tr key={server.id}>
                        <td>
                          <strong>{server.name}</strong>
                          {server.id === defaultPBSID && <span style={{marginLeft: '5px', color: '#fbbf24'}}>⭐ {t('default')}</span>}
                          {server.description && <div style={{fontSize: '0.85em', color: '#999'}}>{server.description}</div>}
                        </td>
                        <td>{server.baseurl}</td>
                        <td>{server.datastore}/{server.namespace || '-'}</td>
                        <td>
                          {serverStatus[server.id] === 'testing' && <span style={{color: '#3b82f6'}}>🔄 Test...</span>}
                          {serverStatus[server.id] === 'online' && <span style={{color: '#10b981'}}>🟢 Online</span>}
                          {serverStatus[server.id] === 'offline' && <span style={{color: '#ef4444'}}>🔴 Offline</span>}
                          {!serverStatus[server.id] && <span style={{color: '#999'}}>⚪ Non testé</span>}
                        </td>
                        <td>
                          <button onClick={() => handleTestPBSConnection(server.id)} style={{marginRight: '5px', padding: '5px 10px', fontSize: '0.9em'}}>
                            🔍 Tester
                          </button>
                          <button onClick={() => handleEditServer(server)} style={{marginRight: '5px', padding: '5px 10px', fontSize: '0.9em'}}>
                            ✏️ Modifier
                          </button>
                          {server.id !== defaultPBSID && (
                            <button onClick={() => handleSetDefaultPBS(server.id)} style={{marginRight: '5px', padding: '5px 10px', fontSize: '0.9em', backgroundColor: '#fbbf24'}}>
                              ⭐ Par défaut
                            </button>
                          )}
                          <button onClick={() => handleDeletePBSServer(server.id)} style={{padding: '5px 10px', fontSize: '0.9em', backgroundColor: '#ef4444', color: 'white'}}>
                            🗑️ Supprimer
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Add/Edit Server Form */}
              <div className="card">
                <h3>{editingServer ? `✏️ ${t('editServer')}` : `➕ ${t('addAnotherServer')}`}</h3>

                <div className="form-group">
                  <label>{t('serverName')}</label>
                  <input
                    type="text"
                    value={serverFormData.name}
                    onChange={(e) => setServerFormData({...serverFormData, name: e.target.value})}
                    placeholder="SSD Rapide"
                  />
                </div>

                {!editingServer && (
                  <div className="form-group">
                    <label>{t('serverID')}</label>
                    <input
                      type="text"
                      value={serverFormData.id}
                      onChange={(e) => setServerFormData({...serverFormData, id: e.target.value})}
                      placeholder="pbs-ssd (laissez vide pour auto-génération)"
                    />
                  </div>
                )}

                <div className="form-group">
                  <label>{t('serverURL')}</label>
                  <input
                    type="text"
                    value={serverFormData.baseurl}
                    onChange={(e) => setServerFormData({...serverFormData, baseurl: e.target.value})}
                    placeholder="https://pbs-ssd.example.com:8007"
                  />
                </div>

                <div className="form-group">
                  <label>{t('authID')}</label>
                  <input
                    type="text"
                    value={serverFormData.authid}
                    onChange={(e) => setServerFormData({...serverFormData, authid: e.target.value})}
                    placeholder="backup@pbs!token-name"
                  />
                </div>

                <div className="form-group">
                  <label>{t('secret')}</label>
                  <input
                    type="password"
                    value={serverFormData.secret}
                    onChange={(e) => setServerFormData({...serverFormData, secret: e.target.value})}
                    placeholder={serverFormData.secret_set ? '•••••••• (laisser vide pour conserver le token actuel)' : 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'}
                  />
                </div>

                <div className="form-group">
                  <label>{t('datastore')}</label>
                  <input
                    type="text"
                    value={serverFormData.datastore}
                    onChange={(e) => setServerFormData({...serverFormData, datastore: e.target.value})}
                    placeholder="ssd-fast"
                  />
                </div>

                <div className="form-group">
                  <label>{t('namespace')}</label>
                  <input
                    type="text"
                    value={serverFormData.namespace}
                    onChange={(e) => setServerFormData({...serverFormData, namespace: e.target.value})}
                    placeholder="clients"
                  />
                </div>

                <div className="form-group">
                  <label>{t('certFingerprint')}</label>
                  <input
                    type="text"
                    value={serverFormData.certfingerprint}
                    onChange={(e) => setServerFormData({...serverFormData, certfingerprint: e.target.value})}
                    placeholder="AA:BB:CC:DD:..."
                  />
                </div>

                <div className="form-group">
                  <label>{t('description')}</label>
                  <textarea
                    value={serverFormData.description}
                    onChange={(e) => setServerFormData({...serverFormData, description: e.target.value})}
                    placeholder="Stockage SSD pour backups critiques"
                    rows="2"
                  />
                </div>

                <div style={{display: 'flex', gap: '10px', marginTop: '20px'}}>
                  {editingServer ? (
                    <>
                      <button onClick={handleUpdatePBSServer} style={{flex: 1}}>
                        💾 Mettre à jour
                      </button>
                      <button onClick={handleCancelEdit} style={{flex: 1, backgroundColor: '#999'}}>
                        ❌ Annuler
                      </button>
                    </>
                  ) : (
                    <button onClick={handleAddPBSServer} style={{flex: 1}}>
                      ➕ {t('addServer')}
                    </button>
                  )}
                </div>
              </div>
            </>
          )}

          {status.visible && activeTab === 'servers' && (
            <div className={`status ${status.type} visible`}>{status.message}</div>
          )}
        </div>

        {/* Backup Tab */}
        <div className={`tab-content ${activeTab === 'backup' ? 'active' : ''}`}>
          <h2>{t('backupTitle')}</h2>

          <div className="form-group">
            <label>{t('backupType')}</label>
            <select value={backupType} onChange={(e) => setBackupType(e.target.value)}>
              <option value="directory">📁 {t('backupTypeDirectory')}</option>
              {/* <option value="machine">💾 {t('backupTypeMachine')}</option> */}
            </select>
          </div>

          {/* Backup Mode Toggle */}
          <div className="form-group">
            <label>{t('executionMode')}</label>
            <div style={{display: 'flex', gap: '10px', marginTop: '10px'}}>
              <button
                onClick={() => setBackupMode('oneshot')}
                style={{
                  flex: 1,
                  padding: '10px',
                  backgroundColor: backupMode === 'oneshot' ? '#667eea' : '#e2e8f0',
                  color: backupMode === 'oneshot' ? 'white' : '#4a5568',
                  border: 'none',
                  borderRadius: '8px',
                  cursor: 'pointer',
                  fontWeight: 'bold'
                }}
              >
                <span className="compact-text-long">⚡ {t('oneshotMode')}</span>
                <span className="compact-text-short">⚡ {t('oneshotModeShort')}</span>
              </button>
              <button
                onClick={() => setBackupMode('scheduled')}
                style={{
                  flex: 1,
                  padding: '10px',
                  backgroundColor: backupMode === 'scheduled' ? '#667eea' : '#e2e8f0',
                  color: backupMode === 'scheduled' ? 'white' : '#4a5568',
                  border: 'none',
                  borderRadius: '8px',
                  cursor: 'pointer',
                  fontWeight: 'bold'
                }}
              >
                <span className="compact-text-long">📅 {t('scheduledMode')}</span>
                <span className="compact-text-short">📅 {t('scheduledModeShort')}</span>
              </button>
            </div>
          </div>

          {/* Scheduling Options */}
          {backupMode === 'scheduled' && (
            <div className="card" style={{marginTop: '20px', padding: '20px'}}>
              <h3 style={{marginTop: 0}}>⏰ {t('schedulingConfig')}</h3>

              {editingJobId && (
                <div className="info-box" style={{backgroundColor: '#fff3cd', borderColor: '#ffc107', marginBottom: '15px'}}>
                  ✏️ <strong>{t('editMode')}</strong> - {t('editModeText')}
                </div>
              )}

              <div className="form-group">
                <label>{t('dailyExecutionTime')}</label>
                <input
                  type="time"
                  value={scheduleTime}
                  onChange={(e) => setScheduleTime(e.target.value)}
                  style={{width: '200px', padding: '10px', fontSize: '16px'}}
                />
              </div>

              <div className="form-group">
                <label style={{display: 'flex', alignItems: 'center', gap: '10px', cursor: 'pointer'}}>
                  <input
                    type="checkbox"
                    checked={runAtStartup}
                    onChange={(e) => setRunAtStartup(e.target.checked)}
                    style={{width: '20px', height: '20px', cursor: 'pointer'}}
                  />
                  <span>🚀 {t('runAtStartup')}</span>
                </label>
              </div>

              <div className="info-box" style={{backgroundColor: '#eef2ff'}}>
                💡 {t('schedulingInfo')} <strong>{scheduleTime}</strong>
                {runAtStartup && <><br/>{t('andAtStartup')}</>}
              </div>
            </div>
          )}

          {backupType === 'directory' ? (
            <div className="form-group">
              <label>{t('directoriesToBackup')}</label>
              <textarea
                value={backupDirs}
                onChange={(e) => {
                  setBackupDirs(e.target.value)
                  // Update config.backupdir with first directory for compatibility
                  const dirs = e.target.value.split('\n').map(d => d.trim()).filter(d => d)
                  setConfig({...config, backupdir: dirs[0] || ''})
                }}
                rows="4"
                placeholder="C:\Data&#10;C:\Users&#10;D:\Documents"
              />
            </div>
          ) : (
            <>
              <div className="form-group">
                <label>{t('physicalDisksToBackup')}</label>
                {physicalDisks.length === 0 ? (
                  <div style={{padding: '10px', backgroundColor: '#f8f9fa', borderRadius: '4px'}}>
                    🔍 {t('loadingDisks')}
                  </div>
                ) : (
                  <div style={{display: 'flex', flexDirection: 'column', gap: '8px'}}>
                    {physicalDisks.map(disk => (
                      <label key={disk.path} style={{display: 'flex', alignItems: 'center', gap: '8px'}}>
                        <input
                          type="checkbox"
                          checked={selectedDrives.includes(disk.path)}
                          onChange={(e) => {
                            if (e.target.checked) {
                              setSelectedDrives([...selectedDrives, disk.path])
                            } else {
                              setSelectedDrives(selectedDrives.filter(d => d !== disk.path))
                            }
                          }}
                        />
                        {disk.label}
                      </label>
                    ))}
                  </div>
                )}
              </div>

              <div className="form-group">
                <label>{t('filesToExclude')}</label>
                <textarea
                  value={excludeList}
                  onChange={(e) => setExcludeList(e.target.value)}
                  rows="4"
                  placeholder="*.tmp&#10;*.log&#10;C:\Windows\Temp"
                />
              </div>
            </>
          )}

          <div className="form-group">
            <label>{t('backupID')}</label>
            <input
              type="text"
              value={config['backup-id']}
              onChange={(e) => setConfig({...config, 'backup-id': e.target.value})}
              placeholder={t('backupIDPlaceholder')}
            />
          </div>

          <div className="form-group">
            <label>
              <input
                type="checkbox"
                checked={config.usevss}
                onChange={(e) => setConfig({...config, usevss: e.target.checked})}
              />
              {t('useVSS')}
            </label>
            {config.usevss && systemInfo.mode === 'Standalone' && !systemInfo.is_admin && (
              <div className="info-box" style={{marginTop: '10px', backgroundColor: '#fff3cd', borderColor: '#ffc107'}}>
                ⚠️ <strong>{t('vssAdminRequired')}</strong><br/>
                {t('vssAdminHint')}
              </div>
            )}
            {config.usevss && systemInfo.service_available && (
              <div className="info-box" style={{marginTop: '10px', backgroundColor: '#d1ecf1', borderColor: '#bee5eb'}}>
                ℹ️ <strong>{t('vssServiceAvailable')}</strong><br/>
                {t('vssServiceHint')}
              </div>
            )}
          </div>

          {backupType === 'directory' && backupMode === 'oneshot' && (
            <div className="form-group">
              <label>
                <input
                  type="checkbox"
                  checked={splitFirstBackup}
                  onChange={(e) => setSplitFirstBackup(e.target.checked)}
                />
                {t('splitFirstBackup')}
              </label>
              <div className="info-box" style={{marginTop: '10px', backgroundColor: '#f8f9fa', borderColor: '#dee2e6'}}>
                ℹ️ {t('splitFirstBackupHint')}
              </div>
            </div>
          )}

          {progress > 0 && progress < 100 && (
            <div style={{marginTop: '20px', marginBottom: '20px', padding: '15px', backgroundColor: '#f8f9fa', borderRadius: '8px', border: '1px solid #dee2e6'}}>
              <div style={{display: 'flex', justifyContent: 'space-between', marginBottom: '10px'}}>
                <strong style={{fontSize: '15px'}}>📊 {t('backupProgress')}</strong>
                <span style={{fontSize: '18px', fontWeight: 'bold', color: '#0066cc'}}>{progress}%</span>
              </div>

              <div className="progress" style={{height: '30px', marginBottom: '12px'}}>
                <div
                  className="progress-bar"
                  style={{
                    width: `${progress}%`,
                    fontSize: '14px',
                    lineHeight: '30px',
                    transition: 'width 0.3s ease',
                    fontWeight: 'bold'
                  }}
                >
                  {progress}%
                </div>
              </div>

              <div style={{display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px', marginBottom: '10px'}}>
                {backupStats.eta !== null && (
                  <div style={{fontSize: '13px', color: '#495057'}}>
                    ⏱️ <strong>{t('timeRemaining')}</strong> {Math.floor(backupStats.eta / 60)}m {backupStats.eta % 60}s
                  </div>
                )}
                {backupStats.speed > 0 && (
                  <div style={{fontSize: '13px', color: '#495057'}}>
                    ⚡ <strong>{t('speed')}</strong> {backupStats.speed.toFixed(1)}%/s
                  </div>
                )}
                {backupStats.startTime && (
                  <div style={{fontSize: '13px', color: '#495057'}}>
                    ⏰ <strong>{t('elapsedTime')}</strong> {Math.floor((Date.now() - backupStats.startTime) / 1000)}s
                  </div>
                )}
                {backupStats.bytesDone > 0 && (
                  <div style={{fontSize: '13px', color: '#495057'}}>
                    📦 <strong>Données :</strong> {Math.round(backupStats.bytesDone / 1048576)}
                    {backupStats.bytesTotal > 0 ? ` / ${Math.round(backupStats.bytesTotal / 1048576)}` : ''} MB
                  </div>
                )}
                {(backupStats.newChunks > 0 || backupStats.reusedChunks > 0) && (
                  <div style={{fontSize: '13px', color: '#495057'}}>
                    🧩 <strong>Chunks :</strong> {backupStats.newChunks} new · {backupStats.reusedChunks} reused
                    {backupStats.failedChunks > 0 ? (
                      <span style={{color: '#c0392b', fontWeight: 'bold'}}> · {backupStats.failedChunks} échoués</span>
                    ) : ''}
                  </div>
                )}
                {backupStats.currentDir && (
                  <div style={{fontSize: '13px', color: '#495057', gridColumn: '1 / -1', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'}}>
                    📁 <strong>Dossier :</strong> {backupStats.currentDir}
                  </div>
                )}
              </div>

              {status.message && status.type === 'info' && (
                <div style={{marginTop: '10px', padding: '8px', backgroundColor: '#fff', borderRadius: '4px', fontSize: '13px', color: '#666', border: '1px solid #e9ecef'}}>
                  {status.message}
                </div>
              )}
            </div>
          )}

          <button className="btn" onClick={handleStartBackup} disabled={progress > 0 && progress < 100}>
            {backupMode === 'oneshot'
              ? (progress > 0 && progress < 100 ? `⏳ ${t('backupInProgress')}` : `🚀 ${t('startBackup')}`)
              : (editingJobId ? `✏️ ${t('updateSchedule')}` : `💾 ${t('saveSchedule')}`)
            }
          </button>
          {backupMode === 'oneshot' && (
            <button className="btn btn-secondary" onClick={() => setProgress(0)} disabled={progress === 0}>{t('stopBackup')}</button>
          )}
          {backupMode === 'scheduled' && editingJobId && (
            <button className="btn btn-secondary" onClick={() => {
              setEditingJobId(null)
              setScheduleTime('02:00')
              setRunAtStartup(false)
              setBackupDirs('')
              setExcludeList('')
              setBackupType('directory')
              setActiveTab('scheduled')
              showStatus(`✖️ ${t('statusEditCancelled')}`, 'info')
            }}>
              ✖️ {t('cancel')}
            </button>
          )}

          {/* Scheduled Jobs List */}
          {backupMode === 'scheduled' && scheduledJobs.length > 0 && (
            <div className="card" style={{marginTop: '30px'}}>
              <h3 style={{marginTop: 0}}>📅 {t('scheduledJobs')}</h3>
              {scheduledJobs.map(job => (
                <div key={job.id} style={{
                  padding: '15px',
                  marginBottom: '10px',
                  backgroundColor: '#f8f9fa',
                  borderRadius: '8px',
                  border: '1px solid #dee2e6'
                }}>
                  <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
                    <div>
                      <strong>{job.name}</strong>
                      <div style={{fontSize: '14px', color: '#6c757d', marginTop: '5px'}}>
                        ⏰ {job.scheduleTime} {job.runAtStartup && '• 🚀 Au démarrage'}
                      </div>
                      <div style={{fontSize: '13px', color: '#6c757d', marginTop: '3px'}}>
                        📁 {job.backupDirs.join(', ')}
                      </div>
                    </div>
                    <div style={{display: 'flex', gap: '10px'}}>
                      <button
                        className="btn"
                        style={{padding: '8px 15px', fontSize: '14px'}}
                        onClick={() => {
                          // Load job data into form for editing
                          setEditingJobId(job.id)
                          setBackupMode('scheduled')
                          setScheduleTime(job.scheduleTime)
                          setRunAtStartup(job.runAtStartup)
                          setBackupDirs(job.backupDirs.join('\n'))
                          setConfig({...config, 'backup-id': job.backupId, usevss: job.useVSS})
                          setBackupType(job.backupType)
                          setExcludeList(job.excludeList.join('\n'))
                          // Switch to backup tab to show the form
                          setActiveTab('backup')
                          showStatus(`✏️ ${t('editModeInfo')}`, 'info')
                          window.scrollTo({top: 0, behavior: 'smooth'})
                        }}
                      >
                        ✏️ {t('editJob')}
                      </button>
                      <button
                        className="btn btn-secondary"
                        style={{padding: '8px 15px', fontSize: '14px'}}
                        onClick={async () => {
                          try {
                            await DeleteScheduledJob(job.id)
                            setScheduledJobs(scheduledJobs.filter(j => j.id !== job.id))
                            showStatus(t('statusJobDeleted'), 'success')
                            // Cancel edit mode if deleting the job being edited
                            if (editingJobId === job.id) {
                              setEditingJobId(null)
                            }
                          } catch (err) {
                            showStatus(`❌ Erreur: ${err}`, 'error')
                          }
                        }}
                      >
                        🗑️ {t('deleteJob')}
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Job History */}
          {jobHistory.length > 0 && (
            <div className="card" style={{marginTop: '30px'}}>
              <h3 style={{marginTop: 0}}>📜 {t('backupHistory')}</h3>
              <div style={{maxHeight: '400px', overflowY: 'auto'}}>
                {jobHistory.slice(0, 6).map(job => (
                  <div key={job.id} style={{
                    padding: '15px',
                    marginBottom: '10px',
                    backgroundColor: job.status === 'success' ? '#d4edda' : job.status === 'failed' ? '#f8d7da' : '#fff3cd',
                    borderRadius: '8px',
                    border: `1px solid ${job.status === 'success' ? '#c3e6cb' : job.status === 'failed' ? '#f5c6cb' : '#ffeaa7'}`
                  }}>
                    <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
                      <div style={{flex: 1}}>
                        <div style={{display: 'flex', alignItems: 'center', gap: '10px'}}>
                          <span style={{fontSize: '20px'}}>
                            {job.status === 'success' ? '✅' : job.status === 'failed' ? '❌' : '⏳'}
                          </span>
                          <strong>{job.name}</strong>
                        </div>
                        <div style={{fontSize: '13px', color: '#6c757d', marginTop: '5px', marginLeft: '30px'}}>
                          🕐 {new Date(job.timestamp).toLocaleString('fr-FR')}
                        </div>
                        {job.message && (
                          <div style={{fontSize: '13px', color: '#495057', marginTop: '5px', marginLeft: '30px'}}>
                            💬 {job.message}
                          </div>
                        )}
                      </div>
                      {job.status === 'failed' && (
                        <button
                          className="btn"
                          style={{padding: '8px 15px', fontSize: '14px'}}
                          onClick={() => {
                            // Re-run failed job
                            setBackupDirs(job.backupDirs.join('\n'))
                            setConfig({...config, 'backup-id': job.backupId, usevss: job.useVSS})
                            showStatus(t('configLoaded'), 'success')
                            window.scrollTo({top: 0, behavior: 'smooth'})
                          }}
                        >
                          🔄 {t('rerun')}
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {status.visible && activeTab === 'backup' && (
            <div className={`status ${status.type} visible`}>{status.message}</div>
          )}
        </div>

        {/* Restore Tab */}
        <div className={`tab-content ${activeTab === 'restore' ? 'active' : ''}`}>
          <h2>{t('restoreTitle')}</h2>

          {/* BETA Warning */}
          <div style={{
            backgroundColor: '#FEF3C7',
            border: '2px solid #F59E0B',
            borderRadius: '8px',
            padding: '12px',
            marginBottom: '20px',
            color: '#92400E'
          }}>
            <strong>⚠️ {t('restoreBetaTitle')}</strong>
            <p style={{margin: '8px 0 0 0', fontSize: '14px'}}>
              {t('restoreBetaIntro')}
              <br/>✅ {t('restoreBetaFilesDirs')}
              <br/>✅ {t('restoreBetaSelective')}
              <br/>✅ {t('restoreBetaTimestamps')}
              <br/>❌ {t('restoreBetaACLs')}
            </p>
          </div>

          {/* PBS server selector + Backup ID */}
          <div style={{display: 'flex', gap: '15px', flexWrap: 'wrap'}}>
            <div className="form-group" style={{flex: '1 1 240px'}}>
              <label>{t('restorePBSServer')}</label>
              <select
                value={restorePBSID}
                onChange={(e) => {
                  setRestorePBSID(e.target.value)
                  setShowSnapshots(false)
                  setSelectedSnapshot(null)
                  setSnapshotEntries([])
                }}
              >
                {pbsServers.length === 0 && <option value="">{t('noPBSServer')}</option>}
                {pbsServers.map(s => (
                  <option key={s.id} value={s.id}>
                    {s.name} {s.id === defaultPBSID ? '⭐' : ''}
                  </option>
                ))}
              </select>
            </div>
            <div className="form-group" style={{flex: '2 1 320px'}}>
              <label>{t('backupIDToRestore')}</label>
              <input
                type="text"
                value={restoreBackupId || hostname}
                onChange={(e) => setRestoreBackupId(e.target.value)}
                placeholder={hostname || "hostname ou ID personnalisé"}
              />
            </div>
          </div>

          <button className="btn" onClick={handleListSnapshots}>📋 {t('listSnapshots')}</button>
          <button
            className="btn"
            type="button"
            onClick={() => setShowSearch(v => !v)}
            style={{marginLeft: '8px'}}
          >
            🔎 {t('searchTitle')}
          </button>

          {showSearch && (
            <div style={{
              marginTop: '16px',
              padding: '14px',
              border: '1px solid #cbd5e1',
              borderRadius: '8px',
              backgroundColor: '#f8fafc'
            }}>
              <p style={{fontSize: '13px', color: '#64748b', marginTop: 0}}>
                {t('searchHint').replace('{prefix}', (restoreBackupId || hostname || '?'))}
              </p>

              <div style={{display: 'flex', gap: '12px', flexWrap: 'wrap', alignItems: 'flex-end'}}>
                <div className="form-group" style={{flex: '2 1 280px', margin: 0}}>
                  <label>{t('searchQueryLabel')}</label>
                  <input
                    type="text"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    placeholder={t('searchQueryPlaceholder')}
                    onKeyDown={(e) => { if (e.key === 'Enter' && !searchRunning) handleSearch() }}
                  />
                </div>
                <div className="form-group" style={{flex: '1 1 160px', margin: 0}}>
                  <label>{t('searchModeLabel')}</label>
                  <select value={searchMode} onChange={(e) => setSearchMode(e.target.value)}>
                    <option value="name">{t('searchModeName')}</option>
                    <option value="regex">{t('searchModeRegex')}</option>
                    <option value="path">{t('searchModePath')}</option>
                  </select>
                </div>
              </div>

              <div style={{display: 'flex', gap: '12px', flexWrap: 'wrap', alignItems: 'flex-end', marginTop: '10px'}}>
                <div className="form-group" style={{flex: '1 1 150px', margin: 0}}>
                  <label>{t('searchFrom')}</label>
                  <input type="date" value={searchFrom} onChange={(e) => setSearchFrom(e.target.value)} />
                </div>
                <div className="form-group" style={{flex: '1 1 150px', margin: 0}}>
                  <label>{t('searchTo')}</label>
                  <input type="date" value={searchTo} onChange={(e) => setSearchTo(e.target.value)} />
                </div>
                <label style={{display: 'flex', alignItems: 'center', gap: '8px', flex: '2 1 260px', fontSize: '14px'}}>
                  <input
                    type="checkbox"
                    checked={searchAssembleMissing}
                    onChange={(e) => setSearchAssembleMissing(e.target.checked)}
                  />
                  {t('searchAssembleMissing')}
                </label>
              </div>

              <div style={{marginTop: '12px', display: 'flex', gap: '8px', alignItems: 'center'}}>
                <button className="btn" type="button" onClick={handleSearch} disabled={searchRunning}>
                  {searchRunning ? `⏳ ${t('searching')}` : `🔎 ${t('searchButton')}`}
                </button>
                {searchRunning && (
                  <button className="btn" type="button" onClick={handleCancelSearch} style={{backgroundColor: '#ef4444'}}>
                    ✖ {t('cancel')}
                  </button>
                )}
                {searchRunning && (
                  <span style={{fontSize: '13px', color: '#64748b'}}>
                    {searchProgress.percent}% — {searchProgress.message}
                  </span>
                )}
              </div>

              {searchResult && (
                <div style={{marginTop: '14px'}}>
                  <p style={{fontSize: '13px', color: '#334155', margin: '0 0 6px 0'}}>
                    {t('searchSummary')
                      .replace('{hits}', searchResult.hits ? searchResult.hits.length : 0)
                      .replace('{searched}', searchResult.snapshots_searched || 0)
                      .replace('{assembled}', searchResult.snapshots_assembled || 0)}
                    {searchResult.truncated ? ` ⚠️ ${t('searchTruncated').replace('{max}', 5000)}` : ''}
                    {searchResult.cancelled ? ` ⚠️ ${t('searchCancelled')}` : ''}
                  </p>
                  {searchResult.snapshots_in_range === 0 && (
                    <p style={{fontSize: '12px', color: '#b45309', margin: '0 0 8px 0'}}>
                      💡 {t('searchNoSnapshotsInRange')}
                    </p>
                  )}
                  {(searchResult.snapshots_skipped > 0 && !searchAssembleMissing) && (
                    <p style={{fontSize: '12px', color: '#b45309', margin: '0 0 8px 0'}}>
                      ⚠️ {t('searchSkippedWarning').replace('{n}', searchResult.snapshots_skipped)}
                    </p>
                  )}

                  <div style={{
                    border: '1px solid #cbd5e1',
                    borderRadius: '8px',
                    maxHeight: '320px',
                    overflowY: 'auto',
                    backgroundColor: '#fff'
                  }}>
                    {(!searchResult.hits || searchResult.hits.length === 0) ? (
                      <p style={{padding: '12px', color: '#718096'}}>{t('searchNoResults')}</p>
                    ) : (
                      searchResult.hits.map((hit, idx) => (
                        <div
                          key={`${hit.backup_id}-${hit.snapshot_time}-${hit.path}-${idx}`}
                          title={hit.origin_path || hit.path}
                          style={{
                            display: 'flex', alignItems: 'center', gap: '10px',
                            padding: '6px 10px', borderBottom: '1px solid #f1f5f9', fontSize: '13px'
                          }}
                        >
                          <span style={{fontSize: '15px'}}>{hit.is_dir ? '📁' : '📄'}</span>
                          <div style={{flex: 1, minWidth: 0}}>
                            <div style={{fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'}}>
                              {hit.path.split('/').pop() || hit.path}
                            </div>
                            <div style={{color: '#64748b', fontSize: '12px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'}}>
                              {hit.origin_path || hit.path}
                            </div>
                            <div style={{color: '#94a3b8', fontSize: '11px'}}>
                              {hit.backup_id} · {new Date(hit.snapshot_time * 1000).toLocaleString()}
                              {hit.is_dir ? '' : ` · ${formatBytes(hit.size)}`}
                            </div>
                          </div>
                          <button
                            className="btn"
                            type="button"
                            onClick={() => handleRestoreHit(hit)}
                            style={{padding: '4px 10px', fontSize: '12px'}}
                          >
                            {t('searchRestoreThis')}
                          </button>
                        </div>
                      ))
                    )}
                  </div>
                </div>
              )}
            </div>
          )}

          {showSnapshots && (
            <div style={{marginTop: '20px'}}>
              <h3>{t('availableSnapshots')}</h3>
              <div className="grid">
                {snapshots.length === 0 ? (
                  <p style={{color: '#718096'}}>{t('noSnapshotFound')}</p>
                ) : (
                  snapshots.map((snap, idx) => {
                    const isActive = selectedSnapshot && selectedSnapshot.id === snap.id && selectedSnapshot.backup_id === snap.backup_id
                    return (
                      <div
                        key={idx}
                        className="card"
                        style={{
                          cursor: 'pointer',
                          border: isActive ? '2px solid #2563eb' : undefined,
                          backgroundColor: isActive ? '#eff6ff' : undefined
                        }}
                        onClick={() => handleSelectSnapshot(snap)}
                      >
                        <h3>📸 {snap.time}</h3>
                        <p style={{color: '#718096', fontSize: '14px', marginTop: '5px'}}>
                          {snap.backup_id}<br/>
                          {t('typeLabel')}: {snap.backup_type || 'N/A'}
                        </p>
                        <button className="btn" style={{marginTop: '10px', width: '100%'}}>
                          {isActive ? `✓ ${t('snapshotSelected')}` : t('selectSnapshot')}
                        </button>
                      </div>
                    )
                  })
                )}
              </div>
            </div>
          )}

          {/* Backup origin banner — driven by the .nimbus_backup_meta.json sidecar */}
          {selectedSnapshot && snapshotMeta && (
            <div style={{
              marginTop: '20px',
              padding: '10px 14px',
              border: '1px solid #c7d2fe',
              backgroundColor: '#eef2ff',
              borderRadius: '8px',
              fontSize: '13px',
              color: '#1e293b',
              display: 'grid',
              gridTemplateColumns: 'auto 1fr',
              columnGap: '12px',
              rowGap: '4px'
            }}>
              <strong>{t('metaOriginalPath') || 'Source d\'origine'}</strong>
              <span style={{fontFamily: 'monospace'}}>{snapshotMeta.original_path || '—'}</span>
              <strong>{t('metaHostname') || 'Machine'}</strong>
              <span>{snapshotMeta.hostname || '—'}{snapshotMeta.os ? ` (${snapshotMeta.os})` : ''}{snapshotMeta.vss_used ? ' · VSS' : ''}</span>
              <strong>{t('metaBackupTime') || 'Sauvegardé le'}</strong>
              <span>{snapshotMeta.backup_time || '—'}{snapshotMeta.client_version ? ` · client ${snapshotMeta.client_version}` : ''}</span>
            </div>
          )}

          {/* Snapshot navigation tree */}
          {selectedSnapshot && (
            <div style={{marginTop: '24px'}}>
              <div style={{display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap'}}>
                <h3 style={{margin: 0}}>📂 {t('snapshotContents')} — {selectedSnapshot.time}</h3>
                <button
                  className="btn"
                  type="button"
                  onClick={handleReloadSnapshot}
                  title={t('reloadTreeHint') || 'Bypass local cache and re-download the snapshot tree'}
                  style={{padding: '4px 10px', fontSize: '13px'}}
                >
                  🔄 {t('reloadTree') || 'Recharger'}
                </button>
              </div>
              <p style={{fontSize: '13px', color: '#64748b', marginBottom: '8px', marginTop: '6px'}}>
                {t('treeHint')}
              </p>
              <div style={{
                border: '1px solid #cbd5e1',
                borderRadius: '8px',
                maxHeight: '360px',
                overflowY: 'auto',
                backgroundColor: '#fff'
              }}>
                {snapshotEntries.length === 0 ? (
                  <p style={{padding: '12px', color: '#718096'}}>{t('loadingOrEmpty')}</p>
                ) : (
                  (() => {
                    const tree = buildTree(snapshotEntries)
                    const roots = tree.get('') || []
                    return roots.map(e => renderTreeNode(e, tree, 0))
                  })()
                )}
              </div>
              <p style={{marginTop: '6px', fontSize: '12px', color: '#64748b'}}>
                {selectedPaths.size === 0
                  ? t('selectionEmptyAllSize').replace('{size}', formatBytes(selectionBytes))
                  : t('selectionCountSize')
                      .replace('{n}', selectedPaths.size)
                      .replace('{size}', formatBytes(selectionBytes))}
              </p>
            </div>
          )}

          {/* Restore mode picker + destination + options + restore button */}
          {selectedSnapshot && (() => {
            const blocker = inPlaceBlocker()
            const isInPlace = restoreMode === 'original'
            const crossHost = isInPlace && crossHostMismatch()
            const blockerTooltip = blocker ? (t(blocker) || '') : ''
            return (
              <div style={{marginTop: '20px'}}>
                {/* Mode picker */}
                <div style={{
                  display: 'flex',
                  flexWrap: 'wrap',
                  gap: '20px',
                  marginBottom: '14px',
                  padding: '10px 14px',
                  border: '1px solid #cbd5e1',
                  borderRadius: '8px',
                  backgroundColor: '#f8fafc'
                }}>
                  <label
                    title={blockerTooltip}
                    style={{
                      display: 'flex', alignItems: 'center', gap: '6px',
                      opacity: blocker ? 0.5 : 1,
                      cursor: blocker ? 'not-allowed' : 'pointer'
                    }}
                  >
                    <input
                      type="radio"
                      name="restoreMode"
                      value="original"
                      checked={restoreMode === 'original'}
                      disabled={!!blocker}
                      onChange={() => setRestoreMode('original')}
                    />
                    <strong>{t('restoreModeInPlace') || 'Restaurer in-place'}</strong>
                    {blocker && <span style={{fontSize: '11px', color: '#dc2626'}}> ({blockerTooltip})</span>}
                  </label>
                  <label style={{display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer'}}>
                    <input
                      type="radio"
                      name="restoreMode"
                      value="alternate"
                      checked={restoreMode !== 'original'}
                      onChange={() => setRestoreMode('alternate_abs')}
                    />
                    <strong>{t('restoreModeAlternate') || 'Restaurer vers un autre emplacement'}</strong>
                  </label>
                </div>

                {/* In-place: warning banner + cross-host override */}
                {isInPlace && (
                  <div style={{
                    marginBottom: '14px',
                    padding: '10px 14px',
                    border: '1px solid #fca5a5',
                    backgroundColor: '#fef2f2',
                    borderRadius: '8px',
                    fontSize: '13px'
                  }}>
                    <div style={{color: '#991b1b', marginBottom: '6px'}}>
                      ⚠️ {t('inPlaceWarning').replace('{path}', snapshotMeta?.original_path || '?')}
                    </div>
                    {crossHost && (
                      <label style={{display: 'flex', alignItems: 'center', gap: '6px', color: '#7c2d12'}}>
                        <input
                          type="checkbox"
                          checked={restoreAllowCrossHost}
                          onChange={(e) => setRestoreAllowCrossHost(e.target.checked)}
                        />
                        {t('crossHostOverride')
                          .replace('{src}', snapshotMeta?.hostname || '?')
                          .replace('{dst}', hostname || '?')}
                      </label>
                    )}
                  </div>
                )}

                {/* Alternate: destination + keep-tree toggle */}
                {!isInPlace && (
                  <>
                    <div className="form-group">
                      <label>{t('destinationPath')}</label>
                      <div style={{display: 'flex', gap: '8px'}}>
                        <input
                          type="text"
                          value={restoreDestPath}
                          onChange={(e) => setRestoreDestPath(e.target.value)}
                          placeholder="C:\Restore"
                          style={{flex: 1}}
                        />
                        <button className="btn" onClick={handleBrowseRestoreDest} type="button">
                          📁 {t('browse')}
                        </button>
                      </div>
                    </div>
                    <label style={{display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '12px'}}>
                      <input
                        type="checkbox"
                        checked={restoreKeepTree}
                        onChange={(e) => setRestoreKeepTree(e.target.checked)}
                      />
                      {t('keepTreeLabel') || 'Conserver l\'arborescence d\'origine'}
                      <span style={{fontSize: '12px', color: '#64748b'}}>
                        {restoreKeepTree
                          ? (t('keepTreeOnHint') || '(dest/Users/alice/doc.txt)')
                          : (t('keepTreeOffHint') || '(dest/doc.txt — recommandé pour un fichier seul)')}
                      </span>
                    </label>
                  </>
                )}

                <div style={{display: 'flex', flexWrap: 'wrap', gap: '12px', marginBottom: '12px'}}>
                  <label style={{display: 'flex', alignItems: 'center', gap: '6px', opacity: isInPlace ? 0.5 : 1}}
                         title={isInPlace ? (t('overwriteForcedInPlace') || '') : ''}>
                    <input
                      type="checkbox"
                      checked={isInPlace ? true : restoreOptions.overwrite}
                      disabled={isInPlace}
                      onChange={(e) => setRestoreOptions(o => ({...o, overwrite: e.target.checked}))}
                    />
                    {t('optionOverwrite')}
                  </label>
                  <label style={{display: 'flex', alignItems: 'center', gap: '6px'}}>
                    <input
                      type="checkbox"
                      checked={restoreOptions.timestamps}
                      onChange={(e) => setRestoreOptions(o => ({...o, timestamps: e.target.checked}))}
                    />
                    {t('optionTimestamps')}
                  </label>
                  <label style={{display: 'flex', alignItems: 'center', gap: '6px', opacity: 0.5}} title={t('optionComingSoon')}>
                    <input type="checkbox" disabled checked={false} />
                    {t('optionACLs')} <span style={{fontSize: '11px'}}>({t('comingSoon')})</span>
                  </label>
                  <label style={{display: 'flex', alignItems: 'center', gap: '6px', opacity: 0.5}} title={t('optionComingSoon')}>
                    <input type="checkbox" disabled checked={false} />
                    {t('optionADS')} <span style={{fontSize: '11px'}}>({t('comingSoon')})</span>
                  </label>
                </div>

                <button
                  className="btn btn-primary"
                  onClick={handleRestoreSnapshot}
                  disabled={restoreLoading || (isInPlace && crossHost && !restoreAllowCrossHost)}
                >
                  {restoreLoading ? `⏳ ${t('restoring')}` : `▶️ ${t('restore')}`}
                </button>

                {restoreLoading && (
                  <div style={{marginTop: '12px'}}>
                    <div style={{height: '8px', backgroundColor: '#e2e8f0', borderRadius: '4px', overflow: 'hidden'}}>
                      <div style={{
                        height: '100%',
                        width: `${restoreProgress}%`,
                        backgroundColor: '#2563eb',
                        transition: 'width 0.3s ease'
                      }}/>
                    </div>
                    <p style={{textAlign: 'center', fontSize: '13px', color: '#64748b', marginTop: '4px'}}>
                      {restoreProgress}%
                    </p>
                  </div>
                )}
              </div>
            )
          })()}

          <div className="info-box" style={{marginTop: '20px'}}>
            💡 <strong>{t('restoreInfo')}</strong> {t('restoreInfoText')}<br/>
            {t('restoreInfoText2')}
          </div>

          {status.visible && activeTab === 'restore' && (
            <div className={`status ${status.type} visible`}>{status.message}</div>
          )}
        </div>

        {/* About Tab */}
        <div className={`tab-content ${activeTab === 'about' ? 'active' : ''}`}>
          <h2 style={{textAlign: 'center'}}>{t('aboutTitle')}</h2>

          <img
            src="https://nimbus.rdem-systems.com/logo.webp"
            alt="Nimbus Backup"
            className="logo"
            onError={(e) => e.target.style.display = 'none'}
          />

          <div style={{textAlign: 'center', marginTop: '30px'}}>
            <h3>Nimbus Backup</h3>
            <p style={{color: '#718096', margin: '10px 0'}}>{t('version')} {appVersion}</p>

            {/* Upsell CTA */}
            <div style={{margin: '20px 0'}}>
              <a
                href={`${t('chooseBackupUrl')}?utm_source=NimbusGui&utm_medium=tooling&utm_campaign=version-${appVersion}&utm_content=version-${appVersion}`}
                target="_blank"
                rel="noopener noreferrer"
                style={{
                  display: 'inline-block',
                  padding: '12px 24px',
                  backgroundColor: '#667eea',
                  color: 'white',
                  textDecoration: 'none',
                  borderRadius: '8px',
                  fontWeight: 'bold',
                  transition: 'background-color 0.3s'
                }}
                onMouseEnter={(e) => e.target.style.backgroundColor = '#5568d3'}
                onMouseLeave={(e) => e.target.style.backgroundColor = '#667eea'}
              >
                📦 {t('orderStorageCTA')}
              </a>
            </div>

            <div className="grid" style={{marginTop: '30px', textAlign: 'left'}}>
              <div className="card">
                <h3>✅ {t('features')}</h3>
                <ul style={{lineHeight: 2, marginLeft: '20px'}}>
                  <li>{t('featuresList.directories')}</li>
                  <li>{t('featuresList.machine')}</li>
                  <li>{t('featuresList.restore')}</li>
                  <li>{t('featuresList.vss')}</li>
                  <li>{t('featuresList.dedup')}</li>
                  <li>{t('featuresList.modern')}</li>
                </ul>
              </div>

              <div className="card">
                <h3>🚀 {t('technology')}</h3>
                <ul style={{lineHeight: 2, marginLeft: '20px'}}>
                  <li>{t('techList.wails')}</li>
                  <li>{t('techList.performance')}</li>
                  <li>{t('techList.interface')}</li>
                  <li>{t('techList.logs')}</li>
                  <li>{t('techList.nogpu')}</li>
                </ul>
              </div>
            </div>

            <p style={{marginTop: '30px'}}>
              <strong>{t('copyright')}</strong><br/>
              <a href="https://nimbus.rdem-systems.com" style={{color: '#667eea'}}>nimbus.rdem-systems.com</a>
            </p>

            <p style={{marginTop: '20px', color: '#718096', fontSize: '12px'}}>
              {t('basedOn')}<br/>
              {t('techStack')}
            </p>
          </div>
        </div>
      </div>
    </>
  )
}

export default App
