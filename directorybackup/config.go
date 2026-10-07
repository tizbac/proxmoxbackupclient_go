package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"pbscommon"
	"strings"
)

type MailSendConfig struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type MailTemplate struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type SMTPConfig struct {
	Host     string           `json:"host"`
	Port     string           `json:"port"`
	Username string           `json:"username"`
	Password string           `json:"password"`
	Insecure bool             `json:"insecure"`
	Mails    []MailSendConfig `json:"mails"`
	Template *MailTemplate    `json:"template"`
}

type Config struct {
	BaseURL          string      `json:"baseurl"`
	CertFingerprint  string      `json:"certfingerprint"`
	AuthID           string      `json:"authid"`
	Secret           string      `json:"secret"`
	PBSUsername      string      `json:"pbs-username"`
	PBSPassword      string      `json:"pbs-password"`
	Datastore        string      `json:"datastore"`
	Namespace        string      `json:"namespace"`
	BackupID         string      `json:"backup-id"`
	BackupSourceDir  string      `json:"backupdir"`
	BackupSourceDirs []string    `json:"backupdirs"`
	BackupStreamName string      `json:"backupstreamname"`
	PxarOut          string      `json:"pxarout"`
	SMTP             *SMTPConfig `json:"smtp"`
	UseVSS           bool        `json:"usevss"`

	// KeyFile is a Proxmox Backup Server encryption key (JSON, as produced by
	// `proxmox-backup-client key create`). When set, chunks are AES-256-GCM
	// encrypted and the snapshot manifest is signed — the same layout as
	// `proxmox-backup-client backup --crypt-mode encrypt --keyfile`.
	KeyFile string `json:"keyfile"`

	// KeyFilePassphrase unlocks a KeyFile that was created with a scrypt or
	// PBKDF2 KDF. Left empty, the passphrase is asked for on the console.
	// Ignored for `--kdf none` key files, which need nothing.
	KeyFilePassphrase string `json:"keyfilepassphrase"`

	// ChangeDetectionMode is legacy (default), data or metadata, as for
	// proxmox-backup-client --change-detection-mode. data and metadata write a
	// split archive (backup.mpxar.didx + backup.ppxar.didx); metadata reads
	// every file like data until reuse of unchanged files is implemented.
	ChangeDetectionMode string `json:"change-detection-mode"`
}

// Dirs returns every configured source directory: "backupdir" first (if set),
// then the entries of "backupdirs", without duplicates. With more than one,
// each directory is backed up as its own backup group.
func (c *Config) Dirs() []string {
	var dirs []string
	seen := map[string]bool{}
	for _, d := range append([]string{c.BackupSourceDir}, c.BackupSourceDirs...) {
		if d != "" && !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// dirListFlag is a repeatable string flag (-backupdir a -backupdir b).
type dirListFlag []string

func (d *dirListFlag) String() string     { return strings.Join(*d, ",") }
func (d *dirListFlag) Set(v string) error { *d = append(*d, v); return nil }

func (c *Config) valid() bool {
	// Authentication is either an API token (authid+secret) or a PBS
	// username (ticket login). The ticket password may be omitted: in that
	// case the CLI asks for it interactively before connecting.
	authOK := (c.AuthID != "" && c.Secret != "") || c.PBSUsername != ""
	baseValid := c.BaseURL != "" && authOK && c.Datastore != "" && (len(c.Dirs()) > 0 || c.BackupStreamName != "")
	if !baseValid {
		return baseValid
	}

	if err := pbscommon.ValidateChangeDetectionMode(c.ChangeDetectionMode); err != nil {
		fmt.Println(err)
		return false
	}

	if c.SMTP != nil {
		mailCfgValid := c.SMTP.Host != "" && c.SMTP.Port != "" && c.SMTP.Username != "" && c.SMTP.Password != ""
		if len(c.SMTP.Mails) == 0 {
			return false
		}
		for i := range c.SMTP.Mails {
			mailCfgValid = mailCfgValid && (c.SMTP.Mails[i].From != "" && c.SMTP.Mails[i].To != "")
		}
		return mailCfgValid
	}

	return true
}

func loadConfig() *Config {
	// Define flags
	baseURLFlag := flag.String("baseurl", "", "Base URL for the proxmox backup server, example: https://192.168.1.10:8007")
	certFingerprintFlag := flag.String("certfingerprint", "", "Certificate fingerprint for SSL connection, example: ea:7d:06:f9...")
	authIDFlag := flag.String("authid", "", "Authentication ID (PBS Api token)")
	secretFlag := flag.String("secret", "", "Secret for authentication")
	pbsUsernameFlag := flag.String("pbsusername", "", "PBS username for ticket login (with -pbspassword; overrides -authid/-secret)")
	pbsPasswordFlag := flag.String("pbspassword", "", "PBS password for ticket login (with -pbsusername)")
	datastoreFlag := flag.String("datastore", "", "Datastore name")
	namespaceFlag := flag.String("namespace", "", "Namespace (optional)")
	backupIDFlag := flag.String("backup-id", "", "Backup ID (optional - if not specified, the hostname is used as the default)")
	var backupSourceDirFlags dirListFlag
	flag.Var(&backupSourceDirFlags, "backupdir", "Backup source directory, must not be symlink. Repeat the flag to back up several directories; each becomes its own backup group (backup-id <base>_<path>)")
	backupStreamNameFlag := flag.String("backupstream", "", "Filename for stream backup")
	pxarOutFlag := flag.String("pxarout", "", "Output PXAR archive for debug purposes (optional)")
	noVSSFlag := flag.Bool("novss", false, "Disable VSS ( For filesystems that don't support it, for example veracrypt )")
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

	changeDetectionFlag := flag.String("change-detection-mode", "", "legacy (default), data or metadata: data/metadata write a split archive (backup.mpxar.didx + backup.ppxar.didx)")

	configFile := flag.String("config", "", "Path to JSON config file. If this flag is provided all the others will override the loaded config file")

	// Parse command line flags
	flag.Parse()

	config := &Config{
		UseVSS: true,
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

	if *changeDetectionFlag != "" {
		config.ChangeDetectionMode = *changeDetectionFlag
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
	if len(backupSourceDirFlags) > 0 {
		config.BackupSourceDir = backupSourceDirFlags[0]
		config.BackupSourceDirs = backupSourceDirFlags[1:]
	}

	if *backupStreamNameFlag != "" {
		config.BackupStreamName = *backupStreamNameFlag
	}
	if *pxarOutFlag != "" {
		config.PxarOut = *pxarOutFlag
	}
	if *noVSSFlag {
		config.UseVSS = false
	}
	if *keyFileFlag != "" {
		config.KeyFile = *keyFileFlag
	}
	if *keyFilePassphraseFlag != "" {
		config.KeyFilePassphrase = *keyFilePassphraseFlag
	}

	initSmtpConfigIfNeeded := func() {
		if config.SMTP == nil {
			config.SMTP = &SMTPConfig{}
		}
	}
	initMailConfsIfNeeded := func() {
		initSmtpConfigIfNeeded()
		if len(config.SMTP.Mails) == 0 {
			config.SMTP.Mails = append(config.SMTP.Mails, MailSendConfig{})
		}
	}
	initTemplateIfNeeded := func() {
		initSmtpConfigIfNeeded()
		if config.SMTP.Template == nil {
			config.SMTP.Template = &MailTemplate{}
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
