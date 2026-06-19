# Integration Guide - HTTP API Service/GUI

## 📋 Architecture

```
GUI (Wails)
    ↓
ModeDetector → IsServiceAvailable?
    ├─ YES → api.Client (HTTP) → Service
    └─ NO  → Direct execution (current code)
```

## 🔧 Étape 1: Modifier `service.go`

Ajouter le serveur HTTP dans le service Windows:

```go
package main

import (
    "github.com/tizbac/proxmoxbackupclient_go/gui/api"
    "github.com/kardianos/service"
    "log"
)

type NimbusService struct {
    app    *App
    server *api.Server
}

func (s *NimbusService) Start(svc service.Service) error {
    s.app = &App{}

    // Start HTTP API server in goroutine
    s.server = api.NewServer("127.0.0.1:18765", s.app)
    go func() {
        if err := s.server.Start(); err != nil {
            writeDebugLog("API server error: " + err.Error())
        }
    }()

    // Start scheduler
    go s.run()
    return nil
}

func (s *NimbusService) run() {
    s.app.CleanupAbandonedJobs()
    s.app.StartScheduler()

    for {
        time.Sleep(1 * time.Minute)
    }
}

func (s *NimbusService) Stop(svc service.Service) error {
    // Cleanup
    return nil
}
```

**Important:** `App` doit implémenter l'interface `api.BackupHandler`:
```go
// Verify App implements BackupHandler
var _ api.BackupHandler = (*App)(nil)
```

## 🔧 Étape 2: Modifier `main.go` (GUI)

Ajouter la détection de mode au démarrage:

```go
package main

import (
    "github.com/tizbac/proxmoxbackupclient_go/gui/api"
    "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
    ctx        context.Context
    apiClient  *api.Client
    mode       api.ExecutionMode
    // ... existing fields
}

func (a *App) startup(ctx context.Context) {
    a.ctx = ctx

    // Detect execution mode
    detector := api.NewModeDetector()
    a.mode = detector.DetectMode()
    a.apiClient = api.NewClient()

    // Log mode
    writeDebugLog("Running in: " + a.mode.String())

    // Show mode info to user
    runtime.EventsEmit(ctx, "mode-detected", api.GetModeDescription(a.mode))

    // ... rest of startup code
}
```

## 🔧 Étape 3: Modifier `StartBackup()` avec routing

Remplacer la logique actuelle par un router:

```go
func (a *App) StartBackup(backupType, backupID string, backupDirs, driveLetters []string, useVSS bool) error {
    writeDebugLog("StartBackup called with mode: " + a.mode.String())

    // Check VSS warning conditions
    if shouldWarn, msg := api.ShouldWarnVSS(useVSS, a.mode, isAdmin()); shouldWarn {
        return fmt.Errorf(msg)
    }

    // Route based on mode
    switch a.mode {
    case api.ModeService:
        return a.startBackupViaService(backupType, backupID, backupDirs, driveLetters, useVSS)
    case api.ModeStandalone:
        return a.startBackupDirect(backupType, backupID, backupDirs, driveLetters, useVSS)
    default:
        return fmt.Errorf("unknown execution mode")
    }
}

// New: Start backup via HTTP service
func (a *App) startBackupViaService(backupType, backupID string, backupDirs, driveLetters []string, useVSS bool) error {
    req := &api.BackupRequest{
        BackupType:   backupType,
        BackupID:     backupID,
        BackupDirs:   backupDirs,
        DriveLetters: driveLetters,
        UseVSS:       useVSS,
    }

    resp, err := a.apiClient.StartBackup(req)
    if err != nil {
        return fmt.Errorf("failed to start backup via service: %w", err)
    }

    if !resp.Success {
        return fmt.Errorf("backup failed: %s", resp.Error)
    }

    writeDebugLog("Backup started via service: " + resp.Message)
    return nil
}

// Existing: Direct backup execution (keep current code)
func (a *App) startBackupDirect(backupType, backupID string, backupDirs, driveLetters []string, useVSS bool) error {
    // KEEP EXISTING BACKUP CODE HERE
    // This is the current StartBackup() logic

    // Validate BackupID
    if backupID == "" {
        return fmt.Errorf("backup ID requis")
    }

    // ... rest of current backup code
}
```

## 🔧 Étape 4: Frontend (React/Wails)

Afficher le mode dans l'UI:

```typescript
// frontend/src/App.tsx
useEffect(() => {
    // Listen for mode detection
    window.runtime.EventsOn("mode-detected", (description: string) => {
        console.log("Execution mode:", description);
        // Optionally show to user
        setModeInfo(description);
    });
}, []);
```

Optionnellement, afficher une badge dans l'UI:
```tsx
<div className="mode-badge">
    {mode === "Service" ? "🟢 Service Mode" : "⚠️ Standalone"}
</div>
```

## 🔧 Étape 5: Tester

### Test Mode Service
```powershell
# 1. Installer le MSI (lance le service)
msiexec /i NimbusBackup.msi /qn

# 2. Vérifier service
Get-Service NimbusService

# 3. Lancer GUI
NimbusBackup.exe

# 4. Check logs
# Devrait voir: "Running in: Service Mode"
```

### Test Mode Standalone
```powershell
# 1. Stop service
Stop-Service NimbusService

# 2. Lancer GUI
NimbusBackup.exe

# 3. Check logs
# Devrait voir: "Running in: Standalone Mode"

# 4. Tester VSS
# Sans admin → Warning
# Avec admin → OK
```

## 🚨 Points d'attention

1. **Thread Safety**: Le service peut recevoir des requêtes HTTP concurrentes
   - Ajouter mutex si nécessaire dans App

2. **Timeout**: GUI attend max 30s pour réponse HTTP
   - Backups longs → retourner immédiatement avec JobID

3. **Logs**: Service et GUI loguent séparément
   - Service: `C:\ProgramData\Nimbus\logs\service.log`
   - GUI: `C:\ProgramData\Nimbus\logs\gui.log`

4. **Firewall**: Localhost:18765 ne devrait pas être bloqué
   - Bind 127.0.0.1 uniquement (pas 0.0.0.0)

## ✅ Checklist d'intégration

- [ ] service.go: Start HTTP server
- [ ] main.go: Add mode detection
- [ ] main.go: Router StartBackup()
- [ ] main.go: Split startBackupViaService() / startBackupDirect()
- [ ] App: Implement api.BackupHandler interface
- [ ] Frontend: Display mode badge (optionnel)
- [ ] Test: Mode Service (avec MSI)
- [ ] Test: Mode Standalone (sans service)
- [ ] Test: VSS warning logic
- [ ] Doc: Update README with modes

---

**Note:** Cette architecture permet d'ajouter facilement le mode Enterprise (HTTP distant) plus tard!
