package machinebackuplib


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
	Datastore       string      `json:"datastore"`
	Namespace       string      `json:"namespace"`
	BackupID        string      `json:"backup-id"`
	BackupDevices   []string    `json:"backupdev"`
	SMTP            *SMTPConfig `json:"smtp"`
	SysTray         bool        `json:"systray"`
	BackupType      string      `json:"backuptype"`
}

func (c *Config) Valid() bool {
	baseValid := c.BaseURL != "" && c.AuthID != "" && c.Secret != "" && c.Datastore != "" && len(c.BackupDevices) > 0
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

