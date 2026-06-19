# Nimbus Backup - Installateur MSI

Ce dossier contient les fichiers nécessaires pour créer l'installateur MSI de Nimbus Backup avec support du service Windows.

## Prérequis

- **WiX Toolset 3.x ou 4.x** : https://wixtoolset.org/
- **Visual Studio Build Tools** (optionnel mais recommandé)
- **NimbusBackup.exe compilé** dans `gui/build/bin/`

## Installation WiX Toolset

### Option 1: Via winget (Windows 11/10)
```powershell
winget install --id WiXToolset.WiX
```

### Option 2: Téléchargement direct
1. Télécharger depuis https://github.com/wixtoolset/wix3/releases
2. Installer le fichier `.exe`
3. Ajouter `C:\Program Files (x86)\WiX Toolset v3.x\bin` au PATH

## Structure du MSI

```
NimbusBackup.msi
├── Service Windows "NimbusBackup"
│   ├── Démarre automatiquement au boot
│   ├── Tourne avec privilèges LocalSystem (admin)
│   ├── Exécute les backups planifiés
│   └── Support VSS garanti
├── Raccourci Menu Démarrer
└── Désinstallation propre
```

## Build du MSI

### Méthode 1: Script automatique
```batch
cd installer/wix
build.bat
```

### Méthode 2: Manuel
```batch
cd installer/wix

# Compilation
candle.exe Product.wxs -ext WixUIExtension -ext WixUtilExtension

# Création du MSI
light.exe Product.wixobj -ext WixUIExtension -ext WixUtilExtension -out NimbusBackup.msi
```

## Fichiers générés

- `NimbusBackup.msi` : Installateur final (à distribuer)
- `*.wixobj` : Fichiers objets intermédiaires (à ignorer)
- `*.wixpdb` : Symboles de debug (à ignorer)

## Configuration du service

Le service Windows créé par le MSI :
- **Nom** : NimbusBackup
- **Nom d'affichage** : Nimbus Backup Service
- **Compte** : LocalSystem (privilèges admin)
- **Démarrage** : Automatique
- **Argument** : `--service` (mode service)

## Personnalisation

### Changer la version
Modifier dans `Product.wxs` :
```xml
<?define ProductVersion = "0.1.44" ?>
```

### Changer l'UUID
L'`UpgradeCode` doit rester constant entre versions pour permettre les mises à jour :
```xml
<?define UpgradeCode = "12345678-1234-1234-1234-123456789012" ?>
```

### Ajouter des fichiers
```xml
<File Id="Config" Source="config.json" />
```

## Mise à jour du code pour le service

Le fichier `gui/main.go` doit supporter le flag `--service` :

```go
func main() {
    isService := false
    for _, arg := range os.Args {
        if arg == "--service" {
            isService = true
            break
        }
    }

    if isService {
        // Mode service Windows
        runAsService()
    } else {
        // Mode GUI normal
        runGUI()
    }
}
```

## Test du MSI

1. **Build** : `build.bat`
2. **Installation** : Double-clic sur `NimbusBackup.msi` (nécessite admin)
3. **Vérification service** :
   ```powershell
   Get-Service NimbusBackup
   ```
4. **Logs** : Vérifier dans Event Viewer → Application
5. **Désinstallation** : Panneau de configuration → Programmes

## Distribution

### GitHub Release
Le workflow GitHub doit être modifié pour inclure le MSI :

```yaml
- name: Build MSI
  working-directory: installer/wix
  run: build.bat

- name: Upload MSI
  uses: actions/upload-artifact@v4
  with:
    name: NimbusBackup-MSI
    path: installer/wix/NimbusBackup.msi
```

## Troubleshooting

### Erreur "candle.exe not found"
→ WiX Toolset n'est pas dans le PATH. Réinstaller ou ajouter manuellement.

### Erreur "Access denied" lors de l'installation
→ Exécuter l'installateur en tant qu'administrateur.

### Le service ne démarre pas
→ Vérifier que l'exe supporte le flag `--service`
→ Consulter Event Viewer pour les erreurs

### Conflit de version
→ Désinstaller l'ancienne version d'abord
→ Vérifier que UpgradeCode est correct

## Ressources

- **WiX Documentation** : https://wixtoolset.org/docs/
- **Service Windows en Go** : https://github.com/kardianos/service
- **MSI Best Practices** : https://learn.microsoft.com/en-us/windows/win32/msi/
