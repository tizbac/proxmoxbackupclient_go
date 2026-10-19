# Proxmox Backup Client : client Windows et Linux pour Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)
[![Ko-fi](https://img.shields.io/badge/Ko--fi-buy%20me%20a%20coffee-FF5E5B?logo=ko-fi&logoColor=white)](https://ko-fi.com/mickbeeby)

> **Construit sur le travail d'autres personnes.** Ce fork s'appuie sur le [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) d'origine de Tiziano Bacocco (tizbac) et sur le client graphique Nimbus Backup de RDEM Systems, qui y a été intégré et dont est issue l'interface graphique de cette application.
>
> ☕ Le client est gratuit et le restera. S'il vous fait gagner du temps, vous pouvez [m'offrir un café sur Ko-fi](https://ko-fi.com/mickbeeby). C'est entièrement facultatif et n'achète ni assistance ni promesse de fonctionnalité.

**Proxmox Backup Client est un client de sauvegarde open source (GPL-3.0) pour Proxmox Backup Server (PBS), sous Windows et Linux.** Il sauvegarde des dossiers sous forme d'archives de fichiers navigables et des machines entières sous forme d'images disque amorçables, selon des planifications ou à la demande, et restaure tout, d'un simple fichier à une machine entière, sur le même ordinateur, sur une machine de remplacement ou dans une machine virtuelle Proxmox VE.

<p align="center">
  <a href="docs/screenshots/overview-how-it-works.png"><img src="docs/screenshots/overview-how-it-works.png" width="400" alt="How it works: Windows and Linux machines back up through the client to Proxmox Backup Server, then restore as files, onto bare metal, or into a Proxmox VE VM"></a>
  <a href="docs/screenshots/overview-screenshots.png"><img src="docs/screenshots/overview-screenshots.png" width="400" alt="Screenshots of the app: Backup Sets, Reports, schedules, a whole PC as a VM, backup progress and restore"></a>
</p>

Je le développe pour mon propre homelab, afin que PBS soit le seul système de sauvegarde d'un parc mixte de serveurs, postes fixes et portables Windows et Linux, à la place de Backup for Workgroups, de miroirs rsync et d'images Clonezilla/RescueZilla. Il a commencé en septembre 2026 comme fork de [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) et l'a largement dépassé : plus de 350 commits depuis, dont 18 corrections et fonctionnalités renvoyées au projet d'origine ([liste ci-dessous](#-contributions-au-projet-dorigine)).

**Vous pouvez l'utiliser tel quel.** C'est officiellement un **logiciel alpha**, utilisé à vos propres risques, sans aucune garantie de quelque nature que ce soit. Je teste moi-même chaque version et je l'utilise régulièrement sur mes propres machines, mais lisez [Statut, rapports de bugs et demandes de fonctionnalités](#-statut-rapports-de-bugs-et-demandes-de-fonctionnalités) avant de vous y fier.

La suite :

- **Proxmox Backup Client GUI** : l'application pour Windows et Linux. Ensembles de sauvegarde avec plusieurs planifications chacun, sauvegardes de dossiers et de disques entiers avec snapshots cohérents (VSS sous Windows, elastio-snap/dattobd sous Linux), restauration de fichiers depuis les sauvegardes de dossiers et de machines, Récupération (Undelete) et Retour arrière (Roll back), plusieurs serveurs PBS, chiffrement côté client et un service d'arrière-plan pour que les planifications s'exécutent sans session ouverte.
- **`proxmoxbackup-directory`** : sauvegardes de dossiers (pxar) en ligne de commande, avec déduplication et détection des changements.
- **`proxmoxbackup-machine`** : sauvegardes à chaud de disques entiers (fidx) en ligne de commande, pour machines Windows et Linux ; l'image est amorçable après une restauration.
- **`proxmoxbackup-nbd`** : attache une sauvegarde de disque comme périphérique NBD (Linux).
- **Une ISO de restauration bare-metal** basée sur Clonezilla Live, avec une restauration guidée de la mise sous tension au redémarrage.

> Mots-clés : proxmox backup client windows · PBS client · Windows VSS backup · physical to virtual (P2V) · bare-metal restore · Proxmox Backup Server interface.

## ⭐ Ce qu'il fait, avec des résultats mesurés

| | Résultat |
|---|---|
| **Sauvegardes de dossiers incrémentielles** | Un partage de serveur de fichiers de 480,9 Go et 761 866 fichiers : première exécution 4 h 17 min, chaque exécution suivante 10 à 15 minutes. Les fichiers inchangés ne sont même pas ouverts (la détection des changements Metadata du client officiel). |
| **Sauvegardes de machines rapides** | L'espace libre, le fichier d'échange et le fichier d'hibernation ne sont pas lus, et l'espace disque vide n'est pas haché. Un disque de portable de 1 To avec 373 Go utilisés : 1 h 23 min à 1 h 44 min avant, 38 minutes maintenant. Un disque de test de 50 Go : 3 min 36 s, maintenant 1 min 15 s. |
| **Fichiers récupérés depuis les sauvegardes de machines** | Parcourir une image disque comme un dossier et restaurer des fichiers isolés, depuis NTFS, FAT, exFAT, ext2/3/4 et XFS. Le lecteur C: de 127 Go d'un portable, avec 430 000 fichiers, se liste en 4 à 15 secondes ; rien n'est monté ni téléchargé au préalable. |
| **Machine physique en machine virtuelle** | Sauvegarder un PC ou un serveur, puis le démarrer comme VM Proxmox VE : restauré par Proxmox VE lui-même, ou avec l'ISO de restauration. [Voir ci-dessous](#-machines-physiques-en-machines-virtuelles-p2v). |
| **Restauration bare-metal** | Démarrer l'ISO (clé USB, CD ou PXE), saisir les informations PBS, choisir la sauvegarde : une machine Windows de 32 Go était revenue à son écran de connexion 8 minutes après la mise sous tension. |
| **Récupération** | Après la suppression de 39 Go sur un partage de 761 000 fichiers, une recherche sur 7 jours a retrouvé exactement ces fichiers en une minute environ, et les a remis en place. |
| **Retour arrière** | Remettre les dossiers d'un ensemble dans l'état d'une sauvegarde choisie, avec un aperçu avant et Défaire après. |
| **Planifications** | Plusieurs par ensemble de sauvegarde, de quelques minutes à une fois par an : « chaque jour à 02:00 » plus « le dernier jour de chaque mois » ou « le 2e mardi », chacune décrite en clair avec ses 3 prochaines exécutions. |
| **Fidélité de la restauration** | Permissions NTFS, attributs, flux de données alternatifs et attributs étendus, dates de création et de dossiers sous Windows ; modes, propriétaires, ACL, attributs étendus et liens symboliques sous Linux. Une passe facultative relit ce qui a été restauré et le compare avec la sauvegarde. |
| **Fonctionne avec les outils de Proxmox** | Les sauvegardes faites ici se restaurent à l'octet près avec le `proxmox-backup-client` officiel, et inversement. [Détails](#-compatible-avec-le-client-proxmox-officiel-dans-les-deux-sens). |

## 🧩 Ce qu'il ajoute par rapport au projet d'origine

Au-delà des [18 modifications intégrées au projet d'origine](#-contributions-au-projet-dorigine), ce client ajoute :

**Sauvegardes**
- Des sauvegardes de dossiers incrémentielles avec la détection des changements du client officiel (modes Data et Metadata, archives pxar v2 scindées), avec une lecture complète toutes les N exécutions.
- Des sauvegardes de machines qui ignorent l'espace libre, le fichier d'échange et le fichier d'hibernation (NTFS sous Windows ; ext2/3/4 et XFS sous Linux), écrivent les secteurs illisibles en zéros et continuent, et affichent pendant l'exécution le nombre de blocs, de disques, de partitions et de chunks.
- Pause, Reprendre et Arrêter pour les sauvegardes de dossiers et de machines, y compris celles lancées par le service d'arrière-plan.
- Flux de données alternatifs, attributs étendus NTFS et dates de création dans les sauvegardes de dossiers Windows.

**Planifications et ensembles de sauvegarde**
- Plusieurs planifications par ensemble de sauvegarde, de quelques minutes à une fois par an (le n-ième jour de la semaine, le dernier jour du mois), chacune décrite en clair avec ses 3 prochaines exécutions.
- Un service d'arrière-plan sous Linux comme sous Windows, et Redémarrer en tant qu'administrateur sous Linux.
- Exécuter maintenant rapide ou complet, un badge VSS / Snapshot, un avertissement **ID partagé** (SHARED ID), les noms d'ensembles écrits dans PBS comme commentaires de snapshot, les alertes par e-mail, les actions après sauvegarde, et l'export et l'import des paramètres.

**Restaurations**
- Des fichiers isolés récupérés depuis les sauvegardes de machines (NTFS, FAT, exFAT, ext2/3/4, XFS), sans monter ni télécharger l'image.
- Récupération et Retour arrière, pour les ensembles de dossiers et de machines, dans la GUI et en ligne de commande.
- Une passe de vérification facultative après une restauration, les dates de création Windows, et une arborescence de snapshot qui se charge un dossier à la fois, pour que les très grandes sauvegardes s'ouvrent immédiatement.
- Une ISO de restauration bare-metal guidée qui restaure aussi dans une VM vide, et des parcours physique vers virtuel testés ([P2V](#-machines-physiques-en-machines-virtuelles-p2v)).
- Une compatibilité à l'octet près avec le `proxmox-backup-client` officiel dans les deux sens, y compris ses snapshots Data et Metadata (archive scindée).

**L'application**
- Une interface reconstruite : barre de menus, menu latéral, Rapports avec l'historique complet des sauvegardes et des restaurations, un Journal des messages, cinq thèmes de couleur et une couleur personnalisée, notifications dans la zone de notification et notifications toast.
- 18 langues d'interface, toutes relues (l'original en a 5).
- Dossier des paramètres réservé aux administrateurs (Windows) ou à son propriétaire (Linux), et un jeton d'API du service renouvelé à chaque démarrage.
- Des paquets MSI, `.deb` et `.rpm`, et les manuels utilisateur pour la GUI, la ligne de commande et la restauration bare-metal.

## 🔄 Machines physiques en machines virtuelles (P2V)

Une sauvegarde de machine d'un PC ou serveur physique peut devenir une machine virtuelle Proxmox VE, de deux façons :

**1. La restaurer dans Proxmox VE.** Réglez l'ensemble de sauvegarde (ou la sauvegarde ponctuelle) sur **Sauvegarder en tant que : Machine virtuelle Proxmox VE** (Backup as: Proxmox VE virtual machine). La sauvegarde est stockée en `vm/<numéro>` avec une petite configuration de VM, si bien que Proxmox VE la liste avec ses autres sauvegardes de VM : ouvrez le stockage PBS dans Proxmox VE, sélectionnez la sauvegarde, **Restore**, choisissez un ID de VM et un stockage. La VM reçoit un disque SATA et le firmware adapté au disque (OVMF pour UEFI/GPT, SeaBIOS pour BIOS/MBR) ; ajoutez une carte réseau avant de la démarrer, et un disque EFI (Pre-Enroll keys décoché) si une VM UEFI ne démarre pas.

**2. La restaurer avec l'ISO de restauration bare-metal dans une VM vide.** Créez une VM avec le même firmware que la machine d'origine (OVMF avec un disque EFI pour UEFI/GPT, SeaBIOS pour BIOS/MBR), un disque SATA au moins aussi grand que l'original et une carte réseau Intel E1000, démarrez-la sur l'ISO et lancez la restauration. Cela fonctionne aussi bien pour les sauvegardes `host` que `vm`, et la même sauvegarde se restaure toujours sur une machine physique.

Parcours testés :

| De | Vers | Parcours | Résultat |
|---|---|---|---|
| Portable Windows 10, UEFI/GPT | VM Proxmox VE | ISO de restauration | Démarrée sur le bureau de l'utilisateur |
| Windows Server, UEFI/GPT | VM Proxmox VE | Restauration Proxmox VE | Démarrée sans modification |
| Poste Xubuntu, UEFI/GPT | VM Proxmox VE | Restauration Proxmox VE, et l'ISO de restauration | Démarrée sur le bureau |
| Linux, BIOS/MBR | VM Proxmox VE | ISO de restauration, et restauration Proxmox VE | Démarrée à l'invite de connexion |
| Linux Mint, sauvegarde chiffrée | VM Proxmox VE | ISO de restauration, clé sur une clé USB | Démarrée |
| PC Linux | le même PC avec un nouveau disque | ISO de restauration | Démarré |

Windows installe le nouveau matériel virtuel au premier démarrage (« Préparation des périphériques »). Le [manuel de restauration bare-metal](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines) contient le tableau des firmwares, les pilotes et les instructions pas à pas.

## 📖 Manuels utilisateur

| Manuel | Contenu |
|---|---|
| **[Manuel de la GUI](docs/manual/GUI.md)** | Installation, serveurs PBS, ensembles de sauvegarde et planifications, sauvegardes incrémentielles, progression, restauration, Récupération, Retour arrière, Rapports, journaux, Préférences, le service d'arrière-plan, dépannage |
| **[Manuel de la ligne de commande](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd` : toutes les options, fichiers de configuration, codes de sortie, planification ; Récupération et Retour arrière en ligne de commande |
| **[Manuel de restauration bare-metal](docs/manual/BMR.md)** | L'ISO de restauration, pas à pas ; sauvegardes `vm` vers Proxmox VE ; physique vers virtuel ; matériel différent |
| [Scénarios de restauration](docs/RESTORE_GUIDE.md) | Quelle restauration utiliser et quand |
| [Index des manuels](docs/manual/README.md) | Une vue d'ensemble des trois manuels |

## 🆕 Nouveau en v0.9.0 : plusieurs planifications par ensemble de sauvegarde, installeurs, un service Linux

**v0.9.1** corrige l'ISO de restauration bare-metal : l'entrée de restauration est de retour dans son menu de démarrage BIOS, une clé Ventoy est reconnue comme disque de démarrage, et un disque cible plus petit que l'original est refusé avant toute écriture. Elle note aussi dans le commentaire PBS du snapshot un e-mail de notification qui n'a pas pu être envoyé.

- **Plusieurs planifications par ensemble de sauvegarde**, de quelques minutes à une fois par an : « chaque jour à 02:00 » plus « chaque mois le dernier jour » ou « le 2e mardi », chaque planification décrite en clair avec ses 3 prochaines exécutions. Les ensembles existants gardent leur planification.
- **Les sauvegardes de dossiers conservent les flux de données alternatifs et les attributs étendus NTFS** (Windows), comme la marque « téléchargé depuis Internet », et une restauration les remet en place. La section Limites connues n'en liste désormais plus aucune.
- **Installeurs** : un **MSI Windows** (application et service d'arrière-plan dans Program Files, entrée dans le menu Démarrer, mises à jour sur place, paramètres conservés ou supprimés à la désinstallation) et des **paquets Linux**, `.deb` (Debian, Ubuntu, Mint) et `.rpm` (Fedora), avec l'application, les trois outils en ligne de commande, les manuels et les entrées de menu.
- **Un service d'arrière-plan sous Linux** (`proxmoxbackupclient.service`) : les ensembles de sauvegarde planifiés s'exécutent en root, y compris sans session ouverte. Activez-le dans Préférences > Avancé > Exécuter en tant que service, comme sous Windows. **Redémarrer en tant qu'administrateur** fonctionne désormais aussi sous Linux (via pkexec).
- **Pause, Reprendre et Arrêter pour les sauvegardes lancées par le service**, avec leur carte de progression dans l'application ouverte.
- **Un avertissement quand deux ensembles de sauvegarde partagent un groupe de sauvegarde** (même serveur, même type et même Backup ID), avec un badge **ID partagé** sur les deux cartes.
- **Paramètres verrouillés** : sous Windows, `C:\ProgramData\ProxmoxBackupClient` n'est plus accessible qu'aux administrateurs ; sous Linux, le dossier des paramètres est privé à son propriétaire.
- **Fichiers récupérés depuis les sauvegardes de machines de tous les systèmes de fichiers courants** : FAT, exFAT, ext2/3/4 et XFS en plus de NTFS. Les restaurations d'images Windows remettent aussi les permissions et les flux de données alternatifs.
- **Les sauvegardes de machines Linux ignorent l'espace libre** (ext2/3/4, XFS), comme celles de Windows depuis la v0.8.0.
- **Récupération et Retour arrière pour les ensembles de machines Linux**, et **en ligne de commande** pour tout ensemble de sauvegarde : `-list-sets`, `-undelete`, `-rollback` (section 12 du [manuel de la ligne de commande](docs/manual/CLI.md)).
- Les outils en ligne de commande écrivent leurs erreurs sur stderr au lieu d'afficher des boîtes de dialogue, et `proxmoxbackup-machine` envoie son e-mail de notification.
- **L'ISO de restauration bare-metal** ne propose jamais la clé USB sur laquelle elle a démarré comme disque cible de la restauration, et le manuel explique comment faire correspondre le firmware (UEFI ou BIOS) quand une machine physique devient une VM.
- **Toutes les langues relues** (17 traductions, environ 1 850 chaînes), et les dates et les nombres suivent la langue de l'application.

Versions précédentes : **v0.8** sauvegardes de machines plus rapides (espace libre ignoré, secteurs défectueux surmontés), fichiers récupérés depuis les sauvegardes de machines, Pause et Reprendre ; **v0.7** Récupération et Retour arrière ; **v0.6** sauvegardes de dossiers incrémentielles. Le [CHANGELOG](CHANGELOG.md) contient toutes les modifications.

## 🤝 Compatible avec le client Proxmox officiel, dans les deux sens

Vérifié avec `proxmox-backup-client` 3.4.9 et PBS 4.2 :

- **Les snapshots Legacy, Data et Metadata faits par ce client se restaurent à l'octet près avec le client officiel**, depuis la GUI comme depuis la ligne de commande, qu'ils aient été faits sous Windows ou sous Linux, et le navigateur de fichiers de PBS 4.2 les ouvre. Sous Linux, les fichiers restaurés conservent aussi exactement leur mode, leur date de modification à la nanoseconde et la cible de leurs liens symboliques.
- **Les snapshots Legacy, Data et Metadata faits par le client officiel se restaurent à l'octet près avec ce client**, entièrement ou sélectivement.
- Même format d'archive scindée (pxar version 2), mêmes modes de détection des changements, même format de fichier de clé de chiffrement (les clés `proxmox-backup-client key create` fonctionnent ici, et les clés générées ici fonctionnent là-bas), mêmes variables d'environnement du type `PBS_REPOSITORY`.

Vos sauvegardes ne sont donc jamais liées à ce client : les outils de Proxmox restaurent chaque snapshot qu'il produit, et il restaure chaque snapshot que produit le client officiel. Le seul ajout est un petit fichier que ce client place à la racine d'un snapshot de dossier, `.proxmox_backup_client_meta.json` (le chemin d'origine), que le client officiel restaure comme un fichier ordinaire.

> ⚠️ **Avertissement :** ce projet **n'est affilié en aucune manière** à **Proxmox Server Solutions GmbH**. « Proxmox », le logo Proxmox et les noms associés sont la propriété de leurs détenteurs respectifs ; ils ne servent ici **qu'à** indiquer la compatibilité. Voir [proxmox.com](https://www.proxmox.com/) pour leurs produits.

## 📦 Téléchargement et installation

👉 **[Télécharger la dernière version](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| Vous avez | Utilisez |
|---|---|
| Windows | L'**installeur MSI** (`ProxmoxBackupClient-<version>-x64.msi`) installe l'application et le service d'arrière-plan dans Program Files, avec une entrée dans le menu Démarrer. Ou décompressez `ProxmoxBackupClient-v<version>-windows-amd64.zip` (l'application et `ProxmoxBackupClientSVC.exe` ; gardez-les dans le même dossier) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Un autre Linux | Décompressez `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (nécessite le runtime WebKitGTK 4.1) |
| Ligne de commande seule | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` ou `-linux-amd64.tar.gz` |
| Une machine à restaurer | `ProxmoxBackupClient-GO-BMR-Restore-v<version>.iso`, l'ISO de restauration bare-metal |

Le MSI, le `.deb` et le `.rpm` sont nouveaux en v0.9.0 : les versions jusqu'à la v0.8.1 n'ont que les fichiers zip et tar.gz. [BUILD.md](BUILD.md) explique comment les compiler vous-même.

Les paquets Linux installent deux entrées de menu : **Proxmox Backup Client** lance l'application sous votre compte (sauvegardes de dossiers de vos propres fichiers), et **Proxmox Backup Client (administrator)** la lance en root, en demandant votre mot de passe via polkit, pour les sauvegardes de machines et les snapshots de blocs. Le service d'arrière-plan est installé mais pas activé ; voir [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **Windows affiche « virus détecté » (par exemple `Trojan:Win32/Sabsik.FL.A!ml`) ou un avertissement SmartScreen ?**
> C'est un **faux positif connu** des applications Go/Wails : ce n'est *pas* un virus. Le suffixe `!ml` indique une détection par un modèle d'apprentissage automatique qui signale les exécutables *non signés et peu répandus*. Vérifiez le téléchargement avec `SHA256SUMS.txt` (ci-dessous).

### 🔎 Vérifier un téléchargement

Chaque version contient un `SHA256SUMS.txt` :

```powershell
Get-FileHash .\ProxmoxBackupClient-0.9.0-x64.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
```

Sous Linux : `sha256sum -c SHA256SUMS.txt --ignore-missing`.

> ℹ️ **Pas encore de signature de code, d'attestation signée par une CI ni d'analyse VirusTotal.** Les versions sont compilées et publiées à la main, pas encore par une chaîne CI capable de produire une attestation de provenance de build vérifiable. Voir [TODO.md](TODO.md) si vous souhaitez aider à la mettre en place.

## ✨ Fonctionnalités

### GUI
- **🌍 Multilingue** : 18 langues d'interface : français, anglais, italien, allemand, polonais, espagnol, bulgare, tchèque, grec, hongrois, letton, lituanien, néerlandais, portugais, roumain, slovaque, turc et ukrainien
- Barre de menus native et menu latéral (Sauvegarde, Restauration, Récupération, Retour arrière, Rapports, Journal des messages, Préférences, À propos)
- **Ensembles de sauvegarde** : tâches nommées réutilisables avec **plusieurs planifications chacune** (de quelques minutes à une fois par an, ou manuel), un badge de type dossier ou machine, un badge VSS / Snapshot, « Exécuter maintenant » (lecture rapide ou complète pour les ensembles Métadonnées), et un avertissement **ID partagé** quand deux ensembles écriraient dans un même groupe de sauvegarde
- **Sauvegardes ponctuelles** de dossiers ou de disques entiers
- Page **Rapports** avec l'historique complet des sauvegardes et des restaurations, et un **Journal des messages** plafonné pour le diagnostic
- Dialogue **Préférences** avec cinq thèmes de couleur (Ambre, Bleu, Vert, Rouge, Sombre) plus une option de couleur personnalisée
- **Sauvegardes de dossiers incrémentielles** (détection des changements, mode Métadonnées) : les fichiers inchangés sont réutilisés sans être lus, comme dans le client officiel, avec une lecture complète facultative toutes les N exécutions
- **Sauvegardes de machines (disque entier)** sous Windows et Linux qui ignorent l'espace libre et survivent aux secteurs défectueux, stockées en `host` ou en sauvegarde `vm` Proxmox VE
- **Physique vers virtuel** : une sauvegarde de machine se restaure dans une VM Proxmox VE, via Proxmox VE ou l'ISO de restauration
- **Restauration** : navigation dans les snapshots, recherche de fichiers (jokers, expressions régulières, chemins complets) et restauration sélective, avec ACL NTFS/POSIX, flux de données alternatifs, horodatages des dossiers, dates de création Windows et une passe de vérification facultative
- **Fichiers récupérés depuis les sauvegardes de machines** : parcourir et restaurer des fichiers de volumes NTFS, FAT, exFAT, ext2/3/4 et XFS dans une image disque
- **Récupération** et **Retour arrière** : trouver les fichiers que les sauvegardes d'un ensemble contiennent encore mais que le disque n'a plus, et les remettre en place ; ou remettre les dossiers d'un ensemble dans l'état d'une sauvegarde choisie, avec un aperçu avant et Défaire après. Ensembles de dossiers et de machines. Voir le [manuel de la GUI](docs/manual/GUI.md#10-undelete)
- **Pause, Reprendre et Arrêter** une sauvegarde en cours
- **Exécuter en tant que service** : installer, démarrer, arrêter ou supprimer le service d'arrière-plan depuis Préférences > Avancé, pour que les planifications s'exécutent sans session ouverte (Windows et Linux)
- Carte de progression en direct : heure de début et fin prévue, fichiers traités sur le total, vitesse de traitement (données et fichiers par seconde) et vitesse d'envoi réelle, fichier en cours, et la raison pour laquelle une exécution lit tous les fichiers
- VSS (Volume Shadow Copy) sous Windows, snapshots de blocs elastio-snap/dattobd sous Linux
- Notifications par e-mail et actions après sauvegarde, clonage d'ensembles de sauvegarde, export et import des paramètres
- Plusieurs serveurs PBS, avec épinglage de l'empreinte du certificat (confiance à la première utilisation), connexion par jeton d'API ou utilisateur/mot de passe
- **Chiffrement côté client** : fichier de clé PBS par serveur (AES-256-GCM, manifeste signé), protection facultative par phrase secrète, voir [Chiffrement côté client](#-chiffrement-côté-client)
- Protection administrateur pour la modification des ensembles de sauvegarde, avec Redémarrer en tant qu'administrateur
- Journalisation de débogage pour le diagnostic

### Outils en ligne de commande
- `proxmoxbackup-directory` : sauvegardes de dossiers (pxar) avec déduplication et `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine` : sauvegardes à chaud de disques entiers de machines Windows (VSS) et Linux (elastio-snap/dattobd) (fidx, incrémentielles, espace libre ignoré sauf avec `-readfreespace`, `-type vm` pour une sauvegarde Proxmox VE)
- `proxmoxbackup-nbd` : serveur NBD pour restaurer des sauvegardes de disque (Linux)
- `ProxmoxBackupClientSVC` (Windows) et `proxmoxbackupclient` (paquets Linux) : `-list-sets`, `-undelete` et `-rollback` pour les ensembles de sauvegarde définis dans la GUI

Les trois premiers acceptent `-keyfile` et `-keyfile-passphrase` pour les snapshots chiffrés. Référence complète : [manuel de la ligne de commande](docs/manual/CLI.md).

### 📸 Captures d'écran

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Ensembles de sauvegarde** : tâches nommées avec un badge pour la planification qui s'exécutera ensuite (DAILY, WEEKLY, MONTHLY, +N pour les autres) et un badge de type dossier/machine, Exécuter maintenant sur chacune

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Progression de la sauvegarde en direct** : débit, temps restant estimé et nombre de chunks nouveaux et réutilisés pendant l'exécution

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restauration** : choisir un serveur PBS et un Backup ID, lister les snapshots ou y chercher par nom de fichier

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**Planifications** : plusieurs par ensemble, de quelques minutes à une fois par an (« chaque mois le dernier jour », « le 2e mardi »), avec les 3 prochaines exécutions

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Sauvegarde en VM Proxmox VE** : choisir le type machine virtuelle et un ID numérique ; le Backup ID PBS s'affiche pendant la saisie

</td>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Ensembles de sauvegarde en VM Proxmox VE** : un ensemble stocké comme sauvegarde `vm` porte un badge `MACHINE (VM)`

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/oneoff-as-vm.png" width="280">](docs/screenshots/oneoff-as-vm.png)
**Sauvegardes ponctuelles** : la même option dans le formulaire de sauvegarde ponctuelle

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**Restauration sélective** : cocher des fichiers ou dossiers dans l'arborescence du snapshot, restaurer sur place ou ailleurs

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Rapports** : historique complet des exécutions avec un panneau de détail (durée, chunks, dossiers sauvegardés)

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-message-log.png" width="280">](docs/screenshots/gui-message-log.png)
**Journal des messages** : un flux de diagnostic plafonné et horodaté couvrant chaque sauvegarde et chaque restauration

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**Multi-PBS** : gérer plusieurs serveurs PBS, tester la connexion, choisir un serveur par défaut

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-edit-server.png" width="280">](docs/screenshots/gui-preferences-edit-server.png)
**Ajouter ou modifier un serveur** : URL, utilisateur/mot de passe ou jeton d'API, datastore et namespace

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-theme-amber.png" width="280">](docs/screenshots/gui-theme-amber.png)
**Thèmes** : cinq palettes de couleurs intégrées plus une couleur personnalisée, qui recolorent toute l'application en direct (voir la bande ci-dessous)

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-advanced.png" width="280">](docs/screenshots/gui-preferences-advanced.png)
**Options de sauvegarde** : expliquées honnêtement : l'extraction parallèle à la restauration est expérimentale et s'est montrée *plus lente* sur notre propre matériel de test

</td>
</tr>
</table>

**🎨 Thèmes de couleur** : le dialogue Préférences lui-même change de couleur à chaque choix, pas seulement un échantillon :

<table>
<tr>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-amber.png" width="180">](docs/screenshots/gui-theme-amber.png)
Ambre

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-blue.png" width="180">](docs/screenshots/gui-theme-blue.png)
Bleu classique

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-green.png" width="180">](docs/screenshots/gui-theme-green.png)
Vert

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-red.png" width="180">](docs/screenshots/gui-theme-red.png)
Rouge

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-dark.png" width="180">](docs/screenshots/gui-theme-dark.png)
Sombre

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-custom.png" width="180">](docs/screenshots/gui-theme-custom.png)
Personnalisé

</td>
</tr>
</table>

**🌍 Multilingue** : la page Rapports en français, en italien et en espagnol (et 15 autres langues, dont l'anglais, l'allemand et le polonais) :

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-lang-french.png" width="280">](docs/screenshots/gui-lang-french.png)
Français

</td>
<td width="33%">

[<img src="docs/screenshots/gui-lang-italian.png" width="280">](docs/screenshots/gui-lang-italian.png)
Italiano

</td>
<td width="33%">

[<img src="docs/screenshots/gui-lang-spanish.png" width="280">](docs/screenshots/gui-lang-spanish.png)
Español

</td>
</tr>
</table>

### Exclusions système intelligentes (sauvegardes de dossiers)
Les sauvegardes de dossiers laissent de côté d'elles-mêmes les éléments suivants :

**Dossiers système :** `System Volume Information` (stockage VSS, peut dépasser 100 Go) et `$RECYCLE.BIN` où qu'ils se trouvent ; `Recovery` seulement à la racine d'un lecteur ; `.pbs-rollback` (le dossier de sécurité du Retour arrière).
**Fichiers système :** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` et `DumpStack.log.tmp`.

**Pourquoi c'est important :** un lecteur peut afficher 1,03 To utilisés alors que les vrais fichiers font environ 141 Go. Sans exclusions, la sauvegarde inclurait les snapshots VSS (espace et temps perdus) ; avec elles, la taille correspond aux données réelles.

**Recommandation :** utilisez une sauvegarde de **dossiers** avec ces exclusions pour les sauvegardes au niveau des fichiers ; utilisez une sauvegarde de **machine** dans un ensemble de sauvegarde séparé pour la restauration bare-metal et le P2V (c'est une image du disque entier ; l'espace libre et les fichiers d'échange et d'hibernation y sont écrits en zéros).

### Sécurité et qualité
- Chiffrement facultatif côté client de chaque chunk, avec un manifeste signé
- Dossier des paramètres réservé aux administrateurs sous Windows, privé à son propriétaire sous Linux
- L'API locale du service n'écoute que sur 127.0.0.1 et exige un jeton, renouvelé à chaque démarrage du service
- Validation des entrées et assainissement des identifiants
- Prévention des traversées de chemin
- Nouvelles tentatives avec backoff exponentiel
- Gestion des erreurs et tests unitaires dans chaque module, plus des tests en conditions réelles et des tests de bout en bout facultatifs contre un vrai PBS

## 🔐 Chiffrement côté client

Les snapshots peuvent être chiffrés avant de quitter la machine, avec un fichier de clé Proxmox Backup Server (le format qu'écrit `proxmox-backup-client key create`, donc les clés sont interchangeables avec le client officiel). Chaque chunk est chiffré en AES-256-GCM et le manifeste est signé. Le serveur PBS ne voit jamais la clé.

**GUI :** ouvrez **Préférences > Informations du compte**, modifiez un serveur et utilisez son onglet **Chiffrement**. **Importer...** une clé que vous avez déjà (une copie du fichier de clé, ou une clé papier lue avec la caméra, depuis un scan ou une photo, ou saisie au clavier) ou **Générer...** une nouvelle clé. L'onglet affiche l'empreinte de la clé. Les rapports indiquent si chaque sauvegarde était chiffrée. **Sans le fichier de clé, un snapshot ne peut pas être restauré : gardez-en une copie en lieu sûr, loin de cette machine.**

**Clés protégées par phrase secrète (scrypt ou PBKDF2) :** choisissez comment l'application déverrouille la clé :
- **Me la demander** : la phrase secrète est demandée une fois par session de l'application et gardée en mémoire uniquement. Les sauvegardes planifiées et au démarrage ne la demandent jamais : elles échouent immédiatement, avec un message clair, tant que la phrase secrète n'a pas été saisie dans cette session. Une sauvegarde lancée par le service d'arrière-plan échoue aussi immédiatement avec une clé « Me la demander ».
- **La mémoriser sur cet ordinateur** : la phrase secrète est stockée dans la configuration comme le jeton PBS, donc toute personne qui peut lire `config.json` peut déverrouiller la clé. C'est le choix pour les tâches planifiées sans surveillance et pour le service d'arrière-plan.

La restauration demande la phrase secrète quand vous sélectionnez un snapshot chiffré.

**CLI :**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` n'est nécessaire que pour une clé protégée, et elle est demandée si l'option est omise. `proxmoxbackup-machine` et `proxmoxbackup-nbd` acceptent les deux mêmes options ; `proxmoxbackup-nbd` en a besoin pour attacher une image disque chiffrée et vérifie le premier chunk avant de toucher au périphérique NBD, donc une mauvaise clé échoue immédiatement.

**Restauration bare-metal :** l'ISO de restauration demande la clé (aucune, un fichier de clé sur une clé USB, ou un chemin saisi) et la phrase secrète d'une clé protégée. La clé n'est jamais écrite dans l'ISO. Voir [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md).

**Limites connues**
- Une clé générée par la GUI est écrite avec une ACL protégée (utilisateur courant, SYSTEM et Administrateurs uniquement). Les fichiers de clé que vous conservez ailleurs gardent les permissions que leur donne leur dossier.
- `config.json` contient en clair les jetons PBS, les mots de passe et toute phrase secrète mémorisée. Depuis la v0.9.0, son dossier n'est accessible qu'aux administrateurs (Windows) ou à son seul propriétaire (Linux) ; jusqu'à la v0.8.2, il était lisible par les utilisateurs locaux sous Windows, donc restreignez-le vous-même sur une machine partagée.
- Restaurer un snapshot chiffré nécessite le même fichier de clé ; aucune récupération n'est possible s'il est perdu.
- Le mode « Me la demander » et le service sont des processus séparés : utilisez « La mémoriser » pour tout ce qui passe par le service.

## Sauvegardes de dossiers plus rapides : détection des changements

Dans le mode Classique par défaut, chaque sauvegarde de dossiers lit tous les fichiers, donc une exécution dure aussi longtemps que la lecture du dossier entier, même si rien n'a changé. Les ensembles de sauvegarde (et la CLI) peuvent utiliser à la place les modes de détection des changements du `proxmox-backup-client` officiel. Le mode Métadonnées est utilisé au quotidien sur un serveur de 480,9 Go et 761 866 fichiers, où une exécution avec peu de changements prend 10 à 15 minutes au lieu de plus de 4 heures.

| Mode | Lit | Stocké sous la forme |
|---|---|---|
| **Classique** (`legacy`, par défaut) | tous les fichiers | un `<name>.pxar.didx` par dossier, comme avant |
| **Données** (`data`) | tous les fichiers | une archive scindée : `<name>.mpxar.didx` (métadonnées) + `<name>.ppxar.didx` (contenu des fichiers) |
| **Métadonnées** (`metadata`) | seulement les fichiers nouveaux et modifiés | la même archive scindée ; les fichiers dont la taille et la date de modification (à la nanoseconde), le mode et le propriétaire correspondent à la sauvegarde précédente en sont repris sans être lus |

En mode **Métadonnées**, la première exécution lit tout (il n'y a encore rien à comparer) ; les exécutions suivantes ne lisent que ce qui a changé. Un fichier modifié d'une façon qui conserve sa taille et sa date de modification n'est pas détecté, c'est pourquoi un ensemble peut aussi **relire tous les fichiers toutes les N exécutions** (une exécution Données) : cette exécution détecte ces changements et supprime le remplissage que la réutilisation laisse dans l'archive. Un fichier qui n'a pas pu être lu entièrement (complété par des zéros après une erreur de lecture) est toujours relu à l'exécution suivante. La réutilisation ne se fait qu'à partir d'une sauvegarde précédente chiffrée de la même façon, avec la même clé.

Comme les données inchangées ne sont ni lues ni envoyées, le trafic vers un PBS distant ou hors site se réduit aussi à peu de chose près à ce qui a réellement changé.

**GUI :** modifiez un ensemble de sauvegarde, onglet **Destination**, **Détection des changements**. Cela ne concerne que les ensembles de dossiers : les sauvegardes de machines (disque entier) sont des images disque et ne sont pas affectées, donc les restaurations bare-metal et Proxmox VE fonctionnent exactement comme avant. Les sauvegardes ponctuelles utilisent le mode Classique. Seules les exécutions terminées avec succès comptent pour la prochaine lecture complète ; la liste des ensembles de sauvegarde affiche « Prochaine relecture complète dans N exécutions ». La carte de progression indique quand et pourquoi une exécution lit tous les fichiers. **Rapports** affiche le mode de chaque exécution et le nombre de fichiers inchangés réutilisés.

**Un Backup ID par ensemble.** Le mode Métadonnées compare avec le snapshot le plus récent du groupe de sauvegarde. Si deux ensembles (ou deux machines) utilisent le même Backup ID sur le même serveur, chaque exécution compare avec le snapshot de l'autre et relit tout. L'éditeur d'ensembles de sauvegarde avertit de ce cas.

**CLI :** `-change-detection-mode legacy|data|metadata`, ou `"change-detection-mode"` dans la configuration JSON. Il n'y a pas d'option CLI « toutes les N exécutions » : planifiez plutôt de temps en temps une exécution avec `-change-detection-mode data`.

**Avant de passer un ensemble en Données ou Métadonnées,** mettez à jour en v0.6.0 ou ultérieure chaque machine depuis laquelle vous restaurez : les versions antérieures à la prise en charge des archives scindées ne savent pas les lire (elles affichent les dossiers sans leurs fichiers). Le `proxmox-backup-client` officiel les restaure à l'octet près (testé avec la 3.4.9), et le navigateur de fichiers de PBS 4.2 les ouvre. La première sauvegarde scindée d'un dossier partage peu de données avec ses anciennes sauvegardes Classique, donc elle coûte à peu près une copie complète de plus de ce dossier sur le datastore, jusqu'à ce que les anciens snapshots soient purgés.

## 🚀 Démarrage rapide (GUI)

1. Installez-le : le MSI sous Windows, le `.deb` ou le `.rpm` sous Linux, ou décompressez le zip ou le tar.gz des [versions publiées](https://github.com/mjb-is/proxmoxbackupclient_go/releases) (voir [Téléchargement et installation](#-téléchargement-et-installation))
2. Lancez-le. Sous Windows, **Exécuter en tant qu'administrateur** pour VSS, les sauvegardes de machines et la modification des ensembles de sauvegarde. Sous Linux, les sauvegardes de dossiers de vos propres fichiers fonctionnent sous votre compte ; utilisez **Proxmox Backup Client (administrator)** (ou lancez-le en root) pour les sauvegardes de machines et les snapshots de blocs
3. Ajoutez votre serveur PBS dans **Préférences > Informations du compte** et cliquez sur **Test**
4. Créez un ensemble de sauvegarde (ou une sauvegarde ponctuelle) et sélectionnez les dossiers ou les disques à sauvegarder
5. Lancez-le. Pour que les planifications s'exécutent sans session ouverte, activez **Préférences > Avancé > Exécuter en tant que service**

Le [manuel de la GUI](docs/manual/GUI.md) détaille chaque étape.

## Sauvegarde complète de machine à chaud

`proxmoxbackup-machine` (et les sauvegardes de machines de la GUI) sauvegardent un système Windows 10/11 ou Windows Server complet, ou une machine Linux, sans aucune interruption de service.

La syntaxe de la commande est en grande partie la même que pour les sauvegardes de dossiers, sauf que `-backupdir` est remplacé par `-backupdev` (à répéter pour plusieurs disques).
Par exemple, un appel pourrait être :

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

La commande ci-dessus examine le disque 0, détecte toutes les partitions montées, en prend un snapshot VSS, puis crée une image de sauvegarde amorçable du disque entier au format fidx. L'espace libre et les fichiers d'échange et d'hibernation entrent dans l'image sous forme de zéros sans être lus (sous Linux, l'espace libre ext2/3/4 et XFS aussi) ; `-readfreespace` lit tout à la place, pour une copie forensique. Un secteur illisible entre dans l'image sous forme de zéros, et l'exécution indique où. Ajoutez `-type vm` pour stocker la sauvegarde comme sauvegarde de VM Proxmox VE ([P2V](#-machines-physiques-en-machines-virtuelles-p2v)).

La sauvegarde suivante sera incrémentielle ; le hachage a été parallélisé, si bien que des vitesses de 1 Go/s peuvent facilement être atteintes.

### Prérequis des sauvegardes de machines Linux (équivalent de VSS)

Les sauvegardes de machines Windows utilisent VSS (d'où le « Exécuter en tant qu'administrateur » ci-dessus) ; Linux n'a pas de VSS, donc ce
projet utilise le module noyau `elastio-snap` (un fork de `dattobd`) pour la même tâche : un
snapshot cohérent à un instant donné d'un périphérique bloc actif pendant qu'on continue à y écrire. Le dépôt
d'origine `elastio/elastio-snap` est archivé ; la version maintenue est
[`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap), c'est celle-là qu'il faut installer. `dattobd`
lui-même est aussi pris en charge s'il est déjà chargé.

Les deux conditions suivantes sont obligatoires, indépendamment l'une de l'autre, sinon une sauvegarde de machine Linux refuse de s'exécuter :

1. **Le module noyau doit être installé et chargé.** L'installation par paquet de dépôt documentée par `elastio-snap`
   (voir [son INSTALL.md](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md)) pointe actuellement
   vers une URL déplacée ou cassée, au moins pour Ubuntu 22.04 (confirmé le 2026-09-24 : le paquet du dépôt
   renvoie 404 après une redirection). La compilation depuis les sources fonctionne de façon fiable :
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
   Cela compile et charge le module pour le démarrage en cours seulement ; consultez la documentation de votre distribution pour le charger automatiquement
   à chaque démarrage (par exemple une entrée dans `/etc/modules-load.d/`) si vous le souhaitez.

2. **Le processus de sauvegarde lui-même doit s'exécuter en root (EUID 0).** Même avec le module chargé, `CreateVSSSnapshot`
   (le chemin de snapshot Linux, malgré son nom centré sur Windows) refuse explicitement de s'exécuter avec un utilisateur non root :
   un snapshot cohérent de disque entier nécessite l'interface de contrôle du module, réservée à root, donc c'est une exigence
   stricte, que l'appartenance au groupe `disk` ou autre ne peut pas remplacer. Pour une sauvegarde de machine, utilisez
   l'entrée de menu **Proxmox Backup Client (administrator)**, le service d'arrière-plan, ou lancez la GUI ou
   `proxmoxbackup-machine` avec `sudo` (ou en root) ; une simple sauvegarde de fichiers ou de dossiers n'a pas cette exigence.

Si une sauvegarde de machine échoue avec `backup device /dev/sdX: open /dev/sdX: permission denied`, il suffit généralement
d'ajouter votre utilisateur au groupe `disk` pour l'accès en lecture de base (`sudo usermod -aG disk $USER`, puis se déconnecter
et se reconnecter), mais cela seul ne suffit PAS pour un vrai snapshot ; vous vous heurterez quand même à l'exigence root ci-dessus
juste après.

### Restauration de fichiers depuis une sauvegarde de disque

La page Restauration de la GUI ouvre une sauvegarde de machine comme une sauvegarde de dossiers : chaque volume de l'image (NTFS, FAT, exFAT, ext2/3/4 ou XFS) est un dossier que vous pouvez parcourir et depuis lequel vous pouvez restaurer. Rien n'est monté.

Sous Linux, vous pouvez aussi attacher une sauvegarde de disque comme périphérique bloc avec l'outil nbd.

Pour utiliser nbd, exécutez d'abord `modprobe nbd max_part=0`.

Pour des raisons inconnues, utiliser `max_part != 0` provoque une boucle infinie de détection des partitions.

L'outil NBD connecte n'importe quelle sauvegarde de disque fixe, qu'il s'agisse d'une sauvegarde de VM ou d'hôte (cela dit, il fonctionne aussi avec les sauvegardes PVE).

Pour l'utiliser, employez une ligne de commande semblable à celle-ci :
`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Si vous omettez `-path`, une interface en mode texte s'affiche pour vous permettre de choisir le fichier fidx.

> Ne l'utilisez pas sur une machine qui fait tourner quoi que ce soit d'important : un système de fichiers corrompu peut faire planter le système d'exploitation, c'est pourquoi Proxmox VE utilise une instance QEMU pour cela.

Démontez tout ce qui se trouve sur le disque nbd avant d'arrêter `proxmoxbackup-nbd`, sinon la partition risque de rester occupée et impossible à démonter.

Si vous obtenez une erreur `Device or resource busy`, vous devez forcer la déconnexion avec `nbd-client -d /dev/nbd0` ou simplement redémarrer.

### Restauration sur une machine physique ou une VM

L'ISO de restauration bare-metal propose une entrée de démarrage entièrement automatisée **« PBS Bare Metal Restore »** : voir [ISO de restauration bare-metal](#-iso-de-restauration-bare-metal) ci-dessous. C'est la première entrée, choisie par défaut à l'expiration du délai, et elle saute les invites de langue et de clavier de Clonezilla : saisissez les informations de connexion PBS, choisissez un snapshot, confirmez le disque cible détecté, et elle se charge de la configuration réseau, de l'attachement NBD, de la restauration elle-même et d'un redémarrage automatique sans autre intervention. La même ISO restaure dans une machine virtuelle vide pour le [P2V](#-machines-physiques-en-machines-virtuelles-p2v).

L'ancien parcours semi-manuel est toujours présent sur la même ISO, comme entrée distincte dans le menu normal de sélection du mode de Clonezilla (« pbs-nbd ») : il vous guide pour les informations de connexion PBS et attache le snapshot choisi à `/dev/nbd0` en arrière-plan, puis vous laisse conduire vous-même la restauration avec l'assistant standard de restauration de disque de Clonezilla.

## Utilisation : sauvegarde de dossiers

La référence complète, avec des exemples pour Windows et Linux, les codes de sortie et la planification, se trouve dans le **[manuel de la ligne de commande](docs/manual/CLI.md)**. Une commande type :

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
  -no-mtime                  Record 1970-01-01 as every file and folder time (advanced; see the CLI manual)
  -backupstream string       Back up standard input as <name>.didx instead of a directory
  -pxarout string            Also write the archive to a local file (debugging)
  -mail-host, -mail-port, -mail-username, -mail-password, -mail-insecure,
  -mail-from, -mail-to, -mail-subject-template, -mail-body-template
                             Email report (optional)
  -config string             JSON config file; flags given on the command line override it
```

Les variables d'environnement `PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD` et `PBS_FINGERPRINT` du client officiel sont aussi lues.

Pour la configuration JSON, un exemple JSON est fourni ; ne remplissez que les champs nécessaires.

Remarque sur les modèles d'e-mail :
le [moteur de modèles de Go](https://pkg.go.dev/text/template) est utilisé pour les sujets et les corps des e-mails ; reportez-vous à sa documentation pour la syntaxe.
Les variables suivantes sont disponibles dans les modèles :

- `.NewChunks` : nombre de nouveaux chunks créés
- `.ReusedChunks` : nombre de chunks réutilisés
- `.Datastore` : nom du datastore
- `.Error` : l'erreur, s'il y en a une (`.ErrorStr` la donne sous forme de texte, vide s'il n'y en a pas)
- `.ReadErrors` : les fichiers ou dossiers qui n'ont pas pu être lus ; `.ReadErrorCount` en donne le nombre
- `.Hostname` : nom d'hôte de la machine
- `.StartTime` : heure de début de la sauvegarde
- `.EndTime` : heure de fin de la sauvegarde
- `.Duration` : durée de la sauvegarde
- `.FromattedDuration` : durée formatée de la sauvegarde (le nom est orthographié ainsi dans le code)
- `.Success` : vrai quand la sauvegarde s'est terminée sans erreur et sans rien d'illisible
- `.Partial` : vrai quand un snapshot a été enregistré mais que certains fichiers n'ont pas pu être lus
- `.Status` : `Success`, `Partial` ou `Failed`

## Sauvegarde de flux

Cela permet de sauvegarder un flux au lieu d'un pxar, ce qui ouvre des possibilités infinies. Par exemple, vous pouvez lancer :

```
mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]
```

Cela permet de profiter de buzhash pour la déduplication même en utilisant tar par exemple, ou le dump SQL lui-même.

## Problèmes connus

Quand l'antimalware Windows Defender est actif, la sauvegarde ralentit jusqu'à 25 % de la vitesse atteignable.

Sous Windows, les outils en ligne de commande prennent un verrou de processus : une deuxième sauvegarde lancée pendant qu'une autre est en cours affiche « Another backup job is running » et s'arrête. Sous Linux, il n'y a pas de tel verrou, donc ne planifiez pas deux exécutions CLI en même temps.

## 📋 Prérequis

- Windows 10/11 ou Windows Server (64 bits), avec les droits administrateur pour VSS et les sauvegardes de machines
- ou Linux (les paquets sont pour Debian, Ubuntu, Mint et Fedora), avec le runtime WebKitGTK pour la GUI, et root plus le module noyau elastio-snap ou dattobd pour des sauvegardes de machines cohérentes
- Un accès réseau à un Proxmox Backup Server
- Pour le P2V : Proxmox VE avec le datastore PBS ajouté comme stockage, ou tout hyperviseur capable de démarrer l'ISO de restauration

## 🔨 Compilation depuis les sources

**[BUILD.md](BUILD.md)** contient les instructions complètes : la GUI et le service d'arrière-plan, les outils en ligne de commande, le MSI et les paquets Linux. En bref, avec Go 1.25 ou ultérieur et Wails CLI v2.13 :

```bash
cd gui
wails build                                    # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe   # the background service
```

```bash
packaging/debian/build-deb.sh     # .deb
packaging/fedora/build-rpm.sh     # .rpm (in Docker)
```

```powershell
installer\wix\build-msi.ps1       # MSI (WiX 3), after building the app and the service
```

Le frontend compilé (`gui/frontend/dist`) est versionné, donc Node.js n'est nécessaire que pour modifier le frontend (`npm install` et `npm run build` dans `gui/frontend`, ou `wails dev` pour le rechargement à chaud). Tests : `make test`.

## 🔧 Utilisation avancée et guides

### Multi-PBS (plusieurs serveurs PBS)

Configurez plusieurs serveurs PBS et choisissez la cible par ensemble de sauvegarde ou par sauvegarde ponctuelle (par exemple `C:\Users` vers un PBS rapide sur SSD chaque jour, `D:\` vers un PBS de grande capacité chaque semaine, plus un serveur de reprise après sinistre).

- **[Guide utilisateur](MULTI_PBS_USER_GUIDE.md)** : ajouter et tester des serveurs, le serveur par défaut, choisir un serveur par sauvegarde, FAQ et dépannage.
- **[Guide d'implémentation](MULTI_PBS_GUIDE.md)** : modèle de données, migration automatique depuis une configuration à un seul PBS, méthodes du backend.

Une ancienne configuration à un seul PBS est automatiquement migrée vers un serveur `default` au premier chargement.

### 💿 ISO de restauration bare-metal

L'ISO de restauration est une ISO Clonezilla Live standard patchée avec les binaires `pbsnbd` / `machinebackup` plus deux entrées de menu. Elle démarre depuis un CD, depuis une clé USB (écrite avec `dd`, ou depuis une clé Ventoy), par PXE, et sur des machines UEFI ou BIOS :

- **`ocs-pbs-bare-metal-restore`** : la première entrée du menu de démarrage, choisie par défaut à l'expiration du délai, **« Proxmox Backup Client Go - PBS Bare Metal Restore »**. Elle saute les invites de langue et de clavier de Clonezilla et son assistant générique, et va directement des identifiants PBS à une restauration terminée suivie d'un redémarrage automatique : DHCP d'abord (une invite d'IP statique seulement en cas d'échec), synchronisation NTP, une clé de chiffrement depuis une clé USB si la sauvegarde est chiffrée, et une détection du disque cible qui ne propose jamais la clé USB sur laquelle elle a démarré. En cas de succès, elle affiche un message de fin et redémarre. Elle restaure indifféremment les sauvegardes Windows et Linux (la restauration se fait au niveau des blocs), sur une machine physique ou dans une VM vide.
- **`ocs-pbs-nbd`** : une entrée **« pbs-nbd »** dans le menu de sélection du mode de Clonezilla. Elle demande vos informations de connexion PBS et le snapshot à restaurer, l'attache à `/dev/nbd0` en arrière-plan, puis passe la main à l'assistant normal de restauration de disque de Clonezilla, que vous conduisez vous-même.

Également corrigés côté restauration : un interblocage de `zstd.NewReader` dans le chemin de récupération des chunks PBS qui bloquait les restaurations de machines entières (proposé au projet d'origine sous la forme de la [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85)), un bug « la première passe de restauration échoue avec 'no partition', la seconde réussit » causé par une course entre la fonction `is_disk_without_part_and_fs()` de Clonezilla et udev, et les machines BIOS/MBR qui ne démarraient pas après une restauration (les données situées entre le MBR et la première partition sont désormais restaurées aussi).

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

Tous les détails (pourquoi une reconstruction complète de l'ISO plutôt qu'un remplacement sur place, prérequis, déroulement du menu, vérification) sont dans **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)** ; son utilisation est décrite dans le **[manuel de restauration bare-metal](docs/manual/BMR.md)**.

### Compiler la GUI Windows

**Windows natif** : voir **[BUILD.md](BUILD.md)** pour la chaîne d'outils (Go, MinGW, Wails) et les commandes pour l'application, le service et le MSI.

**Docker (pour compiler sous Linux).** `./build_gui_windows_docker.sh` compile l'application Windows sous le nom `ProxmoxBackupClientGO.exe`, avec une prise en charge correcte de WebView2, dans un conteneur `golang` jetable (installe mingw et Wails, compile le frontend, lance `wails build`). Il ne compile ni le service d'arrière-plan ni le MSI.

### État des fonctionnalités, changelog et documentation interne

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)** : ce qui est fait, partiel ou pas fait, et depuis quelle version.
- **[CHANGELOG.md](CHANGELOG.md)** : historique des modifications par version, et [Unreleased] pour ce qui est sur master.
- **[TODO.md](TODO.md)** : feuille de route ouverte et idées.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)** : le contenu de chaque version.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)** : le dialogue de désinstallation du MSI (conserver ou supprimer la configuration) et son plan de test.
- **[AGENTS.md](AGENTS.md)** : organisation et conventions du dépôt, pour les contributeurs et les agents de programmation.

## 🔁 Contributions au projet d'origine

Les corrections et fonctionnalités d'intérêt général sont proposées à [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) sous forme de pull requests. Dix-huit ont été intégrées :

  - [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85): Fix zstd decoder goroutine deadlock in `GetChunkData` (hung full-machine restores)
  - [PR #86](https://github.com/tizbac/proxmoxbackupclient_go/pull/86): Fully automated PBS Bare Metal Restore boot entry for the Clonezilla ISO
  - [PR #87](https://github.com/tizbac/proxmoxbackupclient_go/pull/87): Linux snapshots: drive elastio-snap/dattobd through ioctl instead of `elioctl`/`dbdctl`
  - [PR #88](https://github.com/tizbac/proxmoxbackupclient_go/pull/88): Restore: re-apply captured NTFS ACLs (Windows) and POSIX ACLs and xattrs (Linux)
  - [PR #90](https://github.com/tizbac/proxmoxbackupclient_go/pull/90): `machinebackuplib`: make `vm` backups restorable and bootable on Proxmox VE
  - [PR #91](https://github.com/tizbac/proxmoxbackupclient_go/pull/91): GUI: back machines up as `host`, not `vm`, by default
  - [PR #92](https://github.com/tizbac/proxmoxbackupclient_go/pull/92): `patch-clonezilla`: skip patches whose change is already in the base ISO
  - [PR #93](https://github.com/tizbac/proxmoxbackupclient_go/pull/93): `patch-clonezilla`: add the PBS bare-metal restore entry to `isolinux.cfg`
  - [PR #94](https://github.com/tizbac/proxmoxbackupclient_go/pull/94): Restore the data between the MBR and the first partition in the BMR flow (BIOS/MBR machines boot after restore)
  - [PR #95](https://github.com/tizbac/proxmoxbackupclient_go/pull/95): Fix reading passphrase-protected key files made by `proxmox-backup-client`
  - [PR #96](https://github.com/tizbac/proxmoxbackupclient_go/pull/96): `directorybackup`: back up several directories in one run
  - [PR #97](https://github.com/tizbac/proxmoxbackupclient_go/pull/97): Docs: point elastio-snap at the maintained Axcient fork
  - [PR #98](https://github.com/tizbac/proxmoxbackupclient_go/pull/98): `pbscommon`: skip the key file permission check on Windows
  - [PR #99](https://github.com/tizbac/proxmoxbackupclient_go/pull/99): Restore encrypted backups from the Clonezilla BMR ISO
  - [PR #102](https://github.com/tizbac/proxmoxbackupclient_go/pull/102): `snapshot`: delete our VSS shadow copies after every backup, through the VSS API
  - [PR #103](https://github.com/tizbac/proxmoxbackupclient_go/pull/103): `machinebackup`: skip hashing blank (all-zero) chunks
  - [PR #104](https://github.com/tizbac/proxmoxbackupclient_go/pull/104): CLIs: report errors on stderr, never in a message box
  - [PR #105](https://github.com/tizbac/proxmoxbackupclient_go/pull/105): Restore: put back every file time, also 1970 and earlier, and folder times

## 🧪 Statut, rapports de bugs et demandes de fonctionnalités

**Ceci est un logiciel alpha, fourni tel quel, sans aucune garantie de quelque nature que ce soit, et utilisé entièrement à vos propres risques.** Je décline toute responsabilité en cas de perte ou d'endommagement de données, y compris des fichiers sources.

Il est conçu pour mon propre homelab. Je teste moi-même chaque version avant sa publication, et il y est utilisé régulièrement : sauvegardes planifiées de dossiers et de machines Windows et Linux, avec des restaurations, des Récupérations, des Retours arrière, des restaurations bare-metal et des restaurations P2V testées au fil du temps. Aucune sauvegarde n'est fiable tant que vous n'avez pas restauré à partir d'elle, donc testez vos propres restaurations, y compris une restauration bare-metal, avant de vous y fier.

- **Les rapports de bugs sont les bienvenus** : ouvrez une [issue](https://github.com/mjb-is/proxmoxbackupclient_go/issues) avec ce que vous avez fait, ce qui s'est passé et le journal (le Journal des messages, ou les fichiers du dossier des paramètres).
- **Demandes de fonctionnalités** : j'ajoute les fonctionnalités utiles à mon propre usage, donc je ne prends pas de demandes pour d'autres usages. Merci de les adresser au projet d'origine, [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go/issues). Les pull requests qui corrigent des bugs, ou qui seraient clairement utiles à mon usage aussi, sont les bienvenues.

Les rapports de bugs et les correctifs sont l'aide la plus utile. Si vous souhaitez remercier autrement, vous pouvez [m'offrir un café sur Ko-fi](https://ko-fi.com/mickbeeby).

## 🙏 Remerciements et historique

Le client en ligne de commande d'origine a été écrit par Tiziano Bacocco (tizbac) à partir de 2023. Le client graphique de RDEM Systems y a ensuite été intégré, et avec lui la première GUI, la prise en charge multi-PBS et le service Windows. Ce fork a commencé en septembre 2026 pour combler les manques qui empêchaient PBS d'être mon unique système de sauvegarde ; ses versions partent de la v0.3.0, et les corrections d'intérêt général sont renvoyées au projet d'origine.

- Le client d'origine, [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go), de Tiziano Bacocco (tizbac) et ses contributeurs.
- Le client graphique Nimbus Backup de RDEM Systems, point de départ de l'interface graphique de cette application.
- Le fonctionnement et l'esprit de Backup for Workgroups (BFW), dont s'inspirent les ensembles de sauvegarde, la Récupération et le Retour arrière.
- [Clonezilla Live](https://clonezilla.org/), sur lequel repose l'ISO de restauration bare-metal.

## 📄 Licence

GPLv3 : voir le fichier [LICENSE](LICENSE).

---

**Modifications depuis le fork © 2026 mjb-is, à partir du travail de Tiziano Bacocco (tizbac), des contributeurs de Proxmox Backup Client GO et de RDEM Systems (© 2024-2026).**

☕ Si ce client vous est utile, vous pouvez [m'offrir un café sur Ko-fi](https://ko-fi.com/mickbeeby). Merci.
