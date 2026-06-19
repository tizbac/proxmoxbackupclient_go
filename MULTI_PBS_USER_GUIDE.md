# Guide utilisateur - Multi-PBS

## 🎯 À quoi sert Multi-PBS ?

La fonctionnalité **Multi-PBS** permet de configurer plusieurs serveurs Proxmox Backup Server et de choisir lequel utiliser pour chaque backup.

### Cas d'usage

#### Exemple 1 : Performance vs Capacité
```
📁 C:\Users (30 GB, critique) → PBS SSD (rapide, quotidien)
📁 C:\ (500 GB, archive)       → PBS Big Data (lent, hebdomadaire)
```

#### Exemple 2 : Production + Disaster Recovery
```
📁 Tous les dossiers → PBS Production (principal)
                     + PBS DR (secours, réplication)
```

#### Exemple 3 : Multi-datastore
```
📁 Documents → Datastore "documents"
📁 Vidéos   → Datastore "media"
📁 Code     → Datastore "dev-backups"
```

---

## 🚀 Guide pas à pas

### Étape 1 : Accéder à la gestion des serveurs

1. Ouvrez Nimbus Backup
2. Cliquez sur l'onglet **"Serveurs PBS"** (2ème onglet)

![Onglet Serveurs PBS](docs/tab-servers.png)

---

### Étape 2 : Ajouter votre premier serveur PBS

**Note :** Si vous aviez déjà configuré un serveur PBS dans l'ancien onglet "Configuration PBS", il a été **automatiquement migré** vers "Serveurs PBS" avec l'ID `default`.

#### Ajouter un nouveau serveur

1. Remplissez le formulaire en bas de page :

| Champ | Exemple | Requis | Description |
|-------|---------|--------|-------------|
| **Nom du serveur** | `SSD Rapide` | ✅ | Nom affiché dans l'interface |
| **ID du serveur** | `pbs-ssd` | ⚠️ | Laissez vide pour auto-génération |
| **URL du serveur PBS** | `https://pbs-ssd.local:8007` | ✅ | Adresse HTTPS du PBS |
| **Authentication ID** | `backup@pbs!nimbus` | ✅ | API Token (user@realm!token) |
| **Secret (API Token)** | `xxxxxxxx-xxxx-xxxx...` | ✅ | Secret du token |
| **Datastore** | `ssd-fast` | ✅ | Nom du datastore PBS |
| **Namespace** | `clients` | ❌ | Optionnel, pour organiser |
| **Empreinte SSL** | `AA:BB:CC:DD:...` | ❌ | Optionnel, pour validation |
| **Description** | `Stockage SSD rapide` | ❌ | Optionnel, aide-mémoire |

2. Cliquez sur **"➕ Ajouter le serveur"**

---

### Étape 3 : Tester la connexion

Une fois le serveur ajouté, il apparaît dans la liste avec le statut **⚪ Non testé**.

1. Cliquez sur le bouton **"🔍 Tester"** à droite du serveur
2. Statut passe à :
   - **🔄 Test...** pendant la connexion
   - **🟢 Online** si connexion réussie
   - **🔴 Offline** si échec

**Astuce :** Testez tous vos serveurs après configuration pour vérifier qu'ils sont accessibles.

---

### Étape 4 : Définir le serveur par défaut

Le serveur **par défaut** (marqué ⭐) est utilisé automatiquement pour les backups si aucun serveur n'est spécifié.

1. Trouvez le serveur que vous voulez définir par défaut
2. Cliquez sur **"⭐ Par défaut"**
3. L'étoile ⭐ se déplace vers ce serveur

**Note :** Le premier serveur ajouté est automatiquement défini par défaut.

---

### Étape 5 : Utiliser Multi-PBS dans les backups

#### Option 1 : Backup One-Shot (Pas encore implémenté)

*À venir : Dropdown de sélection PBS dans l'onglet "Sauvegarde"*

#### Option 2 : Jobs planifiés (Pas encore implémenté)

*À venir : Sélection PBS lors de la création d'un job*

**Pour l'instant**, tous les backups utilisent automatiquement le serveur **par défaut** (⭐).

---

## 🔧 Gestion avancée

### Modifier un serveur existant

1. Cliquez sur **"✏️ Modifier"** à droite du serveur
2. Le formulaire se remplit avec les infos actuelles
3. Modifiez les champs souhaités
4. Cliquez sur **"💾 Mettre à jour"**

**Attention :** Modifier l'URL ou les credentials peut casser les backups en cours.

---

### Supprimer un serveur

1. Cliquez sur **"🗑️ Supprimer"** à droite du serveur
2. Confirmez la suppression

**Attention :**
- ❌ **Impossible de supprimer le serveur par défaut** → Définissez d'abord un autre serveur par défaut
- ⚠️ Les jobs planifiés utilisant ce serveur **échoueront**

---

### Gérer plusieurs serveurs (exemples)

#### Scénario : Backup quotidien + DR hebdomadaire

1. **Ajouter PBS Production**
   - Nom : `PBS Production`
   - ID : `pbs-prod`
   - URL : `https://pbs-prod.local:8007`
   - Datastore : `backup-prod`
   - ⭐ Définir par défaut

2. **Ajouter PBS DR (Disaster Recovery)**
   - Nom : `PBS DR`
   - ID : `pbs-dr`
   - URL : `https://pbs-dr.remote:8007`
   - Datastore : `backup-dr`

3. **Créer 2 jobs planifiés** (à venir) :
   - Job "Backup quotidien" → `pbs-prod` (2h du matin)
   - Job "Backup DR" → `pbs-dr` (samedi 3h du matin)

---

## 📊 Tableau de bord

### Liste des serveurs

La liste affiche :
- **Nom** : Nom du serveur + ⭐ si défaut + description
- **URL** : Adresse du PBS
- **Datastore** : Datastore/namespace
- **Statut** : 🟢 Online | 🔴 Offline | ⚪ Non testé
- **Actions** : Boutons de gestion

### Compteur

En haut : **"Serveurs configurés (N)"** affiche le nombre total.

---

## ❓ FAQ

### Que se passe-t-il avec mon ancienne configuration ?

✅ **Migration automatique** : Votre ancien serveur PBS (onglet "Configuration PBS") a été automatiquement migré vers **"Serveurs PBS"** avec l'ID `default` et marqué ⭐ par défaut.

Vos backups continuent de fonctionner normalement.

---

### Puis-je avoir 2 serveurs avec le même datastore ?

✅ **Oui**, tant que les URLs sont différentes. Exemple :
- PBS Production : `https://pbs1.local:8007` → datastore `backup`
- PBS DR : `https://pbs2.local:8007` → datastore `backup`

---

### Comment savoir quel serveur est utilisé pour un backup ?

Pour l'instant, c'est toujours le serveur **par défaut** (⭐).

À venir : les jobs planifiés afficheront le PBS utilisé.

---

### Que faire si un serveur est 🔴 Offline ?

1. **Vérifier la connectivité réseau** : Ping, firewall, VPN
2. **Vérifier l'URL** : HTTPS, port 8007, certificat SSL
3. **Vérifier les credentials** : AuthID et Secret corrects
4. **Tester manuellement** : Se connecter à l'interface web PBS

Si le problème persiste, définissez un autre serveur par défaut temporairement.

---

### Puis-je dupliquer un serveur ?

❌ Pas directement. Workaround :
1. Cliquez sur "✏️ Modifier" sur le serveur à dupliquer
2. Copiez manuellement les infos
3. Cliquez sur "❌ Annuler"
4. Remplissez le formulaire d'ajout avec les infos copiées
5. Changez l'ID et le nom
6. Cliquez sur "➕ Ajouter"

---

## 🛠️ Dépannage

### Erreur "serveur PBS 'xxx' introuvable"

**Cause :** Le serveur a été supprimé mais un job planifié l'utilise encore.

**Solution :**
1. Allez dans l'onglet "Sauvegarde"
2. Mode "Planifié"
3. Supprimez ou modifiez le job concerné

---

### Erreur "aucun serveur PBS spécifié"

**Cause :** Aucun serveur par défaut défini.

**Solution :**
1. Allez dans "Serveurs PBS"
2. Cliquez sur "⭐ Par défaut" sur n'importe quel serveur

---

### Le statut reste "🔄 Test..." indéfiniment

**Cause :** Timeout de connexion (PBS inaccessible).

**Solution :**
1. Rafraîchissez la page (F5)
2. Le statut passera à 🔴 Offline
3. Vérifiez la connectivité réseau

---

## 🚀 Prochaines fonctionnalités (à venir)

- [ ] **Dropdown PBS** dans backup One-Shot
- [ ] **Sélection PBS** dans création de jobs planifiés
- [ ] **Migration jobs legacy** vers PBSID (bouton "Migrer")
- [ ] **Export/Import** config multi-PBS
- [ ] **Statistiques** par serveur (espace utilisé, nombre de snapshots)
- [ ] **Groupes de serveurs** (ex: "Production", "DR")

---

**Version :** 0.2.0
**Mainteneur :** RDEM Systems
**Support :** contact@rdem-systems.com
**Date :** 2026-03-23
