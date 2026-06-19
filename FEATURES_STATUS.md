# Nimbus Backup - État des Features (2026-03-23)

## 📊 Vue d'ensemble

**Version actuelle:** v0.1.93
**Dernière release publique:** [GitHub Releases](https://github.com/rdem-systems/proxmoxbackupclient_go/releases)
**Statut général:** 🟢 Production-ready (GUI) | 🟢 Service Windows stable
**Dernière mise à jour:** 2026-03-23

---

## ✅ FEATURES IMPLÉMENTÉES & TESTÉES

### 🎨 Interface GUI (Wails v2 + React)
| Feature | Statut | Version | Notes |
|---------|--------|---------|-------|
| Interface graphique moderne | ✅ STABLE | v0.1.0 | Wails v2, React, Tailwind |
| Configuration PBS (URL, auth, datastore) | ✅ STABLE | v0.1.0 | Validation complète |
| Test de connexion PBS | ✅ STABLE | v0.1.0 | Auth réelle avant backup |
| Détection automatique hostname | ✅ STABLE | v0.1.0 | Pour backup-id |
| Onglet "À propos" avec version | ✅ STABLE | v0.1.23+ | Version dynamique |
| Liens upsell Nimbus Backup | ✅ STABLE | v0.1.31 | Tracking UTM |

### 💾 Backup - Directories
| Feature | Statut | Version | Notes |
|---------|--------|---------|-------|
| Backup multi-dossiers | ✅ STABLE | v0.1.0 | Sélection multiple |
| Calcul taille en arrière-plan | ✅ STABLE | v0.1.0 | Scan async pour ETA précis |
| Progression en temps réel | ✅ STABLE | v0.1.0 | % + vitesse + ETA |
| Barre de progression granulaire | ✅ STABLE | v0.1.0 | Mise à jour tous les 10 MB |
| Support VSS (Volume Shadow Copy) | ✅ STABLE | v0.1.0 | Détection privilèges admin |
| Exclusions personnalisées | ✅ STABLE | v0.1.0 | Patterns wildcards |
| Sauvegarde derniers chemins | ✅ STABLE | v0.1.29 | Auto-fill GUI |
| Junction points skip | ✅ STABLE | v0.1.26 | Évite erreurs "access denied" |
| Fichiers verrouillés skip | ✅ STABLE | v0.1.28 | Graceful skip + rapport |
| Rapport fichiers ignorés | ✅ STABLE | v0.1.28 | ⚠️ N fichiers/dossiers ignorés |

### 🖥️ Backup - Machine (Disques complets)
| Feature | Statut | Version | Notes |
|---------|--------|---------|-------|
| Machine backup (PhysicalDrive) | ❌ DÉSACTIVÉ | N/A | **Windows Defender faux-positif** |
| Code signing requis | ⏳ BLOQUÉ | N/A | Besoin certificat Authenticode |

### 📦 Restauration
| Feature | Statut | Version | Notes |
|---------|--------|---------|-------|
| Liste des snapshots | ❌ RETIRÉ | N/A | **Offload vers members.rdem-systems.com** |
| Restauration fichiers/dossiers | ❌ RETIRÉ | N/A | **Offload vers members.rdem-systems.com** |
| Parcourir catalog PBS | ❌ RETIRÉ | N/A | **Offload vers members.rdem-systems.com** |

**Note:** La restauration n'est **pas implémentée** dans le client desktop. Les clients Nimbus Backup utilisent le portail web [members.rdem-systems.com](https://members.rdem-systems.com) pour restaurer leurs données via l'interface PBS hébergée.

### 🔐 Sécurité & Qualité
| Feature | Statut | Version | Notes |
|---------|--------|---------|-------|
| Validation inputs | ✅ STABLE | v0.1.2 | URL, BackupID, Datastore, AuthID |
| Sanitization credentials | ✅ STABLE | v0.1.2 | Logs sécurisés (secrets masqués) |
| Path traversal prevention | ✅ STABLE | v0.1.2 | ValidatePath() |
| Retry logic exponential backoff | ✅ STABLE | v0.1.1 | Chunk uploads, index |
| Error handling complet | ✅ STABLE | v0.1.0 | Pas de panic(), tous wrappés |
| Structured logging (slog) | ✅ STABLE | v0.1.0 | JSON logs |
| 100% lint compliance | ✅ STABLE | v0.1.17 | golangci-lint v1.64 |

### 📝 Logging & Debug
| Feature | Statut | Version | Notes |
|---------|--------|---------|-------|
| Debug log `debug-gui.log` | ✅ STABLE | v0.1.0 | `C:\ProgramData\NimbusBackup\` |
| Debug log `debug-service.log` | ✅ STABLE | v0.1.78+ | Séparé du GUI |
| Crash reports | ✅ STABLE | v0.1.0 | `crash_report.txt` |
| Progression stable sans clignotement | ✅ STABLE | v0.1.29 | Printf retiré de pxar.go |
| Progression monotone | ✅ STABLE | v0.1.29 | Ne recule plus |

### 🪟 Windows Service
| Feature | Statut | Version | Notes |
|---------|--------|---------|-------|
| Service Windows fonctionnel | ✅ STABLE | v0.1.78+ | `NimbusBackupService` |
| API HTTP locale (localhost:18765) | ✅ STABLE | v0.1.78+ | Communication GUI ↔ Service |
| Exécution backups via Service | ✅ STABLE | v0.1.78+ | VSS sans UAC prompt |
| ReloadConfig avant backup | ✅ STABLE | v0.1.81 | Config fraîche à chaque backup |
| Logs séparés GUI/Service | ✅ STABLE | v0.1.78+ | Évite collisions |
| **VSS Cleanup au démarrage** | ✅ STABLE | **v0.2.0** | **Nettoie snapshots orphelins** |

### 📅 Scheduled Backups (Planification)
| Feature | Statut | Version | Notes |
|---------|--------|---------|-------|
| Création jobs planifiés | ✅ STABLE | v0.1.85+ | Cron + interface GUI |
| Stockage jobs (`jobs.json`) | ✅ STABLE | v0.1.85+ | `C:\ProgramData\NimbusBackup\` |
| Service exécute jobs | ✅ STABLE | v0.1.85+ | Check toutes les minutes |
| Historique jobs | ✅ STABLE | v0.1.85+ | Succès/échecs trackés |
| Édition/suppression jobs | ✅ STABLE | v0.1.85+ | CRUD complet |
| Cleanup abandoned jobs | ✅ STABLE | v0.1.85+ | Détection processus fantômes |
| Run at startup | ✅ STABLE | v0.1.85+ | Jobs "at startup" supportés |

### 🔧 MSI Installer
| Feature | Statut | Version | Notes |
|---------|--------|---------|-------|
| MSI fonctionnel | ✅ STABLE | v0.1.70+ | WiX Toolset |
| Installation Service Windows | ✅ STABLE | v0.1.70+ | Auto-start enabled |
| Upgrade propre | ✅ STABLE | v0.1.92 | Stop service avant upgrade |
| ProgramData créé | ✅ STABLE | v0.1.70+ | `C:\ProgramData\NimbusBackup\` |
| **Dialog désinstallation** | ✅ IMPLÉMENTÉ | **v0.2.0** | **Garder/supprimer config** |
| Code signing | ❌ MANQUANT | N/A | Certificat non acquis |

### 🎯 System Tray
| Feature | Statut | Version | Notes |
|---------|--------|---------|-------|
| Icône tray | ✅ STABLE | v0.1.80+ | Windows uniquement |
| Démarrage minimisé `--minimized` | ✅ STABLE | v0.1.80+ | Lancé par Task Scheduler |
| Menu contextuel tray | ✅ STABLE | v0.1.80+ | Show/Hide/Quit |
| Tooltip dynamique | ✅ STABLE | v0.1.80+ | Affiche dernier backup |

### 🔬 CI/CD & Tests
| Feature | Statut | Version | Notes |
|---------|--------|---------|-------|
| GitHub Actions complète | ✅ STABLE | v0.1.6+ | Build + Test + Lint + Security |
| Tests unitaires chunking | ✅ STABLE | v0.1.2 | 15+ test cases |
| Tests snapshots VSS | ✅ STABLE | v0.1.2 | Windows-specific |
| Gosec security scanning | ✅ STABLE | v0.1.7 | Zero issues |
| Golangci-lint | ✅ STABLE | v0.1.17 | v1.64, 100% compliance |
| Multi-platform builds | ✅ STABLE | v0.1.4+ | Windows/Linux/macOS |

---

## 🟡 FEATURES IMPLÉMENTÉES MAIS NON TESTÉES

### 🌐 API HTTP Remote (Provisioning)
| Feature | Statut | Fichier | Notes |
|---------|--------|---------|-------|
| API Server HTTP | 🟡 CODE PRÉSENT | `gui/api/server.go` | **Non activée par défaut** |
| Endpoints CRUD jobs | 🟡 CODE PRÉSENT | `gui/api/server.go` | **Jamais testée** |
| Auth Bearer token | 🟡 CODE PRÉSENT | `gui/api/server.go` | **Pas de génération token** |
| TLS optionnel | 🟡 CODE PRÉSENT | `gui/api/server.go` | **Pas de cert auto-signé** |

**⚠️ RISQUE:** Cette feature existe dans le code mais n'a jamais été activée ni testée. Elle pourrait ne pas fonctionner.

**📋 Pour activer:**
1. Générer token auth sécurisé
2. Configurer bind address (défaut: localhost:18765 déjà pris par Service API)
3. Tester tous les endpoints CRUD
4. Ajouter whitelist IPs
5. Documenter API (OpenAPI/Swagger)

---

## 🟡 FEATURES PARTIELLEMENT IMPLÉMENTÉES

### Multi-PBS Architecture
**Statut:** ✅ **Backend complet** (v0.2.0) | ⏳ Frontend à développer

**Objectif:** Permettre backups vers plusieurs serveurs PBS différents

**✅ Implémenté (Backend):**
- Structure `PBSServer` avec ID unique
- `Config.PBSServers` map de serveurs
- Migration automatique legacy → multi-PBS
- Jobs référencent PBS par ID (`Job.PBSID`)
- CRUD API complète (Add/Update/Delete/List/SetDefault)
- Documentation complète (`MULTI_PBS_GUIDE.md`)
- Méthodes App exposées au frontend :
  - `ListPBSServers()`
  - `GetPBSServer(id)`
  - `AddPBSServer(pbs)`
  - `UpdatePBSServer(pbs)`
  - `DeletePBSServer(id)`
  - `SetDefaultPBSServer(id)`
  - `TestPBSConnection(id)`

**⏳ À développer (Frontend React):**
- Page "Serveurs PBS" avec liste + CRUD
- Dropdown sélection PBS dans formulaire backup
- Indicateur statut connexion (🟢 Online / 🔴 Offline)
- Migration GUI jobs legacy vers PBSID

**Temps restant:** 2-3 jours frontend

---

## ❌ FEATURES NON IMPLÉMENTÉES (Roadmap)

### 🔴 P0 - CRITIQUE

#### (Aucune - P0 complétée !)

---

### 🟠 P1 - IMPORTANT

#### Code Signing
**Statut:** ❌ Non démarré (bloqueur externe)

**Requis pour:**
- Éviter warning Windows Defender
- Activer Machine Backup (PhysicalDrive)
- Distribution professionnelle

**Coût:** ~300€/an (DigiCert/Sectigo)
**Priorité:** Critique si lancement commercial

#### Installation Silencieuse MSI
**Statut:** 🟡 Spécifié, pas implémenté

**Objectif:** Déploiement GPO/Intune avec config pré-configurée

```powershell
msiexec /i NimbusBackup.msi /qn CONFIGFILE="\\ad-server\deploy\config.json"
```

**Bloqueurs:** Besoin CustomAction WiX + validation JSON
**Temps estimé:** 3-5 jours

#### VSS Cleanup au démarrage
**Statut:** ❌ Non implémenté

**Problème:** Snapshots VSS orphelins après crash
**Solution:** `vssadmin delete shadows /all /quiet` au démarrage service

**Temps estimé:** 2-3 heures

---

### 🟢 P2 - NICE TO HAVE

#### Chiffrement Client-Side
**Statut:** ❌ Non démarré

**Objectif:** Key management + encryption at rest
**Temps estimé:** 3-4 semaines

#### Multi-core Compression
**Statut:** ❌ Non démarré

**Objectif:** Paralléliser chunking ZSTD
**Temps estimé:** 2-3 semaines

#### Bandwidth Limiting
**Statut:** ❌ Non démarré

**Objectif:** Throttling upload pour ne pas saturer réseau
**Temps estimé:** 1 semaine

#### Windows Toast Notifications
**Statut:** ❌ Non démarré

**Objectif:** Notifs Windows 10/11 natives
**Temps estimé:** 3-5 jours

#### English Translation
**Statut:** ❌ Non démarré

**Objectif:** i18n complet (actuellement 100% français)
**Temps estimé:** 1 semaine

---

## 🗑️ DROPPED (Ne seront pas implémentées)

- ❌ UUID machine (hostname suffit)
- ❌ Heartbeat vers API RDEM (overkill, pas de SaaS backend)
- ❌ go-msi (WiX fonctionne parfaitement)
- ❌ Mount FUSE/WinFSP (restauration web PBS suffit)

---

## 📅 ROADMAP MISE À JOUR

### Q1-Q2 2026 (En cours)
- [x] Service Windows stable ✅ (v0.1.78+)
- [x] Scheduled backups ✅ (v0.1.85+)
- [x] System tray ✅ (v0.1.80+)
- [x] MSI installer ✅ (v0.1.70+)
- [ ] **Code signing** ⏳ (bloqué: budget certificat)
- [ ] **Multi-PBS** 🔴 (P0, jamais commencé)

### Q3 2026
- [ ] Installation silencieuse MSI
- [ ] VSS cleanup automatique
- [ ] Logs rotation (max 10 MB)
- [ ] API Remote testée et activable
- [ ] Bandwidth limiting

### Q4 2026 / Q1 2027
- [ ] Chiffrement client-side
- [ ] Multi-core compression
- [ ] English translation
- [ ] Windows toast notifications

### v1.0.0 Production
**Critères:**
- ✅ Service Windows stable (FAIT)
- ✅ Scheduled backups (FAIT)
- ✅ MSI installer (FAIT)
- ⏳ Code signing (BLOQUÉ budget)
- ❌ Chiffrement (pas commencé)
- 📝 Documentation complète (partielle)

---

## 🐛 BUGS CONNUS

### Résolus récemment
- ✅ Progression qui recule (v0.1.29)
- ✅ Clignotement GUI (v0.1.29)
- ✅ Junction points crash (v0.1.26)
- ✅ HTTP/2 connection reuse (v0.1.27-28)
- ✅ Version hardcodée (v0.1.23-25)
- ✅ Service config reload (v0.1.81)

### Actuels
- 🐛 **Désinstallation MSI ne propose pas de garder config** (comportement par défaut = garde tout)
- 🐛 **API Remote jamais testée** (peut ne pas marcher)

---

## 🎯 RECOMMANDATIONS PRIORITAIRES

### Court terme (1-2 semaines)
1. ✅ ~~Implémenter VSS cleanup~~ **FAIT** (v0.2.0)
2. ✅ ~~Améliorer désinstallation MSI~~ **FAIT** (v0.2.0)
3. ✅ ~~Backend Multi-PBS~~ **FAIT** (v0.2.0)
4. **Frontend Multi-PBS** (2-3 jours) - page serveurs + dropdown
5. **Tester MSI uninstall dialog** sur Windows
6. **Documenter installation silencieuse MSI**

### Moyen terme (1-2 mois)
1. **Acquérir certificat code signing** (~300€) → débloque Machine Backup
2. **Tester API Remote** ou la retirer du code si inutilisée
3. **Bandwidth limiting** (feature demandée)

### Long terme (3-6 mois)
1. **Chiffrement client-side** (feature entreprise)
2. **Multi-core compression** (perf boost)
3. **English translation** (marché international)

### Hors scope (délégué au portail web)
- ❌ **Restauration locale** → Utiliser [members.rdem-systems.com](https://members.rdem-systems.com)
- ❌ **Browse snapshots GUI** → Portail web PBS suffit
- ❌ **Selective file restore** → Portail web PBS suffit

---

**Dernière mise à jour:** 2026-03-23
**Mainteneur:** RDEM Systems
**Contact:** contact@rdem-systems.com
