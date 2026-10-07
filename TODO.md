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

### 🟡 Splitting Récursif - AMÉLIORATION 📂 — PARTIAL, audited 2026-09-29
**Problème actuel:** Le splitting ne descend qu'à 1 niveau de profondeur.
**Exemple:**
- Input: `D:\DATA` (850GB)
- Analyse: `D:\DATA\Richard` = 700GB, `D:\DATA\Autre` = 150GB
- **Résultat:** Les 2 splits sont encore >100GB → fragiles

**Audit 2026-09-29 (against actual code, not this doc):** `gui/backup_analysis.go`'s
`AnalyzeBackupDirs()`/`CreateSplitJobs()` still only bin-packs one level deep and puts an oversized
folder into its own "solo bin" — exactly the gap this section describes, still genuinely open. No
recursive descent into an oversized leaf exists yet. Separately, the "Retry logique par split"
task further down (and its own section below) was superseded by a different, simpler design — see
that section's own audit note, don't implement per-split retry.

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

**Frontend — ✅ DONE, confirmed by code audit 2026-09-29:**
- [x] ~~Page "Serveurs PBS" (liste avec CRUD)~~ ✓ `App.jsx` ~line 2200-2260, full add/edit/delete list
- [x] ~~Dropdown "Serveur PBS" dans formulaire backup~~ ✓
- [x] ~~Test connexion par PBS (bouton + indicateur 🟢/🔴)~~ ✓ `handleTestPBSConnection` + a per-server
  status dot (online/offline/testing/untested), plus set-default
- [x] ~~Migration jobs legacy vers PBSID~~ ✓

Nothing left to build here.

---

## 🟠 P1 - IMPORTANT (Architecture Entreprise)

### 🆕 Secrets en Clair dans config.json 🔒
**Problème:** API tokens PBS stockés en plaintext dans `C:\ProgramData\ProxmoxBackupClient\config.json`
**Risque:** Tout admin local ou malware peut lire les credentials PBS.
**Note:** Risque modéré - admin local pourrait avoir accès datastore de toute façon, mais DPAPI ajoute une couche de défense en profondeur.
**Référence audit:** Score 4/10 Security (Secrets)

**Localisation:** `gui/config.go` — `Save()` (now ~line 266-278) still does a plain
`json.MarshalIndent` + atomic write, no encryption step. Confirmed by code audit 2026-09-29: no
`pkg/secrets/dpapi_windows.go` exists anywhere in the repo. **Still genuinely open** — PBS API
tokens really are plaintext in `C:\ProgramData\ProxmoxBackupClient\config.json` today.

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

### ~~Splitting Récursif + Retry Granulaire ⏯️~~ — retry approach SUPERSEDED, audited 2026-09-29
**Problème:** Backups de gros volumes (>1TB) fragiles - tout recommencer si échec.
**Solution SIMPLE:** Splitter intelligemment + retry par split (pas de checkpoints complexes).
**Référence audit:** Score 3/10 Resilience (Resume)

**Audit 2026-09-29:** `backup_inline.go` has a deliberate, documented design decision (~line
876-914) to do whole-job retry instead of per-split retry, relying on PBS's own chunk-level dedup so
a retried job doesn't re-upload anything already on the server. **This was a conscious choice, not a
missed task** — don't build the "Retry logique par split" task below (per-split retry backend +
its own "Retry failed splits" button), that's superseded. **Correction 2026-09-30: "UI: Suivi
multi-splits" is a SEPARATE, still-genuinely-open gap** — showing "Split 3/10 in progress" and a
per-split ✅/⏳/❌ list doesn't depend on retry being per-split or whole-job; no matches for any
split-progress concept exist in `App.jsx` today. Don't drop that one. The recursive-descent half of
this section is tracked in the "Splitting Récursif - AMÉLIORATION" section above instead (still
genuinely open there).

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

### ~~Service Windows - Robustesse~~ ✅ DONE — audited 2026-09-29
- [x] ~~**VSS Cleanup au démarrage**~~ ✅ FAIT (2026-03-23)
  - [x] ~~Appel dans `service.run()`~~ ✓
  - [x] ~~Log les shadows supprimées~~ ✓
  - [x] ~~Build tags Windows/Linux~~ ✓

- [x] ~~**Working Directory fix**~~ — turned out unnecessary, not actually built as planned. Audit
  2026-09-29: `config.go`'s `getConfigDir()` resolves via the `%ProgramData%` environment variable
  as an absolute path, so config-loading was never CWD-dependent in the first place. The problem
  this task targeted doesn't exist; no `os.Chdir` was ever needed.

- [x] ~~**Logs accessibles**~~ ✓ done via this session's "View Logs" button work —
  `app_types.go`'s `GetLogsFolder()` (~line 141-148) + a Preferences → Advanced button.
  `log_rotation.go` (~line 17-22) confirms the exact 10MB/5-file rotation this task specced.

### MSI - Finitions — 🟡 PARTIAL, narrowed 2026-09-29 (only code signing is a real gap now)

- [x] ~~**Désinstallation avec choix config**~~ ✅ FAIT (2026-03-23), confirmed still present by
  code audit 2026-09-29 (`installer/wix/ProductBody.wxi` still has `KEEP_CONFIG`/
  `DeleteConfigFolder`)
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
  - **Audit note 2026-09-29:** unverified either way. No explicit `/quiet` handling in `build.bat`,
    but WiX's default `InstallUISequence` is normally skipped by `msiexec /qn` unless a blocking
    custom dialog is in the way — hasn't actually been tested against a real silent install.

- [ ] **Code Signing** — confirmed still genuinely NOT done by audit 2026-09-29:
  `.github/workflows/build-and-release.yml` explicitly says signing is "pending"/"on the way"
  (SignPath Foundation) at several lines (265, 405, 417, 485). No Authenticode cert, no signing step
  anywhere yet. Same gap as the "Code Signing - Windows Trust" section further down — don't track
  both separately, whichever gets picked up first should close both.
  - [ ] Signer le binaire `.exe`
  - [ ] Signer le `.msi`
  - [ ] Certificat: à obtenir (DigiCert/Sectigo ~300€/an, ou SignPath/Azure Trusted Signing — see
    below)

- [x] ~~**Désinstallation propre**~~ ✓ covered by the "Désinstallation avec choix config" item above
  (same WiX CustomAction stops the service and offers to clean `ProgramData`) — not a separate gap.

### ~~Fréquences de Backup Multiples ⏰~~ ✅ DONE — via a simpler design, audited 2026-09-29

**Audit finding:** the elaborate cron-library plan below (robfig/cron, full cron expressions) is
NOT what got built. Instead `scheduler.go`'s `ScheduledJob` (~line 20-79) has a simpler
`TriggerMode` ("daily"/"interval"/"manual"), `IntervalMinutes`, `WindowStart`/`WindowEnd`,
`DaysOfWeek`, wired up in `App.jsx` (~line 220, `triggerMode` UI state, confirmed present).
Functionally covers every real use case this section listed — hourly-ish via interval, daily,
weekly via days-of-week — without a real cron parser, and never needed a legacy-`ScheduleTime`
migration since it was built fresh rather than evolved from a single-daily-time field. **Nothing
left to build here.** Everything below is the original plan, kept for reference only — it doesn't
describe what actually shipped.

**Problème actuel (original, now resolved a different way) :** Scheduler supporte uniquement backup quotidien à heure fixe (HH:MM)
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

### ~~Multi-jobs - Stabilisation~~ ✅ DONE — audited 2026-09-29
- [x] ~~**Queue management**~~ ✓ `operation_queue.go`'s `acquireOperationSlot` serializes
  backup/restore jobs FIFO (no 2 VSS jobs simultaneously), extended this session with named-queue
  display in the UI, verified live with 3 simultaneously queued jobs on winclient.

- [ ] **Test de charge** — never formally run (5 concurrent jobs, RAM ceiling check), but the
  serialization mechanism itself is real and working, not a gap worth tracking on its own anymore.

---

## 🟢 P2 - NICE TO HAVE (Backlog)

### 🗑️ Undelete: restore files that are in the backup but gone from disk (idea from BFW, 2026-10-07)
Backup for Workgroups offers "undelete": compare a backup set with its live folders and list the files that were deleted, ready to restore. Maps directly onto PBS snapshots.
- [ ] Pick a Backup Set; list the files of its latest snapshot and check each against the live folders (stat only, no content read). Present the missing ones with size and last backed-up date, tick to restore to the original paths (existing selective restore, `RestoreModeOriginal`).
- [ ] "Deleted in the last N days": check the set's recent snapshots too and offer each missing file from the NEWEST snapshot that still has it.
- [ ] Cheap on split (Metadata/Data) snapshots: the file list is the small `.mpxar.didx`, no payload download. Legacy snapshots: use the catalog (written for every snapshot).
- [ ] Needs one backup ID per set (see below), otherwise the set's history mixes in other sets' snapshots.

### ⏪ Roll back: restore a named Backup Set as it was at a chosen date and time (idea from BFW, 2026-10-07)
BFW's "roll back" picks a set, shows its history and restores a point in time. PBS snapshots are exactly that; this is mostly presentation.
- [ ] Start from the Backup Set (not server, then backup ID, then snapshot): show its snapshots as a dated list, or take a date/time and use the snapshot at or before it.
- [ ] Restore to the original locations with an explicit policy: overwrite files that differ / keep files newer than the snapshot / TRUE rollback that also deletes files that did not exist then (off by default, confirmation showing how many files would be deleted).
- [ ] Multi-folder sets: one snapshot holds an archive per folder, restore them all (already how restore works).
- [ ] Depends on one backup ID per set.

### ⚠️ Warn when a Metadata Backup Set shares its backup ID with another set (2026-10-07)
Metadata change detection compares with the NEWEST snapshot in the backup group. On deepthought the production sets "Beeby Property", "Beeby Trading" and "Data" all used backup ID `deepthought`, so any other set running between two Data runs would have forced a full 448 GB read; fixed by giving Data its own ID `deepthought-data`.
- [ ] Backup Set editor: warn (not block) when a Data/Metadata set has the same server + backup ID as another set, and suggest a unique ID.
- [ ] Possibly default new sets to `<hostname>-<set name>` style IDs.

### \U0001F41B Selective restore from a SPLIT (metadata-mode) snapshot downloads most of the data between the selected folders (2026-10-07)
Seen live on deepthought: 4 folders from deepthought-data (480 GB, 158,801 payload chunks) to F:\TestRestore; after 6.5 min ~10,080 chunks (~30 GB) fetched vs ~7.9 GB written, "written" stalling while fetching continued. Diagnosis (branch pxar-v2-read):
- Main cause: `walkRange` calls `payloadSection(refOffset, refSize)` at every PXAR_PAYLOAD_REF BEFORE the include filter runs (pbscommon/pxar_reader.go ~596; filter `pathMatches` ~897/~1168). `payloadSection` (~361) does a 16-byte ReadAt to check the payload header, which fetches the chunk at that file's start and fires a 32-chunk read-ahead. Payloads are packed in walk order, so unselected regions under ~96 MB are downloaded completely and each larger unselected file costs ~33 chunks.
- Multi-include restores use the full `walk()` (~1081, ~1265), visiting every entry (not the goodbye-table fast path).
- Read-ahead is never bounded for split or multi-select: `LimitPrefetchTo` is only called under `len(archiveIncludes) == 1 && !split` (gui/restore_inline.go ~516); `triggerPrefetch` runs on every chunkAt, hits included (pbscommon/didx_reader.go ~258, ~344).
- Cache of 64 chunks (restore_inline.go ~429) vs 4 parallel workers + walker each reading ahead 32 => evictions and refetches; every refetch increments `fetched` (didx_reader.go ~329).
- "Written" adds the weight of unselected entries as the walker passes them (`entryDone` ~1052/~1238), and a selected file only counts when complete, so it stalls on large files and is not real bytes written.
- [ ] Make the payload header check lazy (on first Read of a SELECTED file) or skip payload refs the include filter rejects. Expected to remove most of the over-fetch.
- [ ] Multi-include fast path: ResolveArchivePathBST per include, sort spans, walkRange each; create parent dirs from each include's parent path.
- [ ] Split archives: one metadata-only pass to collect each selected file's payload byte range; replace `prefetchLimit` with an allowed-chunk-range set so read-ahead never leaves a selected range (or the current file).
- [ ] Progress: totals = sum of selected sizes / chunks in those ranges; "written" = real bytes of selected files only (see the restore-totals item).
- [ ] Cache >= (workers + 1) x 33 (~192 chunks) or smaller read-ahead in parallel mode; report refetches separately.
- [ ] Test on pbs-test: selective restore of 2-4 folders from a split snapshot, compare chunks fetched vs the chunk count of the selected ranges.

### 🐛 Restore: "Listing snapshots..." fades after 5 s while the request is still running (2026-10-07)
On deepthought, List available snapshots took ~75 s (PBS busy with a group verify); the info message vanished after 5 s (`showStatus` auto-hides non-persistent messages, App.jsx ~1003; the call at ~1916 does not pass `persist`), leaving a blank screen that looked like "no snapshots". Mick clicked three more times, sending overlapping ListSnapshots calls that all returned together.
- [ ] Keep a visible loading state for the whole call: an indeterminate LINEAR progress bar (sliding stripe, same green as the backup bar) in place where the results will appear, with "Listing snapshots... 12s" under it (persistent, elapsed seconds); one shared component for every server call that can take more than a second; after ~15 s add "PBS is slow to respond (a verify, GC or backup may be running)".
- [ ] Disable List available snapshots / Search for a file while a call is in flight, and ignore responses from superseded requests (request id).
- [ ] Audit other slow calls for the same pattern: Search for a file, opening a snapshot / directory in the tree, restore preview.

### 🐛 Stop logs "no backup running" while it is in fact stopping one (2026-10-07)
- [ ] On deepthought, Stop at 08:14:48 logged `CancelBackup: no backup running (or already cancelled)`, yet the run did cancel at 08:15:30 (the walk only checks between files and a large file was in progress). Find which state CancelBackup checks and fix the message; consider showing "Stopping after the current file..." in the UI.

### 🕒 Progress cards: show Started and Forecast finish as clock times (Mick, 2026-10-07)
On a long run (deepthought-data, 478 GB, several hours) "Time remaining 2h 14m" means working out the clock time yourself. Show wall-clock times next to the h:m:s durations.
- [ ] Backup card: add "Started: Wed 07/10 15:20" (from the run's start time, local clock) and "Forecast finish: Wed 07/10 20:45" (now + ETA), alongside the existing Elapsed and Time remaining. Include the day when the finish falls on a different date than today.
- [ ] Restore card: it currently shows only Speed and Data size (App.jsx restore grid), no Elapsed or Time remaining at all. Add Started, Elapsed, Time remaining and Forecast finish, computed from bytesDone/bytesTotal the same way as the backup card.
- [ ] ETA is currently bytesDone / total elapsed, an average since the start. Consider a smoothed recent rate (e.g. last 5 to 10 minutes) so the forecast reacts when the rate changes (dedup-heavy start vs new data, a slow USB disk).

### 📐 Progress cards: fixed layout from the start, no fields appearing and shuffling (Mick, 2026-10-07)
Every field in the backup card's 2-column grid is conditional (`eta !== null`, `speed > 0`, `startTime`, `bytesDone > 0`, chunk counts > 0, `currentDir`), so fields pop in one by one and the grid reflows, moving items between columns. The restore card has the same pattern.
- [ ] AGREED LAYOUT (Mick, 2026-10-07; mockup https://claude.ai/artifact/Xm269Bs7Fc7X1Tp36XWk45, "Proposed" artboards), top to bottom:
  1. Title + percent, progress bar (unchanged).
  2. 2-column grid: Started / Forecast finish; Elapsed time / Time remaining; Folder / Speed.
  3. Full width, directly under Folder: "Current file:" + path in the existing monospace style, fixed height, one line (no wrap).
  4. The existing grey "Processed:" box, in GB (GiB) form with New / Reused (/ Failed) chunk counts: "Processed: 357.9 GB (333.3 GiB) / 480.7 GB (447.7 GiB) (New: 104,587, Reused: 17,868 chunks)".
  5. Stop button.
  The separate "Data:" and "Chunks:" rows are removed (the Processed box carries both). Before values exist: "Calculating..." (Forecast finish, Time remaining, Speed, Processed total), Current file "Scanning <folder>...".
- [ ] Percent does not match the data: backup_inline.go maps bytes onto 10%..90% (`0.1 + done/total*0.8`, capped at 0.9), so 442.2 of 447.7 GB (98.8%) shows "89%", and 333.3 of 447.7 (74%) showed "70%". The bar then sits at 89-90% through finalization and jumps to 100%. Show the byte fraction as the percent during the data phase (keep a separate stage line such as "Finishing: uploading indexes..." for the tail), so percent, Processed and Time remaining agree.
- [ ] Metadata run with a previous snapshot (deepthought-data second run, 2026-10-07 19:40) had NO total for its first few minutes: at 2m46s "Data: 135.1 GB" with no "/ total", no Time remaining, and the percent on the indeterminate creep (`0.1 + (pos MB % 100)/1000`, capped 50%), so "20%" meant nothing; by 4m47s the background size walk had finished and "234.6 GB / 447.7 GB", 52%, 4m20s remaining appeared. On a fast reuse run the walk is a large share of the run. The previous snapshot's file list (761,866 files, loaded in 11 s at the start) already gives a total size: use it as `sizeEstimate` until the walk finishes, so the total is there from the first second.
- [ ] Two speeds, FreeFileSync style (Mick, 2026-10-07): "870.6 MB/s" on a change-detection run counts unchanged files reused WITHOUT reading, which is a fair "how fast is it getting through the job" figure (1.4 GB/s vs 31 MB/s on a full read) but says nothing about the network. Show both:
  - Processing: bytes covered per second (read or reused) + files per second, e.g. "Processing: 1.4 GB/s (1.3 GiB/s) · 2,165 files/s". Files/s is the real cost driver in metadata mode (a file costs about the same at 1 KB or 10 GB).
  - Upload: bytes actually sent to PBS per second (new chunks only), e.g. "Upload: 0.1 MB/s (0.1 MiB/s)"; zero when every file is reused.
  - Rates are measured over a short rolling window (e.g. last 5 to 10 s), not averaged since the start (Mick): Upload especially, so it shows what the link is doing now (bursts of new data, then zero across reused files). Time remaining / Forecast finish can keep a longer smoothed rate (minutes) so they do not jump about.
  - Add a "Files: 268,500 / 761,865" row; the total comes from the previous snapshot's file list (or the size walk) like the byte total.
  - Grid becomes: Started / Forecast finish; Elapsed time / Time remaining; Files / Processing; Folder / Upload; then Current file (full width). Mockup: "Proposed" artboards in https://claude.ai/artifact/Xm269Bs7Fc7X1Tp36XWk45 (figures illustrative).
- [ ] Always render the full set of rows in a fixed order; show "Calculating..." for values that are not known yet (ETA, speed, forecast finish) and "Not available" where a value cannot exist for this run (e.g. total size unknown, failed chunks on restore).
- [ ] Give the current-file and current-folder lines a fixed height (blank or "Waiting..." when empty) so the card does not change height.
- [ ] Same treatment for the restore card, including the stage line.
- [ ] Restore card totals are the WHOLE snapshot on a selective restore (deepthought, 2026-10-07, a few folders from deepthought-data to F:\TestRestore): "Data: 1.4 GB / 447.7 GB", "Restoring f__data.mpxar.didx: 1.9 GB of 447.7 GB written (501/158801 chunks downloaded)", and the percent sits at 5% (the floor of the 5%..95% restore range, i.e. no real progress). Cause: withSnapshotReader (restore_inline.go ~361) narrows totals to the selection (effectiveSize / effectiveChunks via ResolveArchivePathBST) ONLY when the includes resolve to exactly one clean target; this restore had includes=4, so it fell back to whole-archive totals. Only the selected files are actually fetched and written (lazy chunk reads; F:\TestRestore held just the first selected folder). Fix: sum the spans of every selected path (each resolves to its own range), dedupe overlaps, and use that as bytes/chunks total for the bar, the Data row and the message. Also name the archive being read correctly: on a split snapshot the file contents come from `.ppxar`, not `.mpxar`.
- [ ] Units: show sizes as "GB (GiB)", the same convention as Speed's "MB/s (MiB/s)" (`formatSpeed`, App.jsx), so both values reach the user. Today `formatBytes` (App.jsx) and `formatByteSize` (backup_inline.go) divide by 1024 but label the result KB/MB/GB/TB, so "447.7 GB" is really 447.7 GiB (480.7 GB). Apply to:
  - Backup card Data row: "Data: 357.9 GB (333.3 GiB) / 480.7 GB (447.7 GiB)".
  - Backup card "Processed: ..." status box (built in backup_inline.go `formatByteSize`), keeping the New / Reused (/ Failed) chunk counts: "Processed: 357.9 GB (333.3 GiB) / 480.7 GB (447.7 GiB) (New: 104587, Reused: 17868 chunks)".
  - Restore card Data row and the restore progress message (restore_inline.go, also `formatByteSize`).
  - Decide separately whether file lists, snapshot sizes and the restore selection total use the dual form or just the correct GiB label (dual may be too wide in list columns).

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

#### ⏳ IN PROGRESS: is deepthought's backup CPU-bound? (2026-09-30)

Mick's 447.7GB deepthought→PBS directory backup felt slow even though PBS dedup is doing almost
everything (log showed "New: 2, Reused: 1509" chunks) — investigated and found the read+hash pass
(`PXARArchive.WriteDir`, content-defined chunking over the whole serialized stream) is entirely
single-threaded; only chunk *upload* is parallelized (`chunkUploadWorkers = 8`,
`backup_inline.go`), which barely gets exercised when almost nothing is new. deepthought's CPU is
a weak 4-core/4-thread embedded AMD GX-420GI, making this plausible as the real bottleneck rather
than PBS-server-reboot cold-cache (also a real contributing factor that night, separately).

**Set up:** a CPU-sampling script (`C:\ProgramData\ProxmoxBackupClient\cpu_monitor.ps1`, logging to
`cpu-monitor.csv` every 2s: total system CPU%, per-core breakdown, and the
`ProxmoxBackupClient.exe` process's own CPU both normalized-to-machine and as "cores-worth") is
running on deepthought, started ahead of Mick re-running the backup under observation. **Next
step:** pull `cpu-monitor.csv` once Mick's run finishes and confirm whether it's actually pinned to
one core (~25% total system CPU, one core in the per-core breakdown near 100%) during the read+hash
phase — if confirmed, that's real evidence for eventually parallelizing the hash/chunk pass itself
(much harder than the upload-side parallelism already done, since content-defined chunking is
inherently sequential over the byte stream — would need a genuinely different chunking strategy to
parallelize safely).

#### 💡 IDEA, not started: skip re-reading/re-hashing files unchanged since the last backup

Raised 2026-09-30 while investigating the above. Real Proxmox VE's own speed-up for VM backups
(QEMU dirty bitmaps) doesn't apply here — no filesystem equivalent exists for an arbitrary
directory tree. More importantly: the actual official `proxmox-backup-client` (Linux-only, no
Windows build — exactly the gap this fork fills) does the SAME full re-read+re-chunk on every
run for host/file backups, relying purely on server-side dedup, same as this fork already does.
So this wouldn't be "catching up to Proxmox," it'd be a genuine NEW feature beyond what even
Proxmox's own client does.

**Why it's a real, multi-day job and not a quick tweak:** pxar serializes the whole tree into one
continuous byte stream and cuts chunks by content-defined (rolling-hash) boundaries — NOT per file,
which is exactly why "Reuse chunk" already works so well with zero explicit skip-logic (identical
bytes re-cut at the same boundaries). But it also means a single chunk can span the tail of one
file and the start of the next. To skip *reading* an unchanged file (not just skip *uploading* it)
would need: (1) fetching/parsing the previous backup's catalog into a per-path
size+mtime→chunk-digest map, (2) proving a run of unchanged files sits on a safe chunk boundary
(no adjacent changed file bleeding into the same chunk), (3) splicing old chunk references
directly into the new index instead of re-chunking that span, (4) still re-walking for metadata
(permissions/ACLs/timestamps) even for skipped file content. Real correctness risk if the splicing
logic is wrong (silently wrong chunk reference). Worth doing only if this class of large,
mostly-static dataset (like deepthought's share) is a recurring pain point — not a default
priority.

#### ✅ DONE 2026-10-02: pbs-vm (VM 206) RAM bumped 4GB to 8GB by Mick (verified 7947MB after reboot). Re-check buff/cache and backup speed after a few runs; 12GB is the next step if still cache-starved. Original note:

Mick added 8GB to proxmox07 (pm07) specifically so the PBS VM (VMID 206, `pbs-vm`) could be given
more — prompted by investigating whether a freshly-rebooted PBS server's cold page cache (only
3.8GB total RAM allocated to the VM today) was contributing to slow chunk-lookup performance during
a backup. proxmox07 now has 16GB free / 17GB available (host total 39GB, other VMs: theearth 6GB,
dockerhost-04 8GB). VM 206 is currently `memory: 4096` in its config — not yet resized. Need to
agree a target size with Mick (8GB? 12GB?) and confirm whether the VM's hotplug settings allow a
live bump or need a reboot, before actually applying it.

#### ~~Selective restore from a big combined archive fetches far more chunks than it writes~~ ✅ FIXED 2026-10-01

Mick, 2026-09-30: restored just the "Beeby-Property" subfolder out of the big combined "Deepthought
- Data" snapshot (458GB, `f__data.pxar.didx`) from pbstest-winclient — progress looked like it was
restoring the whole 458GB, not just the selected folder. He cancelled it (correctly cautious: that
VM's C: drive is only 32GB).

**Confirmed NOT a disk-space problem** — winclient's C: still had 12.8GB free after cancelling, so
nothing close to 458GB was ever being written. The real finding is in the actual restore log
(`service-gui.log` on winclient): `read header at offset 3927765450: ... (index 1079/129662):
... context canceled` — 129662 is the WHOLE archive's total chunk count. 1079 chunks fetched to
reach a ~3.9GB offset works out to ~3.6MB/chunk, almost exactly the ~4MB average chunk size, which
is the smoking gun.

**Root cause, confirmed by reading the actual code** (`pbscommon/pxar_reader.go`):
- `ExtractWithRewriter`'s `pr.walk()` visits EVERY entry in the archive sequentially, filtering by
  `pathMatches(e.Path, includes)` only AFTER reaching each one — not a targeted lookup.
- `pr.skip(n)` (used to step over a non-matching file's payload) is pure arithmetic
  (`pr.offset += n`) — genuinely free, confirmed by reading it.
- BUT `pr.readHeader()`/`pr.read()` (used for every file/dir's header, filename, entry struct —
  i.e. once per entry, continuously throughout the whole stream) go through the chunk-backed
  `pr.ra.ReadAt()`. PBS chunks are ~4MB and content-addressed — you can't fetch just 16 header
  bytes, any read at an offset not yet cached pulls the whole chunk covering it.
- Net effect: walking to reach one subfolder deep in a big combined archive fetches a fresh chunk
  roughly every ~4MB of forward progress REGARDLESS of whether that span belongs to a matching or
  skipped file, because the walk still has to read the next entry's header either way. For a large
  enough combined archive, this can approach the cost of reading through most of it — not writing
  it to disk (that part's correctly scoped), but genuinely fetching/decompressing most of its
  chunks over the network. The progress bar's whole-archive-size denominator (logged separately,
  see the progress-display finding below) was — in THIS specific sense — not lying as much as it
  looked at first.

**Two different fixes, very different scope:**
1. **Available today, no code change**: if the target folder also exists as its own separate
   Backup Set/archive (Mick has "Deepthought - Beeby Property" as one), restore from THAT archive
   directly instead of a subfolder selection inside the big combined one — walks only that smaller
   archive's own size.
2. **Real fix, DONE 2026-10-01**: `pbscommon.PXARReader.ResolveArchivePathBST` now uses the archive's
   existing GOODBYE binary-search-tree index (`ca_make_bst`, `pxar.go`) to jump straight to a named
   path component-by-component — only ever touching entries on the direct path to the target, never
   a sibling. `walk()` refactored into `walkRange(cb, startOffset, endOffset, initialPath, rootSeen)`
   so the target's own subtree still walks with the exact same state machine, just starting partway
   into the stream; every existing caller goes through `walk()` unchanged. `ExtractWithRewriter` uses
   this fast path only for a single clean selection, falling back to the proven linear walk on ANY
   failure (not found, unexpected table shape, I/O error) — read-only, so a bug here can only make a
   restore slower via fallback, never wrong. Verified live against the test PBS server
   (`gui/zz_selective_restore_bst_livetest_test.go`): a 512KB target folder inside an archive with a
   40MB sibling restored correctly with zero sibling leakage, and `ResolveArchivePathBST` resolved
   directly to the exact byte span 41.9MB into the archive — proof the sibling's chunks were never
   touched at all. Multiple-selection restores (2+ `IncludePaths` entries) still use the linear walk
   — a reasonable future extension, not needed for the common single-folder case this targets.

**The progress bar's denominator** ✅ FIXED 2026-10-01 — `restore_inline.go`'s `withSnapshotReader`
used to set `archiveSize` from `client.NewDIDXReaderAt`'s whole-archive size and send that straight
through as `bytesTotal`, regardless of `IncludePaths`. Now, when a single clean selection is given,
it calls the same `ResolveArchivePathBST` the BST fast path above uses to get the selection's real
byte span, and a new `pbscommon.DIDXReaderAt.ChunkCountInRange` (precise, via the same
`chunkIndexAt` lookup `chunkAt` itself uses — not an estimate) to get its real chunk count, and
reports progress against THOSE instead of the whole archive.

**One real wrinkle found live, now fixed too:** `ResolveArchivePathBST` itself fetches/caches
whatever chunks it touches while navigating the GOODBYE tables — those fetches fire the progress
callback BEFORE the narrowed totals exist, so without care the caller sees one misleading
whole-archive-sized reading before the narrowing takes effect, and if the actual extraction then
finds everything it needs already cached from resolution (common for a small target), no further
"new chunk" event ever fires to correct it — stuck showing the wrong number forever. Fixed by
suppressing progress reporting entirely during resolution (`resolvingSpan` flag) and emitting one
synthetic, correctly-narrowed update immediately after resolution completes, using the real
fetched-so-far count (`DIDXReaderAt.Stats()`) — so the caller's first-ever reading for a resolved
selection is already correct. Verified live
(`gui/zz_restore_progress_denominator_livetest_test.go`): a 512KB target inside a 42MB archive (big
sibling included) now reports exactly 524,529/524,529 bytes — the target's real size — not the
archive's.

**Follow-up: read-ahead ran past the selection, found live 2026-10-01 (commit after 04ee621).**
Mick restored "Beeby Property" (real source 4,189 files / 609 dirs / 5.756GB on
`\deepthought\media-5-1000gb\Data`) out of the 458GB "deepthought" production snapshot. Progress
read 5.8GB/1698 chunks (the estimate was RIGHT) but kept going to ~107% (2453/1698 chunks); Mick
stopped it there and the destination held only 3,737 files / 495 dirs / 4.69GB, i.e. the restore
was genuinely INCOMPLETE, not complete-with-a-wrong-estimate. Root cause: `DIDXReaderAt.triggerPrefetch`
(added 2026-09-23 for whole-archive restores) reads ahead 32 chunks past whatever it just resolved with no
idea the BST fast path had narrowed the walk to `[targetStart, targetEnd)`, so it fetched (and
counted toward progress) chunks belonging to whatever follows the selection in the archive, wasting
bandwidth/HTTP2 concurrency the real walk needed. Reproduced and fixed
(`gui/zz_bst_realistic_repro_livetest_test.go`): 7-chunk span with a 160MB file after it, unbounded
read-ahead fetched 40 chunks, bounded fetched 8. Fix: `DIDXReaderAt.LimitPrefetchTo(endOffset)` +
`SetPrefetchEnabled(false)` around `ResolveArchivePathBST` (its GOODBYE lookups are random access, so
read-ahead there is pure waste), both wired into `withSnapshotReader`; displayed `fetched` also
clamped to the total so the bar can never show >100% again. Zero value of the new fields = unbounded
read-ahead, so readers built without `NewDIDXReaderAt` (and whole-archive restores) behave exactly as
before (my first version got this wrong and broke `TestDIDXReaderAt_PrefetchOverlapsLatency`; caught
by the unit test run, fixed before commit).
**Update 2026-10-01 (live production run, build aec0af1):** read-ahead fix verified (chunks stopped at
1698/1698, bar clamped at 80%). Restored folder matched live: 4,185/4,186 files identical by path+size
(1 rename, 3 files added after the snapshot). BUT the restore kept fetching chunks after the folder
was written. Real cause: with `parallel_restore: true`, `ExtractWithRewriterParallel` had NO BST fast
path and always did a full linear `pr.walk` (129,662 chunks for a 1,698-chunk folder). Fixed by giving
it the same `ResolveArchivePathBST` + `walkRange` fast path (with fallback to full walk) as the
sequential extractor. Live test Step 5 (test PBS): span 7 chunks, fetched 8, 480/480 files. Still to
do: Mick to re-run the production restore on the new build; expect it to end promptly at 100%.

#### Restore progress bar should be properly representative (Mick, 2026-10-01, after the restore itself is verified)
DONE 2026-10-01 (awaiting Mick's live check). Bar now = bytes written / span size, mapped 0.20-0.95 across archives,
1.0 only at 'Restore completed'. Each PXAR entry carries a Weight; extractors call `PXARReader.SetProgressCallback`
when a file is fully written (parallel: from the worker). Live test asserts the callback reaches ~100% of the span.
Original analysis follows.
Bar % = 0.20 + 0.60*done/total (`restore_inline.go` ~1183), so it runs ~20 points ahead of the Data/chunks
line (live: bar 22% with 70/1698 chunks, 30% with 291/1698) and parks at 80% while files finish writing.
Make the percentage track real work (chunks fetched / span chunks, then files written / files total) and
show a distinct 'finishing, writing files' phase instead of sitting at 80%.
Mick's requirement: the bar must reach 100% only when everything is fully WRITTEN to disk, not when all
chunks are pre-fetched. Design: add an `OnFileWritten(bytes)` callback to `ExtractWithRewriter` and
`ExtractWithRewriterParallel` (called when each file is closed), drive the bar from bytes written /
effectiveSize (span size), reserve the last few points for NTFS ACL/attribute apply + summary, keep the
chunks/speed line as a separate download indicator.

#### Restores never appear on the Reports tab (reported 2026-10-01)
DONE 2026-10-01 (awaiting Mick's live check). `JobHistory` has Kind/Restore* fields; `appendJobHistory` records
success/failed/cancelled restores and the Reports tab shows snapshot, destination, paths, files, size, duration.
New labels are translated in all 18 languages (`tl()` in App.jsx keeps an English fallback for any future key).
Restore stage label under the bar (preparing / locating / transferring / acls via `restore:stage`); NTFS ACLs are
now applied after all archives, in the 95-100% stretch, with a live 'n of m files' count. Original analysis follows.
Reports = `GetJobHistory`, which is only written by backup completions (`main.go` ~1079/~1387,
`scheduler.go` ~944). Restores (success, fail or stopped) go only to the message log, so after a failed
restore the Reports tab's last entry is the previous scheduled backup. Fix idea: add a restore
`JobHistory` entry (type=restore, snapshot, dest, files/bytes, outcome incl. cancelled) from the
restore completion path in `restore_inline.go`, and label restore rows in the Reports list.

#### ✅ Restore skipped mtime for pre-1970 files (fixed 2026-10-01)
Found by diffing a restore against live: 78 files with source mtime 1969-12-31 23:59:59 (Unix -1) were restored with the restore time. The reader stores secs as uint64, `int64()` gives -1, and `e.ModTime > 0` skipped Chtimes (sequential and parallel paths in `pbscommon/pxar_reader.go`). Now `!= 0`. Directory mtimes not checked.

#### E2E results 2026-10-02 (winclient, vdev-d3644a9) and two open findings
Opt-in tests `gui/zz_e2e_directory_livetest_test.go` (`PBS_E2E=1`, optional `PBS_E2E_VSS=1`, `PBS_E2E_DIR`) and `gui/zz_e2e_machine_livetest_windows_test.go` (`PBS_E2E_SRC_DISK`, `PBS_E2E_DST_DISK`, test-PBS only).
Directory mode PASS: 3 backups (full, incremental with VSS + prefetch, unchanged), each restored sequentially and in parallel, 1915-1917 entries compared by sha256, size and mtime, 0 differences; partial restore of one folder, snapshot listing complete, progress monotonic.
Machine mode PASS: 256 MB NTFS VHD disk backed up via `\.\PhysicalDriveN`, fidx read back, all 64 chunks hash to their digests, image written to a second disk, every chunk reads back identical, 306 of 306 files identical, chkdsk clean.
Open findings (not fixed):
- A file whose mtime is exactly 0 (1970-01-01 00:00:00 UTC) restores with the restore time: the archive stores 0 as "no mtime", so `ModTime != 0` cannot tell them apart. Rare, needs a format-level flag to fix.
- FIXED 2026-10-02 (6373ee1, upstream PR tizbac#90 library, #91 GUI "host"): the VM config template used `{{.VMID}}` inside `{{range .Disks}}`, so `BackupType: "vm"` failed at the end. Unit test added. Verified live 02/10 with `PBS_E2E_TYPE=vm` (qemu-server.conf.blob uploaded, 64/64 chunks restored identical).

#### Backup: show the file currently being processed (Mick, 2026-10-02) - DONE 2026-10-02
Directory backups only report progress every 10 MB of data (`gui/backup_inline.go` ~446, `BackupProgressStats.CurrentDir` = the top-level folder being archived), so nothing visible moves while many small files or one big file stream past. Show the file path in the live stats.
- Take the path from the archiver as it starts each file (pxar encode loop), store it in an atomic string on the shared `jobProgress`, and have the existing 10 MB / timer tick read it into `BackupProgressStats` (e.g. `CurrentFile`). Do NOT emit a Wails event per file: a 100k-small-file job would flood the IPC bridge, same lesson as the snapshot-tree stall.
- Also tick on a time basis (about 1 per second) so a stuck or slow file still shows its name when no 10 MB boundary is crossed.
- With prefetch workers the "current" file is the one being encoded, not the one being prefetched.
- UI: one truncated-in-the-middle line under the progress bar on the running job, and in the Message Log only on error or slow-file events, not every file.
- Machine mode is block level, so it has no file names; keep device and partition plus bytes there.
- Implemented differently from the plan above: `PXARArchive.OnFile` -> atomic on `jobProgress` -> a 500 ms ticker in `runBackupInlineInternal` calls `BackupOptions.OnFile` only when the path changed -> Wails event `backup:file` (one small event per tick, not per file) -> one middle-truncated monospace line under the stats while running. VSS shadow paths are mapped back to the logical path. Verified live on the winclient (VSS run emitted only paths under the source). Message Log is untouched.

#### Experimental: machine backup as "vm" type (Mick, 2026-10-02) - BUILT 2026-10-02, awaiting a real PVE restore test
**Built (one-off backups only, English only via `tl` fallbacks):** one-off machine form has "Backup as (experimental)": Host (default) or Proxmox VE virtual machine. VM mode takes a VM ID (1-999999) plus an ID style, shows the resulting PBS backup ID, and warns. Styles: Reserved range = 9000000+ID (default), Leading zeros = 000+ID. The GUI sends `backupType` "machine-vm" with the computed numeric ID; `machineSnapshotType` (gui/machine_snapshot_type.go) maps that to PBS type "vm" in both the standalone and service paths. Scheduled machine backups stay "host". Still to settle by a real restore on a PVE node: do both ID styles list and restore, do `sataN` entries match the `drive-sataN` names, and which style is safer.
Fork only, not for upstream. GUI "Backup as" choice for machine backups: default "host" with the host name as backup ID; optional "vm" that needs a numeric VMID and is labelled experimental. Open questions to settle first by a real restore on a Proxmox VE node:
- Does PVE restore a `vm/<id>` snapshot written by this client (config `sataN` entries vs `drive-sataN.img.fidx` names, where N is the Windows disk number)?
- Windows UEFI/GPT will not boot as generated (no OVMF/EFI disk/TPM in the config); BIOS Linux or data disks are likelier to work.
- Clash avoidance with real VM backups in the `vm/<id>` namespace: leading zeros (`000107`, may still parse as 107 in PVE) versus a reserved high range (e.g. 9000107, valid numeric VMID). Test both; the GUI should add the prefix automatically, show the resulting ID, and warn.

#### Restore stage ideas (Mick, 2026-10-01: wants all three, pick up later)
Follow-ups to the stage label under the restore progress bar (`restore:stage`, built in `1245992`).
1. ✅ DONE (2026-10-02, pending Mick's test) **'Verify after restore' checkbox + function.** Implemented as SHA-256 of each payload while it is written (`PXARReader.SetHashFiles`), then `verifyRestoredFiles` (gui/restore_verify.go, 4 workers) re-reads size + hash from disk before the ACL phase. Bar: extraction 5-80%, verify 80-95%, ACLs 95-100%. Mismatch = restore reports an error, nothing deleted. Counts on Reports (`restoreVerified/restoreVerifyFailed`). Original spec: Optional post-restore pass that re-reads each restored file
   and compares it with what the snapshot holds (size + content hash against the archive, or at minimum the
   PXAR entry size/mtime), reported as its own stage ('Verifying restored files...') with n of m, and a
   pass/fail summary in the Message Log and on the Reports tab (add verified/mismatch counts to the restore
   `JobHistory` entry). Needs: a `VerifyAfterRestore` option on `RestoreOptions`, a checkbox on the restore
   options UI (+ 18 translations), a second read of the span (cheap when chunks are still cached) and a
   decision on what a mismatch does (flag, never delete). Bar: reserve a slice after the ACL phase, so 100%
   would then mean verified.
2. ✅ DONE (2026-10-01, pending Mick's test) **Current file name under the stage label.** Throttled `restore:file` event (or extend `restore:stage`
   detail) from the extractor callback, showing the path being written; truncate long paths in the middle.
   The sequential extractor knows the entry when it starts writing; for the parallel one use the most
   recently started file across workers.
3. **'Creating folders and setting timestamps' stage.** Name the tail of extraction (directory mtimes/attrs
   applied after files) as its own stage between 'Fetching chunks and writing files' and 'Restoring ACLs'.
   Check first whether that phase is long enough on a real restore to be worth a label.
   **DONE 2026-10-02 (pending Mick's live test).** Finding: folder mtimes were NEVER restored (only file mtimes; the e2e test skips folders, so nobody saw it). `gui/restore_dirtimes.go` `applyDirectoryTimes` now sets every restored folder's mtime from the snapshot after all files are written (deepest first, failures counted not fatal), under a new `dirtimes` stage ("Setting folder timestamps...", 18 languages). Unit test `restore_dirtimes_test.go`. The opt-in e2e comparison still skips folder mtimes; enable it next time the live e2e is run.

#### Snapshot picker says only "Loading or empty snapshot..." while a big snapshot's tree loads (idea 2026-10-01)

**DONE 2026-10-02 (2df46c9, pending Mick's timing test): step (3) lazy tree implemented. Backend `gui/snapshot_tree_index.go` keeps ONE snapshot's listing indexed in memory (children per folder + per-folder byte totals); `OpenSnapshotTree` returns only the count + top level, `ListSnapshotChildren` is called when a folder is expanded, `SnapshotSelectionBytes` computes the selection size server-side (debounced 150 ms). Folders with >1000 children render 1000 at a time with a Show-more button. The old `ListSnapshotContents` binding is still there (used by the e2e test and search). Known limit: a restore selection is still by path, unchanged.**

**Re-measured on vdev-d3644a9 (23:52 log): backend returned the 868,492 entries at +2s (cache hit) but 'Snapshot meta' only fired at +54s. The 'reading file list' elapsed counter stalls at ~10s (main thread blocked) then 'building file tree' shows. So the remaining wait is Wails IPC + JSON.parse of 868k entry objects on the UI thread, not the tree build. Next step is the lazy tree (backend returns top level / children of the expanded folder; selection by path prefix; selection size from the backend), or at minimum a compact columnar payload.**

**Status 2026-10-01: steps (1) and (2) below are IMPLEMENTED (memoised tree + cached Intl.Collator, ancestor-set `selectionBytes`, reading/building/empty placeholder with elapsed seconds and entry count, 18 translations), pending Mick's timing test on the deepthought snapshot. Step (3) lazy tree is still open if it remains slow. The 20% initial progress reserve was also shrunk to 2% then 5%.**

Mick: selecting the 458GB "deepthought" snapshot takes a long while to show its tree and the only
feedback is "Loading or empty snapshot..." (`loadingOrEmpty`, `gui/frontend/src/App.jsx` ~3807). Wanted:
show that a snapshot WAS found and that the content is being walked, ideally with progress.
Findings so far (no code changed): (1) `handleSelectSnapshot` clears `snapshotEntries` to `[]` and awaits ONE blocking
`ListSnapshotContents` call; the same placeholder is used for "still loading" and "genuinely empty",
so they cannot be told apart. (2) Backend `assembleSnapshotTree` (`gui/restore_inline.go`) tries the small
`catalog.pcat1.didx` first (`listSnapshotViaCatalog`, a single `ReadAt` of the whole catalog, no progress),
and only on failure falls back to `reader.ListEntries()` over the data archive, which is the multi-GB
walk. A slow load on a big snapshot is either a big catalog download or that fallback; the log line
"Catalog unavailable ... falling back to data-archive walk" says which. Cheap step: a separate
`loadingEntries` bool so the UI says "Snapshot found, reading file list..." instead of "empty". Better:
emit a Wails event from the backend (phase: catalog / archive walk, chunks fetched of total via
`ra.Stats()`, entries found so far) and render it under the spinner. Cache hit path already skips all of this.

**Measured 2026-10-01 (winclient log, deepthought 868,492-entry snapshot): the BACKEND IS NOT THE SLOW PART.**
Catalog fast path, 24MB catalog: 'Listing contents' 23:13:32 -> 'Listed 868492 entries' 23:13:33 (~1s). Yet the next
frontend step (`GetSnapshotMeta`, fired only after `ListSnapshotContents` returns and `setSnapshotEntries` runs)
logs 60-90s later on every load, cached or not. So the wait is Wails IPC of 868k entries as JSON plus the React
side: `buildTree(snapshotEntries)` is called inline in the render (App.jsx ~3830, not memoised), so it rebuilds a
Map over 868k entries on EVERY render (every status/progress/checkbox state change), and `selectionBytes`
(~2036) walks all entries too. Backend phase events would show a bar for the 1s part and nothing for the 60-90s.
Better plan: (1) `useMemo` the tree on `snapshotEntries`; (2) a `loadingEntries` state so the placeholder says
'Snapshot found, N entries, building tree...' (the count is known the moment the call returns); (3) consider
lazy tree: backend returns only the children of the expanded folder (or top level first), so 868k entries never
cross IPC at once. Time each stage (IPC vs buildTree vs first paint) before choosing between (1) and (3).

#### ~~Dev builds all showed "vdev" with no way to tell which commit is actually running~~ ✅ ADOPTED 2026-10-01

Mick: every build tonight showed "vdev" in the title bar (see any screenshot) — `gui/version.go`'s
`appVersion` defaults to the literal string "dev" and is only ever overridden via
`-ldflags -X main.appVersion=...`, which only the CI release workflow
(`.github/workflows/build-and-release.yml`) actually passes (using the git tag). Every ad-hoc dev
build/deploy tonight (local `wails build` and every remote SSH build) passed no `-ldflags` at all.

**Going forward:** every `wails build` I run (local or remote via SSH) passes
`-ldflags "-X main.appVersion=dev-<short-sha>"`, using `git rev-parse --short HEAD` at build time —
e.g. "dev-e2900c8". Shows up everywhere `appVersion` already does (title bar, debug log header,
`GetVersion()`, exported settings bundles, the service's own API server). Not retroactively applied
to anything already deployed tonight — starts from the next build. No code change needed, this is a
command invocation habit; if it's ever worth enforcing for anyone building (not just this session),
wiring it into `wails.json`/a wrapper script would be the next step, not done here.

**Wrinkle found 2026-10-01**: `wails build -ldflags "-X main.appVersion=dev-<sha>"` works fine on
Windows but fails on Linux (rigel, pbstest-linux-client) — `wails` v2.13.0's own ldflags-forwarding
appears to split the value on its internal space before handing it to `go build`, so the linker
only ever sees a bare `-X` with no attached value (`-X flag requires argument of the form
importpath.name=value`). Confirmed reproducible, confirmed plain `go build -ldflags "..."` directly
does NOT have this problem (only `wails build`'s own forwarding does). Workaround used for rigel/
pbstest-linux-client: `sed -i 's/var appVersion = "dev"/var appVersion = "dev-<sha>"/' version.go`,
build, then `git checkout -- version.go` to revert the source — never actually committed, just sets
the compiled-in literal for that one build. Windows builds keep using `-ldflags` directly, no issue
there.

#### ~~Two config tests were silently writing to the REAL system config path~~ ✅ FIXED 2026-10-01

Found while triaging an unrelated test-suite run: `TestConfigSaveLoad`/`TestGetConfigPath`
(`gui/config_test.go`) set `$HOME` to a temp dir, expecting `getConfigPath()` to resolve under it —
testing the OLD fallback behavior from before `getConfigDir()` was rewritten to resolve via
`%ProgramData%` first (deliberately, so the GUI and the Windows Service agree on one config
location — see the "Service Windows - Robustesse" entry elsewhere in this file). `$HOME` has zero
effect on Windows' real home-dir resolution OR on `%ProgramData%`-first logic, so
`TestConfigSaveLoad`'s call to `config.Save()` was actually writing to the REAL
`C:\ProgramData\ProxmoxBackupClient\config.json` on whatever machine ran it — confirmed live: it
clobbered this dev machine's real config with dummy test values (`pbs.example.com` /
`test@pbs!token` / `secret123`), timestamp matching exactly when the test suite was run tonight.
Not yet known whether that dev machine's config held anything real before — ask Mick.

**Fix:** both tests now redirect `%ProgramData%` itself to a temp dir for their duration (restoring
it after), so `getConfigDir()`'s real, unchanged logic resolves somewhere safe instead of the real
system path. Both pass and no longer touch anything outside their own temp dir.

#### ⚠️ UNRESOLVED: app froze (high CPU, totally unresponsive) after a machine backup completed on rigel

Mick, 2026-09-26/27: after "Rigel Full Machine Backup" (57m18s, `success=true`, clean completion
logged) finished, the app became completely unresponsive — no buttons worked, couldn't move or
close the window. `ps aux` confirmed the process itself at 123% CPU (still climbing minutes later),
not just a slow UI. Backend logs show nothing at all after `OnComplete` fired — no further log
lines, no errors, no repeated activity — meaning the Go business logic genuinely finished cleanly;
the sustained CPU was in the SAME process that also hosts the embedded WebKitGTK renderer (Wails
doesn't use Electron's separate-process model), which points at the JS/render side rather than a
Go-side loop, but no specific line of code was identified as the cause. Reviewed the
`backup:progress`/`backup:stats`/`backup:complete` event-handling `useEffect` (empty dependency
array, no resubscription loop, ordinary state updates) and found nothing obviously wrong. Today's
actual frontend diff (the Stop button addition) is minimal/additive and doesn't look like a
plausible cause on inspection either.

**Resolved for THIS session** by: `sudo kill` the frozen process via SSH (confirmed the backup's
own snapshot had already been cleaned up correctly, since it completed before freezing — no
`elioctl destroy` needed this time), restarting `anydesk.service` (didn't help), then a full reboot
of rigel (did fix it — AnyDesk reconnected and input worked again).

**Not root-caused.** Only reproduced once so far, on a real 57-minute machine backup reaching
genuine 100% — most of this session's testing used short synthetic backups via live tests, so this
may be the first time this exact code path (a real machine backup's OnComplete, at scale) has run.
Next time this happens: before killing, try capturing a goroutine dump (`SIGQUIT` on Linux writes
one to stderr) and check if WebKitGTK's own process/thread (if separately visible via `ps`) is the
one actually spinning, to narrow down Go vs. renderer definitively.

#### ~~Progress panel: four fields all show raw base units with no scaling as values grow~~ ✅ FIXED 2026-09-30

Mick, 2026-09-28: spotted live watching a real 478GB deepthought→PBS backup. Same gap in four
separate fields — strongly suggests one shared formatting helper either doesn't exist for these or
isn't being used consistently, fix once and audit every call site rather than patch each field
separately (there are already two working scale-up helpers in the codebase, `formatBytes` in
`App.jsx` line ~1969, KB→MB→GB→TB, and `formatSpeed` line ~79, dec+bin up to GB — neither is used
by these four):

1. **Time remaining** — `App.jsx` line ~2518: `{Math.floor(backupStats.eta / 60)}m {backupStats.eta
   % 60}s`, no hours/days tier. Showed "342m 37s" instead of "5h 42m 37s".
2. **Elapsed time** — `App.jsx` line ~2528: `{Math.floor((Date.now() - backupStats.startTime) /
   1000)}s`, raw seconds only. Showed "2640s".
3. **Data size** — two separate places, both hardcoded `÷ 1048576` + literal `MB` suffix:
   - `App.jsx` lines ~2533-2534 (backup `dataSizeLabel`/"Data:" line) and ~3315-3316 (restore, same
     label) — frontend-side.
   - `gui/backup_inline.go` lines ~422, 428-456 (the "Processed: X / Y MB (New: ..., Reused: ...)"
     status message, built server-side in Go and shown via `status.message`) — same bug, separate
     codebase side entirely, so the frontend fix alone won't catch this one.
   This 478GB job is itself the live example: should already be reading GB, not MB.
4. **Speed** — `formatSpeed` (`App.jsx` line ~79) already scales dec/bin up through GB/s and GiB/s,
   just stops there — no TB/s tier. Not just theoretical: the 10G Ceph fabric went live this session
   (2026-09-28), proven at 9.4 Gbit/s, so a real job nearing ~1GB/s is plausible soon.

Fix should extend `formatBytes`/build a matching duration formatter and actually wire all four call
sites (both frontend React fields AND the Go-side "Processed:" message) through them, rather than
adding a fifth one-off scaling implementation.

#### ~~Backup Set save button label — "Update Schedule" should just say "Save Backup Set"~~ ✅ FIXED 2026-09-30

Mick (2026-09-28): the button on the Backup Set editor currently reads "Update Schedule" when
editing an existing set (`saveSchedule`/`updateSchedule` i18n keys, `App.jsx` ~line 3225) —
"would be better to just be a 'Save Backup Set' button." Unified to one "Save Backup Set" label
across all 18 languages; dropped the now-unused `updateSchedule` key.

#### ~~Tray icon was passive — no reflection of activity, no OS notification~~ ✅ DONE 2026-09-30

Mick: tray tooltip was a static string, and nothing popped up when a scheduled backup finished —
the only prior signal was email (if configured) or opening the window. `UpdateTrayTooltip`
existed but had zero callers (dead code). Added: tooltip now shows "Backup running: X" /
"Restoring: X" while active, "Next: X at \<time\>" (soonest enabled Backup Set) when idle,
refreshed once a minute off the scheduler's own tick and right after any run finishes
(`currentOperationLabel`, `operation_queue.go`, stops the idle refresh from clobbering an active
tooltip). Real Windows toast (Action Center) now fires on every backup/restore completion,
success or failure, via `github.com/go-toast/toast` (shells out to PowerShell's WinRT API, no
CGO). Stubbed for non-Windows and Windows-service mode, neither has a tray.

#### ~~Known Limitations wording could be misread as "ACL restore not done"~~ ✅ FIXED 2026-09-30

Mick screenshot: the single run-on sentence ("...isn't implemented yet. NTFS/ACL permissions ...
are already restored.") reads at a glance like ACL restore itself isn't done — content was
accurate, just buried the reassurance after the negative clause. Split into the limitation
(ADS/legacy NTFS extended attributes) and a separate, visually distinct ✅ reassurance line
(ACLs already restored), across all 18 languages.

#### ~~No Bare Metal Restore Guide existed (the old docs/RESTORE_GUIDE.md was stale/wrong)~~ ✅ DONE 2026-09-30, 🔧 wording follow-up below

Built a real, accurate guide (`gui/docs/BARE_METAL_RESTORE.md`, embedded via `go:embed`) describing
this fork's actual automated Clonezilla-based restore flow (PBS credentials → snapshot picker →
target-disk confirmation → automated restore), not the old generic SystemRescue/CLI-restore
doc that described a mechanism this fork never built. Tools menu: "View Bare Metal Restore Guide"
(in-app overlay, `BMRGuideModal.jsx`, small hand-rolled Markdown renderer — no new npm dependency)
and "Download Bare Metal Restore Guide…" (native save dialog), both work fully offline since the
guide ships in the binary, sitting alongside the existing ISO download.

**Follow-up, Mick (2026-09-30):** the guide's wording only talks about restoring Windows, but the
actual mechanism (`ocs-onthefly`, a raw whole-disk block copy via NBD) doesn't care what OS is on
the source snapshot at all — it works for restoring any OS that has a PBS full-machine/disk
snapshot (Linux, Windows, anything `machinebackuplib`/a physical-disk backup can produce). Reword
`gui/docs/BARE_METAL_RESTORE.md` to be OS-agnostic (currently says things like "put Windows back
on" and the post-restore section assumes Windows-specific repair steps) — keep a Windows-specific
sub-note only where it's genuinely Windows-only (e.g. Startup Repair from a Windows install USB;
a Linux target would use its own bootloader repair instead).

#### ~~No way to stop a Backup-Set-triggered ("Run Now") backup once started~~ ✅ FIXED 2026-09-26

Found live on rigel: a real machine (whole-disk) backup was running with no visible Stop button
anywhere — screenshot showed the "always visible" progress card (Backup Sets landing page) with no
stop control at all. Root cause: `handleStopBackup`/the Stop button only ever existed inside the
one-shot backup FORM, which the app navigates away from immediately once a backup actually starts
(`setShowBackupForm(false)` fires right after). So the Stop button was reachable only for the
handful of seconds before a backup began, never during it — for every trigger path, not just
Backup Sets. Added the same Stop button (reusing the existing `handleStopBackup`/`CancelBackup`
wiring, no new backend needed) directly onto the always-visible progress card itself.

Had to stop that specific rigel backup manually via SSH in the meantime (`sudo kill` the root
backup process, then `elioctl destroy` the two leftover elastio-snap devices it left behind —
a plain SIGTERM skips the app's own snapshot-cleanup defer, so that's a manual step after any
non-UI stop).

#### ~~Pass the Backup Set name through as the PBS snapshot's comment~~ ✅ DONE 2026-09-27 (backend), 2026-10-02 (UI field: "Backup Name (for PBS Comment)" on the one-off form, all 18 languages, also applied to each part of a split one-off backup)

Mick: "we don't seem to have comment wired in to pass through to the backup in pbs" — confirmed:
`BackupManifest.Comment` existed (matches PBS's real manifest schema, already used for READING an
existing snapshot's comment) but nothing anywhere ever WROTE it — zero `.Comment =` assignments in
the whole codebase, no GUI field, every backup created an empty PBS comment.

**Agreed design** (no new field needed for scheduled Backup Sets — reuse what already exists):
- Backup Set-triggered runs (scheduled or Run Now): use the Backup Set's own **Name** as the PBS
  comment (already meaningful, already unique per job, already what Reports labels the run with —
  `a.scheduledJobNameOr(...)`, already set at the right time by `executeScheduledJob`).
- A genuine one-off (Backup tab, no Backup Set at all): a new, explicitly-optional
  **"Backup Name (for PBS Comment)"** text field on the one-off form. Empty stays the existing
  generic "Manual backup - X"/"Backup machine - X" fallback (unchanged from today); a typed value
  overrides it and becomes the comment.

**Done:** `BackupOptions.Comment` end-to-end for directory backups (`gui/backup_inline.go`);
`StartBackup`/`StartMachineBackup` signatures (both `!service` and `service` build variants) plus
`startBackupDirect`/`startMachineBackupDirect`'s precedence logic (explicit comment wins, else
`scheduledJobNameOr(fallback)`); `api.BackupRequest`/`BackupHandler` (the service-mode HTTP
forwarding path); `machinebackuplib.Config` + its own manifest construction (machine backups use a
separate library — this is what "Rigel Full Machine Backup" actually needed); `scheduler.go`'s 2
call sites (pass `""` — scheduled jobs get their comment via `currentScheduledJobName` instead);
all 4 frontend `StartBackup`/`StartMachineBackup` call sites updated to the new arity (passing `''`
for now). Verified: `go build`/`vet` clean on Windows (`!service` and `-tags service`) and Linux,
full `wails build` succeeds.

**Still pending:** the actual new "Backup Name (for PBS Comment)" **input field** on the one-off
form itself (state + `<input>` + i18n keys, all 6 languages) — right now every one-off backup still
gets the generic "Manual backup - X" fallback exactly as before (correct, unchanged behavior — just
not yet the NEW capability). A Backup Set's own name, though, IS now correctly sent as the PBS
comment for every scheduled/Run-Now backup, both directory and machine type.

#### ~~Restore banner + Known Limitations claimed NTFS/ACL restore still unimplemented~~ ✅ FIXED 2026-09-27

Mick: "the banner on restore still says restore in beta and ntfs file permissions not supported, i
think we are past that no" — confirmed both were stale: the Restore tab's "BETA FEATURE" banner
listed "❌ NTFS permissions, ADS, extended attributes (NTFS sidecar sprint pending)" as one block,
and the Known Limitations modal made the same claim, neither updated after the NTFS ACL (Windows)
and POSIX ACL/xattr (Linux) restore work landed. Mick: keep the "beta" framing (restore genuinely
still is), but the checklist needed to be accurate — added a new ✅ line for NTFS/ACL + POSIX ACL
permissions (now done), narrowed the ❌ line to just what's still genuinely missing: ADS (Alternate
Data Streams — the `Zone.Identifier` "downloaded from the internet" tag, etc.) and legacy NTFS
Extended Attributes (a rarely-used OS/2-compat feature, distinct from ACLs). Same fix applied to
the Known Limitations modal's text. All 6 languages.

#### ~~No way to set the default PBS server from the UI~~ ✅ CONFIRMED WORKING (code audit) 2026-09-27

**Reported 2026-09-26 (on rigel):** "there is no way to set which server is the default in the UI"
— Mick had to have me add rigel's production PBS server via a direct `config.json` edit, then
couldn't find a way in the app itself to make it the default over the pre-existing test server.

**Confirmed via full code-path audit — no bug:** `handleSetDefaultPBS` (`App.jsx:880-893`) →
`SetDefaultPBSServer` (`main.go:589-592`) → `Config.SetDefaultPBS` (`config.go:481-488`) validates
the ID exists, sets `DefaultPBSID`, and persists via `c.Save()`; the frontend then updates its own
`defaultPBSID` state immediately on success, so the "⭐ DEFAULT" badge/"☆ Set Default" link swap
re-renders right away with no reload needed. The gating condition for showing the server-list view
at all is `pbsServers.length === 0` (`App.jsx:2061`) — with exactly the ONE server Mick had at the
time, the list view still renders, but that sole server is automatically the default (nothing else
to promote), so it shows only the "⭐ DEFAULT" badge with no clickable link — correct behaviour,
not a bug. Confirmed this was purely that one-server edge case: rigel now has 2+ servers configured,
and the code path is sound for that case. Not re-verified by clicking through the actual running
GUI (no RDP/screenshot access this session) — flag if it still doesn't work once you've tried it.

#### ~~Progress card: kill the duplicate 2nd indicator, auto-scroll to it, and show a queue underneath~~ ✅ DONE 2026-09-27

Started from: "the initial message for physical blocks under vss, or 'Initiating shadow copy' comes
up at the bottom of the page and remains there with some second progress when the top of the page
starts the proper detailed progress indicator." Converged, over several follow-ups, on a single
design — Mick: "the backup or restore showing at the top of the page might still be fine, so let's
stick with that for now and keep the other idea in reserve. We just need to deal with the 2nd
progress indicator and also make sure that when the progress starts we refocus on it by jumping to
it from wherever we are on the page" — plus: "we can then show a queue with the next backup waiting
underneath it which then expands to full progress once it becomes active."

**Decided scope (3 parts):**
1. **Kill the duplicate indicator.** Keep today's behaviour where the detailed progress card only
   appears at the top once real progress exists — don't chase the VSS 0%-rounding fix (that's the
   "other idea," now explicitly parked). Instead stop the OTHER renderer from duplicating it: a
   page-level status div at the bottom of both the Backup and Restore tabs (`App.jsx:3122-3124` and
   `App.jsx:3731-3733`) renders `status.message` completely independently of the top progress card
   (`App.jsx:2373` for backup, gated on `progress > 0 && progress < 100`; restore's own card is
   gated on `restoreLoading` instead, `App.jsx:3695`). Both read the same `status.message` state, so
   once the top card mounts, the exact same text also renders at the bottom — that's the "2nd
   progress indicator." Fix: don't render the bottom status div while its tab's own progress card is
   already showing (mutually exclusive, not both always-on).
2. **Auto-scroll/refocus.** When a backup or restore actually starts (the top progress card first
   mounts), scroll it into view from wherever the user currently is on the page — don't leave them
   having to scroll down manually to discover it started.
3. **Queue display.** When a second item is triggered while one is already running,
   `acquireOperationSlot` (`gui/operation_queue.go`) already fires `onQueued(heldBy)`, and every call
   site (`gui/main.go`, both backup and restore paths) already turns that into a real message via
   `opts.OnProgress(0.01, "Queued — waiting for %s to finish...")` — the backend event already
   exists. Frontend needs a compact "queued" row shown underneath the active progress card (not its
   own separate big card) for whatever's waiting; when the active job finishes and the queued one's
   turn starts, that row expands into the same full detailed progress card the active one has now.

**Implemented:** Backend untouched — all frontend (`App.jsx`). New `backupQueuedMsg`/
`restoreQueuedMsg` state, set by matching on the literal `"Queued —"` prefix inside the existing
`backup:progress`/`restore:progress` handlers instead of letting it fall through to the normal
progress path, so a second click never clobbers whichever job's card is actually active. Both bottom
page-level status divs now also require `!(progress > 0 && progress < 100)` /
`!restoreLoading` to render, so they no longer duplicate the active card's own text. Two new
`useEffect`s (watching `progress`/`restoreLoading` transition from inactive to active) call
`scrollIntoView({behavior:'smooth', block:'start'})` on the relevant card's new ref the moment it
first mounts. Restore's whole progress display (previously nested inside the snapshot-selection
view, `restoreLoading && (...)` block that used to sit next to the Restore/Stop buttons) moved to a
new always-visible card at the top of the Restore tab, structurally mirroring the backup tab's card
— fixes the same "disappears if you browse to a different snapshot mid-restore" class of bug the
Stop-button fix addressed for backup. Verified: `npm run build` clean, `go build`/`vet` clean on both
build tags, full `wails build` succeeds. **Verified live 2026-09-27** on winclient: queued 3 Backup
Sets behind an active one, all 3 showed up correctly named and numbered under the active card.

#### ~~Queued item showed no name, and multiple queued items weren't counted~~ ✅ DONE 2026-09-27

Follow-up to the item above. Mick: "I see a queued message but it doesn't say what backup is queued
and I clicked two after this and it doesn't indicate 2 in queue" — wanted "2 more backups queued...
(1) Name A (2) Name B". Also asked whether this would cover scheduled backups clashing too (recalled
a prior issue about overlapping schedules): the underlying lock (`operation_queue.go`) already
serialized that case safely, the gap was purely that the message never named who's waiting, only who's
holding the slot — same gap for both manual and scheduled triggers.

Backend's `acquireOperationSlot` message now reads `"Queued: <name> — waiting for <heldBy> to
finish..."`, reusing the same `scheduledJobNameOr`-resolved display name already used for the PBS
comment feature, so a scheduled job and a manual Run Now click surface identically. Frontend replaced
the old single queued-message state with one shared `pendingQueue` array (backup and restore share the
one real slot), rendered as a count line plus a numbered list, popped FIFO on completion. New
`queuedCountOne`/`queuedCountMany` i18n keys added across all 18 languages, parity re-verified.

⚠️ Caveat documented in code: a plain `sync.Mutex` isn't fair, so with 3+ simultaneous waiters the
*displayed order* is a best-effort approximation — the actual serialization stays correct regardless.

#### ~~Progress card title doesn't say which Backup Set/restore is running~~ ✅ DONE 2026-09-27

Mick: "where it says 'Backup Progress', should we add ' - <Backup Set Name>' so you know which set is
in progress". Backend now sends a `name` field alongside every `backup:progress`/`restore:progress`
tick (same resolved display name as the queue feature above); frontend tracks it and appends
" - <name>" to the card title. Verified live on winclient: title correctly showed the active Backup
Set's name while 3 others were queued underneath.

#### ~~Reports showed generic labels for some Backup Set runs under concurrent load~~ ✅ DONE 2026-09-27

Found live while stress-testing the queue feature (multiple Run Now clicks in quick succession): a
real race, not cosmetic — `currentScheduledJobName` is a single process-wide field, and
`RunScheduledJobNow` spawns each click as its own unserialized goroutine, so concurrent clicks could
read/clobber each other's name before ever reaching `operation_queue.go`'s actual serialization point.
Fixed by passing the job's name directly as the already-existing `comment` parameter instead of a
separate shared-field lookup (commit `53d191e`, full reasoning in the earlier "Reports:
scheduled-job name reverted to generic" entry's own follow-up above it). ⚠️
`currentScheduledJobPostActions`/`currentScheduledJobTrigger` are the same shared-field pattern, not
fixed (different symptom, never actually observed) — flagged as a related known gap if it ever surfaces.

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

#### ~~Same symptom came back under real concurrent load~~ ✅ FIXED 2026-09-27 — the "race-safety verified" claim above had a gap

Mick, stress-testing the new queue feature (several "Run Now" clicks in quick succession): "Some of
the report descriptions still seem generic while others have the backup name." Same visible bug as
the entry above, different actual cause — the earlier fix's own safety reasoning ("the next backup
can't acquire the slot until the current one's OnComplete has finished") is true, but only covers
`operation_queue.go`'s serialization *after* `acquireOperationSlot` — it never covered the EARLIER
`setScheduledJobName` → `StartBackup` → `scheduledJobNameOr()` read sequence, which happens before
any of that, in whichever fire-and-forget goroutine `RunScheduledJobNow` just spawned (one goroutine
per click, `go a.executeScheduledJob(...)`, completely unserialized at that point). Click several
Backup Sets close together and multiple `executeScheduledJob` instances run genuinely concurrently,
each reading/writing the SAME shared `currentScheduledJobName` field with no atomicity across the
three steps — goroutine A's read can land on goroutine B's name, or on the empty value from a THIRD
job's `OnComplete`-triggered clear landing at just the wrong moment.

**Fix:** stopped relying on the shared field for this data flow at all. `executeScheduledJob` now
passes `job.Name` directly as the `comment` parameter (a real, non-shared parameter `StartBackup`/
`StartMachineBackup` already had, from the PBS-comment feature) instead of `""` + a separate
`scheduledJobNameOr()` lookup. `main.go`'s two `OnComplete` closures reuse the already-resolved local
`comment` variable for the Reports `Name` field instead of re-querying `currentScheduledJobName` a
second time, long after the fact. No shared mutable state left in this specific path.

⚠️ **`currentScheduledJobPostActions`/`currentScheduledJobTrigger` are the exact same shared-field
pattern, not fixed here** — same concurrent-Run-Now-clicks scenario could plausibly apply the WRONG
job's post-backup actions (email/run-app/shutdown) or record the wrong trigger type, though neither
has actually been observed/reported yet. Worth the same treatment if it ever surfaces — Trigger has
no existing parameter slot on `StartBackup`/`StartMachineBackup` to reuse, so fixing it properly
would need a new one (or a small internal-only variant of these functions for the scheduler's own
call path, as opposed to the frontend's direct one-off calls).

#### ~~Email settings deserve their own Preferences tab~~ ✅ DONE 2026-09-26

Moved the whole SMTP/email-notifications section out of Preferences → Advanced into its own
"Email" tab (new `prefsEmail` i18n key, all 6 languages), leaving Advanced for the parallel-restore
toggle and settings export/import. Real SMTP details (found on polaris's Uptime Kuma notification
config, `mail.beebys.net:465`, `mums@beebys.net`) written into pbstest-winclient's `config.json`
directly so the tab isn't empty on next open.

#### ~~Relabel restore-mode radio buttons~~ ✅ DONE 2026-09-26

`restoreModeInPlace`/`restoreModeAlternate` (App.jsx's restore-destination radio group) reworded
in all 6 languages, matched to each language's own already-established noun for the alternate
option (e.g. Italian/English use "path", French/German/Polish/Spanish use "location/place") rather
than forcing a literal "path" cognate everywhere — parallel construction within each language, not
a mechanical find-replace.

#### ~~Language switcher: dropdown UI ready for 18 languages, 12 new ones still need real translations~~ ✅ DONE 2026-09-27

Prompted by discussing Romanian/Latvian/Ukrainian/Baltic-language UK migrant communities. Added to
`LanguageSwitcher.jsx`: bg/cs/el/hu/lv/lt/nl/pt/ro/sk/tr/uk alongside the existing 6, tightened row
padding (10px→5px vertical) and gave the dropdown panel a fixed 210px width (sized for the longest
native name — Nederlands/Slovenčina/Українська — independent of whichever language happens to be
selected, so it never has to wrap), plus a `maxHeight`/scroll safety net.

**Done:** full, real translations added for all 12 languages across all 441 (now 443, after the
queue-count keys) keys, including the nested `featuresList`/`techList` sub-objects — verified
programmatically for exact key parity against the English master, zero missing/extra keys, for all
18 languages. No longer UI-only; every language in the switcher is fully functional.

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

#### Code Signing - Windows Trust 🔐 — still open, confirmed 2026-09-29 (same gap as "MSI - Finitions" above, don't track twice)
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

#### ~~Progress Worker - UI Non-Blocking~~ ✅ DONE — moot, solved via a different mechanism, audited 2026-09-29
**Problème:** Callbacks `onProgress()` synchrones ralentissent le backup si frontend React lent.

**Audit finding:** `gui/progress_worker.go` was never created — but the underlying goal (progress
reporting that never blocks the backup thread) is already achieved a different way: progress runs
via goroutines + `runtime.EventsEmit` (`backup:progress`/`backup:stats`), confirmed extensively
working this session (the named-queue feature, live-verified on winclient/deepthought with multiple
concurrent jobs). No dedicated worker/channel/rate-limiter was needed. Nothing left to build here.

---

### Windows - Compatibilité avancée — 🟡 PARTIAL, narrowed 2026-09-29
- [ ] **LongPath Support** — confirmed still genuinely not addressed: no `\\?\` prefix or
  `longPathAware` manifest anywhere in `gui/*.go` or `installer/wix`.
  - [ ] Ajouter manifeste: `<longPathAware>true</longPathAware>`
  - [ ] Test: backup d'un chemin >260 caractères

- [x] ~~**Gestion des locks**~~ ✓ already solid via VSS, confirmed extensively working this
  session — not a real gap, drop this half of the section.

### Multi-Serveurs PBS
**→ DÉPLACÉ EN P0** (voir "Multi-PBS Architecture" ci-dessus, ✅ DONE)

### ~~Block-Level Splitting avec Offset (Disques Full) 💾~~ ✅ DONE — via a different (better) mechanism, audited 2026-09-29

**Audit finding:** `gui/machine_backup_windows.go.disabled` still exists, untouched — but
`machinebackuplib/windows.go`'s `BackupWindowsDisk` + `enumVolumeDiskOffset` (~line 272) already do
whole-disk, partition-aware, offset-correct streaming backup in ONE job, confirmed working
extensively this session (Rigel bare-metal restore test, verified clean). PBS's own chunk-level
dedup makes the fixed-size N-part scheme this section envisioned unnecessary — no reason to
reactivate the `.disabled` file, it's dead code, superseded. **Nothing left to build here; the
`.disabled` file can be deleted.**

**Status (original, now resolved a different way):** 📝 Documentation seulement - PAS ENCORE EN PROD

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

### Chiffrement (Phase 3) — ✅ DONE in the fork (2026-10-05), a few follow-ups open
Client-side encryption is implemented, but with a PBS key file (AES-256-GCM, signed manifest, same format as `proxmox-backup-client key create`), not the asymmetric key / master key design this section originally planned. See README "Client-side encryption" and CHANGELOG [Unreleased].
- [x] **Key Management**: key file per PBS server, generate / browse / clear in the Encryption tab, fingerprint shown
- [x] Passphrase-protected keys (scrypt/PBKDF2), Ask (session) or Remember (config, like the PBS token)
- [x] Unattended and service runs fail fast on an Ask key instead of prompting
- [x] Generated key files get a protected ACL (current user, SYSTEM, Administrators) on Windows
- [x] **GUI**: Encryption tab, passphrase prompt on backup and restore, Reports column, warning that a snapshot cannot be restored without the key
- [x] CLI `-keyfile` / `-keyfile-passphrase` on directory, machine and nbd tools; Clonezilla BMR key selection
- [ ] Windows Credential Manager / DPAPI store for the key and remembered passphrase (still plain config)
- [ ] Tighten the ACL on existing key files and on `config.json` (holds the PBS token and any remembered passphrase, readable by local Users on a default install)
- [ ] Test a bare-metal boot with a passphrase-protected key, and restore of an encrypted snapshot from the Proxmox VE storage configuration
- [ ] Native-speaker review of the machine-translated encryption strings in the 17 non-French/English languages
- [ ] "Export recovery key" button

### 🆕 Restauration - À Développer FROM SCRATCH 🔄 — ⚠️ MASSIVELY STALE, corrected 2026-09-29
**Status (original, 2026-03):** ❌ PAS IMPLÉMENTÉ - Code actuel = mock/stubs seulement

**Audit correction 2026-09-29: this entire status table is wrong now.** `gui/restore_inline.go` is
1229 lines / 20 functions — full snapshot listing, tree assembly, metadata, path-rewriting,
cancellation, all live and working. NTFS ACL/ADS restore AND Linux POSIX ACL/xattr restore have
both been verified live (per this fork's own project notes). The GUI restore path (Phase 1 below) is
essentially done. Only the standalone bare-metal CLI package and its docs (Phase 2/3 below) are
still missing — and even that may not be needed anymore since the BMR wizard already covers
bare-metal restore through the GUI (see the rigel BMR test project notes). Treat the table below as
historical, not current:

| Scénario (original, now outdated) | Méthode | Status (2026-03, WRONG now) |
|----------|---------|--------|
| Fichier supprimé | GUI restore granulaire | ✅ done, not ❌ |
| Dossier entier | GUI restore granulaire | ✅ done, not ❌ |
| Ransomware | Restore snapshot complet | ✅ done, not ⚠️ Basique |
| Disque HS (bare-metal) | GUI-based BMR wizard, not a separate CLI | ✅ done via a different route |
| P2V / Hardware différent | Fresh Windows + données | ✅ Possible (doc) |

---

#### ~~Phase 1: Restore Granulaire GUI~~ ✅ DONE — audited 2026-09-29 (task list below is historical, not a real plan anymore)

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

#### Phase 2: CLI `proxmoxbackupclient-restore` (bare-metal) — ⚠️ open, but scope question raised 2026-09-29

**Audit finding:** no `cmd/proxmoxbackupclient-restore/` exists. But the GUI's BMR wizard already
covers bare-metal restore end to end (verified live on rigel — real backup, clean fsck after
restore) via a different route than this section envisioned (booting into the full GUI rather than
a minimal CLI on a Linux live USB). **Worth a scope decision before building this**, not just
picking the task back up: is a separate minimal CLI still wanted (e.g. for a headless/no-GUI rescue
path), or does the GUI-based BMR wizard already cover the real need?

**Use case (original):** Restauration depuis Linux live (SystemRescue) après crash disque

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

#### Phase 3: Documentation & Guides — contingent on Phase 2's scope decision above

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

---

## 🗑️ DROP (Ignoré pour l'instant)

- ❌ UUID machine (hostname suffit)
- ❌ Heartbeat vers API distante (overkill)
- ❌ go-msi (WiX fonctionne)
- ❌ Mount FUSE/WinFSP (restauration web suffit)
- ❌ **API Remote - Provisioning Distant / Mode Entreprise (MSP central GUI)** — removed 2026-09-29,
  audit found nothing resembling this anywhere in the repo and no connection to this fork's actual
  current direction; a speculative future-product idea from the original March 2026 audit, not real
  scope. Revisit only if an actual MSP/multi-client use case shows up.

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

## Fork TODO (mjb-is)

### Post-backup action: "Minimise to tray" (before "Exit the app")
- [ ] Add a "Minimise to tray" choice to the Backup Set post-backup actions, ahead of "Exit the app". Today `ExitAppAfter` calls `os.Exit(0)` (`gui/email_notifications.go`, `runPostBackupActions`), which ends the whole process and with it every other schedule. Minimise hides the window to the tray (`MinimizeToTray()`, already used by the close prompt) and keeps the scheduler running.
- [ ] Likely shape: `MinimizeAfter bool json:"minimizeAfter,omitempty"` on `ScheduledJob`, run after the app/email steps and before exit/shutdown; mutually exclusive with exit in the form; Windows only (other platforms have no tray, so hide the option or fall back to leaving the window open).
- [ ] Edge case: a job set to shut down the computer still shuts down after minimising; exit and shutdown stay the last steps.

---

**Dernière mise à jour:** 2026-03-25 (Audit Technique intégré)
**Mainteneur:** Proxmox Backup Client GO contributors
**Référence:** Audit `🔬 Proxmox Backup Client - Audit Technique pour Développeurs`
