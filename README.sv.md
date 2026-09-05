# Proxmox Backup Client — Windows-klient för Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/tizbac/proxmoxbackupclient_go)](https://github.com/tizbac/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-github-orange)](https://github.com/tizbac/proxmoxbackupclient_go)

**Proxmox Backup Client är en öppen källkods-baserad (GPL-3.0) backup-klient för Proxmox Backup Server (PBS) som fungerar på Windows och Linux.**
Det är en **svit av verktyg** för att säkerhetskopiera till PBS:

- **Proxmox Backup Client GUI** (baserat på Nimbus Backup GUI från RDEM Systems) — modernt grafiskt gränssnitt för att säkerhetskopiera Windows-servrar och -arbetsstationer till PBS: konsekventa VSS-ögonblicksbilder, schemalagda jobb, fil- och disklägen, bläddring/återställning av ögonblicksbilder, multi-PBS-stöd och Windows-tjänstläge.
- **`proxmoxbackup-directory`** — kommandoradsverktyg för katalogbackup (PXAR) med deduplicering.
- **`proxmoxbackup-machine`** — kommandoradsverktyg för fullständiga livebackup av Windows-system (FIDX, VSS, inkrementell).
- **`proxmoxbackup-nbd`** — NBD-server för återställning av diskbackup (Linux).

> Nyckelord: proxmox backup client windows · PBS-klient · Windows VSS-backup · oföränderliga fjärrsäkerhetskopior · Proxmox Backup Server-gränssnitt.

> ⚠️ **Ansvarsfriskrivning:** Detta projekt är **på inga sätt associerat med** **Proxmox Server Solutions GmbH**. ”Proxmox”, Proxmox-logotypen och relaterade namn är egendom tillhörande respektive ägare; här används de **enbart** för att ange kompatibilitet. Se [proxmox.com](https://www.proxmox.com/) för deras produkter.

> 🤖 **Denna översättning genererades med AI och kan innehålla små fel. Bidrag för att förbättra den är välkomna.**

## 📦 Nedladdning

👉 **[Ladda ner senaste versionen](https://github.com/tizbac/proxmoxbackupclient_go/releases)**

> ⚠️ **Visar Windows ”virus upptäcktes” (t.ex. `Trojan:Win32/Sabsik.FL.A!ml`) eller en SmartScreen-varning?**
> Detta är ett **känt falskt positivt resultat** för Go/Wails-applikationer — det är *inte* ett virus. Ändelsen `!ml` indikerar en detektering av en maskininlärningsmodell som flaggar *osignerade och ovanliga* körbara filer.
> Se [varför detta händer och hur du verifierar nedladdningen](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Verifiera valfri nedladdning

Varje utgåva tillhandahåller SHA-256-kontrollsummor och en **signerad härkomstattestering** (kryptografisk bevisning att binären producerades av detta arkivs CI, från en exakt commit):

```powershell
Get-FileHash .\ProxmoxBackupClient.exe -Algorithm SHA256   # jämför med SHA256SUMS.txt
gh attestation verify .\ProxmoxBackupClient.exe --repo tizbac/proxmoxbackupclient_go
```

**VirusTotal — 0 detekteringar.** Oberoende rapporter från flera motorer av de senaste MSI-installatörerna:
[0.2.108](https://www.virustotal.com/gui/file/6e8fb7ce9af740d470e947addb8daba4331c0b88e8bfdec9e0697ea8f7f29e9e/detection) ·
[0.2.107](https://www.virustotal.com/gui/file/6fd6c6fa77e0305c129ef882a3745100aa6033187a6d52a4af94149ab6b666d2/detection) ·
[0.2.106](https://www.virustotal.com/gui/file/ad6e56700ed9df8e088906e38cee2e2882fc7045f4e39269de0e379a01784ad7/detection)

> ℹ️ **Kodsignering:** Windows-binärer är **ännu inte Authenticode-signerade** (ett OSS-certifikat via [SignPath Foundation](https://signpath.org) väntas). Under tiden fastställs härkomsten genom attesteringen och kontrollsummorna ovan.

## 📚 Dokumentation

- **Komplett Proxmox-backupguide** — bästa praxis för PBS-distribution
- **Säkerhetskopiera Windows med Proxmox Backup Server** — distributionsguide specifik för Windows
- **PBS vs Veeam** — jämförelse av Proxmox-backup

## ✨ Funktioner

### Proxmox Backup Client GUI (rekommenderad)
- **🌍 Flerspråkig** — gränssnitt på svenska, engelska och fler språk
- Användarvänlig konfiguration med anslutningstest
- Realtidsprogression med genomströmning och återstående tid
- VSS-stöd (Volume Shadow Copy) för konsekventa backup
- Backup av flera mappar, fil- och disklägen
- Bläddring av ögonblicksbilder, filsökning (jokertecken) och återställning
- Stöd för flera PBS-servrar med fästning av certifikatets fingeravtryck (TOFU)
- Windows-tjänstläge + schemalagda backup
- Felsökningsloggning för diagnostik

### 📸 Skärmbilder

![Serverkonfiguration](docs/screenshots/nimbus-gui-liste-servers.png)
*Hantering av flera PBS-servrar med statusindikatorer*

![Formulär för att lägga till server](docs/screenshots/nimbus-gui-add-server-form.png)
*Enkel serverkonfiguration med anslutningstest*

![Engångsbackup](docs/screenshots/nimbus-gui-one-shot-backup.png)
*Backup-progression i realtid med ETA och genomströmning*

### Smarta systemundantag (filläge)
Vid backup av en hel enhet (t.ex. `D:\`) exkluderar Proxmox Backup Client automatiskt:

**Systemmappar:** `System Volume Information` (VSS-lagring, kan nå 100+ GB), `$RECYCLE.BIN`, `Recovery`.
**Systemfiler:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys`.

**Varför det spelar roll:** en enhet kan visa 1,03 TB använt medan de faktiska filerna är ~141 GB. Utan undantagen skulle backupen inkludera VSS-ögonblicksbilderna (slösat utrymme och tid); med dem matchar storleken de verkliga uppgifterna.

**Rekommendation:** använd **filläget** (standard) med auto-undantag för filnivåbackup; använd **diskläget** i ett separat jobb för bare-metal-återställning (inkluderar allt).

### Säkerhet och kvalitet
- Validering av inmatning och rensning av referenser
- Skydd mot sökvägsresning (path traversal)
- Återförsökslogik med exponentiell backoff
- Omfattande felhantering och tester, 100 % lint-efterlevnad

## 🚀 Snabbstart

1. Ladda ner `ProxmoxBackupClient.exe` (eller `.msi`) från utgåvorna
2. Starta det med administratörsrättigheter (krävs för VSS)
3. Konfigurera din PBS-anslutning och testa den
4. Välj vilka mappar som ska säkerhetskopieras
5. Starta backupen

## 📋 Förutsättningar

- Windows 10/11 (64-bitars)
- Administratörsrättigheter (för VSS-ögonblicksbilder)
- Nätverksåtkomst till en Proxmox Backup Server

## 🔨 Kompilera från källkod

### Förutsättningar
- Go 1.22 eller senare
- Node.js 20 eller senare
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Bygge
```bash
cd gui
npm install --prefix frontend
wails build      # eller: wails dev  (hot reload)
```

## 🔧 Avancerad användning och guider

### Multi-PBS (flera PBS-servrar)

Konfigurera flera PBS-servrar och välj mål för varje backup (t.ex. `C:\Users` → snabb SSD-PBS, dagligen; `C:\` → stordata-PBS, veckovis; plus en DR-server).

- **[Användarguide](MULTI_PBS_USER_GUIDE.md)** — lägga till/testa servrar, standardserver, FAQ och felsökning.
- **[Implementeringsguide](MULTI_PBS_GUIDE.md)** — datamodell, automatisk migrering från enkel-PBS-konfiguration, backend-API-metoder.

Befintlig enkel-PBS-konfiguration migreras automatiskt till en `default`-server vid första inläsningen.

### Clonezilla-ISO (bare-metal-återställning)

Räddningsarbetsflödet byggs genom att patch:a en Clonezilla Live-ISO med `pbsnbd` / `machinebackup`-binärerna och en **pbs-nbd**-post i Clonezillas huvudmeny (start från CD, USB via `dd` och UEFI):

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd
```

Fullständiga detaljer (varför en komplet ombyggnad av ISO istället för ett byte på plats, förutsättningar, menyflöde, verifiering) i **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**.

### Bygga Windows-GUI

**Docker (rekommenderas, särskilt vid bygge på Linux).** Skriptet med ett kommando producerar en `ProxmoxBackupClientGO.exe` med korrekt WebView2-stöd, via en engångs-`golang`-container (installation av mingw + Wails, bygge av frontend, körning av `wails build`):

```bash
./build_gui_windows_docker.sh
```

**Nativt Windows (Chocolatey).** Se **[BUILD.md](BUILD.md)** för den kompletta Windows-verktygskedjan:

```powershell
choco install go
choco install mingw
# sedan, i ett shell utan förhöjda rättigheter:
build.bat          # GUI
build_cli.bat      # CLI
```

### Funktionsstatus, ändringslogg och interna dokument

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)** — statusmatris per funktion (implementerad / testad / färdplan).
- **[CHANGELOG.md](CHANGELOG.md)** — ändringshistorik per version.
- **[TODO.md](TODO.md)** — öppen färdplan och idéer.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)** — stabil produktstatus och tillgängliga byggen.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)** — MSI-avinstallationsdialog (behåll/ta bort konfiguration) och dess testplan.
- **[FIXES_SUMMARY.md](FIXES_SUMMARY.md)** — GUI-fixanteckningar (växling katalog- vs maskinläge).

## 🖥️ Tillskrivning av grafiskt gränssnitt

**Proxmox Backup Client GUI** är baserad på **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)**, utvecklad och underhållen av **[RDEM Systems](https://www.rdem-systems.com/)**.

GUI:n (från början en fork av detta projekt) har slagits samman tillbaka i detta arkiv: hela kodbasen, inklusive GUI:n och alla dess funktioner, förblir öppen källkod under GPLv3-licensen. RDEM Systems sponsrar GUI-utvecklingen och tillhandahåller kommersiellt stöd för den.

**Författare till original-CLI:** Tiziano Bacocco (tizbac) · **Licens:** GPLv3

## ⚠️ Varning

Denna programvara tillhandahålls ”i befintligt skick”. Även om vi strävar efter tillförlitlighet, frånsäger vi oss allt ansvar för förlust av eller skada på data. Testa alltid dina backup och verifiera återställning innan du förlitar dig på dem i produktion.

## 📄 Licens

GPLv3 — se filen [LICENSE](LICENSE).

## 🏷️ Branding

Varje bidragsgivare som har bidragit med **minst 5 commits** som lägger till funktionalitet eller fixar, har rätt att få sina branding-uppgifter tillagda för kommersiellt bruk.

De enda villkoren är att företaget som branding pekar på **inte** bedriver någon av följande verksamheter:

- Malware-kampanjer
- Företag som främjar krig (detta gäller alla länder, inklusive västerländska)
- Bedrägerier
- Datastöld
- Människohandel/barnhandel
- Våld
- Diskriminering
- Droger
- All verksamhet som i allmänhet anses olaglig

Om något klagomål dyker upp mot någon av bidragsgivarna, kommer vi att försöka komma i kontakt; om ingen giltig förklaring ges, **avslutar vi omedelbart** denna förmån.

**GPLv3-licensen förblir aktiv**, och du kommer fortfarande att vara fri att forka projektet och bygga dina egna körbara filer.

## Om bidragsgivarna till Proxmox Backup Client GO

Bidragsgivarna till Proxmox Backup Client GO utvecklar och underhåller detta projekt. Programvaran förlitar sig på NTP/NTS-infrastrukturen och de [11 offentliga NTS-servrarna](https://github.com/jauderho/nts-servers) som anges i community-referensen.

## 🤝 Bidra

GUI:n är nu fullt implementerad, men bidrag är fortfarande välkomna, särskilt:

1. Krypteringsstöd (saknas fortfarande)
2. Fysisk-till-virtuell (P2V)-migrering, återställning av en bare-metal-backup till en virtuell maskin (fortfarande ofullständig)
3. Asynkron uppladdning / multicore-uppladdning av chunks (multicore-komprimering är redan implementerad för machine backup)
4. Patch på Proxmox-sidan för att lägga till en annan typ av post i pxar-formatet med Windows-säkerhetsbeskrivningar
5. Stöd för Windows-symlänkar
6. Allt intressant du kommer på :)

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**