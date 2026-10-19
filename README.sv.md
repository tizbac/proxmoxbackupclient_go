# Proxmox Backup Client: Windows- och Linux-klient för Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)
[![Ko-fi](https://img.shields.io/badge/Ko--fi-buy%20me%20a%20coffee-FF5E5B?logo=ko-fi&logoColor=white)](https://ko-fi.com/mickbeeby)

> **Byggt på andras arbete.** Den här forken bygger på Tiziano Bacoccos (tizbac) ursprungliga [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) och på RDEM Systems grafiska klient Nimbus Backup, som slogs ihop med det och som appens grafiska gränssnitt växte fram ur.
>
> ☕ Klienten är gratis och förblir gratis. Om den sparar dig tid kan du [bjuda mig på en kaffe på Ko-fi](https://ko-fi.com/mickbeeby). Det är helt frivilligt och köper varken support eller löften om funktioner.

**Proxmox Backup Client är en backupklient med öppen källkod (GPL-3.0) för Proxmox Backup Server (PBS), för Windows och Linux.** Den säkerhetskopierar mappar som filarkiv som går att bläddra i och hela maskiner som startbara diskavbilder, enligt schema eller på begäran, och tar tillbaka allt från en enda fil till en hel maskin, till samma dator, en ersättningsdator eller en virtuell maskin i Proxmox VE.

<p align="center">
  <a href="docs/screenshots/overview-how-it-works.png"><img src="docs/screenshots/overview-how-it-works.png" width="400" alt="How it works: Windows and Linux machines back up through the client to Proxmox Backup Server, then restore as files, onto bare metal, or into a Proxmox VE VM"></a>
  <a href="docs/screenshots/overview-screenshots.png"><img src="docs/screenshots/overview-screenshots.png" width="400" alt="Screenshots of the app: Backup Sets, Reports, schedules, a whole PC as a VM, backup progress and restore"></a>
</p>

Jag bygger den för mitt eget hemmalabb, för att göra PBS till det enda backupsystemet för en blandning av Windows- och Linux-servrar, stationära datorer och bärbara datorer, i stället för Backup for Workgroups, rsync-speglar och Clonezilla/RescueZilla-avbilder. Den började i september 2026 som en fork av [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) och har vuxit långt bortom den: över 350 commits sedan dess, med 18 rättningar och funktioner skickade tillbaka till originalprojektet ([lista nedan](#-bidrag-till-originalprojektet)).

**Du får gärna använda den som den är.** Den är officiellt **alfaprogramvara**, som används på egen risk, utan någon garanti av något slag. Jag testar varje release själv och den används regelbundet på mina egna maskiner, men läs [Status, felrapporter och funktionsönskemål](#-status-felrapporter-och-funktionsönskemål) innan du förlitar dig på den.

Sviten:

- **Proxmox Backup Client GUI**: appen för Windows och Linux. **Backup Sets** (backupuppsättningar) med flera scheman vardera, mapp- och helhetsdiskbackup med konsekventa ögonblicksbilder (VSS på Windows, elastio-snap/dattobd på Linux), återställning av filer från mapp- och maskinbackuper, **Undelete** (återskapa raderade filer) och **Roll back** (återställ till ett tidigare läge), flera PBS-servrar, kryptering på klientsidan och en bakgrundstjänst så att scheman körs utan att någon är inloggad.
- **`proxmoxbackup-directory`**: mappbackup (pxar) från kommandoraden, med deduplicering och ändringsdetektering.
- **`proxmoxbackup-machine`**: livebackup av hela diskar (fidx) på Windows- och Linux-maskiner från kommandoraden; avbilden går att starta efter en återställning.
- **`proxmoxbackup-nbd`**: kopplar in en diskbackup som NBD-enhet (Linux).
- **En ISO för bare-metal-återställning** byggd på Clonezilla Live, med en guidad återställning från påslag till omstart.

> Nyckelord: proxmox backup-klient för windows · PBS-klient · Windows VSS-backup · fysisk till virtuell (P2V) · bare-metal-återställning · gränssnitt för Proxmox Backup Server.

## ⭐ Vad den gör, med uppmätta resultat

| | Resultat |
|---|---|
| **Inkrementell mappbackup** | En utdelning på en filserver, 480,9 GB och 761 866 filer: första körningen 4 h 17 min, varje senare körning 10 till 15 minuter. Oförändrade filer öppnas inte ens (den officiella klientens ändringsdetektering i läget Metadata). |
| **Snabb maskinbackup** | Ledigt utrymme, växlingsfilen och vilolägesfilen läses inte, och tomt diskutrymme hashas inte. En disk på 1 TB i en bärbar dator med 373 GB använt: tidigare 1 h 23 min till 1 h 44 min, nu 38 minuter. En testdisk på 50 GB: 3 min 36 s, nu 1 min 15 s. |
| **Filer ur maskinbackuper** | Bläddra i en diskavbild som i en mapp och återställ enskilda filer, från NTFS, FAT, exFAT, ext2/3/4 och XFS. En bärbar dators C:-enhet på 127 GB med 430 000 filer listas på 4 till 15 sekunder; inget monteras eller laddas ner först. |
| **Fysisk maskin till virtuell maskin** | Säkerhetskopiera en PC eller server och starta den sedan som en VM i Proxmox VE: återställd via Proxmox VE självt eller med återställnings-ISO:n. [Se nedan](#-fysiska-maskiner-till-virtuella-maskiner-p2v). |
| **Bare-metal-återställning** | Starta ISO:n (USB-sticka, CD eller PXE), ange PBS-uppgifterna, välj backupen: en Windows-maskin på 32 GB var tillbaka vid inloggningsskärmen 8 minuter efter påslag. |
| **Undelete** | Efter att 39 GB raderats från en utdelning med 761 000 filer hittade en 7-dagarssökning exakt de filerna på ungefär en minut och lade tillbaka dem. |
| **Roll back** | Återställ ett sets mappar till hur de var vid en vald backup, med förhandsvisning först och **Undo** (ångra) efteråt. |
| **Scheman** | Flera per Backup Set, från några minuters intervall till en gång om året: ”varje dag kl. 02:00” plus ”sista dagen varje månad” eller ”den 2:a tisdagen”, var och en i klartext med sina nästa 3 körningar. |
| **Trogen återställning** | NTFS-behörigheter, attribut, alternativa dataströmmar och utökade attribut, skapandetider och mapptider på Windows; lägen, ägare, ACL:er, utökade attribut och symboliska länkar på Linux. Ett valfritt pass läser om det som återställts och jämför med backupen. |
| **Fungerar med Proxmox egna verktyg** | Backuper som görs här återställs byte för byte identiskt med den officiella `proxmox-backup-client`, och tvärtom. [Detaljer](#-kompatibel-med-den-officiella-proxmox-klienten-åt-båda-hållen). |

## 🧩 Vad den tillför jämfört med originalprojektet

Utöver de [18 ändringar som slagits ihop med originalprojektet](#-bidrag-till-originalprojektet) tillför den här klienten:

**Backuper**
- Inkrementell mappbackup med den officiella klientens ändringsdetektering (lägena Data och Metadata, uppdelade pxar v2-arkiv), med en fullständig läsning var N:e körning.
- Maskinbackuper som hoppar över ledigt utrymme, växlingsfilen och vilolägesfilen (Windows NTFS; Linux ext2/3/4 och XFS), skriver oläsbara sektorer som nollor och fortsätter, och visar antal block, disk, partition och antal chunks medan de körs.
- **Pause**, **Resume** och **Stop** (pausa, återuppta, stoppa) för mapp- och maskinbackuper, även för backuper som bakgrundstjänsten kör.
- Alternativa dataströmmar, utökade NTFS-attribut och skapandetider i mappbackuper på Windows.

**Scheman och Backup Sets**
- Flera scheman per Backup Set, från några minuters intervall till en gång om året (den n:te veckodagen, månadens sista dag), var och en i klartext med sina nästa 3 körningar.
- En bakgrundstjänst på Linux såväl som på Windows, och **Restart as administrator** (starta om som administratör) på Linux.
- Snabb eller fullständig **Run Now** (kör nu), en VSS / Snapshot-märkning, en **SHARED ID**-varning (delat ID), setnamn som skrivs till PBS som kommentarer på ögonblicksbilderna, e-postlarm, åtgärder efter backup samt export och import av inställningar.

**Återställningar**
- Enskilda filer ur maskinbackuper (NTFS, FAT, exFAT, ext2/3/4, XFS), utan att montera eller ladda ner avbilden.
- Undelete och Roll back, för mapp- och maskinbackupsets, i GUI:t och från kommandoraden.
- Ett valfritt verifieringspass efter en återställning, Windows skapandetider och ett ögonblicksbildsträd som laddar en mapp i taget, så att mycket stora backuper öppnas direkt.
- En guidad ISO för bare-metal-återställning som också återställer till en tom VM, och testade vägar från fysisk till virtuell ([P2V](#-fysiska-maskiner-till-virtuella-maskiner-p2v)).
- Byte för byte identisk kompatibilitet med den officiella `proxmox-backup-client` åt båda hållen, inklusive dess Data- och Metadata-ögonblicksbilder (uppdelat arkiv).

**Appen**
- Ett ombyggt gränssnitt: menyrad, sidomeny, **Reports** (rapporter) med hela backup- och återställningshistoriken, en **Message Log** (meddelandelogg), fem färgteman och en egen färg, notiser i meddelandefältet och popup-notiser.
- 18 gränssnittsspråk, alla granskade (originalet har 5).
- Inställningsmappen låst till administratörer (Windows) eller till sin ägare (Linux), och en API-token för tjänsten som förnyas vid varje start.
- MSI-, `.deb`- och `.rpm`-paket, och användarmanualerna för GUI:t, kommandoraden och bare-metal-återställning.

## 🔄 Fysiska maskiner till virtuella maskiner (P2V)

En maskinbackup av en fysisk PC eller server kan bli en virtuell maskin i Proxmox VE, på två sätt:

**1. Återställ den i Proxmox VE.** Ställ in Backup Setet (eller engångsbackupen) på **Backup as: Proxmox VE virtual machine** (säkerhetskopiera som: virtuell maskin i Proxmox VE). Backupen lagras som `vm/<nummer>` med en liten VM-konfiguration, så Proxmox VE listar den bland sina andra VM-backuper: öppna PBS-lagringen i Proxmox VE, välj backupen, **Restore**, välj ett VM-ID och en lagring. VM:en får en SATA-disk och rätt firmware för disken (OVMF för UEFI/GPT, SeaBIOS för BIOS/MBR); lägg till ett nätverkskort innan du startar den, och en EFI-disk (med Pre-Enroll keys avbockat) om en UEFI-VM inte startar.

**2. Återställ den med ISO:n för bare-metal-återställning till en tom VM.** Skapa en VM med samma firmware som originalmaskinen (OVMF med en EFI-disk för UEFI/GPT, SeaBIOS för BIOS/MBR), en SATA-disk som är minst lika stor som originalet och ett Intel E1000-nätverkskort, starta den från ISO:n och kör återställningen. Det fungerar lika bra för `host`- och `vm`-backuper, och samma backup går fortfarande att återställa till fysisk hårdvara.

Testade vägar:

| Från | Till | Väg | Resultat |
|---|---|---|---|
| Bärbar dator med Windows 10, UEFI/GPT | VM i Proxmox VE | Återställnings-ISO | Startade till användarens skrivbord |
| Windows Server, UEFI/GPT | VM i Proxmox VE | Återställning i Proxmox VE | Startade utan ändringar |
| Xubuntu-skrivbordsdator, UEFI/GPT | VM i Proxmox VE | Återställning i Proxmox VE, och återställnings-ISO:n | Startade till skrivbordet |
| Linux, BIOS/MBR | VM i Proxmox VE | Återställnings-ISO, och återställning i Proxmox VE | Startade till inloggningsprompten |
| Linux Mint, krypterad backup | VM i Proxmox VE | Återställnings-ISO, nyckel från en USB-sticka | Startade |
| Linux-PC | samma PC med en ny disk | Återställnings-ISO | Startade |

Windows installerar den nya virtuella hårdvaran vid första starten (”Getting devices ready”). [Manualen för bare-metal-återställning](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines) har firmwaretabellen, drivrutinerna och steg-för-steg-instruktioner.

## 📖 Användarmanualer

| Manual | Innehåll |
|---|---|
| **[GUI-manual](docs/manual/GUI.md)** | Installation, PBS-servrar, Backup Sets och scheman, inkrementell backup, förlopp, återställning, Undelete, Roll back, Reports, loggar, Preferences, bakgrundstjänsten, felsökning |
| **[Kommandoradsmanual](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: alla flaggor, konfigurationsfiler, slutkoder, schemaläggning; Undelete och Roll back från kommandoraden |
| **[Manual för bare-metal-återställning](docs/manual/BMR.md)** | Återställnings-ISO:n steg för steg; `vm`-backuper till Proxmox VE; fysisk till virtuell; annan hårdvara |
| [Återställningsscenarier](docs/RESTORE_GUIDE.md) | Vilken återställning som passar när |
| [Manualindex](docs/manual/README.md) | En översikt över de tre manualerna |

## 🆕 Nytt i v0.9.0: flera scheman per Backup Set, installationsprogram, en Linux-tjänst

**v0.9.1** rättar ISO:n för bare-metal-återställning: återställningsvalet finns åter i BIOS-startmenyn, en Ventoy-sticka känns igen som startdisken, och en måldisk som är mindre än originalet avvisas innan något skrivs. Den noterar också ett aviseringsmejl som inte kunde skickas i ögonblicksbildens PBS-kommentar.

- **Flera scheman per Backup Set**, från några minuters intervall till en gång om året: ”varje dag kl. 02:00” plus ”varje månad den sista dagen” eller ”den 2:a tisdagen”, varje schema i klartext med sina nästa 3 körningar. Befintliga sets behåller sitt schema.
- **Mappbackuper behåller alternativa dataströmmar och utökade NTFS-attribut** (Windows), till exempel markeringen ”hämtad från internet”, och en återställning lägger tillbaka dem. Listan över kända begränsningar innehåller nu inga.
- **Installationsprogram**: en **Windows-MSI** (appen och bakgrundstjänsten under Program Files, post i Start-menyn, uppgradering på plats, behåll eller radera inställningarna vid avinstallation) och **Linux-paket**, `.deb` (Debian, Ubuntu, Mint) och `.rpm` (Fedora), med appen, de tre kommandoradsverktygen, manualerna och menyposter.
- **En bakgrundstjänst på Linux** (`proxmoxbackupclient.service`): schemalagda Backup Sets körs som root, även när ingen är inloggad. Slås på under **Preferences > Advanced > Run as Service** (inställningar > avancerat > kör som tjänst), som på Windows. **Restart as administrator** fungerar nu även på Linux (via pkexec).
- **Pause, Resume och Stop för backuper som tjänsten kör**, med förloppskortet i den öppna appen.
- **En varning när två Backup Sets delar backupgrupp** (samma server, typ och Backup ID), med en **SHARED ID**-märkning på båda korten.
- **Låsta inställningar**: på Windows är `C:\ProgramData\ProxmoxBackupClient` nu bara öppen för administratörer; på Linux är inställningsmappen privat för ägaren.
- **Filer ur maskinbackuper för alla vanliga filsystem**: FAT, exFAT, ext2/3/4 och XFS utöver NTFS. Återställning av Windows-avbilder tar också tillbaka behörigheter och alternativa dataströmmar.
- **Maskinbackuper på Linux hoppar över ledigt utrymme** (ext2/3/4, XFS), som Windows gör sedan v0.8.0.
- **Undelete och Roll back för Linux-maskinbackupsets**, och **från kommandoraden** för alla Backup Sets: `-list-sets`, `-undelete`, `-rollback` (avsnitt 12 i [kommandoradsmanualen](docs/manual/CLI.md)).
- Kommandoradsverktygen skriver fel till stderr i stället för att visa meddelanderutor, och `proxmoxbackup-machine` skickar sitt notifieringsmejl.
- **ISO:n för bare-metal-återställning** erbjuder aldrig stickan den startade från som disk att återställa till, och manualen förklarar hur firmware (UEFI eller BIOS) ska matcha när en fysisk maskin blir en VM.
- **Alla språk granskade** (17 översättningar, cirka 1 850 strängar), och datum och tal följer appens språk.

Tidigare releaser: **v0.8** snabbare maskinbackup (ledigt utrymme hoppas över, dåliga sektorer överlevs), filer ur maskinbackuper, Pause och Resume; **v0.7** Undelete och Roll back; **v0.6** inkrementell mappbackup. [CHANGELOG](CHANGELOG.md) innehåller varje ändring.

## 🤝 Kompatibel med den officiella Proxmox-klienten, åt båda hållen

Verifierat med `proxmox-backup-client` 3.4.9 och PBS 4.2:

- **Legacy-, Data- och Metadata-ögonblicksbilder från den här klienten återställs byte för byte identiskt med den officiella klienten**, från både GUI och kommandorad, gjorda på Windows eller Linux, och filbläddraren i PBS 4.2 öppnar dem. På Linux behåller de återställda filerna även exakt läge, ändringstid på nanosekunden och mål för symboliska länkar.
- **Legacy-, Data- och Metadata-ögonblicksbilder från den officiella klienten återställs byte för byte identiskt med den här klienten**, helt eller selektivt.
- Samma uppdelade arkivformat (pxar version 2), samma lägen för ändringsdetektering, samma format för krypteringsnyckelfiler (nycklar från `proxmox-backup-client key create` fungerar här, och nycklar som genereras här fungerar där), samma miljövariabler i stil med `PBS_REPOSITORY`.

Dina backuper är alltså aldrig bundna till den här klienten: Proxmox egna verktyg återställer varje ögonblicksbild den gör, och den återställer varje ögonblicksbild som den officiella klienten gör. Det enda extra är en liten fil som den här klienten lägger i roten av en mappögonblicksbild, `.proxmox_backup_client_meta.json` (den ursprungliga sökvägen), som den officiella klienten återställer som en vanlig fil.

> ⚠️ **Ansvarsfriskrivning:** detta projekt är **inte på något sätt anslutet** till **Proxmox Server Solutions GmbH**. ”Proxmox”, Proxmox-logotypen och relaterade namn tillhör respektive ägare; här används de **endast** för att ange kompatibilitet. Se [proxmox.com](https://www.proxmox.com/) för deras produkter.

## 📦 Ladda ner och installera

👉 **[Ladda ner senaste versionen](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| Du har | Använd |
|---|---|
| Windows | **MSI-installationsprogrammet** (`ProxmoxBackupClient-<version>-x64.msi`) installerar appen och bakgrundstjänsten under Program Files, med en post i Start-menyn. Eller packa upp `ProxmoxBackupClient-v<version>-windows-amd64.zip` (appen och `ProxmoxBackupClientSVC.exe`; ha dem i samma mapp) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Annan Linux | Packa upp `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (kräver körtidsmiljön WebKitGTK 4.1) |
| Bara kommandorad | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` eller `-linux-amd64.tar.gz` |
| En maskin att återställa | `ProxmoxBackupClient-GO-BMR-Restore-v<version>.iso`, ISO:n för bare-metal-återställning |

MSI, `.deb` och `.rpm` är nya i v0.9.0: releaser till och med v0.8.1 har bara zip- och tar.gz-filerna. [BUILD.md](BUILD.md) beskriver hur du bygger dem själv.

Linux-paketen installerar två menyposter: **Proxmox Backup Client** kör appen som du (mappbackup av dina egna filer), och **Proxmox Backup Client (administrator)** kör den som root och frågar efter ditt lösenord via polkit, för maskinbackup och blockögonblicksbilder. Bakgrundstjänsten installeras men slås inte på; se [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **Visar Windows ”virus upptäckt” (t.ex. `Trojan:Win32/Sabsik.FL.A!ml`) eller en SmartScreen-varning?**
> Det är ett **känt falsklarm** för Go/Wails-program: det är *inte* ett virus. Suffixet `!ml` anger en detektering från en maskininlärningsmodell som flaggar *osignerade och ovanliga* körbara filer. Kontrollera nedladdningen mot `SHA256SUMS.txt` (nedan).

### 🔎 Verifiera en nedladdning

Varje release innehåller en `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-0.9.0-x64.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
```

På Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`.

> ℹ️ **Ännu ingen kodsignering, CI-signerad attestering eller VirusTotal-skanning.** Releaserna byggs och laddas upp manuellt, ännu inte genom en CI-pipeline som skulle kunna ta fram en verifierbar attestering av byggets ursprung. Se [TODO.md](TODO.md) om du vill hjälpa till att sätta upp det.

## ✨ Funktioner

### GUI
- **🌍 Flerspråkigt**: 18 gränssnittsspråk: franska, engelska, italienska, tyska, polska, spanska, bulgariska, tjeckiska, grekiska, ungerska, lettiska, litauiska, nederländska, portugisiska, rumänska, slovakiska, turkiska och ukrainska
- Inbyggd menyrad och sidomeny (Backup, Restore, Undelete, Roll back, Reports, Message Log, Preferences, About)
- **Backup Sets**: återanvändbara namngivna jobb med **flera scheman vardera** (från några minuters intervall till årligen, eller manuellt), en typmärkning för mapp eller maskin, en VSS / Snapshot-märkning, ”Run Now” (snabb eller fullständig läsning för Metadata-sets) och en **SHARED ID**-varning när två sets skulle skriva till samma backupgrupp
- **Engångsbackup** av mappar eller hela diskar
- Sidan **Reports** med full backup- och återställningshistorik, och en begränsad **Message Log** för diagnostik
- Dialogrutan **Preferences** med fem färgteman (Amber, Blue, Green, Red, Dark) plus ett alternativ för egen färg
- **Inkrementell mappbackup** (ändringsdetektering, läget Metadata): oförändrade filer återanvänds utan att läsas, som i den officiella klienten, med valfri fullständig läsning var N:e körning
- **Maskinbackup (hel disk)** på Windows och Linux som hoppar över ledigt utrymme och överlever dåliga sektorer, lagrad som `host` eller som en `vm`-backup för Proxmox VE
- **Fysisk till virtuell**: en maskinbackup återställs till en VM i Proxmox VE, via Proxmox VE eller återställnings-ISO:n
- **Restore**: bläddring i ögonblicksbilder, filsökning (jokertecken, reguljära uttryck, hela sökvägar) och selektiv återställning, med NTFS/POSIX-ACL:er, alternativa dataströmmar, mapptider, Windows skapandetider och ett valfritt verifieringspass
- **Filer ur maskinbackuper**: bläddra i och återställ filer från NTFS-, FAT-, exFAT-, ext2/3/4- och XFS-volymer i en diskavbild
- **Undelete** och **Roll back**: hitta filer som ett Backup Sets backuper fortfarande har men som disken inte har, och lägg tillbaka dem; eller återställ ett sets mappar till hur de var vid en vald backup, med förhandsvisning först och Undo efteråt. Mapp- och maskinsets. Se [GUI-manualen](docs/manual/GUI.md#10-undelete)
- **Pause, Resume och Stop** för en pågående backup
- **Run as Service**: installera, starta, stoppa eller ta bort bakgrundstjänsten under Preferences > Advanced, så att scheman körs utan att någon är inloggad (Windows och Linux)
- Live-förloppskort: starttid och beräknad sluttid, klara filer av totalt, bearbetningshastighet (data och filer per sekund) och faktisk uppladdningshastighet, aktuell fil, och varför en körning läser varje fil
- VSS (Volume Shadow Copy) på Windows, elastio-snap/dattobd-blockögonblicksbilder på Linux
- E-postnotifieringar och åtgärder efter backup, kloning av Backup Sets, export och import av inställningar
- Flera PBS-servrar, med fastlåst certifikatfingeravtryck (förtroende vid första användning), inloggning med API-token eller användare/lösenord
- **Kryptering på klientsidan**: PBS-nyckelfil per server (AES-256-GCM, signerat manifest), valfritt skydd med lösenfras, se [Kryptering på klientsidan](#-kryptering-på-klientsidan)
- Administratörsskydd för ändring av Backup Sets, med Restart as administrator
- Felsökningsloggning för diagnostik

### Kommandoradsverktyg
- `proxmoxbackup-directory`: mappbackup (pxar) med deduplicering och `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: livebackup av hela diskar på Windows- (VSS) och Linux-maskiner (elastio-snap/dattobd) (fidx, inkrementell, ledigt utrymme hoppas över utom med `-readfreespace`, `-type vm` för en Proxmox VE-backup)
- `proxmoxbackup-nbd`: NBD-server för återställning av diskbackuper (Linux)
- `ProxmoxBackupClientSVC` (Windows) och `proxmoxbackupclient` (Linux-paketen): `-list-sets`, `-undelete` och `-rollback` för Backup Sets som definierats i GUI:t

De tre första tar `-keyfile` och `-keyfile-passphrase` för krypterade ögonblicksbilder. Fullständig referens: [kommandoradsmanualen](docs/manual/CLI.md).

### 📸 Skärmbilder

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets**: namngivna jobb med en märkning för det schema som körs härnäst (DAILY, WEEKLY, MONTHLY, +N för de övriga) och en typmärkning för mapp/maskin, Run Now på varje

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Backupförlopp i realtid**: genomströmning, beräknad tid kvar och antal nya kontra återanvända chunks medan den körs

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restore**: välj en PBS-server och ett Backup ID, lista ögonblicksbilder eller sök i dem efter filnamn

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**Scheman**: flera per set, från några minuters intervall till en gång om året (”varje månad den sista dagen”, ”den 2:a tisdagen”), med de nästa 3 körningarna

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Backup som VM i Proxmox VE**: välj typen virtuell maskin och ett numeriskt ID; PBS-backup-ID:t visas medan du skriver

</td>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Backup Sets som VM:ar i Proxmox VE**: ett set som lagras som `vm`-backup har märkningen `MACHINE (VM)`

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/oneoff-as-vm.png" width="280">](docs/screenshots/oneoff-as-vm.png)
**Engångsbackuper**: samma alternativ i formuläret för engångsbackup

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**Selektiv återställning**: bocka för enskilda filer/mappar i ögonblicksbildens träd och återställ på plats eller någon annanstans

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Reports**: full körhistorik med en detaljpanel (tid, chunks, säkerhetskopierade mappar)

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-message-log.png" width="280">](docs/screenshots/gui-message-log.png)
**Message Log**: ett begränsat diagnostikflöde med tidsstämplar för alla backuper och återställningar

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**Multi-PBS**: hantera flera PBS-servrar, testa anslutningen, välj en standardserver

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-edit-server.png" width="280">](docs/screenshots/gui-preferences-edit-server.png)
**Lägg till/redigera en server**: URL, användare/lösenord eller API-token, datastore och namespace

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-theme-amber.png" width="280">](docs/screenshots/gui-theme-amber.png)
**Teman**: fem inbyggda färgpaletter plus en egen färg, som färgar om hela appen direkt (se raden nedan)

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-advanced.png" width="280">](docs/screenshots/gui-preferences-advanced.png)
**Backupalternativ**: ärligt förklarade: parallell extrahering vid återställning är experimentell och uppmättes som *långsammare* på vår egen testhårdvara

</td>
</tr>
</table>

**🎨 Färgteman**: själva dialogrutan Preferences färgas om vid varje val, inte bara ett färgprov:

<table>
<tr>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-amber.png" width="180">](docs/screenshots/gui-theme-amber.png)
Amber

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-blue.png" width="180">](docs/screenshots/gui-theme-blue.png)
Classic Blue

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-green.png" width="180">](docs/screenshots/gui-theme-green.png)
Green

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-red.png" width="180">](docs/screenshots/gui-theme-red.png)
Red

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-dark.png" width="180">](docs/screenshots/gui-theme-dark.png)
Dark

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-custom.png" width="180">](docs/screenshots/gui-theme-custom.png)
Custom

</td>
</tr>
</table>

**🌍 Flerspråkigt**: sidan Reports på franska, italienska och spanska (och 15 till, bland dem engelska, tyska och polska):

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

### Smarta systemundantag (mappbackup)
Mappbackuper utelämnar följande av sig själva:

**Systemmappar:** `System Volume Information` (VSS-lagring, kan nå över 100 GB) och `$RECYCLE.BIN` var de än ligger; `Recovery` bara i roten av en enhet; `.pbs-rollback` (säkerhetsmappen för Roll back).
**Systemfiler:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` och `DumpStack.log.tmp`.

**Varför det spelar roll:** en enhet kan visa 1,03 TB använt medan de riktiga filerna är ~141 GB. Utan undantag skulle backupen ta med VSS-ögonblicksbilder (bortslösat utrymme och tid); med dem stämmer storleken med de verkliga data.

**Rekommendation:** använd en **mappbackup** med dessa undantag för backup på filnivå; använd en **maskinbackup** i ett separat Backup Set för bare-metal-återställning och P2V (det är en avbild av hela disken; ledigt utrymme och växlings- och vilolägesfilerna går in som nollor).

### Säkerhet och kvalitet
- Valfri kryptering på klientsidan av varje chunk, med ett signerat manifest
- Inställningsmappen bara öppen för administratörer på Windows, privat för ägaren på Linux
- Tjänstens lokala API lyssnar bara på 127.0.0.1 och kräver en token, som förnyas varje gång tjänsten startar
- Validering av indata och sanering av inloggningsuppgifter
- Skydd mot path traversal
- Logik för återförsök med exponentiell backoff
- Felhantering och enhetstester i varje modul, plus valfria livetester och end-to-end-tester mot en riktig PBS

## 🔐 Kryptering på klientsidan

Ögonblicksbilder kan krypteras innan de lämnar maskinen, med en Proxmox Backup Server-nyckelfil (formatet som `proxmox-backup-client key create` skriver, så nycklarna är utbytbara med den officiella klienten). Varje chunk krypteras med AES-256-GCM och manifestet signeras. PBS-servern ser aldrig nyckeln.

**GUI:** öppna **Preferences > Account Information** (inställningar > kontouppgifter), redigera en server och använd dess flik **Encryption** (kryptering). Använd **Import...** för en nyckel du redan har (en kopia av nyckelfilen, eller en pappersnyckel som läses med kameran, från en skanning eller ett foto, eller skrivs in) eller **Generate...** för att skapa en ny. Fliken visar nyckelns fingeravtryck. Reports visar om varje backup var krypterad. **Utan nyckelfilen kan en ögonblicksbild inte återställas: spara en kopia på ett säkert ställe, utanför den här maskinen.**

**Lösenfrasskyddade nycklar (scrypt eller PBKDF2):** välj hur appen låser upp nyckeln:
- **Ask me** (fråga mig): lösenfrasen efterfrågas en gång per appsession och hålls bara i minnet. Schemalagda backuper och backuper vid uppstart frågar aldrig: de misslyckas direkt, med ett tydligt meddelande, tills lösenfrasen har angetts i den sessionen. En backup som körs via bakgrundstjänsten misslyckas också direkt med en Ask-nyckel.
- **Remember on this computer** (kom ihåg på den här datorn): lösenfrasen sparas i konfigurationen som PBS-token, så den som kan läsa `config.json` kan låsa upp nyckeln. Det här är valet för obevakade schemalagda jobb och för bakgrundstjänsten.

Återställningen frågar efter lösenfrasen när du väljer en krypterad ögonblicksbild.

**CLI:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` behövs bara för en skyddad nyckel och efterfrågas om den utelämnas. `proxmoxbackup-machine` och `proxmoxbackup-nbd` tar samma två flaggor; `proxmoxbackup-nbd` behöver dem för att koppla in en krypterad diskavbild och kontrollerar den första chunken innan den rör NBD-enheten, så en fel nyckel misslyckas direkt.

**Bare-metal-återställning:** återställnings-ISO:n frågar efter nyckeln (ingen, en nyckelfil på en USB-sticka eller en inskriven sökväg) och efter lösenfrasen för en skyddad nyckel. Nyckeln skrivs aldrig till ISO:n. Se [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md).

**Kända begränsningar**
- En nyckel som genereras av GUI:t skrivs med en skyddad ACL (bara aktuell användare, SYSTEM och Administrators). Nyckelfiler som du förvarar någon annanstans får de behörigheter som deras mapp ger dem.
- `config.json` innehåller PBS-token, lösenord och eventuella sparade lösenfraser i klartext. Från v0.9.0 är dess mapp bara öppen för administratörer (Windows) eller bara för ägaren (Linux); till och med v0.8.2 kunde lokala Users läsa den på Windows, så begränsa åtkomsten själv på en delad maskin.
- För att återställa en krypterad ögonblicksbild behövs samma nyckelfil; det finns ingen räddning om den går förlorad.
- Flödet med Ask-nyckel och tjänsten är separata processer: använd Remember för allt som körs via tjänsten.

## Snabbare mappbackup: ändringsdetektering

I standardläget Legacy läser varje mappbackup varje fil, så en körning tar lika lång tid som att läsa hela mappen även när inget har ändrats. Backup Sets (och CLI:t) kan i stället använda ändringsdetekteringslägena i den officiella `proxmox-backup-client`. Läget Metadata används dagligen på en server med 480,9 GB och 761 866 filer, där en körning med få ändringar tar 10 till 15 minuter i stället för över 4 timmar.

| Läge | Läser | Lagras som |
|---|---|---|
| **Legacy** (standard) | alla filer | en `<name>.pxar.didx` per mapp, som tidigare |
| **Data** | alla filer | ett uppdelat arkiv: `<name>.mpxar.didx` (metadata) + `<name>.ppxar.didx` (filinnehåll) |
| **Metadata** | bara nya och ändrade filer | samma uppdelade arkiv; filer vars storlek och ändringstid (på nanosekunden), läge och ägare stämmer med föregående backup tas från den utan att läsas |

I läget **Metadata** läser den första körningen allt (det finns ännu inget att jämföra med); senare körningar läser bara det som ändrats. En fil som ändrats på ett sätt som behåller storlek och ändringstid märks inte, och därför kan ett set också **läsa varje fil var N:e körning** (**Read every file every N runs**, en Data-körning): den körningen fångar sådana ändringar och tar bort den utfyllnad som återanvändningen lämnar i arkivet. En fil som inte kunde läsas helt (fylld med nollor efter ett läsfel) läses alltid om vid nästa körning. Återanvändning sker bara från en tidigare backup som krypterats på samma sätt, med samma nyckel.

Eftersom oförändrade data varken läses eller skickas minskar också trafiken till en fjärr-PBS eller extern PBS till lite mer än det som faktiskt ändrats.

**GUI:** redigera ett Backup Set, fliken **Destination**, **Change detection**. Det gäller bara mappsets: maskinbackuper (hel disk) är diskavbilder och påverkas inte, så bare-metal-återställning och återställning i Proxmox VE fungerar precis som förut. Engångsbackuper använder Legacy. Bara körningar som slutförs utan fel räknas mot nästa fullständiga läsning; listan över Backup Sets visar ”Next full read in N runs”. Förloppskortet talar om när och varför en körning läser varje fil. **Reports** visar varje körnings läge och hur många oförändrade filer som återanvändes.

**Ett Backup ID per set.** Läget Metadata jämför med den nyaste ögonblicksbilden i backupgruppen. Om två sets (eller två maskiner) använder samma Backup ID på samma server jämför varje körning med det andras ögonblicksbild och läser allt. Redigeraren för Backup Sets varnar för detta.

**CLI:** `-change-detection-mode legacy|data|metadata`, eller `"change-detection-mode"` i JSON-konfigurationen. Det finns ingen CLI-flagga för ”var N:e körning”: schemalägg i stället en körning med `-change-detection-mode data` då och då.

**Innan du byter ett set till Data eller Metadata,** uppdatera alla maskiner du återställer från till v0.6.0 eller senare: byggen från före stödet för uppdelade arkiv kan inte läsa dem (de visar mapparna utan deras filer). Den officiella `proxmox-backup-client` återställer dem byte för byte identiskt (testat med 3.4.9), och filbläddraren i PBS 4.2 öppnar dem. Den första uppdelade backupen av en mapp delar lite med dess äldre Legacy-backuper, så den kostar ungefär ytterligare en fullständig kopia av den mappen i datastoren tills de gamla ögonblicksbilderna har rensats bort.

## 🚀 Snabbstart (GUI)

1. Installera: MSI på Windows, `.deb` eller `.rpm` på Linux, eller packa upp zip- eller tar.gz-filen från [releaserna](https://github.com/mjb-is/proxmoxbackupclient_go/releases) (se [Ladda ner och installera](#-ladda-ner-och-installera))
2. Starta den. På Windows, **Kör som administratör** för VSS, maskinbackup och ändring av Backup Sets. På Linux fungerar mappbackup av dina egna filer som din användare; använd **Proxmox Backup Client (administrator)** (eller kör som root) för maskinbackup och blockögonblicksbilder
3. Lägg till din PBS-server under **Preferences > Account Information** och klicka på **Test**
4. Skapa ett Backup Set (eller en engångsbackup) och välj de mappar eller diskar som ska säkerhetskopieras
5. Kör det. För att scheman ska köras utan att någon är inloggad, slå på **Preferences > Advanced > Run as Service**

[GUI-manualen](docs/manual/GUI.md) går igenom varje steg.

## Fullständig livebackup av maskinen

`proxmoxbackup-machine` (och GUI:ts maskinbackuper) säkerhetskopierar ett komplett Windows 10/11- eller Windows Server-system, eller en Linux-maskin, utan något driftstopp.

Kommandosyntaxen är i stort sett densamma som för mappbackup, förutom att `-backupdir` ersätts av `-backupdev` (upprepa den för flera diskar).
Ett anrop kan till exempel se ut så här:

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Kommandot ovan tittar på disk 0, hittar alla monterade partitioner, tar en VSS-ögonblicksbild av dem och skapar sedan en startbar backupavbild av hela disken som fidx. Ledigt utrymme och växlings- och vilolägesfilerna går in i avbilden som nollor utan att läsas (på Linux även ledigt utrymme i ext2/3/4 och XFS); `-readfreespace` läser i stället allt, för en forensisk kopia. En sektor som inte går att läsa går in i avbilden som nollor, och körningen anger var. Lägg till `-type vm` för att lagra backupen som en VM-backup för Proxmox VE ([P2V](#-fysiska-maskiner-till-virtuella-maskiner-p2v)).

Nästa backup blir inkrementell; hashningen har parallelliserats så att hastigheter på 1 GB/s lätt kan nås.

### Förutsättningar för maskinbackup på Linux (motsvarigheten till VSS)

Maskinbackup på Windows använder VSS (därav ”Kör som administratör” ovan); Linux har ingen VSS, så det här
projektet använder kärnmodulen `elastio-snap` (en fork av `dattobd`) för samma uppgift: en
konsekvent ögonblicksbild vid en given tidpunkt av en blockenhet i drift, medan det fortsätter att skrivas till den. Det ursprungliga
förrådet `elastio/elastio-snap` är arkiverat; den underhållna versionen är
[`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap), och det är den som ska installeras. `dattobd`
stöds också om den redan är laddad.

Båda följande krävs, oberoende av varandra, annars vägrar en maskinbackup på Linux att köras:

1. **Kärnmodulen måste vara installerad och laddad.** Den dokumenterade installationen av `elastio-snap` via repopaket
   (se [dess INSTALL.md](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md)) pekar för närvarande på
   en flyttad eller trasig URL, åtminstone för Ubuntu 22.04 (bekräftat 2026-09-24: repopaketet
   ger 404 efter en omdirigering). Att bygga från källkod fungerar däremot pålitligt:
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
   Det här bygger och laddar modulen bara för den aktuella uppstarten; läs din distributions dokumentation för att ladda den automatiskt
   vid varje start (t.ex. en post i `/etc/modules-load.d/`) om du vill det.

2. **Själva backupprocessen måste köras som root (EUID 0).** Även med modulen laddad vägrar `CreateVSSSnapshot`
   (ögonblicksbildsvägen för Linux trots det Windows-centrerade namnet) uttryckligen att köras som en användare som inte är root:
   en konsekvent ögonblicksbild av hela disken kräver modulens kontrollgränssnitt som bara root får använda, så detta är ett hårt
   krav, inget som medlemskap i gruppen `disk` eller liknande kan ersätta. För en maskinbackup, använd
   menyposten **Proxmox Backup Client (administrator)**, bakgrundstjänsten, eller kör GUI:t eller
   `proxmoxbackup-machine` med `sudo` (eller som root); en vanlig fil- eller mappbackup har inget sådant krav.

Om en maskinbackup misslyckas med `backup device /dev/sdX: open /dev/sdX: permission denied` beror det oftast
bara på att din användare behöver vara med i gruppen `disk` för grundläggande läsåtkomst (`sudo usermod -aG disk $USER`, logga sedan
ut och in), men enbart det räcker INTE för en riktig ögonblicksbild; du stöter ändå på root-kravet ovan
direkt efteråt.

### Filåterställning från en diskbackup

GUI:ts sida Restore öppnar en maskinbackup som en mappbackup: varje volym i avbilden (NTFS, FAT, exFAT, ext2/3/4 eller XFS) är en mapp du kan bläddra i och återställa från. Inget monteras.

På Linux kan du också koppla in en diskbackup som blockenhet med nbd-verktyget.

För att använda nbd, kör först `modprobe nbd max_part=0`.

Av okända skäl orsakar `max_part != 0` en oändlig loop av partitionssökning.

NBD-verktyget kopplar in vilken fast diskbackup som helst, oavsett om det är en VM- eller host-backup (med det sagt fungerar det även för PVE-backuper).

Använd det med en kommandorad som liknar den här:
`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Om du utelämnar `-path` visas ett terminalgränssnitt där du kan välja fidx-filen.

> Använd inte detta på en maskin som kör något viktigt: ett korrupt filsystem kan krascha operativsystemet, och det är därför Proxmox VE använder en QEMU-instans för detta.

Avmontera allt på nbd-disken innan du stoppar `proxmoxbackup-nbd`, annars förblir partitionen troligen upptagen och omöjlig att avmontera.

Om du får felet `Device or resource busy` måste du tvinga fram en frånkoppling genom att köra `nbd-client -d /dev/nbd0` eller helt enkelt starta om.

### Återställning till en fysisk maskin eller en VM

ISO:n för bare-metal-återställning har en helt automatisk startmenypost **”PBS Bare Metal Restore”**: se [ISO för bare-metal-återställning](#-iso-för-bare-metal-återställning) nedan. Det är den första posten, standardvalet när tiden går ut, och den hoppar över Clonezillas egna frågor om språk och tangentbord: ange dina PBS-anslutningsuppgifter, välj en ögonblicksbild, bekräfta den hittade måldisken, så sköter den nätverksinställningen, NBD-anslutningen, själva återställningen och en automatisk omstart utan fler inmatningar. Samma ISO återställer till en tom virtuell maskin för [P2V](#-fysiska-maskiner-till-virtuella-maskiner-p2v).

Den äldre, halvmanuella vägen finns också kvar på samma ISO, som en egen post i Clonezillas vanliga meny för lägesval (”pbs-nbd”): den guidar dig genom PBS-anslutningsuppgifterna och kopplar den valda ögonblicksbilden till `/dev/nbd0` i bakgrunden, och låter dig sedan sköta själva återställningen för hand i Clonezillas egen vanliga guide för diskåterställning.

## Användning: mappbackup

Den fullständiga referensen, med exempel för Windows och Linux, slutkoder och schemaläggning, finns i **[kommandoradsmanualen](docs/manual/CLI.md)**. Ett typiskt kommando:

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
  -no-mtime                  Record 1970-01-01 as every file and folder time (advanced; see the CLI manual)
  -backupstream string       Back up standard input as <name>.didx instead of a directory
  -pxarout string            Also write the archive to a local file (debugging)
  -mail-host, -mail-port, -mail-username, -mail-password, -mail-insecure,
  -mail-from, -mail-to, -mail-subject-template, -mail-body-template
                             Email report (optional)
  -config string             JSON config file; flags given on the command line override it
```

Den officiella klientens miljövariabler `PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD` och `PBS_FINGERPRINT` läses också.

För JSON-konfiguration finns ett JSON-exempel; fyll bara i de fält som behövs.

Om mejlmallar:
[Gos mallmotor](https://pkg.go.dev/text/template) används för mejlens ämnesrader och brödtext; se dess dokumentation för syntaxen.
Följande variabler finns tillgängliga i mallarna:

- `.NewChunks`: antal nya chunks som skapats
- `.ReusedChunks`: antal återanvända chunks
- `.Datastore`: datastorens namn
- `.Error`: felet, om något (`.ErrorStr` ger det som text, tomt när det inte finns något)
- `.ReadErrors`: de filer eller mappar som inte kunde läsas; `.ReadErrorCount` anger hur många
- `.Hostname`: maskinens värdnamn
- `.StartTime`: när backupen startade
- `.EndTime`: när backupen slutade
- `.Duration`: backupens varaktighet
- `.FromattedDuration`: backupens formaterade varaktighet (namnet stavas så här i koden)
- `.Success`: sant när backupen slutfördes utan fel och utan något oläsbart
- `.Partial`: sant när en ögonblicksbild sparades men vissa filer inte kunde läsas
- `.Status`: `Success`, `Partial` eller `Failed`

## Strömbackup

Gör det möjligt att säkerhetskopiera en ström i stället för en pxar, vilket ger oändliga möjligheter. Du kan till exempel köra:

```
mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]
```

Det gör att buzhash kan användas för deduplicering även när du till exempel använder tar, eller själva SQL-dumpen.

## Kända problem

När Windows Defenders skydd mot skadlig kod är aktivt saktas backupen ner till upp till 25 % av den möjliga hastigheten.

På Windows tar kommandoradsverktygen ett processlås: en andra backup som startas medan en körs skriver ut ”Another backup job is running” och avslutas. På Linux finns inget sådant lås, så schemalägg inte två CLI-körningar samtidigt.

## 📋 Förutsättningar

- Windows 10/11 eller Windows Server (64-bitars), med administratörsrättigheter för VSS och maskinbackup
- eller Linux (paketen är för Debian, Ubuntu, Mint och Fedora), med körtidsmiljön WebKitGTK för GUI:t, samt root och kärnmodulen elastio-snap eller dattobd för konsekventa maskinbackuper
- Nätverksåtkomst till en Proxmox Backup Server
- För P2V: Proxmox VE med PBS-datastoren tillagd som lagring, eller valfri hypervisor som kan starta återställnings-ISO:n

## 🔨 Bygga från källkod

**[BUILD.md](BUILD.md)** har de fullständiga instruktionerna: GUI:t och bakgrundstjänsten, kommandoradsverktygen, MSI:n och Linux-paketen. I korthet, med Go 1.25 eller senare och Wails CLI v2.13:

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

Det byggda gränssnittet (`gui/frontend/dist`) är incheckat, så Node.js behövs bara för att ändra gränssnittet (`npm install` och `npm run build` i `gui/frontend`, eller `wails dev` för omladdning i farten). Tester: `make test`.

## 🔧 Avancerad användning och guider

### Multi-PBS (flera PBS-servrar)

Konfigurera flera PBS-servrar och välj mål per Backup Set eller engångsbackup (t.ex. `C:\Users` till en snabb SSD-PBS dagligen, `D:\` till en PBS för stora datamängder varje vecka, plus en server för katastrofåterställning).

- **[Användarguide](MULTI_PBS_USER_GUIDE.md)**: lägga till och testa servrar, standardservern, välja server per backup, vanliga frågor och felsökning.
- **[Implementationsguide](MULTI_PBS_GUIDE.md)**: datamodell, automatisk migrering från en konfiguration med en enda PBS, backend-metoder.

En äldre konfiguration med en enda PBS migreras automatiskt till en `default`-server vid första inläsningen.

### 💿 ISO för bare-metal-återställning

Återställnings-ISO:n är en vanlig Clonezilla Live-ISO som patchats med binärerna `pbsnbd` / `machinebackup` plus två menyposter. Den startar från CD, från en USB-sticka (skriven med `dd`, eller från en Ventoy-sticka), över PXE, och på UEFI- eller BIOS-maskiner:

- **`ocs-pbs-bare-metal-restore`**: den första startmenyposten, standardvalet när tiden går ut, **”Proxmox Backup Client Go - PBS Bare Metal Restore”**. Den hoppar över Clonezillas frågor om språk och tangentbord och dess allmänna guide, och går direkt från PBS-inloggningsuppgifter till en slutförd återställning med automatisk omstart: DHCP först (en fråga om statisk IP bara om det misslyckas), NTP-synkronisering, en krypteringsnyckel från en USB-sticka om backupen är krypterad, och identifiering av måldisk som aldrig erbjuder stickan den startade från. När det lyckats visar den ett meddelande om att återställningen är klar och startar om. Den återställer Windows- och Linux-backuper på samma sätt (återställningen sker på blocknivå), till fysisk hårdvara eller till en tom VM.
- **`ocs-pbs-nbd`**: en post **”pbs-nbd”** i Clonezillas egen meny för lägesval. Frågar efter dina PBS-anslutningsuppgifter och ögonblicksbilden som ska återställas, kopplar den till `/dev/nbd0` i bakgrunden och lämnar sedan över till Clonezillas vanliga guide för diskåterställning som du sköter för hand.

Också rättat på återställningssidan: ett dödläge i `zstd.NewReader` i PBS-vägen för hämtning av chunks som fick återställningar av hela maskiner att hänga sig (bidraget till originalprojektet som [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85)), ett fel där ”första återställningspasset misslyckas med 'no partition', ett nytt försök lyckas” som orsakades av att Clonezillas egen `is_disk_without_part_and_fs()` tävlade med udev, och BIOS/MBR-maskiner som inte startade efter en återställning (data mellan MBR och den första partitionen återställs nu också).

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

Alla detaljer (varför en fullständig ombyggnad av ISO:n i stället för ett utbyte på plats, förutsättningar, menyflöde, verifiering) finns i **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**; hur den används står i **[manualen för bare-metal-återställning](docs/manual/BMR.md)**.

### Bygga Windows-GUI:t

**Inbyggt i Windows**: se **[BUILD.md](BUILD.md)** för verktygskedjan (Go, MinGW, Wails) och kommandona för appen, tjänsten och MSI:n.

**Docker (vid bygge på Linux).** `./build_gui_windows_docker.sh` bygger Windows-appen som `ProxmoxBackupClientGO.exe`, med korrekt WebView2-stöd, i en engångscontainer av typen `golang` (installerar mingw och Wails, bygger gränssnittet, kör `wails build`). Den bygger inte bakgrundstjänsten eller MSI:n.

### Funktionsstatus, ändringslogg och interna dokument

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**: vad som är klart, delvis klart eller inte gjort, och sedan vilken version.
- **[CHANGELOG.md](CHANGELOG.md)**: ändringshistorik per version, och [Unreleased] för det som finns på master.
- **[TODO.md](TODO.md)**: öppen färdplan och idéer.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**: vad varje release innehåller.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**: MSI:ns avinstallationsdialog (behåll eller radera konfigurationen) och dess testplan.
- **[AGENTS.md](AGENTS.md)**: förrådets struktur och konventioner, för bidragsgivare och kodagenter.

## 🔁 Bidrag till originalprojektet

Rättningar och funktioner av allmänt intresse erbjuds till [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) som pull requests. Arton har slagits ihop:

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

## 🧪 Status, felrapporter och funktionsönskemål

**Det här är alfaprogramvara som tillhandahålls i befintligt skick, utan någon garanti av något slag, och används helt på egen risk.** Jag tar inget ansvar för förlorade eller skadade data, inklusive källfiler.

Den är byggd för mitt eget hemmalabb. Varje release testas av mig innan den släpps, och den används regelbundet där: schemalagda mapp- och maskinbackuper av Windows- och Linux-maskiner, med återställningar, Undelete, Roll back, bare-metal-återställningar och P2V-återställningar testade längs vägen. Ingen backup är bevisad förrän du har återställt från den, så testa dina egna återställningar, inklusive en bare-metal-återställning, innan du förlitar dig på den.

- **Felrapporter är välkomna**: öppna ett [ärende](https://github.com/mjb-is/proxmoxbackupclient_go/issues) med vad du gjorde, vad som hände och loggen (Message Log, eller filerna i inställningsmappen).
- **Funktionsönskemål**: jag lägger till funktioner som hjälper min egen användning, så jag tar inte emot önskemål för andras räkning. Skicka dem i stället till originalprojektet, [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go/issues). Pull requests som rättar fel, eller som tydligt också skulle hjälpa min användning, är välkomna.

Felrapporter och rättelser är den mest användbara hjälpen. Vill du tacka på annat sätt kan du [bjuda mig på en kaffe på Ko-fi](https://ko-fi.com/mickbeeby).

## 🙏 Tack och historik

Den ursprungliga kommandoradsklienten skrevs av Tiziano Bacocco (tizbac) från 2023. RDEM Systems grafiska klient slogs senare ihop med den, och med den kom det första GUI:t, stöd för flera PBS-servrar och Windows-tjänsten. Den här forken startade i september 2026 för att fylla de luckor som hindrade PBS från att vara mitt enda backupsystem; dess releaser börjar på v0.3.0, och rättningar av allmänt intresse går tillbaka till originalprojektet.

- Den ursprungliga klienten, [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go), av Tiziano Bacocco (tizbac) och dess bidragsgivare.
- RDEM Systems grafiska klient Nimbus Backup, utgångspunkten för appens grafiska gränssnitt.
- Arbetsflödet och känslan i Backup for Workgroups (BFW), som Backup Sets, Undelete och Roll back följer.
- [Clonezilla Live](https://clonezilla.org/), som ISO:n för bare-metal-återställning bygger på.

## 📄 Licens

GPLv3: se filen [LICENSE](LICENSE).

---

**Ändringar sedan forken © 2026 mjb-is, byggda på arbete av Tiziano Bacocco (tizbac), bidragsgivarna till Proxmox Backup Client GO och RDEM Systems (© 2024-2026).**

☕ Om den här klienten är till nytta för dig kan du [bjuda mig på en kaffe på Ko-fi](https://ko-fi.com/mickbeeby). Tack!
