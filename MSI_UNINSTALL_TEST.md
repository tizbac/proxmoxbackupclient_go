# MSI Uninstall - Test de la dialog de configuration

## 🎯 Fonctionnalité

Lors de la désinstallation de Nimbus Backup, l'utilisateur peut choisir :
- ✅ **Conserver la configuration** (par défaut) - garde `C:\ProgramData\NimbusBackup`
- ❌ **Supprimer la configuration** - efface complètement le dossier

## 📁 Fichiers concernés

```
C:\ProgramData\NimbusBackup\
├── config.json              # Configuration PBS, identifiants
├── jobs.json                # Tâches planifiées
├── debug-gui.log            # Logs interface
├── debug-service.log        # Logs service Windows
└── (autres fichiers)
```

## 🔧 Modifications apportées

### Product.wxs

1. **Propriété KEEP_CONFIG**
   ```xml
   <Property Id="KEEP_CONFIG" Value="1" />  <!-- 1 = garder (défaut) -->
   ```

2. **Custom Action de suppression**
   ```xml
   <CustomAction Id="DeleteConfigFolder"
                 ExeCommand='cmd.exe /c "if exist ... rmdir /s /q ..."'
                 Execute="deferred" />
   ```

3. **Dialog personnalisé**
   - Titre : "Désinstallation de [ProductName]"
   - Question : "Souhaitez-vous conserver votre configuration ?"
   - 2 options radio :
     - ⭕ Conserver la configuration (KEEP_CONFIG=1)
     - ⭕ Supprimer la configuration (KEEP_CONFIG=0)

4. **Condition d'exécution**
   ```xml
   (REMOVE="ALL") AND NOT UPGRADINGPRODUCTCODE AND (KEEP_CONFIG="0")
   ```
   - `REMOVE="ALL"` : Vraie désinstallation (pas une mise à jour)
   - `NOT UPGRADINGPRODUCTCODE` : Pas un upgrade MSI
   - `KEEP_CONFIG="0"` : Utilisateur a choisi "Supprimer"

## 🧪 Plan de test

### Test 1 : Désinstallation avec conservation (défaut)

1. Installer Nimbus Backup v0.1.92
2. Configurer un serveur PBS
3. Créer 1-2 jobs planifiés
4. Lancer une désinstallation depuis "Programmes et fonctionnalités"
5. **Vérifier** : Dialog s'affiche avec 2 options
6. **Sélectionner** : ⭕ Conserver la configuration (par défaut)
7. Cliquer "Désinstaller"
8. **Vérifier après désinstallation** :
   - ✅ `C:\Program Files\NimbusBackup` supprimé
   - ✅ `C:\ProgramData\NimbusBackup` **EXISTE ENCORE**
   - ✅ `config.json` et `jobs.json` présents
   - ✅ Service Windows désinstallé
   - ✅ Menu démarrer supprimé

### Test 2 : Désinstallation avec suppression

1. Installer Nimbus Backup v0.1.92
2. Configurer un serveur PBS
3. Créer 1-2 jobs planifiés
4. Lancer une désinstallation depuis "Programmes et fonctionnalités"
5. **Vérifier** : Dialog s'affiche avec 2 options
6. **Sélectionner** : ⭕ Supprimer la configuration
7. Cliquer "Désinstaller"
8. **Vérifier après désinstallation** :
   - ✅ `C:\Program Files\NimbusBackup` supprimé
   - ✅ `C:\ProgramData\NimbusBackup` **SUPPRIMÉ COMPLÈTEMENT**
   - ✅ Aucun fichier config.json ou jobs.json
   - ✅ Service Windows désinstallé
   - ✅ Menu démarrer supprimé

### Test 3 : Upgrade (pas de dialog)

1. Installer Nimbus Backup v0.1.92
2. Configurer un serveur PBS
3. Installer Nimbus Backup v0.1.93 (upgrade)
4. **Vérifier** : Pas de dialog de désinstallation
5. **Vérifier après upgrade** :
   - ✅ `C:\ProgramData\NimbusBackup` **PRÉSERVÉ**
   - ✅ config.json et jobs.json **INTACTS**
   - ✅ Application mise à jour

### Test 4 : Annulation

1. Installer Nimbus Backup v0.1.92
2. Lancer une désinstallation
3. Dialog s'affiche
4. Cliquer "Annuler"
5. **Vérifier** :
   - ✅ Application toujours installée
   - ✅ Service toujours actif
   - ✅ Config intacte

## 📝 Checklist de validation

Avant release :
- [ ] Build MSI avec modifications WiX
- [ ] Test 1 : Désinstall + Conserver config ✅
- [ ] Test 2 : Désinstall + Supprimer config ✅
- [ ] Test 3 : Upgrade sans prompt ✅
- [ ] Test 4 : Annulation ✅
- [ ] Vérifier logs Windows Event Viewer (pas d'erreurs)
- [ ] Vérifier registre Windows (clés supprimées proprement)

## 🔨 Build du MSI

```bash
cd installer/wix

# Build avec WiX Toolset
candle.exe Product.wxs
light.exe -ext WixUIExtension Product.wixobj -out NimbusBackup.msi

# Ou via script automatisé (si existant)
./build-msi.bat
```

## 🐛 Troubleshooting

### Dialog ne s'affiche pas
- **Cause** : Condition `REMOVE="ALL"` pas satisfaite
- **Solution** : Vérifier qu'on désinstalle (pas un upgrade)
- **Log** : Activer logging MSI :
  ```cmd
  msiexec /x NimbusBackup.msi /L*V uninstall.log
  ```

### Config pas supprimée malgré KEEP_CONFIG=0
- **Cause** : CustomAction pas exécutée
- **Solution** : Vérifier séquence `InstallExecuteSequence`
- **Debug** : Chercher "DeleteConfigFolder" dans uninstall.log

### Erreur "Access Denied" sur C:\ProgramData
- **Cause** : CustomAction `Impersonate="yes"` (droits utilisateur)
- **Solution** : Déjà fixé avec `Impersonate="no"` (droits system)

## 📊 Statistiques attendues

Après déploiement :
- **80% utilisateurs** : Gardent config (réinstallation, tests, upgrades)
- **20% utilisateurs** : Supprimeraient tout (désinstallation définitive)

## 🎨 Améliorations futures

1. **Warning visuel** si "Supprimer" sélectionné
   - Texte rouge : "⚠️ Cette action est irréversible"
   - Checkbox confirmation : "Je confirme vouloir supprimer mes données"

2. **Export config avant suppression**
   - Bouton "Exporter config.json avant suppression"
   - Sauvegarde dans `C:\Users\%USERNAME%\Downloads\nimbus-backup-config.json`

3. **Statistiques télémétrie** (opt-in)
   - Tracker % utilisateurs qui gardent vs suppriment
   - Améliorer UX selon données

---

**Status:** ✅ Implémenté | ⏳ Tests à faire
**Version:** 0.2.0+
**Mainteneur:** RDEM Systems
**Date:** 2026-03-23
