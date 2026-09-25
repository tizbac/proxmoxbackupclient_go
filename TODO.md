# Proxmox Backup Client - TODO

## ✅ RÉCEMMENT COMPLÉTÉES (v0.1.78-v0.1.92)

### ~~Fix Bug Config Service~~ ✅ RÉSOLU (v0.1.81)
- [x] ~~HTTP API existe~~ ✓ (`gui/api/server.go`)
- [x] ~~Service Windows fonctionne~~ ✓ (`gui/service.go`)
- [x] ~~ReloadConfig avant backup~~ ✓ (v0.1.81)
- [x] ~~Scheduled backups~~ ✓ (v0.1.85+)
- [x] ~~System tray~~ ✓ (v0.1.80+)
- [x] ~~MSI installer~~ ✓ (v0.1.70+)
- [x] ~~Logs séparés GUI/Service~~ ✓ (v0.1.78+)

---

## ✅ RÉCEMMENT COMPLÉTÉES (2026-03-23)

### ~~VSS Cleanup au démarrage~~ ✅ RÉSOLU
- [x] ~~Fonction cleanupVSS() Windows~~ ✓ (`service/vss_cleanup_windows.go`)
- [x] ~~Appel au démarrage du service~~ ✓ (`service/main.go`)
- [x] ~~Log des snapshots supprimés~~ ✓
- [x] ~~Build tags Windows/autres plateformes~~ ✓

### ~~Multi-PBS Architecture~~ ✅ IMPLÉMENTÉ (Backend complet)
- [x] ~~Structure PBSServer avec validation~~ ✓ (`gui/pbs_server.go`)
- [x] ~~Config.PBSServers map~~ ✓ (`gui/config.go`)
- [x] ~~Migration auto legacy → multi-PBS~~ ✓
- [x] ~~CRUD methods (Add/Update/Delete/List)~~ ✓
- [x] ~~Job.PBSID référence serveur~~ ✓ (`gui/jobs.go`)
- [x] ~~Méthodes App exposées au frontend~~ ✓ (`gui/main.go`)
- [x] ~~Documentation complète~~ ✓ (`MULTI_PBS_GUIDE.md`)
- [ ] **Frontend GUI à développer** (liste serveurs, dropdown jobs)

### ~~MSI Uninstall Dialog~~ ✅ IMPLÉMENTÉ
- [x] ~~Propriété KEEP_CONFIG~~ ✓ (`installer/wix/Product.wxs`)
- [x] ~~Custom action DeleteConfigFolder~~ ✓
- [x] ~~Dialog personnalisé avec radio buttons~~ ✓
- [x] ~~Documentation tests~~ ✓ (`MSI_UNINSTALL_TEST.md`)
- [ ] **Tests sur Windows à faire**

---

## 🔴 P0 - CRITIQUE (À faire maintenant)

### 🆕 Backup Metadata & NTFS Fidelity - CRITIQUE ⚠️

#### Étape 0 : Métadonnées de backup (backup-id → chemin original)
**Problème:** `GenerateBackupID()` sanitize les noms de dossiers (espaces→tirets, accents supprimés).
Le backup-id `JDS-SRV-1_D_DATA_BE_stephan_archive-dossiers-solidworks` ne permet plus de retrouver
le chemin original `D:\DATA\BE\stephan\archive dossiers solidworks`.

**Solution:** Fichier `.nimbus_backup_meta.json` stocké dans chaque archive PXAR :
```json
{
  "backup_id": "JDS-SRV-1_D_DATA_BE_stephan_archive-dossiers-solidworks",
  "original_path": "D:\\DATA\\BE\\stephan\\archive dossiers solidworks",
  "hostname": "JDS-SRV-1",
  "backup_time": "2026-04-08T22:15:00Z",
  "client_version": "0.2.51",
  "os": "windows",
  "vss_used": true
}
```

**Tâches:**
- [ ] Créer type `BackupMeta` dans `gui/backup_meta.go`
- [ ] Écrire `.nimbus_backup_meta.json` à la racine de l'archive PXAR avant le backup
- [ ] Lire et afficher les metadata dans l'UI restore (nom original du dossier)

#### Étape 1 : NTFS Metadata Fidelity
**Problème:** Les backups Windows perdent les ACLs, Alternate Data Streams, et timestamps NTFS complets.
**Impact:** Restauration incomplète - permissions perdues, attributs DOS absents.
**Référence audit:** Score 2/10 NTFS Fidelity

**Localisation:** `pbscommon/pxar.go:550-562` - WriteFile() hardcode UID/GID Unix

**Ce qui est PERDU actuellement:**
- ❌ Security Descriptors (DACL, SACL, Owner, Group)
- ❌ Alternate Data Streams (ex: `Zone.Identifier`)
- ❌ Creation Time (seul ModTime est sauvé)
- ❌ DOS Attributes (Hidden, System, Archive, ReadOnly)
- ⚠️ Reparse Points (skippés - OK)

**Sprint 1 - Solution (2 semaines):**
- [ ] **Créer `pkg/ntfs/backup_stream.go`**
  - [ ] Wrapper `windows.BackupRead()` pour lire metadata + ADS
  - [ ] Structure `BackupStream` avec WIN32_STREAM_ID
  - [ ] Parser les stream types: DATA, SECURITY_DATA, ALTERNATE_DATA
  - [ ] Fonction `BackupFileToStream(path) (*BackupStream, error)`

- [ ] **Modifier `pbscommon/pxar.go`**
  - [ ] Créer type `PXARWindowsMetadata` pour sidecar
  - [ ] Stocker SecurityDescriptor ([]byte base64)
  - [ ] Stocker CreationTime + LastAccessTime
  - [ ] Stocker DOS Attributes (uint32)
  - [ ] Stocker ADS entries (name + data)
  - [ ] Générer `.nimbus_meta` à côté de chaque fichier dans PXAR

- [ ] **Implémenter restore avec `windows.BackupWrite()`**
  - [ ] Lire `.nimbus_meta` lors du restore
  - [ ] Appliquer Security Descriptor
  - [ ] Restaurer ADS
  - [ ] Restaurer timestamps complets

- [ ] **Tests round-trip**
  - [ ] Test: fichier avec ACL custom → backup → restore → vérifier ACL identique
  - [ ] Test: fichier avec ADS `Zone.Identifier` → round-trip
  - [ ] Test: fichiers Hidden/System → vérifier attributs après restore

**Temps estimé:** 2 semaines (Sprint 1)

---

### 🆕 Splitting Récursif - AMÉLIORATION 📂
**Problème actuel:** Le splitting ne descend qu'à 1 niveau de profondeur.
**Exemple:**
- Input: `D:\DATA` (850GB)
- Analyse: `D:\DATA\Richard` = 700GB, `D:\DATA\Autre` = 150GB
- **Résultat:** Les 2 splits sont encore >100GB → fragiles

**Solution: Splitting récursif jusqu'à <100GB**

**Algorithme proposé:**
```go
func AnalyzeBackupDirsRecursive(dirs []string, maxSize uint64) []FolderInfo {
    folders := []FolderInfo{}

    for _, dir := range dirs {
        entries, _ := os.ReadDir(dir)

        for _, entry := range entries {
            path := filepath.Join(dir, entry.Name())
            size := calculateDirSize(path)

            if size > maxSize {
                // Trop gros → descendre récursivement
                subFolders := AnalyzeBackupDirsRecursive([]string{path}, maxSize)
                folders = append(folders, subFolders...)
            } else {
                // OK → garder ce niveau
                folders = append(folders, FolderInfo{Path: path, Size: size})
            }
        }
    }

    return folders
}
```

**Exemple avec D:\DATA:**
```
D:\DATA (850GB) → trop gros, descendre
  ├─ D:\DATA\Richard (700GB) → trop gros, descendre encore
  │   ├─ D:\DATA\Richard\Photos (80GB) ✅ OK
  │   ├─ D:\DATA\Richard\Videos (500GB) → trop gros, descendre
  │   │   ├─ D:\DATA\Richard\Videos\2023 (90GB) ✅ OK
  │   │   ├─ D:\DATA\Richard\Videos\2024 (150GB) → trop gros
  │   │   │   ├─ D:\DATA\Richard\Videos\2024\Q1 (40GB) ✅ OK
  │   │   │   └─ D:\DATA\Richard\Videos\2024\Q2 (110GB) → encore trop...
  │   └─ D:\DATA\Richard\Documents (120GB) → trop gros, etc.
  └─ D:\DATA\Autre (150GB) → trop gros, descendre...
```

**Résultat:** Tous les splits finaux sont <100GB

**Tâches:**
- [ ] **Modifier `AnalyzeBackupDirs()` → récursif**
  - [ ] Ajouter paramètre `maxDepth` (sécurité anti-boucle infinie)
  - [ ] Si dossier >100GB, descendre d'un niveau
  - [ ] Répéter jusqu'à tous les folders <100GB OU maxDepth atteint
  - [ ] Gérer cas extrême: 1 fichier de 500GB dans un dossier (impossible à splitter)

- [ ] **Cas limite: Dossier leaf >100GB**
  - Si `D:\DATA\Richard\HugFile` est un dossier avec 1 seul fichier de 500GB
  - → Impossible à splitter plus
  - → Log warning + accepter ce split "trop gros"
  - → Futur: fallback sur block-splitting avec offset (P2)

- [ ] **Tests**
  - [ ] Test: arbre profond avec folders >100GB imbriqués
  - [ ] Test: dossier leaf avec fichier unique 500GB (edge case)
  - [ ] Test: 1000 petits dossiers de 10GB (pas de over-splitting)

**Priorité:** 🟠 P1 - Amélioration importante du splitting existant
**Temps estimé:** 2-3 jours

---

### ~~Multi-PBS Architecture~~ ✅ BACKEND COMPLET (2026-03-23)
**Use case:** Multi-datastore (C:\ → bigdata, C:\Users → ssd) + GUI distante

Backend implémenté :
- [x] ~~Structure config avec map de PBS~~ ✓
- [x] ~~Validation: au moins 1 PBS configuré~~ ✓
- [x] ~~Migration: config actuelle → pbs_servers["default"]~~ ✓
- [x] ~~Jobs référencent PBS par ID~~ ✓
- [x] ~~CRUD API exposée (Add/Update/Delete/List)~~ ✓
- [x] ~~Documentation complète~~ ✓ (MULTI_PBS_GUIDE.md)

**Frontend à développer** (2-3 jours) :
- [ ] Page "Serveurs PBS" (liste avec CRUD)
- [ ] Dropdown "Serveur PBS" dans formulaire backup
- [ ] Test connexion par PBS (bouton + indicateur 🟢/🔴)
- [ ] Migration jobs legacy vers PBSID

**Temps estimé:** 2-3 jours frontend

---

## 🟠 P1 - IMPORTANT (Architecture Entreprise)

### 🆕 Secrets en Clair dans config.json 🔒
**Problème:** API tokens PBS stockés en plaintext dans `C:\ProgramData\ProxmoxBackupClient\config.json`
**Risque:** Tout admin local ou malware peut lire les credentials PBS.
**Note:** Risque modéré - admin local pourrait avoir accès datastore de toute façon, mais DPAPI ajoute une couche de défense en profondeur.
**Référence audit:** Score 4/10 Security (Secrets)

**Localisation:** `gui/config.go:131-143` - Save() écrit JSON en clair

**Sprint 2 - Solution DPAPI (1 semaine):**
- [ ] **Créer `pkg/secrets/dpapi_windows.go`**
  - [ ] Wrapper `CryptProtectData()` avec `CRYPTPROTECT_LOCAL_MACHINE`
  - [ ] Fonction `ProtectSecret(plaintext) (ciphertext string, error)`
  - [ ] Fonction `UnprotectSecret(ciphertext) (plaintext string, error)`
  - [ ] Encoder ciphertext en base64 pour JSON

- [ ] **Modifier `gui/config.go`**
  - [ ] Au `Save()`: détecter secrets sans préfixe `DPAPI:`
  - [ ] Chiffrer avec `ProtectSecret()` et préfixer `DPAPI:`
  - [ ] Au `Load()`: détecter préfixe `DPAPI:` et déchiffrer
  - [ ] Migration automatique: plaintext → DPAPI au premier Load

- [ ] **Tests**
  - [ ] Test: round-trip ProtectSecret → UnprotectSecret
  - [ ] Test: service LocalSystem peut déchiffrer les secrets
  - [ ] Test: migration auto d'une config legacy plaintext
  - [ ] Test: nouvelle install génère secrets chiffrés directement

**Temps estimé:** 1 semaine (Sprint 2)

---

### 🆕 Splitting Récursif + Retry Granulaire ⏯️
**Problème:** Backups de gros volumes (>1TB) fragiles - tout recommencer si échec.
**Solution SIMPLE:** Splitter intelligemment + retry par split (pas de checkpoints complexes).
**Référence audit:** Score 3/10 Resilience (Resume)

**Approche retenue (plus simple que checkpoints):**

Au lieu de gérer des checkpoints de chunks uploadés, on découpe le travail en petites unités atomiques :

**1. File Mode: Splitting récursif jusqu'à <100GB** (voir section ci-dessus)
   - Chaque split = backup indépendant avec son backup-id
   - Si split 3/10 échoue → splits 1-2 et 4-10 sont déjà OK sur PBS
   - Retry: relancer seulement le split qui a échoué

**2. Block Mode: Splitting par offset de 100GB** (futur - voir section P2)
   - Disk 1TB → 10 parts de 100GB
   - Si part 5 échoue → parts 1-4 et 6-10 déjà sauvegardées
   - PBS déduplique automatiquement entre parts

**Avantages vs Checkpoints:**
- ✅ Beaucoup plus simple (pas de checkpoint.json à gérer)
- ✅ PBS gère déjà la déduplication des chunks
- ✅ Retry granulaire au niveau split (pas chunk par chunk)
- ✅ Backup-ids uniques = traçabilité PBS propre
- ✅ Pas de corruption si checkpoint corrompu

**Tâches principales:**
- [x] ~~Splitting par sous-dossier~~ ✅ Déjà implémenté (v0.2.25)
- [ ] **Améliorer: Splitting récursif si folder >100GB** (voir P1 ci-dessus)

- [ ] **⏸️ Cache du Split Plan** (REPORTÉ - dépend des fréquences backup)
  - **Problème actuel :** `AnalyzeBackupDirs()` re-scanne TOUT à chaque backup
  - Pour D:\DATA (850GB) : waste 5-10 minutes à chaque backup

  **Blocker :** Le cache TTL doit être adaptatif selon fréquence du job
  ```
  Backup horaire   → cache 1h (ou pas de cache)
  Backup quotidien → cache 1 jour
  Backup hebdo     → cache 7 jours
  ```

  **Décision :** Implémenter d'abord les fréquences de backup, puis évaluer si cache vraiment nécessaire.

  **Alternative simple :** Invalidation basée uniquement sur folder modtime (pas de TTL fixe)
  - Check rapide `os.Stat()` des top-level folders
  - Si modtime changé → re-scan
  - Sinon → réutiliser dernier split plan
  - **Avantage :** Fonctionne pour toutes les fréquences

- [ ] **Retry logique par split**
  - [ ] Si split échoue → log l'erreur + continuer les autres
  - [ ] À la fin → afficher liste des splits échoués
  - [ ] Bouton "Retry failed splits" dans UI
  - [ ] Backend: méthode `RetryFailedSplits(jobID)`

- [ ] **UI: Suivi multi-splits**
  - [ ] Afficher "Split 3/10 in progress"
  - [ ] Barre de progression globale (tous les splits)
  - [ ] Liste des splits: ✅ réussis, ⏳ en cours, ❌ échoués
  - [ ] Bouton "Retry" pour splits échoués uniquement

**Temps estimé:** 1 semaine (amélioration du système existant)

---

## 🟠 P1 - IMPORTANT (Prochaines semaines)

### Service Windows - Robustesse
- [x] ~~**VSS Cleanup au démarrage**~~ ✅ FAIT (2026-03-23)
  - [x] ~~Appel dans `service.run()`~~ ✓
  - [x] ~~Log les shadows supprimées~~ ✓
  - [x] ~~Build tags Windows/Linux~~ ✓

- [ ] **Working Directory fix**
  ```go
  exePath, _ := os.Executable()
  os.Chdir(filepath.Dir(exePath))
  ```
  - [ ] Force au démarrage du service
  - [ ] Test: config.json trouvé dans ProgramData

- [ ] **Logs accessibles**
  - [ ] Service log dans `C:\ProgramData\ProxmoxBackupClient\logs\service.log`
  - [ ] GUI: bouton "Voir logs du service" (lecture seule)
  - [ ] Rotation: max 10 MB par fichier

### MSI - Finitions

- [x] ~~**Désinstallation avec choix config**~~ ✅ FAIT (2026-03-23)
  - [x] ~~Dialog WiX personnalisé~~ ✓
  - [x] ~~Propriété KEEP_CONFIG~~ ✓
  - [x] ~~CustomAction DeleteConfigFolder~~ ✓
  - [ ] **Tests sur Windows à faire**

- [ ] **Installation Silencieuse (Silent Install)**
  - [ ] **Approche: Config JSON pré-configuré** (propre pour AD/GPO)
    ```powershell
    # Déploiement avec config centralisée
    msiexec /i ProxmoxBackupClient.msi /qn CONFIGFILE="\\ad-server\deploy\proxmoxbackupclient\config.json"
    ```
  - [ ] Property WiX: `CONFIGFILE` (chemin vers config.json)
  - [ ] CustomAction WiX:
    - Si `CONFIGFILE` fourni → copier vers `C:\ProgramData\ProxmoxBackupClient\config.json`
    - Valider JSON avant copie (éviter corruption)
    - Log erreur si fichier inaccessible
  - [ ] Template config.json à fournir:
    ```json
    {
      "pbs_url": "https://pbs.example.com:8007",
      "auth_id": "backup-user@pbs",
      "secret": "your-api-token-secret",
      "datastore": "backup",
      "namespace": "clients",
      "backup_id": "",  // Vide = utilise hostname
      "backup_dirs": ["C:\\Users", "C:\\Important"],
      "exclusions": ["*.tmp", "*.log"],
      "schedule": {
        "enabled": true,
        "time": "02:00",
        "days": ["monday", "wednesday", "friday"]
      },
      "vss_enabled": true
    }
    ```
  - [ ] Test: install silencieux → service démarre avec config OK
  - [ ] Doc: guide déploiement GPO/Intune avec config.json

- [ ] **Code Signing**
  - [ ] Signer le binaire `.exe`
  - [ ] Signer le `.msi`
  - [ ] Certificat: à obtenir (DigiCert/Sectigo ~300€/an)

- [ ] **Désinstallation propre**
  - [ ] Script CustomAction: stop service avant uninstall
  - [ ] Nettoyer `C:\ProgramData\ProxmoxBackupClient` (option: garder config)

### 🆕 Fréquences de Backup Multiples ⏰
**Problème actuel :** Scheduler supporte uniquement backup quotidien à heure fixe (HH:MM)
**Use cases manquants :**
- Backup **horaire** (ex: toutes les heures en journée)
- Backup **hebdomadaire** (ex: dimanche 3h du matin)
- Backup **mensuel** (ex: 1er du mois)
- Backup **custom** (ex: lundi-vendredi à 14h)

**Architecture actuelle :**
```go
type ScheduledJob struct {
    ScheduleTime string `json:"scheduleTime"` // "14:30" = daily at 14:30
    ...
}
```

**Solution proposée : Format Cron + Presets UI**

- [ ] **Modifier ScheduledJob structure**
  ```go
  type ScheduledJob struct {
      ID           string   `json:"id"`
      Name         string   `json:"name"`

      // Nouveau: Support cron + presets
      ScheduleType string   `json:"scheduleType"` // "preset", "cron", "once"
      Preset       string   `json:"preset"`       // "hourly", "daily", "weekly", "monthly"
      CronExpr     string   `json:"cronExpr"`     // Format cron si scheduleType="cron"

      // Pour presets avec paramètres
      Time         string   `json:"time"`         // "14:30" pour daily/weekly
      Weekday      string   `json:"weekday"`      // "monday" pour weekly
      MonthDay     int      `json:"monthDay"`     // 1-31 pour monthly

      // Deprecated (migration)
      ScheduleTime string   `json:"scheduleTime,omitempty"` // Legacy

      RunAtStartup bool     `json:"runAtStartup"`
      ...
  }
  ```

- [ ] **Presets UI simples**
  ```
  Dropdown:
    [x] Hourly        → cron: "0 * * * *"
    [ ] Every 2h      → cron: "0 */2 * * *"
    [ ] Every 6h      → cron: "0 */6 * * *"
    [ ] Daily at...   → time picker: "14:30" → cron: "30 14 * * *"
    [ ] Weekly on...  → day picker + time → cron: "30 14 * * 1" (lundi)
    [ ] Monthly       → day of month + time → cron: "30 14 1 * *"
    [ ] Custom cron   → text input: "*/15 9-17 * * 1-5" (toutes les 15min, 9h-17h, lun-ven)
  ```

- [ ] **Intégrer lib cron Go**
  ```go
  import "github.com/robfig/cron/v3"

  func (a *App) StartScheduler() {
      c := cron.New()

      jobs, _ := a.GetScheduledJobs()
      for _, job := range jobs {
          if !job.Enabled {
              continue
          }

          cronExpr := job.CronExpr
          if job.ScheduleType == "preset" {
              cronExpr = presetToCron(job.Preset, job.Time, job.Weekday, job.MonthDay)
          }

          c.AddFunc(cronExpr, func() {
              a.executeScheduledJob(job)
          })
      }

      c.Start()
  }

  func presetToCron(preset, time, weekday string, monthDay int) string {
      hour, min := parseTime(time) // "14:30" → 14, 30

      switch preset {
      case "hourly":
          return "0 * * * *"
      case "daily":
          return fmt.Sprintf("%d %d * * *", min, hour)
      case "weekly":
          day := weekdayToCron(weekday) // "monday" → 1
          return fmt.Sprintf("%d %d * * %d", min, hour, day)
      case "monthly":
          return fmt.Sprintf("%d %d %d * *", min, hour, monthDay)
      }
  }
  ```

- [ ] **Migration legacy jobs**
  ```go
  // Au Load() des jobs existants:
  if job.ScheduleTime != "" && job.ScheduleType == "" {
      // Legacy format "14:30" → migrate to daily preset
      job.ScheduleType = "preset"
      job.Preset = "daily"
      job.Time = job.ScheduleTime
      job.CronExpr = presetToCron("daily", job.Time, "", 0)
  }
  ```

- [ ] **Tests**
  - [ ] Test: hourly preset → backup toutes les heures
  - [ ] Test: weekly preset → backup lundi à 14h30
  - [ ] Test: custom cron "*/15 9-17 * * 1-5" → backup toutes les 15min en semaine
  - [ ] Test: migration legacy jobs avec ScheduleTime

**Dépendance :** Cette feature bloque l'optimisation du cache du split plan (TTL adaptatif)

**Priorité :** 🟠 P1 - Requis avant optimisations performance
**Temps estimé :** 3-4 jours

---

### Multi-jobs - Stabilisation
- [ ] **Queue management**
  - [ ] Pas de 2 jobs VSS simultanés
  - [ ] File d'attente FIFO
  - [ ] UI: afficher "En attente..." si queue pleine

- [ ] **Test de charge**
  - [ ] Lancer 5 jobs en même temps
  - [ ] Vérifier pas de corruption d'index PBS
  - [ ] RAM usage < 500 MB

---

## 🟢 P2 - NICE TO HAVE (Backlog)

### ⏹️ No way to cancel a running machine backup (directory backups already can)

**Question raised (2026-09-25):** noticed there's no Cancel option once a full machine backup is
running. Is a clean cancel even possible without orphaning the block snapshot?

**Yes — the cleanup design already supports it safely, it's just not wired up.** Both platforms'
`CreateVSSSnapshot` (`snapshot/linux_snapshot.go:356`, `snapshot/win_snapshot.go:82`) use a
deferred cleanup that destroys/releases every snapshot it created, in reverse order, regardless of
*how* the function returns — a normal completion, an early error, or a cancellation signal that
makes the callback return an error all unwind through the same `defer` and clean up correctly.
That's proven safe already: it's the exact same path a normal error takes today. The only thing
that would skip it is the process being killed outright (`kill -9`, a crash, a power cut) rather
than cancelled in-process — that's a real risk either way, not something a Cancel button changes.

**What's actually missing:** the GUI's Stop button (`CancelBackup()`, `gui/backup_inline.go:99`)
only cancels the *directory*-backup path — `RunBackupInline` registers itself with the shared
cancel context (`newBackupContext()`, called at `backup_inline.go:565`), but the machine-backup
path (`runMachineBackupInline`, `backup_inline.go:1216`) calls `machinebackuplib.Backup()` directly
and never registers with that context at all, so Stop has zero effect on a running machine backup
today. `machinebackuplib/linux.go` (and `windows.go`) already return an `errCancelled` sentinel at
a few points in their copy loops, suggesting partial groundwork for exactly this, just never
connected to a real trigger.

- [ ] Give `runMachineBackupInline`/`machinebackuplib.Backup` a cancellation signal (context, or a
      simple flag checked in the copy loop, matching the existing `errCancelled` return points)
- [ ] Wire the GUI's Stop button to it the same way `newBackupContext()` already does for
      directory backups, so one Cancel path covers both backup types
- [ ] Confirm the Stop button is actually visible/enabled during a running machine backup in the
      frontend (not just fixed on the backend) once this lands

### ~~⚠️ Deleting a Backup Set has no confirmation prompt~~ ✅ FIXED 2026-09-25

Added the same `confirm(...)` pattern `handleDeletePBSServer` already used, with a new
`confirmDeleteJob` translation key (mirroring `confirmDeleteServer`) across all 6 languages,
naming the job being deleted.

### 📋 Clone a Backup Set

**Suggested 2026-09-25.** Each Backup Set row already has Run Now/Edit/Delete buttons
(`gui/frontend/src/App.jsx` ~line 2383-2431) — a Clone button would sit naturally alongside them.
Setting up a new Backup Set similar to an existing one (same source/exclude/schedule shape, just a
different destination or a tweak) currently means re-filling the whole wizard from scratch.

- [ ] Reuse the Edit handler's field-population logic (~line 2402-2426, the same one that restores
      `backupDirs`/`selectedDrives`/`excludeList`/schedule/etc. into the form) but leave
      `editingJobId` unset (or explicitly null) so Save creates a new job via `SaveScheduledJob`
      instead of updating the original via `UpdateScheduledJob`
- [ ] Default the cloned name to something like "{original name} (copy)" so it's obviously a copy
      and doesn't collide, but leave it editable before the first save
- [ ] Decide whether `lastRun`/job history should reset for the clone (it should — a clone hasn't
      actually run yet) — `SaveScheduledJob` creating a fresh job with a new ID should already give
      this for free, worth confirming rather than assuming

### 🌍 BMR wizard skips language selection entirely — English only

**Question raised (2026-09-25):** the automated boot entry's whole point is skipping stock
Clonezilla's prompts for speed, and that includes the language/keyboard one —
`locales=en_US.UTF-8 keyboard-layouts=gb` is hardcoded, so it never asks at all, unlike stock
Clonezilla or the manual `pbs-nbd` entry. Should it offer the same 6 languages the GUI supports
instead of skipping the choice outright?

**Two genuinely different things are involved, easy to conflate:**
- Clonezilla's **own** native prompts (`$msg_program_stop`, `$msg_nchc_clonezilla`,
  `$msg_do_u_want_to_do_it_again`, etc. — already used throughout
  `ocs-pbs-bare-metal-restore`) come from Clonezilla's own message catalog, loaded via
  `ask_and_load_lang_set` based on `locales=`. Clonezilla genuinely ships real translations
  (DRBL/NCHC project) — changing `locales=` would very likely localize *these* correctly.
- Almost everything a user actually sees in this wizard is **our own hardcoded English text**
  written directly into the script (every dialog title, every PBS prompt, the final "Restore is
  complete..." message) — none of it goes through Clonezilla's translation system, so changing
  `locales=` alone would do nothing for it and produce a broken-looking mix of a few translated
  native strings next to all our own prompts still in English.

**Real fix, if wanted:** a language-selection `$DIA --menu` as the very first prompt (same 6
languages as the GUI), then a genuine translation table for every one of the wizard's own
strings — the bash equivalent of `translations.js`, not a one-line boot-param change.

**Worth weighing against the wizard's own design goal:** hardcoding `locales=`/`keyboard-layouts=`
was specifically to skip Clonezilla's language/keyboard prompts for speed during a bare-metal
recovery. A language picker adds back exactly one prompt this wizard was built to eliminate —
reasonable to just default to English and move on, given this is a rare, one-off boot rather than
daily-use software, unlike the GUI where full localization clearly earns its keep.

- [ ] Decide: worth it for a rare, one-off boot flow, or leave English-only and revisit if
      non-English users actually ask for it?
- [ ] If yes: build the language picker + full string table in
      `clonezilla-patch/ocs-pbs-bare-metal-restore`, verify Clonezilla's own `$msg_*` strings
      actually do localize correctly for each of the 6 languages (not independently confirmed
      yet, just expected from how Clonezilla's own i18n is documented to work)

### ~~💾 Settings export/import (portability across machines)~~ ✅ DONE 2026-09-25

Preferences → Advanced now has Export/Import Settings buttons (`gui/settings_export.go`,
`App.ExportSettings`/`App.ImportSettings`), producing a single JSON bundle covering `config.json`
(`PBSServers`, `DefaultPBSID`, theme, SMTP host settings) and `scheduled_jobs.json` (the actual
Backup Sets). Import upserts by ID rather than replacing wholesale, so it can't wipe out unrelated
local servers or jobs.

**Secrets question resolved as:** exclude by default, via an "Include passwords/tokens" checkbox
that defaults off, with a clear in-UI warning when turned on. A server whose secret was excluded
at export time keeps whatever's already configured locally for that same server ID on import,
rather than blanking it. Passphrase-encrypted export was considered and skipped for now — plain
JSON plus a clear warning was judged sufficient for a personal/homelab tool; revisit if this ever
needs to be safe to email or store somewhere less trusted.

### 📦 Release pipeline for this fork (MSI, CI attestation, VirusTotal)

**Found 2026-09-25:** the README's "Verifying a download" section described upstream's release
process (SHA-256 checksums, a signed GitHub attestation, VirusTotal scans of MSI installers) as
if this fork had the same — it didn't. `v0.3.0` was built and uploaded by hand (`wails build` +
`gh release create`), so there was no `SHA256SUMS.txt` (now added), no attestation, no VirusTotal
scan, and no MSI at all (just a plain `.exe` zip). README corrected to say so plainly rather than
implying guarantees this fork doesn't actually provide yet.

To actually close this gap for a future release:
- [ ] **MSI installer** — the WiX project already exists (`installer/wix/Product.wxs`,
      `build.bat`), but WiX Toolset (`candle.exe`/`light.exe`) isn't installed on the current dev
      machine, and `installer/wix/*.wxs` should be checked for stale "NimbusBackup"/"AcmeBackup"
      branding before trusting it builds the right thing for this fork.
- [ ] **CI-based build provenance attestation** — a meaningful attestation has to be generated by
      a GitHub Actions workflow (via OIDC), not from a personal machine; that means setting up an
      actual release workflow (`.github/workflows/release.yml`) that builds Windows/Linux/the BMR
      ISO and attests them in one automated run, rather than the current by-hand process.
- [ ] **VirusTotal scan** — needs either a manual upload via virustotal.com or a VT API key; not
      set up yet either way.
- [ ] **Code signing** — same status as upstream (no Authenticode cert yet); a real fix here
      would remove the SmartScreen/Defender false-positive warning entirely instead of just
      explaining it away.

### 🎨 GUI polish (this fork)

#### ~~Restore progress bar doesn't match the backup one~~ ✅ FIXED 2026-09-25
Restore now uses the same `.progress`/`.progress-bar` CSS classes as backup (30px, themed via
`var(--accent)`, percentage rendered inside the bar) instead of a bespoke 8px div hardcoded to
`#2563eb`. The transfer speed still shows as its own line below, since that's extra detail backup's
bar doesn't have, not a styling mismatch.

#### ~~VSS label in Preferences → Destination says "Windows Shadow Copy" only~~ ✅ FIXED 2026-09-25
`useVSS` label now reads "Use VSS (Windows Shadow Copy / Linux Snapshot)" (and equivalent in all
6 languages), since the same checkbox gates elastio-snap/dattobd on Linux as much as VSS on
Windows.

### 📧 Email notifications in the GUI (engine already exists, just not wired to it)

**What's already there:** `clientcommon/mail.go` is a complete, working SMTP client
(`SetupMailClient`/`SendMail`, TLS/STARTTLS/plain on 465/587/25) plus a templated `MailCtx`
(Go `text/template`, fields: `Success`/`Partial`/`Status`/`Duration`/`NewChunks`/`ReusedChunks`/
`ReadErrors`/`Hostname`/`StartTime`/`EndTime`/`ErrorStr`). `proxmoxbackup-directory` fully wires
it up already — `-mail-host`/`-mail-port`/`-mail-username`/`-mail-password`/`-mail-insecure`/
`-mail-from`/`-mail-to`/`-mail-subject-template`/`-mail-body-template` flags, config-file driven
via `SMTPConfig` (`directorybackup/config.go`), and it even supports multiple from/to pairs
(`SMTP.Mails []...`). `proxmoxbackup-machine` defines the same `-mail-*` flags
(`machinebackup/main.go`) but never actually calls `SetupMailClient`/`SendMail` — dead/unwired.

**What's missing:** the GUI itself (`gui/*.go`) has zero references to any of this — no
Preferences tab, no per-Backup-Set option, nothing. Every notification in the GUI today is
purely visual (Reports/Message Log), nothing leaves the machine.

**Two design directions to weigh** (not mutually exclusive):
- [ ] **Global**: one SMTP setup in Preferences (reusing `clientcommon`'s existing client/config
      shape), used to push a message whenever *anything* completes — effectively mailing out
      Message Log entries or a Reports summary as they happen.
- [ ] **Per-Backup-Set**: notification options on each `ScheduledJob` (on success / on failure /
      always / never, maybe its own subject/body template override), so a Backup Set can opt in
      or out independently once the global SMTP account is configured.

A sensible shape is probably: one global SMTP account in Preferences (host/port/auth, matching
what `directorybackup` already accepts), then a lightweight per-Backup-Set toggle that reuses it.
Wire `machinebackup`'s already-declared-but-dead `-mail-*` flags while at it, so both CLI tools
actually behave the same way.

### 🖥️ Post-backup actions (shutdown PC, exit app, run an application)

**Suggested 2026-09-25**, referencing Backup for Workgroups' own "Special Items" wizard step
("When Your Backup Session Completes": send e-mail, close BFW, turn off your computer, write
results to the Event log; plus "Run an Application" before/after). This fork's Backup Set editor
has no equivalent — worth adding as its own chevron tab (same pattern as the planned "Alerts" tab
above), covering:
- [ ] **Turn off the computer** after this Backup Set completes — the one item with a genuine,
      already-open reference implementation upstream: PR #43
      (https://github.com/tizbac/proxmoxbackupclient_go/pull/43, "feat: add option for shutting
      down pc after backup") touches root-level `config.go`/`main.go` from before the module
      refactor, so it isn't directly portable, but the feature idea and its shutdown-invocation
      approach are worth a look before reimplementing from scratch.
- [ ] **Exit Proxmox Backup Client Go** after this Backup Set completes (BFW's "Close Backup for
      Workgroups" equivalent) — relevant mainly for the one-shot/manual-run case, not scheduled
      background jobs.
- [ ] **Run an application** before and/or after the backup (BFW's "Run an Application" section) —
      a command/path field plus before/after timing, presumably with its own success/failure
      handling (does an after-backup command run on failure too, or only on success?).
- [ ] **Send an e-mail** on completion — already tracked in full above ("Email notifications in the
      GUI"); this tab is a natural place to surface that per-Backup-Set toggle once it exists,
      rather than a separate thing. **Final shape confirmed 2026-09-25: two independent fields**,
      not a single success/failure/always/never selector — "On completion, email to: [address]"
      and "On failed backup, email to: [address]", each with its own on/off, since the recipient
      may genuinely differ (e.g. failures going somewhere more urgent than routine successes), or
      one may be wanted without the other. Both reuse the global SMTP account from Preferences.

Should probably be scoped per-Backup-Set (like BFW's own wizard, which is per backup job) rather
than global in Preferences, since "shut down after this backup" only makes sense for specific jobs
(e.g. an overnight one-shot), not every scheduled run.

### ~~🔧 Two small pulls from upstream~~ ✅ DONE 2026-09-25

Found while surveying tizbac's open PRs for anything relevant to this fork (see
[[project_windows_pbs_client_fork]] memory for the full survey) — both applied cleanly, no
design discussion needed:

- [x] **Fix the `config.json.example` JSON bug** (upstream PR #81,
      https://github.com/tizbac/proxmoxbackupclient_go/pull/81) — the `"to"` fields in the `mails`
      array were missing their closing quote (`"to": "receiver1@example.com` with no trailing `"`).
      Fixed.
- [x] **Pull in PR #76's `PBS_*` environment variable support**
      (https://github.com/tizbac/proxmoxbackupclient_go/pull/76) — adds `PBS_REPOSITORY` (and the
      atom vars `PBS_SERVER`/`PBS_PORT`/`PBS_DATASTORE`/`PBS_AUTH_ID`/`PBS_PASSWORD`/
      `PBS_FINGERPRINT`) to `directorybackup`, `machinebackup`, and `nbd`, via a new
      `pbscommon/pbsrepo.go` that ports the real `proxmox-backup-client`'s own repository-URL
      regex/parsing from its Rust source. Wired into `directorybackup`/`machinebackup`/`nbd`
      (before config-file/flag processing, so flags still win, then env vars, then built-in
      defaults). Original PR's full test suite ported over unchanged (`pbscommon/pbsrepo_test.go`),
      all passing.

### 🆕 Sprint 4 - Polish & Production Ready (1 semaine)

#### Code Signing - Windows Trust 🔐
**Problème actuel:**
- ❌ Windows SmartScreen warning
- ❌ Windows Defender suspicion sur raw disk access (VSS)
- ❌ GPO entreprise bloque .exe non signés
- ❌ MSI non signé = installation bloquée par IT

**Solution retenue : Azure Trusted Signing** (moderne + CI/CD friendly)

**Avantages vs EV Certificate classique:**
- ✅ **108€/an** au lieu de 300-500€/an
- ✅ **Pas de clé USB** physique (EV cert requirement)
- ✅ **Cloud-native** : intégration GitHub Actions directe
- ✅ **Microsoft-approved** : bonne réputation SmartScreen
- ✅ **Timestamping inclus** : signature valide même après expiration

**Mise en place (Sprint 4):**

- [ ] **1. Provisionner Azure Trusted Signing**
  - [ ] Créer compte Azure (ou utiliser existant)
  - [ ] Activer "Azure Trusted Signing" dans le portail
  - [ ] Créer "Signing Identity" avec validation domaine
  - [ ] Plan : Basic (108€/an, suffisant pour projet open-source/PME)
  - [ ] Récupérer credentials : Tenant ID, Client ID, Client Secret

- [ ] **2. GitHub Actions Workflow**
  - [ ] Créer `.github/workflows/release.yml`
  - [ ] Stocker credentials dans GitHub Secrets:
    - `AZURE_TENANT_ID`
    - `AZURE_CLIENT_ID`
    - `AZURE_CLIENT_SECRET`
    - `AZURE_SIGNING_ENDPOINT`

**Workflow complet:**
```yaml
# .github/workflows/release.yml
name: Build, Sign & Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build-sign-release:
    runs-on: windows-latest

    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Setup Wails
        run: go install github.com/wailsapp/wails/v2/cmd/wails@latest

      - name: Build
        run: wails build -clean -platform windows/amd64

      - name: Sign EXE with Azure Trusted Signing
        uses: azure/trusted-signing-action@v0.5.0
        with:
          azure-tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          azure-client-id: ${{ secrets.AZURE_CLIENT_ID }}
          azure-client-secret: ${{ secrets.AZURE_CLIENT_SECRET }}
          endpoint: ${{ secrets.AZURE_SIGNING_ENDPOINT }}
          trusted-signing-account-name: ProxmoxBackupClient
          certificate-profile-name: Production
          files-folder: build/bin
          files-folder-filter: exe
          file-digest: SHA256
          timestamp-rfc3161: http://timestamp.acs.microsoft.com
          timestamp-digest: SHA256

      - name: Build MSI Installer
        run: |
          # TODO: Intégrer WiX build ici
          # wix build installer/Product.wxs

      - name: Sign MSI with Azure Trusted Signing
        uses: azure/trusted-signing-action@v0.5.0
        with:
          azure-tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          azure-client-id: ${{ secrets.AZURE_CLIENT_ID }}
          azure-client-secret: ${{ secrets.AZURE_CLIENT_SECRET }}
          endpoint: ${{ secrets.AZURE_SIGNING_ENDPOINT }}
          trusted-signing-account-name: ProxmoxBackupClient
          certificate-profile-name: Production
          files-folder: build/installer
          files-folder-filter: msi
          file-digest: SHA256
          timestamp-rfc3161: http://timestamp.acs.microsoft.com
          timestamp-digest: SHA256

      - name: Verify Signatures
        run: |
          signtool verify /pa /v build/bin/ProxmoxBackupClient.exe
          signtool verify /pa /v build/installer/ProxmoxBackupClient.msi

      - name: Submit to Microsoft Security Intelligence
        run: |
          # Soumettre binaire signé à MS pour analyse (éviter faux positifs Defender)
          curl -X POST "https://www.microsoft.com/en-us/wdsi/filesubmission" \
            -F "file=@build/bin/ProxmoxBackupClient.exe" \
            -F "type=FalsePositive" \
            -F "comment=Proxmox Backup Client - Signed software, accesses raw disk via VSS for backup imaging"
        continue-on-error: true  # Ne pas bloquer release si API MS down

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            build/bin/ProxmoxBackupClient.exe
            build/installer/ProxmoxBackupClient.msi
          body: |
            ## 🔐 Code Signed
            This release is signed with **Azure Trusted Signing**.

            Verify signature:
            ```powershell
            Get-AuthenticodeSignature ProxmoxBackupClient.exe | Format-List
            ```
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **3. Tests post-signature**
  - [ ] Vérifier signature visible dans propriétés EXE (Digital Signatures tab)
  - [ ] Install sur Windows 11 fresh → pas de SmartScreen warning
  - [ ] Windows Defender ne bloque pas après soumission MS
  - [ ] GPO entreprise accepte l'EXE signé
  - [ ] MSI install sans UAC warning supplémentaire

- [ ] **4. Documentation**
  - [ ] README : mentionner "Code signed with Azure Trusted Signing"
  - [ ] Doc admin : comment vérifier la signature
  - [ ] Doc GPO : whitelister le certificat si nécessaire

**Coût total annuel:**
```
Azure Trusted Signing Basic:  108€/an
Temps dev setup (1x):          ~4h
Soumission MS (automatique):   0€
Support client "Defender":     0h (plus de faux positifs)
─────────────────────────────────────────
Total annuel:                  108€ + café ☕
ROI:                           Immédiat (zéro friction install)
```

**vs Alternative sans signature:**
```
Certificat:                    0€
Support client "SmartScreen":  ∞ heures 😱
Support client "Defender":     ∞ heures 💀
Réputation produit:            📉
Taux d'adoption entreprise:    20% (bloqué par IT)
─────────────────────────────────────────
Total:                         Inestimable en temps perdu
```

#### Progress Worker - UI Non-Blocking
**Problème:** Callbacks `onProgress()` synchrones ralentissent le backup si frontend React lent.
**Localisation:** `backup_inline.go:168-201`

**Solution:**
- [ ] **Créer `gui/progress_worker.go`**
  - [ ] Worker goroutine avec channel bufferisé (cap: 100)
  - [ ] Rate limiting: max 2 updates/sec (500ms min delay)
  - [ ] Conserver seulement le dernier update (drop intermédiaires)
  - [ ] Callback vers frontend asynchrone

- [ ] **Intégrer dans backup**
  - [ ] Remplacer appels `onProgress()` directs par `worker.Update()`
  - [ ] Worker s'occupe du throttling
  - [ ] Pas de blocking sur thread backup

**Temps estimé Sprint 4:** 1 semaine

---

### Windows - Compatibilité avancée
- [ ] **LongPath Support**
  - [ ] Ajouter manifeste: `<longPathAware>true</longPathAware>`
  - [ ] Test: backup d'un chemin >260 caractères

- [ ] **Gestion des locks**
  - [ ] Détecter fichier ouvert sans VSS
  - [ ] Erreur propre: "Fichier X verrouillé, activer VSS?"

### API Remote - Provisioning Distant (Phase 2)
**Use case:** MSP gère 100+ clients Proxmox Backup Client depuis interface centrale

- [ ] **API Remote activable**
  ```json
  {
    "api": {
      "remote_enabled": false,  // Désactivé par défaut (sécurité)
      "bind_address": "0.0.0.0:18765",  // Si activé
      "auth_token": "generated-at-install",
      "tls_cert": "/path/to/cert.pem",  // Optionnel
      "allowed_ips": ["192.168.1.0/24"]  // Whitelist
    }
  }
  ```
  - [ ] Flag service: `--remote-api` pour activer
  - [ ] Auth: Bearer token (généré install, 32 chars)
  - [ ] TLS: Certificat auto-signé ou fourni
  - [ ] Rate limiting: max 10 req/s par IP
  - [ ] Whitelist IPs configurables

- [ ] **Endpoints Provisioning**
  - `GET /api/v1/info` - Info système (hostname, version, mode)
  - `GET /api/v1/pbs` - Liste serveurs PBS configurés
  - `POST /api/v1/pbs` - Ajouter serveur PBS
  - `PUT /api/v1/pbs/{id}` - Modifier serveur PBS
  - `DELETE /api/v1/pbs/{id}` - Supprimer serveur PBS
  - `POST /api/v1/pbs/{id}/test` - Test connexion
  - `GET /api/v1/jobs` - Liste jobs
  - `POST /api/v1/jobs` - Créer job
  - `PUT /api/v1/jobs/{id}` - Modifier job
  - `DELETE /api/v1/jobs/{id}` - Supprimer job
  - `POST /api/v1/backup` - Lancer backup manuel

- [ ] **GUI Centrale MSP** (Futur produit séparé)
  - Dashboard: grille avec tous les clients
  - Actions groupées: "Backup tout le parc"
  - Alertes: machine pas vue depuis 24h
  - Statistiques globales

**Temps estimé:** 2-3 semaines

### Multi-Serveurs PBS
**→ DÉPLACÉ EN P0** (voir "Multi-PBS Architecture" ci-dessus)

### Block-Level Splitting avec Offset (Disques Full) 💾
**Status:** 📝 Documentation seulement - PAS ENCORE EN PROD

**Concept:** Splitter les backups de disques physiques en chunks de 100GB avec offset.

**Architecture proposée:**
```go
// Backup d'un disque de 1TB en 10 parts de 100GB
type BlockSplitJob struct {
    DiskNumber    int
    StartOffset   uint64  // Bytes
    EndOffset     uint64  // Bytes
    Size          uint64  // 100GB
    BackupID      string  // hostname_disk0_part1, part2...
}

// Exemple: Disk 0 (1TB)
// Split 1: offset 0 → 100GB (part1)
// Split 2: offset 100GB → 200GB (part2)
// ...
// Split 10: offset 900GB → 1TB (part10)
```

**Avantages:**
- Chaque backup est <100GB → plus résilient
- PBS déduplique automatiquement les chunks identiques entre parts
- Si part5 échoue, parts 1-4 et 6-10 sont déjà sauvegardés
- Retry logique simple: relancer seulement la part qui a échoué

**Contrainte PBS:** 100GB doit être un multiple de 4MB (chunk size PBS)
- 100GB = 107,374,182,400 bytes
- 4MB = 4,194,304 bytes
- 107,374,182,400 / 4,194,304 = 25,600 chunks ✅ Multiple exact

**Implémentation (Futur - Phase 3):**
- [ ] Réactiver `machine_backup_windows.go.disabled`
- [ ] Modifier pour utiliser `FixedIndex` avec offset start/end
- [ ] Créer `AnalyzePhysicalDisk(diskNum)` → liste de BlockSplitJobs
- [ ] Backup séquentiel de chaque part avec VSS snapshot unique
- [ ] Backup ID: `hostname_disk0_part1`, `part2`, etc.

**Tests requis:**
- [ ] Test: 500GB disk → 5 parts de 100GB
- [ ] Test: Interrompre part3 → retry part3 seulement
- [ ] Test: PBS déduplique bien entre parts (pas de duplication chunks)
- [ ] Test: Restore fonctionnel (recoller les parts)

**Timeline:** Phase 3 (après NTFS Fidelity + Resume Logic)

---

### Chiffrement (Phase 3)
- [ ] **Key Management**
  - [ ] Génération clé asymétrique
  - [ ] Stockage: Windows Credential Manager (DPAPI)
  - [ ] Export: bouton "Sauvegarder clé de récupération"

- [ ] **GUI**
  - [ ] Checkbox "Activer chiffrement"
  - [ ] Warning: "Sans la clé, restauration impossible!"

### 🆕 Restauration - À Développer FROM SCRATCH 🔄
**Status:** ❌ PAS IMPLÉMENTÉ - Code actuel = mock/stubs seulement

**État actuel:**
- ⚠️ Structure `RestoreOptions` existe (`restore_inline.go`) - code stub
- ⚠️ `ListSnapshotsInline()` existe - code basique non testé
- ⚠️ `RestoreManager` existe - **code mock complet** (`restore.go`)
- ❌ Aucune restauration fonctionnelle actuellement
- ❌ Pas de GUI de navigation dans les snapshots
- ❌ Pas de restore sélectif (fichiers/dossiers)
- ❌ Pas de restore ACLs/ADS (dépend NTFS Fidelity)
- ❌ Pas de CLI `proxmoxbackupclient-restore`

**Décision:** Feature majeure à développer après stabilisation du backup (NTFS Fidelity + Splitting)

**Scénarios à couvrir (voir doc/RESTORE_GUIDE.md):**

| Scénario | Méthode | Status |
|----------|---------|--------|
| Fichier supprimé | GUI restore granulaire | ❌ À implémenter |
| Dossier entier | GUI restore granulaire | ❌ À implémenter |
| Ransomware | Restore snapshot complet | ⚠️ Basique |
| Disque HS (bare-metal) | CLI restore + boot repair | ❌ À implémenter |
| P2V / Hardware différent | Fresh Windows + données | ✅ Possible (doc) |

---

#### Phase 1: Restore Granulaire GUI (dépend NTFS Fidelity P0)

**Prérequis:** NTFS metadata sidecar implémenté (Sprint 1)

- [ ] **Backend: Compléter `restore_inline.go`**
  - [ ] `RestoreGranular(opts RestoreOptions, filters []string)`
    - Télécharger PXAR depuis PBS
    - Parser PXAR et extraire fichiers matchant filters
    - Lire metadata sidecar `.nimbus_meta` pour chaque fichier
    - Appliquer ACLs avec `windows.BackupWrite()` (si `--restore-acls`)
    - Restaurer ADS (si `--restore-ads`)
    - Restaurer timestamps complets (Creation, LastAccess, Modified)

  - [ ] `ListSnapshotContents(backupID, snapshotTime)`
    - Liste l'arborescence complète d'un snapshot
    - Retourne structure tree pour UI (folders + files)

  - [ ] `DownloadPXARPartial(path, filters)`
    - Optimisation: ne télécharger que les parties nécessaires du PXAR
    - Éviter de télécharger 500GB pour restaurer 1 fichier

- [ ] **Frontend: UI de navigation snapshots**
  - [ ] Onglet "Restauration" dans GUI
  - [ ] Liste déroulante des snapshots disponibles (date/heure)
  - [ ] Treeview navigation dans l'arborescence du snapshot
  - [ ] Checkboxes pour sélectionner fichiers/dossiers
  - [ ] Champ "Destination" (path picker)
  - [ ] Options avancées:
    ```
    ☑️ Restaurer les permissions NTFS (ACLs)
    ☑️ Restaurer les flux alternatifs (ADS)
    ☑️ Restaurer les timestamps complets
    ☑️ Écraser les fichiers existants
    ```
  - [ ] Barre de progression + estimations
  - [ ] Bouton "Restaurer"

- [ ] **Tests**
  - [ ] Test: restaurer 1 fichier avec ACL custom
  - [ ] Test: restaurer dossier avec sous-arborescence + permissions
  - [ ] Test: restaurer fichier avec ADS (Zone.Identifier)
  - [ ] Test: restaurer 10GB, vérifier progress accurate
  - [ ] Test: restaurer sur chemin avec espaces/accents

**Temps estimé:** 1 semaine (après NTFS Fidelity Sprint 1 complété)

---

#### Phase 2: CLI `proxmoxbackupclient-restore` (bare-metal)

**Use case:** Restauration depuis Linux live (SystemRescue) après crash disque

- [ ] **Créer package CLI séparé** `cmd/proxmoxbackupclient-restore/`
  ```go
  package main

  import (
      "flag"
      "fmt"
      "pbscommon"
      "restore"
  )

  func main() {
      server := flag.String("server", "", "PBS server URL")
      fingerprint := flag.String("fingerprint", "", "Cert fingerprint")
      authID := flag.String("auth", "", "Auth ID (user@realm!token)")
      secret := flag.String("secret", "", "Token secret")
      datastore := flag.String("datastore", "", "Datastore name")
      snapshot := flag.String("snapshot", "", "Snapshot ID or 'latest'")
      dest := flag.String("dest", "", "Destination path")
      restoreACLs := flag.Bool("restore-acls", false, "Restore NTFS ACLs")
      restoreADS := flag.Bool("restore-ads", false, "Restore ADS")
      include := flag.String("include", "", "Include pattern (glob)")
      exclude := flag.String("exclude", "", "Exclude pattern (glob)")
      dryRun := flag.Bool("dry-run", false, "Simulate without restoring")

      flag.Parse()

      // Validate required flags
      if *server == "" || *authID == "" || *secret == "" {
          fmt.Println("Error: --server, --auth, --secret required")
          flag.Usage()
          os.Exit(1)
      }

      // Execute restore
      opts := restore.RestoreOptions{
          BaseURL:      *server,
          AuthID:       *authID,
          Secret:       *secret,
          Datastore:    *datastore,
          SnapshotID:   *snapshot,
          DestPath:     *dest,
          RestoreACLs:  *restoreACLs,
          RestoreADS:   *restoreADS,
          Include:      *include,
          Exclude:      *exclude,
          DryRun:       *dryRun,
      }

      if err := restore.ExecuteRestore(opts); err != nil {
          fmt.Printf("Restore failed: %v\n", err)
          os.Exit(1)
      }

      fmt.Println("Restore completed successfully")
  }
  ```

- [ ] **Cross-compile pour Linux**
  - [ ] Build static binary (CGO_ENABLED=0)
  - [ ] Inclure dans ISO SystemRescue custom (optionnel)
  - [ ] Doc: comment copier sur clé USB Linux live

- [ ] **Tests**
  - [ ] Test: restore complet depuis Linux live vers /mnt/windows
  - [ ] Test: option `--include "Users/**"` filtre correct
  - [ ] Test: `--dry-run` ne touche pas le filesystem
  - [ ] Test: restore 500GB, monitoring progress

**Temps estimé:** 3-4 jours

---

#### Phase 3: Documentation & Guides

- [ ] **Créer `docs/RESTORE_GUIDE.md`** (fourni ci-dessus)
  - Guide complet avec tous les scénarios
  - Matrice de décision
  - Commandes CLI détaillées
  - FAQ troubleshooting

- [ ] **Créer `docs/BARE_METAL_RESTORE.md`**
  - Guide pas-à-pas avec screenshots
  - Partitionnement GPT/MBR
  - Réparation bootloader Windows
  - Drivers et premier boot

- [ ] **Créer `docs/P2V_MIGRATION.md`**
  - Limites Windows (hardware change)
  - Méthode recommandée (fresh install + données)
  - Méthode alternative (safe mode + drivers)
  - Taux de succès selon scénarios

- [ ] **Intégrer dans GUI**
  - [ ] Bouton "?" dans onglet Restauration → ouvre RESTORE_GUIDE.md
  - [ ] Liens contextuels vers doc selon scénario

**Temps estimé:** 2 jours (rédaction + intégration)

---

#### Roadmap Restauration

```
Sprint 1 (en cours): NTFS Fidelity ← BLOCKER pour restore ACLs
  ↓
Sprint Restore-1 (1 semaine): GUI restore granulaire
  ↓
Sprint Restore-2 (3-4 jours): CLI proxmoxbackupclient-restore
  ↓
Sprint Restore-3 (2 jours): Documentation complète
```

**Total:** 2-3 semaines (après NTFS Fidelity complété)

**Priorité:** 🟠 P1 - Feature majeure utilisateur
**Blocker:** NTFS Fidelity doit être fait d'abord (sinon restore incomplet)

### Mode Entreprise (Phase 5)
**→ DÉPLACÉ EN P1** (voir "API Remote - Provisioning Distant")

---

## 🗑️ DROP (Ignoré pour l'instant)

- ❌ UUID machine (hostname suffit)
- ❌ Heartbeat vers API distante (overkill)
- ❌ go-msi (WiX fonctionne)
- ❌ Mount FUSE/WinFSP (restauration web suffit)

---

## 📅 Roadmap suggérée (Mise à jour post-audit)

### 🎯 Roadmap basée sur Audit Technique Mars 2026

**Sprint 1 (2 semaines) - NTFS Fidelity** 🔴 P0
- Implémenter `pkg/ntfs/backup_stream.go` (BackupRead wrapper)
- Modifier `pxar.go` pour stocker metadata NTFS sidecar
- Implémenter restore avec BackupWrite
- Tests: round-trip ACL + ADS
- **Livrable:** Backups Windows avec metadata NTFS complets (ACLs, ADS, timestamps)
- **Blocker Business:** Restaurations actuelles perdent les permissions - inacceptable entreprise

**Sprint 2 (1 semaine) - Secrets DPAPI** 🟠 P1
- Implémenter `pkg/secrets/dpapi_windows.go`
- Modifier `config.go` Load/Save avec chiffrement
- Migration auto: plaintext → DPAPI au premier Load
- Tests: service LocalSystem peut déchiffrer
- **Livrable:** Credentials PBS chiffrés avec DPAPI Windows
- **Note:** Défense en profondeur - admin local a déjà accès potentiel aux datastores

**Sprint 3 (1 semaine) - Splitting Récursif + Retry** 🟠 P1
- Améliorer `AnalyzeBackupDirs()` → récursif si folder >100GB
- Retry logique par split (pas de checkpoint complexe)
- UI: suivi multi-splits avec retry des échecs
- Tests: gros arbre de dossiers imbriqués
- **Livrable:** Backups découpés finement + retry granulaire au niveau split

**Sprint 4 (1 semaine) - Polish** 🟢 P2
- Code signing (achat cert + CI/CD)
- Progress worker (rate limiting UI)
- Tests finaux
- Documentation
- **Livrable:** v1.0.0 Production Ready

**Timeline Total:** 4-5 semaines (1 mois)
**Note:** Approche simplifiée avec Splitting Récursif (pas de checkpoints complexes)

---

### 📊 Scores Audit Technique (Mars 2026)

| Domaine | Score Actuel | Score Cible v1.0 |
|---------|--------------|------------------|
| Core Backup Engine | 7/10 | 8/10 |
| NTFS Fidelity | 2/10 | 9/10 ✅ Sprint 1 |
| Security (Secrets) | 4/10 | 8/10 ✅ Sprint 2 |
| Security (Encryption) | 0/10 | 5/10 (v1.1) |
| Resilience (Resume) | 3/10 | 9/10 ✅ Sprint 3 |
| Architecture | 7/10 | 8/10 |
| UX/Distribution | 5/10 | 9/10 ✅ Sprint 4 |

**Score Global Cible:** 8/10 pour v1.0.0

---

### 📚 Tests Critiques à Ajouter

**NTFS Round-trip Tests** (`pkg/ntfs/backup_stream_test.go`):
- [ ] TestBackupRestoreACL - Fichier avec ACL custom
- [ ] TestBackupRestoreADS - Fichier avec Alternate Data Streams
- [ ] TestBackupRestoreDOSAttributes - Hidden/System/Archive
- [ ] TestBackupRestoreTimestamps - CreationTime/LastAccessTime

**DPAPI Tests** (`pkg/secrets/dpapi_test.go`):
- [ ] TestDPAPIRoundTrip - Chiffrement/déchiffrement
- [ ] TestDPAPIServiceAccount - LocalSystem peut déchiffrer
- [ ] TestConfigMigration - Config legacy → DPAPI

**Splitting Récursif Tests** (`gui/backup_analysis_test.go`):
- [ ] TestRecursiveSplitting - Dossier 850GB avec sous-dossier 700GB
- [ ] TestDeepNestedFolders - Arbre profond avec folders >100GB imbriqués
- [ ] TestLeafFolderTooLarge - Dossier final avec 1 fichier de 500GB (edge case)
- [ ] TestSplitRetryLogic - Échec d'un split → retry seulement celui-là

---

**Dernière mise à jour:** 2026-03-25 (Audit Technique intégré)
**Mainteneur:** Proxmox Backup Client GO contributors
**Référence:** Audit `🔬 Proxmox Backup Client - Audit Technique pour Développeurs`
