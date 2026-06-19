# 🔧 Nimbus Backup - Guide de Restauration

> **⚠️ STATUT : À IMPLÉMENTER**
> Ce guide décrit les features de restauration **à développer**.
> Aucune restauration n'est actuellement fonctionnelle dans Nimbus v0.2.x

---

## 📋 Scénarios couverts (roadmap)

| Scénario | Méthode | Temps estimé | Status |
|----------|---------|--------------|--------|
| Fichier supprimé/corrompu | Restore granulaire GUI | 2 min | ❌ À faire |
| Dossier entier à restaurer | Restore granulaire GUI | 5-10 min | ❌ À faire |
| Ransomware | Restore snapshot complet | 30-60 min | ❌ À faire |
| Disque HS (même machine) | Restore bare-metal | 45-90 min | ❌ À faire |
| Serveur complet HS | Fresh install + données | 1-2h | 📝 Doc seulement |
| P2V (Physical to Virtual) | Fresh install + données | 1-2h | 📝 Doc seulement |

---

## 1️⃣ Restore granulaire (fichiers/dossiers)

> **Cas d'usage:** Fichier supprimé, document corrompu, retour arrière

### Via GUI Nimbus (À DÉVELOPPER)

```
1. Ouvrir Nimbus Backup
2. Onglet "Restauration"
3. Sélectionner le snapshot (date/heure)
4. Naviguer dans l'arborescence
5. Cocher fichiers/dossiers à restaurer
6. Choisir destination
7. Options:
   ☑️ Restaurer les permissions (ACLs)
   ☑️ Restaurer les flux alternatifs (ADS)
   ☑️ Restaurer les timestamps
8. Cliquer "Restaurer"
```

**Les permissions NTFS seront restaurées automatiquement** grâce aux métadonnées sidecar (feature NTFS Fidelity - Sprint 1).

---

## 2️⃣ Restore complet après ransomware

> **Cas d'usage:** Chiffrement ransomware, corruption massive

### Procédure

```
1. DÉCONNECTER la machine du réseau (éviter re-propagation)
2. Identifier le dernier snapshot SAIN (avant infection)
3. Option A: Restore par-dessus (si Windows boot encore)
   - Restore dossiers de données (D:\, E:\, Users, etc.)
   - NE PAS restaurer Windows\System32 sur système live

4. Option B: Restore bare-metal (si Windows compromis)
   - Voir section "Disque HS" ci-dessous
```

⚠️ **Important:** Toujours restaurer un snapshot ANTÉRIEUR à l'infection.

---

## 3️⃣ Restore bare-metal (Disque HS - même machine)

> **Cas d'usage:** SSD/HDD mort, remplacé par un neuf

### Prérequis

- ✅ Nouveau disque installé dans la machine
- ✅ Clé USB [SystemRescue](https://www.system-rescue.org/) (ou autre Linux live)
- ✅ Clé USB Windows Installation (pour réparer le boot)
- ✅ Accès réseau au serveur PBS

### Étape 1: Boot SystemRescue

```bash
# Booter sur la clé USB SystemRescue
# Choisir "Boot SystemRescue with default options"
```

### Étape 2: Partitionner le nouveau disque

```bash
# Identifier le disque (généralement sda ou nvme0n1)
lsblk

# Pour système UEFI (GPT) - cas moderne
parted /dev/sda mklabel gpt
parted /dev/sda mkpart EFI fat32 1MiB 512MiB
parted /dev/sda set 1 esp on
parted /dev/sda mkpart Windows ntfs 512MiB 100%
mkfs.fat -F32 /dev/sda1
mkfs.ntfs -f /dev/sda2

# Pour système Legacy BIOS (MBR) - vieux serveurs
parted /dev/sda mklabel msdos
parted /dev/sda mkpart primary ntfs 1MiB 100%
parted /dev/sda set 1 boot on
mkfs.ntfs -f /dev/sda1
```

### Étape 3: Monter et restaurer

```bash
# Monter la partition Windows
mkdir -p /mnt/windows
mount /dev/sda2 /mnt/windows -t ntfs-3g    # UEFI
# ou
mount /dev/sda1 /mnt/windows -t ntfs-3g    # Legacy BIOS

# Lancer la restauration Nimbus (CLI À DÉVELOPPER)
nimbus-restore \
  --server https://pbs.votreserveur.com:8007 \
  --fingerprint "AA:BB:CC:..." \
  --auth "backup@pbs!token-name" \
  --secret "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
  --datastore "backups" \
  --snapshot "SERVER01/latest" \
  --dest /mnt/windows \
  --restore-acls \
  --restore-ads

# Démonter proprement
umount /mnt/windows
```

### Étape 4: Réparer le bootloader Windows

```
1. Retirer clé USB SystemRescue
2. Insérer clé USB Windows Installation
3. Booter dessus
4. Choisir la langue → "Suivant"
5. Cliquer "Réparer l'ordinateur" (en bas à gauche)
6. Dépannage → Réparation du démarrage
7. Laisser Windows réparer
8. Redémarrer
```

### Étape 5: Premier boot Windows

```
✅ Windows démarre normalement
✅ Vérifier les données
✅ Vérifier les permissions (clic droit → Propriétés → Sécurité)
✅ Reconnecter au réseau
✅ Relancer Nimbus Backup pour reprendre les sauvegardes
```

---

## 4️⃣ Changement de serveur / P2V

> **Cas d'usage:** Serveur physique mort, migration vers VM

### ⚠️ Limitation Windows

Windows n'aime pas les changements de hardware (chipset, contrôleur disque). Un restore complet sur hardware différent peut causer:
- BSOD `INACCESSIBLE_BOOT_DEVICE`
- Écran bleu au démarrage
- Drivers manquants

**Ce n'est pas une limite de Nimbus, c'est un comportement Windows.**

### Méthode recommandée (fiable)

```
1. Installer Windows fresh sur le nouveau serveur/VM
2. Configurer Windows (nom machine, domaine, etc.)
3. Installer Nimbus Backup
4. Restaurer les DONNÉES:
   - D:\, E:\ (disques de données)
   - C:\Users (profils utilisateurs)
   - Dossiers applicatifs spécifiques
5. Réinstaller les applications
6. Les données + permissions sont restaurées par Nimbus
```

**Temps:** ~1-2h selon volume de données

### Méthode alternative (peut fonctionner)

```
1. Restore complet Nimbus sur nouveau hardware
2. Booter Windows en Mode Sans Échec (F8 / Shift+F8)
3. Laisser Windows détecter le nouveau hardware
4. Installer les drivers (VMware Tools, Hyper-V IC, etc.)
5. Redémarrer normalement
6. 🤞 Croiser les doigts
```

**Taux de succès:** ~60-70% selon les configurations

---

## 📊 Matrice de décision

```
Disque HS, même machine?
├─ OUI → Restore bare-metal (Section 3)
└─ NON → Hardware différent?
         ├─ OUI → Fresh install + données (Section 4)
         └─ NON (VM identique) → Restore bare-metal (Section 3)
```

---

## 🛠️ Commandes nimbus-restore (À DÉVELOPPER)

### Options principales

| Option | Description |
|--------|-------------|
| `--server URL` | URL du serveur PBS |
| `--fingerprint FP` | Empreinte certificat PBS |
| `--auth ID` | Auth ID (user@realm!token) |
| `--secret SECRET` | Token secret |
| `--datastore NAME` | Nom du datastore |
| `--snapshot ID` | ID snapshot ou "latest" |
| `--dest PATH` | Chemin de destination |
| `--restore-acls` | Restaurer les permissions NTFS |
| `--restore-ads` | Restaurer les Alternate Data Streams |
| `--include PATTERN` | Filtrer les fichiers à restaurer |
| `--exclude PATTERN` | Exclure des fichiers |
| `--dry-run` | Simuler sans restaurer |

### Exemples

```bash
# Restore complet
nimbus-restore --server https://pbs:8007 --snapshot "SRV01/latest" \
  --dest /mnt/windows --restore-acls --restore-ads

# Restore seulement les Users
nimbus-restore --server https://pbs:8007 --snapshot "SRV01/2026-03-20" \
  --dest /mnt/windows --include "Users/**" --restore-acls

# Restore un dossier spécifique
nimbus-restore --server https://pbs:8007 --snapshot "SRV01/latest" \
  --dest /mnt/restore --include "Data/Compta/**"

# Dry-run pour vérifier
nimbus-restore --server https://pbs:8007 --snapshot "SRV01/latest" \
  --dest /mnt/windows --dry-run
```

---

## ❓ FAQ

### Le restore est très lent, c'est normal?

Le premier restore télécharge tout depuis PBS. Vitesse dépend de:
- Bande passante réseau
- Performance PBS
- Vitesse disque destination

### Les permissions ne sont pas restaurées?

Vérifier:
- Option `--restore-acls` activée
- Fichiers `.nimbus_meta` présents dans le backup
- Partition NTFS (pas FAT32)

### Windows ne boot pas après restore?

1. Vérifier que le bootloader est réparé (Section 3, Étape 4)
2. Si hardware différent → Section 4 (fresh install)

### Je peux restaurer sur un NAS/partage réseau?

Oui pour les fichiers, mais les ACLs NTFS ne seront pas appliquées sur un partage SMB. Restaurer sur disque local d'abord si les permissions sont critiques.

---

## 📞 Support

- **Documentation:** https://nimbus.rdem-systems.com/docs
- **Email:** support@rdem-systems.com
- **Urgence disaster recovery:** Contacter le support avec priorité haute

---

## 🚧 Développement

**Voir:** `TODO.md` section "Restauration - À Développer FROM SCRATCH"

**Roadmap:**
1. Sprint 1 (2 semaines) : NTFS Fidelity ← Blocker
2. Sprint Restore-1 (1 semaine) : GUI restore granulaire
3. Sprint Restore-2 (3-4 jours) : CLI nimbus-restore
4. Sprint Restore-3 (2 jours) : Documentation complète

**Total:** 2-3 semaines (après NTFS Fidelity)

---

*Nimbus Backup - RDEM Systems*
*Version 1.0 - Mars 2026*
*Document de spécification - Features à implémenter*
