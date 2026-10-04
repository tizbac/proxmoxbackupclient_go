package main

import (
	"clientcommon"

	"encoding/json"
	"flag"
	"fmt"

	"machinebackuplib"

	"github.com/tawesoft/golib/v2/dialog"
	"os"
	"runtime"

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

func fatalError(msg string, err error) {
	fmt.Fprintf(os.Stderr, "Fatal error: %s: %v\n", msg, err)
	os.Exit(1)
}

func main() {

	cfg := loadConfig()

	if ok := cfg.Valid(); !ok {
		if runtime.GOOS == "windows" {
			usage := "All options are mandatory:\n"
			flag.VisitAll(func(f *flag.Flag) {
				usage += "-" + f.Name + " " + f.Usage + "\n"
			})
			dialog.Error(usage)
		} else {
			fmt.Println("All options are mandatory")

			flag.PrintDefaults()
		}
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

		dialog.Error("Backup jobs need to run exclusively, please wait until the previous job has finished")
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

	result, err := machinebackuplib.Backup(cfg, func(percentage float64, message string) bool {
		return false
	})
	if err != nil {
		fatalError("backup", err)
	}
	for _, disk := range result.Disks {
		fmt.Printf("Backed up disk %d (%d bytes)\n", disk.Index, disk.Size)
	}
}
