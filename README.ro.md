# Proxmox Backup Client: client Windows și Linux pentru Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)
[![Ko-fi](https://img.shields.io/badge/Ko--fi-buy%20me%20a%20coffee-FF5E5B?logo=ko-fi&logoColor=white)](https://ko-fi.com/mickbeeby)

> **Construit pe munca altora.** Acest fork se bazează pe [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) original al lui Tiziano Bacocco (tizbac) și pe clientul grafic Nimbus Backup al RDEM Systems, care a fost integrat în acesta și din care a pornit interfața grafică a acestei aplicații.
>
> ☕ Clientul este gratuit și va rămâne gratuit. Dacă vă economisește timp, puteți [să-mi oferiți o cafea pe Ko-fi](https://ko-fi.com/mickbeeby). Este complet opțional și nu cumpără nici asistență, nici promisiuni de funcționalități.

**Proxmox Backup Client este un client de backup open-source (GPL-3.0) pentru Proxmox Backup Server (PBS), pentru Windows și Linux.** Face backup folderelor ca arhive de fișiere care pot fi parcurse și mașinilor întregi ca imagini de disc bootabile, după planificare sau la cerere, și readuce orice, de la un singur fișier până la o mașină întreagă, pe același calculator, pe unul de înlocuire sau într-o mașină virtuală Proxmox VE.

Îl dezvolt pentru propriul meu homelab, ca PBS să fie singurul sistem de backup pentru un amestec de servere, desktopuri și laptopuri Windows și Linux, în locul Backup for Workgroups, al oglindirilor rsync și al imaginilor Clonezilla/RescueZilla. A început în septembrie 2026 ca fork al [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) și a crescut mult dincolo de acesta: peste 350 de commit-uri de atunci, cu 18 corecturi și funcționalități trimise înapoi proiectului original ([lista mai jos](#-contribuții-la-proiectul-original)).

**Îl puteți folosi așa cum este.** Este oficial **software alfa**, folosit pe propriul risc, fără nicio garanție de niciun fel. Testez personal fiecare versiune și îl folosesc în mod curent pe propriile mașini, dar citiți [Stare, raportarea erorilor și cereri de funcționalități](#-stare-raportarea-erorilor-și-cereri-de-funcționalități) înainte de a vă baza pe el.

Suita:

- **Proxmox Backup Client GUI**: aplicația pentru Windows și Linux. Seturi de Backup cu mai multe planificări fiecare, backup de foldere și de discuri întregi cu instantanee coerente (VSS pe Windows, elastio-snap/dattobd pe Linux), restaurarea fișierelor din backupuri de foldere și de mașină, Recuperare fișiere șterse și Revenire, mai multe servere PBS, criptare la client și un serviciu de fundal, ca planificările să ruleze fără nimeni autentificat.
- **`proxmoxbackup-directory`**: backup de foldere (pxar) din linia de comandă, cu deduplicare și detectarea modificărilor.
- **`proxmoxbackup-machine`**: backup live al discurilor întregi (fidx) pentru mașini Windows și Linux, din linia de comandă; imaginea este bootabilă după o restaurare.
- **`proxmoxbackup-nbd`**: atașează un backup de disc ca dispozitiv NBD (Linux).
- **Un ISO de restaurare bare-metal** construit pe Clonezilla Live, cu o restaurare ghidată de la pornire până la repornire.

> Cuvinte cheie: client proxmox backup pentru windows · client PBS · backup Windows cu VSS · fizic la virtual (P2V) · restaurare bare-metal · interfață pentru Proxmox Backup Server.

## ⭐ Ce face, cu rezultate măsurate

| | Rezultat |
|---|---|
| **Backupuri incrementale de foldere** | O partajare de server de fișiere de 480,9 GB cu 761.866 de fișiere: prima rulare 4 h 17 min, fiecare rulare ulterioară 10 până la 15 minute. Fișierele nemodificate nici nu sunt deschise (detectarea modificărilor Metadate a clientului oficial). |
| **Backupuri de mașină rapide** | Spațiul liber, fișierul de paginare și fișierul de hibernare nu sunt citite, iar spațiul gol de pe disc nu este calculat. Un disc de laptop de 1 TB cu 373 GB ocupați: înainte 1 h 23 min până la 1 h 44 min, acum 38 de minute. Un disc de test de 50 GB: 3 min 36 s, acum 1 min 15 s. |
| **Fișiere din backupurile de mașină** | Parcurgeți o imagine de disc ca pe un folder și restaurați fișiere individuale, din NTFS, FAT, exFAT, ext2/3/4 și XFS. Unitatea C: de 127 GB a unui laptop, cu 430.000 de fișiere, se listează în 4 până la 15 secunde; nimic nu este montat sau descărcat în prealabil. |
| **Mașină fizică în mașină virtuală** | Faceți backup unui PC sau server, apoi porniți-l ca VM Proxmox VE: restaurat prin Proxmox VE însuși sau cu ISO-ul de restaurare. [Vedeți mai jos](#-mașini-fizice-transformate-în-mașini-virtuale-p2v). |
| **Restaurare bare-metal** | Porniți ISO-ul (stick USB, CD sau PXE), introduceți datele PBS, alegeți backupul: o mașină Windows de 32 GB a revenit la ecranul de autentificare la 8 minute după pornire. |
| **Recuperare fișiere șterse** | După ștergerea a 39 GB dintr-o partajare cu 761.000 de fișiere, o căutare pe 7 zile a găsit exact acele fișiere în aproximativ un minut și le-a pus la loc. |
| **Revenire** | Readuce folderele unui set la starea dintr-un backup ales, cu o previzualizare înainte și Anulare după. |
| **Planificări** | Mai multe per Set de Backup, de la câteva minute până la o dată pe an: „zilnic la 02:00” plus „ultima zi a fiecărei luni” sau „a 2-a marți”, fiecare descrisă în cuvinte, cu următoarele 3 rulări. |
| **Fidelitatea restaurării** | Permisiuni NTFS, atribute, fluxuri de date alternative și atribute extinse, date de creare și datele folderelor pe Windows; moduri, proprietari, ACL-uri, atribute extinse și legături simbolice pe Linux. O trecere opțională recitește ce s-a restaurat și compară cu backupul. |
| **Funcționează cu instrumentele Proxmox** | Backupurile făcute aici se restaurează identic, octet cu octet, cu `proxmox-backup-client` oficial, și invers. [Detalii](#-compatibil-cu-clientul-oficial-proxmox-în-ambele-sensuri). |

## 🧩 Ce adaugă față de proiectul original

Dincolo de cele [18 modificări integrate în proiectul original](#-contribuții-la-proiectul-original), acest client adaugă:

**Backupuri**
- Backupuri incrementale de foldere cu detectarea modificărilor a clientului oficial (modurile Date și Metadate, arhive pxar v2 divizate), cu o citire completă la fiecare N rulări.
- Backupuri de mașină care sar peste spațiul liber, fișierul de paginare și fișierul de hibernare (NTFS pe Windows; ext2/3/4 și XFS pe Linux), scriu sectoarele ilizibile ca zerouri și continuă, și arată în timpul rulării numărul de blocuri, discul, partiția și numărul de fragmente.
- Pauză, Reia și Oprește pentru backupuri de foldere și de mașină, inclusiv pentru backupurile rulate de serviciul de fundal.
- Fluxuri de date alternative, atribute extinse NTFS și date de creare în backupurile de foldere Windows.

**Planificări și Seturi de Backup**
- Mai multe planificări per Set de Backup, de la câteva minute până la o dată pe an (a n-a zi din săptămână, ultima zi a lunii), fiecare descrisă în cuvinte, cu următoarele 3 rulări.
- Un serviciu de fundal pe Linux, ca și pe Windows, și Repornește ca administrator pe Linux.
- Execută acum rapid sau complet, o insignă VSS / Instantaneu, un avertisment **ID comun**, numele seturilor scrise în PBS ca comentarii ale instantaneelor, alerte prin e-mail, acțiuni după backup și exportul și importul setărilor.

**Restaurări**
- Fișiere individuale din backupurile de mașină (NTFS, FAT, exFAT, ext2/3/4, XFS), fără montarea sau descărcarea imaginii.
- Recuperare fișiere șterse și Revenire, pentru Seturi de Backup de foldere și de mașină, în GUI și din linia de comandă.
- O trecere opțională de verificare după restaurare, datele de creare Windows și un arbore de instantanee care încarcă câte un folder pe rând, astfel încât backupurile foarte mari se deschid imediat.
- Un ISO de restaurare bare-metal ghidat, care restaurează și într-o VM goală, și rute testate de la fizic la virtual ([P2V](#-mașini-fizice-transformate-în-mașini-virtuale-p2v)).
- Compatibilitate octet cu octet cu `proxmox-backup-client` oficial în ambele sensuri, inclusiv cu instantaneele sale Data și Metadata (arhivă divizată).

**Aplicația**
- O interfață refăcută: bară de meniu, meniu lateral, Rapoarte cu istoricul complet al backupurilor și restaurărilor, un Jurnal de mesaje, cinci teme de culoare și o culoare personalizată, notificări în zona de notificare și notificări pop-up.
- 18 limbi de interfață, toate revizuite (originalul are 5).
- Folderul de setări restricționat la administratori (Windows) sau la proprietarul său (Linux) și un token API al serviciului reînnoit la fiecare pornire.
- Pachete MSI, `.deb` și `.rpm`, plus manualele de utilizare pentru GUI, linia de comandă și restaurarea bare-metal.

## 🔄 Mașini fizice transformate în mașini virtuale (P2V)

Un backup de mașină al unui PC sau server fizic poate deveni o mașină virtuală Proxmox VE, în două moduri:

**1. Restaurați-l în Proxmox VE.** Setați Setul de Backup (sau backupul unic) la **Backup ca: Mașină virtuală Proxmox VE**. Backupul este stocat ca `vm/<număr>` cu o mică configurație de VM, astfel încât Proxmox VE îl afișează împreună cu celelalte backupuri de VM: deschideți stocarea PBS în Proxmox VE, selectați backupul, **Restore**, alegeți un ID de VM și o stocare. VM-ul primește un disc SATA și firmware-ul potrivit pentru disc (OVMF pentru UEFI/GPT, SeaBIOS pentru BIOS/MBR); adăugați o placă de rețea înainte de a-l porni și un disc EFI („Pre-Enroll keys” nebifat) dacă un VM UEFI nu pornește.

**2. Restaurați-l cu ISO-ul de restaurare bare-metal într-o VM goală.** Creați o VM cu același firmware ca mașina originală (OVMF cu un disc EFI pentru UEFI/GPT, SeaBIOS pentru BIOS/MBR), un disc SATA cel puțin la fel de mare ca originalul și o placă de rețea Intel E1000, porniți-o de pe ISO și rulați restaurarea. Funcționează la fel pentru backupurile `host` și `vm`, iar același backup se poate restaura în continuare și pe o mașină fizică.

Rute testate:

| De la | La | Rută | Rezultat |
|---|---|---|---|
| Laptop Windows 10, UEFI/GPT | VM Proxmox VE | ISO de restaurare | A pornit la desktopul utilizatorului |
| Windows Server, UEFI/GPT | VM Proxmox VE | Restaurare Proxmox VE | A pornit fără modificări |
| Desktop Xubuntu, UEFI/GPT | VM Proxmox VE | Restaurare Proxmox VE și ISO-ul de restaurare | A pornit la desktop |
| Linux, BIOS/MBR | VM Proxmox VE | ISO de restaurare și restaurare Proxmox VE | A pornit la ecranul de autentificare |
| Linux Mint, backup criptat | VM Proxmox VE | ISO de restaurare, cheie de pe un stick USB | A pornit |
| PC Linux | același PC cu un disc nou | ISO de restaurare | A pornit |

Windows configurează noul hardware virtual la prima pornire („Getting devices ready”). [Manualul restaurării bare-metal](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines) conține tabelul de firmware, driverele și instrucțiunile pas cu pas.

## 📖 Manuale de utilizare

| Manual | Conținut |
|---|---|
| **[Manualul GUI](docs/manual/GUI.md)** | Instalare, servere PBS, Seturi de Backup și planificări, backupuri incrementale, progres, restaurare, Recuperare fișiere șterse, Revenire, Rapoarte, jurnale, Preferințe, serviciul de fundal, depanare |
| **[Manualul liniei de comandă](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: toate opțiunile, fișiere de configurare, coduri de ieșire, planificare; Recuperare și Revenire din linia de comandă |
| **[Manualul restaurării bare-metal](docs/manual/BMR.md)** | ISO-ul de restaurare pas cu pas; backupuri `vm` pe Proxmox VE; fizic la virtual; hardware diferit |
| [Scenarii de restaurare](docs/RESTORE_GUIDE.md) | Ce restaurare să folosiți și când |
| [Indexul manualelor](docs/manual/README.md) | O prezentare generală a celor trei manuale |

## 🆕 Nou în v0.9.0: mai multe planificări per Set de Backup, instalatoare, un serviciu Linux

- **Mai multe planificări per Set de Backup**, de la câteva minute până la o dată pe an: „zilnic la 02:00” plus „lunar, în ultima zi” sau „a 2-a marți”, fiecare planificare descrisă în cuvinte, cu următoarele 3 rulări. Seturile existente își păstrează planificarea.
- **Backupurile de foldere păstrează fluxurile de date alternative și atributele extinse NTFS** (Windows), cum ar fi marcajul „descărcat de pe internet”, iar restaurarea le pune la loc. Lista de limitări cunoscute nu mai conține niciuna.
- **Instalatoare**: un **MSI pentru Windows** (aplicația și serviciul de fundal în Program Files, intrare în meniul Start, actualizări pe loc, păstrarea sau ștergerea setărilor la dezinstalare) și **pachete Linux**, `.deb` (Debian, Ubuntu, Mint) și `.rpm` (Fedora), cu aplicația, cele trei instrumente în linie de comandă, manualele și intrări în meniu.
- **Un serviciu de fundal pe Linux** (`proxmoxbackupclient.service`): Seturile de Backup planificate rulează ca root, chiar și când nu este nimeni autentificat. Se activează din Preferințe > Avansat > Rulare ca serviciu, ca pe Windows. **Repornește ca administrator** funcționează acum și pe Linux (prin pkexec).
- **Pauză, Reia și Oprește pentru backupurile rulate de serviciu**, cu cardul lor de progres în aplicația deschisă.
- **Un avertisment când două Seturi de Backup împart un grup de backup** (același server, tip și ID backup), cu o insignă **ID comun** pe ambele carduri.
- **Setări protejate**: pe Windows `C:\ProgramData\ProxmoxBackupClient` este acum accesibil doar administratorilor; pe Linux folderul de setări este privat pentru proprietarul său.
- **Fișiere din backupurile de mașină pentru toate sistemele de fișiere uzuale**: FAT, exFAT, ext2/3/4 și XFS, pe lângă NTFS. Restaurările imaginilor Windows refac și permisiunile și fluxurile de date alternative.
- **Backupurile de mașină Linux sar peste spațiul liber** (ext2/3/4, XFS), ca cele Windows începând cu v0.8.0.
- **Recuperare și Revenire pentru Seturile de Backup de mașină Linux** și **din linia de comandă** pentru orice Set de Backup: `-list-sets`, `-undelete`, `-rollback` (secțiunea 12 din [manualul liniei de comandă](docs/manual/CLI.md)).
- Instrumentele în linie de comandă scriu erorile în stderr în loc să afișeze ferestre de mesaj, iar `proxmoxbackup-machine` trimite e-mailul de notificare.
- **ISO-ul de restaurare bare-metal** nu oferă niciodată ca disc țintă stickul de pe care a pornit, iar manualul explică potrivirea firmware-ului (UEFI sau BIOS) când o mașină fizică devine VM.
- **Toate limbile revizuite** (17 traduceri, aproximativ 1.850 de texte), iar datele și numerele urmează limba aplicației.

Versiuni anterioare: **v0.8** backupuri de mașină mai rapide (spațiul liber sărit, sectoarele defecte depășite), fișiere din backupurile de mașină, Pauză și Reia; **v0.7** Recuperare fișiere șterse și Revenire; **v0.6** backupuri incrementale de foldere. [CHANGELOG](CHANGELOG.md) conține toate modificările.

## 🤝 Compatibil cu clientul oficial Proxmox, în ambele sensuri

Verificat cu `proxmox-backup-client` 3.4.9 și PBS 4.2:

- **Instantaneele Legacy, Data și Metadata create de acest client se restaurează identic, octet cu octet, cu clientul oficial**, atât din GUI cât și din linia de comandă, create pe Windows sau Linux, iar browserul de fișiere din PBS 4.2 le deschide. Pe Linux, fișierele restaurate își păstrează și modul exact, data modificării la nanosecundă și țintele legăturilor simbolice.
- **Instantaneele Legacy, Data și Metadata create de clientul oficial se restaurează identic, octet cu octet, cu acest client**, complet sau selectiv.
- Același format de arhivă divizată (pxar versiunea 2), aceleași moduri de detectare a modificărilor, același format al fișierului cheie de criptare (cheile create cu `proxmox-backup-client key create` funcționează aici, iar cheile generate aici funcționează acolo), aceleași variabile de mediu de tip `PBS_REPOSITORY`.

Așadar backupurile dumneavoastră nu sunt niciodată legate de acest client: instrumentele Proxmox restaurează fiecare instantaneu pe care îl creează, iar el restaurează fiecare instantaneu creat de clientul oficial. Singurul element în plus este un mic fișier pe care acest client îl adaugă la rădăcina unui instantaneu de folder, `.proxmox_backup_client_meta.json` (calea originală), pe care clientul oficial îl restaurează ca fișier obișnuit.

> ⚠️ **Declinarea responsabilității:** acest proiect **nu este afiliat în niciun fel** cu **Proxmox Server Solutions GmbH**. „Proxmox”, sigla Proxmox și denumirile asociate aparțin proprietarilor lor; aici sunt folosite **doar** pentru a indica compatibilitatea. Vedeți [proxmox.com](https://www.proxmox.com/) pentru produsele lor.

## 📦 Descărcare și instalare

👉 **[Descărcați ultima versiune](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| Aveți | Folosiți |
|---|---|
| Windows | **Instalatorul MSI** (`ProxmoxBackupClient-<version>-x64.msi`) instalează aplicația și serviciul de fundal în Program Files, cu intrare în meniul Start. Sau dezarhivați `ProxmoxBackupClient-v<version>-windows-amd64.zip` (aplicația și `ProxmoxBackupClientSVC.exe`; păstrați-le în același folder) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Alt Linux | Dezarhivați `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (necesită runtime-ul WebKitGTK 4.1) |
| Doar linie de comandă | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` sau `-linux-amd64.tar.gz` |
| O mașină de restaurat | `ProxmoxBackupClient-GO-BMR-Restore-v<version>.iso`, ISO-ul de restaurare bare-metal |

MSI-ul, `.deb` și `.rpm` sunt noi în v0.9.0: versiunile până la v0.8.1 au doar fișierele zip și tar.gz. [BUILD.md](BUILD.md) descrie cum le construiți singuri.

Pachetele Linux instalează două intrări de meniu: **Proxmox Backup Client** rulează aplicația cu utilizatorul dumneavoastră (backupuri de foldere ale propriilor fișiere), iar **Proxmox Backup Client (administrator)** o rulează ca root, cerând parola prin polkit, pentru backupuri de mașină și instantanee de bloc. Serviciul de fundal este instalat, dar nu este activat; vedeți [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **Windows afișează „virus detectat” (de ex. `Trojan:Win32/Sabsik.FL.A!ml`) sau un avertisment SmartScreen?**
> Este un **fals pozitiv cunoscut** pentru aplicațiile Go/Wails: *nu* este un virus. Sufixul `!ml` indică o detecție a unui model de învățare automată care semnalează executabilele *nesemnate și puțin răspândite*. Verificați descărcarea cu `SHA256SUMS.txt` (mai jos).

### 🔎 Verificarea unei descărcări

Fiecare versiune include un `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-0.9.0-x64.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
```

Pe Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`.

> ℹ️ **Încă nu există semnare de cod, atestare semnată prin CI sau scanare VirusTotal.** Versiunile sunt construite și încărcate manual, nu încă printr-un pipeline CI care ar putea produce o atestare verificabilă a provenienței compilării. Vedeți [TODO.md](TODO.md) dacă doriți să ajutați la configurarea acesteia.

## ✨ Funcționalități

### GUI
- **🌍 Multilingvă**: 18 limbi de interfață: franceză, engleză, italiană, germană, poloneză, spaniolă, bulgară, cehă, greacă, maghiară, letonă, lituaniană, neerlandeză, portugheză, română, slovacă, turcă și ucraineană
- Bară de meniu nativă și meniu lateral (Backup, Restaurare, Recuperare fișiere șterse, Revenire, Rapoarte, Jurnal de mesaje, Preferințe, Despre)
- **Seturi de Backup**: joburi cu nume, reutilizabile, cu **mai multe planificări fiecare** (de la câteva minute până la anual, sau manual), o insignă de tip folder sau mașină, o insignă VSS / Instantaneu, „Execută acum” (rapid sau cu citire completă pentru seturile Metadate) și un avertisment **ID comun** când două seturi ar scrie în același grup de backup
- **Backupuri unice** de foldere sau discuri întregi
- Pagina **Rapoarte** cu istoricul complet al backupurilor și restaurărilor și un **Jurnal de mesaje** limitat, pentru diagnosticare
- Dialogul **Preferințe** cu cinci teme de culoare (Amber, Albastru, Verde, Roșu, Întunecat) plus o opțiune de culoare personalizată
- **Backupuri incrementale de foldere** (detectarea modificărilor, modul Metadate): fișierele nemodificate sunt reutilizate fără a fi citite, ca în clientul oficial, cu o citire completă opțională la fiecare N rulări
- **Backupuri de mașină (disc întreg)** pe Windows și Linux care sar peste spațiul liber și rezistă sectoarelor defecte, stocate ca `host` sau ca backup `vm` Proxmox VE
- **Fizic la virtual**: un backup de mașină se restaurează într-o VM Proxmox VE, prin Proxmox VE sau prin ISO-ul de restaurare
- **Restaurare**: parcurgerea instantaneelor, căutarea fișierelor (wildcard-uri, expresii regulate, căi complete) și restaurare selectivă, cu ACL-uri NTFS/POSIX, fluxuri de date alternative, datele folderelor, datele de creare Windows și o trecere opțională de verificare
- **Fișiere din backupurile de mașină**: parcurgeți și restaurați fișiere din volume NTFS, FAT, exFAT, ext2/3/4 și XFS dintr-o imagine de disc
- **Recuperare fișiere șterse** și **Revenire**: găsiți fișierele pe care backupurile unui Set de Backup încă le au, dar discul nu, și puneți-le la loc; sau readuceți folderele unui set la starea dintr-un backup ales, cu o previzualizare înainte și Anulare după. Seturi de foldere și de mașină. Vedeți [manualul GUI](docs/manual/GUI.md#10-undelete)
- **Pauză, Reia și Oprește** un backup în curs
- **Rulare ca serviciu**: instalați, porniți, opriți sau eliminați serviciul de fundal din Preferințe > Avansat, ca planificările să ruleze fără nimeni autentificat (Windows și Linux)
- Card de progres live: ora de început și ora estimată de terminare, fișiere procesate din total, viteza de procesare (date și fișiere pe secundă) și viteza reală de încărcare, fișierul curent și motivul pentru care o rulare citește toate fișierele
- VSS (Volume Shadow Copy) pe Windows, instantanee de bloc elastio-snap/dattobd pe Linux
- Notificări prin e-mail și acțiuni după backup, clonarea Seturilor de Backup, exportul și importul setărilor
- Mai multe servere PBS, cu fixarea amprentei certificatului (încredere la prima utilizare), autentificare cu token API sau utilizator/parolă
- **Criptare la client**: fișier cheie PBS per server (AES-256-GCM, manifest semnat), protecție opțională cu frază de acces, vedeți [Criptare la client](#-criptare-la-client)
- Protecție de administrator pentru modificarea Seturilor de Backup, cu Repornește ca administrator
- Jurnalizare de depanare pentru diagnosticare

### Instrumente în linie de comandă
- `proxmoxbackup-directory`: backup de foldere (pxar) cu deduplicare și `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: backup live al discurilor întregi pentru mașini Windows (VSS) și Linux (elastio-snap/dattobd) (fidx, incremental, spațiul liber sărit dacă nu se dă `-readfreespace`, `-type vm` pentru un backup Proxmox VE)
- `proxmoxbackup-nbd`: server NBD pentru restaurarea backupurilor de disc (Linux)
- `ProxmoxBackupClientSVC` (Windows) și `proxmoxbackupclient` (pachetele Linux): `-list-sets`, `-undelete` și `-rollback` pentru Seturile de Backup definite în GUI

Primele trei acceptă `-keyfile` și `-keyfile-passphrase` pentru instantanee criptate. Referința completă: [manualul liniei de comandă](docs/manual/CLI.md).

### 📸 Capturi de ecran

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Seturi de Backup**: joburi cu nume, cu o insignă pentru planificarea care rulează următoarea (ZILNIC, SĂPTĂMÂNAL, LUNAR, +N pentru celelalte) și o insignă de tip folder/mașină, Execută acum pe fiecare

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Progresul backupului, live**: debit, timp estimat și numărul de fragmente noi față de cele reutilizate, în timpul rulării

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restaurare**: alegeți un server PBS și un ID backup, listați instantaneele sau căutați în ele după numele fișierului

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**Planificări**: mai multe per set, de la câteva minute până la o dată pe an („lunar, în ultima zi”, „a 2-a marți”), cu următoarele 3 rulări

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Backup ca VM Proxmox VE**: alegeți tipul mașină virtuală și un ID numeric; ID-ul backup din PBS este afișat pe măsură ce tastați

</td>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Seturi de Backup ca VM-uri Proxmox VE**: un set stocat ca backup `vm` poartă insigna `MACHINE (VM)`

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/oneoff-as-vm.png" width="280">](docs/screenshots/oneoff-as-vm.png)
**Backupuri unice**: aceeași opțiune în formularul de backup unic

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**Restaurare selectivă**: bifați fișiere sau foldere individuale din arborele instantaneului și restaurați pe loc sau în altă parte

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Rapoarte**: istoricul complet al rulărilor, cu panou de detalii (durată, fragmente, foldere salvate)

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-message-log.png" width="280">](docs/screenshots/gui-message-log.png)
**Jurnal de mesaje**: un flux de diagnosticare limitat, cu marcaje de timp, pentru toate backupurile și restaurările

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**Multi-PBS**: gestionați mai multe servere PBS, testați conexiunea, alegeți unul implicit

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-edit-server.png" width="280">](docs/screenshots/gui-preferences-edit-server.png)
**Adăugarea sau editarea unui server**: URL, utilizator/parolă sau token API, datastore și namespace

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-theme-amber.png" width="280">](docs/screenshots/gui-theme-amber.png)
**Teme**: cinci palete de culori integrate plus o culoare personalizată, care recolorează instantaneu întreaga aplicație (vedeți banda de mai jos)

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-advanced.png" width="280">](docs/screenshots/gui-preferences-advanced.png)
**Opțiuni de backup**: explicate onest: extragerea paralelă la restaurare este experimentală și a fost măsurată ca fiind *mai lentă* pe propriul nostru hardware de test

</td>
</tr>
</table>

**🎨 Teme de culoare**: chiar dialogul Preferințe se recolorează la fiecare alegere, nu doar o mostră:

<table>
<tr>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-amber.png" width="180">](docs/screenshots/gui-theme-amber.png)
Amber

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-blue.png" width="180">](docs/screenshots/gui-theme-blue.png)
Albastru clasic

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-green.png" width="180">](docs/screenshots/gui-theme-green.png)
Verde

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-red.png" width="180">](docs/screenshots/gui-theme-red.png)
Roșu

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-dark.png" width="180">](docs/screenshots/gui-theme-dark.png)
Întunecat

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-custom.png" width="180">](docs/screenshots/gui-theme-custom.png)
Personalizat

</td>
</tr>
</table>

**🌍 Multilingvă**: pagina Rapoarte în franceză, italiană și spaniolă (și încă 15, printre care engleză, germană și poloneză):

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

### Excluderi inteligente de sistem (backup de foldere)
Backupurile de foldere omit automat următoarele:

**Foldere de sistem:** `System Volume Information` (stocarea VSS, poate depăși 100 GB) și `$RECYCLE.BIN`, oriunde s-ar afla; `Recovery` doar la rădăcina unei unități; `.pbs-rollback` (folderul de siguranță al Revenirii).
**Fișiere de sistem:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` și `DumpStack.log.tmp`.

**De ce contează:** o unitate poate afișa 1,03 TB ocupați, deși fișierele reale au ~141 GB. Fără excluderi, backupul ar include instantaneele VSS (spațiu și timp irosite); cu ele, dimensiunea corespunde datelor reale.

**Recomandare:** folosiți un backup de **foldere** cu aceste excluderi pentru backupurile la nivel de fișier; folosiți un backup de **mașină** într-un Set de Backup separat pentru restaurarea bare-metal și P2V (este o imagine a întregului disc; spațiul liber și fișierele de paginare și hibernare intră ca zerouri).

### Securitate și calitate
- Criptare opțională la client a fiecărui fragment, cu manifest semnat
- Folderul de setări accesibil doar administratorilor pe Windows, privat pentru proprietarul său pe Linux
- API-ul local al serviciului ascultă doar pe 127.0.0.1 și necesită un token, reînnoit la fiecare pornire a serviciului
- Validarea intrărilor și curățarea credențialelor
- Prevenirea path traversal
- Logică de reîncercare cu temporizare exponențială
- Tratarea erorilor și teste unitare în fiecare modul, plus teste live opționale și teste end-to-end pe un PBS real

## 🔐 Criptare la client

Instantaneele pot fi criptate înainte de a părăsi mașina, folosind un fișier cheie Proxmox Backup Server (formatul scris de `proxmox-backup-client key create`, deci cheile sunt interschimbabile cu clientul oficial). Fiecare fragment este criptat cu AES-256-GCM, iar manifestul este semnat. Serverul PBS nu vede niciodată cheia.

**GUI:** deschideți **Preferințe > Informații despre cont**, editați un server și folosiți fila **Criptare**. Alegeți un fișier cheie existent sau apăsați **Generează...** pentru unul nou (opțiunea Avansat îl protejează cu o frază de acces). Fila afișează amprenta cheii. Rapoartele arată dacă fiecare backup a fost criptat. **Fără fișierul cheie, un instantaneu nu poate fi restaurat: păstrați o copie într-un loc sigur, departe de această mașină.**

**Chei protejate cu frază de acces (scrypt sau PBKDF2):** alegeți cum deblochează aplicația cheia:
- **Întreabă-mă**: fraza de acces este cerută o dată pe sesiune a aplicației și păstrată doar în memorie. Backupurile planificate și cele de la pornire nu o cer niciodată: eșuează imediat, cu un mesaj clar, până când fraza de acces a fost introdusă în acea sesiune. Un backup rulat prin serviciul de fundal eșuează și el imediat cu o cheie în modul Întreabă-mă.
- **Ține minte pe acest calculator**: fraza de acces este stocată în configurație, ca tokenul PBS, deci oricine poate citi `config.json` poate debloca cheia. Aceasta este varianta pentru joburile planificate nesupravegheate și pentru serviciul de fundal.

Restaurarea cere fraza de acces când selectați un instantaneu criptat.

**CLI:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` este necesar doar pentru o cheie protejată și este cerut interactiv dacă lipsește. `proxmoxbackup-machine` și `proxmoxbackup-nbd` acceptă aceleași două opțiuni; `proxmoxbackup-nbd` are nevoie de ele pentru a atașa o imagine de disc criptată și verifică primul fragment înainte de a atinge dispozitivul NBD, astfel încât o cheie greșită eșuează imediat.

**Restaurare bare-metal:** ISO-ul de restaurare cere cheia (niciuna, un fișier cheie pe un stick USB sau o cale tastată) și fraza de acces pentru o cheie protejată. Cheia nu este scrisă niciodată în ISO. Vedeți [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md).

**Limitări cunoscute**
- O cheie generată de GUI este scrisă cu un ACL protejat (doar utilizatorul curent, SYSTEM și Administrators). Fișierele cheie păstrate în altă parte au permisiunile pe care le dă folderul lor.
- `config.json` conține în text simplu tokenurile PBS, parolele și orice frază de acces memorată. Începând cu v0.9.0 folderul său este accesibil doar administratorilor (Windows) sau doar proprietarului (Linux); până la v0.8.2 putea fi citit de Utilizatorii locali pe Windows, deci restricționați-l singuri pe o mașină partajată.
- Restaurarea unui instantaneu criptat necesită același fișier cheie; dacă acesta se pierde, nu există nicio cale de recuperare.
- Fluxul cu cheie Întreabă-mă și serviciul sunt procese separate: folosiți Ține minte pentru tot ce rulează prin serviciu.

## Backupuri de foldere mai rapide: detectarea modificărilor

În modul implicit Clasic, fiecare backup de foldere citește toate fișierele, deci o rulare durează cât citirea întregului folder chiar dacă nimic nu s-a schimbat. Seturile de Backup (și CLI-ul) pot folosi în schimb modurile de detectare a modificărilor ale `proxmox-backup-client` oficial. Modul Metadate este folosit zilnic pe un server de 480,9 GB cu 761.866 de fișiere, unde o rulare cu puține modificări durează 10 până la 15 minute în loc de peste 4 ore.

| Mod | Citește | Stocat ca |
|---|---|---|
| **Legacy** (Clasic, implicit) | toate fișierele | un `<name>.pxar.didx` per folder, ca înainte |
| **Data** (Date) | toate fișierele | o arhivă divizată: `<name>.mpxar.didx` (metadate) + `<name>.ppxar.didx` (conținutul fișierelor) |
| **Metadata** (Metadate) | doar fișierele noi și modificate | aceeași arhivă divizată; fișierele a căror dimensiune și dată a modificării (la nanosecundă), mod și proprietar coincid cu backupul anterior sunt preluate din acesta fără a fi citite |

În modul **Metadate**, prima rulare citește tot (încă nu există nimic cu care să compare); rulările următoare citesc doar ce s-a modificat. Un fișier modificat astfel încât își păstrează dimensiunea și data modificării nu este observat, de aceea un set poate și să **recitească toate fișierele la fiecare N rulări** (o rulare Date): acea rulare prinde astfel de modificări și elimină umplutura pe care reutilizarea o lasă în arhivă. Un fișier care nu a putut fi citit complet (completat cu zerouri după o eroare de citire) este întotdeauna citit din nou la rularea următoare. Reutilizarea are loc doar dintr-un backup anterior criptat în același fel, cu aceeași cheie.

Deoarece datele nemodificate nu sunt nici citite, nici trimise, traficul către un PBS la distanță sau extern scade și el la puțin peste ce s-a modificat efectiv.

**GUI:** editați un Set de Backup, fila **Destinație**, **Detectarea modificărilor**. Se aplică doar seturilor de foldere: backupurile de mașină (disc întreg) sunt imagini de disc și nu sunt afectate, deci restaurările bare-metal și pe Proxmox VE funcționează exact ca înainte. Backupurile unice folosesc Clasic. Doar rulările încheiate cu succes contează pentru următoarea citire completă; lista Seturilor de Backup afișează „Următoarea citire completă peste N rulări”. Cardul de progres spune când și de ce o rulare citește toate fișierele. **Rapoarte** arată modul fiecărei rulări și câte fișiere nemodificate au fost reutilizate.

**Un ID backup per set.** Modul Metadate compară cu cel mai nou instantaneu din grupul de backup. Dacă două seturi (sau două mașini) folosesc același ID backup pe același server, fiecare rulare se compară cu instantaneul celuilalt și citește tot. Editorul Setului de Backup avertizează despre acest lucru.

**CLI:** `-change-detection-mode legacy|data|metadata` sau `"change-detection-mode"` în configurația JSON. Nu există o opțiune CLI „la fiecare N rulări”: planificați în schimb ocazional o rulare cu `-change-detection-mode data`.

**Înainte de a trece un set la Date sau Metadate,** actualizați la v0.6.0 sau mai nou toate mașinile de pe care restaurați: versiunile dinaintea suportului pentru arhive divizate nu le pot citi (afișează folderele fără fișierele lor). `proxmox-backup-client` oficial le restaurează identic, octet cu octet (testat cu 3.4.9), iar browserul de fișiere din PBS 4.2 le deschide. Primul backup divizat al unui folder are puțin în comun cu backupurile sale Clasic mai vechi, deci costă aproximativ încă o copie completă a acelui folder în datastore, până când instantaneele vechi sunt eliminate.

## 🚀 Pornire rapidă (GUI)

1. Instalați-o: MSI-ul pe Windows, `.deb` sau `.rpm` pe Linux, sau dezarhivați zip-ul ori tar.gz-ul din [versiuni](https://github.com/mjb-is/proxmoxbackupclient_go/releases) (vedeți [Descărcare și instalare](#-descărcare-și-instalare))
2. Porniți-o. Pe Windows, **Executare ca administrator** pentru VSS, backupuri de mașină și modificarea Seturilor de Backup. Pe Linux, backupurile de foldere ale propriilor fișiere funcționează cu utilizatorul dumneavoastră; folosiți **Proxmox Backup Client (administrator)** (sau rulați ca root) pentru backupuri de mașină și instantanee de bloc
3. Adăugați serverul PBS în **Preferințe > Informații despre cont** și apăsați **Testează**
4. Creați un Set de Backup (sau un backup unic) și selectați folderele sau discurile de salvat
5. Rulați-l. Ca planificările să ruleze fără nimeni autentificat, activați **Preferințe > Avansat > Rulare ca serviciu**

[Manualul GUI](docs/manual/GUI.md) descrie fiecare pas.

## Backup live complet al mașinii

`proxmoxbackup-machine` (și backupurile de mașină din GUI) salvează un sistem Windows 10/11 sau Windows Server complet, sau o mașină Linux, fără nicio oprire.

Sintaxa comenzii este în mare aceeași ca pentru backupul de foldere, doar că `-backupdir` este înlocuit cu `-backupdev` (repetați-l pentru mai multe discuri).
De exemplu, o invocare ar putea fi:

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Comanda de mai sus examinează Discul 0, detectează toate partițiile montate, face un instantaneu VSS al acestora și apoi creează o imagine de backup bootabilă a întregului disc ca fidx. Spațiul liber și fișierele de paginare și hibernare intră în imagine ca zerouri, fără a fi citite (pe Linux, și spațiul liber ext2/3/4 și XFS); `-readfreespace` citește în schimb tot, pentru o copie criminalistică. Un sector care nu poate fi citit intră în imagine ca zerouri, iar rularea indică unde. Adăugați `-type vm` pentru a stoca backupul ca backup de VM Proxmox VE ([P2V](#-mașini-fizice-transformate-în-mașini-virtuale-p2v)).

Următorul backup va fi incremental; calculul hash-urilor a fost paralelizat, astfel încât se pot atinge ușor viteze de 1 GB/s.

### Cerințe pentru backupul de mașină pe Linux (echivalentul VSS)

Backupul de mașină pe Windows folosește VSS (de aici „Executare ca administrator” de mai sus); Linux nu are VSS, așa că acest
proiect folosește modulul de kernel `elastio-snap` (un fork al `dattobd`) pentru aceeași sarcină: un
instantaneu coerent, la un moment dat, al unui dispozitiv de bloc live, în timp ce se scrie în continuare pe el. Depozitul original
`elastio/elastio-snap` este arhivat; versiunea întreținută este
[`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap), pe care trebuie să o instalați. `dattobd`
însuși este de asemenea acceptat, dacă este deja încărcat.

Ambele condiții de mai jos sunt obligatorii, independent una de alta, altfel backupul de mașină Linux refuză să ruleze:

1. **Modulul de kernel trebuie să fie instalat și încărcat.** Instalarea documentată a `elastio-snap` prin pachetul din
   depozit (vedeți [INSTALL.md al acestuia](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md)) indică în prezent
   un URL mutat sau defect, cel puțin pentru Ubuntu 22.04 (confirmat la 2026-09-24: pachetul din depozit
   dă 404 după o redirecționare). Compilarea din sursă funcționează în schimb fiabil:
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
   Astfel modulul este compilat și încărcat doar pentru pornirea curentă; consultați documentația distribuției pentru a-l încărca automat
   la fiecare pornire (de ex. o intrare în `/etc/modules-load.d/`), dacă doriți acest lucru.

2. **Procesul de backup însuși trebuie să ruleze ca root (EUID 0).** Chiar și cu modulul încărcat, `CreateVSSSnapshot`
   (calea de instantanee pentru Linux, în ciuda numelui centrat pe Windows) refuză explicit să ruleze ca utilizator non-root:
   un instantaneu coerent al întregului disc are nevoie de interfața de control a modulului, rezervată lui root, deci aceasta este o cerință
   strictă, nu ceva ce poate fi înlocuit cu apartenența la grupul `disk` sau ceva similar. Pentru un backup de mașină, folosiți
   intrarea de meniu **Proxmox Backup Client (administrator)**, serviciul de fundal sau rulați GUI-ul ori
   `proxmoxbackup-machine` cu `sudo` (sau ca root); un backup simplu de fișiere sau directoare nu are această cerință.

Dacă un backup de mașină eșuează cu `backup device /dev/sdX: open /dev/sdX: permission denied`, de obicei
este suficient să adăugați utilizatorul în grupul `disk` pentru accesul de bază la citire (`sudo usermod -aG disk $USER`, apoi
deconectare și reconectare), dar doar asta NU este suficient pentru un instantaneu real; veți întâlni cerința de root de mai sus
imediat după aceea.

### Restaurarea fișierelor dintr-un backup de disc

Pagina Restaurare din GUI deschide un backup de mașină ca pe unul de foldere: fiecare volum din imagine (NTFS, FAT, exFAT, ext2/3/4 sau XFS) este un folder pe care îl puteți parcurge și din care puteți restaura. Nimic nu este montat.

Pe Linux puteți și să atașați un backup de disc ca dispozitiv de bloc, cu instrumentul nbd.

Pentru a folosi nbd, rulați mai întâi `modprobe nbd max_part=0`.

Din motive necunoscute, folosirea `max_part != 0` provoacă o buclă infinită de sondare a partițiilor.

Instrumentul NBD conectează orice backup de disc fix, indiferent dacă este un backup de VM sau de host (acestea fiind spuse, funcționează și pentru backupurile PVE).

Pentru a-l folosi, folosiți o linie de comandă similară cu aceasta:
`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Dacă omiteți `-path`, apare o interfață în terminal care vă permite să alegeți fișierul fidx.

> Nu folosiți acest lucru pe o mașină care rulează ceva important: un sistem de fișiere corupt poate bloca sistemul de operare, motiv pentru care Proxmox VE folosește o instanță QEMU pentru aceasta.

Demontați tot de pe discul nbd înainte de a opri `proxmoxbackup-nbd`, altfel partiția va rămâne probabil ocupată și imposibil de demontat.

Dacă primiți eroarea `Device or resource busy`, trebuie să forțați deconectarea rulând `nbd-client -d /dev/nbd0` sau pur și simplu să reporniți.

### Restaurarea pe o mașină fizică sau într-o VM

ISO-ul de restaurare bare-metal are o intrare de meniu de boot **„PBS Bare Metal Restore”** complet automată: vedeți [ISO de restaurare bare-metal](#-iso-de-restaurare-bare-metal) mai jos. Este prima intrare, cea implicită la expirarea timpului, și sare peste întrebările despre limbă și tastatură ale Clonezilla: introduceți datele de conectare la PBS, alegeți un instantaneu, confirmați discul țintă detectat, iar ea se ocupă de configurarea rețelei, atașarea NBD, restaurarea propriu-zisă și o repornire automată, fără altă intervenție. Același ISO restaurează într-o mașină virtuală goală pentru [P2V](#-mașini-fizice-transformate-în-mașini-virtuale-p2v).

Ruta mai veche, semi-manuală, este livrată în continuare pe același ISO, ca intrare separată în meniul normal de selectare a modului din Clonezilla („pbs-nbd”): vă ghidează prin datele de conectare la PBS și atașează în fundal instantaneul ales la `/dev/nbd0`, apoi vă lasă să efectuați manual restaurarea prin asistentul standard de restaurare a discurilor din Clonezilla.

## Utilizare: backup de directoare

Referința completă, cu exemple pentru Windows și Linux, coduri de ieșire și planificare, se află în **[manualul liniei de comandă](docs/manual/CLI.md)**. O comandă tipică:

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

Sunt citite și variabilele de mediu `PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD` și `PBS_FINGERPRINT` ale clientului oficial.

Pentru configurarea JSON este furnizat un exemplu JSON; completați doar câmpurile necesare.

Notă despre șabloanele de e-mail:
pentru subiectele și corpurile e-mailurilor se folosește [motorul de șabloane Go](https://pkg.go.dev/text/template); consultați documentația acestuia pentru sintaxă.
Următoarele variabile sunt disponibile în șabloane:

- `.NewChunks`: numărul de fragmente noi create
- `.ReusedChunks`: numărul de fragmente reutilizate
- `.Datastore`: numele datastore-ului
- `.Error`: eroarea, dacă există (`.ErrorStr` o dă ca text, gol când nu există)
- `.ReadErrors`: fișierele sau folderele care nu au putut fi citite; `.ReadErrorCount` indică numărul lor
- `.Hostname`: numele de host al mașinii
- `.StartTime`: momentul în care a început backupul
- `.EndTime`: momentul în care s-a încheiat backupul
- `.Duration`: durata backupului
- `.FromattedDuration`: durata formatată a backupului (numele este scris astfel în cod)
- `.Success`: adevărat când backupul s-a încheiat fără erori și fără nimic ilizibil
- `.Partial`: adevărat când a fost salvat un instantaneu, dar unele fișiere nu au putut fi citite
- `.Status`: `Success`, `Partial` sau `Failed`

## Backup de flux

Permite backupul unui flux în loc de un pxar, ceea ce deschide posibilități nelimitate. De exemplu, puteți rula:

```
mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]
```

Astfel se poate folosi buzhash pentru deduplicare chiar și cu tar, de exemplu, sau cu dump-ul SQL în sine.

## Probleme cunoscute

Când antimalware-ul Windows Defender este activ, backupul este încetinit până la 25% din viteza posibilă.

Pe Windows, instrumentele în linie de comandă iau o blocare de proces: un al doilea backup pornit în timp ce unul rulează afișează „Another backup job is running” și se încheie. Pe Linux nu există o astfel de blocare, deci nu planificați două rulări CLI în același timp.

## 📋 Cerințe preliminare

- Windows 10/11 sau Windows Server (64 de biți), cu drepturi de administrator pentru VSS și backupuri de mașină
- sau Linux (pachetele sunt pentru Debian, Ubuntu, Mint și Fedora), cu runtime-ul WebKitGTK pentru GUI, plus root și modulul de kernel elastio-snap sau dattobd pentru backupuri de mașină coerente
- Acces în rețea la un Proxmox Backup Server
- Pentru P2V: Proxmox VE cu datastore-ul PBS adăugat ca stocare, sau orice hipervizor care poate porni ISO-ul de restaurare

## 🔨 Compilare din sursă

**[BUILD.md](BUILD.md)** conține instrucțiunile complete: GUI-ul și serviciul de fundal, instrumentele în linie de comandă, MSI-ul și pachetele Linux. Pe scurt, cu Go 1.25 sau mai nou și Wails CLI v2.13:

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

Interfața compilată (`gui/frontend/dist`) este inclusă în depozit, deci Node.js este necesar doar pentru a modifica interfața (`npm install` și `npm run build` în `gui/frontend`, sau `wails dev` pentru reîncărcare la cald). Teste: `make test`.

## 🔧 Utilizare avansată și ghiduri

### Multi-PBS (mai multe servere PBS)

Configurați mai multe servere PBS și alegeți ținta pentru fiecare Set de Backup sau backup unic (de ex. `C:\Users` zilnic pe un PBS rapid cu SSD, `D:\` săptămânal pe un PBS de mare capacitate, plus un server de recuperare în caz de dezastru).

- **[Ghid de utilizare](MULTI_PBS_USER_GUIDE.md)**: adăugarea și testarea serverelor, serverul implicit, alegerea unui server pentru fiecare backup, întrebări frecvente și depanare.
- **[Ghid de implementare](MULTI_PBS_GUIDE.md)**: modelul de date, migrarea automată dintr-o configurație cu un singur PBS, metodele backend.

O configurație veche cu un singur PBS este migrată automat la un server `default` la prima încărcare.

### 💿 ISO de restaurare bare-metal

ISO-ul de restaurare este un ISO Clonezilla Live standard, modificat cu binarele `pbsnbd` / `machinebackup` plus două intrări de meniu. Pornește de pe CD, de pe un stick USB (scris cu `dd` sau de pe un stick Ventoy), prin PXE, pe mașini UEFI sau BIOS:

- **`ocs-pbs-bare-metal-restore`**: prima intrare din meniul de boot, cea implicită la expirarea timpului, **„Proxmox Backup Client Go - PBS Bare Metal Restore”**. Sare peste întrebările despre limbă și tastatură ale Clonezilla și peste asistentul său generic și trece direct de la credențialele PBS la o restaurare încheiată, cu repornire automată: mai întâi DHCP (o cerere de IP static doar dacă acesta eșuează), sincronizare NTP, o cheie de criptare de pe un stick USB dacă backupul este criptat și o detectare a discului țintă care nu oferă niciodată stickul de pe care a pornit. La reușită afișează un mesaj de finalizare și repornește. Restaurează la fel backupurile Windows și Linux (restaurarea se face la nivel de bloc), pe o mașină fizică sau într-o VM goală.
- **`ocs-pbs-nbd`**: o intrare **„pbs-nbd”** în meniul de selectare a modului din Clonezilla. Cere datele de conectare la PBS și instantaneul de restaurat, îl atașează la `/dev/nbd0` în fundal, apoi predă controlul asistentului normal de restaurare a discurilor din Clonezilla, pe care îl conduceți manual.

Tot pe partea de restaurare s-au corectat: un blocaj (deadlock) al `zstd.NewReader` în calea de preluare a fragmentelor PBS, care bloca restaurările de mașini întregi (contribuit proiectului original ca [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85)), o eroare de tipul „prima trecere de restaurare eșuează cu 'no partition', a doua reușește”, cauzată de o condiție de cursă între `is_disk_without_part_and_fs()` din Clonezilla și udev, și mașinile BIOS/MBR care nu porneau după o restaurare (datele dintre MBR și prima partiție sunt acum restaurate și ele).

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

Toate detaliile (de ce o reconstruire completă a ISO-ului în loc de o înlocuire pe loc, cerințe preliminare, fluxul meniului, verificare) se află în **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**; utilizarea, în **[manualul restaurării bare-metal](docs/manual/BMR.md)**.

### Compilarea GUI-ului Windows

**Windows nativ**: vedeți **[BUILD.md](BUILD.md)** pentru lanțul de instrumente (Go, MinGW, Wails) și comenzile pentru aplicație, serviciu și MSI.

**Docker (la compilarea pe Linux).** `./build_gui_windows_docker.sh` compilează aplicația Windows ca `ProxmoxBackupClientGO.exe`, cu suport WebView2 corect, într-un container `golang` de unică folosință (instalează mingw și Wails, compilează interfața, rulează `wails build`). Nu compilează serviciul de fundal și nici MSI-ul.

### Starea funcționalităților, jurnalul modificărilor și documentația internă

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**: ce este gata, parțial sau nerealizat, și din ce versiune.
- **[CHANGELOG.md](CHANGELOG.md)**: istoricul modificărilor pe versiuni și [Unreleased] pentru ce se află pe master.
- **[TODO.md](TODO.md)**: foaia de parcurs deschisă și idei.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**: ce conține fiecare versiune.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**: dialogul de dezinstalare MSI (păstrarea sau ștergerea configurației) și planul său de testare.
- **[AGENTS.md](AGENTS.md)**: structura depozitului și convențiile, pentru contribuitori și agenți de programare.

## 🔁 Contribuții la proiectul original

Corecturile și funcționalitățile de interes general sunt oferite [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) ca pull request-uri. Optsprezece au fost integrate:

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

## 🧪 Stare, raportarea erorilor și cereri de funcționalități

**Acesta este software alfa, oferit ca atare, fără nicio garanție de niciun fel, și folosit în întregime pe propriul risc.** Nu îmi asum nicio responsabilitate pentru date pierdute sau deteriorate, inclusiv fișierele sursă.

Este construit pentru propriul meu homelab. Fiecare versiune este testată de mine înainte de a fi publicată și este folosită acolo în mod curent: backupuri planificate de foldere și de mașină pentru mașini Windows și Linux, cu restaurări, Recuperare fișiere șterse, Revenire, restaurări bare-metal și restaurări P2V testate pe parcurs. Niciun backup nu este dovedit până când nu ați restaurat din el, așa că testați-vă propriile restaurări, inclusiv una bare-metal, înainte de a vă baza pe el.

- **Rapoartele de erori sunt binevenite**: deschideți un [issue](https://github.com/mjb-is/proxmoxbackupclient_go/issues) cu ce ați făcut, ce s-a întâmplat și jurnalul (Jurnalul de mesaje sau fișierele din folderul de setări).
- **Cereri de funcționalități**: adaug funcționalități care îmi folosesc în propria utilizare, așa că nu accept cereri pentru alte utilizări. Vă rog să le trimiteți proiectului original, [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go/issues). Pull request-urile care corectează erori, sau care ar ajuta clar și utilizarea mea, sunt binevenite.

Rapoartele de erori și corecturile sunt cel mai util ajutor. Dacă doriți să mulțumiți altfel, puteți [să-mi oferiți o cafea pe Ko-fi](https://ko-fi.com/mickbeeby).

## 🙏 Mulțumiri și istoric

Clientul original în linie de comandă a fost scris de Tiziano Bacocco (tizbac) începând din 2023. Ulterior, clientul grafic al RDEM Systems a fost integrat în el, și odată cu acesta primul GUI, suportul multi-PBS și serviciul Windows. Acest fork a început în septembrie 2026 pentru a acoperi lipsurile care împiedicau PBS să fie singurul meu sistem de backup; versiunile sale pornesc de la v0.3.0, iar corecturile de interes general se întorc în proiectul original.

- Clientul original, [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go), de Tiziano Bacocco (tizbac) și contribuitorii săi.
- Clientul grafic Nimbus Backup al RDEM Systems, punctul de pornire al interfeței grafice a acestei aplicații.
- Fluxul de lucru și stilul Backup for Workgroups (BFW), pe care le urmează Seturile de Backup, Recuperarea fișierelor șterse și Revenirea.
- [Clonezilla Live](https://clonezilla.org/), pe care este construit ISO-ul de restaurare bare-metal.

## 📄 Licență

GPLv3: vedeți fișierul [LICENSE](LICENSE).

---

**Modificări de la fork © 2026 mjb-is, pe baza muncii lui Tiziano Bacocco (tizbac), a contribuitorilor Proxmox Backup Client GO și a RDEM Systems (© 2024-2026).**

☕ Dacă acest client vă este util, puteți [să-mi oferiți o cafea pe Ko-fi](https://ko-fi.com/mickbeeby). Mulțumesc.
