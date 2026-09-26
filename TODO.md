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

**⚠️ CORRECTED 2026-09-26 — this whole section was written for the March 2026 audit and is now
half stale.** Two things happened since without this section being updated: verified against
current code (`git show`, not assumption), not just re-describing the old plan.

#### ~~Étape 0 : Métadonnées de backup~~ ✅ DONE
`.nimbus_backup_meta.json` (now `.proxmox_backup_client_meta.json`) is fully implemented, backup
and restore: written at `gui/backup_inline.go:1502`, read back at `gui/restore_inline.go:650`
(`tryReadBackupMeta`), and does exactly what this section originally asked — shows the real
original folder path in the restore UI instead of the sanitized backup-id. Gracefully falls back
to nil for legacy snapshots with no meta file. `gui/backup_meta.go:12`.

#### Étape 1 : NTFS Metadata Fidelity — ACLs/attributes ✅ DONE 2026-09-26, ADS/Creation Time still open

- ✅ **ACLs (Security Descriptors) and DOS attributes: captured AND restored, end to end.**
  Capture: `gui/backup_meta_windows.go` (commit `ee3d6e0`) reads each file's SDDL string +
  owner/group/DACL (SACL deliberately excluded) plus DOS attributes during the pxar walk, uploads
  one gzipped JSON blob per snapshot (`proxmox-client-acls.json.gz.blob`, `gui/backup_meta.go:17`).
  Restore: `pbscommon.PBSClient.DownloadBlob` (the `UploadBlob` inverse) fetches it,
  `gui/restore_meta_windows.go`'s `applyNTFSMetadata` calls `SetNamedSecurityInfo`/
  `SetFileAttributes` per file, wired into `RestoreSnapshotInline` per archive after extraction.
  Opt-in via `RestoreOptions.RestoreACLs` — that field already existed end to end (frontend
  checkbox state → `RestoreSnapshot` → this field) but was permanently `false` behind a disabled
  "(coming soon)" checkbox; enabled it now that the sidecar it was waiting for exists.
  **Verified live**: a real file with a non-default DACL (explicit deny-Everyone-write ACE) and
  the Hidden attribute, backed up and restored through a real PBS round-trip, both the DACL (SDDL
  string comparison) and the attribute matched exactly
  (`gui/zz_ntfs_acl_restore_livetest_test.go`).
- ❌ **Alternate Data Streams** (e.g. `Zone.Identifier`): still genuinely untouched, no code
  anywhere, before or after the ACL work.
  - [ ] Needs both capture (add an ADS field to `FileMetaEntry` + a collector) and restore
        application — the checkbox for this already exists in the UI too, still correctly
        disabled/"(coming soon)" since nothing backs it yet.
- ❌ **Creation Time**: still untouched — only `ModTime` is ever captured/restored.
  - [ ] Genuinely greenfield on both sides.
- **UID/GID hardcoding** in `pbscommon/pxar.go`: unchanged (`uid: 1000, gid: 1000`), with an
  existing comment noting this is fine since the project targets Windows — separate from the
  Windows-specific SDDL/attrs blob, not something the ACL work touches.

#### ~~🐧 Linux side has no equivalent at all — POSIX ACLs / xattrs~~ ✅ DONE 2026-09-26

Turned out simpler than the original options below suggested: no cgo, no shelling out to
`getfacl`/`setfacl` needed at all. POSIX ACLs are themselves stored by the kernel as ordinary
extended attributes under reserved names (`system.posix_acl_access`/`system.posix_acl_default`,
a stable kernel ABI), so a generic xattr walk (`unix.Listxattr`/`unix.Getxattr`/`unix.Setxattr`)
captures and restores them for free without parsing that binary format at all.

`gui/backup_meta_linux.go` (capture) / `gui/restore_meta_linux.go` (restore) — same
`NTFSMetaCollector`/`applyNTFSMetadata` names the Windows implementation uses (shared naming
convention already established there), gated behind the same `RestoreOptions.RestoreACLs` flag,
new `FileMetaEntry.Xattrs map[string][]byte` field. Restore UI's ACL checkbox re-worded to be
platform-neutral instead of NTFS-specific, all 6 languages.

**Verified live** on the real Linux test client (`gui/zz_posix_acl_restore_livetest_test.go`):
set a real file's ACL via the actual `setfacl` tool (a named-user ACE) plus a generic `user.*`
xattr, backed up and restored through a real PBS round-trip, confirmed both came back correctly
— including checking the restored file with the real `getfacl` tool afterward, not just a raw
byte comparison, proving the kernel genuinely accepts what was written back, not just that bytes
were copied.

Went with a side-car blob matching the Windows approach (not PXAR's own native ACL/xattr entry
types) — simpler, and consistent with how the Windows side already works, at the cost of the
archives not round-tripping through a generic PXAR reader alone. `PXAR_ACL_USER` etc. in
`pbscommon/pxar.go` remain unused constants; native-entry support is still a possible future
alternative if that trade-off ever matters.

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

### ~~⏹️ No way to cancel a running machine backup~~ ✅ ALREADY WORKED — correcting an earlier wrong note (2026-09-25)

**Question raised (2026-09-25):** noticed there's no Cancel option once a full machine backup is
running. Is a clean cancel even possible without orphaning the block snapshot?

An earlier version of this entry claimed `runMachineBackupInline` never registers with the
shared cancel context and that Stop has zero effect on a running machine backup. **That was
wrong** — a from-scratch code read looked plausible enough to write down, but it wasn't checked
against real behaviour before being recorded. Verified empirically instead: a live test
(`gui/zz_cancel_machine_backup_livetest_test.go`, `TestCancelMachineBackupLive`) ran a real
machine backup of a real disk (`/dev/sda` on `pbstest-linux-client`) against the isolated test
PBS, called the exact same `CancelBackup()` the Stop button calls 4 seconds in, and
`RunBackupInline` returned in 4.3s total — a clean, fast cancel, not a 8-10 minute full run.
PBS's fixed-index writer correctly rejected the incomplete upload server-side (no partial
snapshot ever got committed), and `CreateVSSSnapshot`'s deferred cleanup
(`snapshot/linux_snapshot.go:356`, `snapshot/win_snapshot.go:82`) still runs regardless, so the
block snapshot itself is torn down properly either way.

Checked the frontend too: the Stop button's only condition is `disabled={!backupRunning}`
(`App.jsx` ~line 2813), and `setBackupRunning(true)` fires before `StartMachineBackup` the same
as it does for directory backups (~line 1441) — nothing machine-backup-specific gates it. Both
ends already work. Nothing to build here; the live test stays as a permanent regression check.

### ~~⚠️ Deleting a Backup Set has no confirmation prompt~~ ✅ FIXED 2026-09-25

Added the same `confirm(...)` pattern `handleDeletePBSServer` already used, with a new
`confirmDeleteJob` translation key (mirroring `confirmDeleteServer`) across all 6 languages,
naming the job being deleted.

### ~~📋 Clone a Backup Set~~ ✅ DONE 2026-09-26

New Clone button next to Edit/Delete on each Backup Set row, reusing Edit's field-population logic
but leaving `editingJobId` unset so Save creates a new job (`SaveScheduledJob`) instead of updating
the original (`UpdateScheduledJob`). Name defaults to "{name} (copy)" (all 6 languages), editable
before the first save. `lastRun`/history reset for free since the clone gets its own fresh ID.

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

#### ~~Branded builds could still have their accent color overridden via the Theme tab~~ ✅ DONE 2026-09-26

Prompted by tizbac asking (PR #78) whether filename-based branding (`gui/brand.go`, unchanged —
byte-identical to what's already in his repo) still works under this fork's restyling. Mick: "if
branding is applied could we potentially just disallow theme picking in the UI, not display the
option at all, so branded colours cannot be overridden." A real brand's accent is a deliberate
vendor choice, not a default — the Theme tab (Preferences) is now hidden entirely when
`brand.is_default` is false, and any theme saved BEFORE a build was rebranded is now ignored
outright at startup too (not just made unreachable going forward), so a stale saved theme can't
leak through either. Verified live: renamed the built exe to `NimbusBackup.exe` with zero rebuild —
title bar and accent immediately switched to the Nimbus identity, confirming the filename mechanism
itself needs no PR (already identical upstream); the actual gap tizbac was probing is the newer
Theme Picker layered on top, which does NOT exist upstream yet — a PR for the general UI/theme work
is under discussion, not yet started.

#### ~~Progress bar/total size not aggregated across multiple directories~~ ✅ FIXED 2026-09-26

**Mick (2026-09-26):** backed up one local folder + one network share (mixed job) — "it still said
total 1.1GB but the end file was 1.2GB with both folders. Maybe the progress bar isn't aggregating
both folders sizes for the progress?" Confirmed: exactly that. Each directory's background
size-scan populated a PRIVATE, per-directory `*atomic.Uint64` fed straight into that directory's
own `ChunkState` — the live "Data: X / Y MB" readout and percentage were always scoped to
whichever ONE directory was currently being archived, never the sum across the whole job.

**Fix:** new `jobProgress` struct (`gui/backup_inline.go`), one instance shared across every
directory in an attempt: `sizeEstimate` (every directory's own background-scanned size, summed as
each one's scan completes) and `bytesDoneBase` (bytes already archived by directories that
finished before the current one — repurposes the pre-existing job-wide `totalSize` accumulator
that already tracked exactly this for the final report, now also feeding the LIVE baseline). The
backwards-progress guard moved from per-`ChunkState` (reset fresh each directory) to `jobProgress`
itself, so it still holds across a directory boundary.

**Verified live**: two directories of known, different sizes (3MB + 15MB) backed up in one job —
`BytesTotal` reported during the run matched the combined 18MB exactly, never just one directory's
own size. Kept as a permanent regression test (`gui/zz_progress_aggregation_livetest_test.go`).

**Follow-up found live 2026-09-26** (same day, real mixed local+network job on pbstest-winclient):
Mick — "it seems to show progress in two passes and separately for each folder, rather than
showing the combined progress." The math itself was already correct at every moment (screenshot 1:
network folder mid-run, total 1100MB — its own size, since the local folder's scan genuinely hadn't
started yet; screenshot 2: local folder's turn, total 1245MB — 1100+145, the true combined size,
confirming the aggregation fix above DID work), but each directory's background size-scan only
launched once that directory's own turn in the loop arrived, so the combined total only became
fully known partway through the job — visibly two distinct "totals" rather than one number that
holds steady from the start. Fixed by hoisting every directory's background scan to launch
concurrently right when the attempt begins (instead of sequentially, one per directory, as its own
turn comes up) — the combined total is now normally fully known well before the first directory
even finishes. Re-verified against the same regression test after the change: still exactly 18MB.

#### ~~VSS/snapshot fired for network-share backup dirs and failed outright~~ ✅ FIXED 2026-09-26

**Found live 2026-09-26**, immediately after the tree-picker network-drive fix below: created a
Backup Set pointed at `\\DEEPTHOUGHT\backup-8000gb\Test Files` with VSS on — failed outright:
`VSS_VOLUME_SUPPORT - snapshots are not supported for drive \\DEEPTHOUGHT\backup-8000gb\` (also
surfaced a `%!s(<nil>)` Go formatting artifact from the third-party `go-vss` library's own error
message, sidestepped rather than patched, since the real fix means that code path is never reached
for a network path at all). Mick: "we need to gate VSS to not fire for network shares m, including
if there is a mix of local and network drives."

**Fix:** new `pathSupportsSnapshot(path string) bool` (`gui/snapshot_path_windows.go` /
`_linux.go` / `_other.go`) — Windows checks `GetDriveType` on the path's volume root (handles both
a raw UNC path and a drive letter mapped to one, verified live against both forms); Linux checks
`Statfs` against known network/pseudo filesystem magic numbers (NFS/SMB/CIFS/FUSE) so a CIFS/NFS
mount gets the same graceful fallback rather than elastio-snap/dattobd failing to find a backing
block device. `backupDirectory` (`gui/backup_inline.go`) checks this **per directory**, not once
for the whole job — a job backing up one local folder and one network share still gets a real
snapshot for the local one; only the network path silently backs up live instead, logged clearly
rather than failing the whole run.

**Verified:** `pathSupportsSnapshot` tested live against real paths (both a raw UNC path and a
mapped network drive letter correctly return false; local drives correctly return true). Full
`go build`/`vet` clean on Windows and Linux. **Confirmed live end-to-end** (Mick, 2026-09-26):
re-ran the network-share Backup Set with VSS **on** this time — completed, correctly skipped the
snapshot for the network path automatically.

#### ~~Tree picker only shows local drives, not mapped network shares~~ ✅ FIXED 2026-09-26

**Mick (2026-09-26):** "the tree picker only lists local drives, so if i map a network shared it
doesn't present as available to choose a folder for backup." Confirmed Windows-specific (Linux
network shares are ordinary filesystem mounts, so they already work); confirmed the app was running
elevated ("Run as Administrator") when this was seen.

**Root cause:** `GetLogicalDrives()` (`dirlist_windows.go`) only reports drives visible in the
calling process's own logon session. A drive mapped in the normal interactive desktop session gets
a SEPARATE logon session when a process is later elevated via UAC ("Run as Administrator") — a
well-documented Windows behavior — so the elevated app's own `GetLogicalDrives()` call genuinely
can't see it at the OS level, confirmed via a direct Go test isolating just that one API call.
`GetDriveType` itself has no bias against network drives (`DRIVE_REMOTE` shows up fine when the
session isn't split) — the problem is purely session visibility, not drive-type filtering.

**Fix:** `listRoots()` now falls back to `HKCU\Network\<Letter>\RemotePath`, which records every
mapped drive per-user regardless of logon session, for any letter `GetLogicalDrives` missed. Points
the resulting root straight at the raw UNC target (`\\server\share`) rather than the drive letter,
since opening the UNC path directly establishes a fresh SMB connection using cached credentials —
verified this works correctly (`filepath.Join`/`Clean`/`os.ReadDir` all handle a real `\\server\share`
UNC root correctly, confirmed against a real live share). Purely additive: a letter `GetLogicalDrives`
already sees (the normal non-broken case) is left alone, so nothing changes when the app isn't
elevated or the drive genuinely isn't there.

#### ~~Reports: scheduled-job name reverted to generic "Manual backup - X" label~~ ✅ FIXED 2026-09-26

**Bug found live 2026-09-26:** ran "Backup with VSS" and "Backup without VSS" (two named Backup
Sets) via Run Now — both showed up in Reports as generic "Manual backup - pbstest-winclient"
instead of their real names.

**Root cause:** `executeScheduledJob` set `currentScheduledJobName`/`currentScheduledJobPostActions`
right before calling `StartBackup`, then cleared both via `defer` immediately after. In standalone
(GUI) mode `StartBackup` is fire-and-forget (the real backup runs in a goroutine and
`executeScheduledJob` returns immediately), so the defer cleared both fields before the backup even
started, let alone before `OnComplete` (main.go) read them seconds later to write the Reports entry.
Same bug therefore also silently broke the post-backup actions (email/run-app/shutdown) added for
item #7, in standalone mode specifically — not caught by that work's own testing, which never
exercised the Run Now path with more than one job's timing gap.

**Fix:** stopped clearing via `defer` right after the setter. The three genuinely-final points now
clear explicitly instead: `OnComplete` (main.go, the normal standalone case, once it has actually
consumed the values) or one of `executeScheduledJob`'s two synchronous branches (service mode; or
standalone's immediate pre-goroutine failure, where `OnComplete` never runs). Race-safety verified
via `operation_queue.go`: the next backup can't acquire the slot until the current one's `OnComplete`
has already finished, since the slot release is deferred around the same `RunBackupInline` call that
invokes `OnComplete` synchronously.

Also added, while in the area (Mick: "the detail in the report should add more info, such as the
mode Scheduled/Manual, VSS On/Off etc"): `JobHistory` gained `BackupType` ("directory"/"machine")
and `Trigger` ("scheduled"/"startup"/"manual"/"oneoff") fields, threaded through the same
set-at-start/clear-at-consumption lifecycle as the name fix above. Reports detail pane now shows
Mode and Type rows, and VSS reads "On"/"Off" text instead of a checkmark/dash. All new i18n keys
added across all 6 languages, careful to rename around a pre-existing `backupTypeDirectory`/
`backupTypeMachine` key pair (used elsewhere for the Backup Set type selector) rather than silently
overriding it — caught via a key-count check before it shipped.

**Verified live** on pbstest-winclient: re-ran both jobs after deploying the fix, Reports showed
"Backup with VSS"/"Backup without VSS" correctly.

#### ~~Email settings deserve their own Preferences tab~~ ✅ DONE 2026-09-26

Moved the whole SMTP/email-notifications section out of Preferences → Advanced into its own
"Email" tab (new `prefsEmail` i18n key, all 6 languages), leaving Advanced for the parallel-restore
toggle and settings export/import. Real SMTP details (found on polaris's Uptime Kuma notification
config, `mail.beebys.net:465`, `mums@beebys.net`) written into pbstest-winclient's `config.json`
directly so the tab isn't empty on next open.

#### Relabel restore-mode radio buttons

- [ ] "Restore in-place" → "Restore to original path"
- [ ] the other restore-mode option → "Restore to alternate path"

Find the relevant labels/keys in `App.jsx` + `translations.js` (all 6 languages) — likely the
restore-destination radio group on the Restore tab.

#### ~~Restore progress bar doesn't match the backup one~~ ✅ FIXED 2026-09-25
Restore now uses the same `.progress`/`.progress-bar` CSS classes as backup (30px, themed via
`var(--accent)`, percentage rendered inside the bar) instead of a bespoke 8px div hardcoded to
`#2563eb`. The transfer speed still shows as its own line below, since that's extra detail backup's
bar doesn't have, not a styling mismatch.

#### ~~VSS label in Preferences → Destination says "Windows Shadow Copy" only~~ ✅ FIXED 2026-09-25
`useVSS` label now reads "Use VSS (Windows Shadow Copy / Linux Snapshot)" (and equivalent in all
6 languages), since the same checkbox gates elastio-snap/dattobd on Linux as much as VSS on
Windows.

### ~~📧 Email notifications in the GUI~~ ✅ DONE 2026-09-25

Global SMTP account in Preferences > Advanced (`SetSMTPSettings`/`SendTestEmail`,
`gui/email_notifications.go`), reusing the existing `clientcommon/mail.go` engine. Per-Backup-Set
on-completion/on-failure email toggles live on the new Alerts tab (see below) — two independent
fields with their own recipient, not a single success/failure/always/never selector.

### ~~🖥️ Post-backup actions (shutdown PC, exit app, run an application)~~ ✅ DONE 2026-09-25

New "Alerts" chevron tab on the Backup Set editor (scheduled Backup Sets only), modelled on
Backup for Workgroups' "Special Items" step: on-completion/on-failure email (reusing the global
SMTP account above, each with its own recipient), run an application before/after the backup,
exit the app after the backup, shut down the computer after the backup — fired in that order,
shutdown always last since it's irreversible. New `ScheduledJob` fields are all-off zero values,
so pre-existing jobs need no migration (verified via a JSON round-trip check).

Fires from `executeScheduledJob`'s existing service-mode synchronous branch, and via a new
`currentScheduledJobPostActions` field (mirrors the existing `currentScheduledJobName` pattern)
read inside `startBackupDirect`/`startMachineBackupDirect`'s `OnComplete` for standalone mode.

**Verified:** `gui` builds clean under both default and `-tags service`; a full `wails build`
succeeds and generates the new bindings correctly; frontend build clean, all 6 languages have
matching key coverage; `ScheduledJob`'s new fields round-trip through JSON correctly, including
old-shape jobs loading with everything safely off.

**Not independently verified:** actual SMTP delivery against a real mail server, and live
GUI rendering/interaction (no reachable test client with a live desktop session at the time —
the JSX follows the exact same structural patterns as neighboring, already-working sections).
Worth a real click-through and a test email with real SMTP credentials before relying on this.

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
