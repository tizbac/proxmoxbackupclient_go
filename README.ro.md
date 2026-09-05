# Proxmox Backup Client — Client Windows pentru Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/tizbac/proxmoxbackupclient_go)](https://github.com/tizbac/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-github-orange)](https://github.com/tizbac/proxmoxbackupclient_go)

**Proxmox Backup Client este un client de backup open-source (GPL-3.0) pentru Proxmox Backup Server (PBS), care funcționează pe Windows și Linux.**
Este o **suites de instrumente** pentru backup către PBS:

- **Proxmox Backup Client GUI** (bazată pe GUI-ul Nimbus Backup de la RDEM Systems) — interfață grafică modernă pentru backupul serverelor și stațiilor de lucru Windows către PBS: snapshot-uri coerente VSS, job-uri planificate, moduri fișier și disc, navigare/restaurare snapshot-uri, suport multi-PBS și mod serviciu Windows.
- **`proxmoxbackup-directory`** — instrument de linie de comandă pentru backup de directoare (PXAR) cu deduplicare.
- **`proxmoxbackup-machine`** — instrument de linie de comandă pentru backup complet live al sistemelor Windows (FIDX, VSS, incremental).
- **`proxmoxbackup-nbd`** — server NBD pentru restaurarea backupurilor de disc (Linux).

> Cuvinte cheie: client proxmox backup windows · client PBS · backup Windows VSS · backup offsite imutabil · interfață Proxmox Backup Server.

> ⚠️ **Declinare de responsabilitate:** acest proiect **nu este afiliat în niciun fel** de **Proxmox Server Solutions GmbH**. „Proxmox”, logo-ul Proxmox și denumirile conexe aparțin proprietarilor lor respectivi; aici sunt folosite **doar** pentru a indica compatibilitatea. Vedeți [proxmox.com](https://www.proxmox.com/) pentru produsele lor.

> 🤖 **Această traducere a fost generată cu inteligență artificială și poate conține mici erori. Contribuțiile pentru îmbunătățirea ei sunt binevenite.**

## 📦 Descărcare

👉 **[Descărcați ultima versiune](https://github.com/tizbac/proxmoxbackupclient_go/releases)**

> ⚠️ **Windows afișează „virus detectat” (ex. `Trojan:Win32/Sabsik.FL.A!ml`) sau un avertisment SmartScreen?**
> Este un **rezultat fals pozitiv** cunoscut pentru aplicațiile Go/Wails — *nu* este un virus. Sufixul `!ml` indică o detecție de către un model de machine learning care semnalează executabilele *nesemnate și neobișnuite*.
> Citiți [de ce se întâmplă acest lucru și cum să verificați descărcarea](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Verificarea oricărei descărcări

Fiecare versiune oferă sume de control SHA-256 și o **atestare de proveniență semnată** (dovadă criptografică că binarul a fost produs de CI-ul acestui depozit, dintr-un commit precis):

```powershell
Get-FileHash .\ProxmoxBackupClient.exe -Algorithm SHA256   # comparați cu SHA256SUMS.txt
gh attestation verify .\ProxmoxBackupClient.exe --repo tizbac/proxmoxbackupclient_go
```

**VirusTotal — 0 detecții.** Rapoarte independente cu mai multe motoare ale instalatoarelor MSI recente:
[0.2.108](https://www.virustotal.com/gui/file/6e8fb7ce9af740d470e947addb8daba4331c0b88e8bfdec9e0697ea8f7f29e9e/detection) ·
[0.2.107](https://www.virustotal.com/gui/file/6fd6c6fa77e0305c129ef882a3745100aa6033187a6d52a4af94149ab6b666d2/detection) ·
[0.2.106](https://www.virustotal.com/gui/file/ad6e56700ed9df8e088906e38cee2e2882fc7045f4e39269de0e379a01784ad7/detection)

> ℹ️ **Semnarea codului:** binarele Windows **nu sunt încă semnate Authenticode** (un certificat OSS prin [SignPath Foundation](https://signpath.org) este în așteptare). Între timp, proveniența este stabilită prin atestarea și sumele de control de mai sus.

## 📚 Documentație

- **Ghidul complet de backup Proxmox** — bune practici de implementare PBS
- **Backup Windows cu Proxmox Backup Server** — ghid de implementare specific pentru Windows
- **PBS vs Veeam** — comparație de backup Proxmox

## ✨ Funcționalități

### Proxmox Backup Client GUI (recomandată)
- **🌍 Multilingvă** — interfață în română, engleză și alte limbi
- Configurare prietenoasă cu test de conexiune
- Progresul backupului în timp real cu viteză și timp rămas
- Suport VSS (Volume Shadow Copy) pentru backupuri coerente
- Backup multi-dosare, moduri fișier și disc
- Navigare snapshot-uri, căutare fișiere (jokere) și restaurare
- Suport multi-server PBS cu fixarea amprentei certificatului (TOFU)
- Mod serviciu Windows + backupuri programate
- Jurnalizare de debug pentru diagnosticare

### 📸 Capturi de ecran

![Configurare servere](docs/screenshots/nimbus-gui-liste-servers.png)
*Gestionare multi-server PBS cu indicatoare de stare*

![Formular de adăugare server](docs/screenshots/nimbus-gui-add-server-form.png)
*Configurare simplă a serverului cu test de conexiune*

![Backup imediat](docs/screenshots/nimbus-gui-one-shot-backup.png)
*Progresul backupului în timp real cu ETA și viteză*

### Excluderi inteligente de sistem (mod fișier)
La backupul unui disc întreg (ex. `D:\`), Proxmox Backup Client exclude automat:

**Dosare de sistem:** `System Volume Information` (stocare VSS, poate ajunge la 100+ GB), `$RECYCLE.BIN`, `Recovery`.
**Fișiere de sistem:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys`.

**De ce este important:** un disc poate afișa 1,03 TB utilizați în timp ce fișierele reale sunt ~141 GB. Fără excluderi, backupul ar include snapshot-urile VSS (spațiu și timp irosite); cu ele, dimensiunea corespunde datelor reale.

**Recomandare:** folosiți **modul fișier** (implicit) cu auto-excluderi pentru backupuri la nivel de fișier; folosiți **modul disc** într-un job separat pentru restaurarea bare-metal (include tot).

### Securitate și calitate
- Validarea intrărilor și curățarea acreditărilor
- Prevenirea path traversal (traversarea căilor)
- Logică de reîncercare cu backoff exponențial
- Gestionarea completă a erorilor și teste, conformitate lint 100%

## 🚀 Pornire rapidă

1. Descărcați `ProxmoxBackupClient.exe` (sau `.msi`) din versiuni
2. Rulați-l cu drepturi de administrator (necesare pentru VSS)
3. Configurați conexiunea PBS și testați-o
4. Selectați dosarele de backup
5. Porniți backupul

## 📋 Cerințe

- Windows 10/11 (64 biți)
- Drepturi de administrator (pentru snapshot-uri VSS)
- Acces la rețea la un server Proxmox Backup Server

## 🔨 Compilare din sursă

### Cerințe
- Go 1.22 sau mai nou
- Node.js 20 sau mai nou
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Compilare
```bash
cd gui
npm install --prefix frontend
wails build      # sau: wails dev  (reîncărcare la cald)
```

## 🔧 Utilizare avansată și ghiduri

### Multi-PBS (mai multe servere PBS)

Configurați mai multe servere PBS și alegeți ținta pentru fiecare backup (ex. `C:\Users` → PBS SSD rapid, zilnic; `C:\` → PBS big-data, săptămânal; plus un server DR).

- **[Ghid de utilizator](MULTI_PBS_USER_GUIDE.md)** — adăugare/testare servere, server implicit, FAQ și depanare.
- **[Ghid de implementare](MULTI_PBS_GUIDE.md)** — model de date, migrare automată de la configurarea mono-PBS, metode API backend.

Configurarea mono-PBS existentă este migrată automat la un server `default` la prima încărcare.

### ISO Clonezilla (restaurare bare-metal)

Fluxul de salvare este construit prin patch-ul unui ISO Clonezilla Live cu binarele `pbsnbd` / `machinebackup` și o intrare **pbs-nbd** în meniul principal al Clonezilla (pornire de pe CD, USB prin `dd` și UEFI):

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd
```

Detalii complete (de ce o reconstrucție completă a ISO în loc de înlocuire pe loc, cerințe, fluxul meniului, verificare) în **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**.

### Compilarea GUI Windows

**Docker (recomandat, mai ales la compilarea pe Linux).** Scriptul dintr-o singură comandă produce un `ProxmoxBackupClientGO.exe` cu suport WebView2 adecvat, folosind un container `golang` de unică folosință (instalare mingw + Wails, build frontend, rulare `wails build`):

```bash
./build_gui_windows_docker.sh
```

**Windows nativ (Chocolatey).** Vedeți **[BUILD.md](BUILD.md)** pentru configurarea completă a toolchain-ului Windows:

```powershell
choco install go
choco install mingw
# apoi, într-un shell neelevat:
build.bat          # GUI
build_cli.bat      # CLI
```

### Starea funcționalităților, changelog și documente interne

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)** — matrice de stare pe funcționalitate (implementat / testat / foaie de parcurs).
- **[CHANGELOG.md](CHANGELOG.md)** — istoricul modificărilor pe versiune.
- **[TODO.md](TODO.md)** — foaia de parcurs deschisă și idei.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)** — stare stabilă a produsului și versiuni disponibile.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)** — dialogul de dezinstalare MSI (păstrare/ștergere configurație) și planul său de testare.
- **[FIXES_SUMMARY.md](FIXES_SUMMARY.md)** — note de fixuri GUI (comutare mod director vs. mașină).

## 🖥️ Atribuirea interfeței grafice

**Proxmox Backup Client GUI** se bazează pe **[GUI Nimbus Backup](https://nimbus.rdem-systems.com)**, dezvoltată și întreținută de **[RDEM Systems](https://www.rdem-systems.com/)**.

GUI (inițial un fork al acestui proiect) a fost integrată înapoi în acest depozit: întregul cod, inclusiv GUI și toate funcționalitățile sale, rămâne open-source sub licența GPLv3. RDEM Systems sponsorizează dezvoltarea GUI și oferă suport comercial pentru ea.

**Autorul CLI-ului original:** Tiziano Bacocco (tizbac) · **Licență:** GPLv3

## ⚠️ Avertisment

Acest software este furnizat „ca atare”. Deși urmărim fiabilitatea, declinăm orice responsabilitate pentru pierderea sau deteriorarea datelor. Testați întotdeauna backupurile și verificați restaurarea înainte de a vă baza pe ele în producție.

## 📄 Licență

GPLv3 — vedeți fișierul [LICENSE](LICENSE).

## 🏷️ Branding

Fiecare contribuitor care a contribuit cu **cel puțin 5 commit-uri** care adaugă funcționalități sau rezolvări, are dreptul de a-și adăuga datele de branding pentru uz comercial.

Singurele condiții sunt că firma către care indică brandingul **nu** desfășoară niciuna dintre următoarele activități:

- Campanii de malware
- Firme care promovează războiul (acest lucru se aplică oricărei țări, inclusiv țărilor occidentale)
- Escrocherii
- Furt de date
- Trafic de persoane/copii
- Violență
- Discriminare
- Droguri
- Orice activitate în general recunoscută ca ilegală

Dacă apare vreo plângere împotriva oricărui contribuitor, vom încerca să luăm legătura; dacă nu se oferă o explicație valabilă, vom **înceta imediat** acest beneficiu.

**Licența GPLv3 rămâne activă**, și veți fi în continuare liberi să fork-uiți proiectul și să vă compilați propriile executabile.

## Despre contribuitorii Proxmox Backup Client GO

Contribuitorii Proxmox Backup Client GO dezvoltă și întrețin acest proiect. Software-ul se bazează pe infrastructura NTP/NTS și pe [11 servere NTS publice](https://github.com/jauderho/nts-servers) enumerate în referința comunității.

## 🤝 Contribuie

GUI este acum complet implementată, dar contribuțiile sunt în continuare binevenite, în special:

1. Suport pentru criptare (încă lipsește)
2. Migrare fizic-către-virtual (P2V), restaurarea unui backup bare-metal într-o mașină virtuală (încă incompletă)
3. Încărcare asincronă / încărcare multicore a chunk-urilor (compresia multicore este deja implementată pentru machine backup)
4. Patch de partea Proxmox pentru a adăuga un alt tip de intrare în formatul pxar cu descriptori de securitate Windows
5. Suport pentru link-uri simbolice Windows
6. Orice interesant vă vine în minte :)

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**