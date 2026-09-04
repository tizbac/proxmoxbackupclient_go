# Proxmox Backup Client — Client Windows per Proxmox Backup Server

[🇬🇧 English](README.md) | [🇫🇷 Français](README.fr.md) | [🇮🇹 Italiano](README.it.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/tizbac/proxmoxbackupclient_go)](https://github.com/tizbac/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-github-orange)](https://github.com/tizbac/proxmoxbackupclient_go)

**Proxmox Backup Client è un client di backup open-source (GPL-3.0) per Proxmox Backup Server (PBS), funzionante su Windows e Linux.**

Una GUI moderna (**Proxmox Backup Client GUI**, basata sulla GUI Nimbus Backup di RDEM Systems) per eseguire il backup di server e workstation Windows verso PBS — snapshot coerenti via VSS, job pianificati, modalità file e disco, navigazione/ripristino degli snapshot, supporto multi-PBS e modalità servizio Windows — oltre a una suite CLI completa per backup di directory, stream e macchina intera.

> Parole chiave: client proxmox backup windows · client PBS · backup Windows VSS · backup remoto immutabile · interfaccia Proxmox Backup Server.

## 📦 Download

👉 **[Scarica l'ultima release](https://github.com/tizbac/proxmoxbackupclient_go/releases)**

> ⚠️ **Windows segnala « virus rilevato » (es. `Trojan:Win32/Sabsik.FL.A!ml`) o un avviso SmartScreen?**
> Si tratta di un **falso positivo** noto per le applicazioni Go/Wails — *non* è un virus. Il suffisso `!ml` indica una rilevazione da parte di un modello di machine learning che segnala eseguibili *non firmati e poco diffusi*.
> Scopri [perché accade e come verificare il download](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Verifica di qualsiasi download

Ogni release fornisce checksum SHA-256 e una **attestazione di provenienza firmata** (prova crittografica che il binario è stato prodotto dalla CI di questo repository, a partire da un commit preciso):

```powershell
Get-FileHash .\ProxmoxBackupClient.exe -Algorithm SHA256   # confrontare con SHA256SUMS.txt
gh attestation verify .\ProxmoxBackupClient.exe --repo tizbac/proxmoxbackupclient_go
```

**VirusTotal — 0 rilevamenti.** Report multi-motore indipendenti dei recenti installer MSI:
[0.2.108](https://www.virustotal.com/gui/file/6e8fb7ce9af740d470e947addb8daba4331c0b88e8bfdec9e0697ea8f7f29e9e/detection) ·
[0.2.107](https://www.virustotal.com/gui/file/6fd6c6fa77e0305c129ef882a3745100aa6033187a6d52a4af94149ab6b666d2/detection) ·
[0.2.106](https://www.virustotal.com/gui/file/ad6e56700ed9df8e088906e38cee2e2882fc7045f4e39269de0e379a01784ad7/detection)

> ℹ️ **Firma del codice:** i binari Windows **non sono ancora firmati Authenticode** (un certificato OSS tramite [SignPath Foundation](https://signpath.org) è in attesa). Nel frattempo la provenienza è stabilita tramite l'attestazione e i checksum sopra.

## ✨ Funzionalità

### GUI — Proxmox Backup Client GUI (consigliata)
- **🌍 Multilingue** — interfacce in francese, inglese, italiano, tedesco e polacco
- Configurazione intuitiva con test di connessione
- Avanzamento del backup in tempo reale con velocità e tempo rimanente
- Supporto VSS (Volume Shadow Copy) per backup coerenti
- Backup multi-cartella, modalità file e disco
- Navigazione negli snapshot, ricerca file (jolly) e ripristino
- Supporto multi-servers PBS con pinning dell'impronta del certificato (TOFU)
- Modalità servizio Windows + backup pianificati
- Annullamento del backup, cronologia (ultime 6) e riesecuzione
- Registrazione di debug per la diagnostica

### Strumenti CLI
- `proxmoxbackup-directory` — backup di directory (PXAR) con deduplicazione
- `proxmoxbackup-machine` — backup live completo del sistema Windows (FIDX, VSS, incrementale)
- `proxmoxbackup-nbd` — server NBD per ripristinare backup disco (Linux)

### 📸 Schermate

![Configurazione dei server](docs/screenshots/nimbus-gui-liste-servers.png)
*Gestione multi-server PBS con indicatori di stato*

![Modulo di aggiunta server](docs/screenshots/nimbus-gui-add-server-form.png)
*Configurazione semplice del server con test di connessione*

![Backup immediato](docs/screenshots/nimbus-gui-one-shot-backup.png)
*Avanzamento del backup in tempo reale con ETA e velocità*

### Esclusioni di sistema intelligenti (modalità file)
Quando si esegue il backup di un intero drive (es. `D:\`), la GUI esclude automaticamente:

**Cartelle di sistema:** `System Volume Information` (archivio VSS, può raggiungere 100+ GB), `$RECYCLE.BIN`, `Recovery`.
**File di sistema:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys`.

**Perché è importante:** un disco può mostrare 1,03 TB utilizzati mentre i file reali sono ~141 GB. Senza esclusioni il backup includerebbe gli snapshot VSS (spazio e tempo sprecati); con esse, la dimensione corrisponde ai dati reali.

**Raccomandazione:** usa la **modalità file** (predefinita) con auto-esclusioni per i backup a livello di file; usa la **modalità disco** in un job separato per il ripristino bare-metal (include tutto).

### Sicurezza e qualità
- Validazione degli input e pulizia delle credenziali
- Prevenzione delle path traversal
- Logica di retry con backoff esponenziale
- Gestione completa degli errori e test, conformità lint al 100%

## 🚀 Avvio rapido (GUI)

1. Scarica `ProxmoxBackupClient.exe` (o il `.msi`) dalle release
2. Avvialo con diritti di amministratore (necessari per VSS)
3. Configura la connessione PBS e testala
4. Seleziona le cartelle da salvare
5. Avvia il backup

## NUOVO! — Backup live completo della macchina

È stata aggiunta una nuova funzionalità che consente di eseguire il backup di un intero sistema Windows 10/11 e delle rispettive versioni server senza alcuna interruzione.

La sintassi del comando è quasi la stessa, tranne che per `-backupdir string`.

Nel caso dell'eseguibile di machine backup al posto di backupdir c'è `-backupdev`.
Ad esempio un'invocazione potrebbe essere:

`machinebackup.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Il comando sopra esaminerà il Disco 0, rileverà tutte le partizioni montate, creerà uno snapshot VSS di queste e infine genererà un'immagine di backup avviabile dell'intero disco in formato FIDX.

Il backup successivo sarà incrementale; l'hashing è stato parallelizzato, quindi è facile raggiungere velocità di 1 GB/sec.

### Ripristino dei file — NUOVO!

Il ripristino dei file è possibile usando lo strumento nbd.

Per usare nbd esegui prima `modprobe nbd max_part=0`.

Per motivi sconosciuti, l'uso di `max_part != 0` causa un loop infinito nel probing delle partizioni.

Lo strumento NBD collegherà qualsiasi backup di disco fisso, indipendentemente dal fatto che sia un backup di VM o di host (funziona anche per i backup PVE).

Per usarlo usa una riga di comando simile a questa:
`./pbsnbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Se ometti `-path`, apparirà un'interfaccia a terminale che ti permetterà di selezionare il file fidx.

> Attenzione a non usarlo su una macchina con dati importanti (un filesystem corrotto può potenzialmente mandare in crash il sistema operativo, ecco perché Proxmox VE usa un'istanza QEMU per questo).

Assicurati inoltre di aver smontato tutto dal disco nbd prima di fermare pbsnbd, altrimenti probabilmente ti ritroverai con una partizione occupata e non smontabile. Se qualcuno sa come risolvere, faccelo sapere.

Se ricevi un errore `Device or resource busy`, devi forzare la disconnessione eseguendo `nbd-client -d /dev/nbd0` o semplicemente riavviare.

### Ripristino su macchina fisica

Verrà rilasciato un sistema live CD / avvio PXE che permetterà di accedere a un server PBS, selezionare il backup e avviare clonezilla. Per ora il metodo migliore è avviare una clonezilla live e copiarvi l'eseguibile del server nbd; prima di procedere con clonezilla, su un altro tty, avvia `pbsnbd`.

Suggerisco anche di copiare i parametri della riga di comando come authid, baseurl, fingerprint ecc., sono una scocciatura da digitare a mano!

Una volta avviato pbsnbd, puoi usare l'opzione disco-a-disco-locale di clonezilla.

## Utilizzo — Backup di directory

Un comando tipico sarebbe simile a:

```shell
proxmoxbackupgo.exe -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -backupdir "C:\path\to\backup" -datastore "datastorename"
```

```
proxmoxbackupgo.exe
  -authid string
        Authentication ID (PBS Api token)
  -secret string
        Secret for authentication
  -backupdir string
        Backup source directory, must not be symlink
  -baseurl string
        Base URL for the proxmox backup server, example: https://192.168.1.10:8007
  -certfingerprint string
        Certificate fingerprint for SSL connection, example: ea:7d:06:f9...
  -datastore string
        Datastore name
  -namespace string
        Namespace (optional)
  -backup-id string
        Backup ID (optional - if not specified, the hostname is used as the default for host-type backups)
  -pxarout string
        Output PXAR archive for debug purposes (optional)
  -backupstream string  ***NEW***
        Filename for stream backup
  -mail-host string
        mail notification system: mail server host(optional)
  -mail-port string
        mail notification system: mail server port(optional)
  -mail-username string
        mail notification system: mail server username(optional)
  -mail-password string
        mail notification system: mail server password(optional)
  -mail-insecure bool
        mail notification system: allow insecure communications(optional)
  -mail-from string
        mail notification system: sender mail(optional)
  -mail-to string
        mail notification system: receiver mail(optional)

  -mail-subject-template string
        mail notification system: mail subject template(optional)
  -mail-body-template string
        mail notification system: mail body template(optional)

  -config string
        Path to JSON config file. If this flag is provided all the others will override the loaded config file
```

Per la configurazione JSON viene fornito un esempio JSON, compila solo i campi necessari.

Nota sul templating delle mail:
Viene usato il [motore di templating di Go](https://pkg.go.dev/text/template) per oggetto e corpo delle mail, fai riferimento alla documentazione per la sintassi.
Le seguenti variabili sono disponibili per il templating:

- `.NewChunks`: numero di nuovi chunk creati
- `.ReusedChunks`: numero di chunk riutilizzati
- `.Datastore`: nome del datastore
- `.Error`: messaggio di errore se presente
- `.Hostname`: hostname della macchina
- `.StartTime`: ora di inizio del backup
- `.EndTime`: ora di fine del backup
- `.Duration`: durata del backup
- `.FromattedDuration`: durata formattata del backup
- `.Success`: booleano che indica se il backup è riuscito
- `.Status`: rappresentazione testuale dello stato del backup [SUCCESS, FAILURE]

## Stream Backup

Questo consente di eseguire il backup di uno stream invece di un PXAR, aprendo infinite possibilità. Ad esempio puoi invocare:

```
mysqldump yourdatabase | ./proxmoxbackupgo -backupstream yourdatabase.sql [other options]
```

Ciò consente di sfruttare buzhash per la deduplicazione anche quando si usa tar, ad esempio, o il dump sql stesso. E se qualcuno volesse provarci, dovrebbe essere possibile con qualche hack convogliare il comando DISM per generare un'immagine WIM e ottenere un backup completo dell'host.

## Problemi noti

L'antimalware Windows Defender attivo rallenta il backup fino al 25% della velocità raggiungibile.

~~Al momento non esiste alcun meccanismo per impedire l'avvio di due istanze contemporaneamente, il che compromette VSS e il backup~~
Se usi l'utilità di pianificazione di Windows, teoricamente dovrebbe impedire che due istanze partano contemporaneamente quando originate dallo stesso job.

## 📋 Prerequisiti

- Windows 10/11 (64 bit)
- Diritti di amministratore (per gli snapshot VSS)
- Accesso di rete a un server Proxmox Backup Server
- Linux funziona anche, soprattutto per lo sviluppo

## 🔨 Compilazione dai sorgenti

### Prerequisiti
- Go 1.22 o successivo
- Node.js 20 o successivo
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### GUI
```bash
cd gui
npm install --prefix frontend
wails build      # oppure: wails dev  (hot reload)
```

### Progetto completo (Makefile)
```bash
make install-deps   # installa Wails CLI + dipendenze frontend
make                # compila tutto (CLI + GUI + Service)
```

Gli artefatti vengono collocati nella directory `dist/`. Vedi `make help` per tutti i target (`cli`, `gui`, `service`, `test`, `lint`, `security-check`, `release`...).

## 🌐 Supporto tecnico in Italia

Premesso che il progetto e **TUTTI** i suoi contributi resteranno per sempre pubblici e licenziati sotto licenza GPLv3, lo sponsor principale di questo progetto e anche delle più recenti funzionalità di machine backup, E.T.I. Srl ( https://etitech.net ), può fornire a chi ne ha bisogno supporto per le implementazioni Proxmox in generale e specificamente per gli strumenti di backup Windows.

Non ci sarà mai un'edizione «community» e una «enterprise» separate: il supporto tecnico sarà esclusivamente un servizio indipendente.
Qualsiasi personalizzazione richiesta, anche se lo sviluppo dovesse essere a pagamento, verrà rilasciata sotto GPLv3 come l'intero progetto.

## 🖥️ Attribuzione della GUI

La **Proxmox Backup Client GUI** è basata sulla **[GUI Nimbus Backup](https://nimbus.rdem-systems.com)**, sviluppata e mantenuta da **[RDEM Systems](https://www.rdem-systems.com/)**.

La GUI (originariamente un fork di questo progetto) è stata integrata in questo repository: l'intero codebase, inclusa la GUI e tutte le sue funzionalità, rimane open-source sotto licenza GPLv3. RDEM Systems sponsorizza lo sviluppo della GUI e ne offre il supporto commerciale.

## ⚠️ Avvertenza

Questo software è fornito «così com'è». Anche se miriamo all'affidabilità, decliniamo ogni responsabilità per perdita o danneggiamento dei dati. Testa sempre i tuoi backup e verifica il ripristino prima di affidarti ad essi in produzione.

Il software è ancora di qualità alpha e non ci assumiamo alcuna responsabilità per qualsiasi tipo di danno o perdita di dati, anche dei file sorgente.

## 📄 Licenza

GPLv3 — vedi il file [LICENSE](LICENSE).

## 🤝 Contribuisci

I contributi sono benvenuti, in particolare:

1. Icona nella tray per mostrare l'avanzamento del backup e il backup in corso
2. Supporto alla crittografia
3. Un modo per configurare tramite GUI e magari creare un file di job JSON simile a freefilesync
4. Upload/compressione asincroni e upload/compressione dei chunk multicore
5. Patch lato Proxmox per aggiungere un altro tipo di voce al formato pxar con i descrittori di sicurezza Windows
6. Supporto per i symlink Windows
7. Qualsiasi cosa interessante ti venga in mente :)

## Informazioni sui contributori di Proxmox Backup Client GO

I contributori di Proxmox Backup Client GO sviluppano e mantengono questo progetto. Il software si basa sull'infrastruttura NTP/NTS e gli [11 server NTS pubblici](https://github.com/jauderho/nts-servers) elencati nel riferimento della community.

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**