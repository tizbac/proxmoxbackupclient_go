# Nimbus Backup — Status & notes

> Per-version changes are listed in the “Changes since…” section of each release (above) and in [CHANGELOG.md](CHANGELOG.md). This page describes the **stable** state of the product.

## 📦 Available builds

### NimbusBackup.msi (installer — recommended for production)
- ✅ **Windows service**: starts automatically at system boot
- ✅ **Persistent admin privileges**: the service runs as LocalSystem (VSS guaranteed)
- ✅ **Scheduled backups**: run automatically, even after a reboot
- ✅ **Clean uninstall**: full cleanup via Control Panel

### NimbusBackup.exe (standalone)
- ✅ **Manual and scheduled backups**: work as long as the app is running
- ❌ **No persistence across reboots**: no service → prefer the MSI in production
- 💡 **Use case**: one-off backups or testing

## ✅ Features

### Backup & restore
- One-shot (immediate) and scheduled (configurable time) backups
- **Auto-split of large backups** (>100 GB) into balanced jobs (~100 GB) with per-job retry
- **VSS** (Volume Shadow Copy) for consistent backups
- File/folder exclusions + **automatic exclusion of Windows system folders** (System Volume Information, $RECYCLE.BIN, pagefile.sys…)
- Snapshot restore and browsing (**fast catalog reads**)
- Robust long-running backups (**30 s keep-alive**, validated on 11 h+ backups)
- **Multi-server PBS support**

### Interface & configuration
- Wails GUI (Go + React), **in English and French**
- Backup history and re-run of failed jobs
- Progress bar with statistics, minimize to tray
- PBS configuration with connection test, **certificate fingerprint pinning (TOFU)** and namespaces

## 📌 Known issues
- ⚠️ The **standalone .exe** does not persist across reboots → use the **MSI** in production.
- ⚠️ The exclusion format is not validated on input.
