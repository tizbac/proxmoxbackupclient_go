# Proxmox Backup Client: client Windows e Linux per Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client è un client di backup open source (GPL-3.0) per Proxmox Backup Server (PBS), per Windows e Linux.** Esegue il backup di cartelle come archivi di file sfogliabili e di macchine intere come immagini disco avviabili, secondo pianificazioni o su richiesta, e ripristina qualsiasi cosa, da un singolo file a una macchina intera, sullo stesso computer, su una macchina sostitutiva o in una macchina virtuale Proxmox VE.

Lo sviluppo per il mio homelab, per fare di PBS l'unico sistema di backup di un insieme misto di server, desktop e portatili Windows e Linux, al posto di Backup for Workgroups, di mirror rsync e di immagini Clonezilla/RescueZilla. È nato a settembre 2026 come fork di [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) e lo ha ormai superato di molto: oltre 350 commit da allora, con 18 correzioni e funzionalità inviate al progetto originale ([elenco più sotto](#-contributi-al-progetto-originale)).

**Siete liberi di usarlo così com'è.** È ufficialmente **software alpha**, da usare a vostro rischio, senza alcuna garanzia di alcun tipo. Collaudo personalmente ogni release e lo uso regolarmente sulle mie macchine, ma leggete [Stato, segnalazioni di bug e richieste di funzionalità](#-stato-segnalazioni-di-bug-e-richieste-di-funzionalità) prima di farci affidamento.

La suite:

- **Proxmox Backup Client GUI**: l'app per Windows e Linux. Set di backup con più pianificazioni ciascuno, backup di cartelle e di dischi interi con snapshot coerenti (VSS su Windows, elastio-snap/dattobd su Linux), ripristino di file dai backup di cartelle e di macchine, Recupero (Undelete) e Rollback, più server PBS, cifratura lato client e un servizio in background, così le pianificazioni girano anche senza nessun utente collegato.
- **`proxmoxbackup-directory`**: backup di cartelle (pxar) da riga di comando, con deduplicazione e rilevamento delle modifiche.
- **`proxmoxbackup-machine`**: backup a caldo di dischi interi (fidx) da riga di comando, per macchine Windows e Linux; l'immagine è avviabile dopo un ripristino.
- **`proxmoxbackup-nbd`**: collega un backup di disco come dispositivo NBD (Linux).
- **Una ISO di ripristino bare-metal** basata su Clonezilla Live, con un ripristino guidato dall'accensione al riavvio.

> Parole chiave: proxmox backup client windows · PBS client · Windows VSS backup · physical to virtual (P2V) · bare-metal restore · Proxmox Backup Server interface.

## ⭐ Cosa fa, con risultati misurati

| | Risultato |
|---|---|
| **Backup incrementali di cartelle** | Una condivisione di un file server da 480,9 GB con 761.866 file: prima esecuzione 4 h 17 min, ogni esecuzione successiva da 10 a 15 minuti. I file invariati non vengono nemmeno aperti (il rilevamento delle modifiche Metadata del client ufficiale). |
| **Backup macchina veloci** | Lo spazio libero, il file di paging e il file di ibernazione non vengono letti, e lo spazio vuoto del disco non viene sottoposto a hash. Il disco da 1 TB di un portatile con 373 GB occupati: prima da 1 h 23 min a 1 h 44 min, ora 38 minuti. Un disco di prova da 50 GB: 3 min 36 s, ora 1 min 15 s. |
| **File dai backup macchina** | Sfogliare un'immagine disco come una cartella e ripristinare singoli file, da NTFS, FAT, exFAT, ext2/3/4 e XFS. L'unità C: da 127 GB di un portatile, con 430.000 file, viene elencata in 4-15 secondi; nulla viene montato o scaricato prima. |
| **Da macchina fisica a macchina virtuale** | Fare il backup di un PC o di un server, poi avviarlo come VM Proxmox VE: ripristinato tramite Proxmox VE stesso, oppure con la ISO di ripristino. [Vedere più sotto](#-macchine-fisiche-in-macchine-virtuali-p2v). |
| **Ripristino bare-metal** | Avviare la ISO (chiavetta USB, CD o PXE), inserire i dati del PBS, scegliere il backup: una macchina Windows da 32 GB era di nuovo alla schermata di accesso 8 minuti dopo l'accensione. |
| **Recupero** | Dopo la cancellazione di 39 GB da una condivisione di 761.000 file, una ricerca su 7 giorni ha trovato esattamente quei file in circa un minuto e li ha rimessi al loro posto. |
| **Rollback** | Riportare le cartelle di un set allo stato di un backup scelto, con un'anteprima prima e Annulla dopo. |
| **Pianificazioni** | Più di una per set di backup, da una ogni pochi minuti a una all'anno: "ogni giorno alle 02:00" più "l'ultimo giorno di ogni mese" o "il 2° martedì", ciascuna descritta a parole con le sue prossime 3 esecuzioni. |
| **Fedeltà del ripristino** | Permessi NTFS, attributi, stream di dati alternativi e attributi estesi, date di creazione e delle cartelle su Windows; modi, proprietari, ACL, attributi estesi e link simbolici su Linux. Un passaggio facoltativo rilegge quanto ripristinato e lo confronta con il backup. |
| **Funziona con gli strumenti di Proxmox** | I backup creati qui si ripristinano byte per byte con il `proxmox-backup-client` ufficiale, e viceversa. [Dettagli](#-compatibile-con-il-client-proxmox-ufficiale-in-entrambe-le-direzioni). |

## 🧩 Cosa aggiunge rispetto al progetto originale

Oltre alle [18 modifiche integrate nel progetto originale](#-contributi-al-progetto-originale), questo client aggiunge:

**Backup**
- Backup incrementali di cartelle con il rilevamento delle modifiche del client ufficiale (modalità Data e Metadata, archivi pxar v2 divisi), con una lettura completa ogni N esecuzioni.
- Backup macchina che saltano lo spazio libero, il file di paging e il file di ibernazione (NTFS su Windows; ext2/3/4 e XFS su Linux), scrivono come zeri i settori illeggibili e proseguono, e mostrano durante l'esecuzione il numero di blocchi, dischi, partizioni e chunk.
- Pausa, Riprendi e Ferma per i backup di cartelle e di macchine, anche per quelli eseguiti dal servizio in background.
- Stream di dati alternativi, attributi estesi NTFS e date di creazione nei backup di cartelle Windows.

**Pianificazioni e set di backup**
- Più pianificazioni per set di backup, da una ogni pochi minuti a una all'anno (l'n-esimo giorno della settimana, l'ultimo giorno del mese), ciascuna descritta a parole con le sue prossime 3 esecuzioni.
- Un servizio in background su Linux oltre che su Windows, e Riavvia come amministratore su Linux.
- Esegui ora rapido o completo, un badge VSS / Snapshot, un avviso **ID condiviso** (SHARED ID), i nomi dei set scritti in PBS come commenti degli snapshot, avvisi via e-mail, azioni dopo il backup, ed esportazione e importazione delle impostazioni.

**Ripristini**
- Singoli file dai backup macchina (NTFS, FAT, exFAT, ext2/3/4, XFS), senza montare né scaricare l'immagine.
- Recupero e Rollback, per i set di backup di cartelle e di macchine, nella GUI e da riga di comando.
- Una verifica facoltativa dopo un ripristino, le date di creazione Windows, e un albero degli snapshot che carica una cartella alla volta, così anche i backup molto grandi si aprono subito.
- Una ISO di ripristino bare-metal guidata che ripristina anche in una VM vuota, e percorsi da fisico a virtuale collaudati ([P2V](#-macchine-fisiche-in-macchine-virtuali-p2v)).
- Compatibilità byte per byte con il `proxmox-backup-client` ufficiale in entrambe le direzioni, inclusi i suoi snapshot Data e Metadata (archivio diviso).

**L'app**
- Un'interfaccia ricostruita: barra dei menu, menu laterale, Report con la cronologia completa di backup e ripristini, un Registro messaggi, cinque temi di colore e un colore personalizzato, notifiche nell'area di notifica e notifiche toast.
- 18 lingue dell'interfaccia, tutte riviste (l'originale ne ha 5).
- Cartella delle impostazioni riservata agli amministratori (Windows) o al suo proprietario (Linux), e un token API del servizio rinnovato a ogni avvio.
- Pacchetti MSI, `.deb` e `.rpm`, e i manuali utente per la GUI, la riga di comando e il ripristino bare-metal.

## 🔄 Macchine fisiche in macchine virtuali (P2V)

Un backup macchina di un PC o di un server fisico può diventare una macchina virtuale Proxmox VE, in due modi:

**1. Ripristinarlo in Proxmox VE.** Impostare il set di backup (o il backup una tantum) su **Backup come: Macchina virtuale Proxmox VE** (Backup as: Proxmox VE virtual machine). Il backup viene salvato come `vm/<numero>` con una piccola configurazione di VM, così Proxmox VE lo elenca insieme agli altri backup di VM: aprire lo storage PBS in Proxmox VE, selezionare il backup, **Restore**, scegliere un ID di VM e uno storage. La VM riceve un disco SATA e il firmware giusto per il disco (OVMF per UEFI/GPT, SeaBIOS per BIOS/MBR); aggiungere una scheda di rete prima di avviarla, e un disco EFI (Pre-Enroll keys non selezionato) se una VM UEFI non si avvia.

**2. Ripristinarlo con la ISO di ripristino bare-metal in una VM vuota.** Creare una VM con lo stesso firmware della macchina originale (OVMF con un disco EFI per UEFI/GPT, SeaBIOS per BIOS/MBR), un disco SATA grande almeno quanto l'originale e una scheda di rete Intel E1000, avviarla dalla ISO ed eseguire il ripristino. Funziona allo stesso modo per i backup `host` e `vm`, e lo stesso backup si ripristina comunque anche su una macchina fisica.

Percorsi collaudati:

| Da | A | Percorso | Risultato |
|---|---|---|---|
| Portatile Windows 10, UEFI/GPT | VM Proxmox VE | ISO di ripristino | Avviata sul desktop dell'utente |
| Windows Server, UEFI/GPT | VM Proxmox VE | Ripristino Proxmox VE | Avviata senza modifiche |
| Desktop Xubuntu, UEFI/GPT | VM Proxmox VE | Ripristino Proxmox VE, e la ISO di ripristino | Avviata sul desktop |
| Linux, BIOS/MBR | VM Proxmox VE | ISO di ripristino, e ripristino Proxmox VE | Avviata al prompt di accesso |
| Linux Mint, backup cifrato | VM Proxmox VE | ISO di ripristino, chiave da una chiavetta USB | Avviata |
| PC Linux | lo stesso PC con un nuovo disco | ISO di ripristino | Avviato |

Al primo avvio Windows configura il nuovo hardware virtuale ("Preparazione dei dispositivi"). Il [manuale del ripristino bare-metal](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines) contiene la tabella dei firmware, i driver e le istruzioni passo per passo.

## 📖 Manuali utente

| Manuale | Contenuto |
|---|---|
| **[Manuale della GUI](docs/manual/GUI.md)** | Installazione, server PBS, set di backup e pianificazioni, backup incrementali, avanzamento, ripristino, Recupero, Rollback, Report, log, Preferenze, il servizio in background, risoluzione dei problemi |
| **[Manuale della riga di comando](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: tutte le opzioni, file di configurazione, codici di uscita, pianificazione; Recupero e Rollback da riga di comando |
| **[Manuale del ripristino bare-metal](docs/manual/BMR.md)** | La ISO di ripristino, passo per passo; backup `vm` su Proxmox VE; da fisico a virtuale; hardware diverso |
| [Scenari di ripristino](docs/RESTORE_GUIDE.md) | Quale ripristino usare e quando |
| [Indice dei manuali](docs/manual/README.md) | Una panoramica dei tre manuali |

## 🆕 Novità della v0.9.0: più pianificazioni per set di backup, installer, un servizio Linux

- **Più pianificazioni per set di backup**, da una ogni pochi minuti a una all'anno: "ogni giorno alle 02:00" più "ogni mese l'ultimo giorno" o "il 2° martedì", ciascuna pianificazione descritta a parole con le sue prossime 3 esecuzioni. I set esistenti mantengono la loro pianificazione.
- **I backup di cartelle conservano gli stream di dati alternativi e gli attributi estesi NTFS** (Windows), come il contrassegno "scaricato da Internet", e un ripristino li rimette. La sezione Limiti noti ora non ne elenca più nessuno.
- **Installer**: un **MSI per Windows** (app e servizio in background in Program Files, voce nel menu Start, aggiornamenti sul posto, impostazioni mantenute o eliminate alla rimozione) e **pacchetti Linux**, `.deb` (Debian, Ubuntu, Mint) e `.rpm` (Fedora), con l'app, i tre strumenti a riga di comando, i manuali e le voci di menu.
- **Un servizio in background su Linux** (`proxmoxbackupclient.service`): i set di backup pianificati girano come root, anche quando nessuno è collegato. Si attiva in Preferenze > Avanzate > Esegui come servizio, come su Windows. **Riavvia come amministratore** ora funziona anche su Linux (tramite pkexec).
- **Pausa, Riprendi e Ferma per i backup eseguiti dal servizio**, con la loro scheda di avanzamento nell'app aperta.
- **Un avviso quando due set di backup condividono un gruppo di backup** (stesso server, tipo e ID Backup), con un badge **ID condiviso** su entrambe le schede.
- **Impostazioni protette**: su Windows `C:\ProgramData\ProxmoxBackupClient` è ora accessibile solo agli amministratori; su Linux la cartella delle impostazioni è privata del suo proprietario.
- **File dai backup macchina di tutti i file system più comuni**: FAT, exFAT, ext2/3/4 e XFS oltre a NTFS. I ripristini di immagini Windows rimettono anche permessi e stream di dati alternativi.
- **I backup macchina Linux saltano lo spazio libero** (ext2/3/4, XFS), come quelli Windows dalla v0.8.0.
- **Recupero e Rollback per i set di backup di macchine Linux**, e **da riga di comando** per qualsiasi set di backup: `-list-sets`, `-undelete`, `-rollback` (sezione 12 del [manuale della riga di comando](docs/manual/CLI.md)).
- Gli strumenti a riga di comando scrivono gli errori su stderr invece di mostrare finestre di messaggio, e `proxmoxbackup-machine` invia la sua e-mail di notifica.
- **La ISO di ripristino bare-metal** non propone mai come disco di destinazione la chiavetta da cui è stata avviata, e il manuale spiega come far corrispondere il firmware (UEFI o BIOS) quando una macchina fisica diventa una VM.
- **Tutte le lingue riviste** (17 traduzioni, circa 1.850 stringhe), e date e numeri seguono la lingua dell'app.

Release precedenti: **v0.8** backup macchina più veloci (spazio libero saltato, settori danneggiati superati), file dai backup macchina, Pausa e Riprendi; **v0.7** Recupero e Rollback; **v0.6** backup incrementali di cartelle. Il [CHANGELOG](CHANGELOG.md) contiene tutte le modifiche.

## 🤝 Compatibile con il client Proxmox ufficiale, in entrambe le direzioni

Verificato con `proxmox-backup-client` 3.4.9 e PBS 4.2:

- **Gli snapshot Legacy, Data e Metadata creati da questo client si ripristinano byte per byte con il client ufficiale**, sia dalla GUI sia da riga di comando, creati su Windows o su Linux, e il file browser di PBS 4.2 li apre. Su Linux i file ripristinati mantengono anche esattamente il modo, la data di modifica al nanosecondo e la destinazione dei link simbolici.
- **Gli snapshot Legacy, Data e Metadata creati dal client ufficiale si ripristinano byte per byte con questo client**, interamente o in modo selettivo.
- Stesso formato di archivio diviso (pxar versione 2), stesse modalità di rilevamento delle modifiche, stesso formato del file di chiave di cifratura (le chiavi di `proxmox-backup-client key create` funzionano qui, e le chiavi generate qui funzionano lì), stesse variabili d'ambiente in stile `PBS_REPOSITORY`.

Quindi i vostri backup non sono mai legati a questo client: gli strumenti di Proxmox ripristinano ogni snapshot che crea, e questo client ripristina ogni snapshot creato dal client ufficiale. L'unica aggiunta è un piccolo file che questo client mette alla radice di uno snapshot di cartella, `.proxmox_backup_client_meta.json` (il percorso originale), che il client ufficiale ripristina come un normale file.

> ⚠️ **Avvertenza:** questo progetto **non è in alcun modo affiliato** a **Proxmox Server Solutions GmbH**. "Proxmox", il logo Proxmox e i nomi correlati sono di proprietà dei rispettivi titolari; qui sono usati **solo** per indicare la compatibilità. Vedere [proxmox.com](https://www.proxmox.com/) per i loro prodotti.

## 📦 Download e installazione

👉 **[Scarica l'ultima release](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| Se hai | Usa |
|---|---|
| Windows | L'**installer MSI** (`ProxmoxBackupClient-<version>-x64.msi`) installa l'app e il servizio in background in Program Files, con una voce nel menu Start. Oppure estrai `ProxmoxBackupClient-v<version>-windows-amd64.zip` (l'app e `ProxmoxBackupClientSVC.exe`; tienili nella stessa cartella) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Un altro Linux | Estrai `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (richiede il runtime WebKitGTK 4.1) |
| Solo riga di comando | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` oppure `-linux-amd64.tar.gz` |
| Una macchina da ripristinare | `ProxmoxBackupClient-GO-BMR-Restore-v<version>.iso`, la ISO di ripristino bare-metal |

MSI, `.deb` e `.rpm` sono novità della v0.9.0: le release fino alla v0.8.1 hanno solo i file zip e tar.gz. [BUILD.md](BUILD.md) spiega come compilarli da sé.

I pacchetti Linux installano due voci di menu: **Proxmox Backup Client** avvia l'app con il vostro utente (backup di cartelle dei vostri file), e **Proxmox Backup Client (administrator)** la avvia come root, chiedendo la password tramite polkit, per i backup macchina e gli snapshot a blocchi. Il servizio in background viene installato ma non attivato; vedere [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **Windows mostra "virus rilevato" (ad esempio `Trojan:Win32/Sabsik.FL.A!ml`) o un avviso di SmartScreen?**
> È un **falso positivo noto** per le applicazioni Go/Wails: *non* è un virus. Il suffisso `!ml` indica un rilevamento da parte di un modello di machine learning che segnala gli eseguibili *non firmati e poco diffusi*. Verificate il download con `SHA256SUMS.txt` (più sotto).

### 🔎 Verificare un download

Ogni release include un `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-0.9.0-x64.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
```

Su Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`.

> ℹ️ **Non ci sono ancora firma del codice, attestazione firmata da CI né scansione VirusTotal.** Le release vengono compilate e caricate a mano, non ancora tramite una pipeline CI in grado di produrre un'attestazione verificabile della provenienza della build. Vedere [TODO.md](TODO.md) se volete aiutare a predisporla.

## ✨ Funzionalità

### GUI
- **🌍 Multilingue**: 18 lingue dell'interfaccia: francese, inglese, italiano, tedesco, polacco, spagnolo, bulgaro, ceco, greco, ungherese, lettone, lituano, olandese, portoghese, rumeno, slovacco, turco e ucraino
- Barra dei menu nativa e menu laterale (Backup, Ripristino, Recupero, Rollback, Report, Registro messaggi, Preferenze, Informazioni)
- **Set di backup**: job con nome riutilizzabili con **più pianificazioni ciascuno** (da una ogni pochi minuti a una all'anno, o manuale), un badge del tipo cartella o macchina, un badge VSS / Snapshot, "Esegui ora" (lettura rapida o completa per i set Metadati), e un avviso **ID condiviso** quando due set scriverebbero nello stesso gruppo di backup
- **Backup una tantum** di cartelle o di dischi interi
- Pagina **Report** con la cronologia completa di backup e ripristini, e un **Registro messaggi** di dimensione limitata per la diagnostica
- Finestra **Preferenze** con cinque temi di colore (Ambra, Blu, Verde, Rosso, Scuro) più un'opzione per un colore personalizzato
- **Backup incrementali di cartelle** (rilevamento delle modifiche, modalità Metadati): i file invariati vengono riutilizzati senza essere letti, come nel client ufficiale, con una lettura completa facoltativa ogni N esecuzioni
- **Backup macchina (disco intero)** su Windows e Linux che saltano lo spazio libero e superano i settori danneggiati, salvati come `host` o come backup `vm` di Proxmox VE
- **Da fisico a virtuale**: un backup macchina si ripristina in una VM Proxmox VE, tramite Proxmox VE o la ISO di ripristino
- **Ripristino**: navigazione degli snapshot, ricerca di file (caratteri jolly, espressioni regolari, percorsi completi) e ripristino selettivo, con ACL NTFS/POSIX, stream di dati alternativi, date delle cartelle, date di creazione Windows e una verifica facoltativa
- **File dai backup macchina**: sfogliare e ripristinare file da volumi NTFS, FAT, exFAT, ext2/3/4 e XFS all'interno di un'immagine disco
- **Recupero** e **Rollback**: trovare i file che i backup di un set contengono ancora ma che sul disco non ci sono più, e rimetterli al loro posto; oppure riportare le cartelle di un set allo stato di un backup scelto, con un'anteprima prima e Annulla dopo. Set di cartelle e di macchine. Vedere il [manuale della GUI](docs/manual/GUI.md#10-undelete)
- **Pausa, Riprendi e Ferma** un backup in corso
- **Esegui come servizio**: installare, avviare, fermare o rimuovere il servizio in background da Preferenze > Avanzate, così le pianificazioni girano senza nessun utente collegato (Windows e Linux)
- Scheda di avanzamento in tempo reale: ora di inizio e fine prevista, file elaborati sul totale, velocità di elaborazione (dati e file al secondo) e velocità di upload effettiva, file corrente, e il motivo per cui un'esecuzione sta leggendo tutti i file
- VSS (Volume Shadow Copy) su Windows, snapshot a blocchi elastio-snap/dattobd su Linux
- Notifiche e-mail e azioni dopo il backup, clonazione dei set di backup, esportazione e importazione delle impostazioni
- Più server PBS, con pinning dell'impronta del certificato (fiducia al primo utilizzo), accesso con token API o utente/password
- **Cifratura lato client**: file di chiave PBS per server (AES-256-GCM, manifest firmato), protezione facoltativa con passphrase, vedere [Cifratura lato client](#-cifratura-lato-client)
- Protezione da amministratore per la modifica dei set di backup, con Riavvia come amministratore
- Log di debug per la diagnostica

### Strumenti a riga di comando
- `proxmoxbackup-directory`: backup di cartelle (pxar) con deduplicazione e `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: backup a caldo di dischi interi di macchine Windows (VSS) e Linux (elastio-snap/dattobd) (fidx, incrementale, spazio libero saltato salvo con `-readfreespace`, `-type vm` per un backup Proxmox VE)
- `proxmoxbackup-nbd`: server NBD per ripristinare backup di disco (Linux)
- `ProxmoxBackupClientSVC` (Windows) e `proxmoxbackupclient` (pacchetti Linux): `-list-sets`, `-undelete` e `-rollback` per i set di backup definiti nella GUI

I primi tre accettano `-keyfile` e `-keyfile-passphrase` per gli snapshot cifrati. Riferimento completo: [manuale della riga di comando](docs/manual/CLI.md).

### 📸 Screenshot

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Set di backup**: job con nome con un badge per la pianificazione che verrà eseguita per prima (DAILY, WEEKLY, MONTHLY, +N per le altre) e un badge del tipo cartella/macchina, Esegui ora su ciascuno

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Avanzamento del backup in tempo reale**: velocità, tempo stimato e numero di chunk nuovi e riutilizzati durante l'esecuzione

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Ripristino**: scegliere un server PBS e un ID Backup, elencare gli snapshot o cercarvi per nome di file

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**Pianificazioni**: più di una per set, da una ogni pochi minuti a una all'anno ("ogni mese l'ultimo giorno", "il 2° martedì"), con le prossime 3 esecuzioni

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Backup come VM Proxmox VE**: scegliere il tipo macchina virtuale e un ID numerico; l'ID Backup PBS viene mostrato mentre si digita

</td>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Set di backup come VM Proxmox VE**: un set salvato come backup `vm` porta un badge `MACHINE (VM)`

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/oneoff-as-vm.png" width="280">](docs/screenshots/oneoff-as-vm.png)
**Backup una tantum**: la stessa opzione nel modulo del backup una tantum

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**Ripristino selettivo**: selezionare singoli file o cartelle dall'albero dello snapshot, ripristinare sul posto o altrove

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Report**: cronologia completa delle esecuzioni con un pannello di dettaglio (durata, chunk, cartelle salvate)

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-message-log.png" width="280">](docs/screenshots/gui-message-log.png)
**Registro messaggi**: un flusso diagnostico di dimensione limitata, con data e ora, che copre ogni backup e ripristino

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**Multi-PBS**: gestire più server PBS, verificare la connessione, sceglierne uno predefinito

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-edit-server.png" width="280">](docs/screenshots/gui-preferences-edit-server.png)
**Aggiungere o modificare un server**: URL, utente/password o token API, datastore e namespace

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-theme-amber.png" width="280">](docs/screenshots/gui-theme-amber.png)
**Temi**: cinque tavolozze di colori integrate più un colore personalizzato, che ricolorano l'intera app in tempo reale (vedere la striscia più sotto)

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-advanced.png" width="280">](docs/screenshots/gui-preferences-advanced.png)
**Opzioni di backup**: spiegate con onestà: l'estrazione parallela nel ripristino è sperimentale e sul nostro hardware di prova è risultata *più lenta*

</td>
</tr>
</table>

**🎨 Temi di colore**: la finestra Preferenze stessa cambia colore a ogni scelta, non solo un campione:

<table>
<tr>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-amber.png" width="180">](docs/screenshots/gui-theme-amber.png)
Ambra

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-blue.png" width="180">](docs/screenshots/gui-theme-blue.png)
Blu classico

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-green.png" width="180">](docs/screenshots/gui-theme-green.png)
Verde

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-red.png" width="180">](docs/screenshots/gui-theme-red.png)
Rosso

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-dark.png" width="180">](docs/screenshots/gui-theme-dark.png)
Scuro

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-custom.png" width="180">](docs/screenshots/gui-theme-custom.png)
Personalizzato

</td>
</tr>
</table>

**🌍 Multilingue**: la pagina Report in francese, italiano e spagnolo (e in altre 15 lingue, tra cui inglese, tedesco e polacco):

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

### Esclusioni di sistema intelligenti (backup di cartelle)
I backup di cartelle escludono automaticamente:

**Cartelle di sistema:** `System Volume Information` (archivio VSS, può superare i 100 GB) e `$RECYCLE.BIN` ovunque si trovino; `Recovery` solo alla radice di un'unità; `.pbs-rollback` (la cartella di sicurezza del Rollback).
**File di sistema:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` e `DumpStack.log.tmp`.

**Perché è importante:** un'unità può mostrare 1,03 TB occupati mentre i file reali sono circa 141 GB. Senza esclusioni il backup includerebbe gli snapshot VSS (spazio e tempo sprecati); con le esclusioni la dimensione corrisponde ai dati reali.

**Consiglio:** usare un backup di **cartelle** con queste esclusioni per i backup a livello di file; usare un backup **macchina** in un set di backup separato per il ripristino bare-metal e il P2V (è un'immagine dell'intero disco; lo spazio libero e i file di paging e di ibernazione vi entrano come zeri).

### Sicurezza e qualità
- Cifratura facoltativa lato client di ogni chunk, con un manifest firmato
- Cartella delle impostazioni accessibile solo agli amministratori su Windows, privata del suo proprietario su Linux
- L'API locale del servizio è in ascolto solo su 127.0.0.1 e richiede un token, rinnovato a ogni avvio del servizio
- Validazione degli input e sanificazione delle credenziali
- Prevenzione del path traversal
- Nuovi tentativi con backoff esponenziale
- Gestione degli errori e unit test in ogni modulo, più test dal vivo facoltativi e test end-to-end su un vero PBS

## 🔐 Cifratura lato client

Gli snapshot possono essere cifrati prima di lasciare la macchina, usando un file di chiave di Proxmox Backup Server (il formato scritto da `proxmox-backup-client key create`, quindi le chiavi sono intercambiabili con il client ufficiale). Ogni chunk viene cifrato con AES-256-GCM e il manifest è firmato. Il server PBS non vede mai la chiave.

**GUI:** aprire **Preferenze > Informazioni account**, modificare un server e usare la sua scheda **Cifratura**. Selezionare un file di chiave esistente oppure **Genera...** una nuova chiave (l'opzione avanzata la protegge con una passphrase). La scheda mostra l'impronta della chiave. I report indicano se ogni backup era cifrato. **Senza il file di chiave uno snapshot non può essere ripristinato: conservatene una copia in un luogo sicuro, lontano da questa macchina.**

**Chiavi protette da passphrase (scrypt o PBKDF2):** scegliere come l'app sblocca la chiave:
- **Chiedimela**: la passphrase viene chiesta una volta per sessione dell'app e tenuta solo in memoria. I backup pianificati e quelli all'avvio non la chiedono mai: falliscono subito, con un messaggio chiaro, finché la passphrase non è stata inserita in quella sessione. Anche un backup eseguito dal servizio in background fallisce subito con una chiave "Chiedimela".
- **Ricorda su questo computer**: la passphrase viene salvata nella configurazione come il token PBS, quindi chiunque possa leggere `config.json` può sbloccare la chiave. È la scelta per i job pianificati non presidiati e per il servizio in background.

Il ripristino chiede la passphrase quando si seleziona uno snapshot cifrato.

**CLI:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` serve solo per una chiave protetta e viene chiesta se omessa. `proxmoxbackup-machine` e `proxmoxbackup-nbd` accettano le stesse due opzioni; `proxmoxbackup-nbd` ne ha bisogno per collegare un'immagine disco cifrata e controlla il primo chunk prima di toccare il dispositivo NBD, quindi una chiave sbagliata fallisce subito.

**Ripristino bare-metal:** la ISO di ripristino chiede la chiave (nessuna, un file di chiave su una chiavetta USB, o un percorso digitato) e la passphrase di una chiave protetta. La chiave non viene mai scritta nella ISO. Vedere [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md).

**Limiti noti**
- Una chiave generata dalla GUI viene scritta con un'ACL protetta (solo utente corrente, SYSTEM e Administrators). I file di chiave conservati altrove mantengono i permessi che dà loro la cartella in cui si trovano.
- `config.json` contiene in chiaro i token PBS, le password e ogni passphrase memorizzata. Dalla v0.9.0 la sua cartella è accessibile solo agli amministratori (Windows) o solo al suo proprietario (Linux); fino alla v0.8.2 era leggibile dagli utenti locali su Windows, quindi su una macchina condivisa limitatene voi l'accesso.
- Il ripristino di uno snapshot cifrato richiede lo stesso file di chiave; se va perso non c'è modo di recuperarlo.
- Il flusso "Chiedimela" e il servizio sono processi separati: usare "Ricorda" per tutto ciò che passa dal servizio.

## Backup di cartelle più veloci: rilevamento delle modifiche

Nella modalità predefinita Classico ogni backup di cartelle legge tutti i file, quindi un'esecuzione dura quanto la lettura dell'intera cartella anche quando non è cambiato nulla. I set di backup (e la CLI) possono invece usare le modalità di rilevamento delle modifiche del `proxmox-backup-client` ufficiale. La modalità Metadati è in uso quotidiano su un server da 480,9 GB e 761.866 file, dove un'esecuzione con poche modifiche richiede da 10 a 15 minuti invece di oltre 4 ore.

| Modalità | Legge | Salvato come |
|---|---|---|
| **Classico** (`legacy`, predefinita) | tutti i file | un `<name>.pxar.didx` per cartella, come prima |
| **Dati** (`data`) | tutti i file | un archivio diviso: `<name>.mpxar.didx` (metadati) + `<name>.ppxar.didx` (contenuto dei file) |
| **Metadati** (`metadata`) | solo i file nuovi e modificati | lo stesso archivio diviso; i file la cui dimensione e data di modifica (al nanosecondo), il modo e il proprietario corrispondono al backup precedente vengono ripresi da quello senza essere letti |

In modalità **Metadati** la prima esecuzione legge tutto (non c'è ancora nulla con cui confrontare); le esecuzioni successive leggono solo ciò che è cambiato. Un file modificato in un modo che ne mantiene la dimensione e la data di modifica non viene notato, ed è per questo che un set può anche **rileggere tutti i file ogni N esecuzioni** (un'esecuzione Dati): quell'esecuzione coglie tali modifiche ed elimina il riempimento che il riutilizzo lascia nell'archivio. Un file che non è stato possibile leggere completamente (riempito di zeri dopo un errore di lettura) viene sempre riletto all'esecuzione successiva. Il riutilizzo avviene solo da un backup precedente cifrato nello stesso modo, con la stessa chiave.

Poiché i dati invariati non vengono né letti né inviati, anche il traffico verso un PBS remoto o fuori sede si riduce a poco più di ciò che è effettivamente cambiato.

**GUI:** modificare un set di backup, scheda **Destinazione**, **Rilevamento modifiche**. Si applica solo ai set di cartelle: i backup macchina (disco intero) sono immagini disco e non ne sono interessati, quindi i ripristini bare-metal e Proxmox VE funzionano esattamente come prima. I backup una tantum usano Classico. Solo le esecuzioni completate con successo contano per la prossima lettura completa; l'elenco dei set di backup mostra "Prossima lettura completa tra N esecuzioni". La scheda di avanzamento indica quando e perché un'esecuzione sta leggendo tutti i file. **Report** mostra la modalità di ogni esecuzione e quanti file invariati sono stati riutilizzati.

**Un ID Backup per set.** La modalità Metadati confronta con lo snapshot più recente del gruppo di backup. Se due set (o due macchine) usano lo stesso ID Backup sullo stesso server, ogni esecuzione confronta con lo snapshot dell'altro e legge tutto. L'editor dei set di backup avvisa di questo caso.

**CLI:** `-change-detection-mode legacy|data|metadata`, oppure `"change-detection-mode"` nella configurazione JSON. Non esiste un'opzione CLI "ogni N esecuzioni": pianificare invece ogni tanto un'esecuzione con `-change-detection-mode data`.

**Prima di passare un set a Dati o Metadati,** aggiornare alla v0.6.0 o successiva ogni macchina da cui si ripristina: le build precedenti al supporto degli archivi divisi non sanno leggerli (mostrano le cartelle senza i loro file). Il `proxmox-backup-client` ufficiale li ripristina byte per byte (provato con la 3.4.9), e il file browser di PBS 4.2 li apre. Il primo backup diviso di una cartella condivide poco con i suoi vecchi backup Classico, quindi costa all'incirca una copia completa in più di quella cartella sul datastore, finché i vecchi snapshot non vengono eliminati.

## 🚀 Avvio rapido (GUI)

1. Installarlo: l'MSI su Windows, il `.deb` o l'`.rpm` su Linux, oppure estrarre lo zip o il tar.gz dalle [release](https://github.com/mjb-is/proxmoxbackupclient_go/releases) (vedere [Download e installazione](#-download-e-installazione))
2. Avviarlo. Su Windows, **Esegui come amministratore** per VSS, i backup macchina e la modifica dei set di backup. Su Linux, i backup di cartelle dei propri file funzionano con il proprio utente; usare **Proxmox Backup Client (administrator)** (o avviarlo come root) per i backup macchina e gli snapshot a blocchi
3. Aggiungere il server PBS in **Preferenze > Informazioni account** e premere **Test**
4. Creare un set di backup (o un backup una tantum) e selezionare le cartelle o i dischi di cui fare il backup
5. Eseguirlo. Perché le pianificazioni girino senza nessun utente collegato, attivare **Preferenze > Avanzate > Esegui come servizio**

Il [manuale della GUI](docs/manual/GUI.md) descrive ogni passaggio.

## Backup completo a caldo della macchina

`proxmoxbackup-machine` (e i backup macchina della GUI) eseguono il backup di un sistema Windows 10/11 o Windows Server completo, o di una macchina Linux, senza alcun fermo.

La sintassi del comando è in gran parte la stessa dei backup di cartelle, salvo che `-backupdir` è sostituito da `-backupdev` (da ripetere per più dischi).
Ad esempio, un'invocazione potrebbe essere:

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Il comando qui sopra esamina il disco 0, rileva tutte le partizioni montate, ne fa uno snapshot VSS e poi crea un'immagine di backup avviabile dell'intero disco in formato fidx. Lo spazio libero e i file di paging e di ibernazione entrano nell'immagine come zeri senza essere letti (su Linux anche lo spazio libero ext2/3/4 e XFS); `-readfreespace` invece legge tutto, per una copia forense. Un settore che non si riesce a leggere entra nell'immagine come zeri, e l'esecuzione indica dove. Aggiungere `-type vm` per salvare il backup come backup di VM Proxmox VE ([P2V](#-macchine-fisiche-in-macchine-virtuali-p2v)).

Il backup successivo sarà incrementale; il calcolo degli hash è stato parallelizzato, quindi si raggiungono facilmente velocità di 1 GB/s.

### Requisiti per il backup macchina su Linux (equivalente di VSS)

Il backup macchina su Windows usa VSS (da cui il "Esegui come amministratore" qui sopra); Linux non ha VSS, quindi questo
progetto usa il modulo kernel `elastio-snap` (un fork di `dattobd`) per lo stesso compito: uno
snapshot coerente, a un istante preciso, di un dispositivo a blocchi attivo mentre continua a essere scritto. Il repository
originale `elastio/elastio-snap` è archiviato; la versione mantenuta è
[`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap), ed è quella da installare. È supportato anche
`dattobd` stesso, se è già caricato.

Entrambe le condizioni seguenti sono necessarie, indipendentemente l'una dall'altra, altrimenti un backup macchina Linux si rifiuta di partire:

1. **Il modulo kernel deve essere installato e caricato.** L'installazione tramite pacchetto di repository documentata da `elastio-snap`
   (vedere [il suo INSTALL.md](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md)) attualmente
   punta a un URL spostato o non funzionante almeno per Ubuntu 22.04 (confermato il 2026-09-24: il pacchetto del repository
   restituisce 404 dopo un reindirizzamento). La compilazione dai sorgenti invece funziona in modo affidabile:
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
   Così il modulo viene compilato e caricato solo per l'avvio corrente; consultare la documentazione della propria distribuzione per caricarlo automaticamente
   a ogni avvio (ad esempio con una voce in `/etc/modules-load.d/`), se lo si desidera.

2. **Il processo di backup stesso deve girare come root (EUID 0).** Anche con il modulo caricato, `CreateVSSSnapshot`
   (il percorso degli snapshot Linux, nonostante il nome orientato a Windows) si rifiuta esplicitamente di girare come utente non root:
   uno snapshot coerente dell'intero disco richiede l'interfaccia di controllo del modulo, riservata a root, quindi è un requisito
   rigido, che l'appartenenza al gruppo `disk` o simili non può sostituire. Per un backup macchina, usare
   la voce di menu **Proxmox Backup Client (administrator)**, il servizio in background, oppure avviare la GUI o
   `proxmoxbackup-machine` con `sudo` (o come root); un semplice backup di file o cartelle non ha questo requisito.

Se un backup macchina fallisce con `backup device /dev/sdX: open /dev/sdX: permission denied`, di solito basta
aggiungere il proprio utente al gruppo `disk` per l'accesso in lettura di base (`sudo usermod -aG disk $USER`, poi uscire
e rientrare), ma questo da solo NON basta per un vero snapshot; ci si scontrerà comunque con il requisito di root descritto sopra
subito dopo.

### Ripristino di file da un backup di disco

La pagina Ripristino della GUI apre un backup macchina come un backup di cartelle: ogni volume dell'immagine (NTFS, FAT, exFAT, ext2/3/4 o XFS) è una cartella che si può sfogliare e da cui si può ripristinare. Non viene montato nulla.

Su Linux si può anche collegare un backup di disco come dispositivo a blocchi con lo strumento nbd.

Per usare nbd, eseguire prima `modprobe nbd max_part=0`.

Per motivi sconosciuti, usare `max_part != 0` provoca un ciclo infinito di rilevamento delle partizioni.

Lo strumento NBD collega qualsiasi backup di disco fisso, che sia un backup di VM o di host (detto questo, funziona anche con i backup PVE).

Per usarlo, utilizzare una riga di comando simile a questa:
`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Se si omette `-path`, compare un'interfaccia testuale che permette di scegliere il file fidx.

> Non usarlo su una macchina che esegue qualcosa di importante: un file system corrotto può mandare in crash il sistema operativo, ed è per questo che Proxmox VE usa un'istanza QEMU per questa operazione.

Smontare tutto ciò che si trova sul disco nbd prima di fermare `proxmoxbackup-nbd`, altrimenti è probabile che la partizione resti occupata e impossibile da smontare.

Se si ottiene un errore `Device or resource busy`, occorre forzare la disconnessione con `nbd-client -d /dev/nbd0` o semplicemente riavviare.

### Ripristino su una macchina fisica o una VM

La ISO di ripristino bare-metal ha una voce di avvio completamente automatica **"PBS Bare Metal Restore"**: vedere [ISO di ripristino bare-metal](#-iso-di-ripristino-bare-metal) più sotto. È la prima voce, quella predefinita allo scadere del timeout, e salta le richieste di lingua e tastiera di Clonezilla: si inseriscono i dati di connessione al PBS, si sceglie uno snapshot, si conferma il disco di destinazione rilevato, e la ISO si occupa della configurazione di rete, del collegamento NBD, del ripristino vero e proprio e di un riavvio automatico senza ulteriori interventi. La stessa ISO ripristina in una macchina virtuale vuota per il [P2V](#-macchine-fisiche-in-macchine-virtuali-p2v).

Sulla stessa ISO c'è ancora anche il vecchio percorso semi-manuale, come voce a sé nel normale menu di selezione della modalità di Clonezilla ("pbs-nbd"): guida nell'inserimento dei dati di connessione al PBS e collega in background lo snapshot scelto a `/dev/nbd0`, poi lascia condurre a mano il ripristino vero e proprio con la procedura guidata standard di ripristino del disco di Clonezilla.

## Uso: backup di cartelle

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

Vengono lette anche le variabili d'ambiente `PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD` e `PBS_FINGERPRINT` del client ufficiale.

Per la configurazione JSON è fornito un esempio JSON; compilare solo i campi necessari.

Nota sui modelli delle e-mail:
per l'oggetto e il corpo delle e-mail si usa il [motore di template di Go](https://pkg.go.dev/text/template); per la sintassi fare riferimento alla sua documentazione.
Nei modelli sono disponibili le seguenti variabili:

- `.NewChunks`: numero di nuovi chunk creati
- `.ReusedChunks`: numero di chunk riutilizzati
- `.Datastore`: nome del datastore
- `.Error`: l'errore, se presente (`.ErrorStr` lo fornisce come testo, vuoto se non ce n'è)
- `.ReadErrors`: i file o le cartelle che non è stato possibile leggere; `.ReadErrorCount` indica quanti sono
- `.Hostname`: nome host della macchina
- `.StartTime`: ora di inizio del backup
- `.EndTime`: ora di fine del backup
- `.Duration`: durata del backup
- `.FromattedDuration`: durata formattata del backup (nel codice il nome è scritto proprio così)
- `.Success`: vero quando il backup è terminato senza errori e senza nulla di illeggibile
- `.Partial`: vero quando è stato salvato uno snapshot ma alcuni file non è stato possibile leggerli
- `.Status`: `Success`, `Partial` o `Failed`

## Backup di stream

Permette di fare il backup di uno stream invece che di un pxar, con possibilità infinite. Ad esempio si può eseguire:

```
mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]
```

Questo permette di sfruttare buzhash per la deduplicazione anche usando ad esempio tar, o il dump SQL stesso.

## Problemi noti

Con l'antimalware di Windows Defender attivo, il backup rallenta fino al 25% della velocità raggiungibile.

Su Windows gli strumenti a riga di comando acquisiscono un lock di processo: un secondo backup avviato mentre un altro è in corso stampa "Another backup job is running" ed esce. Su Linux non esiste un lock del genere, quindi non pianificate due esecuzioni CLI nello stesso momento.

## 📋 Requisiti

- Windows 10/11 o Windows Server (64 bit), con diritti di amministratore per VSS e i backup macchina
- oppure Linux (i pacchetti sono per Debian, Ubuntu, Mint e Fedora), con il runtime WebKitGTK per la GUI, e root più il modulo kernel elastio-snap o dattobd per backup macchina coerenti
- Accesso di rete a un Proxmox Backup Server
- Per il P2V: Proxmox VE con il datastore PBS aggiunto come storage, oppure qualsiasi hypervisor in grado di avviare la ISO di ripristino

## 🔨 Compilare dai sorgenti

**[BUILD.md](BUILD.md)** contiene le istruzioni complete: la GUI e il servizio in background, gli strumenti a riga di comando, l'MSI e i pacchetti Linux. In breve, con Go 1.25 o successivo e la Wails CLI v2.13:

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

Il frontend compilato (`gui/frontend/dist`) è nel repository, quindi Node.js serve solo per modificare il frontend (`npm install` e `npm run build` in `gui/frontend`, oppure `wails dev` per il ricaricamento a caldo). Test: `make test`.

## 🔧 Uso avanzato e guide

### Multi-PBS (più server PBS)

Configurare più server PBS e scegliere la destinazione per ogni set di backup o backup una tantum (ad esempio `C:\Users` su un PBS veloce su SSD ogni giorno, `D:\` su un PBS di grande capacità ogni settimana, più un server di disaster recovery).

- **[Guida utente](MULTI_PBS_USER_GUIDE.md)**: aggiungere e verificare i server, il server predefinito, scegliere un server per ogni backup, FAQ e risoluzione dei problemi.
- **[Guida all'implementazione](MULTI_PBS_GUIDE.md)**: modello dei dati, migrazione automatica da una configurazione a un solo PBS, metodi del backend.

Una vecchia configurazione a un solo PBS viene migrata automaticamente a un server `default` al primo caricamento.

### 💿 ISO di ripristino bare-metal

La ISO di ripristino è una ISO Clonezilla Live standard modificata con i binari `pbsnbd` / `machinebackup` più due voci di menu. Si avvia da CD, da chiavetta USB (scritta con `dd`, o da una chiavetta Ventoy), via PXE, e su macchine UEFI o BIOS:

- **`ocs-pbs-bare-metal-restore`**: la prima voce del menu di avvio, quella predefinita allo scadere del timeout, **"Proxmox Backup Client Go - PBS Bare Metal Restore"**. Salta le richieste di lingua e tastiera di Clonezilla e la sua procedura guidata generica, e passa direttamente dalle credenziali PBS a un ripristino completato con riavvio automatico: prima DHCP (una richiesta di IP statico solo se fallisce), sincronizzazione NTP, una chiave di cifratura da una chiavetta USB se il backup è cifrato, e un rilevamento del disco di destinazione che non propone mai la chiavetta da cui è stata avviata. Se il ripristino riesce, mostra un messaggio di completamento e riavvia. Ripristina allo stesso modo backup Windows e Linux (il ripristino avviene a livello di blocchi), su una macchina fisica o in una VM vuota.
- **`ocs-pbs-nbd`**: una voce **"pbs-nbd"** nel menu di selezione della modalità di Clonezilla. Chiede i dati di connessione al PBS e lo snapshot da ripristinare, lo collega in background a `/dev/nbd0`, poi passa alla normale procedura guidata di ripristino del disco di Clonezilla, da condurre a mano.

Corretti anche sul lato del ripristino: un deadlock di `zstd.NewReader` nel percorso di recupero dei chunk PBS che bloccava i ripristini di macchine intere (contribuito al progetto originale come [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85)), un bug per cui "il primo passaggio di ripristino fallisce con 'no partition', ripetendolo riesce" causato da una corsa tra la funzione `is_disk_without_part_and_fs()` di Clonezilla e udev, e le macchine BIOS/MBR che non si avviavano dopo un ripristino (ora vengono ripristinati anche i dati tra l'MBR e la prima partizione).

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

Tutti i dettagli (perché una ricostruzione completa della ISO invece di una sostituzione sul posto, requisiti, flusso del menu, verifica) sono in **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**; l'uso è descritto nel **[manuale del ripristino bare-metal](docs/manual/BMR.md)**.

### Compilare la GUI per Windows

**Windows nativo**: vedere **[BUILD.md](BUILD.md)** per la toolchain (Go, MinGW, Wails) e i comandi per l'app, il servizio e l'MSI.

**Docker (per compilare su Linux).** `./build_gui_windows_docker.sh` compila l'app Windows come `ProxmoxBackupClientGO.exe`, con un supporto corretto di WebView2, in un container `golang` usa e getta (installa mingw e Wails, compila il frontend, esegue `wails build`). Non compila il servizio in background né l'MSI.

### Stato delle funzionalità, changelog e documentazione interna

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**: cosa è fatto, parziale o non fatto, e da quale versione.
- **[CHANGELOG.md](CHANGELOG.md)**: cronologia delle modifiche per versione, e [Unreleased] per ciò che è su master.
- **[TODO.md](TODO.md)**: roadmap aperta e idee.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**: cosa contiene ogni release.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**: la finestra di disinstallazione dell'MSI (mantenere o eliminare la configurazione) e il suo piano di test.
- **[AGENTS.md](AGENTS.md)**: struttura e convenzioni del repository, per i contributori e gli agenti di programmazione.

## 🔁 Contributi al progetto originale

Le correzioni e le funzionalità di utilità generale vengono proposte a [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) come pull request. Diciotto sono state integrate:

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

## 🧪 Stato, segnalazioni di bug e richieste di funzionalità

**Questo è software alpha, fornito così com'è, senza alcuna garanzia di alcun tipo, e usato interamente a proprio rischio.** Non mi assumo alcuna responsabilità per dati persi o danneggiati, compresi i file di origine.

È costruito per il mio homelab. Collaudo personalmente ogni release prima di pubblicarla, e lì è in uso regolare: backup pianificati di cartelle e di macchine Windows e Linux, con ripristini, Recuperi, Rollback, ripristini bare-metal e ripristini P2V collaudati strada facendo. Nessun backup è affidabile finché non ci si è ripristinato sopra, quindi provate i vostri ripristini, compreso un ripristino bare-metal, prima di farci affidamento.

- **Le segnalazioni di bug sono benvenute**: aprite una [issue](https://github.com/mjb-is/proxmoxbackupclient_go/issues) indicando cosa avete fatto, cosa è successo e il log (il Registro messaggi, o i file nella cartella delle impostazioni).
- **Richieste di funzionalità**: aggiungo le funzionalità utili al mio uso, quindi non accetto richieste per esigenze altrui. Inviatele al progetto originale, [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go/issues). Sono benvenute le pull request che correggono bug, o che sarebbero chiaramente utili anche al mio uso.

## 🙏 Riconoscimenti e storia

Il client a riga di comando originale è stato scritto da Tiziano Bacocco (tizbac) a partire dal 2023. In seguito vi è stato integrato il client grafico di RDEM Systems, e con esso la prima GUI, il supporto multi-PBS e il servizio Windows. Questo fork è nato a settembre 2026 per colmare le lacune che impedivano a PBS di essere il mio unico sistema di backup; le sue release partono dalla v0.3.0, e le correzioni di utilità generale tornano al progetto originale.

- Il client originale, [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go), di Tiziano Bacocco (tizbac) e dei suoi contributori.
- Il flusso di lavoro e lo stile di Backup for Workgroups (BFW), a cui si ispirano i set di backup, il Recupero e il Rollback.
- [Clonezilla Live](https://clonezilla.org/), su cui è basata la ISO di ripristino bare-metal.

## 📄 Licenza

GPLv3: vedere il file [LICENSE](LICENSE).

---

**Modifiche dal fork © 2026 mjb-is, a partire dal lavoro di Tiziano Bacocco (tizbac), dei contributori di Proxmox Backup Client GO e di RDEM Systems (© 2024-2026).**
