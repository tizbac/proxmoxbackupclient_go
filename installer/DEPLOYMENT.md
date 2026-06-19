# Nimbus Backup - Silent Installation Guide (Enterprise)

## 📋 Vue d'ensemble

Ce guide explique comment déployer Nimbus Backup en masse via GPO, Intune, ou scripts de déploiement.

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
\\ad-server\NETLOGON\nimbus\config.json
# OU
\\fileserver\Deployments\Nimbus\config.json
```

**Permissions requises:** Lecture pour "Domain Computers"

## 🚀 Déploiement

### Option A - GPO (Group Policy)

1. **Créer une GPO** dans `Computer Configuration > Policies > Software Settings > Software Installation`

2. **Ajouter le package MSI** avec options avancées:
   ```
   Package: \\server\share\NimbusBackup.msi
   Deployment method: Assigned
   ```

3. **Paramètres d'installation**:
   - Dans "Modifications", ajouter une transformation (.mst) OU
   - Utiliser la ligne de commande avancée:
     ```
     CONFIGFILE="\\ad-server\NETLOGON\nimbus\config.json"
     ```

4. **Appliquer la GPO** au bon OU (ex: `Computers/Workstations`)

### Option B - Intune (Microsoft Endpoint Manager)

1. **Créer une application Win32**
   - Fichier source: `NimbusBackup.msi`
   - Format: Win32 app (.intunewin)

2. **Commande d'installation**:
   ```powershell
   msiexec /i NimbusBackup.msi /qn CONFIGFILE="C:\Windows\Temp\nimbus-config.json"
   ```

3. **Script de pré-requis**:
   ```powershell
   # Télécharger config depuis Azure Storage ou créer localement
   $config = Invoke-RestMethod "https://yourblob.blob.core.windows.net/configs/nimbus.json"
   $config | Out-File "C:\Windows\Temp\nimbus-config.json"
   ```

4. **Détection**:
   - Fichier: `C:\Program Files\RDEM Systems\NimbusBackup\bin\nimbus-service.exe`
   - Service: `NimbusService` (Running)

### Option C - Script PowerShell

```powershell
# deploy-nimbus.ps1
$msiPath = "\\server\deploy\NimbusBackup.msi"
$configPath = "\\server\deploy\nimbus-config.json"

# Installation silencieuse avec config
Start-Process msiexec.exe -ArgumentList "/i `"$msiPath`" /qn CONFIGFILE=`"$configPath`"" -Wait

# Vérifier le service
if (Get-Service -Name "NimbusService" -ErrorAction SilentlyContinue) {
    Write-Host "✅ Nimbus Backup installed successfully"
    Start-Service -Name "NimbusService"
} else {
    Write-Error "❌ Installation failed"
    exit 1
}
```

## 🔍 Vérification

### Vérifier l'installation

```powershell
# Service installé et démarré
Get-Service NimbusService

# Config présente
Test-Path "C:\ProgramData\Nimbus\config.json"

# Premier backup dans les logs
Get-Content "C:\ProgramData\Nimbus\logs\service.log" -Tail 50
```

### Logs d'installation MSI

En cas d'échec, générer un log détaillé:
```powershell
msiexec /i NimbusBackup.msi /qn CONFIGFILE="config.json" /l*v install.log
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

Utilisez l'API RDEM pour monitorer l'état des backups:
```powershell
# Script de monitoring (à planifier quotidiennement)
$machines = Get-ADComputer -Filter * -SearchBase "OU=Workstations,DC=domain,DC=com"
foreach ($machine in $machines) {
    # Check last backup via API RDEM
    # Send alert if > 24h
}
```

## 🆘 Dépannage

### Le service ne démarre pas

1. Vérifier les Event Logs:
   ```powershell
   Get-EventLog -LogName Application -Source "NimbusService" -Newest 10
   ```

2. Tester manuellement:
   ```powershell
   cd "C:\Program Files\RDEM Systems\NimbusBackup\bin"
   .\nimbus-service.exe --service
   ```

### Config non prise en compte

```powershell
# Vérifier le contenu
Get-Content "C:\ProgramData\Nimbus\config.json" | ConvertFrom-Json | Format-List
```

## 📞 Support

**RDEM Systems**
- Web: https://nimbus.rdem-systems.com
- Email: support@rdem-systems.com
- Doc: https://nimbus.rdem-systems.com/docs

---

**Version:** 0.2.0
**Dernière mise à jour:** 2026-03-19
