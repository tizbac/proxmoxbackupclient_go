# Proxmox Backup Client: client Windows și Linux pentru Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

> ℹ️ Aceasta este o traducere a [README-ului în engleză](README.md). Acolo unde cele două diferă, versiunea în engleză este cea actuală.

> ⚠️ **Notă privind mărcile comerciale și afilierea.** Acesta este un **proiect comunitar independent**, nu un produs oficial Proxmox: **nu este afiliat cu Proxmox Server Solutions GmbH, nu este aprobat și nici sponsorizat de aceasta**. **Proxmox®** este o marcă înregistrată a Proxmox Server Solutions GmbH în UE, în SUA și în alte țări și este folosită aici **numai** pentru a descrie compatibilitatea. Conform propriilor îndrumări Proxmox privind mărcile (mărcile înregistrate nu trebuie folosite în numele produselor sau aplicațiilor, de exemplu „XY App for Proxmox Virtual Environment” în loc de „Proxmox XY App”), **redenumirea clientului fără marcă este în curs**: produsul numit în prezent *Proxmox Backup Client* trece la un nume care nu conține marca Proxmox. Regulile exacte se găsesc în [kitul media Proxmox](https://www.proxmox.com/en/about/company-details/media-kit).

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/tizbac/proxmoxbackupclient_go)](https://github.com/tizbac/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client este un client de backup open-source (GPL-3.0) pentru Proxmox Backup Server (PBS), pentru Windows și Linux.** Face backup folderelor ca arhive de fișiere care pot fi răsfoite și mașinilor întregi ca imagini de disc bootabile, după o programare sau la cerere. Restaurează orice, de la un singur fișier la o mașină întreagă: pe același computer, pe un computer înlocuitor sau într-o mașină virtuală Proxmox VE.

Este o **suită de instrumente**:

- **Proxmox Backup Client GUI** (creat de RDEM Systems ca Nimbus Backup GUI, refăcut și extins de mjb-is): aplicația pentru Windows și Linux. Seturi de backup cu mai multe programări fiecare, backup-uri de foldere și de discuri întregi cu instantanee consecvente (VSS pe Windows, elastio-snap/dattobd pe Linux), restaurarea fișierelor din backup-urile de foldere și de mașini, Recuperare fișiere șterse și Revenire, mai multe servere PBS, criptare pe partea clientului și un serviciu de fundal, astfel încât programările rulează fără ca cineva să fie autentificat.
- **`proxmoxbackup-directory`**: backup-uri de foldere (pxar) din linia de comandă, cu deduplicare și detectarea modificărilor.
- **`proxmoxbackup-machine`**: backup-uri live de discuri întregi (fidx) din linia de comandă, pentru mașini Windows și Linux. Imaginea este bootabilă după restaurare.
- **`proxmoxbackup-nbd`**: atașează un backup de disc ca dispozitiv bloc NBD (Linux).
- **Un ISO de restaurare bare-metal** bazat pe Clonezilla Live, cu o restaurare ghidată de la pornire până la repornire.

> Cuvinte cheie: proxmox backup client windows · client PBS · backup Windows cu VSS · backup-uri offsite imuabile · fizic în virtual (P2V) · restaurare bare-metal · interfață pentru Proxmox Backup Server.

<a id="-new-in-20"></a>

## 💡 De ce acest client

`proxmox-backup-client` oficial este un instrument în linie de comandă pentru Linux. Acest client lucrează cu același server și aceleași formate de backup și adaugă:

- **Windows.** Nu există un client oficial pentru Windows. Aici există instrumente în linie de comandă și o aplicație, backup-uri de foldere prin instantanee VSS și backup-uri live ale discurilor întregi.
- **O aplicație pentru Windows și Linux**: seturi de backup programate, rulate de un serviciu de fundal fără să fie nevoie ca cineva să fie autentificat, restaurare cu căutare, Recuperare fișiere șterse și Revenire, rapoarte și e-mail.
- **Mașini întregi**: imagini de disc bootabile, fișiere individuale din ele și un ISO de restaurare bare-metal ghidată, pe același computer, pe hardware nou sau într-o mașină virtuală Proxmox VE.
- **Detaliile fișierelor Windows păstrate**: permisiunile NTFS, atributele, fluxurile de date alternative, atributele extinse și orele de creare sunt înregistrate cu fiecare backup de folder, iar restaurarea acestui client le pune la loc.
- **Fără blocare**: ce salvează acest client se restaurează cu clientul oficial, iar ce salvează clientul oficial se restaurează cu acesta (vedeți mai jos).

## 🆕 Noutăți în 2.0

Versiunea 2.0 aduce în proiect un volum mare de lucru la interfața grafică, la serviciul de fundal și la motorul de backup (vedeți [CHANGELOG](CHANGELOG.md)). Față de 1.2:

- **Aplicația se conectează la serviciul de fundal cu drepturi de administrator.** Cu serviciul instalat, serverele PBS și seturile de backup ale computerului aparțin serviciului și doar serviciul citește folderul său de setări. Aplicația cere o singură dată drepturi de administrator pentru a se conecta (UAC pe Windows, pkexec pe Linux; pe un cont standard, numele și parola unui administrator). Doar un mic program ajutător rulează cu drepturi ridicate, iar aplicația păstrează tokenul serviciului numai în memorie. Un utilizator care nu poate răspunde la solicitare poate alege **Folosește setările mele** și își păstrează propriile servere și seturi de backup, care rulează cât timp aplicația este deschisă; **Conectare** oferă ulterior copierea lor în cele ale computerului. Aplicația însăși rulează fără drepturi de administrator și oferă **Repornește ca administrator** acolo unde o operațiune are nevoie de ele. Vedeți secțiunea 15 din [manualul interfeței grafice](docs/manual/GUI.md#connecting-to-the-background-service).
- **Aplicația verifică faptul că vorbește cu serviciul.** Înainte de a trimite ceva către API-ul local al serviciului, aplicația verifică cine răspunde: propriul proces al serviciului pe Windows, un socket deținut de root pe Linux. Un program care ocupă portul cât timp serviciul este oprit nu primește nimic.
- **Seturi de backup cu mai multe programări fiecare**, de la câteva minute la o dată pe an (ultima zi a lunii, a 2-a marți), fiecare descrisă în cuvinte simple, cu următoarele 3 rulări.
- **Backup-uri incrementale de foldere** cu detectarea modificărilor din clientul oficial (modurile **Date** și **Metadate**): fișierele nemodificate nu mai sunt citite din nou.
- **Backup-uri de mașină mai rapide**, care omit spațiul liber, fișierul de paginare și fișierul de hibernare și scriu sectoarele ilizibile ca zerouri în loc să eșueze.
- **Mai multe backup-uri simultan** (**Număr maxim de backup-uri simultane**, de la 1 la 8, implicit 3), inclusiv în serviciul de fundal, fiecare cu propriul instantaneu, propriul jurnal, **Pauză** și **Oprește**.
- **Limite de viteză** pentru încărcarea în PBS, descărcarea din PBS și citirea de pe disc, fiecare fiind un buget comun pentru tot ce rulează în același timp.
- **Recuperare fișiere șterse și Revenire** pentru seturile de backup de foldere și de mașini, în interfața grafică și din linia de comandă.
- **Fișiere recuperate din backup-urile de mașini**: răsfoiți volume NTFS, FAT, exFAT, ext2/3/4 și XFS dintr-o imagine de disc și restaurați fișiere individuale.
- **Chei de criptare pe care le puteți păstra**: salvați o copie, tipăriți o cheie pe hârtie cu cod QR și importați o cheie dintr-un fișier, de la cameră, dintr-o fotografie sau din text tastat.
- **Seturile de backup de mașini își fixează discurile după identitate**, astfel încât un disc care primește alt număr după o schimbare de hardware rămâne cel salvat, iar un disc lipsă face rularea să eșueze în loc să fie salvat alt disc.
- **Partițiile pot fi excluse dintr-un backup de mașină**: bifați partițiile de salvat sub fiecare disc. O partiție exclusă nu este citită și nu i se face instantaneu, iar în imagine intră ca zerouri, astfel încât o restaurare o readuce la locul ei și la dimensiunea ei, goală și neformatată. Selectorul de discuri marchează partițiile de care are nevoie un disc restaurat pentru a porni.
- **Backup-uri de mașini Linux cu elastio-snap sau dattobd.** Un set de mașină Linux poate dezactiva instantaneele și poate citi discurile live, dar numai dacă proprietarul bifează că acceptă că backup-ul poate fi inconsecvent.
- **Fiecare utilizator își poate alege propria temă de culori** în aplicația fără marcă. O aplicație cu marcă păstrează culorile mărcii sale.
- **Activarea și dezactivarea unui set de backup** din editorul său, nu doar din meniul său, și un avertisment la salvare atunci când un set nu ar rula așa cum a fost salvat (dezactivat, nicio programare activă, o problemă cu instantaneul, un disc lipsă).
- **Pachete Linux** (`.deb`, `.rpm` și Arch), cu numele de pachet și de serviciu `proxmoxbackupclient`, împreună cu serviciul de fundal.
- **Un ISO ghidat de restaurare bare-metal**, care restaurează și într-o mașină virtuală goală (P2V). O mașină salvată ca mașină virtuală Proxmox VE poate fi restaurată și direct din PBS în Proxmox VE, ca orice VM al cărei backup l-a făcut chiar Proxmox VE.
- **Versiuni cu marcă**: aceleași fișiere binare sunt livrate ca programe de instalare Proxmox Backup Client, Nimbus Backup, Acme Backup și Etitech Backup.
- **18 limbi ale interfeței.**

## ⭐ Ce face, cu rezultate măsurate

| | Rezultat |
|---|---|
| **Backup-uri incrementale de foldere** | Un folder partajat de 480,9 GB cu 761.866 fișiere: prima rulare 4 h 17 min, fiecare rulare ulterioară între 10 și 15 minute. Fișierele nemodificate nici măcar nu sunt deschise (detectarea modificărilor **Metadate**, ca în clientul oficial). |
| **Backup-uri de mașină rapide** | Spațiul liber, fișierul de paginare și fișierul de hibernare nu sunt citite, iar spațiul gol de pe disc nu este trecut prin hash. Un disc de laptop de 1 TB cu 373 GB ocupați dura înainte între 1 h 23 min și 1 h 44 min, iar acum durează 38 de minute. |
| **Conexiuni lente și cu trafic contorizat** | Se trimit doar datele noi, comprimate, iar aplicația arată ce a trecut cu adevărat prin rețea. Un mic PC Linux dintr-o locație la distanță, pe o conexiune 4G (4,8 Mbit/s la încărcare): întreaga imagine a discului de 128 GB plus folderul personal în 11 minute, cu aproximativ 172 MiB transferați. |
| **Fișiere recuperate din backup-urile de mașini** | Unitatea C: de 127 GB a unui laptop, cu 430.000 de fișiere, se listează în 4 până la 15 secunde. Nimic nu este montat sau descărcat în prealabil. |
| **Restaurare bare-metal** | Porniți de pe ISO (stick USB, CD sau rețea), introduceți datele PBS, alegeți backup-ul: o mașină Windows de 32 GB era din nou la ecranul de autentificare la 8 minute după pornire. |
| **Recuperare fișiere șterse** | După ștergerea a 39 GB dintr-un folder partajat cu 761.000 de fișiere, o căutare **Recuperare fișiere șterse** în backup-urile din ultimele 7 zile a găsit exact acele fișiere în aproximativ un minut și le-a pus la loc. |
| **Fidelitatea restaurării** | Permisiuni NTFS, atribute, fluxuri de date alternative, atribute extinse, ore de creare și ore ale folderelor pe Windows; moduri, proprietari, ACL-uri, atribute extinse și legături simbolice pe Linux. |

## 📖 Manuale de utilizare

| Manual | Conținut |
|---|---|
| **[Manualul interfeței grafice](docs/manual/GUI.md)** | Instalare, servere PBS, seturi de backup și programări, backup-uri incrementale, progres, restaurare, Recuperare fișiere șterse, Revenire, Rapoarte, jurnale, Preferințe, serviciul de fundal, depanare |
| **[Manualul liniei de comandă](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: fiecare opțiune, fișiere de configurare, coduri de ieșire, programare; Recuperare fișiere șterse și Revenire din linia de comandă |
| **[Manualul de restaurare bare-metal](docs/manual/BMR.md)** | ISO-ul de restaurare, pas cu pas; backup-uri `vm` în Proxmox VE; fizic în virtual; hardware diferit |
| [Scenarii de restaurare](docs/RESTORE_GUIDE.md) | Ce restaurare să folosiți și când |
| [Indexul manualelor](docs/manual/README.md) | O prezentare generală a manualelor |

Aceleași manuale sunt integrate în aplicație (**Ajutor**, F1), deci funcționează offline și corespund versiunii instalate.

<a id="-download"></a>

## 📦 Descărcare

👉 **[Descărcați cea mai recentă versiune](https://github.com/tizbac/proxmoxbackupclient_go/releases)**

| Aveți | Folosiți |
|---|---|
| Windows | **Programul de instalare MSI**, `ProxmoxBackupClient-<version>.msi`. Instalează aplicația și serviciul de fundal în Program Files, pornește serviciul și adaugă o intrare în meniul Start. Programele de instalare cu marcă (`NimbusBackup-<version>.msi`, `AcmeBackup-<version>.msi`, `EtitechBackup-<version>.msi`) sunt aceeași versiune sub altă marcă |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-*.rpm` |
| Arch | Pachetul construit din [packaging/arch/PKGBUILD](packaging/arch/PKGBUILD) (`make pkgarch`) |
| Doar linia de comandă | Arhivele pentru fiecare platformă (`proxmoxbackup-cli-*` și `proxmoxbackupclient_go_*`) |
| O mașină de restaurat | ISO-ul de restaurare bare-metal ([manualul de restaurare bare-metal](docs/manual/BMR.md#3-get-the-iso-and-make-the-boot-media)) sau construiți-l singur ([mai jos](#-bare-metal-restore-iso)) |

Când o versiune nu include un pachet Linux, construiți-l cu `make deb`, `make pkgfedora` sau `make pkgarch` ([BUILD.md](BUILD.md), [packaging/debian/README.md](packaging/debian/README.md)).

Pachetele Linux instalează o singură intrare de meniu, **Proxmox Backup Client**, care pornește aplicația cu utilizatorul dvs. O instalare nouă activează serviciul de fundal (`proxmoxbackupclient.service`); o actualizare păstrează alegerea făcută. Pachetele au nevoie de polkit (pkexec), pe care aplicația îl folosește pentru a se conecta la serviciu.

> ⚠️ **Windows afișează „virus detectat” (de ex. `Trojan:Win32/Sabsik.FL.A!ml`) sau un avertisment SmartScreen?**
> Este un **fals pozitiv cunoscut** pentru aplicațiile Go/Wails: *nu* este un virus. Sufixul `!ml` indică o detecție a unui model de învățare automată, care semnalează fișierele executabile *nesemnate și neobișnuite*. Verificați descărcarea așa cum se arată mai jos.

### 🔎 Verificarea oricărei descărcări

Fiecare versiune oferă sume de control SHA-256 (`SHA256SUMS.txt`, afișate și pe pagina versiunii) și o **atestare de proveniență semnată**: dovada criptografică faptului că fișierul a fost produs de CI-ul acestui depozit, dintr-un commit precis.

```powershell
Get-FileHash .\ProxmoxBackupClient-<version>.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
gh attestation verify .\ProxmoxBackupClient-<version>.msi --repo tizbac/proxmoxbackupclient_go
```

Pe Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`.

Când programele de instalare ale unei versiuni trec curat de scanarea VirusTotal, pagina versiunii include linkuri către rapoarte.

> ℹ️ **Semnarea codului:** fișierele binare Windows **nu sunt încă semnate Authenticode** (un certificat OSS prin [SignPath Foundation](https://signpath.org) este în așteptare). Între timp, proveniența este stabilită prin atestarea și sumele de control de mai sus.

## 🚀 Pornire rapidă (interfața grafică)

1. Instalați-l: MSI-ul pe Windows, pachetul `.deb`, `.rpm` sau Arch pe Linux ([Descărcare](#-download)).
2. Porniți **Proxmox Backup Client**. Programele de instalare configurează serviciul de fundal, așa că aplicația cere o singură dată drepturi de administrator pentru a se conecta la el (UAC pe Windows, parola dvs. prin pkexec pe Linux). Dacă nu le puteți da, alegeți **Folosește setările mele**.
3. Adăugați serverul PBS în **Preferințe > Informații despre cont** și apăsați **Testează**.
4. Creați un set de backup (sau un backup unic) și selectați folderele sau discurile de salvat.
5. Rulați-l. Cu serviciul de fundal, programările sale rulează indiferent dacă cineva este autentificat sau nu.

[Manualul interfeței grafice](docs/manual/GUI.md) descrie fiecare pas în detaliu.

### 🪟 Instalare nesupravegheată / silențioasă (MSI)

MSI-ul respectă comutatorul standard de instalare silențioasă al Windows Installer, plus proprietăți publice pentru fiecare valoare implicită:

```powershell
# Silent install with the defaults (service: Automatic + started)
msiexec /i ProxmoxBackupClient-<version>.msi /quiet

# Silent install that installs the service but does NOT enable/autostart it
# (created as Manual and left stopped)
msiexec /i ProxmoxBackupClient-<version>.msi /quiet SERVICEAUTOSTART=0

# Silent install that also applies a settings file exported from another computer
msiexec /i ProxmoxBackupClient-<version>.msi /quiet CONFIGFILE="C:\Deploy\settings.json"

# Custom directory + verbose install log
msiexec /i ProxmoxBackupClient-<version>.msi /quiet INSTALLFOLDER="D:\Apps\ProxmoxBackupClient" /L*v "C:\Temp\install.log"

# Silent uninstall (KEEP_CONFIG=0 also purges %ProgramData%\<name>)
msiexec /x ProxmoxBackupClient-<version>.msi /quiet KEEP_CONFIG=0
```

| Proprietate | Implicit | Semnificație |
| --- | --- | --- |
| `SERVICEAUTOSTART` | `1` | `0` = serviciul este instalat ca **Manual** și lăsat **oprit**: instalat, dar niciodată activat sau pornit automat |
| `AUTOSTART` | `1` | `0` = aplicația nu pornește în zona de notificare la autentificarea fiecărui utilizator |
| `DESKTOPSHORTCUTS` | `1` | `0` = fără scurtătură pe desktop (intrarea din meniul Start este adăugată întotdeauna) |
| `CONFIGFILE` | | Calea completă a unui fișier salvat cu **Preferințe > Exportă setările...**: serverele PBS și seturile de backup din el sunt îmbinate înainte de pornirea serviciului. ID-urile de backup formate din numele computerului care a exportat sunt formate din numele fiecărui computer. Vedeți [installer/DEPLOYMENT.md](installer/DEPLOYMENT.md) |
| `KEEP_CONFIG` | `1` | Doar la dezinstalare: `0` = șterge și directorul de stare `%ProgramData%\<name>` |
| `INSTALLFOLDER` | `Program Files` | Directorul de instalare (poate fi modificat și din interfața de instalare) |

Rulați cu drepturi ridicate (administrator); codul de ieșire `0` = succes, `3010` = succes + este necesară repornirea. MSI-urile cu marcă acceptă exact aceleași comutatoare.

## ✨ Funcționalități

### Interfața grafică
Funcționalitățile principale sunt enumerate în [Noutăți în 2.0](#-new-in-20). În plus:
- 18 limbi ale interfeței: franceză, engleză, italiană, germană, poloneză, spaniolă, bulgară, cehă, greacă, maghiară, letonă, lituaniană, neerlandeză, portugheză, română, slovacă, turcă și ucraineană
- **Backup-uri unice** ale folderelor sau ale discurilor întregi și backup-uri de mașini stocate ca `host` sau ca backup `vm` Proxmox VE, cu opțiunea **Copie criminalistică**, care citește și spațiul liber
- **Restaurare** cu căutare de fișiere (metacaractere, expresii regulate, căi complete), restaurare selectivă, ACL-uri NTFS/POSIX, fluxuri de date alternative, orele de creare Windows și o verificare opțională
- **Pauză, Reia și Oprește** pentru un backup în curs, inclusiv din zona de notificare pe Windows și pentru rulările făcute de serviciul de fundal
- Card de progres în timp real (estimarea finalizării, fișiere procesate, viteza de încărcare, date transferate, raport de compresie), **Rapoarte**, **Jurnal de mesaje** și **Vezi jurnalele complete**
- Notificări prin e-mail, acțiuni după backup, clonarea seturilor de backup, exportul și importul setărilor
- Mai multe servere PBS, cu fixarea amprentei certificatului (încredere la prima utilizare), autentificare cu token API sau cu utilizator/parolă
- Cinci teme de culori și o culoare personalizată, alese de fiecare utilizator (în aplicația fără marcă; o aplicație cu marcă păstrează culorile mărcii sale)

### Instrumente în linia de comandă
- `proxmoxbackup-directory`: backup-uri de foldere (pxar) cu deduplicare și `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: backup-uri live ale discurilor întregi pentru mașini Windows (VSS) și Linux (elastio-snap/dattobd) (fidx, incrementale, spațiul liber omis dacă nu se folosește `-readfreespace`, `-type vm` pentru un backup Proxmox VE, `-skippartition <disk>:<number>` pentru a exclude o partiție)
- `proxmoxbackup-nbd`: server NBD pentru restaurarea backup-urilor de disc (Linux)
- `ProxmoxBackupClientSVC` (Windows) și `proxmoxbackupclient` (pachetele Linux): `-list-sets`, `-undelete` și `-rollback` pentru seturile de backup definite în interfața grafică

Primele trei acceptă `-keyfile` și `-keyfile-passphrase` pentru instantanee criptate. Referința completă: [manualul liniei de comandă](docs/manual/CLI.md).

### 📸 Capturi de ecran

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Seturi de backup**: lucrări cu nume, cu etichete de programare și de tip, și **Execută acum** pe fiecare

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Progres backup**: debit, estimarea finalizării, blocuri noi și reutilizate

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restaurare**: alegeți un server PBS și un ID de backup, listați instantaneele sau căutați în ele după numele fișierului

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**Programări**: mai multe pentru fiecare set, cu următoarele 3 rulări

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Backup ca mașină virtuală Proxmox VE**: ID-ul de backup din PBS este afișat pe măsură ce tastați

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Rapoarte**: istoricul complet al rulărilor, cu un panou de detalii

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-machine-partitions.png" width="280">](docs/screenshots/gui-machine-partitions.png)
**Seturi de mașină**: bifați sub fiecare disc partițiile de salvat

</td>
<td width="33%">

[<img src="docs/screenshots/gui-themes.png" width="280">](docs/screenshots/gui-themes.png)
**Teme**: șase teme de culori, alese de fiecare utilizator

</td>
<td width="33%">

[<img src="docs/screenshots/gui-languages.png" width="280">](docs/screenshots/gui-languages.png)
**Limbi**: 18, aici franceză, germană, italiană, spaniolă, poloneză și ucraineană

</td>
</tr>
</table>

### Excluderi inteligente de sistem (backup-uri de foldere)
Backup-urile de foldere omit singure următoarele:

**Foldere de sistem:** `System Volume Information` (spațiul de stocare VSS, poate depăși 100 GB) și `$RECYCLE.BIN` oriunde s-ar afla; `Recovery` doar la rădăcina unei unități; `.pbs-rollback` (folderul de siguranță al funcției Revenire).
**Fișiere de sistem:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` și `DumpStack.log.tmp`.

**De ce contează:** o unitate poate afișa 1,03 TB ocupați, în timp ce fișierele reale au ~141 GB. Fără excluderi, backup-ul ar include instantaneele VSS (spațiu și timp irosite); cu ele, dimensiunea corespunde datelor reale.

**Recomandare:** folosiți un backup de **foldere** pentru backup-uri la nivel de fișier; folosiți un backup de **mașină** într-un set de backup separat pentru restaurarea bare-metal și P2V (o imagine a întregului disc, în care spațiul liber și fișierele de paginare și de hibernare intră ca zerouri).

### Securitate și calitate
- Folderul de setări al serviciului este accesibil doar pentru SYSTEM și Administrators (Windows) sau doar pentru root (Linux); API-ul său local ascultă pe 127.0.0.1 și cere un token, înlocuit la fiecare pornire și la fiecare 24 de ore
- Aplicația nu trimite nimic către acest API până nu verifică faptul că răspunde chiar serviciul (procesul serviciului pe Windows, un socket deținut de root pe Linux)
- Pe Windows, tokenurile și parolele PBS din `config.json` sunt criptate pentru computerul respectiv (DPAPI)
- Validarea datelor de intrare, curățarea datelor de autentificare, prevenirea traversării căilor (path traversal), reîncercări cu temporizare exponențială
- Teste unitare în fiecare modul, plus teste live opționale și teste end-to-end cu un PBS real

## 🔐 Criptare pe partea clientului

Instantaneele pot fi criptate înainte de a părăsi mașina, folosind un fișier cheie Proxmox Backup Server (formatul pe care îl scrie `proxmox-backup-client key create`, deci cheile sunt interschimbabile cu clientul oficial). Fiecare bloc este criptat cu AES-256-GCM, iar manifestul este semnat. Serverul PBS nu vede niciodată cheia.

**Interfața grafică:** deschideți **Preferințe > Informații despre cont**, editați un server și folosiți fila sa **Criptare**. Cu **Importă...** aduceți o cheie pe care o aveți deja (o copie a fișierului cheie sau o cheie pe hârtie citită cu camera, dintr-o scanare sau fotografie, ori tastată), iar cu **Generează...** creați una nouă. Fila afișează amprenta cheii. **Salvați o copie…** și **Tipăriți o cheie pe hârtie…** (cu cod QR) păstrează o copie departe de computer. Rapoartele arată dacă fiecare backup a fost criptat. **Fără fișierul cheie, un instantaneu nu poate fi restaurat: păstrați o copie într-un loc sigur, departe de această mașină.**

**Chei protejate cu frază de acces (scrypt sau PBKDF2):** **Întreabă-mă** păstrează fraza de acces în memorie pe durata unei sesiuni a aplicației; **Ține minte pe acest calculator** o stochează ca pe tokenul PBS și este alegerea potrivită pentru programări și pentru serviciul de fundal.

**Linia de comandă:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` (sau `PBS_ENCRYPTION_PASSWORD`) este necesar doar pentru o cheie protejată și este cerut interactiv când lipsește. `proxmoxbackup-machine` și `proxmoxbackup-nbd` acceptă aceleași opțiuni. `proxmoxbackup-nbd` verifică cheia pe instantaneu înainte de a atinge dispozitivul NBD, astfel încât o cheie greșită eșuează imediat.

**Restaurare bare-metal:** ISO-ul de restaurare cere cheia (niciuna, un fișier cheie pe un stick USB sau o cale tastată) și fraza de acces a unei chei protejate. Cheia nu este scrisă niciodată pe ISO.

Restaurarea unui instantaneu criptat necesită același fișier cheie; nu există nicio cale de recuperare dacă acesta se pierde.

## Backup-uri de foldere mai rapide: detectarea modificărilor

În modul implicit **Clasic** (Legacy), fiecare backup de foldere citește fiecare fișier, deci o rulare durează cât citirea întregului folder chiar și atunci când nu s-a schimbat nimic. Seturile de backup și `proxmoxbackup-directory` pot folosi în schimb modurile de detectare a modificărilor ale clientului oficial `proxmox-backup-client`:

| Mod | Citește | Stocat ca |
|---|---|---|
| **Clasic** (Legacy, implicit) | fiecare fișier | un `<name>.pxar.didx` pentru fiecare folder |
| **Date** (Data) | fiecare fișier | o arhivă divizată: `<name>.mpxar.didx` (metadate) + `<name>.ppxar.didx` (conținutul fișierelor) |
| **Metadate** (Metadata) | doar fișierele noi și modificate | aceeași arhivă divizată; fișierele a căror dimensiune, oră de modificare (la nanosecundă), mod și proprietar coincid cu backup-ul anterior sunt preluate din acesta fără a fi citite |

În modul **Metadate**, prima rulare citește totul; rulările ulterioare citesc doar ce s-a modificat. Un fișier modificat astfel încât își păstrează dimensiunea și ora de modificare nu este observat, motiv pentru care un set poate și **Recitește toate fișierele la fiecare** N rulări. Dați fiecărui set de backup propriul ID de backup: modul **Metadate** compară cu cel mai nou instantaneu din grupul de backup. În linia de comandă folosiți `-change-detection-mode`.

**Înainte de a trece un set la Date sau Metadate,** actualizați la 2.0 fiecare mașină de pe care faceți restaurări: versiunile anterioare afișează folderele unei arhive divizate fără fișierele lor. Vedeți [secțiunea 7 din manualul interfeței grafice](docs/manual/GUI.md#7-incremental-folder-backups-change-detection).

<a id="-physical-machines-into-virtual-machines-p2v"></a>

## 🔄 Mașini fizice în mașini virtuale (P2V)

Un backup de mașină al unui PC sau server fizic poate deveni o mașină virtuală Proxmox VE, în două moduri:

1. **Restaurați-l în Proxmox VE.** Setați setul de backup (sau backup-ul unic) pe **Backup ca: Mașină virtuală Proxmox VE**. Backup-ul este stocat ca `vm/<number>`, așa că Proxmox VE îl listează împreună cu celelalte backup-uri de VM și îl restaurează ca VM (un singur disc SATA; adăugați o placă de rețea înainte de a o porni).
2. **Restaurați-l cu ISO-ul de restaurare într-o VM goală** cu același firmware ca mașina originală, un disc SATA cel puțin la fel de mare și o placă de rețea Intel E1000. Acest lucru funcționează la fel pentru backup-urile `host` și `vm`, iar același backup se poate restaura în continuare pe bare metal.

Rutele testate includ Windows 10 și Windows Server (UEFI/GPT), desktopuri Linux (UEFI/GPT și BIOS/MBR) și un backup Linux criptat, cu cheia pe un stick USB. [Manualul de restaurare bare-metal](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines) conține tabelul complet, opțiunile de firmware, driverele și instrucțiuni pas cu pas.

## 🤝 Compatibil cu clientul oficial Proxmox, în ambele direcții

Verificat cu `proxmox-backup-client` 3.4.9 și PBS 4.2:

- **Instantaneele Legacy, Data și Metadata create de acest client se restaurează identic, byte cu byte, cu clientul oficial**, atât din interfața grafică, cât și din linia de comandă, create pe Windows sau pe Linux, iar browserul de fișiere din PBS 4.2 le deschide.
- **Instantaneele Legacy, Data și Metadata create de clientul oficial se restaurează identic, byte cu byte, cu acest client**, integral sau selectiv.
- Același format de arhivă divizată (pxar versiunea 2), aceleași moduri de detectare a modificărilor, același format al fișierului cheie de criptare, aceleași variabile de mediu de tip `PBS_REPOSITORY`.

Backup-urile dvs. nu sunt niciodată legate de acest client. Două elemente suplimentare însoțesc arhivele standard, iar clientul oficial restaurează fișierele normal și fără ele:

- `.proxmox_backup_client_meta.json`, la rădăcina unui instantaneu de folder, înregistrează calea originală. Clientul oficial îl restaurează ca pe un fișier obișnuit.
- `proxmox-client-acls.json.gz.blob`, salvat cu fiecare instantaneu de folder, înregistrează ce nu încape în arhivă pe Windows (permisiuni NTFS, atribute, ore de creare, fluxuri de date alternative și atribute extinse) și, pe Linux, atributele extinse și ACL-urile POSIX. Clientul oficial și interfața web PBS îl ignoră; restaurarea acestui client le pune la loc.

Pe Linux, clientul oficial păstrează ACL-urile POSIX, atributele extinse și capabilitățile fișierelor în arhiva însăși. Acest client nu scrie și nu citește încă aceste intrări, așa că pe Linux ACL-urile nu trec de la un client la altul în niciun sens. Realizarea acestui lucru în modul oficial, ca opțiune, este în [planul de dezvoltare](TODO.md).

## Backup live al întregii mașini

`proxmoxbackup-machine` (și backup-urile de mașină din interfața grafică) salvează un sistem complet Windows 10/11 sau Windows Server, ori o mașină Linux, fără nicio întrerupere a funcționării.

Sintaxa comenzii este în mare parte aceeași ca pentru backup-urile de foldere, cu excepția faptului că `-backupdir` este înlocuit de `-backupdev` (repetați-l pentru mai multe discuri). De exemplu:

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Comanda de mai sus examinează Discul 0, detectează toate partițiile montate, le face un instantaneu VSS și apoi creează o imagine bootabilă a întregului disc ca fidx. Spațiul liber și fișierele de paginare și de hibernare intră în imagine ca zerouri, fără a fi citite (pe Linux, și spațiul liber ext2/3/4 și XFS); `-readfreespace` citește în schimb totul, pentru o copie criminalistică. Un sector care nu poate fi citit intră în imagine ca zerouri, iar rularea indică unde. Adăugați `-type vm` (cu un `-backup-id` numeric) pentru a stoca backup-ul ca backup de VM Proxmox VE ([P2V](#-physical-machines-into-virtual-machines-p2v)).

Următorul backup este incremental. Calculul hash-urilor este paralelizat, astfel încât se pot atinge viteze de 1 GB/s.

Pentru a exclude o partiție, de exemplu o bibliotecă de jocuri sau o partiție de date deja acoperită de un backup de foldere, adăugați `-skippartition <disk>:<number>` (repetați pentru mai multe), cum ar fi `-skippartition \\.\PhysicalDrive0:4` sau `-skippartition /dev/sda:3`. Partiția nu este citită și nu i se face instantaneu, iar în imagine intră ca zerouri: tabela de partiții este păstrată, deci o restaurare readuce partiția la locul ei și la dimensiunea ei, goală și neformatată. În interfața grafică, debifați partiția sub discul ei în setul de backup.

### Cerințe pentru backup-ul mașinilor Linux (echivalentul VSS)

Backup-ul mașinilor Windows folosește VSS. Linux nu are VSS, așa că backup-ul de mașină al unui sistem Linux în funcțiune face un instantaneu la un moment dat al fiecărei partiții montate cu ajutorul unui modul de kernel: `elastio-snap` (fork-ul întreținut, [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap)) sau `dattobd`.

Sunt necesare două lucruri, altfel un backup de mașină Linux refuză să ruleze:

1. **Modulul de kernel trebuie să fie instalat și încărcat.** Instalați-l prin DKMS, ca să fie recompilat după fiecare actualizare de kernel, și configurați-l să se încarce la fiecare pornire. Secțiunea [Instantanee Linux: instalarea elastio-snap sau dattobd](docs/manual/GUI.md#linux-snapshots-installing-elastio-snap-or-dattobd) din manualul interfeței grafice conține pași testați pentru Debian, Ubuntu, Mint și Fedora, plus Secure Boot și verificări. În noiembrie 2026, niciunul dintre cele două module nu se compilează pe kernelul 7 (Fedora 44 după actualizări).

2. **Backup-ul trebuie să ruleze ca root.** Interfața de control a modulului este accesibilă doar pentru root, iar instantaneul este refuzat unui utilizator obișnuit. Serviciul de fundal rulează ca root; fără el, folosiți **Repornește ca administrator** în aplicație sau rulați `proxmoxbackup-machine` cu `sudo`. Un simplu backup de foldere nu are nevoie de root.

În interfața grafică, un set de backup de mașină Linux poate debifa **Folosește instantanee** și își poate citi discurile live, dar numai cu a doua casetă bifată („Rulează fără instantaneu: accept că backupul poate fi inconsecvent”). Fără ea, fiecare rulare se oprește și spune de ce. Partițiile care nu sunt montate sunt citite direct.

### Restaurarea fișierelor dintr-un backup de disc

Pagina **Restaurare** a interfeței grafice deschide un backup de mașină la fel ca pe un backup de foldere: fiecare volum din imagine (NTFS, FAT, exFAT, ext2/3/4 sau XFS) este un folder pe care îl puteți răsfoi și din care puteți restaura. Nimic nu este montat.

Pe Linux puteți de asemenea atașa un backup de disc ca dispozitiv bloc cu instrumentul nbd. Rulați mai întâi `modprobe nbd max_part=0`: cu `max_part != 0`, kernelul poate intra într-o buclă infinită sondând partițiile.

Instrumentul NBD conectează orice backup de disc fix, fie că este un backup de VM sau de host (deci funcționează și pentru backup-urile Proxmox VE). De exemplu:

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Dacă omiteți `-path`, o interfață în terminal vă permite să alegeți fișierul fidx; `-list` afișează în schimb toate imaginile.

> Nu folosiți acest lucru pe o mașină care rulează ceva important: un sistem de fișiere corupt poate bloca sistemul de operare, motiv pentru care Proxmox VE folosește o instanță QEMU pentru asta.

Demontați totul de pe discul nbd înainte de a opri `proxmoxbackup-nbd`, altfel partiția va rămâne probabil ocupată. Dacă primiți eroarea `Device or resource busy`, forțați deconectarea cu `nbd-client -d /dev/nbd0` sau reporniți.

### Restaurare pe o mașină fizică sau într-o VM

Folosiți [ISO-ul de restaurare bare-metal](#-bare-metal-restore-iso), pe bare metal sau într-o mașină virtuală goală. [Manualul de restaurare bare-metal](docs/manual/BMR.md) îl descrie pas cu pas.

## Utilizare: backup de director

Referința completă, cu exemple pentru Windows și Linux, coduri de ieșire și programare, se află în **[manualul liniei de comandă](docs/manual/CLI.md)**. O comandă tipică:

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
  -backupdir string          Source directory, must not be a symlink; repeat for several
  -change-detection-mode     legacy (default), data or metadata
  -novss                     Do not snapshot the source (VSS on Windows, elastio-snap/dattobd on Linux)
  -keyfile string            PBS encryption key file
  -keyfile-passphrase string Passphrase for a protected key (asked for when omitted)
  -no-mtime                  Record 1970-01-01 as every file and folder time (see below)
  -backupstream string       Back up standard input as <name>.didx instead of a directory
  -pxarout string            Also write the archive to a local file (debugging)
  -mail-host, -mail-port, -mail-username, -mail-password, -mail-insecure,
  -mail-from, -mail-to, -mail-subject-template, -mail-body-template
                             Email report (optional)
  -config string             JSON config file; flags given on the command line override it
```

Sunt citite și variabilele de mediu `PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD` și `PBS_FINGERPRINT` ale clientului oficial.

Pentru configurarea JSON vedeți [config.json.example](config.json.example); completați doar câmpurile de care aveți nevoie.

### Mai multe directoare într-o singură rulare

Repetați `-backupdir` pentru a face backup mai multor directoare cu o singură comandă:

```shell
proxmoxbackup-directory.exe -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -datastore "datastorename" -backupdir "C:\Data" -backupdir "D:\Photos"
```

Fiecare director este salvat ca propriul grup de backup, cu ID-ul de backup `<backup-id>_<path>` (`-backup-id` are ca valoare implicită numele gazdei). Retenția, curățarea (pruning) și restaurarea tratează așadar fiecare director ca pe o serie independentă. Un director care eșuează nu le oprește pe celelalte, iar codul de ieșire raportează eșecul. `-pxarout` nu poate fi combinat cu mai multe directoare, iar două directoare care ar ajunge la același ID de backup sunt respinse înainte de a se încărca ceva.

Într-un fișier de configurare JSON folosiți `"backupdirs": ["C:\\Data", "D:\\Photos"]`. Câmpul unic `"backupdir"` funcționează în continuare și poate fi combinat cu `"backupdirs"`. Criptarea funcționează ca pentru un singur director: cheia este citită o singură dată, iar grupul de backup al fiecărui director este criptat cu ea.

### Excluderea orelor din arhivă

Ora de modificare a fiecărui fișier și folder face parte din arhivă, deci o oră care se schimbă modifică octeții arhivei, iar blocurile din jurul acelei intrări sunt încărcate din nou, deși nimic din ele nu s-a schimbat. Pentru majoritatea datelor acesta este compromisul corect. Unii arbori de foldere însă își tot ating folderele, în timp ce fișierele rămân aceleași: un depozit care își pune datele în foldere cu nume de hash (`aa/bb/<rest of the hash>`) poate ajunge să încarce din nou o mare parte a arhivei la fiecare rulare doar din acest motiv.

`-no-mtime` (`"no-mtime": true` într-un fișier de configurare JSON) înregistrează în schimb 1970-01-01 ca oră a fiecărui fișier și folder, astfel încât doar modificările reale de conținut costă blocuri noi. Orele reale nu sunt păstrate nicăieri: o restaurare datează fiecare element 1970-01-01. Cu `-change-detection-mode metadata` fiecare fișier este citit, ca în modul `data`. Opțiunea este dezactivată implicit; folosiți-o pentru o sursă la care orele nu contează.

### Șabloane de e-mail

Funcționează și un releu care nu cere autentificare (adesea pe portul 25): omiteți numele de utilizator și parola. Expeditorul poate include un nume (`Backups <backup@example.com>`).

Pentru subiectele și corpul e-mailurilor se folosește [motorul de șabloane din Go](https://pkg.go.dev/text/template); consultați documentația lui pentru sintaxă. Sunt disponibile următoarele variabile:

- `.NewChunks`: numărul de blocuri noi create
- `.ReusedChunks`: numărul de blocuri reutilizate
- `.Datastore`: numele datastore-ului
- `.Error`: eroarea, dacă există (`.ErrorStr` o oferă ca text, gol când nu există)
- `.ReadErrors`: fișierele sau folderele care nu au putut fi citite; `.ReadErrorCount` este numărul lor
- `.Hostname`: numele gazdei mașinii
- `.StartTime`: ora la care a început backup-ul
- `.EndTime`: ora la care s-a încheiat backup-ul
- `.Duration`: durata backup-ului
- `.FromattedDuration`: durata formatată a backup-ului (numele este scris astfel în cod)
- `.Success`: true când backup-ul s-a încheiat fără erori și fără nimic ilizibil
- `.Partial`: true când a fost salvat un instantaneu, dar unele fișiere nu au putut fi citite
- `.Status`: `Success`, `Partial` sau `Failed`

## Backup de flux

Acesta face backup unui flux în loc de o arhivă pxar. De exemplu:

```
mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]
```

Se folosește fragmentarea buzhash pentru deduplicarea fluxului în sine, de exemplu o arhivă tar sau un dump SQL.

## Probleme cunoscute

Când antimalware-ul Windows Defender este activ, poate încetini un backup la aproximativ 25% din viteza care se poate atinge.

Pe Windows, instrumentele din linia de comandă iau un blocaj de proces: un al doilea backup pornit în timp ce unul rulează afișează „Another backup job is running” și se încheie cu codul 2. Pe Linux nu există un astfel de blocaj, deci nu programați două rulări în linia de comandă în același timp. (Interfața grafică și serviciul de fundal rulează în siguranță mai multe backup-uri simultan, vedeți [Mai multe backup-uri simultan](docs/manual/GUI.md#67-several-backups-at-once).)

## 📋 Cerințe preliminare

- Windows 10/11 sau Windows Server (64 de biți); drepturi de administrator pentru instalare și pentru conectarea aplicației la serviciul de fundal
- sau Linux: pachetele sunt pentru Debian, Ubuntu, Mint, Fedora și Arch; interfața grafică are nevoie de runtime-ul WebKitGTK și de polkit; backup-urile de mașină consecvente au nevoie de modulul de kernel elastio-snap sau dattobd și de root
- Acces în rețea la un Proxmox Backup Server
- Pentru P2V: Proxmox VE cu datastore-ul PBS adăugat ca stocare, sau orice hipervizor care poate porni de pe ISO-ul de restaurare

## 🔨 Construire din surse

**[BUILD.md](BUILD.md)** conține instrucțiunile complete: interfața grafică și serviciul de fundal, instrumentele din linia de comandă, MSI-ul și pachetele Linux. Pe scurt, cu Go 1.25 sau mai nou și Wails CLI v2.13:

```bash
cd gui
wails build                                                       # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe    # the background service
```

```bash
make deb          # .deb
make pkgfedora    # .rpm (in Docker)
make pkgarch      # Arch package (on Arch)
make msi          # Windows MSIs on Linux (Docker + Wine + WiX), one per brand
make clonezilla   # the bare-metal restore ISO
```

Frontend-ul construit (`gui/frontend/dist`) este inclus în depozit, deci Node.js este necesar doar pentru a modifica frontend-ul (`npm install` și `npm run build` în `gui/frontend`, sau `wails dev` pentru reîncărcare la cald). Teste: `make test`.

## 🔧 Utilizare avansată și ghiduri

### Multi-PBS (mai multe servere PBS)

Configurați mai multe servere PBS și alegeți destinația pentru fiecare set de backup sau backup unic (de ex. `C:\Users` zilnic către un PBS rapid pe SSD, `D:\` săptămânal către un PBS pentru volume mari de date, plus un server de recuperare în caz de dezastru).

- **[Ghidul utilizatorului](MULTI_PBS_USER_GUIDE.md)**: adăugarea și testarea serverelor, serverul implicit, alegerea serverului pentru fiecare backup, întrebări frecvente și depanare.
- **[Ghidul de implementare](MULTI_PBS_GUIDE.md)**: modelul de date, migrarea automată dintr-o configurație cu un singur PBS, metodele backend-ului.

Configurația veche cu un singur PBS este migrată automat la un server `default` la prima încărcare.

<a id="-bare-metal-restore-iso"></a>

### 💿 ISO de restaurare bare-metal

ISO-ul de restaurare este un ISO Clonezilla Live standard, modificat cu fișierele binare `pbsnbd` / `machinebackup` plus două intrări de meniu. Pornește de pe CD, de pe un stick USB (scris cu `dd` sau de pe un stick Ventoy) și pe mașini UEFI sau BIOS:

- **`ocs-pbs-bare-metal-restore`**: prima intrare, cea implicită, din meniul de pornire, **PBS Bare Metal Restore**. Sare peste întrebările Clonezilla despre limbă și tastatură și merge direct de la datele de autentificare PBS la o restaurare finalizată: mai întâi DHCP (o solicitare de IP static doar dacă acesta eșuează), o cheie de criptare de pe un stick USB dacă backup-ul este criptat și o detectare a discului țintă care nu oferă niciodată stick-ul de pe care a pornit. Restaurează la fel de bine backup-uri Windows și Linux (restaurarea se face la nivel de bloc), pe bare metal sau într-o VM goală.
- **`ocs-pbs-nbd`**: o intrare **pbs-nbd** în meniul de selecție a modului din Clonezilla. Cere datele de conectare PBS și instantaneul, îl atașează la `/dev/nbd0`, apoi predă controlul asistentului normal de restaurare a discurilor din Clonezilla, pe care îl conduceți manual.

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

`make clonezilla` descarcă ISO-ul standard și face același lucru. Toate detaliile (de ce o reconstruire completă a ISO-ului în loc de o înlocuire pe loc, cerințe, fluxul meniului, verificare) se află în **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**; utilizarea lui, în **[manualul de restaurare bare-metal](docs/manual/BMR.md)**.

### Construirea interfeței grafice pentru Windows

**Nativ pe Windows**: vedeți **[BUILD.md](BUILD.md)** pentru lanțul de instrumente (Go și MinGW din Chocolatey, Wails) și comenzile pentru aplicație, serviciu și MSI.

**Docker (la construirea pe Linux).** `./build_gui_windows_docker.sh` construiește aplicația Windows ca `ProxmoxBackupClientGO.exe`, cu suport WebView2 corect, într-un container `golang` de unică folosință. Nu construiește serviciul de fundal și nici MSI-ul.

### Istoricul modificărilor și documentația internă

- **[CHANGELOG.md](CHANGELOG.md)**: istoricul modificărilor pentru fiecare versiune.
- **[TODO.md](TODO.md)**: foaia de parcurs deschisă și idei.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**: starea stabilă a produsului și versiunile disponibile.
- **[installer/DEPLOYMENT.md](installer/DEPLOYMENT.md)**: implementarea MSI-ului pe multe computere.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**: dialogul de dezinstalare MSI (păstrarea/ștergerea configurației) și planul său de testare.
- **[AGENTS.md](AGENTS.md)**: structura și convențiile depozitului, pentru contributori și agenți de programare.

## 🌐 Suport tehnic în Italia

Fiind stabilit că proiectul și **TOATE** contribuțiile sale vor rămâne pentru totdeauna publice și licențiate sub licența GPLv3, principalul sponsor al acestui proiect, precum și al celor mai recente funcții de backup de mașină, E.T.I. Srl ( https://etitech.net ), poate oferi, celor care au nevoie, suport pentru implementări Proxmox în general și în special pentru instrumentele de backup Windows.

Nu va exista niciodată o ediție „community” și una „enterprise” diferite; doar suportul tehnic va fi un serviciu independent.
Orice personalizare pe care o veți cere, chiar dacă dezvoltarea poate fi plătită, va fi publicată sub GPLv3, la fel ca întregul proiect.

## 🖥️ Originea interfeței grafice

**Proxmox Backup Client GUI** a fost creat de **[RDEM Systems](https://www.rdem-systems.com/)** ca **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)**: au proiectat și construit aplicația originală și oferă suport comercial pentru Nimbus Backup, versiunea ei cu marcă. **[mjb-is](https://github.com/mjb-is)** a refăcut-o apoi și a extins-o, inspirându-se din fluxul de lucru și stilul altor sisteme de backup, iar această muncă a fost integrată înapoi în acest depozit. Întregul cod, inclusiv interfața grafică și toate funcționalitățile sale, rămâne open-source sub licența GPLv3.

## ⚠️ Avertisment

Acest software este furnizat „ca atare”. Deși urmărim fiabilitatea, nu ne asumăm nicio responsabilitate pentru pierderea sau deteriorarea datelor. Testați întotdeauna backup-urile și verificați restaurarea înainte de a vă baza pe ele în producție. Niciun backup nu este dovedit până nu ați restaurat din el, așa că testați-vă propriile restaurări, inclusiv una bare-metal.

Software-ul este încă de calitate alpha și nu ne asumăm nicio responsabilitate pentru niciun fel de daune sau pierderi de date, nici măcar ale fișierelor sursă.

Rapoartele de erori sunt binevenite: deschideți un [issue](https://github.com/tizbac/proxmoxbackupclient_go/issues) cu ce ați făcut, ce s-a întâmplat și jurnalul (**Jurnal de mesaje** sau **Vezi jurnalele complete**).

## 📄 Licență

GPLv3: vedeți fișierul [LICENSE](LICENSE).

## 🏷️ Branding

Fiecare contributor care a adus **cel puțin 5 commituri** care adaugă funcționalități sau remedieri are dreptul să i se adauge datele de branding pentru uz comercial.

Singurele condiții sunt ca firma către care trimite brandingul să **nu** desfășoare niciuna dintre următoarele:

- Campanii de malware
- Afaceri care promovează războiul (se aplică oricărei țări, inclusiv celor occidentale)
- Escrocherii
- Furt de date
- Trafic de copii
- Violență
- Discriminare
- Droguri
- Orice activitate recunoscută în general ca ilegală

Dacă vreunul dintre contributori primește o plângere, vom încerca să luăm legătura; dacă nu se oferă nicio explicație validă, vom **înceta imediat** acest beneficiu.

**Licența GPLv3 rămâne în vigoare**, iar dvs. veți fi în continuare liber să faceți un fork al proiectului și să vă construiți propriile executabile.

## 🤝 Contribuiți

Contribuțiile sunt binevenite, mai ales:

1. Traduceri ale fișierelor README și ale manualelor de utilizare pentru versiunea 2.0
2. Încărcarea asincronă / pe mai multe nuclee a blocurilor (compresia pe mai multe nuclee este deja implementată pentru backup-ul de mașină)
3. Un patch pe partea Proxmox pentru a adăuga formatului pxar un alt tip de intrare, care să conțină descriptorii de securitate Windows
4. Suport pentru legăturile simbolice și joncțiunile Windows (în prezent sunt omise)
5. Testare pe mai mult hardware, mai multe distribuții Linux și kerneluri, în special restaurări bare-metal
6. Orice lucru interesant care vă trece prin minte :)

## Despre contributorii Proxmox Backup Client GO

Contributorii Proxmox Backup Client GO dezvoltă și întrețin acest proiect. Software-ul se bazează pe infrastructura NTP/NTS și pe cele [11 servere NTS publice](https://github.com/jauderho/nts-servers) enumerate în referința comunității.

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
