# Proxmox Backup Client: πρόγραμμα-πελάτης Windows και Linux για το Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Το Proxmox Backup Client είναι πρόγραμμα-πελάτης αντιγράφων ασφαλείας ανοιχτού κώδικα (GPL-3.0) για το Proxmox Backup Server (PBS), σε Windows και Linux.**

Είναι μια **σουίτα εργαλείων** για αντίγραφα ασφαλείας στο PBS:

- **Proxmox Backup Client GUI** (βασισμένο στο Nimbus Backup GUI της RDEM Systems): γραφικό πρόγραμμα για διακομιστές και σταθμούς εργασίας Windows και Linux, με συνεπή στιγμιότυπα (VSS στα Windows, elastio-snap/dattobd στο Linux), Σύνολα Αντιγράφων Ασφαλείας με χρονοδιαγράμματα, αντίγραφα φακέλων και ολόκληρων δίσκων, επαναφορά, Ανάκτηση διαγραμμένων και Επαναφορά κατάστασης, πολλούς διακομιστές PBS και υπηρεσία παρασκηνίου σε Windows και Linux.
- **`proxmoxbackup-directory`**: αντίγραφα φακέλων (pxar) από τη γραμμή εντολών, με απαλοιφή διπλοτύπων και εντοπισμό αλλαγών.
- **`proxmoxbackup-machine`**: αντίγραφα ολόκληρων δίσκων (fidx) μηχανημάτων Windows (VSS) και Linux (elastio-snap/dattobd) εν λειτουργία· η εικόνα εκκινεί μετά από επαναφορά bare-metal.
- **`proxmoxbackup-nbd`**: συνδέει ένα αντίγραφο δίσκου ως συσκευή NBD για επαναφορά (Linux).
- Ένα **ISO επαναφοράς bare-metal** βασισμένο στο Clonezilla Live.

> ⚠️ **Αποποίηση ευθύνης:** Αυτό το έργο **δεν συνδέεται με κανέναν τρόπο** με την **Proxmox Server Solutions GmbH**. Το «Proxmox», το λογότυπο Proxmox και τα σχετικά ονόματα ανήκουν στους κατόχους τους και χρησιμοποιούνται εδώ **μόνο** για να δηλώσουν συμβατότητα. Δείτε το [proxmox.com](https://www.proxmox.com/).

> ℹ️ **Σχετικά με αυτή τη σελίδα.** Είναι μεταφρασμένη σύνοψη. Το αγγλικό [README.md](README.md) είναι το έγγραφο αναφοράς με όλες τις λεπτομέρειες. Τα εγχειρίδια είναι στα αγγλικά. Τα ονόματα μενού και κουμπιών δίνονται όπως στην ελληνική διεπαφή της εφαρμογής. Η μετάφραση έγινε με τη βοήθεια τεχνητής νοημοσύνης· οι διορθώσεις είναι ευπρόσδεκτες.

## 📖 Εγχειρίδια χρήσης

| Εγχειρίδιο | Περιεχόμενο |
|---|---|
| **[GUI](docs/manual/GUI.md)** | Εγκατάσταση, διακομιστές PBS, Σύνολα και χρονοδιαγράμματα, αυξητικά αντίγραφα, επαναφορά, Ανάκτηση διαγραμμένων, Επαναφορά κατάστασης, Αναφορές, Προτιμήσεις, υπηρεσία παρασκηνίου, προβλήματα |
| **[Γραμμή εντολών](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: όλες οι παράμετροι, ρυθμίσεις, κωδικοί εξόδου, προγραμματισμός· Ανάκτηση διαγραμμένων και Επαναφορά κατάστασης από τη γραμμή εντολών |
| **[Επαναφορά bare-metal](docs/manual/BMR.md)** | Το ISO επαναφοράς βήμα προς βήμα· στιγμιότυπα `vm` στο Proxmox VE· διαφορετικό υλικό |
| [Σενάρια επαναφοράς](docs/RESTORE_GUIDE.md) | Ποια επαναφορά και πότε |
| [Ευρετήριο](docs/manual/README.md) | Επισκόπηση των τριών εγχειριδίων |

## 🆕 Στο master, για την επόμενη έκδοση

Στο [CHANGELOG](CHANGELOG.md) στην ενότητα [Unreleased], χωρίς να έχουν κυκλοφορήσει ακόμη:

- **Προγράμματα εγκατάστασης**: **MSI για Windows** (εφαρμογή και υπηρεσία στο Program Files, μενού Έναρξη, αναβάθμιση επί τόπου, διατήρηση ή διαγραφή ρυθμίσεων κατά την αφαίρεση) και **πακέτα Linux** `.deb` (Debian, Ubuntu, Mint) και `.rpm` (Fedora).
- **Υπηρεσία παρασκηνίου στο Linux** (`proxmoxbackupclient.service`): τα προγραμματισμένα Σύνολα τρέχουν ως root χωρίς συνδεδεμένο χρήστη. Ενεργοποίηση από **Προτιμήσεις > Προχωρημένα > Εκτέλεση ως υπηρεσία**, όπως στα Windows. Η **Επανεκκίνηση ως διαχειριστής** λειτουργεί και στο Linux (μέσω pkexec).
- **Παύση, Συνέχεια και Διακοπή για αντίγραφα που εκτελεί η υπηρεσία**.
- **Προειδοποίηση όταν δύο Σύνολα γράφουν στην ίδια ομάδα αντιγράφων** (ίδιος διακομιστής, τύπος και Backup ID), με σήμανση **Κοινό ID**.
- **Κλειδωμένες ρυθμίσεις**: το `C:\ProgramData\ProxmoxBackupClient` μόνο για διαχειριστές· στο Linux ο φάκελος ρυθμίσεων είναι ιδιωτικός για τον κάτοχο.
- **Αρχεία από αντίγραφα μηχανημάτων με FAT, exFAT, ext2/3/4 και XFS**, όχι μόνο NTFS· η επαναφορά εικόνων Windows επαναφέρει και δικαιώματα και εναλλακτικές ροές δεδομένων.
- **Τα αντίγραφα μηχανημάτων Linux παραλείπουν τον ελεύθερο χώρο** (ext2/3/4, XFS).
- **Ανάκτηση διαγραμμένων και Επαναφορά κατάστασης για μηχανήματα Linux**, και **από τη γραμμή εντολών**: `-list-sets`, `-undelete`, `-rollback` (ενότητα 12 του [εγχειριδίου γραμμής εντολών](docs/manual/CLI.md)).
- Τα εργαλεία γραμμής εντολών γράφουν τα σφάλματα στο stderr αντί για παράθυρα μηνυμάτων· το `proxmoxbackup-machine` στέλνει email ειδοποίησης.

## 💽 Νέο στην v0.8.0

- **Τα αντίγραφα μηχανημάτων στα Windows παραλείπουν τον ελεύθερο χώρο** (και τα αρχεία σελιδοποίησης και αδρανοποίησης): ένας μισοάδειος δίσκος διαβάζεται στον μισό περίπου χρόνο. Ένας **κατεστραμμένος τομέας** δεν σταματά πλέον το αντίγραφο: γράφεται ως μηδενικά και αναφέρεται.
- **Επαναφορά αρχείων από αντίγραφο μηχανήματος**: κάθε τόμος NTFS της εικόνας ανοίγει ως φάκελος, χωρίς προσάρτηση ή λήψη (ένας δίσκος C: 127 GB με 430.000 αρχεία εμφανίζεται σε 4 έως 15 δευτερόλεπτα). Ένα αντίγραφο μηχανήματος καλύπτει και την επαναφορά bare-metal και το «χρειάζομαι πίσω εκείνο το αρχείο».
- **Ανάκτηση διαγραμμένων** και **Επαναφορά κατάστασης** για Σύνολα μηχανημάτων (Windows), στους επιλεγμένους φακέλους (από προεπιλογή Desktop, Documents, Pictures, Music, Videos, Downloads).
- **Παύση** και **Συνέχεια**, σήμανση **VSS / Snapshot ON-OFF**, διατήρηση των **χρόνων δημιουργίας** των Windows.
- **v0.8.2:** ο κενός χώρος δεν κατακερματίζεται πια, έτσι ένας σχεδόν άδειος δίσκος ολοκληρώνεται στο ένα τρίτο του χρόνου (50 GB: από 3m36s σε 1m15s).

## 🗑️ Νέο στην v0.7.0: Ανάκτηση διαγραμμένων και Επαναφορά κατάστασης

- Η **Ανάκτηση διαγραμμένων** βρίσκει αρχεία που υπάρχουν στα αντίγραφα ενός Συνόλου αλλά όχι πια στον δίσκο (στο πιο πρόσφατο αντίγραφο ή σε 7 ή 30 ημέρες), τα δείχνει σε δέντρο με το πότε χάθηκαν και τα επαναφέρει στη θέση τους ή αλλού. Μετά τη διαγραφή 39 GB από φάκελο 761.000 αρχείων, η αναζήτηση 7 ημερών τα βρήκε σε περίπου ένα λεπτό.
- Η **Επαναφορά κατάστασης** επαναφέρει τους φακέλους ενός Συνόλου σε ένα επιλεγμένο αντίγραφο: πρώτα προεπισκόπηση, μετά επαναφορά. Ό,τι αντικαθίσταται μετακινείται στην άκρη στον ίδιο δίσκο, οπότε η **Αναίρεση** τα επαναφέρει όλα.

Επίσης: προτεινόμενο Backup ID, ώρα επόμενης εκτέλεσης σε κάθε κάρτα και η υπηρεσία παρασκηνίου στη λήψη για Windows. Δείτε το [εγχειρίδιο GUI](docs/manual/GUI.md#10-undelete).

## ⚡ Νέο στην v0.6.0: αυξητικά αντίγραφα φακέλων

Στη λειτουργία **Μεταδεδομένα** (όπως στο επίσημο `proxmox-backup-client`) τα αμετάβλητα αρχεία (ίδιο μέγεθος, χρόνος τροποποίησης σε νανοδευτερόλεπτα, δικαιώματα και κάτοχος) δεν διαβάζονται: τα δεδομένα τους παίρνονται από το προηγούμενο στιγμιότυπο. Διακομιστής αρχείων 480,9 GB με 761.866 αρχεία:

| Εκτέλεση | Χρόνος | Νέα τμήματα που στάλθηκαν |
|---|---|---|
| Πρώτη εκτέλεση Μεταδεδομένων | 4 ώρες 17 λεπτά | 135.549 |
| Επόμενες εκτελέσεις | περίπου 10 έως 15 λεπτά | 4 έως 40 |

Η κίνηση προς ένα απομακρυσμένο PBS περιορίζεται σχεδόν σε ό,τι άλλαξε. Η ρύθμιση περιγράφεται παρακάτω, στον εντοπισμό αλλαγών.

## 🤝 Συμβατό με το επίσημο πρόγραμμα-πελάτη Proxmox

Επαληθευμένο με `proxmox-backup-client` 3.4.9 και PBS 4.2: τα στιγμιότυπα Legacy, Data και Metadata **επαναφέρονται ταυτόσημα byte προς byte και προς τις δύο κατευθύνσεις** (από GUI και γραμμή εντολών, σε Windows και Linux), και ο περιηγητής αρχείων του PBS 4.2 τα ανοίγει. Ίδια μορφή pxar έκδοση 2, ίδιες λειτουργίες εντοπισμού αλλαγών, ίδια μορφή κλειδιού (`proxmox-backup-client key create`), ίδιες μεταβλητές `PBS_REPOSITORY`. Η μόνη προσθήκη είναι το αρχείο `.proxmox_backup_client_meta.json` (η αρχική διαδρομή) στη ρίζα ενός στιγμιότυπου φακέλου, που το επίσημο πρόγραμμα επαναφέρει ως κανονικό αρχείο.

## 🔧 Σχετικά με αυτό το fork

Το fork του **mjb-is**: **https://github.com/mjb-is/proxmoxbackupclient_go**. Στόχος: το PBS ως μοναδικό σύστημα αντιγράφων σε ένα homelab, με αξιόπιστο GUI και πλήρως αυτοματοποιημένη επαναφορά bare-metal. Ό,τι είναι γενικής χρήσης πηγαίνει στο αρχικό έργο· δεκαεπτά pull requests έχουν συγχωνευθεί (λίστα στο [αγγλικό README](README.md#-about-this-fork)).

## 📦 Λήψη και εγκατάσταση

👉 **[Λήψη της τελευταίας έκδοσης](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| Έχετε | Χρησιμοποιήστε |
|---|---|
| Windows | **MSI** (`ProxmoxBackupClient-<version>-x64.msi`): εφαρμογή και υπηρεσία παρασκηνίου στο Program Files, μενού Έναρξη. Ή `ProxmoxBackupClient-v<version>-windows-amd64.zip` (εφαρμογή και `ProxmoxBackupClientSVC.exe` στον ίδιο φάκελο) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Άλλο Linux | Αποσυμπιέστε το `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (χρειάζεται WebKitGTK) |
| Μόνο γραμμή εντολών | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` ή `-linux-amd64.tar.gz` |

Τα MSI, `.deb` και `.rpm` υπάρχουν μόνο στο master (οι εκδόσεις έως v0.8.1 έχουν μόνο zip και tar.gz)· δημιουργήστε τα με το [BUILD.md](BUILD.md). Στο Linux η καταχώριση **Proxmox Backup Client** τρέχει την εφαρμογή ως εσάς και η **Proxmox Backup Client (administrator)** ως root μέσω polkit, για αντίγραφα μηχανημάτων. Η υπηρεσία εγκαθίσταται αλλά δεν ενεργοποιείται: δείτε το [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **Τα Windows αναφέρουν ιό (π.χ. `Trojan:Win32/Sabsik.FL.A!ml`) ή προειδοποίηση SmartScreen;** Είναι **γνωστό ψευδώς θετικό** για εφαρμογές Go/Wails χωρίς υπογραφή (το `!ml` σημαίνει μοντέλο μηχανικής μάθησης). [Περισσότερα](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Επαλήθευση λήψης

Κάθε έκδοση έχει `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-v0.8.1-windows-amd64.zip -Algorithm SHA256   # compare with SHA256SUMS.txt
```

Στο Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`. Δεν υπάρχει ακόμη υπογραφή κώδικα, πιστοποίηση CI ή σάρωση VirusTotal (δείτε το [TODO.md](TODO.md)).

## ✨ Δυνατότητες

### GUI (συνιστάται)
- **18 γλώσσες διεπαφής** (γαλλικά, αγγλικά, ιταλικά, γερμανικά, πολωνικά, ισπανικά, βουλγαρικά, τσεχικά, ελληνικά, ουγγρικά, λετονικά, λιθουανικά, ολλανδικά, πορτογαλικά, ρουμανικά, σλοβακικά, τουρκικά, ουκρανικά)
- Μενού: Αντίγραφο ασφαλείας, Επαναφορά, Ανάκτηση διαγραμμένων, Επαναφορά κατάστασης, Αναφορές, Αρχείο καταγραφής μηνυμάτων, Προτιμήσεις, Σχετικά
- **Σύνολα Αντιγράφων Ασφαλείας**: χρονοδιάγραμμα (ημερήσιο / περιοδικό / χειροκίνητο), σημάνσεις τύπου και VSS / Snapshot, **Εκτέλεση τώρα**, προειδοποίηση **Κοινό ID**· μοναδικά αντίγραφα φακέλων και δίσκων
- **Αυξητικά αντίγραφα φακέλων** (Μεταδεδομένα) με πλήρη ανάγνωση κάθε N εκτελέσεις
- **Αντίγραφα μηχανημάτων** σε Windows και Linux χωρίς ελεύθερο χώρο και ανθεκτικά σε κατεστραμμένους τομείς, ως `host` ή ως στιγμιότυπο `vm` του Proxmox VE
- **Επαναφορά**: περιήγηση, αναζήτηση (μπαλαντέρ, κανονικές εκφράσεις), επιλεκτική επαναφορά με ACL NTFS/POSIX και χρόνους φακέλων, **Επαλήθευση μετά την επαναφορά**· αρχεία από εικόνες δίσκων
- **Ανάκτηση διαγραμμένων**, **Επαναφορά κατάστασης**, **Παύση, Συνέχεια και Διακοπή**
- **Εκτέλεση ως υπηρεσία** (Προτιμήσεις > Προχωρημένα): χρονοδιαγράμματα χωρίς συνδεδεμένο χρήστη (Windows από v0.7.0· Linux στο master)
- Κάρτα προόδου, Αναφορές, Αρχείο καταγραφής μηνυμάτων, πέντε χρωματικά θέματα, ειδοποιήσεις email, ενέργειες μετά το αντίγραφο, εξαγωγή και εισαγωγή ρυθμίσεων
- Πολλοί διακομιστές PBS, καρφίτσωμα αποτυπώματος πιστοποιητικού, σύνδεση με API token ή κωδικό
- **Κρυπτογράφηση στην πλευρά του πελάτη** (AES-256-GCM, υπογεγραμμένο manifest)

### Εργαλεία γραμμής εντολών
- `proxmoxbackup-directory`: αντίγραφα φακέλων με `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: αντίγραφα ολόκληρων δίσκων (fidx, αυξητικά· το `-readfreespace` διαβάζει και τον ελεύθερο χώρο)
- `proxmoxbackup-nbd`: διακομιστής NBD για επαναφορά (Linux)
- `ProxmoxBackupClientSVC` (Windows) και `proxmoxbackupclient` (Linux): `-list-sets`, `-undelete`, `-rollback` (στο master)

Τα τρία πρώτα δέχονται `-keyfile` και `-keyfile-passphrase`. Αναφορά: [εγχειρίδιο γραμμής εντολών](docs/manual/CLI.md).

### 📸 Στιγμιότυπα οθόνης

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Σύνολα Αντιγράφων Ασφαλείας**: εργασίες με σημάνσεις προγραμματισμού και τύπου, Εκτέλεση τώρα σε καθεμία

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Πρόοδος αντιγράφου**: ρυθμός, εκτιμώμενος χρόνος, νέα και επαναχρησιμοποιημένα τμήματα

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Επαναφορά**: διακομιστής PBS και Backup ID, λίστα στιγμιότυπων ή αναζήτηση ονόματος αρχείου

</td>
</tr>
</table>

Τα υπόλοιπα στιγμιότυπα οθόνης: στο [αγγλικό README](README.md#-screenshots).

### Εξαιρέσεις συστήματος και ασφάλεια
Τα αντίγραφα φακέλων παραλείπουν αυτόματα `System Volume Information`, `$RECYCLE.BIN`, `Recovery` (στη ρίζα μονάδας), `.pbs-rollback`, `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` και `DumpStack.log.tmp`. Για αρχεία χρησιμοποιήστε αντίγραφο **φακέλων**· για επαναφορά bare-metal ξεχωριστό Σύνολο με αντίγραφο **μηχανήματος**.

Ασφάλεια: κρυπτογράφηση τμημάτων, κλειδωμένος φάκελος ρυθμίσεων (στο master), τοπικό API υπηρεσίας μόνο στο 127.0.0.1 με token, έλεγχος εισόδου, αποτροπή διάσχισης διαδρομών, επαναλήψεις, unit tests και δοκιμές από άκρο σε άκρο.

## 🔐 Κρυπτογράφηση στην πλευρά του πελάτη

Τα στιγμιότυπα κρυπτογραφούνται πριν φύγουν με αρχείο κλειδιού Proxmox Backup Server (μορφή `proxmox-backup-client key create`): AES-256-GCM, υπογεγραμμένο manifest. Ο διακομιστής PBS δεν βλέπει ποτέ το κλειδί.

**GUI:** **Προτιμήσεις > Πληροφορίες λογαριασμού**, επεξεργασία διακομιστή, καρτέλα **Κρυπτογράφηση**: επιλέξτε αρχείο κλειδιού ή **Δημιουργία...**. Για κλειδί με φράση πρόσβασης: **Να με ρωτά** (η φράση μένει στη μνήμη για τη συνεδρία· τα προγραμματισμένα αντίγραφα και η υπηρεσία αποτυγχάνουν αμέσως χωρίς αυτήν) ή **Απομνημόνευση σε αυτόν τον υπολογιστή** (η φράση στο `config.json`· για αυτόματες εργασίες και την υπηρεσία). **Χωρίς το αρχείο κλειδιού δεν γίνεται επαναφορά: κρατήστε αντίγραφο του κλειδιού μακριά από αυτό το μηχάνημα.**

```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```

Το ISO του Clonezilla ζητά κλειδί και φράση κατά την επαναφορά· το κλειδί δεν γράφεται ποτέ στο ISO. Το `config.json` κρατά tokens και φράσεις σε απλό κείμενο (έως την v0.8.2 στα Windows ήταν αναγνώσιμο από τοπικούς χρήστες).

## Ταχύτερα αντίγραφα φακέλων: εντοπισμός αλλαγών

| Λειτουργία | Διαβάζει | Αποθηκεύεται ως |
|---|---|---|
| **Κλασικό** (Legacy, προεπιλογή) | όλα τα αρχεία | `<name>.pxar.didx` |
| **Δεδομένα** (Data) | όλα τα αρχεία | διαιρεμένο αρχείο `<name>.mpxar.didx` + `<name>.ppxar.didx` |
| **Μεταδεδομένα** (Metadata) | μόνο νέα και αλλαγμένα | το ίδιο διαιρεμένο αρχείο |

**GUI:** Σύνολο, καρτέλα **Προορισμός**, **Εντοπισμός αλλαγών**. Ορίστε **Ανάγνωση όλων των αρχείων κάθε** N εκτελέσεις για να πιάνονται σπάνιες αλλαγές που κρατούν μέγεθος και χρόνο (π.χ. 42 για εκτέλεση κάθε 4 ώρες). **Δώστε σε κάθε Σύνολο δικό του Backup ID**, αλλιώς τα Σύνολα αναγκάζουν το ένα το άλλο να διαβάζει τα πάντα. Μόνο για Σύνολα φακέλων· τα μοναδικά αντίγραφα χρησιμοποιούν το Κλασικό. **CLI:** `-change-detection-mode legacy|data|metadata`.

Πριν περάσετε σε Δεδομένα ή Μεταδεδομένα, ενημερώστε σε v0.6.0 ή νεότερη κάθε μηχάνημα από το οποίο κάνετε επαναφορά. Το πρώτο διαιρεμένο αντίγραφο κοστίζει περίπου ένα ακόμη πλήρες αντίγραφο μέχρι να διαγραφούν τα παλιά στιγμιότυπα.

## 🚀 Γρήγορη εκκίνηση (GUI)

1. Εγκαταστήστε MSI, `.deb` ή `.rpm`, ή αποσυμπιέστε zip ή tar.gz από τις [εκδόσεις](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
2. Στα Windows, **Εκτέλεση ως διαχειριστής** για VSS και αντίγραφα μηχανημάτων· στο Linux, **Proxmox Backup Client (administrator)** για αντίγραφα μηχανημάτων
3. Προσθέστε τον διακομιστή στις **Προτιμήσεις > Πληροφορίες λογαριασμού** και πατήστε **Δοκιμή**
4. Δημιουργήστε Σύνολο (ή μοναδικό αντίγραφο) και επιλέξτε φακέλους ή δίσκους
5. Εκτελέστε. Για χρονοδιαγράμματα χωρίς συνδεδεμένο χρήστη, ενεργοποιήστε **Προτιμήσεις > Προχωρημένα > Εκτέλεση ως υπηρεσία**

## Πλήρες αντίγραφο μηχανήματος εν λειτουργία

Αντί για `-backupdir` δίνεται `-backupdev` (επαναλαμβάνεται):

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Η εντολή παίρνει στιγμιότυπο VSS όλων των διαμερισμάτων του δίσκου 0 και δημιουργεί εκκινήσιμη εικόνα fidx. Ο ελεύθερος χώρος γράφεται ως μηδενικά χωρίς ανάγνωση· τα επόμενα αντίγραφα είναι αυξητικά, έως 1 GB/s.

### Προαπαιτούμενα αντιγράφων μηχανημάτων Linux (αντίστοιχο του VSS)

Χρειάζεται το άρθρωμα πυρήνα [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap) (ή ένα ήδη φορτωμένο `dattobd`). Η εγκατάσταση από αποθετήριο πακέτων δεν λειτουργεί αυτή τη στιγμή για Ubuntu 22.04· μεταγλωττίστε από τον πηγαίο κώδικα:

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

Το αντίγραφο μηχανήματος πρέπει να τρέχει **ως root** (καταχώριση **Proxmox Backup Client (administrator)**, υπηρεσία ή `sudo`)· η ομάδα `disk` δεν αρκεί.

### Επαναφορά αρχείων από αντίγραφο δίσκου

Η σελίδα Επαναφορά ανοίγει ένα αντίγραφο μηχανήματος ως φακέλους (NTFS από v0.8.0· FAT, exFAT, ext2/3/4 και XFS στο master). Στο Linux μπορείτε επίσης να το συνδέσετε μέσω NBD μετά από `modprobe nbd max_part=0`:

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Χωρίς `-path` εμφανίζεται επιλογή αρχείου. Μην το κάνετε σε σημαντικό μηχάνημα· αποπροσαρτήστε τα πάντα πριν το σταματήσετε· με `Device or resource busy` εκτελέστε `nbd-client -d /dev/nbd0`.

### Επαναφορά σε φυσικό μηχάνημα

Το ISO Clonezilla Live αυτού του fork έχει την αυτόματη καταχώριση **«PBS Bare Metal Restore»**: στοιχεία PBS, στιγμιότυπο, δίσκος προορισμού, και τα υπόλοιπα γίνονται μόνα τους ως την επανεκκίνηση. Υπάρχει και η ημι-χειροκίνητη καταχώριση «pbs-nbd». Δείτε το [εγχειρίδιο](docs/manual/BMR.md).

## Χρήση: αντίγραφο φακέλων

Πλήρης αναφορά: **[εγχειρίδιο γραμμής εντολών](docs/manual/CLI.md)**.

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

Διαβάζονται και οι μεταβλητές περιβάλλοντος του επίσημου προγράμματος (`PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD`, `PBS_FINGERPRINT`). Τα email χρησιμοποιούν [πρότυπα Go](https://pkg.go.dev/text/template). Στα Windows ένα δεύτερο αντίγραφο δεν ξεκινά όσο τρέχει το πρώτο· στο Linux δεν υπάρχει τέτοιο κλείδωμα. Το Windows Defender επιβραδύνει το αντίγραφο έως 25%.

## 🔨 Μεταγλώττιση από τον πηγαίο κώδικα

Όλα στο **[BUILD.md](BUILD.md)** (Go 1.25+, Wails CLI v2.13):

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

## 🔧 Προχωρημένη χρήση και οδηγοί

- **Πολλοί διακομιστές PBS:** [οδηγός χρήστη](MULTI_PBS_USER_GUIDE.md) και [οδηγός υλοποίησης](MULTI_PBS_GUIDE.md).
- **ISO Clonezilla Live:** η δημιουργία του περιγράφεται στο **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**:

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**: τι είναι έτοιμο και από ποια έκδοση. **[CHANGELOG.md](CHANGELOG.md)**: ιστορικό αλλαγών. **[TODO.md](TODO.md)**: σχέδια. **[RELEASE_NOTES.md](RELEASE_NOTES.md)**, **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**, **[AGENTS.md](AGENTS.md)**.

## 🌐 Τεχνική υποστήριξη στην Ιταλία

Το έργο και **ΟΛΕΣ** οι συνεισφορές μένουν για πάντα ανοιχτά υπό GPLv3. Ο κύριος χορηγός, η E.T.I. Srl ( https://etitech.net ), προσφέρει υποστήριξη για Proxmox και εργαλεία αντιγράφων Windows. Δεν θα υπάρξουν χωριστές εκδόσεις «community» και «enterprise»· κάθε προσαρμογή κυκλοφορεί υπό GPLv3.

## 🖥️ Προέλευση του GUI

Το **Proxmox Backup Client GUI** βασίζεται στο **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)** της **[RDEM Systems](https://www.rdem-systems.com/)**, που συγχωνεύθηκε ξανά σε αυτό το αποθετήριο υπό GPLv3. Η RDEM Systems χορηγεί την ανάπτυξη του GUI και παρέχει εμπορική υποστήριξη.

## ⚠️ Προειδοποίηση

Το λογισμικό παρέχεται «ως έχει», είναι σε ποιότητα alpha, και δεν φέρουμε ευθύνη για απώλεια ή ζημιά δεδομένων. Δοκιμάζετε πάντα τα αντίγραφα και την επαναφορά.

## 📄 Άδεια

GPLv3: δείτε το [LICENSE](LICENSE).

## 🏷️ Branding

Συνεισφέροντες με **τουλάχιστον 5 commits** λειτουργιών ή διορθώσεων μπορούν να προσθέσουν το branding τους για εμπορική χρήση, εφόσον η εταιρεία **δεν** σχετίζεται με κακόβουλο λογισμικό, προώθηση πολέμου, απάτες, κλοπή δεδομένων, εμπορία παιδιών, βία, διακρίσεις, ναρκωτικά ή άλλη παράνομη δραστηριότητα. **Η άδεια GPLv3 παραμένει σε ισχύ.**

## 🤝 Συνεισφορά

Ευπρόσδεκτη βοήθεια: έλεγχος των μεταφράσεων κρυπτογράφησης και διαχείριση κλειδιών πέρα από αρχείο (π.χ. Windows DPAPI), P2V (το στιγμιότυπο `vm` λειτουργεί ήδη, λείπουν κάρτα δικτύου, TPM, οδηγοί virtio), ασύγχρονη αποστολή τμημάτων, περιγραφείς ασφαλείας Windows στο pxar, συμβολικοί σύνδεσμοι Windows και κάθε ενδιαφέρουσα ιδέα :)

## Σχετικά με τους συνεισφέροντες του Proxmox Backup Client GO

Οι συνεισφέροντες του Proxmox Backup Client GO αναπτύσσουν και συντηρούν το έργο. Το λογισμικό βασίζεται στην υποδομή NTP/NTS και στους [11 δημόσιους διακομιστές NTS](https://github.com/jauderho/nts-servers).

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
