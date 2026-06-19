# Nimbus Backup — Client Windows pour Proxmox Backup Server

[🇬🇧 English](README.md) | 🇫🇷 Français

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/rdemsystems/NimbusBackupClient)](https://github.com/rdemsystems/NimbusBackupClient/releases)
[![Documentation](https://img.shields.io/badge/docs-nimbus.rdem--systems.com-orange)](https://nimbus.rdem-systems.com/?utm_source=github)

**Nimbus Backup est un client de sauvegarde Windows open-source (GPL-3.0) pour Proxmox Backup Server (PBS).**
Une interface graphique moderne pour sauvegarder serveurs et postes Windows vers PBS — snapshots cohérents via VSS, tâches planifiées, modes fichier et disque, navigation/restauration de snapshots, support multi-PBS et mode service Windows. Besoin d'un stockage PBS **déporté et immuable** sans auto-héberger ? Voir le [service infogéré](#️-pbs-infogéré-déporté--immuable) ci-dessous.

> Mots-clés : client proxmox backup windows · client PBS · sauvegarde Windows VSS · sauvegarde déportée immuable · interface Proxmox Backup Server.

## 📦 Téléchargement

👉 **[Télécharger la dernière version](https://github.com/rdemsystems/NimbusBackupClient/releases)**

> ⚠️ **Windows affiche « virus détecté » (ex. `Trojan:Win32/Sabsik.FL.A!ml`) ou un avertissement SmartScreen ?**
> C'est un **faux positif** connu pour les applications Go/Wails — ce n'est *pas* un virus. Le suffixe `!ml` indique une détection par un modèle de machine learning qui signale les exécutables *non signés et peu répandus*.
> Lisez [pourquoi cela arrive et comment vérifier le téléchargement](https://nimbus.rdem-systems.com/faux-positif-antivirus/?utm_source=github).

### 🔎 Vérifier n'importe quel téléchargement

Chaque release fournit des empreintes SHA-256 et une **attestation de provenance signée** (preuve cryptographique que le binaire a été produit par la CI de ce dépôt, à partir d'un commit précis) :

```powershell
Get-FileHash .\NimbusBackup.exe -Algorithm SHA256   # comparer avec SHA256SUMS.txt
gh attestation verify .\NimbusBackup.exe --repo rdemsystems/NimbusBackupClient
```

**VirusTotal — 0 détection.** Rapports multi-moteurs indépendants des installeurs MSI récents :
[0.2.108](https://www.virustotal.com/gui/file/6e8fb7ce9af740d470e947addb8daba4331c0b88e8bfdec9e0697ea8f7f29e9e/detection) ·
[0.2.107](https://www.virustotal.com/gui/file/6fd6c6fa77e0305c129ef882a3745100aa6033187a6d52a4af94149ab6b666d2/detection) ·
[0.2.106](https://www.virustotal.com/gui/file/ad6e56700ed9df8e088906e38cee2e2882fc7045f4e39269de0e379a01784ad7/detection)

> ℹ️ **Signature de code :** les binaires Windows ne sont **pas encore signés Authenticode** (un certificat OSS via [SignPath Foundation](https://signpath.org) est en attente). En attendant, la provenance est établie via l'attestation et les empreintes ci-dessus.

## ☁️ PBS infogéré (déporté & immuable)

Vous ne voulez pas auto-héberger Proxmox Backup Server ? Utilisez nos datastores PBS entièrement infogérés, **déportés et immuables** :
👉 **[Configurez votre sauvegarde & voir les tarifs](https://nimbus.rdem-systems.com/choisir-mon-backup/?utm_source=github)**

- ✅ À partir de 12 €/To/mois
- ✅ 1 To d'essai gratuit
- ✅ Une [cible de sauvegarde externalisée pour votre PBS](https://nimbus.rdem-systems.com/sauvegarde-proxmox-externalisee/?utm_source=github) — datastores immuables et isolés
- ✅ [NimbusBackup — hébergement PBS infogéré en France](https://nimbus.rdem-systems.com/?utm_source=github)

## 📚 Documentation

- **Guide complet de sauvegarde Proxmox** — bonnes pratiques de déploiement PBS ([🇫🇷 FR](https://nimbus.rdem-systems.com/blog/guide-complet-backup-proxmox/?utm_source=github))
- **Sauvegarder Windows avec Proxmox Backup Server** — guide de déploiement spécifique Windows ([🇫🇷 FR](https://nimbus.rdem-systems.com/blog/sauvegarder-windows-proxmox-backup-server/?utm_source=github))
- **PBS vs Veeam** — comparatif backup Proxmox ([🇫🇷 FR](https://nimbus.rdem-systems.com/blog/pbs-vs-veeam-comparatif-backup-proxmox/?utm_source=github))

## ✨ Fonctionnalités

### Interface graphique (recommandée)
- **🌍 Multilingue** — interface en français et en anglais
- Configuration conviviale avec test de connexion
- Progression de sauvegarde en temps réel avec débit et temps restant
- Support VSS (Volume Shadow Copy) pour des sauvegardes cohérentes
- Sauvegarde multi-dossiers, modes fichier et disque
- Navigation dans les snapshots, recherche de fichiers (jokers) et restauration
- Support multi-serveurs PBS, épinglage d'empreinte de certificat (TOFU)
- Mode service Windows + sauvegardes planifiées
- Journalisation de débogage pour le diagnostic

### 📸 Captures d'écran

![Configuration des serveurs](docs/screenshots/nimbus-gui-liste-servers.png)
*Gestion multi-serveurs PBS avec indicateurs d'état*

![Formulaire d'ajout de serveur](docs/screenshots/nimbus-gui-add-server-form.png)
*Configuration de serveur simple avec test de connexion*

![Sauvegarde immédiate](docs/screenshots/nimbus-gui-one-shot-backup.png)
*Progression de sauvegarde en temps réel avec ETA et débit*

### Exclusions système intelligentes (mode fichier)
Lors de la sauvegarde d'un disque entier (ex. `D:\`), Nimbus Backup exclut automatiquement :

**Dossiers système :** `System Volume Information` (stockage VSS, peut atteindre 100+ Go), `$RECYCLE.BIN`, `Recovery`.
**Fichiers système :** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys`.

**Pourquoi c'est important :** un disque peut afficher 1,03 To utilisés alors que les fichiers réels font ~141 Go. Sans exclusions, la sauvegarde inclurait les snapshots VSS (espace et temps gaspillés) ; avec elles, la taille correspond aux données réelles.

**Recommandation :** utilisez le **mode fichier** (par défaut) avec auto-exclusions pour les sauvegardes au niveau fichier ; utilisez le **mode disque** dans une tâche séparée pour la restauration bare-metal (inclut tout).

### Sécurité & qualité
- Validation des entrées et nettoyage des identifiants
- Prévention des traversées de chemin (path traversal)
- Logique de réessai avec backoff exponentiel
- Gestion d'erreurs complète et tests, conformité lint à 100 %

## 🚀 Démarrage rapide

1. Téléchargez `NimbusBackup.exe` (ou le `.msi`) depuis les releases
2. Lancez-le avec les droits administrateur (requis pour VSS)
3. Configurez votre connexion PBS et testez-la
4. Sélectionnez les dossiers à sauvegarder
5. Lancez la sauvegarde

## 📋 Prérequis

- Windows 10/11 (64 bits)
- Droits administrateur (pour les snapshots VSS)
- Accès réseau à un serveur Proxmox Backup Server

## 🔨 Compilation depuis les sources

### Prérequis
- Go 1.22 ou ultérieur
- Node.js 20 ou ultérieur
- Wails CLI : `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Build
```bash
cd gui
npm install --prefix frontend
wails build      # ou : wails dev  (rechargement à chaud)
```

## 📝 Projet d'origine

Ce projet est un fork de [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go), enrichi d'une interface graphique moderne et de fonctionnalités supplémentaires pour les utilisateurs Windows.

**Original :** Proxmox Backup Client en Go · **Auteur :** Tiziano Bacocco (tizbac) · **Licence :** GPLv3

| Fonctionnalité            | tizbac/proxmoxbackupclient_go | NimbusBackupClient (ce fork) |
|---------------------------|:-----------------------------:|:----------------------------:|
| Mode CLI                  | ✅                             | ✅                            |
| GUI Wails                 | ❌                             | ✅                            |
| Multilingue (FR/EN)       | ❌                             | ✅                            |
| Progression en temps réel | ❌                             | ✅                            |
| Exclusions système        | ❌                             | ✅                            |
| Support multi-PBS         | ❌                             | ✅                            |
| Pipelines CI/CD           | ❌                             | ✅                            |
| Tests complets            | ❌                             | ✅                            |

## ⚠️ Avertissement

Ce logiciel est fourni « tel quel ». Bien que nous visions la fiabilité, nous déclinons toute responsabilité en cas de perte ou de dommage de données. Testez toujours vos sauvegardes et vérifiez la restauration avant de vous y fier en production.

## 📄 Licence

GPLv3 — voir le fichier [LICENSE](LICENSE).

## À propos de RDEM Systems

NimbusBackupClient est développé et maintenu par [RDEM Systems](https://www.rdem-systems.com/), un fournisseur d'infrastructure français spécialisé dans l'infogérance Proxmox VE/PBS et l'infrastructure NTP/NTS. Nous exploitons [11 serveurs NTS publics](https://github.com/jauderho/nts-servers) listés dans la référence communautaire, et proposons un [hébergement PBS entièrement infogéré](https://nimbus.rdem-systems.com/?utm_source=github) pour ceux qui ne veulent pas auto-héberger.

---

**© 2024-2026 RDEM Systems. Tous droits réservés.**

--- 
[Trésor pour une chasse partenaire](https://dynamite-games-pontoise.fr/tresor/DGP-ETE-2026-GFDSCS55)
