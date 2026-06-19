# Multi-PBS Architecture - Guide d'implémentation

## 🎯 Vue d'ensemble

Nimbus Backup supporte maintenant **plusieurs serveurs PBS** pour un même client. Cela permet des cas d'usage comme :
- Backup `C:\` vers PBS big-data (gros volumes, lent)
- Backup `C:\Users` vers PBS SSD (petit volume, rapide)
- Backup vers PBS production + PBS DR (disaster recovery)

## 🏗️ Architecture

### Structure de données

```go
// Config globale (gui/config.go)
type Config struct {
    // Nouveau: Map de serveurs PBS (clé = ID serveur)
    PBSServers map[string]*PBSServer `json:"pbs_servers"`

    // ID du serveur PBS par défaut
    DefaultPBSID string `json:"default_pbs_id"`

    // Legacy: anciens champs (rétrocompatibilité)
    BaseURL string `json:"baseurl,omitempty"` // Deprecated
    // ...
}

// Serveur PBS individuel (gui/pbs_server.go)
type PBSServer struct {
    ID              string `json:"id"`          // "default", "pbs-ssd", "pbs-bigdata"
    Name            string `json:"name"`        // "SSD Rapide", "Stockage Big Data"
    BaseURL         string `json:"baseurl"`
    CertFingerprint string `json:"certfingerprint"`
    AuthID          string `json:"authid"`
    Secret          string `json:"secret"`
    Datastore       string `json:"datastore"`
    Namespace       string `json:"namespace"`
    Description     string `json:"description,omitempty"`
    IsOnline        bool   `json:"is_online,omitempty"` // Statut connexion
}

// Job (gui/jobs.go)
type Job struct {
    // Nouveau: référence PBS par ID
    PBSID string `json:"pbs_id,omitempty"` // "pbs-ssd", "pbs-bigdata"

    // Legacy: config PBS embarquée (rétrocompatibilité)
    PBSConfig Config `json:"pbs_config,omitempty"` // Deprecated

    // ...
}
```

## 🔄 Migration automatique

### Ancienne config (single PBS)

```json
{
  "baseurl": "https://pbs.example.com:8007",
  "authid": "backup@pbs",
  "secret": "xxx",
  "datastore": "backup",
  "namespace": "clients"
}
```

### Nouvelle config (multi-PBS)

Au premier chargement, **migration automatique** :

```json
{
  "pbs_servers": {
    "default": {
      "id": "default",
      "name": "Serveur PBS Principal",
      "baseurl": "https://pbs.example.com:8007",
      "authid": "backup@pbs",
      "secret": "xxx",
      "datastore": "backup",
      "namespace": "clients",
      "description": "Serveur PBS par défaut (migré depuis ancienne config)"
    }
  },
  "default_pbs_id": "default",

  "baseurl": "https://pbs.example.com:8007",  // Gardé pour compatibilité
  "authid": "backup@pbs",  // Mais ne sera plus utilisé
  // ...
}
```

## 📝 Utilisation backend (Go)

### Ajouter un serveur PBS

```go
app := NewApp()

newPBS := &PBSServer{
    ID:          "pbs-ssd",
    Name:        "SSD Rapide",
    BaseURL:     "https://pbs-ssd.example.com:8007",
    AuthID:      "backup@pbs",
    Secret:      "yyy",
    Datastore:   "ssd-fast",
    Namespace:   "clients",
    Description: "Stockage SSD pour backups critiques",
}

err := app.AddPBSServer(newPBS)
```

### Lister les serveurs PBS

```go
servers := app.ListPBSServers()
for _, pbs := range servers {
    fmt.Printf("PBS %s: %s (%s)\n", pbs.ID, pbs.Name, pbs.BaseURL)
}
```

### Créer un job avec PBS spécifique

```go
job := &Job{
    Name:        "Backup SSD",
    PBSID:       "pbs-ssd",  // Référence le serveur SSD
    Folders:     []string{"C:\\Users", "C:\\Projects"},
    Schedule:    "daily",
    ScheduleCron: "0 2 * * *",
}

app.SaveScheduledJob(job)
```

### Résoudre la config PBS d'un job

```go
globalConfig := LoadConfig()

// Job peut référencer PBS par ID ou avoir config embarquée (legacy)
pbsConfig, err := job.GetPBSConfig(globalConfig)
if err != nil {
    log.Fatal(err)
}

// Utiliser pbsConfig pour le backup
client := pbscommon.NewPBSClient(pbsConfig.BaseURL, ...)
```

## 🎨 Utilisation frontend (à implémenter)

### Liste des serveurs PBS

```javascript
// Récupérer tous les serveurs PBS
const servers = await window.go.main.App.ListPBSServers();

// Afficher dans un dropdown
<select name="pbs-server">
  {servers.map(pbs => (
    <option value={pbs.id}>
      {pbs.name} ({pbs.datastore})
    </option>
  ))}
</select>
```

### Ajouter un serveur PBS

```javascript
const newServer = {
  id: "pbs-dr",
  name: "Disaster Recovery",
  baseurl: "https://pbs-dr.example.com:8007",
  authid: "backup@pbs",
  secret: "zzz",
  datastore: "disaster-recovery",
  namespace: "clients",
  description: "Serveur de secours"
};

await window.go.main.App.AddPBSServer(newServer);
```

### Tester connexion PBS

```javascript
try {
  await window.go.main.App.TestPBSConnection("pbs-dr");
  alert("✅ Connexion PBS réussie !");
} catch (err) {
  alert("❌ Erreur: " + err);
}
```

### Créer un job avec PBS spécifique

```javascript
const job = {
  name: "Backup critique",
  pbs_id: "pbs-ssd",  // Serveur SSD rapide
  folders: ["C:\\Users", "C:\\Documents"],
  schedule: "daily",
  schedule_cron: "0 3 * * *"
};

await window.go.main.App.SaveScheduledJob(job);
```

## 🔧 API Methods (exposées au frontend)

| Méthode | Description | Params | Return |
|---------|-------------|--------|--------|
| `ListPBSServers()` | Liste tous les serveurs PBS | - | `[]*PBSServer` |
| `GetPBSServer(id)` | Récupère un serveur par ID | `id: string` | `*PBSServer` |
| `AddPBSServer(pbs)` | Ajoute un nouveau serveur | `pbs: PBSServer` | `error` |
| `UpdatePBSServer(pbs)` | Met à jour un serveur | `pbs: PBSServer` | `error` |
| `DeletePBSServer(id)` | Supprime un serveur | `id: string` | `error` |
| `SetDefaultPBSServer(id)` | Définit le PBS par défaut | `id: string` | `error` |
| `GetDefaultPBSID()` | ID du PBS par défaut | - | `string` |
| `TestPBSConnection(id)` | Teste connexion PBS | `id: string` | `error` |

## 🛡️ Rétrocompatibilité

### Jobs existants (legacy)

Les jobs créés avant Multi-PBS ont une config PBS **embarquée** :

```json
{
  "id": "job_123",
  "name": "Old Job",
  "pbs_config": {  // Config embarquée (deprecated)
    "baseurl": "https://pbs.example.com:8007",
    "authid": "backup@pbs",
    // ...
  }
}
```

**Résolution automatique** :
1. Si `pbs_id` présent → utilise serveur référencé
2. Sinon si `pbs_config.baseurl` présent → utilise config embarquée
3. Sinon → utilise serveur par défaut (`default_pbs_id`)

### Migration des jobs legacy (optionnel)

```go
// Migrer un job vers PBSID
job.MigrateToPBSID("default")
// Efface job.PBSConfig et définit job.PBSID = "default"
```

## 📊 Exemple complet : Use case multi-datastore

```go
// Setup 1: Ajouter 2 serveurs PBS
pbsBigData := &PBSServer{
    ID: "pbs-bigdata",
    Name: "Big Data Storage",
    BaseURL: "https://pbs-bigdata.local:8007",
    Datastore: "bigdata-slow",
}
app.AddPBSServer(pbsBigData)

pbsSSD := &PBSServer{
    ID: "pbs-ssd",
    Name: "SSD Fast",
    BaseURL: "https://pbs-ssd.local:8007",
    Datastore: "ssd-fast",
}
app.AddPBSServer(pbsSSD)

// Setup 2: Créer jobs différents
jobBigData := &Job{
    Name: "Backup gros volumes",
    PBSID: "pbs-bigdata",
    Folders: []string{"C:\\", "D:\\"},
    Schedule: "weekly",
}

jobCritical := &Job{
    Name: "Backup critique quotidien",
    PBSID: "pbs-ssd",
    Folders: []string{"C:\\Users", "C:\\Projects"},
    Schedule: "daily",
}
```

Résultat :
- **C:\\ + D:\\** → PBS big-data (lent, gros espace) → 1x/semaine
- **C:\\Users** → PBS SSD (rapide, petit) → 1x/jour

## ✅ Tests de migration

### Test 1 : Config vierge
```bash
# Aucune config existante
config := LoadConfig()
# → config.PBSServers = {} (vide)
# → config.DefaultPBSID = "" (vide)
```

### Test 2 : Migration depuis legacy
```bash
# Ancienne config avec baseurl="https://pbs.local:8007"
config := LoadConfig()
# → Auto-migration activée
# → config.PBSServers["default"] créé
# → config.DefaultPBSID = "default"
# → Sauvegarde automatique
```

### Test 3 : Config déjà migrée
```bash
# Config avec pbs_servers existant
config := LoadConfig()
# → Pas de migration (pbs_servers déjà présent)
# → Load normal
```

## 🚀 Prochaines étapes (Frontend)

Pour finaliser Multi-PBS, il faut créer l'UI :

1. **Page "Serveurs PBS"**
   - Liste des serveurs avec statut (🟢 Online / 🔴 Offline)
   - Boutons : Ajouter, Modifier, Supprimer, Tester
   - Indicateur serveur par défaut (⭐)

2. **Formulaire job modifié**
   - Dropdown "Serveur PBS" (au lieu de champs manuels)
   - Pré-sélection du serveur par défaut
   - Affichage datastore/namespace du serveur sélectionné

3. **Page Config simplifiée**
   - Si Multi-PBS activé → rediriger vers "Serveurs PBS"
   - Sinon → formulaire legacy (rétrocompat)

---

**Status:** ✅ Backend complet | ⏳ Frontend à développer
**Version:** 0.2.0+
**Mainteneur:** RDEM Systems
**Date:** 2026-03-23
