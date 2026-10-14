# Proxmox Backup Client: Windows- und Linux-Client für Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client ist ein quelloffener (GPL-3.0) Backup-Client für Proxmox Backup Server (PBS), für Windows und Linux.** Er sichert Ordner als durchsuchbare Dateiarchive und ganze Rechner als bootfähige Datenträgerabbilder, nach Zeitplan oder auf Abruf, und stellt alles wieder her, von einer einzelnen Datei bis zu einem ganzen Rechner, auf denselben Computer, auf ein Ersatzgerät oder in eine virtuelle Maschine unter Proxmox VE.

Ich entwickle ihn für mein eigenes Homelab, damit PBS das einzige Sicherungssystem für eine Mischung aus Windows- und Linux-Servern, Desktops und Laptops wird, anstelle von Backup for Workgroups, rsync-Spiegeln und Clonezilla/RescueZilla-Abbildern. Er begann im September 2026 als Fork von [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) und ist weit darüber hinausgewachsen: seitdem über 350 Commits, davon 18 Fehlerbehebungen und Funktionen, die an das Ursprungsprojekt zurückgegeben wurden ([Liste unten](#-beiträge-zum-ursprungsprojekt)).

**Sie dürfen ihn gern so verwenden, wie er ist.** Er ist offiziell **Alpha-Software**, die Nutzung erfolgt auf eigenes Risiko, ohne jegliche Garantie oder Gewährleistung. Ich teste jedes Release selbst, und er ist auf meinen eigenen Rechnern regelmäßig im Einsatz, aber lesen Sie [Status, Fehlerberichte und Funktionswünsche](#-status-fehlerberichte-und-funktionswünsche), bevor Sie sich darauf verlassen.

Die Sammlung:

- **Proxmox Backup Client GUI**: die App für Windows und Linux. Sicherungssätze mit jeweils mehreren Zeitplänen, Ordner- und Ganzdatenträgersicherungen mit konsistenten Snapshots (VSS unter Windows, elastio-snap/dattobd unter Linux), Wiederherstellung von Dateien aus Ordner- und Maschinensicherungen, Gelöschte Dateien (Undelete) und Zurücksetzen (Roll back), mehrere PBS-Server, clientseitige Verschlüsselung und ein Hintergrunddienst, damit Zeitpläne auch ohne angemeldeten Benutzer laufen.
- **`proxmoxbackup-directory`**: Ordnersicherungen (pxar) per Kommandozeile, mit Deduplizierung und Änderungserkennung.
- **`proxmoxbackup-machine`**: Live-Sicherungen ganzer Datenträger (fidx) per Kommandozeile, für Windows- und Linux-Rechner; das Abbild ist nach einer Wiederherstellung bootfähig.
- **`proxmoxbackup-nbd`**: bindet eine Datenträgersicherung als NBD-Gerät ein (Linux).
- **Eine ISO für die Bare-Metal-Wiederherstellung** auf Basis von Clonezilla Live, mit einer geführten Wiederherstellung vom Einschalten bis zum Neustart.

> Stichwörter: proxmox backup client windows · PBS client · Windows VSS backup · physical to virtual (P2V) · bare-metal restore · Proxmox Backup Server interface.

## ⭐ Was er kann, mit gemessenen Ergebnissen

| | Ergebnis |
|---|---|
| **Inkrementelle Ordnersicherungen** | Eine Dateiserver-Freigabe mit 480,9 GB und 761.866 Dateien: erster Lauf 4 h 17 min, jeder weitere Lauf 10 bis 15 Minuten. Unveränderte Dateien werden nicht einmal geöffnet (die Metadata-Änderungserkennung des offiziellen Clients). |
| **Schnelle Maschinensicherungen** | Freier Speicher, die Auslagerungsdatei und die Ruhezustandsdatei werden nicht gelesen, und leerer Speicherplatz wird nicht gehasht. Eine 1-TB-Laptopplatte mit 373 GB Belegung: vorher 1 h 23 min bis 1 h 44 min, jetzt 38 Minuten. Eine 50-GB-Testplatte: 3 min 36 s, jetzt 1 min 15 s. |
| **Dateien aus Maschinensicherungen** | Ein Datenträgerabbild wie einen Ordner durchsuchen und einzelne Dateien wiederherstellen, von NTFS, FAT, exFAT, ext2/3/4 und XFS. Das 127-GB-Laufwerk C: eines Laptops mit 430.000 Dateien wird in 4 bis 15 Sekunden aufgelistet; vorher wird nichts eingebunden oder heruntergeladen. |
| **Physischer Rechner als virtuelle Maschine** | Einen PC oder Server sichern und ihn dann als VM unter Proxmox VE starten: wiederhergestellt über Proxmox VE selbst oder mit der Wiederherstellungs-ISO. [Siehe unten](#-physische-rechner-als-virtuelle-maschinen-p2v). |
| **Bare-Metal-Wiederherstellung** | Die ISO starten (USB-Stick, CD oder PXE), die PBS-Daten eingeben, die Sicherung auswählen: Ein Windows-Rechner mit 32 GB war 8 Minuten nach dem Einschalten wieder am Anmeldebildschirm. |
| **Gelöschte Dateien** | Nachdem 39 GB aus einer Freigabe mit 761.000 Dateien gelöscht worden waren, fand eine 7-Tage-Suche genau diese Dateien in etwa einer Minute und stellte sie zurück. |
| **Zurücksetzen** | Die Ordner eines Satzes auf den Stand einer gewählten Sicherung zurückbringen, mit einer Vorschau vorher und Rückgängig danach. |
| **Zeitpläne** | Mehrere pro Sicherungssatz, von alle paar Minuten bis einmal im Jahr: „täglich um 02:00“ plus „am letzten Tag jedes Monats“ oder „am 2. Dienstag“, jeder in Klartext mit seinen nächsten 3 Läufen. |
| **Genauigkeit der Wiederherstellung** | NTFS-Berechtigungen, Attribute, alternative Datenströme und erweiterte Attribute, Erstellungs- und Ordnerzeiten unter Windows; Modi, Eigentümer, ACLs, erweiterte Attribute und symbolische Links unter Linux. Ein optionaler Durchgang liest das Wiederhergestellte erneut und vergleicht es mit der Sicherung. |
| **Funktioniert mit den Werkzeugen von Proxmox** | Hier erstellte Sicherungen lassen sich mit dem offiziellen `proxmox-backup-client` byte-identisch wiederherstellen, und umgekehrt. [Details](#-kompatibel-mit-dem-offiziellen-proxmox-client-in-beide-richtungen). |

## 🧩 Was er gegenüber dem Ursprungsprojekt hinzufügt

Über die [18 in das Ursprungsprojekt übernommenen Änderungen](#-beiträge-zum-ursprungsprojekt) hinaus bietet dieser Client:

**Sicherungen**
- Inkrementelle Ordnersicherungen mit der Änderungserkennung des offiziellen Clients (Modi Data und Metadata, geteilte pxar-v2-Archive), mit einem vollständigen Lesen alle N Läufe.
- Maschinensicherungen, die freien Speicher, die Auslagerungsdatei und die Ruhezustandsdatei überspringen (Windows NTFS; Linux ext2/3/4 und XFS), unlesbare Sektoren als Nullen schreiben und weiterlaufen, und während des Laufs die Anzahl der Blöcke, Datenträger, Partitionen und Chunks anzeigen.
- Pause, Fortsetzen und Stoppen für Ordner- und Maschinensicherungen, auch für Sicherungen, die der Hintergrunddienst ausführt.
- Alternative Datenströme, erweiterte NTFS-Attribute und Erstellungszeiten in Windows-Ordnersicherungen.

**Zeitpläne und Sicherungssätze**
- Mehrere Zeitpläne pro Sicherungssatz, von alle paar Minuten bis einmal im Jahr (der n-te Wochentag, der letzte Tag des Monats), jeder in Klartext mit seinen nächsten 3 Läufen.
- Ein Hintergrunddienst unter Linux wie unter Windows, und Als Administrator neu starten unter Linux.
- Schnelles oder vollständiges Jetzt ausführen, ein Kennzeichen VSS / Snapshot, eine Warnung **Geteilte ID** (SHARED ID), Satznamen, die als Snapshot-Kommentare in PBS geschrieben werden, E-Mail-Benachrichtigungen, Aktionen nach der Sicherung sowie Export und Import der Einstellungen.

**Wiederherstellungen**
- Einzelne Dateien aus Maschinensicherungen (NTFS, FAT, exFAT, ext2/3/4, XFS), ohne das Abbild einzubinden oder herunterzuladen.
- Gelöschte Dateien und Zurücksetzen, für Ordner- und Maschinen-Sicherungssätze, in der GUI und per Kommandozeile.
- Ein optionaler Prüfdurchgang nach einer Wiederherstellung, Windows-Erstellungszeiten und ein Snapshot-Baum, der jeweils einen Ordner lädt, sodass auch sehr große Sicherungen sofort geöffnet werden.
- Eine geführte ISO für die Bare-Metal-Wiederherstellung, die auch in eine leere VM wiederherstellt, und getestete Wege von physisch zu virtuell ([P2V](#-physische-rechner-als-virtuelle-maschinen-p2v)).
- Byte-identische Kompatibilität mit dem offiziellen `proxmox-backup-client` in beide Richtungen, einschließlich seiner Data- und Metadata-Snapshots (geteiltes Archiv).

**Die App**
- Eine neu aufgebaute Oberfläche: Menüleiste, Seitenmenü, Berichte mit dem vollständigen Verlauf der Sicherungen und Wiederherstellungen, ein Meldungsprotokoll, fünf Farbschemata und eine eigene Farbe, Benachrichtigungen im Infobereich und als Toast.
- 18 Oberflächensprachen, jede überarbeitet (das Original hat 5).
- Einstellungsordner nur für Administratoren (Windows) bzw. nur für seinen Eigentümer (Linux) zugänglich, und ein API-Token des Dienstes, das bei jedem Start erneuert wird.
- MSI-, `.deb`- und `.rpm`-Pakete sowie die Benutzerhandbücher für die GUI, die Kommandozeile und die Bare-Metal-Wiederherstellung.

## 🔄 Physische Rechner als virtuelle Maschinen (P2V)

Eine Maschinensicherung eines physischen PCs oder Servers kann auf zwei Wegen zu einer virtuellen Maschine unter Proxmox VE werden:

**1. In Proxmox VE wiederherstellen.** Stellen Sie den Sicherungssatz (oder die einmalige Sicherung) auf **Sichern als: Virtuelle Maschine von Proxmox VE** (Backup as: Proxmox VE virtual machine). Die Sicherung wird als `vm/<Nummer>` mit einer kleinen VM-Konfiguration gespeichert, sodass Proxmox VE sie zusammen mit seinen anderen VM-Sicherungen auflistet: Öffnen Sie den PBS-Speicher in Proxmox VE, wählen Sie die Sicherung, **Restore**, und wählen Sie eine VM-ID und einen Speicher. Die VM erhält einen SATA-Datenträger und die zum Datenträger passende Firmware (OVMF für UEFI/GPT, SeaBIOS für BIOS/MBR); fügen Sie vor dem Start eine Netzwerkkarte hinzu, und eine EFI-Disk (Pre-Enroll keys nicht angehakt), falls eine UEFI-VM nicht startet.

**2. Mit der ISO für die Bare-Metal-Wiederherstellung in eine leere VM wiederherstellen.** Legen Sie eine VM mit derselben Firmware wie der ursprüngliche Rechner an (OVMF mit einer EFI-Disk für UEFI/GPT, SeaBIOS für BIOS/MBR), mit einem SATA-Datenträger, der mindestens so groß ist wie das Original, und einer Netzwerkkarte Intel E1000, starten Sie sie von der ISO und führen Sie die Wiederherstellung aus. Das funktioniert für `host`- und `vm`-Sicherungen gleichermaßen, und dieselbe Sicherung lässt sich weiterhin auf echte Hardware wiederherstellen.

Getestete Wege:

| Von | Nach | Weg | Ergebnis |
|---|---|---|---|
| Windows-10-Laptop, UEFI/GPT | VM unter Proxmox VE | Wiederherstellungs-ISO | Startete auf den Desktop des Benutzers |
| Windows Server, UEFI/GPT | VM unter Proxmox VE | Wiederherstellung in Proxmox VE | Startete ohne Änderungen |
| Xubuntu-Desktop, UEFI/GPT | VM unter Proxmox VE | Wiederherstellung in Proxmox VE, und die Wiederherstellungs-ISO | Startete auf den Desktop |
| Linux, BIOS/MBR | VM unter Proxmox VE | Wiederherstellungs-ISO, und Wiederherstellung in Proxmox VE | Startete bis zur Anmeldeaufforderung |
| Linux Mint, verschlüsselte Sicherung | VM unter Proxmox VE | Wiederherstellungs-ISO, Schlüssel von einem USB-Stick | Startete |
| Linux-PC | derselbe PC mit einer neuen Platte | Wiederherstellungs-ISO | Startete |

Windows richtet die neue virtuelle Hardware beim ersten Start ein („Geräte werden vorbereitet“). Das [Handbuch zur Bare-Metal-Wiederherstellung](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines) enthält die Firmware-Tabelle, die Treiber und eine Schritt-für-Schritt-Anleitung.

## 📖 Handbücher

| Handbuch | Inhalt |
|---|---|
| **[GUI-Handbuch](docs/manual/GUI.md)** | Installation, PBS-Server, Sicherungssätze und Zeitpläne, inkrementelle Sicherungen, Fortschritt, Wiederherstellung, Gelöschte Dateien, Zurücksetzen, Berichte, Protokolle, Einstellungen, der Hintergrunddienst, Fehlerbehebung |
| **[Kommandozeilen-Handbuch](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: alle Optionen, Konfigurationsdateien, Exit-Codes, Zeitplanung; Gelöschte Dateien und Zurücksetzen per Kommandozeile |
| **[Handbuch zur Bare-Metal-Wiederherstellung](docs/manual/BMR.md)** | Die Wiederherstellungs-ISO, Schritt für Schritt; `vm`-Sicherungen auf Proxmox VE; physisch zu virtuell; abweichende Hardware |
| [Wiederherstellungsszenarien](docs/RESTORE_GUIDE.md) | Welche Wiederherstellung wann |
| [Übersicht der Handbücher](docs/manual/README.md) | Ein Überblick über die drei Handbücher |

## 🆕 Neu in v0.9.0: mehrere Zeitpläne pro Sicherungssatz, Installer, ein Linux-Dienst

- **Mehrere Zeitpläne pro Sicherungssatz**, von alle paar Minuten bis einmal im Jahr: „täglich um 02:00“ plus „jeden Monat am letzten Tag“ oder „am 2. Dienstag“, jeder Zeitplan in Klartext mit seinen nächsten 3 Läufen. Bestehende Sätze behalten ihren Zeitplan.
- **Ordnersicherungen behalten alternative Datenströme und erweiterte NTFS-Attribute** (Windows), etwa die Markierung „aus dem Internet heruntergeladen“, und eine Wiederherstellung stellt sie wieder her. Unter Bekannte Einschränkungen ist jetzt nichts mehr aufgeführt.
- **Installer**: ein **Windows-MSI** (App und Hintergrunddienst unter Program Files, Eintrag im Startmenü, Aktualisierung an Ort und Stelle, Einstellungen beim Entfernen behalten oder löschen) und **Linux-Pakete**, `.deb` (Debian, Ubuntu, Mint) und `.rpm` (Fedora), mit der App, den drei Kommandozeilenwerkzeugen, den Handbüchern und Menüeinträgen.
- **Ein Hintergrunddienst unter Linux** (`proxmoxbackupclient.service`): Geplante Sicherungssätze laufen als root, auch wenn niemand angemeldet ist. Eingeschaltet wird er unter Einstellungen > Erweitert > Als Dienst ausführen, wie unter Windows. **Als Administrator neu starten** funktioniert jetzt auch unter Linux (über pkexec).
- **Pause, Fortsetzen und Stoppen für Sicherungen, die der Dienst ausführt**, mit ihrer Fortschrittskarte in der geöffneten App.
- **Eine Warnung, wenn zwei Sicherungssätze eine Sicherungsgruppe teilen** (gleicher Server, Typ und gleiche Sicherungs-ID), mit einem Kennzeichen **Geteilte ID** auf beiden Karten.
- **Einstellungen abgesichert**: Unter Windows ist `C:\ProgramData\ProxmoxBackupClient` jetzt nur noch für Administratoren zugänglich; unter Linux ist der Einstellungsordner privat für seinen Eigentümer.
- **Dateien aus Maschinensicherungen aller gängigen Dateisysteme**: FAT, exFAT, ext2/3/4 und XFS ebenso wie NTFS. Wiederherstellungen von Windows-Abbildern stellen auch Berechtigungen und alternative Datenströme wieder her.
- **Linux-Maschinensicherungen überspringen freien Speicher** (ext2/3/4, XFS), wie die unter Windows seit v0.8.0.
- **Gelöschte Dateien und Zurücksetzen für Linux-Maschinen-Sicherungssätze**, und **per Kommandozeile** für jeden Sicherungssatz: `-list-sets`, `-undelete`, `-rollback` (Abschnitt 12 des [Kommandozeilen-Handbuchs](docs/manual/CLI.md)).
- Die Kommandozeilenwerkzeuge geben Fehler auf stderr aus, statt Meldungsfenster anzuzeigen, und `proxmoxbackup-machine` verschickt seine Benachrichtigungs-E-Mail.
- **Die ISO für die Bare-Metal-Wiederherstellung** bietet nie den Stick, von dem sie gestartet wurde, als Zieldatenträger an, und das Handbuch erklärt, wie die Firmware (UEFI oder BIOS) passen muss, wenn aus einem physischen Rechner eine VM wird.
- **Alle Sprachen überarbeitet** (17 Übersetzungen, rund 1.850 Texte), und Datums- und Zahlenformate folgen der Sprache der App.

Frühere Releases: **v0.8** schnellere Maschinensicherungen (freier Speicher übersprungen, defekte Sektoren überstanden), Dateien aus Maschinensicherungen, Pause und Fortsetzen; **v0.7** Gelöschte Dateien und Zurücksetzen; **v0.6** inkrementelle Ordnersicherungen. Das [CHANGELOG](CHANGELOG.md) enthält jede Änderung.

## 🤝 Kompatibel mit dem offiziellen Proxmox-Client, in beide Richtungen

Geprüft mit `proxmox-backup-client` 3.4.9 und PBS 4.2:

- **Legacy-, Data- und Metadata-Snapshots dieses Clients lassen sich mit dem offiziellen Client byte-identisch wiederherstellen**, sowohl aus der GUI als auch von der Kommandozeile erstellt, unter Windows oder Linux, und der Dateibrowser von PBS 4.2 öffnet sie. Unter Linux behalten die wiederhergestellten Dateien außerdem exakt ihren Modus, ihre Änderungszeit auf die Nanosekunde und die Ziele ihrer symbolischen Links.
- **Legacy-, Data- und Metadata-Snapshots des offiziellen Clients lassen sich mit diesem Client byte-identisch wiederherstellen**, vollständig oder selektiv.
- Gleiches geteiltes Archivformat (pxar Version 2), gleiche Modi der Änderungserkennung, gleiches Format der Schlüsseldatei für die Verschlüsselung (mit `proxmox-backup-client key create` erzeugte Schlüssel funktionieren hier, und hier erzeugte Schlüssel funktionieren dort), gleiche Umgebungsvariablen im Stil von `PBS_REPOSITORY`.

Ihre Sicherungen sind also nie an diesen Client gebunden: Die Werkzeuge von Proxmox stellen jeden Snapshot wieder her, den er erstellt, und er stellt jeden Snapshot wieder her, den der offizielle Client erstellt. Das einzige Extra ist eine kleine Datei, die dieser Client im Wurzelverzeichnis eines Ordner-Snapshots ablegt, `.proxmox_backup_client_meta.json` (der ursprüngliche Pfad), die der offizielle Client als gewöhnliche Datei wiederherstellt.

> ⚠️ **Hinweis:** Dieses Projekt ist **in keiner Weise verbunden** mit der **Proxmox Server Solutions GmbH**. „Proxmox“, das Proxmox-Logo und zugehörige Namen sind Eigentum ihrer jeweiligen Inhaber; sie werden hier **nur** zur Angabe der Kompatibilität verwendet. Ihre Produkte finden Sie auf [proxmox.com](https://www.proxmox.com/).

## 📦 Download und Installation

👉 **[Neuestes Release herunterladen](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| Sie haben | Verwenden Sie |
|---|---|
| Windows | Der **MSI-Installer** (`ProxmoxBackupClient-<version>-x64.msi`) installiert die App und den Hintergrunddienst unter Program Files, mit einem Eintrag im Startmenü. Oder entpacken Sie `ProxmoxBackupClient-v<version>-windows-amd64.zip` (die App und `ProxmoxBackupClientSVC.exe`; lassen Sie beide im selben Ordner) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Ein anderes Linux | Entpacken Sie `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (benötigt die WebKitGTK-4.1-Laufzeit) |
| Nur Kommandozeile | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` oder `-linux-amd64.tar.gz` |
| Einen Rechner zum Wiederherstellen | `ProxmoxBackupClient-GO-BMR-Restore-v<version>.iso`, die ISO für die Bare-Metal-Wiederherstellung |

MSI, `.deb` und `.rpm` sind neu in v0.9.0: Releases bis v0.8.1 enthalten nur die zip- und tar.gz-Dateien. [BUILD.md](BUILD.md) beschreibt, wie Sie sie selbst bauen.

Die Linux-Pakete installieren zwei Menüeinträge: **Proxmox Backup Client** startet die App als Sie selbst (Ordnersicherungen Ihrer eigenen Dateien), und **Proxmox Backup Client (administrator)** startet sie als root, wobei über polkit nach Ihrem Passwort gefragt wird, für Maschinensicherungen und Block-Snapshots. Der Hintergrunddienst wird installiert, aber nicht eingeschaltet; siehe [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **Windows meldet „Virus erkannt“ (z. B. `Trojan:Win32/Sabsik.FL.A!ml`) oder zeigt eine SmartScreen-Warnung?**
> Das ist ein **bekannter Fehlalarm** bei Go/Wails-Anwendungen: Es ist *kein* Virus. Die Endung `!ml` bedeutet eine Erkennung durch ein Machine-Learning-Modell, das *unsignierte und wenig verbreitete* ausführbare Dateien markiert. Prüfen Sie den Download mit `SHA256SUMS.txt` (siehe unten).

### 🔎 Einen Download prüfen

Jedes Release enthält eine `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-0.9.0-x64.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
```

Unter Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`.

> ℹ️ **Noch keine Codesignatur, keine von einer CI signierte Attestierung und kein VirusTotal-Scan.** Releases werden von Hand gebaut und hochgeladen, noch nicht über eine CI-Pipeline, die eine überprüfbare Attestierung der Build-Herkunft erzeugen könnte. Siehe [TODO.md](TODO.md), falls Sie beim Einrichten helfen möchten.

## ✨ Funktionen

### GUI
- **🌍 Mehrsprachig**: 18 Oberflächensprachen: Französisch, Englisch, Italienisch, Deutsch, Polnisch, Spanisch, Bulgarisch, Tschechisch, Griechisch, Ungarisch, Lettisch, Litauisch, Niederländisch, Portugiesisch, Rumänisch, Slowakisch, Türkisch und Ukrainisch
- Native Menüleiste und Seitenmenü (Sicherung, Wiederherstellung, Gelöschte Dateien, Zurücksetzen, Berichte, Meldungsprotokoll, Einstellungen, Über)
- **Sicherungssätze**: wiederverwendbare, benannte Aufträge mit **jeweils mehreren Zeitplänen** (von alle paar Minuten bis jährlich, oder manuell), ein Typ-Kennzeichen Ordner oder Maschine, ein Kennzeichen VSS / Snapshot, „Jetzt ausführen“ (schnelles oder vollständiges Lesen bei Metadaten-Sätzen) und eine Warnung **Geteilte ID**, wenn zwei Sätze in dieselbe Sicherungsgruppe schreiben würden
- **Einmalige Sicherungen** von Ordnern oder ganzen Datenträgern
- Seite **Berichte** mit dem vollständigen Verlauf der Sicherungen und Wiederherstellungen, und ein in der Größe begrenztes **Meldungsprotokoll** zur Diagnose
- Dialog **Einstellungen** mit fünf Farbschemata (Bernstein, Blau, Grün, Rot, Dunkel) plus einer Option für eine eigene Farbe
- **Inkrementelle Ordnersicherungen** (Änderungserkennung, Modus Metadaten): Unveränderte Dateien werden ungelesen wiederverwendet, wie beim offiziellen Client, mit einem optionalen vollständigen Lesen alle N Läufe
- **Maschinensicherungen (ganzer Datenträger)** unter Windows und Linux, die freien Speicher überspringen und defekte Sektoren überstehen, gespeichert als `host` oder als `vm`-Sicherung für Proxmox VE
- **Physisch zu virtuell**: Eine Maschinensicherung lässt sich in eine VM unter Proxmox VE wiederherstellen, über Proxmox VE oder die Wiederherstellungs-ISO
- **Wiederherstellung**: Durchsuchen der Snapshots, Dateisuche (Platzhalter, reguläre Ausdrücke, vollständige Pfade) und selektive Wiederherstellung, mit NTFS/POSIX-ACLs, alternativen Datenströmen, Ordnerzeitstempeln, Windows-Erstellungszeiten und einem optionalen Prüfdurchgang
- **Dateien aus Maschinensicherungen**: Dateien von NTFS-, FAT-, exFAT-, ext2/3/4- und XFS-Volumes in einem Datenträgerabbild durchsuchen und wiederherstellen
- **Gelöschte Dateien** und **Zurücksetzen**: Dateien finden, die die Sicherungen eines Sicherungssatzes noch enthalten, der Datenträger aber nicht mehr, und sie zurückstellen; oder die Ordner eines Satzes auf den Stand einer gewählten Sicherung zurückbringen, mit einer Vorschau vorher und Rückgängig danach. Ordner- und Maschinen-Sätze. Siehe das [GUI-Handbuch](docs/manual/GUI.md#10-undelete)
- **Pause, Fortsetzen und Stoppen** einer laufenden Sicherung
- **Als Dienst ausführen**: den Hintergrunddienst unter Einstellungen > Erweitert installieren, starten, stoppen oder entfernen, damit Zeitpläne ohne angemeldeten Benutzer laufen (Windows und Linux)
- Live-Fortschrittskarte: Startzeit und voraussichtliches Ende, erledigte Dateien von der Gesamtzahl, Verarbeitungsgeschwindigkeit (Daten und Dateien pro Sekunde) und tatsächliche Upload-Geschwindigkeit, aktuelle Datei, und warum ein Lauf jede Datei liest
- VSS (Volume Shadow Copy) unter Windows, Block-Snapshots mit elastio-snap/dattobd unter Linux
- E-Mail-Benachrichtigungen und Aktionen nach der Sicherung, Klonen von Sicherungssätzen, Export und Import der Einstellungen
- Mehrere PBS-Server, mit Pinning des Zertifikat-Fingerabdrucks (Vertrauen bei der ersten Verwendung), Anmeldung per API-Token oder Benutzer/Passwort
- **Clientseitige Verschlüsselung**: PBS-Schlüsseldatei pro Server (AES-256-GCM, Manifest signiert), optionaler Schutz durch eine Passphrase, siehe [Clientseitige Verschlüsselung](#-clientseitige-verschlüsselung)
- Administratorschutz für das Ändern von Sicherungssätzen, mit Als Administrator neu starten
- Debug-Protokollierung zur Diagnose

### Kommandozeilenwerkzeuge
- `proxmoxbackup-directory`: Ordnersicherungen (pxar) mit Deduplizierung und `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: Live-Sicherungen ganzer Datenträger von Windows- (VSS) und Linux-Rechnern (elastio-snap/dattobd) (fidx, inkrementell, freier Speicher übersprungen außer mit `-readfreespace`, `-type vm` für eine Sicherung für Proxmox VE)
- `proxmoxbackup-nbd`: NBD-Server zum Wiederherstellen von Datenträgersicherungen (Linux)
- `ProxmoxBackupClientSVC` (Windows) und `proxmoxbackupclient` (Linux-Pakete): `-list-sets`, `-undelete` und `-rollback` für in der GUI definierte Sicherungssätze

Die ersten drei akzeptieren `-keyfile` und `-keyfile-passphrase` für verschlüsselte Snapshots. Vollständige Referenz: [Kommandozeilen-Handbuch](docs/manual/CLI.md).

### 📸 Bildschirmfotos

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Sicherungssätze**: benannte Aufträge mit einem Kennzeichen für den Zeitplan, der als Nächstes läuft (DAILY, WEEKLY, MONTHLY, +N für die übrigen), und einem Typ-Kennzeichen Ordner/Maschine, Jetzt ausführen auf jedem

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Live-Fortschritt der Sicherung**: Durchsatz, Restzeit und die Anzahl neuer und wiederverwendeter Chunks während des Laufs

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Wiederherstellung**: PBS-Server und Sicherungs-ID wählen, Snapshots auflisten oder sie nach Dateinamen durchsuchen

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**Zeitpläne**: mehrere pro Satz, von alle paar Minuten bis einmal im Jahr („jeden Monat am letzten Tag“, „am 2. Dienstag“), mit den nächsten 3 Läufen

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Sichern als VM für Proxmox VE**: den Typ virtuelle Maschine und eine numerische ID wählen; die PBS-Sicherungs-ID wird beim Tippen angezeigt

</td>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Sicherungssätze als VMs für Proxmox VE**: Ein als `vm`-Sicherung gespeicherter Satz trägt ein Kennzeichen `MACHINE (VM)`

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/oneoff-as-vm.png" width="280">](docs/screenshots/oneoff-as-vm.png)
**Einmalige Sicherungen**: dieselbe Option im Formular für einmalige Sicherungen

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**Selektive Wiederherstellung**: einzelne Dateien oder Ordner im Snapshot-Baum ankreuzen, an Ort und Stelle oder anderswo wiederherstellen

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Berichte**: vollständiger Verlauf der Läufe mit einem Detailbereich (Dauer, Chunks, gesicherte Ordner)

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-message-log.png" width="280">](docs/screenshots/gui-message-log.png)
**Meldungsprotokoll**: ein in der Größe begrenzter Diagnose-Feed mit Zeitstempeln über alle Sicherungen und Wiederherstellungen

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**Multi-PBS**: mehrere PBS-Server verwalten, die Verbindung testen, einen Standard wählen

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-edit-server.png" width="280">](docs/screenshots/gui-preferences-edit-server.png)
**Server hinzufügen oder bearbeiten**: URL, Benutzer/Passwort oder API-Token, Datastore und Namespace

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-theme-amber.png" width="280">](docs/screenshots/gui-theme-amber.png)
**Farbschemata**: fünf eingebaute Farbpaletten plus eine eigene Farbe, die die ganze App live umfärben (siehe die Leiste unten)

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-advanced.png" width="280">](docs/screenshots/gui-preferences-advanced.png)
**Sicherungsoptionen**: ehrlich erklärt: Das parallele Entpacken bei der Wiederherstellung ist experimentell und war auf unserer eigenen Testhardware messbar *langsamer*

</td>
</tr>
</table>

**🎨 Farbschemata**: Der Einstellungsdialog selbst färbt sich bei jeder Auswahl um, nicht nur ein Farbmuster:

<table>
<tr>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-amber.png" width="180">](docs/screenshots/gui-theme-amber.png)
Bernstein

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-blue.png" width="180">](docs/screenshots/gui-theme-blue.png)
Klassisches Blau

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-green.png" width="180">](docs/screenshots/gui-theme-green.png)
Grün

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-red.png" width="180">](docs/screenshots/gui-theme-red.png)
Rot

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-dark.png" width="180">](docs/screenshots/gui-theme-dark.png)
Dunkel

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-custom.png" width="180">](docs/screenshots/gui-theme-custom.png)
Eigene Farbe

</td>
</tr>
</table>

**🌍 Mehrsprachig**: die Seite Berichte auf Französisch, Italienisch und Spanisch (und in 15 weiteren Sprachen, darunter Englisch, Deutsch und Polnisch):

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-lang-french.png" width="280">](docs/screenshots/gui-lang-french.png)
Français

</td>
<td width="33%">

[<img src="docs/screenshots/gui-lang-italian.png" width="280">](docs/screenshots/gui-lang-italian.png)
Italiano

</td>
<td width="33%">

[<img src="docs/screenshots/gui-lang-spanish.png" width="280">](docs/screenshots/gui-lang-spanish.png)
Español

</td>
</tr>
</table>

### Intelligente Systemausschlüsse (Ordnersicherungen)
Ordnersicherungen lassen Folgendes von selbst weg:

**Systemordner:** `System Volume Information` (VSS-Speicher, kann über 100 GB erreichen) und `$RECYCLE.BIN`, wo immer sie liegen; `Recovery` nur im Wurzelverzeichnis eines Laufwerks; `.pbs-rollback` (der Sicherheitsordner von Zurücksetzen).
**Systemdateien:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` und `DumpStack.log.tmp`.

**Warum das wichtig ist:** Ein Laufwerk kann 1,03 TB Belegung anzeigen, während die echten Dateien rund 141 GB umfassen. Ohne Ausschlüsse würde die Sicherung VSS-Snapshots enthalten (verschwendeter Platz und verschwendete Zeit); mit ihnen entspricht die Größe den echten Daten.

**Empfehlung:** Verwenden Sie für Sicherungen auf Dateiebene eine **Ordner**sicherung mit diesen Ausschlüssen; verwenden Sie für die Bare-Metal-Wiederherstellung und P2V eine **Maschinen**sicherung in einem eigenen Sicherungssatz (sie ist ein Abbild des ganzen Datenträgers; freier Speicher sowie Auslagerungs- und Ruhezustandsdateien gehen als Nullen ein).

### Sicherheit und Qualität
- Optionale clientseitige Verschlüsselung jedes Chunks, mit einem signierten Manifest
- Einstellungsordner unter Windows nur für Administratoren zugänglich, unter Linux privat für seinen Eigentümer
- Die lokale API des Dienstes lauscht nur auf 127.0.0.1 und verlangt ein Token, das bei jedem Start des Dienstes erneuert wird
- Eingabeprüfung und Bereinigung von Zugangsdaten
- Schutz vor Path Traversal
- Wiederholungslogik mit exponentiellem Backoff
- Fehlerbehandlung und Unit-Tests in jedem Modul, dazu optionale Live-Tests und End-to-End-Tests gegen einen echten PBS

## 🔐 Clientseitige Verschlüsselung

Snapshots können verschlüsselt werden, bevor sie den Rechner verlassen, mit einer Schlüsseldatei für Proxmox Backup Server (das Format, das `proxmox-backup-client key create` schreibt, sodass Schlüssel mit dem offiziellen Client austauschbar sind). Jeder Chunk wird mit AES-256-GCM verschlüsselt und das Manifest wird signiert. Der PBS-Server sieht den Schlüssel nie.

**GUI:** Öffnen Sie **Einstellungen > Kontoinformationen**, bearbeiten Sie einen Server und verwenden Sie seinen Reiter **Verschlüsselung**. Wählen Sie eine vorhandene Schlüsseldatei oder **Generieren...** Sie einen neuen Schlüssel (die erweiterte Option schützt ihn mit einer Passphrase). Der Reiter zeigt den Fingerabdruck des Schlüssels. Die Berichte zeigen, ob jede Sicherung verschlüsselt war. **Ohne die Schlüsseldatei kann ein Snapshot nicht wiederhergestellt werden: Bewahren Sie eine Kopie an einem sicheren Ort auf, getrennt von diesem Rechner.**

**Durch eine Passphrase geschützte Schlüssel (scrypt oder PBKDF2):** Wählen Sie, wie die App den Schlüssel entsperrt:
- **Nachfragen**: Die Passphrase wird einmal pro Sitzung der App abgefragt und nur im Arbeitsspeicher gehalten. Geplante Sicherungen und Sicherungen beim Start fragen nie nach: Sie schlagen sofort mit einer klaren Meldung fehl, bis die Passphrase in dieser Sitzung eingegeben wurde. Eine über den Hintergrunddienst ausgeführte Sicherung schlägt bei einem Schlüssel mit „Nachfragen“ ebenfalls sofort fehl.
- **Auf diesem Computer merken**: Die Passphrase wird wie das PBS-Token in der Konfiguration gespeichert, sodass jeder, der `config.json` lesen kann, den Schlüssel entsperren kann. Das ist die Wahl für unbeaufsichtigte geplante Aufträge und für den Hintergrunddienst.

Die Wiederherstellung fragt nach der Passphrase, wenn Sie einen verschlüsselten Snapshot auswählen.

**CLI:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` ist nur bei einem geschützten Schlüssel nötig und wird abgefragt, wenn es fehlt. `proxmoxbackup-machine` und `proxmoxbackup-nbd` akzeptieren dieselben zwei Optionen; `proxmoxbackup-nbd` braucht sie, um ein verschlüsseltes Datenträgerabbild einzubinden, und prüft den ersten Chunk, bevor es das NBD-Gerät anfasst, sodass ein falscher Schlüssel sofort fehlschlägt.

**Bare-Metal-Wiederherstellung:** Die Wiederherstellungs-ISO fragt nach dem Schlüssel (keiner, eine Schlüsseldatei auf einem USB-Stick oder ein eingegebener Pfad) und nach der Passphrase eines geschützten Schlüssels. Der Schlüssel wird nie in die ISO geschrieben. Siehe [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md).

**Bekannte Einschränkungen**
- Ein von der GUI erzeugter Schlüssel wird mit einer geschützten ACL geschrieben (nur aktueller Benutzer, SYSTEM und Administratoren). Schlüsseldateien, die Sie anderswo aufbewahren, behalten die Berechtigungen, die ihr Ordner ihnen gibt.
- `config.json` enthält die PBS-Tokens, Passwörter und jede gemerkte Passphrase im Klartext. Ab v0.9.0 ist ihr Ordner nur für Administratoren (Windows) bzw. nur für seinen Eigentümer (Linux) zugänglich; bis v0.8.2 war er unter Windows für lokale Benutzer lesbar, schränken Sie ihn also auf einem gemeinsam genutzten Rechner selbst ein.
- Die Wiederherstellung eines verschlüsselten Snapshots braucht dieselbe Schlüsseldatei; geht sie verloren, gibt es keine Rettung.
- Der Ablauf mit „Nachfragen“ und der Dienst sind getrennte Prozesse: Verwenden Sie „Merken“ für alles, was über den Dienst läuft.

## Schnellere Ordnersicherungen: Änderungserkennung

Im standardmäßigen Modus Klassisch liest jede Ordnersicherung jede Datei, sodass ein Lauf so lange dauert wie das Lesen des ganzen Ordners, selbst wenn sich nichts geändert hat. Sicherungssätze (und die CLI) können stattdessen die Modi der Änderungserkennung des offiziellen `proxmox-backup-client` verwenden. Der Modus Metadaten ist täglich auf einem Server mit 480,9 GB und 761.866 Dateien im Einsatz, wo ein Lauf mit wenigen Änderungen 10 bis 15 Minuten statt über 4 Stunden dauert.

| Modus | Liest | Gespeichert als |
|---|---|---|
| **Klassisch** (`legacy`, Standard) | jede Datei | ein `<name>.pxar.didx` pro Ordner, wie bisher |
| **Daten** (`data`) | jede Datei | ein geteiltes Archiv: `<name>.mpxar.didx` (Metadaten) + `<name>.ppxar.didx` (Dateiinhalte) |
| **Metadaten** (`metadata`) | nur neue und geänderte Dateien | dasselbe geteilte Archiv; Dateien, deren Größe und Änderungszeit (auf die Nanosekunde), Modus und Eigentümer mit der vorherigen Sicherung übereinstimmen, werden ungelesen aus ihr übernommen |

Im Modus **Metadaten** liest der erste Lauf alles (es gibt noch nichts zum Vergleichen); spätere Läufe lesen nur, was sich geändert hat. Eine Datei, die so geändert wurde, dass Größe und Änderungszeit gleich bleiben, wird nicht bemerkt, weshalb ein Satz auch **alle N Läufe jede Datei lesen** kann (ein Daten-Lauf): Dieser Lauf erfasst solche Änderungen und entfernt das Auffüllmaterial, das die Wiederverwendung im Archiv hinterlässt. Eine Datei, die nicht vollständig gelesen werden konnte (nach einem Lesefehler mit Nullen aufgefüllt), wird beim nächsten Lauf immer erneut gelesen. Wiederverwendet wird nur aus einer vorherigen Sicherung, die auf dieselbe Weise und mit demselben Schlüssel verschlüsselt ist.

Da unveränderte Daten weder gelesen noch gesendet werden, sinkt auch der Verkehr zu einem entfernten oder externen PBS auf kaum mehr als das, was sich tatsächlich geändert hat.

**GUI:** Bearbeiten Sie einen Sicherungssatz, Reiter **Ziel**, **Änderungserkennung**. Das gilt nur für Ordner-Sätze: Maschinensicherungen (ganzer Datenträger) sind Datenträgerabbilder und nicht betroffen, sodass Bare-Metal- und Proxmox-VE-Wiederherstellungen genau wie bisher funktionieren. Einmalige Sicherungen verwenden Klassisch. Nur erfolgreich abgeschlossene Läufe zählen für das nächste vollständige Lesen; die Liste der Sicherungssätze zeigt „Nächstes vollständiges Lesen in N Läufen“. Die Fortschrittskarte zeigt, wann und warum ein Lauf jede Datei liest. **Berichte** zeigt den Modus jedes Laufs und wie viele unveränderte Dateien wiederverwendet wurden.

**Eine Sicherungs-ID pro Satz.** Der Modus Metadaten vergleicht mit dem neuesten Snapshot der Sicherungsgruppe. Verwenden zwei Sätze (oder zwei Rechner) dieselbe Sicherungs-ID auf demselben Server, vergleicht jeder Lauf mit dem Snapshot des anderen und liest alles. Der Editor für Sicherungssätze warnt davor.

**CLI:** `-change-detection-mode legacy|data|metadata`, oder `"change-detection-mode"` in der JSON-Konfiguration. Eine CLI-Option „alle N Läufe“ gibt es nicht: Planen Sie stattdessen gelegentlich einen Lauf mit `-change-detection-mode data`.

**Bevor Sie einen Satz auf Daten oder Metadaten umstellen,** aktualisieren Sie jeden Rechner, von dem aus Sie wiederherstellen, auf v0.6.0 oder neuer: Versionen vor der Unterstützung geteilter Archive können diese nicht lesen (sie zeigen die Ordner ohne ihre Dateien). Der offizielle `proxmox-backup-client` stellt sie byte-identisch wieder her (getestet mit 3.4.9), und der Dateibrowser von PBS 4.2 öffnet sie. Die erste geteilte Sicherung eines Ordners teilt wenig mit seinen älteren Klassisch-Sicherungen und kostet daher ungefähr eine weitere vollständige Kopie dieses Ordners im Datastore, bis die alten Snapshots bereinigt sind.

## 🚀 Schnellstart (GUI)

1. Installieren: das MSI unter Windows, das `.deb` oder `.rpm` unter Linux, oder das zip bzw. tar.gz aus den [Releases](https://github.com/mjb-is/proxmoxbackupclient_go/releases) entpacken (siehe [Download und Installation](#-download-und-installation))
2. Starten. Unter Windows **Als Administrator ausführen** für VSS, Maschinensicherungen und das Ändern von Sicherungssätzen. Unter Linux funktionieren Ordnersicherungen Ihrer eigenen Dateien als Sie selbst; verwenden Sie **Proxmox Backup Client (administrator)** (oder starten Sie die App als root) für Maschinensicherungen und Block-Snapshots
3. Den PBS-Server unter **Einstellungen > Kontoinformationen** hinzufügen und mit **Test** prüfen
4. Einen Sicherungssatz (oder eine einmalige Sicherung) anlegen und die zu sichernden Ordner oder Datenträger auswählen
5. Ausführen. Damit Zeitpläne ohne angemeldeten Benutzer laufen, **Einstellungen > Erweitert > Als Dienst ausführen** einschalten

Das [GUI-Handbuch](docs/manual/GUI.md) führt durch jeden Schritt.

## Vollständige Live-Sicherung des Rechners

`proxmoxbackup-machine` (und die Maschinensicherungen der GUI) sichern ein komplettes Windows-10/11- oder Windows-Server-System oder einen Linux-Rechner, ohne jede Ausfallzeit.

Die Befehlssyntax ist größtenteils dieselbe wie bei Ordnersicherungen, nur dass `-backupdir` durch `-backupdev` ersetzt wird (für mehrere Datenträger wiederholen).
Ein Aufruf könnte zum Beispiel so aussehen:

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Der obige Befehl betrachtet Datenträger 0, erkennt alle eingebundenen Partitionen, erstellt von ihnen einen VSS-Snapshot und erzeugt dann ein bootfähiges Sicherungsabbild des ganzen Datenträgers als fidx. Freier Speicher sowie die Auslagerungs- und Ruhezustandsdateien gehen ungelesen als Nullen in das Abbild ein (unter Linux auch freier Speicher von ext2/3/4 und XFS); `-readfreespace` liest stattdessen alles, für eine forensische Kopie. Ein Sektor, der nicht gelesen werden kann, geht als Nullen in das Abbild ein, und der Lauf listet auf, wo. Fügen Sie `-type vm` hinzu, um die Sicherung als VM-Sicherung für Proxmox VE zu speichern ([P2V](#-physische-rechner-als-virtuelle-maschinen-p2v)).

Die nächste Sicherung ist inkrementell; das Hashing wurde parallelisiert, sodass sich Geschwindigkeiten von 1 GB/s leicht erreichen lassen.

### Voraussetzungen für Linux-Maschinensicherungen (VSS-Entsprechung)

Die Maschinensicherung unter Windows verwendet VSS (daher das „Als Administrator ausführen“ oben); Linux hat kein VSS, daher verwendet dieses
Projekt für dieselbe Aufgabe das Kernelmodul `elastio-snap` (ein Fork von `dattobd`): einen
konsistenten Snapshot eines aktiven Blockgeräts zu einem bestimmten Zeitpunkt, während weiter darauf geschrieben wird. Das ursprüngliche
Repository `elastio/elastio-snap` ist archiviert; die gepflegte Version ist
[`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap), und die sollten Sie installieren. `dattobd`
selbst wird ebenfalls unterstützt, wenn es bereits geladen ist.

Beide folgenden Bedingungen sind erforderlich, unabhängig voneinander, sonst verweigert eine Linux-Maschinensicherung den Start:

1. **Das Kernelmodul muss installiert und geladen sein.** Die dokumentierte Installation von `elastio-snap` über ein Repository-Paket
   (siehe [dessen INSTALL.md](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md)) verweist derzeit
   mindestens für Ubuntu 22.04 auf eine verschobene oder kaputte URL (bestätigt am 2026-09-24: Das Repository-Paket
   liefert nach einer Weiterleitung 404). Das Bauen aus dem Quellcode funktioniert dagegen zuverlässig:
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
   Damit wird das Modul nur für den aktuellen Bootvorgang gebaut und geladen; wie es bei jedem Start automatisch geladen wird,
   steht in der Dokumentation Ihrer Distribution (z. B. ein Eintrag in `/etc/modules-load.d/`), falls Sie das möchten.

2. **Der Sicherungsprozess selbst muss als root laufen (EUID 0).** Auch bei geladenem Modul weigert sich `CreateVSSSnapshot`
   (der Snapshot-Pfad unter Linux, trotz des auf Windows bezogenen Namens) ausdrücklich, als Nicht-root-Benutzer zu laufen:
   Ein konsistenter Snapshot des ganzen Datenträgers braucht die nur für root zugängliche Steuerschnittstelle des Moduls, daher ist das eine harte
   Anforderung, die eine Mitgliedschaft in der Gruppe `disk` oder Ähnliches nicht ersetzen kann. Verwenden Sie für eine Maschinensicherung
   den Menüeintrag **Proxmox Backup Client (administrator)**, den Hintergrunddienst, oder starten Sie die GUI oder
   `proxmoxbackup-machine` mit `sudo` (oder als root); eine einfache Datei- oder Ordnersicherung hat diese Anforderung nicht.

Schlägt eine Maschinensicherung mit `backup device /dev/sdX: open /dev/sdX: permission denied` fehl, genügt es meist,
Ihren Benutzer für den grundlegenden Lesezugriff in die Gruppe `disk` aufzunehmen (`sudo usermod -aG disk $USER`, danach ab-
und wieder anmelden), aber das allein reicht NICHT für einen echten Snapshot; Sie stoßen dann unmittelbar danach trotzdem
auf die oben beschriebene root-Anforderung.

### Dateien aus einer Datenträgersicherung wiederherstellen

Die Seite Wiederherstellung der GUI öffnet eine Maschinensicherung wie eine Ordnersicherung: Jedes Volume im Abbild (NTFS, FAT, exFAT, ext2/3/4 oder XFS) ist ein Ordner, den Sie durchsuchen und aus dem Sie wiederherstellen können. Es wird nichts eingebunden.

Unter Linux können Sie eine Datenträgersicherung mit dem nbd-Werkzeug auch als Blockgerät einbinden.

Um nbd zu verwenden, führen Sie zuerst `modprobe nbd max_part=0` aus.

Aus unbekannten Gründen führt `max_part != 0` zu einer endlosen Schleife beim Erkennen der Partitionen.

Das NBD-Werkzeug bindet jede Sicherung eines festen Datenträgers ein, egal ob es eine VM- oder Host-Sicherung ist (es funktioniert also auch mit PVE-Sicherungen).

Verwenden Sie dazu eine Befehlszeile wie diese:
`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Wenn Sie `-path` weglassen, erscheint eine Textoberfläche, in der Sie die fidx-Datei auswählen können.

> Verwenden Sie das nicht auf einem Rechner, auf dem etwas Wichtiges läuft: Ein beschädigtes Dateisystem kann das Betriebssystem abstürzen lassen, weshalb Proxmox VE dafür eine QEMU-Instanz verwendet.

Hängen Sie alles auf der nbd-Platte aus, bevor Sie `proxmoxbackup-nbd` beenden, sonst bleibt die Partition wahrscheinlich belegt und lässt sich nicht mehr aushängen.

Erhalten Sie den Fehler `Device or resource busy`, müssen Sie die Trennung mit `nbd-client -d /dev/nbd0` erzwingen oder einfach neu starten.

### Wiederherstellung auf einen physischen Rechner oder eine VM

Die ISO für die Bare-Metal-Wiederherstellung hat einen vollautomatischen Booteintrag **„PBS Bare Metal Restore“**: siehe [ISO für die Bare-Metal-Wiederherstellung](#-iso-für-die-bare-metal-wiederherstellung) unten. Er ist der erste Eintrag, der nach Ablauf der Wartezeit standardmäßig startet, und überspringt die Sprach- und Tastaturabfragen von Clonezilla: Geben Sie Ihre PBS-Verbindungsdaten ein, wählen Sie einen Snapshot, bestätigen Sie den erkannten Zieldatenträger, und er erledigt die Netzwerkeinrichtung, das Einbinden per NBD, die eigentliche Wiederherstellung und einen automatischen Neustart ohne weitere Eingaben. Dieselbe ISO stellt für [P2V](#-physische-rechner-als-virtuelle-maschinen-p2v) in eine leere virtuelle Maschine wieder her.

Der ältere, halbmanuelle Weg ist ebenfalls noch auf derselben ISO enthalten, als eigener Eintrag im normalen Modusauswahlmenü von Clonezilla („pbs-nbd“): Er führt Sie durch die PBS-Verbindungsdaten und bindet den gewählten Snapshot im Hintergrund an `/dev/nbd0` an, überlässt Ihnen dann aber die eigentliche Wiederherstellung von Hand mit dem standardmäßigen Datenträger-Wiederherstellungsassistenten von Clonezilla.

## Verwendung: Ordnersicherung

Die vollständige Referenz, mit Beispielen für Windows und Linux, Exit-Codes und Zeitplanung, steht im **[Kommandozeilen-Handbuch](docs/manual/CLI.md)**. Ein typischer Befehl:

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

Die Umgebungsvariablen `PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD` und `PBS_FINGERPRINT` des offiziellen Clients werden ebenfalls gelesen.

Für die JSON-Konfiguration liegt ein JSON-Beispiel bei; füllen Sie nur die benötigten Felder aus.

Hinweis zu E-Mail-Vorlagen:
Für Betreff und Text der E-Mails wird die [Template-Engine von Go](https://pkg.go.dev/text/template) verwendet; die Syntax finden Sie in deren Dokumentation.
Folgende Variablen stehen in den Vorlagen zur Verfügung:

- `.NewChunks`: Anzahl der neu erstellten Chunks
- `.ReusedChunks`: Anzahl der wiederverwendeten Chunks
- `.Datastore`: Name des Datastores
- `.Error`: der Fehler, falls vorhanden (`.ErrorStr` liefert ihn als Text, leer, wenn es keinen gibt)
- `.ReadErrors`: die Dateien oder Ordner, die nicht gelesen werden konnten; `.ReadErrorCount` gibt ihre Anzahl an
- `.Hostname`: Hostname des Rechners
- `.StartTime`: Startzeit der Sicherung
- `.EndTime`: Endzeit der Sicherung
- `.Duration`: Dauer der Sicherung
- `.FromattedDuration`: formatierte Dauer der Sicherung (der Name wird im Code so geschrieben)
- `.Success`: wahr, wenn die Sicherung ohne Fehler und ohne Unlesbares abgeschlossen wurde
- `.Partial`: wahr, wenn ein Snapshot gespeichert wurde, aber einige Dateien nicht gelesen werden konnten
- `.Status`: `Success`, `Partial` oder `Failed`

## Stream-Sicherung

Damit lässt sich statt eines pxar ein Datenstrom sichern, was endlose Möglichkeiten eröffnet. Sie können zum Beispiel Folgendes aufrufen:

```
mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]
```

So lässt sich buzhash für die Deduplizierung nutzen, auch wenn Sie zum Beispiel tar verwenden, oder für den SQL-Dump selbst.

## Bekannte Probleme

Ein aktiver Malwareschutz von Windows Defender verlangsamt die Sicherung auf bis zu 25 % der erreichbaren Geschwindigkeit.

Unter Windows setzen die Kommandozeilenwerkzeuge eine Prozesssperre: Eine zweite Sicherung, die gestartet wird, während eine andere läuft, gibt „Another backup job is running“ aus und beendet sich. Unter Linux gibt es keine solche Sperre, planen Sie also keine zwei CLI-Läufe zur selben Zeit.

## 📋 Voraussetzungen

- Windows 10/11 oder Windows Server (64 Bit), mit Administratorrechten für VSS und Maschinensicherungen
- oder Linux (die Pakete sind für Debian, Ubuntu, Mint und Fedora), mit der WebKitGTK-Laufzeit für die GUI, sowie root und dem Kernelmodul elastio-snap oder dattobd für konsistente Maschinensicherungen
- Netzwerkzugriff auf einen Proxmox Backup Server
- Für P2V: Proxmox VE mit dem PBS-Datastore als Speicher hinzugefügt, oder ein beliebiger Hypervisor, der die Wiederherstellungs-ISO starten kann

## 🔨 Aus dem Quellcode bauen

**[BUILD.md](BUILD.md)** enthält die vollständige Anleitung: die GUI und den Hintergrunddienst, die Kommandozeilenwerkzeuge, das MSI und die Linux-Pakete. Kurz gefasst, mit Go 1.25 oder neuer und der Wails CLI v2.13:

```bash
cd gui
wails build                                    # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe   # the background service
```

```bash
packaging/debian/build-deb.sh     # .deb
packaging/fedora/build-rpm.sh     # .rpm (in Docker)
```

```powershell
installer\wix\build-msi.ps1       # MSI (WiX 3), after building the app and the service
```

Das gebaute Frontend (`gui/frontend/dist`) liegt im Repository, Node.js wird also nur benötigt, um das Frontend zu ändern (`npm install` und `npm run build` in `gui/frontend`, oder `wails dev` für Hot Reload). Tests: `make test`.

## 🔧 Fortgeschrittene Nutzung und Anleitungen

### Multi-PBS (mehrere PBS-Server)

Richten Sie mehrere PBS-Server ein und wählen Sie das Ziel pro Sicherungssatz oder einmaliger Sicherung (z. B. `C:\Users` täglich auf einen schnellen SSD-PBS, `D:\` wöchentlich auf einen PBS für große Datenmengen, dazu ein Server für die Notfallwiederherstellung).

- **[Benutzerhandbuch](MULTI_PBS_USER_GUIDE.md)**: Server hinzufügen und testen, der Standardserver, Auswahl eines Servers pro Sicherung, FAQ und Fehlerbehebung.
- **[Implementierungsleitfaden](MULTI_PBS_GUIDE.md)**: Datenmodell, automatische Migration aus einer Konfiguration mit einem einzigen PBS, Backend-Methoden.

Eine alte Konfiguration mit einem einzigen PBS wird beim ersten Laden automatisch auf einen Server `default` migriert.

### 💿 ISO für die Bare-Metal-Wiederherstellung

Die Wiederherstellungs-ISO ist eine unveränderte Clonezilla-Live-ISO, gepatcht mit den Binärdateien `pbsnbd` / `machinebackup` und zwei Menüeinträgen. Sie startet von CD, von einem USB-Stick (mit `dd` geschrieben oder von einem Ventoy-Stick), über PXE und auf UEFI- oder BIOS-Rechnern:

- **`ocs-pbs-bare-metal-restore`**: der erste Booteintrag, der nach Ablauf der Wartezeit standardmäßig startet, **„Proxmox Backup Client Go - PBS Bare Metal Restore“**. Er überspringt die Sprach- und Tastaturabfragen von Clonezilla und dessen allgemeinen Assistenten und geht direkt von den PBS-Zugangsdaten zu einer abgeschlossenen Wiederherstellung mit automatischem Neustart: zuerst DHCP (eine Abfrage für eine statische IP nur, wenn das fehlschlägt), NTP-Synchronisierung, ein Verschlüsselungsschlüssel von einem USB-Stick, falls die Sicherung verschlüsselt ist, und eine Erkennung des Zieldatenträgers, die nie den Stick anbietet, von dem gestartet wurde. Bei Erfolg zeigt er eine Abschlussmeldung und startet neu. Er stellt Windows- und Linux-Sicherungen gleichermaßen wieder her (die Wiederherstellung erfolgt auf Blockebene), auf echte Hardware oder in eine leere VM.
- **`ocs-pbs-nbd`**: ein Eintrag **„pbs-nbd“** im Modusauswahlmenü von Clonezilla. Er fragt Ihre PBS-Verbindungsdaten und den wiederherzustellenden Snapshot ab, bindet ihn im Hintergrund an `/dev/nbd0` an und übergibt dann an den normalen Datenträger-Wiederherstellungsassistenten von Clonezilla, den Sie von Hand bedienen.

Ebenfalls auf der Wiederherstellungsseite behoben: ein Deadlock von `zstd.NewReader` im Abrufpfad für PBS-Chunks, der Wiederherstellungen ganzer Rechner hängen ließ (an das Ursprungsprojekt beigetragen als [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85)), ein Fehler „der erste Wiederherstellungsdurchgang scheitert mit 'no partition', ein erneuter Lauf gelingt“, verursacht durch eine Race Condition zwischen Clonezillas eigener Funktion `is_disk_without_part_and_fs()` und udev, und BIOS/MBR-Rechner, die nach einer Wiederherstellung nicht starteten (die Daten zwischen dem MBR und der ersten Partition werden jetzt ebenfalls wiederhergestellt).

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

Alle Details (warum ein vollständiger Neuaufbau der ISO statt eines Austauschs an Ort und Stelle, Voraussetzungen, Menüablauf, Überprüfung) stehen in **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**; die Verwendung im **[Handbuch zur Bare-Metal-Wiederherstellung](docs/manual/BMR.md)**.

### Die Windows-GUI bauen

**Nativ unter Windows**: Siehe **[BUILD.md](BUILD.md)** für die Toolchain (Go, MinGW, Wails) und die Befehle für die App, den Dienst und das MSI.

**Docker (beim Bauen unter Linux).** `./build_gui_windows_docker.sh` baut die Windows-App als `ProxmoxBackupClientGO.exe`, mit korrekter WebView2-Unterstützung, in einem Wegwerf-Container `golang` (installiert mingw und Wails, baut das Frontend, führt `wails build` aus). Den Hintergrunddienst und das MSI baut es nicht.

### Funktionsstatus, Changelog und interne Dokumentation

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**: was fertig, teilweise fertig oder nicht fertig ist, und seit welcher Version.
- **[CHANGELOG.md](CHANGELOG.md)**: Änderungsverlauf pro Version, und [Unreleased] für das, was auf master ist.
- **[TODO.md](TODO.md)**: offene Roadmap und Ideen.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**: was jedes Release enthält.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**: der Deinstallationsdialog des MSI (Konfiguration behalten oder löschen) und sein Testplan.
- **[AGENTS.md](AGENTS.md)**: Aufbau und Konventionen des Repositorys, für Mitwirkende und Coding-Agenten.

## 🔁 Beiträge zum Ursprungsprojekt

Fehlerbehebungen und Funktionen von allgemeinem Nutzen werden [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) als Pull Requests angeboten. Achtzehn wurden übernommen:

  - [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85): Fix zstd decoder goroutine deadlock in `GetChunkData` (hung full-machine restores)
  - [PR #86](https://github.com/tizbac/proxmoxbackupclient_go/pull/86): Fully automated PBS Bare Metal Restore boot entry for the Clonezilla ISO
  - [PR #87](https://github.com/tizbac/proxmoxbackupclient_go/pull/87): Linux snapshots: drive elastio-snap/dattobd through ioctl instead of `elioctl`/`dbdctl`
  - [PR #88](https://github.com/tizbac/proxmoxbackupclient_go/pull/88): Restore: re-apply captured NTFS ACLs (Windows) and POSIX ACLs and xattrs (Linux)
  - [PR #90](https://github.com/tizbac/proxmoxbackupclient_go/pull/90): `machinebackuplib`: make `vm` backups restorable and bootable on Proxmox VE
  - [PR #91](https://github.com/tizbac/proxmoxbackupclient_go/pull/91): GUI: back machines up as `host`, not `vm`, by default
  - [PR #92](https://github.com/tizbac/proxmoxbackupclient_go/pull/92): `patch-clonezilla`: skip patches whose change is already in the base ISO
  - [PR #93](https://github.com/tizbac/proxmoxbackupclient_go/pull/93): `patch-clonezilla`: add the PBS bare-metal restore entry to `isolinux.cfg`
  - [PR #94](https://github.com/tizbac/proxmoxbackupclient_go/pull/94): Restore the data between the MBR and the first partition in the BMR flow (BIOS/MBR machines boot after restore)
  - [PR #95](https://github.com/tizbac/proxmoxbackupclient_go/pull/95): Fix reading passphrase-protected key files made by `proxmox-backup-client`
  - [PR #96](https://github.com/tizbac/proxmoxbackupclient_go/pull/96): `directorybackup`: back up several directories in one run
  - [PR #97](https://github.com/tizbac/proxmoxbackupclient_go/pull/97): Docs: point elastio-snap at the maintained Axcient fork
  - [PR #98](https://github.com/tizbac/proxmoxbackupclient_go/pull/98): `pbscommon`: skip the key file permission check on Windows
  - [PR #99](https://github.com/tizbac/proxmoxbackupclient_go/pull/99): Restore encrypted backups from the Clonezilla BMR ISO
  - [PR #102](https://github.com/tizbac/proxmoxbackupclient_go/pull/102): `snapshot`: delete our VSS shadow copies after every backup, through the VSS API
  - [PR #103](https://github.com/tizbac/proxmoxbackupclient_go/pull/103): `machinebackup`: skip hashing blank (all-zero) chunks
  - [PR #104](https://github.com/tizbac/proxmoxbackupclient_go/pull/104): CLIs: report errors on stderr, never in a message box
  - [PR #105](https://github.com/tizbac/proxmoxbackupclient_go/pull/105): Restore: put back every file time, also 1970 and earlier, and folder times

## 🧪 Status, Fehlerberichte und Funktionswünsche

**Dies ist Alpha-Software, bereitgestellt wie besehen, ohne jegliche Garantie oder Gewährleistung, und die Nutzung erfolgt vollständig auf eigenes Risiko.** Ich übernehme keine Verantwortung für verlorene oder beschädigte Daten, einschließlich der Quelldateien.

Sie ist für mein eigenes Homelab gebaut. Jedes Release teste ich selbst, bevor es veröffentlicht wird, und dort ist sie regelmäßig im Einsatz: geplante Ordner- und Maschinensicherungen von Windows- und Linux-Rechnern, wobei nebenbei Wiederherstellungen, Gelöschte Dateien, Zurücksetzen, Bare-Metal-Wiederherstellungen und P2V-Wiederherstellungen getestet werden. Keine Sicherung ist bewiesen, solange Sie nicht aus ihr wiederhergestellt haben, also testen Sie Ihre eigenen Wiederherstellungen, einschließlich einer Bare-Metal-Wiederherstellung, bevor Sie sich darauf verlassen.

- **Fehlerberichte sind willkommen**: Eröffnen Sie ein [Issue](https://github.com/mjb-is/proxmoxbackupclient_go/issues) mit dem, was Sie getan haben, was passiert ist, und dem Protokoll (Meldungsprotokoll oder die Dateien im Einstellungsordner).
- **Funktionswünsche**: Ich füge Funktionen hinzu, die meiner eigenen Nutzung helfen, daher nehme ich keine Wünsche für andere Anwendungsfälle an. Bitte richten Sie diese an das Ursprungsprojekt, [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go/issues). Pull Requests, die Fehler beheben oder eindeutig auch meiner Nutzung helfen würden, sind willkommen.

## 🙏 Danksagungen und Geschichte

Der ursprüngliche Kommandozeilen-Client wurde ab 2023 von Tiziano Bacocco (tizbac) geschrieben. Später wurde der grafische Client von RDEM Systems darin aufgenommen, und mit ihm die erste GUI, die Multi-PBS-Unterstützung und der Windows-Dienst. Dieser Fork begann im September 2026, um die Lücken zu schließen, die PBS daran hinderten, mein einziges Sicherungssystem zu sein; seine Releases beginnen mit v0.3.0, und Fehlerbehebungen von allgemeinem Nutzen gehen an das Ursprungsprojekt zurück.

- Der ursprüngliche Client, [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go), von Tiziano Bacocco (tizbac) und seinen Mitwirkenden.
- Der Arbeitsablauf und das Bediengefühl von Backup for Workgroups (BFW), an denen sich Sicherungssätze, Gelöschte Dateien und Zurücksetzen orientieren.
- [Clonezilla Live](https://clonezilla.org/), auf dem die ISO für die Bare-Metal-Wiederherstellung aufbaut.

## 📄 Lizenz

GPLv3: siehe die Datei [LICENSE](LICENSE).

---

**Änderungen seit dem Fork © 2026 mjb-is, aufbauend auf der Arbeit von Tiziano Bacocco (tizbac), den Mitwirkenden von Proxmox Backup Client GO und RDEM Systems (© 2024-2026).**
