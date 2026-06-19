# Nimbus Backup - TODO

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
**Problème:** API tokens PBS stockés en plaintext dans `C:\ProgramData\NimbusBackup\config.json`
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
  - [ ] Service log dans `C:\ProgramData\Nimbus\logs\service.log`
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
    msiexec /i NimbusBackup.msi /qn CONFIGFILE="\\ad-server\deploy\nimbus\config.json"
    ```
  - [ ] Property WiX: `CONFIGFILE` (chemin vers config.json)
  - [ ] CustomAction WiX:
    - Si `CONFIGFILE` fourni → copier vers `C:\ProgramData\Nimbus\config.json`
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
  - [ ] Nettoyer `C:\ProgramData\Nimbus` (option: garder config)

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
          trusted-signing-account-name: NimbusBackup
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
          trusted-signing-account-name: NimbusBackup
          certificate-profile-name: Production
          files-folder: build/installer
          files-folder-filter: msi
          file-digest: SHA256
          timestamp-rfc3161: http://timestamp.acs.microsoft.com
          timestamp-digest: SHA256

      - name: Verify Signatures
        run: |
          signtool verify /pa /v build/bin/NimbusBackup.exe
          signtool verify /pa /v build/installer/NimbusBackup.msi

      - name: Submit to Microsoft Security Intelligence
        run: |
          # Soumettre binaire signé à MS pour analyse (éviter faux positifs Defender)
          curl -X POST "https://www.microsoft.com/en-us/wdsi/filesubmission" \
            -F "file=@build/bin/NimbusBackup.exe" \
            -F "type=FalsePositive" \
            -F "comment=Proxmox Backup Client - Signed software, accesses raw disk via VSS for backup imaging"
        continue-on-error: true  # Ne pas bloquer release si API MS down

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            build/bin/NimbusBackup.exe
            build/installer/NimbusBackup.msi
          body: |
            ## 🔐 Code Signed
            This release is signed with **Azure Trusted Signing**.

            Verify signature:
            ```powershell
            Get-AuthenticodeSignature NimbusBackup.exe | Format-List
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
**Use case:** MSP gère 100+ clients Nimbus depuis interface centrale

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
- ❌ Pas de CLI `nimbus-restore`

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

#### Phase 2: CLI `nimbus-restore` (bare-metal)

**Use case:** Restauration depuis Linux live (SystemRescue) après crash disque

- [ ] **Créer package CLI séparé** `cmd/nimbus-restore/`
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
Sprint Restore-2 (3-4 jours): CLI nimbus-restore
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
- ❌ Heartbeat vers API RDEM (overkill)
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
**Mainteneur:** RDEM Systems
**Référence:** Audit `🔬 Nimbus Backup Client - Audit Technique pour Développeurs`
