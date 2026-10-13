# Proxmox Backup Client: Windows- und Linux-Client für Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client ist ein quelloffener (GPL-3.0) Backup-Client für Proxmox Backup Server (PBS), für Windows und Linux.** Es ist eine **Sammlung von Werkzeugen**:

- **Proxmox Backup Client GUI** (basiert auf der Nimbus Backup GUI von RDEM Systems): grafischer Client für Windows und Linux, mit konsistenten Snapshots (VSS unter Windows, elastio-snap/dattobd unter Linux), geplanten Sicherungssätzen, Ordner- und Ganzdatenträgersicherungen, Wiederherstellung, „Gelöschte Dateien“ (Undelete) und „Zurücksetzen“ (Roll back), mehreren PBS-Servern und einem Hintergrunddienst.
- **`proxmoxbackup-directory`**: Ordnersicherungen (pxar) per Kommandozeile, mit Deduplizierung und Änderungserkennung.
- **`proxmoxbackup-machine`**: Live-Sicherungen ganzer Datenträger (fidx) von Windows- (VSS) und Linux-Rechnern (elastio-snap/dattobd); das Abbild ist nach einer Bare-Metal-Wiederherstellung bootfähig.
- **`proxmoxbackup-nbd`**: bindet eine Datenträgersicherung zur Wiederherstellung als NBD-Gerät ein (Linux).
- Eine **ISO für Bare-Metal-Wiederherstellung** auf Basis von Clonezilla Live.

> ⚠️ **Hinweis:** Dieses Projekt ist **in keiner Weise verbunden** mit der **Proxmox Server Solutions GmbH**. „Proxmox“, das Proxmox-Logo und zugehörige Namen gehören ihren jeweiligen Inhabern; sie dienen hier **nur** zur Angabe der Kompatibilität. Siehe [proxmox.com](https://www.proxmox.com/).

> ℹ️ **Diese Seite ist eine übersetzte Zusammenfassung.** Maßgeblich ist die [englische README.md](README.md) mit allen Details. Die Handbücher sind auf Englisch.

## 📖 Handbücher

| Handbuch | Inhalt |
|---|---|
| **[GUI-Handbuch](docs/manual/GUI.md)** | Installation, PBS-Server, Sicherungssätze, inkrementelle Sicherungen, Wiederherstellung, Gelöschte Dateien, Zurücksetzen, Berichte, Einstellungen, Dienst, Fehlerbehebung |
| **[Kommandozeilen-Handbuch](docs/manual/CLI.md)** | Die drei Werkzeuge: Optionen, Konfigurationsdateien, Exit-Codes, Zeitplanung; Gelöschte Dateien und Zurücksetzen per Kommandozeile |
| **[Handbuch Bare-Metal-Wiederherstellung](docs/manual/BMR.md)** | Die Wiederherstellungs-ISO Schritt für Schritt; `vm`-Snapshots auf Proxmox VE; abweichende Hardware |

Siehe auch die [Wiederherstellungsszenarien](docs/RESTORE_GUIDE.md) und die [Übersicht der Handbücher](docs/manual/README.md).

## 🆕 Auf master, für das nächste Release

Im [CHANGELOG](CHANGELOG.md) unter [Unreleased], noch nicht veröffentlicht:

- **Installer**: **Windows-MSI** (App und Dienst unter Program Files, Startmenü, Aktualisierung an Ort und Stelle, Einstellungen beim Entfernen behalten oder löschen) und **Linux-Pakete** `.deb` (Debian, Ubuntu, Mint) und `.rpm` (Fedora).
- **Hintergrunddienst unter Linux** (`proxmoxbackupclient.service`): Geplante Sätze laufen als root, auch ohne angemeldeten Benutzer (Einstellungen > Erweitert > Als Dienst ausführen). **Als Administrator neu starten** funktioniert auch unter Linux (über pkexec).
- **Pause, Fortsetzen und Stoppen** für Sicherungen, die der Dienst ausführt.
- Warnung und Kennzeichen **Geteilte ID**, wenn zwei Sätze eine Sicherungsgruppe teilen (gleicher Server, Typ und Sicherungs-ID).
- **Einstellungen abgesichert**: `C:\ProgramData\ProxmoxBackupClient` nur für Administratoren unter Windows; Ordner nur für den Eigentümer unter Linux.
- **Dateien aus Maschinensicherungen** auch von FAT, exFAT, ext2/3/4 und XFS neben NTFS; Windows-Abbilder stellen auch Berechtigungen und alternative Datenströme wieder her.
- Linux-Maschinensicherungen überspringen freien Speicher (ext2/3/4, XFS).
- Gelöschte Dateien und Zurücksetzen für Linux-Maschinen-Sätze, und **per Kommandozeile**: `-list-sets`, `-undelete`, `-rollback`.
- Die Kommandozeilenwerkzeuge geben Fehler auf stderr aus, und `proxmoxbackup-machine` verschickt seine Benachrichtigungs-E-Mail.

## 💽 Neu in v0.8.0

- **Windows-Maschinensicherungen überspringen freien Speicher** (und Auslagerungs- und Ruhezustandsdateien): Eine halb leere Platte ist in etwa der halben Zeit gelesen. Ein **defekter Sektor** stoppt die Sicherung nicht mehr (er wird als Nullen geschrieben und gemeldet).
- **Dateien aus einer Maschinensicherung wiederherstellen**: Jedes NTFS-Volume im Abbild ist ein Ordner zum Durchsuchen und Ankreuzen, ohne Einbinden oder Herunterladen (127 GB mit 430.000 Dateien in 4 bis 15 Sekunden aufgelistet). Eine Maschinensicherung deckt so die Bare-Metal-Wiederherstellung und die verlorene Datei ab.
- **Gelöschte Dateien und Zurücksetzen** für Maschinen-Sätze (Windows), für die gewählten Ordner (standardmäßig Desktop, Documents, Pictures, Music, Videos und Downloads).
- **Pause und Fortsetzen**, Kennzeichen **VSS / Snapshot ON-OFF**, Windows-Erstellungszeiten gesichert und wiederhergestellt.
- **v0.8.2:** Leerer Speicher wird nicht mehr Block für Block gehasht (50-GB-Testplatte: vorher 3m36s, jetzt 1m15s).

## 🗑️ Neu in v0.7.0: Gelöschte Dateien und Zurücksetzen

- **Gelöschte Dateien** findet Dateien, die in den Sicherungen eines Satzes, aber nicht mehr auf dem Datenträger sind (neueste Sicherung oder letzte 7 bzw. 30 Tage), zeigt sie als Baum mit dem Zeitpunkt ihres Verschwindens und stellt die angekreuzten wieder her. Nach dem Löschen von 39 GB aus einer Freigabe mit 761.000 Dateien fand die 7-Tage-Suche sie in etwa einer Minute.
- **Zurücksetzen** bringt die Ordner eines Satzes nach einer Vorschau auf den Stand einer gewählten Sicherung. Jede ersetzte oder entfernte Datei wird vorher auf demselben Laufwerk beiseitegelegt, sodass **Rückgängig** alles zurückholt.

Außerdem: Bestätigungsdialoge, nächster Lauf auf jeder Karte, vorgeschlagene Sicherungs-ID, Dienst im Windows-Download enthalten. Siehe [GUI-Handbuch](docs/manual/GUI.md#10-undelete).

## ⚡ Neu in v0.6.0: inkrementelle Ordnersicherungen

Mit derselben **Änderungserkennung** wie der offizielle `proxmox-backup-client` vergleicht der Modus **Metadaten** Größe, Änderungszeit (auf die Nanosekunde), Modus und Eigentümer jeder Datei mit dem vorherigen Snapshot; unveränderte Dateien werden nicht geöffnet, ihre Chunks werden wiederverwendet. Auf einem Server mit 480,9 GB und 761.866 Dateien dauert der erste Lauf 4 h 17 min, die folgenden etwa 10 bis 15 Minuten (4 bis 40 neue Chunks). Der Verkehr zu einem entfernten PBS sinkt entsprechend. Siehe [GUI-Handbuch](docs/manual/GUI.md#7-incremental-folder-backups-change-detection).

## 🤝 Kompatibel mit dem offiziellen Proxmox-Client

Geprüft mit `proxmox-backup-client` 3.4.9 und PBS 4.2, in beide Richtungen:

- Legacy-, Data- und Metadata-Snapshots dieses Clients (Windows oder Linux) lassen sich mit dem offiziellen Client **byte-identisch** wiederherstellen, und der Dateibrowser von PBS 4.2 öffnet sie; Snapshots des offiziellen Clients lassen sich mit diesem Client byte-identisch wiederherstellen.
- Gleiches geteiltes Archivformat (pxar Version 2), gleiche Erkennungsmodi, gleiches Format der Schlüsseldatei (`proxmox-backup-client key create`), gleiche Umgebungsvariablen wie `PBS_REPOSITORY`.

Ihre Sicherungen sind also nie an diesen Client gebunden. Einziger Zusatz: eine kleine Datei `.proxmox_backup_client_meta.json` im Wurzelverzeichnis von Ordner-Snapshots, die der offizielle Client als gewöhnliche Datei wiederherstellt.

## 📦 Download und Installation

👉 **[Neuestes Release herunterladen](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| Sie haben | Verwenden Sie |
|---|---|
| Windows | Den **MSI-Installer** (`ProxmoxBackupClient-<version>-x64.msi`) oder das zip `ProxmoxBackupClient-v<version>-windows-amd64.zip` (App und `ProxmoxBackupClientSVC.exe` im selben Ordner) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Anderes Linux | `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (benötigt die WebKitGTK-Laufzeit) |
| Nur Kommandozeile | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` oder `-linux-amd64.tar.gz` |

MSI, `.deb` und `.rpm` sind neu auf master (bis v0.8.1 nur zip und tar.gz); bis dahin siehe [BUILD.md](BUILD.md). Unter Linux startet der Menüeintrag **Proxmox Backup Client (administrator)** die App über polkit als root, für Maschinensicherungen; der Dienst wird installiert, aber nicht eingeschaltet ([packaging/debian/README.md](packaging/debian/README.md)).

**Voraussetzungen:** Windows 10/11 oder Windows Server, 64 Bit (Administratorrechte für VSS und Maschinensicherungen), oder Linux mit WebKitGTK (dazu root und elastio-snap oder dattobd für Maschinensicherungen); Netzwerkzugriff auf einen Proxmox Backup Server.

> ⚠️ **Windows meldet einen Virus (`Trojan:Win32/Sabsik.FL.A!ml`) oder SmartScreen warnt?** Das ist ein **bekannter Fehlalarm** bei unsignierten Go/Wails-Anwendungen. Details beim [Ursprungsprojekt](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Einen Download prüfen

Jedes Release dieses Forks enthält eine `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-v0.8.1-windows-amd64.zip -Algorithm SHA256   # compare with SHA256SUMS.txt
```

Unter Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`. Codesignatur und CI-Attestierung gibt es noch nicht (siehe [TODO.md](TODO.md)).

## ✨ Funktionen

### GUI (empfohlen)
- **18 Oberflächensprachen**, darunter Deutsch, Französisch und Italienisch
- Seitenmenü: Sicherung, Wiederherstellung, Gelöschte Dateien, Zurücksetzen, Berichte, Meldungsprotokoll, Einstellungen, Über
- **Sicherungssätze** mit Zeitplan (täglich, Intervall, manuell), „Jetzt ausführen“, Typ- und VSS / Snapshot-Kennzeichen; **einmalige Sicherungen**
- **Inkrementelle Ordnersicherungen** (Metadaten) mit vollständigem Neulesen alle N Läufe
- **Maschinensicherungen** unter Windows und Linux, als `host` oder als Proxmox-VE-`vm`-Snapshot
- Selektive **Wiederherstellung** mit Suche, NTFS/POSIX-ACLs, Zeitstempeln und optionaler Prüfung; Dateien aus Maschinensicherungen
- **Pause, Fortsetzen, Stoppen**; **Als Dienst ausführen** (Einstellungen > Erweitert)
- Berichte, E-Mail-Benachrichtigungen, Aktionen nach der Sicherung, Export und Import der Einstellungen, fünf Farbschemata
- Mehrere PBS-Server, Zertifikat-Pinning, API-Token oder Benutzer/Passwort; **clientseitige Verschlüsselung**

### Kommandozeilenwerkzeuge
- `proxmoxbackup-directory`: Ordner (pxar), `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: ganze Datenträger (fidx, inkrementell, freier Speicher übersprungen außer mit `-readfreespace`)
- `proxmoxbackup-nbd`: NBD-Server zum Wiederherstellen von Datenträgersicherungen (Linux)
- `ProxmoxBackupClientSVC` (Windows) und `proxmoxbackupclient` (Linux): `-list-sets`, `-undelete`, `-rollback` (auf master)

### 📸 Bildschirmfotos

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Sicherungssätze**: Kennzeichen und „Jetzt ausführen“

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Live-Fortschritt**: Durchsatz, Restzeit, Chunks

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Wiederherstellung**: Snapshots auflisten oder nach Dateien durchsuchen

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Sätze als Proxmox-VE-VMs**: Kennzeichen `MACHINE (VM)`

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Berichte**: vollständiger Verlauf mit Details

</td>
<td width="33%">

[<img src="docs/screenshots/gui-message-log.png" width="280">](docs/screenshots/gui-message-log.png)
**Meldungsprotokoll**: Diagnose mit Zeitstempeln

</td>
</tr>
</table>

Alle Bildschirmfotos finden Sie in der [englischen README](README.md).

### Intelligente Systemausschlüsse
Ordnersicherungen überspringen `System Volume Information`, `$RECYCLE.BIN`, `Recovery` (im Wurzelverzeichnis eines Laufwerks), `.pbs-rollback`, `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` und `DumpStack.log.tmp`. Ein Laufwerk mit 1,03 TB Belegung kann nur 141 GB echte Dateien enthalten. Empfehlung: **Ordner**sicherung für Dateien, **Maschinen**sicherung in einem eigenen Satz für die Bare-Metal-Wiederherstellung.

### Sicherheit und Qualität
Optionale Verschlüsselung mit signiertem Manifest; lokale Dienst-API nur auf 127.0.0.1 und mit Token; Eingabeprüfung, Schutz vor Path Traversal, Wiederholungen mit exponentiellem Backoff; Unit- und End-to-End-Tests.

## 🔐 Clientseitige Verschlüsselung

Jeder Chunk wird vor dem Verlassen des Rechners mit AES-256-GCM verschlüsselt, mit einer PBS-Schlüsseldatei (austauschbar mit dem offiziellen Client); das Manifest ist signiert, der Server sieht den Schlüssel nie.

**GUI:** **Einstellungen > Kontoinformationen**, Server bearbeiten, Reiter **Verschlüsselung**: Schlüssel wählen oder **Generieren...**. Für einen geschützten Schlüssel: **Nachfragen** (einmal pro Sitzung) oder **Auf diesem Computer merken** (nötig für den Dienst und unbeaufsichtigte Aufträge). **Ohne die Schlüsseldatei ist keine Wiederherstellung möglich: Bewahren Sie eine Kopie getrennt vom Rechner auf.**

**CLI:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` ist nur bei einem geschützten Schlüssel nötig (wird sonst abgefragt); `proxmoxbackup-machine` und `proxmoxbackup-nbd` kennen dieselben Optionen. Die Clonezilla-ISO fragt nach Schlüssel und Passphrase und schreibt den Schlüssel nie in die ISO.

**Einschränkungen:** `config.json` enthält Tokens, Passwörter und gemerkte Passphrasen im Klartext (auf master ist der Ordner geschützt; bis v0.8.2 war er unter Windows für lokale Benutzer lesbar). Ein verlorener Schlüssel ist nicht wiederherstellbar.

## Schnellere Ordnersicherungen: Änderungserkennung

| Modus | Liest | Gespeichert als |
|---|---|---|
| **Klassisch** (`legacy`, Standard) | jede Datei | ein `<name>.pxar.didx` pro Ordner |
| **Daten** (`data`) | jede Datei | geteiltes Archiv: `<name>.mpxar.didx` + `<name>.ppxar.didx` |
| **Metadaten** (`metadata`) | nur neue und geänderte Dateien | dasselbe geteilte Archiv; unveränderte Dateien werden ungelesen aus der vorherigen Sicherung übernommen |

**GUI:** Satz bearbeiten, Reiter **Ziel**, **Änderungserkennung** (nur Ordner-Sätze; Maschinensicherungen bleiben unverändert). Mit **Alle Dateien neu lesen alle N Läufe** werden auch seltene Änderungen erfasst, die Größe und Zeit gleich lassen. Geben Sie jedem Satz **eine eigene Sicherungs-ID**: Zwei Sätze mit derselben ID vergleichen sich gegenseitig und lesen alles.

**CLI:** `-change-detection-mode legacy|data|metadata`; eine Option „alle N Läufe“ gibt es nicht, planen Sie gelegentlich einen Lauf mit `-change-detection-mode data`. Bevor Sie auf Daten oder Metadaten umstellen, aktualisieren Sie jeden Rechner, der wiederherstellt, auf v0.6.0 oder neuer: Ältere Versionen lesen keine geteilten Archive.

## 🚀 Schnellstart (GUI)

1. MSI, `.deb` oder `.rpm` installieren (oder zip / tar.gz entpacken)
2. Unter Windows **Als Administrator ausführen** für VSS und Maschinensicherungen; unter Linux **Proxmox Backup Client (administrator)** für Maschinensicherungen
3. PBS-Server unter **Einstellungen > Kontoinformationen** hinzufügen und mit **Test** prüfen
4. Einen Sicherungssatz (oder eine einmalige Sicherung) anlegen und Ordner oder Datenträger auswählen
5. Ausführen; damit Zeitpläne ohne angemeldeten Benutzer laufen, **Einstellungen > Erweitert > Als Dienst ausführen** einschalten

Das [GUI-Handbuch](docs/manual/GUI.md) erklärt jeden Schritt.

## Vollständige Live-Sicherung des Rechners

Gleiche Syntax wie bei Ordnersicherungen, mit `-backupdev` statt `-backupdir`:

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Der Befehl erstellt einen VSS-Snapshot der Partitionen von Datenträger 0 und ein bootfähiges Abbild des ganzen Datenträgers (fidx). Freier Speicher geht ungelesen als Nullen ein (`-readfreespace` liest alles). Folgende Sicherungen sind inkrementell.

### Voraussetzungen für Linux-Maschinensicherungen

Linux hat kein VSS: Das Projekt nutzt das Kernelmodul `elastio-snap` (ein Fork von `dattobd`); installieren Sie die gepflegte Version [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap) (ein bereits geladenes `dattobd` geht auch). Zwei Bedingungen sind Pflicht:

1. **Das Kernelmodul muss installiert und geladen sein.** Das dokumentierte Repository-Paket verweist auf eine kaputte URL (mindestens für Ubuntu 22.04); das Bauen aus dem Quellcode funktioniert:
   ```bash
   sudo apt-get install linux-headers-$(uname -r) build-essential   # Debian/Ubuntu
   git clone --depth 1 https://github.com/Axcient/elastio-snap.git
   cd elastio-snap
   sudo make
   sudo make install
   sudo depmod -a     # required - `make install` only copies the .ko, it never
                       # runs depmod, so a bare `modprobe elastio-snap` right
                       # after a fresh install fails with "not found in
                       # directory" even though the file is right there
                       # (confirmed live 2026-09-24)
   sudo modprobe elastio-snap
   lsmod | grep elastio-snap   # confirms it loaded
   ```
   Das Modul bleibt nur bis zum nächsten Neustart geladen (für das automatische Laden siehe `/etc/modules-load.d/`).

2. **Die Sicherung muss als root laufen (EUID 0).** Die Gruppe `disk` reicht nicht: Verwenden Sie den Eintrag **Proxmox Backup Client (administrator)**, den Hintergrunddienst oder `sudo`. Eine Ordnersicherung hat diese Anforderung nicht.

### Dateien aus einer Datenträgersicherung wiederherstellen

Die Seite Wiederherstellung öffnet eine Maschinensicherung wie einen Ordner, ohne etwas einzubinden. Unter Linux lässt sie sich auch per NBD einbinden (nach `modprobe nbd max_part=0`):

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Ohne `-path` erscheint eine Textoberfläche zur Auswahl. Nicht auf wichtigen Rechnern verwenden; vor dem Beenden alles aushängen, sonst `nbd-client -d /dev/nbd0`.

### Wiederherstellung auf einen physischen Rechner

Die angepasste Clonezilla-ISO bietet einen vollautomatischen Eintrag **„PBS Bare Metal Restore“**: PBS-Daten eingeben, Snapshot wählen, Datenträger bestätigen, danach laufen Netzwerk, Wiederherstellung und Neustart ohne weiteres Zutun. Der halbmanuelle Eintrag „pbs-nbd“ bleibt verfügbar. Siehe [BMR-Handbuch](docs/manual/BMR.md).

## Verwendung: Ordnersicherung

Vollständige Referenz im **[Kommandozeilen-Handbuch](docs/manual/CLI.md)**. Ein typischer Befehl:

```shell
proxmoxbackup-directory.exe -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -backupdir "C:\path\to\backup" -datastore "datastorename"
```

```
proxmoxbackup-directory
  -baseurl string            PBS URL, example: https://192.168.1.10:8007
  -certfingerprint string    Certificate fingerprint (asked for interactively when omitted)
  -authid string             API token ID (user@realm!token)
  -secret string             API token secret
  -pbsusername string        PBS user for ticket login (with -pbspassword; overrides -authid/-secret)
  -pbspassword string        Password for -pbsusername (asked for when omitted)
  -datastore string          Datastore name
  -namespace string          Namespace (optional)
  -backup-id string          Backup ID (default: the hostname)
  -backupdir string          Source directory; repeat for several (each becomes group <id>_<path>)
  -change-detection-mode     legacy (default), data or metadata
  -novss                     Do not snapshot the source (VSS on Windows, elastio-snap/dattobd on Linux)
  -keyfile string            PBS encryption key file
  -keyfile-passphrase string Passphrase for a protected key (asked for when omitted)
  -backupstream string       Back up standard input as <name>.didx instead of a directory
  -pxarout string            Also write the archive to a local file (debugging)
  -mail-host, -mail-port, -mail-username, -mail-password, -mail-insecure,
  -mail-from, -mail-to, -mail-subject-template, -mail-body-template
                             Email report (optional)
  -config string             JSON config file; flags given on the command line override it
```

Die Umgebungsvariablen `PBS_REPOSITORY` usw. des offiziellen Clients werden ebenfalls gelesen.

## 🔨 Aus dem Quellcode bauen

Alles steht in **[BUILD.md](BUILD.md)** (GUI, Dienst, CLI-Werkzeuge, MSI, Linux-Pakete). Kurz gefasst, mit Go 1.25 oder neuer und der Wails CLI v2.13:

```bash
cd gui
wails build                                    # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe   # the background service
```

Das gebaute Frontend liegt im Repository: Node.js braucht man nur, um es zu ändern. Tests: `make test`.

## 🔧 Fortgeschrittene Nutzung und Anleitungen

- **Multi-PBS** (mehrere Server, Ziel pro Satz wählbar): [Benutzerhandbuch](MULTI_PBS_USER_GUIDE.md) und [Implementierungsleitfaden](MULTI_PBS_GUIDE.md).
- **Clonezilla-ISO für Bare-Metal-Wiederherstellung**: entsteht durch Patchen einer Clonezilla-Live-ISO, Details in [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md):

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

- [FEATURES_STATUS.md](FEATURES_STATUS.md): was fertig ist und seit welcher Version; [CHANGELOG.md](CHANGELOG.md): Verlauf pro Version; [TODO.md](TODO.md): Roadmap; [RELEASE_NOTES.md](RELEASE_NOTES.md); [AGENTS.md](AGENTS.md): Aufbau des Repositorys.

## 🌐 Technischer Support in Italien

Das Projekt und **alle** Beiträge bleiben für immer öffentlich und unter GPLv3. E.T.I. Srl ( https://etitech.net ), Hauptsponsor des Projekts und der neuesten Maschinensicherungsfunktionen, bietet bei Bedarf Support für Proxmox-Installationen und die Windows-Backup-Werkzeuge an. Getrennte Ausgaben „Community“ und „Enterprise“ wird es nie geben: Support ist ein unabhängiger Dienst, und jede gewünschte Anpassung wird, auch bei bezahlter Entwicklung, unter GPLv3 veröffentlicht.

## 🖥️ Herkunft der GUI

Die GUI basiert auf der **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)** von **[RDEM Systems](https://www.rdem-systems.com/)**, die ihre Entwicklung sponsert und kommerziellen Support anbietet. Der gesamte Code bleibt unter GPLv3.

## ⚠️ Warnung

Die Software wird „wie besehen“ bereitgestellt und hat noch Alpha-Qualität; wir übernehmen keine Haftung für Datenverlust. Testen Sie Sicherungen und Wiederherstellungen vor dem Produktiveinsatz.

## 📄 Lizenz

GPLv3: siehe [LICENSE](LICENSE).

## 🏷️ Branding

Jeder Mitwirkende mit mindestens **5 Commits** für Funktionen oder Fehlerbehebungen darf sein Branding für kommerzielle Nutzung hinzufügen lassen, sofern das Unternehmen keine Malware-Kampagnen, Kriegsförderung (in welchem Land auch immer), Betrug, Datendiebstahl, Kinderhandel, Gewalt, Diskriminierung, Drogen oder sonstige illegale Tätigkeiten betreibt. Bei einer Beschwerde ohne stichhaltige Erklärung wird dieser Vorteil sofort entzogen. Die GPLv3 bleibt gültig: Sie dürfen das Projekt jederzeit forken.

## 🤝 Mitwirken

Beiträge sind willkommen, besonders:

1. Verschlüsselung: muttersprachliche Durchsicht der maschinell übersetzten Texte, Schlüsselverwaltung über die Schlüsseldatei hinaus (z. B. Windows DPAPI)
2. Physisch-zu-virtuell-Migration (P2V): Die Grundlagen funktionieren über den Typ `vm`; es fehlen Netzwerkkarte, TPM, virtio-Treiber, echte CPU/RAM-Werte, andere Busse als SATA
3. Asynchroner / mehrkerniger Upload der Chunks
4. Windows-Sicherheitsbeschreibungen im pxar-Format (Patch auf Proxmox-Seite)
5. Windows-Symlinks

## Über

Die Mitwirkenden von Proxmox Backup Client GO entwickeln und pflegen dieses Projekt, das sich auf die NTP/NTS-Infrastruktur und die [11 öffentlichen NTS-Server](https://github.com/jauderho/nts-servers) stützt.

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
