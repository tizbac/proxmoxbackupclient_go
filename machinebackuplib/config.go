package machinebackuplib

import "pbscommon"

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
	BaseURL         string      `json:"baseurl"`
	CertFingerprint string      `json:"certfingerprint"`
	AuthID          string      `json:"authid"`
	Secret          string      `json:"secret"`
	PBSUsername     string      `json:"pbs-username"`
	PBSPassword     string      `json:"pbs-password"`
	Ticket          string      `json:"ticket"`
	CSRFToken       string      `json:"csrf-token"`
	Datastore       string      `json:"datastore"`
	Namespace       string      `json:"namespace"`
	BackupID        string      `json:"backup-id"`
	BackupDevices   []string    `json:"backupdev"`
	SMTP            *SMTPConfig `json:"smtp"`
	SysTray         bool        `json:"systray"`
	BackupType      string      `json:"backuptype"`

	// KeyFile is a Proxmox Backup Server encryption key (JSON, as produced by
	// `proxmox-backup-client key create`). When set, every fixed-index chunk
	// is AES-256-GCM encrypted and the snapshot manifest is signed — the same
	// on-disk layout as `proxmox-backup-client backup --crypt-mode encrypt`.
	KeyFile string `json:"keyfile"`

	// KeyFilePassphrase unlocks a scrypt/PBKDF2 protected KeyFile. Empty means
	// the caller prompts for it.
	KeyFilePassphrase string `json:"keyfilepassphrase"`

	// Crypt is KeyFile after it has been read and unlocked. Callers (the CLI,
	// the GUI) fill this in — via clientcommon.LoadCryptConfig — so that the
	// passphrase prompt stays with whoever owns the console. It is never
	// serialized: a -config file carries KeyFile, not the unlocked key.
	Crypt *pbscommon.CryptConfig `json:"-"`
}

func (c *Config) Valid() bool {
	// Authentication is an API token (authid+secret), a PBS username
	// (ticket login; the password may be omitted and is then asked for
	// interactively by the CLI), or a pre-obtained session ticket.
	authOK := (c.AuthID != "" && c.Secret != "") || c.PBSUsername != "" || c.Ticket != ""
	baseValid := c.BaseURL != "" && authOK && c.Datastore != "" && len(c.BackupDevices) > 0
	if !baseValid {
		return baseValid
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
