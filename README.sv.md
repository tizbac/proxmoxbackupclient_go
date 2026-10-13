# Proxmox Backup Client: Windows- och Linux-klient för Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client är en backupklient med öppen källkod (GPL-3.0) för Proxmox Backup Server (PBS), som fungerar på Windows och Linux.**

Det är en **svit av verktyg** för säkerhetskopiering till PBS:

- **Proxmox Backup Client GUI** (baserat på Nimbus Backup GUI från RDEM Systems): en grafisk klient för att säkerhetskopiera Windows- och Linux-servrar och arbetsstationer till PBS, med konsekventa ögonblicksbilder (VSS på Windows, elastio-snap/dattobd på Linux), Backup Sets med scheman, backup av mappar och hela diskar, återställning, Undelete och Roll back, flera PBS-servrar och en bakgrundstjänst på Windows och Linux.
- **`proxmoxbackup-directory`**: kommandoradsverktyg för mappbackup (pxar) med deduplicering och ändringsdetektering.
- **`proxmoxbackup-machine`**: kommandoradsverktyg för livebackup av hela diskar (fidx) på Windows- (VSS) och Linux-datorer (elastio-snap/dattobd); avbilden går att starta efter en bare-metal-återställning.
- **`proxmoxbackup-nbd`**: kopplar in en diskbackup som NBD-enhet för återställning (Linux).
- En **ISO för bare-metal-återställning** byggd på Clonezilla Live.

> ⚠️ **Ansvarsfriskrivning:** detta projekt är **inte på något sätt anslutet** till **Proxmox Server Solutions GmbH**. ”Proxmox”, Proxmox-logotypen och relaterade namn tillhör respektive ägare; här används de **endast** för att ange kompatibilitet. Se [proxmox.com](https://www.proxmox.com/) för deras produkter.

> ℹ️ **Den här sidan är en översatt sammanfattning.** Referensen är den [engelska README.md](README.md), som innehåller alla detaljer. Användarmanualerna finns på engelska: [GUI](docs/manual/GUI.md), [kommandorad](docs/manual/CLI.md) och [bare-metal-återställning](docs/manual/BMR.md). Appen har inget svenskt gränssnitt, så menyer, flikar och knappar anges här med sina engelska namn.

## 📖 Användarmanualer

| Manual | Innehåll |
|---|---|
| **[GUI-manual](docs/manual/GUI.md)** | Installation, PBS-servrar, Backup Sets och scheman, inkrementell backup, förlopp, återställning, Undelete, Roll back, Reports, loggar, Preferences, bakgrundstjänsten, felsökning |
| **[Kommandoradsmanual](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: alla flaggor, konfigurationsfiler, slutkoder, schemaläggning; Undelete och Roll back från kommandoraden |
| **[Manual för bare-metal-återställning](docs/manual/BMR.md)** | Återställnings-ISO:n steg för steg; `vm`-ögonblicksbilder till Proxmox VE; annan hårdvara |
| [Återställningsscenarier](docs/RESTORE_GUIDE.md) | Vilken återställning som passar när |
| [Manualindex](docs/manual/README.md) | Översikt över de tre manualerna |

## 🆕 Nytt i v0.9.0: flera scheman per Backup Set, installationsprogram, en Linux-tjänst

- **Flera scheman per Backup Set**, från några minuters intervall till en gång om året: ”varje dag kl. 02:00” plus ”varje månad den sista dagen” eller ”den 2:a tisdagen”, varje schema i klartext med sina nästa 3 körningar. Befintliga sets behåller sitt schema.
- **Mappbackuper behåller alternativa dataströmmar och utökade NTFS-attribut** (Windows), till exempel markeringen ”hämtad från internet”, och en återställning lägger tillbaka dem. Listan över kända begränsningar är nu tom.
- **Installationsprogram**: en **Windows-MSI** (appen och bakgrundstjänsten under Program Files, post i Start-menyn, uppgradering på plats, behåll eller radera inställningarna vid avinstallation) och **Linux-paket**, `.deb` (Debian, Ubuntu, Mint) och `.rpm` (Fedora), med appen, de tre kommandoradsverktygen, manualerna och menyposter.
- **Bakgrundstjänst på Linux** (`proxmoxbackupclient.service`): schemalagda Backup Sets körs som root, även när ingen är inloggad. Slås på under Preferences > Advanced > Run as Service, som på Windows. **Restart as administrator** fungerar nu även på Linux (via pkexec).
- **Pause, Resume och Stop för backuper som tjänsten kör**, med förloppskortet i den öppna appen.
- **En varning när två Backup Sets delar backupgrupp** (samma server, typ och Backup ID), med en **SHARED ID**-märkning på båda korten.
- **Låsta inställningar**: på Windows är `C:\ProgramData\ProxmoxBackupClient` nu bara öppen för administratörer; på Linux är inställningsmappen privat för ägaren.
- **Filer ur maskinbackuper för alla vanliga filsystem**: FAT, exFAT, ext2/3/4 och XFS utöver NTFS. Återställning av Windows-avbilder tar också tillbaka behörigheter och alternativa dataströmmar.
- **Maskinbackuper på Linux hoppar över ledigt utrymme** (ext2/3/4, XFS), som Windows gör sedan v0.8.0.
- **Undelete och Roll back för Linux-maskinbackupsets**, och **från kommandoraden** för alla Backup Sets: `-list-sets`, `-undelete`, `-rollback` (avsnitt 12 i [kommandoradsmanualen](docs/manual/CLI.md)).
- Kommandoradsverktygen skriver fel till stderr i stället för att visa meddelanderutor, och `proxmoxbackup-machine` skickar sitt notifieringsmejl.
- **ISO:n för bare-metal-återställning** erbjuder aldrig stickan den startade från som disk att återställa till, och manualen förklarar hur firmware (UEFI eller BIOS) ska matcha när en fysisk maskin blir en VM.
- **Alla språk granskade** (17 översättningar, cirka 1 850 strängar), och datum och tal följer appens språk.
- v0.8.2:s snabbare hantering av tomt diskutrymme (taggad, aldrig publicerad separat) finns också med i den här versionen.

Se [CHANGELOG](CHANGELOG.md) för hela listan.

## 💽 Nytt i v0.8.0: snabbare maskinbackup, filer ur den, Pause/Resume

- **Maskinbackuper hoppar över ledigt utrymme** (och växlings- och vilolägesfilerna) på Windows, så en halvtom disk läses på ungefär halva tiden, och en **dålig sektor** stoppar inte längre backupen: den hamnar som nollor i avbilden och körningen talar om var.
- Förloppskortet visar block av totalt, disk och partition, nya / återanvända / tomma chunks och uppladdningshastigheten.
- **Återställ filer ur en maskinbackup**: varje NTFS-volym i avbilden är en mapp som du kan bläddra i, söka i och bocka för. Inget monteras eller laddas ner först; bara filtabellen och sedan de valda filernas data läses (en bärbar dators C: på 127 GB med 430 000 filer listas på 4 till 15 sekunder). En maskinbackup per bärbar dator räcker alltså både för bare-metal-återställning och för ”jag behöver tillbaka den filen”.
- **Undelete och Roll back** fungerar på maskinbackupsets (Windows), på de mappar du väljer för setet (som standard Skrivbord, Dokument, Bilder, Musik, Videoklipp och Hämtade filer).
- **Pause och Resume** för en pågående backup, en **VSS / Snapshot ON-OFF**-märkning på varje setkort, och Windows **skapandetider** säkerhetskopieras och återställs.
- **v0.8.2:** tomt diskutrymme hashas inte längre block för block, så en nästan tom disk går på ungefär en tredjedel av tiden (en testdisk på 50 GB: 3m36s, nu 1m15s).

## 🗑️ Nytt i v0.7.0: Undelete och Roll back

Två nya sidor i GUI:t, för mappbackupsets:

- **Undelete** hittar filer som finns i ett sets backuper men inte längre på disken (i den senaste backupen eller i alla från de senaste 7 eller 30 dagarna), visar dem som ett mappträd med när varje fil försvann (och ”probably moved to ...” när samma fil finns någon annanstans), och återställer de markerade där de låg eller till en annan mapp. Efter att 39 GB raderats från en utdelning med 761 000 filer hittade en 7-dagarssökning exakt de filerna på ungefär en minut.
- **Roll back** återställer ett sets mappar till hur de såg ut vid en vald backup. Den visar först vad som skulle ändras (saknade, ändrade, tillkomna sedan dess) och återställer sedan. Allt som ersätts eller tas bort flyttas först undan på samma enhet, så **Undo** lägger tillbaka allt.

Bara backupernas fillistor och mapparnas listningar läses; ingen fil öppnas förrän den återställs. Också nytt: bekräftelsedialoger i appen, Next run på varje setkort, ett föreslaget Backup ID per set (`deepthought-data`), unika setnamn och bakgrundstjänsten med i Windows-nedladdningen. Se [GUI-manualen](docs/manual/GUI.md#10-undelete).

## ⚡ Nytt i v0.6.0: inkrementell mappbackup

Mappbackuper kan hoppa över oförändrade filer, med samma **ändringsdetektering** som den officiella `proxmox-backup-client`. I läget **Metadata** jämför klienten varje fils storlek, ändringstid (på nanosekunden), läge och ägare med föregående ögonblicksbild. Oförändrade filer öppnas inte ens: deras data återanvänds. Bara nya och ändrade filer läses och laddas upp.

Uppmätt på en riktig filserver (480,9 GB, 761 866 filer på USB-diskar):

| Körning | Tid | Läser | Nya chunks uppladdade |
|---|---|---|---|
| Första Metadata-körningen (inget att jämföra med) | 4 h 17 min | alla filer | 135 549 |
| Följande körningar | cirka 10 till 15 min | bara ändrade filer; alla 761 866 oförändrade återanvänds | 4 till 40 |

Det minskar också trafiken till en fjärr-PBS till lite mer än det som faktiskt ändrats. Så använder du det: redigera ett mapp-Backup Set, fliken **Destination**, **Change detection: Metadata**. Ställ in **Read every file every N runs** så att var N:e körning läser allt (till exempel 42 för ett set som körs var 4:e timme, ungefär en gång i veckan). Ge varje set **ett eget Backup ID**. Kommandorad: `-change-detection-mode metadata`. Legacy är fortfarande standard.

## 🤝 Kompatibel med den officiella Proxmox-klienten, åt båda hållen

Verifierat med `proxmox-backup-client` 3.4.9 och PBS 4.2:

- **Legacy-, Data- och Metadata-ögonblicksbilder från den här klienten återställs byte för byte identiskt med den officiella klienten**, från både GUI och kommandorad, gjorda på Windows eller Linux, och filbläddraren i PBS 4.2 öppnar dem. På Linux behåller de återställda filerna även exakt läge, ändringstid på nanosekunden och mål för symboliska länkar.
- **Legacy-, Data- och Metadata-ögonblicksbilder från den officiella klienten återställs identiskt med den här klienten**, helt eller selektivt.
- Samma uppdelade arkivformat (pxar version 2), samma lägen för ändringsdetektering, samma nyckelfilsformat (nycklar från `proxmox-backup-client key create` fungerar här, och tvärtom), samma miljövariabler i stil med `PBS_REPOSITORY`.

Dina backuper är alltså aldrig bundna till den här klienten. Det enda extra är en liten fil i roten av en mappögonblicksbild, `.proxmox_backup_client_meta.json` (ursprunglig sökväg), som den officiella klienten återställer som en vanlig fil.

## 📦 Ladda ner och installera

👉 **[Ladda ner senaste versionen](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| Du har | Använd |
|---|---|
| Windows | **MSI-installationsprogrammet** (`ProxmoxBackupClient-<version>-x64.msi`) installerar appen och bakgrundstjänsten under Program Files, med en post i Start-menyn. Eller packa upp `ProxmoxBackupClient-v<version>-windows-amd64.zip` (appen och `ProxmoxBackupClientSVC.exe`; ha dem i samma mapp) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Annan Linux | Packa upp `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (kräver WebKitGTK) |
| Bara kommandorad | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` eller `-linux-amd64.tar.gz` |

MSI, `.deb` och `.rpm` är nya på master: releaser till och med v0.8.1 har bara zip- och tar.gz-filerna. Tills en release innehåller installationsprogrammen bygger du dem enligt [BUILD.md](BUILD.md).

Linux-paketen installerar två menyposter: **Proxmox Backup Client** kör appen som du (mappbackup av dina egna filer), och **Proxmox Backup Client (administrator)** kör den som root och frågar efter ditt lösenord via polkit, för maskinbackup och blockögonblicksbilder. Bakgrundstjänsten installeras men slås inte på; se [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **Visar Windows ”virus upptäckt” (t.ex. `Trojan:Win32/Sabsik.FL.A!ml`) eller en SmartScreen-varning?** Det är ett **känt falsklarm** för osignerade och ovanliga Go/Wails-program, *inte* ett virus. Mer information i [originalprojektet](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Verifiera en nedladdning

Varje release från den här forken innehåller en `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-v0.8.1-windows-amd64.zip -Algorithm SHA256   # compare with SHA256SUMS.txt
```

På Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`. Forkens egna byggen har ännu ingen kodsignering, CI-signerad attestering eller VirusTotal-skanning (se [TODO.md](TODO.md)).

## ✨ Funktioner

### GUI: Proxmox Backup Client GUI (rekommenderas)
- **🌍 Flerspråkigt**: 18 gränssnittsspråk (franska, engelska, italienska, tyska, polska, spanska, bulgariska, tjeckiska, grekiska, ungerska, lettiska, litauiska, nederländska, portugisiska, rumänska, slovakiska, turkiska och ukrainska)
- Inbyggd menyrad och sidomeny (Backup, Restore, Undelete, Roll back, Reports, Message Log, Preferences, About)
- **Backup Sets**: namngivna jobb med dagligt / intervall / manuellt schema, typmärkning och VSS / Snapshot-märkning, ”Run Now” och varningen **SHARED ID**
- **Engångsbackup** av mappar eller hela diskar; sidan **Reports** med full historik och en begränsad **Message Log**
- **Preferences** med fem färgteman (Amber, Classic Blue, Green, Red, Dark) och en egen färg
- **Inkrementell mappbackup** (Metadata), med valfri fullständig läsning var N:e körning
- **Maskinbackup** på Windows och Linux som hoppar över ledigt utrymme och klarar dåliga sektorer, lagrad som `host` eller som Proxmox VE-ögonblicksbild av typen `vm`
- **Restore**: bläddring i ögonblicksbilder, filsökning (jokertecken, reguljära uttryck, hela sökvägar) och selektiv återställning, med NTFS/POSIX-ACL:er, mapptider, Windows skapandetider och valfri verifiering
- **Filer ur maskinbackuper** (NTFS; FAT, exFAT, ext2/3/4 och XFS på master)
- **Undelete** och **Roll back** (från v0.7.0), för mapp- och maskinsets, med förhandsvisning och Undo
- **Pause, Resume och Stop** för en pågående backup
- **Run as Service** under Preferences > Advanced, så att scheman körs utan att någon är inloggad (Windows från v0.7.0; Linux på master)
- Live-förloppskort, VSS på Windows, elastio-snap/dattobd-blockögonblicksbilder på Linux
- E-postnotifieringar och åtgärder efter backup, kloning av Backup Sets, export och import av inställningar
- Flera PBS-servrar, med fastlåst certifikatfingeravtryck, API-token eller användare/lösenord
- **Kryptering på klientsidan** med en PBS-nyckelfil per server
- Administratörsskydd för ändring av Backup Sets (Windows), med Restart as administrator

### Kommandoradsverktyg
- `proxmoxbackup-directory`: mappbackup (pxar) med deduplicering och `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: livebackup av hela diskar på Windows (VSS) och Linux (elastio-snap/dattobd) (fidx, inkrementell, ledigt utrymme hoppas över utom med `-readfreespace`)
- `proxmoxbackup-nbd`: NBD-server för återställning av diskbackuper (Linux)
- `ProxmoxBackupClientSVC` (Windows) och `proxmoxbackupclient` (Linux-paketen): `-list-sets`, `-undelete` och `-rollback` för Backup Sets som definierats i GUI:t (på master)

De tre första tar `-keyfile` och `-keyfile-passphrase`. Fullständig referens: [kommandoradsmanualen](docs/manual/CLI.md).

### 📸 Skärmbilder

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets**: namngivna jobb med schema- och typmärkning, Run Now på varje

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Live-förlopp**: genomströmning, beräknad tid kvar och nya kontra återanvända chunks

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restore**: välj PBS-server och Backup ID, lista ögonblicksbilder eller sök efter filnamn

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**Selektiv återställning**: bocka för filer eller mappar i trädet och återställ på plats eller någon annanstans

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Reports**: full körhistorik med detaljpanel (tid, chunks, säkerhetskopierade mappar)

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**Multi-PBS**: hantera flera PBS-servrar, testa anslutningen, välj en standardserver

</td>
</tr>
</table>

Fler skärmbilder (`vm`-ögonblicksbilder, teman, språk) i den [engelska README-filen](README.md).

### Smarta systemundantag (mappbackup)
Följande utelämnas automatiskt: `System Volume Information` och `$RECYCLE.BIN` var de än ligger, `Recovery` bara i roten av en enhet (på master), `.pbs-rollback` (säkerhetsmappen för Roll back), samt filerna `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` och `DumpStack.log.tmp`. En enhet kan visa 1,03 TB använt fast de riktiga filerna är ~141 GB; utan undantag skulle backupen ta med VSS-ögonblicksbilderna. Rekommendation: **mappbackup** för filerna och **maskinbackup** i ett separat Backup Set för bare-metal-återställning.

### Säkerhet och kvalitet
- Valfri kryptering på klientsidan av varje chunk, med signerat manifest
- Inställningsmappen bara öppen för administratörer på Windows, privat för ägaren på Linux (på master)
- Tjänstens lokala API lyssnar bara på 127.0.0.1 och kräver en token som förnyas vid varje start
- Validering av indata, sanering av inloggningsuppgifter, skydd mot path traversal, återförsök med exponentiell backoff
- Enhetstester i varje modul, plus valfria livetester och end-to-end-tester mot en riktig PBS

## 🔐 Kryptering på klientsidan

Ögonblicksbilder kan krypteras innan de lämnar datorn, med en Proxmox Backup Server-nyckelfil (formatet som `proxmox-backup-client key create` skriver, så nycklar fungerar i båda klienterna). Varje chunk krypteras med AES-256-GCM och manifestet signeras. PBS-servern ser aldrig nyckeln.

**GUI:** öppna **Preferences > Account Information**, redigera en server och använd fliken **Encryption**. Välj en befintlig nyckelfil eller **Generate...** en ny. För en lösenfrasskyddad nyckel väljer du **Ask me** (en gång per appsession; schemalagda backuper och tjänstens backuper misslyckas direkt utan den) eller **Remember on this computer** (sparas i `config.json`; valet för obevakade jobb och för tjänsten). **Utan nyckelfilen kan en ögonblicksbild inte återställas: spara en kopia på ett säkert ställe, utanför den här datorn.**

**CLI:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```

**Bare-metal:** Clonezilla-ISO:n frågar efter nyckeln och lösenfrasen; nyckeln skrivs aldrig till ISO:n. Se [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md).

## Snabbare mappbackup: ändringsdetektering

| Läge | Läser | Lagras som |
|---|---|---|
| **Legacy** (standard) | alla filer | en `<name>.pxar.didx` per mapp |
| **Data** | alla filer | ett uppdelat arkiv: `<name>.mpxar.didx` (metadata) + `<name>.ppxar.didx` (filinnehåll) |
| **Metadata** | bara nya och ändrade filer | samma uppdelade arkiv; filer med samma storlek, ändringstid, läge och ägare tas från föregående backup utan att läsas |

Första Metadata-körningen läser allt. En ändring som behåller storlek och ändringstid märks inte, därför finns **Read every file every N runs**. Det gäller bara mappsets: maskinbackuper och återställning till bare metal eller Proxmox VE påverkas inte. Engångsbackuper använder Legacy. **Ett Backup ID per set:** om två set använder samma Backup ID på samma server jämför varje körning med det andra setets ögonblicksbild och läser allt. CLI: `-change-detection-mode legacy|data|metadata` (det finns ingen flagga för ”var N:e körning”: schemalägg en `data`-körning då och då). **Innan du byter ett set till Data eller Metadata,** uppdatera alla datorer du återställer från till v0.6.0 eller senare.

## 🚀 Snabbstart (GUI)

1. Installera: MSI på Windows, `.deb` eller `.rpm` på Linux, eller packa upp zip- eller tar.gz-filen från [releaserna](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
2. Starta. På Windows, **Kör som administratör** för VSS, maskinbackup och ändring av Backup Sets. På Linux, använd **Proxmox Backup Client (administrator)** (eller root) för maskinbackup och blockögonblicksbilder
3. Lägg till din PBS-server under **Preferences > Account Information** och klicka **Test**
4. Skapa ett Backup Set (eller en engångsbackup) och välj mappar eller diskar
5. Kör det. För att scheman ska köras utan att någon är inloggad, slå på **Preferences > Advanced > Run as Service**

[GUI-manualen](docs/manual/GUI.md) går igenom varje steg.

## Fullständig livebackup av maskinen

`proxmoxbackup-machine` (och GUI:ts maskinbackuper) säkerhetskopierar ett komplett Windows 10/11-, Windows Server- eller Linux-system utan driftstopp. Syntaxen är som för mappbackup, men `-backupdir` ersätts av `-backupdev` (upprepa för flera diskar):

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Kommandot hittar de monterade partitionerna på disk 0, tar en VSS-ögonblicksbild och skapar en startbar avbild av hela disken som fidx. Ledigt utrymme och växlings- och vilolägesfilerna går in som nollor utan att läsas; `-readfreespace` läser allt (forensisk kopia). Nästa backup är inkrementell, med hastigheter på upp till 1 GB/s.

### Förutsättningar för maskinbackup på Linux (motsvarigheten till VSS)

Linux har ingen VSS; projektet använder kärnmodulen `elastio-snap` (en fork av `dattobd`), i den underhållna versionen [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap). `dattobd` stöds också om den redan är laddad. Båda följande krävs:

1. **Modulen måste vara installerad och laddad.** Det dokumenterade repopaketet fungerar inte, åtminstone på Ubuntu 22.04; att bygga från källkod fungerar:
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
   Det laddar modulen bara för den aktuella uppstarten (för att ladda den vid varje start, t.ex. en post i `/etc/modules-load.d/`).

2. **Backupprocessen måste köras som root (EUID 0).** Medlemskap i gruppen `disk` räcker inte. Använd menyposten **Proxmox Backup Client (administrator)**, bakgrundstjänsten eller `sudo`. Mappbackup har inget sådant krav.

### Filåterställning från en diskbackup

GUI:ts sida Restore öppnar en maskinbackup som en mappbackup: varje volym är en mapp du kan bläddra i och återställa från (NTFS från v0.8.0; FAT, exFAT, ext2/3/4 och XFS på master). Inget monteras.

På Linux kan du också koppla in en diskbackup som blockenhet med nbd-verktyget. Kör först `modprobe nbd max_part=0` (med `max_part != 0` uppstår en oändlig loop av partitionssökning):

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Utan `-path` visas ett terminalgränssnitt där du väljer fidx-filen. Använd det inte på en dator som kör viktiga tjänster (ett korrupt filsystem kan krascha operativsystemet), och avmontera allt innan du stoppar pbsnbd. Vid `Device or resource busy`, kör `nbd-client -d /dev/nbd0` eller starta om.

### Återställning till en fysisk dator

Forken levererar en patchad Clonezilla Live-ISO med en helt automatisk startmenypost **”PBS Bare Metal Restore”** (första position, standardval när tiden går ut): ange PBS-uppgifterna, välj en ögonblicksbild, bekräfta den automatiskt hittade måldisken, så sköter den nätverk, NBD-anslutning, återställning och omstart. Den äldre halvmanuella vägen (”pbs-nbd” i Clonezillas lägesmeny) finns kvar.

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

Detaljer i **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)** och i [manualen för bare-metal-återställning](docs/manual/BMR.md).

## Användning: mappbackup

Fullständig referens finns i **[kommandoradsmanualen](docs/manual/CLI.md)**. Ett typiskt kommando:

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

Den officiella klientens miljövariabler `PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD` och `PBS_FINGERPRINT` läses också. Mejlmallarna använder [Gos mallmotor](https://pkg.go.dev/text/template); listan över variabler finns i den engelska README-filen.

## 📋 Förutsättningar

- Windows 10/11 eller Windows Server (64-bitars), med administratörsrättigheter för VSS och maskinbackup
- eller Linux (paketen är för Debian, Ubuntu, Mint och Fedora), med WebKitGTK för GUI:t, samt root och kärnmodulen elastio-snap eller dattobd för konsekventa maskinbackuper
- Nätverksåtkomst till en Proxmox Backup Server

## 🔨 Bygga från källkod

**[BUILD.md](BUILD.md)** har de fullständiga instruktionerna (GUI, bakgrundstjänst, kommandoradsverktyg, MSI och Linux-paket). I korthet, med Go 1.25 eller senare och Wails CLI v2.13:

```bash
cd gui
wails build                                    # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe   # the background service
```

Tester: `make test`.

## 🔧 Avancerad användning och guider

- **[MULTI_PBS_USER_GUIDE.md](MULTI_PBS_USER_GUIDE.md)** och **[MULTI_PBS_GUIDE.md](MULTI_PBS_GUIDE.md)**: flera PBS-servrar (användarguide och implementationsguide).
- **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**: ISO:n för bare-metal-återställning.
- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**: vad som är klart, delvis klart eller inte gjort.
- **[CHANGELOG.md](CHANGELOG.md)**: ändringshistorik per version.
- **[TODO.md](TODO.md)**: färdplan och idéer.

## 🌐 Teknisk support i Italien

Projektet och **ALLA** dess bidrag förblir offentliga under GPLv3. E.T.I. Srl ( https://etitech.net ), projektets huvudsponsor, kan erbjuda support för Proxmox-installationer och för backupverktyg för Windows. Det kommer aldrig att finnas separata ”community”- och ”enterprise”-utgåvor; alla anpassningar, även betalda, släpps under GPLv3.

## 🖥️ GUI-attribution

**Proxmox Backup Client GUI** bygger på **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)**, utvecklat och underhållet av **[RDEM Systems](https://www.rdem-systems.com/)**. Det har slagits ihop med det här förrådet och förblir öppen källkod under GPLv3. RDEM Systems sponsrar GUI-utvecklingen och erbjuder kommersiell support.

## ⚠️ Varning

Programvaran tillhandahålls ”i befintligt skick” och håller fortfarande alfakvalitet. Vi tar inget ansvar för förlust av eller skada på data. Testa alltid dina backuper och verifiera återställningen innan du förlitar dig på dem i produktion.

## 📄 Licens

GPLv3: se filen [LICENSE](LICENSE).

## 🏷️ Varumärke

Varje bidragsgivare med **minst 5 commits** som tillför funktioner eller rättningar har rätt att få sina varumärkesuppgifter tillagda för kommersiellt bruk, förutsatt att företaget **inte** ägnar sig åt skadlig kod, krigsfrämjande verksamhet, bedrägerier, datastöld, barnhandel, våld, diskriminering, droger eller annan olaglig verksamhet. Vid ett klagomål utan giltig förklaring **dras förmånen in omedelbart**. **GPLv3-licensen gäller fortfarande**.

## 🤝 Bidra

Bidrag är välkomna, särskilt: granskning av krypteringstexterna av modersmålstalare och nyckelhantering bortom en nyckelfil; fysisk-till-virtuell migrering (P2V); asynkron eller flerkärnig uppladdning av chunks; en Proxmox-patch för Windows-säkerhetsbeskrivare i pxar; stöd för symboliska länkar i Windows; och allt annat intressant du kommer på :)

## Om Proxmox Backup Client GO-bidragsgivarna

Proxmox Backup Client GO-bidragsgivarna utvecklar och underhåller projektet. Programvaran förlitar sig på NTP/NTS-infrastrukturen och de [11 offentliga NTS-servrarna](https://github.com/jauderho/nts-servers) i communityns referenslista.

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
