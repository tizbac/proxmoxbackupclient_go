# Instructions de Build - MSI avec Service Windows

## 1. Ajouter la dépendance du service Windows

```bash
cd gui
go get github.com/kardianos/service
go mod tidy
```

## 2. Builder l'application Wails

```bash
cd gui
wails build -clean -platform windows/amd64
```

L'exécutable sera dans `gui/build/bin/NimbusBackup.exe`

## 3. Builder le MSI

### Prérequis
- WiX Toolset installé : https://wixtoolset.org/
- NimbusBackup.exe compilé (étape 2)

### Build
```bash
cd installer/wix
build.bat
```

Le MSI sera créé : `installer/wix/NimbusBackup.msi`

## 4. Tester le MSI

### Installation
1. Double-clic sur `NimbusBackup.msi`
2. Suivre l'assistant d'installation
3. Vérifier que le service est créé :
   ```powershell
   Get-Service NimbusBackup
   ```

### Vérification du service
```powershell
# Status du service
Get-Service NimbusBackup | Select-Object Name, Status, StartType

# Démarrer manuellement (si nécessaire)
Start-Service NimbusBackup

# Voir les logs
Get-EventLog -LogName Application -Source NimbusBackup -Newest 10
```

### Désinstallation
- Via Panneau de configuration → Programmes
- Ou : `msiexec /x NimbusBackup.msi`

## 5. Distribution

Le MSI peut être distribué via :
- GitHub Releases
- Site web RDEM Systems
- Téléchargement direct

## Structure finale

```
NimbusBackup-0.1.44/
├── NimbusBackup.exe     (Standalone - backups manuels)
└── NimbusBackup.msi     (Installateur - service Windows)
```

## Différences exe vs MSI

### NimbusBackup.exe
- ✅ Backups manuels
- ✅ Backups planifiés (quand lancé)
- ❌ Persistance au reboot
- 💡 Usage: Tests, backups ponctuels

### NimbusBackup.msi
- ✅ Service Windows automatique
- ✅ Démarre au boot système
- ✅ Toujours en admin (VSS garanti)
- ✅ Backups planifiés fiables
- 💡 Usage: Production

## Workflow GitHub Actions

Le workflow doit être modifié pour builder le MSI :

```yaml
# Après build-gui job
- name: Install WiX
  run: |
    choco install wixtoolset -y
    $env:PATH += ";C:\Program Files (x86)\WiX Toolset v3.11\bin"

- name: Build MSI
  working-directory: installer/wix
  run: |
    candle.exe Product.wxs -ext WixUIExtension -ext WixUtilExtension
    light.exe Product.wixobj -ext WixUIExtension -ext WixUtilExtension -out NimbusBackup.msi

- name: Upload MSI
  uses: actions/upload-artifact@v4
  with:
    name: NimbusBackup-MSI
    path: installer/wix/NimbusBackup.msi
```

## Troubleshooting

### Le service ne démarre pas
1. Vérifier Event Viewer → Application
2. Vérifier que l'exe supporte `--service` flag
3. Tester manuellement : `NimbusBackup.exe --service`

### Erreur "Service already exists"
Désinstaller l'ancienne version d'abord.

### VSS ne fonctionne toujours pas
Le service doit tourner avec le compte LocalSystem (configuré dans Product.wxs).
