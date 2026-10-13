# Proxmox Backup Client: client Windows și Linux pentru Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client este un client de backup open-source (GPL-3.0) pentru Proxmox Backup Server (PBS), care funcționează pe Windows și Linux.**

Este o **suită de instrumente** pentru backup în PBS:

- **Proxmox Backup Client GUI** (bazată pe GUI-ul Nimbus Backup de la RDEM Systems): un client grafic pentru backupul serverelor și stațiilor de lucru Windows și Linux în PBS, cu instantanee coerente (VSS pe Windows, elastio-snap/dattobd pe Linux), Seturi de Backup cu planificare, backup de foldere și de discuri întregi, restaurare, Recuperare fișiere șterse și Revenire, mai multe servere PBS și un serviciu de fundal pe Windows și Linux.
- **`proxmoxbackup-directory`**: instrument în linie de comandă pentru backupul folderelor (pxar), cu deduplicare și detectarea modificărilor.
- **`proxmoxbackup-machine`**: instrument în linie de comandă pentru backupul live al discurilor întregi (fidx) pe mașini Windows (VSS) și Linux (elastio-snap/dattobd); imaginea pornește după o restaurare bare-metal.
- **`proxmoxbackup-nbd`**: atașează un backup de disc ca dispozitiv NBD pentru restaurare (Linux).
- O **imagine ISO de restaurare bare-metal** bazată pe Clonezilla Live.

> ⚠️ **Declinarea responsabilității:** acest proiect **nu este afiliat în niciun fel** cu **Proxmox Server Solutions GmbH**. „Proxmox”, sigla Proxmox și denumirile asociate aparțin proprietarilor lor; aici sunt folosite **doar** pentru a indica compatibilitatea. Vedeți [proxmox.com](https://www.proxmox.com/) pentru produsele lor.

> ℹ️ **Această pagină este un rezumat tradus.** Referința este [README.md în engleză](README.md), care conține toate detaliile. Manualele de utilizare sunt în engleză: [GUI](docs/manual/GUI.md), [linie de comandă](docs/manual/CLI.md) și [restaurare bare-metal](docs/manual/BMR.md).

## 📖 Manuale de utilizare

| Manual | Conținut |
|---|---|
| **[Manualul GUI](docs/manual/GUI.md)** | Instalare, servere PBS, Seturi de Backup și planificări, backupuri incrementale, progres, restaurare, Recuperare fișiere șterse, Revenire, Rapoarte, jurnale, Preferințe, serviciul de fundal, depanare |
| **[Manualul liniei de comandă](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: toate opțiunile, fișiere de configurare, coduri de ieșire, planificare; Recuperare și Revenire din linia de comandă |
| **[Manualul restaurării bare-metal](docs/manual/BMR.md)** | ISO-ul de restaurare pas cu pas; instantanee `vm` pe Proxmox VE; hardware diferit |
| [Scenarii de restaurare](docs/RESTORE_GUIDE.md) | Ce restaurare să folosiți și când |
| [Indexul manualelor](docs/manual/README.md) | Prezentare generală a celor trei manuale |

## 🆕 Pe master, pentru următoarea versiune

Sunt în [CHANGELOG](CHANGELOG.md) sub [Unreleased] și încă nu fac parte dintr-o versiune publicată:

- **Instalatoare**: un **MSI pentru Windows** (aplicația și serviciul în Program Files, intrare în meniul Start, actualizări pe loc, păstrarea sau ștergerea setărilor la dezinstalare) și **pachete Linux** `.deb` (Debian, Ubuntu, Mint) și `.rpm` (Fedora), cu aplicația, cele trei instrumente în linie de comandă, manualele și intrări în meniu.
- **Serviciu de fundal pe Linux** (`proxmoxbackupclient.service`): Seturile de Backup planificate rulează ca root, chiar și când nu este nimeni autentificat. Se activează din Preferințe > Avansat > Rulare ca serviciu, ca pe Windows. **Repornește ca administrator** funcționează acum și pe Linux (prin pkexec).
- **Pauză, Reia și Oprește pentru backupurile rulate de serviciu**, cu cardul lor de progres în aplicația deschisă.
- **Un avertisment când două Seturi de Backup împart un grup de backup** (același server, tip și ID backup), cu o insignă **ID comun** pe ambele carduri.
- **Setări protejate**: pe Windows `C:\ProgramData\ProxmoxBackupClient` este accesibil doar administratorilor; pe Linux folderul de setări este privat pentru proprietarul său.
- **Fișiere din backupurile de mașină pentru toate sistemele de fișiere uzuale**: FAT, exFAT, ext2/3/4 și XFS, pe lângă NTFS. Restaurările imaginilor Windows refac și permisiunile și fluxurile de date alternative.
- **Backupurile de mașină Linux sar peste spațiul liber** (ext2/3/4, XFS), ca cele Windows din v0.8.0.
- **Recuperare și Revenire pentru Seturile de Backup de mașină Linux**, și **din linia de comandă** pentru orice set: `-list-sets`, `-undelete`, `-rollback` (secțiunea 12 din [manualul liniei de comandă](docs/manual/CLI.md)).
- Instrumentele în linie de comandă scriu erorile în stderr în loc să afișeze ferestre de mesaj, iar `proxmoxbackup-machine` trimite e-mailul de notificare.

## 💽 Nou în v0.8.0: backupuri de mașină mai rapide, fișiere din ele, Pauză/Reia

- **Backupurile de mașină sar peste spațiul liber** (și peste fișierele de paginare și hibernare) pe Windows, deci un disc pe jumătate gol se citește în aproximativ jumătate din timp, iar un **sector defect** nu mai oprește backupul: intră în imagine ca zerouri, iar rularea indică unde.
- Cardul de progres arată blocurile din total, discul și partiția, fragmentele noi / reutilizate / goale și viteza de încărcare.
- **Restaurarea fișierelor dintr-un backup de mașină**: fiecare volum NTFS din imagine este un folder pe care îl puteți parcurge, căuta și bifa. Nimic nu este montat sau descărcat în prealabil; se citesc doar tabela de fișiere și apoi datele fișierelor alese (un C: de laptop de 127 GB cu 430.000 de fișiere se listează în 4 până la 15 secunde). Un singur backup de mașină per laptop acoperă atât restaurarea bare-metal, cât și „am nevoie înapoi de fișierul acela”.
- **Recuperare și Revenire** funcționează pe Seturile de Backup de mașină (Windows), pe folderele alese pentru set (implicit Desktop, Documente, Imagini, Muzică, Videoclipuri și Descărcări).
- **Pauză și Reia** pentru un backup în curs, o **insignă VSS / Instantaneu** pe fiecare card și **datele de creare** Windows salvate și restaurate.
- **v0.8.2:** spațiul gol nu mai este calculat bloc cu bloc, deci un disc aproape gol durează cam o treime din timp (un disc de test de 50 GB: de la 3m36s la 1m15s).

## 🗑️ Nou în v0.7.0: Recuperare fișiere șterse și Revenire

Două pagini noi în GUI, pentru Seturile de Backup de foldere:

- **Recuperare fișiere șterse** găsește fișierele care sunt în backupurile unui set, dar nu mai sunt pe disc (în cel mai recent backup sau în toate din ultimele 7 sau 30 de zile), le afișează ca arbore de foldere cu momentul dispariției (și „probabil mutat în ...” când același fișier există în altă parte) și restaurează fișierele bifate unde se aflau sau în alt folder. După ștergerea a 39 GB dintr-o partajare cu 761.000 de fișiere, o căutare pe 7 zile a găsit exact acele fișiere în aproximativ un minut.
- **Revenire** readuce folderele unui set la starea dintr-un backup ales. Mai întâi arată ce s-ar schimba (lipsă, modificate, adăugate între timp), apoi restaurează. Tot ce înlocuiește sau elimină este mutat întâi deoparte pe aceeași unitate, deci **Anulare** pune totul la loc.

Se citesc doar listele de fișiere ale backupurilor și listările folderelor; niciun fișier nu este deschis până la restaurare. Tot nou: dialoguri de confirmare în aplicație, Următoarea rulare pe fiecare card, un ID backup sugerat per set (`deepthought-data`), nume unice de set și serviciul de fundal inclus în descărcarea pentru Windows. Vedeți [manualul GUI](docs/manual/GUI.md#10-undelete).

## ⚡ Nou în v0.6.0: backupuri incrementale de foldere

Backupurile de foldere pot sări peste fișierele nemodificate, folosind aceeași **detectare a modificărilor** ca `proxmox-backup-client` oficial. În modul **Metadate**, clientul compară dimensiunea, data modificării (la nanosecundă), modul și proprietarul fiecărui fișier cu instantaneul anterior. Fișierele nemodificate nici nu sunt deschise: datele lor sunt reutilizate. Doar fișierele noi și modificate sunt citite și încărcate.

Măsurat pe un server de fișiere real (480,9 GB, 761.866 de fișiere pe discuri USB):

| Rulare | Durată | Citire | Fragmente noi încărcate |
|---|---|---|---|
| Prima rulare Metadate (nimic de comparat) | 4 h 17 min | toate fișierele | 135.549 |
| Rulările următoare | aproximativ 10 până la 15 min | doar fișierele modificate; toate cele 761.866 nemodificate reutilizate | 4 până la 40 |

Traficul către un PBS la distanță scade la puțin peste ce s-a modificat efectiv. Utilizare: editați un Set de Backup de foldere, fila **Destinație**, **Detectarea modificărilor: Metadate**. Setați **Recitește toate fișierele la fiecare N rulări**, astfel încât fiecare a N-a rulare să fie o citire completă (de exemplu 42 pentru un set care rulează la fiecare 4 ore, cam o dată pe săptămână). Dați fiecărui set **propriul ID backup**. Linie de comandă: `-change-detection-mode metadata`. Clasic rămâne modul implicit.

## 🤝 Compatibil cu clientul oficial Proxmox, în ambele sensuri

Verificat cu `proxmox-backup-client` 3.4.9 și PBS 4.2:

- **Instantaneele Legacy, Data și Metadata create de acest client se restaurează identic, octet cu octet, cu clientul oficial**, atât din GUI cât și din linia de comandă, create pe Windows sau Linux, iar browserul de fișiere din PBS 4.2 le deschide. Pe Linux, fișierele restaurate își păstrează și modul exact, data modificării la nanosecundă și țintele legăturilor simbolice.
- **Instantaneele Legacy, Data și Metadata create de clientul oficial se restaurează identic cu acest client**, complet sau selectiv.
- Același format de arhivă divizată (pxar versiunea 2), aceleași moduri de detectare a modificărilor, același format al fișierului cheie (cheile `proxmox-backup-client key create` funcționează aici și invers), aceleași variabile de mediu de tip `PBS_REPOSITORY`.

Backupurile dumneavoastră nu sunt legate de acest client. Singurul element în plus este un mic fișier la rădăcina unui instantaneu de folder, `.proxmox_backup_client_meta.json` (calea originală), pe care clientul oficial îl restaurează ca fișier obișnuit.

## 📦 Descărcare și instalare

👉 **[Descărcați ultima versiune](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| Aveți | Folosiți |
|---|---|
| Windows | **Instalatorul MSI** (`ProxmoxBackupClient-<version>-x64.msi`) instalează aplicația și serviciul de fundal în Program Files, cu intrare în meniul Start. Sau dezarhivați `ProxmoxBackupClient-v<version>-windows-amd64.zip` (aplicația și `ProxmoxBackupClientSVC.exe`; păstrați-le în același folder) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Alt Linux | Dezarhivați `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (necesită WebKitGTK) |
| Doar linie de comandă | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` sau `-linux-amd64.tar.gz` |

MSI-ul, `.deb` și `.rpm` sunt noi pe master: versiunile până la v0.8.1 au doar fișierele zip și tar.gz. Până când o versiune include instalatoarele, construiți-le conform [BUILD.md](BUILD.md).

Pachetele Linux instalează două intrări de meniu: **Proxmox Backup Client** rulează aplicația cu utilizatorul dumneavoastră (backupul propriilor fișiere), iar **Proxmox Backup Client (administrator)** o rulează ca root, cerând parola prin polkit, pentru backupuri de mașină și instantanee de bloc. Serviciul de fundal este instalat, dar nu este activat; vedeți [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **Windows afișează „virus detectat” (de ex. `Trojan:Win32/Sabsik.FL.A!ml`) sau un avertisment SmartScreen?** Este un **fals pozitiv cunoscut** pentru aplicațiile Go/Wails nesemnate și puțin răspândite. Detalii în [proiectul original](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Verificarea unei descărcări

Fiecare versiune a acestui fork include un `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-v0.8.1-windows-amd64.zip -Algorithm SHA256   # compare with SHA256SUMS.txt
```

Pe Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`. Compilările acestui fork nu au încă semnare de cod, atestare CI sau scanare VirusTotal (vedeți [TODO.md](TODO.md)).

## ✨ Funcționalități

### GUI: Proxmox Backup Client GUI (recomandată)
- **🌍 Multilingvă**: 18 limbi de interfață (franceză, engleză, italiană, germană, poloneză, spaniolă, bulgară, cehă, greacă, maghiară, letonă, lituaniană, neerlandeză, portugheză, română, slovacă, turcă și ucraineană)
- Bară de meniu nativă și meniu lateral (Backup, Restaurare, Recuperare fișiere șterse, Revenire, Rapoarte, Jurnal de mesaje, Preferințe, Despre)
- **Seturi de Backup**: joburi cu nume, planificare zilnică / la interval / manuală, insigne de tip și VSS / Instantaneu, „Execută acum” și avertismentul **ID comun**
- **Backup unic** de foldere sau discuri întregi; pagina **Rapoarte** cu istoricul complet și un **Jurnal de mesaje** limitat
- **Preferințe** cu cinci teme de culoare (Amber, Albastru clasic, Verde, Roșu, Întunecat) și o culoare personalizată
- **Backupuri incrementale de foldere** (Metadate), cu citire completă opțională la fiecare N rulări
- **Backupuri de mașină** pe Windows și Linux care sar peste spațiul liber și rezistă sectoarelor defecte, stocate ca `host` sau ca instantaneu `vm` Proxmox VE
- **Restaurare**: parcurgerea instantaneelor, căutare (wildcard-uri, expresii regulate, căi complete), restaurare selectivă cu ACL NTFS/POSIX, datele folderelor, datele de creare Windows și verificare opțională
- **Fișiere din backupuri de mașină** (NTFS; FAT, exFAT, ext2/3/4 și XFS pe master)
- **Recuperare fișiere șterse** și **Revenire** (din v0.7.0), pentru seturi de foldere și de mașină, cu previzualizare și Anulare
- **Pauză, Reia și Oprește** un backup în curs
- **Rulare ca serviciu** din Preferințe > Avansat, ca planificările să ruleze fără nimeni autentificat (Windows din v0.7.0; Linux pe master)
- Card de progres live, VSS pe Windows, instantanee de bloc elastio-snap/dattobd pe Linux
- Notificări prin e-mail, acțiuni după backup, clonarea seturilor, exportul și importul setărilor
- Mai multe servere PBS, cu fixarea amprentei certificatului, token API sau utilizator/parolă
- **Criptare la client** cu un fișier cheie PBS per server
- Protecție de administrator pentru modificarea Seturilor de Backup (Windows), cu Repornește ca administrator

### Instrumente în linie de comandă
- `proxmoxbackup-directory`: backup de foldere (pxar) cu deduplicare și `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: backup live al discurilor întregi pe Windows (VSS) și Linux (elastio-snap/dattobd) (fidx, incremental, spațiul liber sărit dacă nu se dă `-readfreespace`)
- `proxmoxbackup-nbd`: server NBD pentru restaurarea backupurilor de disc (Linux)
- `ProxmoxBackupClientSVC` (Windows) și `proxmoxbackupclient` (pachetele Linux): `-list-sets`, `-undelete` și `-rollback` pentru Seturile de Backup definite în GUI (pe master)

Primele trei acceptă `-keyfile` și `-keyfile-passphrase`. Referința completă: [manualul liniei de comandă](docs/manual/CLI.md).

### 📸 Capturi de ecran

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Seturi de Backup**: joburi cu nume, insigne de planificare și de tip, Execută acum pe fiecare

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Progres live**: debit, timp estimat și fragmente noi față de reutilizate

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restaurare**: alegeți serverul PBS și ID-ul backup, listați instantaneele sau căutați după numele fișierului

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**Restaurare selectivă**: bifați fișiere sau foldere din arbore și restaurați pe loc sau în altă parte

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Rapoarte**: istoric complet cu panou de detalii (durată, fragmente, foldere salvate)

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**Multi-PBS**: gestionați mai multe servere PBS, testați conexiunea, alegeți unul implicit

</td>
</tr>
</table>

Mai multe capturi (instantanee `vm`, teme, limbi) în [README-ul în engleză](README.md).

### Excluderi inteligente de sistem (backup de foldere)
Sunt omise automat: `System Volume Information` și `$RECYCLE.BIN` oriunde s-ar afla, `Recovery` doar la rădăcina unei unități (pe master), `.pbs-rollback` (folderul de siguranță al Revenirii), plus fișierele `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` și `DumpStack.log.tmp`. O unitate poate afișa 1,03 TB ocupați, deși fișierele reale au ~141 GB; fără excluderi, backupul ar include instantaneele VSS. Recomandare: backup de **foldere** pentru fișiere și backup de **mașină** într-un Set de Backup separat pentru restaurarea bare-metal.

### Securitate și calitate
- Criptare opțională la client a fiecărui fragment, cu manifest semnat
- Folderul de setări accesibil doar administratorilor pe Windows, privat pe Linux (pe master)
- API-ul local al serviciului ascultă doar pe 127.0.0.1 și cere un token reînnoit la fiecare pornire
- Validarea intrărilor, curățarea credențialelor, prevenirea path traversal, reîncercări cu temporizare exponențială
- Teste unitare în fiecare modul, plus teste live și end-to-end pe un PBS real

## 🔐 Criptare la client

Instantaneele pot fi criptate înainte de a părăsi mașina, cu un fișier cheie Proxmox Backup Server (formatul scris de `proxmox-backup-client key create`, interschimbabil cu clientul oficial). Fiecare fragment este criptat cu AES-256-GCM, iar manifestul este semnat. Serverul PBS nu vede niciodată cheia.

**GUI:** **Preferințe > Informații despre cont**, editați un server și folosiți fila **Criptare**. Alegeți un fișier cheie existent sau apăsați **Generează...**. Pentru o cheie protejată cu frază de acces alegeți **Întreabă-mă** (o dată pe sesiune; backupurile planificate și cele ale serviciului eșuează imediat fără ea) sau **Ține minte pe acest calculator** (salvată în `config.json`; varianta pentru joburi nesupravegheate și pentru serviciu). **Fără fișierul cheie, un instantaneu nu poate fi restaurat: păstrați o copie într-un loc sigur, departe de această mașină.**

**CLI:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```

**Bare-metal:** ISO-ul Clonezilla cere cheia și fraza de acces; cheia nu este scrisă niciodată în ISO. Vedeți [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md).

## Backupuri de foldere mai rapide: detectarea modificărilor

| Mod | Citește | Stocat ca |
|---|---|---|
| **Legacy** (Clasic, implicit) | toate fișierele | un `<name>.pxar.didx` per folder |
| **Data** (Date) | toate fișierele | arhivă divizată: `<name>.mpxar.didx` (metadate) + `<name>.ppxar.didx` (conținut) |
| **Metadata** (Metadate) | doar fișierele noi și modificate | aceeași arhivă divizată; fișierele cu aceeași dimensiune, dată, mod și proprietar sunt preluate din backupul anterior fără a fi citite |

Prima rulare Metadate citește tot. O modificare care păstrează dimensiunea și data nu este observată, de aceea există **Recitește toate fișierele la fiecare N rulări**. Se aplică doar seturilor de foldere: backupurile de mașină și restaurările bare-metal sau pe Proxmox VE nu sunt afectate. Backupurile unice folosesc Clasic. **Un ID backup per set:** dacă două seturi folosesc același ID pe același server, fiecare rulare se compară cu instantaneul celuilalt și citește tot. CLI: `-change-detection-mode legacy|data|metadata` (nu există opțiune „la fiecare N rulări”: planificați ocazional o rulare `data`). **Înainte de a trece un set la Date sau Metadate,** actualizați la v0.6.0 sau mai nou toate mașinile de pe care restaurați.

## 🚀 Pornire rapidă (GUI)

1. Instalați: MSI pe Windows, `.deb` sau `.rpm` pe Linux, sau dezarhivați zip-ul ori tar.gz-ul din [versiuni](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
2. Porniți aplicația. Pe Windows, **Executare ca administrator** pentru VSS, backupuri de mașină și modificarea Seturilor de Backup. Pe Linux, folosiți **Proxmox Backup Client (administrator)** (sau root) pentru backupuri de mașină și instantanee de bloc
3. Adăugați serverul PBS în **Preferințe > Informații despre cont** și apăsați **Testează**
4. Creați un Set de Backup (sau un backup unic) și alegeți folderele sau discurile
5. Rulați-l. Ca planificările să ruleze fără nimeni autentificat, activați **Preferințe > Avansat > Rulare ca serviciu**

[Manualul GUI](docs/manual/GUI.md) descrie fiecare pas.

## Backup live complet al mașinii

`proxmoxbackup-machine` (și backupurile de mașină din GUI) salvează un sistem Windows 10/11, Windows Server sau Linux complet, fără oprire. Sintaxa este ca la backupul de foldere, dar `-backupdir` este înlocuit cu `-backupdev` (repetat pentru mai multe discuri):

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Comanda detectează partițiile montate ale discului 0, face un instantaneu VSS și creează o imagine bootabilă a întregului disc ca fidx. Spațiul liber și fișierele de paginare și hibernare intră ca zerouri fără a fi citite; `-readfreespace` citește tot (copie criminalistică). Următorul backup este incremental, cu viteze de până la 1 GB/s.

### Cerințe pentru backupul de mașină pe Linux (echivalentul VSS)

Linux nu are VSS; proiectul folosește modulul de kernel `elastio-snap` (un fork al `dattobd`), în versiunea întreținută [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap). Și `dattobd` este acceptat dacă e deja încărcat. Ambele condiții sunt obligatorii:

1. **Modulul trebuie instalat și încărcat.** Pachetul din depozit documentat nu funcționează cel puțin pe Ubuntu 22.04; compilarea din sursă funcționează:
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
   Astfel se încarcă doar pentru pornirea curentă (pentru încărcare la fiecare pornire, de ex. o intrare în `/etc/modules-load.d/`).

2. **Procesul de backup trebuie să ruleze ca root (EUID 0).** Apartenența la grupul `disk` nu este suficientă. Folosiți intrarea **Proxmox Backup Client (administrator)**, serviciul de fundal sau `sudo`. Backupul de foldere nu are această cerință.

### Restaurarea fișierelor dintr-un backup de disc

Pagina Restaurare din GUI deschide un backup de mașină ca pe unul de foldere: fiecare volum este un folder pe care îl puteți parcurge (NTFS din v0.8.0; FAT, exFAT, ext2/3/4 și XFS pe master). Nimic nu este montat.

Pe Linux puteți atașa un backup de disc și ca dispozitiv de bloc cu instrumentul nbd. Rulați întâi `modprobe nbd max_part=0` (cu `max_part != 0` apare o buclă infinită de sondare a partițiilor):

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Fără `-path` apare o interfață în terminal pentru alegerea fișierului fidx. Nu îl folosiți pe o mașină cu servicii importante (un sistem de fișiere corupt poate bloca sistemul de operare) și demontați tot înainte de a opri pbsnbd. La eroarea `Device or resource busy`, rulați `nbd-client -d /dev/nbd0` sau reporniți.

### Restaurarea pe o mașină fizică

Fork-ul livrează un ISO Clonezilla Live modificat cu o intrare de boot **„PBS Bare Metal Restore”** complet automată (prima poziție, implicită la expirarea timpului): introduceți datele PBS, alegeți un instantaneu, confirmați discul țintă detectat automat, iar ea se ocupă de rețea, atașarea NBD, restaurare și repornire. Ruta semi-manuală mai veche („pbs-nbd” în meniul de moduri Clonezilla) rămâne disponibilă.

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

Detalii în **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)** și în [manualul restaurării bare-metal](docs/manual/BMR.md).

## Utilizare: backup de directoare

Referința completă este în **[manualul liniei de comandă](docs/manual/CLI.md)**. O comandă tipică:

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

Sunt citite și variabilele de mediu `PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD` și `PBS_FINGERPRINT` ale clientului oficial. Șabloanele de e-mail folosesc [motorul de șabloane Go](https://pkg.go.dev/text/template); lista variabilelor este în README-ul în engleză.

## 📋 Cerințe preliminare

- Windows 10/11 sau Windows Server (64 de biți), cu drepturi de administrator pentru VSS și backupuri de mașină
- sau Linux (pachete pentru Debian, Ubuntu, Mint și Fedora), cu WebKitGTK pentru GUI, plus root și modulul elastio-snap sau dattobd pentru backupuri de mașină coerente
- Acces în rețea la un Proxmox Backup Server

## 🔨 Compilare din sursă

**[BUILD.md](BUILD.md)** conține instrucțiunile complete (GUI, serviciu, instrumente în linie de comandă, MSI și pachete Linux). Pe scurt, cu Go 1.25 sau mai nou și Wails CLI v2.13:

```bash
cd gui
wails build                                    # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe   # the background service
```

Teste: `make test`.

## 🔧 Utilizare avansată și ghiduri

- **[MULTI_PBS_USER_GUIDE.md](MULTI_PBS_USER_GUIDE.md)** și **[MULTI_PBS_GUIDE.md](MULTI_PBS_GUIDE.md)**: mai multe servere PBS (ghid de utilizare și de implementare).
- **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**: ISO-ul de restaurare bare-metal.
- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**: ce este gata, parțial sau nerealizat.
- **[CHANGELOG.md](CHANGELOG.md)**: istoricul modificărilor pe versiuni.
- **[TODO.md](TODO.md)**: foaie de parcurs și idei.

## 🌐 Suport tehnic în Italia

Proiectul și **TOATE** contribuțiile sale vor rămâne publice sub licența GPLv3. E.T.I. Srl ( https://etitech.net ), principalul sponsor, poate oferi suport pentru implementări Proxmox și pentru instrumentele de backup Windows. Nu vor exista ediții „community” și „enterprise” separate; orice personalizare, chiar și plătită, va fi publicată sub GPLv3.

## 🖥️ Atribuirea GUI

**Proxmox Backup Client GUI** se bazează pe **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)**, dezvoltată și întreținută de **[RDEM Systems](https://www.rdem-systems.com/)**. A fost reintegrată în acest depozit și rămâne open-source sub GPLv3. RDEM Systems sponsorizează dezvoltarea GUI și oferă suport comercial.

## ⚠️ Avertisment

Acest software este oferit „ca atare” și are încă o calitate alfa. Nu ne asumăm nicio responsabilitate pentru pierderea sau deteriorarea datelor. Testați întotdeauna backupurile și verificați restaurarea înainte de a vă baza pe ele în producție.

## 📄 Licență

GPLv3: vedeți fișierul [LICENSE](LICENSE).

## 🏷️ Branding

Orice contribuitor cu **cel puțin 5 commit-uri** care adaugă funcționalități sau corecturi are dreptul să își adauge datele de branding pentru uz comercial, cu condiția ca firma respectivă să **nu** fie implicată în malware, promovarea războiului, escrocherii, furt de date, trafic de copii, violență, discriminare, droguri sau activități ilegale. La o plângere fără explicație validă, beneficiul este **retras imediat**. **Licența GPLv3 rămâne în vigoare**.

## 🤝 Contribuții

Contribuțiile sunt binevenite, mai ales: revizuirea nativă a textelor despre criptare și o gestionare a cheilor dincolo de fișierul cheie; migrarea fizic-virtual (P2V); încărcarea asincronă sau multi-core a fragmentelor; un patch Proxmox pentru descriptorii de securitate Windows în pxar; legăturile simbolice Windows; și orice idee interesantă :)

## Despre contribuitorii Proxmox Backup Client GO

Contribuitorii Proxmox Backup Client GO dezvoltă și întrețin acest proiect. Software-ul se bazează pe infrastructura NTP/NTS și pe cele [11 servere NTS publice](https://github.com/jauderho/nts-servers) din referința comunității.

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
