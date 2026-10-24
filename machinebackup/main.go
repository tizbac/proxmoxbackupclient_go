package main

import (
	"clientcommon"

	"encoding/json"
	"flag"
	"fmt"

	"machinebackuplib"

	"os"
	"strings"
	"time"

	"pbscommon"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return fmt.Sprintf("%v", *i)
}

// Set is an implementation of the flag.Value interface
func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func loadConfig() *machinebackuplib.Config {
	// Define flags

	var backupdevs arrayFlags

	baseURLFlag := flag.String("baseurl", "", "Base URL for the proxmox backup server, example: https://192.168.1.10:8007")
	certFingerprintFlag := flag.String("certfingerprint", "", "Certificate fingerprint for SSL connection, example: ea:7d:06:f9...")
	authIDFlag := flag.String("authid", "", "Authentication ID (PBS Api token)")
	secretFlag := flag.String("secret", "", "Secret for authentication")
	pbsUsernameFlag := flag.String("pbsusername", "", "PBS username for ticket login (with -pbspassword; overrides -authid/-secret)")
	pbsPasswordFlag := flag.String("pbspassword", "", "PBS password for ticket login (with -pbsusername)")
	datastoreFlag := flag.String("datastore", "", "Datastore name")
	namespaceFlag := flag.String("namespace", "", "Namespace (optional)")
	backupIDFlag := flag.String("backup-id", "", "Backup ID (optional - if not specified, the hostname is used as the default)")
	backupTypeFlag := flag.String("type", "", "host|vm , vm will allow to restore as VM inside proxmox VE (Physical to Virtual), also it enables to use file restore feature, use numeric backupid")
	flag.Var(&backupdevs, "backupdev", "Can be specified multiple times,Backup device file ( On windows it can be \\\\.\\PhysicalDriveN , in that case VSS will be leveraged to take consistent snapshot), on linux can be /dev/sdX or whatever ; mounted partitions get a consistent point-in-time snapshot via the elastio-snap/dattobd kernel module (requires root)")
	readFreeSpaceFlag := flag.Bool("readfreespace", false, "Windows: also read free space and the paging/hibernation files (a forensic copy); by default they go into the image as zeros without being read")
	sysTrayFlag := flag.Bool("systray", false, "Enable systray( Note it can cause issues when running with no user logged in )")
	keyFileFlag := flag.String("keyfile", "", "Path to a Proxmox Backup Server encryption key file (encrypts chunks and signs the manifest; same format as proxmox-backup-client key create)")
	keyFilePassphraseFlag := flag.String("keyfile-passphrase", "", "Passphrase for a scrypt/PBKDF2 protected -keyfile (prompted for when omitted)")
	mailHostFlag := flag.String("mail-host", "", "mail notification system: mail server host(optional)")
	mailPortFlag := flag.String("mail-port", "", "mail notification system: mail server port(optional)")
	mailUsernameFlag := flag.String("mail-username", "", "mail notification system: mail server username(optional)")
	mailPasswordFlag := flag.String("mail-password", "", "mail notification system: mail server password(optional)")
	mailInsecureFlag := flag.Bool("mail-insecure", false, "mail notification system: allow insecure communications(optional)")
	mailFromFlag := flag.String("mail-from", "", "mail notification system: sender mail(optional)")
	mailToFlag := flag.String("mail-to", "", "mail notification system: receiver mail(optional)")
	mailSubjectTemplateFlag := flag.String("mail-subject-template", "", "mail notification system: mail subject template(optional)")
	mailBodyTemplateFlag := flag.String("mail-body-template", "", "mail notification system: mail body template(optional)")

	configFile := flag.String("config", "", "Path to JSON config file. If this flag is provided all the others will override the loaded config file")

	// Parse command line flags
	flag.Parse()

	config := &machinebackuplib.Config{
		BackupType: "host",
	}
	pbscommon.ApplyPBSEnvVars(&config.BaseURL, &config.AuthID, &config.Secret, &config.Datastore, &config.CertFingerprint)
	if *configFile != "" {
		file, err := os.ReadFile(*configFile)
		if err != nil {
			fmt.Printf("Error reading config file: %v\n", err)
			os.Exit(1)
		}
		err = json.Unmarshal(file, config)
		if err != nil {
			fmt.Printf("Error parsing config file: %v\n", err)
			os.Exit(1)
		}
	}

	if *baseURLFlag != "" {
		config.BaseURL = *baseURLFlag
	}
	if *certFingerprintFlag != "" {
		config.CertFingerprint = *certFingerprintFlag
	}
	if *authIDFlag != "" {
		config.AuthID = *authIDFlag
	}
	if *secretFlag != "" {
		config.Secret = *secretFlag
	}
	if *pbsUsernameFlag != "" {
		config.PBSUsername = *pbsUsernameFlag
	}
	if *pbsPasswordFlag != "" {
		config.PBSPassword = *pbsPasswordFlag
	}
	if *datastoreFlag != "" {
		config.Datastore = *datastoreFlag
	}
	if *namespaceFlag != "" {
		config.Namespace = *namespaceFlag
	}
	if *backupIDFlag != "" {
		config.BackupID = *backupIDFlag
	}
	config.BackupDevices = backupdevs
	if *readFreeSpaceFlag {
		config.ReadFreeSpace = true
	}
	if *sysTrayFlag {
		config.SysTray = true
	}
	if *keyFileFlag != "" {
		config.KeyFile = *keyFileFlag
	}
	if *keyFilePassphraseFlag != "" {
		config.KeyFilePassphrase = *keyFilePassphraseFlag
	}

	if *backupTypeFlag != "" {
		config.BackupType = *backupTypeFlag
	}

	initSmtpConfigIfNeeded := func() {
		if config.SMTP == nil {
			config.SMTP = &machinebackuplib.SMTPConfig{}
		}
	}
	initMailConfsIfNeeded := func() {
		initSmtpConfigIfNeeded()
		if len(config.SMTP.Mails) == 0 {
			config.SMTP.Mails = append(config.SMTP.Mails, machinebackuplib.MailSendConfig{})
		}
	}
	initTemplateIfNeeded := func() {
		initSmtpConfigIfNeeded()
		if config.SMTP.Template == nil {
			config.SMTP.Template = &machinebackuplib.MailTemplate{}
		}
	}

	if *mailHostFlag != "" {
		initSmtpConfigIfNeeded()
		config.SMTP.Host = *mailHostFlag
	}
	if *mailPortFlag != "" {
		initSmtpConfigIfNeeded()
		config.SMTP.Port = *mailPortFlag
	}
	if *mailUsernameFlag != "" {
		initSmtpConfigIfNeeded()
		config.SMTP.Username = *mailUsernameFlag
	}
	if *mailPasswordFlag != "" {
		initSmtpConfigIfNeeded()
		config.SMTP.Password = *mailPasswordFlag
	}
	if *mailInsecureFlag {
		initSmtpConfigIfNeeded()
		config.SMTP.Insecure = *mailInsecureFlag
	}
	if *mailFromFlag != "" {
		initMailConfsIfNeeded()
		config.SMTP.Mails[0].From = *mailFromFlag
	}
	if *mailToFlag != "" {
		initMailConfsIfNeeded()
		config.SMTP.Mails[0].To = *mailToFlag
	}
	if *mailSubjectTemplateFlag != "" {
		initTemplateIfNeeded()
		config.SMTP.Template.Subject = *mailSubjectTemplateFlag
	}
	if *mailBodyTemplateFlag != "" {
		initTemplateIfNeeded()
		config.SMTP.Template.Body = *mailBodyTemplateFlag
	}

	return config
}

// sendResultMail sends the notification e-mail when -mail-* (or the config
// file's smtp section) asks for one. A mail problem is reported on stderr
// and never changes the backup's own exit code. It returns false when a
// mail could not be sent.
func sendResultMail(cfg *machinebackuplib.Config, ctx clientcommon.MailCtx) bool {
	if cfg.SMTP == nil || len(cfg.SMTP.Mails) == 0 {
		return true
	}
	build := func(custom, def string) string {
		if custom != "" {
			if s, err := ctx.BuildStr(custom); err == nil {
				return s
			} else {
				fmt.Fprintln(os.Stderr, "Cannot use the custom mail template: "+err.Error())
			}
		}
		s, _ := ctx.BuildStr(def)
		return s
	}
	var subjT, bodyT string
	if cfg.SMTP.Template != nil {
		subjT, bodyT = cfg.SMTP.Template.Subject, cfg.SMTP.Template.Body
	}
	subject := build(subjT, machinebackuplib.DefaultMailSubjectTemplate)
	body := build(bodyT, machinebackuplib.DefaultMailBodyTemplate)
	client, err := clientcommon.SetupMailClient(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.Insecure)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot connect to the mail server: "+err.Error())
		return false
	}
	defer client.Quit()
	sent := true
	for _, m := range cfg.SMTP.Mails {
		if err := clientcommon.SendMail(m.From, m.To, subject, body, client); err != nil {
			fmt.Fprintln(os.Stderr, "Cannot send the notification e-mail: "+err.Error())
			sent = false
		} else {
			fmt.Printf("Notification e-mail sent to %s\n", m.To)
		}
	}
	return sent
}

func hostnameOrEmpty() string {
	h, _ := os.Hostname()
	return h
}

func fatalError(msg string, err error) {
	fmt.Fprintf(os.Stderr, "Fatal error: %s: %v\n", msg, err)
	os.Exit(1)
}

func main() {

	cfg := loadConfig()

	// Errors go to stderr with a non-zero exit, never a message box: a box
	// waits for a click that an unattended scheduled run never gets.
	if ok := cfg.Valid(); !ok {
		fmt.Fprintln(os.Stderr, "Missing or invalid options. Options:")
		flag.CommandLine.SetOutput(os.Stderr)
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Without -certfingerprint, fetch the fingerprint the server presents and
	// ask the user to confirm it before pinning.
	fingerprint, err := clientcommon.ConfirmFingerprint(cfg.BaseURL, cfg.CertFingerprint)
	if err != nil {
		fatalError("certificate fingerprint confirmation", err)
	}
	cfg.CertFingerprint = fingerprint

	// Ticket login without -pbspassword: ask for it on the console.
	if cfg.PBSUsername != "" && cfg.PBSPassword == "" {
		cfg.PBSPassword, err = clientcommon.PromptPassword(fmt.Sprintf("Password for %s: ", cfg.PBSUsername))
		if err != nil {
			fatalError("reading PBS password", err)
		}
	}

	L := clientcommon.Locking{}

	lock_ok := L.AcquireProcessLock()
	if !lock_ok {

		fmt.Fprintln(os.Stderr, "Another backup job is running; backup jobs run one at a time. Try again when it has finished.")
		os.Exit(2)
	}
	defer L.ReleaseProcessLock()

	if cfg.SysTray {
		machinebackuplib.SysTraySetup()
	}

	// Read and unlock -keyfile (prompting for a passphrase when the key file
	// is scrypt/PBKDF2 protected and none was given). Left nil for a plain
	// snapshot.
	if cfg.Crypt, err = clientcommon.LoadCryptConfig(cfg.KeyFile, cfg.KeyFilePassphrase); err != nil {
		fatalError("encryption key", err)
	}

	// One progress line per whole percent (block lines come every 4 MiB),
	// every other message as it comes.
	jp := &machinebackuplib.JobProgress{}
	cfg.Progress = jp
	counts := func() string {
		return fmt.Sprintf("new %d, reused %d (%d blank), new/changed data %s, transferred %s, free space not read %s",
			jp.NewChunks.Load(), jp.ReusedChunks.Load(), jp.BlankChunks.Load(),
			machinebackuplib.BytesToString(int64(jp.UploadedBytes.Load())), machinebackuplib.BytesToString(int64(jp.SentBytes())),
			machinebackuplib.BytesToString(int64(jp.FreeSkipped.Load())))
	}
	lastPct := -1
	start := time.Now()
	result, err := machinebackuplib.Backup(cfg, func(percentage float64, message string) bool {
		if !strings.Contains(message, ": Block ") {
			fmt.Printf("%s\n", message)
		} else if p := int(percentage * 100); p != lastPct {
			lastPct = p
			fmt.Printf("%3d%%  %s  (%s)\n", p, message, counts())
		}
		return false
	})
	// The notification e-mail (-mail-*): sent for a failed run too, before
	// the exit.
	var readErrors []string
	for _, r := range jp.Unreadable() {
		readErrors = append(readErrors, fmt.Sprintf("%s at byte %d (%d bytes)", r.Disk, r.Offset, r.Length))
	}
	mailSent := sendResultMail(cfg, clientcommon.MailCtx{
		NewChunks: jp.NewChunks.Load(), ReusedChunks: jp.ReusedChunks.Load(), Datastore: cfg.Datastore,
		Error: err, ReadErrors: readErrors, Hostname: hostnameOrEmpty(), StartTime: start, EndTime: time.Now(),
	})
	if err != nil {
		fatalError("backup", err)
	}
	// A failed backup has no snapshot to note it on; a good one gets the mail
	// problem in its PBS comment, where it is seen when the backup is viewed.
	if !mailSent {
		if nerr := result.AddNote(clientcommon.MailFailedNote); nerr != nil {
			fmt.Fprintln(os.Stderr, "Cannot add the mail warning to the PBS snapshot comment: "+nerr.Error())
		} else {
			fmt.Println("Mail warning added to the PBS snapshot comment.")
		}
	}
	for _, disk := range result.Disks {
		fmt.Printf("Backed up disk %d (%d bytes)\n", disk.Index, disk.Size)
	}
	fmt.Printf("Done in %s: %s read; %s\n", time.Since(start).Round(time.Second),
		machinebackuplib.BytesToString(int64(jp.BytesTotal.Load())), counts())
	// Sectors the disk could not read are zeros in the image: the snapshot is
	// kept, but say where and exit 3 so a scheduled run is noticed.
	if bad := jp.Unreadable(); len(bad) > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: %s of the disk could not be read and is zeros in the image:\n",
			machinebackuplib.BytesToString(int64(jp.UnreadableBytes.Load())))
		for _, r := range bad {
			fmt.Fprintf(os.Stderr, "  %s at byte %d (%d bytes)\n", r.Disk, r.Offset, r.Length)
		}
		os.Exit(3)
	}
}
