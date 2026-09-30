const { RequestQuit, MinimizeToTray } = window.go?.main?.App || {}

// Shown when the titlebar X is clicked, instead of silently minimizing to
// tray with no visible way back to a real quit. See main.go's beforeClose.
export default function ClosePromptModal({ onClose }) {
  const handleExit = () => {
    RequestQuit?.()
  }
  const handleMinimize = () => {
    MinimizeToTray?.()
    onClose()
  }

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
        style={{ maxWidth: '420px', width: '90%', margin: 0 }}
        onClick={(e) => e.stopPropagation()}
      >
        <h3 style={{ marginTop: 0 }}>Close Proxmox Backup Client?</h3>
        <p style={{ color: '#4a5568', lineHeight: 1.5 }}>
          <strong>Minimize to Tray</strong> keeps the app running in the background — scheduled
          Backup Sets still fire normally. <strong>Exit</strong> closes it completely, and no
          scheduled backups will run until you open the app again.
        </p>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '16px' }}>
          <button className="btn btn-secondary" onClick={handleExit}>Exit</button>
          <button className="btn" onClick={handleMinimize}>Minimize to Tray</button>
        </div>
      </div>
    </div>
  )
}
