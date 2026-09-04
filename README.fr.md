# Proxmox Backup Client — Client Windows pour Proxmox Backup Server

[🇬🇧 English](README.md) | 🇫🇷 Français | [🇮🇹 Italiano](README.it.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/tizbac/proxmoxbackupclient_go)](https://github.com/tizbac/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-github-orange)](https://github.com/tizbac/proxmoxbackupclient_go)

**Proxmox Backup Client est un client de sauvegarde Windows open-source (GPL-3.0) pour Proxmox Backup Server (PBS).**
Une interface graphique moderne (**Proxmox Backup Client GUI**, basée sur la GUI Nimbus Backup de RDEM Systems) pour sauvegarder serveurs et postes Windows vers PBS — snapshots cohérents via VSS, tâches planifiées, modes fichier et disque, navigation/restauration de snapshots, support multi-PBS et mode service Windows.

> Mots-clés : client proxmox backup windows · client PBS · sauvegarde Windows VSS · sauvegarde déportée immuable · interface Proxmox Backup Server.

## 📦 Téléchargement

👉 **[Télécharger la dernière version](https://github.com/tizbac/proxmoxbackupclient_go/releases)**

> ⚠️ **Windows affiche « virus détecté » (ex. `Trojan:Win32/Sabsik.FL.A!ml`) ou un avertissement SmartScreen ?**
> C'est un **faux positif** connu pour les applications Go/Wails — ce n'est *pas* un virus. Le suffixe `!ml` indique une détection par un modèle de machine learning qui signale les exécutables *non signés et peu répandus*.
> Lisez [pourquoi cela arrive et comment vérifier le téléchargement](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Vérifier n'importe quel téléchargement

Chaque release fournit des empreintes SHA-256 et une **attestation de provenance signée** (preuve cryptographique que le binaire a été produit par la CI de ce dépôt, à partir d'un commit précis) :

```powershell
Get-FileHash .\ProxmoxBackupClient.exe -Algorithm SHA256   # comparer avec SHA256SUMS.txt
gh attestation verify .\ProxmoxBackupClient.exe --repo tizbac/proxmoxbackupclient_go
```

**VirusTotal — 0 détection.** Rapports multi-moteurs indépendants des installeurs MSI récents :
[0.2.108](https://www.virustotal.com/gui/file/6e8fb7ce9af740d470e947addb8daba4331c0b88e8bfdec9e0697ea8f7f29e9e/detection) ·
[0.2.107](https://www.virustotal.com/gui/file/6fd6c6fa77e0305c129ef882a3745100aa6033187a6d52a4af94149ab6b666d2/detection) ·
[0.2.106](https://www.virustotal.com/gui/file/ad6e56700ed9df8e088906e38cee2e2882fc7045f4e39269de0e379a01784ad7/detection)

> ℹ️ **Signature de code :** les binaires Windows ne sont **pas encore signés Authenticode** (un certificat OSS via [SignPath Foundation](https://signpath.org) est en attente). En attendant, la provenance est établie via l'attestation et les empreintes ci-dessus.

## 📚 Documentation

- **Guide complet de sauvegarde Proxmox** — bonnes pratiques de déploiement PBS
- **Sauvegarder Windows avec Proxmox Backup Server** — guide de déploiement spécifique Windows
- **PBS vs Veeam** — comparatif backup Proxmox

## ✨ Fonctionnalités

### Proxmox Backup Client GUI (recommandée)
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
Lors de la sauvegarde d'un disque entier (ex. `D:\`), Proxmox Backup Client exclut automatiquement :

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

1. Téléchargez `ProxmoxBackupClient.exe` (ou le `.msi`) depuis les releases
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

## 🔧 Utilisation avancée et guides

### Multi-PBS (plusieurs serveurs PBS)

Configurez plusieurs serveurs PBS et choisissez la cible pour chaque sauvegarde (ex. `C:\Users` → PBS SSD rapide, quotidien ; `C:\` → PBS big-data, hebdomadaire ; plus un serveur DR).

- **[Guide utilisateur](MULTI_PBS_USER_GUIDE.md)** — ajout/test des serveurs, serveur par défaut, FAQ et dépannage.
- **[Guide d'implémentation](MULTI_PBS_GUIDE.md)** — modèle de données, migration automatique depuis une config mono-PBS, méthodes API backend.

La configuration mono-PBS existante est automatiquement migrée vers un serveur `default` au premier chargement.

### ISO Clonezilla (restauration bare-metal)

Le workflow de secours est construit en patchant une ISO Clonezilla Live avec les binaires `pbsnbd` / `machinebackup` et une entrée **pbs-nbd** dans le menu principal de Clonezilla (démarrage CD, USB via `dd`, et UEFI) :

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd
```

Détails complets (pourquoi une reconstruction complète plutôt qu'un remplacement, prérequis, flux du menu, vérification) dans **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**.

### Compilation de la GUI Windows

**Docker (recommandé, surtout lorsqu'on compile sous Linux).** Le script en une commande produit un `ProxmoxBackupClientGO.exe` avec le support WebView2 adéquat, via un conteneur `golang` jetable (installation de mingw + Wails, build du frontend, exécution de `wails build`) :

```bash
./build_gui_windows_docker.sh
```

**Windows natif (Chocolatey).** Voir **[BUILD.md](BUILD.md)** pour la configuration complète de la toolchain Windows :

```powershell
choco install go
choco install mingw
# puis, dans un shell non élevé :
build.bat          # GUI
build_cli.bat      # CLI
```

### Statut des fonctionnalités, changelog et docs internes

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)** — matrice de statut par fonctionnalité (implémenté / testé / roadmap).
- **[CHANGELOG.md](CHANGELOG.md)** — historique des changements par version.
- **[TODO.md](TODO.md)** — roadmap et idées ouvertes.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)** — état stable du produit et builds disponibles.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)** — dialogue de désinstallation MSI (conserver/supprimer la config) et son plan de test.
- **[FIXES_SUMMARY.md](FIXES_SUMMARY.md)** — notes de correctifs GUI (bascule mode répertoire vs machine).

## 🖥️ Attribution de l'interface graphique

L'interface graphique **Proxmox Backup Client GUI** est basée sur la **[GUI Nimbus Backup](https://nimbus.rdem-systems.com)**, développée et maintenue par **[RDEM Systems](https://www.rdem-systems.com/)**.

La GUI (à l'origine un fork de ce projet) a été fusionnée dans ce dépôt : l'intégralité du code, y compris la GUI et toutes ses fonctionnalités, reste open-source sous licence GPLv3. RDEM Systems sponsorise le développement de la GUI et en assure le support commercial.

**Auteur du CLI d'origine :** Tiziano Bacocco (tizbac) · **Licence :** GPLv3

## ⚠️ Avertissement

Ce logiciel est fourni « tel quel ». Bien que nous visions la fiabilité, nous déclinons toute responsabilité en cas de perte ou de dommage de données. Testez toujours vos sauvegardes et vérifiez la restauration avant de vous y fier en production.

## 📄 Licence

GPLv3 — voir le fichier [LICENSE](LICENSE).

## À propos de Proxmox Backup Client GO contributors

Proxmox Backup Client GO contributors développe et maintient ce projet. Le logiciel s'appuie sur l'infrastructure NTP/NTS et les [11 serveurs NTS publics](https://github.com/jauderho/nts-servers) listés dans la référence communautaire.

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**

--- 
[Trésor pour une chasse partenaire](https://dynamite-games-pontoise.fr/tresor/DGP-ETE-2026-GFDSCS55)
