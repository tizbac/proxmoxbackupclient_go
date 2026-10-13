# Proxmox Backup Client: client Windows e Linux per Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client è un client di backup open source (GPL-3.0) per Proxmox Backup Server (PBS), per Windows e Linux.** È una **suite di strumenti**:

- **Proxmox Backup Client GUI** (basata sulla GUI Nimbus Backup di RDEM Systems): client grafico per Windows e Linux, con snapshot coerenti (VSS su Windows, elastio-snap/dattobd su Linux), set di backup pianificati, backup di cartelle e di dischi interi, ripristino, Recupero (Undelete) e Rollback, più server PBS e un servizio in background.
- **`proxmoxbackup-directory`**: backup di cartelle (pxar) da riga di comando, con deduplicazione e rilevamento delle modifiche.
- **`proxmoxbackup-machine`**: backup a caldo di dischi interi (fidx) di macchine Windows (VSS) e Linux (elastio-snap/dattobd); l'immagine è avviabile dopo un ripristino bare-metal.
- **`proxmoxbackup-nbd`**: collega un backup di disco come dispositivo NBD per il ripristino (Linux).
- Una **ISO di ripristino bare-metal** basata su Clonezilla Live.

> ⚠️ **Avvertenza:** questo progetto **non è in alcun modo affiliato** a **Proxmox Server Solutions GmbH**. "Proxmox", il logo Proxmox e i nomi correlati appartengono ai rispettivi proprietari; qui servono **solo** a indicare la compatibilità. Vedere [proxmox.com](https://www.proxmox.com/).

> ℹ️ **Questa pagina è un riassunto tradotto.** Il riferimento è il [README.md inglese](README.md), che contiene tutti i dettagli. I manuali sono in inglese.

## 📖 Manuali utente

| Manuale | Contenuto |
|---|---|
| **[Manuale della GUI](docs/manual/GUI.md)** | Installazione, server PBS, set di backup, backup incrementali, ripristino, Recupero, Rollback, Report, Preferenze, servizio, risoluzione dei problemi |
| **[Manuale della riga di comando](docs/manual/CLI.md)** | I tre strumenti: opzioni, file di configurazione, codici di uscita, pianificazione; Recupero e Rollback da riga di comando |
| **[Manuale del ripristino bare-metal](docs/manual/BMR.md)** | La ISO di ripristino passo per passo; snapshot `vm` su Proxmox VE; hardware diverso |

Vedere anche gli [scenari di ripristino](docs/RESTORE_GUIDE.md) e l'[indice dei manuali](docs/manual/README.md).

## 🆕 Novità della v0.9.0: più pianificazioni per set di backup, installer, un servizio Linux

- **Più pianificazioni per set di backup**, da una ogni pochi minuti a una all'anno ("ogni giorno alle 02:00" più "l'ultimo giorno del mese" o "il 2° martedì"), ciascuna descritta a parole con le sue prossime 3 esecuzioni. I set esistenti mantengono la loro pianificazione.
- **I backup di cartelle conservano gli stream di dati alternativi e gli attributi estesi NTFS** (Windows), come il contrassegno "scaricato da Internet", e il ripristino li rimette.
- **Installer**: **MSI per Windows** (app e servizio in Program Files, menu Start, aggiornamento sul posto, impostazioni mantenute o eliminate alla rimozione) e **pacchetti Linux** `.deb` (Debian, Ubuntu, Mint) e `.rpm` (Fedora).
- **Servizio in background su Linux** (`proxmoxbackupclient.service`): i set pianificati girano come root anche senza utenti collegati (Preferenze > Avanzate > Esegui come servizio). **Riavvia come amministratore** funziona anche su Linux (tramite pkexec).
- **Pausa, Riprendi e Ferma** per i backup eseguiti dal servizio.
- Avviso e badge **ID condiviso** quando due set condividono un gruppo di backup (stesso server, tipo e ID Backup).
- **Impostazioni protette**: `C:\ProgramData\ProxmoxBackupClient` solo per gli amministratori su Windows; cartella privata del proprietario su Linux.
- **File ripristinabili dai backup macchina** anche da FAT, exFAT, ext2/3/4 e XFS oltre a NTFS; le immagini Windows rimettono anche permessi e stream di dati alternativi.
- I backup macchina Linux saltano lo spazio libero (ext2/3/4, XFS).
- Recupero e Rollback per i set macchina Linux, e **da riga di comando**: `-list-sets`, `-undelete`, `-rollback`.
- Gli strumenti a riga di comando scrivono gli errori su stderr, e `proxmoxbackup-machine` invia la sua e-mail di notifica.
- La **ISO di ripristino bare-metal** non propone mai come disco di destinazione la chiavetta da cui è partita, e il manuale spiega come far corrispondere il firmware (UEFI o BIOS) quando una macchina fisica diventa una VM.
- **Tutte le lingue riviste** (17 traduzioni, circa 1.850 stringhe); date e numeri seguono la lingua dell'app.
- È inclusa anche l'accelerazione della v0.8.2 sullo spazio vuoto (taggata, mai pubblicata da sola).

L'elenco completo è nel [CHANGELOG](CHANGELOG.md).

## 💽 Novità della v0.8.0

- **I backup macchina Windows saltano lo spazio libero** (e i file di paging e ibernazione): un disco mezzo vuoto si legge in circa metà del tempo. Un **settore danneggiato** non ferma più il backup (viene scritto come zeri e segnalato).
- **Ripristino di file da un backup macchina**: ogni volume NTFS dell'immagine è una cartella da sfogliare, cercare e selezionare, senza montare né scaricare nulla (127 GB e 430.000 file elencati in 4-15 secondi). Un solo backup macchina copre così sia il ripristino bare-metal sia il file perso.
- **Recupero e Rollback** sui set macchina (Windows), per le cartelle scelte (di default Desktop, Documents, Pictures, Music, Videos e Downloads).
- **Pausa e Riprendi**, badge **VSS / Snapshot ON-OFF**, date di creazione Windows salvate e ripristinate.
- **v0.8.2:** lo spazio vuoto non viene più sottoposto a hash blocco per blocco (disco di prova da 50 GB: da 3m36s a 1m15s).

## 🗑️ Novità della v0.7.0: Recupero e Rollback

- **Recupero** trova i file presenti nei backup di un set ma non più sul disco (ultimo backup, oppure ultimi 7 o 30 giorni), li mostra in un albero con il momento della scomparsa e ripristina quelli selezionati. Dopo aver cancellato 39 GB da una condivisione di 761.000 file, la ricerca su 7 giorni li ha ritrovati in circa un minuto.
- **Rollback** riporta le cartelle di un set allo stato di un backup scelto, dopo un'anteprima. Ogni file sostituito o rimosso viene prima spostato da parte sullo stesso disco, quindi **Annulla** rimette tutto com'era.

Inoltre: finestre di conferma, prossima esecuzione su ogni scheda, ID Backup suggerito, servizio incluso nel download per Windows. Vedere il [manuale della GUI](docs/manual/GUI.md#10-undelete).

## ⚡ Novità della v0.6.0: backup incrementali di cartelle

Con lo stesso **rilevamento delle modifiche** del `proxmox-backup-client` ufficiale, la modalità **Metadati** confronta dimensione, data di modifica (al nanosecondo), modo e proprietario di ogni file con lo snapshot precedente; i file invariati non vengono aperti e i loro chunk vengono riutilizzati. Su un server da 480,9 GB e 761.866 file la prima esecuzione richiede 4 h 17 min, le successive circa 10-15 minuti (da 4 a 40 chunk nuovi). Il traffico verso un PBS remoto si riduce di conseguenza. Vedere il [manuale della GUI](docs/manual/GUI.md#7-incremental-folder-backups-change-detection).

## 🤝 Compatibile con il client Proxmox ufficiale

Verificato con `proxmox-backup-client` 3.4.9 e PBS 4.2, in entrambe le direzioni:

- Gli snapshot Legacy, Data e Metadata di questo client (Windows o Linux) si ripristinano **byte per byte** con il client ufficiale, e il file browser di PBS 4.2 li apre; quelli del client ufficiale si ripristinano byte per byte con questo client.
- Stesso formato di archivio diviso (pxar versione 2), stesse modalità di rilevamento, stesso formato del file di chiave (`proxmox-backup-client key create`), stesse variabili d'ambiente `PBS_REPOSITORY`.

I vostri backup non sono quindi mai legati a questo client. Unica aggiunta: un piccolo file `.proxmox_backup_client_meta.json` alla radice degli snapshot di cartelle, che il client ufficiale ripristina come un normale file.

## 📦 Download e installazione

👉 **[Scarica l'ultima release](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| Se hai | Usa |
|---|---|
| Windows | L'**installer MSI** (`ProxmoxBackupClient-<version>-x64.msi`), oppure lo zip `ProxmoxBackupClient-v<version>-windows-amd64.zip` (app e `ProxmoxBackupClientSVC.exe` nella stessa cartella) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Altro Linux | `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (richiede il runtime WebKitGTK) |
| Solo riga di comando | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` oppure `-linux-amd64.tar.gz` |

MSI, `.deb` e `.rpm` sono novità di master (fino alla v0.8.1 solo zip e tar.gz); nel frattempo vedere [BUILD.md](BUILD.md). Su Linux la voce di menu **Proxmox Backup Client (administrator)** avvia l'app come root tramite polkit, per i backup macchina; il servizio è installato ma non attivato ([packaging/debian/README.md](packaging/debian/README.md)).

**Requisiti:** Windows 10/11 o Windows Server a 64 bit (diritti di amministratore per VSS e backup macchina), oppure Linux con WebKitGTK (e root più elastio-snap o dattobd per i backup macchina); accesso di rete a un Proxmox Backup Server.

> ⚠️ **Windows segnala un virus (`Trojan:Win32/Sabsik.FL.A!ml`) o SmartScreen avvisa?** È un **falso positivo noto** delle applicazioni Go/Wails non firmate. Dettagli nel [progetto originale](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Verificare un download

Ogni release di questo fork include un `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-v0.8.1-windows-amd64.zip -Algorithm SHA256   # compare with SHA256SUMS.txt
```

Su Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`. Non ci sono ancora firma del codice né attestazione da CI (vedere [TODO.md](TODO.md)).

## ✨ Funzionalità

### GUI (consigliata)
- **18 lingue dell'interfaccia**, tra cui italiano, francese e tedesco
- Menu laterale: Backup, Ripristino, Recupero, Rollback, Report, Registro messaggi, Preferenze, Informazioni
- **Set di backup** pianificati (giornaliero, a intervalli, manuale) con "Esegui ora", badge del tipo e VSS / Snapshot; **backup una tantum**
- **Backup incrementali di cartelle** (Metadati) con rilettura completa ogni N esecuzioni
- **Backup macchina** Windows e Linux, come `host` o come snapshot `vm` di Proxmox VE
- **Ripristino** selettivo con ricerca, ACL NTFS/POSIX, date e verifica facoltativa; file dai backup macchina
- **Pausa, Riprendi, Ferma**; **Esegui come servizio** (Preferenze > Avanzate)
- Report, notifiche e-mail, azioni dopo il backup, esportazione e importazione delle impostazioni, cinque temi di colore
- Più server PBS, pinning del certificato, token API o utente/password; **cifratura lato client**

### Strumenti a riga di comando
- `proxmoxbackup-directory`: cartelle (pxar), `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: dischi interi (fidx, incrementale, spazio libero saltato salvo `-readfreespace`)
- `proxmoxbackup-nbd`: server NBD per ripristinare backup di disco (Linux)
- `ProxmoxBackupClientSVC` (Windows) e `proxmoxbackupclient` (Linux): `-list-sets`, `-undelete`, `-rollback` (su master)

### 📸 Screenshot

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Set di backup**: badge ed "Esegui ora"

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Avanzamento in tempo reale**: velocità, tempo stimato, chunk

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Ripristino**: elencare gli snapshot o cercarvi un file

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Set come VM Proxmox VE**: badge `MACHINE (VM)`

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Report**: cronologia completa con dettagli

</td>
<td width="33%">

[<img src="docs/screenshots/gui-lang-italian.png" width="280">](docs/screenshots/gui-lang-italian.png)
**In italiano**: la pagina Report

</td>
</tr>
</table>

Tutti gli screenshot sono nel [README inglese](README.md).

### Esclusioni di sistema intelligenti
I backup di cartelle saltano `System Volume Information`, `$RECYCLE.BIN`, `Recovery` (alla radice di un'unità), `.pbs-rollback`, `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` e `DumpStack.log.tmp`. Un'unità che mostra 1,03 TB occupati può contenere solo 141 GB di file reali. Consiglio: backup di **cartelle** per i file, backup **macchina** in un set separato per il ripristino bare-metal.

### Sicurezza e qualità
Cifratura facoltativa con manifest firmato; API locale del servizio solo su 127.0.0.1, con token; validazione degli input, prevenzione del path traversal, nuovi tentativi con backoff esponenziale; unit test e test end-to-end.

## 🔐 Cifratura lato client

Ogni chunk viene cifrato con AES-256-GCM prima di lasciare la macchina, con un file di chiave PBS (intercambiabile con il client ufficiale); il manifest è firmato e il server non vede mai la chiave.

**GUI:** **Preferenze > Informazioni account**, modificare un server, scheda **Cifratura**: scegliere una chiave oppure **Genera...**. Per una chiave protetta: **Chiedimela** (una volta per sessione) oppure **Ricorda su questo computer** (necessario per il servizio e i job non presidiati). **Senza il file di chiave nessun ripristino è possibile: conservatene una copia fuori dalla macchina.**

**CLI:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` serve solo per una chiave protetta (viene chiesto se manca); `proxmoxbackup-machine` e `proxmoxbackup-nbd` accettano le stesse opzioni. La ISO Clonezilla chiede la chiave e la passphrase, senza mai scrivere la chiave nella ISO.

**Limiti:** `config.json` contiene in chiaro token, password e passphrase memorizzate (su master la sua cartella è protetta; fino alla v0.8.2 era leggibile dagli utenti locali su Windows). Una chiave persa non si recupera.

## Backup di cartelle più veloci: rilevamento delle modifiche

| Modalità | Legge | Salvato come |
|---|---|---|
| **Classico** (`legacy`, predefinita) | tutti i file | un `<name>.pxar.didx` per cartella |
| **Dati** (`data`) | tutti i file | archivio diviso: `<name>.mpxar.didx` + `<name>.ppxar.didx` |
| **Metadati** (`metadata`) | solo i file nuovi e modificati | lo stesso archivio diviso; i file invariati vengono ripresi dal backup precedente senza essere letti |

**GUI:** modificare un set, scheda **Destinazione**, **Rilevamento modifiche** (solo set di cartelle; i backup macchina non cambiano). Impostare **Rileggi tutti i file ogni N esecuzioni** per cogliere le rare modifiche che lasciano invariate dimensione e data. Dare a ogni set **il proprio ID Backup**: due set con lo stesso ID si confrontano l'uno con l'altro e rileggono tutto.

**CLI:** `-change-detection-mode legacy|data|metadata`; non esiste un'opzione "ogni N esecuzioni", pianificare ogni tanto un `-change-detection-mode data`. Prima di passare a Dati o Metadati, aggiornare alla v0.6.0 o successiva ogni macchina che ripristina: le versioni precedenti non leggono gli archivi divisi.

## 🚀 Avvio rapido (GUI)

1. Installare l'MSI, il `.deb` o l'`.rpm` (oppure estrarre lo zip / tar.gz)
2. Su Windows, **Esegui come amministratore** per VSS e backup macchina; su Linux, **Proxmox Backup Client (administrator)** per i backup macchina
3. Aggiungere il server PBS in **Preferenze > Informazioni account** e premere **Test**
4. Creare un set di backup (o un backup una tantum) e scegliere cartelle o dischi
5. Eseguirlo; perché le pianificazioni girino senza utenti collegati, attivare **Preferenze > Avanzate > Esegui come servizio**

Il [manuale della GUI](docs/manual/GUI.md) descrive ogni passaggio.

## Backup completo a caldo della macchina

Stessa sintassi dei backup di cartelle, con `-backupdev` al posto di `-backupdir`:

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Il comando fa uno snapshot VSS delle partizioni del disco 0 e crea un'immagine avviabile dell'intero disco (fidx). Lo spazio libero entra come zeri senza essere letto (`-readfreespace` per leggere tutto). I backup successivi sono incrementali.

### Requisiti per il backup macchina su Linux

Linux non ha VSS: il progetto usa il modulo kernel `elastio-snap` (un fork di `dattobd`); installare la versione mantenuta [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap) (va bene anche `dattobd` già caricato). Servono due condizioni:

1. **Il modulo kernel deve essere installato e caricato.** Il pacchetto di repository documentato punta a un URL non valido (almeno per Ubuntu 22.04); la compilazione dai sorgenti funziona:
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
   Il modulo resta caricato solo fino al riavvio (vedere `/etc/modules-load.d/` per caricarlo a ogni avvio).

2. **Il backup deve girare come root (EUID 0).** Il gruppo `disk` non basta: usare la voce **Proxmox Backup Client (administrator)**, il servizio in background oppure `sudo`. Un backup di cartelle non ha questo requisito.

### Ripristino di file da un backup di disco

La pagina Ripristino apre un backup macchina come una cartella, senza montare nulla. Su Linux lo si può anche collegare via NBD (dopo `modprobe nbd max_part=0`):

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Senza `-path` compare un'interfaccia testuale per scegliere. Da evitare su macchine importanti; smontare tutto prima di fermarlo, altrimenti `nbd-client -d /dev/nbd0`.

### Ripristino su una macchina fisica

La ISO Clonezilla modificata offre una voce completamente automatica **"PBS Bare Metal Restore"**: dati PBS, scelta dello snapshot, conferma del disco, poi rete, ripristino e riavvio senza altri interventi. Resta disponibile anche la voce semi-manuale "pbs-nbd". Vedere il [manuale BMR](docs/manual/BMR.md).

## Uso: backup di cartelle

Riferimento completo nel **[manuale della riga di comando](docs/manual/CLI.md)**. Un comando tipico:

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

Vengono lette anche le variabili d'ambiente `PBS_REPOSITORY` e simili del client ufficiale.

## 🔨 Compilare dai sorgenti

Tutto è in **[BUILD.md](BUILD.md)** (GUI, servizio, strumenti CLI, MSI, pacchetti Linux). In breve, con Go 1.25 o successivo e Wails CLI v2.13:

```bash
cd gui
wails build                                    # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe   # the background service
```

Il frontend compilato è nel repository: Node.js serve solo per modificarlo. Test: `make test`.

## 🔧 Uso avanzato e guide

- **Multi-PBS** (più server, destinazione scelta per set): [guida utente](MULTI_PBS_USER_GUIDE.md) e [guida all'implementazione](MULTI_PBS_GUIDE.md).
- **ISO Clonezilla per il ripristino bare-metal**: si ottiene modificando una ISO Clonezilla Live, dettagli in [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md):

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

- [FEATURES_STATUS.md](FEATURES_STATUS.md): cosa è fatto o no, e da quale versione; [CHANGELOG.md](CHANGELOG.md): cronologia per versione; [TODO.md](TODO.md): roadmap; [RELEASE_NOTES.md](RELEASE_NOTES.md); [AGENTS.md](AGENTS.md): struttura del repository.

## 🌐 Supporto tecnico in Italia

Il progetto e **tutti** i suoi contributi resteranno per sempre pubblici e in GPLv3. E.T.I. Srl ( https://etitech.net ), sponsor principale del progetto e delle ultime funzioni di backup macchina, offre a chi ne ha bisogno supporto per i deployment Proxmox e per gli strumenti di backup Windows. Non ci sarà mai un'edizione "community" separata da una "enterprise": il supporto è un servizio indipendente, e ogni personalizzazione richiesta, anche se a pagamento, sarà rilasciata in GPLv3.

## 🖥️ Attribuzione della GUI

La GUI è basata sulla **[GUI Nimbus Backup](https://nimbus.rdem-systems.com)** di **[RDEM Systems](https://www.rdem-systems.com/)**, che ne sponsorizza lo sviluppo e fornisce il supporto commerciale. Tutto il codice resta in GPLv3.

## ⚠️ Avvertenza

Software fornito "così com'è", ancora di qualità alpha; decliniamo ogni responsabilità per la perdita di dati. Testare backup e ripristini prima dell'uso in produzione.

## 📄 Licenza

GPLv3: vedere [LICENSE](LICENSE).

## 🏷️ Branding

Ogni contributore con almeno **5 commit** di funzionalità o correzioni può far aggiungere il proprio branding per uso commerciale, a condizione che l'azienda non sia coinvolta in campagne di malware, promozione della guerra (in qualsiasi paese), truffe, furto di dati, traffico di minori, violenza, discriminazione, droga o altre attività illegali. In caso di reclamo senza una spiegazione valida, il beneficio viene revocato immediatamente. La licenza GPLv3 resta in vigore: si è sempre liberi di fare un fork del progetto.

## 🤝 Contribuire

Contributi benvenuti, in particolare:

1. Cifratura: revisione da madrelingua dei testi tradotti automaticamente, gestione delle chiavi oltre il file (ad esempio DPAPI di Windows)
2. Migrazione da fisico a virtuale (P2V): le basi funzionano con il tipo `vm`; mancano scheda di rete, TPM, driver virtio, CPU/RAM reali, bus diversi da SATA
3. Upload asincrono / multicore dei chunk
4. Descrittori di sicurezza Windows nel formato pxar (patch lato Proxmox)
5. Link simbolici Windows

## Informazioni

I contributori di Proxmox Backup Client GO sviluppano e mantengono questo progetto, che si basa sull'infrastruttura NTP/NTS e sugli [11 server NTS pubblici](https://github.com/jauderho/nts-servers).

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
