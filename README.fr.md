# Proxmox Backup Client : client Windows et Linux pour Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client est un client de sauvegarde open source (GPL-3.0) pour Proxmox Backup Server (PBS), sous Windows et Linux.** C'est une **suite d'outils** :

- **Proxmox Backup Client GUI** (basée sur la GUI Nimbus Backup de RDEM Systems) : client graphique pour Windows et Linux, avec snapshots cohérents (VSS sous Windows, elastio-snap/dattobd sous Linux), ensembles de sauvegarde planifiés, sauvegardes de dossiers et de disques entiers, restauration, Récupération (Undelete) et Retour arrière (Roll back), plusieurs serveurs PBS et un service d'arrière-plan.
- **`proxmoxbackup-directory`** : sauvegardes de dossiers (pxar) en ligne de commande, avec déduplication et détection des changements.
- **`proxmoxbackup-machine`** : sauvegardes à chaud de disques entiers (fidx) de machines Windows (VSS) et Linux (elastio-snap/dattobd) ; l'image est amorçable après une restauration bare-metal.
- **`proxmoxbackup-nbd`** : attache une sauvegarde de disque comme périphérique NBD pour la restauration (Linux).
- Une **ISO de restauration bare-metal** basée sur Clonezilla Live.

> ⚠️ **Avertissement :** ce projet **n'est affilié en aucune manière** à **Proxmox Server Solutions GmbH**. « Proxmox », le logo Proxmox et les noms associés appartiennent à leurs propriétaires ; ils ne servent ici **qu'à** indiquer la compatibilité. Voir [proxmox.com](https://www.proxmox.com/).

> ℹ️ **Cette page est un résumé traduit.** La référence est le [README.md anglais](README.md), qui contient tous les détails. Les manuels sont en anglais.

## 📖 Manuels utilisateur

| Manuel | Contenu |
|---|---|
| **[Manuel de la GUI](docs/manual/GUI.md)** | Installation, serveurs PBS, ensembles de sauvegarde, sauvegardes incrémentielles, restauration, Récupération, Retour arrière, Rapports, Préférences, service, dépannage |
| **[Manuel de la ligne de commande](docs/manual/CLI.md)** | Les trois outils : options, fichiers de configuration, codes de sortie, planification ; Récupération et Retour arrière en ligne de commande |
| **[Manuel de restauration bare-metal](docs/manual/BMR.md)** | L'ISO de restauration pas à pas ; snapshots `vm` vers Proxmox VE ; matériel différent |

Voir aussi les [scénarios de restauration](docs/RESTORE_GUIDE.md) et l'[index des manuels](docs/manual/README.md).

## 🆕 Sur master, pour la prochaine version

Dans le [CHANGELOG](CHANGELOG.md) sous [Unreleased], pas encore publié :

- **Installeurs** : **MSI Windows** (application et service dans Program Files, menu Démarrer, mise à jour sur place, paramètres conservés ou supprimés à la désinstallation) et **paquets Linux** `.deb` (Debian, Ubuntu, Mint) et `.rpm` (Fedora).
- **Service d'arrière-plan sous Linux** (`proxmoxbackupclient.service`) : les ensembles planifiés tournent en root, même sans session ouverte (Préférences > Avancé > Exécuter en tant que service). **Redémarrer en tant qu'administrateur** fonctionne aussi sous Linux (via pkexec).
- **Pause, Reprendre et Arrêter** pour les sauvegardes lancées par le service.
- Avertissement et badge **ID partagé** quand deux ensembles partagent un groupe de sauvegarde (même serveur, type et Backup ID).
- **Paramètres verrouillés** : `C:\ProgramData\ProxmoxBackupClient` réservé aux administrateurs sous Windows ; dossier privé à son propriétaire sous Linux.
- **Fichiers restaurables depuis les sauvegardes machine** en FAT, exFAT, ext2/3/4 et XFS, en plus de NTFS ; les images Windows remettent aussi permissions et flux de données alternatifs.
- Les sauvegardes machine Linux ignorent l'espace libre (ext2/3/4, XFS).
- Récupération et Retour arrière pour les ensembles machine Linux, et **en ligne de commande** : `-list-sets`, `-undelete`, `-rollback`.
- Les outils en ligne de commande écrivent leurs erreurs sur stderr, et `proxmoxbackup-machine` envoie son e-mail de notification.

## 💽 Nouveau en v0.8.0

- **Les sauvegardes machine Windows ignorent l'espace libre** (et les fichiers d'échange et d'hibernation) : un disque à moitié vide se lit en deux fois moins de temps. Un **secteur défectueux** n'arrête plus la sauvegarde (il est écrit en zéros et signalé).
- **Restaurer des fichiers depuis une sauvegarde machine** : chaque volume NTFS de l'image est un dossier à parcourir, rechercher et cocher, sans rien monter ni télécharger (127 Go et 430 000 fichiers listés en 4 à 15 secondes). Une seule sauvegarde machine couvre ainsi la restauration bare-metal et le fichier perdu.
- **Récupération et Retour arrière** sur les ensembles machine (Windows), pour les dossiers choisis (par défaut Desktop, Documents, Pictures, Music, Videos et Downloads).
- **Pause et Reprendre**, badge **VSS / Snapshot ON-OFF**, dates de création Windows sauvegardées et restaurées.
- **v0.8.2 :** l'espace vide n'est plus haché bloc par bloc (disque de test de 50 Go : 3 min 36 s, désormais 1 min 15 s).

## 🗑️ Nouveau en v0.7.0 : Récupération et Retour arrière

- **Récupération** trouve les fichiers présents dans les sauvegardes d'un ensemble mais plus sur le disque (dernière sauvegarde, ou 7 ou 30 derniers jours), les montre en arborescence avec la date de leur disparition, et restaure ceux que vous cochez. Après la suppression de 39 Go sur un partage de 761 000 fichiers, la recherche sur 7 jours les a retrouvés en une minute environ.
- **Retour arrière** remet les dossiers d'un ensemble dans l'état d'une sauvegarde choisie, après un aperçu. Chaque fichier remplacé ou supprimé est d'abord mis de côté sur le même disque, donc **Défaire** remet tout en place.

Aussi : dialogues de confirmation, prochaine exécution sur chaque carte, Backup ID suggéré, service inclus dans le téléchargement Windows. Voir le [manuel de la GUI](docs/manual/GUI.md#10-undelete).

## ⚡ Nouveau en v0.6.0 : sauvegardes de dossiers incrémentielles

Avec la même **détection des changements** que le `proxmox-backup-client` officiel, le mode **Métadonnées** compare taille, date de modification (à la nanoseconde), mode et propriétaire de chaque fichier avec le snapshot précédent ; les fichiers inchangés ne sont pas ouverts et leurs chunks sont réutilisés. Sur un serveur de 480,9 Go et 761 866 fichiers, la première exécution prend 4 h 17 min, les suivantes 10 à 15 minutes environ (4 à 40 nouveaux chunks). Le trafic vers un PBS distant se réduit d'autant. Voir le [manuel de la GUI](docs/manual/GUI.md#7-incremental-folder-backups-change-detection).

## 🤝 Compatible avec le client Proxmox officiel

Vérifié avec `proxmox-backup-client` 3.4.9 et PBS 4.2, dans les deux sens :

- Les snapshots Legacy, Data et Metadata de ce client (Windows ou Linux) se restaurent **à l'octet près** avec le client officiel, et le navigateur de fichiers de PBS 4.2 les ouvre ; ceux du client officiel se restaurent à l'octet près avec ce client.
- Même format d'archive scindée (pxar version 2), mêmes modes de détection, même format de fichier de clé (`proxmox-backup-client key create`), mêmes variables d'environnement `PBS_REPOSITORY`.

Vos sauvegardes ne sont donc jamais liées à ce client. Seul ajout : un petit fichier `.proxmox_backup_client_meta.json` à la racine des snapshots de dossiers, restauré comme un fichier ordinaire par le client officiel.

## 📦 Téléchargement et installation

👉 **[Télécharger la dernière version](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| Vous avez | Utilisez |
|---|---|
| Windows | L'**installeur MSI** (`ProxmoxBackupClient-<version>-x64.msi`), ou le zip `ProxmoxBackupClient-v<version>-windows-amd64.zip` (application et `ProxmoxBackupClientSVC.exe` dans le même dossier) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Autre Linux | `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (nécessite le runtime WebKitGTK) |
| Ligne de commande seule | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` ou `-linux-amd64.tar.gz` |

Le MSI, le `.deb` et le `.rpm` sont nouveaux sur master (jusqu'à la v0.8.1, seulement zip et tar.gz) ; en attendant, voir [BUILD.md](BUILD.md). Sous Linux, l'entrée de menu **Proxmox Backup Client (administrator)** lance l'application en root via polkit, pour les sauvegardes machine ; le service est installé mais pas activé ([packaging/debian/README.md](packaging/debian/README.md)).

**Prérequis :** Windows 10/11 ou Windows Server 64 bits (droits administrateur pour VSS et les sauvegardes machine), ou Linux avec WebKitGTK (et root plus elastio-snap ou dattobd pour les sauvegardes machine) ; un accès réseau à un Proxmox Backup Server.

> ⚠️ **Windows signale un virus (`Trojan:Win32/Sabsik.FL.A!ml`) ou SmartScreen avertit ?** C'est un **faux positif connu** des applications Go/Wails non signées. Détails chez le [projet d'origine](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Vérifier un téléchargement

Chaque version de ce fork contient un `SHA256SUMS.txt` :

```powershell
Get-FileHash .\ProxmoxBackupClient-v0.8.1-windows-amd64.zip -Algorithm SHA256   # compare with SHA256SUMS.txt
```

Sous Linux : `sha256sum -c SHA256SUMS.txt --ignore-missing`. Pas encore de signature de code ni d'attestation CI (voir [TODO.md](TODO.md)).

## ✨ Fonctionnalités

### GUI (recommandée)
- **18 langues d'interface**, dont le français, l'italien et l'allemand
- Menu latéral : Sauvegarde, Restauration, Récupération, Retour arrière, Rapports, Journal des messages, Préférences, À propos
- **Ensembles de sauvegarde** planifiés (quotidien, intervalle, manuel) avec « Exécuter maintenant », badges de type et VSS / Snapshot ; **sauvegardes ponctuelles**
- **Sauvegardes de dossiers incrémentielles** (Métadonnées) avec relecture complète toutes les N exécutions
- **Sauvegardes machine** Windows et Linux, en `host` ou en snapshot `vm` Proxmox VE
- **Restauration** sélective avec recherche, ACL NTFS/POSIX, horodatages et vérification facultative ; fichiers depuis les sauvegardes machine
- **Pause, Reprendre, Arrêter** ; **Exécuter en tant que service** (Préférences > Avancé)
- Rapports, notifications par e-mail, actions après sauvegarde, export et import des paramètres, cinq thèmes de couleur
- Plusieurs serveurs PBS, épinglage du certificat, jeton API ou utilisateur/mot de passe ; **chiffrement côté client**

### Outils en ligne de commande
- `proxmoxbackup-directory` : dossiers (pxar), `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine` : disques entiers (fidx, incrémentiel, espace libre ignoré sauf `-readfreespace`)
- `proxmoxbackup-nbd` : serveur NBD pour restaurer des sauvegardes de disque (Linux)
- `ProxmoxBackupClientSVC` (Windows) et `proxmoxbackupclient` (Linux) : `-list-sets`, `-undelete`, `-rollback` (sur master)

### 📸 Captures d'écran

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Ensembles de sauvegarde** : badges et « Exécuter maintenant »

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Progression en direct** : débit, temps restant, chunks

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restauration** : lister les snapshots ou y chercher un fichier

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Ensembles en VM Proxmox VE** : badge `MACHINE (VM)`

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Rapports** : historique complet avec détail

</td>
<td width="33%">

[<img src="docs/screenshots/gui-lang-french.png" width="280">](docs/screenshots/gui-lang-french.png)
**En français** : la page Rapports

</td>
</tr>
</table>

Toutes les captures sont dans le [README anglais](README.md).

### Exclusions système intelligentes
Les sauvegardes de dossiers ignorent `System Volume Information`, `$RECYCLE.BIN`, `Recovery` (à la racine d'un lecteur), `.pbs-rollback`, `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` et `DumpStack.log.tmp`. Un disque affichant 1,03 To utilisés peut ne contenir que 141 Go de vrais fichiers. Conseil : sauvegarde de **dossiers** pour les fichiers, sauvegarde **machine** dans un ensemble séparé pour la restauration bare-metal.

### Sécurité et qualité
Chiffrement facultatif avec manifeste signé ; API locale du service sur 127.0.0.1 uniquement, avec jeton ; validation des entrées, prévention des traversées de chemin, nouvelles tentatives avec backoff exponentiel ; tests unitaires et de bout en bout.

## 🔐 Chiffrement côté client

Chaque chunk est chiffré en AES-256-GCM avant de quitter la machine, avec un fichier de clé PBS (interchangeable avec le client officiel) ; le manifeste est signé et le serveur ne voit jamais la clé.

**GUI :** **Préférences > Informations du compte**, modifier un serveur, onglet **Chiffrement** : choisir une clé ou **Générer…**. Pour une clé protégée : **Me la demander** (une fois par session) ou **La mémoriser sur cet ordinateur** (nécessaire pour le service et les tâches sans surveillance). **Sans le fichier de clé, aucune restauration n'est possible : gardez-en une copie hors de la machine.**

**CLI :**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` n'est utile que pour une clé protégée (demandé s'il manque) ; `proxmoxbackup-machine` et `proxmoxbackup-nbd` acceptent les mêmes options. L'ISO Clonezilla demande la clé et sa phrase secrète, sans jamais l'écrire dans l'ISO.

**Limites :** `config.json` contient en clair jetons, mots de passe et phrases secrètes mémorisées (sur master, son dossier est protégé ; jusqu'à la v0.8.2 il était lisible par les utilisateurs locaux sous Windows). Une clé perdue ne se récupère pas.

## Sauvegardes de dossiers plus rapides : détection des changements

| Mode | Lit | Stocké en |
|---|---|---|
| **Classique** (`legacy`, par défaut) | tous les fichiers | un `<name>.pxar.didx` par dossier |
| **Données** (`data`) | tous les fichiers | archive scindée : `<name>.mpxar.didx` + `<name>.ppxar.didx` |
| **Métadonnées** (`metadata`) | seulement les fichiers nouveaux et modifiés | la même archive scindée ; les fichiers inchangés sont repris de la sauvegarde précédente sans être lus |

**GUI :** modifier un ensemble, onglet **Destination**, **Détection des changements** (ensembles de dossiers uniquement ; les sauvegardes machine ne changent pas). Réglez **Relire tous les fichiers toutes les N exécutions** pour attraper les rares changements qui gardent taille et date. Donnez à chaque ensemble **son propre Backup ID** : deux ensembles qui partagent un ID se comparent l'un à l'autre et relisent tout.

**CLI :** `-change-detection-mode legacy|data|metadata` ; pas d'option « toutes les N exécutions », planifiez de temps en temps un `-change-detection-mode data`. Avant de passer en Données ou Métadonnées, mettez en v0.6.0 ou plus chaque machine qui restaure : les versions antérieures ne lisent pas les archives scindées.

## 🚀 Démarrage rapide (GUI)

1. Installez le MSI, le `.deb` ou le `.rpm` (ou décompressez le zip / tar.gz)
2. Sous Windows, **Exécuter en tant qu'administrateur** pour VSS et les sauvegardes machine ; sous Linux, **Proxmox Backup Client (administrator)** pour les sauvegardes machine
3. Ajoutez votre serveur PBS dans **Préférences > Informations du compte** et cliquez sur **Test**
4. Créez un ensemble de sauvegarde (ou une sauvegarde ponctuelle) et choisissez dossiers ou disques
5. Lancez-le ; pour les planifications sans session ouverte, activez **Préférences > Avancé > Exécuter en tant que service**

Le [manuel de la GUI](docs/manual/GUI.md) détaille chaque étape.

## Sauvegarde complète de machine à chaud

Même syntaxe que pour les dossiers, avec `-backupdev` au lieu de `-backupdir` :

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

La commande prend un snapshot VSS des partitions du disque 0 et crée une image amorçable du disque entier (fidx). L'espace libre est écrit en zéros sans être lu (`-readfreespace` pour tout lire). Les sauvegardes suivantes sont incrémentielles.

### Prérequis pour les sauvegardes machine Linux

Linux n'a pas de VSS : le projet utilise le module noyau `elastio-snap` (un fork de `dattobd`) ; installez la version maintenue [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap) (`dattobd` déjà chargé convient aussi). Deux conditions sont obligatoires :

1. **Le module noyau doit être installé et chargé.** Le paquet de dépôt documenté pointe vers une URL cassée (au moins pour Ubuntu 22.04) ; la compilation depuis les sources fonctionne :
   ```bash
   sudo apt-get install linux-headers-$(uname -r) build-essential   # Debian/Ubuntu
   git clone --depth 1 https://github.com/Axcient/elastio-snap.git
   cd elastio-snap
   sudo make
   sudo make install
   sudo depmod -a     # required - `make install` only copies the .ko, it never
                       # runs depmod, so a bare `modprobe elastio-snap` right
                       # after a fresh install fails with "not found in
                       # directory" even though the file is right there
                       # (confirmed live 2026-09-24)
   sudo modprobe elastio-snap
   lsmod | grep elastio-snap   # confirms it loaded
   ```
   Le module n'est chargé que jusqu'au prochain redémarrage (voir `/etc/modules-load.d/` pour le charger à chaque démarrage).

2. **La sauvegarde doit tourner en root (EUID 0).** Le groupe `disk` ne suffit pas : utilisez l'entrée **Proxmox Backup Client (administrator)**, le service d'arrière-plan, ou `sudo`. Une sauvegarde de dossiers n'a pas cette exigence.

### Restauration de fichiers depuis une sauvegarde disque

La page Restauration ouvre une sauvegarde machine comme un dossier, sans rien monter. Sous Linux, on peut aussi l'attacher en NBD (après `modprobe nbd max_part=0`) :

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Sans `-path`, une interface texte permet de choisir. À éviter sur une machine importante ; démontez tout avant d'arrêter, sinon `nbd-client -d /dev/nbd0`.

### Restauration sur une machine physique

L'ISO Clonezilla patchée propose une entrée entièrement automatique **« PBS Bare Metal Restore »** : paramètres PBS, choix du snapshot, confirmation du disque, puis réseau, restauration et redémarrage sans autre intervention. L'entrée semi-manuelle « pbs-nbd » reste disponible. Voir le [manuel BMR](docs/manual/BMR.md).

## Utilisation : sauvegarde de dossiers

Référence complète dans le **[manuel de la ligne de commande](docs/manual/CLI.md)**. Commande type :

```shell
proxmoxbackup-directory.exe -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -backupdir "C:\path\to\backup" -datastore "datastorename"
```

```
proxmoxbackup-directory
  -baseurl string            PBS URL, example: https://192.168.1.10:8007
  -certfingerprint string    Certificate fingerprint (asked for interactively when omitted)
  -authid string             API token ID (user@realm!token)
  -secret string             API token secret
  -pbsusername string        PBS user for ticket login (with -pbspassword; overrides -authid/-secret)
  -pbspassword string        Password for -pbsusername (asked for when omitted)
  -datastore string          Datastore name
  -namespace string          Namespace (optional)
  -backup-id string          Backup ID (default: the hostname)
  -backupdir string          Source directory; repeat for several (each becomes group <id>_<path>)
  -change-detection-mode     legacy (default), data or metadata
  -novss                     Do not snapshot the source (VSS on Windows, elastio-snap/dattobd on Linux)
  -keyfile string            PBS encryption key file
  -keyfile-passphrase string Passphrase for a protected key (asked for when omitted)
  -backupstream string       Back up standard input as <name>.didx instead of a directory
  -pxarout string            Also write the archive to a local file (debugging)
  -mail-host, -mail-port, -mail-username, -mail-password, -mail-insecure,
  -mail-from, -mail-to, -mail-subject-template, -mail-body-template
                             Email report (optional)
  -config string             JSON config file; flags given on the command line override it
```

Les variables d'environnement `PBS_REPOSITORY` et apparentées du client officiel sont aussi lues.

## 🔨 Compilation depuis les sources

Tout est dans **[BUILD.md](BUILD.md)** (GUI, service, outils CLI, MSI, paquets Linux). En bref, avec Go 1.25 ou plus et Wails CLI v2.13 :

```bash
cd gui
wails build                                    # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe   # the background service
```

Le frontend compilé est versionné : Node.js ne sert qu'à le modifier. Tests : `make test`.

## 🔧 Utilisation avancée et guides

- **Multi-PBS** (plusieurs serveurs, cible choisie par ensemble) : [guide utilisateur](MULTI_PBS_USER_GUIDE.md) et [guide d'implémentation](MULTI_PBS_GUIDE.md).
- **ISO Clonezilla de restauration bare-metal** : construite en patchant une ISO Clonezilla Live, détails dans [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md) :

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

- [FEATURES_STATUS.md](FEATURES_STATUS.md) : ce qui est fait ou non, et depuis quelle version ; [CHANGELOG.md](CHANGELOG.md) : historique par version ; [TODO.md](TODO.md) : feuille de route ; [RELEASE_NOTES.md](RELEASE_NOTES.md) ; [AGENTS.md](AGENTS.md) : organisation du dépôt.

## 🌐 Support technique en Italie

Le projet et **toutes** ses contributions restent publics sous GPLv3. E.T.I. Srl ( https://etitech.net ), principal sponsor, propose du support pour Proxmox et les outils de sauvegarde Windows ; il n'y aura jamais d'édition « enterprise » séparée.

## 🖥️ Attribution de l'interface graphique

La GUI est basée sur la **[GUI Nimbus Backup](https://nimbus.rdem-systems.com)** de **[RDEM Systems](https://www.rdem-systems.com/)**, qui sponsorise son développement et en assure le support commercial. Tout le code reste sous GPLv3.

## ⚠️ Avertissement

Logiciel fourni « tel quel », encore en qualité alpha ; nous déclinons toute responsabilité en cas de perte de données. Testez vos sauvegardes et vos restaurations avant la production.

## 📄 Licence

GPLv3 : voir [LICENSE](LICENSE).

## 🏷️ Branding

Tout contributeur ayant au moins **5 commits** de fonctionnalités ou de corrections peut faire ajouter son branding à usage commercial, à condition que l'entreprise ne pratique ni campagnes de malware, ni promotion de la guerre (dans tout pays), ni arnaques, vol de données, trafic d'enfants, violence, discrimination, drogues, ni toute autre activité illégale. En cas de plainte sans explication valable, cet avantage prend fin immédiatement. La licence GPLv3 reste active : vous restez libre de forker le projet.

## 🤝 Contribuer

Contributions bienvenues, en particulier :

1. Chiffrement : relecture native des textes traduits automatiquement, gestion des clés au-delà du fichier (par exemple DPAPI Windows)
2. Migration physique vers virtuel (P2V) : les bases marchent via le type `vm` ; manquent carte réseau, TPM, pilotes virtio, CPU/RAM réels, bus autres que SATA
3. Envoi asynchrone / multicœur des chunks
4. Descripteurs de sécurité Windows dans le format pxar (patch côté Proxmox)
5. Liens symboliques Windows

## À propos

Les contributeurs de Proxmox Backup Client GO développent et maintiennent ce projet, qui s'appuie sur l'infrastructure NTP/NTS et les [11 serveurs NTS publics](https://github.com/jauderho/nts-servers).

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
