# Salvanza for Proxmox Backup Server: Windows- och Linux-klient för Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

> ℹ️ Detta är en översättning av den [engelska README-filen](README.md). Där de skiljer sig åt är det den engelska som gäller.

> ⚠️ **Meddelande om varumärken och anknytning.** Detta är ett **oberoende communityprojekt**, inte en officiell Proxmox-produkt: det är **inte anknutet till, godkänt av eller sponsrat av Proxmox Server Solutions GmbH**. **Proxmox®** är ett registrerat varumärke som tillhör Proxmox Server Solutions GmbH i EU, USA och andra länder, och används här **endast** för att beskriva kompatibilitet. I enlighet med Proxmox egna riktlinjer för varumärken (registrerade varumärken får inte användas i produkt- eller appnamn, till exempel ”XY App for Proxmox Virtual Environment” i stället för ”Proxmox XY App”) **har klienten utan varumärke bytt namn**: produkten som tidigare hette *Proxmox Backup Client* heter nu **Salvanza for Proxmox Backup Server** (kort Salvanza), ett namn som inte innehåller varumärket Proxmox. De exakta reglerna finns i [Proxmox mediakit](https://www.proxmox.com/en/about/company-details/media-kit).

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/salvanza/salvanza_client)](https://github.com/salvanza/salvanza_client/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Salvanza for Proxmox Backup Server (tidigare Proxmox Backup Client) är en säkerhetskopieringsklient med öppen källkod (GPL-3.0) för Proxmox Backup Server (PBS), för Windows och Linux.** Den säkerhetskopierar mappar som filarkiv som går att bläddra i och hela datorer som startbara diskavbilder, enligt schema eller på begäran. Den återställer allt från en enda fil till en hel dator: till samma dator, till en ersättningsdator eller till en virtuell maskin i Proxmox VE.

Projektets webbplats: **[salvanza.org](https://salvanza.org/)** · Källkod, versioner och ärenden: **[github.com/salvanza/salvanza_client](https://github.com/salvanza/salvanza_client)**

Det är en **verktygssvit**:

- **Salvanza-appen**, `salvanza-gui` (ursprungligen skapad av RDEM Systems som Nimbus Backup GUI, omarbetad och utökad av mjb-is): appen för Windows och Linux. **Backup Sets** (säkerhetskopieringsuppsättningar) med flera scheman var, säkerhetskopior av mappar och hela diskar med konsekventa ögonblicksbilder (VSS i Windows, elastio-snap/dattobd i Linux), återställning av filer från mapp- och datorsäkerhetskopior, **Undelete** (ångra borttagning) och **Roll back** (återgå), flera PBS-servrar, kryptering på klientsidan och en bakgrundstjänst, `salvanza-svc`, så att scheman körs utan att någon är inloggad.
- **`salvanza-cli`**, ett enda kommandoradsverktyg med ett underkommando för varje uppgift:
  - **`salvanza-cli backup-folder`**: säkerhetskopiering av mappar (pxar) med deduplicering och ändringsidentifiering.
  - **`salvanza-cli backup-disk`**: säkerhetskopiering av hela diskar (fidx) under drift för Windows- och Linux-datorer. Avbilden är startbar efter en återställning.
  - **`salvanza-cli attach-nbd`**: ansluter en disksäkerhetskopia som en NBD-blockenhet (Linux).
- **En ISO för bare metal-återställning** (Salvanza-BMR) byggd på Clonezilla Live, med guidad återställning från påslag till omstart.

> Nyckelord: salvanza · proxmox backup server client windows · PBS-klient · Windows-säkerhetskopiering med VSS · oföränderliga säkerhetskopior utanför huset · fysisk till virtuell (P2V) · bare metal-återställning · gränssnitt för Proxmox Backup Server.

<a id="-new-in-20"></a>

## 💡 Varför använda den här klienten

Den officiella `proxmox-backup-client` är ett kommandoradsverktyg för Linux. Den här klienten arbetar mot samma server med samma säkerhetskopieformat och lägger till:

- **Windows.** Det finns ingen officiell klient för Windows. Här finns kommandoradsverktyg och en app, säkerhetskopior av mappar via VSS-ögonblicksbilder och livesäkerhetskopior av hela diskar.
- **En app för Windows och Linux**: schemalagda **Backup Sets** som körs av en bakgrundstjänst, så att ingen behöver vara inloggad, återställning med sökning, **Undelete** och **Roll back**, rapporter och e-post.
- **Hela datorer**: startbara diskavbilder, enskilda filer ur dem och en guidad ISO för återställning från grunden (bare metal), till samma dator, ny hårdvara eller en virtuell maskin i Proxmox VE.
- **Windows filinformation bevaras**: NTFS-behörigheter, attribut, alternativa dataströmmar, utökade attribut och skapandetider registreras med varje mappsäkerhetskopia, och den här klientens återställning lägger tillbaka dem.
- **Ingen inlåsning**: det den här klienten säkerhetskopierar kan återställas med den officiella klienten, och det den officiella klienten säkerhetskopierar kan återställas med den här (se nedan).

## 🆕 Nytt i 2.0

Version 2.0 för in en stor mängd arbete på det grafiska gränssnittet, bakgrundstjänsten och säkerhetskopieringsmotorn i projektet (se [CHANGELOG](CHANGELOG.md)). Jämfört med 1.2:

- **Appen ansluter till bakgrundstjänsten med administratörsbehörighet.** När tjänsten är installerad tillhör datorns PBS-servrar och **Backup Sets** (säkerhetskopieringsuppsättningar) tjänsten, och bara tjänsten läser sin inställningsmapp. Appen ber en gång om administratörsbehörighet för att ansluta (UAC i Windows, pkexec i Linux; på ett standardkonto en administratörs namn och lösenord). Bara ett litet hjälpprogram körs med förhöjd behörighet, och appen håller tjänstens token enbart i minnet. En användare som inte kan svara på frågan kan välja **Use my own settings** (använd mina egna inställningar) och behålla sina egna servrar och uppsättningar, som körs medan appen är öppen; **Connect** (anslut) erbjuder senare att kopiera dem till datorns. Själva appen körs utan administratörsbehörighet och erbjuder **Restart as administrator** (starta om som administratör) där en åtgärd kräver det. Se avsnitt 15 i [handboken för det grafiska gränssnittet](docs/manual/GUI.md#connecting-to-the-background-service).
- **Appen kontrollerar att den pratar med tjänsten.** Innan den skickar något till tjänstens lokala API kontrollerar appen vem som svarar: tjänstens egen process i Windows, en socket som ägs av root i Linux. Ett program som tar porten medan tjänsten är stoppad får ingenting.
- **Uppsättningar med flera scheman var**, från med några minuters mellanrum till en gång om året (månadens sista dag, andra tisdagen), vart och ett beskrivet i klartext med sina 3 nästa körningar.
- **Inkrementella mappsäkerhetskopior** med den officiella klientens ändringsidentifiering (lägena **Data** (data) och **Metadata** (metadata)): oförändrade filer läses inte igen.
- **Snabbare datorsäkerhetskopior** som hoppar över ledigt utrymme, växlingsfilen och vilolägesfilen, och skriver oläsbara sektorer som nollor i stället för att misslyckas.
- **Flera säkerhetskopior samtidigt** (**Maximum Concurrent Backups** (högsta antal samtidiga säkerhetskopior), 1 till 8, standard 3), även i bakgrundstjänsten, var och en med egen ögonblicksbild, egen logg, **Pause** (pausa) och **Stop** (stoppa).
- **Hastighetsgränser** för uppladdning till PBS, nedladdning från PBS och diskläsning, var och en en gemensam budget för allt som körs samtidigt.
- **Undelete och Roll back** för uppsättningar med mappar och datorer, i det grafiska gränssnittet och från kommandoraden.
- **Filer tillbaka från datorsäkerhetskopior**: bläddra i NTFS-, FAT-, exFAT-, ext2/3/4- och XFS-volymer i en diskavbild och återställ enskilda filer.
- **Krypteringsnycklar du kan spara**: spara en kopia, skriv ut en pappersnyckel med QR-kod och importera en nyckel från en fil, kameran, ett foto eller inskriven text.
- **Uppsättningar för datorsäkerhetskopior känner igen sina diskar på identitet**, så en disk som får ett nytt nummer efter ett hårdvarubyte är fortfarande den som säkerhetskopieras, och en disk som saknas får körningen att misslyckas i stället för att en annan disk säkerhetskopieras.
- **Partitioner kan uteslutas från en datorsäkerhetskopia**: markera under varje disk de partitioner som ska säkerhetskopieras. En utesluten partition läses inte och tas inte med i ögonblicksbilden, utan läggs in i avbilden som nollor, så en återställning ger tillbaka den på sin plats och i sin storlek, tom och oformaterad. Diskväljaren markerar de partitioner som en återställd disk behöver för att starta.
- **Linux-datorsäkerhetskopior med elastio-snap eller dattobd.** En uppsättning för en Linux-dator kan stänga av ögonblicksbilder och läsa diskarna under drift, men bara när ägaren kryssar i att de accepterar att säkerhetskopian kan vara inkonsekvent.
- **Varje användare kan välja sitt eget färgtema** i appen utan varumärke. En app med varumärke behåller sitt varumärkes färger.
- **Aktivera och inaktivera en uppsättning** både från dess redigerare och från dess meny, och en varning när du sparar om en uppsättning inte skulle köras som den sparats (inaktiverad, inget schema påslaget, ett problem med ögonblicksbilden, en disk som saknas).
- **Ett enda kommandoradsverktyg, `salvanza-cli`**, med underkommandona `backup-folder`, `backup-disk` och `attach-nbd` i stället för de tre separata verktygen; deras tidigare namn fungerar fortfarande.
- **Linux-paket** (`.deb`, `.rpm` och Arch), med paketet `salvanza` och tjänsten `salvanza.service`, inklusive bakgrundstjänsten.
- **En guidad ISO för bare metal-återställning**, som även återställer till en tom virtuell maskin (P2V). En dator som säkerhetskopierats som en virtuell maskin i Proxmox VE kan också återställas direkt från PBS i Proxmox VE, som vilken virtuell maskin som helst som Proxmox VE själv har säkerhetskopierat.
- **Byggen med varumärke**: samma binärfiler levereras som installationsprogram för Salvanza, Nimbus Backup och Etitech Backup, vart och ett med egna programnamn och inställningsmappar.
- **18 gränssnittsspråk.**

## 🔁 Uppgradera från Proxmox Backup Client

Salvanza är det nya namnet på Proxmox Backup Client, och uppgraderingen sker automatiskt. När du installerar Salvanza (MSI-filen eller Linux-paketet) tas den tidigare installationen av Proxmox Backup Client, eller av ett annat varumärke, bort tillsammans med dess bakgrundstjänst. Första gången Salvanza startar flyttar den över de tidigare inställningarna (PBS-servrar, nycklar, **Backup Sets** och historik) från `C:\ProgramData\ProxmoxBackupClient`, `%APPDATA%\ProxmoxBackupClient`, `~/.proxmox-backup-guardian` eller `/var/lib/proxmoxbackupclient`, och sparar en daterad kopia av inställningsfilerna i den nya mappen, under `pre-salvanza-<date>`. Dina säkerhetskopior på PBS rörs inte.

| Före | Nu |
|---|---|
| `ProxmoxBackupClient.exe`, `ProxmoxBackupClientSVC.exe` | `salvanza-gui.exe`, `salvanza-svc.exe` (Windows-tjänsten `salvanza-svc`) |
| `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd` | `salvanza-cli backup-folder`, `salvanza-cli backup-disk`, `salvanza-cli attach-nbd` (de tidigare namnen fungerar fortfarande) |
| `C:\Program Files\ProxmoxBackupClient`, `C:\ProgramData\ProxmoxBackupClient` | `C:\Program Files\Salvanza`, `C:\ProgramData\Salvanza` |
| Linux-paketet och tjänsten `proxmoxbackupclient` | Paketet `salvanza`, tjänsten `salvanza.service`, inställningar i `/var/lib/salvanza`, dina egna i `~/.salvanza` |

## ⭐ Vad den gör, med uppmätta resultat

| | Resultat |
|---|---|
| **Inkrementella mappsäkerhetskopior** | En filresurs på 480,9 GB med 761 866 filer: första körningen 4 h 17 min, varje senare körning 10 till 15 minuter. Oförändrade filer öppnas inte ens (ändringsidentifiering med **Metadata** (metadata), som i den officiella klienten). |
| **Snabba datorsäkerhetskopior** | Ledigt utrymme, växlingsfilen och vilolägesfilen läses inte, och tomt diskutrymme hashas inte. En bärbar dators disk på 1 TB med 373 GB använt tog tidigare mellan 1 h 23 min och 1 h 44 min och tar nu 38 minuter. |
| **Långsamma och datapriserade förbindelser** | Bara nya data skickas, komprimerade, och appen visar vad som faktiskt gick över nätverket. En liten Linux-dator på en avlägsen plats med en 4G-anslutning (4,8 Mbit/s upp): hela diskavbilden på 128 GB plus hemmappen på 11 minuter, med ungefär 172 MiB överfört. |
| **Filer tillbaka från datorsäkerhetskopior** | En bärbar dators C:-enhet på 127 GB med 430 000 filer listas på 4 till 15 sekunder. Ingenting monteras eller laddas ned först. |
| **Bare metal-återställning** | Starta ISO:n (USB-minne, cd eller nätverk), ange PBS-uppgifterna, välj säkerhetskopian: en Windows-dator på 32 GB var tillbaka vid inloggningsskärmen 8 minuter efter påslag. |
| **Undelete** | Efter att 39 GB raderats från en filresurs med 761 000 filer hittade en sökning med **Undelete** (ångra borttagning) i säkerhetskopiorna från de senaste 7 dagarna exakt de filerna på ungefär en minut och lade tillbaka dem. |
| **Återställningens trohet** | NTFS-behörigheter, attribut, alternativa dataströmmar, utökade attribut, skapandetider och mapptider i Windows; rättigheter (mode), ägare, ACL:er, utökade attribut och symboliska länkar i Linux. |

## 📖 Användarhandböcker

| Handbok | Innehåll |
|---|---|
| **[Handbok för det grafiska gränssnittet](docs/manual/GUI.md)** | Installation, PBS-servrar, **Backup Sets** (säkerhetskopieringsuppsättningar) och scheman, inkrementella säkerhetskopior, förlopp, återställning, **Undelete** (ångra borttagning), **Roll back** (återgå), **Reports** (rapporter), loggar, **Preferences** (inställningar), bakgrundstjänsten, felsökning |
| **[Handbok för kommandoraden](docs/manual/CLI.md)** | `salvanza-cli backup-folder`, `backup-disk`, `attach-nbd`: varje flagga, konfigurationsfiler, slutkoder, schemaläggning |
| **[Handbok för bare metal-återställning](docs/manual/BMR.md)** | Återställnings-ISO:n steg för steg; `vm`-säkerhetskopior till Proxmox VE; fysisk till virtuell; annan hårdvara |
| [Återställningsscenarier](docs/RESTORE_GUIDE.md) | Vilken återställning som ska användas när |
| [Handboksindex](docs/manual/README.md) | En översikt över handböckerna |

Samma handböcker finns inbyggda i appen (**Help** (hjälp), F1), så de fungerar offline och motsvarar den installerade versionen.

<a id="-download"></a>

## 📦 Hämta

👉 **[Hämta den senaste versionen](https://github.com/salvanza/salvanza_client/releases)**

| Du har | Använd |
|---|---|
| Windows | **MSI-installationsprogrammet**, `Salvanza-<version>-x64.msi`. Det installerar appen, bakgrundstjänsten och `salvanza-cli` i `C:\Program Files\Salvanza`, lägger till den mappen i PATH, startar tjänsten och lägger till en post i Start-menyn. Installationsprogrammen med varumärke (`NimbusBackup-<version>-x64.msi`, `EtitechBackup-<version>-x64.msi`) är samma bygge under ett annat varumärke |
| Debian, Ubuntu, Mint | `sudo apt install ./salvanza_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./salvanza-<version>-*.rpm` |
| Arch | Paketet som byggs från [packaging/arch/PKGBUILD](packaging/arch/PKGBUILD) (`make pkgarch`) |
| Bara kommandoraden | `salvanza-cli`-arkiven per plattform |
| En dator att återställa | ISO:n för bare metal-återställning ([handbok för bare metal-återställning](docs/manual/BMR.md#3-get-the-iso-and-make-the-boot-media)), eller bygg den själv ([nedan](#-bare-metal-restore-iso)) |

När en version inte innehåller något Linux-paket bygger du det med `make deb`, `make pkgfedora` eller `make pkgarch` ([BUILD.md](BUILD.md), [packaging/debian/README.md](packaging/debian/README.md)).

Linux-paketen installerar en menypost, **Salvanza for Proxmox Backup Server**, som startar appen som du. En ny installation slår på bakgrundstjänsten (`salvanza.service`); en uppgradering behåller det val du gjort. Paketet `salvanza` ersätter de tidigare paketen `proxmoxbackupclient` och `pbsgo`. Paketen behöver polkit (pkexec), som appen använder för att ansluta till tjänsten.

> ⚠️ **Visar Windows ”virus hittat” (t.ex. `Trojan:Win32/Sabsik.FL.A!ml`) eller en SmartScreen-varning?**
> Det här är ett **känt falskt positivt resultat** för Go/Wails-program: det är *inte* ett virus. Suffixet `!ml` anger en identifiering av en maskininlärningsmodell som flaggar *osignerade och ovanliga* körbara filer. Verifiera nedladdningen enligt nedan.

### 🔎 Verifiera en nedladdning

Varje version har SHA-256-kontrollsummor (`SHA256SUMS.txt`, som också visas på versionssidan) och ett **signerat ursprungsintyg** (attestation): ett kryptografiskt bevis på att filen skapades av detta repositorys CI, från en exakt commit.

```powershell
Get-FileHash .\Salvanza-<version>-x64.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
gh attestation verify .\Salvanza-<version>-x64.msi --repo salvanza/salvanza_client
```

I Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`.

När en versions installationsprogram är rena i VirusTotal länkar versionssidan till rapporterna.

> ℹ️ **Kodsignering:** Windows-binärfilerna är **ännu inte Authenticode-signerade** (ett OSS-certifikat via [SignPath Foundation](https://signpath.org) väntar). Under tiden fastställs ursprunget genom intyget och kontrollsummorna ovan.

## 🚀 Snabbstart (grafiskt gränssnitt)

1. Installera: MSI-filen i Windows, `.deb`-, `.rpm`- eller Arch-paketet i Linux ([Hämta](#-download)).
2. Starta **Salvanza for Proxmox Backup Server**. Installationsprogrammen sätter upp bakgrundstjänsten, så appen ber en gång om administratörsbehörighet för att ansluta till den (UAC i Windows, ditt lösenord via pkexec i Linux). Om du inte kan ge den, välj **Use my own settings** (använd mina egna inställningar).
3. Lägg till din PBS-server under **Preferences > Account Information** (inställningar > kontoinformation) och klicka på **Test** (testa).
4. Skapa en **Backup Set** (säkerhetskopieringsuppsättning) eller en engångssäkerhetskopia och välj de mappar eller diskar som ska säkerhetskopieras.
5. Kör den. Med bakgrundstjänsten körs dess scheman oavsett om någon är inloggad eller inte.

[Handboken för det grafiska gränssnittet](docs/manual/GUI.md) går igenom varje steg.

### 🪟 Obevakad / tyst installation (MSI)

MSI-filen följer Windows Installers vanliga växel för tyst installation, plus offentliga egenskaper för varje standardvärde:

```powershell
# Silent install with the defaults (service: Automatic + started)
msiexec /i Salvanza-<version>-x64.msi /quiet

# Silent install that installs the service but does NOT enable/autostart it
# (created as Manual and left stopped)
msiexec /i Salvanza-<version>-x64.msi /quiet SERVICEAUTOSTART=0

# Silent install that also applies a settings file exported from another computer
msiexec /i Salvanza-<version>-x64.msi /quiet CONFIGFILE="C:\Deploy\settings.json"

# Custom directory + verbose install log
msiexec /i Salvanza-<version>-x64.msi /quiet INSTALLFOLDER="D:\Apps\Salvanza" /L*v "C:\Temp\install.log"

# Silent uninstall (KEEP_CONFIG=0 also purges %ProgramData%\Salvanza)
msiexec /x Salvanza-<version>-x64.msi /quiet KEEP_CONFIG=0
```

| Egenskap | Standard | Betydelse |
| --- | --- | --- |
| `SERVICEAUTOSTART` | `1` | `0` = tjänsten installeras som **Manual** (manuell) och lämnas **stoppad**: installerad, men aldrig aktiverad eller automatiskt startad |
| `AUTOSTART` | `1` | `0` = starta inte appen i meddelandefältet när varje användare loggar in |
| `DESKTOPSHORTCUTS` | `1` | `0` = ingen genväg på skrivbordet (posten i Start-menyn läggs alltid till) |
| `CONFIGFILE` | | Fullständig sökväg till en fil som sparats med **Preferences > Export Settings** (inställningar > exportera inställningar): dess PBS-servrar och uppsättningar slås ihop innan tjänsten startar. Säkerhetskopierings-ID:n som bildats av den exporterande datorns namn bildas av varje dators eget namn. Se [installer/DEPLOYMENT.md](installer/DEPLOYMENT.md) |
| `KEEP_CONFIG` | `1` | Endast avinstallation: `0` = ta även bort tillståndskatalogen `%ProgramData%\Salvanza` (ett bygge med varumärke har sin egen, till exempel `%ProgramData%\NimbusBackup`) |
| `INSTALLFOLDER` | `C:\Program Files\Salvanza` | Installationskatalog (kan även ändras i installationsgränssnittet) |

Kör med förhöjd behörighet (administratör); slutkod `0` = lyckades, `3010` = lyckades + omstart krävs. MSI-filerna med varumärke tar exakt samma växlar.

## ✨ Funktioner

### Grafiskt gränssnitt
Huvudfunktionerna finns listade under [Nytt i 2.0](#-new-in-20). Dessutom:
- 18 gränssnittsspråk: franska, engelska, italienska, tyska, polska, spanska, bulgariska, tjeckiska, grekiska, ungerska, lettiska, litauiska, nederländska, portugisiska, rumänska, slovakiska, turkiska och ukrainska
- **Engångssäkerhetskopior** av mappar eller hela diskar, och datorsäkerhetskopior lagrade som `host` eller som en `vm`-säkerhetskopia i Proxmox VE, med alternativet **Forensic copy** (kriminalteknisk kopia) som även läser ledigt utrymme
- **Återställning** med filsökning (jokertecken, reguljära uttryck, fullständiga sökvägar), selektiv återställning, NTFS/POSIX-ACL:er, alternativa dataströmmar, skapandetider i Windows och en valfri verifieringsomgång
- **Pause, Resume och Stop** (pausa, återuppta, stoppa) för en pågående säkerhetskopia, även från meddelandefältet i Windows och för körningar som bakgrundstjänsten gör
- Förloppskort i realtid (beräknad sluttid, klara filer, uppladdningshastighet, överförda data, komprimeringsgrad), **Reports** (rapporter), **Message Log** (meddelandelogg) och **View Full Logs** (visa fullständiga loggar)
- Aviseringar via e-post, åtgärder efter säkerhetskopiering, kloning av uppsättningar, export och import av inställningar
- Flera PBS-servrar, med fastlåsning av certifikatets fingeravtryck (förtroende vid första användning), inloggning med API-token eller användare/lösenord
- Fem färgteman och en egen färg, som varje användare väljer själv (i appen utan varumärke; en app med varumärke behåller sitt varumärkes färger)

### Kommandorad: `salvanza-cli`
- `salvanza-cli backup-folder`: mappsäkerhetskopior (pxar) med deduplicering och `-change-detection-mode legacy|data|metadata`
- `salvanza-cli backup-disk`: säkerhetskopior av hela diskar under drift för Windows- (VSS) och Linux-datorer (elastio-snap/dattobd) (fidx, inkrementella, ledigt utrymme hoppas över om inte `-readfreespace` anges, `-type vm` för en Proxmox VE-säkerhetskopia, `-skippartition <disk>:<number>` för att utesluta en partition)
- `salvanza-cli attach-nbd`: NBD-server för att återställa disksäkerhetskopior (Linux)
- `salvanza-cli version` skriver ut versionen, `salvanza-cli help` listar underkommandona och `salvanza-cli <command> -help` listar ett underkommandos alternativ
- Tjänstprogrammet, `salvanza-svc` (`salvanza-svc.exe` i Windows; i Linux även appen, `salvanza-gui`): `-list-sets`, `-undelete` och `-rollback` för uppsättningar som definierats i det grafiska gränssnittet

De tre underkommandona tar `-keyfile` och `-keyfile-passphrase` för krypterade ögonblicksbilder. Verktygens tidigare namn, `proxmoxbackup-directory`, `proxmoxbackup-machine` och `proxmoxbackup-nbd` (samt `pbsdirectorybackup`, `pbsmachinebackup`, `pbsnbd`), kör fortfarande samma underkommandon med samma flaggor, så befintliga skript fortsätter att fungera. Fullständig referens: [handboken för kommandoraden](docs/manual/CLI.md).

### 📸 Skärmbilder

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets** (säkerhetskopieringsuppsättningar): namngivna jobb med schema- och typmärken, **Run Now** (kör nu) på varje

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Backup progress** (förlopp): genomströmning, beräknad sluttid, nya och återanvända block

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restore** (återställ): välj en PBS-server och ett säkerhetskopierings-ID, lista ögonblicksbilder eller sök i dem efter filnamn

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**Schedules** (scheman): flera per uppsättning, med de 3 nästa körningarna

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Säkerhetskopia som virtuell maskin i Proxmox VE**: säkerhetskopierings-ID:t i PBS visas medan du skriver

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Reports** (rapporter): fullständig körhistorik med en detaljpanel

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-machine-partitions.png" width="280">](docs/screenshots/gui-machine-partitions.png)
**Uppsättningar för datorsäkerhetskopior**: markera under varje disk de partitioner som ska säkerhetskopieras

</td>
<td width="33%">

[<img src="docs/screenshots/gui-themes.png" width="280">](docs/screenshots/gui-themes.png)
**Themes** (teman): sex färgteman, som varje användare väljer själv

</td>
<td width="33%">

[<img src="docs/screenshots/gui-languages.png" width="280">](docs/screenshots/gui-languages.png)
**Språk**: 18, här franska, tyska, italienska, spanska, polska och ukrainska

</td>
</tr>
</table>

### Smarta systemundantag (mappsäkerhetskopior)
Mappsäkerhetskopior utesluter följande av sig själva:

**Systemmappar:** `System Volume Information` (VSS-lagring, kan bli 100+ GB) och `$RECYCLE.BIN` var de än finns; `Recovery` bara i roten av en enhet; `.pbs-rollback` (säkerhetsmappen för **Roll back** (återgå)).
**Systemfiler:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` och `DumpStack.log.tmp`.

**Varför det spelar roll:** en enhet kan visa 1,03 TB använt medan de verkliga filerna är ~141 GB. Utan undantag skulle säkerhetskopian ta med VSS-ögonblicksbilder (bortkastat utrymme och tid); med dem motsvarar storleken de verkliga data.

**Rekommendation:** använd en **mapp**säkerhetskopia för säkerhetskopiering på filnivå; använd en **dator**säkerhetskopia i en separat uppsättning för bare metal-återställning och P2V (en avbild av hela disken, där ledigt utrymme, växlingsfilen och vilolägesfilen läggs in som nollor).

### Säkerhet och kvalitet
- Tjänstens inställningsmapp är bara öppen för SYSTEM och Administrators (Windows) eller bara för root (Linux); dess lokala API lyssnar på 127.0.0.1 och kräver en token som byts vid varje start och var 24:e timme
- Appen skickar ingenting till det API:et förrän den har kontrollerat att det är tjänsten själv som svarar (tjänstens process i Windows, en socket som ägs av root i Linux)
- I Windows är PBS-token och lösenord i `config.json` krypterade för datorn (DPAPI)
- Validering av indata, sanering av inloggningsuppgifter, skydd mot sökvägstraversering (path traversal), nya försök med exponentiell fördröjning
- Enhetstester i varje modul, plus valfria live-tester och helhetstester mot en riktig PBS

## 🔐 Kryptering på klientsidan

Ögonblicksbilder kan krypteras innan de lämnar datorn, med en nyckelfil för Proxmox Backup Server (det format som `proxmox-backup-client key create` skriver, så nycklarna kan användas omväxlande med den officiella klienten). Varje block krypteras med AES-256-GCM och manifestet signeras. PBS-servern ser aldrig nyckeln.

**Grafiskt gränssnitt:** öppna **Preferences > Account Information** (inställningar > kontoinformation), redigera en server och använd dess flik **Encryption** (kryptering). Med **Import...** (importera) hämtar du en nyckel du redan har (en kopia av nyckelfilen, eller en pappersnyckel som läses med kameran, från en skanning eller ett foto, eller skrivs in), eller så skapar du en ny med **Generate...** (generera). Fliken visar nyckelns fingeravtryck. **Save a copy...** (spara en kopia) och **Print a paper key...** (skriv ut en pappersnyckel, med QR-kod) håller en kopia borta från datorn. Rapporterna visar om varje säkerhetskopia var krypterad. **Utan nyckelfilen kan en ögonblicksbild inte återställas: förvara en kopia på ett säkert ställe, borta från den här datorn.**

**Lösenfrasskyddade nycklar (scrypt eller PBKDF2):** **Ask me** (fråga mig) håller lösenfrasen i minnet under en appsession; **Remember on this computer** (kom ihåg på den här datorn) lagrar den som PBS-token och är valet för scheman och bakgrundstjänsten.

**Kommandorad:**
```shell
salvanza-cli backup-folder -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` (eller `PBS_ENCRYPTION_PASSWORD`) behövs bara för en skyddad nyckel, och efterfrågas om den utelämnas. `salvanza-cli backup-disk` och `salvanza-cli attach-nbd` tar samma flaggor. `attach-nbd` kontrollerar nyckeln mot ögonblicksbilden innan den rör NBD-enheten, så en felaktig nyckel misslyckas direkt.

**Bare metal-återställning:** återställnings-ISO:n frågar efter nyckeln (ingen, en nyckelfil på ett USB-minne eller en inskriven sökväg) och efter lösenfrasen för en skyddad nyckel. Nyckeln skrivs aldrig till ISO:n.

För att återställa en krypterad ögonblicksbild krävs samma nyckelfil; det finns inget sätt att återställa om den går förlorad.

## Snabbare mappsäkerhetskopior: ändringsidentifiering

I standardläget **Legacy** (klassiskt) läser varje mappsäkerhetskopia varje fil, så en körning tar lika lång tid som att läsa hela mappen även när inget har ändrats. Uppsättningar och `salvanza-cli backup-folder` kan i stället använda ändringsidentifieringslägena i den officiella `proxmox-backup-client`:

| Läge | Läser | Lagras som |
|---|---|---|
| **Legacy** (standard) | varje fil | en `<name>.pxar.didx` per mapp |
| **Data** (data) | varje fil | ett delat arkiv: `<name>.mpxar.didx` (metadata) + `<name>.ppxar.didx` (filinnehåll) |
| **Metadata** (metadata) | bara nya och ändrade filer | samma delade arkiv; filer vars storlek, ändringstid (på nanosekunden), rättigheter (mode) och ägare stämmer med föregående säkerhetskopia hämtas därifrån utan att läsas |

I läget **Metadata** läser den första körningen allt; senare körningar läser bara det som ändrats. En fil som ändrats på ett sätt som behåller dess storlek och ändringstid märks inte, och därför kan en uppsättning också **läsa alla filer var N:e körning** (inställningen **Read every file every**). Ge varje uppsättning ett eget säkerhetskopierings-ID (Backup ID): läget **Metadata** jämför med den senaste ögonblicksbilden i säkerhetskopieringsgruppen. På kommandoraden använder du `-change-detection-mode`.

**Innan du byter en uppsättning till Data eller Metadata** ska du uppdatera varje dator du återställer från till 2.0: tidigare versioner visar mapparna i ett delat arkiv utan deras filer. Se [avsnitt 7 i handboken för det grafiska gränssnittet](docs/manual/GUI.md#7-incremental-folder-backups-change-detection).

<a id="-physical-machines-into-virtual-machines-p2v"></a>

## 🔄 Fysiska datorer till virtuella maskiner (P2V)

En datorsäkerhetskopia av en fysisk pc eller server kan bli en virtuell maskin i Proxmox VE, på två sätt:

1. **Återställ den i Proxmox VE.** Ställ in uppsättningen (eller engångssäkerhetskopian) på **Backup as: Proxmox VE virtual machine** (säkerhetskopiera som: virtuell maskin i Proxmox VE). Säkerhetskopian lagras som `vm/<number>`, så Proxmox VE listar den bland sina andra VM-säkerhetskopior och återställer den som en virtuell maskin (en SATA-disk; lägg till ett nätverkskort innan du startar den).
2. **Återställ den med återställnings-ISO:n till en tom virtuell maskin** med samma firmware som den ursprungliga datorn, en SATA-disk som är minst lika stor och ett Intel E1000-nätverkskort. Det fungerar för både `host`- och `vm`-säkerhetskopior, och samma säkerhetskopia kan fortfarande återställas till fysisk hårdvara (bare metal).

Testade vägar omfattar Windows 10 och Windows Server (UEFI/GPT), Linux-skrivbord (UEFI/GPT och BIOS/MBR) och en krypterad Linux-säkerhetskopia med nyckeln på ett USB-minne. [Handboken för bare metal-återställning](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines) har hela tabellen, firmwarevalen, drivrutinerna och steg-för-steg-instruktioner.

## 🤝 Kompatibel med den officiella Proxmox-klienten, åt båda hållen

Verifierat med `proxmox-backup-client` 3.4.9 och PBS 4.2:

- **Legacy-, Data- och Metadata-ögonblicksbilder som skapats av den här klienten återställs byte för byte identiskt med den officiella klienten**, både från det grafiska gränssnittet och kommandoraden, skapade i Windows eller Linux, och filbläddraren i PBS 4.2 öppnar dem.
- **Legacy-, Data- och Metadata-ögonblicksbilder som skapats av den officiella klienten återställs byte för byte identiskt med den här klienten**, helt eller selektivt.
- Samma format för delade arkiv (pxar version 2), samma lägen för ändringsidentifiering, samma format för krypteringsnyckelfiler, samma miljövariabler i stil med `PBS_REPOSITORY`.

Dina säkerhetskopior är aldrig bundna till den här klienten. Två tillägg följer med standardarkiven, och den officiella klienten återställer filerna som vanligt även utan dem:

- `.proxmox_backup_client_meta.json` i roten av en mappögonblicksbild registrerar den ursprungliga sökvägen. Den officiella klienten återställer den som en vanlig fil.
- `proxmox-client-acls.json.gz.blob`, som sparas med varje mappögonblicksbild, registrerar det som arkivet inte har plats för i Windows (NTFS-behörigheter, attribut, skapandetider, alternativa dataströmmar och utökade attribut) och, i Linux, de utökade attributen och POSIX-ACL:erna. Den officiella klienten och PBS webbgränssnitt hoppar över den; den här klientens återställning lägger tillbaka dem.

I Linux sparar den officiella klienten POSIX-ACL:er, utökade attribut och filkapabiliteter i själva arkivet. Den här klienten skriver och läser ännu inte de posterna, så i Linux följer ACL:er inte med mellan de två klienterna åt något håll. Att göra det på det officiella sättet, som ett val, finns i [färdplanen](TODO.md).

## Säkerhetskopiering av hela datorn under drift

`salvanza-cli backup-disk` (och det grafiska gränssnittets datorsäkerhetskopior) säkerhetskopierar ett komplett Windows 10/11- eller Windows Server-system, eller en Linux-dator, utan något avbrott.

Kommandosyntaxen är i stort sett densamma som för mappsäkerhetskopior, förutom att `-backupdir` ersätts av `-backupdev` (upprepa den för flera diskar). Till exempel:

`salvanza-cli backup-disk -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Kommandot ovan tittar på disk 0, hittar alla monterade partitioner, tar en VSS-ögonblicksbild av dem och skapar sedan en startbar avbild av hela disken som fidx. Ledigt utrymme, växlingsfilen och vilolägesfilen läggs in i avbilden som nollor utan att läsas (i Linux även ledigt utrymme i ext2/3/4 och XFS); `-readfreespace` läser i stället allt, för en kriminalteknisk kopia. En sektor som inte kan läsas läggs in i avbilden som nollor, och körningen talar om var. Lägg till `-type vm` (med ett numeriskt `-backup-id`) för att lagra säkerhetskopian som en VM-säkerhetskopia i Proxmox VE ([P2V](#-physical-machines-into-virtual-machines-p2v)).

Nästa säkerhetskopia är inkrementell. Hashningen sker parallellt, så hastigheter på 1 GB/s kan nås.

För att utesluta en partition, till exempel ett spelbibliotek eller en datapartition som redan täcks av en mappsäkerhetskopia, lägg till `-skippartition <disk>:<number>` (upprepa för flera), till exempel `-skippartition \\.\PhysicalDrive0:4` eller `-skippartition /dev/sda:3`. Partitionen läses inte och tas inte med i ögonblicksbilden, utan läggs in i avbilden som nollor: partitionstabellen behålls, så en återställning ger tillbaka partitionen på sin plats och i sin storlek, tom och oformaterad. I det grafiska gränssnittet avmarkerar du partitionen under dess disk i uppsättningen.

### Förutsättningar för säkerhetskopiering av Linux-datorer (motsvarigheten till VSS)

Säkerhetskopiering av Windows-datorer använder VSS. Linux har ingen VSS, så en datorsäkerhetskopia av en Linux-dator i drift tar en ögonblicksbild vid en viss tidpunkt av varje monterad partition med en kärnmodul: `elastio-snap` (den underhållna forken, [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap)) eller `dattobd`.

Två saker krävs, annars vägrar en Linux-datorsäkerhetskopia att köras:

1. **Kärnmodulen måste vara installerad och inläst.** Installera den via DKMS, så att den byggs om efter varje kärnuppdatering, och se till att den läses in vid varje uppstart. Avsnittet [Ögonblicksbilder i Linux: installera elastio-snap eller dattobd](docs/manual/GUI.md#linux-snapshots-installing-elastio-snap-or-dattobd) i handboken för det grafiska gränssnittet har testade steg för Debian, Ubuntu, Mint och Fedora, samt Secure Boot och kontroller. I november 2026 går ingen av modulerna att bygga på kärna 7 (Fedora 44 efter dess uppdateringar).

2. **Säkerhetskopieringen måste köras som root.** Modulens styrgränssnitt är bara tillgängligt för root, och ögonblicksbilden nekas för en vanlig användare. Bakgrundstjänsten körs som root; utan den använder du **Restart as administrator** (starta om som administratör) i appen, eller kör `salvanza-cli backup-disk` med `sudo`. En vanlig mappsäkerhetskopia behöver inte root.

I det grafiska gränssnittet kan en uppsättning för en Linux-dator avmarkera **Use snapshots** (använd ögonblicksbilder) och läsa sina diskar under drift, men bara om den andra rutan är ikryssad (”Run without a snapshot: I accept the backup may be inconsistent”, det vill säga ”kör utan ögonblicksbild: jag accepterar att säkerhetskopian kan vara inkonsekvent”). Utan den stoppar varje körning och förklarar varför. Partitioner som inte är monterade läses direkt.

### Återställa filer från en disksäkerhetskopia

Sidan **Restore** (återställ) i det grafiska gränssnittet öppnar en datorsäkerhetskopia precis som en mappsäkerhetskopia: varje volym i avbilden (NTFS, FAT, exFAT, ext2/3/4 eller XFS) är en mapp du kan bläddra i och återställa från. Ingenting monteras.

I Linux kan du också ansluta en disksäkerhetskopia som en blockenhet med `salvanza-cli attach-nbd`. Kör först `modprobe nbd max_part=0`: med `max_part != 0` kan kärnan fastna i en oändlig loop när den söker efter partitioner.

`attach-nbd` ansluter vilken säkerhetskopia av en fast disk som helst, oavsett om det är en VM- eller host-säkerhetskopia (så det fungerar även för säkerhetskopior från Proxmox VE). Till exempel:

`sudo salvanza-cli attach-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Om du utelämnar `-path` låter ett terminalgränssnitt dig välja fidx-filen; `-list` skriver i stället ut alla avbilder.

> Använd inte detta på en dator som kör något viktigt: ett skadat filsystem kan krascha operativsystemet, vilket är anledningen till att Proxmox VE använder en QEMU-instans för detta.

Avmontera allt på nbd-disken innan du stoppar `salvanza-cli attach-nbd`, annars förblir partitionen sannolikt upptagen. Om du får felet `Device or resource busy`, tvinga fram en frånkoppling med `nbd-client -d /dev/nbd0`, eller starta om.

### Återställa till en fysisk dator eller en virtuell maskin

Använd [ISO:n för bare metal-återställning](#-bare-metal-restore-iso), till fysisk hårdvara eller till en tom virtuell maskin. [Handboken för bare metal-återställning](docs/manual/BMR.md) går igenom det steg för steg.

## Användning: säkerhetskopiering av kataloger

Den fullständiga referensen, med exempel för Windows och Linux, slutkoder och schemaläggning, finns i **[handboken för kommandoraden](docs/manual/CLI.md)**. Ett typiskt kommando:

```shell
salvanza-cli backup-folder -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -backupdir "C:\path\to\backup" -datastore "datastorename"
```

```
salvanza-cli backup-folder
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

Den officiella klientens miljövariabler `PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD` och `PBS_FINGERPRINT` läses också.

För JSON-konfiguration, se [config.json.example](config.json.example); fyll bara i de fält du behöver.

### Flera kataloger i en körning

Upprepa `-backupdir` för att säkerhetskopiera mer än en katalog med ett enda kommando:

```shell
salvanza-cli backup-folder -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -datastore "datastorename" -backupdir "C:\Data" -backupdir "D:\Photos"
```

Varje katalog säkerhetskopieras som en egen säkerhetskopieringsgrupp, med säkerhetskopierings-ID:t `<backup-id>_<path>` (`-backup-id` är som standard värdnamnet). Lagringstid, rensning (pruning) och återställning behandlar därför varje katalog som en oberoende serie. En katalog som misslyckas stoppar inte de andra, och slutkoden rapporterar felet. `-pxarout` kan inte kombineras med flera kataloger, och två kataloger som skulle få samma säkerhetskopierings-ID avvisas innan något laddas upp.

I en JSON-konfigurationsfil använder du `"backupdirs": ["C:\\Data", "D:\\Photos"]`. Det enskilda fältet `"backupdir"` fungerar fortfarande och kan kombineras med `"backupdirs"`. Kryptering fungerar som för en enda katalog: nyckeln läses en gång, och varje katalogs säkerhetskopieringsgrupp krypteras med den.

### Utelämna tider ur arkivet

Varje fils och mapps ändringstid ingår i arkivet, så en tid som ändras ändrar arkivets byte, och blocken runt den posten laddas upp igen trots att inget i dem har ändrats. För de flesta data är det rätt avvägning. Vissa trädstrukturer rör dock hela tiden sina mappar medan filerna är desamma: en lagring som sorterar sina data i mappar med hashnamn (`aa/bb/<rest of the hash>`) kan av just det skälet ladda upp en stor del av arkivet igen vid varje körning.

`-no-mtime` (`"no-mtime": true` i en JSON-konfigurationsfil) registrerar i stället 1970-01-01 som tid för varje fil och mapp, så att bara verkliga innehållsändringar kostar nya block. De verkliga tiderna sparas ingenstans: en återställning daterar varje objekt 1970-01-01. Med `-change-detection-mode metadata` läses varje fil, som i läget `data`. Alternativet är avstängt som standard; använd det för en källa där tiderna inte spelar någon roll.

### E-postmallar

Ett relä som inte kräver inloggning (ofta på port 25) fungerar: utelämna användarnamn och lösenord. Avsändaren kan innehålla ett namn (`Backups <backup@example.com>`).

[Go:s mallmotor](https://pkg.go.dev/text/template) används för ämnesrader och meddelandetext; se dess dokumentation för syntaxen. Följande variabler finns:

- `.NewChunks`: antal nya block som skapats
- `.ReusedChunks`: antal block som återanvänts
- `.Datastore`: datalagringens (datastore) namn
- `.Error`: felet, om det finns något (`.ErrorStr` ger det som text, tom när det inte finns något)
- `.ReadErrors`: de filer eller mappar som inte kunde läsas; `.ReadErrorCount` är hur många
- `.Hostname`: datorns värdnamn
- `.StartTime`: när säkerhetskopieringen startade
- `.EndTime`: när säkerhetskopieringen slutade
- `.Duration`: säkerhetskopieringens varaktighet
- `.FromattedDuration`: säkerhetskopieringens formaterade varaktighet (namnet stavas så i koden)
- `.Success`: true när säkerhetskopieringen avslutades utan fel och utan något oläsbart
- `.Partial`: true när en ögonblicksbild sparades men vissa filer inte kunde läsas
- `.Status`: `Success`, `Partial` eller `Failed`

## Säkerhetskopiering av dataströmmar

Detta säkerhetskopierar en dataström i stället för ett pxar-arkiv. Till exempel:

```
mysqldump yourdatabase | salvanza-cli backup-folder -backupstream yourdatabase.sql [other options]
```

Här används buzhash-uppdelning för deduplicering av själva dataströmmen, till exempel ett tar-arkiv eller en SQL-dump.

## Kända problem

Om skyddet mot skadlig kod i Windows Defender är aktivt kan en säkerhetskopiering bli långsammare, ner till ungefär 25 % av den möjliga hastigheten.

I Windows tar `salvanza-cli backup-folder` och `backup-disk` ett processlås: en andra säkerhetskopiering som startas medan en redan körs skriver ut ”Another backup job is running” och avslutas med kod 2. I Linux finns inget sådant lås, så schemalägg inte två kommandoradskörningar samtidigt. (Det grafiska gränssnittet och bakgrundstjänsten kör flera säkerhetskopior samtidigt på ett säkert sätt, se [Flera säkerhetskopior samtidigt](docs/manual/GUI.md#67-several-backups-at-once).)

## 📋 Förutsättningar

- Windows 10/11 eller Windows Server (64-bitars); administratörsbehörighet för att installera och för att ansluta appen till bakgrundstjänsten
- eller Linux: paketen är för Debian, Ubuntu, Mint, Fedora och Arch; det grafiska gränssnittet behöver WebKitGTK-körmiljön och polkit; konsekventa datorsäkerhetskopior behöver kärnmodulen elastio-snap eller dattobd och root
- Nätverksåtkomst till en Proxmox Backup Server
- För P2V: Proxmox VE med PBS-datalagringen tillagd som lagring, eller valfri hypervisor som kan starta återställnings-ISO:n

## 🔨 Bygga från källkod

**[BUILD.md](BUILD.md)** har de fullständiga instruktionerna: det grafiska gränssnittet och bakgrundstjänsten, kommandoradsverktygen, MSI-filen och Linux-paketen. Kortfattat, med Go 1.25 eller senare och Wails CLI v2.13:

```bash
cd gui
wails build                                                       # the app, gui/build/bin/salvanza-gui.exe
go build -tags service -o build/bin/salvanza-svc.exe              # the background service
cd ../cli
GOWORK=off go build -o ../gui/build/bin/salvanza-cli.exe .        # the command line
```

```bash
make cli          # salvanza-cli, in dist/
make deb          # .deb
make pkgfedora    # .rpm (in Docker)
make pkgarch      # Arch package (on Arch)
make msi          # Windows MSIs on Linux (Docker + Wine + WiX), one per brand
make clonezilla   # the bare-metal restore ISO, dist/Salvanza-BMR-<version>.iso
```

Det byggda gränssnittet (`gui/frontend/dist`) är incheckat, så Node.js behövs bara för att ändra frontend-koden (`npm install` och `npm run build` i `gui/frontend`, eller `wails dev` för omladdning i realtid). Tester: `make test`.

## 🔧 Avancerad användning och guider

### Multi-PBS (flera PBS-servrar)

Konfigurera flera PBS-servrar och välj mål per uppsättning eller engångssäkerhetskopia (t.ex. `C:\Users` dagligen till en snabb SSD-PBS, `D:\` varje vecka till en PBS för stora datamängder, plus en server för katastrofåterställning).

- **[Användarguide](MULTI_PBS_USER_GUIDE.md)**: lägga till och testa servrar, standardservern, välja server per säkerhetskopia, vanliga frågor och felsökning.
- **[Implementeringsguide](MULTI_PBS_GUIDE.md)**: datamodell, automatisk migrering från en konfiguration med en enda PBS, backend-metoder.

En äldre konfiguration med en enda PBS migreras automatiskt till en server med namnet `default` vid första inläsningen.

<a id="-bare-metal-restore-iso"></a>

### 💿 ISO för bare metal-återställning

Återställnings-ISO:n, Salvanza-BMR, är en vanlig Clonezilla Live-ISO som patchats med binärfilen `pbsnbd` (koden för `salvanza-cli attach-nbd`, byggd separat från `nbd/cmd/pbsnbd`) plus två menyposter. Den startar från cd, från ett USB-minne (skrivet med `dd`, eller från ett Ventoy-minne), och på UEFI- eller BIOS-datorer:

- **`ocs-pbs-bare-metal-restore`**: den första posten i startmenyn, som är förvald, **PBS Bare Metal Restore**. Den hoppar över Clonezillas frågor om språk och tangentbord och går direkt från PBS-inloggningsuppgifter till en slutförd återställning: först DHCP (en fråga om statisk IP bara om det misslyckas), en krypteringsnyckel från ett USB-minne om säkerhetskopian är krypterad, och identifiering av måldisk som aldrig erbjuder minnet den startade från. Den återställer Windows- och Linux-säkerhetskopior lika bra (återställningen sker på blocknivå), till fysisk hårdvara eller till en tom virtuell maskin.
- **`ocs-pbs-nbd`**: en post **pbs-nbd** i Clonezillas egen meny för val av läge. Den frågar efter dina PBS-anslutningsuppgifter och ögonblicksbilden, ansluter den till `/dev/nbd0` och lämnar sedan över till Clonezillas vanliga guide för diskåterställning, som du styr för hand.

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

`make clonezilla` hämtar den vanliga ISO:n, bygger `pbsnbd` och gör samma sak, och skriver `dist/Salvanza-BMR-<version>.iso`. Alla detaljer (varför ISO:n byggs om helt i stället för att bytas ut på plats, förutsättningar, menyflöde, verifiering) finns i **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**; hur den används, i **[handboken för bare metal-återställning](docs/manual/BMR.md)**.

### Bygga det grafiska gränssnittet för Windows

**Direkt i Windows**: se **[BUILD.md](BUILD.md)** för verktygskedjan (Go och MinGW från Chocolatey, Wails) och kommandona för appen, tjänsten och MSI-filen.

**Docker (vid bygge i Linux).** `./build_gui_windows_docker.sh` bygger Windows-appen, med korrekt stöd för WebView2, i en tillfällig `golang`-container. Den bygger inte bakgrundstjänsten eller MSI-filen.

### Ändringslogg och intern dokumentation

- **[CHANGELOG.md](CHANGELOG.md)**: ändringshistorik per version.
- **[TODO.md](TODO.md)**: öppen färdplan och idéer.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**: stabilt produktläge och tillgängliga byggen.
- **[installer/DEPLOYMENT.md](installer/DEPLOYMENT.md)**: distribuera MSI-filen till många datorer.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**: dialogrutan för avinstallation av MSI (behåll/radera konfigurationen) och dess testplan.
- **[AGENTS.md](AGENTS.md)**: repositoryts struktur och konventioner, för bidragsgivare och kodagenter.

## 🌐 Teknisk support i Italien

Med förbehållet att projektet och **ALLA** dess bidrag för alltid förblir offentliga och licensierade under GPLv3, kan projektets huvudsponsor, som också sponsrat de senaste funktionerna för datorsäkerhetskopiering, E.T.I. Srl ( https://etitech.net ), erbjuda den som behöver det support för Proxmox-installationer i allmänhet och för säkerhetskopieringsverktyg för Windows i synnerhet.

Det kommer aldrig att finnas en separat ”community”- och ”enterprise”-utgåva; bara den tekniska supporten kommer att vara en fristående tjänst.
Alla anpassningar som du beställer, även om utvecklingen kan vara betald, släpps under GPLv3 precis som hela projektet.

## 🖥️ Det grafiska gränssnittets ursprung

**Salvanza-appen** (tidigare Proxmox Backup Client GUI) skapades ursprungligen av **[RDEM Systems](https://www.rdem-systems.com/)** som **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)**: de utformade och byggde den ursprungliga appen och erbjuder kommersiell support kring Nimbus Backup, dess version med varumärke. **[mjb-is](https://github.com/mjb-is)** omarbetade och utökade den sedan, med inspiration från arbetsflödet och känslan i andra säkerhetskopieringssystem, och det arbetet har förts tillbaka in i det här repositoryt. Hela kodbasen, inklusive det grafiska gränssnittet och alla dess funktioner, förblir öppen källkod under GPLv3-licensen.

## ⚠️ Varning

Denna programvara tillhandahålls ”i befintligt skick”. Även om vi strävar efter tillförlitlighet tar vi inget ansvar för förlust av eller skada på data. Testa alltid dina säkerhetskopior och verifiera återställningen innan du förlitar dig på dem i produktion. Ingen säkerhetskopia är bevisad förrän du har återställt från den, så testa dina egna återställningar, även en bare metal-återställning.

Programvaran håller fortfarande alfakvalitet och vi tar inget ansvar för någon form av skada eller dataförlust, inte ens av källfiler.

Felrapporter är välkomna: öppna ett [ärende](https://github.com/salvanza/salvanza_client/issues) med vad du gjorde, vad som hände och loggen (**Message Log** (meddelandelogg) eller **View Full Logs** (visa fullständiga loggar)).

## 📄 Licens

GPLv3: se filen [LICENSE](LICENSE).

## 🏷️ Varumärkesprofilering

Varje bidragsgivare som har bidragit med **minst 5 commits** som tillför funktionalitet eller rättningar har rätt att få sina varumärkesuppgifter tillagda för kommersiell användning.

De enda villkoren är att företaget som varumärket pekar på **inte** bedriver något av följande:

- Kampanjer med skadlig kod
- Verksamhet som främjar krig (gäller alla länder, även västerländska)
- Bedrägerier
- Datastöld
- Människohandel med barn
- Våld
- Diskriminering
- Droger
- All verksamhet som allmänt anses olaglig

Om ett klagomål når någon av bidragsgivarna försöker vi ta kontakt; om ingen giltig förklaring ges **avslutar vi omedelbart** den förmånen.

**GPLv3-licensen gäller fortfarande**, och du är fortfarande fri att forka projektet och bygga dina egna körbara filer.

## 🤝 Bidra

Bidrag är välkomna, särskilt:

1. Översättningar av README-filerna och användarhandböckerna för version 2.0
2. Asynkron / flerkärnig uppladdning av block (flerkärnig komprimering är redan implementerad för datorsäkerhetskopiering)
3. En patch på Proxmox-sidan för att lägga till en ny typ av post i pxar-formatet med Windows säkerhetsbeskrivare
4. Stöd för symboliska länkar och knutpunkter (junctions) i Windows (de hoppas över i dag)
5. Tester på mer hårdvara, fler Linux-distributioner och kärnor, särskilt bare metal-återställningar
6. Allt annat intressant du kan komma på :)

## Om bidragsgivarna till Salvanza

Bidragsgivarna till Salvanza (tidigare bidragsgivarna till Proxmox Backup Client GO) utvecklar och underhåller detta projekt. Programvaran förlitar sig på NTP/NTS-infrastrukturen och de [11 offentliga NTS-servrarna](https://github.com/jauderho/nts-servers) som listas i communityns referens.

---

**© 2024-2026 Salvanza contributors and RDEM Systems.**
