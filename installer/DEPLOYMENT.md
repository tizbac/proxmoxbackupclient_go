# Proxmox Backup Client - Silent Installation Guide (Enterprise)

## 📋 Vue d'ensemble

Ce guide explique comment déployer Proxmox Backup Client en masse via GPO, Intune, ou scripts de déploiement.

## ⚙️ Paramètres du MSI (propriétés publiques)

Toutes les commandes de ce guide sont silencieuses grâce au switch standard de Windows Installer (`/quiet` ou `/qn`) — le MSI n'a pas besoin d'une propriété dédiée pour ça. Il accepte en plus les propriétés publiques suivantes (en MAJUSCULES — obligatoire pour toute propriété MSI passée en ligne de commande) :

| Propriété | Défaut | Effet |
| --- | --- | --- |
| `SERVICEAUTOSTART` | `1` | `0` = service installé en **Manuel** et laissé **arrêté** — installé, mais jamais activé ni autodémarré |
| `KEEP_CONFIG` | `1` | Uniquement à la désinstallation : `0` = supprime aussi le dossier `%ProgramData%\<ExeName>` |
| `INSTALLFOLDER` | `Program Files` | Répertoire d'installation (les chemins contenant des espaces sont entre guillemets) |

```powershell
# Installation silencieuse, réglages par défaut (service : Auto + démarré)
msiexec /i ProxmoxBackupClient.msi /quiet

# Installer le service SANS l'activer ni l'autodémarrer (Manuel + arrêté)
msiexec /i ProxmoxBackupClient.msi /quiet SERVICEAUTOSTART=0

# Répertoire personnalisé + log détaillé
msiexec /i ProxmoxBackupClient.msi /quiet INSTALLFOLDER="D:\Apps\ProxmoxBackupClient" /L*v "C:\Temp\install.log"

# Désinstallation silencieuse qui purge aussi la configuration
msiexec /x ProxmoxBackupClient.msi /quiet KEEP_CONFIG=0
```

Codes de sortie : `0` = succès, `3010` = succès + redémarrage requis. Exécution avec droits administrateur. Les MSI de marque (`NimbusBackup.msi`, `AcmeBackup.msi`, `EtitechBackup.msi`) acceptent exactement les mêmes propriétés.

> ⚠️ Le MSI ne prend **pas** de propriété `CONFIGFILE` : la configuration vit dans `%ProgramData%\<ExeName>` (dossier créé et protégé par le service). Elle se règle via la GUI, l'API locale, ou en déposant un `config.json` à cet emplacement avant le premier démarrage du service.

## 🔧 Préparation

### 1. Créer la configuration centralisée

Copiez `config.example.json` et personnalisez-le pour votre environnement:

```json
{
  "pbs_url": "https://pbs.mycompany.local:8007",
  "auth_id": "windows-backups@pbs",
  "secret": "xxxx-xxxx-xxxx-xxxx",
  "datastore": "windows-clients",
  "namespace": "workstations",
  "backup_id": "",
  "backup_dirs": ["C:\\Users", "C:\\ProgramData\\AppData"],
  "exclusions": ["*.tmp", "*.log", "Temp\\*"],
  "schedule": {
    "enabled": true,
    "time": "02:00",
    "days": ["monday", "wednesday", "friday"]
  },
  "vss_enabled": true
}
```

**Important:**
- `backup_id` vide = utilise le hostname de la machine
- Chemins Windows: utilisez `\\` pour échapper les backslashes

### 2. Placer le fichier config

```powershell
# Sur un partage réseau accessible
\\ad-server\NETLOGON\proxmoxbackupclient\config.json
# OU
\\fileserver\Deployments\ProxmoxBackupClient\config.json
```

**Permissions requises:** Lecture pour "Domain Computers"

## 🚀 Déploiement

### Option A - GPO (Group Policy)

1. **Créer une GPO** dans `Computer Configuration > Policies > Software Settings > Software Installation`

2. **Ajouter le package MSI** avec options avancées:
   ```
   Package: \\server\share\ProxmoxBackupClient.msi
   Deployment method: Assigned
   ```

3. **Paramètres d'installation** (propriétés publiques, voir ci-dessus) :
   - Utiliser la ligne de commande avancée :
     ```
     SERVICEAUTOSTART=0
     ```
     (ajuster selon le comportement d'autodémarrage souhaité)

4. **Appliquer la GPO** au bon OU (ex: `Computers/Workstations`)

### Option B - Intune (Microsoft Endpoint Manager)

1. **Créer une application Win32**
   - Fichier source: `ProxmoxBackupClient.msi`
   - Format: Win32 app (.intunewin)

2. **Commande d'installation**:
   ```powershell
   msiexec /i ProxmoxBackupClient.msi /qn
   ```

3. **Script de pré-requis**:
   ```powershell
   # Télécharger config depuis Azure Storage ou créer localement
   $config = Invoke-RestMethod "https://yourblob.blob.core.windows.net/configs/proxmoxbackupclient.json"
   $config | Out-File "C:\Windows\Temp\proxmoxbackupclient-config.json"
   ```

4. **Détection**:
   - Fichier: `C:\Program Files\ProxmoxBackupClient\ProxmoxBackupClient.exe`
   - Service: `ProxmoxBackupClientSVC` (démarré sauf avec `SERVICEAUTOSTART=0`)

### Option C - Script PowerShell

```powershell
# deploy-proxmoxbackupclient.ps1
$msiPath = "\\server\deploy\ProxmoxBackupClient.msi"

# Installation silencieuse (propriétés publiques, cf. "Paramètres du MSI")
Start-Process msiexec.exe -ArgumentList "/i `"$msiPath`" /qn" -Wait

# Vérifier le service (nom réel : <ExeName>SVC, ex. ProxmoxBackupClientSVC)
if (Get-Service -Name "ProxmoxBackupClientSVC" -ErrorAction SilentlyContinue) {
    Write-Host "✅ Proxmox Backup Client installed successfully"
} else {
    Write-Error "❌ Installation failed"
    exit 1
}
```

## 🔍 Vérification

### Vérifier l'installation

```powershell
# Service installé (et démarré, sauf avec SERVICEAUTOSTART=0)
Get-Service ProxmoxBackupClientSVC

# Config présente
Test-Path "C:\ProgramData\ProxmoxBackupClient\config.json"

# Premier backup dans les logs
Get-Content "C:\ProgramData\ProxmoxBackupClient\debug-service.log" -Tail 50
```

### Logs d'installation MSI

En cas d'échec, générer un log détaillé:
```powershell
msiexec /i ProxmoxBackupClient.msi /qn /l*v install.log
```

Chercher les erreurs:
```powershell
Select-String -Path install.log -Pattern "error|failed" -Context 2,2
```

## 🛡️ Sécurité

### Protection du Token API

Le fichier `config.json` contient le secret PBS. **Bonnes pratiques:**

1. **Permissions NTFS strictes** sur le partage:
   - `Domain Computers`: Lecture seule
   - `Domain Admins`: Contrôle total
   - Refuser: `Domain Users`

2. **Chiffrement au repos** (optionnel):
   ```powershell
   # Chiffrer avec DPAPI (machine-level)
   # À implémenter dans une future version
   ```

3. **Rotation du token**: Prévoir un script pour mettre à jour `config.json` et redémarrer les services.

## 📊 Monitoring

### Dashboard centralisé

Utilisez l'API du service pour monitorer l'état des backups:
```powershell
# Script de monitoring (à planifier quotidiennement)
$machines = Get-ADComputer -Filter * -SearchBase "OU=Workstations,DC=domain,DC=com"
foreach ($machine in $machines) {
    # Check last backup via l'API du service
    # Send alert if > 24h
}
```

## 🆘 Dépannage

### Le service ne démarre pas

1. Vérifier les Event Logs:
   ```powershell
   Get-EventLog -LogName Application -Source "ProxmoxBackupClientService" -Newest 10
   ```

2. Tester manuellement:
   ```powershell
   cd "C:\Program Files\Proxmox Backup Client\bin"
   .\proxmoxbackupclient-service.exe --service
   ```

### Config non prise en compte

```powershell
# Vérifier le contenu
Get-Content "C:\ProgramData\ProxmoxBackupClient\config.json" | ConvertFrom-Json | Format-List
```

## 📞 Support

**Proxmox Backup Client GO contributors**
- Web: https://github.com/tizbac/proxmoxbackupclient_go
- Doc: https://github.com/tizbac/proxmoxbackupclient_go

---

**Version:** 0.2.0
**Dernière mise à jour:** 2026-03-19
