package directorybackup

import (
	"clientcommon"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"

	"pbscommon"
	"strings"
)

// version is salvanza-cli's, which it sets with SetVersion (the build stamps
// it with -ldflags "-X main.version=..."); -version prints it.
var version string

// SetVersion sets the version -version prints.
func SetVersion(v string) { version = v }

// usageSummary heads -help's text.
const usageSummary = "Backs up folders, or standard input (-backupstream), to a Proxmox Backup Server\nas a host snapshot."

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

	// NoMTime records 0 (1970-01-01) as every file's and folder's time instead
	// of the real one, so entries whose time changes but whose content does not
	// leave the archive bytes, and so its chunks, unchanged. Restored items get
	// 1970-01-01. Metadata change detection reads every file with it.
	NoMTime bool `json:"no-mtime"`

	// ExcludeList holds the patterns of files and folders to leave out, the
	// GUI's exclusions (pbscommon's PXARArchive.ExcludeList): a pattern
	// without / or \ matches a name at any depth, one with a separator is
	// taken from each folder backed up; not case sensitive. -exclude adds to
	// the config file's list.
	ExcludeList []string `json:"exclude"`

	// Compression is how hard chunks are compressed before they go to PBS:
	// fastest, default (zstd level 3), better or best; empty is default.
	Compression string `json:"compression"`

	// ReadAhead is how many files of each folder are read in the background
	// ahead of the one being archived (pbscommon's PXARArchive.PrefetchWorkers),
	// 1 to MaxReadAhead; 0, the default, reads one file at a time. The app's
	// "Multi-threaded backup reading". The archive is the same either way.
	ReadAhead int `json:"read-ahead"`

	// Progress draws a progress bar while the backup runs. The folder is
	// walked once beforehand to count its files and bytes; a stream
	// (-backupstream) has no size to count, so its bar is indeterminate.
	Progress bool `json:"progress"`

	// The speed limits, "upload-limit", "download-limit" and
	// "disk-read-limit" in MB/s; a flag overrides the file.
	clientcommon.SpeedLimits

	// "shutdown-after": turn the computer off once the backup has run.
	clientcommon.AfterRun
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

// MaxReadAhead is the most files -read-ahead may read at once, as in the app.
const MaxReadAhead = 32

func (c *Config) valid() bool { return len(c.problems()) == 0 }

// problems says what stops a run with these settings, one line each: the
// options that are missing, then the ones that are wrong. The missing ones are
// named (tizbac/proxmoxbackupclient_go#55 only had "All options are
// mandatory").
func (c *Config) problems() []string {
	var p []string
	if m := c.missing(); len(m) > 0 {
		p = append(p, "Missing options: "+strings.Join(m, ", "))
	}
	if err := pbscommon.ValidateChangeDetectionMode(c.ChangeDetectionMode); err != nil {
		p = append(p, err.Error())
	}
	if c.SMTP != nil {
		if problem := c.SMTP.problem(); problem != "" {
			p = append(p, problem)
		}
	}
	if problem := c.SpeedLimitProblem(); problem != "" {
		p = append(p, problem)
	}
	if !pbscommon.ValidCompressionLevel(c.Compression) {
		p = append(p, fmt.Sprintf("-compression is %q: use fastest, default, better or best", c.Compression))
	}
	if len(c.ExcludeList) > 0 && len(c.Dirs()) == 0 && c.BackupStreamName != "" {
		p = append(p, "-exclude leaves out files and folders of -backupdir; it does not apply to -backupstream")
	}
	if c.ReadAhead > 0 && len(c.Dirs()) == 0 && c.BackupStreamName != "" {
		p = append(p, "-read-ahead reads the files of -backupdir; it does not apply to -backupstream")
	}
	if c.ReadAhead < 0 || c.ReadAhead > MaxReadAhead {
		p = append(p, fmt.Sprintf("-read-ahead is %d: use 0 (off) to %d files", c.ReadAhead, MaxReadAhead))
	}
	for _, pat := range c.ExcludeList {
		if problem := excludePatternProblem(pat); problem != "" {
			p = append(p, problem)
		}
	}
	return p
}

// excludePatternProblem says why an -exclude pattern can never match, or "".
// The archive writer matches patterns with path.Match after turning \ into /
// and ignores a malformed one, so "[abc" would leave nothing out unnoticed.
// An empty pattern is skipped there, as a blank line in the GUI's list is.
func excludePatternProblem(pat string) string {
	return excludePatternProblemOn(pat, runtime.GOOS)
}

func excludePatternProblemOn(pat, goos string) string {
	// On Windows "C:\Temp\" as the last argument passes C:\Temp" (and a
	// later one swallows the rest of the line), which matches nothing; a
	// file name there cannot hold a quote anyway.
	if goos == "windows" && strings.Contains(pat, `"`) {
		return fmt.Sprintf(`-exclude came through as [%s], with a quote in it: on Windows a path ending in \ inside quotes passes a quote and everything after it. Leave out the last \ ("C:\Temp") or the quotes`, pat)
	}
	if !strings.ContainsAny(pat, "*?[") {
		return ""
	}
	if _, err := path.Match(strings.ToLower(strings.ReplaceAll(pat, `\`, "/")), ""); err != nil {
		return fmt.Sprintf("-exclude %q is not a valid pattern (an unclosed [ or a bad range)", pat)
	}
	return ""
}

// missing names the options a run needs and does not have. Authentication is
// either an API token (authid+secret) or a PBS username (ticket login); the
// ticket password may be omitted, and is then asked for before connecting.
func (c *Config) missing() []string {
	var m []string
	if c.BaseURL == "" {
		m = append(m, "-baseurl")
	}
	if (c.AuthID == "" || c.Secret == "") && c.PBSUsername == "" {
		switch {
		case c.AuthID != "":
			m = append(m, "-secret (for -authid)")
		case c.Secret != "":
			m = append(m, "-authid (for -secret)")
		default:
			m = append(m, "-authid and -secret, or -pbsusername")
		}
	}
	if c.Datastore == "" {
		m = append(m, "-datastore")
	}
	if len(c.Dirs()) == 0 && c.BackupStreamName == "" {
		m = append(m, "-backupdir (or -backupstream)")
	}
	return m
}

// problem says what is missing from the mail settings, or "". A relay that
// takes mail without a login needs no username or password: leave both empty
// (tizbac/proxmoxbackupclient_go#74, where an empty username only printed
// the help).
func (s *SMTPConfig) problem() string {
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
	var excludeFlags dirListFlag
	flag.Var(&excludeFlags, "exclude", "Leave out files and folders matching this pattern; repeat for several. Without / or \\ it matches a name at any depth (*.tmp, node_modules); with one it is taken from the folder backed up (logs/*.log, C:\\Users\\alice\\Temp). Not case sensitive. Adds to the config file's \"exclude\" list")
	compressionFlag := flag.String("compression", "", "How hard chunks are compressed before they go to PBS: fastest, default (zstd level 3), better or best (default: default). Encrypted backups are compressed too, before encryption")
	readAheadFlag := flag.Int("read-ahead", 0, "Read up to this many files of each folder in the background ahead of the one being archived, 1 to 32, which helps when reading from the disk is the slow part; 0 or left out reads one file at a time. Each file read ahead, up to 128 MB, is held in memory until it is archived, so a folder can hold up to this many at a time (and its parent folders theirs, while it is backed up). The archive is the same either way")
	noMTimeFlag := flag.Bool("no-mtime", false, "Record 1970-01-01 as every file and folder time instead of the real one, so time-only changes (a folder whose contents come and go) do not change the archive and re-upload its chunks; restored items get 1970-01-01 (advanced, optional)")
	progressFlag := flag.Bool("progress", false, "Draw a progress bar on standard error while the backup runs. The folder is walked once beforehand to count its files and bytes (-backupstream has no size to count: its bar is indeterminate). The per-chunk lines are left out; other messages go above the bar")

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
	versionFlag := flag.Bool("version", false, "Print the version and exit")
	applySpeedLimitFlags := clientcommon.SpeedLimitFlags(flag.CommandLine)
	applyShutdownAfterFlag := clientcommon.ShutdownAfterFlag(flag.CommandLine)

	// A stray argument, an unknown option or (on Windows) a quote in a path
	// stops here with what is wrong; -help prints the usage.
	clientcommon.ParseFlags(flag.CommandLine, os.Args[1:], usageSummary, "backupdir", "pxarout", "keyfile", "config")
	if *versionFlag {
		fmt.Println(clientcommon.VersionString(clientcommon.ToolName(), version))
		os.Exit(0)
	}

	config := &Config{
		UseVSS: true,
	}
	pbscommon.ApplyPBSEnvVars(&config.BaseURL, &config.AuthID, &config.Secret, &config.Datastore, &config.CertFingerprint)
	if *configFile != "" {
		file, err := os.ReadFile(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
			os.Exit(1)
		}
		err = json.Unmarshal(file, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing config file: %v\n", err)
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
	if *noMTimeFlag {
		config.NoMTime = true
	}
	if *progressFlag {
		config.Progress = true
	}
	config.ExcludeList = append(config.ExcludeList, excludeFlags...)
	if *compressionFlag != "" {
		config.Compression = *compressionFlag
	}
	// Given at all, -read-ahead overrides the file, -read-ahead 0 too.
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "read-ahead" {
			config.ReadAhead = *readAheadFlag
		}
	})
	applySpeedLimitFlags(&config.SpeedLimits)
	applyShutdownAfterFlag(&config.AfterRun)

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
