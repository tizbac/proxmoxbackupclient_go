package machinebackuplib

import (
	"fmt"
	"pbscommon"
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
	SysTray         bool        `json:"systray"` // old -systray: read so config files keep working; there is no tray icon
	BackupType      string      `json:"backuptype"`
	// Comment is sent as-is as the PBS snapshot's manifest comment.
	Comment string `json:"comment,omitempty"`

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

	// Progress, when set, receives the job's running figures (chunks new,
	// reused and blank, bytes uploaded, current disk and partition) for a
	// caller to sample on its own timer. Nil for the CLI.
	Progress *JobProgress `json:"-"`

	// ReadFreeSpace reads every sector of the disk, free space included (a
	// forensic copy). By default the free clusters of each snapshotted NTFS
	// volume, and its paging, hibernation and swap files, are sent as zeros
	// without being read.
	ReadFreeSpace bool `json:"readfreespace,omitempty"`

	// LiveRead (Linux) reads a disk's mounted partitions live, without a
	// block snapshot, so the image may be inconsistent: files that change
	// during the backup can be caught half written. Set only when the user
	// has turned snapshots off for a machine Backup Set and accepted that.
	// Without it a mounted disk needs a snapshot (elastio-snap or dattobd,
	// and root), and the backup stops if none can be taken. Windows always
	// reads a volume in use through VSS and ignores it.
	LiveRead bool `json:"liveread,omitempty"`

	// SkipPartitions are the partitions to leave out of each disk, keyed by
	// the device as BackupDevices gives it. A partition left out is not read
	// or snapshotted and goes into the image as zeros (PartitionSkip).
	SkipPartitions map[string][]PartitionSkip `json:"skippartitions,omitempty"`
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
		return c.SMTP.Problem() == ""
	}

	return true
}

// Problem says what is missing from the mail settings, or "". A relay that
// takes mail without a login needs no username or password: leave both empty
// (tizbac/proxmoxbackupclient_go#74, where an empty username only printed
// the help).
func (s *SMTPConfig) Problem() string {
	if s.Host == "" || s.Port == "" {
		return `smtp: "host" and "port" are needed`
	}
	if s.Username == "" && s.Password != "" {
		return `smtp: a "password" needs a "username"`
	}
	if len(s.Mails) == 0 {
		return `smtp: "mails" needs at least one "from" and "to"`
	}
	for i, m := range s.Mails {
		if m.From == "" || m.To == "" {
			return fmt.Sprintf(`smtp: mails[%d] needs both "from" and "to"`, i)
		}
	}
	return ""
}
