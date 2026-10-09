package machinebackup

import (
	"clientcommon"

	"encoding/json"
	"flag"
	"fmt"
	"log"

	"machinebackuplib"

	"os"
	"runtime"
	"strings"
	"time"

	"pbscommon"
	"retry"
)

// version is salvanza-cli's, which it sets with SetVersion (the build stamps
// it with -ldflags "-X main.version=..."); -version prints it.
var version string

// SetVersion sets the version -version prints.
func SetVersion(v string) { version = v }

// usageSummary heads -help's text.
const usageSummary = "Backs up whole disks to a Proxmox Backup Server as a host or vm snapshot."

// reconnectWaits are the waits before each new attempt after a lost
// connection (retry.ReconnectWaits; a test sets them short).
var reconnectWaits = retry.ReconnectWaits

type arrayFlags []string

func (i *arrayFlags) String() string {
	return fmt.Sprintf("%v", *i)
}

// Set is an implementation of the flag.Value interface
func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// loadConfig reads the command line and the -config file. The speed limits
// and -shutdown-after come back on their own: machinebackuplib.Config is the
// app's too, which sets those from its own settings.
func loadConfig() (*machinebackuplib.Config, clientcommon.SpeedLimits, clientcommon.AfterRun, bool) {
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
	liveReadFlag := flag.Bool("liveread", false, "Linux: read mounted partitions live, without a snapshot, so elastio-snap or dattobd is not needed. The image may be inconsistent: files that change during the backup can be caught half written. Windows always reads a volume in use through VSS")
	compressionFlag := flag.String("compression", "", "How hard chunks are compressed before they go to PBS: fastest, default (zstd level 3), better or best (default: default). Encrypted backups are compressed too, before encryption")
	readFreeSpaceFlag := flag.Bool("readfreespace", false, "Windows: also read free space and the paging/hibernation files (a forensic copy); by default they go into the image as zeros without being read")
	progressFlag := flag.Bool("progress", false, "Draw a progress bar on standard error while the backup runs, with the percentage of the data actually read (free space and partitions left out are counted as done at once, as in the app). Before the size is known the bar is indeterminate. The per-chunk lines are left out; other messages go above the bar")
	var skipparts arrayFlags
	flag.Var(&skipparts, "skippartition", "Can be specified multiple times. Leave a partition out of a whole-disk backup: <device>:<partition number>, e.g. \\\\.\\PhysicalDrive0:4 or /dev/sda:3. It is not read or snapshotted and goes into the image as zeros: a restore gives it back empty and unformatted")
	sysTrayFlag := flag.Bool("systray", false, "Does nothing: accepted so older scripts keep working (the tray icon was taken out of the command-line tools)")
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
	versionFlag := flag.Bool("version", false, "Print the version and exit")
	applySpeedLimitFlags := clientcommon.SpeedLimitFlags(flag.CommandLine)
	applyShutdownAfterFlag := clientcommon.ShutdownAfterFlag(flag.CommandLine)

	// A stray argument, an unknown option or (on Windows) a quote in a path
	// stops here with what is wrong; -help prints the usage.
	clientcommon.ParseFlags(flag.CommandLine, os.Args[1:], usageSummary, "backupdev", "keyfile", "config")
	if *versionFlag {
		fmt.Println(clientcommon.VersionString(clientcommon.ToolName(), version))
		os.Exit(0)
	}

	config := &machinebackuplib.Config{
		BackupType: "host",
	}
	pbscommon.ApplyPBSEnvVars(&config.BaseURL, &config.AuthID, &config.Secret, &config.Datastore, &config.CertFingerprint)
	var limits clientcommon.SpeedLimits
	var after clientcommon.AfterRun
	if *configFile != "" {
		file, err := os.ReadFile(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
			os.Exit(1)
		}
		err = json.Unmarshal(file, config)
		if err == nil {
			err = json.Unmarshal(file, &limits)
		}
		if err == nil {
			err = json.Unmarshal(file, &after)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing config file: %v\n", err)
			os.Exit(1)
		}
	}
	applySpeedLimitFlags(&limits)
	applyShutdownAfterFlag(&after)

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
	// -backupdev replaces the config file's device list only when given;
	// otherwise the file's "backupdev" is kept.
	if len(backupdevs) > 0 {
		config.BackupDevices = backupdevs
	}
	if *readFreeSpaceFlag {
		config.ReadFreeSpace = true
	}
	if *liveReadFlag {
		config.LiveRead = true
	}
	if *compressionFlag != "" {
		config.CompressionLevel = *compressionFlag
	}
	// -skippartition adds to the config file's "skippartitions".
	for _, v := range skipparts {
		dev, skip, err := machinebackuplib.ParseSkipPartition(v)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if config.SkipPartitions == nil {
			config.SkipPartitions = map[string][]machinebackuplib.PartitionSkip{}
		}
		config.SkipPartitions[dev] = append(config.SkipPartitions[dev], skip)
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

	return config, limits, after, *progressFlag
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
	defer func() { _ = client.Quit() }() // the mails are sent or reported by then
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

// problems says what stops a run with cfg, one line each: the options that
// are missing (named, tizbac/proxmoxbackupclient_go#55), then the ones that
// are wrong. None means machinebackuplib's Valid holds too.
func problems(cfg *machinebackuplib.Config) []string {
	var m, p []string
	if cfg.BaseURL == "" {
		m = append(m, "-baseurl")
	}
	if (cfg.AuthID == "" || cfg.Secret == "") && cfg.PBSUsername == "" && cfg.Ticket == "" {
		switch {
		case cfg.AuthID != "":
			m = append(m, "-secret (for -authid)")
		case cfg.Secret != "":
			m = append(m, "-authid (for -secret)")
		default:
			m = append(m, "-authid and -secret, or -pbsusername")
		}
	}
	if cfg.Datastore == "" {
		m = append(m, "-datastore")
	}
	if len(cfg.BackupDevices) == 0 {
		m = append(m, "-backupdev")
	}
	if len(m) > 0 {
		p = append(p, "Missing options: "+strings.Join(m, ", "))
	}
	if cfg.BackupType != "host" && cfg.BackupType != "vm" {
		p = append(p, fmt.Sprintf("-type is %q: use host or vm", cfg.BackupType))
	}
	if !pbscommon.ValidCompressionLevel(cfg.CompressionLevel) {
		p = append(p, fmt.Sprintf("-compression is %q: use fastest, default, better or best", cfg.CompressionLevel))
	}
	if cfg.SMTP != nil {
		if s := cfg.SMTP.Problem(); s != "" {
			p = append(p, s)
		}
	}
	return p
}

func hostnameOrEmpty() string {
	h, _ := os.Hostname()
	return h
}

// machineProgressLocation is the line under the bar: what the job is doing and
// where it is reading, from the same figures the app's progress card shows.
func machineProgressLocation(jp *machinebackuplib.JobProgress) string {
	w := jp.Where()
	switch w.Phase {
	case "connecting":
		return "connecting to PBS"
	case "snapshot":
		if w.Disk != "" {
			return "taking a snapshot of " + w.Disk
		}
		return "taking a snapshot"
	case "finishing":
		return "finishing"
	}
	if w.Disk == "" {
		return "preparing"
	}
	s := w.Disk
	if w.DiskCount > 1 {
		s = fmt.Sprintf("disk %d/%d %s", w.DiskNo, w.DiskCount, w.Disk)
	}
	if w.DiskModel != "" {
		s += " (" + w.DiskModel + ")"
	}
	if w.Partition != "" {
		s += " · " + w.Partition
	}
	return s
}

func fatalError(msg string, err error) {
	fmt.Fprintf(os.Stderr, "Fatal error: %s: %v\n", msg, err)
	os.Exit(1)
}

func Main() {

	cfg, limits, after, showProgress := loadConfig()

	// Errors go to stderr with a non-zero exit, never a message box: a box
	// waits for a click that an unattended scheduled run never gets.
	p := problems(cfg)
	if problem := limits.SpeedLimitProblem(); problem != "" {
		p = append(p, problem)
	}
	if len(p) > 0 {
		for _, line := range p {
			fmt.Fprintln(os.Stderr, line)
		}
		fmt.Fprintf(os.Stderr, "Options can also come from -config <file> and the PBS_* environment variables. Run %s -help for all of them.\n", clientcommon.ToolName())
		os.Exit(1)
	}
	if line := limits.ApplySpeedLimits(); line != "" {
		fmt.Println(line)
	}
	if cfg.LiveRead && runtime.GOOS == "windows" {
		fmt.Println("-liveread has no effect on Windows: a volume in use is always read through VSS.")
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

	// The tray icon was taken out of the command-line tools (bfdc27f); the
	// option and the config file's "systray" stay accepted, so a script that
	// gives them keeps working, and the run says they do nothing.
	if cfg.SysTray {
		fmt.Println("-systray does nothing: the command-line tools show no tray icon.")
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
	// -progress: the FastCopy bar, on standard error (standard output stays
	// as it was), fed the same read percentage the app's progress card uses
	// (JobProgress.ReadProgress), so free space and partitions left out count
	// as done at once. Until the disk sizes are known the bar is
	// indeterminate. SetQuiet drops the per-chunk lines.
	var progressBar *clientcommon.Progress
	var restoreProgress func()
	var restoreLog func()
	if showProgress {
		progressBar = clientcommon.NewProgress(os.Stderr, true)
		restoreProgress = progressBar.CaptureStdout()
		if progressBar.Active() {
			oldLog := log.Writer()
			log.SetOutput(progressBar.Writer())
			restoreLog = func() { log.SetOutput(oldLog) }
		}
		progressBar.StartCopy(0, 0)
		jp.SetQuiet(true)
	}
	counts := func() string {
		s := fmt.Sprintf("new %d, reused %d (%d blank), new/changed data %s, transferred %s, free space not read %s",
			jp.NewChunks.Load(), jp.ReusedChunks.Load(), jp.BlankChunks.Load(),
			machinebackuplib.BytesToString(int64(jp.UploadedBytes.Load())), machinebackuplib.BytesToString(int64(jp.SentBytes())),
			machinebackuplib.BytesToString(int64(jp.FreeSkipped.Load())))
		if n := jp.LeftOut.Load(); n > 0 {
			s += ", partitions left out " + machinebackuplib.BytesToString(int64(n))
		}
		return s
	}
	lastPct := -1
	start := time.Now()
	progress := func(percentage float64, message string) bool {
		if progressBar != nil && progressBar.Active() {
			// The bar shows the percentage and its own counters; the block
			// lines would only scroll it.
			if !strings.Contains(message, ": Block ") {
				progressBar.Line("%s", message)
			}
			return false
		}
		if !strings.Contains(message, ": Block ") {
			fmt.Printf("%s\n", message)
		} else if p := int(percentage * 100); p != lastPct {
			lastPct = p
			fmt.Printf("%3d%%  %s  (%s)\n", p, message, counts())
		}
		return false
	}

	// With the bar on, a ticker samples the job's figures and feeds the bar,
	// as the app's progress card does.
	sampleProgress := func() {
		done, total := jp.ReadProgress()
		if total > 0 {
			progressBar.SetTotal(0, int64(total))
		}
		progressBar.SetBytesDone(int64(done))
		progressBar.Current(machineProgressLocation(jp))
	}
	var tickerDone chan struct{}
	var tickerExited chan struct{}
	if progressBar != nil {
		tickerDone = make(chan struct{})
		tickerExited = make(chan struct{})
		go func() {
			defer close(tickerExited)
			t := time.NewTicker(250 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-tickerDone:
					return
				case <-t.C:
					sampleProgress()
				}
			}
		}()
	}
	stopTicker := func() {
		if tickerDone == nil {
			return
		}
		select {
		case <-tickerDone:
		default:
			close(tickerDone)
		}
		<-tickerExited
		tickerDone = nil
		if progressBar != nil {
			sampleProgress() // the last tick may be older than the end of the run
		}
	}
	// A lost connection (a mobile link dropping, PBS restarting) ends the PBS
	// session, and with it every chunk sent in it, so the backup starts again
	// with a new session after a wait that grows each time, as the app does
	// (retry.Reconnectable, retry.ReconnectWaits). The wait also lets PBS
	// release the backup group; an attempt made while it still holds it fails
	// at once and waits again.
	// PBS refuses a second snapshot of a group in the same second (two runs
	// chained in a script): that is waited out once, without counting as an
	// attempt. A second refusal points at the clock, and the error says so.
	var result *machinebackuplib.BackupResult
	waitedSecond := false
	for attempt := 1; ; attempt++ {
		result, err = machinebackuplib.Backup(cfg, progress)
		if err != nil && !waitedSecond && pbscommon.IsBackupTimeTaken(err) {
			waitedSecond = true
			attempt--
			if progressBar != nil && progressBar.Active() {
				progressBar.Line("PBS already has a backup of this group from this second; starting again in the next second.")
			} else {
				fmt.Fprintln(os.Stderr, "PBS already has a backup of this group from this second; starting again in the next second.")
			}
			jp.Reset()
			if progressBar != nil {
				progressBar.StartCopy(0, 0)
			}
			lastPct = -1
			pbscommon.WaitNextSecond()
			continue
		}
		if err == nil || attempt > len(reconnectWaits) || !retry.Reconnectable(err) {
			break
		}
		wait := reconnectWaits[attempt-1]
		if progressBar != nil && progressBar.Active() {
			progressBar.Line("Connection to the server lost on attempt %d of %d (%s): %v", attempt, len(reconnectWaits)+1, counts(), err)
			progressBar.Line("Starting again at %s with a new session; chunks sent in the lost one are sent again.", time.Now().Add(wait).Format("15:04:05"))
			progressBar.StartCopy(0, 0)
		} else {
			fmt.Fprintf(os.Stderr, "Connection to the server lost on attempt %d of %d (%s): %v\n", attempt, len(reconnectWaits)+1, counts(), err)
			fmt.Fprintf(os.Stderr, "Starting again at %s with a new session; chunks sent in the lost one are sent again.\n", time.Now().Add(wait).Format("15:04:05"))
		}
		jp.Reset()
		lastPct = -1
		time.Sleep(wait)
	}
	stopTicker()
	if progressBar != nil {
		// A good run is shown complete; a failed one keeps the figures it
		// reached, which say where it stopped.
		if err == nil {
			progressBar.Complete()
		}
		if restoreLog != nil {
			restoreLog()
		}
		restoreProgress()
		progressBar.Finish()
	}
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
		fmt.Fprintf(os.Stderr, "Fatal error: backup: %v\n", err)
		// -shutdown-after: the backup has run, and failed; exit 1 still says so.
		clientcommon.ShutdownAfterRun(after)
		os.Exit(1)
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
	bad := jp.Unreadable()
	if len(bad) > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: %s of the disk could not be read and is zeros in the image:\n",
			machinebackuplib.BytesToString(int64(jp.UnreadableBytes.Load())))
		for _, r := range bad {
			fmt.Fprintf(os.Stderr, "  %s at byte %d (%d bytes)\n", r.Disk, r.Offset, r.Length)
		}
	}
	// -shutdown-after: the backup has run and the mail has gone.
	clientcommon.ShutdownAfterRun(after)
	if len(bad) > 0 {
		os.Exit(3)
	}
}
