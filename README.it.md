# Proxmox Backup Client: client Windows e Linux per Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

> ℹ️ Questa è una traduzione del [README in inglese](README.md). Dove i due testi differiscono, quello aggiornato è il README in inglese.

> ⚠️ **Avviso su marchi e affiliazione.** Questo è un **progetto comunitario indipendente**, non un prodotto ufficiale Proxmox: **non è affiliato a Proxmox Server Solutions GmbH, né approvato o sponsorizzato da essa**. **Proxmox®** è un marchio registrato di Proxmox Server Solutions GmbH nell'UE, negli Stati Uniti e in altri paesi, e viene usato qui **solo** per indicare la compatibilità. Seguendo le linee guida di Proxmox sui propri marchi (i marchi registrati non devono comparire nei nomi di prodotti o app: ad esempio "XY App for Proxmox Virtual Environment" e non "Proxmox XY App"), **è in corso la ridenominazione del client senza marchio**: il prodotto oggi chiamato *Proxmox Backup Client* sta passando a un nome che non contiene il marchio Proxmox. Le regole esatte sono nel [media kit di Proxmox](https://www.proxmox.com/en/about/company-details/media-kit).

[![Licenza](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/tizbac/proxmoxbackupclient_go)](https://github.com/tizbac/proxmoxbackupclient_go/releases)
[![Documentazione](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client è un client di backup open source (GPL-3.0) per Proxmox Backup Server (PBS), per Windows e Linux.** Esegue il backup delle cartelle come archivi di file sfogliabili e di intere macchine come immagini disco avviabili, secondo pianificazioni o su richiesta. Ripristina qualsiasi cosa, da un singolo file a un'intera macchina: sullo stesso computer, su una macchina sostitutiva o in una macchina virtuale Proxmox VE.

È una **suite di strumenti**:

- **Proxmox Backup Client GUI** (nata da RDEM Systems come Nimbus Backup GUI, rielaborata ed estesa da mjb-is): l'app per Windows e Linux. Set di backup con più pianificazioni ciascuno, backup di cartelle e di dischi interi con snapshot coerenti (VSS su Windows, elastio-snap/dattobd su Linux), ripristino di file dai backup di cartelle e di macchine, Recupero e Rollback, più server PBS, cifratura lato client e un servizio in background che esegue le pianificazioni anche senza utenti connessi.
- **`proxmoxbackup-directory`**: backup di cartelle (pxar) da riga di comando, con deduplicazione e rilevamento delle modifiche.
- **`proxmoxbackup-machine`**: backup a caldo di dischi interi (fidx) di macchine Windows e Linux da riga di comando. Dopo il ripristino l'immagine è avviabile.
- **`proxmoxbackup-nbd`**: collega un backup di disco come dispositivo a blocchi NBD (Linux).
- **Una ISO di ripristino bare-metal** basata su Clonezilla Live, con un ripristino guidato dall'accensione al riavvio.

> Parole chiave: proxmox backup client windows · client PBS · backup VSS Windows · backup offsite immutabili · da fisico a virtuale (P2V) · ripristino bare-metal · interfaccia Proxmox Backup Server.

<a id="-new-in-20"></a>
## 🆕 Novità della 2.0

La versione 2.0 porta nel progetto un ampio lavoro sulla GUI, sul servizio in background e sul motore di backup (vedi il [CHANGELOG](CHANGELOG.md)). Rispetto alla 1.2:

- **L'app si connette al servizio in background con i diritti di amministratore.** Con il servizio installato, i server PBS e i set di backup del computer appartengono al servizio, e solo il servizio legge la propria cartella delle impostazioni. L'app chiede una volta i diritti di amministratore per connettersi (UAC su Windows, pkexec su Linux; su un account standard, nome e password di un amministratore). Solo un piccolo programma ausiliario viene elevato, e l'app tiene il token del servizio solo in memoria. Chi non può rispondere alla richiesta può scegliere **Usa le mie impostazioni** e mantenere i propri server e set di backup, che vengono eseguiti mentre l'app è aperta; in seguito **Connetti** propone di copiarli in quelli del computer. L'app stessa funziona senza diritti di amministratore e offre **Riavvia come amministratore** quando un'operazione li richiede. Vedi la sezione 15 del [manuale della GUI](docs/manual/GUI.md#connecting-to-the-background-service).
- **L'app verifica di parlare davvero con il servizio.** Prima di inviare qualsiasi cosa all'API locale del servizio, l'app controlla chi risponde: il processo del servizio stesso su Windows, un socket di proprietà di root su Linux. Un programma che occupa la porta mentre il servizio è fermo non riceve nulla.
- **Set di backup con più pianificazioni ciascuno**, da ogni pochi minuti a una volta l'anno (l'ultimo giorno del mese, il 2° martedì), ognuna descritta a parole con le sue prossime 3 esecuzioni.
- **Backup incrementali di cartelle** con il rilevamento delle modifiche del client ufficiale (modalità Dati e Metadati): i file non modificati non vengono riletti.
- **Backup macchina più veloci** che saltano lo spazio libero, il file di paging e il file di ibernazione, e scrivono zeri al posto dei settori illeggibili invece di fallire.
- **Più backup contemporaneamente** (**Numero massimo di backup simultanei**, da 1 a 8, predefinito 3), anche nel servizio in background, ciascuno con il proprio snapshot, il proprio registro, Pausa e Ferma.
- **Limiti di velocità** per l'invio a PBS, lo scaricamento da PBS e la lettura dei dischi, ciascuno un unico budget condiviso da tutto ciò che è in esecuzione nello stesso momento.
- **Recupero e Rollback** per i set di backup di cartelle e di macchine, nella GUI e da riga di comando.
- **File recuperati dai backup macchina**: sfoglia i volumi NTFS, FAT, exFAT, ext2/3/4 e XFS all'interno di un'immagine disco e ripristina singoli file.
- **Chiavi di cifratura da conservare**: salvane una copia, stampa una chiave su carta con un codice QR e importa una chiave da un file, dalla fotocamera, da una foto o da testo digitato.
- **I set di backup macchina riconoscono i dischi dalla loro identità**, così un disco che cambia numero dopo una modifica hardware resta quello di cui si esegue il backup, e un disco mancante fa fallire l'esecuzione invece di far eseguire il backup di un altro disco.
- **Si possono escludere partizioni da un backup macchina**: spunta sotto ogni disco le partizioni di cui eseguire il backup. Una partizione esclusa non viene letta né inclusa nello snapshot ed entra nell'immagine come zeri, quindi un ripristino la restituisce al suo posto e con la sua dimensione, vuota e non formattata. Il selettore dei dischi evidenzia le partizioni di cui un disco ripristinato ha bisogno per avviarsi.
- **Backup macchina Linux con elastio-snap o dattobd.** Un set macchina Linux può disattivare gli snapshot e leggere i dischi a caldo, ma solo se il suo proprietario spunta che accetta che il backup possa essere incoerente.
- **Ogni utente può scegliere il proprio tema di colori** nell'app senza marchio. Un'app con marchio mantiene i colori del suo marchio.
- **Abilita e disabilita un set di backup** dal suo editor oltre che dal suo menu, con un avviso al salvataggio quando un set non verrebbe eseguito così come è salvato (disabilitato, nessuna pianificazione attiva, un problema di snapshot, un disco mancante).
- **Pacchetti Linux** (`.deb`, `.rpm` e Arch), con nome di pacchetto e di servizio `proxmoxbackupclient`, insieme al servizio in background.
- **Una ISO di ripristino bare-metal guidata**, che ripristina anche in una macchina virtuale vuota (P2V). Una macchina di cui è stato eseguito il backup come macchina virtuale Proxmox VE può anche essere ripristinata direttamente da PBS in Proxmox VE, come qualsiasi VM di cui Proxmox VE stesso abbia eseguito il backup.
- **Build con marchio**: gli stessi binari sono distribuiti come installer Proxmox Backup Client, Nimbus Backup, Acme Backup ed Etitech Backup.
- **18 lingue dell'interfaccia.**

## ⭐ Cosa fa, con risultati misurati

| | Risultato |
|---|---|
| **Backup incrementali di cartelle** | Una condivisione di file da 480,9 GB con 761.866 file: prima esecuzione 4 h 17 min, ogni esecuzione successiva da 10 a 15 minuti. I file non modificati non vengono nemmeno aperti (rilevamento delle modifiche Metadati, come nel client ufficiale). |
| **Backup macchina veloci** | Lo spazio libero, il file di paging e il file di ibernazione non vengono letti, e lo spazio vuoto del disco non viene sottoposto a hash. Il disco da 1 TB di un portatile, con 373 GB occupati, è passato da una durata tra 1 h 23 min e 1 h 44 min a soli 38 minuti. |
| **Collegamenti lenti e a consumo** | Vengono inviati solo i dati nuovi, compressi, e l'app mostra quanto ha davvero attraversato la rete. Un piccolo PC Linux in una sede remota con connessione 4G (4,8 Mbit/s in upload): l'intera immagine del suo disco da 128 GB più la sua cartella home in 11 minuti, con circa 172 MiB trasferiti. |
| **File recuperati dai backup macchina** | L'unità C: da 127 GB di un portatile, con 430.000 file, viene elencata in 4-15 secondi. Prima non viene montato né scaricato nulla. |
| **Ripristino bare-metal** | Avvia la ISO (chiavetta USB, CD o rete), inserisci i dati di PBS, scegli il backup: una macchina Windows da 32 GB era di nuovo alla schermata di accesso 8 minuti dopo l'accensione. |
| **Recupero** | Dopo l'eliminazione di 39 GB da una condivisione di 761.000 file, una ricerca di Recupero sugli ultimi 7 giorni di backup ha trovato esattamente quei file in circa un minuto e li ha rimessi al loro posto. |
| **Fedeltà del ripristino** | Permessi NTFS, attributi, stream di dati alternativi, attributi estesi, date di creazione e date delle cartelle su Windows; modalità, proprietari, ACL, attributi estesi e collegamenti simbolici su Linux. |

## 📖 Manuali utente

| Manuale | Contenuto |
|---|---|
| **[Manuale della GUI](docs/manual/GUI.md)** | Installazione, server PBS, set di backup e pianificazioni, backup incrementali, avanzamento, ripristino, Recupero, Rollback, Report, registri, Preferenze, il servizio in background, risoluzione dei problemi |
| **[Manuale della riga di comando](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: tutte le opzioni, file di configurazione, codici di uscita, pianificazione; Recupero e Rollback da riga di comando |
| **[Manuale del ripristino bare-metal](docs/manual/BMR.md)** | La ISO di ripristino, passo per passo; backup `vm` su Proxmox VE; da fisico a virtuale; hardware diverso |
| [Scenari di ripristino](docs/RESTORE_GUIDE.md) | Quale ripristino usare e quando |
| [Indice dei manuali](docs/manual/README.md) | Una panoramica dei manuali |

Gli stessi manuali sono integrati nell'app (**Guida**, F1), quindi funzionano offline e corrispondono alla versione installata.

<a id="-download"></a>
## 📦 Download

👉 **[Scarica l'ultima release](https://github.com/tizbac/proxmoxbackupclient_go/releases)**

| Hai | Usa |
|---|---|
| Windows | L'**installer MSI**, `ProxmoxBackupClient-<version>.msi`. Installa l'app e il servizio in background in Program Files, avvia il servizio e aggiunge una voce nel menu Start. Gli installer con marchio (`NimbusBackup-<version>.msi`, `AcmeBackup-<version>.msi`, `EtitechBackup-<version>.msi`) sono la stessa build con un altro marchio |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-*.rpm` |
| Arch | Il pacchetto generato da [packaging/arch/PKGBUILD](packaging/arch/PKGBUILD) (`make pkgarch`) |
| Solo la riga di comando | Gli archivi per piattaforma (`proxmoxbackup-cli-*` e `proxmoxbackupclient_go_*`) |
| Una macchina da ripristinare | La ISO di ripristino bare-metal ([manuale del ripristino bare-metal](docs/manual/BMR.md#3-get-the-iso-and-make-the-boot-media)), oppure compilala tu stesso ([più avanti](#-bare-metal-restore-iso)) |

Quando una release non include un pacchetto Linux, compilalo con `make deb`, `make pkgfedora` o `make pkgarch` ([BUILD.md](BUILD.md), [packaging/debian/README.md](packaging/debian/README.md)).

I pacchetti Linux installano una sola voce di menu, **Proxmox Backup Client**, che avvia l'app con il tuo utente. Una nuova installazione attiva il servizio in background (`proxmoxbackupclient.service`); un aggiornamento mantiene la scelta che hai fatto. I pacchetti richiedono polkit (pkexec), che l'app usa per connettersi al servizio.

> ⚠️ **Windows segnala "virus rilevato" (ad es. `Trojan:Win32/Sabsik.FL.A!ml`) o mostra un avviso di SmartScreen?**
> È un **falso positivo noto** per le applicazioni Go/Wails: *non* è un virus. Il suffisso `!ml` indica un rilevamento da parte di un modello di machine learning che segnala gli eseguibili *non firmati e poco comuni*. Verifica il download come mostrato qui sotto.

### 🔎 Verificare un download

Ogni release fornisce i checksum SHA-256 (`SHA256SUMS.txt`, riportati anche nella pagina della release) e un'**attestazione di provenienza firmata**: la prova crittografica che il file è stato prodotto dalla CI di questo repository, a partire da un commit preciso.

```powershell
Get-FileHash .\ProxmoxBackupClient-<version>.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
gh attestation verify .\ProxmoxBackupClient-<version>.msi --repo tizbac/proxmoxbackupclient_go
```

Su Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`.

Quando gli installer di una release risultano puliti su VirusTotal, la pagina della release rimanda ai rapporti.

> ℹ️ **Firma del codice:** i binari Windows **non sono ancora firmati con Authenticode** (è in attesa un certificato OSS tramite [SignPath Foundation](https://signpath.org)). Nel frattempo la provenienza è garantita dall'attestazione e dai checksum indicati sopra.

## 🚀 Avvio rapido (GUI)

1. Installalo: l'MSI su Windows, il pacchetto `.deb`, `.rpm` o Arch su Linux ([Download](#-download)).
2. Avvia **Proxmox Backup Client**. Gli installer configurano il servizio in background, quindi l'app chiede una volta i diritti di amministratore per connettersi (UAC su Windows, la tua password tramite pkexec su Linux). Se non puoi concederli, scegli **Usa le mie impostazioni**.
3. Aggiungi il tuo server PBS in **Preferenze > Informazioni account** e verificalo con **Test**.
4. Crea un set di backup (o un backup una tantum) e seleziona le cartelle o i dischi di cui eseguire il backup.
5. Eseguilo. Con il servizio in background, le sue pianificazioni vengono eseguite che ci sia o no un utente connesso.

Il [manuale della GUI](docs/manual/GUI.md) descrive ogni passaggio.

### 🪟 Installazione automatica / silenziosa (MSI)

L'MSI rispetta l'opzione silenziosa standard di Windows Installer, più proprietà pubbliche per ogni valore predefinito:

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

| Proprietà | Predefinito | Significato |
| --- | --- | --- |
| `SERVICEAUTOSTART` | `1` | `0` = servizio installato come **Manuale** e lasciato **arrestato**: installato, ma mai abilitato né avviato automaticamente |
| `AUTOSTART` | `1` | `0` = non avviare l'app nell'area di notifica all'accesso di ogni utente |
| `DESKTOPSHORTCUTS` | `1` | `0` = nessun collegamento sul desktop (la voce nel menu Start viene sempre aggiunta) |
| `CONFIGFILE` | | Percorso completo di un file salvato con **Preferenze > Esporta impostazioni...**: i suoi server PBS e set di backup vengono uniti prima dell'avvio del servizio. Gli ID backup ricavati dal nome del computer di origine vengono ricavati dal nome di ciascun computer. Vedi [installer/DEPLOYMENT.md](installer/DEPLOYMENT.md) |
| `KEEP_CONFIG` | `1` | Solo disinstallazione: `0` = elimina anche la cartella di stato `%ProgramData%\<name>` |
| `INSTALLFOLDER` | `Program Files` | Cartella di installazione (modificabile anche nell'interfaccia) |

Esegui con privilegi elevati (amministratore); codice di uscita `0` = successo, `3010` = successo + riavvio necessario. Gli MSI con marchio accettano esattamente le stesse opzioni.

## ✨ Funzionalità

### GUI
Le funzionalità principali sono elencate in [Novità della 2.0](#-new-in-20). Inoltre:
- 18 lingue dell'interfaccia: francese, inglese, italiano, tedesco, polacco, spagnolo, bulgaro, ceco, greco, ungherese, lettone, lituano, olandese, portoghese, rumeno, slovacco, turco e ucraino
- **Backup una tantum** di cartelle o di dischi interi, e backup macchina salvati come `host` o come backup `vm` di Proxmox VE, con un'opzione di **copia forense** che legge anche lo spazio libero
- **Ripristino** con ricerca dei file (caratteri jolly, espressioni regolari, percorsi completi), ripristino selettivo, ACL NTFS/POSIX, stream di dati alternativi, date di creazione di Windows e una verifica facoltativa
- **Pausa, Riprendi e Ferma** per un backup in corso, anche dall'area di notifica su Windows e per le esecuzioni del servizio in background
- Scheda di avanzamento in tempo reale (fine prevista, file elaborati, velocità di upload, dati trasferiti, rapporto di compressione), **Report**, un **Registro messaggi** e **Visualizza registri completi**
- Notifiche via email, azioni dopo il backup, clonazione dei set di backup, esportazione e importazione delle impostazioni
- Più server PBS, con pinning dell'impronta del certificato (fiducia al primo utilizzo), accesso con token API o con utente e password
- Cinque temi di colori e un colore personalizzato, scelti da ciascun utente (nell'app senza marchio; un'app con marchio mantiene i colori del suo marchio)

### Strumenti da riga di comando
- `proxmoxbackup-directory`: backup di cartelle (pxar) con deduplicazione e `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: backup a caldo di dischi interi di macchine Windows (VSS) e Linux (elastio-snap/dattobd) (fidx, incrementali, spazio libero saltato salvo con `-readfreespace`, `-type vm` per un backup Proxmox VE, `-skippartition <disk>:<number>` per escludere una partizione)
- `proxmoxbackup-nbd`: server NBD per ripristinare i backup di dischi (Linux)
- `ProxmoxBackupClientSVC` (Windows) e `proxmoxbackupclient` (pacchetti Linux): `-list-sets`, `-undelete` e `-rollback` per i set di backup definiti nella GUI

I primi tre accettano `-keyfile` e `-keyfile-passphrase` per gli snapshot cifrati. Riferimento completo: [manuale della riga di comando](docs/manual/CLI.md).

### 📸 Schermate

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Set di backup**: job con nome, con badge di pianificazione e di tipo, ed Esegui ora su ciascuno

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Avanzamento del backup**: velocità, fine prevista, chunk nuovi e riutilizzati

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Ripristino**: scegli un server PBS e un ID backup, elenca gli snapshot o cercali per nome di file

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**Pianificazioni**: più di una per set, con le prossime 3 esecuzioni

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Backup come VM Proxmox VE**: l'ID backup di PBS viene mostrato mentre scrivi

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Report**: storico completo delle esecuzioni con un pannello di dettaglio

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-machine-partitions.png" width="280">](docs/screenshots/gui-machine-partitions.png)
**Set macchina**: spunta sotto ogni disco le partizioni di cui eseguire il backup

</td>
<td width="33%">

[<img src="docs/screenshots/gui-themes.png" width="280">](docs/screenshots/gui-themes.png)
**Temi**: sei temi di colore, scelti da ciascun utente

</td>
<td width="33%">

[<img src="docs/screenshots/gui-languages.png" width="280">](docs/screenshots/gui-languages.png)
**Lingue**: 18, qui francese, tedesco, italiano, spagnolo, polacco e ucraino

</td>
</tr>
</table>

### Esclusioni di sistema intelligenti (backup di cartelle)
I backup di cartelle escludono da soli questi elementi:

**Cartelle di sistema:** `System Volume Information` (archivio VSS, può superare i 100 GB) e `$RECYCLE.BIN` ovunque si trovino; `Recovery` solo nella radice di un'unità; `.pbs-rollback` (la cartella di sicurezza di Rollback).
**File di sistema:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` e `DumpStack.log.tmp`.

**Perché è importante:** un'unità può mostrare 1,03 TB occupati mentre i file reali sono ~141 GB. Senza esclusioni il backup includerebbe gli snapshot VSS (spazio e tempo sprecati); con le esclusioni, la dimensione corrisponde ai dati reali.

**Consiglio:** usa un backup di **cartelle** per i backup a livello di file; usa un backup **macchina** in un set di backup separato per il ripristino bare-metal e il P2V (un'immagine del disco intero, in cui lo spazio libero e i file di paging e di ibernazione entrano come zeri).

### Sicurezza e qualità
- La cartella delle impostazioni del servizio è accessibile solo a SYSTEM e agli amministratori (Windows) o solo a root (Linux); la sua API locale è in ascolto su 127.0.0.1 e richiede un token, sostituito a ogni avvio e ogni 24 ore
- L'app non invia nulla a quell'API finché non ha verificato che a rispondere sia il servizio stesso (il processo del servizio su Windows, un socket di proprietà di root su Linux)
- Su Windows, i token e le password PBS in `config.json` sono cifrati per il computer (DPAPI)
- Validazione degli input, sanificazione delle credenziali, prevenzione del path traversal, nuovi tentativi con backoff esponenziale
- Test unitari in ogni modulo, più test dal vivo facoltativi e test end-to-end su un PBS reale

## 🔐 Cifratura lato client

Gli snapshot possono essere cifrati prima di lasciare la macchina, usando un file di chiave di Proxmox Backup Server (il formato scritto da `proxmox-backup-client key create`, quindi le chiavi sono intercambiabili con il client ufficiale). Ogni chunk è cifrato con AES-256-GCM e il manifest è firmato. Il server PBS non vede mai la chiave.

**GUI:** apri **Preferenze > Informazioni account**, modifica un server e usa la sua scheda **Cifratura**. Con **Importa...** carichi una chiave che hai già (una copia del file di chiave, oppure una chiave su carta letta con la fotocamera, da una scansione o una foto, o digitata) e con **Genera...** ne crei una nuova. La scheda mostra l'impronta della chiave. **Salva una copia…** e **Stampa una chiave su carta…** (con un codice QR) permettono di conservarne una copia lontano dal computer. I report indicano se ogni backup era cifrato. **Senza il file di chiave uno snapshot non può essere ripristinato: conservane una copia in un luogo sicuro, lontano da questa macchina.**

**Chiavi protette da passphrase (scrypt o PBKDF2):** **Chiedimela** tiene la passphrase in memoria per una sessione dell'app; **Ricorda su questo computer** la memorizza come il token PBS ed è la scelta giusta per le pianificazioni e il servizio in background.

**Riga di comando:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` (o `PBS_ENCRYPTION_PASSWORD`) serve solo per una chiave protetta, e se manca la passphrase viene richiesta. `proxmoxbackup-machine` e `proxmoxbackup-nbd` accettano le stesse opzioni. `proxmoxbackup-nbd` verifica la chiave rispetto allo snapshot prima di toccare il dispositivo NBD, quindi una chiave sbagliata fallisce subito.

**Ripristino bare-metal:** la ISO di ripristino chiede la chiave (nessuna, un file di chiave su una chiavetta USB o un percorso digitato) e la passphrase di una chiave protetta. La chiave non viene mai scritta nella ISO.

Il ripristino di uno snapshot cifrato richiede lo stesso file di chiave; se va perso, non c'è modo di recuperarlo.

## Backup di cartelle più veloci: rilevamento delle modifiche

Nella modalità predefinita Classico ogni backup di cartella legge ogni file, quindi un'esecuzione dura quanto la lettura dell'intera cartella anche quando non è cambiato nulla. I set di backup e `proxmoxbackup-directory` possono invece usare le modalità di rilevamento delle modifiche del `proxmox-backup-client` ufficiale:

| Modalità | Legge | Salvato come |
|---|---|---|
| **Classico** (Legacy, predefinito) | ogni file | un `<name>.pxar.didx` per cartella |
| **Dati** (Data) | ogni file | un archivio diviso: `<name>.mpxar.didx` (metadati) + `<name>.ppxar.didx` (contenuto dei file) |
| **Metadati** (Metadata) | solo i file nuovi e modificati | lo stesso archivio diviso; i file la cui dimensione, data di modifica (al nanosecondo), modalità e proprietario coincidono con il backup precedente vengono presi da esso senza essere letti |

In modalità **Metadati** la prima esecuzione legge tutto; le esecuzioni successive leggono solo ciò che è cambiato. Un file modificato in modo da mantenere dimensione e data di modifica non viene rilevato, ed è per questo che un set può anche **rileggere tutti i file ogni N esecuzioni**. Dai a ogni set di backup il proprio ID backup: la modalità Metadati confronta con lo snapshot più recente del gruppo di backup. Da riga di comando usa `-change-detection-mode`.

**Prima di passare un set a Dati o Metadati,** aggiorna alla 2.0 tutte le macchine da cui esegui i ripristini: le versioni precedenti mostrano le cartelle di un archivio diviso senza i loro file. Vedi la [sezione 7 del manuale della GUI](docs/manual/GUI.md#7-incremental-folder-backups-change-detection).

<a id="-physical-machines-into-virtual-machines-p2v"></a>
## 🔄 Da macchine fisiche a macchine virtuali (P2V)

Il backup macchina di un PC o di un server fisico può diventare una macchina virtuale Proxmox VE, in due modi:

1. **Ripristinarlo in Proxmox VE.** Imposta il set di backup (o il backup una tantum) su **Backup come: Macchina virtuale Proxmox VE**. Il backup viene salvato come `vm/<number>`, quindi Proxmox VE lo elenca insieme agli altri backup di VM e lo ripristina come VM (un disco SATA; aggiungi una scheda di rete prima di avviarla).
2. **Ripristinarlo con la ISO di ripristino in una VM vuota** con lo stesso firmware della macchina originale, un disco SATA almeno altrettanto grande e una scheda di rete Intel E1000. Funziona allo stesso modo per i backup `host` e `vm`, e lo stesso backup si può comunque ripristinare anche su bare metal.

I percorsi testati comprendono Windows 10 e Windows Server (UEFI/GPT), desktop Linux (UEFI/GPT e BIOS/MBR) e un backup Linux cifrato con la chiave su una chiavetta USB. Il [manuale del ripristino bare-metal](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines) contiene la tabella completa, le scelte del firmware, i driver e le istruzioni passo per passo.

## 🤝 Compatibile con il client ufficiale di Proxmox, in entrambe le direzioni

Verificato con `proxmox-backup-client` 3.4.9 e PBS 4.2:

- **Gli snapshot Classico, Dati e Metadati creati da questo client si ripristinano byte per byte identici con il client ufficiale**, sia dalla GUI sia da riga di comando, creati su Windows o su Linux, e il browser dei file di PBS 4.2 li apre.
- **Gli snapshot Classico, Dati e Metadati creati dal client ufficiale si ripristinano byte per byte identici con questo client**, completamente o in modo selettivo.
- Stesso formato di archivio diviso (pxar versione 2), stesse modalità di rilevamento delle modifiche, stesso formato del file di chiave di cifratura, stesse variabili d'ambiente in stile `PBS_REPOSITORY`.

I tuoi backup non sono mai vincolati a questo client. L'unica aggiunta è un piccolo file nella radice di uno snapshot di cartella, `.proxmox_backup_client_meta.json` (il percorso originale), che il client ufficiale ripristina come un normale file.

## Backup a caldo dell'intera macchina

`proxmoxbackup-machine` (e i backup macchina della GUI) eseguono il backup di un sistema Windows 10/11 o Windows Server completo, o di una macchina Linux, senza alcun fermo.

La sintassi del comando è in gran parte la stessa dei backup di cartelle, salvo che `-backupdir` è sostituito da `-backupdev` (da ripetere per più dischi). Ad esempio:

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Il comando qui sopra esamina il disco 0, rileva tutte le partizioni montate, ne crea uno snapshot VSS e poi crea un'immagine di backup avviabile dell'intero disco in formato fidx. Lo spazio libero e i file di paging e di ibernazione entrano nell'immagine come zeri senza essere letti (su Linux anche lo spazio libero di ext2/3/4 e XFS); `-readfreespace` legge invece tutto, per una copia forense. Un settore che non si può leggere entra nell'immagine come zeri, e l'esecuzione indica dove. Aggiungi `-type vm` (con un `-backup-id` numerico) per salvare il backup come backup di VM Proxmox VE ([P2V](#-physical-machines-into-virtual-machines-p2v)).

Il backup successivo è incrementale. Il calcolo degli hash è parallelizzato, quindi si possono raggiungere velocità di 1 GB/s.

Per escludere una partizione, ad esempio una libreria di giochi o una partizione di dati già coperta da un backup di cartelle, aggiungi `-skippartition <disk>:<number>` (da ripetere per più partizioni), come `-skippartition \\.\PhysicalDrive0:4` o `-skippartition /dev/sda:3`. La partizione non viene letta né inclusa nello snapshot ed entra nell'immagine come zeri: la tabella delle partizioni viene mantenuta, quindi un ripristino restituisce la partizione al suo posto e con la sua dimensione, vuota e non formattata. Nella GUI, togli la spunta dalla partizione sotto il suo disco nel set di backup.

### Prerequisiti per il backup macchina su Linux (equivalente di VSS)

Il backup macchina su Windows usa VSS. Linux non ha VSS, quindi il backup macchina di un host Linux in funzione richiede il modulo del kernel `elastio-snap` (un fork di `dattobd`) per creare uno snapshot puntuale del dispositivo a blocchi mentre viene scritto. Funziona anche `dattobd` stesso, se è già caricato.

Il repository originale `elastio/elastio-snap` è archiviato e non si compila più sui kernel attuali. Usa il fork mantenuto, [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap) (ramo `develop`). La versione 0.12.11 di quel fork si è compilata e ha funzionato su Debian 13 con il kernel 6.12. Il `.deb` suggerisce il pacchetto `elastio-snap-dkms`.

Servono due cose, altrimenti un backup macchina Linux si rifiuta di partire:

1. **Il modulo del kernel deve essere installato e caricato.** Compilazione dai sorgenti:
   ```bash
   sudo apt-get install linux-headers-$(uname -r) build-essential   # Debian/Ubuntu
   git clone --depth 1 https://github.com/Axcient/elastio-snap.git
   cd elastio-snap
   sudo make
   sudo make install
   sudo depmod -a      # `make install` does not run depmod, and modprobe fails without it
   sudo modprobe elastio-snap
   lsmod | grep elastio_snap
   ```
   Questo lo carica solo per l'avvio corrente. Aggiungi una voce in `/etc/modules-load.d/` per caricarlo a ogni avvio. Il suo [INSTALL.md](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md) tratta le installazioni da pacchetto.

2. **Il backup deve essere eseguito come root.** L'interfaccia di controllo del modulo è riservata a root, e lo snapshot viene rifiutato a un utente normale. Il servizio in background viene eseguito come root; senza di esso, usa **Riavvia come amministratore** nell'app, oppure esegui `proxmoxbackup-machine` con `sudo`. Un semplice backup di cartelle non richiede root.

Nella GUI, un set di backup macchina Linux può togliere la spunta da **Usa snapshot** e leggere i dischi a caldo, ma solo con la seconda casella spuntata ("Esegui senza snapshot: accetto che il backup possa essere incoerente"). Senza di essa, ogni esecuzione si ferma e ne spiega il motivo. Le partizioni non montate vengono lette direttamente.

### Ripristino di file da un backup di disco

La pagina Ripristino della GUI apre un backup macchina come un backup di cartelle: ogni volume dell'immagine (NTFS, FAT, exFAT, ext2/3/4 o XFS) è una cartella che puoi sfogliare e da cui puoi ripristinare. Non viene montato nulla.

Su Linux puoi anche collegare un backup di disco come dispositivo a blocchi con lo strumento nbd. Esegui prima `modprobe nbd max_part=0`: con `max_part != 0` il kernel può entrare in un ciclo infinito mentre sonda le partizioni.

Lo strumento NBD collega qualsiasi backup di disco fisso, che sia un backup di VM o di host (quindi funziona anche per i backup di Proxmox VE). Ad esempio:

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Se ometti `-path`, un'interfaccia testuale ti permette di selezionare il file fidx; `-list` invece stampa tutte le immagini.

> Non usarlo su una macchina che esegue qualcosa di importante: un file system corrotto può mandare in crash il sistema operativo, ed è per questo che Proxmox VE usa un'istanza QEMU per questa operazione.

Smonta tutto ciò che si trova sul disco nbd prima di fermare `proxmoxbackup-nbd`, altrimenti è probabile che la partizione resti occupata. Se ricevi un errore `Device or resource busy`, forza la disconnessione con `nbd-client -d /dev/nbd0`, oppure riavvia.

### Ripristino su una macchina fisica o su una VM

Usa la [ISO di ripristino bare-metal](#-bare-metal-restore-iso), su bare metal o in una macchina virtuale vuota. Il [manuale del ripristino bare-metal](docs/manual/BMR.md) la descrive passo per passo.

## Utilizzo: backup di directory

Il riferimento completo, con esempi per Windows e Linux, codici di uscita e pianificazione, è nel **[manuale della riga di comando](docs/manual/CLI.md)**. Un comando tipico:

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

Vengono lette anche le variabili d'ambiente `PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD` e `PBS_FINGERPRINT` del client ufficiale.

Per la configurazione JSON vedi [config.json.example](config.json.example); compila solo i campi che ti servono.

### Più directory in un'unica esecuzione

Ripeti `-backupdir` per eseguire il backup di più directory con un solo comando:

```shell
proxmoxbackup-directory.exe -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -datastore "datastorename" -backupdir "C:\Data" -backupdir "D:\Photos"
```

Il backup di ogni directory viene salvato in un proprio gruppo di backup, con l'ID backup `<backup-id>_<path>` (`-backup-id` ha come valore predefinito il nome host). Conservazione, pruning e ripristino trattano quindi ogni directory come una serie indipendente. Una directory che fallisce non ferma le altre, e il codice di uscita segnala l'errore. `-pxarout` non può essere combinato con più directory, e due directory che finirebbero con lo stesso ID backup vengono rifiutate prima che venga caricato qualsiasi dato.

In un file di configurazione JSON usa `"backupdirs": ["C:\\Data", "D:\\Photos"]`. Il campo singolo `"backupdir"` continua a funzionare e può essere combinato con `"backupdirs"`. La cifratura funziona come per una singola directory: la chiave viene letta una volta, e il gruppo di backup di ogni directory viene cifrato con essa.

### Escludere le date dall'archivio

La data di modifica di ogni file e cartella fa parte dell'archivio, quindi una data che cambia modifica i byte dell'archivio, e i chunk attorno a quella voce vengono caricati di nuovo anche se al loro interno non è cambiato nulla. Per la maggior parte dei dati è il compromesso giusto. Alcuni alberi, però, continuano a toccare le proprie cartelle mentre i file restano uguali: un archivio che organizza i dati in cartelle con nomi di hash (`aa/bb/<rest of the hash>`) può finire per caricare di nuovo gran parte dell'archivio a ogni esecuzione solo per questo motivo.

`-no-mtime` (`"no-mtime": true` in un file di configurazione JSON) registra invece 1970-01-01 come data di ogni file e cartella, così solo le vere modifiche di contenuto costano nuovi chunk. Le date reali non vengono conservate da nessuna parte: un ripristino data ogni elemento al 1970-01-01. Con `-change-detection-mode metadata` ogni file viene letto, come in modalità `data`. L'opzione è disattivata per impostazione predefinita; usala per una sorgente in cui le date non contano.

### Modelli per le email

Funziona anche un relay che non richiede autenticazione (spesso sulla porta 25): ometti nome utente e password. Il mittente può includere un nome (`Backups <backup@example.com>`).

Per l'oggetto e il corpo delle email si usa il [motore di template di Go](https://pkg.go.dev/text/template); consulta la sua documentazione per la sintassi. Sono disponibili queste variabili:

- `.NewChunks`: numero di nuovi chunk creati
- `.ReusedChunks`: numero di chunk riutilizzati
- `.Datastore`: nome del datastore
- `.Error`: l'eventuale errore (`.ErrorStr` lo fornisce come testo, vuoto se non ce n'è)
- `.ReadErrors`: i file o le cartelle che non è stato possibile leggere; `.ReadErrorCount` indica quanti sono
- `.Hostname`: nome host della macchina
- `.StartTime`: ora di inizio del backup
- `.EndTime`: ora di fine del backup
- `.Duration`: durata del backup
- `.FromattedDuration`: durata formattata del backup (il nome è scritto così nel codice)
- `.Success`: vero quando il backup è terminato senza errori e senza nulla di illeggibile
- `.Partial`: vero quando uno snapshot è stato salvato ma alcuni file non è stato possibile leggerli
- `.Status`: `Success`, `Partial` o `Failed`

## Backup di stream

Esegue il backup di uno stream invece che di un archivio pxar. Ad esempio:

```
mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]
```

Usa il chunking buzhash per deduplicare lo stream stesso, ad esempio un archivio tar o un dump SQL.

## Problemi noti

Con l'antimalware di Windows Defender attivo, un backup può rallentare fino a circa il 25% della velocità raggiungibile.

Su Windows gli strumenti da riga di comando acquisiscono un lock di processo: un secondo backup avviato mentre uno è in corso stampa "Another backup job is running" ed esce con codice 2. Su Linux questo lock non c'è, quindi non pianificare due esecuzioni da riga di comando nello stesso momento. (La GUI e il servizio in background eseguono più backup contemporaneamente in sicurezza, vedi [Più backup contemporaneamente](docs/manual/GUI.md#67-several-backups-at-once).)

## 📋 Prerequisiti

- Windows 10/11 o Windows Server (64 bit); diritti di amministratore per l'installazione e per connettere l'app al servizio in background
- oppure Linux: i pacchetti sono per Debian, Ubuntu, Mint, Fedora e Arch; la GUI richiede il runtime WebKitGTK e polkit; i backup macchina coerenti richiedono il modulo del kernel elastio-snap o dattobd e root
- Accesso di rete a un Proxmox Backup Server
- Per il P2V: Proxmox VE con il datastore PBS aggiunto come storage, oppure qualsiasi hypervisor in grado di avviare la ISO di ripristino

## 🔨 Compilazione dai sorgenti

**[BUILD.md](BUILD.md)** contiene le istruzioni complete: la GUI e il servizio in background, gli strumenti da riga di comando, l'MSI e i pacchetti Linux. In breve, con Go 1.25 o successivo e la CLI Wails v2.13:

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

Il frontend compilato (`gui/frontend/dist`) è incluso nel repository, quindi Node.js serve solo per modificare il frontend (`npm install` e `npm run build` in `gui/frontend`, oppure `wails dev` per il ricaricamento a caldo). Test: `make test`.

## 🔧 Uso avanzato e guide

### Multi-PBS (più server PBS)

Configura più server PBS e scegli la destinazione per ogni set di backup o backup una tantum (ad es. `C:\Users` ogni giorno verso un PBS veloce su SSD, `D:\` ogni settimana verso un PBS per grandi volumi di dati, più un server di disaster recovery).

- **[Guida utente](MULTI_PBS_USER_GUIDE.md)**: aggiunta e test dei server, server predefinito, scelta di un server per ogni backup, FAQ e risoluzione dei problemi.
- **[Guida all'implementazione](MULTI_PBS_GUIDE.md)**: modello dei dati, migrazione automatica da una configurazione con un solo PBS, metodi del backend.

Una vecchia configurazione con un solo PBS viene migrata automaticamente a un server `default` al primo caricamento.

<a id="-bare-metal-restore-iso"></a>
### 💿 ISO di ripristino bare-metal

La ISO di ripristino è una ISO standard di Clonezilla Live modificata con i binari `pbsnbd` / `machinebackup` più due voci di menu. Si avvia da CD, da chiavetta USB (scritta con `dd`, o da una chiavetta Ventoy), e su macchine UEFI o BIOS:

- **`ocs-pbs-bare-metal-restore`**: la prima voce del menu di avvio, quella predefinita, **PBS Bare Metal Restore**. Salta le domande di Clonezilla su lingua e tastiera e va direttamente dalle credenziali PBS a un ripristino completato: prima DHCP (la richiesta di un IP statico solo se fallisce), una chiave di cifratura da una chiavetta USB se il backup è cifrato, e un rilevamento del disco di destinazione che non propone mai la chiavetta da cui si è avviata. Ripristina allo stesso modo i backup Windows e Linux (il ripristino avviene a livello di blocco), su bare metal o in una VM vuota.
- **`ocs-pbs-nbd`**: una voce **pbs-nbd** nel menu di selezione della modalità di Clonezilla. Chiede i dati di connessione a PBS e lo snapshot, lo collega a `/dev/nbd0`, poi passa alla normale procedura guidata di ripristino del disco di Clonezilla, che conduci tu manualmente.

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

`make clonezilla` scarica la ISO standard e fa lo stesso. Tutti i dettagli (perché una ricostruzione completa della ISO invece di una sostituzione sul posto, prerequisiti, flusso dei menu, verifica) sono in **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**; il suo utilizzo, nel **[manuale del ripristino bare-metal](docs/manual/BMR.md)**.

### Compilare la GUI per Windows

**Windows nativo**: vedi **[BUILD.md](BUILD.md)** per la toolchain (Go e MinGW da Chocolatey, Wails) e i comandi per l'app, il servizio e l'MSI.

**Docker (per compilare su Linux).** `./build_gui_windows_docker.sh` compila l'app Windows come `ProxmoxBackupClientGO.exe`, con il pieno supporto di WebView2, in un container `golang` usa e getta. Non compila il servizio in background né l'MSI.

### Changelog e documentazione interna

- **[CHANGELOG.md](CHANGELOG.md)**: storico delle modifiche per versione.
- **[TODO.md](TODO.md)**: roadmap aperta e idee.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**: stato stabile del prodotto e build disponibili.
- **[installer/DEPLOYMENT.md](installer/DEPLOYMENT.md)**: distribuzione dell'MSI su molti computer.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**: finestra di disinstallazione dell'MSI (mantenere o eliminare la configurazione) e il relativo piano di test.
- **[AGENTS.md](AGENTS.md)**: struttura del repository e convenzioni, per contributori e agenti di programmazione.

## 🌐 Supporto tecnico in Italia

Fermo restando che il progetto e **TUTTI** i suoi contributi resteranno per sempre pubblici e rilasciati con licenza GPLv3, il principale sponsor di questo progetto, e anche delle più recenti funzionalità di backup macchina, E.T.I. Srl ( https://etitech.net ), può fornire a chi ne ha bisogno supporto per i deployment Proxmox in generale e, in particolare, per gli strumenti di backup per Windows.

Non ci sarà mai un'edizione "community" distinta da un'edizione "enterprise": solo il supporto tecnico sarà un servizio indipendente.
Qualsiasi personalizzazione richiesta, anche se lo sviluppo potrà essere a pagamento, verrà rilasciata con licenza GPLv3 come l'intero progetto.

## 🖥️ Attribuzione della GUI

La **Proxmox Backup Client GUI** è nata da **[RDEM Systems](https://www.rdem-systems.com/)** come **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)**: RDEM Systems ha progettato e realizzato l'app originale e offre supporto commerciale per Nimbus Backup, la sua versione con marchio. **[mjb-is](https://github.com/mjb-is)** l'ha poi rielaborata ed estesa, ispirandosi al flusso di lavoro e allo stile di altri sistemi di backup, e quel lavoro è stato integrato in questo repository. L'intero codice, compresa la GUI e tutte le sue funzionalità, resta open source con licenza GPLv3.

## ⚠️ Avvertenza

Questo software è fornito "così com'è". Pur puntando all'affidabilità, decliniamo ogni responsabilità per la perdita o il danneggiamento di dati. Testa sempre i tuoi backup e verifica il ripristino prima di affidarti a essi in produzione. Nessun backup è affidabile finché non hai eseguito un ripristino da esso, quindi prova i tuoi ripristini, compreso un ripristino bare-metal.

Il software è ancora di qualità alpha e non ci assumiamo alcuna responsabilità per qualsiasi tipo di danno o perdita di dati, anche dei file sorgente.

Le segnalazioni di bug sono benvenute: apri una [issue](https://github.com/tizbac/proxmoxbackupclient_go/issues) indicando cosa hai fatto, cosa è successo e il registro (il Registro messaggi, oppure **Visualizza registri completi**).

## 📄 Licenza

GPLv3: vedi il file [LICENSE](LICENSE).

## 🏷️ Branding

Ogni contributore che abbia fornito **almeno 5 commit** che aggiungono funzionalità o correzioni ha il diritto di far aggiungere i propri dati di branding per uso commerciale.

Le uniche condizioni sono che l'azienda a cui rimanda il branding **non** svolga nessuna delle seguenti attività:

- Campagne di malware
- Attività che promuovono la guerra (vale per qualsiasi paese, compresi quelli occidentali)
- Truffe
- Furto di dati
- Traffico di minori
- Violenza
- Discriminazione
- Droga
- Qualsiasi attività generalmente riconosciuta come illegale

Se a uno dei contributori arriva un reclamo, cercheremo di metterci in contatto; se non viene fornita una spiegazione valida, **revocheremo immediatamente** quel beneficio.

La **licenza GPLv3 resta valida**, e sarai comunque libero di fare un fork del progetto e compilare i tuoi eseguibili.

## 🤝 Contribuire

I contributi sono benvenuti, in particolare:

1. Traduzioni dei README e dei manuali utente per la versione 2.0
2. Upload asincrono / multicore dei chunk (la compressione multicore è già implementata per il backup macchina)
3. Una patch lato Proxmox per aggiungere al formato pxar un altro tipo di voce contenente i descrittori di sicurezza di Windows
4. Supporto per i collegamenti simbolici e le giunzioni di Windows (oggi vengono saltati)
5. Test su più hardware, distribuzioni Linux e kernel, in particolare ripristini bare-metal
6. Qualsiasi idea interessante che ti venga in mente :)

## Informazioni sui contributori di Proxmox Backup Client GO

I contributori di Proxmox Backup Client GO sviluppano e mantengono questo progetto. Il software si basa sull'infrastruttura NTP/NTS e sugli [11 server NTS pubblici](https://github.com/jauderho/nts-servers) elencati nel riferimento della comunità.

---

**© 2024-2026 Proxmox Backup Client GO Contributors e RDEM Systems.**
